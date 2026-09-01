package gitutil

// Spec 121 Bead 1 (R4, AC-8/AC-9/AC-19): real-git fixtures for the
// net-effect already-merged predicate, per the Testing Strategy's "real
// bare-origin fixtures, never faked ancestry" discipline. These exercise
// the mechanism directly (ContentSubsumed/NetEffectLanded); the consumer
// wiring (executor probe, doctor suppression) is pinned in their own
// packages.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func neRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return string(out)
}

func neWriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestNetEffectLanded_SquashMerged is AC-8's core mechanism: a branch whose
// commits were squash-merged into main (its own SHAs never appear in
// main's history) reports landed. RED on today's main (no such predicate
// existed; ancestry alone would report false here).
func TestNetEffectLanded_SquashMerged(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--squash", "feature")
	neRunGit(t, dir, "commit", "-m", "squash merge feature")

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !landed {
		t.Error("a squash-merged branch must be reported landed")
	}
}

// TestNetEffectLanded_GenuinelyUnmerged is AC-9's negative half: a branch
// carrying novel commits never merged anywhere must NOT be reported landed
// — the normal push path stays unchanged.
func TestNetEffectLanded_GenuinelyUnmerged(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "novel content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "novel feature work")
	neRunGit(t, dir, "checkout", "main")

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if landed {
		t.Error("a genuinely unmerged branch must NOT be reported landed")
	}
}

// TestNetEffectLanded_TrueMergeCommit sanity-checks the ordinary
// merge-commit case (ancestry holds): the predicate must agree with
// ancestry when nothing has been reverted.
func TestNetEffectLanded_TrueMergeCommit(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !landed {
		t.Error("a true-merge-commit branch must be reported landed")
	}
	isAnc, ancErr := IsAncestor(dir, "feature", "main")
	if ancErr != nil || !isAnc {
		t.Fatalf("sanity: feature must be an ancestor of main here")
	}
}

// TestNetEffectLanded_TrueMergeThenRevert is AC-19(iv)'s underlying
// mechanism pin: a TRUE (non-squash) merge — ref remains an ANCESTOR of
// target — whose content is subsequently reverted on target must still be
// reported NOT landed. This is the ancestor-collapse case the doc comment
// names: merge-base(ref, target) trivially resolves to ref itself once
// ancestry holds, which would make a naive implementation report landed
// regardless of the later revert; NetEffectLanded's fallback to ref's own
// parent as the base is what makes this polarity correct.
func TestNetEffectLanded_TrueMergeThenRevert(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")
	neRunGit(t, dir, "revert", "--no-edit", "-m", "1", "HEAD")

	isAnc, ancErr := IsAncestor(dir, "feature", "main")
	if ancErr != nil || !isAnc {
		t.Fatalf("sanity: feature must remain an ancestor of main after the revert")
	}

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if landed {
		t.Error("a true-merge whose content was later reverted must NOT be reported landed, even though ancestry still holds")
	}
}

// TestNetEffectLanded_SquashThenRevert is AC-19(i): a squash-merged
// branch's content, subsequently REVERTED on main's first-parent chain,
// must NOT be reported landed — main's CURRENT tree no longer carries the
// content. RED on today's main (ancestry-only would never even reach this
// case since ancestry never held for a squash to begin with — this pins
// the net-effect mechanism's own polarity, not a revert of ancestry).
func TestNetEffectLanded_SquashThenRevert(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--squash", "feature")
	neRunGit(t, dir, "commit", "-m", "squash merge feature")
	neRunGit(t, dir, "revert", "--no-edit", "HEAD")

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if landed {
		t.Error("a squash-merge whose content was later reverted on main must NOT be reported landed")
	}
}

// TestNetEffectLanded_PartiallyLandedPlusNovel is AC-19(ii): only PART of a
// branch's content is present on main (a hand-applied partial cherry-pick)
// — the branch as a whole must NOT be reported landed, even though some of
// its content is present.
func TestNetEffectLanded_PartiallyLandedPlusNovel(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "f1.txt", "f1\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "f1")
	neWriteFile(t, dir, "f2.txt", "f2\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "f2")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "checkout", "feature", "--", "f1.txt")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "partial: only f1 landed")

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if landed {
		t.Error("a partially-landed-plus-novel branch must NOT be reported landed")
	}
}

// TestNetEffectLanded_SquashThenUnrelatedLaterChanges is AC-19(iii): a
// squash-merge followed by UNRELATED later changes on main must STILL be
// reported landed.
func TestNetEffectLanded_SquashThenUnrelatedLaterChanges(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--squash", "feature")
	neRunGit(t, dir, "commit", "-m", "squash merge feature")
	neWriteFile(t, dir, "other.txt", "unrelated\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "unrelated later change")

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !landed {
		t.Error("a squash-merge followed by unrelated later main changes must still be reported landed")
	}
}

// TestNetEffectLanded_TrackerOnlySupersedingExportSubsumed is leg (b): a
// tracker-only carrier bumps an epic to in_progress, and main is
// INDEPENDENTLY (a superseding export) already closed for that same epic —
// a genuine textual conflict at leg (a) (both sides touch the same JSONL
// line), confined to .beads/issues.jsonl, so leg (b)'s status-total-order
// subsumption applies: closed (rank 2) subsumes in_progress (rank 1).
func TestNetEffectLanded_TrackerOnlySupersedingExportSubsumed(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"open"}`+"\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "seed tracker export")

	neRunGit(t, dir, "checkout", "-b", "carrier")
	neWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"in_progress"}`+"\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "carrier: bump to in_progress")

	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"closed"}`+"\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "main: superseding export closes epic-1")

	landed, err := NetEffectLanded(dir, "carrier", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !landed {
		t.Error("a tracker-only carrier whose content main's LATER superseding export already satisfies must be reported landed")
	}
}

// TestNetEffectLanded_TrackerOnlyCarrierNotYetSubsumed is leg (b)'s
// negative half: main has NOT (yet) recorded the carrier's asserted
// status — the carrier must NOT be reported landed. No conflict occurs
// here (main is unchanged from the merge-base on that path), so this also
// pins that leg (b) fires on a clean-but-tree-mismatched leg (a) result,
// not only on a textual conflict.
func TestNetEffectLanded_TrackerOnlyCarrierNotYetSubsumed(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"open"}`+"\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "seed tracker export")

	neRunGit(t, dir, "checkout", "-b", "carrier")
	neWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"in_progress"}`+"\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "carrier: bump to in_progress")
	neRunGit(t, dir, "checkout", "main") // main stays at the seed export

	landed, err := NetEffectLanded(dir, "carrier", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if landed {
		t.Error("a carrier whose asserted status main has not yet recorded must NOT be reported landed")
	}
}

// TestNetEffectLanded_NonTrackerDiffNeverReachesLegB pins that leg (b) is
// gated on the diff being CONFINED to the tracker path: a branch that
// touches an unrelated file (never subsumed, no textual conflict on the
// tracker path at all — the mismatch is on the unrelated file) must not be
// reported landed even though it never conflicts.
func TestNetEffectLanded_NonTrackerDiffNeverReachesLegB(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "carrier")
	neWriteFile(t, dir, "other.txt", "novel\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "carrier touches an unrelated file")
	neRunGit(t, dir, "checkout", "main")

	landed, err := NetEffectLanded(dir, "carrier", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if landed {
		t.Error("a diff not confined to the tracker path must never be reported landed via leg (b)")
	}
}

// TestNetEffectLanded_PureRevertNoAdditionNotVacuouslyLanded is bead-4 fix
// round 1's BLOCKING-4 regression (G1-4/O1-1/O2-1/S1-1): a branch whose
// cumulative diff against merge-base(branch, target) nets to EMPTY (it
// reintroduces content target once carried, then reverts its OWN
// reintroduction, with no other change) must never be reported landed —
// applying an empty diff changes nothing, so a "landed" answer here would
// prove nothing was ever introduced, not that anything landed. Two
// targets, per the panel's own reproduction: an UNTOUCHED target (the
// simplest vacuous shape) and a DIVERGED one carrying unrelated content
// the branch never saw — proving the (pre-fix) vacuous match was
// target-independent, not a same-content coincidence.
//
// Scope-defeating mutation (not merely a sensitivity one): reverting
// NetEffectLanded's refTree/baseTree guard above (restoring the version
// that calls ContentSubsumed unconditionally) turns both subtests RED,
// with `landed=true` logged in both — the exact misclassification the
// panel reproduced against real production code before this fix.
func TestNetEffectLanded_PureRevertNoAdditionNotVacuouslyLanded(t *testing.T) {
	buildStaleBead := func(t *testing.T) string {
		t.Helper()
		dir := initGitRepo(t)
		neRunGit(t, dir, "checkout", "-b", "spec-target")
		neWriteFile(t, dir, "landed.txt", "landed after the branch's snapshot\n")
		neRunGit(t, dir, "add", ".")
		neRunGit(t, dir, "commit", "-m", "spec-target adds landed.txt")
		neRunGit(t, dir, "checkout", "-b", "stale-bead", "spec-target")
		neRunGit(t, dir, "rm", "landed.txt")
		neRunGit(t, dir, "commit", "-m", "revert to the pre-landed.txt snapshot (no other change)")
		neRunGit(t, dir, "checkout", "main")
		return dir
	}

	t.Run("against_untouched_target", func(t *testing.T) {
		dir := buildStaleBead(t)

		landed, err := NetEffectLanded(dir, "stale-bead", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if landed {
			t.Error("a branch whose net diff against merge-base(branch, main) is empty must NOT be reported landed against main — nothing was ever introduced for main to have landed")
		}
	})

	t.Run("against_diverged_target_with_unrelated_content", func(t *testing.T) {
		dir := buildStaleBead(t)
		neWriteFile(t, dir, "unrelated.txt", "unrelated content the branch never saw\n")
		neRunGit(t, dir, "add", ".")
		neRunGit(t, dir, "commit", "-m", "main diverges with unrelated content")

		landed, err := NetEffectLanded(dir, "stale-bead", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if landed {
			t.Error("the vacuous match must not fire even when target has diverged with unrelated content the branch never touched — proving the prior bug was target-independent, not a same-content coincidence")
		}
	})
}

// TestContentSubsumedOutcome_Trichotomy pins the spec 121 final-review r2
// F2-2r discriminator on real-git fixtures: the three-way outcome of a
// merge M's own change (base=M^1, ours=tip, theirs=M) is LANDED while the
// content survives, CLEAN-DIVERGENCE after a genuine `git revert M` (the
// tip returns to the base state on M's paths — the backed-out shape), and
// CONFLICT when the tip itself rewrote M's region (landed-then-evolved).
// It also pins that ContentSubsumed — NetEffectLanded's leg-(a) boolean
// projection — collapses BOTH non-landed shapes to false, unchanged, so
// the Bead-1 doctor/probe consumers (AC-19(iv)) are behaviorally
// untouched by the trichotomy's introduction.
func TestContentSubsumedOutcome_Trichotomy(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "seed.txt", "seed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "seed")
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")
	mergeSHA := neRunGit(t, dir, "rev-parse", "HEAD")
	mergeSHA = mergeSHA[:len(mergeSHA)-1] // trim trailing newline
	base := mergeSHA + "^1"

	// (a) content present at the tip → LANDED.
	if got, err := ContentSubsumedOutcome(dir, base, mergeSHA, "main"); err != nil || got != SubsumptionLanded {
		t.Fatalf("landed shape: got %v, %v; want SubsumptionLanded", got, err)
	}

	// (b) tip rewrote M's own file → CONFLICT (landed-then-evolved), and
	// the boolean projection still reads false (not subsumed).
	neWriteFile(t, dir, "feature.txt", "rewritten by later work\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "later rewrite")
	if got, err := ContentSubsumedOutcome(dir, base, mergeSHA, "main"); err != nil || got != SubsumptionConflict {
		t.Fatalf("evolved shape: got %v, %v; want SubsumptionConflict", got, err)
	}
	if landed, err := ContentSubsumed(dir, base, mergeSHA, "main"); err != nil || landed {
		t.Fatalf("boolean projection must stay false on the conflict shape, got %v, %v", landed, err)
	}

	// (c) a genuine revert of M (fresh repo state: back out the rewrite
	// first, then revert the merge) → CLEAN divergence, boolean false.
	neRunGit(t, dir, "revert", "--no-edit", "HEAD")              // undo the rewrite
	neRunGit(t, dir, "revert", "--no-edit", "-m", "1", mergeSHA) // back out M
	if got, err := ContentSubsumedOutcome(dir, base, mergeSHA, "main"); err != nil || got != SubsumptionCleanDivergence {
		t.Fatalf("reverted shape: got %v, %v; want SubsumptionCleanDivergence", got, err)
	}
	if landed, err := ContentSubsumed(dir, base, mergeSHA, "main"); err != nil || landed {
		t.Fatalf("boolean projection must stay false on the reverted shape, got %v, %v", landed, err)
	}
}

// TestContentSubsumed_MergeTreeInfraErrorPropagates is the OLD-GIT subtest
// (panel O1): a stubbed merge-tree primitive returning the unsupported-
// --write-tree-shaped error (simulating git < 2.38) must propagate as an
// ERROR from BOTH ContentSubsumed and NetEffectLanded — NEVER a guessed
// boolean, and leg (b) must never be silently reached on this path.
func TestContentSubsumed_MergeTreeInfraErrorPropagates(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")

	orig := mergeTreeWriteTreeFn
	t.Cleanup(func() { mergeTreeWriteTreeFn = orig })
	simulated := errors.New(`fatal: unknown option '--write-tree'`)
	mergeTreeWriteTreeFn = func(workdir, base, ours, theirs string) (mergeTreeResult, error) {
		return mergeTreeResult{}, simulated
	}

	if _, err := ContentSubsumed(dir, "main", "feature", "main"); err == nil {
		t.Fatal("ContentSubsumed must propagate the old-git infra error, never a boolean")
	}

	landed, err := NetEffectLanded(dir, "feature", "main")
	if err == nil {
		t.Fatalf("NetEffectLanded must propagate the old-git infra error, got landed=%v, nil error", landed)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated infra failure, got: %v", err)
	}
}

// TestNetEffectLanded_RejectsOptionLikeOperands is the SEC-5 argv-hygiene
// pin every gitutil ref-bearing entry point carries.
func TestNetEffectLanded_RejectsOptionLikeOperands(t *testing.T) {
	dir := initGitRepo(t)
	if _, err := NetEffectLanded(dir, "-x", "main"); err == nil {
		t.Error("expected a rejection for an option-like ref operand")
	}
	if _, err := NetEffectLanded(dir, "main", "-x"); err == nil {
		t.Error("expected a rejection for an option-like target operand")
	}
}

// revertShapeMergeFixture builds a repo where branch "feature" (one file,
// feature.txt) is --no-ff merged into main, and returns (dir, mergeSHA).
// The base shape every RevertShape subtest below varies from.
func revertShapeMergeFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")
	mergeSHA := neRunGit(t, dir, "rev-parse", "HEAD")
	return dir, mergeSHA[:len(mergeSHA)-1] // trim trailing newline
}

// TestRevertShape_TrueRevert is the positive half of the spec 125 R3
// sub-classification: after a genuine `git revert -m 1 M` with no later
// rework, the tip carries none of M's introduced content, so the reverse
// un-apply is a clean no-op — revert-shape true.
func TestRevertShape_TrueRevert(t *testing.T) {
	dir, mergeSHA := revertShapeMergeFixture(t)
	neRunGit(t, dir, "revert", "--no-edit", "-m", "1", mergeSHA)

	rev, err := RevertShape(dir, mergeSHA, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rev {
		t.Error("a genuine `git revert M` tip must classify as revert-shape")
	}
}

// TestRevertShape_ContentPresentNotRevertShape: with M's content fully
// present at the tip (nothing reverted), un-applying M would REMOVE that
// content — the un-apply changes the tip, so NOT revert-shape.
func TestRevertShape_ContentPresentNotRevertShape(t *testing.T) {
	dir, mergeSHA := revertShapeMergeFixture(t)

	rev, err := RevertShape(dir, mergeSHA, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev {
		t.Error("a tip still carrying M's content must NOT classify as revert-shape")
	}
}

// TestRevertShape_PartialSupersessionNotRevertShape is the AC-5 mechanism
// pin (the 8nhe.2 evolved case): M lands content across TWO surfaces;
// later honest work removes-and-replaces ONE surface while M's other
// content remains at the tip. The forward check reads this as
// SubsumptionCleanDivergence (asserted as the shape-precondition), but the
// reverse un-apply would still remove the SURVIVING surface — the tip
// carries SOME of M's content, so NOT revert-shape (evolved → identify).
func TestRevertShape_PartialSupersessionNotRevertShape(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "surface-a.txt", "alpha payload\n")
	neWriteFile(t, dir, "surface-b.txt", "beta payload\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature: two surfaces")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")
	mergeSHA := neRunGit(t, dir, "rev-parse", "HEAD")
	mergeSHA = mergeSHA[:len(mergeSHA)-1]

	// Later honest work: surface-a removed AND replaced at a different
	// path; surface-b (M's other content) remains at the tip.
	neRunGit(t, dir, "rm", "surface-a.txt")
	neWriteFile(t, dir, "surface-a2.txt", "superseding replacement for alpha\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "supersede surface-a with surface-a2")

	// Shape-precondition: the FORWARD check reads this partial
	// supersession as CleanDivergence — the arm spec 125 sub-classifies.
	outcome, err := ContentSubsumedOutcome(dir, mergeSHA+"^1", mergeSHA, "main")
	if err != nil || outcome != SubsumptionCleanDivergence {
		t.Fatalf("fixture shape-precondition: want SubsumptionCleanDivergence from the forward check, got %v, %v", outcome, err)
	}

	rev, err := RevertShape(dir, mergeSHA, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev {
		t.Error("a tip retaining PART of M's content (partial supersession) must NOT classify as revert-shape")
	}
}

// TestRevertShape_TrueRevertWithCoincidentalBlobElsewhere is the
// G-BLOCK-1 rename-safety pin (RED against a rename-detection-ON impl):
// M introduces a distinctive blob at path X; M is then genuinely reverted
// (X removed); unrelated later work happens to recreate the IDENTICAL
// blob at a DIFFERENT path Y. With merge-ort's default rename detection,
// the reverse un-apply reads "X renamed to Y" and produces a rename/delete
// CONFLICT — so a rename-detecting RevertShape returns false (evolved →
// IDENTIFY), an UNSAFE false-positive attestation of a truly-reverted
// merge. With rename detection OFF the un-apply is a clean path-based
// no-op → revert-shape TRUE (the merge REFUSES, correct). The blob is
// deliberately multi-line so rename detection would fire on the ON path.
func TestRevertShape_TrueRevertWithCoincidentalBlobElsewhere(t *testing.T) {
	dir := initGitRepo(t)
	blob := "line one\nline two\nline three\nline four\nline five\nline six\nline seven\nline eight\n"
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "X.txt", blob)
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature introduces X")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")
	mergeSHA := neRunGit(t, dir, "rev-parse", "HEAD")
	mergeSHA = mergeSHA[:len(mergeSHA)-1]

	// Genuine revert of M (X removed).
	neRunGit(t, dir, "revert", "--no-edit", "-m", "1", mergeSHA)
	// Unrelated later work recreates the identical blob at a DIFFERENT path.
	neWriteFile(t, dir, "Y.txt", blob)
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "unrelated: identical blob at Y")

	rev, err := RevertShape(dir, mergeSHA, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rev {
		t.Error("a true revert must classify as revert-shape even when a coincidentally-identical blob exists at an unrelated path (G-BLOCK-1: rename detection must be OFF)")
	}
}

// TestRevertShape_UnApplyConflictNotRevertShape: the tip rewrote M's own
// file, so un-applying M (which would delete it) conflicts with the tip's
// modification — the tip built ON M's region (evolved), NOT revert-shape.
func TestRevertShape_UnApplyConflictNotRevertShape(t *testing.T) {
	dir, mergeSHA := revertShapeMergeFixture(t)
	neWriteFile(t, dir, "feature.txt", "rewritten by later work\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "later rewrite of M's file")

	rev, err := RevertShape(dir, mergeSHA, "main")
	if err != nil {
		t.Fatalf("a conflicting un-apply is a definitive classification, not an error: %v", err)
	}
	if rev {
		t.Error("a conflicting un-apply (tip built on M's region) must NOT classify as revert-shape")
	}
}

// TestRevertShape_NonMergeErrors: a <2-parent commit has no M^1/M^2 to
// anchor the un-apply on — RevertShape must FAIL with an error, never
// guess a classification (spec 125 R3's parent guard at the primitive).
func TestRevertShape_NonMergeErrors(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "plain.txt", "plain\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "a plain non-merge commit")
	plainSHA := neRunGit(t, dir, "rev-parse", "HEAD")
	plainSHA = plainSHA[:len(plainSHA)-1]

	if _, err := RevertShape(dir, plainSHA, "main"); err == nil {
		t.Fatal("RevertShape on a non-merge commit must error, never classify")
	}
}

// TestRevertShape_InfraErrorPropagates is the plan-gate O2-1 pin at the
// primitive: a merge-tree infra failure (simulating git < 2.38's
// unsupported --write-tree, exit >= 2) must PROPAGATE as an error from
// RevertShape — never be mapped to a definite bool in either direction.
func TestRevertShape_InfraErrorPropagates(t *testing.T) {
	dir, mergeSHA := revertShapeMergeFixture(t)

	orig := mergeTreeWriteTreeNoRenamesFn
	t.Cleanup(func() { mergeTreeWriteTreeNoRenamesFn = orig })
	simulated := errors.New(`fatal: unknown option '--write-tree'`)
	mergeTreeWriteTreeNoRenamesFn = func(workdir, base, ours, theirs string) (mergeTreeResult, error) {
		return mergeTreeResult{}, simulated
	}

	rev, err := RevertShape(dir, mergeSHA, "main")
	if err == nil {
		t.Fatalf("RevertShape must propagate the infra error, got rev=%v, nil error", rev)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated infra failure, got: %v", err)
	}
}

// TestRevertShape_RejectsOptionLikeOperands: the SEC-5 argv-hygiene pin,
// same as every other gitutil ref-bearing entry point.
func TestRevertShape_RejectsOptionLikeOperands(t *testing.T) {
	dir := initGitRepo(t)
	if _, err := RevertShape(dir, "-x", "main"); err == nil {
		t.Error("expected a rejection for an option-like mergeSHA operand")
	}
	if _, err := RevertShape(dir, "main", "-x"); err == nil {
		t.Error("expected a rejection for an option-like target operand")
	}
}

// --- PreviewDeletedPaths (spec 127 bead 1) ---------------------------------

// TestPreviewDeletedPaths_GenuineDeletion is the baseline positive: a
// branch that actually removes a file target still carries reports that
// file as a D-path.
func TestPreviewDeletedPaths_GenuineDeletion(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "keep.txt", "keep\n")
	neWriteFile(t, dir, "gone.txt", "gone\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "add keep+gone")
	neRunGit(t, dir, "checkout", "-b", "cleanup")
	neRunGit(t, dir, "rm", "gone.txt")
	neRunGit(t, dir, "commit", "-m", "remove gone.txt")
	neRunGit(t, dir, "checkout", "main")

	deleted, err := PreviewDeletedPaths(dir, "main", "cleanup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "gone.txt" {
		t.Errorf("PreviewDeletedPaths = %v, want [gone.txt]", deleted)
	}
}

// TestPreviewDeletedPaths_LargeRenameNeverReadsAsDeletion is AC-8(ii)'s
// primitive-level pin: a directory move/rename must surface as an R-path
// via --find-renames, never as a D-path — the exact miss the pinned
// rename detection exists to prevent (without it, plumbing diffs default
// to no rename detection and a move misreads as delete+add).
func TestPreviewDeletedPaths_LargeRenameNeverReadsAsDeletion(t *testing.T) {
	dir := initGitRepo(t)
	body := "content line one\nmore lines here to make similarity high\nline three\nline four\n"
	for _, n := range []string{"1", "2", "3", "4", "5"} {
		neWriteFile(t, dir, "dir/f"+n+".txt", body+n+"\n")
	}
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "add dir/")
	neRunGit(t, dir, "checkout", "-b", "mover")
	neRunGit(t, dir, "mv", "dir", "newdir")
	neRunGit(t, dir, "commit", "-m", "move dir -> newdir")
	neRunGit(t, dir, "checkout", "main")

	deleted, err := PreviewDeletedPaths(dir, "main", "mover")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("PreviewDeletedPaths = %v, want none — a directory move must read as renames, never deletions", deleted)
	}
}

// TestPreviewDeletedPaths_HonestModifyDeleteConflictYieldsEmptyDSet is the
// pinned disposition for a GENUINE, isolated modify/delete conflict: `git
// merge-tree --write-tree` retains TARGET's (ours') own version at the
// conflicted path itself (verified on git 2.51.2: exit 1's stdout says
// "Version main of f.txt left in tree"), so diffing target against the
// conflicted preview's tree shows NO change there at all — an empty
// D-set, correctly, but NOT because a conflict is special-cased away
// (spec 127 bead-1 fix round, O1-1/O2-1/O3-2 deleted that short-circuit —
// see the doc comment above, which used to claim "no D-set is derivable
// from a preview that never resolved to a tree"). It is empty here
// because target's own content at the ONLY path in play is genuinely
// unchanged by the preview — contrast the next test, where a conflict
// co-occurs with a genuine, unrelated deletion.
func TestPreviewDeletedPaths_HonestModifyDeleteConflictYieldsEmptyDSet(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "f.txt", "v1\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "add f.txt")
	neRunGit(t, dir, "checkout", "-b", "del")
	neRunGit(t, dir, "rm", "f.txt")
	neRunGit(t, dir, "commit", "-m", "delete f.txt")
	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, "f.txt", "v2\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "modify f.txt")

	deleted, err := PreviewDeletedPaths(dir, "main", "del")
	if err != nil {
		t.Fatalf("a conflicted preview must not be classified as an infra error, got: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("PreviewDeletedPaths on an isolated modify/delete conflict = %v, want none", deleted)
	}
}

// TestPreviewDeletedPaths_ConflictOnOnePathDoesNotMaskDeletionOnAnother is
// the RED-on-the-prior-bug fixture (spec 127 bead-1 fix round,
// O1-1/O2-1/O3-2): a candidate merge that both CONFLICTS on one path and
// DELETES a different, unrelated target-present path must still report
// the unrelated deletion. Before this fix, PreviewDeletedPaths returned
// (nil, nil) unconditionally on ANY conflict — masking the deletion of
// landed.txt behind the unrelated conflict on f.txt, the exact
// "certifying the destruction" failure this predicate exists to prevent.
func TestPreviewDeletedPaths_ConflictOnOnePathDoesNotMaskDeletionOnAnother(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "f.txt", "v1\n")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "add f.txt+landed.txt")
	neRunGit(t, dir, "checkout", "-b", "del")
	neRunGit(t, dir, "rm", "f.txt", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "delete f.txt and landed.txt")
	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, "f.txt", "v2\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "modify f.txt")

	deleted, err := PreviewDeletedPaths(dir, "main", "del")
	if err != nil {
		t.Fatalf("a conflicted preview must not be classified as an infra error, got: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "landed.txt" {
		t.Errorf("PreviewDeletedPaths on a conflicted-but-not-fully-blind preview = %v, want [landed.txt] (the conflict on f.txt must not mask the unrelated deletion)", deleted)
	}
}

// TestPreviewDeletedPaths_MergeBaseInfraErrorPropagates and
// TestPreviewDeletedPaths_MergeTreeInfraErrorPropagates prove PreviewDeletedPaths
// never classifies a git infra failure into an empty (safe-looking) D-set
// (the spec 125 O2-1 discipline, applied to this new primitive): both the
// merge-base resolution and the merge-tree preview propagate.
func TestPreviewDeletedPaths_MergeBaseInfraErrorPropagates(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "f.txt", "x\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")

	orig := mergeBaseFn
	t.Cleanup(func() { mergeBaseFn = orig })
	simulated := errors.New("simulated merge-base failure")
	mergeBaseFn = func(workdir, ref, target string) (string, error) { return "", simulated }

	deleted, err := PreviewDeletedPaths(dir, "main", "feature")
	if err == nil {
		t.Fatalf("expected the merge-base failure to propagate, got deleted=%v, nil error", deleted)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
}

func TestPreviewDeletedPaths_MergeTreeInfraErrorPropagates(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "f.txt", "x\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")

	orig := mergeTreeWriteTreeFn
	t.Cleanup(func() { mergeTreeWriteTreeFn = orig })
	simulated := errors.New(`fatal: unknown option '--write-tree'`)
	mergeTreeWriteTreeFn = func(workdir, base, ours, theirs string) (mergeTreeResult, error) {
		return mergeTreeResult{}, simulated
	}

	deleted, err := PreviewDeletedPaths(dir, "main", "feature")
	if err == nil {
		t.Fatalf("expected the merge-tree failure to propagate, got deleted=%v, nil error", deleted)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
}

// TestPreviewDeletedPaths_DiffInfraErrorPropagates: the trailing
// name-status diff's own failure must propagate too, not just the
// merge-tree leg's.
func TestPreviewDeletedPaths_DiffInfraErrorPropagates(t *testing.T) {
	dir := initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feature")
	neWriteFile(t, dir, "f.txt", "x\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")

	orig := diffNameStatusBucketsFn
	t.Cleanup(func() { diffNameStatusBucketsFn = orig })
	simulated := errors.New("simulated diff failure")
	diffNameStatusBucketsFn = func(workdir, from, to string) ([]string, []string, []string, error) {
		return nil, nil, nil, simulated
	}

	deleted, err := PreviewDeletedPaths(dir, "main", "feature")
	if err == nil {
		t.Fatalf("expected the diff failure to propagate, got deleted=%v, nil error", deleted)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
}

// TestPreviewDeletedPaths_RejectsOptionLikeOperands: the SEC-5 argv-hygiene
// pin, same as every other gitutil ref-bearing entry point.
func TestPreviewDeletedPaths_RejectsOptionLikeOperands(t *testing.T) {
	dir := initGitRepo(t)
	if _, err := PreviewDeletedPaths(dir, "-x", "main"); err == nil {
		t.Error("expected a rejection for an option-like target operand")
	}
	if _, err := PreviewDeletedPaths(dir, "main", "-x"); err == nil {
		t.Error("expected a rejection for an option-like branch operand")
	}
}

// TestPreviewDeletedPaths_MutatesNothing asserts the non-mutating
// discipline: refs and worktree status are byte-identical before and
// after a PreviewDeletedPaths call (the same house check as
// ContentSubsumedOutcome's own preview).
func TestPreviewDeletedPaths_MutatesNothing(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "keep.txt", "keep\n")
	neWriteFile(t, dir, "gone.txt", "gone\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "add keep+gone")
	neRunGit(t, dir, "checkout", "-b", "cleanup")
	neRunGit(t, dir, "rm", "gone.txt")
	neRunGit(t, dir, "commit", "-m", "remove gone.txt")
	neRunGit(t, dir, "checkout", "main")

	refsBefore := neRunGit(t, dir, "for-each-ref")
	statusBefore := neRunGit(t, dir, "status", "--porcelain")

	if _, err := PreviewDeletedPaths(dir, "main", "cleanup"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	refsAfter := neRunGit(t, dir, "for-each-ref")
	statusAfter := neRunGit(t, dir, "status", "--porcelain")
	if refsBefore != refsAfter {
		t.Errorf("refs changed:\nbefore: %q\nafter:  %q", refsBefore, refsAfter)
	}
	if statusBefore != statusAfter {
		t.Errorf("worktree/index status changed:\nbefore: %q\nafter:  %q", statusBefore, statusAfter)
	}
}

// --- spec 127 bead-1 fix round 2, G1-N1's ruling: diffNameStatusBuckets'
// per-status classification -------------------------------------------------

// TestDiffNameStatusBuckets_TypeChangeExcludedFromAllThreeBuckets is the
// real-git behavioral half of G1-N1's fix: a T-status (typechange) path
// — a target-present regular file turned into a symlink — must be
// classified with a WRITTEN reason (excluded, same as R/C) rather than
// silently falling through an unnamed default, and it must not appear in
// added, deleted, or modified.
func TestDiffNameStatusBuckets_TypeChangeExcludedFromAllThreeBuckets(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "kind.txt", "a regular file\n")
	neWriteFile(t, dir, "untouched.txt", "untouched\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neRunGit(t, dir, "checkout", "-b", "typechange")
	if err := os.Remove(filepath.Join(dir, "kind.txt")); err != nil {
		t.Fatalf("remove kind.txt: %v", err)
	}
	if err := os.Symlink("untouched.txt", filepath.Join(dir, "kind.txt")); err != nil {
		t.Fatalf("symlink kind.txt: %v", err)
	}
	neRunGit(t, dir, "add", "kind.txt")
	neRunGit(t, dir, "commit", "-m", "kind.txt: regular file -> symlink")
	neRunGit(t, dir, "checkout", "main")

	statusOut := neRunGit(t, dir, "diff", "--name-status", "--find-renames", "main", "typechange", "--", "kind.txt")
	if !strings.HasPrefix(strings.TrimSpace(statusOut), "T") {
		t.Fatalf("fixture invariant broken: expected a T-status record for kind.txt, got %q", statusOut)
	}

	added, deleted, modified, err := diffNameStatusBuckets(dir, "main", "typechange")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, bucket := range [][]string{added, deleted, modified} {
		for _, p := range bucket {
			if p == "kind.txt" {
				t.Fatalf("kind.txt (a T-status typechange) must not appear in added=%v deleted=%v modified=%v", added, deleted, modified)
			}
		}
	}
}

// TestDiffNameStatusBuckets_UnhandledStatusFailsClosed is G1-N1's ruling
// applied to statuses this predicate has never seen and never will
// safely guess about: U (unmerged) and X (unknown), plus a hypothetical
// future letter ("Z") no version of git emits today. Real git never
// emits these for a commit-ish-to-commit-ish diff, so the seam is
// exercised directly rather than via a real-git fixture — but the
// SHAPE (a status this switch has not classified with a written reason)
// is exactly what the round-1 fix's bare `default` arm let through
// silently for T; this pins the INVERTED default (fail closed with an
// error) for anything this switch does not explicitly recognize.
func TestDiffNameStatusBuckets_UnhandledStatusFailsClosed(t *testing.T) {
	for _, status := range []string{"U", "X", "Z"} {
		t.Run(status, func(t *testing.T) {
			orig := execCommand
			t.Cleanup(func() { execCommand = orig })
			execCommand = func(name string, args ...string) *exec.Cmd {
				// The format STRING itself (single-quoted, so the shell
				// passes it to printf verbatim) contains the literal
				// text `\000` — printf's OWN escape interpretation turns
				// that into a real NUL byte in its output; a NUL byte
				// cannot survive as a literal byte inside a Go string
				// handed to exec.Command's argv (execve truncates C
				// strings at NUL), so it must be produced this way
				// rather than embedded directly.
				script := "printf '" + status + "\\000somefile.txt\\000'"
				return exec.Command("/bin/sh", "-c", script)
			}

			added, deleted, modified, err := diffNameStatusBuckets(unhandledStatusTestWorkdir, "from", "to")
			if err == nil {
				t.Fatalf("expected a non-nil error for unhandled status %q, got added=%v deleted=%v modified=%v", status, added, deleted, modified)
			}
			if !strings.Contains(err.Error(), status) {
				t.Errorf("expected the error to name the unhandled status %q, got: %v", status, err)
			}
		})
	}
}

// unhandledStatusTestWorkdir is a placeholder workdir for
// TestDiffNameStatusBuckets_UnhandledStatusFailsClosed's seam-injected
// git calls, which never actually touch a real repository (execCommand
// itself is stubbed) — any string works, but rejectOptionLike still runs
// first, so it must not look option-like.
const unhandledStatusTestWorkdir = "/tmp/mindspec-unhandled-status-test-workdir"
