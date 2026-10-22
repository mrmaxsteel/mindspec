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

func TestExecutor_SlotTrustQueryScrubsConfigEnvironment(t *testing.T) {
	f := newSlotTrustFixture(t)
	poison := filepath.Join(t.TempDir(), "caller.config")
	if err := os.WriteFile(poison, []byte("[invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"count", "parameters"} {
		t.Run(source, func(t *testing.T) {
			if source == "count" {
				t.Setenv("GIT_CONFIG_COUNT", "1")
				t.Setenv("GIT_CONFIG_KEY_0", "include.path")
				t.Setenv("GIT_CONFIG_VALUE_0", poison)
			} else {
				t.Setenv("GIT_CONFIG_PARAMETERS", "'"+strings.ReplaceAll("include.path="+poison, "'", "'\\''")+"'")
			}
			if out, err := exec.Command("git", "config", "--global", "--get-all", "safe.directory").CombinedOutput(); err == nil || !strings.Contains(string(out), "bad config line") {
				t.Fatalf("config environment control did not refuse: %v %s", err, out)
			}
			if _, err := slotGit(f.root, "-c", "safe.directory="+f.root, "check-ignore", "--quiet", "--", "ignored"); err != nil {
				t.Fatalf("isolated control: %v", err)
			}
			if err := NewMindspecExecutor(f.root).CheckSlotIgnore(f.root, "ignored"); err != nil {
				t.Fatalf("trust query inherited repository config environment: %v", err)
			}
		})
	}
}

func TestExecutor_SlotIgnoreRejectsDefaultGlobalExcludes(t *testing.T) {
	f := newSlotTrustFixture(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := os.Mkdir(filepath.Join(xdg, "git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "git", "ignore"), []byte("global-only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", f.root, "check-ignore", "--quiet", "--", "global-only").CombinedOutput(); err != nil {
		t.Fatalf("default global exclude control did not pass: %v %s", err, out)
	}
	err := NewMindspecExecutor(f.root).CheckSlotIgnore(f.root, "global-only")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("CheckSlotIgnore must reject a default global-only ignore: %v", err)
	}
}

func TestExecutor_SlotIgnorePreservesIncludedSafeDirectory(t *testing.T) {
	f := newSlotTrustFixture(t)
	included := filepath.Join(t.TempDir(), "trust.config")
	if err := os.WriteFile(included, []byte("[safe]\n\tdirectory =\n\tdirectory = "+strconv.Quote(f.root)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.global, []byte("[include]\n\tpath = "+strconv.Quote(included)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", f.root, "check-ignore", "--quiet", "--", "ignored").CombinedOutput(); err != nil {
		t.Fatalf("included trust control did not pass: %v %s", err, out)
	}
	if err := NewMindspecExecutor(f.root).CheckSlotIgnore(f.root, "ignored"); err != nil {
		t.Fatalf("CheckSlotIgnore with included safe.directory: %v", err)
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

func TestExecutor_SlotIgnoreRejectsConditionalSafeDirectory(t *testing.T) {
	f := newSlotTrustFixture(t)
	included := filepath.Join(t.TempDir(), "conditional-trust.config")
	if err := os.WriteFile(included, []byte("[safe]\n\tdirectory = "+strconv.Quote(f.root)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.global, []byte("[includeIf \"gitdir:**/\"]\n\tpath = "+strconv.Quote(included)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1")
	if out, err := exec.Command("git", "-C", f.root, "check-ignore", "--quiet", "--", "ignored").CombinedOutput(); err == nil || !strings.Contains(string(out), "dubious ownership") {
		t.Fatalf("conditional trust ownership control did not refuse: %v %s", err, out)
	}
	if err := NewMindspecExecutor(f.root).CheckSlotIgnore(f.root, "ignored"); err == nil || !strings.Contains(err.Error(), "dubious ownership") {
		t.Fatalf("CheckSlotIgnore accepted conditional safe.directory: %v", err)
	}
}

func TestExecutor_CloneSlotTreeIgnoresDefaultGlobalAttributes(t *testing.T) {
	f := newSlotTrustFixture(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := os.Mkdir(filepath.Join(xdg, "git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "git", "attributes"), []byte("* text eol=crlf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	control := filepath.Join(t.TempDir(), "control")
	if out, err := exec.Command("git", "clone", "--shared", "--no-checkout", "--", f.root, control).CombinedOutput(); err != nil {
		t.Fatalf("attributes control clone: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", control, "checkout", "--detach", f.sha, "--").CombinedOutput(); err != nil {
		t.Fatalf("attributes control checkout: %v %s", err, out)
	}
	if raw, err := os.ReadFile(filepath.Join(control, ".gitignore")); err != nil || string(raw) != "ignored\r\n" {
		t.Fatalf("global attributes control must use CRLF: %q %v", raw, err)
	}
	tree := filepath.Join(t.TempDir(), "tree")
	if err := NewMindspecExecutor(f.root).CloneSlotTree(tree, f.sha); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(filepath.Join(tree, ".gitignore")); err != nil || string(raw) != "ignored\n" {
		t.Fatalf("CloneSlotTree must keep LF bytes: %q %v", raw, err)
	}
}

func TestExecutor_SlotTrustQueryCeilingAndRemoval(t *testing.T) {
	f := newSlotTrustFixture(t)
	host, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", host, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("temp repository: %v %s", err, out)
	}
	tmp := filepath.Join(host, "tmp")
	if err := os.Mkdir(tmp, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	included := filepath.Join(t.TempDir(), "conditional-trust.config")
	if err := os.WriteFile(included, []byte("[safe]\n\tdirectory = "+strconv.Quote(f.root)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.global, []byte("[includeIf "+strconv.Quote("gitdir:"+filepath.ToSlash(filepath.Join(host, ".git")))+"]\n\tpath = "+strconv.Quote(included)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Prove that a query below TMPDIR reads the conditional trust without a ceiling.
	control := exec.Command("git", "config", "--global", "--includes", "--get-all", "safe.directory")
	control.Dir = tmp
	control.Env = slotGitEnv(true)
	if out, err := control.CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != f.root {
		t.Fatalf("conditional trust control did not match: %v %s", err, out)
	}
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1")
	if err := NewMindspecExecutor(f.root).CheckSlotIgnore(f.root, "ignored"); err == nil || !strings.Contains(err.Error(), "dubious ownership") {
		t.Errorf("CheckSlotIgnore accepted trust from the TMPDIR repository: %v", err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "mindspec-slot-trust-") {
			t.Errorf("trust query left a temporary directory: %s", entry.Name())
		}
	}
}

func TestExecutor_CheckSlotIgnoreErrorKinds(t *testing.T) {
	f := newSlotTrustFixture(t)
	executor := NewMindspecExecutor(f.root)
	t.Run("ignored", func(t *testing.T) {
		if err := executor.CheckSlotIgnore(f.root, "ignored"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("not ignored", func(t *testing.T) {
		err := executor.CheckSlotIgnore(f.root, "unignored")
		if !errors.Is(err, ErrSlotNotIgnored) {
			t.Fatalf("exit 1 must match ErrSlotNotIgnored: %v", err)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Fatalf("ignore refusal lost exit 1: %v", err)
		}
		_, original := slotMainGit(f.root, "-c", "core.excludesFile=/dev/null", "check-ignore", "--quiet", "--", "unignored")
		if original == nil || err.Error() != original.Error() {
			t.Fatalf("ignore refusal text changed: got %v, want %v", err, original)
		}
	})
	t.Run("trust query failure", func(t *testing.T) {
		t.Setenv("TMPDIR", filepath.Join(f.root, "absent-temp-parent"))
		err := executor.CheckSlotIgnore(f.root, "ignored")
		if err == nil || errors.Is(err, ErrSlotNotIgnored) || !strings.Contains(err.Error(), "create repository trust query directory") {
			t.Fatalf("trust query failure must be operational: %v", err)
		}
	})
	t.Run("git failure", func(t *testing.T) {
		err := executor.CheckSlotIgnore(t.TempDir(), "ignored")
		var exit *exec.ExitError
		if err == nil || errors.Is(err, ErrSlotNotIgnored) || !errors.As(err, &exit) || exit.ExitCode() != 128 {
			t.Fatalf("exit 128 must be operational: %v", err)
		}
	})
}
