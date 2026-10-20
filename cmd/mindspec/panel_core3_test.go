package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const core3Time = "2026-10-20T00:00:00Z"

func core3JSON(t *testing.T, dir, name string, value interface{}) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func core3Append(t *testing.T, dir string, values ...interface{}) {
	t.Helper()
	var lines [][]byte
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, raw)
	}
	if err := panel.AppendRecords(dir, lines...); err != nil {
		t.Fatal(err)
	}
}
func core3Fixture(t *testing.T, root, slug, gate string, rc bool) panel.Registration {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	id := "mindspec-core3.1"
	p := panel.Panel{Spec: "130-core3", Target: "spec/130-core3", Gate: gate, Round: 1,
		ExpectedReviewers: 2, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", Telemetry: "required",
		Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}}}
	if gate == "bead" {
		p.BeadID = &id
		p.Target = "bead/" + id
	}
	event := panel.RoundRecord{Record: panel.RecordRound, Spec: p.Spec, Panel: slug, Round: p.Round,
		ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate,
		BeadID: p.BeadID, Roster: p.Roster, ExpectedReviewers: p.ExpectedReviewers, Telemetry: p.Telemetry, CreatedAt: core3Time}
	core3Append(t, dir, event)
	core3JSON(t, dir, panel.FileName, p)
	verdict := "APPROVE"
	if rc {
		verdict = "REQUEST_CHANGES"
	}
	core3JSON(t, dir, "R1-round-1.json", map[string]interface{}{"verdict": verdict, "rationale": "test finding"})
	core3JSON(t, dir, "R2-round-1.json", map[string]string{"verdict": "APPROVE"})
	if rc {
		res, err := panel.Tally(dir)
		if err != nil {
			t.Fatal(err)
		}
		ref := panel.Refutation{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: res.Verdicts[0].SlotSHA256,
			FindingText: "test finding", Ground: "disproven", Reason: "tested", Evidence: "abc123", By: "operator", CreatedAt: core3Time}
		core3Append(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: p.Spec, Panel: slug, Refutation: ref})
		p.Refutations = []panel.Refutation{ref}
		core3JSON(t, dir, panel.FileName, p)
	}
	return panel.Registration{Dir: dir, Panel: p}
}
func core3Cover(t *testing.T, reg panel.Registration) {
	t.Helper()
	row := panel.DispositionRow{Record: panel.RecordDisposition, ID: "core3-row", Spec: reg.Panel.Spec, Gate: reg.Panel.Gate,
		Panel: reg.Slug(), Reviewer: "R1", Model: "model-a", Severity: "major", Summary: "test finding", ConvergentWith: []string{},
		Disposition: "false-contamination", CreatedAt: core3Time}
	core3Append(t, reg.Dir, row)
}
func core3Manifest(t *testing.T, reg panel.Registration) {
	t.Helper()
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.PassRule(true) {
		t.Fatalf("fixture must pass without freshness: %+v", res)
	}
	if err := panel.WriteCoverageManifest(res); err != nil {
		t.Fatal(err)
	}
}

func core3Tally(t *testing.T, reg panel.Registration, stale bool) error {
	t.Helper()
	root := filepath.Dir(filepath.Dir(reg.Dir))
	withTestChdir(t, root)
	stubNonBeadRevParse(t, func(_, _ string) (string, error) {
		if stale {
			return "advanced", nil
		}
		return reg.Panel.ReviewedHeadSHA, nil
	})
	_, err := runPanelVerbCmd("tally", reg.Slug())
	return err
}
func core3Manifests(t *testing.T, dir string) []panel.CoverageManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "dispositions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []panel.CoverageManifest
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var m panel.CoverageManifest
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatal(err)
		}
		if m.Record == panel.RecordPanelManifest {
			out = append(out, m)
		}
	}
	return out
}
func TestTally_WritesManifestOnAllow(t *testing.T) {
	root := mkPanelTestRoot(t, "")
	reg := core3Fixture(t, root, "coverage", "final_review", false)
	core3JSON(t, reg.Dir, "outsider-round-1.json", map[string]string{"verdict": "APPROVE"})
	if err := core3Tally(t, reg, false); err != nil {
		t.Fatal(err)
	}
	ms := core3Manifests(t, reg.Dir)
	if len(ms) != 1 || len(ms[0].Slots) != 2 || ms[0].Slots[0].Slot != "R1" || ms[0].Slots[1].Slot != "R2" {
		t.Fatalf("manifest must contain exactly roster slots: %+v", ms)
	}
	if err := panel.CheckCore3(reg); err != nil {
		t.Fatal(err)
	}
}
func TestTally_AppendsManifestOnChange(t *testing.T) {
	root := mkPanelTestRoot(t, "")
	reg := core3Fixture(t, root, "coverage", "final_review", false)
	if err := core3Tally(t, reg, false); err != nil {
		t.Fatal(err)
	}
	if err := core3Tally(t, reg, false); err != nil {
		t.Fatal(err)
	}
	if n := len(core3Manifests(t, reg.Dir)); n != 1 {
		t.Fatalf("unchanged tally appended: %d", n)
	}
	core3JSON(t, reg.Dir, "R1-round-1.json", map[string]string{"verdict": "REQUEST_CHANGES", "rationale": "changed finding"})
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	ref := panel.Refutation{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: res.Verdicts[0].SlotSHA256, FindingText: "changed finding", Ground: "disproven", Reason: "tested", Evidence: "abc123", By: "operator", CreatedAt: core3Time}
	core3Append(t, reg.Dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: reg.Panel.Spec, Panel: reg.Slug(), Refutation: ref})
	reg.Panel.Refutations = []panel.Refutation{ref}
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	core3Cover(t, reg)
	if err := core3Tally(t, reg, false); err != nil {
		t.Fatal(err)
	}
	ms := core3Manifests(t, reg.Dir)
	if len(ms) != 2 || ms[1].Slots[0].Verdict != "REQUEST_CHANGES" {
		t.Fatalf("changed derivation must append: %+v", ms)
	}
	if err := panel.CheckCore3(reg); err != nil {
		t.Fatal(err)
	}
}
func TestTally_AppendsManifestPerRound(t *testing.T) {
	root := mkPanelTestRoot(t, "")
	reg := core3Fixture(t, root, "coverage", "final_review", false)
	if err := core3Tally(t, reg, false); err != nil {
		t.Fatal(err)
	}
	reg.Panel.Round = 2
	p := reg.Panel
	core3Append(t, reg.Dir, panel.RoundRecord{Record: panel.RecordRound, Spec: p.Spec, Panel: reg.Slug(), Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, BeadID: p.BeadID, Roster: p.Roster, ExpectedReviewers: 2, Telemetry: p.Telemetry, CreatedAt: "2026-10-20T01:00:00Z"})
	core3JSON(t, reg.Dir, panel.FileName, p)
	for _, slot := range []string{"R1", "R2"} {
		core3JSON(t, reg.Dir, slot+"-round-2.json", map[string]string{"verdict": "APPROVE"})
	}
	if err := core3Tally(t, reg, false); err != nil {
		t.Fatal(err)
	}
	ms := core3Manifests(t, reg.Dir)
	if len(ms) != 2 || ms[0].Round != 1 || ms[1].Round != 2 {
		t.Fatalf("equal verdicts at new round must append: %+v", ms)
	}
}
func TestTally_ManifestWhenOnlyStale(t *testing.T) {
	root := mkPanelTestRoot(t, "")
	reg := core3Fixture(t, root, "coverage", "final_review", true)
	core3Cover(t, reg)
	subjectCLIGit(t, root, "init", "-q", "-b", "main")
	subjectCLIWrite(t, root, "subject.txt", "changed subject")
	subjectCLIGit(t, root, "add", "subject.txt")
	subjectCLIGit(t, root, "commit", "-qm", "subject")
	tip := subjectCLIGit(t, root, "rev-parse", "HEAD")
	withTestChdir(t, root)
	stubNonBeadRevParse(t, func(_, _ string) (string, error) { return tip, nil })
	_, err := runPanelVerbCmd("tally", reg.Slug())
	if err == nil || !strings.Contains(err.Error(), "commits landed after review") {
		t.Fatalf("stale target must still block tally: %v", err)
	}
	if n := len(core3Manifests(t, reg.Dir)); n != 1 {
		t.Fatalf("stale tally must record coverage: %d", n)
	}
	if err := panel.CheckCore3(reg); err != nil {
		t.Fatalf("coverage must ignore staleness: %v", err)
	}
}

func TestTally_NoManifestWhenPassRuleFails(t *testing.T) {
	root := mkPanelTestRoot(t, "")
	reg := core3Fixture(t, root, "coverage", "final_review", false)
	core3JSON(t, reg.Dir, "R1-round-1.json", map[string]string{"verdict": "REQUEST_CHANGES", "rationale": "unresolved finding"})
	if err := core3Tally(t, reg, false); err == nil {
		t.Fatal("unresolved finding must block tally")
	}
	if ms := core3Manifests(t, reg.Dir); len(ms) != 0 {
		t.Fatalf("blocked panel must record no manifest: %+v", ms)
	}
}

func TestTally_NoManifestOnAbandonedOrLegacy(t *testing.T) {
	for _, scenario := range []string{"abandoned", "legacy", "single legacy marker"} {
		t.Run(scenario, func(t *testing.T) {
			root := mkPanelTestRoot(t, "")
			reg := core3Fixture(t, root, "coverage", "final_review", false)
			if scenario == "abandoned" {
				reg.Panel.Abandoned = true
				reg.Panel.AbandonReason = "operator: superseded"
				core3Append(t, reg.Dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: reg.Panel.Spec, Panel: reg.Slug(), Reason: reg.Panel.AbandonReason, By: "operator", CreatedAt: core3Time})
			} else {
				reg.Panel.Telemetry = ""
				if scenario == "legacy" {
					if err := os.Rename(filepath.Join(reg.Dir, "dispositions.jsonl"), filepath.Join(reg.Dir, "saved-ledger")); err != nil {
						t.Fatal(err)
					}
					core3Cover(t, reg)
				}
			}
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			err := core3Tally(t, reg, false)
			if scenario == "single legacy marker" {
				if err == nil || !strings.Contains(err.Error(), "only one legacy marker") {
					t.Fatalf("one marker must block tally: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if ms := core3Manifests(t, reg.Dir); len(ms) != 0 {
				t.Fatalf("%s panel must record no manifest: %+v", scenario, ms)
			}
		})
	}
}

func TestTally_Core3RecoveryOperands(t *testing.T) {
	reg := panel.Registration{Dir: "/review/quote' ; $(echo probe)", Panel: panel.Panel{Spec: "130-core3", Gate: "final_review"}}
	err := panelCoverageWriteFailure(reg, errors.New("write failed"))
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Fatalf("missing recovery: %v", err)
	}
	recovery := strings.Split(err.Error(), "recovery: ")[1]
	if matches := guard.FindFloorMatches(recovery); len(matches) != 0 {
		t.Fatalf("unsafe recovery: %+v %s", matches, recovery)
	}
}
