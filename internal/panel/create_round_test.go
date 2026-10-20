package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func createRoster(n int) []RosterSlot {
	slots := make([]RosterSlot, n)
	for i := range slots {
		slots[i] = RosterSlot{fmt.Sprintf("R%d", i+1), fmt.Sprintf("model-%d", i%2), "correctness"}
	}
	return slots
}
func roundCreateInput() CreateInput {
	return CreateInput{Spec: "130", Target: "main", Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:fixture", Gate: "final_review", Roster: createRoster(4)}
}
func createMust(t *testing.T, dir string, in CreateInput) {
	t.Helper()
	if err := Create(dir, in); err != nil {
		t.Fatal(err)
	}
}
func createWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
func createRead(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func createRoundEvents(t *testing.T, dir string) []RoundRecord {
	t.Helper()
	lines, err := ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	var rounds []RoundRecord
	for _, line := range lines {
		var kind recordDiscriminator
		if err := json.Unmarshal(line, &kind); err != nil {
			t.Fatal(err)
		}
		if kind.Record == RecordRound {
			var r RoundRecord
			if err := json.Unmarshal(line, &r); err != nil {
				t.Fatal(err)
			}
			rounds = append(rounds, r)
		}
	}
	return rounds
}
func createAppend(t *testing.T, dir string, event interface{}) {
	t.Helper()
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendRecords(dir, data); err != nil {
		t.Fatal(err)
	}
}
func createEntries() CarriedEntries {
	return CarriedEntries{
		Refutations: []Refutation{
			{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: strings.Repeat("a", 64), FindingText: "first ask", Ground: "disproven", Reason: "tested", Evidence: "abc123", By: "operator", CreatedAt: "2026-10-20T00:00:00Z"},
			{Slot: "R2", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("b", 64), FindingText: "second ask", Ground: "disproven", Reason: "tested too", Evidence: "abc123", By: "operator", CreatedAt: "2026-10-20T00:00:00Z"},
		},
		Deferrals: []Deferral{{Slot: "R3", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("c", 64), FindingText: "future ask", Bead: "mindspec-followup", By: "operator", CreatedAt: "2026-10-20T00:00:00Z"}},
		Rebinds:   []Rebind{{Round: 1, FromSHA: "old123", ToSHA: "abc123", Paths: []string{"OWNERSHIP.yaml"}, Reason: "ownership", CreatedAt: "2026-10-20T00:00:00Z"}},
	}
}
func createCarryFixture(t *testing.T, legacy bool) (string, CreateInput, CarriedEntries) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "p")
	in := roundCreateInput()
	entries := createEntries()
	var p Panel
	if !legacy {
		createMust(t, dir, in)
		p = load(dir).Panel
		for _, r := range entries.Refutations {
			createAppend(t, dir, RefutationRecord{Record: RecordRefutation, Spec: in.Spec, Panel: "p", Refutation: r})
		}
		for _, d := range entries.Deferrals {
			createAppend(t, dir, DeferralRecord{Record: RecordDeferral, Spec: in.Spec, Panel: "p", Deferral: d})
		}
		for _, r := range entries.Rebinds {
			createAppend(t, dir, RebindRecord{Record: RecordRebind, Spec: in.Spec, Panel: "p", SubjectDigest: in.SubjectDigest, Rebind: r})
		}
	} else {
		p = Panel{Spec: in.Spec, Target: in.Target, Round: 1, ExpectedReviewers: 4, ReviewedHeadSHA: in.ReviewedHeadSHA}
		// Hand-authored legacy entries do not have the new finding metadata.
		entries.Refutations = []Refutation{{Slot: "F1", Round: 1, Reason: "old first", Evidence: "abc123"}, {Slot: "codex-R4", Round: 1, Reason: "old second", Evidence: "abc123"}}
	}
	p.Refutations, p.Deferrals, p.Rebinds = entries.Refutations, entries.Deferrals, entries.Rebinds
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	createWrite(t, dir, "R1-round-1.json", `{"verdict":"REQUEST_CHANGES","rationale":"ask"}`)
	in.Round = 2
	return dir, in, entries
}
func assertCreateEntries(t *testing.T, dir string, want CarriedEntries) {
	t.Helper()
	p := load(dir).Panel
	got := CarriedEntries{p.Refutations, p.Deferrals, p.Rebinds}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("carried entries changed:\ngot %+v\nwant %+v", got, want)
	}
	if field, err := CheckHandEdits(dir, p); err != nil || field != "" {
		t.Fatalf("unbacked registration %q: %v", field, err)
	}
}
func TestCreate_CarriesRefutationsAcrossRoundBump(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			dir, in, want := createCarryFixture(t, legacy)
			for round := 2; round <= 4; round++ {
				in.Round = round
				createMust(t, dir, in)
				assertCreateEntries(t, dir, want)
				if len(load(dir).Panel.Refutations) != 2 {
					t.Fatalf("round %d: both refutations must survive", round)
				}
			}
		})
	}
}
func TestCreate_LegacyBumpRecordsCarried(t *testing.T) {
	dir, in, want := createCarryFixture(t, true)
	createMust(t, dir, in)
	events := createRoundEvents(t, dir)
	if len(events) != 1 || !reflect.DeepEqual(events[0].Carried, want) {
		t.Fatalf("first round event must baseline all legacy entries: %+v", events)
	}
	assertCreateEntries(t, dir, want)
	// Later events must not replace the first event's baseline.
	for round := 3; round <= 4; round++ {
		in.Round = round
		createMust(t, dir, in)
		assertCreateEntries(t, dir, want)
		events = createRoundEvents(t, dir)
		if len(events) != round-1 || !reflect.DeepEqual(events[0].Carried, want) {
			t.Fatalf("first baseline changed: %+v", events)
		}
		if !reflect.DeepEqual(events[len(events)-1].Carried, CarriedEntries{}) {
			t.Fatalf("later baseline: %+v", events)
		}
	}
}
func TestCreate_DropsUnbackedEntries(t *testing.T) {
	dir, in, want := createCarryFixture(t, false)
	p := load(dir).Panel
	p.Refutations = append(p.Refutations, Refutation{Slot: "manual", Round: 1, Reason: "unbacked"})
	p.Deferrals[0].Bead = "mindspec-manual"
	p.Rebinds = append(p.Rebinds, Rebind{Round: 1, FromSHA: "forged", ToSHA: "manual"})
	// Timestamp changes are explicitly ignored, but preserved in the snapshot.
	p.Refutations[0].CreatedAt = "2026-10-20T00:01:00Z"
	want.Refutations[0].CreatedAt = p.Refutations[0].CreatedAt
	want.Deferrals = nil
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	var reports []string
	in.ReportDropped = func(s string) { reports = append(reports, s) }
	createMust(t, dir, in)
	assertCreateEntries(t, dir, want)
	if len(reports) != 3 {
		t.Fatalf("each drop must be named: %v", reports)
	}
	for i, needle := range []string{"refutations[2] slot manual", "deferrals[0] slot R3 round 1 finding ccr[0] bead mindspec-manual", "rebinds[1] round 1 from forged"} {
		if !strings.Contains(reports[i], needle) {
			t.Fatalf("drop not identified: %q want %q", reports[i], needle)
		}
	}
}
func TestCreate_RevivalAppendsAbandonHistory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	in := roundCreateInput()
	createMust(t, dir, in)
	p := load(dir).Panel
	p.Abandoned = true
	p.AbandonReason = "operator: rework"
	p.AbandonHistory = []AbandonHistory{{Reason: "earlier", AbandonedAt: "old", RevivedAt: "old"}}
	createAppend(t, dir, AbandonRecord{Record: RecordAbandon, Spec: in.Spec, Panel: "p", Reason: p.AbandonReason, By: "operator", CreatedAt: "2026-10-20T00:00:00Z"})
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	in.Round = 2
	createMust(t, dir, in)
	got := load(dir).Panel
	if got.Abandoned || got.AbandonReason != "" || len(got.AbandonHistory) != 2 {
		t.Fatalf("revival history: %+v", got)
	}
	h := got.AbandonHistory[1]
	if h.Reason != p.AbandonReason || h.AbandonedAt != "2026-10-20T00:00:00Z" || got.AbandonHistory[0] != p.AbandonHistory[0] {
		t.Fatalf("history: %+v", got.AbandonHistory)
	}
	if _, err := time.Parse(time.RFC3339Nano, h.RevivedAt); err != nil {
		t.Fatal(err)
	}
	in.Round = 3
	createMust(t, dir, in)
	if len(load(dir).Panel.AbandonHistory) != 2 {
		t.Fatal("must append only on revival")
	}
}
func TestCreate_ArchivesBriefOnBump(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	in := roundCreateInput()
	in.Round = 2
	createMust(t, dir, in)
	original := createRead(t, dir, "BRIEF.md") + "\r\noperator notes\r\n"
	createWrite(t, dir, "BRIEF.md", original)
	createWrite(t, dir, "R1-round-2.json", `{"verdict":"APPROVE"}`)
	in.Round = 3
	createMust(t, dir, in)
	if got := createRead(t, dir, "BRIEF-round-2.md"); got != original {
		t.Fatalf("archive changed: %q", got)
	}
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.LatestRound != 2 || len(res.Verdicts) != 1 {
		t.Fatalf("archive counted as verdict: %+v", res)
	}
	if !strings.HasSuffix(createRead(t, dir, "BRIEF.md"), "\r\noperator notes\r\n") {
		t.Fatal("authored body changed")
	}
}
func TestCreate_NeverOverwritesArchive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	in := roundCreateInput()
	createMust(t, dir, in)
	createWrite(t, dir, "BRIEF-round-1.md", "original archive\x1b")
	createWrite(t, dir, "BRIEF.md", "later working brief")
	in.Round = 2
	createMust(t, dir, in)
	if got := createRead(t, dir, "BRIEF-round-1.md"); got != "original archive\x1b" {
		t.Fatalf("existing archive overwritten: %q", got)
	}
}
func TestCreate_PriorRoundRegion(t *testing.T) {
	for _, payload := range []string{briefHeaderOpen, briefHeaderClose,
		"<!-- mindspec:prior-round -->", "<!-- /mindspec:prior-round -->", "```", "```text",
		"<!--  /mindspec:panel-header -->"} {
		t.Run(payload, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "p")
			in := roundCreateInput()
			createMust(t, dir, in)
			createWrite(t, dir, "consolidated-round-1.md", payload)
			in.Round = 2
			createMust(t, dir, in)
			brief := createRead(t, dir, "BRIEF.md")
			for _, marker := range []string{briefHeaderOpen, briefHeaderClose,
				"<!-- mindspec:prior-round -->", "<!-- /mindspec:prior-round -->"} {
				if got := len(findMarkerLines(brief, marker)); got != 1 {
					t.Fatalf("region must have one %q line, got %d: %q", marker, got, brief)
				}
			}
			if !strings.Contains(brief, "> "+payload+"\n") {
				t.Fatalf("missing neutralised prior content: %q", brief)
			}
			in.Round = 3
			createMust(t, dir, in)
			brief = createRead(t, dir, "BRIEF.md")
			for _, marker := range []string{briefHeaderOpen, briefHeaderClose,
				"<!-- mindspec:prior-round -->", "<!-- /mindspec:prior-round -->"} {
				if got := len(findMarkerLines(brief, marker)); got != 1 {
					t.Fatalf("next bump has %d %q lines: %q", got, marker, brief)
				}
			}
		})
	}
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprint(present), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "p")
			in := roundCreateInput()
			in.Round = 2
			createMust(t, dir, in)
			if present {
				createWrite(t, dir, "consolidated-round-2.md", "prior ask \x1b[31mred")
			}
			in.Round = 3
			createMust(t, dir, in)
			header := extractHeader(t, createRead(t, dir, "BRIEF.md"))
			if !strings.Contains(header, "<!-- mindspec:prior-round -->") || !strings.Contains(header, "<!-- /mindspec:prior-round -->") {
				t.Fatal("missing prior-round region")
			}
			if present {
				if strings.ContainsRune(header, '\x1b') || !strings.Contains(header, `prior ask \x1b[31mred`) {
					t.Fatalf("unsafe or missing prior content: %q", header)
				}
			} else if !strings.Contains(header, "none recorded") {
				t.Fatal("missing absence text")
			}
		})
	}
}
func TestCreate_RosterCarriedForward(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	in := roundCreateInput()
	createMust(t, dir, in)
	want := load(dir).Panel.Roster
	// Neither hand edits nor changed config choose the next roster.
	p := load(dir).Panel
	p.Roster[0].Model = "manual"
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	in.Round = 2
	in.Roster = createRoster(6)
	in.Roster[0].Model = "new-config"
	createMust(t, dir, in)
	got := load(dir).Panel
	if !reflect.DeepEqual(got.Roster, want) || got.ExpectedReviewers != 4 {
		t.Fatalf("roster reset on bump: %+v", got)
	}
	rounds := createRoundEvents(t, dir)
	if len(rounds) != 2 || !reflect.DeepEqual(rounds[1].Roster, want) {
		t.Fatalf("round roster differs: %+v", rounds)
	}
}
func TestCreate_ReviewersOverridesCarriedRoster(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	in := roundCreateInput()
	createMust(t, dir, in)
	in.Round = 2
	in.ReviewersOverride = true
	in.Roster = createRoster(3)
	createMust(t, dir, in)
	p := load(dir).Panel
	if len(p.Roster) != 3 || p.ExpectedReviewers != 3 {
		t.Fatalf("override ignored: %+v", p)
	}
	rounds := createRoundEvents(t, dir)
	if len(rounds) != 2 || !reflect.DeepEqual(rounds[1].Roster, p.Roster) || rounds[1].ExpectedReviewers != 3 {
		t.Fatalf("round not co-written: %+v", rounds)
	}
	if field, err := CheckHandEdits(dir, p); err != nil || field != "" {
		t.Fatalf("unbacked %s %v", field, err)
	}
	want := p.Roster
	in.Round = 3
	in.ReviewersOverride = false
	in.Roster = createRoster(6)
	createMust(t, dir, in)
	p = load(dir).Panel
	if !reflect.DeepEqual(p.Roster, want) || p.ExpectedReviewers != 3 {
		t.Fatalf("round 3 must keep round 2 roster: %+v", p)
	}
	rounds = createRoundEvents(t, dir)
	if len(rounds) != 3 || !reflect.DeepEqual(rounds[2].Roster, want) || rounds[2].ExpectedReviewers != 3 {
		t.Fatalf("round 3 event must keep round 2 roster: %+v", rounds)
	}
	if field, err := CheckHandEdits(dir, p); err != nil || field != "" {
		t.Fatalf("round 3 unbacked %s %v", field, err)
	}
}
func TestCreate_BriefListsRoster(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	in := roundCreateInput()
	createMust(t, dir, in)
	header := extractHeader(t, createRead(t, dir, "BRIEF.md"))
	for _, r := range in.Roster {
		if !strings.Contains(header, r.Slot+" "+r.Model+" "+r.Lens+"\n") {
			t.Fatalf("missing roster slot %+v: %s", r, header)
		}
	}
	p := load(dir).Panel
	rounds := createRoundEvents(t, dir)
	if p.Telemetry != "required" || len(rounds) != 1 || rounds[0].Telemetry != "required" {
		t.Fatal("required telemetry not co-written")
	}
}

func TestCreate_LegacyPathRefusesBeforeWrite(t *testing.T) {
	for _, kind := range []string{"refutation", "deferral", "rebind"} {
		for _, token := range []string{"/tmp/", "/Users/"} {
			t.Run(kind+token, func(t *testing.T) {
				dir, in, _ := createCarryFixture(t, true)
				p := load(dir).Panel
				var entry string
				switch kind {
				case "refutation":
					p.Refutations[0].Finding = "ccr[1]"
					p.Refutations[0].Evidence = "see " + token + "probe/result"
					entry = "refutation[0] slot F1 finding ccr[1]"
				case "deferral":
					p.Deferrals[0].FindingText = "see " + token + "probe/result"
					entry = "deferral[0] slot R3 finding ccr[0]"
				case "rebind":
					p.Rebinds[0].Reason = "see " + token + "probe/result"
					entry = "rebind[0] round 1 from old123 to abc123"
				}
				if err := WriteSnapshot(dir, p); err != nil {
					t.Fatal(err)
				}
				// A legacy ledger can already contain non-verb records.
				createAppend(t, dir, CoverageManifest{Record: RecordPanelManifest, Spec: in.Spec, Panel: "p", Gate: in.Gate, Round: 1,
					Slots: []ManifestSlot{{Slot: "R1", Model: "model-a", Verdict: VerdictApprove}}, CreatedAt: ledgerTime})
				createWrite(t, dir, "BRIEF.md", "legacy brief")
				beforePanel := createRead(t, dir, FileName)
				beforeLedger := createRead(t, dir, dispositionsFileName)
				beforeFiles, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				err = Create(dir, in)
				var pathErr *CreateLegacyPathError
				if !errors.As(err, &pathErr) || !strings.Contains(err.Error(), entry) {
					t.Fatalf("missing entry-specific path refusal: %v", err)
				}
				wantRecovery := "remove the absolute path from " + entry + " in the legacy panel.json, then run create again"
				if pathErr.Recovery() != wantRecovery {
					t.Fatalf("wrong recovery: %q", pathErr.Recovery())
				}
				if got := createRead(t, dir, FileName); got != beforePanel {
					t.Fatal("refusal changed panel.json")
				}
				if got := createRead(t, dir, dispositionsFileName); got != beforeLedger {
					t.Fatal("refusal changed ledger")
				}
				if got := createRead(t, dir, "BRIEF.md"); got != "legacy brief" {
					t.Fatal("refusal changed BRIEF")
				}
				afterFiles, err := os.ReadDir(dir)
				if err != nil || !reflect.DeepEqual(beforeFiles, afterFiles) {
					t.Fatalf("refusal changed directory: %v %v", afterFiles, err)
				}
			})
		}
	}
}

func TestCreate_DropsLegacyApproveThreshold(t *testing.T) {
	dir, in, _ := createCarryFixture(t, true)
	p := load(dir).Panel
	p.ApproveThresholdExpr = "unanimous"
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(createRead(t, dir, FileName)), &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["approve_threshold"]; !exists {
		t.Fatal("fixture must contain approve_threshold")
	}
	createMust(t, dir, in)
	// Unmarshal into a fresh map: removed keys remain in a reused map.
	fields = nil
	if err := json.Unmarshal([]byte(createRead(t, dir, FileName)), &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["approve_threshold"]; exists {
		t.Fatal("round bump retained legacy approve_threshold")
	}
}

func TestCreate_DropReportPrecedesBriefWrite(t *testing.T) {
	dir, in, _ := createCarryFixture(t, false)
	p := load(dir).Panel
	p.Refutations = append(p.Refutations, Refutation{Slot: "manual", Round: 1, Finding: "ccr[0]"})
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	var reports []string
	in.ReportDropped = func(message string) {
		reports = append(reports, message)
		if len(load(dir).Panel.Refutations) != 2 {
			t.Fatal("drop report preceded snapshot write")
		}
		// Inject a BRIEF write failure after the report, with no permissions dependency.
		brief := filepath.Join(dir, "BRIEF.md")
		if err := os.Rename(brief, filepath.Join(dir, "saved-BRIEF.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(brief, 0755); err != nil {
			t.Fatal(err)
		}
	}
	err := Create(dir, in)
	if err == nil || !strings.Contains(err.Error(), "write "+filepath.Join(dir, "BRIEF.md")) {
		t.Fatalf("expected injected BRIEF write failure: %v", err)
	}
	if len(reports) != 1 || !strings.Contains(reports[0], "refutations[2] slot manual round 1 finding ccr[0]") {
		t.Fatalf("lost dropped-entry report: %v", reports)
	}
}
