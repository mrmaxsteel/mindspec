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
