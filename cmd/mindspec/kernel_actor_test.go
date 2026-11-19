package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

// TestS134FakeBDProcess is a subprocess helper for binary tests. It only uses a
// serialized fake store in a temporary directory, never an installed tracker.
func TestS134FakeBDProcess(t *testing.T) {
	if os.Getenv("MINDSPEC_TEST_FAKE_BD") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	path := os.Getenv("MINDSPEC_TEST_FAKE_STORE")
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fake := kerneltest.NewFakeBD()
	if err = json.Unmarshal(data, &fake.Records); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	out, err := fake.Run(args[1:]...)
	if err != nil {
		var code interface{ ExitCode() int }
		fmt.Fprintln(os.Stderr, err)
		if errors.As(err, &code) {
			os.Exit(code.ExitCode())
		}
		os.Exit(1)
	}
	data, err = json.Marshal(fake.Records)
	if err == nil {
		err = os.WriteFile(path, data, 0600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if _, err = os.Stdout.Write(out); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
func actorCmdGit(t *testing.T, f *kerneltest.Fixture, args ...string) string {
	t.Helper()
	data, err := f.Command("git", args...)
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}
func actorCmdFixture(t *testing.T) (*kerneltest.Fixture, string) {
	t.Helper()
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	actorCmdGit(t, f, "branch", "spec/"+f.SpecID)
	actorCmdGit(t, f, "branch", "bead/"+f.Items[0])
	id := f.Items[0] + ".impl.a1"
	f.Create(id, f.Items[0])
	return f, id
}
func actorCmdStore(t *testing.T, f *kerneltest.Fixture) string {
	t.Helper()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "store.json")
	data, err := json.Marshal(f.Fake.Records)
	f.Must(err)
	f.Must(os.WriteFile(statePath, data, 0600))
	executable, err := os.Executable()
	f.Must(err)
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nexec " + quote(executable) + " -test.run='^TestS134FakeBDProcess$' -- \"$@\"\n"
	f.Must(os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MINDSPEC_TEST_FAKE_BD", "1")
	t.Setenv("MINDSPEC_TEST_FAKE_STORE", statePath)
	return statePath
}
func actorCmdRead(t *testing.T, path string, id string) kernel.Record {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows map[string]kernel.Record
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Key() == id {
			return r
		}
	}
	t.Fatal("missing row")
	return kernel.Record{}
}
func actorCmdRun(t *testing.T, f *kerneltest.Fixture, bin string, args ...string) (string, int) {
	t.Helper()
	out, err := f.Command(bin, args...)
	if err == nil {
		return string(out), 0
	}
	var code *exec.ExitError
	if !errors.As(err, &code) {
		t.Fatalf("binary failed to start: %v", err)
	}
	return string(out) + err.Error(), code.ExitCode()
}
func TestS134LeaseCmd_GenerationAndTakeRecovery(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, _ := actorCmdFixture(t)
	gate := f.Items[0] + ".review"
	id := gate + ".r1.R1"
	f.Create(id, gate)
	store := actorCmdStore(t, f)
	out, code := actorCmdRun(t, f, bin, "lease", id)
	lines := strings.Fields(out)
	if code != 0 || len(lines) != 3 || lines[0] != "generation" || lines[1] != "1" || lines[2] == "" {
		t.Fatalf("first lease: %d %s", code, out)
	}
	token := lines[2]
	row := actorCmdRead(t, store, id)
	lease, _, err := kernel.Decode[kernel.LeaseRecord](row.Metadata, kernel.KeyLease)
	f.Must(err)
	if lease.Generation != 1 || lease.TokenHash == token {
		t.Fatal("binary did not store fenced lease")
	}
	before, err := os.ReadFile(store)
	f.Must(err)
	out, code = actorCmdRun(t, f, bin, "lease", id)
	if code == 0 || !strings.Contains(out, "lease --take") {
		t.Fatalf("live lease: %d %s", code, out)
	}
	after, err := os.ReadFile(store)
	f.Must(err)
	if string(before) != string(after) {
		t.Fatal("live refusal wrote")
	}
	lease.Expires = time.Now().Add(-time.Hour)
	row.Metadata[kernel.KeyLease] = lease
	var rows map[string]kernel.Record
	f.Must(json.Unmarshal(after, &rows))
	rows[row.ID] = row
	data, err := json.Marshal(rows)
	f.Must(err)
	f.Must(os.WriteFile(store, data, 0600))
	out, code = actorCmdRun(t, f, bin, "lease", id)
	if code != 0 || !strings.Contains(out, "generation 2") || strings.Contains(out, token) {
		t.Fatalf("expiry: %d %s", code, out)
	}
}
func TestS134SubmitCmd_ImplementAndHalt(t *testing.T) {
	bin := buildMindspecBinary(t)
	t.Run("implement", func(t *testing.T) {
		f, id := actorCmdFixture(t)
		lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{EnsureWorktree: func(string, string, string) (string, error) { return f.Root, nil }})
		f.Must(err)
		f.Must(os.WriteFile(filepath.Join(f.Root, "proof.json"), []byte("{}"), 0644))
		actorCmdGit(t, f, "add", "proof.json")
		actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "implementation")
		commit := actorCmdGit(t, f, "rev-parse", "HEAD")
		actorCmdGit(t, f, "update-ref", "refs/heads/bead/"+f.Items[0], commit)
		store := actorCmdStore(t, f)
		out, code := actorCmdRun(t, f, bin, "submit", id, "--lease", lease.Token, "--commit", commit)
		if code != 0 {
			t.Fatalf("submit: %d %s", code, out)
		}
		row := actorCmdRead(t, store, id)
		receipt, ok, err := kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
		f.Must(err)
		if row.Status != "closed" || !ok || receipt.Kind != "passed" || receipt.Commit != commit || receipt.Blob != actorCmdGit(t, f, "rev-parse", commit+":proof.json") {
			t.Fatalf("binary receipt/close missing: %+v", row)
		}
		before, err := os.ReadFile(store)
		f.Must(err)
		out, code = actorCmdRun(t, f, bin, "submit", id, "--lease", lease.Token, "--commit", commit)
		after, err := os.ReadFile(store)
		f.Must(err)
		if code != 0 || string(before) != string(after) {
			t.Fatalf("binary retry: %d %s", code, out)
		}
	})
	t.Run("halt", func(t *testing.T) {
		f, _ := actorCmdFixture(t)
		d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
		store := actorCmdStore(t, f)
		for i := 0; i < 2; i++ {
			out, code := actorCmdRun(t, f, bin, "submit", d, "--answer", "halt")
			if code != 4 || !strings.Contains(out, `"status":"halted"`) {
				t.Fatalf("halt exit: %d %s", code, out)
			}
			row := actorCmdRead(t, store, d)
			receipt, _, err := kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
			f.Must(err)
			if row.Status != "closed" || receipt.Answer == nil || receipt.Answer.Option != "halt" {
				t.Fatalf("halt receipt/close missing: %+v", row)
			}
		}
	})
}
