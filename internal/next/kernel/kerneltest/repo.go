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
	return commandWithTimeout(t, root, 30*time.Second, false, name, args...)
}
func commandWithTimeout(t testing.TB, root string, limit time.Duration, combined bool, name string, args ...string) ([]byte, error) {
	t.Helper()
	return commandIn(t, root, root, limit, combined, name, args...)
}
func commandIn(t testing.TB, root, dir string, limit time.Duration, combined bool, name string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := commandContext(t, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if key != "HOME" && !strings.HasPrefix(key, "XDG_") && !strings.HasPrefix(key, "BEADS_") && !strings.HasPrefix(key, "BD_") && !strings.HasPrefix(key, "DOLT_") && !strings.HasPrefix(key, "GIT_") && !strings.HasPrefix(key, "OTEL_") && key != "CLAUDE_CODE_ENABLE_TELEMETRY" {
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
	var out []byte
	var err error
	if combined {
		out, err = cmd.CombinedOutput()
	} else {
		out, err = cmd.Output()
	}
	if ctx.Err() != nil {
		return out, fmt.Errorf("command timed out: %w", ctx.Err())
	}
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return out, fmt.Errorf("%w: %s", err, e.Stderr)
		}
	}
	return out, err
}

// commandContext reserves time for error output before the test deadline.
// A zero limit uses the test deadline or a bounded fallback.
// Temporary directory cleanup can use the reporting margin; it is not a cleanup guarantee.
func commandContext(t testing.TB, limit time.Duration) (context.Context, context.CancelFunc) {
	if limit > 0 {
		return context.WithTimeout(context.Background(), limit) // #nosec G118 -- The caller cancels the returned context.
	}
	if timed, ok := t.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, ok := timed.Deadline(); ok {
			return context.WithDeadline(context.Background(), deadline.Add(-5*time.Second)) // #nosec G118 -- The caller cancels the returned context.
		}
	}
	return context.WithTimeout(context.Background(), 90*time.Minute) // #nosec G118 -- The caller cancels the returned context.
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
	write(t, filepath.Join(root, ".beads", "config.yaml"), []byte("sync.remote: \"\"\nmetrics.disabled: true\n"))
	if out, err := commandWithTimeout(t, root, 90*time.Second, false, path, "init", "--sandbox", "--skip-hooks", "--skip-agents", "--non-interactive", "--remote=", "-q", "--database", "beads", "--prefix", "kernel"); err != nil {
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

// EmptyRepo creates a real isolated repository without seeding lifecycle records.
// Binary tests use it to create their fixtures through the public commands.
func EmptyRepo(t testing.TB) *Fixture {
	t.Helper()
	return &Fixture{Root: NewRepo(t), t: t}
}

// BinaryCommand captures both output streams and uses the test deadline.
// It uses the same isolated environment as tracker and git fixture commands.
func (f *Fixture) BinaryCommand(name string, args ...string) ([]byte, error) {
	return commandWithTimeout(f.t, f.Root, 0, true, name, args...)
}

// BinaryCommandIn retains the store and home while selecting a live worktree.
func (f *Fixture) BinaryCommandIn(dir, name string, args ...string) ([]byte, error) {
	return commandIn(f.t, f.Root, dir, 0, true, name, args...)
}
