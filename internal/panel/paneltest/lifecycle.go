package paneltest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// Lifecycle seeds a local approval panel and its durable verdicts. State is
// pass, incomplete, reject, hard block, unresolved, coverage, abandoned,
// integrity, round mismatch or malformed. Git facts belong to the caller.
func Lifecycle(t testing.TB, dir, spec, gate, sha, digest, state string) Fixture {
	t.Helper()
	p := panel.Panel{Spec: spec, Target: "spec/" + spec, Gate: gate, Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: sha, SubjectDigest: digest,
		Roster: []panel.RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "correctness"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "correctness"}}}
	if err := panel.Create(dir, panel.CreateInput{Spec: spec, Target: p.Target, Gate: gate, Round: 1, ReviewedHeadSHA: sha, SubjectDigest: digest, Roster: p.Roster}); err != nil {
		t.Fatal(err)
	}
	f := Fixture{Dir: dir, Panel: p}
	for _, slot := range p.Roster {
		if state == "incomplete" && slot.Slot == "R2" {
			continue
		}
		verdict := "APPROVE"
		if slot.Slot == "R1" {
			if state == "reject" {
				verdict = "REJECT"
			}
			if state == "unresolved" {
				verdict = "REQUEST_CHANGES"
			}
		}
		v := Verdict(verdict)
		v.Slot = slot.Slot
		v.ReviewedSHA = sha
		v.HardBlock = state == "hard block" && slot.Slot == "R1"
		if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: slot.Slot, Round: 1}, Bytes(t, v), Resolve); err != nil {
			t.Fatal(err)
		}
	}
	res, err := panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.Panel = *res.Panel
	if state != "coverage" {
		if err := panel.WriteCoverageManifest(res); err != nil {
			t.Fatal(err)
		}
	}
	switch state {
	case "abandoned":
		Seed(t, dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: spec, Panel: filepath.Base(dir), Reason: "operator stopped the review", By: "operator", CreatedAt: CreatedAt})
		f.Panel.Abandoned = true
		f.Panel.AbandonReason = "operator stopped the review"
		if err := panel.WriteSnapshot(dir, f.Panel); err != nil {
			t.Fatal(err)
		}
	case "integrity":
		f.Panel.ReviewedHeadSHA = ReviewedSHA
		if err := panel.WriteSnapshot(dir, f.Panel); err != nil {
			t.Fatal(err)
		}
	case "round mismatch":
		if err := os.WriteFile(filepath.Join(dir, "R1-round-2.json"), []byte(`{"verdict":"APPROVE"}`), 0644); err != nil {
			t.Fatal(err)
		}
	case "malformed":
		if err := os.WriteFile(filepath.Join(dir, panel.FileName), []byte("{"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}
