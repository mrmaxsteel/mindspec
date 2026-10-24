package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// Use the real create verb so dispatch and submit share a registered roster.
func newDispatchFixture(t *testing.T) slotFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	slotTestGit(t, root, "init", "-q", "-b", "main")
	slotTestGitInput(t, root, "commit refs/heads/main\ncommitter Slot Test <slot@example.invalid> 1760000000 +0000\ndata 7\ninitial\nM 100644 inline README.md\ndata 6\nhello\nM 100644 inline .gitignore\ndata 18\n.mindspec-scratch\n\n\ndone\n", "fast-import", "--quiet")
	slotTestGit(t, root, "checkout", "-f", "main")
	slotWrite(t, filepath.Join(root, ".mindspec", "config.yaml"), "panel: {isolation_root: .mindspec-scratch}\n")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	withTestChdir(t, root)
	stubWorktreeListEmpty(t)
	resetPanelCreateFlags(t)
	out, err := runPanelCmd("create", "p", "--gate", "adhoc", "--target", "main", "--reviewers", "2")
	if err != nil {
		t.Fatalf("create: %v %s", err, out)
	}
	reg, err := findPanelRegistration(root, "p", "")
	if err != nil || reg.Err != nil {
		t.Fatalf("registration: %v %v", err, reg.Err)
	}
	slotWrite(t, filepath.Join(reg.Dir, "BRIEF.md"), "Review this fixture.\n")
	resetV2Flags(t)
	v := paneltest.Verdict("APPROVE")
	v.Slot, v.Model, v.ReviewedSHA = "R2", reg.Panel.Roster[1].Model, reg.Panel.ReviewedHeadSHA
	file := filepath.Join(root, "other-slot.json")
	slotWrite(t, file, string(paneltest.Bytes(t, v)))
	if out, err := runPanelCmd("submit", "p", "--slot", "R2", "--round", "1", "--file", file); err != nil {
		t.Fatalf("other slot submit: %v %s", err, out)
	}
	resetV2Flags(t)
	return slotFixture{root: root, dir: reg.Dir, isolation: ".mindspec-scratch", p: reg.Panel}
}

func dispatchVerdictFile(t *testing.T, f slotFixture, attempt int) string {
	t.Helper()
	v := paneltest.Verdict("APPROVE")
	v.ReviewedSHA = f.p.ReviewedHeadSHA
	v.Model = f.p.Roster[0].Model
	path := filepath.Join(f.layout(t, "R1").AttemptDir(attempt), "verdict.json")
	slotWrite(t, path, string(paneltest.Bytes(t, v)))
	return path
}
func dispatchSubmit(t *testing.T, file string) (string, error) {
	t.Helper()
	resetV2Flags(t)
	return runPanelCmd("submit", "--file", file)
}
func dispatchVerify(t *testing.T, f slotFixture, want string) {
	t.Helper()
	resetV2Flags(t)
	out, err := runPanelCmd("verify", "p", "--json")
	code := 1
	if want == "final" {
		code = 0
	}
	if got := otelExitCode(err); got != code {
		t.Fatalf("verify exit %d, want %d: %s %v", got, code, out, err)
	}
	var slots []panel.SlotStatus
	if err := json.NewDecoder(strings.NewReader(out)).Decode(&slots); err != nil {
		t.Fatalf("verify JSON: %v\n%s", err, out)
	}
	for _, s := range slots {
		if s.Slot == "R1" {
			if s.Status != want || s.Kind != "harness" {
				t.Fatalf("R1 = %+v, want %s/harness", s, want)
			}
			return
		}
	}
	t.Fatalf("verify omitted R1: %s", out)
}
func assertDispatchRecords(t *testing.T, f slotFixture, want []string) {
	t.Helper()
	attempts, err := panel.SlotAttempts(f.dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, a := range attempts["R1"] {
		got = append(got, a.Status)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attempt events = %v, want %v", got, want)
	}
}

func TestPanelSubmit_DispatchThenSubmitE2E(t *testing.T) {
	f := newDispatchFixture(t)
	checkoutOK(t, "--slot", "R1", "--dispatch")
	l := f.layout(t, "R1")
	if got := slotTestGit(t, l.Tree, "rev-parse", "HEAD"); got != f.p.ReviewedHeadSHA {
		t.Fatalf("clone HEAD = %s", got)
	}
	assertDispatchRecords(t, f, []string{"dispatched"})
	dispatchVerify(t, f, "running")
	file := dispatchVerdictFile(t, f, 1)
	withTestChdir(t, filepath.Dir(file))
	if out, err := dispatchSubmit(t, "verdict.json"); err != nil {
		t.Fatalf("short submit from attempt: %v %s", err, out)
	}
	if !bytes.Equal(slotRead(t, file), slotRead(t, filepath.Join(f.dir, "R1-round-1.json"))) {
		t.Fatal("submitted bytes differ")
	}
	records, err := panel.LatestSubmissions(f.dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if records["R1"].Seq != 1 || records["R1"].Via != "direct" {
		t.Fatalf("submission = %+v", records)
	}
	assertDispatchRecords(t, f, []string{"dispatched", "submitted"})
	dispatchVerify(t, f, "final")
	withTestChdir(t, f.root)
	checkoutOK(t, "--slot", "R1", "--dispatch", "--replace")
	before := slotSnapshot(t, f.dir)
	withTestChdir(t, filepath.Dir(file))
	out, err := dispatchSubmit(t, "verdict.json")
	if err == nil || !strings.Contains(err.Error()+out, "attempt 1 was superseded by 2") {
		t.Fatalf("stale attempt accepted: %v %s", err, out)
	}
	if !reflect.DeepEqual(before, slotSnapshot(t, f.dir)) {
		t.Fatal("refused submit changed panel bytes")
	}
	dispatchVerify(t, f, "running")
}

func TestPanelDispatch_StaleFileRefused(t *testing.T) {
	f := newDispatchFixture(t)
	checkoutOK(t, "--slot", "R1", "--dispatch")
	file := dispatchVerdictFile(t, f, 1)
	if out, err := dispatchSubmit(t, file); err != nil {
		t.Fatalf("initial submit: %v %s", err, out)
	}
	dispatchVerify(t, f, "final")
	checkoutOK(t, "--slot", "R1", "--dispatch", "--replace")
	// The current attempt has no verdict. An older final is no replacement.
	missing := filepath.Join(f.layout(t, "R1").AttemptDir(2), "verdict.json")
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("new attempt already has a verdict: %v", err)
	}
	before := slotSnapshot(t, f.dir)
	withTestChdir(t, filepath.Dir(missing))
	out, err := dispatchSubmit(t, "verdict.json")
	if err == nil || !strings.Contains(err.Error()+out, "verdict.json") {
		t.Fatalf("absent replacement accepted: %v %s", err, out)
	}
	if !reflect.DeepEqual(before, slotSnapshot(t, f.dir)) {
		t.Fatal("refusal changed panel or ledger")
	}
	assertDispatchRecords(t, f, []string{"dispatched", "submitted", "dispatched"})
	dispatchVerify(t, f, "running")
	// Positive control: the same final bytes close the new attempt.
	replacement := dispatchVerdictFile(t, f, 2)
	if out, err := dispatchSubmit(t, replacement); err != nil {
		t.Fatalf("replacement submit: %v %s", err, out)
	}
	dispatchVerify(t, f, "final")
	records, err := panel.LatestSubmissions(f.dir, 1)
	if err != nil || records["R1"].Seq != 1 {
		t.Fatalf("same-bytes replacement added a verdict: %+v %v", records, err)
	}
	assertDispatchRecords(t, f, []string{"dispatched", "submitted", "dispatched", "submitted"})
}
