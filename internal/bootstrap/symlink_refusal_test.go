package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/safeio"
)

// TestRun_RefusesSymlinkedCLAUDEmd plants a symlink at <root>/CLAUDE.md
// pointing at a decoy file and asserts bootstrap.Run refuses to write through
// it. SEC-2 regression: the append-managed-block path used a bare
// os.OpenFile(O_APPEND), which followed symlinks.
func TestRun_RefusesSymlinkedCLAUDEmd(t *testing.T) {
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

	_, err := Run(root, false)
	if err == nil {
		t.Fatal("Run returned nil error; want ErrSymlinkRefused")
	}
	if !errors.Is(err, safeio.ErrSymlinkRefused) {
		t.Fatalf("Run err = %v; want errors.Is(err, safeio.ErrSymlinkRefused)", err)
	}

	got, err := os.ReadFile(decoy)
	if err != nil {
		t.Fatalf("read decoy: %v", err)
	}
	if string(got) != original {
		t.Errorf("decoy modified through symlink: got %q, want %q", string(got), original)
	}
}

// TestRun_RefusesSymlinkedFreshCLAUDEmd asserts the greenfield WriteFile path
// also refuses. A dangling symlink at <root>/CLAUDE.md routes through the
// "file doesn't exist" branch since os.Stat returns ENOENT for the target;
// the helper's Lstat pre-check catches it before the rename would replace
// the symlink with a regular file.
func TestRun_RefusesSymlinkedFreshCLAUDEmd(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	decoyDir := t.TempDir()
	// Dangling target — does not exist.
	decoy := filepath.Join(decoyDir, "nonexistent.txt")

	link := filepath.Join(root, "CLAUDE.md")
	if err := os.Symlink(decoy, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := Run(root, false)
	if err == nil {
		t.Fatal("Run returned nil error; want ErrSymlinkRefused")
	}
	if !errors.Is(err, safeio.ErrSymlinkRefused) {
		t.Fatalf("Run err = %v; want errors.Is(err, safeio.ErrSymlinkRefused)", err)
	}

	// Symlink should remain a symlink — rename never happened.
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link is no longer a symlink (mode %v)", info.Mode())
	}
}
