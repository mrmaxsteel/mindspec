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
