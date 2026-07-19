// Spec 121 R4 / R2(c): the net-effect already-merged predicate. ADR-0041
// §2(iii) (see .mindspec/adr/ADR-0041-gate-before-mutate.md) requires
// "already landed on origin/main" to be re-derived from CURRENT-state
// content equivalence, not from SHA ancestry alone, wherever the hosting
// workflow can discard a branch's SHAs entirely (a squash merge). Before
// this file, both the protected-main FinalizeEpic probe
// (internal/executor/mindspec_executor.go) and the doctor merged-carrier
// suppression (internal/lifecycle/finalize_orphans.go) tested SHA ancestry
// only — a documented blind spot (mindspec-3xqm item 1) that a squash-merged
// branch defeats by construction (its commits never appear in origin/main's
// history at all).
//
// NetEffectLanded is the ONE exported symbol both consumers route through
// (AC-17 anti-drift): a two-leg content evaluation —
//
//   - leg (a), tree subsumption: a three-way `git merge-tree --write-tree`
//     (base = merge-base(ref, target); ours = target; theirs = ref) is
//     landed iff the merge result's tree OID equals target's CURRENT tree
//     OID with no conflict. This is NET EFFECT, not historical patch
//     presence: a squash-merge is landed (target's tree already contains
//     ref's content, so re-applying it is a no-op); a squash-then-REVERT on
//     target is NOT landed (the revert removed the content from target's
//     current tree, so re-applying ref's diff would change it again);
//     a squash followed by unrelated later target changes is STILL landed
//     (the unrelated changes do not touch what ref introduced).
//   - leg (b), tracker-payload subsumption: reached ONLY when leg (a)
//     returns a definitive NOT-landed (never on a leg-(a) infra error) AND
//     ref's entire diff against the merge-base is confined to
//     .beads/issues.jsonl — the tracker-only carrier shape, derived from
//     the diff itself, never from the caller. Every id→status assertion
//     ref's export changes relative to the merge-base must already be
//     satisfied by target's CURRENT committed export — equal, or a
//     LATER status in the total order open < in_progress < closed. This is
//     what makes "a LATER superseding export" land as suppressed even
//     though leg (a) reads a superseding export as a tree conflict/mismatch.
//
// Exit-code trichotomy (panel O1's pin, load-bearing for both legs):
// `git merge-tree --write-tree` exit 0 means compare the written tree OID;
// exit 1 means CONFLICT — a leg-(a) NOT-landed answer (or, for a
// JSONL-confined diff, the fall-through into leg (b)) — NEVER an infra
// error; exit >= 2 (including "unknown option" on git < 2.38, where
// --write-tree does not exist) is an infra failure, ALWAYS propagated as an
// error, NEVER guessed into a boolean. Conflating exit 1 with infra would
// silently break the superseding-export suppression and R2(c)'s self-loop
// avoidance.
//
// ADR-0041 §2(iii) cites this predicate as the completing doctrine; see the
// amendment text for the pinned consequences restated above.
package gitutil

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// trackerJSONLPath is the tracker export path leg (b) recognizes as the
// tracker-only carrier shape. Hardcoded (not imported from
// internal/workspace) so this file stays a leaf — gitutil must not import
// any package above it in the dependency graph.
const trackerJSONLPath = ".beads/issues.jsonl"

// statusRank is leg (b)'s total order (ADR-0041 §2(iii) / spec 121 R4): a
// changed id→status assertion is satisfied only by an EQUAL or LATER
// status in target's committed export — a non-terminal status on main can
// never subsume a carrier's close.
var statusRank = map[string]int{
	"open":        0,
	"in_progress": 1,
	"closed":      2,
}

// statusSubsumes reports whether committed (target's current recorded
// status for some id) subsumes changed (ref's asserted status for that same
// id): equal strings always subsume; otherwise both must be recognized
// members of the total order and committed's rank must be >= changed's.
// An unrecognized status string that isn't a literal match never subsumes
// — leg (b) never guesses past an unknown status.
func statusSubsumes(committed, changed string) bool {
	if committed == changed {
		return true
	}
	cr, cok := statusRank[strings.ToLower(strings.TrimSpace(committed))]
	nr, nok := statusRank[strings.ToLower(strings.TrimSpace(changed))]
	if !cok || !nok {
		return false
	}
	return cr >= nr
}

// mergeTreeResult is the exit-code-classified outcome of `git merge-tree
// --write-tree`.
type mergeTreeResult struct {
	// treeOID is the resulting tree's OID, populated only on a clean merge
	// (exit 0).
	treeOID string
	// conflict is true on exit 1 — a leg-(a) NOT-landed answer, never an
	// infra error.
	conflict bool
}

// mergeTreeWriteTreeFn is the injectable seam over the raw `git merge-tree
// --write-tree` exec + exit-code classification, so a test can simulate an
// unsupported-option failure (git < 2.38, where --write-tree does not
// exist) without needing an actually-old git binary on the test host.
var mergeTreeWriteTreeFn = execMergeTreeWriteTree

func execMergeTreeWriteTree(workdir, base, ours, theirs string) (mergeTreeResult, error) {
	cmd := execCommand("git", gitArgs(workdir, "merge-tree", "--write-tree", "--merge-base="+base, ours, theirs)...)
	out, err := cmd.Output()
	if err == nil {
		return mergeTreeResult{treeOID: strings.TrimSpace(firstLine(string(out)))}, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		// CONFLICT (panel O1's trichotomy): a definitive leg-(a) NOT-landed
		// answer, never infra — the caller must not treat this as a failure.
		return mergeTreeResult{conflict: true}, nil
	}
	// exit >= 2 (fatal git error, including "unknown option '--write-tree'"
	// on git < 2.38) or a non-ExitError (git missing from PATH): infra,
	// always propagated.
	return mergeTreeResult{}, fmt.Errorf("git merge-tree --write-tree: %w", err)
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

// mergeBaseFn resolves the merge-base of ref and target (seamed for tests).
var mergeBaseFn = gitMergeBase

func gitMergeBase(workdir, ref, target string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, "merge-base", ref, target)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("merge-base %s %s: %w", ref, target, err)
	}
	// A criss-cross history can yield multiple merge-base lines; the first
	// is sufficient for this predicate's purposes (any valid common
	// ancestor produces a comparable three-way result).
	return firstLine(strings.TrimSpace(string(out))), nil
}

// treeOIDFn resolves ref's tree OID via `git rev-parse --verify --quiet
// <ref>^{tree}` (seamed for tests).
var treeOIDFn = gitTreeOID

func gitTreeOID(workdir, ref string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, "rev-parse", "--verify", "--quiet", ref+"^{tree}")...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse %s^{tree}: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// refParentFn resolves ref's first parent via `git rev-parse --verify
// --quiet <ref>^` (seamed for tests) — NetEffectLanded's fallback base when
// ref is already an ancestor of target (see its doc comment).
var refParentFn = gitRefParent

func gitRefParent(workdir, ref string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, "rev-parse", "--verify", "--quiet", ref+"^")...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse %s^: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ContentSubsumed is the shared three-way primitive behind leg (a): it
// reports whether target's CURRENT tree already subsumes ref's content,
// computed as a three-way merge of base (the merge-base commit/tree),
// ours=target, theirs=ref — landed iff the merge result's tree OID equals
// target's own tree OID with no conflict. base, ref, and target may be any
// commit-ish (a SHA, a branch, a remote-tracking ref); workdir=="" uses the
// current directory.
//
// A CONFLICT (merge-tree exit 1) is a definitive NOT-landed answer, never
// an infra error. An infra failure (merge-tree exit >= 2 — including
// "unsupported option" on git < 2.38, where --write-tree does not exist —
// or a tree-resolution failure) is always propagated, never guessed into a
// boolean.
func ContentSubsumed(workdir, base, ref, target string) (bool, error) {
	if err := rejectOptionLike(base); err != nil {
		return false, err
	}
	if err := rejectOptionLike(ref); err != nil {
		return false, err
	}
	if err := rejectOptionLike(target); err != nil {
		return false, err
	}

	targetTree, err := treeOIDFn(workdir, target)
	if err != nil {
		return false, fmt.Errorf("resolving tree of %s: %w", target, err)
	}

	res, err := mergeTreeWriteTreeFn(workdir, base, target, ref)
	if err != nil {
		return false, err
	}
	if res.conflict {
		return false, nil
	}
	return res.treeOID == targetTree, nil
}

// parseIssueStatuses parses a .beads/issues.jsonl blob (one JSON object per
// line) into an id→status map. A deliberate, stdlib-only DUPLICATE of
// internal/lifecycle's issueStatusesInJSONL (the plan's leaf-package
// choice): gitutil must not import internal/lifecycle (internal/executor
// imports gitutil, and internal/lifecycle imports internal/phase, which
// executor's package contract forbids importing — see the plan's "Net-effect
// predicate home" rationale), so the ~20 lines of parsing are kept here
// independently rather than shared.
func parseIssueStatuses(data []byte) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if _, dup := out[rec.ID]; !dup {
			out[rec.ID] = rec.Status
		}
	}
	return out
}

// jsonlStatusesAt reads path's committed content at ref and parses it. An
// unreadable ref/path (the file did not exist yet at that commit, or the
// commit predates the tracker) is treated as an empty export rather than an
// error — the common "file added later" shape, not an infra failure.
func jsonlStatusesAt(workdir, ref, path string) map[string]string {
	data, err := FileAtRef(workdir, ref, path)
	if err != nil {
		return map[string]string{}
	}
	return parseIssueStatuses(data)
}

// NetEffectLanded is the ONE exported already-landed predicate both R4
// consumers route through (AC-17): does ref's content already exist,
// net-effect, in target's CURRENT state? See the package doc comment for
// the two-leg mechanism and its pinned consequences (ADR-0041 §2(iii)).
//
// Ancestor-collapse fallback: when ref is ALREADY an ancestor of target
// (a true, non-squash merge), `merge-base(ref, target)` trivially resolves
// to ref itself — ref's diff against ITSELF is empty, which would make leg
// (a) vacuously "landed" no matter what target did AFTERWARD, including a
// later revert of ref's own content (AC-19(iv), the doctor consumer: a
// truly-merged-then-reverted carrier must be re-detected as NOT landed
// even though ancestry still holds — R4 pins that ancestry alone is not
// sufficient there). In that case the base is ref's OWN first parent
// instead, so leg (a) evaluates ref's genuine content-introducing change
// against a state that precedes it, regardless of ref's current ancestry
// relationship to target.
//
// Leg (b) is reached ONLY when leg (a) returns a definitive NOT-landed
// (false, nil) — never when leg (a) errors. An infra failure at either leg
// is always propagated as an error, never guessed into a boolean (the
// git < 2.38 case: leg (a)'s merge-tree --write-tree is unsupported, so the
// caller must not silently fall through to a leg-(b) "success").
func NetEffectLanded(workdir, ref, target string) (bool, error) {
	if err := rejectOptionLike(ref); err != nil {
		return false, err
	}
	if err := rejectOptionLike(target); err != nil {
		return false, err
	}

	refSHA, err := RevParseRef(workdir, ref)
	if err != nil {
		return false, fmt.Errorf("resolving %s: %w", ref, err)
	}

	base, err := mergeBaseFn(workdir, ref, target)
	if err != nil {
		return false, fmt.Errorf("finding merge-base of %s and %s: %w", ref, target, err)
	}
	if base == refSHA {
		parentBase, perr := refParentFn(workdir, ref)
		if perr != nil {
			return false, fmt.Errorf("resolving parent of ancestor ref %s: %w", ref, perr)
		}
		base = parentBase
	}

	landed, err := ContentSubsumed(workdir, base, ref, target)
	if err != nil {
		// Infra failure (e.g. git < 2.38's unsupported --write-tree):
		// propagate. Never fall through to leg (b) on an undetermined leg
		// (a) — that would silently guess past the failure.
		return false, err
	}
	if landed {
		return true, nil
	}

	// Leg (b): the tracker-only-carrier fallback, entered only on leg (a)'s
	// definitive NOT-landed answer, and only when ref's WHOLE diff against
	// the merge-base is confined to the tracker export (the carrier shape,
	// derived from the diff itself — never from the caller).
	changed, err := DiffNameOnly(workdir, base, ref)
	if err != nil {
		return false, fmt.Errorf("diffing %s against merge-base: %w", ref, err)
	}
	if len(changed) != 1 || changed[0] != trackerJSONLPath {
		return false, nil
	}

	baseStatuses := jsonlStatusesAt(workdir, base, trackerJSONLPath)
	refStatuses := jsonlStatusesAt(workdir, ref, trackerJSONLPath)
	targetData, err := FileAtRef(workdir, target, trackerJSONLPath)
	if err != nil {
		// Unlike base/ref, target's committed export is the datum leg (b)
		// confirms subsumption AGAINST — an unreadable target here is a
		// genuine infra condition (target is normally "origin/main", which
		// should always resolve), so it is propagated rather than treated
		// as an empty export.
		return false, fmt.Errorf("reading %s at %s: %w", trackerJSONLPath, target, err)
	}
	targetStatuses := parseIssueStatuses(targetData)

	for id, changedStatus := range refStatuses {
		if baseStatuses[id] == changedStatus {
			continue // unchanged assertion — not part of ref's diff payload
		}
		committedStatus, ok := targetStatuses[id]
		if !ok || !statusSubsumes(committedStatus, changedStatus) {
			return false, nil
		}
	}
	return true, nil
}
