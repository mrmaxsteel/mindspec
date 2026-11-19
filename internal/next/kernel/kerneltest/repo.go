// Package kerneltest provides isolated kernel fixtures and typed seeders.
package kerneltest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

// RequireBD fails instead of skipping when the pinned executable is absent.
func RequireBD(t testing.TB) string {
	t.Helper()
	path, err := exec.LookPath("bd")
	if err != nil {
		t.Fatalf("bd 1.3.4 is required: %v", err)
	}
	out, err := command(t, "", path, "--version")
	if err != nil || !pinnedVersion(string(out)) {
		t.Fatalf("bd 1.3.4 is required: %v: %s", err, out)
	}
	return path
}
func command(t testing.TB, root, name string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if key != "HOME" && !strings.HasPrefix(key, "XDG_") && !strings.HasPrefix(key, "BEADS_") && !strings.HasPrefix(key, "BD_") && !strings.HasPrefix(key, "DOLT_") && !strings.HasPrefix(key, "GIT_") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	// Keep configuration outside the repository so snapshots contain only fixture data.
	homeRoot := root
	if homeRoot == "" {
		homeRoot = t.TempDir()
	}
	home := filepath.Join(filepath.Dir(homeRoot), ".kernel-home")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(cmd.Env, "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, "config"), "XDG_DATA_HOME="+filepath.Join(home, "data"), "XDG_CACHE_HOME="+filepath.Join(home, "cache"))
	if root != "" {
		cmd.Env = append(cmd.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(root), "BEADS_DIR="+filepath.Join(root, ".beads"))
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return out, fmt.Errorf("%w: %s", err, e.Stderr)
		}
	}
	return out, err
}
func gitRepo(t testing.TB) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "kernel")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "--allow-empty", "-qm", "initial"}} {
		if out, err := command(t, root, "git", args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

// NewRepo creates a private git repository and a real sandbox bd store.
func NewRepo(t testing.TB) string {
	t.Helper()
	path := RequireBD(t)
	root := gitRepo(t)
	// Pin a local store before init can search enclosing configuration.
	write(t, filepath.Join(root, ".beads", "config.yaml"), []byte("sync.remote: \"\"\n"))
	if out, err := command(t, root, path, "init", "--sandbox", "--skip-hooks", "--skip-agents", "--non-interactive", "--remote=", "-q", "--database", "beads", "--prefix", "kernel"); err != nil {
		t.Fatalf("bd init: %v %s", err, out)
	}
	if _, err := bead.EnsureBeadsConfig(root, false); err != nil {
		t.Fatal(err)
	}
	return root
}
func Runner(t testing.TB, root string) kernel.Runner {
	t.Helper()
	return func(args ...string) ([]byte, error) { return command(t, root, "bd", args...) }
}

// SetClock fixes durable timestamps. The caller must not run in parallel.
func SetClock(t testing.TB, at time.Time) {
	old := kernel.Now
	kernel.Now = func() time.Time { return at }
	t.Cleanup(func() { kernel.Now = old })
}
func WithRules(t testing.TB, reg *kernel.Registry) {
	t.Helper()
	t.Cleanup(kernel.ReplaceRulesForTest(reg))
}
func write(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func pinnedVersion(output string) bool {
	for _, field := range strings.Fields(output) {
		if field == "1.3.4" {
			return true
		}
	}
	return false
}
