package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

// The step type refusal prints mindspec setup. Run in a linked worktree, that
// recovery must fix the beads config that the check reads, so the retry passes.
func TestSetup_RecoveryFromLinkedWorktree(t *testing.T) {
	main, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Setup Test", "-c", "user.email=setup@example.invalid"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(main, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(main, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, ".beads", "config.yaml"), []byte("issue-prefix: demo\ntypes.custom: gate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(main, "add", ".beads/config.yaml")
	git(main, "commit", "-qm", "beads config")
	tree := filepath.Join(main, ".worktrees", "worktree-spec")
	git(main, "worktree", "add", "-q", "-b", "spec/demo", tree)

	// Plan approval checks the main checkout's config.
	err = kernel.RequireStepType(main)
	if err == nil {
		t.Fatal("the fixture config already has the step type")
	}
	lines := strings.Split(err.Error(), "\n")
	recovery := strings.TrimPrefix(lines[len(lines)-1], "recovery: ")
	if recovery != "mindspec setup" {
		t.Fatalf("recovery = %q, want mindspec setup", recovery)
	}

	// Run the printed recovery where the refusal was printed.
	withTestChdir(t, tree)
	var out bytes.Buffer
	setupCmd.SetOut(&out)
	t.Cleanup(func() { setupCmd.SetOut(nil) })
	if err := setupCmd.RunE(setupCmd, nil); err != nil {
		t.Fatalf("%s: %v", recovery, err)
	}
	if err := kernel.RequireStepType(main); err != nil {
		t.Fatalf("retry after %s in the worktree still refused: %v\n%s", recovery, err, out.String())
	}
}

// Setup from a linked worktree must not give the beads config a prefix that
// differs from the store's. bd then refuses each new ID that plan approval
// creates in the store.
func TestSetup_LinkedWorktreeKeepsStorePrefix(t *testing.T) {
	if testing.Short() {
		t.Skip("real bd flow is skipped under -short")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Fatalf("real bd required: %v", err)
	}
	main, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0")
	run := func(dir, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	git := func(args ...string) {
		t.Helper()
		run(main, "git", append([]string{"-c", "user.name=Setup Test", "-c", "user.email=setup@example.invalid"}, args...)...)
	}
	git("init", "-q", "-b", "main")
	run(main, "bd", "init", "--prefix", "kx", "--quiet")
	git("add", "-A")
	git("commit", "-q", "--no-verify", "--allow-empty", "-m", "beads store")
	tree := filepath.Join(main, ".worktrees", "worktree-spec")
	git("worktree", "add", "-q", "-b", "spec/demo", tree)

	withTestChdir(t, tree)
	var out bytes.Buffer
	setupCmd.SetOut(&out)
	t.Cleanup(func() { setupCmd.SetOut(nil) })
	if err := setupCmd.RunE(setupCmd, nil); err != nil {
		t.Fatalf("mindspec setup: %v\n%s", err, out.String())
	}
	if err := kernel.RequireStepType(main); err != nil {
		t.Fatalf("setup did not add the step type: %v", err)
	}
	// Plan approval creates each record with an explicit ID in the store.
	run(main, "bd", "create", "plan record", "--type", "task", "--id", "kx-plan1", "--json")
	data, err := os.ReadFile(filepath.Join(main, ".beads", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "issue-prefix:") && strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "issue-prefix:")) != "kx" {
			t.Fatalf("setup wrote a prefix that differs from the store's: %q", line)
		}
	}
}
