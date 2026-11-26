package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

// gateRound decodes the round record of the gate.
func (e *kernelE2E) gateRound(gate string) kernel.GateRound {
	e.t.Helper()
	round, ok, err := kernel.Decode[kernel.GateRound](e.row(gate).Metadata, kernel.KeyRound)
	e.must(err)
	if !ok {
		e.t.Fatalf("%s has no round", gate)
	}
	return round
}

// refuteAll submits a verify record that refutes each finding of the open
// verify, and returns the verify action.
func (e *kernelE2E) refuteAll() kernel.Action {
	e.t.Helper()
	a, findings := e.verifyAction()
	returns := []kernel.VerifyReturn{}
	for _, f := range findings {
		returns = append(returns, returnFor(f, "refuted", a.Subject.Commit))
	}
	e.ok("submit", a.Step, "--lease", e.lease(a.Step), "--record", e.jsonFile("verify.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: returns}))
	return a
}

// slotPrompt prints the panel prompt of one slot of the gate's registered
// panel.
func (e *kernelE2E) slotPrompt(gate, slot string) string {
	e.t.Helper()
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok {
		e.t.Fatalf("no registration for %s", gate)
	}
	return e.ok("panel", "prompt", reg.Slug, "--spec", e.f.SpecID, "--slot", slot)
}

// AC20: open gate, re-panel 2 on J, and a verify with no reproduced return
// authorises round 2 with prior round 1. A commit S before panel create gives
// a judge whose old subject is J and whose paths start at J. Its re-panel
// withdraws round 2, retires the verify and opens one verify attempt a2 on S.
// An all-refuted a2 authorises round 2 again on S with the latest slot count,
// scope and a fresh lens attempt. The gate stays open until that round passes.
func TestS134E2E_CriterionRepanelWithdrawnBeforeCreate(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.apply()
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	e.submitSlot(slots[1], "APPROVE")
	e.submitSlot(slots[2], "APPROVE")
	j := e.commit(e.tree(item), "changed.txt", "J\n")
	e.submitSlot(slots[0], "REQUEST_CHANGES")
	first := e.gateRound(gate)
	if first.Round != 1 {
		t.Fatalf("round %+v, want round 1", first)
	}
	e.absent(gate+".r1.verify", "a refused tally on a moved subject gave a verify before the judge closed")
	e.judge("re-panel", 2)
	a := e.refuteAll()
	if a.Step != gate+".r1.verify" || a.Subject.Commit != j {
		t.Fatalf("verify %s on %s, want %s.r1.verify on %s", a.Step, a.Subject.Commit, gate, j)
	}
	e.apply()
	two := e.gateRound(gate)
	if two.Round != 2 || two.Subject != j || two.SlotCount != 2 || two.Prior == nil || two.Prior.Round != 1 || two.Lens == "" {
		t.Fatalf("round after the verify %+v, want round 2 of 2 slots on %s with prior round 1", two, j)
	}
	if e.row(gate).Status == "closed" {
		t.Fatal("the verify closed the gate under a re-panel")
	}
	s := e.commit(e.tree(item), "later.txt", "S\n")
	step, token, path := e.judgement("re-panel", 3, func(jm *kernel.Judgement) {
		if jm.OldSubject != j || jm.NewSubject != s || !reflect.DeepEqual(jm.Paths, []string{"later.txt"}) {
			t.Fatalf("judge %s..%s paths %v, want %s..%s paths [later.txt]", jm.OldSubject, jm.NewSubject, jm.Paths, j, s)
		}
	})
	if step == gate+".judge.a1" {
		t.Fatalf("the second judge reused %s", step)
	}
	e.ok("submit", step, "--lease", token, "--record", path)
	withdrawn := e.gateRound(gate)
	if withdrawn.Round != 1 || withdrawn.Verify != a.Step+".a2" || withdrawn.Subject != first.Subject {
		t.Fatalf("round after the withdraw %+v, want round 1 on %s with verify %s.a2", withdrawn, first.Subject, a.Step)
	}
	a2 := e.refuteAll()
	if a2.Step != a.Step+".a2" || a2.Subject.Commit != s {
		t.Fatalf("verify %s on %s, want %s.a2 on %s", a2.Step, a2.Subject.Commit, a.Step, s)
	}
	out := e.apply()
	e.absent(a.Step+".a3", "the withdraw opened more than one verify attempt")
	if e.row(gate).Status == "closed" {
		t.Fatalf("the gate closed after a2: %+v", e.receipt(gate))
	}
	again := e.gateRound(gate)
	if again.Round != 2 || again.Subject != s || again.SlotCount != 3 || !reflect.DeepEqual(again.Paths, []string{"later.txt"}) || again.Prior == nil || again.Prior.Verify != a2.Step {
		t.Fatalf("round after a2 %+v, want round 2 of 3 slots on %s scoped to later.txt with prior verify %s", again, s, a2.Step)
	}
	if again.Lens == "" || again.Lens == two.Lens {
		t.Fatalf("round 2 lens %q, want a fresh attempt after %q", again.Lens, two.Lens)
	}
	e.create(out)
	e.reviewSlots("APPROVE", 3)
	e.apply()
	if r := e.receipt(gate); r.Kind != "passed" || r.Subject != s {
		t.Fatalf("gate receipt %+v, want passed on %s", r, s)
	}
}

// AC20: with the round-2 fix open after an applied verify, a fix commit and
// then a panel refute of the reproduced finding: no judge and no verify
// attempt a2 while the gate is open; the gate closes passed on round 1's
// review subject, the fix closes superseded, and the next pass opens judge a1
// on the passed gate.
func TestS134E2E_CriterionFixCommitThenRefute(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	f, fix := e.frozenFix(gate, nil)
	round := e.gateRound(gate)
	moved := e.commit(e.tree(item), "fix-work.txt", "work\n")
	e.apply()
	e.absent(gate+".judge.a1", "a fix commit gave a judge on the open gate")
	e.absent(round.Verify+".a2", "a fix commit gave a second verify")
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	e.ok("panel", "refute", reg.Slug, "--spec", e.f.SpecID, "--slot", f.Slot, "--round", "1", "--finding", f.Finding, "--by", "operator", "--ground", "disproven", "--reason", "The subject covers the finding.", "--evidence", e.commit(e.proof, "refute-evidence.txt", "evidence\n"))
	e.apply()
	e.absent(round.Verify+".a2", "the refute gave a second verify")
	if r := e.receipt(gate); r.Kind != "passed" || r.Subject != round.Subject {
		t.Fatalf("gate receipt %+v, want passed on round 1's subject %s", r, round.Subject)
	}
	if r := e.receipt(fix); r.Kind != "superseded" {
		t.Fatalf("fix receipt %+v, want superseded", r)
	}
	e.apply()
	judge := e.row(gate + ".judge.a1")
	sub, _, err := kernel.Decode[kernel.Subject](judge.Metadata, kernel.KeySubject)
	e.must(err)
	if judge.Status != "open" || sub.Commit != moved {
		t.Fatalf("judge %s on %s, want open on %s", judge.Status, sub.Commit, moved)
	}
}

// editRoundOne runs git gc --prune=now and then edits each round-1 verdict
// file of the gate on disk, with no commit. A file that is gone is skipped.
func (e *kernelE2E) editRoundOne(gate string) {
	e.t.Helper()
	e.git(e.cwd, "gc", "--prune=now", "-q")
	for _, slot := range []string{"R1", "R2", "R3"} {
		name := filepath.Join(e.cwd, e.panelFile(gate, slot+"-round-1.json"))
		data, err := os.ReadFile(name)
		if os.IsNotExist(err) {
			continue
		}
		e.must(err)
		var doc map[string]any
		e.must(json.Unmarshal(data, &doc))
		doc["verdict"] = "REQUEST_CHANGES"
		doc["rationale"] = "Edited on disk after the gc."
		doc["concrete_changes_required"] = []any{map[string]any{"text": "Edited finding on disk after the gc.", "basis": "traced"}}
		edited, err := json.Marshal(doc)
		e.must(err)
		e.must(os.WriteFile(name, edited, 0600))
	}
}

// AC20: after git gc --prune=now and an edit of each round-1 verdict file on
// disk, the round-2 slot body is unchanged: before panel create 2 the body
// shows the findings and returns of the verify record and no edited text, and
// after panel create 2 a second gc and edit leave each slot body as it was.
func TestS134E2E_CriterionRoundTwoBodyAfterGC(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.refusedRound("REQUEST_CHANGES", "APPROVE", "APPROVE")
	e.verifyReproduced()
	findings, _, err := kernel.Decode[[]kernel.Finding](e.row(gate+".r1.verify").Metadata, kernel.KeyFindings)
	e.must(err)
	if len(findings) != 1 || findings[0].Text == "" {
		t.Fatalf("verify findings %+v, want one finding with text", findings)
	}
	fix := gate + ".r2.fix"
	e.ok("submit", fix, "--lease", e.lease(fix), "--commit", e.commit(e.tree(item), "fixed.txt", "fixed\n"))
	next := e.draftLenses(e.apply())
	if len(next.Run) == 0 || strings.Join(next.Run[0].Argv, " ") != "mindspec panel create "+item {
		t.Fatalf("no round-2 panel create: %+v", next)
	}
	e.editRoundOne(gate)
	e.ok(next.Run[0].Argv[1:]...)
	if r := e.gateRound(gate); r.Round != 2 {
		t.Fatalf("round %+v, want round 2", r)
	}
	before := map[string]string{}
	for _, slot := range []string{"R1", "R2", "R3"} {
		body := e.slotPrompt(gate, slot)
		at := strings.Index(body, "Finding: R1 round 1 "+findings[0].Finding)
		if at < 0 || !strings.Contains(body[at:], "Text: "+findings[0].Text+"\n") || !strings.Contains(body[at:], "Return: reproduced") {
			t.Fatalf("the %s round-2 body must show the verify's finding and return:\n%s", slot, body)
		}
		if strings.Contains(body, "Edited") {
			t.Fatalf("the %s round-2 body shows text from the edited files:\n%s", slot, body)
		}
		before[slot] = body
	}
	e.editRoundOne(gate)
	for _, slot := range []string{"R1", "R2", "R3"} {
		if after := e.slotPrompt(gate, slot); after != before[slot] {
			t.Fatalf("the %s round-2 body changed after the gc and the edits\nbefore:\n%s\nafter:\n%s", slot, before[slot], after)
		}
	}
}

// AC20: round refused, a commit J and a carry-over: the verify subject is J.
// After a reproduced return, the fix submit of J exits non-zero and leaves the
// fix open, and a commit past J exits 0.
func TestS134E2E_CriterionFixPastCarriedSubject(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.refusedRound("REQUEST_CHANGES", "APPROVE", "APPROVE")
	j := e.commit(e.tree(item), "carried.txt", "J\n")
	e.judge("carry-over", 0)
	a, findings := e.verifyAction()
	if a.Subject.Commit != j {
		t.Fatalf("verify %s on %s, want the new subject %s", a.Step, a.Subject.Commit, j)
	}
	returns := []kernel.VerifyReturn{}
	for _, f := range findings {
		returns = append(returns, returnFor(f, "reproduced", ""))
	}
	e.ok("submit", a.Step, "--lease", e.lease(a.Step), "--record", e.jsonFile("verify.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: returns}))
	fix := gate + ".r2.fix"
	token := e.lease(fix)
	out, code := e.run("submit", fix, "--lease", token, "--commit", j)
	if code == 0 {
		t.Fatalf("the fix submit of the carried subject exited 0:\n%s", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if strings.Count(out, "recovery:") != 1 || !strings.HasPrefix(lines[len(lines)-1], "recovery:") {
		t.Fatalf("the refusal must end with one recovery line:\n%s", out)
	}
	if r := e.row(fix); r.Status == "closed" {
		t.Fatal("a refused fix submit closed the fix")
	}
	e.ok("submit", fix, "--lease", token, "--commit", e.commit(e.tree(item), "fixed.txt", "fixed\n"))
	if r := e.row(fix); r.Status != "closed" {
		t.Fatalf("fix %s, want closed", r.Status)
	}
}

// verifyRecordFile writes a verify record for the action that gives each
// finding the word at the same index, and returns its path.
func (e *kernelE2E) verifyRecordFile(a kernel.Action, findings []kernel.Finding, words ...string) string {
	e.t.Helper()
	returns := []kernel.VerifyReturn{}
	for i, f := range findings {
		returns = append(returns, returnFor(f, words[i], a.Subject.Commit))
	}
	return e.jsonFile("verify.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: returns})
}

// AC20: verify a1 refutes a finding and closes, then a commit and a
// carry-over: the round's verify is a2, and a2 reproduces the finding. The
// refutation of a1 discharges nothing: the gate stays open and the fix is
// created. A later verb refutation of the other finding leaves the gate and
// the fix open, because no kept refutation covers the first one.
func TestS134E2E_CriterionStaleVerifyRefutation(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.refusedRound("REQUEST_CHANGES", "REQUEST_CHANGES", "APPROVE")
	a, findings := e.verifyAction()
	if len(findings) != 2 {
		t.Fatalf("verify findings %+v, want two", findings)
	}
	token := e.lease(a.Step)
	moved := e.commit(e.tree(item), "moved.txt", "moved\n")
	e.ok("submit", a.Step, "--lease", token, "--record", e.verifyRecordFile(a, findings, "refuted", "reproduced"))
	if r := e.receipt(a.Step); r.Kind != "passed" {
		t.Fatalf("a1 receipt %+v, want passed", r)
	}
	e.absent(gate+".r2.fix", "a verify on a moved subject gave a fix before the judge")
	e.judge("carry-over", 0)
	e.apply()
	if r := e.gateRound(gate); r.Verify != a.Step+".a2" {
		t.Fatalf("round verify %q, want %s.a2", r.Verify, a.Step)
	}
	a2, again := e.verifyAction()
	if a2.Step != a.Step+".a2" || a2.Subject.Commit != moved || len(again) != 2 {
		t.Fatalf("verify %s on %s with %d findings, want %s.a2 on %s with two", a2.Step, a2.Subject.Commit, len(again), a.Step, moved)
	}
	e.ok("submit", a2.Step, "--lease", e.lease(a2.Step), "--record", e.verifyRecordFile(a2, again, "reproduced", "reproduced"))
	e.apply()
	fix := gate + ".r2.fix"
	reproduced, ok, err := kernel.Decode[[]kernel.Finding](e.row(fix).Metadata, kernel.KeyReproduced)
	e.must(err)
	if !ok || len(reproduced) != 2 || e.row(fix).Status == "closed" {
		t.Fatalf("fix %s reproduced %+v, want open with both findings", e.row(fix).Status, reproduced)
	}
	if e.row(gate).Status == "closed" {
		t.Fatalf("the gate closed on a1's refutation: %+v", e.receipt(gate))
	}
	g := again[1]
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	e.ok("panel", "refute", reg.Slug, "--spec", e.f.SpecID, "--slot", g.Slot, "--round", "1", "--finding", g.Finding, "--by", "operator", "--ground", "disproven", "--reason", "The subject covers the finding.", "--evidence", e.commit(e.proof, "refute-evidence.txt", "evidence\n"))
	e.apply()
	if e.row(gate).Status == "closed" {
		t.Fatalf("a1's refutation discharged the first finding: %+v", e.receipt(gate))
	}
	if e.row(fix).Status == "closed" {
		t.Fatalf("a1's refutation closed the fix: %+v", e.receipt(fix))
	}
}

// AC21: a verify leased by generation 1 and taken by generation 2; after the
// generation-2 receipt, a generation-1 submit exits non-zero with one
// recovery line, last, writes no closing conflict and changes no receipt, and
// the next pass creates no verify attempt a2.
func TestS134E2E_CriterionVerifyStaleGenerationNoConflict(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	e.refusedRound("REQUEST_CHANGES", "APPROVE", "APPROVE")
	a, findings := e.verifyAction()
	first := e.lease(a.Step)
	second := e.lease(a.Step, "--take", "--reason", "Replace the verify actor.")
	e.ok("submit", a.Step, "--lease", second, "--record", e.verifyRecordFile(a, findings, "reproduced"))
	receipt := e.receipt(a.Step)
	if receipt.Generation != 2 {
		t.Fatalf("receipt %+v, want generation 2", receipt)
	}
	out, code := e.run("submit", a.Step, "--lease", first, "--record", e.verifyRecordFile(a, findings, "refuted"))
	if code == 0 {
		t.Fatalf("a generation-1 submit after the generation-2 receipt exited 0:\n%s", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if strings.Count(out, "recovery:") != 1 || !strings.HasPrefix(lines[len(lines)-1], "recovery:") {
		t.Fatalf("the refusal must end with one recovery line:\n%s", out)
	}
	pin, has, err := kernel.Decode[kernel.Closing](e.row(a.Step).Metadata, kernel.KeyClosing)
	e.must(err)
	if !has || pin.Conflict != nil || pin.Generation != 2 {
		t.Fatalf("closing pin %+v (present %v), want generation 2 with no conflict", pin, has)
	}
	if after := e.receipt(a.Step); !reflect.DeepEqual(after, receipt) {
		t.Fatalf("receipt changed from %+v to %+v", receipt, after)
	}
	e.apply()
	e.absent(a.Step+".a2", "a refused generation-1 submit gave a second verify")
	if r := e.row(gate + ".r2.fix"); r.Status != "open" {
		t.Fatalf("fix %s, want open from the generation-2 record", r.Status)
	}
}
