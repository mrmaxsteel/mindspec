package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_UnknownHook(t *testing.T) {
	t.Parallel()
	r := Run("nonexistent", &Input{}, nil, true)
	if r.Action != Pass {
		t.Error("unknown hook should pass")
	}
}

func TestPreCommit_AllowWhenIdle(t *testing.T) {
	t.Parallel()
	r := Run("pre-commit", &Input{}, &HookState{Mode: "idle"}, true)
	if r.Action != Pass {
		t.Errorf("expected pass for idle mode, got %v", r.Action)
	}
}

func TestPreCommit_AllowWhenNilState(t *testing.T) {
	t.Parallel()
	r := Run("pre-commit", &Input{}, nil, true)
	if r.Action != Pass {
		t.Errorf("expected pass for nil state, got %v", r.Action)
	}
}

func TestPreCommit_AllowWithEscapeHatch(t *testing.T) {
	// Can't be parallel — modifies environment
	t.Setenv("MINDSPEC_ALLOW_MAIN", "1")
	r := Run("pre-commit", &Input{}, &HookState{Mode: "implement"}, true)
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
	r := Run("pre-commit", &Input{}, st, true)
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
	r := Run("pre-commit", &Input{}, st, true)
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
	r := Run("pre-commit", &Input{}, st, true)
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
	r := Run("pre-commit", &Input{}, st, true)
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
	r := Run("pre-commit", &Input{}, st, true)
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
