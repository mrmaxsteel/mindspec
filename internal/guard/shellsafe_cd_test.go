package guard

import (
	"errors"
	"strings"
	"testing"
)

// TestWorktreeRootRejectionRecoveryConverges_NeverBlockDegrade is AC-13's
// companion subtest: the never-block degrade path falls back to
// config.DefaultConfig() and emits ONE escaped warning naming the
// worktree_root the degrade landed on, rather than blocking or silently
// swallowing the failure. termsafe.Escape's identity-on-printable-ASCII
// behavior means a printable hostile value (a shell metacharacter) still
// appears in the warning verbatim — only a control byte/newline would be
// escaped — so this asserts the CONTROL-byte case, the one the R4
// doctrine actually targets.
func TestWorktreeRootRejectionRecoveryConverges_NeverBlockDegrade(t *testing.T) {
	cfgErr := errors.New("invalid worktree_root \".worktrees\x00evil\"")
	cfg, warning := degradeConfigOnError(cfgErr)

	if cfg.WorktreeRoot != ".worktrees" {
		t.Errorf("degraded cfg.WorktreeRoot = %q, want %q", cfg.WorktreeRoot, ".worktrees")
	}
	if !strings.HasPrefix(warning, "warning: ") {
		t.Errorf("degrade warning missing the warning: prefix: %q", warning)
	}
	if strings.ContainsRune(warning, 0) {
		t.Errorf("degrade warning contains a raw NUL byte — cfgErr.Error() was not escaped: %q", warning)
	}
}

// TestExecutableCdRendersShellSafe is AC-12 (this package's slice):
// CheckCWD's recovery `cd` line routes through the single shell-safe
// emitter — a space-bearing active-worktree path is POSIX single-quoted;
// a clean path renders byte-identical to today (no change on this repo's
// existing worktree paths, which never carry spaces).
func TestExecutableCdRendersShellSafe(t *testing.T) {
	t.Run("space-bearing active worktree is quoted", func(t *testing.T) {
		stubGuard(t)
		wt := "/repo/.worktrees/worktree spec 001 with spaces"
		readGuardStateFn = func(root string) (*guardState, error) {
			return &guardState{ActiveWorktree: wt}, nil
		}
		getwdFn = func() (string, error) { return "/repo", nil }

		err := CheckCWD("/repo")
		if err == nil {
			t.Fatal("expected a refusal when CWD is main and a worktree is active")
		}
		want := "recovery: cd '" + wt + "'"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("recovery line not shell-safe quoted; got:\n%s\nwant substring: %s", err.Error(), want)
		}
	})

	t.Run("clean active worktree renders byte-identical", func(t *testing.T) {
		stubGuard(t)
		wt := "/repo/.worktrees/worktree-bead-abc"
		readGuardStateFn = func(root string) (*guardState, error) {
			return &guardState{ActiveWorktree: wt}, nil
		}
		getwdFn = func() (string, error) { return "/repo", nil }

		err := CheckCWD("/repo")
		if err == nil {
			t.Fatal("expected a refusal when CWD is main and a worktree is active")
		}
		want := "recovery: cd " + wt
		if !strings.Contains(err.Error(), want) {
			t.Errorf("recovery line changed for a clean path; got:\n%s\nwant substring: %s", err.Error(), want)
		}
		if strings.Contains(err.Error(), "cd '"+wt) {
			t.Errorf("clean path must NOT be quoted; got:\n%s", err.Error())
		}
	})
}
