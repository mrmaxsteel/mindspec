package complete

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// PanelDeferredEntry retains finding boundaries after a slot-level discharge.
// The audit must not read later verdicts or refutations to reconstruct them.
type PanelDeferredEntry struct {
	panel.Deferral
	FindingTexts []string `json:"finding_texts,omitempty"`
}

// CheckPanelDeferrals is shared by completion and non-bead approval. The
// audited form is retained for callers with item-level historical records.
func CheckPanelDeferrals(reg panel.Registration, entries []panel.Deferral, audited bool) error {
	if audited {
		var audit []PanelDeferredEntry
		for _, d := range entries {
			audit = append(audit, PanelDeferredEntry{Deferral: d, FindingTexts: []string{d.FindingText}})
		}
		return CheckAuditedPanelDeferrals(reg, audit)
	}
	_, err := checkedPanelDeferrals(reg, entries)
	return err
}

func deferralFailure(reg panel.Registration, d panel.Deferral, audited bool, err error) error {
	recovery := fmt.Sprintf("bd reopen %s", core3Quote(d.Bead))
	if !audited {
		finding := ""
		if d.Finding != "slot" {
			finding = " --finding " + core3Quote(d.Finding)
		}
		recovery += fmt.Sprintf(" or mindspec panel refute %s --spec %s --slot %s --round %d%s --ground disproven --evidence '<fixing commit>' --reason '<why>' --by '<who>'", core3Quote(reg.Slug()), core3Quote(reg.Panel.Spec), core3Quote(d.Slot), d.Round, finding)
	}
	return guard.NewFailure(fmt.Sprintf("panel %s slot %s finding %s follow-up %s: %s", termsafe.Escape(reg.Slug()), termsafe.Escape(d.Slot), termsafe.Escape(d.Finding), termsafe.Escape(d.Bead), termsafe.Escape(err.Error())), recovery)
}

func checkDeferral(reg panel.Registration, d panel.Deferral, texts []string, epic string, epicErr error) error {
	if epicErr != nil {
		return epicErr
	}
	reviewedBead := ""
	if reg.Panel.BeadID != nil {
		reviewedBead = *reg.Panel.BeadID
	}
	return bead.CheckDeferral(d.Bead, epic, reviewedBead, texts, panel.RedactFindingPaths)
}

func checkedPanelDeferrals(reg panel.Registration, entries []panel.Deferral) ([]PanelDeferredEntry, error) {
	if reg.Panel.Abandoned || len(entries) == 0 {
		return nil, nil
	}
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		return nil, err
	}
	epic, epicErr := phase.FindEpicBySpecID(reg.Panel.Spec)
	var valid []PanelDeferredEntry
	type failed struct {
		d   panel.Deferral
		err error
	}
	var failures []failed
	for _, d := range entries {
		texts := res.DeferralTexts(d)
		if err := checkDeferral(reg, d, texts, epic, epicErr); err != nil {
			failures = append(failures, failed{d, err})
			continue
		}
		valid = append(valid, PanelDeferredEntry{Deferral: d, FindingTexts: texts})
	}
	// Voiding a deferral can leave another discharge for the same finding.
	// Recompute resolution with only the deferrals whose live checks passed.
	copyPanel := *res.Panel
	copyPanel.Deferrals = nil
	for _, d := range valid {
		copyPanel.Deferrals = append(copyPanel.Deferrals, d.Deferral)
	}
	res.Panel = &copyPanel
	if len(failures) > 0 && len(res.UnresolvedVerdicts()) > 0 {
		for _, failure := range failures {
			for _, v := range res.UnresolvedVerdicts() {
				if v.Slot == failure.d.Slot {
					return nil, deferralFailure(reg, failure.d, false, failure.err)
				}
			}
		}
	}
	return valid, nil
}

// CheckAuditedPanelDeferrals uses only the completion-time text boundaries.
func CheckAuditedPanelDeferrals(reg panel.Registration, entries []PanelDeferredEntry) error {
	if reg.Panel.Abandoned || len(entries) == 0 {
		return nil
	}
	epic, epicErr := phase.FindEpicBySpecID(reg.Panel.Spec)
	for _, d := range entries {
		texts := d.FindingTexts
		if len(texts) == 0 {
			texts = []string{d.FindingText}
		}
		if err := checkDeferral(reg, d.Deferral, texts, epic, epicErr); err != nil {
			return deferralFailure(reg, d.Deferral, true, err)
		}
	}
	return nil
}

// DecodePanelDeferred reads the durable audit, failing closed on malformed
// entries. It is also used for self-contained pending markers.
func DecodePanelDeferred(meta map[string]interface{}, key string) ([]PanelDeferredEntry, error) {
	raw, present := meta[key]
	if !present {
		return nil, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(string(data), "[") {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	var entries []PanelDeferredEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	for _, d := range entries {
		record, err := json.Marshal(panel.DeferralRecord{Record: panel.RecordDeferral, Spec: "audit", Panel: "audit", Deferral: d.Deferral})
		if err != nil {
			return nil, err
		}
		if err := panel.Validate(record); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func unionDeferrals(existing, add []PanelDeferredEntry) []PanelDeferredEntry {
	out := append([]PanelDeferredEntry{}, existing...)
	seen := map[string]bool{}
	key := func(d PanelDeferredEntry) string { d.CreatedAt = ""; b, _ := json.Marshal(d); return string(b) }
	for _, d := range existing {
		seen[key(d)] = true
	}
	for _, d := range add {
		if !seen[key(d)] {
			out = append(out, d)
			seen[key(d)] = true
		}
	}
	return out
}

func persistDeferralPending(beadID string, entries []PanelDeferredEntry) error {
	if len(entries) == 0 {
		return nil
	}
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return err
	}
	existing, err := DecodePanelDeferred(meta, "deferral_pending_entries")
	if err != nil {
		return err
	}
	return completeMergeMetadataFn(beadID, map[string]interface{}{"deferral_pending_entries": unionDeferrals(existing, entries)})
}

// reconcilePendingDeferrals settles the durable obligation before every close,
// including reruns whose registration disappeared or whose gate was skipped.
func reconcilePendingDeferrals(beadID string) error {
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	pending, err := DecodePanelDeferred(meta, "deferral_pending_entries")
	if err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	existing, err := DecodePanelDeferred(meta, "panel_deferred_entries")
	if err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	if len(pending) == 0 {
		return nil
	}
	if err := completeMergeMetadataFn(beadID, map[string]interface{}{"panel_deferred": true, "panel_deferred_at": time.Now().UTC().Format(time.RFC3339), "panel_deferred_entries": unionDeferrals(existing, pending)}); err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	return nil
}

func checkAndRecordPanelDeferrals(beadID string, results []PanelGateResult, enabled bool) error {
	if !enabled || panelSkipEnvFn() {
		return nil
	}
	var entries []PanelDeferredEntry
	for _, result := range results {
		if result.Decision.Action != panel.Allow {
			continue
		}
		checked, err := checkedPanelDeferrals(result.Registration, result.Decision.AppliedDeferrals)
		if err != nil {
			return err
		}
		entries = append(entries, checked...)
	}
	if err := persistDeferralPending(beadID, entries); err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	return nil
}

// PanelDeferredAuditFailure makes metadata errors fail closed with a retry.
func PanelDeferredAuditFailure(beadID string, err error) error {
	return guard.NewFailure(fmt.Sprintf("cannot read or record deferred findings for %s: %s", idrender.Bead(beadID), termsafe.Escape(err.Error())), fmt.Sprintf("mindspec complete %s", idrender.Bead(beadID)))
}
