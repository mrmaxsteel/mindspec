package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

const bodyReason = "The moved subject covers the finding."

// movedRefusedRound reviews the first work item on a subject that moves
// during the round: R1 asks for changes and R2 and R3 approve. It returns the
// moved subject.
func (e *kernelE2E) movedRefusedRound() string {
	e.t.Helper()
	e.apply()
	e.create(e.implement(0))
	moved := e.commit(e.tree(e.f.Items[0]), "during-round.txt", "moved\n")
	slots := e.slotActions()
	if len(slots) != 3 {
		e.t.Fatalf("slots: %+v", slots)
	}
	for i, word := range []string{"REQUEST_CHANGES", "APPROVE", "APPROVE"} {
		e.submitSlot(slots[i], word)
	}
	return moved
}

// verbRefute runs panel refute of one round-1 finding with evidence in the
// proof repository.
func (e *kernelE2E) verbRefute(gate, slot, finding string) {
	e.t.Helper()
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	e.ok("panel", "refute", reg.Slug, "--spec", e.f.SpecID, "--slot", slot, "--round", "1", "--finding", finding, "--by", "operator", "--ground", "disproven", "--reason", bodyReason, "--evidence", e.commit(e.proof, "refute-"+slot+".txt", "evidence\n"))
}

// createRoundTwo drafts the round-2 lens and runs the printed panel create.
func (e *kernelE2E) createRoundTwo(gate string) {
	e.t.Helper()
	next := e.draftLenses(e.apply())
	if len(next.Run) == 0 || strings.Join(next.Run[0].Argv, " ") != "mindspec panel create "+e.f.Items[0] {
		e.t.Fatalf("no round-2 panel create: %+v", next)
	}
	e.ok(next.Run[0].Argv[1:]...)
	if r := e.gateRound(gate); r.Round != 2 {
		e.t.Fatalf("round %+v, want round 2", r)
	}
}

// bodyShows checks that each round-2 slot body shows the round-1 finding of
// R1 and, after it, each wanted line.
func (e *kernelE2E) bodyShows(gate string, slots int, finding string, want ...string) {
	e.t.Helper()
	for i := 1; i <= slots; i++ {
		slot := "R" + string(rune('0'+i))
		body := e.slotPrompt(gate, slot)
		at := strings.Index(body, "Finding: R1 round 1 "+finding+"\n")
		if at < 0 {
			e.t.Fatalf("the %s round-2 body does not show the round-1 finding %s:\n%s", slot, finding, body)
		}
		for _, line := range want {
			if !strings.Contains(body[at:], line+"\n") {
				e.t.Fatalf("the %s round-2 body does not show %q after the finding:\n%s", slot, line, body)
			}
		}
	}
}

// AC20: open gate, round 1 refused on a moved subject, re-panel 2, the verify
// open on the new subject, then panel refute of each finding: the verify
// closes superseded, round 2 of 2 slots is authorised with prior round 1 and
// no prior verify, the gate stays open, and the round-2 slot body lists each
// round-1 finding with its refutation.
func TestS134E2E_CriterionRoundTwoBodyVerbRefuted(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	moved := e.movedRefusedRound()
	e.judge("re-panel", 2)
	a, findings := e.verifyAction()
	if a.Subject.Commit != moved || len(findings) != 1 {
		t.Fatalf("verify %s on %s with %+v, want one finding on %s", a.Step, a.Subject.Commit, findings, moved)
	}
	for _, f := range findings {
		e.verbRefute(gate, f.Slot, f.Finding)
	}
	e.apply()
	if r := e.receipt(a.Step); r.Kind != "superseded" {
		t.Fatalf("verify receipt %+v, want superseded", r)
	}
	if e.row(gate).Status == "closed" {
		t.Fatalf("the retally closed the gate: %+v", e.receipt(gate))
	}
	round := e.gateRound(gate)
	if round.Round != 2 || round.SlotCount != 2 || round.Prior == nil || round.Prior.Round != 1 || round.Prior.Verify != "" {
		t.Fatalf("round %+v, want round 2 of 2 slots with prior round 1 and no prior verify", round)
	}
	e.createRoundTwo(gate)
	for _, f := range findings {
		e.bodyShows(gate, 2, f.Finding, "Return: refuted", "Reason: "+bodyReason)
	}
}

// AC20: the same re-panel 2, the verify reproduces f and the fix is open, then
// panel refute of f: the fix closes superseded, round 2 of 2 slots is
// authorised with prior verify that verify, and the round-2 slot body shows f
// with its return reproduced and the refutation.
func TestS134E2E_CriterionRoundTwoBodyReproducedRefuted(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	moved := e.movedRefusedRound()
	e.judge("re-panel", 2)
	a, findings := e.verifyAction()
	if a.Subject.Commit != moved || len(findings) != 1 {
		t.Fatalf("verify %s on %s with %+v, want one finding on %s", a.Step, a.Subject.Commit, findings, moved)
	}
	e.ok("submit", a.Step, "--lease", e.lease(a.Step), "--record", e.verifyRecordFile(a, findings, "reproduced"))
	e.apply()
	fix := gate + ".r2.fix"
	if r := e.row(fix); r.Status != "open" {
		t.Fatalf("fix %s is %s, want open", fix, r.Status)
	}
	f := findings[0]
	e.verbRefute(gate, f.Slot, f.Finding)
	e.apply()
	if r := e.receipt(fix); r.Kind != "superseded" {
		t.Fatalf("fix receipt %+v, want superseded", r)
	}
	if e.row(gate).Status == "closed" {
		t.Fatalf("the retally closed the gate: %+v", e.receipt(gate))
	}
	round := e.gateRound(gate)
	if round.Round != 2 || round.SlotCount != 2 || round.Prior == nil || round.Prior.Verify != a.Step {
		t.Fatalf("round %+v, want round 2 of 2 slots with prior verify %s", round, a.Step)
	}
	e.createRoundTwo(gate)
	e.bodyShows(gate, 2, f.Finding, "Return: reproduced", "Return: refuted", "Reason: "+bodyReason)
}

// AC20: open gate, round 1 refused on a moved subject, panel refute of its
// finding while the judge is open, re-panel 2, then a commit replaces that
// round-1 verdict with APPROVE before panel create 2: the round-2 slot body
// shows the round-1 finding with its refutation.
func TestS134E2E_CriterionRoundTwoBodyVerdictReplaced(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	e.movedRefusedRound()
	e.apply()
	if r := e.row(gate + ".judge.a1"); r.Status != "open" {
		t.Fatalf("judge is %s, want open", r.Status)
	}
	e.verbRefute(gate, "R1", "ccr[0]")
	e.judge("re-panel", 2)
	e.apply()
	if e.row(gate).Status == "closed" {
		t.Fatalf("the re-panel closed the gate: %+v", e.receipt(gate))
	}
	round := e.gateRound(gate)
	if round.Round != 2 || round.SlotCount != 2 || round.Prior == nil || round.Prior.Round != 1 {
		t.Fatalf("round %+v, want round 2 of 2 slots with prior round 1", round)
	}
	name := e.panelFile(gate, "R1-round-1.json")
	data, err := os.ReadFile(filepath.Join(e.cwd, name))
	e.must(err)
	var doc map[string]any
	e.must(json.Unmarshal(data, &doc))
	doc["verdict"] = "APPROVE"
	doc["concrete_changes_required"] = []any{}
	doc["rationale"] = "Replaced with an approval."
	replaced, err := json.Marshal(doc)
	e.must(err)
	e.commit(e.cwd, name, string(replaced))
	e.createRoundTwo(gate)
	e.bodyShows(gate, 2, "ccr[0]", "Return: refuted", "Reason: "+bodyReason)
}

// repoSnapshot is the git status of the root, spec, item and proof trees, and the
// subject digest: the item tree's HEAD commit, HEAD tree and index entries.
func (e *kernelE2E) repoSnapshot() string {
	e.t.Helper()
	tree := e.tree(e.f.Items[0])
	var b strings.Builder
	for _, root := range []string{e.f.Root, e.cwd, tree, e.proof} {
		b.WriteString(root + "\n" + e.git(root, "status", "--porcelain", "--untracked-files=all") + "\n")
	}
	b.WriteString(e.git(tree, "rev-parse", "HEAD", "HEAD^{tree}") + "\n")
	b.WriteString(e.git(tree, "ls-files", "-s") + "\n")
	b.WriteString(e.git(e.cwd, "rev-parse", "HEAD", "HEAD^{tree}") + "\n")
	return b.String()
}

// AC20: lens and verify submit leave git status --porcelain and the subject
// digest unchanged.
func TestS134E2E_CriterionLensVerifySubmitNoTreeChange(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	e.apply()
	out := e.implement(0)
	var lens kernel.Action
	for _, a := range out.Actions {
		if a.Kind == "lens" {
			lens = a
		}
	}
	if lens.Step == "" {
		t.Fatalf("no lens step: %+v", out)
	}
	token := e.lease(lens.Step)
	file := e.lensFile(e.lensRecord(lens))
	before := e.repoSnapshot()
	raw := e.ok("submit", lens.Step, "--lease", token, "--record", file)
	if after := e.repoSnapshot(); after != before {
		t.Fatalf("lens submit changed the trees\nbefore:\n%s\nafter:\n%s", before, after)
	}
	e.must(json.Unmarshal([]byte(raw), &out))
	if len(out.Run) == 0 {
		t.Fatalf("no panel create after the lens: %+v", out)
	}
	e.ok(out.Run[0].Argv[1:]...)
	for i, a := range e.slotActions() {
		word := "APPROVE"
		if i == 0 {
			word = "REQUEST_CHANGES"
		}
		e.submitSlot(a, word)
	}
	a, findings := e.verifyAction()
	token = e.lease(a.Step)
	record := e.verifyRecordFile(a, findings, "reproduced")
	before = e.repoSnapshot()
	e.ok("submit", a.Step, "--lease", token, "--record", record)
	if after := e.repoSnapshot(); after != before {
		t.Fatalf("verify submit changed the trees\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if r := e.receipt(a.Step); r.Kind != "passed" {
		t.Fatalf("verify receipt %+v, want passed", r)
	}
}

// lensAction returns the lens action of a pass, or fails.
func (e *kernelE2E) lensAction(out kernel.Output) kernel.Action {
	e.t.Helper()
	for _, a := range out.Actions {
		if a.Kind == "lens" {
			return a
		}
	}
	e.t.Fatalf("no lens step: %+v", out)
	return kernel.Action{}
}

// passedRepanelThenCommit passes the first gate, commits, judges a re-panel of
// first slots, and, when lensPassed, closes its lens step. Then an ordinary
// commit before panel create: the pass authorises no review slot or panel
// create, opens exactly one judge, on the newest commit, whose old subject is
// the accepted subject, and leaves the a2 gate and its lens step as they are.
// The pass that applies the judge's re-panel of second slots closes the a2
// gate superseded, closes an open lens step superseded or keeps the receipt
// of a passed one and retires it, and creates the a3 gate and lens attempt
// a2, which its round names; the lens accepts only the entries of second
// slots. It returns the a3 gate.
func (e *kernelE2E) passedRepanelThenCommit(first, second int, lensPassed bool) string {
	e.t.Helper()
	item := e.f.Items[0]
	gate := item + ".review"
	e.apply()
	e.create(e.implement(0))
	e.reviewSlots("APPROVE", 3)
	accepted := e.receipt(gate).Subject
	if accepted == "" {
		e.t.Fatalf("gate receipt %+v, want passed", e.receipt(gate))
	}
	e.commit(e.tree(item), "scoped.txt", "scoped\n")
	e.judge("re-panel", first)
	a2 := gate + ".a2"
	lens := e.lensAction(e.apply())
	if lensPassed {
		e.ok("submit", lens.Step, "--lease", e.lease(lens.Step), "--record", e.lensFile(e.lensRecord(lens)))
		if r := e.receipt(lens.Step); r.Kind != "passed" {
			e.t.Fatalf("lens receipt %+v, want passed", r)
		}
	}
	later := e.commit(e.tree(item), "later.txt", "later\n")
	out := e.apply()
	var judges []kernel.Action
	for _, a := range out.Actions {
		if a.Kind == "review-slot" {
			e.t.Fatalf("an ordinary commit authorised a round: %+v", out)
		}
		if a.Kind == "change-judge" {
			judges = append(judges, a)
		}
	}
	for _, r := range out.Run {
		if len(r.Argv) > 2 && r.Argv[1] == "panel" {
			e.t.Fatalf("an ordinary commit authorised a round: %+v", out)
		}
	}
	if len(judges) != 1 || judges[0].Subject.Commit != later {
		e.t.Fatalf("judges %+v, want exactly one on the newest commit %s: %+v", judges, later, out)
	}
	var open []string
	for id, r := range e.f.State().Records {
		if t, _, err := r.Tuple(); err == nil && t.Kind == "change-judge" && r.Status != "closed" {
			open = append(open, id)
		}
	}
	if len(open) != 1 || open[0] != judges[0].Step {
		e.t.Fatalf("open judges %v, want only %s", open, judges[0].Step)
	}
	// The commit pass leaves the scoped round as it is: the judge rules first.
	e.unclosed(a2, "the commit pass")
	if r := e.gateRound(a2); r.Lens != lens.Step || r.SlotCount != first {
		e.t.Fatalf("after the commit pass, %s round %+v, want lens %s and %d slots", a2, r, lens.Step, first)
	}
	if lensPassed {
		if r := e.receipt(lens.Step); r.Kind != "passed" {
			e.t.Fatalf("after the commit pass, lens %s receipt %+v, want passed", lens.Step, r)
		}
	} else {
		e.unclosed(lens.Step, "the commit pass")
	}
	a3 := gate + ".a3"
	e.absent(a3, "an ordinary commit gave a new gate before the judge")
	e.judgeBriefRange(judges[0], accepted, later, "scoped.txt", "later.txt")
	step, token, path := e.judgement("re-panel", second, func(j *kernel.Judgement) {
		if j.OldSubject != accepted || j.NewSubject != later {
			e.t.Fatalf("judge %s..%s, want %s..%s", j.OldSubject, j.NewSubject, accepted, later)
		}
	})
	e.ok("submit", step, "--lease", token, "--record", path)
	// The pass that applies the re-panel retires the scoped round.
	if r := e.receipt(a2); r.Kind != "superseded" {
		e.t.Fatalf("%s receipt %+v, want superseded", a2, r)
	}
	want := "superseded"
	if lensPassed {
		want = "passed"
	}
	if r := e.receipt(lens.Step); r.Kind != want {
		e.t.Fatalf("lens %s receipt %+v, want %s", lens.Step, r, want)
	}
	round := e.gateRound(a3)
	if round.Lens != lens.Step+".a2" || round.SlotCount != second {
		e.t.Fatalf("when the re-panel is applied, %s round %+v, want lens %s.a2 and %d slots", a3, round, lens.Step, second)
	}
	e.unclosed(round.Lens, "the re-panel pass")
	next := e.lensAction(e.apply())
	round = e.gateRound(a3)
	if next.Step != lens.Step+".a2" || round.Lens != next.Step || round.SlotCount != second {
		e.t.Fatalf("lens %s, round %+v, want lens %s.a2 named by a round of %d slots", next.Step, round, lens.Step, second)
	}
	if r := e.row(a3); r.Status == "closed" {
		e.t.Fatalf("%s is closed", a3)
	}
	record := e.lensRecord(next)
	if len(record.Lenses) != second {
		e.t.Fatalf("lens entries %+v, want %d", record.Lenses, second)
	}
	token = e.lease(next.Step)
	if first != second {
		short := kernel.LensRecord{Step: next.Step, Lenses: record.Lenses[:first]}
		if out, code := e.run("submit", next.Step, "--lease", token, "--record", e.lensFile(short)); code == 0 {
			e.t.Fatalf("a lens of %d entries was accepted for %d slots:\n%s", first, second, out)
		}
	}
	e.ok("submit", next.Step, "--lease", token, "--record", e.lensFile(record))
	return a3
}

// AC20: after a passed-gate re-panel and before panel create, an ordinary
// commit: the pass creates a judge on the newest commit whose diff starts at
// the accepted subject, authorises no round, and leaves the a2 gate and its
// open lens step as they are; when the judge's re-panel with another slot
// count is applied, the pass closes both superseded and creates the a3 gate
// and lens attempt a2 with that count's entries.
func TestS134E2E_CriterionPassedRepanelOrdinaryCommit(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	e.passedRepanelThenCommit(2, 3, false)
}

// AC20: as the test above, but the lens step of the a2 gate had closed passed:
// the commit pass leaves it as it is, and when the re-panel is applied it
// keeps its receipt and is retired.
func TestS134E2E_CriterionPassedRepanelPassedLens(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	e.passedRepanelThenCommit(2, 2, true)
}

// unclosed fails when the record id is closed or holds a close receipt.
func (e *kernelE2E) unclosed(id, when string) {
	e.t.Helper()
	r := e.row(id)
	_, ok, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	e.must(err)
	if r.Status == "closed" || ok {
		e.t.Fatalf("after %s, %s is %s (receipt %v), want open with no receipt", when, id, r.Status, ok)
	}
}

// noNewJudge fails when the pass offers a judge or the store holds a judge
// attempt after the closed judge a1 of the gate.
func (e *kernelE2E) noNewJudge(out kernel.Output, gate, when string) {
	e.t.Helper()
	for _, a := range out.Actions {
		if a.Kind == "change-judge" {
			e.t.Fatalf("%s gave a judge after a closed judgement: %+v", when, out)
		}
	}
	e.absent(gate+".judge.a2", when+" gave a second judge")
}

// AC20: no judge follows a closed judgement on the passed-gate path: after a
// re-panel, the scoped round's authorisation, its passing tally and complete
// create no judge.
func TestS134E2E_CriterionNoJudgeAfterPassedRepanel(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.apply()
	e.create(e.implement(0))
	e.reviewSlots("APPROVE", 3)
	scoped := e.commit(e.tree(item), "scoped.txt", "scoped\n")
	e.judge("re-panel", 2)
	out := e.apply()
	e.noNewJudge(out, gate, "the scoped round's authorisation")
	lens := e.lensAction(out)
	e.ok("submit", lens.Step, "--lease", e.lease(lens.Step), "--record", e.lensFile(e.lensRecord(lens)))
	out = e.apply()
	e.noNewJudge(out, gate, "the pass after the lens")
	e.create(out)
	e.reviewSlots("APPROVE", 2)
	e.noNewJudge(e.apply(), gate, "the scoped round's passing tally")
	if r := e.receipt(gate + ".a2"); r.Kind != "passed" || r.Subject != scoped {
		t.Fatalf("scoped gate receipt %+v, want passed on %s", r, scoped)
	}
	e.ok("complete", item)
	e.noNewJudge(e.apply(), gate, "complete")
}

// AC20: no judge follows a closed judgement on the open-gate path: a refused
// tally on a moved subject creates the judge and no verify until it closes;
// after its re-panel, the verify retally with no reproduced return, the scoped
// round 2's authorisation and its tally create no judge.
func TestS134E2E_CriterionNoJudgeAfterOpenRepanel(t *testing.T) {
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
	out := e.apply()
	judged := false
	for _, a := range out.Actions {
		if a.Kind == "verify" {
			t.Fatalf("a verify before the judge closed: %+v", out)
		}
		judged = judged || a.Kind == "change-judge"
	}
	if !judged {
		t.Fatalf("a refused tally on a moved subject gave no judge: %+v", out)
	}
	e.absent(gate+".r1.verify", "a refused tally on a moved subject gave a verify before the judge closed")
	e.judge("re-panel", 2)
	e.refuteAll()
	out = e.apply()
	e.noNewJudge(out, gate, "the verify retally")
	if round := e.gateRound(gate); round.Round != 2 || round.Subject != j {
		t.Fatalf("round %+v, want round 2 on %s", round, j)
	}
	lens := e.lensAction(out)
	e.ok("submit", lens.Step, "--lease", e.lease(lens.Step), "--record", e.lensFile(e.lensRecord(lens)))
	out = e.apply()
	e.noNewJudge(out, gate, "the scoped round 2's authorisation")
	e.create(out)
	e.reviewSlots("APPROVE", 2)
	e.noNewJudge(e.apply(), gate, "the round 2 tally")
	if r := e.receipt(gate); r.Kind != "passed" || r.Subject != j {
		t.Fatalf("gate receipt %+v, want passed on %s", r, j)
	}
}

// judgeBriefRange runs the change judge's printed brief command through the
// binary and checks that the brief starts at the accepted subject, ends at the
// newest subject, and that its changed paths and diff hold every named file
// changed since the accepted subject, with the other judge brief fields.
func (e *kernelE2E) judgeBriefRange(judge kernel.Action, accepted, newest string, files ...string) {
	e.t.Helper()
	if len(judge.Brief) < 2 {
		e.t.Fatalf("judge %s prints no brief command: %+v", judge.Step, judge)
	}
	body := e.ok(judge.Brief[1:]...)
	want := []string{
		"## change-judge\n",
		"Step: " + judge.Step + "\n",
		"Subject: " + newest + "\n",
		"Accepted subject: " + accepted + "\n",
		"New subject: " + newest + "\n",
		"Slot ceiling: ",
		"## Changed paths\n",
		"## Diff\n",
		"Diff text: ",
		"Return carry-over with a reason, or re-panel with slot_count and paths covering every changed path.",
	}
	for _, f := range files {
		want = append(want, "- Path: "+f+"\n")
	}
	for _, w := range want {
		if !strings.Contains(body, w) {
			e.t.Fatalf("judge brief lacks %q (accepted %s, newest %s):\n%s", w, accepted, newest, body)
		}
	}
	_, diff, _ := strings.Cut(body, "Diff text: ")
	for _, f := range files {
		if !strings.Contains(diff, "b/"+f) {
			e.t.Fatalf("judge brief diff lacks the change to %s (accepted %s, newest %s):\n%s", f, accepted, newest, body)
		}
	}
}
