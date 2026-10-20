package panel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGate_SubFloorRegistrationError(t *testing.T) {
	dir, _ := passFixture(t, 1, VerdictApprove)
	d := passDecision(t, dir)
	if d.Action != Block || !strings.Contains(d.Message, "registration") || !strings.Contains(d.Message, "--reviewers 2") || strings.Contains(d.Message, "/ms-bead-fix") {
		t.Fatalf("floor: %+v", d)
	}
}
func TestGate_HandEditDetected(t *testing.T) {
	edits := map[string]func(*Panel){
		"round":              func(p *Panel) { p.Round++ },
		"roster":             func(p *Panel) { p.Roster[0].Model = "edited" },
		"gate":               func(p *Panel) { p.Gate = "final_review" },
		"target":             func(p *Panel) { p.Target = "main" },
		"bead_id":            func(p *Panel) { p.BeadID = ptr("mindspec-other.1") },
		"expected_reviewers": func(p *Panel) { p.ExpectedReviewers++ },
		"reviewed_head_sha":  func(p *Panel) { p.ReviewedHeadSHA = "def456" },
		"subject_digest":     func(p *Panel) { p.SubjectDigest = "sha256:edited" },
		"refutations":        func(p *Panel) { p.Refutations = append(p.Refutations, Refutation{Slot: "R1", Round: 1}) },
		"rebinds":            func(p *Panel) { p.Rebinds = append(p.Rebinds, Rebind{FromSHA: "abc123", ToSHA: "def456"}) },
		"abandoned":          func(p *Panel) { p.Abandoned = true; p.AbandonReason = "hand edit" },
	}
	for field, edit := range edits {
		t.Run(field, func(t *testing.T) {
			dir, p := passFixture(t, 2, VerdictApprove)
			makeNonLegacy(t, dir, &p)
			if d := passDecision(t, dir); d.Action != Allow {
				t.Fatalf("baseline: %+v", d)
			}
			edit(&p)
			writePassJSON(t, dir, FileName, p)
			if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "changed outside the panel verbs") {
				t.Fatalf("hand edit %s: %+v", field, d)
			}
		})
	}
	t.Run("hand-cleared abandoned", func(t *testing.T) {
		dir, p := passFixture(t, 2, VerdictApprove)
		makeNonLegacy(t, dir, &p)
		a := AbandonRecord{Record: RecordAbandon, Spec: p.Spec, Panel: "p", Reason: "halt", By: "operator", CreatedAt: ledgerLater}
		if err := AppendRecords(dir, mustMarshal(t, a)); err != nil {
			t.Fatal(err)
		}
		p.Abandoned = true
		p.AbandonReason = a.Reason
		writePassJSON(t, dir, FileName, p)
		if d := passDecision(t, dir); d.Action != Warn {
			t.Fatalf("legitimate abandonment: %+v", d)
		}
		p.Abandoned = false
		writePassJSON(t, dir, FileName, p)
		if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "changed outside the panel verbs") {
			t.Fatalf("cleared abandonment: %+v", d)
		}
	})
}
func TestGate_HandChangedEntryPayload(t *testing.T) {
	for _, kind := range []string{"refutation", "deferral", "rebind"} {
		t.Run(kind, func(t *testing.T) {
			dir, p := passFixture(t, 2, VerdictApprove)
			raws := ledgerFixtures(t, "p")
			var ref Refutation
			_ = json.Unmarshal(raws[1], &ref)
			var def Deferral
			_ = json.Unmarshal(raws[2], &def)
			var rb Rebind
			_ = json.Unmarshal(raws[3], &rb)
			p.Refutations = []Refutation{ref}
			p.Deferrals = []Deferral{def}
			p.Rebinds = []Rebind{rb}
			makeNonLegacy(t, dir, &p)
			p.Refutations[0].CreatedAt = ledgerLater
			writePassJSON(t, dir, FileName, p)
			if d := passDecision(t, dir); d.Action != Allow {
				t.Fatalf("timestamp-only retry: %+v", d)
			}
			switch kind {
			case "refutation":
				p.Refutations[0].Reason = "edited"
			case "deferral":
				p.Deferrals[0].Bead = "mindspec-other"
			case "rebind":
				p.Rebinds[0].FromSHA = "999999"
			}
			writePassJSON(t, dir, FileName, p)
			if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "changed outside the panel verbs") {
				t.Fatalf("payload edit: %+v", d)
			}
		})
	}
}
func TestGate_OneMarkerBlocks(t *testing.T) {
	for _, marker := range []string{"telemetry", "round"} {
		t.Run(marker, func(t *testing.T) {
			dir, p := passFixture(t, 2, VerdictApprove)
			makeNonLegacy(t, dir, &p)
			if marker == "telemetry" {
				p.Telemetry = ""
				writePassJSON(t, dir, FileName, p)
			} else {
				if err := os.Rename(filepath.Join(dir, dispositionsFileName), filepath.Join(dir, "saved-ledger")); err != nil {
					t.Fatal(err)
				}
			}
			if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "changed outside the panel verbs") {
				t.Fatalf("single marker: %+v", d)
			}
		})
	}
}
func TestGate_LegacyHandEditExempt(t *testing.T) {
	dir, p := passFixture(t, 2, VerdictRequestChanges)
	p.Refutations = []Refutation{{Slot: "R1", Round: 1}, {Slot: "R2", Round: 1}}
	writePassJSON(t, dir, FileName, p)
	if d := passDecision(t, dir); d.Action != Allow {
		t.Fatalf("legacy refutations: %+v", d)
	}
}
func TestGate_CarriedBaseline(t *testing.T) {
	dir, p := passFixture(t, 2, VerdictRequestChanges)
	p.Refutations = []Refutation{{Slot: "R1", Round: 1, Reason: "legacy"}}
	makeNonLegacy(t, dir, &p)
	if r := passTally(t, dir); r.IntegrityErr != nil {
		t.Fatalf("carried baseline: %v", r.IntegrityErr)
	}
	if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "unresolved") {
		t.Fatalf("unpinned carried entry discharged nonlegacy: %+v", d)
	}
}
