package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHookDispatchHostileFieldsEscaped pins AC-16: the current branch name
// — which git permits to carry non-ASCII printable runes, including a
// Trojan-Source bidi-override (U+202E) — is escaped before it reaches
// either Block message, so a hostile branch name cannot masquerade as
// (or visually confuse with) a legitimate one. Control bytes/NUL/newline
// are NOT reachable here: git's own check-ref-format rejects them at
// branch-creation time, so the bidi-override rune is the realistic,
// git-creatable hostile fixture for this sink (round-5-style
// discriminator: still a real vector, per spec 116 Background).
func TestHookDispatchHostileFieldsEscaped(t *testing.T) {
	t.Run("protected-branch block (idle mode)", func(t *testing.T) {
		root := t.TempDir()
		mustGitInit(t, root)

		hostileBranch := "evil‮reversed"
		cmd := exec.Command("git", "checkout", "-b", hostileBranch)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git checkout -b %q: %v\n%s", hostileBranch, err, out)
		}

		mindspecDir := filepath.Join(root, ".mindspec")
		os.MkdirAll(mindspecDir, 0o755)
		os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: ["`+hostileBranch+`"]
enforcement:
  pre_commit_hook: true
`), 0o644)

		origDir, _ := os.Getwd()
		os.Chdir(root)
		defer os.Chdir(origDir)

		r := Run("pre-commit", &Input{}, staticState(&HookState{Mode: "idle"}), true)
		if r.Action != Block {
			t.Fatalf("expected block for idle mode on protected branch, got %v", r.Action)
		}
		if strings.Contains(r.Message, hostileBranch) {
			t.Errorf("block message must not contain the RAW hostile branch name:\n%s", r.Message)
		}
		if !strings.Contains(r.Message, "evil") || !strings.Contains(r.Message, "reversed") {
			t.Errorf("block message should still name the branch (escaped form):\n%s", r.Message)
		}
	})

	t.Run("spec-branch-during-implement block", func(t *testing.T) {
		root := t.TempDir()
		mustGitInit(t, root)

		hostileBranch := "spec/evil‮reversed"
		cmd := exec.Command("git", "checkout", "-b", hostileBranch)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git checkout -b %q: %v\n%s", hostileBranch, err, out)
		}

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
			t.Fatalf("expected block on spec/ branch during implement, got %v", r.Action)
		}
		if strings.Contains(r.Message, hostileBranch) {
			t.Errorf("block message must not contain the RAW hostile branch name:\n%s", r.Message)
		}
	})

	// Clean-fixture byte-identity: TestPreCommit_BlockWhenIdleOnProtectedBranch
	// and TestPreCommit_BlockOnSpecBranchDuringImplement (this package)
	// already pin that a normal branch name renders unescaped; not
	// duplicated here.
}
