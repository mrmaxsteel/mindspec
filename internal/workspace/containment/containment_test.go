package containment

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// TestWorktreeRootPredicate is AC-10: the worktree_root ingress predicate
// plus symlink-aware containment, including the lexical-insufficiency
// discriminator and the escaping-is-insufficient proof.
func TestWorktreeRootPredicate(t *testing.T) {
	t.Run("negative charset and traversal values reject", func(t *testing.T) {
		hostile := []string{
			"/etc/worktrees",                // absolute
			"../evil",                       // .. segment
			"a/../b",                        // .. segment mid-path
			".worktrees && echo INJECTED #", // shell metacharacters
			".worktrees;rm -rf /",           // shell metacharacters
			".worktrees\x00",                // NUL byte
			".worktrees\n.git",              // embedded newline
			"",                              // empty
			".worktrees/",                   // trailing slash -> empty segment
			"$HOME/.worktrees",              // metacharacter
			"worktrees `id`",                // backtick
		}
		for _, raw := range hostile {
			if err := ValidateWorktreeRoot(raw); err == nil {
				t.Errorf("ValidateWorktreeRoot(%q): expected rejection, got nil", raw)
			}
		}
	})

	t.Run("escaping-is-insufficient proof", func(t *testing.T) {
		// Each printable-hostile value must pass termsafe.Escape UNCHANGED
		// (it is printable ASCII, so Escape is the identity) — proving
		// that escaping alone would NOT have caught it. The charset
		// predicate is what actually rejects these, not escaping.
		printableHostile := []string{
			".worktrees && echo INJECTED #",
			".worktrees;rm -rf /",
			"$HOME/.worktrees",
			"worktrees `id`",
			"../evil",
		}
		for _, raw := range printableHostile {
			if got := termsafe.Escape(raw); got != raw {
				t.Errorf("termsafe.Escape(%q) = %q, want unchanged (identity on printable ASCII) — the escaping-is-insufficient proof requires this", raw, got)
			}
			if err := ValidateWorktreeRoot(raw); err == nil {
				t.Errorf("ValidateWorktreeRoot(%q): expected rejection despite termsafe.Escape being a no-op", raw)
			}
		}
	})

	t.Run("positive clean values pass byte-identically", func(t *testing.T) {
		clean := []string{
			".worktrees",
			".trees",
			"nested/worktrees",
			"a.b-c_d/e.f",
		}
		for _, raw := range clean {
			if err := ValidateWorktreeRoot(raw); err != nil {
				t.Errorf("ValidateWorktreeRoot(%q): unexpected rejection: %v", raw, err)
			}
		}
	})

	t.Run("symlinked ancestor rejects while lexical Rel alone would pass — the lexical-insufficiency discriminator", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink fixture requires POSIX symlink semantics")
		}
		root := t.TempDir()
		outside := t.TempDir() // a directory OUTSIDE root entirely

		// root/.worktrees is a SYMLINK pointing outside root.
		wtRootLink := filepath.Join(root, ".worktrees")
		if err := os.Symlink(outside, wtRootLink); err != nil {
			t.Fatalf("os.Symlink: %v", err)
		}

		composed := filepath.Join(root, ".worktrees", "worktree-spec-999-x")

		// The charset predicate happily accepts ".worktrees" — it's clean.
		if err := ValidateWorktreeRoot(".worktrees"); err != nil {
			t.Fatalf("ValidateWorktreeRoot(%q): unexpected rejection: %v", ".worktrees", err)
		}

		// The purely lexical check WOULD pass this composed path: it is
		// lexically "root/.worktrees/worktree-spec-999-x", which is
		// lexically under root.
		if !lexicalRelContained(root, composed) {
			t.Fatalf("lexicalRelContained(%q, %q) = false; want true (this is exactly the discriminator: lexical alone must accept it)", root, composed)
		}

		// The physical, symlink-aware check must REJECT it: .worktrees
		// resolves outside root.
		if err := CheckContainment(root, composed); err == nil {
			t.Fatalf("CheckContainment(%q, %q): expected rejection (symlinked ancestor escapes root), got nil — lexical-only would have wrongly accepted this", root, composed)
		}
	})

	t.Run("clean nested composed path passes containment", func(t *testing.T) {
		root := t.TempDir()
		composed := filepath.Join(root, ".worktrees", "worktree-spec-001-x")
		if err := CheckContainment(root, composed); err != nil {
			t.Errorf("CheckContainment(%q, %q): unexpected rejection: %v", root, composed, err)
		}
	})
}
