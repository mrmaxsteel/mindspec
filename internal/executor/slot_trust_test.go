package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutor_SlotOpsPreserveGlobalSafeDirectory(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[safe]\n\tdirectory = *\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	bin := t.TempDir()
	// Simulate a different owner only for the main repository.
	// The slot tree has the current user's ownership and keeps full isolation.
	wrapper := "#!/bin/sh\nprev=\nfor arg do\n" +
		"  if [ \"$prev\" = -C ] && [ \"$arg\" = " + quote(root) + " ]; then\n" +
		"    export GIT_TEST_ASSUME_DIFFERENT_OWNER=1\n  fi\n  prev=$arg\ndone\n" +
		"exec " + quote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	git := func(input string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Stdin = strings.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture operation %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("", "init", "--quiet")
	git("commit refs/heads/main\ncommitter Slot Test <slot@example.invalid> 1760000000 +0000\ndata 7\ninitial\nM 100644 inline .gitignore\ndata 8\nignored\n\n\ndone\n", "fast-import", "--quiet")
	git("", "checkout", "main")
	if err := os.Mkdir(filepath.Join(root, "ignored"), 0o700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "-c", "safe.directory=", "status", "--porcelain").CombinedOutput(); err == nil || !strings.Contains(string(out), "dubious ownership") {
		t.Fatalf("ownership control did not refuse: %v %s", err, out)
	}
	executor := NewMindspecExecutor(root)
	t.Run("ignore", func(t *testing.T) {
		if err := executor.CheckSlotIgnore(root, filepath.Join(root, "ignored")); err != nil {
			t.Fatalf("CheckSlotIgnore with global safe.directory: %v", err)
		}
	})
	t.Run("clone", func(t *testing.T) {
		sha := git("", "rev-parse", "HEAD")
		tree := filepath.Join(t.TempDir(), "tree")
		if err := executor.CloneSlotTree(tree, sha); err != nil {
			t.Fatalf("CloneSlotTree with global safe.directory: %v", err)
		}
		if err := executor.CheckSlotTree(tree, sha); err != nil {
			t.Fatalf("CheckSlotTree with global safe.directory: %v", err)
		}
	})
}
