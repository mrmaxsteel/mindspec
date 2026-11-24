package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestS134PanelRepanel_LaterRounds(t *testing.T) {
	for _, kind := range []string{"bead", "final"} {
		for _, trigger := range []string{"subject-move", "refused-round"} {
			t.Run(kind+"/"+trigger, func(t *testing.T) {
				e := finalKernelFixture(t)
				e.apply()
				e.create(e.implement(0))
				e.review("APPROVE")
				item := e.f.Items[0]
				base, tree := item+".review", e.tree(item)
				if kind == "final" {
					e.ok("complete", item)
					e.create(e.apply())
					e.review("APPROVE")
					base, tree = e.f.Epic+".final", e.cwd
				}
				gate := base + ".a2"
				e.commit(tree, "changed.txt", "change after review\n")
				e.judge("re-panel", 2)
				if trigger == "subject-move" {
					before, ok, err := kernel.Decode[kernel.GateRound](e.row(gate).Metadata, kernel.KeyRound)
					e.must(err)
					if !ok || before.SlotCount != 2 || !samePaths(before.Paths, []string{"changed.txt"}) {
						t.Fatalf("missing scoped authorization: %+v", before)
					}
					e.commit(tree, "later.txt", "change after judgement\n")
					e.apply()
					after, ok, err := kernel.Decode[kernel.GateRound](e.row(gate).Metadata, kernel.KeyRound)
					e.must(err)
					if !ok || after.Subject != before.Subject || after.Round != before.Round || after.SlotCount != before.SlotCount || !samePaths(after.Paths, before.Paths) || !samePaths(after.Slots, before.Slots) {
						t.Fatalf("subject move reset scope without a judgement: before=%+v after=%+v", before, after)
					}
					return
				} else {
					e.create(e.apply())
					repanelReview(t, e, gate, 2, "REQUEST_CHANGES", []string{"changed.txt"})
					e.verifyReproduced()
					fix := base + ".r3.fix"
					token := e.lease(fix)
					head := e.commit(tree, "fixed.txt", "fixed\n")
					e.ok("submit", fix, "--lease", token, "--commit", head)
				}
				e.create(e.apply())
				repanelReview(t, e, gate, 3, "APPROVE", nil)
				e.must(kernel.CheckPassedGate(e.f.State(), base))
				if kind == "bead" {
					e.ok("complete", item)
					if !kernel.ItemDone(e.f.State(), item) {
						t.Fatal("passing full round did not permit completion")
					}
				} else {
					e.ok("impl", "approve", e.f.SpecID)
					out := e.apply()
					if out.Status != "done" {
						t.Fatalf("passing final round did not permit approval: %+v", out)
					}
				}
			})
		}
	}
}

func repanelReview(t *testing.T, e *kernelE2E, gate string, size int, word string, paths []string) {
	t.Helper()
	round, ok, err := kernel.Decode[kernel.GateRound](e.row(gate).Metadata, kernel.KeyRound)
	e.must(err)
	if !ok || round.SlotCount != size || !samePaths(round.Paths, paths) {
		t.Fatalf("round authority: %+v; want %d slots and paths %v", round, size, paths)
	}
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok || len(reg.Slots) != size {
		t.Fatalf("registered roster: %+v; want %d slots", reg, size)
	}
	dir := filepath.Join(e.f.SpecDir, "reviews", reg.Slug)
	result, err := panel.Tally(dir)
	e.must(err)
	if !samePaths(result.Panel.Scope, paths) || result.Panel.ExpectedReviewers != size {
		t.Fatalf("registered scope and size: %+v", result.Panel)
	}
	lines, err := panel.ReadLedger(dir)
	e.must(err)
	found := false
	for _, line := range lines {
		var event panel.RoundRecord
		e.must(json.Unmarshal(line, &event))
		if event.Record == panel.RecordRound && event.Round == round.Round {
			found = true
			if !samePaths(event.Scope, paths) || event.ExpectedReviewers != size {
				t.Fatalf("ledger scope and size: %+v", event)
			}
		}
	}
	if !found {
		t.Fatal("registered round is absent from the ledger")
	}
	out := e.apply()
	n := 0
	for _, action := range out.Actions {
		if action.Kind != "review-slot" {
			accepted, err := kernel.AcceptedSubject(e.f.State(), gate)
			e.must(err)
			t.Fatalf("unexpected action: %+v; gate: %+v; accepted: %s", action, e.row(gate), accepted)
		}
		token := e.lease(action.Step)
		e.ok("submit", action.Step, "--lease", token, "--artifact", e.verdict(action, word))
		n++
	}
	if n != size {
		t.Fatalf("submitted %d slots, want %d", n, size)
	}
}

func samePaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
