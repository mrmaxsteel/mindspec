package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS134Final_MovedHeadAllowsNextPanel(t *testing.T) {
	e := finalKernelFixture(t)
	e.apply()
	e.create(e.implement(0))
	out := e.apply()
	var a kernel.Action
	for _, candidate := range out.Actions {
		if strings.HasSuffix(candidate.Step, ".R2") {
			a = candidate
		}
	}
	if a.Step == "" {
		t.Fatalf("no second review slot: %+v", out)
	}
	token := e.lease(a.Step)
	artifact := e.verdict(a, "APPROVE")
	item := e.f.Items[0]
	e.commit(e.tree(item), "moved.txt", "new subject\n")
	refusal := e.refuse("stale", "submit", a.Step, "--lease", token, "--artifact", artifact)
	if strings.Contains(refusal, "Usage:") {
		t.Fatalf("submit refusal printed usage: %s", refusal)
	}
	e.apply()
	gate := item + ".review"
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("%s.r1.R%d", gate, i)
		r := e.row(id)
		receipt, has, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
		e.must(err)
		if r.Status != "closed" || !has || receipt.Kind != "superseded" {
			t.Fatalf("slot was not covered: %+v", r)
		}
	}
	e.ok("panel", "create", item, "--round", "2")
	e.review("APPROVE")
	r := e.row(gate)
	receipt, has, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	e.must(err)
	if r.Status != "closed" || !has || receipt.Kind != "passed" {
		t.Fatalf("new slots were not submittable: %+v", r)
	}
}

func TestS134PanelCreate_IgnoresOlderOpenSlots(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	old := gate + ".r1.R1"
	// A foreign child keeps the old slot open even after the cleanup pass.
	f.Fake.Records["foreign-slot-child"] = kernel.Record{ID: "foreign-slot-child", Parent: f.ID(old), Type: "task", Status: "open"}
	kernelPanelAuthorise(f, gate, head, 2, 2)
	out, err = kernelPanelCreate(t, f, "1", "--round", "2")
	kernelPanelRequire(t, out, err)
	if kernelPanelRecord(t, f, old).Status != "open" {
		t.Fatal("panel create closed foreign child's parent")
	}
	for _, id := range []string{gate + ".r2.R1", gate + ".r2.R2"} {
		if kernelPanelRecord(t, f, id).Status != "open" {
			t.Fatal("current round slot absent")
		}
	}
}

func TestS134PanelTally_UsesKernelRecovery(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	dir := filepath.Join(f.SpecDir, "reviews", "1")
	for i := 1; i <= 2; i++ {
		slot := fmt.Sprintf("R%d", i)
		v := paneltest.Verdict("REQUEST_CHANGES")
		v.Slot, v.Round, v.ReviewedSHA, v.Model = slot, 1, head, "test-model"
		v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "repair this result", Basis: "traced"}}
		_, err = panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "1", Slot: slot, Round: 1}, paneltest.Bytes(t, v), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
			return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
		})
		f.Must(err)
	}
	out, err = kernelPanelRun(t, panelTallyCmd, "1", "--spec", f.SpecID)
	if err == nil || !strings.Contains(err.Error(), "recovery: mindspec next-action "+shellQuoteTarget(f.SpecID)+" --apply") || strings.Contains(err.Error(), "panel create") {
		t.Fatalf("blocked enrolled tally recovery: %s %v", out, err)
	}
}

func TestS134SubmitCmd_SupersededSlotRecovery(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	id := gate + ".r1.R1"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "superseded", Subject: head}))
	f.Must(f.Store.SetStatusIf(id, "in_progress", "closed"))
	file := filepath.Join(f.SpecDir, "reviews", "1", "old.json")
	v := paneltest.Verdict("APPROVE")
	v.Slot, v.Round, v.ReviewedSHA = "R1", 1, head
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0600))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "old verdict")
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	if err == nil || !strings.Contains(err.Error(), "superseded") || !strings.Contains(err.Error(), "recovery: mindspec next-action") {
		t.Fatalf("superseded recovery: %v", err)
	}
}

func TestS134Final_PreKernelPlanApprovalGuidance(t *testing.T) {
	e := finalKernelFixture(t)
	path := os.Getenv("MINDSPEC_TEST_FAKE_STORE")
	data, err := os.ReadFile(path)
	e.must(err)
	var rows map[string]kernel.Record
	e.must(json.Unmarshal(data, &rows))
	// Retain the older approved work graph, with no kernel witnesses.
	for id, r := range rows {
		if tuple, has, err := r.Tuple(); err == nil && has && tuple.Kind != "" {
			delete(rows, id)
			continue
		}
		if r.ID == e.f.Epic {
			delete(r.Metadata, kernel.KeyRun)
			delete(r.Metadata, kernel.KeyPlanApproval)
			delete(r.Metadata, kernel.KeyLinks)
			rows[id] = r
		}
	}
	data, err = json.Marshal(rows)
	e.must(err)
	e.must(os.WriteFile(path, data, 0600))
	out := e.ok("plan", "approve", e.f.SpecID)
	if !strings.Contains(out, "  2. Run `mindspec next` to claim your first bead") || strings.Contains(out, "mindspec next-action") {
		t.Fatalf("pre-kernel approval printed an unavailable next action: %s", out)
	}
}

func TestS134NextActionCmd_WaitOmitsUsage(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, id := actorCmdFixture(t)
	_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{EnsureWorktree: func(string, string, string) (string, error) { return f.Root, nil }})
	f.Must(err)
	actorCmdStore(t, f)
	out, code := actorCmdRun(t, f, bin, "next-action", f.SpecID, "--json")
	if code != 3 || !strings.Contains(out, `"status":"wait"`) || strings.Contains(out, "Usage:") {
		t.Fatalf("waiting result printed usage or lost its exit code: %d %s", code, out)
	}
}
