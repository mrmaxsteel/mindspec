package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// kernelE2EFiveSlotConfig has five correctness slots, so R2 and R5 can be
// uncounted while R1, R3 and R4 are counted.
const kernelE2EFiveSlotConfig = "enforcement:\n  panel_gate: false\nauto_open_finalize_pr: false\npanel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: correctness\n      count: 5\n"

// attempts runs a pass and returns its slot steps. The steps must be exactly
// the ones named, and the pass must print no verify, fix, judge or decision.
func (e *kernelE2E) attempts(want ...string) []kernel.Action {
	e.t.Helper()
	out := e.apply()
	var slots []kernel.Action
	var got []string
	for _, a := range out.Actions {
		switch a.Kind {
		case "verify", "fix", "change-judge":
			e.t.Fatalf("the pass gave %s, want only the slot attempts %v: %+v", a.Step, want, out)
		case "review-slot":
			slots = append(slots, a)
			got = append(got, a.Step)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") || len(out.Decisions) != 0 {
		e.t.Fatalf("slot attempts %v and decisions %+v, want only %v", got, out.Decisions, want)
	}
	return slots
}

// uncountedDecision submits round 1 of the first item on the five-slot roster
// with R2 and R5 VOID on attempts 1, 2 and 3 and the other slots APPROVE. It
// checks that the pass then gives no slot attempt and one decision that lists
// R2 and R5 at attempt 3, and returns the decision.
func (e *kernelE2E) uncountedDecision(gate string) string {
	e.t.Helper()
	e.refusedRound("APPROVE", "VOID", "APPROVE", "APPROVE", "VOID")
	for _, attempt := range []string{"a2", "a3"} {
		for _, a := range e.attempts(gate+".r1.R2."+attempt, gate+".r1.R5."+attempt) {
			e.submitAttempt(a, "VOID")
		}
	}
	decision := gate + ".decide.uncounted.r1"
	out := e.apply()
	for _, a := range out.Actions {
		if a.Kind == "review-slot" || a.Kind == "verify" || a.Kind == "fix" {
			e.t.Fatalf("attempt 3 VOID gave %s, want only the decision: %+v", a.Step, out)
		}
	}
	if len(out.Decisions) != 1 || out.Decisions[0].Step != decision {
		e.t.Fatalf("decisions %+v, want only %s", out.Decisions, decision)
	}
	record, _, err := kernel.Decode[kernel.DecisionRecord](e.row(decision).Metadata, kernel.KeyDecision)
	e.must(err)
	if len(record.Slots) != 2 || record.Slots[0] != (kernel.DecisionSlot{Slot: "R2", Attempt: 3}) || record.Slots[1] != (kernel.DecisionSlot{Slot: "R5", Attempt: 3}) {
		e.t.Fatalf("decision slots %+v, want R2 and R5 at attempt 3", record.Slots)
	}
	return decision
}

// absent fails if the store holds the record.
func (e *kernelE2E) absent(id, why string) {
	e.t.Helper()
	r, err := e.f.Store.Lookup(id)
	e.must(err)
	if r != nil {
		e.t.Fatalf("%s: %s exists with status %s", why, id, r.Status)
	}
}

// AC19: slots R2 and R5 VOID on attempts 1, 2 and 3 with R1, R3 and R4
// APPROVE give one uncounted-slot decision that lists R2 and R5 at attempt 3.
// While it is open, a commit edits R3's closed verdict file: no R3 attempt.
// Redispatch gives attempt 4 of R2 and R5 only; VOID on both gives attempt 5
// of R2 and R5 and attempt 2 of R3, not a decision.
func TestS134E2E_CriterionUncountedDecisionTwoSlots(t *testing.T) {
	t.Parallel()
	e := newKernelE2EWithConfig(t, kernelE2EBinary(t), kerneltest.EmptyRepo(t), ".mindspec", kernelE2EFiveSlotConfig, 1)
	gate := e.f.Items[0] + ".review"
	decision := e.uncountedDecision(gate)
	name := e.panelFile(gate, "R3-round-1.json")
	data, err := os.ReadFile(filepath.Join(e.cwd, name))
	if err != nil {
		t.Fatalf("R3's closed verdict file is not in the panel directory: %v", err)
	}
	var doc map[string]any
	e.must(json.Unmarshal(data, &doc))
	doc["rationale"] = "edited while the decision is open"
	edited, err := json.Marshal(doc)
	e.must(err)
	e.commit(e.cwd, name, string(edited))
	out := e.apply()
	for _, a := range out.Actions {
		if a.Kind == "review-slot" || a.Kind == "verify" || a.Kind == "fix" {
			t.Fatalf("the edit gave %s while the decision is open: %+v", a.Step, out)
		}
	}
	if len(out.Decisions) != 1 || out.Decisions[0].Step != decision {
		t.Fatalf("decisions %+v, want only %s", out.Decisions, decision)
	}
	e.absent(gate+".r1.R3.a2", "the edit gave an R3 attempt while the decision is open")
	e.ok("submit", decision, "--answer", "redispatch")
	for _, a := range e.attempts(gate+".r1.R2.a4", gate+".r1.R5.a4") {
		e.submitAttempt(a, "VOID")
	}
	e.attempts(gate+".r1.R2.a5", gate+".r1.R3.a2", gate+".r1.R5.a5")
}

// AC19: the same decision open, then a commit S on the work item: the pass
// creates no judge. After redispatch and APPROVE on both attempt 4 steps, the
// pass creates the judge and the gate stays open.
func TestS134E2E_CriterionJudgeHeldByUncountedDecision(t *testing.T) {
	t.Parallel()
	e := newKernelE2EWithConfig(t, kernelE2EBinary(t), kerneltest.EmptyRepo(t), ".mindspec", kernelE2EFiveSlotConfig, 1)
	item := e.f.Items[0]
	gate := item + ".review"
	decision := e.uncountedDecision(gate)
	e.commit(e.tree(item), "during-decision.txt", "moved\n")
	out := e.apply()
	for _, a := range out.Actions {
		if a.Kind == "change-judge" {
			t.Fatalf("a judge while the decision is open: %+v", out)
		}
	}
	e.absent(gate+".judge.a1", "a judge while the decision is open")
	e.ok("submit", decision, "--answer", "redispatch")
	for _, a := range e.attempts(gate+".r1.R2.a4", gate+".r1.R5.a4") {
		e.submitAttempt(a, "APPROVE")
	}
	e.apply()
	if r := e.row(gate + ".judge.a1"); r.Status == "closed" {
		t.Fatalf("the judge is closed: %+v", r)
	}
	if r := e.row(gate); r.Status == "closed" {
		t.Fatalf("the gate closed before the judge: %+v", r)
	}
}

// AC20: with a round limit of 1, round 1 refused on a moved subject and a
// re-panel of 2 slots, the verify reproduces f: the limit decision. A panel
// refute of f supersedes it with attempt 2, and continue:1 on attempt 2 gives
// round 2's lens step and no fix.
func TestS134E2E_CriterionContinueAtLimit(t *testing.T) {
	t.Parallel()
	e := newKernelE2EWithConfig(t, kernelE2EBinary(t), kerneltest.EmptyRepo(t), ".mindspec", kernelE2EConfig+"  max_rounds: 1\n", 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.apply()
	e.create(e.implement(0))
	moved := e.commit(e.tree(item), "during-round.txt", "moved\n")
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	for i, word := range []string{"REQUEST_CHANGES", "APPROVE", "APPROVE"} {
		e.submitSlot(slots[i], word)
	}
	e.judge("re-panel", 2)
	e.verifyReproduced()
	first := gate + ".decide.limit.r1"
	if r := e.row(first); r.Status != "open" {
		t.Fatalf("limit decision %s is %s, want open", first, r.Status)
	}
	e.absent(gate+".r2.fix", "the reproduced finding at the limit gave a fix")
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	e.ok("panel", "refute", reg.Slug, "--spec", e.f.SpecID, "--slot", "R1", "--round", "1", "--finding", "ccr[0]", "--by", "operator", "--ground", "disproven", "--reason", "The moved subject covers the finding.", "--evidence", moved)
	e.apply()
	if r := e.receipt(first); r.Kind != "superseded" {
		t.Fatalf("limit decision receipt %+v, want superseded", r)
	}
	if r := e.row(first + ".a2"); r.Status != "open" {
		t.Fatalf("limit decision attempt 2 is %s, want open", r.Status)
	}
	e.ok("submit", first+".a2", "--answer", "continue:1")
	lens := false
	for _, a := range e.apply().Actions {
		lens = lens || a.Kind == "lens" && a.Round == 2
		if a.Kind == "fix" {
			t.Fatalf("continue gave a fix: %+v", a)
		}
	}
	if !lens {
		t.Fatal("continue gave no round-2 lens step")
	}
	e.absent(gate+".r2.fix", "continue gave a fix")
}

// refutations reads the refutation records of the gate's panel ledger.
func (e *kernelE2E) refutations(gate string) []panel.RefutationRecord {
	e.t.Helper()
	lines, err := panel.ReadLedger(filepath.Join(e.cwd, e.panelFile(gate, "")))
	e.must(err)
	var out []panel.RefutationRecord
	for _, line := range lines {
		var r panel.RefutationRecord
		e.must(json.Unmarshal(line, &r))
		if r.Record == panel.RecordRefutation {
			out = append(out, r)
		}
	}
	return out
}

// AC22: the final-gate slot prompt has the spec summary from spec.md, and its
// files in scope are the diff from the final base (the merge base with main)
// to the reviewed subject, so the work item's file is in scope.
func TestS134E2E_CriterionFinalPromptScope(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	e.apply()
	e.create(e.implement(0))
	e.review("APPROVE")
	e.ok("complete", item)
	final := e.draftLenses(e.apply())
	e.create(final)
	gate := e.f.Epic + ".final"
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok {
		t.Fatalf("no registration for %s", gate)
	}
	prompt := e.ok("panel", "prompt", reg.Slug, "--spec", e.f.SpecID, "--slot", "R1")
	summary := strings.Index(prompt, "Spec summary: ")
	if summary < 0 || !strings.Contains(prompt[summary:], "Exercise the kernel.") {
		t.Fatalf("the final prompt must carry the spec summary from spec.md:\n%s", prompt)
	}
	scope := strings.Index(prompt, "## Files in Scope")
	if scope < 0 || !strings.Contains(prompt[scope:], "- Path: item-0.txt\n") {
		t.Fatalf("the final prompt must scope the diff from the final base:\n%s", prompt)
	}
}

// longFinding is a 4096-byte finding text with a requirement after byte 2048.
func longFinding() (text, requirement string) {
	requirement = "Requirement after byte 2048: keep the whole finding text in the round-2 brief."
	head := strings.Repeat("a", 2100) + " "
	tail := " " + strings.Repeat("z", 4096-len(head)-len(requirement)-2) + "."
	return head + requirement + tail, requirement
}

// AC19: the fix created and leased, then a commit replaces f's round-1
// verdict file with APPROVE and a commit adds a stray R9-round-1.json. The
// pass creates no verify attempt 2 and no slot attempt, and the fix keeps its
// reproduced findings. After the fix closes, panel create 2 exits 0, and the
// round-2 slot prompt shows f as the verify record names it, with its return,
// its whole 4096-byte text, and no text from the edit or the stray file.
func TestS134E2E_CriterionFixHeldOverVerdictEdits(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	text, requirement := longFinding()
	if len(text) != 4096 || strings.Index(text, requirement) <= 2048 {
		t.Fatalf("finding text is %d bytes with the requirement at %d", len(text), strings.Index(text, requirement))
	}
	e.apply()
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		t.Fatalf("slots: %+v", slots)
	}
	v := paneltest.Verdict("REQUEST_CHANGES")
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: text, Basis: "traced"}}
	e.submitVerdict(slots[0], v)
	e.submitSlot(slots[1], "APPROVE")
	e.submitSlot(slots[2], "APPROVE")
	a, findings := e.verifyAction()
	if len(findings) != 1 || findings[0].Slot != "R1" || findings[0].Text != text {
		t.Fatalf("verify findings %+v, want R1's whole finding", findings)
	}
	f := findings[0]
	e.ok("submit", a.Step, "--lease", e.lease(a.Step), "--record", e.jsonFile("verify.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{returnFor(f, "reproduced", "")}}))
	fix := gate + ".r2.fix"
	before, ok, err := kernel.Decode[[]kernel.Finding](e.row(fix).Metadata, kernel.KeyReproduced)
	e.must(err)
	if !ok || len(before) != 1 || before[0].Slot != "R1" {
		t.Fatalf("fix reproduced %+v, want f", before)
	}
	token := e.lease(fix)
	// The edit replaces f's verdict file; no copy of the original stays on disk.
	name := e.panelFile(gate, "R1-round-1.json")
	data, err := os.ReadFile(filepath.Join(e.cwd, name))
	e.must(err)
	var doc map[string]any
	e.must(json.Unmarshal(data, &doc))
	doc["verdict"] = "APPROVE"
	doc["rationale"] = "Edited text after the fix lease."
	doc["concrete_changes_required"] = []any{}
	edited, err := json.Marshal(doc)
	e.must(err)
	e.commit(e.cwd, name, string(edited))
	data, err = os.ReadFile(filepath.Join(e.cwd, e.panelFile(gate, "R2-round-1.json")))
	e.must(err)
	doc = map[string]any{}
	e.must(json.Unmarshal(data, &doc))
	doc["slot"] = "R9"
	doc["rationale"] = "Stray slot text from an extra file."
	stray, err := json.Marshal(doc)
	e.must(err)
	e.commit(e.cwd, e.panelFile(gate, "R9-round-1.json"), string(stray))
	out := e.apply()
	for _, n := range out.Actions {
		if n.Kind == "review-slot" || n.Kind == "verify" {
			t.Fatalf("the edits gave %s while the fix is open: %+v", n.Step, out)
		}
	}
	e.absent(a.Step+".a2", "the edits gave a second verify")
	after, _, err := kernel.Decode[[]kernel.Finding](e.row(fix).Metadata, kernel.KeyReproduced)
	e.must(err)
	if len(after) != 1 || after[0] != before[0] {
		t.Fatalf("fix reproduced changed from %+v to %+v", before, after)
	}
	e.ok("submit", fix, "--lease", token, "--commit", e.commit(e.tree(item), "fixed.txt", "fixed\n"))
	next := e.draftLenses(e.apply())
	if len(next.Run) == 0 || strings.Join(next.Run[0].Argv, " ") != "mindspec panel create "+item {
		t.Fatalf("no round-2 panel create: %+v", next)
	}
	e.ok(next.Run[0].Argv[1:]...)
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	if reg.Round != 2 {
		t.Fatalf("registration %+v, want round 2", reg)
	}
	prompt := e.ok("panel", "prompt", reg.Slug, "--spec", e.f.SpecID, "--slot", "R1")
	at := strings.Index(prompt, "Finding: R1 round 1 "+f.Finding)
	if at < 0 || !strings.Contains(prompt[at:], "Text: "+text+"\n") || !strings.Contains(prompt[at:], "Return: reproduced") {
		t.Fatalf("the round-2 prompt must show f whole with its return:\n%s", prompt)
	}
	if !strings.Contains(prompt, requirement) {
		t.Fatal("the round-2 prompt lost the requirement after byte 2048")
	}
	for _, word := range []string{"Edited text after the fix lease.", "Stray slot text from an extra file.", "Finding: R9"} {
		if strings.Contains(prompt, word) {
			t.Fatalf("the round-2 prompt shows %q from the edit or the stray file:\n%s", word, prompt)
		}
	}
}

// refutationRows reads the audited-refuted disposition rows of one slot.
func (e *kernelE2E) refutationRows(gate, slot string) []panel.DispositionRow {
	e.t.Helper()
	lines, err := panel.ReadLedger(filepath.Join(e.cwd, e.panelFile(gate, "")))
	e.must(err)
	var out []panel.DispositionRow
	for _, line := range lines {
		var r panel.DispositionRow
		if json.Unmarshal(line, &r) == nil && r.Record == "disposition" && r.Reviewer == slot && r.Disposition == "audited-refuted" {
			out = append(out, r)
		}
	}
	return out
}

// AC19: findings f and g. A panel refute of g while the verify is open, then
// the verify refutes f and reproduces g: the gate stays open and the fix
// names g. A second panel refute of g with other evidence appends a record
// and its row, and the retally closes the gate passed.
func TestS134E2E_CriterionVerbRefutationOrder(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	e.refusedRound("REQUEST_CHANGES", "REQUEST_CHANGES", "APPROVE")
	a, findings := e.verifyAction()
	if len(findings) != 2 || findings[0].Slot != "R1" || findings[1].Slot != "R2" {
		t.Fatalf("verify findings %+v, want R1 and R2", findings)
	}
	f, g := findings[0], findings[1]
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	refute := func(evidence string) {
		e.t.Helper()
		e.ok("panel", "refute", reg.Slug, "--spec", e.f.SpecID, "--slot", "R2", "--round", "1", "--finding", g.Finding, "--by", "operator", "--ground", "disproven", "--reason", "The subject covers the finding.", "--evidence", evidence)
	}
	refute(a.Subject.Commit)
	if r := e.row(a.Step); r.Status == "closed" {
		t.Fatalf("the verb closed the verify: %+v", r)
	}
	e.ok("submit", a.Step, "--lease", e.lease(a.Step), "--record", e.jsonFile("verify.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{returnFor(f, "refuted", a.Subject.Commit), returnFor(g, "reproduced", "")}}))
	e.apply()
	if r := e.row(gate); r.Status == "closed" {
		t.Fatalf("a verb refutation made before the verify closed passed the gate: %+v", e.receipt(gate))
	}
	fix := gate + ".r2.fix"
	reproduced, ok, err := kernel.Decode[[]kernel.Finding](e.row(fix).Metadata, kernel.KeyReproduced)
	e.must(err)
	if !ok || len(reproduced) != 1 || reproduced[0].Slot != "R2" {
		t.Fatalf("fix reproduced %+v, want only g", reproduced)
	}
	records, rows := len(e.refutations(gate)), len(e.refutationRows(gate, "R2"))
	refute(e.commit(e.proof, "other-evidence.txt", "other evidence\n"))
	if got := len(e.refutations(gate)); got != records+1 {
		t.Fatalf("the second refute of g gave %d refutation records, want %d", got, records+1)
	}
	if got := len(e.refutationRows(gate, "R2")); got != rows+1 {
		t.Fatalf("the second refute of g gave %d rows, want %d", got, rows+1)
	}
	e.apply()
	if r := e.receipt(gate); r.Kind != "passed" {
		t.Fatalf("gate receipt %+v, want passed after the second refute", r)
	}
}

// AC21: a lens submit of L1 killed after its close writes and before the
// receipt, then lease --take: the generation-2 submit of L2 exits 0 (a lens
// has no closing pin), panel create exits 0, and each slot's focus is from L2.
func TestS134E2E_CriterionLensKilledThenTaken(t *testing.T) {
	t.Parallel()
	bin := kernelE2EBinary(t)
	crash := kernelE2ECrashBinary(t)
	e := newKernelE2E(t, bin, 1)
	gate := e.f.Items[0] + ".review"
	e.apply()
	var lens kernel.Action
	for _, a := range e.implement(0).Actions {
		if a.Kind == "lens" {
			lens = a
		}
	}
	if lens.Step == "" {
		t.Fatal("no lens action after the implementation")
	}
	draft := func(word string) (kernel.LensRecord, string) {
		r := e.lensRecord(lens)
		for i := range r.Lenses {
			r.Lenses[i].Lens = word + " focus for " + r.Lenses[i].Slot + "."
		}
		return r, e.lensFile(r)
	}
	first := e.lease(lens.Step)
	_, l1 := draft("L1")
	out, err := e.f.BinaryCommandIn(e.cwd, crash, "submit", lens.Step, "--lease", first, "--record", l1)
	if err == nil || !strings.Contains(string(out), "simulated kill") {
		t.Fatalf("the crash binary did not stop at the crash point: %v %s", err, out)
	}
	if r := e.row(lens.Step); r.Status == "closed" {
		t.Fatalf("the kill point closed the lens step: %+v", r)
	}
	second := e.lease(lens.Step, "--take", "--reason", "Replace the lens actor.")
	l2record, l2 := draft("L2")
	var next kernel.Output
	e.must(json.Unmarshal([]byte(e.ok("submit", lens.Step, "--lease", second, "--record", l2)), &next))
	if r := e.receipt(lens.Step); r.Generation != 2 {
		t.Fatalf("lens receipt %+v, want generation 2", r)
	}
	e.create(next)
	data, err := os.ReadFile(filepath.Join(e.cwd, e.panelFile(gate, "panel.json")))
	e.must(err)
	var p panel.Panel
	e.must(json.Unmarshal(data, &p))
	want := map[string]string{}
	for _, l := range l2record.Lenses {
		want[l.Slot] = l.Lens
	}
	seen := 0
	for _, slot := range p.Roster {
		if focus, ok := want[slot.Slot]; ok {
			seen++
			if slot.Focus != focus {
				t.Fatalf("slot %s focus %q, want L2's %q", slot.Slot, slot.Focus, focus)
			}
		}
	}
	if seen == 0 || seen != len(want) {
		t.Fatalf("roster %+v covers %d of the %d L2 slots", p.Roster, seen, len(want))
	}
}

// frozenFix makes a round with R1 and R2 APPROVE and R3 REQUEST_CHANGES (f is
// R3's ccr[0]). Before the verify, stray (if not nil) puts an off-roster
// INVALID R9-round-1.json in the panel directory before the last slot
// submits. The verify reproduces every
// finding, and the fix opens. It returns f and the fix step.
func (e *kernelE2E) frozenFix(gate string, stray func(name string)) (kernel.Finding, string) {
	e.t.Helper()
	e.apply()
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != 3 {
		e.t.Fatalf("slots: %+v", slots)
	}
	e.submitSlot(slots[0], "APPROVE")
	e.submitSlot(slots[1], "APPROVE")
	if stray != nil {
		stray(e.panelFile(gate, "R9-round-1.json"))
	}
	e.submitSlot(slots[2], "REQUEST_CHANGES")
	a, findings := e.verifyAction()
	want := 1
	if stray != nil {
		want = 2
	}
	if len(findings) != want || findings[0].Slot != "R3" {
		e.t.Fatalf("verify findings %+v, want R3's finding and %d in all", findings, want)
	}
	returns := []kernel.VerifyReturn{}
	for _, f := range findings {
		returns = append(returns, returnFor(f, "reproduced", ""))
	}
	e.ok("submit", a.Step, "--lease", e.lease(a.Step), "--record", e.jsonFile("verify.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: returns}))
	fix := gate + ".r2.fix"
	if r := e.row(fix); r.Status != "open" {
		e.t.Fatalf("fix %s is %s, want open", fix, r.Status)
	}
	out := e.apply()
	if len(out.Decisions) != 0 {
		e.t.Fatalf("the reproduced findings gave a decision: %+v", out)
	}
	return findings[0], fix
}

// refuteHeld runs panel refute of f, which must exit 0, and then checks that
// the gate and the fix stay open.
func (e *kernelE2E) refuteHeld(gate, fix string, f kernel.Finding) {
	e.t.Helper()
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	e.ok("panel", "refute", reg.Slug, "--spec", e.f.SpecID, "--slot", f.Slot, "--round", "1", "--finding", f.Finding, "--by", "operator", "--ground", "disproven", "--reason", "The subject covers the finding.", "--evidence", e.commit(e.proof, "refute-evidence.txt", "evidence\n"))
	e.apply()
	if r := e.row(gate); r.Status == "closed" {
		e.t.Fatalf("the refute closed the gate on a changed read set: %+v", e.receipt(gate))
	}
	if r := e.row(fix); r.Status == "closed" {
		e.t.Fatalf("the refute closed the fix on a changed read set: %+v", e.receipt(fix))
	}
}

// AC20 frozen-refutation fixtures (i) and (ii): the fix open after the verify
// reproduced only f, then (i) a commit edits R2's round-1 verdict or (ii) a
// commit adds an APPROVE R9-round-1.json. A panel refute of f exits 0, and the
// gate and the fix stay open.
func TestS134E2E_CriterionFrozenRefutationEdits(t *testing.T) {
	t.Parallel()
	bin := kernelE2EBinary(t)
	t.Run("i edit R2", func(t *testing.T) {
		t.Parallel()
		e := newKernelE2E(t, bin, 1)
		gate := e.f.Items[0] + ".review"
		f, fix := e.frozenFix(gate, nil)
		name := e.panelFile(gate, "R2-round-1.json")
		data, err := os.ReadFile(filepath.Join(e.cwd, name))
		e.must(err)
		var doc map[string]any
		e.must(json.Unmarshal(data, &doc))
		doc["rationale"] = "Edited after the verify."
		edited, err := json.Marshal(doc)
		e.must(err)
		e.commit(e.cwd, name, string(edited))
		e.refuteHeld(gate, fix, f)
	})
	t.Run("ii add R9 APPROVE", func(t *testing.T) {
		t.Parallel()
		e := newKernelE2E(t, bin, 1)
		gate := e.f.Items[0] + ".review"
		f, fix := e.frozenFix(gate, nil)
		data, err := os.ReadFile(filepath.Join(e.cwd, e.panelFile(gate, "R2-round-1.json")))
		e.must(err)
		var doc map[string]any
		e.must(json.Unmarshal(data, &doc))
		doc["slot"] = "R9"
		stray, err := json.Marshal(doc)
		e.must(err)
		e.commit(e.cwd, e.panelFile(gate, "R9-round-1.json"), string(stray))
		e.refuteHeld(gate, fix, f)
	})
}

// AC20 frozen-refutation fixtures (iii) to (v): the verify reproduced f and
// the veto of an off-roster INVALID R9-round-1.json. Then (iii) a commit
// removes that file, (iv) the file is moved off disk with no commit, or (v)
// the file was never committed and is moved off disk. A panel refute of f
// exits 0, and the gate and the fix stay open.
func TestS134E2E_CriterionFrozenRefutationStray(t *testing.T) {
	t.Parallel()
	bin := kernelE2EBinary(t)
	off := func(e *kernelE2E, name string) {
		e.t.Helper()
		e.must(os.Rename(filepath.Join(e.cwd, name), filepath.Join(e.t.TempDir(), "R9-round-1.json")))
	}
	for _, c := range []struct {
		name   string
		commit bool
		remove func(e *kernelE2E, name string)
	}{
		{"iii committed removal", true, func(e *kernelE2E, name string) {
			e.git(e.cwd, "rm", "-q", "--", name)
			e.git(e.cwd, "commit", "-qm", "remove the stray verdict")
		}},
		{"iv removal not committed", true, off},
		{"v never committed", false, off},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := newKernelE2E(t, bin, 1)
			gate := e.f.Items[0] + ".review"
			var name string
			f, fix := e.frozenFix(gate, func(path string) {
				name = path
				if c.commit {
					e.commit(e.cwd, path, `{"verdict":"INVALID"}`)
				} else {
					e.write(e.cwd, path, `{"verdict":"INVALID"}`)
				}
			})
			c.remove(e, name)
			e.refuteHeld(gate, fix, f)
		})
	}
}
