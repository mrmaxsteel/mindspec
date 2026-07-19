// Spec 119 Bead 3 (R7): finalize-orphan detection. Bug wu7t's protected-main
// recovery flow (executor.MindspecExecutor.finalizeOrphanedSpecBranch,
// internal/workspace.FinalizeBranchPrefix = "chore/finalize-") creates a
// fresh chore/finalize-<specID> carrier from origin/main when a spec branch
// is already a dead PR carrier. If that recovery is interrupted or its PR is
// never opened/merged, it leaves TWO kinds of residue behind:
//
//   - an outstanding, unmerged chore/finalize-<specID> branch, and
//   - main's committed .beads/issues.jsonl staying stale relative to bd's
//     live epic status, which leaves the shipped bd post-merge hook poised
//     to silently REVERT Dolt's close on the next merge/FF.
//
// These predicates are exported here — NOT executor-private, NOT
// doctor-private — so both `mindspec doctor` and the generated `mindspec
// instruct` guidance (internal/instruct) can import and render the SAME
// finding text (P8, AC-15). internal/lifecycle sits outside the
// enforcement-package boundary pin (ADR-0030), so it uses internal/gitutil
// directly, mirroring the shipped orphans.go precedent.
package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Injectable seams (mirroring orphans.go's pattern) so finalize-orphan
// detection is unit-testable without a live repo.
var (
	localBranchRefsFn           = gitutil.LocalBranchRefs
	finalizeOrphanCommitCountFn = gitutil.CommitCount
	finalizeOrphanDiffStatFn    = gitutil.DiffStat
	fileAtRefFn                 = gitutil.FileAtRef
	revParseRefFn               = gitutil.RevParseRef

	// finalizeOrphanNetEffectFn is the ONE exported already-merged
	// predicate (gitutil.NetEffectLanded) the doctor merged-carrier
	// suppression routes through (spec 121 R4, ADR-0041 §2(iii)). Unlike
	// the FinalizeEpic probe, SHA ancestry alone is NOT sufficient here:
	// the suppression decision is the net-effect/current-content test EVEN
	// WHEN ancestry holds, so a carrier truly (non-squash) merged and then
	// REVERTED on origin/main is flagged again — suppressing it would hide
	// a genuinely-stranded export behind a historical merge. AC-17
	// anti-drift pins this seam's default to be the identical symbol the
	// executor probe falls back to.
	finalizeOrphanNetEffectFn = gitutil.NetEffectLanded
)

// FinalizeOrphan describes a leftover artifact from an interrupted
// protected-main finalize recovery (bug wu7t). One shared shape (P8,
// AC-15) so `doctor` and the generated `instruct` guidance render
// IDENTICAL message + recovery text from THIS predicate — never a private
// reimplementation.
type FinalizeOrphan struct {
	// Kind is "finalize_branch" (an outstanding chore/finalize-<specID>
	// branch), "stale_tracker" (origin/main's committed
	// .beads/issues.jsonl disagrees with bd's live epic status), or
	// "pull_advisory" (spec 121 R2(c): origin/main's committed export
	// already agrees, but the LOCAL main checkout has not yet been
	// pulled — never a re-finalize recovery, which would self-loop).
	Kind string
	// SpecID is the spec this finalize orphan belongs to.
	SpecID string
	// Branch is the outstanding chore/finalize-<specID> carrier
	// (Kind == "finalize_branch" only; empty otherwise).
	Branch string
	// CommitCount / DiffStat summarize Branch's stranded work, computed
	// against origin/main (Requirement 7: NEVER possibly-stale local
	// main) — populated for Kind == "finalize_branch" only.
	CommitCount int
	DiffStat    string
	// Message is the rendered human-readable finding text — the SAME
	// string doctor and instruct must both surface (AC-15).
	Message string
}

// RecoveryCommand names the forward re-invocation that clears the orphan
// (ADR-0035 recovery-line convention).
//
// R4: o.Branch is the spine-validated `chore/finalize-<specID>` branch
// operand (workspace.FinalizeBranchPrefix) and stays RAW, matching the
// `spec/<id>`/`bead/<id>` convention; o.SpecID is an ID-typed position —
// idrender.Spec.
func (o FinalizeOrphan) RecoveryCommand() string {
	switch o.Kind {
	case "finalize_branch":
		return fmt.Sprintf("open a PR for %s and merge it (or delete the branch if it is superseded)", o.Branch)
	case "pull_advisory":
		// R2(c): origin/main's export already agrees — the recovery is a
		// plain pull of local main, NEVER `mindspec impl approve`, which
		// would self-loop against an export that already landed
		// (ADR-0041 §2(i)'s deadlock-free rule).
		return "git pull (update local main to the already-landed origin/main export)"
	default:
		return fmt.Sprintf("mindspec impl approve %s", idrender.Spec(o.SpecID))
	}
}

// FullMessage combines Message and RecoveryCommand into the single rendered
// line `mindspec doctor` (internal/doctor) and the generated `mindspec
// instruct` guidance (internal/instruct) both surface verbatim (Spec 119
// Bead 2, AC-15/P8) — one template, defined once here, so the two
// consumers cannot drift into differently-worded renderings of the same
// finding.
func (o FinalizeOrphan) FullMessage() string {
	return fmt.Sprintf("%s Run `%s`.", o.Message, o.RecoveryCommand())
}

// FindOutstandingFinalizeBranches scans workdir's LOCAL branches for a
// surviving chore/finalize-<specID> carrier (workspace.FinalizeBranchPrefix).
// finalizeOrphanedSpecBranch leaves this branch behind LOCALLY on success —
// a retry deletes and recreates it fresh from origin/main, per its own doc
// comment — so a branch surviving past the run that created it means the PR
// it carries was never opened, merged, and cleaned up. Stats are computed
// against origin/main (Requirement 7), never local main — this predicate
// never even reads local main.
//
// Merged-carrier suppression (spec 119 final-review G1, spec 121 R4):
// because the recovery flow deliberately leaves the carrier branch behind
// LOCALLY even after its PR merges, "the branch exists" alone is NOT proof
// of stranded work. The suppression decision is the NET-EFFECT already-
// merged predicate (gitutil.NetEffectLanded via finalizeOrphanNetEffectFn),
// not SHA ancestry alone (ADR-0041 §2(iii)): a carrier whose content
// origin/main's current committed export already carries — including a
// squash merge, or a LATER superseding export — is suppressed, while one
// truly (even non-squash) merged and then REVERTED on origin/main is
// flagged again, because ancestry alone would wrongly suppress it forever.
// When a carrier's net-effect landed state CANNOT be determined, that
// branch is never asserted "unmerged" from absence of proof: it is skipped
// and the first such error is returned alongside the provable findings (the
// mixed-list contract ScanOrphanedClosedBeads pioneered — later provable
// findings survive an earlier branch's error).
func FindOutstandingFinalizeBranches(workdir string) ([]FinalizeOrphan, error) {
	branches, err := localBranchRefsFn(workdir)
	if err != nil {
		return nil, fmt.Errorf("listing local branches: %w", err)
	}
	var out []FinalizeOrphan
	var firstErr error
	for _, b := range branches {
		if !strings.HasPrefix(b, workspace.FinalizeBranchPrefix) {
			continue
		}
		// Confirmation before assertion: a carrier whose content is
		// already landed (net effect) on origin/main is benign residue,
		// not an orphan. A landed-check failure is recorded and the
		// branch skipped — never reported as "unmerged" without proof.
		landed, neErr := finalizeOrphanNetEffectFn(workdir, b, "origin/main")
		if neErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("checking net-effect landed state of %s against origin/main: %w", b, neErr)
			}
			continue
		}
		if landed {
			continue
		}
		// Reverse-derivation gate (ADR-0042 §1 reverse, spec 120 AC-23,
		// round-4 G2): specID is parsed back OUT of an agent-creatable
		// local git branch name (git refnames admit shell/Markdown
		// metacharacters). A malformed result is skipped with one
		// escaped warning — never minted as a FinalizeOrphan, never
		// composed, rendered, or embedded as an ID.
		specID := strings.TrimPrefix(b, workspace.FinalizeBranchPrefix)
		if err := idvalidate.SpecID(specID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed finalize branch %s: %v\n", termsafe.Escape(b), err)
			continue
		}
		o := FinalizeOrphan{Kind: "finalize_branch", SpecID: specID, Branch: b}
		if count, cErr := finalizeOrphanCommitCountFn(workdir, "origin/main", b); cErr == nil {
			o.CommitCount = count
		}
		if stat, sErr := finalizeOrphanDiffStatFn(workdir, "origin/main", b); sErr == nil {
			o.DiffStat = stat
		}
		// R4: b is the spine-validated finalize-branch operand (stays RAW);
		// specID is an ID-typed position (idrender.Spec).
		o.Message = fmt.Sprintf(
			"finalize branch %s is unmerged (%d commit(s) ahead of origin/main) — spec %s's epic-close export never reached main",
			b, o.CommitCount, idrender.Spec(specID),
		)
		out = append(out, o)
	}
	return out, firstErr
}

// mainExportRef resolves which ref's committed .beads/issues.jsonl export
// the stale-tracker classifier consults (spec 121 R2(c), ADR-0041 §2(i)):
// origin/main when it exists — refreshed by the CALLER's own fetch
// discipline (e.g. the FinalizeEpic probe's `git fetch origin main`), never
// possibly-stale local main — falling back to local main ONLY in the
// no-remote direct workflow (no origin/main ref at all: RevParseRef's
// ErrRefNotFound). Any OTHER rev-parse failure (a transient/structural git
// error) propagates rather than silently falling back.
func mainExportRef(workdir string) (string, error) {
	if _, err := revParseRefFn(workdir, "origin/main"); err != nil {
		if errors.Is(err, gitutil.ErrRefNotFound) {
			return "main", nil
		}
		return "", err
	}
	return "origin/main", nil
}

// StaleTrackerOnMain reports whether epicID's committed status inside the
// refreshed origin/main:.beads/issues.jsonl export (falling back to local
// main only when no origin/main ref exists — mainExportRef) is a
// non-terminal ("open"/"in_progress") status while liveClosed is true — bd's
// LIVE state already has the epic closed. That divergence is the tell-tale
// left by bug wu7t: main never received the finalize export that would have
// synced it, which leaves the shipped bd post-merge hook poised to silently
// revert Dolt's close on the next merge/FF.
//
// R2(c) (spec 121): consulting origin/main (rather than possibly-stale local
// main) means this finding correctly clears the moment the finalize export
// actually reaches origin/main, even before the local checkout has pulled.
// When origin/main already agrees but the DISTINCT local main ref still
// lags, that residual is harmless local staleness — surfaced only as a
// "pull_advisory" finding whose recovery is a plain pull, never the
// self-looping `mindspec impl approve` recovery a stale_tracker finding
// carries (ADR-0041 §2(i)'s deadlock-free rule).
//
// Returns (nil, nil) — not itself an error — when liveClosed is false, when
// epicID is not present in the consulted export, or when the two statuses
// already agree with no local lag. A genuine git-read failure (bad ref, no
// such path) is propagated so a caller can distinguish "no finding" from
// "could not check".
func StaleTrackerOnMain(workdir, specID, epicID string, liveClosed bool) (*FinalizeOrphan, error) {
	if !liveClosed {
		return nil, nil
	}
	ref, err := mainExportRef(workdir)
	if err != nil {
		return nil, fmt.Errorf("resolving main export ref: %w", err)
	}
	data, err := fileAtRefFn(workdir, ref, ".beads/issues.jsonl")
	if err != nil {
		return nil, fmt.Errorf("reading %s:.beads/issues.jsonl: %w", ref, err)
	}
	if o := staleTrackerFinding(specID, epicID, issueStatusesInJSONL(data)); o != nil {
		return o, nil
	}
	if ref == "origin/main" {
		// Deliberate swallow: this secondary local-main read is
		// advisory-only (a "pull the latest main" nudge, never a
		// stranded-carrier finding) — a read failure here (no local main
		// ref, a transient git error) just means the advisory does not
		// fire this run, never that the primary origin/main-agreement
		// result above is discarded or an error is manufactured for a
		// best-effort convenience surface.
		if localData, lErr := fileAtRefFn(workdir, "main", ".beads/issues.jsonl"); lErr == nil {
			localStatus := issueStatusesInJSONL(localData)[epicID]
			if adv := pullAdvisoryFinding(specID, epicID, localStatus); adv != nil {
				return adv, nil
			}
		}
	}
	return nil, nil
}

// staleTrackerFinding is the pure classification core shared by
// StaleTrackerOnMain and ScanIntegrityFindings (spec 119 final-review F1):
// given the consulted ref's ALREADY-PARSED committed id→status map, it
// reports the stale-tracker divergence for one live-closed epic, or nil.
// The single home of the finding's message template — neither consumer
// re-derives it.
func staleTrackerFinding(specID, epicID string, committed map[string]string) *FinalizeOrphan {
	committedStatus, found := committed[epicID]
	if !found || strings.EqualFold(committedStatus, "closed") {
		return nil
	}
	// R4: epicID/specID are ID-typed positions (idrender.Bead/idrender.Spec);
	// committedStatus already renders through %q (strconv.Quote-equivalent),
	// which is inherently forced-safe.
	return &FinalizeOrphan{
		Kind:   "stale_tracker",
		SpecID: specID,
		Message: fmt.Sprintf(
			"epic %s (spec %s) is closed in bd but main's committed .beads/issues.jsonl still shows status %q — the finalize export never reached main",
			idrender.Bead(epicID), idrender.Spec(specID), committedStatus,
		),
	}
}

// pullAdvisoryFinding is the pure classification core for R2(c)'s
// local-main-lag advisory (spec 121): given a live-closed epic whose
// committed status is ALREADY terminal on origin/main (staleTrackerFinding
// found nothing there), but the local main ref's own checked-out export
// still shows localStatus as a non-terminal (or absent-from-local, the zero
// value) status for it, the recovery is a plain pull — NEVER `mindspec impl
// approve`, which would self-loop against an export that already landed
// (ADR-0041 §2(i)). Returns nil when localStatus is empty (absent from
// local main's export) or already closed (agreement — no lag).
func pullAdvisoryFinding(specID, epicID, localStatus string) *FinalizeOrphan {
	if localStatus == "" || strings.EqualFold(localStatus, "closed") {
		return nil
	}
	return &FinalizeOrphan{
		Kind:   "pull_advisory",
		SpecID: specID,
		Message: fmt.Sprintf(
			"epic %s (spec %s) is closed on origin/main but the local main checkout's .beads/issues.jsonl still shows status %q",
			idrender.Bead(epicID), idrender.Spec(specID), localStatus,
		),
	}
}

// issueStatusesInJSONL parses a .beads/issues.jsonl blob (one JSON object
// per line) ONCE into an id→status map, so an aggregate scan over many
// epics reads and decodes main's committed export a single time (F1).
func issueStatusesInJSONL(data []byte) map[string]string {
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
