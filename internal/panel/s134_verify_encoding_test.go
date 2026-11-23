package panel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS134Records_DispositionEncoding(t *testing.T) {
	for _, kind := range []string{RecordRefutation, RecordDeferral} {
		for _, mode := range []string{"verify", "enrolled", "legacy"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				step, enrolled := "", false
				if mode != "legacy" {
					enrolled = true
				}
				if mode == "verify" {
					step = "test-work.1.review.r1.verify"
				}
				ref := Refutation{VerifyStep: step, Enrolled: enrolled, Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), FindingText: "fix this", Ground: "disproven", Reason: "checked", Evidence: "https://example.invalid/proof", By: "verifier", CreatedAt: "2026-01-02T03:04:05.5Z"}
				d := Deferral{VerifyStep: step, Enrolled: enrolled, Slot: ref.Slot, Round: ref.Round, Finding: ref.Finding, FindingSHA256: ref.FindingSHA256, FindingText: ref.FindingText, Bead: "follow-up", By: ref.By, CreatedAt: ref.CreatedAt}
				var event any = RefutationRecord{Record: RecordRefutation, Spec: "x", Panel: "review", Refutation: ref}
				id := RefutationRowID("review", ref)
				if kind == RecordDeferral {
					event = DeferralRecord{Record: RecordDeferral, Spec: "x", Panel: "review", Deferral: d}
					id = DeferralRowID("review", d)
				}
				data, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				golden, err := os.ReadFile(filepath.Join("testdata", "verify-"+kind+"-"+mode+".golden"))
				if err != nil {
					t.Fatal(err)
				}
				if string(data)+"\n"+id+"\n" != string(golden) {
					t.Fatalf("ledger encoding or row id changed:\n%s\n%s\nwant:\n%s", data, id, golden)
				}
				if err := Validate(data); err != nil {
					t.Fatal(err)
				}
				m, err := decodeRecord(data)
				if err != nil {
					t.Fatal(err)
				}
				_, key := recordKey(m)
				if mode == "verify" && !strings.Contains(key, step) || mode == "enrolled" && !strings.Contains(key, ref.CreatedAt) || mode == "legacy" && strings.Contains(key, ref.CreatedAt) {
					t.Fatalf("wrong ledger key: %s", key)
				}
				if kind == RecordDeferral {
					d.SourceVerdict, d.SourceFileSHA256 = VerdictRequestChanges, strings.Repeat("b", 64)
					if got := DeferralRowID("review", d); !strings.HasSuffix(got, ":deferral"+dispositionSuffix(step, enrolled, d.CreatedAt)) {
						t.Fatalf("bound deferral omitted its suffix: %s", got)
					}
				}
			})
		}
	}
}

func TestS134Records_VerifyDeferralComparable(t *testing.T) {
	// Independent decodes must compare by value, so a verify_step pointer cannot
	// silently make the completeness floor discard a bound, effective deferral.
	data := `{"slot":"R1","round":1,"finding":"ccr[0]","finding_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","finding_text":"fix this","bead":"follow-up","by":"verifier","created_at":"2026-01-02T03:04:05Z","verify_step":"test-work.1.review.r1.verify","enrolled":true,"source_verdict":"REQUEST_CHANGES","source_file_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`
	var a, b Deferral
	if err := json.Unmarshal([]byte(data), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(data), &b); err != nil {
		t.Fatal(err)
	}
	round := 1
	row := testRow(DeferralRowID("p", a), "p", "R1", nil)
	row.Disposition, row.EvidenceRef, row.Round, row.CreatedAt = "confirmed-deferred", a.Bead, &round, a.CreatedAt
	specDir := t.TempDir()
	manifest := testManifest("p", round, []ManifestSlot{{Slot: "R1", Model: "sonnet", Verdict: VerdictRequestChanges}})
	event := DeferralRecord{Record: RecordDeferral, Spec: "999", Panel: "p", Deferral: a}
	writeDispositionsFile(t, specDir, "p", mustMarshal(t, event), mustMarshal(t, row), mustMarshal(t, manifest))
	current := &Result{Panel: &Panel{Deferrals: []Deferral{b}}, LatestRound: round, Verdicts: []Verdict{{Slot: a.Slot, Round: round, Verdict: VerdictRequestChanges, FileSHA256: a.SourceFileSHA256, Findings: []Finding{{ID: a.Finding, Text: a.FindingText, SHA256: a.FindingSHA256}}}}}
	if err := checkCompletenessAt(filepath.Join(specDir, "reviews", "p"), "p", current); err != nil {
		t.Fatalf("bound verify deferral did not cover its slot: %v", err)
	}
	if a != b {
		t.Fatal("independent verify deferrals do not compare by value")
	}
}
