// Package paneltest provides local v2 ledger fixtures for panel consumers.
package paneltest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

const CreatedAt = "2026-10-21T00:00:00Z"
const ReviewedSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type Fixture struct {
	Dir     string
	Panel   panel.Panel
	SlotDir string
}

func New(t testing.TB) Fixture { return newFixture(t, 2) }

// NewLegacy creates a v1 round without verdict_schema.
func NewLegacy(t testing.TB) Fixture { return newFixture(t, 0) }

func newFixture(t testing.TB, schema int) Fixture {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "reviews", "p")
	p := panel.Panel{Spec: "test", Target: "main", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: ReviewedSHA, SubjectDigest: "sha256:fixture", Gate: "adhoc", Telemetry: "required", Roster: []panel.RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "correctness"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "correctness"}}}
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	f := Fixture{Dir: dir, Panel: p, SlotDir: filepath.Join(root, "scratch", "test", "p", "r1", "R1")}
	f.Round(t, schema)
	return f
}
func (f Fixture) Round(t testing.TB, schema int) {
	t.Helper()
	p := f.Panel
	Seed(t, f.Dir, panel.RoundRecord{Record: panel.RecordRound, VerdictSchema: schema, Spec: p.Spec, Panel: filepath.Base(f.Dir), Round: p.Round, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, BeadID: p.BeadID, Roster: p.Roster, ExpectedReviewers: p.ExpectedReviewers, Telemetry: p.Telemetry, CreatedAt: CreatedAt})
}

// LegacyRound writes a round event with no verdict_schema for v1 test migration.
func (f Fixture) LegacyRound(t testing.TB) { f.Round(t, 0) }
func Seed(t testing.TB, dir string, records ...any) {
	t.Helper()
	raw := make([][]byte, len(records))
	for i, r := range records {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		raw[i] = b
	}
	if err := panel.AppendRecords(dir, raw...); err != nil {
		t.Fatal(err)
	}
}
func Verdict(verdict string) panel.VerdictV2 {
	v := panel.VerdictV2{Schema: 2, Slot: "R1", Round: 1, ReviewedSHA: ReviewedSHA, Status: "final", Verdict: verdict, Model: "m", Rationale: "reviewed", ConcreteChangesRequired: []panel.ChangeRequired{}, CommandsRun: []panel.CommandRun{}}
	if verdict == "REQUEST_CHANGES" {
		v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "fix the property", Basis: "traced"}}
	}
	if verdict == "VOID" {
		reason := "cannot review"
		v.VoidReason = &reason
		v.Rationale = ""
	}
	return v
}
func Bytes(t testing.TB, v panel.VerdictV2) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func (f Fixture) Launch(status string, round, attempt int) panel.LaunchRecord {
	return panel.LaunchRecord{Record: panel.RecordLaunch, Spec: f.Panel.Spec, Panel: filepath.Base(f.Dir), Slot: "R1", Round: round, Attempt: attempt, Family: "harness", Model: "m", Status: status, RunnerKind: "harness", RunnerSource: "default", CreatedAt: CreatedAt, PGID: 100, ArgvSHA256: strings.Repeat("b", 64), NonceSHA256: strings.Repeat("c", 64)}
}
func (f Fixture) Attempt(t testing.TB, k int) string {
	t.Helper()
	dir := filepath.Join(f.SlotDir, "attempt-"+itoa(k))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}
func itoa(k int) string { b, _ := json.Marshal(k); return string(b) }
func Resolve(p panel.Panel, s panel.RosterSlot) (panel.SubmitResolution, error) {
	return panel.SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "harness", RunnerSource: "default"}, nil
}
func (f Fixture) Target() panel.SubmitTarget {
	return panel.SubmitTarget{Dir: f.Dir, Spec: f.Panel.Spec, Panel: filepath.Base(f.Dir), Slot: "R1", Round: f.Panel.Round, SlotDir: f.SlotDir}
}
