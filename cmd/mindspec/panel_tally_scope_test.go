package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// An ad hoc tally keeps its own recovery when a sibling spec is enrolled.
func TestS134PanelTally_AdhocWithEnrolledSibling(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, _ := actorCmdFixture(t)
	rebindWrite(t, f.Root, ".beads/config.yaml", "types.custom: gate,step\n")
	rebindWrite(t, f.Root, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: test-lens\n      count: 2\n")
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=T", "-c", "user.email=t@example.invalid", "commit", "-qm", "cfg")
	actorCmdStore(t, f)
	out, code := actorCmdRun(t, f, bin, "panel", "create", "demo", "--gate", "adhoc", "--target", "HEAD")
	t.Logf("create exit=%d out=%s", code, out)
	if code != 0 {
		t.Fatalf("create failed")
	}
	var dir string
	_ = filepath.Walk(f.Root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Name() == "panel.json" && strings.Contains(p, "demo") {
			dir = filepath.Dir(p)
		}
		return nil
	})
	t.Logf("panel dir=%s", dir)
	data, _ := os.ReadFile(filepath.Join(dir, "panel.json"))
	t.Logf("panel.json=%s", data)
	f.Must(os.WriteFile(filepath.Join(dir, "R1-round-1.json"), []byte(`{"verdict":"REJECT"}`), 0o644))
	out, code = actorCmdRun(t, f, bin, "panel", "tally", "demo")
	t.Logf("tally exit=%d out=%s", code, out)
	if code == 0 {
		t.Fatal("blocked tally succeeded")
	}
	if strings.Contains(out, "next-action") {
		t.Errorf("ad hoc tally routed to kernel recovery of enrolled spec %s", f.SpecID)
	}
	if !strings.Contains(out, "mindspec panel create") {
		t.Errorf("adhoc re-panel recovery missing")
	}
}

func TestS134PanelTally_PreKernelTrackerFailure(t *testing.T) {
	f, _ := panelPreKernelFixture(t)
	old := panelKernelStateFn
	t.Cleanup(func() { panelKernelStateFn = old })
	panelKernelStateFn = func(_, _ string) (*kernel.State, error) { return nil, nil }
	out, err := kernelPanelRun(t, panelCreateCmd, "legacy", "--spec", f.SpecID, "--gate", "final_review", "--target", "spec/"+f.SpecID)
	kernelPanelRequire(t, out, err)
	calls := 0
	panelKernelStateFn = func(_, _ string) (*kernel.State, error) { calls++; return nil, errors.New("tracker unavailable") }
	out, err = kernelPanelRun(t, panelTallyCmd, "legacy", "--spec", f.SpecID)
	if calls != 1 || err == nil || !strings.Contains(err.Error(), "mindspec panel create") || strings.Contains(err.Error(), "tracker unavailable") || strings.Contains(err.Error(), "next-action") {
		t.Fatalf("legacy recovery lost on tracker failure: calls=%d out=%s err=%v", calls, out, err)
	}
}

// An orchestrated bead or final gate whose tracker cannot be read keeps the
// exit of an Allow decision: it prints no Next block and exits 0.
func TestS134PanelTally_OrchestratedTrackerFailureKeepsAllow(t *testing.T) {
	f, head := panelPreKernelFixture(t)
	old := panelKernelStateFn
	t.Cleanup(func() { panelKernelStateFn = old })
	panelKernelStateFn = func(_, _ string) (*kernel.State, error) { return nil, nil }
	out, err := kernelPanelRun(t, panelCreateCmd, "orch", "--spec", f.SpecID, "--gate", "final_review", "--target", "spec/"+f.SpecID)
	kernelPanelRequire(t, out, err)
	dir := filepath.Join(f.SpecDir, "reviews", "orch")
	for _, slot := range []string{"R1", "R2"} {
		v := paneltest.Verdict(panel.VerdictApprove)
		v.Slot, v.Round, v.ReviewedSHA, v.Model = slot, 1, head, "test-model"
		file := filepath.Join(dir, slot+"-round-1.json")
		f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0o644))
		out, err = kernelPanelRun(t, panelSubmitCmd, "orch", "--spec", f.SpecID, "--slot", slot, "--round", "1", "--file", file)
		kernelPanelRequire(t, out, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "panel.json"))
	f.Must(err)
	var doc map[string]any
	f.Must(json.Unmarshal(raw, &doc))
	doc["orchestrated"] = true
	raw, err = json.Marshal(doc)
	f.Must(err)
	f.Must(os.WriteFile(filepath.Join(dir, "panel.json"), raw, 0o644))
	calls := 0
	panelKernelStateFn = func(_, _ string) (*kernel.State, error) { calls++; return nil, errors.New("tracker unavailable") }
	out, err = kernelPanelRun(t, panelTallyCmd, "orch", "--spec", f.SpecID)
	if calls == 0 {
		t.Fatalf("the tally did not read the tracker: %s", out)
	}
	if err != nil {
		t.Fatalf("an Allow tally on an unreadable tracker exited non-zero: %v\n%s", err, out)
	}
	if strings.Contains(out, "Next:") {
		t.Fatalf("an orchestrated gate printed a Next block:\n%s", out)
	}
}
