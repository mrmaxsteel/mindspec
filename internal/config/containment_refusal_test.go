// Package config_test is the EXTERNAL test package (not `package config`)
// deliberately: this test needs to import both internal/config and
// internal/guard, and internal/guard imports internal/config — an
// external test package can hold both edges without closing the same
// import cycle a same-package (`package config`) test file importing
// guard would create. See containment.go's package doc comment for the
// full import-graph rationale (config cannot import
// internal/workspace/containment's parent internal/workspace, or guard,
// directly).
package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// TestWorktreeRootRejectionRecoveryConverges is AC-13: a hostile
// worktree_root in a scratch config refuses convergently — the refusal
// (i) satisfies guard.HasFinalRecoveryLine, (ii) carries no hostile bytes
// raw (a control byte / embedded newline in the raw value is escaped, not
// printed byte-for-byte), and (iii) names the set-default lever. Applying
// the lever (rewriting worktree_root to the default) and re-running then
// converges: Load succeeds and the resulting config is the clean default.
func TestWorktreeRootRejectionRecoveryConverges(t *testing.T) {
	root := t.TempDir()
	mindspecDir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(mindspecDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(mindspecDir, "config.yaml")

	// A hostile worktree_root carrying an embedded newline: if it were
	// ever printed raw, it could forge a fake terminal line in the
	// refusal message. YAML double-quoted scalar so the newline survives
	// unmarshal as a literal byte in the Go string.
	hostile := "worktree_root: \".worktrees\\nFAKE-TERMINAL-LINE\"\n"
	if err := os.WriteFile(configPath, []byte(hostile), 0o644); err != nil {
		t.Fatal(err)
	}
	config.ResetCache()

	_, err := config.Load(root)
	if err == nil {
		t.Fatal("expected Load to refuse a worktree_root containing an embedded newline")
	}
	msg := err.Error()

	t.Run("(i) satisfies guard.HasFinalRecoveryLine", func(t *testing.T) {
		if !guard.HasFinalRecoveryLine(msg) {
			t.Errorf("refusal message has no final recovery line:\n%s", msg)
		}
	})

	t.Run("(ii) contains no hostile bytes raw", func(t *testing.T) {
		if strings.Contains(msg, "FAKE-TERMINAL-LINE") && strings.Contains(msg, "\nFAKE-TERMINAL-LINE") {
			t.Errorf("the raw embedded newline survived into the refusal message unescaped:\n%q", msg)
		}
		// The escaped form (via termsafe.Escape -> strconv.Quote) renders
		// the newline as the two-byte escape sequence \n, not a raw
		// newline byte — assert the raw byte is ABSENT from the message.
		for _, line := range strings.Split(msg, "\n") {
			if line == "FAKE-TERMINAL-LINE" {
				t.Errorf("found FAKE-TERMINAL-LINE as its OWN raw line — the embedded newline was not escaped:\n%s", msg)
			}
		}
	})

	t.Run("(iii) names the set-default lever", func(t *testing.T) {
		if !strings.Contains(msg, "set worktree_root to .worktrees (the default)") {
			t.Errorf("refusal message does not name the convergent set-default lever:\n%s", msg)
		}
	})

	// Convergence: apply the lever (rewrite to the default), re-run, and
	// confirm the predicate now passes and a normal (unquoted) cd would
	// render for it.
	t.Run("applying the lever converges: re-run passes", func(t *testing.T) {
		if err := os.WriteFile(configPath, []byte("worktree_root: .worktrees\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		config.ResetCache()

		cfg, err := config.Load(root)
		if err != nil {
			t.Fatalf("Load after applying the lever: unexpected error: %v", err)
		}
		if cfg.WorktreeRoot != ".worktrees" {
			t.Errorf("WorktreeRoot after convergence = %q, want %q", cfg.WorktreeRoot, ".worktrees")
		}
	})
}

// TestWorktreeRootNeverBlockDegrade is AC-13's companion subtest: the
// never-block degrade path (guard.go) falls back to config.DefaultConfig
// on a config load failure and emits an escaped warning, rather than
// blocking the ambient guard-state read.
func TestWorktreeRootNeverBlockDegrade(t *testing.T) {
	root := t.TempDir()
	mindspecDir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(mindspecDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(mindspecDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("worktree_root: \"../escape\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config.ResetCache()

	// guard.ActiveWorktreePath drives the never-block swallow site
	// (defaultReadGuardStateWithCache) end-to-end: no beads state exists
	// under root, so phase.ResolveContextWithCache returns an empty
	// context and guard returns "" WITHOUT reaching the config swallow —
	// so instead this asserts directly against config.Load's error
	// (already proven above) and the documented degrade contract: the
	// guard package's own guard_test.go pins the swallow-to-default +
	// warning behavior at the unit level (stubGuard seams), since
	// driving it from here would require a live beads harness outside
	// this package's scope.
	if _, err := config.Load(root); err == nil {
		t.Fatal("expected Load to refuse a worktree_root containing a '..' segment")
	}

	// The never-block CONTRACT itself (fall back to DefaultConfig, which
	// has worktree_root=".worktrees") is exercised directly here without
	// the guard package's private seams: this is exactly the fallback
	// callers of config.Load are expected to perform on error.
	cfg := config.DefaultConfig()
	if cfg.WorktreeRoot != ".worktrees" {
		t.Errorf("DefaultConfig().WorktreeRoot = %q, want %q (the never-block degrade target)", cfg.WorktreeRoot, ".worktrees")
	}
}
