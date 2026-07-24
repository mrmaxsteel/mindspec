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
	"reflect"
	"strings"
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
// ancestor, nothing to do" path is unaffected once bound. Final-review
// G3-1 sharpened "already recorded" to "recorded AND MATCHING the located
// merge"; spec 125's G2-1 sharpened it FURTHER — the stored merge SHA
// AND the stored second parent must BOTH agree with the located merge —
// so this fixture stores the REAL merge SHA *and* the REAL second parent
// (any other stored value on EITHER key is a stale/latent-bad binding the
// executor must now overwrite; see
// TestFinalizeEpic_StaleBindingMismatchRewritesCorrectBinding and the
// G2-1 empty/wrong-second-parent siblings below).
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
	mergeSHA := refHash(t, dir, "spec/121-g2b")
	beadTip := refHash(t, dir, "bead/mindspec-g2b.1")

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
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     mergeSHA,
			"mindspec_landed_second_parent": beadTip,
		}, nil
	}

	if _, err := g.FinalizeEpic("epic-1", "121-g2b", "spec/121-g2b", []string{"mindspec-g2b.1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeCalls != 0 {
		t.Errorf("an already-bound (SHA AND second-parent matching) candidate must NOT be re-written, writeCalls=%d", writeCalls)
	}
}

// TestFinalizeEpic_StaleBindingMismatchRewritesCorrectBinding is the spec
// 121 final-review G3-1 fixture: a PRE-EXISTING binding whose stored
// mindspec_landed_merge_sha does NOT match the merge located by identity
// (a reopened bead's leftover, or a wrong/crafted value) must be
// OVERWRITTEN with the correct binding — never skipped as "already bound".
// The pre-fix skip returned nil on ANY non-empty stored SHA, so cleanup
// proceeded, destroyed the surviving-branch datum, and left the WRONG
// binding behind for FindLandedMerge to contradict later (stuck in
// attested-restore with no admissible datum).
func TestFinalizeEpic_StaleBindingMismatchRewritesCorrectBinding(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-g31")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-g31")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-g31")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g31.1", "g31.txt")
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-g31.1", "bead/mindspec-g31.1")
	mergeSHA := refHash(t, dir, "spec/121-g31")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g31.1", Path: beadWt, Branch: "bead/mindspec-g31.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-g31.1": beadWt,
			"worktree-spec-121-g31":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	var boundSHA, boundSecondParent string
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		boundSecondParent, _ = updates["mindspec_landed_second_parent"].(string)
		return nil
	}
	// A well-formed but WRONG stored SHA — a stale binding from some prior
	// life of this bead id, contradicting the merge located by identity.
	mergeBindingReadFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_merge_sha": "1111111111111111111111111111111111111111"}, nil
	}

	if _, err := g.FinalizeEpic("epic-1", "121-g31", "spec/121-g31", []string{"mindspec-g31.1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeCalls != 1 {
		t.Fatalf("a MISMATCHED pre-existing binding must be overwritten with the located merge's binding, writeCalls=%d", writeCalls)
	}
	if boundSHA != mergeSHA {
		t.Errorf("expected the overwrite to record the LOCATED merge %q, got %q", mergeSHA, boundSHA)
	}
	if boundSecondParent == "" {
		t.Error("expected a populated second parent in the overwrite")
	}
}

// TestFinalizeEpic_StaleBindingOverwriteFailurePreservesBranch is G3-1's
// fail-closed companion: when the correcting overwrite itself FAILS, the
// bead's branch must survive (cleanup suppressed, recoverable refusal) —
// the wrong stored binding alone must never license destroying the
// surviving-branch datum.
func TestFinalizeEpic_StaleBindingOverwriteFailurePreservesBranch(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-g31f")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-g31f")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-g31f")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g31f.1", "g31f.txt")
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-g31f.1", "bead/mindspec-g31f.1")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g31f.1", Path: beadWt, Branch: "bead/mindspec-g31f.1"},
	}

	mainHashBefore := refHash(t, dir, "main")
	mergeBindingFn = func(string, map[string]interface{}) error { return errSimulatedBindingWrite }
	mergeBindingReadFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_merge_sha": "1111111111111111111111111111111111111111"}, nil
	}

	_, err := g.FinalizeEpic("epic-1", "121-g31f", "spec/121-g31f", []string{"mindspec-g31f.1"})
	if err == nil {
		t.Fatal("expected the failed correcting overwrite to refuse")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-g31f.1") {
		t.Error("bead branch must survive a failed binding overwrite")
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("cleanup must be SUPPRESSED on a failed binding overwrite, got %v", fake.removeCalls)
	}
	if refHash(t, dir, "main") != mainHashBefore {
		t.Error("main must be untouched by the aborted finalize")
	}
}

// TestFinalizeEpic_SpecWorktreeMissing_MergedUnboundBindsBeforeCleanup is
// the spec 121 final-review G1-1 fixture: the spec WORKTREE is gone (a
// prior partial run removed it) but the bead's branch is already merged
// into the spec branch with NO binding recorded. The pre-fix shape gated
// the entire ancestry-check/binding/abort block on os.Stat(specWtPath)
// while the cleanup leg deleted allow-set branches unconditionally — so
// this exact state destroyed the merged-but-unbound branch with neither
// the surviving-branch datum nor a binding, the no-evidence
// attested-restore state R5 exists to prevent. Post-fix: the binding legs
// run on g.Root regardless of the worktree, so the branch is BOUND before
// any cleanup may touch it.
func TestFinalizeEpic_SpecWorktreeMissing_MergedUnboundBindsBeforeCleanup(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-g11")

	// Merge the bead into the spec branch through a TEMPORARY worktree,
	// then remove it — reproducing "a prior run merged, then the worktree
	// was lost/removed before the binding was ever recorded".
	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-121-g11")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/121-g11")
	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g11.1", "g11.txt")
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-g11.1", "bead/mindspec-g11.1")
	mergeSHA := refHash(t, dir, "spec/121-g11")
	runGitIn(t, dir, "worktree", "remove", "--force", specWtPath)
	if _, statErr := os.Stat(specWtPath); statErr == nil {
		t.Fatal("test setup: the spec worktree must be gone")
	}

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g11.1", Path: beadWt, Branch: "bead/mindspec-g11.1"},
	}
	fake.onRemove = func(name string) {
		if name == "worktree-mindspec-g11.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWt).Run()
		}
	}

	var boundSHA string
	var branchAliveAtBindTime bool
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		// The load-bearing ordering: the surviving-branch datum must still
		// exist at the moment the binding is written — bind BEFORE destroy.
		branchAliveAtBindTime = branchExistsIn(t, dir, "bead/mindspec-g11.1")
		return nil
	}

	result, err := g.FinalizeEpic("epic-1", "121-g11", "spec/121-g11", []string{"mindspec-g11.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Fatalf("MergeStrategy = %q, want direct", result.MergeStrategy)
	}
	if writeCalls != 1 {
		t.Fatalf("the merged-but-unbound bead must be BOUND despite the missing spec worktree, writeCalls=%d", writeCalls)
	}
	if boundSHA != mergeSHA {
		t.Errorf("expected the bound merge SHA to be the original merge %q, got %q", mergeSHA, boundSHA)
	}
	if !branchAliveAtBindTime {
		t.Error("the bead branch must still exist when the binding is written (bind before destroy)")
	}
}

// TestFinalizeEpic_SpecWorktreeMissing_UnmergedRefusesNotDestroyed is
// G1-1's other leg: a GENUINELY UNMERGED bead + a missing spec worktree
// cannot merge (MergeInto needs the worktree) — finalize must refuse
// recoverably, and above all the cleanup leg must never run: the pre-fix
// fall-through skipped merge AND binding, then cleanup force-deleted the
// unmerged branch's only copy of its work.
func TestFinalizeEpic_SpecWorktreeMissing_UnmergedRefusesNotDestroyed(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/121-g11u")
	// No spec worktree is ever created for spec/121-g11u.

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g11u.1", "g11u.txt")
	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g11u.1", Path: beadWt, Branch: "bead/mindspec-g11u.1"},
	}

	mainHashBefore := refHash(t, dir, "main")
	var writeCalls int
	mergeBindingFn = func(string, map[string]interface{}) error {
		writeCalls++
		return nil
	}

	_, err := g.FinalizeEpic("epic-1", "121-g11u", "spec/121-g11u", []string{"mindspec-g11u.1"})
	if err == nil {
		t.Fatal("expected the unmerged-bead-with-missing-worktree state to refuse")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-g11u.1") {
		t.Error("the unmerged bead branch must survive the refusal")
	}
	if writeCalls != 0 {
		t.Errorf("nothing was merged, so nothing may be bound, writeCalls=%d", writeCalls)
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("cleanup must never run on the refusal, got %v", fake.removeCalls)
	}
	if refHash(t, dir, "main") != mainHashBefore {
		t.Error("main must be untouched by the refusal")
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

// --- Spec 125 Bead 1 (R1/R2/R5 executor half) ---
//
// R1: `complete`/`FinalizeEpic` reliably persist the landed-binding
// EVEN THROUGH the pinned conflict-recovery/default-subject miss shape
// (AC-1, AC-1c). R2: a genuine locate miss for a bead that DID merge is
// LOUD (fail-closed, cleanup-suppressing), never silent; the legitimate
// "nothing to bind" state is classified by FIRST-PARENT MEMBERSHIP, not
// an own-commit-count/merge-base metric proven insufficient to
// distinguish a merged-then-ancestor bead from a true orphan (AC-3, AC-4,
// AC-4b). The idempotent-skip is tightened to require BOTH the stored
// merge SHA and second parent to agree with the located merge (G2-1).

// commitSubject returns the subject line of sha in dir.
func commitSubject(t *testing.T, dir, sha string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "log", "--format=%s", "-1", sha).Output()
	if err != nil {
		t.Fatalf("git log --format=%%s -1 %s: %v", sha, err)
	}
	return strings.TrimSpace(string(out))
}

// TestCompleteBead_ConflictRecoveryMissStillBinds is AC-1: the real
// conflict-recovery shape (2nd bead under a spec, a genuine add/add
// conflict on a tracked file standing in for the auto-committed
// `.beads/issues.jsonl`, resolved by MANUALLY re-running the merge
// WITHOUT `-m` — reproducing an operator following the pre-fix recovery
// line verbatim, which lands git's DEFAULT subject, never the exact
// "Merge <beadBranch>" MergeInto itself would have produced), then a
// recovery re-run of production CompleteBead. Before spec 125,
// `locateLandedMergeByIdentity`'s exact-subject scan MISSES this shape
// and the silent-nil swallow completes with no binding (RED today); after
// R1/R2/R5, identity is corroborated by second parent alone, so the
// binding is persisted regardless of the merge's subject format, and the
// branch/worktree are cleaned up.
func TestCompleteBead_ConflictRecoveryMissStillBinds(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/125-ac1")
	runGitIn(t, dir, "branch", "bead/mindspec-ac1.2")

	// Spec worktree independently adds the tracked file (simulating a
	// prior bead's already-merged `.beads/issues.jsonl` export).
	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-ac1")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-ac1")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })
	if err := os.MkdirAll(filepath.Join(specWtPath, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specWtPath, ".beads", "issues.jsonl"), []byte("spec-side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "chore: commit remaining spec artifacts")

	// The 2nd bead's own worktree independently adds the SAME path — an
	// add/add conflict on merge — plus its own deliverable commit.
	beadWtDir := filepath.Join(dir, ".wt-bead-ac1-2")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-ac1.2")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run() })
	if err := os.MkdirAll(filepath.Join(beadWtDir, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadWtDir, ".beads", "issues.jsonl"), []byte("bead-side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadWtDir, "ac1.txt"), []byte("bead work\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead work")

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-ac1.2",
		Path:   beadWtDir,
		Branch: "bead/mindspec-ac1.2",
	}}
	fake.onRemove = func(name string) {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	}

	// First run: CompleteBead's own MergeInto hits the real conflict.
	err := g.CompleteBead("mindspec-ac1.2", "spec/125-ac1", "")
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	if !branchExistsIn(t, dir, "bead/mindspec-ac1.2") {
		t.Fatal("test setup: the conflicted bead branch must survive")
	}

	// Recovery: an operator following the PRE-FIX recovery line verbatim
	// (no `-m`) resolves the conflict and commits — landing git's DEFAULT
	// subject, never the exact "Merge bead/mindspec-ac1.2".
	_ = exec.Command("git", "-C", specWtPath, "merge", "--no-ff", "bead/mindspec-ac1.2").Run()
	if err := os.WriteFile(filepath.Join(specWtPath, ".beads", "issues.jsonl"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "--no-edit")

	mergeSHA := refHash(t, dir, "spec/125-ac1")
	beadTip := refHash(t, dir, "bead/mindspec-ac1.2")
	if subj := commitSubject(t, dir, mergeSHA); subj == "Merge bead/mindspec-ac1.2" {
		t.Fatal("test setup: the recovery merge's subject must NOT be the exact form (this is the default-subject miss shape)")
	}

	var boundSHA, boundSecondParent string
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		boundSecondParent, _ = updates["mindspec_landed_second_parent"].(string)
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }

	// Recovery re-run of production CompleteBead: the bead branch is now
	// already an ancestor (no-op MergeInto), and the binding must still
	// be recorded, corroborated by second parent — not subject text.
	if err := g.CompleteBead("mindspec-ac1.2", "spec/125-ac1", ""); err != nil {
		t.Fatalf("the recovery re-run must succeed, got: %v", err)
	}
	if writeCalls != 1 {
		t.Fatalf("expected exactly one binding write, got %d", writeCalls)
	}
	if boundSHA != mergeSHA {
		t.Errorf("bound merge SHA = %q, want the actual merge commit %q", boundSHA, mergeSHA)
	}
	if boundSecondParent != beadTip {
		t.Errorf("bound second parent = %q, want the bead's tip %q", boundSecondParent, beadTip)
	}
	if branchExistsIn(t, dir, "bead/mindspec-ac1.2") {
		t.Error("bead branch should be deleted after the converged recovery re-run")
	}
	if _, statErr := os.Stat(beadWtDir); statErr == nil {
		t.Error("bead worktree should be removed after the converged recovery re-run")
	}
}

// TestFinalizeEpic_AlreadyAncestorDefaultSubjectBinds is AC-1c's F1-3
// already-ancestor sub-case: a bead branch merged into the spec branch by
// some PRIOR process (e.g. an operator following the pre-fix
// conflict-recovery line verbatim) whose merge commit carries git's
// DEFAULT subject — never the exact `Merge bead/<id>` MergeInto itself
// would produce. R1 persists REGARDLESS of the merge's subject format:
// the already-ancestor leg must still record the binding, corroborated
// by second-parent identity.
func TestFinalizeEpic_AlreadyAncestorDefaultSubjectBinds(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/125-ac1c")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-ac1c")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-ac1c")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-ac1c.1", "ac1c.txt")

	// Simulate a PRIOR merge (out-of-band, carrying git's default
	// recovery subject) landing the bead — built non-interactively via
	// --no-commit + an explicit -m matching git's own default template,
	// so the fixture needs no interactive editor.
	runGitIn(t, specWtPath, "merge", "--no-ff", "--no-commit", "bead/mindspec-ac1c.1")
	runGitIn(t, specWtPath, "commit", "-m", "Merge branch 'bead/mindspec-ac1c.1' into spec/125-ac1c")
	mergeSHA := refHash(t, dir, "spec/125-ac1c")
	beadTip := refHash(t, dir, "bead/mindspec-ac1c.1")
	if subj := commitSubject(t, dir, mergeSHA); subj == "Merge bead/mindspec-ac1c.1" {
		t.Fatal("test setup: merge subject must NOT be the exact form")
	}

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-ac1c.1", Path: beadWt, Branch: "bead/mindspec-ac1c.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-ac1c.1": beadWt,
			"worktree-spec-125-ac1c":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	var boundSHA, boundSecondParent string
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		boundSecondParent, _ = updates["mindspec_landed_second_parent"].(string)
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }

	result, err := g.FinalizeEpic("epic-1", "125-ac1c", "spec/125-ac1c", []string{"mindspec-ac1c.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Fatalf("MergeStrategy = %q, want direct", result.MergeStrategy)
	}
	if writeCalls != 1 {
		t.Fatalf("expected the default-subject already-ancestor bead to be BOUND, writeCalls=%d", writeCalls)
	}
	if boundSHA != mergeSHA {
		t.Errorf("bound merge SHA = %q, want %q", boundSHA, mergeSHA)
	}
	if boundSecondParent != beadTip {
		t.Errorf("bound second parent = %q, want the bead's tip %q", boundSecondParent, beadTip)
	}
}

// TestFinalizeEpic_AutoMergeLegPersistsBindingOnSuccess is AC-1c's other
// producer leg: a genuinely UNMERGED bead going through FinalizeEpic's
// auto-merge leg (mindspec_executor.go's `default:` case) on a PLAIN
// success path (no injected failure) must ALSO persist the binding,
// matching the real merge and its second parent, before cleanup — R1
// covers both CompleteBead and FinalizeEpic, not just the
// failure-recovery kill-test shape.
func TestFinalizeEpic_AutoMergeLegPersistsBindingOnSuccess(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/125-ac1c-auto")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-ac1c-auto")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-ac1c-auto")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-ac1c-auto.1", "auto.txt")
	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-ac1c-auto.1", Path: beadWt, Branch: "bead/mindspec-ac1c-auto.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-ac1c-auto.1": beadWt,
			"worktree-spec-125-ac1c-auto":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	beadTip := refHash(t, dir, "bead/mindspec-ac1c-auto.1")

	var boundSHA, boundSecondParent string
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		boundSecondParent, _ = updates["mindspec_landed_second_parent"].(string)
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }

	result, err := g.FinalizeEpic("epic-1", "125-ac1c-auto", "spec/125-ac1c-auto", []string{"mindspec-ac1c-auto.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Fatalf("MergeStrategy = %q, want direct", result.MergeStrategy)
	}
	if writeCalls != 1 {
		t.Fatalf("expected the auto-merge leg to persist the binding on a plain success path, writeCalls=%d", writeCalls)
	}
	if boundSecondParent != beadTip {
		t.Errorf("bound second parent = %q, want the bead's tip %q", boundSecondParent, beadTip)
	}
	if boundSHA == "" {
		t.Error("expected a populated merge SHA")
	}
}

// TestCompleteBead_ForcedLocateMissRefusesLoudAndConverges is AC-3: after
// a REAL merge with own commits, a locate MISS (forced via the
// locateLandedMergeFn seam — the mechanism this bead adds, since a real
// miss is otherwise reproduced by the conflict-recovery shape above) must
// refuse LOUD via a guard.NewFailure naming the bead/branch/recovery,
// SUPPRESS cleanup (branch and worktree survive as the corroborating
// datum), and converge on a subsequent successful run — never the old
// silent-nil swallow that let cleanup proceed and destroyed the datum.
func TestCompleteBead_ForcedLocateMissRefusesLoudAndConverges(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/125-ac3")
	runGitIn(t, dir, "branch", "bead/mindspec-ac3.1")

	beadWtDir := filepath.Join(dir, ".wt-bead-ac3-1")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-ac3.1")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run() })
	if err := os.WriteFile(filepath.Join(beadWtDir, "ac3.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead work")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-ac3")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-ac3")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	fake.listEntries = []bead.WorktreeListEntry{{
		Name: "worktree-mindspec-ac3.1", Path: beadWtDir, Branch: "bead/mindspec-ac3.1",
	}}
	fake.onRemove = func(name string) {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	}

	origLocate := locateLandedMergeFn
	t.Cleanup(func() { locateLandedMergeFn = origLocate })
	locateLandedMergeFn = func(string, string, string) (string, string, error) {
		return "", "", errNoLandedMergeIdentified
	}

	var writeCalls int
	mergeBindingFn = func(string, map[string]interface{}) error {
		writeCalls++
		return nil
	}

	err := g.CompleteBead("mindspec-ac3.1", "spec/125-ac3", "")
	if err == nil {
		t.Fatal("expected the forced locate miss on a MERGED bead to refuse LOUD")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
	if writeCalls != 0 {
		t.Errorf("no binding may be written on a locate miss, writeCalls=%d", writeCalls)
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("cleanup must be SUPPRESSED on the loud refusal, got %v", fake.removeCalls)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-ac3.1") {
		t.Error("bead branch must survive the loud refusal as the corroborating datum")
	}

	// Restore the real locate: a subsequent run must converge.
	locateLandedMergeFn = origLocate
	var boundSHA string
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		return nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }

	if err := g.CompleteBead("mindspec-ac3.1", "spec/125-ac3", ""); err != nil {
		t.Fatalf("the re-run must converge once the locate is no longer forced to miss, got: %v", err)
	}
	if boundSHA == "" {
		t.Error("expected the converged re-run to record a binding")
	}
	if branchExistsIn(t, dir, "bead/mindspec-ac3.1") {
		t.Error("bead branch should be deleted after the converged re-run")
	}
}

// TestCompleteBead_TrueOrphanQuietNoBinding is AC-4: a true bd_close
// orphan — a trivially-ancestor bead branch with ZERO own commits since
// its fork point, so `git merge --no-ff` performs no merge and creates no
// commit — completes QUIETLY: no binding is written, no refusal, no
// landed-binding warning noise. The bead's tip is the second parent of NO
// first-parent merge on spec, so the first-parent-membership
// discriminator correctly takes the quiet path.
func TestCompleteBead_TrueOrphanQuietNoBinding(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/125-ac4")
	runGitIn(t, dir, "branch", "bead/mindspec-ac4.1")

	beadWtDir := filepath.Join(dir, ".wt-bead-ac4-1")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-ac4.1")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run() })

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-ac4")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-ac4")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	fake.listEntries = []bead.WorktreeListEntry{{
		Name: "worktree-mindspec-ac4.1", Path: beadWtDir, Branch: "bead/mindspec-ac4.1",
	}}
	fake.onRemove = func(name string) {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	}

	var writeCalls int
	mergeBindingFn = func(string, map[string]interface{}) error {
		writeCalls++
		return nil
	}

	stderr := captureStderrAround(t, func() {
		if err := g.CompleteBead("mindspec-ac4.1", "spec/125-ac4", ""); err != nil {
			t.Fatalf("a true orphan bead must complete quietly, got: %v", err)
		}
	})
	if writeCalls != 0 {
		t.Errorf("a true orphan must record NO binding, writeCalls=%d", writeCalls)
	}
	if strings.Contains(stderr, "landed") || strings.Contains(stderr, "binding") {
		t.Errorf("expected no landed-binding warning noise, got:\n%s", stderr)
	}
	if branchExistsIn(t, dir, "bead/mindspec-ac4.1") {
		t.Error("orphan branch should be deleted after quiet completion")
	}
}

// TestCompleteBead_MergedThenAncestorForcedMissIsLoud is AC-4b: after a
// REAL merge with own commits, the bead branch is — like a true orphan —
// BOTH already an ancestor of the spec branch (`rev-list beadTip
// ^specBranch` == 0) AND has `merge-base(beadTip, specBranch) ==
// beadTip`: the exact two metrics the Background/R2 falsifier proves
// INSUFFICIENT to distinguish "a merged-then-ancestor bead" from "a true
// bd_close orphan that never diverged" (ensureLandedBinding runs AFTER
// the merge, so both read identically under these metrics). With the
// locate forced to miss, the structural first-parent-membership
// discriminator must still refuse LOUD (the bead's tip IS the second
// parent of a first-parent merge on spec) — a count/merge-base classifier
// would read BOTH metrics as "nothing to bind" and silently pass,
// RE-HIDING exactly the bug this spec fixes.
func TestCompleteBead_MergedThenAncestorForcedMissIsLoud(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/125-ac4b")
	runGitIn(t, dir, "branch", "bead/mindspec-ac4b.1")

	beadWtDir := filepath.Join(dir, ".wt-bead-ac4b-1")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-ac4b.1")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run() })
	if err := os.WriteFile(filepath.Join(beadWtDir, "ac4b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead work")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-ac4b")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-ac4b")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	// A REAL merge, production shape — own commits present.
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-ac4b.1", "bead/mindspec-ac4b.1")
	mergeSHA := refHash(t, dir, "spec/125-ac4b")
	beadTip := refHash(t, dir, "bead/mindspec-ac4b.1")

	// The proven-insufficient metrics: BOTH read byte-identical to a true
	// orphan after the merge.
	if !isAncestorIn(t, dir, "bead/mindspec-ac4b.1", "spec/125-ac4b") {
		t.Fatal("test setup: the bead branch must be an ancestor of spec after the merge (rev-list beadTip ^spec == 0)")
	}
	mergeBase, mbErr := g.MergeBase("bead/mindspec-ac4b.1", "spec/125-ac4b")
	if mbErr != nil {
		t.Fatalf("MergeBase: %v", mbErr)
	}
	if mergeBase != beadTip {
		t.Fatalf("test setup: merge-base must equal the bead's own tip (byte-identical-to-orphan precondition), got %q want %q", mergeBase, beadTip)
	}

	fake.listEntries = []bead.WorktreeListEntry{{
		Name: "worktree-mindspec-ac4b.1", Path: beadWtDir, Branch: "bead/mindspec-ac4b.1",
	}}
	fake.onRemove = func(name string) {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	}

	origLocate := locateLandedMergeFn
	t.Cleanup(func() { locateLandedMergeFn = origLocate })
	locateLandedMergeFn = func(string, string, string) (string, string, error) {
		return "", "", errNoLandedMergeIdentified
	}

	var writeCalls int
	mergeBindingFn = func(string, map[string]interface{}) error {
		writeCalls++
		return nil
	}

	err := g.CompleteBead("mindspec-ac4b.1", "spec/125-ac4b", "")
	if err == nil {
		t.Fatal("expected the forced miss on a MERGED-then-ancestor bead to refuse LOUD — a count/merge-base classifier would silently pass this fixture (both metrics are byte-identical to a true orphan)")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
	if writeCalls != 0 {
		t.Errorf("no binding may be written on a locate miss, writeCalls=%d", writeCalls)
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("cleanup must be SUPPRESSED on the loud refusal, got %v", fake.removeCalls)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-ac4b.1") {
		t.Error("bead branch must survive the loud refusal")
	}
	if refHash(t, dir, "spec/125-ac4b") != mergeSHA {
		t.Error("spec branch must be unchanged by the refusal")
	}
}

// TestFinalizeEpic_ExistingBindingEmptySecondParentRewrites is the G2-1
// idempotent-skip tightening (CRITICAL FIX): a pre-existing binding whose
// stored `mindspec_landed_merge_sha` ALREADY matches the located merge
// (the pre-125 skip's ONLY check) but whose stored
// `mindspec_landed_second_parent` is EMPTY must still be RE-WRITTEN — the
// old skip would leave this latent bad binding untouched forever.
func TestFinalizeEpic_ExistingBindingEmptySecondParentRewrites(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/125-g21e")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-g21e")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-g21e")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g21e.1", "g21e.txt")
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-g21e.1", "bead/mindspec-g21e.1")
	mergeSHA := refHash(t, dir, "spec/125-g21e")
	beadTip := refHash(t, dir, "bead/mindspec-g21e.1")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g21e.1", Path: beadWt, Branch: "bead/mindspec-g21e.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-g21e.1": beadWt,
			"worktree-spec-125-g21e":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	var boundSHA, boundSecondParent string
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		boundSecondParent, _ = updates["mindspec_landed_second_parent"].(string)
		return nil
	}
	// Merge SHA is CORRECT (the old skip's only check) but second parent
	// is EMPTY — a latent bad binding.
	mergeBindingReadFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     mergeSHA,
			"mindspec_landed_second_parent": "",
		}, nil
	}

	if _, err := g.FinalizeEpic("epic-1", "125-g21e", "spec/125-g21e", []string{"mindspec-g21e.1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeCalls != 1 {
		t.Fatalf("a binding with a correct SHA but an EMPTY second parent must be RE-WRITTEN, writeCalls=%d", writeCalls)
	}
	if boundSHA != mergeSHA {
		t.Errorf("expected the rewrite to keep the correct merge SHA %q, got %q", mergeSHA, boundSHA)
	}
	if boundSecondParent != beadTip {
		t.Errorf("expected the rewrite to populate the second parent %q, got %q", beadTip, boundSecondParent)
	}
}

// TestFinalizeEpic_ExistingBindingWrongSecondParentRewrites is G2-1's
// sibling: a correct stored merge SHA but a WRONG stored second parent
// (not merely empty) must ALSO be re-written, never skipped.
func TestFinalizeEpic_ExistingBindingWrongSecondParentRewrites(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/125-g21w")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-125-g21w")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/125-g21w")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	beadWt := plantBeadWorktree(t, dir, "bead/mindspec-g21w.1", "g21w.txt")
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-g21w.1", "bead/mindspec-g21w.1")
	mergeSHA := refHash(t, dir, "spec/125-g21w")
	beadTip := refHash(t, dir, "bead/mindspec-g21w.1")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-g21w.1", Path: beadWt, Branch: "bead/mindspec-g21w.1"},
	}
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-g21w.1": beadWt,
			"worktree-spec-125-g21w":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	var boundSHA, boundSecondParent string
	var writeCalls int
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		writeCalls++
		boundSHA, _ = updates["mindspec_landed_merge_sha"].(string)
		boundSecondParent, _ = updates["mindspec_landed_second_parent"].(string)
		return nil
	}
	// Merge SHA correct, second parent WRONG (a crafted/stale value).
	mergeBindingReadFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     mergeSHA,
			"mindspec_landed_second_parent": "3333333333333333333333333333333333333333",
		}, nil
	}

	if _, err := g.FinalizeEpic("epic-1", "125-g21w", "spec/125-g21w", []string{"mindspec-g21w.1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeCalls != 1 {
		t.Fatalf("a binding with a correct SHA but a WRONG second parent must be RE-WRITTEN, writeCalls=%d", writeCalls)
	}
	if boundSHA != mergeSHA {
		t.Errorf("expected the rewrite to keep the correct merge SHA %q, got %q", mergeSHA, boundSHA)
	}
	if boundSecondParent != beadTip {
		t.Errorf("expected the rewrite to correct the second parent to %q, got %q", beadTip, boundSecondParent)
	}
}

// --- AC-11(i) anti-drift: executor-half seam pointer pins ---

// TestMergeBindingFn_IsBeadMergeMetadata pins the production default of
// the merge-time binding WRITE seam to the real bead.MergeMetadata
// symbol — never a private reimplementation — so the hermetic tests
// provably exercise the real write path.
func TestMergeBindingFn_IsBeadMergeMetadata(t *testing.T) {
	if reflect.ValueOf(mergeBindingFn).Pointer() != reflect.ValueOf(bead.MergeMetadata).Pointer() {
		t.Fatal("mergeBindingFn must be bead.MergeMetadata (AC-11(i) anti-drift)")
	}
}

// TestMergeBindingReadFn_IsBeadGetMetadata pins the production default of
// the merge-time binding READ seam to the real bead.GetMetadata symbol.
func TestMergeBindingReadFn_IsBeadGetMetadata(t *testing.T) {
	if reflect.ValueOf(mergeBindingReadFn).Pointer() != reflect.ValueOf(bead.GetMetadata).Pointer() {
		t.Fatal("mergeBindingReadFn must be bead.GetMetadata (AC-11(i) anti-drift)")
	}
}

// TestLocateLandedMergeFn_IsLocateLandedMergeByIdentity pins the
// production default of the new R2 locate seam to the real
// locateLandedMergeByIdentity function — so a test-forced miss
// (AC-3/AC-4b) is the ONLY way to divert ensureLandedBinding from the
// real ground-truth locate, never an accidental drift.
func TestLocateLandedMergeFn_IsLocateLandedMergeByIdentity(t *testing.T) {
	if reflect.ValueOf(locateLandedMergeFn).Pointer() != reflect.ValueOf(locateLandedMergeByIdentity).Pointer() {
		t.Fatal("locateLandedMergeFn must be locateLandedMergeByIdentity (AC-11(i) anti-drift)")
	}
}
