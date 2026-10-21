package executor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type slotTrustFixture struct {
	root, global, cloneArgs, slotCalls, sha string
}

func newSlotTrustFixture(t *testing.T) slotTrustFixture {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	// Preserve the empty reset and the path as separate, ordered values.
	if err := os.WriteFile(global, []byte("[safe]\n\tdirectory =\n\tdirectory = "+strconv.Quote(root)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	bin := t.TempDir()
	cloneArgs := filepath.Join(t.TempDir(), "clone-args")
	slotCalls := filepath.Join(t.TempDir(), "slot-calls")
	// Simulate a different owner only for the main repository.
	// Record clone options because shared clones can bypass the owner check.
	wrapper := "#!/bin/sh\nprev=\nfor arg do\n" +
		"  if [ \"$prev\" = -C ]; then printf '%s\\t%s\\n' \"$arg\" \"${GIT_CONFIG_GLOBAL-unset}\" >> " + quote(slotCalls) + "; fi\n" +
		"  if [ \"$prev\" = -C ] && [ \"$arg\" = " + quote(root) + " ]; then\n" +
		"    export GIT_TEST_ASSUME_DIFFERENT_OWNER=1\n  fi\n" +
		"  if [ \"$arg\" = clone ]; then printf '%s\\n' \"$@\" > " + quote(cloneArgs) + "; fi\n" +
		"  prev=$arg\ndone\nexec " + quote(realGit) + " \"$@\"\n"
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
	// This Git must accept command options as protected configuration.
	cmd := exec.Command("git", "-C", root, "-c", "safe.directory=", "-c", "safe.directory="+root, "status", "--porcelain")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command trust control did not pass: %v %s", err, out)
	}
	return slotTrustFixture{root, global, cloneArgs, slotCalls, git("", "rev-parse", "HEAD")}
}

func (f slotTrustFixture) addGlobal(t *testing.T, config string) {
	t.Helper()
	file, err := os.OpenFile(f.global, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(config); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExecutor_SlotOpsPreserveGlobalSafeDirectory(t *testing.T) {
	f := newSlotTrustFixture(t)
	executor := NewMindspecExecutor(f.root)
	t.Run("ignore", func(t *testing.T) {
		if err := executor.CheckSlotIgnore(f.root, filepath.Join(f.root, "ignored")); err != nil {
			t.Fatalf("CheckSlotIgnore with global safe.directory: %v", err)
		}
	})
	t.Run("alternates", func(t *testing.T) {
		got, err := executor.slotAlternates()
		want := filepath.Join(f.root, ".git", "objects") + "\n"
		if err != nil || string(got) != want {
			t.Fatalf("slotAlternates with global safe.directory: %q, %v", got, err)
		}
	})
	t.Run("clone", func(t *testing.T) {
		tree := filepath.Join(t.TempDir(), "tree")
		if err := executor.CloneSlotTree(tree, f.sha); err != nil {
			t.Fatalf("CloneSlotTree with global safe.directory: %v", err)
		}
		if err := executor.CheckSlotTree(tree, f.sha); err != nil {
			t.Fatalf("CheckSlotTree with global safe.directory: %v", err)
		}
		raw, err := os.ReadFile(f.cloneArgs)
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
		var trust []string
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-c" && strings.HasPrefix(args[i+1], "safe.directory=") {
				trust = append(trust, args[i+1])
			}
		}
		want := []string{"safe.directory=", "safe.directory=" + f.root}
		if !reflect.DeepEqual(trust, want) {
			t.Fatalf("clone trust options: got %q, want %q", trust, want)
		}
	})
}

func TestExecutor_SlotIgnoreRejectsGlobalExcludes(t *testing.T) {
	f := newSlotTrustFixture(t)
	excludes := filepath.Join(t.TempDir(), "excludes")
	if err := os.WriteFile(excludes, []byte("global-only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.addGlobal(t, "[core]\n\texcludesFile = "+strconv.Quote(excludes)+"\n")
	if out, err := exec.Command("git", "-C", f.root, "check-ignore", "--quiet", "--", "global-only").CombinedOutput(); err != nil {
		t.Fatalf("global exclude control did not pass: %v %s", err, out)
	}
	err := NewMindspecExecutor(f.root).CheckSlotIgnore(f.root, "global-only")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("CheckSlotIgnore must reject a global-only ignore: %v", err)
	}
}

func TestExecutor_SlotTreeIgnoresGlobalConfig(t *testing.T) {
	f := newSlotTrustFixture(t)
	template := t.TempDir()
	marker := "global-template-marker"
	if err := os.WriteFile(filepath.Join(template, marker), []byte("global template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(parent, "tree")
	redirect := filepath.Join(t.TempDir(), "redirect.config")
	if err := os.WriteFile(redirect, []byte("[core]\n\tworktree = "+strconv.Quote(filepath.Join(t.TempDir(), "absent"))+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.addGlobal(t, "[init]\n\ttemplateDir = "+strconv.Quote(template)+"\n"+
		"[includeIf "+strconv.Quote("gitdir:"+filepath.ToSlash(tree)+"/.git")+"]\n\tpath = "+strconv.Quote(redirect)+"\n")
	// Prove that Git reads the global template before testing its isolation.
	control := t.TempDir()
	if out, err := exec.Command("git", "-C", control, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("template control: %v %s", err, out)
	}
	if raw, err := os.ReadFile(filepath.Join(control, ".git", marker)); err != nil || string(raw) != "global template\n" {
		t.Fatalf("global template control did not copy marker: %q %v", raw, err)
	}
	executor := NewMindspecExecutor(f.root)
	if err := executor.CloneSlotTree(tree, f.sha); err != nil {
		t.Fatalf("CloneSlotTree read global slot settings: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(tree, ".git", marker)); !os.IsNotExist(err) {
		t.Fatalf("CloneSlotTree copied the global template marker: %v", err)
	}
	if err := executor.CheckSlotTree(tree, f.sha); err != nil {
		t.Fatalf("CheckSlotTree read global slot settings: %v", err)
	}
	calls, err := os.ReadFile(f.slotCalls)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, line := range strings.Split(string(calls), "\n") {
		dir, global, ok := strings.Cut(line, "\t")
		if ok && dir == tree {
			seen = true
			if global != "/dev/null" {
				t.Fatalf("slot-tree Git call kept global config: %q", global)
			}
		}
	}
	if !seen {
		t.Fatal("no slot-tree Git calls were recorded")
	}
	cmd := exec.Command("git", "-C", tree, "config", "--get", "core.worktree")
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), "absent") {
		t.Fatalf("slot redirect control did not read global config: %v %s", err, out)
	}
}
