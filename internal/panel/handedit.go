package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

// LegacyPanel checks BOTH markers from Terms, including field presence (an
// explicitly empty telemetry field is not an absent marker). An unregistered
// directory without round events is accepted here for legacy disposition tools.
func LegacyPanel(dir string) (bool, error) {
	lines, err := ReadLedger(dir)
	if err != nil {
		return false, err
	}
	return legacyPanelFromLedger(dir, lines)
}

func legacyPanelFromLedger(dir string, lines [][]byte) (bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	var fields map[string]json.RawMessage
	if err == nil {
		if err := json.Unmarshal(data, &fields); err != nil {
			return false, err
		}
	}
	_, telemetry := fields["telemetry"]
	round := false
	for _, line := range lines {
		var disc recordDiscriminator
		if err := json.Unmarshal(line, &disc); err != nil {
			return false, err
		}
		if disc.Record == RecordRound {
			round = true
		}
	}
	if telemetry != round {
		return false, fmt.Errorf("panel registration has only one legacy marker (telemetry and round event must occur together)")
	}
	return !telemetry, nil
}

// CheckHandEdits is the library-only R5.3 check; the gate wires it in separately.
// The first return value names the first unbacked field, deterministically.
func CheckHandEdits(dir string, p Panel) (string, error) {
	lines, err := ReadLedger(dir)
	if err != nil {
		return "", err
	}
	return checkHandEditsFromLedger(dir, p, lines)
}

func checkHandEditsFromLedger(dir string, p Panel, lines [][]byte) (string, error) {
	legacy, err := legacyPanelFromLedger(dir, lines)
	if err != nil {
		return "", err
	}
	backed, err := planDeferralsFromLedger(lines)
	if err != nil {
		return "", err
	}
	for i, entry := range p.PlanDeferrals {
		if !matchesPlanDeferral(entry, backed) {
			return fmt.Sprintf("plan_deferrals[%d]", i), nil
		}
	}
	if legacy {
		return "", nil
	}
	return FirstUnbackedField(p, lines)
}

// FirstUnbackedField compares a snapshot to validated append-order ledger
// events. It does not mutate either input. Call LegacyPanel first when reading
// disk to distinguish an absent telemetry field from an explicitly empty one.
func FirstUnbackedField(p Panel, lines [][]byte) (string, error) {
	var latest, first *RoundRecord
	var refutations []Refutation
	var deferrals []Deferral
	var rebinds []Rebind
	var lastRebind *RebindRecord
	var abandoned *AbandonRecord
	for _, line := range lines {
		if err := Validate(line); err != nil {
			return "", err
		}
		var disc recordDiscriminator
		if err := json.Unmarshal(line, &disc); err != nil {
			return "", err
		}
		switch disc.Record {
		case RecordRound:
			var r RoundRecord
			if err := json.Unmarshal(line, &r); err != nil {
				return "", err
			}
			latest = &r
			if first == nil {
				first = &r
			}
			lastRebind, abandoned = nil, nil
		case RecordRefutation:
			var r Refutation
			if err := json.Unmarshal(line, &r); err != nil {
				return "", err
			}
			refutations = append(refutations, r)
		case RecordDeferral:
			var d Deferral
			if err := json.Unmarshal(line, &d); err != nil {
				return "", err
			}
			deferrals = append(deferrals, d)
		case RecordRebind:
			var r RebindRecord
			if err := json.Unmarshal(line, &r); err != nil {
				return "", err
			}
			rebinds = append(rebinds, r.Rebind)
			lastRebind = &r
		case RecordAbandon:
			var a AbandonRecord
			if err := json.Unmarshal(line, &a); err != nil {
				return "", err
			}
			abandoned = &a
		}
	}
	if (p.Telemetry != "") != (latest != nil) {
		return "telemetry", nil
	}
	if latest == nil {
		return "", nil
	} // legacy
	comparisons := []struct {
		name             string
		actual, expected interface{}
	}{
		{"round", p.Round, latest.Round},
		{"roster", p.Roster, latest.Roster},
		{"gate", p.Gate, latest.Gate},
		{"target", p.Target, latest.Target},
		{"bead_id", p.BeadID, latest.BeadID},
		{"expected_reviewers", p.ExpectedReviewers, latest.ExpectedReviewers},
		{"verdict_schema", p.VerdictSchema, latest.VerdictSchema},
		{"telemetry", p.Telemetry, latest.Telemetry},
	}
	for _, c := range comparisons {
		if !reflect.DeepEqual(c.actual, c.expected) {
			return c.name, nil
		}
	}
	if p.ExpectedReviewers != len(p.Roster) {
		return "expected_reviewers", nil
	}
	sha := latest.ReviewedHeadSHA
	digest := latest.SubjectDigest
	if lastRebind != nil {
		sha = lastRebind.ToSHA
		digest = lastRebind.SubjectDigest
	}
	if p.ReviewedHeadSHA != sha {
		return "reviewed_head_sha", nil
	}
	if p.SubjectDigest != digest {
		return "subject_digest", nil
	}
	refutations = append(refutations, first.Carried.Refutations...)
	deferrals = append(deferrals, first.Carried.Deferrals...)
	rebinds = append(rebinds, first.Carried.Rebinds...)
	for i, entry := range p.Refutations {
		if !matchesRefutation(entry, refutations) {
			return fmt.Sprintf("refutations[%d]", i), nil
		}
	}
	for i, entry := range p.Deferrals {
		if !matchesDeferral(entry, deferrals) {
			return fmt.Sprintf("deferrals[%d]", i), nil
		}
	}
	for i, entry := range p.Rebinds {
		if !matchesRebind(entry, rebinds) {
			return fmt.Sprintf("rebinds[%d]", i), nil
		}
	}
	if p.Abandoned != (abandoned != nil) {
		return "abandoned", nil
	}
	reason := ""
	if abandoned != nil {
		reason = abandoned.Reason
	}
	if p.AbandonReason != reason {
		return "abandon_reason", nil
	}
	return "", nil
}

func matchesRefutation(entry Refutation, events []Refutation) bool {
	entry.CreatedAt = ""
	for _, event := range events {
		event.CreatedAt = ""
		if reflect.DeepEqual(entry, event) {
			return true
		}
	}
	return false
}
func matchesDeferral(entry Deferral, events []Deferral) bool {
	entry.CreatedAt = ""
	for _, event := range events {
		event.CreatedAt = ""
		if reflect.DeepEqual(entry, event) {
			return true
		}
	}
	return false
}
func matchesRebind(entry Rebind, events []Rebind) bool {
	entry.CreatedAt = ""
	for _, event := range events {
		event.CreatedAt = ""
		if reflect.DeepEqual(entry, event) {
			return true
		}
	}
	return false
}
