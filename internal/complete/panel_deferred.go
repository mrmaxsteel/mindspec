package complete

import (
	"encoding/json"
	"fmt"
	"path/filepath"
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
// Findings stores each member's id, raw-text hash and redacted text beside
// FindingTexts. Old slot entries without Findings retain text-only coverage.
// The audit must not read later verdicts or refutations to reconstruct them.
type PanelDeferredEntry struct {
	panel.Deferral
	Panel        string          `json:"panel,omitempty"`
	Spec         string          `json:"spec,omitempty"`
	FindingTexts []string        `json:"finding_texts,omitempty"`
	Findings     []panel.Finding `json:"findings,omitempty"`
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
	scope := "panel " + termsafe.Escape(reg.Slug())
	if reg.Dir == "" && reg.Panel.BeadID != nil {
		scope = "bead " + idrender.Bead(*reg.Panel.BeadID)
	}
	return guard.NewFailure(fmt.Sprintf("%s slot %s finding %s follow-up %s: %s", scope, termsafe.Escape(d.Slot), termsafe.Escape(d.Finding), termsafe.Escape(d.Bead), termsafe.Escape(err.Error())), recovery)
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
		var findings []panel.Finding
		for _, v := range res.Verdicts {
			if v.Slot != d.Slot {
				continue
			}
			for _, f := range v.Findings {
				if d.Finding != "slot" && d.Finding != f.ID {
					continue
				}
				item := d
				item.Finding = f.ID
				if len(res.DeferralTexts(item)) > 0 {
					findings = append(findings, f)
				}
			}
		}
		valid = append(valid, PanelDeferredEntry{Deferral: d, FindingTexts: texts, Findings: findings})
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
	return checkAuditedPanelDeferrals(reg, entries, true)
}

func checkAuditedPanelDeferrals(reg panel.Registration, entries []PanelDeferredEntry, audited bool) error {
	if reg.Panel.Abandoned || len(entries) == 0 {
		return nil
	}
	epic, epicErr := phase.FindEpicBySpecID(reg.Panel.Spec)
	type checked struct {
		entry    PanelDeferredEntry
		findings []panel.Finding
		err      error
	}
	var checks []checked
	for _, d := range entries {
		texts := d.FindingTexts
		if len(texts) == 0 {
			texts = []string{d.FindingText}
		}
		findings := d.Findings
		if d.Finding != "slot" || len(findings) == 0 {
			findings = nil
			for _, text := range texts {
				f := panel.Finding{Text: text}
				if d.Finding != "slot" {
					f.ID, f.SHA256 = d.Finding, d.FindingSHA256
				}
				findings = append(findings, f)
			}
		}
		checks = append(checks, checked{d, findings, checkDeferral(reg, d.Deferral, texts, epic, epicErr)})
	}
	for _, failed := range checks {
		if failed.err == nil {
			continue
		}
		needed := map[panel.Finding]int{}
		for _, finding := range failed.findings {
			needed[finding]++
			// Legacy slot alternatives must also retain text multiplicity.
			if finding.ID != "" {
				needed[panel.Finding{Text: finding.Text}]++
			}
		}
		for finding, count := range needed {
			items := map[string]bool{}
			slotCount := 0
			for _, valid := range checks {
				if valid.err != nil || valid.entry.Panel != failed.entry.Panel || valid.entry.Spec != failed.entry.Spec || valid.entry.Slot != failed.entry.Slot || valid.entry.Round != failed.entry.Round {
					continue
				}
				// Old slot records have no member identities. Preserve their text
				// coverage; new records require both the id and raw-text hash.
				if valid.entry.Finding == "slot" && failed.entry.Finding == "slot" && valid.entry.FindingSHA256 != failed.entry.FindingSHA256 {
					continue
				}
				matches := 0
				for _, f := range valid.findings {
					if f.Text == finding.Text && (f.ID == "" || finding.ID == "" || (f.ID == finding.ID && f.SHA256 == finding.SHA256)) {
						matches++
					}
				}
				if valid.entry.Finding == "slot" && matches > slotCount {
					slotCount = matches
				} else if valid.entry.Finding != "slot" && matches > 0 {
					items[valid.entry.Finding] = true
				}
			}
			// One item record cannot cover two findings with identical text.
			if slotCount < count && len(items) < count {
				return deferralFailure(reg, failed.entry.Deferral, audited, failed.err)
			}
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

// UnionDeferrals combines settled and pending records with the resume union rule.
func UnionDeferrals(existing, add []PanelDeferredEntry) []PanelDeferredEntry {
	return unionDeferrals(existing, add)
}

// PanelDeferredRegistrationMatches binds an audit identity to its ledger round.
// A renamed registration cannot supersede another registration's audit.
func PanelDeferredRegistrationMatches(reg panel.Registration, specID, slug string) bool {
	if reg.Err != nil || reg.Panel.Spec != specID || reg.Slug() != slug {
		return false
	}
	lines, err := panel.ReadLedger(reg.Dir)
	if err != nil {
		return false
	}
	var latest panel.RoundRecord
	for _, line := range lines {
		var round panel.RoundRecord
		if err := json.Unmarshal(line, &round); err != nil {
			return false
		}
		if round.Record == panel.RecordRound {
			latest = round
		}
	}
	return latest.Record == panel.RecordRound && latest.Spec == specID && latest.Panel == slug
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
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return err
	}
	existing, err := DecodePanelDeferred(meta, "deferral_pending_entries")
	if err != nil {
		return err
	}
	if len(existing) == 0 && len(entries) == 0 {
		return nil
	}
	// A retry can supersede a deferral with a refutation. Keep only this run's checked entries.
	return completeMergeMetadataFn(beadID, map[string]interface{}{"deferral_pending_entries": unionDeferrals(nil, entries)})
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
		for i := range checked {
			checked[i].Panel = result.Registration.Slug()
			checked[i].Spec = result.Registration.Panel.Spec
		}
		entries = append(entries, checked...)
	}
	if err := replaceGatedDeferrals(beadID, results, entries, false); err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	return nil
}

// replaceGatedDeferrals replaces only the registrations checked by this attempt.
// The gate records pending entries. Settlement replaces their settled audit
// before close. Other panels and unrelated metadata remain durable on retries.
func replaceGatedDeferrals(beadID string, results []PanelGateResult, entries []PanelDeferredEntry, settle bool) error {
	var gated []panel.Registration
	for _, result := range results {
		reg := result.Registration
		if PanelDeferredRegistrationMatches(reg, reg.Panel.Spec, reg.Slug()) {
			field, err := panel.CheckHandEdits(reg.Dir, reg.Panel)
			if err == nil && field == "" && (result.Decision.Action == panel.Allow || reg.Panel.Abandoned) {
				gated = append(gated, reg)
			}
		}
	}
	if len(gated) == 0 && len(entries) == 0 {
		return nil
	}
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return err
	}
	pending, err := DecodePanelDeferred(meta, "deferral_pending_entries")
	if err != nil {
		return err
	}
	keepOtherPanels := func(prior []PanelDeferredEntry) []PanelDeferredEntry {
		var kept []PanelDeferredEntry
		for _, entry := range prior {
			replaced := false
			for _, reg := range gated {
				if entry.Panel == reg.Slug() && (entry.Spec == "" || entry.Spec == reg.Panel.Spec) {
					replaced = true
					break
				}
			}
			if !replaced {
				kept = append(kept, entry)
			}
		}
		return kept
	}
	if !settle {
		return completeMergeMetadataFn(beadID, map[string]interface{}{"deferral_pending_entries": unionDeferrals(keepOtherPanels(pending), entries)})
	}
	audit, err := DecodePanelDeferred(meta, "panel_deferred_entries")
	if err != nil {
		return err
	}
	return completeMergeMetadataFn(beadID, map[string]interface{}{"panel_deferred_entries": unionDeferrals(keepOtherPanels(audit), pending)})
}

// PanelDeferredAuditFailure makes metadata errors fail closed. Callers can
// mark the bead closed to recover by reopening it; completion retries by default.
func PanelDeferredAuditFailure(beadID string, err error, closed ...bool) error {
	recovery := fmt.Sprintf("mindspec complete %s", idrender.Bead(beadID))
	if len(closed) > 0 && closed[0] {
		recovery = fmt.Sprintf("bd reopen %s", idrender.Bead(beadID))
	}
	return guard.NewFailure(fmt.Sprintf("cannot read or record deferred findings for %s: %s", idrender.Bead(beadID), termsafe.Escape(err.Error())), recovery)
}

// recheckResumedPanelDeferrals checks live tracker facts from the durable audit.
// A materialization marker attests content, not the follow-up's current status.
func recheckResumedPanelDeferrals(beadID, specID string, roots []string, enabled bool) error {
	if !enabled || panelSkipEnvFn() {
		return nil
	}
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	pending, err := DecodePanelDeferred(meta, "deferral_pending_entries")
	if err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	audit, err := DecodePanelDeferred(meta, "panel_deferred_entries")
	if err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	entries := unionDeferrals(audit, pending)
	if len(entries) == 0 {
		return nil
	}
	regs := panel.ForBead(panelScanFn(roots...), beadID)
	groups := map[string][]PanelDeferredEntry{}
	for _, entry := range entries {
		groupSpec := entry.Spec
		if groupSpec == "" {
			groupSpec = specID
		}
		key, _ := json.Marshal([]string{groupSpec, entry.Panel})
		groups[string(key)] = append(groups[string(key)], entry)
	}
	var effective []PanelDeferredEntry
	for _, group := range groups {
		first := group[0]
		groupSpec := first.Spec
		if groupSpec == "" {
			groupSpec = specID
		}
		reg := panel.Registration{Dir: filepath.Join("review", first.Panel), Panel: panel.Panel{Spec: groupSpec, BeadID: &beadID}}
		for _, current := range regs {
			if !PanelDeferredRegistrationMatches(current, groupSpec, first.Panel) {
				continue
			}
			// Only verb-backed abandonment or refutation can supersede the audit.
			field, err := panel.CheckHandEdits(current.Dir, current.Panel)
			if current.Err != nil || err != nil || field != "" {
				continue
			}
			reg = current
			if reg.Panel.Abandoned {
				group = nil
				break
			}
			res, err := panel.Tally(current.Dir)
			if err != nil {
				return err
			}
			refs := res.AppliedRefutations()
			var needed []PanelDeferredEntry
			changed := false
			for _, entry := range group {
				remaining, superseded := trimRefutedDeferral(entry, refs, res)
				changed = changed || superseded
				if remaining != nil {
					needed = append(needed, *remaining)
				}
			}
			if changed {
				var applied []refutationPendingEntry
				for _, ref := range refs {
					entry := pendingRefutation(ref, current.Slug())
					entry.Spec = current.Panel.Spec
					applied = append(applied, entry)
				}
				if err := persistPendingEntries(beadID, applied); err != nil {
					return err
				}
			}
			group = needed
			break
		}
		if err := checkAuditedPanelDeferrals(reg, group, false); err != nil {
			return err
		}
		effective = append(effective, group...)
	}
	if err := persistDeferralPending(beadID, effective); err != nil {
		return PanelDeferredAuditFailure(beadID, err)
	}
	// The bead is still open. A prior attempt may have settled an audit
	// that a later refutation or abandonment has now superseded.
	if len(audit) > 0 {
		if err := completeMergeMetadataFn(beadID, map[string]interface{}{"panel_deferred_entries": unionDeferrals(nil, effective)}); err != nil {
			return PanelDeferredAuditFailure(beadID, err)
		}
	}
	return nil
}

// trimRefutedDeferral preserves the discharge identity and removes only covered
// members. Historical slots without member identities require a whole-slot match.
func trimRefutedDeferral(entry PanelDeferredEntry, refs []panel.Refutation, res *panel.Result) (*PanelDeferredEntry, bool) {
	for _, ref := range refs {
		if ref.Slot == entry.Slot && ref.Round == entry.Round && ref.Finding == entry.Finding && ref.FindingSHA256 == entry.FindingSHA256 {
			return nil, true
		}
	}
	findings := entry.Findings
	if entry.Finding != "slot" {
		findings = []panel.Finding{{ID: entry.Finding, SHA256: entry.FindingSHA256}}
	}
	var remaining []panel.Finding
	for _, finding := range findings {
		covered := false
		for _, ref := range refs {
			if res.RefutationCoversFinding(ref, entry.Slot, entry.Round, finding) {
				covered = true
				break
			}
		}
		if !covered {
			remaining = append(remaining, finding)
		}
	}
	if len(remaining) == len(findings) {
		return &entry, false
	}
	if len(remaining) == 0 {
		return nil, true
	}
	entry.Findings = remaining
	entry.FindingTexts = nil
	for _, finding := range remaining {
		entry.FindingTexts = append(entry.FindingTexts, finding.Text)
	}
	return &entry, true
}
