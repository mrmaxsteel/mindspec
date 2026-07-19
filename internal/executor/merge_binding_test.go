package executor

// Spec 121 Bead 2 (R5(b), AC-22): the merge-time landed-binding fixtures.
// Both producer legs (CompleteBead's merge leg, FinalizeEpic's auto-merge
// leg) record a durable landed-binding via mergeBindingFn IMMEDIATELY after
// a real gitutil.MergeInto succeeds and BEFORE any branch/worktree cleanup.
// A binding-write failure must SUPPRESS that bead's cleanup and refuse
// recoverably (never warn-and-continue); a re-run must locate the SAME
// merge M by identity (never rev-parse the current tip/HEAD) and converge.
// These are KILL TESTS (R9): the failure is injected AFTER a REAL merge
// has already landed in a real-git fixture, never fabricated.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

var errSimulatedBindingWrite = errors.New("simulated bd metadata write failure")

// TestCompleteBead_BindingWriteFailure_SuppressesCleanupAndConverges is
// AC-22's CompleteBead-leg kill test: after a REAL MergeInto lands, an
// injected mergeBindingFn failure must suppress the worktree/branch
// cleanup and refuse recoverably; a re-run — with the failure fixed, and
// AFTER an unrelated commit has advanced the spec branch's current tip
// PAST the original merge M — must still record M's own SHA (never the
// now-advanced current tip) and converge cleanup.
func TestCompleteBead_BindingWriteFailure_SuppressesCleanupAndConverges(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/121-bind")
	runGitIn(t, dir, "branch", "bead/mindspec-bind.1")

	beadWtDir := filepath.Join(dir, ".wt-bead-bind-1")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-bind.1")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run() })
	if err := os.WriteFile(filepath.Join(beadWtDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead work")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-bind")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-bind")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-bind.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-bind.1",
	}}
	fake.onRemove = func(name string) {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	}

	// newRepoExecutor's default stub (stubMergeBindingSeams) succeeds;
	// override it to inject the KILL — the merge itself must land for
	// real before this failure fires.
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		return errSimulatedBindingWrite
	}

	err := g.CompleteBead("mindspec-bind.1", "spec/121-bind", "")
	if err == nil {
		t.Fatal("expected the binding-write failure to refuse")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("cleanup must be SUPPRESSED on a binding-write failure, got removeCalls=%v", fake.removeCalls)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-bind.1") {
		t.Error("bead branch must survive as the corroborating datum")
	}
	if writeCalls != 1 {
		t.Errorf("expected exactly one binding-write attempt, got %d", writeCalls)
	}
	mergeSHA := refHash(t, dir, "spec/121-bind")

	// Advance the spec branch with UNRELATED work between the two runs —
	// the discriminator that proves the re-run locates M by IDENTITY,
	// never by rev-parsing the (now different) current tip.
	if err := os.WriteFile(filepath.Join(specWtPath, "unrelated.txt"), []byte("y"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "unrelated later work")
	advancedTip := refHash(t, dir, "spec/121-bind")
	if advancedTip == mergeSHA {
		t.Fatal("test setup: the spec branch must have advanced past M")
	}

	var boundMergeSHA, boundSecondParent string
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		boundMergeSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		boundSecondParent, _ = updates["mindspec_landed_second_parent"].(string)
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }

	if err := g.CompleteBead("mindspec-bind.1", "spec/121-bind", ""); err != nil {
		t.Fatalf("re-run must converge, got: %v", err)
	}
	if branchExistsIn(t, dir, "bead/mindspec-bind.1") {
		t.Error("bead branch should be deleted after the converged re-run")
	}
	if boundMergeSHA != mergeSHA {
		t.Errorf("expected the recorded merge SHA to be the ORIGINAL merge M %q, got %q (current tip is %q)", mergeSHA, boundMergeSHA, advancedTip)
	}
	if boundSecondParent == "" {
		t.Error("expected a populated second parent")
	}
}

// TestFinalizeEpic_BindingWriteFailure_SuppressesCleanupAndConverges is
// AC-22's FinalizeEpic-leg kill test: same discipline as the CompleteBead
// leg above, but through the auto-merge loop — a binding-write failure
// must abort BEFORE the push/direct-merge/cleanup legs further down ever
// run (main stays untouched, the spec branch and bead branch both
// survive), and a re-run converges.
func TestFinalizeEpic_BindingWriteFailure_SuppressesCleanupAndConverges(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-fin")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-fin")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-fin")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-fbind.1", "fbind.txt")
	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-fbind.1", Path: beadWt, Branch: "bead/mindspec-fbind.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-fbind.1": beadWt,
			"worktree-spec-121-fin":     specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	mainHashBefore := refHash(t, dir, "main")
	mergeBindingFn = func(string, map[string]interface{}) error { return errSimulatedBindingWrite }

	_, err := g.FinalizeEpic("epic-1", "121-fin", "spec/121-fin", []string{"mindspec-fbind.1"})
	if err == nil {
		t.Fatal("expected the binding-write failure to abort finalize")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-fbind.1") {
		t.Error("bead branch must survive")
	}
	if !branchExistsIn(t, dir, "spec/121-fin") {
		t.Error("spec branch must survive (aborted before push/direct-merge/cleanup)")
	}
	if refHash(t, dir, "main") != mainHashBefore {
		t.Error("main must be untouched by the aborted finalize")
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("no cleanup calls expected on the abort, got %v", fake.removeCalls)
	}
	mergeSHA := refHash(t, dir, "spec/121-fin")

	// Advance the spec branch with unrelated work between runs, proving
	// the re-run's locate-by-identity finds M, not the new current tip.
	if err := os.WriteFile(filepath.Join(specWtPath, "unrelated.txt"), []byte("y"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "unrelated later work")
	advancedTip := refHash(t, dir, "spec/121-fin")
	if advancedTip == mergeSHA {
		t.Fatal("test setup: the spec branch must have advanced past M")
	}

	var boundSHA string
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }

	result, err := g.FinalizeEpic("epic-1", "121-fin", "spec/121-fin", []string{"mindspec-fbind.1"})
	if err != nil {
		t.Fatalf("re-run must converge, got: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Fatalf("MergeStrategy = %q, want direct", result.MergeStrategy)
	}
	if boundSHA != mergeSHA {
		t.Errorf("expected the recorded merge SHA to be M %q, got %q (current tip was %q)", mergeSHA, boundSHA, advancedTip)
	}
	if branchExistsIn(t, dir, "bead/mindspec-fbind.1") {
		t.Error("bead branch should be deleted after the converged re-run")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "fbind.txt")); statErr != nil {
		t.Errorf("the bead's content must reach main after convergence: %v", statErr)
	}
}

// TestFinalizeEpic_AlreadyMergedUnboundReRunBinds is the plan's G2
// convergence-hole subtest: a bead branch that is ALREADY an ancestor of
// the spec branch (merged by some prior run whose binding write failed
// and suppressed cleanup, leaving the branch survives+unbound) must be
// BOUND on this run, not silently skipped past — the auto-merge loop's
// ordinary "already an ancestor, nothing to do" skip case must check the
// landed-binding metadata and, finding it ABSENT, run the same
// locate-by-identity + binding write before the cleanup leg.
func TestFinalizeEpic_AlreadyMergedUnboundReRunBinds(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-g2")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-g2")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-g2")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g2.1", "g2.txt")

	// Simulate the PRIOR run's outcome directly: the bead branch is
	// ALREADY merged into the spec branch (a real --no-ff merge, the
	// exact gitutil.MergeInto message) but no binding was ever recorded —
	// the branch/worktree survive because that prior binding write failed
	// and suppressed cleanup.
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-g2.1", "bead/mindspec-g2.1")
	mergeSHA := refHash(t, dir, "spec/121-g2")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g2.1", Path: beadWt, Branch: "bead/mindspec-g2.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-g2.1": beadWt,
			"worktree-spec-121-g2":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	var boundSHA string
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil } // absent

	result, err := g.FinalizeEpic("epic-1", "121-g2", "spec/121-g2", []string{"mindspec-g2.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Fatalf("MergeStrategy = %q, want direct", result.MergeStrategy)
	}
	if writeCalls != 1 {
		t.Errorf("expected the already-merged-but-unbound candidate to be BOUND (not skipped past), writeCalls=%d", writeCalls)
	}
	if boundSHA != mergeSHA {
		t.Errorf("expected the bound merge SHA to be the ORIGINAL merge %q, got %q", mergeSHA, boundSHA)
	}
	if branchExistsIn(t, dir, "bead/mindspec-g2.1") {
		t.Error("bead branch should be cleaned up once bound")
	}
}

// TestFinalizeEpic_AlreadyMergedAlreadyBoundSkipsRewrite: the mirror
// convergence case — a candidate whose binding IS already recorded must
// NOT be re-written (idempotent no-op), so the ordinary "already an
// ancestor, nothing to do" path is unaffected once bound.
func TestFinalizeEpic_AlreadyMergedAlreadyBoundSkipsRewrite(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-g2b")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-g2b")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-g2b")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g2b.1", "g2b.txt")
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-g2b.1", "bead/mindspec-g2b.1")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g2b.1", Path: beadWt, Branch: "bead/mindspec-g2b.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-g2b.1": beadWt,
			"worktree-spec-121-g2b":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_merge_sha": "already-bound-sha"}, nil
	}

	if _, err := g.FinalizeEpic("epic-1", "121-g2b", "spec/121-g2b", []string{"mindspec-g2b.1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeCalls != 0 {
		t.Errorf("an already-bound candidate must NOT be re-written, writeCalls=%d", writeCalls)
	}
}

// TestFinalizeEpic_IsAncestorFailure_AbortsBeforeBindOrCleanup is the panel
// fix-round F1-1 finding: BOTH binding legs in the auto-merge loop were
// guarded by `ancErr == nil` alone — when gitutil.IsAncestor itself ERRORS
// (a genuine infra failure, not "definitively not an ancestor"), the loop
// used to silently fall through past BOTH binding arms, and the downstream
// cleanup leg (gated only on allow-set membership, with no re-check of its
// own) would force-delete the bead branch via `git branch -D` regardless.
// A possibly-already-merged-but-UNBOUND branch destroyed that way loses
// BOTH the surviving-branch datum and the binding — landing exactly in the
// no-evidence attested-restore state R5 exists to prevent. This must abort
// recoverably instead, mirroring CompleteBead's own `ancErr != nil ||
// !isAnc` safety-check abort. The failure is REAL (a genuine
// `git merge-base --is-ancestor` error against a candidate ref that does
// not exist as a branch — a fatal git error, exit >= 2 — not a stubbed
// seam), so this is load-bearing evidence the fail-closed arm actually
// fires on infra trouble, not just a fabricated boolean.
func TestFinalizeEpic_IsAncestorFailure_AbortsBeforeBindOrCleanup(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-ancerr")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-ancerr")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-ancerr")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	// A well-formed bead/<id> branch NAME that does NOT exist as a real
	// git ref: `git merge-base --is-ancestor bead/mindspec-ancerr.1
	// spec/121-ancerr` fails with a genuine fatal error (exit >= 2, "not a
	// valid object name"), not the false/exit-1 "not an ancestor" case —
	// the real ancErr != nil this fix must catch.
	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-ancerr.1", Path: "", Branch: "bead/mindspec-ancerr.1"},
	}

	mainHashBefore := refHash(t, dir, "main")
	var writeCalls int
	mergeBindingFn = func(string, map[string]interface{}) error {
		writeCalls++
		return nil
	}

	_, err := g.FinalizeEpic("epic-1", "121-ancerr", "spec/121-ancerr", []string{"mindspec-ancerr.1"})
	if err == nil {
		t.Fatal("expected the IsAncestor infra failure to abort finalize")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
	if writeCalls != 0 {
		t.Errorf("expected NO binding-write attempt (aborted before either binding arm), got %d", writeCalls)
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("expected NO cleanup calls (aborted before the cleanup leg), got %v", fake.removeCalls)
	}
	if refHash(t, dir, "main") != mainHashBefore {
		t.Error("main must be untouched by the aborted finalize")
	}
}
