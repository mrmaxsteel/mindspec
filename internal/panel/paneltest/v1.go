package paneltest

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// CreateV1 seeds the historical file-based round used by verb tests.
// Create tests check the current v2 contract.
func CreateV1(dir string, in panel.CreateInput) error {
	beadID := ""
	if in.BeadID != nil {
		beadID = *in.BeadID
	}
	binding, err := panel.CreateBeadID(in.Gate, in.Target, beadID)
	if err != nil {
		return err
	}
	var p panel.Panel
	if data, err := os.ReadFile(filepath.Join(dir, panel.FileName)); err == nil {
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
	}
	p.BeadID = binding
	p.Spec = in.Spec
	p.Target = in.Target
	p.Round = in.Round
	p.Roster = in.Roster
	p.ExpectedReviewers = len(in.Roster)
	p.ReviewedHeadSHA = in.ReviewedHeadSHA
	p.SubjectDigest = in.SubjectDigest
	p.Gate = in.Gate
	p.Telemetry = "required"
	spec := p.Spec
	if spec == "" {
		spec = "adhoc"
	}
	event := panel.RoundRecord{Record: panel.RecordRound, Spec: spec, Panel: filepath.Base(dir), Round: p.Round, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, BeadID: p.BeadID, Roster: p.Roster, ExpectedReviewers: p.ExpectedReviewers, Telemetry: p.Telemetry, CreatedAt: "2026-10-20T00:00:00Z"}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := panel.AppendRecords(dir, raw); err != nil {
		return err
	}
	return panel.WriteSnapshot(dir, p)
}
