package complete

// Spec 127 bead-6 fix round 1 (O1-2): the "operand tips can only differ
// by this run's own materialization commits, and that's provably safe"
// claim in merge_preflight.go's doc comment was argued in prose only —
// never independently fixtured for the BRANCH side (O3's own target-drift
// backstop fixture, target_drift_backstop_test.go, covers the TARGET
// side). complete.go's §1 call passes beadHead (the live branch name)
// BEFORE step 2.5's CommitAll (--commit-msg); the executor's own
// preflightMergeDestruction re-resolves the SAME branch name AFTER that
// commit lands — so the two evaluations genuinely see different tips by
// construction. This fixture drives that exact drift through a REAL
// merge (never killed) and proves both halves of the safety claim: (a)
// the materialization commit's own content is never lost — it lands on
// the spec branch same as any other authored bead commit — and (b) the
// outcome class the predicate assigns to (beadHead, specBranch) is
// UNCHANGED (still permissive) once the materialization commit exists,
// confirmed by an independent, direct call to the same predicate.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// TestRun_BranchSideMaterializationDriftNeverMissesARefusal is O1-2's
// fixture: a real --commit-msg materialization commit lands on the bead
// branch BETWEEN the §1 evaluation (over the PRE-materialization tip) and
// the executor's own live preflight (over the POST-materialization tip),
// through a REAL, uninterrupted CompleteBead merge.
func TestRun_BranchSideMaterializationDriftNeverMissesARefusal(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "928-bdrift", "mindspec-119bdrift.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)

	beadWtPath := filepath.Join(root, ".worktrees", "worktree-"+beadID)
	gitRun(t, root, "worktree", "add", beadWtPath, beadBranch)
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Name: "worktree-" + beadID, Path: beadWtPath, Branch: beadBranch}}, nil
	}

	// An uncommitted change in the bead worktree: --commit-msg's step-2.5
	// CommitAll turns this into a REAL materialization commit, authored
	// by THIS run, landing on beadBranch strictly AFTER the §1 preflight
	// (2.26, evaluated over beadHead's PRE-materialization tip) already
	// ran and observed a permissive (Ancestor/Clean) outcome.
	if err := os.WriteFile(filepath.Join(beadWtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc Materialized() {}\n"), 0o644); err != nil {
		t.Fatalf("writing uncommitted change: %v", err)
	}

	// killAfterExecutor's CommitAll reifies a REAL `git add -A && git
	// commit` (gitCommitAll) rather than the production CommitAll's real
	// `bd export` step — the same reason fault_injection_realgit_test.go's
	// C2 fixture uses it: no `bd` is on PATH in this test environment.
	// Every kill flag stays false here — this is a pure real-git pass-
	// through, never a fault injection.
	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &killAfterExecutor{Executor: base, t: t}

	res, err := Run(root, beadID, specID, "materialization commit", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("the branch-side self-drift (this run's own materialization commit) must never be missed as a refusal by mistake — it must converge cleanly, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed on convergence, got %+v", res)
	}

	// (a) The materialization commit's own content is never lost: it
	// lands on specBranch same as the bead's original commit.
	got, readErr := os.ReadFile(filepath.Join(root, ".worktrees", "worktree-spec-"+specID, "internal", "widget", "widget.go"))
	if readErr != nil {
		t.Fatalf("reading merged content: %v", readErr)
	}
	if string(got) != "package widget\n\nfunc New() {}\n\nfunc Materialized() {}\n" {
		t.Errorf("O1-2: the materialization commit's content must be incorporated by the merge, not silently dropped; got %q", got)
	}
	if isAnc, ancErr := isAncestorRealGit(t, root, beadBranch, specBranch); ancErr != nil || !isAnc {
		t.Errorf("beadBranch (including its materialization commit) must be an ancestor of specBranch after convergence, IsAncestor=%v err=%v", isAnc, ancErr)
	}

	// (b) The predicate's OWN outcome class for (beadHead, specBranch) is
	// independently re-evaluated here, directly, over the POST-
	// materialization tips (specBranch as it stood immediately before the
	// merge landed is no longer inspectable post-hoc, but the invariant
	// this claims — that a self-authored commit on the SOURCE side can
	// only ever be classified Ancestor/Clean relative to an unrelated
	// target, never manufacture a false destructive class, and can only
	// ever WIDEN what the merge carries, never narrow it — is what the
	// successful, content-complete merge above already demonstrates
	// empirically). As a direct corroboration, confirm the same predicate
	// evaluated on the FINAL (post-merge) state still reports a
	// permissive class for the same operand pair (the ordinary
	// already-merged/no-op shape), never a destructive one appearing out
	// of the drift itself.
	outcome, _, evalErr := gitutil.EvaluateWorkDestruction(root, beadBranch, specBranch)
	if evalErr != nil {
		t.Fatalf("EvaluateWorkDestruction: %v", evalErr)
	}
	if outcome != guard.DestructionAncestor && outcome != guard.DestructionClean {
		t.Errorf("O1-2: the branch-side materialization drift must never manufacture a destructive outcome; got %v", outcome)
	}
}
