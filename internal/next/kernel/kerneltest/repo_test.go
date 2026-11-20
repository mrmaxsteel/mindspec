package kerneltest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestS134Probe_PinnedVersion(t *testing.T) {
	for output, want := range map[string]bool{"bd version 1.3.4": true, "bd version 1.3.4 (release)": true, "bd version 1.3.40": false, "bd version 1.3.3": false, "bd version 1.3.4-dev": false} {
		if got := pinnedVersion(output); got != want {
			t.Errorf("version %q accepted=%t want=%t", output, got, want)
		}
	}
}

func TestS134Fixture_Isolation(t *testing.T) {
	outer := gitRepo(t)
	// A missing path exposes accidental remote discovery.
	poison := "sync.remote: " + filepath.Join(outer, "missing-remote") + "\n"
	write(t, filepath.Join(outer, ".beads", "config.yaml"), []byte(poison))
	home := t.TempDir()
	write(t, filepath.Join(home, ".config", "beads", "config.yaml"), []byte(poison))
	write(t, filepath.Join(home, ".beads", "config.yaml"), []byte(poison))
	write(t, filepath.Join(home, ".gitconfig"), []byte("[init]\n templateDir = /missing-kernel-template\n"))
	tmp := filepath.Join(outer, "tmp")
	if err := os.Mkdir(tmp, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("BEADS_DIR", filepath.Join(outer, ".beads"))
	t.Setenv("BEADS_SYNC_REMOTE", filepath.Join(outer, "missing-env-remote"))
	root := NewRepo(t)
	if _, err := Runner(t, root)("list", "--all", "--json"); err != nil {
		t.Fatal(err)
	}
}

func TestS134Fixture_EmptyBinaryRepo(t *testing.T) {
	f := EmptyRepo(t)
	rows, err := Runner(t, f.Root)("list", "--all", "--json")
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(f.Root, ".beads/config.yaml"))
	if err != nil || !strings.Contains(string(config), "metrics.disabled: true") {
		t.Fatalf("network metrics must be disabled: %v", err)
	}
	var records []json.RawMessage
	if err := json.Unmarshal(rows, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("empty repository seeded records: %s", rows)
	}
	out, err := f.BinaryCommand("sh", "-c", "printf stdout; printf stderr >&2")
	if err != nil || string(out) != "stdoutstderr" {
		t.Fatalf("combined binary output: %s %v", out, err)
	}
	caller := filepath.Join(f.Root, "caller")
	if err := os.Mkdir(caller, 0700); err != nil {
		t.Fatal(err)
	}
	out, err = f.BinaryCommandIn(caller, "pwd")
	physical, resolveErr := filepath.EvalSymlinks(caller)
	if err != nil || resolveErr != nil || strings.TrimSpace(string(out)) != physical {
		t.Fatalf("binary cwd: %s %v %v", out, err, resolveErr)
	}
	rootHome, err := f.BinaryCommand("sh", "-c", `printf %s "$HOME"`)
	if err != nil {
		t.Fatal(err)
	}
	callerHome, err := f.BinaryCommandIn(caller, "sh", "-c", `printf %s "$HOME"`)
	if err != nil || len(rootHome) == 0 || string(callerHome) != string(rootHome) {
		t.Fatal("binary worktree changed the isolated home")
	}

}

type deadlineFixture struct {
	testing.TB
	at time.Time
}

func (t deadlineFixture) Deadline() (time.Time, bool) { return t.at, true }

func TestS134Fixture_BinaryDeadline(t *testing.T) {
	at := time.Now().Add(10 * time.Minute)
	timed := deadlineFixture{TB: t, at: at}
	ctx, cancel := commandContext(timed, 0)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || !deadline.Equal(at.Add(-5*time.Second)) {
		t.Fatalf("binary command deadline: %v, present %t; test deadline %v", deadline, ok, at)
	}
	expired := deadlineFixture{TB: t, at: time.Now().Add(-time.Second)}
	f := &Fixture{Root: t.TempDir(), t: expired}
	for _, run := range []func() ([]byte, error){
		func() ([]byte, error) { return f.BinaryCommand("sh", "-c", "exit 0") },
		func() ([]byte, error) { return f.BinaryCommandIn(f.Root, "sh", "-c", "exit 0") },
	} {
		if _, err := run(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("binary command ignored the test deadline: %v", err)
		}
	}
	for _, combined := range []bool{false, true} {
		expired := deadlineFixture{TB: t, at: time.Now().Add(-time.Second)}
		_, err := commandIn(expired, t.TempDir(), "", 0, combined, "sh", "-c", "exit 0")
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("timeout reported as a command refusal: %v", err)
		}
	}
}
