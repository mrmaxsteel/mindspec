package complete

// Spec 128 Bead 6 (OQ-1(a); ADR-0041 §1 amendment): the full failure-
// injection + red-on-revert fixture matrix for the prospective-snapshot
// protocol (plan §Bead 6 step 7), all twenty legs (i)-(xx). Every leg
// runs on the DEFAULT FILES ref backend explicitly (no `-c
// extensions.refStorage=reftable` override anywhere in this file) and
// drives REAL git via setupRealGitFaultFixture/wireRealGitFaultSeams —
// never a mock standing in for the mechanism under test.
//
// Inventory (test name -> leg):
//   TestBead6Leg_i_UntrackedFileGatesBeforeCommit          (i)
//   TestBead6Leg_ii_PreCommitHookNeverRuns                 (ii)
//   TestBead6Leg_iii_ArtifactExcludedFromGateMeasuredTree  (iii)
//   TestBead6Leg_iv_GateToCommitIntervalMutationIsOrdinaryDirt (iv)
//   TestBead6Leg_v_UserCommitNeverTouchesArtifact           (v)
//   TestBead6Leg_vi_UnchangedWorktreeIsCleanAfterMaterialize (vi)
//   TestBead6Leg_vii_ArtifactByteIdenticalUntilSyncCommit    (vii)
//   TestBead6Leg_viii_a_ResumeKillBeforeReset_IntervalEditRefuses (viii-a)
//   TestBead6Leg_viii_b_ResumeAfterClearingDirtConverges     (viii-b)
//   TestBead6Leg_viii_b_StaleForeignMarkerFallsThroughToNormalRun (viii-b, stale-marker variant)
//   TestBead6Leg_ix_KillAfterSStageAdvance_ConvergesAndIntervalEditRefuses (ix)
//   TestBead6Leg_x_KillDuringPreservedMergeConflict          (x)
//   TestBead6Leg_xi_ForeignCommitAfterMaterializationBlocksOnStaleSHA (xi)
//   TestBead6Leg_xii_KillBetweenMarkerAndRefAdvance_MStage   (xii)
//   TestBead6Leg_xiii_KillBetweenMarkerAndRefAdvance_SStage  (xiii)
//   TestBead6Leg_xiv_ConcurrentPublisherCASRefusesWithoutOverwriting (xiv)
//   TestBead6Leg_xv_EmptyCElision                            (xv)
//   TestBead6Leg_xvi_NoOpDElision                            (xvi)
//   TestBead6Leg_xvii_OrdinaryArtifactChangingD               (xvii)
//   TestBead6Leg_xviii_KillAfterRWithCElided_NoMarker        (xviii)
//   TestBead6Leg_xix_KillAfterNoOpS_ResumeIdempotent          (xix)
//   TestBead6Leg_xx_BothStagesElided_NoMarkerRoute            (xx)

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// wireBeadWorktree registers a single bead worktree entry (the shape
// every leg below needs) and returns its path.
func wireBeadWorktree(t *testing.T, root, specID, beadID, beadBranch string) string {
	t.Helper()
	wtPath := filepath.Join(root, ".worktrees", "worktree-"+beadID)
	gitRun(t, root, "worktree", "add", wtPath, beadBranch)
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Name: "worktree-" + beadID, Path: wtPath, Branch: beadBranch}}, nil
	}
	return wtPath
}

// gitCommonDir resolves the shared `.git` common directory for a
// worktree (where hooks live, shared across every linked worktree).
func gitCommonDir(t *testing.T, wtPath string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", wtPath, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		t.Fatalf("rev-parse --git-common-dir: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(wtPath, dir)
	}
	return dir
}

// treeOIDAtRef resolves ref's own tree OID (`git rev-parse <ref>^{tree}`).
func treeOIDAtRef(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref+"^{tree}").Output()
	if err != nil {
		t.Fatalf("rev-parse %s^{tree}: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// commitParent resolves ref's first parent SHA.
func commitParent(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref+"^").Output()
	if err != nil {
		t.Fatalf("rev-parse %s^: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// --- (i): an UNTRACKED file gates before any commit ------------------------
//
// The gate measures the bead worktree's PROSPECTIVE content (T), which
// `add -A` stages regardless of tracked/untracked status — an untracked
// file OUTSIDE any OWNERSHIP-claimed domain trips adr-divergence-unowned
// exactly like a tracked one, and the refusal fires BEFORE any commit
// exists (tip unchanged). (The fixture's own OWNERSHIP.yaml, committed
// on the bead branch at fork, already classifies as a "doc" for the
// generic doc-sync lane across the full base..T range — this leg
// isolates the untracked-file claim on the ADR-divergence lane, which
// that pre-existing doc commit does not satisfy for an UNOWNED domain.)
func TestBead6Leg_i_UntrackedFileGatesBeforeCommit(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg1", "mindspec-128leg1.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)
	preTip := gateRevParse(t, root, beadBranch)

	// UNTRACKED (never `git add`ed) file in a domain NO OWNERSHIP.yaml
	// claims — the fixture only declares "widget".
	if err := os.MkdirAll(filepath.Join(wtPath, "internal", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "core", "untracked.go"), []byte("package core\n\nfunc Untracked() {}\n"), 0o644); err != nil {
		t.Fatalf("writing untracked file: %v", err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	_, err := Run(root, beadID, specID, "", base, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the gate to refuse on the untracked, unowned source file")
	}
	if !strings.Contains(err.Error(), "adr-divergence") {
		t.Errorf("expected an adr-divergence refusal, got: %v", err)
	}
	if got := gateRevParse(t, root, beadBranch); got != preTip {
		t.Errorf("tip must stay unchanged across the refusal: got %s want %s", got, preTip)
	}
}

// --- (ii): a pre-commit hook never runs -------------------------------------
//
// Materialization builds C/D via `git commit-tree` (plumbing) — hooks
// never run. A pre-commit hook that would corrupt the commit (writing a
// marker file and modifying tracked content) has NO EFFECT: the marker
// never appears, and the committed tree is exactly T.
func TestBead6Leg_ii_PreCommitHookNeverRuns(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg2", "mindspec-128leg2.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	hooksDir := filepath.Join(gitCommonDir(t, wtPath), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks dir: %v", err)
	}
	markerPath := filepath.Join(wtPath, "HOOK-RAN")
	hookScript := "#!/bin/sh\ntouch " + markerPath + "\necho corrupted >> internal/widget/widget.go\nexit 0\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(hookScript), 0o755); err != nil {
		t.Fatalf("writing pre-commit hook: %v", err)
	}

	// A real, doc-sync-clean change: modify a file whose domain's own doc
	// gate is satisfied (widget has no doc/ dir, so no source/doc split
	// applies) with an artifact-only follow-up so the run converges.
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Two() {}\n"), 0o644); err != nil {
		t.Fatalf("writing change: %v", err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	res, err := Run(root, beadID, specID, "work", base, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected completion, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if _, statErr := os.Stat(markerPath); statErr == nil {
		t.Error("the pre-commit hook RAN — plumbing (commit-tree) must never invoke hooks")
	}
	if fileContainsAtRef(t, root, beadBranch, "internal/widget/widget.go", "corrupted") {
		t.Error("the hook's corruption reached the committed tree — commit-tree plumbing must be hook-immune")
	}
}

// --- (iii): the artifact is unchanged-at-tip in T; the sync commit alone changes it ---
func TestBead6Leg_iii_ArtifactExcludedFromGateMeasuredTree(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg3", "mindspec-128leg3.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// Commit a tracked artifact at "X" first, so there is a tip blob to
	// preserve.
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x","status":"X"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "-A")
	gitRun(t, wtPath, "commit", "-q", "-m", "tracked artifact at X")

	// Now dirty the artifact (simulating live Dolt drift) AND make an
	// unrelated real source change.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x","status":"Y"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Three() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	res, err := Run(root, beadID, specID, "work", base, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected completion, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}

	newTip := gateRevParse(t, root, beadBranch)
	cCommit := commitParent(t, root, newTip) // D's parent is C (or A if C elided)
	// The M-stage commit (C, the tip's own first parent below D) must
	// still carry the artifact byte-identical to tip ("X") — never "Y".
	if !fileContainsAtRef(t, root, cCommit, ".beads/issues.jsonl", `"status":"X"`) {
		t.Error("expected the M-stage commit to carry the artifact UNCHANGED (status X), never the live-drifted content")
	}
	if fileContainsAtRef(t, root, cCommit, ".beads/issues.jsonl", `"status":"Y"`) {
		t.Error("the M-stage commit must NEVER see the artifact's drifted content")
	}
	// Only the FINAL tip (D) carries the refreshed artifact.
	if !fileContainsAtRef(t, root, newTip, ".beads/issues.jsonl", `"status":"Y"`) {
		t.Error("expected the S-stage (final tip) commit to carry the refreshed artifact content (status Y)")
	}
}

// --- (iv): a gate->commit interval mutation is ordinary unstaged dirt ------
//
// A file written into the worktree AFTER the snapshot is taken but
// BEFORE the materialization commit lands is absent from that commit
// (built from the already-captured T) and, after the mixed reset,
// reads as ORDINARY untracked dirt — never an index/HEAD split. The run
// is stopped right after the M-stage advance lands (before the SEPARATE
// S-stage re-snapshots, which would legitimately pick the race file up
// on its own later re-measurement — a different, non-buggy event this
// leg is not about) so the assertion targets C itself, unambiguously.
func TestBead6Leg_iv_GateToCommitIntervalMutationIsOrdinaryDirt(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg4", "mindspec-128leg4.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Four() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &snapshotRaceExecutor{Executor: base, t: t, killAfterFirstReset: true, injectAfterFirstSnapshot: func() {
		if err := os.WriteFile(filepath.Join(wtPath, "race-file.txt"), []byte("interval\n"), 0o644); err != nil {
			t.Fatalf("writing race file: %v", err)
		}
	}}

	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the post-advance kill to fail Run")
	}
	cCommit := gateRevParse(t, root, beadBranch)
	if fileExistsAtRef(t, root, cCommit, "race-file.txt") {
		t.Error("the interval-race file must be ABSENT from the M-stage commit (built from the already-captured snapshot)")
	}
	if status := gitStatusPorcelain(t, wtPath); !strings.Contains(status, "race-file.txt") {
		t.Errorf("expected the race file to survive as ordinary untracked dirt after the mixed reset, got status: %q", status)
	}
}

// snapshotRaceExecutor calls injectAfterFirstSnapshot once, immediately
// after the FIRST real SnapshotWorktreeTree call returns — modeling a
// worktree mutation landing in the gate->commit interval — and,
// optionally, kills right after the FIRST real ResetIndexToHead call
// (letting R's index reset run for real, so the "ordinary untracked
// dirt, never a phantom split" claim is directly observable) but BEFORE
// the S-stage's own SEPARATE re-snapshot would otherwise legitimately
// pick the same file up on its own later measurement.
type snapshotRaceExecutor struct {
	executor.Executor
	t                        *testing.T
	injectAfterFirstSnapshot func()
	fired                    bool

	killAfterFirstReset bool
	resetCalls          int
}

func (e *snapshotRaceExecutor) SnapshotWorktreeTree(wtPath string, exclude []string) (string, error) {
	tree, err := e.Executor.SnapshotWorktreeTree(wtPath, exclude)
	if err == nil && !e.fired && e.injectAfterFirstSnapshot != nil {
		e.fired = true
		e.injectAfterFirstSnapshot()
	}
	return tree, err
}

func (e *snapshotRaceExecutor) ResetIndexToHead(wtPath string) error {
	if err := e.Executor.ResetIndexToHead(wtPath); err != nil {
		return err
	}
	e.resetCalls++
	if e.killAfterFirstReset && e.resetCalls == 1 {
		return fakeErr("fault-injection: kill after R's real index reset landed, before the S-stage re-snapshot")
	}
	return nil
}

// --- (v): the user commit's diff vs tip NEVER touches the artifact --------
func TestBead6Leg_v_UserCommitNeverTouchesArtifact(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg5", "mindspec-128leg5.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed BASELINE artifact first: next.CheckDirtyTreeDetail's real
	// porcelain classification reports a genuinely UNTRACKED .beads/ as the
	// directory entry itself (`?? .beads/`), not the per-file
	// `.beads/issues.jsonl` shape the artifact classifier matches — tracking
	// it first makes the subsequent edit a per-file `M .beads/issues.jsonl`.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "-A")
	gitRun(t, wtPath, "commit", "-q", "-m", "tracked artifact")
	preTip := gateRevParse(t, root, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Five() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	if _, err := Run(root, beadID, specID, "work", base, CompleteOpts{}); err != nil {
		t.Fatalf("expected completion, got: %v", err)
	}
	newTip := gateRevParse(t, root, beadBranch)
	cCommit := commitParent(t, root, newTip)
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", preTip, cCommit).Output()
	if err != nil {
		t.Fatalf("diff %s %s: %v", preTip, cCommit, err)
	}
	if strings.Contains(string(out), ".beads/issues.jsonl") {
		t.Errorf("the user (M-stage) commit's diff vs tip must NEVER touch the named artifact, got diff: %s", out)
	}
}

// --- (vi): an unchanged-after-snapshot worktree is status-CLEAN afterward ---
func TestBead6Leg_vi_UnchangedWorktreeIsCleanAfterMaterialize(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg6", "mindspec-128leg6.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Six() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	if _, err := Run(root, beadID, specID, "work", base, CompleteOpts{}); err != nil {
		t.Fatalf("expected completion, got: %v", err)
	}
	// The bead worktree was removed by the real CompleteBead cleanup —
	// re-derive the equivalent check against the spec worktree, which now
	// carries the merged content and must ALSO be clean.
	specWt := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	if status := gitStatusPorcelain(t, specWt); status != "" {
		t.Errorf("expected a clean spec worktree after the merge landed, got:\n%s", status)
	}
}

// --- (vii): the excluded artifact is byte-identical at tip/T/user-commit ---
func TestBead6Leg_vii_ArtifactByteIdenticalUntilSyncCommit(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg7", "mindspec-128leg7.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	const tipArtifact = `{"id":"x","n":1}` + "\n"
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(tipArtifact), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "-A")
	gitRun(t, wtPath, "commit", "-q", "-m", "tracked artifact")
	tip := gateRevParse(t, root, beadBranch)

	const driftedArtifact = `{"id":"x","n":2}` + "\n"
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(driftedArtifact), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Seven() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	if _, err := Run(root, beadID, specID, "work", base, CompleteOpts{}); err != nil {
		t.Fatalf("expected completion, got: %v", err)
	}
	newTip := gateRevParse(t, root, beadBranch)
	cCommit := commitParent(t, root, newTip)

	if !fileContainsAtRef(t, root, tip, `.beads/issues.jsonl`, `"n":1`) {
		t.Fatal("fixture sanity: tip should carry n:1")
	}
	if !fileContainsAtRef(t, root, cCommit, `.beads/issues.jsonl`, `"n":1`) {
		t.Error("the excluded artifact must be byte-identical to tip in the user commit (n:1)")
	}
	if !fileContainsAtRef(t, root, newTip, `.beads/issues.jsonl`, `"n":2`) {
		t.Error("only the final (sync) commit should carry the drifted artifact content (n:2)")
	}
}

// --- shared decorators for the resume/marker/race legs (viii-xx) -----------

// markerAdvanceRaceExecutor extends killAfterExecutor's shape with a
// SEPARATE kill point BETWEEN PublishStageMarker and the following
// AdvanceBranchExpectedOld — legs (xii)/(xiii)'s "marker published, ref
// not yet advanced" window. killPublishAfterCall selects which
// 1-indexed PublishStageMarker call to kill after (0 disables).
type markerAdvanceRaceExecutor struct {
	executor.Executor
	t *testing.T

	killPublishAfterCall int
	publishCalls         int

	killAdvanceAfterCall int
	advanceCalls         int

	// killCompleteBeadBeforeCall forces an error at the TOP of
	// CompleteBead, before the real merge is ever attempted — modeling a
	// kill in the pre-merge window with no mutation to let land (legs
	// xix/xx).
	killCompleteBeadBeforeCall bool
	completeBeadCalls          int

	// killOnSnapshotCall, when non-zero, forces an error on that
	// 1-indexed SnapshotWorktreeTree call, AFTER the real (non-mutating)
	// snapshot itself returns successfully — modeling a kill between the
	// S-stage's own snapshot and its first durable write (leg xviii).
	killOnSnapshotCall int
	snapshotCalls      int
}

func (e *markerAdvanceRaceExecutor) PublishStageMarker(beadID, oid, expectedOld string) error {
	if err := e.Executor.PublishStageMarker(beadID, oid, expectedOld); err != nil {
		return err
	}
	e.publishCalls++
	if e.killPublishAfterCall != 0 && e.publishCalls == e.killPublishAfterCall {
		return fakeErr("fault-injection: kill after PublishStageMarker landed, before the ref advance")
	}
	return nil
}

func (e *markerAdvanceRaceExecutor) AdvanceBranchExpectedOld(branch, newSHA, expectedOld string) error {
	if err := e.Executor.AdvanceBranchExpectedOld(branch, newSHA, expectedOld); err != nil {
		return err
	}
	e.advanceCalls++
	if e.killAdvanceAfterCall != 0 && e.advanceCalls == e.killAdvanceAfterCall {
		return fakeErr("fault-injection: kill after the branch advance landed")
	}
	return nil
}

func (e *markerAdvanceRaceExecutor) SnapshotWorktreeTree(wtPath string, exclude []string) (string, error) {
	tree, err := e.Executor.SnapshotWorktreeTree(wtPath, exclude)
	if err != nil {
		return tree, err
	}
	e.snapshotCalls++
	if e.killOnSnapshotCall != 0 && e.snapshotCalls == e.killOnSnapshotCall {
		return "", fakeErr("fault-injection: kill immediately after the snapshot landed, before any write")
	}
	return tree, nil
}

func (e *markerAdvanceRaceExecutor) CompleteBead(beadID, specBranch, msg, overrideReason string, resolveMerge bool) error {
	if e.killCompleteBeadBeforeCall && e.completeBeadCalls == 0 {
		e.completeBeadCalls++
		return fakeErr("fault-injection: kill immediately before the merge attempt starts")
	}
	e.completeBeadCalls++
	return e.Executor.CompleteBead(beadID, specBranch, msg, overrideReason, resolveMerge)
}

// readMarker is the test-side inspection of a bead's write-ahead marker,
// via gitutil directly (tests are not bound by ADR-0030's executor-only
// rule — only production code is).
func readMarker(t *testing.T, root, beadID string) (oid, stage string, present bool) {
	t.Helper()
	oid, stage, present, err := gitutil.ReadMaterializationMarker(root, beadID)
	if err != nil {
		t.Fatalf("ReadMaterializationMarker: %v", err)
	}
	return oid, stage, present
}

// --- (viii-a)/(viii-b): resume-kill before the reset ------------------------
func TestBead6Leg_viii_a_ResumeKillBeforeReset_IntervalEditRefuses(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg8", "mindspec-128leg8.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)
	checkDirtyTreeFn = next.CheckDirtyTreeDetail

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Eight() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killAdvanceAfterCall: 1}

	// KILL: marker published, ref advanced to C, then the decorator kills
	// BEFORE ResetIndexToHead runs.
	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	oid, stage, present := readMarker(t, root, beadID)
	tipAfterKill := gateRevParse(t, root, beadBranch)
	if !present || stage != gitutil.StageMaterialization || oid != tipAfterKill {
		t.Fatalf("expected marker==tip==C (stage=materialization), got marker=%s stage=%s present=%v tip=%s", oid, stage, present, tipAfterKill)
	}

	// An interval edit lands during the "downtime".
	editPath := filepath.Join(wtPath, "interval-edit.txt")
	if err := os.WriteFile(editPath, []byte("uncommitted work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ex.killAdvanceAfterCall = 0
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the S-exit contract to refuse on the genuine interval edit")
	}
	if !strings.Contains(err.Error(), "interval-edit.txt") {
		t.Errorf("refusal should name the interval-edit path, got: %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected a recovery line, got: %v", err)
	}
	// NO rollback: the branch tip is untouched by the refusal.
	if got := gateRevParse(t, root, beadBranch); got != tipAfterKill {
		t.Errorf("expected no rollback — tip must stay at %s, got %s", tipAfterKill, got)
	}
	// The worktree is NOT removed while the edit sits uncommitted.
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Errorf("expected the bead worktree to survive the refusal, stat error: %v", statErr)
	}
	if content, readErr := os.ReadFile(editPath); readErr != nil || string(content) != "uncommitted work\n" {
		t.Errorf("expected the interval edit's content to survive on disk, got %q err=%v", content, readErr)
	}
	if ex.completeBeadCalls != 0 {
		t.Errorf("expected NO merge attempt before the S-exit refusal, got %d", ex.completeBeadCalls)
	}

	// --- (viii-b): clearing the dirt converges. ---
	if err := os.Remove(editPath); err != nil {
		t.Fatal(err)
	}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected convergence after clearing the interval edit, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be a real ancestor of the spec branch after convergence")
	}
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected the write-ahead marker to be cleared after a successful completion")
	}
}

// --- (viii-b, stale-marker variant): a foreign tip falls through cleanly ---
func TestBead6Leg_viii_b_StaleForeignMarkerFallsThroughToNormalRun(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg8b", "mindspec-128leg8b.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc EightB() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killAdvanceAfterCall: 1}
	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	markerOID, _, present := readMarker(t, root, beadID)
	if !present {
		t.Fatal("expected a marker after the kill")
	}

	// A GENUINELY FOREIGN commit lands on the branch — not the marker,
	// not its parent (a real, unrelated, additional commit).
	gitRun(t, wtPath, "commit", "--allow-empty", "-q", "-m", "foreign commit")
	foreignTip := gateRevParse(t, root, beadBranch)
	if foreignTip == markerOID {
		t.Fatal("fixture sanity: the foreign tip must differ from the marker")
	}

	ex.killAdvanceAfterCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected the foreign-tip run to fall through and converge (no panel registered), got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected the stale marker to have been cleared (foreign-tip classification)")
	}
}

// --- (ix): kill after the S-stage ref advance -------------------------------
func TestBead6Leg_ix_KillAfterSStageAdvance_ConvergesAndIntervalEditRefuses(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg9", "mindspec-128leg9.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Nine() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed BASELINE artifact first: next.CheckDirtyTreeDetail's real
	// porcelain classification reports a genuinely UNTRACKED .beads/ as the
	// directory entry itself (`?? .beads/`), not the per-file
	// `.beads/issues.jsonl` shape the artifact classifier matches — tracking
	// it first makes the subsequent edit a per-file `M .beads/issues.jsonl`.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killAdvanceAfterCall: 2}

	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	oid, stage, present := readMarker(t, root, beadID)
	tip := gateRevParse(t, root, beadBranch)
	if !present || stage != gitutil.StageArtifactSync || oid != tip {
		t.Fatalf("expected marker==tip==D (stage=artifact-sync), got marker=%s stage=%s present=%v tip=%s", oid, stage, present, tip)
	}

	// Interval edit variant: refuses the same way (ix's second half).
	editPath := filepath.Join(wtPath, "interval-edit-ix.txt")
	if err := os.WriteFile(editPath, []byte("edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ex.killAdvanceAfterCall = 0
	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the S-exit contract to refuse on the interval edit even resuming from the D marker")
	} else if !strings.Contains(err.Error(), "interval-edit-ix.txt") {
		t.Errorf("expected the refusal to name the interval-edit path, got: %v", err)
	}

	// Clear it and converge for real.
	if err := os.Remove(editPath); err != nil {
		t.Fatal(err)
	}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected convergence, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
}

// --- (x): kill during a preserved merge conflict ----------------------------
func TestBead6Leg_x_KillDuringPreservedMergeConflict(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg10", "mindspec-128leg10.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// A CONFLICTING spec-side change to the SAME path the bead touches.
	specWt := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	writeFile(t, specWt, "internal/widget/widget.go", "package widget\n\nfunc New() {}\n\nfunc SpecSide() {}\n")
	gitRun(t, specWt, "add", "-A")
	gitRun(t, specWt, "commit", "-q", "-m", "spec-side conflicting change")

	// The bead-side change to the SAME path, plus real artifact dirt so
	// the tip reaches D (marker D) before the merge is ever attempted.
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc BeadSide() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed BASELINE artifact first: next.CheckDirtyTreeDetail's real
	// porcelain classification reports a genuinely UNTRACKED .beads/ as the
	// directory entry itself (`?? .beads/`), not the per-file
	// `.beads/issues.jsonl` shape the artifact classifier matches — tracking
	// it first makes the subsequent edit a per-file `M .beads/issues.jsonl`.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	// No forced kill needed: materialization completes for real (M+S
	// both build), THEN the real merge conflicts on its own.
	_, err := Run(root, beadID, specID, "work", base, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the real merge conflict to fail Run")
	}
	if !strings.Contains(err.Error(), "CLOSED in Dolt") {
		t.Errorf("expected the closed-but-unmerged conflict message, got: %v", err)
	}
	oid, stage, present := readMarker(t, root, beadID)
	tip := gateRevParse(t, root, beadBranch)
	if !present || stage != gitutil.StageArtifactSync || oid != tip {
		t.Fatalf("expected marker==tip==D to survive the merge conflict, got marker=%s stage=%s present=%v tip=%s", oid, stage, present, tip)
	}

	// Re-run WITHOUT --resolve-merge: the pre-existing preserved-merge
	// machinery refuses again, naming the resolution steps — no
	// re-panel, no re-materialization (the discriminator resumes
	// straight at the S-exit-then-merge route).
	if _, err := Run(root, beadID, specID, "", base, CompleteOpts{}); err == nil {
		t.Fatal("expected the conflict to still block without --resolve-merge")
	}

	// Resolve the conflict for real, stage it, then re-run WITH
	// --resolve-merge: converges through the pre-existing spec-127
	// machinery.
	writeFile(t, specWt, "internal/widget/widget.go", "package widget\n\nfunc New() {}\n\nfunc Resolved() {}\n")
	gitRun(t, specWt, "add", "-A")

	res, err := Run(root, beadID, specID, "", base, CompleteOpts{ResolveMerge: true})
	if err != nil {
		t.Fatalf("expected --resolve-merge to converge, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !fileContainsAtRef(t, root, specBranch, "internal/widget/widget.go", "Resolved") {
		t.Error("expected the resolved content to land on the spec branch")
	}
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected the marker to be cleared after the preserved-merge conflict finally converges")
	}
}

// --- (xi): a genuinely foreign commit trips the panel stale-SHA gate ------
func TestBead6Leg_xi_ForeignCommitAfterMaterializationBlocksOnStaleSHA(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg11", "mindspec-128leg11.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// Register a panel reviewing the bead's ORIGINAL (pre-materialization)
	// tip — the SHA the panel actually reviewed.
	reviewedSHA := gateRevParse(t, root, beadBranch)
	writePanel(t, root, specID+"-"+beadID, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 2,
		ReviewedHeadSHA: reviewedSHA,
	}, map[string]string{
		"a-round-1.json": "APPROVE",
		"b-round-1.json": "APPROVE",
	})

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Eleven() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killAdvanceAfterCall: 1}
	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}

	// A genuinely foreign commit lands — advancing PAST the marker, not
	// finishing an unfinished advance.
	gitRun(t, wtPath, "commit", "--allow-empty", "-q", "-m", "foreign commit after materialization")

	ex.killAdvanceAfterCall = 0
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the panel stale-SHA gate to block on the foreign post-materialization commit")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "stale") && !strings.Contains(err.Error(), "reviewed") {
		t.Errorf("expected a stale-SHA panel block, got: %v", err)
	}
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected the marker to be cleared once the tip is classified FOREIGN")
	}
}

// --- (xii)/(xiii): kill between marker publication and the ref advance -----
func TestBead6Leg_xii_KillBetweenMarkerAndRefAdvance_MStage(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg12", "mindspec-128leg12.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)
	preTip := gateRevParse(t, root, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Twelve() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killPublishAfterCall: 1}

	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	// Marker published (naming C), but the ref advance never ran — tip
	// still at A == marker's own first parent.
	oid, stage, present := readMarker(t, root, beadID)
	if !present || stage != gitutil.StageMaterialization {
		t.Fatalf("expected a published materialization marker, got stage=%s present=%v", stage, present)
	}
	if got := gateRevParse(t, root, beadBranch); got != preTip {
		t.Errorf("expected the ref advance to NOT have run yet, tip should still be %s, got %s", preTip, got)
	}
	if parent := commitParent(t, root, oid); parent != preTip {
		t.Fatalf("fixture sanity: marker's first parent should be the pre-run tip")
	}

	ex.killPublishAfterCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected the plain re-run to finish the advance and converge, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
}

func TestBead6Leg_xiii_KillBetweenMarkerAndRefAdvance_SStage(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg13", "mindspec-128leg13.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Thirteen() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed BASELINE artifact first: next.CheckDirtyTreeDetail's real
	// porcelain classification reports a genuinely UNTRACKED .beads/ as the
	// directory entry itself (`?? .beads/`), not the per-file
	// `.beads/issues.jsonl` shape the artifact classifier matches — tracking
	// it first makes the subsequent edit a per-file `M .beads/issues.jsonl`.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	// The FIRST publish (M-stage, absent->C) must succeed and its
	// advance must complete for the run to reach the SECOND publish
	// (S-stage, C->D) — kill at publish call #2.
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killPublishAfterCall: 2}

	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	oid, stage, present := readMarker(t, root, beadID)
	if !present || stage != gitutil.StageArtifactSync {
		t.Fatalf("expected a published artifact-sync marker, got stage=%s present=%v", stage, present)
	}
	tipAfterKill := gateRevParse(t, root, beadBranch)
	if parent := commitParent(t, root, oid); parent != tipAfterKill {
		t.Fatalf("expected the marker's first parent (%s) to equal the current tip (%s) — the S-stage advance never ran", parent, tipAfterKill)
	}

	ex.killPublishAfterCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected the plain re-run to finish the advance and converge, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
}

// --- (xiv): concurrent-publisher CAS race -----------------------------------
type concurrentPublisherExecutor struct {
	executor.Executor
	t        *testing.T
	root     string
	beadID   string
	rivalOID string
	injected bool
}

func (e *concurrentPublisherExecutor) PublishStageMarker(beadID, oid, expectedOld string) error {
	if !e.injected {
		e.injected = true
		// A "concurrent invocation" wins the race and publishes first.
		if err := gitutil.PublishMaterializationMarker(e.root, e.beadID, e.rivalOID, ""); err != nil {
			e.t.Fatalf("simulating the concurrent publisher: %v", err)
		}
	}
	return e.Executor.PublishStageMarker(beadID, oid, expectedOld)
}

func TestBead6Leg_xiv_ConcurrentPublisherCASRefusesWithoutOverwriting(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg14", "mindspec-128leg14.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Fourteen() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real, valid, pre-existing commit stands in for the rival
	// invocation's own (different) materialization commit.
	rivalOID := gateRevParse(t, root, beadBranch)

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &concurrentPublisherExecutor{Executor: base, t: t, root: root, beadID: beadID, rivalOID: rivalOID}

	_, err := Run(root, beadID, specID, "work", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the CAS race to refuse Run")
	}
	if !strings.Contains(err.Error(), "materialization") {
		t.Errorf("expected a materialization-race refusal, got: %v", err)
	}
	oid, _, present := readMarker(t, root, beadID)
	if !present || oid != rivalOID {
		t.Errorf("expected the rival's marker OID (%s) to survive UNCHANGED, got oid=%s present=%v", rivalOID, oid, present)
	}
}

// --- (xv): empty-C elision (D parents directly on A) ------------------------
func TestBead6Leg_xv_EmptyCElision(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg15", "mindspec-128leg15.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// NO non-artifact work — only artifact dirt, so M elides entirely.
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed BASELINE artifact first: next.CheckDirtyTreeDetail's real
	// porcelain classification reports a genuinely UNTRACKED .beads/ as the
	// directory entry itself (`?? .beads/`), not the per-file
	// `.beads/issues.jsonl` shape the artifact classifier matches — tracking
	// it first makes the subsequent edit a per-file `M .beads/issues.jsonl`.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
	// preTip is captured AFTER the baseline-artifact commit — that commit
	// is real, prior "A" work Run() must treat as already-landed tip, not
	// part of this run's own prospective content.
	preTip := gateRevParse(t, root, beadBranch)
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killAdvanceAfterCall: 1}
	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	oid, stage, present := readMarker(t, root, beadID)
	if !present || stage != gitutil.StageArtifactSync {
		t.Fatalf("expected D's marker directly (M elided, no C ever published), got stage=%s present=%v", stage, present)
	}
	if parent := commitParent(t, root, oid); parent != preTip {
		t.Errorf("expected D to parent DIRECTLY on A (%s) since C elided, got parent %s", preTip, parent)
	}

	ex.killAdvanceAfterCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected convergence, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
}

// --- (xvi): no-op-D elision --------------------------------------------------
func TestBead6Leg_xvi_NoOpDElision(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg16", "mindspec-128leg16.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// Real non-artifact work, NO artifact dirt at all — D must elide.
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Sixteen() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killAdvanceAfterCall: 1}
	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	oid, stage, present := readMarker(t, root, beadID)
	tip := gateRevParse(t, root, beadBranch)
	if !present || stage != gitutil.StageMaterialization || oid != tip {
		t.Fatalf("expected marker==tip==C (stage=materialization), got marker=%s stage=%s present=%v tip=%s", oid, stage, present, tip)
	}

	ex.killAdvanceAfterCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected the resume to replay R, elide S, pass the S-exit contract, and converge, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	// D elided: the final merged tip's own parent chain never gained a
	// second (artifact-sync) commit beyond C.
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
}

// --- (xvii): ordinary artifact-changing D — trailer classification --------
func TestBead6Leg_xvii_OrdinaryArtifactChangingD(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg17", "mindspec-128leg17.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Seventeen() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed BASELINE artifact first: next.CheckDirtyTreeDetail's real
	// porcelain classification reports a genuinely UNTRACKED .beads/ as the
	// directory entry itself (`?? .beads/`), not the per-file
	// `.beads/issues.jsonl` shape the artifact classifier matches — tracking
	// it first makes the subsequent edit a per-file `M .beads/issues.jsonl`.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killAdvanceAfterCall: 2}
	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}

	// The marker's OWN trailer — never a tree-diff guess — identifies D.
	oid, stage, present := readMarker(t, root, beadID)
	if !present {
		t.Fatal("expected a marker after the kill")
	}
	body, err := gitutil.CommitMessageBody(root, oid)
	if err != nil {
		t.Fatalf("CommitMessageBody: %v", err)
	}
	if !strings.Contains(body, gitutil.MindspecStageTrailerKey+": "+gitutil.StageArtifactSync) {
		t.Errorf("expected the marker commit to carry the pinned Mindspec-Stage trailer naming artifact-sync, got body: %q", body)
	}
	if stage != gitutil.StageArtifactSync {
		t.Fatalf("expected ReadMaterializationMarker's OWN classification (from the trailer) to say artifact-sync, got %q", stage)
	}

	ex.killAdvanceAfterCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected convergence, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
}

// --- (xviii): kill after R with C elided, before any D publication --------
func TestBead6Leg_xviii_KillAfterRWithCElided_NoMarker(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg18", "mindspec-128leg18.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// Real artifact dirt (so D WOULD build), no non-artifact work (M
	// elides).
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed BASELINE artifact first: next.CheckDirtyTreeDetail's real
	// porcelain classification reports a genuinely UNTRACKED .beads/ as the
	// directory entry itself (`?? .beads/`), not the per-file
	// `.beads/issues.jsonl` shape the artifact classifier matches — tracking
	// it first makes the subsequent edit a per-file `M .beads/issues.jsonl`.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
	// preTip is captured AFTER the baseline-artifact commit.
	preTip := gateRevParse(t, root, beadBranch)
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	// M elides (no advance call #1 will ever happen from M); the FIRST
	// SnapshotWorktreeTree call to actually matter for a durable write is
	// the S-stage's own (no-exclude) snapshot — kill immediately after
	// IT returns, before D is ever built or published.
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killOnSnapshotCall: 2}

	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the kill to fail Run")
	}
	// Nothing durable happened: no marker, tip unchanged.
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected NO marker — the kill landed before any publication")
	}
	if got := gateRevParse(t, root, beadBranch); got != preTip {
		t.Errorf("expected the tip to remain at the reviewed SHA %s, got %s", preTip, got)
	}

	ex.killOnSnapshotCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected an ORDINARY re-entry (no marker) to re-measure and converge, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
}

// --- (xix): kill after a no-op S, pre-merge, marker C present -------------
func TestBead6Leg_xix_KillAfterNoOpS_ResumeIdempotent(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg19", "mindspec-128leg19.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// Real non-artifact work (M builds C); no artifact dirt (S elides).
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Nineteen() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	// Let M's advance land for real (no kill there); kill immediately
	// BEFORE the merge attempt starts — S has already elided by this
	// point (no mutation to let land at the merge boundary itself).
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killCompleteBeadBeforeCall: true}

	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the pre-merge kill to fail Run")
	}
	oid, stage, present := readMarker(t, root, beadID)
	tip := gateRevParse(t, root, beadBranch)
	if !present || stage != gitutil.StageMaterialization || oid != tip {
		t.Fatalf("expected marker==tip==C (S elided, never published its own marker), got marker=%s stage=%s present=%v tip=%s", oid, stage, present, tip)
	}

	ex.killCompleteBeadBeforeCall = false
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected the resume (idempotent R replay + no-op S replay + S-exit + merge) to converge, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
	if ex.completeBeadCalls != 2 {
		t.Errorf("expected exactly 2 CompleteBead calls (1 pre-merge kill + 1 real convergence), got %d", ex.completeBeadCalls)
	}
}

// --- (xx): both stages elided, kill pre-merge — the no-marker route -------
func TestBead6Leg_xx_BothStagesElided_NoMarkerRoute(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg20", "mindspec-128leg20.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	_ = wireBeadWorktree(t, root, specID, beadID, beadBranch)
	preTip := gateRevParse(t, root, beadBranch)

	// NOTHING to materialize at all: no --commit-msg content, no
	// artifact dirt (checkDirtyTreeFn's default-clean stub, set by
	// saveAndRestore, already applies).
	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &markerAdvanceRaceExecutor{Executor: base, t: t, killCompleteBeadBeforeCall: true}

	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the pre-merge kill to fail Run")
	}
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected NO marker — both stages elided, so nothing was ever published")
	}
	if got := gateRevParse(t, root, beadBranch); got != preTip {
		t.Errorf("expected the tip to remain unchanged at %s, got %s", preTip, got)
	}

	ex.killCompleteBeadBeforeCall = false
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected the no-marker route (an ordinary re-run) to converge, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
	if ex.completeBeadCalls != 2 {
		t.Errorf("expected exactly 2 CompleteBead calls (1 pre-merge kill + 1 real convergence), got %d", ex.completeBeadCalls)
	}
}

// --- (xxi): fix-round G1 — a mutation landing between the pre-S dirt
// classification and the S-stage's own SnapshotWorktreeTree call must
// REFUSE, never land silently in commit D. -------------------------

// sStageSnapshotRaceExecutor counts SnapshotWorktreeTree calls and, on
// the SECOND call (the S-stage's own artifact-sync `add -A` snapshot),
// injects a worktree write BEFORE delegating to the real snapshot —
// modeling an external writer (an editor flush, a concurrent agent)
// landing in the exact window between runMaterialization's pre-S dirt
// classification and the S-stage's own tree2 snapshot. This is the
// round-1 panel's R4 slot's EXECUTED repro: at 482bcd2f (pre-fix-round),
// the injected file was silently swept into commit D and the run
// SUCCEEDED with BeadClosed, the file present at the final bead tip.
// Fix-round G1 closes this by classifying D's own exact delta
// (changedFromCurrent) and refusing on any non-artifact path there,
// before MaterializeStageCommit/PublishStageMarker ever runs.
type sStageSnapshotRaceExecutor struct {
	executor.Executor
	t             *testing.T
	inject        func()
	snapshotCalls int
}

func (e *sStageSnapshotRaceExecutor) SnapshotWorktreeTree(wtPath string, exclude []string) (string, error) {
	e.snapshotCalls++
	if e.snapshotCalls == 2 && e.inject != nil {
		e.inject()
	}
	return e.Executor.SnapshotWorktreeTree(wtPath, exclude)
}

func TestBead6Leg_xxi_SStageSnapshotRaceRefusesBeforeCommittingD(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg21", "mindspec-128leg21.1"
	root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)
	preTip := gateRevParse(t, root, beadBranch)

	// Pre-existing content so the M-stage snapshot (call 1) has
	// something real to build C from — mirrors leg iv's setup.
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc TwentyOne() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &sStageSnapshotRaceExecutor{Executor: base, t: t, inject: func() {
		if err := os.WriteFile(filepath.Join(wtPath, "race-file.txt"), []byte("interval\n"), 0o644); err != nil {
			t.Fatalf("writing race file: %v", err)
		}
	}}

	if _, err := Run(root, beadID, specID, "work", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the S-stage snapshot race to REFUSE rather than complete successfully")
	}

	// The M-stage commit C must have landed (the refusal is a POST-R,
	// PRE-D-publish refusal, not a rejection of the whole run) — the
	// branch tip must have moved off preTip to C, but no FURTHER: the
	// race file must never appear at any committed tip.
	cCommit := gateRevParse(t, root, beadBranch)
	if cCommit == preTip {
		t.Error("expected the M-stage commit C to have landed before the S-stage refusal fired")
	}
	if fileExistsAtRef(t, root, cCommit, "race-file.txt") {
		t.Error("the race file must be ABSENT from every committed tip — it must never reach commit D")
	}
	if status := gitStatusPorcelain(t, wtPath); !strings.Contains(status, "race-file.txt") {
		t.Errorf("expected the race file to survive on disk as untracked dirt behind the refusal, got status: %q", status)
	}
	// The M-stage's OWN write-ahead marker (published before the S-stage
	// race even runs) legitimately survives, naming C as the resumable
	// stage — the refusal happens strictly AFTER M's publish and strictly
	// BEFORE the S-stage's own PublishStageMarker call, so this is the
	// correct resumption evidence for a later re-run, not a leak.
	if oid, stage, present := readMarker(t, root, beadID); !present || oid != cCommit || stage != stageMaterialization {
		t.Errorf("expected the surviving marker to name C (%s) at stage %q, got oid=%q stage=%q present=%v", cCommit, stageMaterialization, oid, stage, present)
	}
}

// --- (xxii): fix-round G5 — resume after a stage advance does not trip
// the panel's stale-SHA gate, even with a LIVE panel registered.

// TestBead6Leg_xxii_ResumeAfterStageAdvanceDoesNotTripStaleSHA is R2's
// positive resume leg (round-1 panel G5): a panel registered at A, a
// stage advance landing for real then killed (marker == tip == the new
// commit), a re-run WITHOUT any foreign commit landing on the bead
// branch must CONVERGE (BeadClosed, ancestor, marker cleared) rather
// than trip the panel's stale-SHA check against a tip that no longer
// equals the panel's own ReviewedHeadSHA (A) — proving Run's
// resumeStage=="" guard on the panel gate holds even with a real,
// otherwise-would-trip panel in place, not just in the marker-only legs
// (ix/xii) that register no panel at all.
//
// Deliberately ARTIFACT-ONLY dirt (no user-authored .go change): the
// pre-existing panel gate (internal/panel/gate.go's dirty-tree clause,
// Req 11 — bead 6 does not touch it) unconditionally BLOCKS whenever a
// panel is registered AND the worktree carries UNCOMMITTED USER dirt
// ("would auto-commit them past review") — a live panel and fresh user
// work are mutually exclusive by design, so the only stage this leg CAN
// exercise under a live panel is the artifact-sync stage (D), which the
// M-stage's own snapshot excludes entirely (materializedArtifactPaths).
// The property under test — resume bypasses the panel-gate re-check
// entirely (Run's resumeStage!="" skip) — is IDENTICAL regardless of
// which named stage's advance gets killed.
func TestBead6Leg_xxii_ResumeAfterStageAdvanceDoesNotTripStaleSHA(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg22", "mindspec-128leg22.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed baseline artifact first (mirrors leg xvii): a genuinely
	// untracked .beads/ classifies as the directory entry itself, not
	// the per-file artifact shape the classifier matches. Committed (and
	// the panel registered at the RESULTING tip) BEFORE the actual dirty
	// edit below and before the panel is registered, so the baseline
	// commit itself is never mistaken for foreign drift against the
	// panel's own reviewed SHA.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")

	a := gateRevParse(t, root, beadBranch)
	writePanel(t, root, specID+"-leg22", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: a,
	}, approveVerdicts(6))

	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"leg22"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	// No user dirt exists, so the M-stage is elided entirely — the
	// artifact-sync (S) stage's own advance is the FIRST (and only)
	// AdvanceBranchExpectedOld call this run makes.
	ex := &killAfterExecutor{Executor: base, t: t, killAdvanceAfterCall: 1}

	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{}); err == nil {
		t.Fatal("expected the post-advance kill to fail Run")
	}

	dCommit := gateRevParse(t, root, beadBranch)
	if dCommit == a {
		t.Fatal("expected the artifact-sync advance to have moved the tip off A before the kill landed")
	}
	if oid, stage, present := readMarker(t, root, beadID); !present || oid != dCommit || stage != stageArtifactSync {
		t.Fatalf("expected a surviving marker naming the artifact-sync commit (%s), got oid=%q stage=%q present=%v", dCommit, oid, stage, present)
	}

	ex.killAdvanceAfterCall = 0
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected the resume (no foreign commit) to converge WITHOUT tripping the panel's stale-SHA gate (registered at A=%s, tip now %s), got: %v", a, dCommit, err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected the write-ahead marker to be cleared after convergence")
	}
}

// --- (xxiii): fix-round G5 — the MISSING AC-6 composite flagship fixture.

// setupAC6FlagshipFixture builds a real-git fixture whose bead branch tip
// is a source-only change RELATIVE TO ITS OWN MERGE-BASE: spec.md,
// plan.md, the ADR, and the domain's OWNERSHIP.yaml are all committed on
// MAIN before the spec/bead branches ever fork, so ATTRIBUTION (which
// reads the full snapshot tree, independent of diff range) still finds
// the domain properly claimed — but the base..head DIFF the doc-sync
// gate measures contains ONLY the new .go file, no doc file at all,
// genuinely tripping doc-sync's plain "source changed, no docs updated"
// lane (never ADR-divergence, which never sees anything unowned).
func setupAC6FlagshipFixture(t *testing.T, specID, beadID string) (root, specBranch, beadBranch string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	gitRun(t, root, "config", "user.name", "test")
	gitRun(t, root, "config", "commit.gpgsign", "false")
	writeFile(t, root, ".mindspec/docs/specs/"+specID+"/spec.md", "# Spec\n\n## Impacted Domains\n\n- flagship\n")
	writeFile(t, root, ".mindspec/docs/specs/"+specID+"/plan.md",
		"---\nstatus: Approved\nspec_id: \""+specID+"\"\nversion: \"1\"\nadr_citations:\n  - ADR-9600\n---\n\n# Plan\n")
	writeFile(t, root, ".mindspec/docs/adr/ADR-9600.md",
		"# ADR-9600: Flagship\n\n"+
			"- **Date**: 2026-01-01\n"+
			"- **Status**: Accepted\n"+
			"- **Domain(s)**: flagship\n"+
			"- **Supersedes**: n/a\n"+
			"- **Superseded-by**: n/a\n\n"+
			"## Decision\nTest fixture.\n")
	writeFile(t, root, ".mindspec/docs/domains/flagship/OWNERSHIP.yaml", "paths:\n  - internal/flagship/**\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base (docs+ownership pre-fork)")

	specBranch = "spec/" + specID
	beadBranch = "bead/" + beadID
	gitRun(t, root, "checkout", "-q", "-b", specBranch)
	gitRun(t, root, "checkout", "-q", "-b", beadBranch)
	writeFile(t, root, "internal/flagship/flagship.go", "package flagship\n\nfunc New() {}\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "impl: flagship source-only change")
	gitRun(t, root, "checkout", "-q", "main")

	specWtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir worktrees dir: %v", err)
	}
	gitRun(t, root, "worktree", "add", specWtPath, specBranch)

	return root, specBranch, beadBranch
}

// TestBead6Leg_xxiii_AC6CompositeFlagship_DocSyncRefusalThenAllowDocSkewConverges
// is round-1 panel G5's missing composite: the spec's headline two-run
// retry-convergence assertion (AC-6), pinned end-to-end against a REAL
// panel and REAL doc-sync gate. A panel registered at reviewed SHA A +
// a dirty tracker + a doc-sync-failing change: run 1 REFUSES at doc-sync
// (never the panel's stale-SHA gate) with the tip asserted STILL A; a
// plain retry meets the IDENTICAL gate; run 2 with --allow-doc-skew is
// decided by doc-sync (converges), never falling through to a
// manufactured stale-SHA block — the #186 regression this whole bead
// exists to close, pinned at the composite level R4's hand-driven
// flagship run exercised live.
func TestBead6Leg_xxiii_AC6CompositeFlagship_DocSyncRefusalThenAllowDocSkewConverges(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "930-leg23", "mindspec-128leg23.1"
	root, specBranch, beadBranch := setupAC6FlagshipFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)

	// A committed BASELINE artifact first (mirrors leg xvii/xxii): a
	// genuinely untracked .beads/ collapses to the directory entry
	// itself in `git status --porcelain` (`?? .beads/`), which the
	// panel gate's own artifact-path exemption
	// (internal/panel/gate.go's isArtifactPath, an EXACT match on
	// `.beads/issues.jsonl`) does NOT recognize — tracking it first (and
	// registering the panel AFTER, at the resulting tip) makes the
	// later edit a per-file `M .beads/issues.jsonl` line, which the
	// exemption does match, so this dirt never trips the pre-existing
	// panel-gate dirty-tree block (Req 11) a live panel would otherwise
	// apply.
	if err := os.MkdirAll(filepath.Join(wtPath, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"base"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "--", ".beads")
	gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")

	a := gateRevParse(t, root, beadBranch)
	writePanel(t, root, specID+"-leg23", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: a,
	}, approveVerdicts(6))

	// The genuine dirty tracker file, alongside the bead branch's own
	// already-committed doc-sync-failing source change.
	if err := os.WriteFile(filepath.Join(wtPath, ".beads", "issues.jsonl"), []byte(`{"id":"leg23-x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}

	// Run 1: refuses at doc-sync, NOT at the panel's stale-SHA gate — the
	// tip must remain byte-identical to A.
	if _, err := Run(root, beadID, specID, "", base, CompleteOpts{}); err == nil {
		t.Fatal("expected run 1 to refuse at doc-sync")
	} else {
		if !strings.Contains(err.Error(), "doc-sync") {
			t.Errorf("expected run 1's refusal to be attributable to doc-sync, got: %v", err)
		}
		if strings.Contains(err.Error(), "git merge bead/"+beadID) {
			t.Errorf("run 1 must NOT fail at the panel's stale-SHA gate (the raw-merge fence), got: %v", err)
		}
	}
	if got := gateRevParse(t, root, beadBranch); got != a {
		t.Errorf("expected the tip to remain EXACTLY A (%s) after the doc-sync refusal, got %s", a, got)
	}

	// A plain retry meets the IDENTICAL doc-sync gate — never a
	// manufactured staleness (the #186 mechanism this bead closes).
	if _, err := Run(root, beadID, specID, "", base, CompleteOpts{}); err == nil {
		t.Fatal("expected the plain retry to meet the identical doc-sync gate")
	} else if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("expected the retry's refusal to be attributable to doc-sync (not staleness), got: %v", err)
	}
	if got := gateRevParse(t, root, beadBranch); got != a {
		t.Errorf("expected the tip to remain EXACTLY A (%s) after the retry's refusal too, got %s", a, got)
	}

	// Run 2: --allow-doc-skew lets doc-sync's OWN gate class decide
	// (and here, pass) — never re-routed through the panel's stale-SHA
	// gate.
	res, err := Run(root, beadID, specID, "", base, CompleteOpts{AllowDocSkew: "test: ac-6 flagship override"})
	if err != nil {
		t.Fatalf("expected --allow-doc-skew to let doc-sync decide (and converge), got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be an ancestor of the spec branch after convergence")
	}
	if _, _, present := readMarker(t, root, beadID); present {
		t.Error("expected the write-ahead marker to be cleared after convergence")
	}
}
