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

// submitVerdict leases one slot and submits the given verdict for it.
func (e *kernelE2E) submitVerdict(a kernel.Action, v panel.VerdictV2) {
	e.t.Helper()
	token := e.lease(a.Step)
	r := e.row(a.Step)
	tuple, _, err := r.Tuple()
	e.must(err)
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(r.Parent).Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok {
		e.t.Fatalf("no registration for %s", a.Step)
	}
	v.Slot = fmt.Sprintf("R%d", tuple.Slot)
	v.Round = a.Round
	v.ReviewedSHA = a.Subject.Commit
	v.Model = "test-model"
	path := filepath.Join(".mindspec/specs", e.f.SpecID, "reviews", reg.Slug, fmt.Sprintf("verdict-%d-%s.json", a.Round, v.Slot))
	e.commit(e.proof, path, string(paneltest.Bytes(e.t, v)))
	e.ok("submit", a.Step, "--lease", token, "--artifact", filepath.Join(e.proof, path))
}

// panelFile is the panel-directory path of one file of the item's review gate.
func (e *kernelE2E) panelFile(gate, name string) string {
	e.t.Helper()
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok {
		e.t.Fatalf("no registration for %s", gate)
	}
	return filepath.Join(".mindspec/specs", e.f.SpecID, "reviews", reg.Slug, name)
}

// retryPasses checks that the pass after the last slot gives no verify and no
// decision but the slot's attempt a2, and that APPROVE on a2 passes the gate.
func (e *kernelE2E) retryPasses(gate, slot string) {
	e.t.Helper()
	out := e.apply()
	var a2 *kernel.Action
	for i, a := range out.Actions {
		if a.Kind == "verify" || a.Kind == "fix" || strings.Contains(a.Step, ".decide.") {
			e.t.Fatalf("the pass gave %s, want only the next attempt of %s: %+v", a.Step, slot, out)
		}
		if a.Step == gate+".r1."+slot+".a2" {
			a2 = &out.Actions[i]
		}
	}
	if a2 == nil {
		e.t.Fatalf("no %s.r1.%s.a2 attempt: %+v", gate, slot, out)
	}
	e.submitAttempt(*a2, "APPROVE")
	e.apply()
	e.ok("complete", strings.TrimSuffix(gate, ".review"))
}

// AC19: with every slot closed, an INVALID verdict file for R1 gives no verify
// and the attempt R1.a2, whose APPROVE passes the gate.
func TestS134E2E_CriterionInvalidVerdictRetries(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	e.apply()
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	e.submitSlot(slots[0], "APPROVE")
	// The binary accepts only well-formed submissions, so the INVALID content
	// reaches the panel directory as a commit over R1's closed verdict file. A
	// verdict file over the size limit is refused when the state is read, so
	// it is not a route to an INVALID slot.
	e.commit(e.cwd, e.panelFile(gate, "R1-round-1.json"), `{"verdict":"INVALID"}`)
	e.submitSlot(slots[1], "APPROVE")
	e.submitSlot(slots[2], "APPROVE")
	e.retryPasses(gate, "R1")
}

// AC19: all slots APPROVE, and a panel-directory commit that edits R1's closed
// verdict file (still a well-formed APPROVE) before the last slot submits: no
// verify, the attempt R1.a2, and its APPROVE passes the gate.
func TestS134E2E_CriterionClosedVerdictEditRetries(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	e.apply()
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	e.submitSlot(slots[0], "APPROVE")
	e.submitSlot(slots[1], "APPROVE")
	name := e.panelFile(gate, "R1-round-1.json")
	data, err := os.ReadFile(filepath.Join(e.cwd, name))
	if err != nil {
		t.Fatalf("R1's closed verdict file is not in the panel directory: %v", err)
	}
	var doc map[string]any
	e.must(json.Unmarshal(data, &doc))
	doc["rationale"] = "edited after the close"
	edited, err := json.Marshal(doc)
	e.must(err)
	e.commit(e.cwd, name, string(edited))
	e.submitSlot(slots[2], "APPROVE")
	e.retryPasses(gate, "R1")
}

// AC19: a hard_block REQUEST_CHANGES verdict gives only its veto finding.
func TestS134E2E_CriterionHardBlockGivesOnlyVeto(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	e.apply()
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	v := paneltest.Verdict("REQUEST_CHANGES")
	v.HardBlock = true
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "first change", Basis: "traced"}, {Text: "second change", Basis: "traced"}}
	e.submitVerdict(slots[0], v)
	e.submitSlot(slots[1], "APPROVE")
	e.submitSlot(slots[2], "APPROVE")
	out := e.apply()
	verify := gate + ".r1.verify"
	found := false
	for _, a := range out.Actions {
		found = found || a.Step == verify
		if a.Kind == "fix" {
			t.Fatalf("a hard_block verdict gave a fix before the verify: %+v", out)
		}
	}
	if !found {
		t.Fatalf("no verify for the hard_block verdict: %+v", out)
	}
	findings, _, err := kernel.Decode[[]kernel.Finding](e.row(verify).Metadata, kernel.KeyFindings)
	e.must(err)
	if len(findings) != 1 || findings[0].Finding != "veto" || findings[0].Slot != "R1" {
		t.Fatalf("a hard_block verdict must give only its veto finding: %+v", findings)
	}
}
