package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

// slotActions returns the review-slot actions of the current pass.
func (e *kernelE2E) slotActions() []kernel.Action {
	e.t.Helper()
	var slots []kernel.Action
	for _, a := range e.apply().Actions {
		if a.Kind == "review-slot" {
			slots = append(slots, a)
		}
	}
	return slots
}

// submitSlot leases one slot and submits the word for it.
func (e *kernelE2E) submitSlot(a kernel.Action, word string) {
	e.t.Helper()
	token := e.lease(a.Step)
	e.ok("submit", a.Step, "--lease", token, "--artifact", e.verdict(a, word))
}

// receipt decodes the close receipt of a closed step.
func (e *kernelE2E) receipt(id string) kernel.Receipt {
	e.t.Helper()
	r := e.row(id)
	receipt, ok, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	e.must(err)
	if r.Status != "closed" || !ok {
		e.t.Fatalf("%s is not closed with a receipt: %+v", id, r)
	}
	return receipt
}

// AC19: with every slot closed, a stray REJECT verdict file in the panel
// directory gives a verify whose one finding is the veto of that file, and no
// fix.
func TestS134E2E_CriterionStrayRejectGivesVeto(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	e.apply()
	gate := item + ".review"
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	for _, a := range slots[:2] {
		e.submitSlot(a, "APPROVE")
	}
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok {
		t.Fatal("no registration")
	}
	stray := filepath.Join(".mindspec/specs", e.f.SpecID, "reviews", reg.Slug, "R9-round-1.json")
	e.commit(e.cwd, stray, `{"verdict":"REJECT","rationale":"stray veto","concrete_changes_required":["stray change"]}`)
	e.submitSlot(slots[2], "APPROVE")
	out := e.apply()
	verify := gate + ".r1.verify"
	found := false
	for _, a := range out.Actions {
		found = found || a.Step == verify
		if a.Kind == "fix" {
			t.Fatalf("a stray veto gave a fix before the verify: %+v", out)
		}
	}
	if !found {
		t.Fatalf("no verify for the stray veto: %+v", out)
	}
	findings, _, err := kernel.Decode[[]kernel.Finding](e.row(verify).Metadata, kernel.KeyFindings)
	e.must(err)
	if len(findings) != 1 || findings[0].Finding != "veto" || findings[0].Slot != "R9" {
		t.Fatalf("the verify must hold one veto finding for the stray file: %+v", findings)
	}
	if e.row(gate).Status == "closed" {
		t.Fatal("the gate closed past a stray veto")
	}
}

// AC20: a commit on the work item during round 1, then the last verdict: the
// pass opens the change judge on the round subject and the gate stays open. A
// re-panel that omits a changed path exits non-zero naming it; a carry-over
// then closes the gate passed on the new subject and complete exits 0.
func TestS134E2E_CriterionCommitDuringRound(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	e.apply()
	gate := item + ".review"
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	subject := slots[0].Subject.Commit
	for _, a := range slots[:2] {
		e.submitSlot(a, "APPROVE")
	}
	e.commit(e.tree(item), "kept.txt", "kept\n")
	moved := e.commit(e.tree(item), "omitted.txt", "omitted\n")
	e.submitSlot(slots[2], "APPROVE")
	e.apply()
	judge := gate + ".judge.a1"
	if e.row(judge).Status != "open" || e.row(gate).Status != "open" {
		t.Fatalf("judge %s, gate %s", e.row(judge).Status, e.row(gate).Status)
	}
	step, token, path := e.judgement("re-panel", 2, func(j *kernel.Judgement) {
		if j.OldSubject != subject || j.NewSubject != moved {
			t.Fatalf("judge subjects %s..%s, want %s..%s", j.OldSubject, j.NewSubject, subject, moved)
		}
		j.Paths = []string{"kept.txt"}
	})
	e.refuse("omitted.txt", "submit", step, "--lease", token, "--record", path)
	if status := e.row(judge).Status; status == "closed" {
		t.Fatalf("a refused judgement closed the judge: %s", status)
	}
	carry, err := json.Marshal(kernel.Judgement{Value: "carry-over", OldSubject: subject, NewSubject: moved, Reason: "The review covers this change."})
	e.must(err)
	path = filepath.Join(t.TempDir(), "carry-over.json")
	e.must(os.WriteFile(path, carry, 0600))
	e.ok("submit", step, "--lease", token, "--record", path)
	e.apply()
	if r := e.receipt(gate); r.Kind != "passed" || r.Subject != moved {
		t.Fatalf("gate receipt %+v, want passed on %s", r, moved)
	}
	e.ok("complete", item)
}

// AC21: a lens step leased by generation 1 and taken by generation 2, then a
// commit that moves its subject: the pass closes it superseded and opens the
// next attempt on the new subject; the generation-1 submit exits non-zero, and
// the new attempt and panel create exit 0.
func TestS134E2E_CriterionLensTakeOnMovedSubject(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	e.apply()
	var lens kernel.Action
	for _, a := range e.implement(0).Actions {
		if a.Kind == "lens" {
			lens = a
		}
	}
	if lens.Step == "" {
		t.Fatal("no lens step")
	}
	first := e.lease(lens.Step)
	file := e.lensFile(e.lensRecord(lens))
	e.lease(lens.Step, "--take", "--reason", "Replace the lens actor.")
	moved := e.commit(e.tree(item), "moved.txt", "moved\n")
	out := e.apply()
	if r := e.receipt(lens.Step); r.Kind != "superseded" {
		t.Fatalf("lens receipt %+v, want superseded", r)
	}
	var next kernel.Action
	for _, a := range out.Actions {
		if a.Kind == "lens" {
			next = a
		}
	}
	if next.Step != lens.Step+".a2" || next.Subject.Commit != moved {
		t.Fatalf("next lens %+v, want %s.a2 on %s", next, lens.Step, moved)
	}
	if _, code := e.run("submit", lens.Step, "--lease", first, "--record", file); code == 0 {
		t.Fatal("a generation-1 submit closed a superseded lens step")
	}
	e.create(out)
}

const kernelE2EAdversaryConfig = "enforcement:\n  panel_gate: false\nauto_open_finalize_pr: false\npanel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: correctness\n      count: 3\n    - family: harness\n      model: test-model\n      lens: adversarial\n"

// AC22: a 3-slot scoped round on a roster whose adversary is fourth has slots
// R1, R2 and R3, with R3 the adversary. A lens for the adversary slot, a lens
// with a missing slot and an empty lens each exit non-zero with the step open;
// panel create before the lens closes exits non-zero naming the lens step, and
// with a brief file exits non-zero. A lens that names R1 and R2 only, the
// panel, its slots, the tally and complete exit 0.
func TestS134E2E_CriterionScopedRoundLens(t *testing.T) {
	t.Parallel()
	e := newKernelE2EWithConfig(t, kernelE2EBinary(t), kerneltest.EmptyRepo(t), ".mindspec", kernelE2EAdversaryConfig, 1)
	item := e.f.Items[0]
	e.apply()
	e.create(e.implement(0))
	e.reviewSlots("APPROVE", 4)
	e.commit(e.tree(item), "scoped.txt", "scoped\n")
	e.judge("re-panel", 3)
	var lens kernel.Action
	for _, a := range e.apply().Actions {
		if a.Kind == "lens" {
			lens = a
		}
	}
	if lens.Step == "" {
		t.Fatal("no lens step for the scoped round")
	}
	e.refuse(lens.Step, "panel", "create", item)
	brief := filepath.Join(t.TempDir(), "brief.md")
	e.must(os.WriteFile(brief, []byte("# Brief\n\nReview it.\n"), 0600))
	e.refuse("recovery:", "panel", "create", item, "--brief-file", brief)
	token := e.lease(lens.Step)
	good := e.lensRecord(lens)
	var slots []string
	for _, l := range good.Lenses {
		slots = append(slots, l.Slot)
	}
	if strings.Join(slots, ",") != "R1,R2" {
		t.Fatalf("lens slots %v, want R1,R2", slots)
	}
	for name, lenses := range map[string][]kernel.SlotLens{
		"adversary": append(append([]kernel.SlotLens{}, good.Lenses...), kernel.SlotLens{Slot: "R3", Lens: "Review adversarially."}),
		"missing":   good.Lenses[:1],
		"empty":     {good.Lenses[0], {Slot: "R2", Lens: ""}},
	} {
		bad := kernel.LensRecord{Step: lens.Step, Lenses: lenses}
		if out, code := e.run("submit", lens.Step, "--lease", token, "--record", e.lensFile(bad)); code == 0 || !strings.Contains(out, "recovery:") {
			t.Fatalf("%s lens accepted: %d %s", name, code, out)
		}
		if status := e.row(lens.Step).Status; status == "closed" {
			t.Fatalf("%s lens closed the step: %s", name, status)
		}
	}
	raw := e.ok("submit", lens.Step, "--lease", token, "--record", e.lensFile(good))
	var out kernel.Output
	e.must(json.Unmarshal([]byte(raw), &out))
	e.create(out)
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(item+".review.a2").Metadata, kernel.KeyRegistration)
	e.must(err)
	if prompt := e.ok("panel", "prompt", reg.Slug, "--spec", e.f.SpecID, "--slot", "R3"); !strings.Contains(prompt, "adversarial") {
		t.Fatalf("the adversary slot prompt names no adversarial lens:\n%s", prompt)
	}
	e.reviewSlots("APPROVE", 3)
	e.ok("complete", item)
}
