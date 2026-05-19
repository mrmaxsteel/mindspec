package setup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/safeio"
)

// TestRunClaude_RefusesSymlinkedCLAUDEmd plants a symlink at <root>/CLAUDE.md
// pointing at a decoy file and asserts RunClaude refuses to write through it.
// SEC-2 regression: prior to mindspec-ldyg, the bare os.OpenFile(O_APPEND)
// would have followed the symlink and appended the managed block onto the
// decoy file's content.
func TestRunClaude_RefusesSymlinkedCLAUDEmd(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	decoyDir := t.TempDir()
	decoy := filepath.Join(decoyDir, "decoy.txt")
	const original = "untouchable\n"
	if err := os.WriteFile(decoy, []byte(original), 0o644); err != nil {
		t.Fatalf("seed decoy: %v", err)
	}

	link := filepath.Join(root, "CLAUDE.md")
	if err := os.Symlink(decoy, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := RunClaude(root, false)
	if err == nil {
		t.Fatal("RunClaude returned nil error; want ErrSymlinkRefused")
	}
	if !errors.Is(err, safeio.ErrSymlinkRefused) {
		t.Fatalf("RunClaude err = %v; want errors.Is(err, safeio.ErrSymlinkRefused)", err)
	}

	got, err := os.ReadFile(decoy)
	if err != nil {
		t.Fatalf("read decoy: %v", err)
	}
	if string(got) != original {
		t.Errorf("decoy modified through symlink: got %q, want %q", string(got), original)
	}
}

// TestRunClaude_RefusesSymlinkedSettings plants a symlink at
// <root>/.claude/settings.json pointing at a decoy and asserts ensureSettings
// refuses. .claude/settings.json drives the Claude CLI's hook execution, so
// the blast radius of a redirected write is high.
func TestRunClaude_RefusesSymlinkedSettings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	decoyDir := t.TempDir()
	decoy := filepath.Join(decoyDir, "decoy.json")
	const original = "{\"keep\":true}\n"
	if err := os.WriteFile(decoy, []byte(original), 0o644); err != nil {
		t.Fatalf("seed decoy: %v", err)
	}

	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	link := filepath.Join(claudeDir, "settings.json")
	if err := os.Symlink(decoy, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := RunClaude(root, false)
	if err == nil {
		t.Fatal("RunClaude returned nil error; want ErrSymlinkRefused")
	}
	if !errors.Is(err, safeio.ErrSymlinkRefused) {
		t.Fatalf("RunClaude err = %v; want errors.Is(err, safeio.ErrSymlinkRefused)", err)
	}

	got, err := os.ReadFile(decoy)
	if err != nil {
		t.Fatalf("read decoy: %v", err)
	}
	if string(got) != original {
		t.Errorf("decoy modified through symlink: got %q, want %q", string(got), original)
	}
}

// TestRunCopilot_RefusesSymlinkedInstructions plants a symlink at
// <root>/.github/copilot-instructions.md pointing at a decoy and asserts
// RunCopilot refuses to follow it.
func TestRunCopilot_RefusesSymlinkedInstructions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	decoyDir := t.TempDir()
	decoy := filepath.Join(decoyDir, "decoy.md")
	const original = "do-not-touch\n"
	if err := os.WriteFile(decoy, []byte(original), 0o644); err != nil {
		t.Fatalf("seed decoy: %v", err)
	}

	githubDir := filepath.Join(root, ".github")
	if err := os.MkdirAll(githubDir, 0o755); err != nil {
		t.Fatalf("mkdir .github: %v", err)
	}
	link := filepath.Join(githubDir, "copilot-instructions.md")
	if err := os.Symlink(decoy, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := RunCopilot(root, false)
	if err == nil {
		t.Fatal("RunCopilot returned nil error; want ErrSymlinkRefused")
	}
	if !errors.Is(err, safeio.ErrSymlinkRefused) {
		t.Fatalf("RunCopilot err = %v; want errors.Is(err, safeio.ErrSymlinkRefused)", err)
	}

	got, err := os.ReadFile(decoy)
	if err != nil {
		t.Fatalf("read decoy: %v", err)
	}
	if string(got) != original {
		t.Errorf("decoy modified through symlink: got %q, want %q", string(got), original)
	}
}
