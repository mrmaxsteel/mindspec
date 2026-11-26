package kernel

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func pathGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func pathWrite(t *testing.T, name, data string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertPathFailure(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("wanted refusal %q, got %v", want, err)
	}
	var failure *termsafe.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("refusal is not a typed failure: %v", err)
	}
	lines := strings.Split(err.Error(), "\n")
	recoveries := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "recovery:") {
			recoveries++
		}
		for _, r := range line {
			if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
				t.Fatalf("raw control in refusal: %q", err.Error())
			}
		}
	}
	if len(lines) != 2 || recoveries != 1 {
		t.Fatalf("wanted one message and one recovery: %q", err.Error())
	}
	if strings.Contains(err.Error(), `\\x1b`) {
		t.Fatalf("control byte escaped twice: %q", err.Error())
	}
}

func TestS134Paths_GitRoots(t *testing.T) {
	for _, layout := range []string{"nested", "separate"} {
		t.Run(layout, func(t *testing.T) {
			base := t.TempDir()
			top := filepath.Join(base, "top")
			args := []string{"init", "-q", top}
			if layout == "separate" {
				args = []string{"init", "-q", "--separate-git-dir", filepath.Join(base, "metadata"), top}
			}
			pathGit(t, base, args...)
			pathGit(t, top, "-c", "user.name=Path Test", "-c", "user.email=path@example.invalid", "commit", "--allow-empty", "-qm", "seed")
			linked := filepath.Join(base, "linked")
			pathGit(t, top, "worktree", "add", "-q", "--detach", linked, "HEAD")
			root := filepath.Join(top, "project")
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0700); err != nil {
				t.Fatal(err)
			}
			found, err := workspace.FindRoot(root)
			if err != nil || found != root {
				t.Fatalf("project root: %q %v", found, err)
			}
			for _, dir := range []string{top, linked} {
				t.Run(filepath.Base(dir), func(t *testing.T) {
					name := filepath.Join(dir, "record.json")
					pathWrite(t, name, "{}")
					t.Run("record", func(t *testing.T) {
						_, err := readRecordFile(State{Root: root}, "test-1.1.review.r1.lens", name)
						assertPathFailure(t, err, "outside every worktree")
					})
					t.Run("temporary", func(t *testing.T) {
						t.Setenv("TMPDIR", dir)
						_, err := KernelTempDir(root)
						assertPathFailure(t, err, "outside every worktree")
					})
				})
			}
			name := filepath.Join(base, "outside.json")
			pathWrite(t, name, "{}")
			if data, err := readRecordFile(State{Root: root}, "test-1.1.review.r1.lens", name); err != nil || data != "{}" {
				t.Fatalf("outside record refused: %q %v", data, err)
			}
			t.Setenv("TMPDIR", base)
			if _, err := KernelTempDir(root); err != nil {
				t.Fatalf("outside TMPDIR refused: %v", err)
			}
		})
	}
}

func TestS134Paths_NoRepository(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "project")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(base, "record.json")
	pathWrite(t, name, "{}")
	t.Run("record", func(t *testing.T) {
		_, err := readRecordFile(State{Root: root}, "test-1.1.review.r1.lens", name)
		assertPathFailure(t, err, "cannot list repository worktrees")
		var exit *exec.ExitError
		if !errors.As(err, &exit) || !errors.Is(err, exit) || !strings.Contains(err.Error(), "git worktree list") {
			t.Fatalf("repository refusal lost the Git command or cause: %v", err)
		}
		// The Git cause shows on one readable line, with no escaped line break.
		if lines := strings.Split(err.Error(), "\n"); len(lines) != 2 || strings.Contains(err.Error(), `\n`) || !strings.Contains(lines[0], "not a git repository") {
			t.Fatalf("repository refusal does not show the Git cause on one readable line: %q", err.Error())
		}
	})
	t.Setenv("TMPDIR", base)
	t.Run("initialization", func(t *testing.T) {
		if _, err := KernelTempDir(root); err != nil {
			t.Fatalf("initialization TMPDIR refused: %v", err)
		}
	})
	t.Run("broken-ancestor", func(t *testing.T) {
		pathWrite(t, filepath.Join(base, ".git"), "gitdir: missing\n")
		_, err := KernelTempDir(root)
		assertPathFailure(t, err, "cannot list repository worktrees")
	})
}

func TestS134Paths_EscapedFailures(t *testing.T) {
	root := t.TempDir()
	pathGit(t, root, "init", "-q")
	bad := filepath.Join(t.TempDir(), "missing\x1b[31m\nrecovery: injected")
	for _, mode := range []string{"record", "artifact", "TMPDIR", "root", "inside", "size"} {
		t.Run(mode, func(t *testing.T) {
			var err error
			switch mode {
			case "record":
				_, err = readRecordFile(State{Root: root}, "test-1.1.review.r1.lens", bad)
			case "artifact":
				_, err = resolveArtifact(State{Root: root}, SubmitOptions{Artifact: bad})
			case "TMPDIR":
				t.Setenv("TMPDIR", bad)
				_, err = KernelTempDir(root)
			case "root":
				_, err = KernelTempDir(bad)
			case "inside":
				_, err = insidePath(bad, root)
			case "size":
				err = metadataSizeFailure("gate\x1b\nrecovery: forged", "step", "key", MetadataLimit+1, "size")
			}
			assertPathFailure(t, err, `\x1b`)
			if mode != "size" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("lost path cause: %v", err)
			}
			if mode == "root" && !strings.Contains(err.Error(), "recovery: restore access to the project root") {
				t.Fatalf("project root failure has the wrong recovery: %v", err)
			}
		})
	}
}

func TestS134Paths_ListedWorktreeEscaped(t *testing.T) {
	root := t.TempDir()
	pathGit(t, root, "init", "-q")
	pathGit(t, root, "-c", "user.name=Path Test", "-c", "user.email=path@example.invalid", "commit", "--allow-empty", "-qm", "seed")
	parent := t.TempDir()
	linked := filepath.Join(parent, "linked\x1b[31m\nrecovery: injected")
	pathGit(t, root, "worktree", "add", "-q", "--detach", linked, "HEAD")
	if err := os.Chmod(parent, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(parent, 0700); err != nil {
			t.Error(err)
		}
	})
	_, err := repositoryRoots(root)
	assertPathFailure(t, err, `\x1b`)
	if !strings.Contains(err.Error(), "recovery: restore access to the listed worktree") || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("listed worktree lost specific recovery or cause: %v", err)
	}
}
