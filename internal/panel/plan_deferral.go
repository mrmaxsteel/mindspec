package panel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// PlanDeferral pins one finding for resolution in the plan.
type PlanDeferral struct {
	ID            string `json:"id"`
	Slot          string `json:"slot"`
	Round         int    `json:"round"`
	Finding       string `json:"finding"`
	FindingSHA256 string `json:"finding_sha256"`
	FindingText   string `json:"finding_text"`
	By            string `json:"by"`
	CreatedAt     string `json:"created_at"`
}

type PlanDeferralRecord struct {
	Record string `json:"record"`
	Spec   string `json:"spec"`
	Panel  string `json:"panel"`
	PlanDeferral
}

// PlanDeferralID is stable across retries, actors and timestamps.
func PlanDeferralID(spec, panel, slot string, round int, finding Finding) string {
	b, _ := json.Marshal([]any{spec, panel, slot, round, finding.ID, finding.SHA256})
	sum := sha256.Sum256(b)
	return "plan-" + hex.EncodeToString(sum[:])
}

func matchesPlanDeferral(entry PlanDeferral, events []PlanDeferral) bool {
	entry.CreatedAt = ""
	for _, event := range events {
		event.CreatedAt = ""
		if entry == event {
			return true
		}
	}
	return false
}

func planDeferralsFromLedger(lines [][]byte) ([]PlanDeferral, error) {
	var out []PlanDeferral
	for _, line := range lines {
		var event PlanDeferralRecord
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, err
		}
		if event.Record == RecordDeferredToPlan {
			out = append(out, event.PlanDeferral)
		}
	}
	return out, nil
}

// EffectivePlanDeferrals returns only ledger-backed, current text matches.
// Other gates do not use plan deferrals to discharge findings.
func (r *Result) EffectivePlanDeferrals() []PlanDeferral {
	if r.Panel == nil || r.Panel.Gate != "spec_approve" || r.IntegrityErr != nil {
		return nil
	}
	lines, err := ReadLedger(r.Dir)
	if err != nil {
		return nil
	}
	backed, err := planDeferralsFromLedger(lines)
	if err != nil {
		return nil
	}
	var out []PlanDeferral
	for _, d := range r.Panel.PlanDeferrals {
		if d.FindingSHA256 == "" || !matchesPlanDeferral(d, backed) {
			continue
		}
		for _, v := range r.Verdicts {
			if r.refutationApplies(v, Refutation{Slot: d.Slot, Round: d.Round, Finding: d.Finding, FindingSHA256: d.FindingSHA256}) {
				out = append(out, d)
			}
		}
	}
	return out
}
