package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// kernelE2ECrash is the binary whose submit stops after the close writes and
// before the receipt, as a process killed at that point does.
var kernelE2ECrash struct {
	once sync.Once
	data []byte
	err  error
	out  []byte
}

// kernelE2ECrashBinary builds the crash binary. Only the hook that runs between
// the close writes and the receipt changes: it returns an error after its own
// work. A mutation overlay in GOFLAGS stays in the build.
func kernelE2ECrashBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("built-binary lifecycle runs in the explicit end-to-end suite")
	}
	kernelE2ECrash.once.Do(func() {
		root := repoRootFromTestDir(t)
		dir, err := os.MkdirTemp("", "mindspec-crash-binary-")
		if err != nil {
			kernelE2ECrash.err = err
			return
		}
		replace := map[string]string{}
		var goflags []string
		for _, flag := range strings.Fields(os.Getenv("GOFLAGS")) {
			name, ok := strings.CutPrefix(flag, "-overlay=")
			if !ok {
				goflags = append(goflags, flag)
				continue
			}
			data, err := os.ReadFile(name)
			if err != nil {
				kernelE2ECrash.err = err
				return
			}
			var inherited struct{ Replace map[string]string }
			if err := json.Unmarshal(data, &inherited); err != nil {
				kernelE2ECrash.err = err
				return
			}
			for k, v := range inherited.Replace {
				replace[k] = v
			}
		}
		target := filepath.Join(root, "cmd", "mindspec", "submit.go")
		source := target
		if v, ok := replace[target]; ok {
			source = v
		}
		data, err := os.ReadFile(source)
		if err != nil {
			kernelE2ECrash.err = err
			return
		}
		const hook = "return publishSubmittedReview(fresh, options.Step, receipt)"
		if strings.Count(string(data), hook) != 1 {
			kernelE2ECrash.err = fmt.Errorf("the crash point %q is not in %s once", hook, source)
			return
		}
		crashed := strings.Replace(string(data), hook, "if err := publishSubmittedReview(fresh, options.Step, receipt); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\treturn fmt.Errorf(\"simulated kill after the close writes\")", 1)
		file := filepath.Join(dir, "submit.go")
		if kernelE2ECrash.err = os.WriteFile(file, []byte(crashed), 0600); kernelE2ECrash.err != nil {
			return
		}
		replace[target] = file
		overlay, err := json.Marshal(map[string]any{"Replace": replace})
		if err != nil {
			kernelE2ECrash.err = err
			return
		}
		name := filepath.Join(dir, "overlay.json")
		if kernelE2ECrash.err = os.WriteFile(name, overlay, 0600); kernelE2ECrash.err != nil {
			return
		}
		bin := filepath.Join(dir, "mindspec")
		cmd := exec.Command("go", "build", "-overlay="+name, "-o", bin, "./cmd/mindspec")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOFLAGS="+strings.Join(goflags, " "))
		kernelE2ECrash.out, kernelE2ECrash.err = cmd.CombinedOutput()
		if kernelE2ECrash.err == nil {
			kernelE2ECrash.data, kernelE2ECrash.err = os.ReadFile(bin)
		}
	})
	if kernelE2ECrash.err != nil {
		t.Fatalf("build the crash binary: %v\n%s", kernelE2ECrash.err, kernelE2ECrash.out)
	}
	path := filepath.Join(t.TempDir(), "mindspec-crash")
	if err := os.WriteFile(path, kernelE2ECrash.data, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

// submitAttempt leases one slot attempt and submits the word for it from a
// verdict file named for the attempt, so a repeated word is a new file. It
// returns the submit output.
func (e *kernelE2E) submitAttempt(a kernel.Action, word string) string {
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
	v := paneltest.Verdict(word)
	v.Slot = fmt.Sprintf("R%d", tuple.Slot)
	v.Round = a.Round
	v.ReviewedSHA = a.Subject.Commit
	v.Model = "test-model"
	if word == "VOID" {
		reason := "cannot review " + a.Step
		v.VoidReason = &reason
	}
	path := filepath.Join(".mindspec/specs", e.f.SpecID, "reviews", reg.Slug, fmt.Sprintf("verdict-%d-%s-%s.json", a.Round, v.Slot, a.Step[strings.LastIndex(a.Step, ".")+1:]))
	e.commit(e.proof, path, string(paneltest.Bytes(e.t, v)))
	return e.ok("submit", a.Step, "--lease", token, "--artifact", filepath.Join(e.proof, path))
}

// refusedRound implements the first item and submits the words for its three
// slots, in slot order.
func (e *kernelE2E) refusedRound(words ...string) {
	e.t.Helper()
	e.apply()
	e.create(e.implement(0))
	slots := e.slotActions()
	if len(slots) != len(words) {
		e.t.Fatalf("slots: %+v", slots)
	}
	for i, a := range slots {
		e.submitSlot(a, words[i])
	}
}

// verifyAction returns the verify action of the current pass and its findings.
func (e *kernelE2E) verifyAction() (kernel.Action, []kernel.Finding) {
	e.t.Helper()
	out := e.apply()
	for _, a := range out.Actions {
		if a.Kind == "fix" {
			e.t.Fatalf("a refused round gave a fix before the verify: %+v", out)
		}
	}
	for _, a := range out.Actions {
		if a.Kind == "verify" {
			findings, _, err := kernel.Decode[[]kernel.Finding](e.row(a.Step).Metadata, kernel.KeyFindings)
			e.must(err)
			return a, findings
		}
	}
	e.t.Fatalf("no verify action: %+v", out)
	return kernel.Action{}, nil
}

// returnFor is the return of one finding with the word given.
func returnFor(f kernel.Finding, word, evidence string) kernel.VerifyReturn {
	r := kernel.VerifyReturn{Slot: f.Slot, Round: f.Round, Finding: f.Finding, FindingSHA256: f.FindingSHA256, Return: word}
	if word == "refuted" {
		r.Ground, r.Reason, r.Evidence = "disproven", "The implementation covers the finding.", evidence
	}
	return r
}

// jsonFile writes the value to a new file and returns its path.
func (e *kernelE2E) jsonFile(name string, v any) string {
	e.t.Helper()
	data, err := json.Marshal(v)
	e.must(err)
	path := filepath.Join(e.t.TempDir(), name)
	e.must(os.WriteFile(path, data, 0600))
	return path
}

// AC19: two REQUEST_CHANGES findings give one verify with both findings and no
// fix. Each malformed verify record exits non-zero and leaves the step open.
// One refuted and one reproduced return give the fix, which names only the
// reproduced finding.
func TestS134E2E_CriterionMalformedVerifyRecords(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.refusedRound("REQUEST_CHANGES", "REQUEST_CHANGES", "APPROVE")
	a, findings := e.verifyAction()
	if a.Step != gate+".r1.verify" || len(findings) != 2 || findings[0].Slot != "R1" || findings[1].Slot != "R2" {
		t.Fatalf("verify %s findings %+v, want %s.r1.verify with the R1 and R2 findings", a.Step, findings, gate)
	}
	f, g := findings[0], findings[1]
	evidence := a.Subject.Commit
	token := e.lease(a.Step)
	record := func(returns ...kernel.VerifyReturn) kernel.VerifyRecord {
		return kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: returns}
	}
	badGround := returnFor(f, "refuted", evidence)
	badGround.Ground = "obvious"
	noReason := returnFor(f, "refuted", evidence)
	noReason.Reason = ""
	noEvidence := returnFor(f, "refuted", evidence)
	noEvidence.Evidence = "no/such/evidence.txt"
	noTarget := returnFor(f, "deferred", "")
	noTarget.Reason = "A follow-up owns it."
	badTarget := noTarget
	badTarget.Target = "no-such-item"
	cases := map[string]any{
		"omits a finding":     record(returnFor(f, "reproduced", "")),
		"repeats a finding":   record(returnFor(f, "reproduced", ""), returnFor(g, "reproduced", ""), returnFor(g, "reproduced", "")),
		"has no returns":      record(),
		"unknown return":      record(returnFor(f, "maybe", ""), returnFor(g, "reproduced", "")),
		"ground outside":      record(badGround, returnFor(g, "reproduced", "")),
		"no reason":           record(noReason, returnFor(g, "reproduced", "")),
		"no evidence":         record(noEvidence, returnFor(g, "reproduced", "")),
		"deferred no target":  record(noTarget, returnFor(g, "reproduced", "")),
		"deferred bad target": record(badTarget, returnFor(g, "reproduced", "")),
		"unknown field":       map[string]any{"step": a.Step, "subject": a.Subject.Commit, "returns": []kernel.VerifyReturn{returnFor(f, "reproduced", ""), returnFor(g, "reproduced", "")}, "extra": true},
	}
	for name, v := range cases {
		out, code := e.run("submit", a.Step, "--lease", token, "--record", e.jsonFile("verify.json", v))
		if code == 0 || !strings.Contains(out, "recovery:") {
			t.Fatalf("a verify record that %s was accepted: %d %s", name, code, out)
		}
		if r := e.row(a.Step); r.Status == "closed" {
			t.Fatalf("a verify record that %s closed the step", name)
		}
	}
	e.ok("submit", a.Step, "--lease", token, "--record", e.jsonFile("verify.json", record(returnFor(f, "refuted", evidence), returnFor(g, "reproduced", ""))))
	fix := gate + ".r2.fix"
	reproduced, ok, err := kernel.Decode[[]kernel.Finding](e.row(fix).Metadata, kernel.KeyReproduced)
	e.must(err)
	if !ok || len(reproduced) != 1 || reproduced[0].Slot != "R2" {
		t.Fatalf("fix reproduced %+v, want the R2 finding only", reproduced)
	}
	var action kernel.Action
	for _, next := range e.apply().Actions {
		if next.Step == fix {
			action = next
		}
	}
	if action.Step == "" {
		t.Fatalf("no fix action %s", fix)
	}
	brief := e.ok(action.Brief[1:]...)
	if !strings.Contains(brief, "Finding: R2 round 1") || strings.Contains(brief, "Finding: R1 round 1") {
		t.Fatalf("the fix brief must name only the reproduced finding:\n%s", brief)
	}
	// The verify writes the refutation record of the refuted finding.
	refs := e.refutations(gate)
	if len(refs) != 1 || refs[0].VerifyStep != a.Step || refs[0].Slot != "R1" || refs[0].Finding != f.Finding {
		t.Fatalf("refutation records %+v, want one for R1 by %s", refs, a.Step)
	}
	// After the fix closes, round 2 names the verify as its prior, and the
	// round-2 slot brief shows both findings with their returns.
	token = e.lease(fix)
	e.ok("submit", fix, "--lease", token, "--commit", e.commit(e.tree(item), "fixed.txt", "fixed\n"))
	next := e.apply()
	round, _, err := kernel.Decode[kernel.GateRound](e.row(gate).Metadata, kernel.KeyRound)
	e.must(err)
	if round.Round != 2 || round.Prior == nil || round.Prior.Verify != a.Step {
		t.Fatalf("round %+v, want round 2 whose prior names %s", round, a.Step)
	}
	e.create(next)
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	prompt := e.ok("panel", "prompt", reg.Slug, "--spec", e.f.SpecID, "--slot", "R1")
	r1 := strings.Index(prompt, "Finding: R1 round 1 "+f.Finding)
	r2 := strings.Index(prompt, "Finding: R2 round 1 "+g.Finding)
	refuted := strings.Index(prompt, "Return: refuted")
	reproducedAt := strings.LastIndex(prompt, "Return: reproduced")
	if r1 < 0 || r2 < r1 || refuted < r1 || refuted > r2 || reproducedAt < r2 {
		t.Fatalf("the round-2 slot brief must show both findings with their returns:\n%s", prompt)
	}
	// AC22: the round-2 prompt lists the paths changed since round 1's subject
	// and tells the slot not to raise a refuted or deferred finding again.
	changed := strings.Index(prompt, "## Paths changed since the prior round subject\n\n- Path: fixed.txt\n")
	asks := strings.Index(prompt, "## Prior-Round Asks")
	if changed < 0 || asks < changed || !strings.Contains(prompt, "Do not raise a refuted or deferred finding again without new evidence.") {
		t.Fatalf("the round-2 prompt must list the changed paths and the refuted-or-deferred line:\n%s", prompt)
	}
}

// AC19: slots R1 and R2 VOID on attempts 1, 2 and 3, with R3 APPROVE, give
// no verify and the next attempt of both slots after each VOID pair, then one
// uncounted-slot decision that lists R1 and R2 at attempt 3 and no counted
// slot. Each printed slot step is leased by its submit; while the decision is
// open, no slot, verify or fix step is printed. Redispatch gives
// attempt 4 of both; VOID on both gives attempt 5 of both, not a decision;
// APPROVE on attempt 5 closes the gate passed. Each pair is submitted with no
// pass between the two submits, so the submit pre-pass runs between them.
func TestS134E2E_CriterionVoidAttemptsAndUncountedDecision(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	gate := e.f.Items[0] + ".review"
	e.refusedRound("VOID", "VOID", "APPROVE")
	pair := func(attempt int) []kernel.Action {
		t.Helper()
		out := e.apply()
		var slots []kernel.Action
		for _, a := range out.Actions {
			if a.Kind == "verify" || a.Kind == "fix" {
				t.Fatalf("a VOID verdict gave %s: %+v", a.Kind, out)
			}
			if a.Kind == "review-slot" {
				slots = append(slots, a)
			}
		}
		want := []string{fmt.Sprintf("%s.r1.R1.a%d", gate, attempt), fmt.Sprintf("%s.r1.R2.a%d", gate, attempt)}
		if len(slots) != 2 || slots[0].Step != want[0] || slots[1].Step != want[1] || len(out.Decisions) != 0 {
			t.Fatalf("pass %+v, want only the slot attempts %v", out, want)
		}
		return slots
	}
	submitPair := func(slots []kernel.Action, word string) kernel.Output {
		t.Helper()
		var raw string
		for _, a := range slots {
			raw = e.submitAttempt(a, word)
		}
		var out kernel.Output
		e.must(json.Unmarshal([]byte(strings.Split(raw, "\n")[0]), &out))
		return out
	}
	submitPair(pair(2), "VOID")
	opened := submitPair(pair(3), "VOID")
	decision := gate + ".decide.uncounted.r1"
	// The output that opens the decision lists it.
	if len(opened.Decisions) != 1 || opened.Decisions[0].Step != decision {
		t.Fatalf("the submit that opened the decision does not list it: %+v", opened)
	}
	out := e.apply()
	for _, a := range out.Actions {
		if a.Kind == "review-slot" || a.Kind == "verify" || a.Kind == "fix" {
			t.Fatalf("attempt 3 VOID gave %s, want only the decision: %+v", a.Step, out)
		}
	}
	if len(out.Decisions) != 1 || out.Decisions[0].Step != decision {
		t.Fatalf("decisions %+v, want %s: %+v", out.Decisions, decision, out)
	}
	record, _, err := kernel.Decode[kernel.DecisionRecord](e.row(decision).Metadata, kernel.KeyDecision)
	e.must(err)
	if len(record.Slots) != 2 || record.Slots[0] != (kernel.DecisionSlot{Slot: "R1", Attempt: 3}) || record.Slots[1] != (kernel.DecisionSlot{Slot: "R2", Attempt: 3}) {
		t.Fatalf("decision slots %+v, want R1 and R2 at attempt 3", record.Slots)
	}
	e.ok("submit", decision, "--answer", "redispatch")
	submitPair(pair(4), "VOID")
	submitPair(pair(5), "APPROVE")
	if out := e.apply(); len(out.Decisions) != 0 {
		t.Fatalf("a reset count gave a decision: %+v", out)
	}
	if r := e.receipt(gate); r.Kind != "passed" {
		t.Fatalf("gate receipt %+v, want passed", r)
	}
}

// AC20: a verify open, then a commit on the work item and a carry-over: the
// verify closes superseded and the next verify attempt opens on the new
// subject; the gate stays open.
func TestS134E2E_CriterionVerifySupersededByCarryOver(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	gate := item + ".review"
	e.refusedRound("REQUEST_CHANGES", "APPROVE", "APPROVE")
	first, _ := e.verifyAction()
	stale := e.lease(first.Step)
	moved := e.commit(e.tree(item), "moved.txt", "moved\n")
	e.judge("carry-over", 0)
	out := e.apply()
	if r := e.receipt(first.Step); r.Kind != "superseded" {
		t.Fatalf("verify receipt %+v, want superseded", r)
	}
	var next kernel.Action
	for _, a := range out.Actions {
		if a.Kind == "verify" {
			next = a
		}
	}
	if next.Step != first.Step+".a2" || next.Subject.Commit != moved {
		t.Fatalf("next verify %+v, want %s.a2 on %s", next, first.Step, moved)
	}
	if _, code := e.run("submit", first.Step, "--lease", stale, "--record", e.jsonFile("verify.json", kernel.VerifyRecord{Step: first.Step, Subject: first.Subject.Commit})); code == 0 {
		t.Fatal("a submit to the superseded verify exited 0")
	}
	if e.row(gate).Status == "closed" {
		t.Fatal("the gate closed past a superseded verify")
	}
}

// AC21: generation 1 of a slot leased and taken by generation 2; the
// generation-1 submit exits 0, its receipt names generation 1, generation 2's
// lease ends superseded, and a generation-2 submit exits non-zero. A verify
// submit killed after its close writes and before the receipt, then the same
// submit: one receipt whose time is that of the closing pin. The same kill,
// then a submit of another record: non-zero naming the closing pin, and the
// next pass opens the next verify attempt, whose fix names only the finding
// that attempt reproduced.
func TestS134E2E_CriterionVerifyKilledBeforeReceipt(t *testing.T) {
	t.Parallel()
	bin := kernelE2EBinary(t)
	crash := kernelE2ECrashBinary(t)
	kill := func(e *kernelE2E, step, token, record string) kernel.Closing {
		e.t.Helper()
		out, err := e.f.BinaryCommandIn(e.cwd, crash, "submit", step, "--lease", token, "--record", record)
		if err == nil || !strings.Contains(string(out), "simulated kill") {
			e.t.Fatalf("the crash binary did not stop at the crash point: %v %s", err, out)
		}
		r := e.row(step)
		pin, ok, err := kernel.Decode[kernel.Closing](r.Metadata, kernel.KeyClosing)
		e.must(err)
		if _, has := r.Metadata[kernel.KeyClose]; has || r.Status == "closed" || !ok {
			e.t.Fatalf("the kill point must leave a closing pin and no receipt: %+v", r)
		}
		return pin
	}
	t.Run("same record", func(t *testing.T) {
		t.Parallel()
		e := newKernelE2E(t, bin, 1)
		item := e.f.Items[0]
		gate := item + ".review"
		e.apply()
		e.create(e.implement(0))
		slots := e.slotActions()
		if len(slots) != 3 {
			t.Fatalf("slots: %+v", slots)
		}
		e.submitSlot(slots[0], "REQUEST_CHANGES")
		e.submitSlot(slots[1], "REQUEST_CHANGES")
		first := e.lease(slots[2].Step)
		second := e.lease(slots[2].Step, "--take", "--reason", "Replace the slot actor.")
		e.ok("submit", slots[2].Step, "--lease", first, "--artifact", e.verdict(slots[2], "APPROVE"))
		if r := e.receipt(slots[2].Step); r.Generation != 1 || r.SupersededGeneration != 2 {
			t.Fatalf("slot receipt %+v, want generation 1 superseding 2", r)
		}
		lease, _, err := kernel.Decode[kernel.LeaseRecord](e.row(slots[2].Step).Metadata, kernel.KeyLease)
		e.must(err)
		if lease.Ended != "superseded" {
			t.Fatalf("generation-2 lease %+v, want ended superseded", lease)
		}
		if _, code := e.run("submit", slots[2].Step, "--lease", second, "--artifact", e.verdict(slots[2], "REQUEST_CHANGES")); code == 0 {
			t.Fatal("a generation-2 submit of another verdict exited 0")
		}
		a, findings := e.verifyAction()
		if len(findings) != 2 {
			t.Fatalf("verify findings %+v", findings)
		}
		token := e.lease(a.Step)
		x := e.jsonFile("x.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{returnFor(findings[0], "refuted", a.Subject.Commit), returnFor(findings[1], "refuted", a.Subject.Commit)}})
		pin := kill(e, a.Step, token, x)
		e.ok("submit", a.Step, "--lease", token, "--record", x)
		if r := e.receipt(a.Step); r.At != pin.At.UTC().Format("2006-01-02T15:04:05.999999999Z07:00") {
			t.Fatalf("verify receipt at %q, want the closing pin's %s", r.At, pin.At)
		}
		// The rerun of the same record gives one refutation record per finding.
		per := map[string]int{}
		for _, r := range e.refutations(gate) {
			per[r.Slot+" "+r.Finding]++
		}
		if len(per) != 2 || per[findings[0].Slot+" "+findings[0].Finding] != 1 || per[findings[1].Slot+" "+findings[1].Finding] != 1 {
			t.Fatalf("refutation records per finding %v, want one for each of the two findings", per)
		}
		e.apply()
		if r := e.receipt(gate); r.Kind != "passed" || r.Retally != "verify" {
			t.Fatalf("gate receipt %+v, want passed by the verify retally", r)
		}
	})
	t.Run("other record", func(t *testing.T) {
		t.Parallel()
		e := newKernelE2E(t, bin, 1)
		gate := e.f.Items[0] + ".review"
		e.refusedRound("REQUEST_CHANGES", "REQUEST_CHANGES", "APPROVE")
		a, findings := e.verifyAction()
		f, g := findings[0], findings[1]
		token := e.lease(a.Step)
		x := e.jsonFile("x.json", kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{returnFor(f, "refuted", a.Subject.Commit), returnFor(g, "refuted", a.Subject.Commit)}})
		kill(e, a.Step, token, x)
		other := e.git(e.f.Root, "rev-parse", "HEAD")
		y := kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{returnFor(f, "refuted", other), returnFor(g, "reproduced", "")}}
		e.refuse("mindspec_closing", "submit", a.Step, "--lease", token, "--record", e.jsonFile("y.json", y))
		var next kernel.Action
		for _, n := range e.apply().Actions {
			if n.Kind == "verify" {
				next = n
			}
		}
		if next.Step != a.Step+".a2" {
			t.Fatalf("next verify %q, want %s.a2", next.Step, a.Step)
		}
		y.Step = next.Step
		y.Subject = next.Subject.Commit
		e.ok("submit", next.Step, "--lease", e.lease(next.Step), "--record", e.jsonFile("y2.json", y))
		reproduced, ok, err := kernel.Decode[[]kernel.Finding](e.row(gate+".r2.fix").Metadata, kernel.KeyReproduced)
		e.must(err)
		if !ok || len(reproduced) != 1 || reproduced[0].Slot != g.Slot {
			t.Fatalf("fix reproduced %+v, want only %s", reproduced, g.Slot)
		}
	})
	t.Run("generation 2", func(t *testing.T) {
		t.Parallel()
		e := newKernelE2E(t, bin, 1)
		gate := e.f.Items[0] + ".review"
		e.refusedRound("REQUEST_CHANGES", "REQUEST_CHANGES", "APPROVE")
		a, findings := e.verifyAction()
		f, g := findings[0], findings[1]
		token := e.lease(a.Step)
		x := kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{returnFor(f, "refuted", a.Subject.Commit), returnFor(g, "refuted", a.Subject.Commit)}}
		kill(e, a.Step, token, e.jsonFile("x.json", x))
		second := e.lease(a.Step, "--take", "--reason", "Replace the verify actor.")
		y := kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{returnFor(f, "reproduced", ""), returnFor(g, "refuted", a.Subject.Commit)}}
		if out, code := e.run("submit", a.Step, "--lease", second, "--record", e.jsonFile("y.json", y)); code == 0 {
			t.Fatalf("a generation-2 submit of another record exited 0: %s", out)
		}
		var next kernel.Action
		for _, n := range e.apply().Actions {
			if n.Kind == "verify" {
				next = n
			}
		}
		if next.Step != a.Step+".a2" {
			t.Fatalf("next verify %q, want %s.a2", next.Step, a.Step)
		}
		y.Step = next.Step
		y.Subject = next.Subject.Commit
		e.ok("submit", next.Step, "--lease", e.lease(next.Step), "--record", e.jsonFile("y2.json", y))
		byA2 := 0
		for _, r := range e.refutations(gate) {
			if r.VerifyStep == next.Step && r.Slot == g.Slot && r.Finding == g.Finding {
				byA2++
			}
		}
		if byA2 != 1 {
			t.Fatalf("refutation records of g by %s: %d, want 1", next.Step, byA2)
		}
		// The killed submit wrote X's rows; each verify attempt keys its own
		// row of g, so the two rows have distinct ids.
		ids := map[string]bool{}
		rows := e.refutationRows(gate, g.Slot)
		for _, r := range rows {
			ids[r.ID] = true
		}
		if len(rows) != 2 || len(ids) != 2 {
			t.Fatalf("rows of g %+v, want two with distinct ids (X's and %s's)", rows, next.Step)
		}
		fix := gate + ".r2.fix"
		reproduced, ok, err := kernel.Decode[[]kernel.Finding](e.row(fix).Metadata, kernel.KeyReproduced)
		e.must(err)
		if !ok || len(reproduced) != 1 || reproduced[0].Slot != f.Slot {
			t.Fatalf("fix reproduced %+v, want only %s", reproduced, f.Slot)
		}
		reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
		e.must(err)
		e.ok("panel", "refute", reg.Slug, "--spec", e.f.SpecID, "--slot", g.Slot, "--round", "1", "--finding", g.Finding, "--by", "operator", "--ground", "disproven", "--reason", "The subject covers the finding.", "--evidence", e.commit(e.proof, "verb-evidence.txt", "verb evidence\n"))
		e.apply()
		if r := e.row(gate); r.Status == "closed" {
			t.Fatalf("the refute of g closed the gate: %+v", e.receipt(gate))
		}
		if r := e.row(fix); r.Status == "closed" {
			t.Fatalf("the refute of g closed the fix: %+v", e.receipt(fix))
		}
	})
}

const kernelE2ETwoAdversaryConfig = "enforcement:\n  panel_gate: false\nauto_open_finalize_pr: false\npanel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: correctness\n      count: 2\n    - family: harness\n      model: test-model\n      lens: adversarial\n      count: 2\n"

// AC22: on a roster with two adversary slots, a 2-slot scoped round has a lens
// record with an empty lens list. Its round-2 slot prompt lists the paths
// changed since round 1's subject and the line about refuted or deferred
// findings.
func TestS134E2E_CriterionTwoAdversaryScopedRound(t *testing.T) {
	t.Parallel()
	e := newKernelE2EWithConfig(t, kernelE2EBinary(t), kerneltest.EmptyRepo(t), ".mindspec", kernelE2ETwoAdversaryConfig, 1)
	item := e.f.Items[0]
	e.apply()
	e.create(e.implement(0))
	e.reviewSlots("APPROVE", 4)
	e.commit(e.tree(item), "scoped.txt", "scoped\n")
	e.judge("re-panel", 2)
	var lens kernel.Action
	for _, a := range e.apply().Actions {
		if a.Kind == "lens" {
			lens = a
		}
	}
	if lens.Step == "" {
		t.Fatal("no lens step for the scoped round")
	}
	if record := e.lensRecord(lens); len(record.Lenses) != 0 {
		t.Fatalf("lens record %+v, want an empty lens list for two adversary slots", record)
	}
	raw := e.ok("submit", lens.Step, "--lease", e.lease(lens.Step), "--record", e.lensFile(kernel.LensRecord{Step: lens.Step, Lenses: []kernel.SlotLens{}}))
	var out kernel.Output
	e.must(json.Unmarshal([]byte(raw), &out))
	e.create(out)
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(item+".review.a2").Metadata, kernel.KeyRegistration)
	e.must(err)
	if strings.Join(reg.Slots, ",") != "R1,R2" {
		t.Fatalf("scoped round slots %v, want R1,R2", reg.Slots)
	}
	for _, slot := range reg.Slots {
		prompt := e.ok("panel", "prompt", reg.Slug, "--spec", e.f.SpecID, "--slot", slot)
		for _, want := range []string{"scoped.txt", "adversarial", "Do not raise a refuted or deferred finding again without new evidence."} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("slot %s prompt lacks %q:\n%s", slot, want, prompt)
			}
		}
	}
	e.reviewSlots("APPROVE", 2)
	e.ok("complete", item)
}
