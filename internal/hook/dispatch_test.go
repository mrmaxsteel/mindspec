package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// staticState wraps a fixed HookState in the lazy provider shape Run expects.
func staticState(st *HookState) func() *HookState {
	return func() *HookState { return st }
}

func TestRun_UnknownHook(t *testing.T) {
	t.Parallel()
	r := Run("nonexistent", &Input{}, nil, true)
	if r.Action != Pass {
		t.Error("unknown hook should pass")
	}
}

func TestPreCommit_BlockWhenIdleOnProtectedBranch(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	r := Run("pre-commit", &Input{}, staticState(&HookState{Mode: "idle"}), true)
	if r.Action != Block {
		t.Errorf("expected block for idle mode on protected branch, got %v", r.Action)
	}
	if !strings.Contains(r.Message, "git checkout -b fix/") {
		t.Error("block message should suggest creating a fix branch")
	}
}

func TestPreCommit_AllowWhenIdleOnNonProtectedBranch(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)

	cmd := exec.Command("git", "checkout", "-b", "fix/something")
	cmd.Dir = root
	cmd.CombinedOutput()

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	r := Run("pre-commit", &Input{}, staticState(&HookState{Mode: "idle"}), true)
	if r.Action != Pass {
		t.Errorf("expected pass for idle mode on non-protected branch, got %v", r.Action)
	}
}

func TestPreCommit_AllowWhenNilState(t *testing.T) {
	// Even on a protected branch, nil state means mindspec is not
	// initialized — the hook must allow the commit.
	root := t.TempDir()
	mustGitInit(t, root)

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	r := Run("pre-commit", &Input{}, staticState(nil), true)
	if r.Action != Pass {
		t.Errorf("expected pass for nil state, got %v", r.Action)
	}
}

func TestPreCommit_AllowWhenNilStateFn(t *testing.T) {
	t.Parallel()
	r := Run("pre-commit", &Input{}, nil, true)
	if r.Action != Pass {
		t.Errorf("expected pass for nil state provider, got %v", r.Action)
	}
}

func TestPreCommit_StateNotResolvedOnNonProtectedBranch(t *testing.T) {
	// PERF-3: on a non-protected, non-spec branch the hook short-circuits
	// to Pass without resolving state (no beads subprocess fan-out).
	root := t.TempDir()
	mustGitInit(t, root)

	cmd := exec.Command("git", "checkout", "-b", "fix/something")
	cmd.Dir = root
	cmd.CombinedOutput()

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	resolved := false
	stateFn := func() *HookState {
		resolved = true
		return &HookState{Mode: "implement"}
	}
	r := Run("pre-commit", &Input{}, stateFn, true)
	if r.Action != Pass {
		t.Errorf("expected pass on non-protected branch, got %v", r.Action)
	}
	if resolved {
		t.Error("state should not be resolved on a non-protected, non-spec branch")
	}
}

func TestPreCommit_AllowWithEscapeHatch(t *testing.T) {
	// Can't be parallel — modifies environment
	t.Setenv("MINDSPEC_ALLOW_MAIN", "1")
	r := Run("pre-commit", &Input{}, staticState(&HookState{Mode: "implement"}), true)
	if r.Action != Pass {
		t.Errorf("expected pass with escape hatch, got %v", r.Action)
	}
}

func TestPreCommit_BlockOnProtectedBranch(t *testing.T) {
	// Create a temp git repo on "main" branch with mindspec config
	root := t.TempDir()
	mustGitInit(t, root)

	// Write .mindspec/config.yaml with pre_commit_hook: true
	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	// Change CWD to the temp repo so getCurrentBranch and workspace.FindLocalRoot work
	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	st := &HookState{
		Mode:           "implement",
		ActiveWorktree: "/some/worktree",
	}
	r := Run("pre-commit", &Input{}, staticState(st), true)
	if r.Action != Block {
		t.Errorf("expected block on protected branch, got %v", r.Action)
	}
	if r.Message == "" {
		t.Error("block message should not be empty")
	}
}

func TestPreCommit_AllowOnNonProtectedBranch(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)

	// Create and switch to a non-protected branch
	cmd := exec.Command("git", "checkout", "-b", "feature/test")
	cmd.Dir = root
	cmd.CombinedOutput()

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	st := &HookState{Mode: "implement"}
	r := Run("pre-commit", &Input{}, staticState(st), true)
	if r.Action != Pass {
		t.Errorf("expected pass on non-protected branch, got %v", r.Action)
	}
}

func TestPreCommit_AllowWhenEnforcementDisabled(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: false
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	st := &HookState{Mode: "implement"}
	r := Run("pre-commit", &Input{}, staticState(st), true)
	if r.Action != Pass {
		t.Errorf("expected pass when enforcement disabled, got %v", r.Action)
	}
}

func TestPreCommit_BlockOnSpecBranchDuringImplement(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)

	// Create and switch to a spec/ branch
	cmd := exec.Command("git", "checkout", "-b", "spec/042-greeting-feature")
	cmd.Dir = root
	cmd.CombinedOutput()

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	st := &HookState{
		Mode:           "implement",
		ActiveWorktree: "/some/bead-worktree",
	}
	r := Run("pre-commit", &Input{}, staticState(st), true)
	if r.Action != Block {
		t.Errorf("expected block on spec/ branch during implement, got %v", r.Action)
	}
	if r.Message == "" {
		t.Error("block message should not be empty")
	}
	if !strings.Contains(r.Message, "mindspec next") {
		t.Error("block message should suggest mindspec next")
	}
}

func TestPreCommit_AllowOnSpecBranchDuringSpec(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)

	cmd := exec.Command("git", "checkout", "-b", "spec/042-greeting-feature")
	cmd.Dir = root
	cmd.CombinedOutput()

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	// Spec mode — commits on spec/ branches are fine
	st := &HookState{Mode: "spec"}
	r := Run("pre-commit", &Input{}, staticState(st), true)
	if r.Action != Pass {
		t.Errorf("expected pass on spec/ branch during spec mode, got %v", r.Action)
	}
}

func mustGitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "config", "user.email", "test@test.dev")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()
	// Create initial commit so HEAD exists
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "init")
	cmd.Dir = dir
	cmd.Run()
}

// --- Spec 093 Req 1: commit-gate legitimacy context + C2-1 coverage truth ---
//
// The spec-branch Block message is a hook Block (HC-5 exception: Emit
// protocol, no `recovery:` line) — its text is asserted here instead of
// the guard convention test.

// specBranchBlock drives the pre-commit hook on a spec/ branch during
// implement mode and returns the Block message.
func specBranchBlock(t *testing.T, st *HookState) string {
	t.Helper()
	root := t.TempDir()
	mustGitInit(t, root)

	cmd := exec.Command("git", "checkout", "-b", "spec/093-skills-thin-down")
	cmd.Dir = root
	cmd.CombinedOutput()

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	r := Run("pre-commit", &Input{}, staticState(st), true)
	if r.Action != Block {
		t.Fatalf("expected block on spec/ branch during implement, got %v", r.Action)
	}
	return r.Message
}

func TestPreCommit_SpecBranchBlock_LegitimacyContext(t *testing.T) {
	msg := specBranchBlock(t, &HookState{
		Mode:           "implement",
		ActiveWorktree: "/repo/.worktrees/worktree-spec-093/.worktrees/worktree-mindspec-ab12",
	})

	// Core block text (spec 093 Req 1 message, verbatim fragments).
	for _, want := range []string{
		"mindspec: commits on spec branch 'spec/093-skills-thin-down' are blocked during implement mode.",
		"Implementation code belongs on bead branches.",
		"Run: mindspec next   (to claim a bead and create a bead worktree)",
		// G1-3: the conditional cd affordance is PRESERVED.
		"Or switch to your bead worktree: cd /repo/.worktrees/worktree-spec-093/.worktrees/worktree-mindspec-ab12",
		// Legitimacy context migrated from ms-bead-fix / ms-spec-final-review.
		"Legitimate direct spec-branch commits (final-review fix-ups: PR-body precision,",
		"stray-file reverts, CI-unblocking test fixes) may use the escape hatch:",
		"MINDSPEC_ALLOW_MAIN=1 git commit ...",
		"Do NOT use the escape hatch to land feature code outside a bead branch.",
		// C2-1 coverage truth: bead/ branches are NEVER commit-gated.
		"bead/ branches always pass",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("block message missing %q:\n%s", want, msg)
		}
	}

	// C2-1: the message must not claim the gate covers bead branches.
	if !strings.Contains(msg, "spec/ branches during implement") {
		t.Errorf("block message must state the actual gate coverage (spec/ implement-only):\n%s", msg)
	}
}

func TestPreCommit_SpecBranchBlock_NoActiveWorktree_OmitsCdLine(t *testing.T) {
	// G1-3 conditionality: with no active worktree there is no cd line —
	// the rewrite must not turn the conditional affordance into an
	// unconditional one.
	msg := specBranchBlock(t, &HookState{Mode: "implement"})
	if strings.Contains(msg, "Or switch to your bead worktree") {
		t.Errorf("cd line must be conditional on an active worktree:\n%s", msg)
	}
	// The legitimacy context is unconditional.
	if !strings.Contains(msg, "Legitimate direct spec-branch commits") {
		t.Errorf("legitimacy context must be present without an active worktree:\n%s", msg)
	}
}

func TestPreCommit_ProtectedBranchBlock_KeepsBareHint(t *testing.T) {
	// Spec 093 Req 1: the protected-branch message keeps its BARE escape
	// hatch hint — there is no legitimate routine use on main, so it
	// gains none of the spec-branch legitimacy context.
	root := t.TempDir()
	mustGitInit(t, root)

	mindspecDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(mindspecDir, 0o755)
	os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	r := Run("pre-commit", &Input{}, staticState(&HookState{Mode: "implement"}), true)
	if r.Action != Block {
		t.Fatalf("expected block on protected branch, got %v", r.Action)
	}
	if !strings.Contains(r.Message, "Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit ...") {
		t.Errorf("protected-branch message must keep its bare hint:\n%s", r.Message)
	}
	if strings.Contains(r.Message, "Legitimate direct spec-branch commits") {
		t.Errorf("protected-branch message must NOT gain the spec-branch legitimacy context:\n%s", r.Message)
	}
	if strings.Contains(r.Message, "Gate coverage:") {
		t.Errorf("protected-branch message is unchanged save the bare hint:\n%s", r.Message)
	}
}
