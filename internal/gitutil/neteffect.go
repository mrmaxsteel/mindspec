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
	// treeOID is the resulting tree's OID. `git merge-tree --write-tree`
	// prints the tree OID as its FIRST stdout line on BOTH exit 0 and exit
	// 1 (spec 127 bead-1 fix round, O1-1/O2-1/O3-2): on a conflict the
	// printed tree still contains every unconflicted path's merged content
	// (conflicted paths carry conflict-marker blobs, plus the trailing
	// numbered-stage/informational lines merge-tree also writes to
	// stdout), so a D-set is derivable from it even when conflict is true.
	// Verified on git 2.51.2: `git merge-tree --write-tree` exit 1 still
	// emits the tree OID first, unconditionally.
	treeOID string
	// conflict is true on exit 1 — a leg-(a) NOT-landed answer, never an
	// infra error. It no longer means "no tree is available": see treeOID.
	conflict bool
}

// mergeTreeWriteTreeFn is the injectable seam over the raw `git merge-tree
// --write-tree` exec + exit-code classification, so a test can simulate an
// unsupported-option failure (git < 2.38, where --write-tree does not
// exist) without needing an actually-old git binary on the test host.
var mergeTreeWriteTreeFn = execMergeTreeWriteTree

func execMergeTreeWriteTree(workdir, base, ours, theirs string) (mergeTreeResult, error) {
	return runMergeTreeWriteTree(workdir, base, ours, theirs, false)
}

// mergeTreeWriteTreeNoRenamesFn is the injectable seam over `git merge-tree
// --write-tree` with rename/copy detection DISABLED (spec 125 G-BLOCK-1) —
// used ONLY by RevertShape's reverse un-apply. merge-ort does rename
// detection by default, which lets a coincidentally-identical blob at an
// UNRELATED path count as M's "moved" content: a true revert of M (its
// content removed at its original path) plus unrelated later work that
// happens to recreate the same blob elsewhere merges as a rename/delete
// CONFLICT instead of a clean no-op, so a rename-detecting RevertShape
// would misread the revert as evolved and IDENTIFY it — an unsafe
// false-positive landed attestation. With `-c merge.renames=false` the
// un-apply is PATH-based: M's content counts only at its original paths.
// (Verified effective on the repo's git 2.51: the rename/delete conflict
// collapses to a clean tree-equal no-op.)
var mergeTreeWriteTreeNoRenamesFn = execMergeTreeWriteTreeNoRenames

func execMergeTreeWriteTreeNoRenames(workdir, base, ours, theirs string) (mergeTreeResult, error) {
	return runMergeTreeWriteTree(workdir, base, ours, theirs, true)
}

// runMergeTreeWriteTree is the shared exec + exit-code classification for
// both the rename-detecting (leg-(a) / ContentSubsumedOutcome) and the
// rename-disabled (RevertShape) merge-tree calls. When noRenames is set a
// top-level `-c merge.renames=false` config override precedes the
// subcommand, suppressing ort's rename/copy detection.
func runMergeTreeWriteTree(workdir, base, ours, theirs string, noRenames bool) (mergeTreeResult, error) {
	var argv []string
	if noRenames {
		argv = gitArgs(workdir, "-c", "merge.renames=false", "merge-tree", "--write-tree", "--merge-base="+base, ours, theirs)
	} else {
		argv = gitArgs(workdir, "merge-tree", "--write-tree", "--merge-base="+base, ours, theirs)
	}
	cmd := execCommand("git", argv...)
	out, err := cmd.Output()
	if err == nil {
		return mergeTreeResult{treeOID: strings.TrimSpace(firstLine(string(out)))}, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		// CONFLICT (panel O1's trichotomy): a definitive leg-(a) NOT-landed
		// answer, never infra — the caller must not treat this as a
		// failure. The first stdout line is still the merged tree's OID
		// (see mergeTreeResult.treeOID); capture it rather than discarding
		// stdout, so a conflict-tolerant caller (PreviewDeletedPaths) can
		// still derive a D-set.
		return mergeTreeResult{conflict: true, treeOID: strings.TrimSpace(firstLine(string(out)))}, nil
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

// Subsumption is ContentSubsumedOutcome's three-way answer (spec 121
// final-review r2 F2-2r): the two NOT-subsumed shapes of the three-way
// merge carry OPPOSITE meanings for a caller asking "was this landed
// change later backed out?", and collapsing them to one boolean is what
// produced the R5(d) honest-rewrite permanent-refusal deadlock.
type Subsumption int

const (
	// SubsumptionCleanDivergence: the three-way merges CLEANLY but the
	// result differs from target's tree — re-applying ref's change would
	// alter target, meaning target sits at (or has cleanly returned to)
	// the base state on ref's own paths: the change was BACKED OUT (the
	// exact shape a `git revert` leaves behind). Deliberately the ZERO
	// value, so an error-path return (or a caller that wrongly ignores
	// the error) reads as the fail-closed "not landed" answer, never a
	// phantom identification.
	SubsumptionCleanDivergence Subsumption = iota
	// SubsumptionLanded: the merge result's tree equals target's CURRENT
	// tree with no conflict — ref's content is present, net-effect, at
	// target's tip.
	SubsumptionLanded
	// SubsumptionConflict: the three-way CONFLICTS — target's tip has
	// ITSELF advanced past base on ref's own region, incompatibly with
	// re-applying ref. For a ref already in target's history this means
	// later work evolved/superseded ref's content rather than backing it
	// out (landed-then-evolved); for an out-of-history ref it simply
	// means the content is not subsumed.
	SubsumptionConflict
)

// ContentSubsumedOutcome is the shared three-way primitive behind leg (a):
// it classifies how target's CURRENT tree relates to ref's content,
// computed as a three-way merge of base (the merge-base commit/tree),
// ours=target, theirs=ref (see Subsumption for the trichotomy). base, ref,
// and target may be any commit-ish (a SHA, a branch, a remote-tracking
// ref); workdir=="" uses the current directory.
//
// A CONFLICT (merge-tree exit 1) is a definitive classification, never an
// infra error. An infra failure (merge-tree exit >= 2 — including
// "unsupported option" on git < 2.38, where --write-tree does not exist —
// or a tree-resolution failure) is always propagated, never guessed into
// an outcome.
func ContentSubsumedOutcome(workdir, base, ref, target string) (Subsumption, error) {
	if err := rejectOptionLike(base); err != nil {
		return SubsumptionCleanDivergence, err
	}
	if err := rejectOptionLike(ref); err != nil {
		return SubsumptionCleanDivergence, err
	}
	if err := rejectOptionLike(target); err != nil {
		return SubsumptionCleanDivergence, err
	}

	targetTree, err := treeOIDFn(workdir, target)
	if err != nil {
		return SubsumptionCleanDivergence, fmt.Errorf("resolving tree of %s: %w", target, err)
	}

	res, err := mergeTreeWriteTreeFn(workdir, base, target, ref)
	if err != nil {
		return SubsumptionCleanDivergence, err
	}
	if res.conflict {
		return SubsumptionConflict, nil
	}
	if res.treeOID == targetTree {
		return SubsumptionLanded, nil
	}
	return SubsumptionCleanDivergence, nil
}

// ContentSubsumed is the boolean projection of ContentSubsumedOutcome —
// "is ref's content present, net-effect, at target's CURRENT tip?" — the
// answer NetEffectLanded's leg (a) consumes (both NOT-subsumed shapes are
// equally "not landed" there: leg (b)'s tracker-carrier fallback and the
// AC-19(iv) revert re-detection both depend on that collapse, so this
// projection's behavior is deliberately IDENTICAL to the pre-F2-2r
// boolean).
func ContentSubsumed(workdir, base, ref, target string) (bool, error) {
	outcome, err := ContentSubsumedOutcome(workdir, base, ref, target)
	if err != nil {
		return false, err
	}
	return outcome == SubsumptionLanded, nil
}

// RevertShape is spec 125 R3's sub-classification of a
// SubsumptionCleanDivergence outcome: the REVERSE "un-apply" no-op test.
// Where ContentSubsumedOutcome(base=M^1, ref=M, target=tip) asks "what
// happens if M's change is RE-APPLIED to the tip?", RevertShape asks the
// reverse — "does UN-applying M's change from the tip do anything?" — as
// the three-way merge-tree(base = M, ours = target's tip, theirs = M^1)
// with rename/copy detection DISABLED.
//
// It reports revert-shape (true) iff the un-apply is CLEAN and its result
// tree EQUALS target's current tree: the tip already carries NONE of M's
// introduced content AT ITS ORIGINAL PATHS, so backing M out changes
// nothing — exactly what `git revert M` leaves behind, and ALSO the
// clean-full-removal residual (M's content later removed cleanly and fully
// by honest work), which is content-INDISTINGUISHABLE from a revert. That
// residual refusing is a DELIBERATE false-negative floor: any datum that
// accepted clean full removal would accept every real revert too. Any
// other outcome is NOT revert-shape (false): the un-apply CHANGES the tip
// (M's content is present at the tip, wholly or PARTIALLY — the
// partial-supersession evolved shape) or CONFLICTS (the tip built on M's
// region — evolved).
//
// G-BLOCK-1 (rename-safety, load-bearing): the un-apply MUST be
// PATH-based, so rename/copy detection is disabled
// (mergeTreeWriteTreeNoRenamesFn). merge-ort's default rename detection
// would let a coincidentally-identical blob at an UNRELATED later path
// count as M's "moved" content — a true revert of M plus unrelated work
// that recreates the same blob elsewhere merges as a rename/delete
// CONFLICT instead of a clean no-op, so a rename-detecting RevertShape
// would misread the revert as evolved and IDENTIFY it (an UNSAFE
// false-positive landed attestation). This is why RevertShape does NOT
// route through ContentSubsumedOutcome (whose leg-(a) merge-tree keeps
// rename detection ON for the spec-121 forward semantics): the reverse
// un-apply needs the rename-off posture specifically.
//
// mergeSHA must be a real merge: M^1 and M^2 are resolved explicitly and a
// <2-parent commit fails with an error — never a first-parent guess.
// (Callers additionally exclude octopus/non-bead candidates before ever
// consulting the discrimination; see internal/lifecycle.FindLandedMerge.)
// The internally-resolved M^1 is by construction the same commit as the
// merge's Parents[0] in a FirstParentMerges scan, so the forward re-apply
// check and this reverse un-apply check share the same base anchoring.
//
// Infra-error discipline (spec 125 plan-gate O2-1): on ANY git/tree-
// resolution failure — including `git merge-tree --write-tree` being
// unsupported (git < 2.38) or exiting >= 2 — the error is PROPAGATED as
// (false, non-nil error). An undetermined result is never mapped to a
// definite classification in either direction; the caller must fail
// closed on a non-nil error, never read it as "identify" or "refuse".
func RevertShape(workdir, mergeSHA, target string) (bool, error) {
	if err := rejectOptionLike(mergeSHA); err != nil {
		return false, err
	}
	if err := rejectOptionLike(target); err != nil {
		return false, err
	}
	firstParent, err := RevParseRef(workdir, mergeSHA+"^1")
	if err != nil {
		return false, fmt.Errorf("resolving first parent of merge %s: %w", mergeSHA, err)
	}
	if _, err := RevParseRef(workdir, mergeSHA+"^2"); err != nil {
		return false, fmt.Errorf("revert-shape check requires a >=2-parent merge, but %s has no second parent: %w", mergeSHA, err)
	}

	targetTree, err := treeOIDFn(workdir, target)
	if err != nil {
		return false, fmt.Errorf("resolving tree of %s: %w", target, err)
	}
	// Reverse un-apply, rename detection OFF (G-BLOCK-1):
	// base = M, ours = target's tip, theirs = M^1.
	res, err := mergeTreeWriteTreeNoRenamesFn(workdir, mergeSHA, target, firstParent)
	if err != nil {
		return false, err
	}
	if res.conflict {
		// The tip built on M's own region — un-applying M conflicts with
		// the tip's own later edits there → evolved, NOT revert-shape.
		return false, nil
	}
	// Revert-shape iff the un-apply is a clean no-op (result tree equals
	// the tip's current tree — the tip carries none of M's content at its
	// original paths).
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
//
// Second vacuous-match guard (bead-4 fix round 1, BLOCKING-4 — G1-4/O1-1/
// O2-1/S1-1): the ancestor-collapse fallback above corrects ONE vacuous
// shape (ref itself already an ancestor of target). There is a SECOND,
// distinct one it does not touch: leg (a)'s three-way merge (base, ours =
// target, theirs = ref) necessarily reproduces target's OWN tree,
// unconditionally, whenever ref's tree at its tip equals its tree at
// base — i.e. ref's cumulative diff against that base is EMPTY (the
// add-then-revert shape: a branch that reintroduces content target once
// carried and then reverts its own reintroduction, ending up with no net
// change of its own at all). Applying an empty diff to any tree returns
// that tree unchanged, so leg (a) would report SubsumptionLanded — and
// therefore "landed" — for EVERY possible target, including one that
// never carried ref's content at all (verified end-to-end against both an
// untouched and a diverged target; see
// TestNetEffectLanded_PureRevertNoAdditionNotVacuouslyLanded). Nothing was
// ever introduced by ref relative to this base, so nothing can be
// reported landed: leg (a) is skipped entirely in that case, falling
// through to leg (b) exactly as a genuine NOT-landed leg (a) result would
// (leg (b)'s own diff-confined-to-tracker check also finds nothing here,
// since the diff it inspects is the same empty one).
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

	refTree, err := treeOIDFn(workdir, refSHA)
	if err != nil {
		return false, fmt.Errorf("resolving tree of %s: %w", ref, err)
	}
	baseTree, err := treeOIDFn(workdir, base)
	if err != nil {
		return false, fmt.Errorf("resolving tree of merge-base %s: %w", base, err)
	}

	if refTree != baseTree {
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

// diffNameStatusBucketsFn is the injectable seam over diffNameStatusBuckets
// (error-forcing for tests, default pointer-pinned).
var diffNameStatusBucketsFn = diffNameStatusBuckets

// diffNameStatusBuckets runs `git diff --name-status --find-renames -z from
// to` and buckets the result into added (A-status: paths present at `to`
// but absent at `from`), deleted (D-status: paths present at `from` but
// absent at `to`), and modified (M-status: present at both, but content
// and/or mode changed — git's plumbing reports a pure mode-only change as
// "M" too, with an unchanged blob OID, so this bucket also carries
// mode-only novel work). Rename/copy status (R/C) counts as NEITHER added,
// deleted, nor modified — the content moved, it was not introduced,
// removed, or edited in place — which is exactly what --find-renames is
// pinned for (spec 127 bead 1): without it, plumbing diffs default to no
// rename detection and a large move would misread as a delete+add pair.
// `from`/`to` may be any commit-ish (branch, SHA, or a bare tree OID, e.g.
// the result of a merge-tree preview).
//
// `-z` (spec 127 bead-1 fix round, S1-1/O1-2/G1-1): WITHOUT it, git
// C-quotes any path byte that needs escaping — which by default (
// core.quotepath defaults to true) includes every non-ASCII UTF-8 byte,
// and ALWAYS includes control characters such as a literal tab or
// newline — into a double-quoted, backslash/octal-escaped literal token,
// and a tab inside a path also breaks the '\t'-field-splitting itself,
// not merely the quoting. `-z` sidesteps quoting entirely: git emits
// each record NUL-separated with paths raw and unescaped — status, then
// one path (A/D/M/…) or two paths (R/C: from-path, to-path), each
// terminated by a NUL, with no trailing record after the final NUL.
// Parsing therefore walks fixed-width NUL-delimited records rather than
// splitting a line on '\t'/'\n', so a literal tab or newline BYTE inside
// a path can never be misread as a field or record boundary.
//
// Every status letter git can emit here is classified below WITH A
// WRITTEN REASON (spec 127 bead-1 fix round 2, G1-N1's ruling): the prior
// shape of this switch had a bare `default` arm that silently treated any
// status this file's authors had not enumerated — T (typechange) among
// them — as "none of the three buckets", identically to the deliberate
// R/C exclusion, with no record that a decision had even been made. That
// is the fail-open shape this predicate exists to eliminate everywhere
// else, reappearing here: a caller (novelPaths, in workdestruction.go)
// that trusted "not added, not deleted, not modified" to mean "untouched"
// was wrong for T, and would be wrong again for the next status letter a
// future git version introduces. Known statuses are each named with a
// reason (T joins R/C's exclusion, not A/D/M's buckets — see the case
// below); anything this switch has NOT been taught — including U
// (unmerged, which should never appear when diffing two commit-ish/
// tree-ish arguments, since both `from` and `to` here are never the
// index or working tree) and X (git's own admission that it could not
// classify the change) — fails CLOSED with an error instead of falling
// through. novelPaths' caller (EvaluateWorkDestruction) turns that error
// into DestructionEvidenceError, never a guessed outcome.
func diffNameStatusBuckets(workdir, from, to string) (added, deleted, modified []string, err error) {
	if err := rejectOptionLike(from); err != nil {
		return nil, nil, nil, err
	}
	if err := rejectOptionLike(to); err != nil {
		return nil, nil, nil, err
	}
	cmd := execCommand("git", gitArgs(workdir, "diff", "--name-status", "--find-renames", "-z", from, to)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("diff --name-status --find-renames -z %s %s: %w", from, to, err)
	}
	raw := string(out)
	if raw == "" {
		return nil, nil, nil, nil
	}
	fields := strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	for i := 0; i < len(fields); {
		status := fields[i]
		if status == "" {
			// A trailing empty field from the final NUL (or a malformed
			// record) — nothing left to consume either way.
			break
		}
		switch status[0] {
		case 'R', 'C':
			// Two-path form: status, from-path, to-path. Renamed/copied
			// content counts as none of the three buckets (see doc
			// comment above).
			if i+2 >= len(fields) {
				return nil, nil, nil, fmt.Errorf("diff --name-status -z %s %s: truncated rename/copy record %q", from, to, status)
			}
			i += 3
		case 'A':
			if i+1 >= len(fields) {
				return nil, nil, nil, fmt.Errorf("diff --name-status -z %s %s: truncated add record", from, to)
			}
			added = append(added, fields[i+1])
			i += 2
		case 'D':
			if i+1 >= len(fields) {
				return nil, nil, nil, fmt.Errorf("diff --name-status -z %s %s: truncated delete record", from, to)
			}
			deleted = append(deleted, fields[i+1])
			i += 2
		case 'M':
			if i+1 >= len(fields) {
				return nil, nil, nil, fmt.Errorf("diff --name-status -z %s %s: truncated modify record", from, to)
			}
			modified = append(modified, fields[i+1])
			i += 2
		case 'T':
			// Type change (spec 127 bead-1 fix round 2, G1-N1): the path
			// is present, under the SAME name, at both `from` and `to` —
			// only its KIND changed (e.g. regular file <-> symlink, or
			// file <-> gitlink), not merely its content. Structurally a
			// single-path record, exactly like A/D/M. Deliberately
			// excluded from all three buckets, for the SAME reason R/C
			// is: this file's only caller that consumes the added
			// bucket for anything beyond evidence (novelPaths, in
			// workdestruction.go, via snapshotRevertMatch's strip)
			// needs "paths the branch introduced at a location target
			// never had" — T is by definition a path target ALREADY
			// had, so it cannot belong in `added` regardless of which
			// bucket a future caller might fold it into instead
			// (corrected, spec 127 bead-1 fix round 3->4, O3r-1: the
			// prior wording here said folding T into `modified` would
			// reopen the A+M-masking over-match ruling 2 rolled back —
			// no longer the live reason, since `modified` is discarded
			// by snapshotRevertMatch's only caller as of that same
			// ruling; that risk would need restating, not merely
			// re-citing, if a future caller ever consumed `modified`
			// again). So T stays a stated, fixtured miss (see
			// StatedLimit_TypeChangeOfNovelWorkIsMissed in
			// workdestruction_test.go) — named on purpose, unlike the
			// silent `default` this case replaces.
			if i+1 >= len(fields) {
				return nil, nil, nil, fmt.Errorf("diff --name-status -z %s %s: truncated type-change record", from, to)
			}
			i += 2
		case 'U', 'X':
			// Unmerged (U) and unknown (X) (spec 127 bead-1 fix round 2,
			// G1-N1's ruling): neither has a bucket this predicate can
			// place it in with a stated reason — U marks an index/
			// working-tree conflict this predicate never diffs against,
			// and X is git's own admission that it could not classify
			// the change — so both fail CLOSED rather than silently
			// landing in "none of the three buckets" the way the T-status
			// hole did before this fix.
			return nil, nil, nil, fmt.Errorf("diff --name-status -z %s %s: unhandled diff status %q (git reported a change this predicate does not classify as added, deleted, or modified — failing closed rather than silently excluding it)", from, to, status)
		default:
			// Any status letter this switch has not been taught — a
			// future git version, or a code this fix's authors did not
			// anticipate — fails closed for the identical reason (spec
			// 127 bead-1 fix round 2, G1-N1's ruling: "any diff status
			// not explicitly classified with a written reason must fail
			// CLOSED"). This inverts the prior default, which silently
			// treated every unrecognized status — including T, the
			// concrete hole G1-N1 found — as "none of the three
			// buckets" with no record that a decision had even been
			// made.
			return nil, nil, nil, fmt.Errorf("diff --name-status -z %s %s: unrecognized diff status %q", from, to, status)
		}
	}
	return added, deleted, modified, nil
}

// PreviewDeletedPaths is the read-only D-set primitive behind the spec 127
// stale-deletion leg (internal/gitutil.EvaluateWorkDestruction): the paths
// a non-mutating merge preview of branch into target would DELETE
// relative to target's CURRENT tip tree.
//
// Mechanics: `git merge-tree --write-tree` (base = merge-base(target,
// branch); ours = target; theirs = branch — the same three-way convention
// ContentSubsumedOutcome uses, rename detection ON) previews the merge;
// the resulting tree is then diffed against target's tip with
// diffNameStatusBuckets (--find-renames pinned), keeping only the deleted
// bucket. A large rename/directory-move (AC-8(ii)) therefore reads as
// R-paths, never D-paths.
//
// A CONFLICTED preview (merge-tree exit 1) does NOT short-circuit to an
// empty D-set (spec 127 bead-1 fix round, O1-1/O2-1/O3-2 — reversing this
// file's prior doc comment, which claimed "no D-set is derivable from a
// preview that never resolved to a tree": FALSE on git >= 2.38, verified
// end-to-end — `git merge-tree --write-tree` still prints the merged
// tree's OID as its first line on exit 1, with every UNCONFLICTED path
// merged normally; only the conflicted path(s) carry conflict-marker
// content instead of a clean merge result). The D-set is derived from
// that tree exactly as on a clean preview: a conflicting edit on ONE path
// must never mask a genuine deletion on another, unrelated path — the
// ordering-mask failure this predicate exists to prevent, at reduced
// width, if it did. Any git infra failure — resolving the merge-base, the
// merge-tree preview itself (exit >= 2, including "unknown option" on
// git < 2.38), or the trailing diff — is always propagated as a non-nil
// error, never classified into an empty D-set (the spec 125 O2-1
// discipline: absence of evidence is never safety).
//
// Non-mutating throughout: `git merge-tree --write-tree` writes only
// unreferenced loose tree objects (the same discipline as
// ContentSubsumedOutcome's own preview) — refs, the index, and the
// worktree are untouched.
func PreviewDeletedPaths(workdir, target, branch string) ([]string, error) {
	if err := rejectOptionLike(target); err != nil {
		return nil, err
	}
	if err := rejectOptionLike(branch); err != nil {
		return nil, err
	}

	base, err := mergeBaseFn(workdir, target, branch)
	if err != nil {
		return nil, fmt.Errorf("finding merge-base of %s and %s: %w", target, branch, err)
	}

	res, err := mergeTreeWriteTreeFn(workdir, base, target, branch)
	if err != nil {
		return nil, err
	}

	_, deleted, _, err := diffNameStatusBucketsFn(workdir, target, res.treeOID)
	if err != nil {
		return nil, err
	}
	return deleted, nil
}
