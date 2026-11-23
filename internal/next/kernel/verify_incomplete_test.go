package kernel_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS134Verify_UncountedCap(t *testing.T) {
	for _, mode := range []string{"edit-unlisted", "subject-move"} {
		t.Run(mode, func(t *testing.T) { verifyUncountedCap(t, mode) })
	}
}

func verifyUncountedCap(t *testing.T, mode string) {
	f := reducerFixture(t)
	put(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "panel:\n  reviewers:\n    - family: harness\n      count: 5\n")
	gate := f.Items[0] + ".review"
	head := gateHead(t, f, gate)
	roster := []string{"R1", "R2", "R3", "R4", "R5"}
	round := kernel.GateRound{Round: 1, Subject: head, SlotCount: 5, Slots: []string{}}
	for _, slot := range roster {
		round.Slots = append(round.Slots, gate+".r1."+slot)
	}
	kerneltest.Authorise(f, gate, round)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: roster})
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	submitSlot := func(slot, word string, attempt int) {
		t.Helper()
		v := paneltest.Verdict(word)
		v.Slot, v.ReviewedSHA, v.Model = slot, head, "test-model"
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: slot, Round: 1}, paneltest.Bytes(t, v), verifyResolution)
		f.Must(err)
		id := gate + ".r1." + slot
		if attempt > 1 {
			id += fmt.Sprintf(".a%d", attempt)
		}
		closePass(t, f, id)
	}
	for _, slot := range []string{"R1", "R3", "R4"} {
		submitSlot(slot, panel.VerdictApprove, 1)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		for _, slot := range []string{"R2", "R5"} {
			submitSlot(slot, "VOID", attempt)
		}
		out := reduce(t, f, true)
		if attempt < 3 {
			for _, slot := range []string{"R2", "R5"} {
				if !action(out, fmt.Sprintf("%s.r1.%s.a%d", gate, slot, attempt+1)) {
					t.Fatalf("uncounted slot did not retry: %+v", out)
				}
			}
		} else {
			decision := gate + ".decide.uncounted.r1"
			if len(out.Decisions) != 1 || out.Decisions[0].Step != decision || !reflect.DeepEqual(out.Decisions[0].Options, []string{"redispatch", "halt"}) {
				t.Fatalf("third uncounted attempt bypassed the decision: %+v", out)
			}
			d, _, err := kernel.Decode[kernel.DecisionRecord](record(t, f, decision).Metadata, kernel.KeyDecision)
			f.Must(err)
			if !reflect.DeepEqual(d.Slots, []kernel.DecisionSlot{{Slot: "R2", Attempt: 3}, {Slot: "R5", Attempt: 3}}) {
				t.Fatalf("cap did not list both slots at attempt 3: %+v", d)
			}
			for _, slot := range roster {
				id := fmt.Sprintf("%s.r1.%s.a4", gate, slot)
				if r, err := f.Store.Lookup(id); err != nil || r != nil {
					t.Fatalf("cap created %s without an answer", id)
				}
			}
		}
	}
	// A changed closed slot is held too while the operator decision is open.
	if mode == "edit-unlisted" {
		put(t, filepath.Join(dir, "R3-round-1.json"), `{"verdict":"INVALID"}`)
		reducerGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
		reducerGit(t, f, "add", "-f", "--", dir)
		reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "edit unlisted slot while decision is open")
	} else {
		beadCommit(t, f, "bead/"+f.Items[0])
	}
	out := reduce(t, f, true)
	if action(out, gate+".r1.R3.a2") || action(out, gate+".judge.a1") {
		t.Fatal("open uncounted decision redispatched R3 or created a judge")
	}
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: gate + ".decide.uncounted.r1", Answer: "redispatch"})
	f.Must(err)
	out = reduce(t, f, true)
	if !action(out, gate+".r1.R2.a4") || !action(out, gate+".r1.R5.a4") || action(out, gate+".r1.R3.a2") {
		t.Fatalf("redispatch did not serve exactly its listed slots: %+v", out)
	}
	for _, slot := range []string{"R2", "R5"} {
		word := "VOID"
		if mode == "subject-move" {
			word = panel.VerdictApprove
		}
		submitSlot(slot, word, 4)
	}
	out = reduce(t, f, true)
	if mode == "subject-move" {
		if !action(out, gate+".judge.a1") || record(t, f, gate).Status == "closed" || action(out, gate+".r1.R3.a2") {
			t.Fatalf("completed redispatch did not wait for its moved-subject judge: %+v", out)
		}
		return
	}
	if len(out.Decisions) != 0 || !action(out, gate+".r1.R2.a5") || !action(out, gate+".r1.R5.a5") || !action(out, gate+".r1.R3.a2") {
		t.Fatalf("redispatch did not reset each listed count: %+v", out)
	}
}
