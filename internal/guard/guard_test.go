package guard

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func stubGuard(t *testing.T) {
	t.Helper()
	origState := readGuardStateFn
	origConfig := loadConfigFn
	origGetwd := getwdFn
	t.Cleanup(func() {
		readGuardStateFn = origState
		loadConfigFn = origConfig
		getwdFn = origGetwd
	})

	loadConfigFn = func(root string) (*config.Config, error) { return config.DefaultConfig(), nil }
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{}, nil
	}
	getwdFn = func() (string, error) { return "/repo", nil }
}

func TestCheckCWD_NoWorktreeActive(t *testing.T) {
	stubGuard(t)
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{ActiveWorktree: ""}, nil
	}

	if err := CheckCWD("/repo"); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestCheckCWD_CWDMatchesWorktree(t *testing.T) {
	stubGuard(t)
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{
			ActiveWorktree: "/repo/.worktrees/worktree-bead-abc",
		}, nil
	}
	getwdFn = func() (string, error) { return "/repo/.worktrees/worktree-bead-abc", nil }

	if err := CheckCWD("/repo"); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestCheckCWD_CWDIsMain(t *testing.T) {
	stubGuard(t)
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{
			ActiveWorktree: "/repo/.worktrees/worktree-bead-abc",
		}, nil
	}
	getwdFn = func() (string, error) { return "/repo", nil }

	err := CheckCWD("/repo")
	if err == nil {
		t.Fatal("expected error when CWD is main")
	}
	if !strings.Contains(err.Error(), "cd /repo/.worktrees/worktree-bead-abc") {
		t.Errorf("error should mention worktree path, got: %v", err)
	}
}

func TestCheckCWD_GuardsDisabled(t *testing.T) {
	stubGuard(t)
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.Enforcement.CLIGuards = false
		return cfg, nil
	}
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{
			ActiveWorktree: "/repo/.worktrees/worktree-bead-abc",
		}, nil
	}
	getwdFn = func() (string, error) { return "/repo", nil }

	if err := CheckCWD("/repo"); err != nil {
		t.Errorf("expected nil when guards disabled, got: %v", err)
	}
}

func TestCheckCWD_AllowsSpecWorktree(t *testing.T) {
	stubGuard(t)
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{
			ActiveSpec:     "051-test",
			ActiveWorktree: "/repo/.worktrees/worktree-spec-051-test/.worktrees/worktree-bead-abc",
		}, nil
	}
	// CWD is the spec worktree, not the bead worktree
	getwdFn = func() (string, error) { return "/repo/.worktrees/worktree-spec-051-test", nil }

	if err := CheckCWD("/repo"); err != nil {
		t.Errorf("expected nil error for spec worktree CWD, got: %v", err)
	}
}

func TestIsMainCWD(t *testing.T) {
	stubGuard(t)
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{
			ActiveWorktree: "/repo/.worktrees/worktree-bead-abc",
		}, nil
	}
	getwdFn = func() (string, error) { return "/repo", nil }

	if !IsMainCWD("/repo") {
		t.Error("expected IsMainCWD to return true")
	}
}

// TestGuardStateIgnoresMalformedWorktreeDirs is spec 120 AC-5's companion:
// guard's ActiveWorktree/spec-worktree composition never panics or matches
// on a malformed ActiveSpec value (the defense-in-depth degrade this
// package applies on top of D2's workspace.DetectWorktreeContext gate,
// which already prevents ActiveSpec from ever legitimately holding a
// hostile worktree-dir-derived value). A malformed ActiveSpec simply fails
// the "also allow spec worktree" CWD check rather than composing a hostile
// path.
func TestGuardStateIgnoresMalformedWorktreeDirs(t *testing.T) {
	stubGuard(t)
	readGuardStateFn = func(root string) (*guardState, error) {
		return &guardState{
			ActiveSpec:     "x;evil",
			ActiveWorktree: "/repo/.worktrees/worktree-bead-abc",
		}, nil
	}
	getwdFn = func() (string, error) { return "/repo", nil }

	err := CheckCWD("/repo")
	if err == nil {
		t.Fatal("expected error when CWD is main and worktree is active")
	}
	// The malformed ActiveSpec must not have been silently accepted as a
	// second valid CWD location — the failure still names the real
	// ActiveWorktree, never a path composed from "x;evil".
	if strings.Contains(err.Error(), "x;evil") {
		t.Errorf("guard failure embedded the malformed ActiveSpec raw: %v", err)
	}
	if !strings.Contains(err.Error(), "worktree-bead-abc") {
		t.Errorf("expected failure to still name the real active worktree, got: %v", err)
	}
}
