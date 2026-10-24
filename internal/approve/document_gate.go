package approve

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func appendDocumentGateWarnings(warnings *[]string, text string) {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if line != "" {
			*warnings = append(*warnings, strings.TrimPrefix(line, "warning: "))
		}
	}
}

func documentGateAuditFailure(err error) error {
	return fmt.Errorf("warning: panel gate audit is pending: %s\nrecovery: run the same command again in the same environment", termsafe.Escape(err.Error()))
}

// specPlanDeferralIDs keeps all accepted ledger obligations, including older
// rounds and abandoned panels. A nil list means that no list is due.
func specPlanDeferralIDs(root, worktree, specID string, exec executor.Executor) ([]string, error) {
	ids := map[string]bool{}
	for _, reg := range lifecycleRegistrations(panel.Scan(complete.PanelGateRoots(root, worktree, specID)...), specID, "spec_approve") {
		lines, err := panel.ReadLedger(reg.Dir)
		if err != nil {
			return nil, guard.NewFailure("cannot read plan deferrals: "+termsafe.Escape(err.Error()), "repair the panel ledger and run the same command again")
		}
		if err := collectPlanDeferralIDs(lines, specID, ids); err != nil {
			return nil, err
		}
	}
	if err := trackedPlanDeferralIDs(worktree, specID, exec, ids); err != nil {
		return nil, guard.NewFailure("cannot read plan deferrals: "+termsafe.Escape(err.Error()), "repair the panel ledger and run the same command again")
	}
	return sortedDeferralIDs(ids), nil
}

func collectPlanDeferralIDs(lines [][]byte, specID string, ids map[string]bool) error {
	// The round record supplies the gate. Do not trust a mutable snapshot to
	// decide whether an older round still imposes a plan obligation.
	specRound := false
	for _, line := range lines {
		if err := panel.Validate(line); err != nil {
			return fmt.Errorf("invalid plan deferral ledger: %w", err)
		}
		var r panel.RoundRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		if r.Record == panel.RecordRound {
			specRound = r.Spec == specID && r.Gate == "spec_approve"
		}
		if r.Record == panel.RecordDeferredToPlan && specRound && r.Spec == specID {
			var d panel.PlanDeferralRecord
			if err := json.Unmarshal(line, &d); err != nil {
				return err
			}
			ids[d.ID] = true
		}
	}
	return nil
}

func sortedDeferralIDs(ids map[string]bool) []string {
	var result []string
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func recordedPlanDeferralIDs(metadata map[string]interface{}) ([]string, error) {
	raw, ok := metadata["plan_deferral_ids"]
	if !ok {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("cannot read epic plan_deferral_ids: %w", err)
	}
	var ids []string
	if string(data) == "null" || json.Unmarshal(data, &ids) != nil {
		return nil, fmt.Errorf("invalid epic plan_deferral_ids")
	}
	for _, id := range ids {
		if id == "" {
			return nil, fmt.Errorf("empty epic plan_deferral_ids entry")
		}
	}
	return ids, nil
}

// checkPlanResolutions unions the durable epic list with all tracked spec
// approval ledgers. Snapshot deletion, abandonment and replacement cannot
// remove an obligation that an earlier approval accepted.
func checkPlanResolutions(worktree, specID, content string, metadata map[string]interface{}, exec executor.Executor) error {
	refuse := func(message string) error {
		return guard.NewFailure("plan resolutions: "+termsafe.Escape(message), "name each deferred finding id in the plan's ## Resolutions section and run the same command again")
	}
	ids := map[string]bool{}
	recorded, err := recordedPlanDeferralIDs(metadata)
	if err != nil {
		return refuse(err.Error())
	}
	for _, id := range recorded {
		ids[id] = true
	}
	if err := trackedPlanDeferralIDs(worktree, specID, exec, ids); err != nil {
		return refuse(err.Error())
	}
	section := planResolutionsSection(content)
	var missing []string
	for _, id := range sortedDeferralIDs(ids) {
		if !resolutionNamesID(section, id) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return refuse("missing deferred finding ids: " + strings.Join(missing, ", ") + "; ids must appear in visible text; hidden text, including link destinations and titles, does not count")
	}
	return nil
}

// trackedPlanDeferralIDs reads from one commit of the spec checkout. A
// primary-checkout executor shares its object database, but not its HEAD.
func trackedPlanDeferralIDs(worktree, specID string, exec executor.Executor, ids map[string]bool) error {
	head, err := exec.RevParseRef(worktree, "HEAD")
	if err != nil {
		return fmt.Errorf("cannot resolve tracked panel ledger commit: %w", err)
	}
	listing, err := exec.LsTree(worktree, head)
	if err != nil {
		return fmt.Errorf("%s", "cannot read tracked panel ledgers: "+err.Error())
	}
	for _, line := range strings.Split(listing, "\n") {
		_, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if strings.HasPrefix(name, "\"") {
			name, err = strconv.Unquote(name)
			if err != nil {
				return fmt.Errorf("%s", err.Error())
			}
		}
		matched := false
		for _, base := range []string{".mindspec/specs/", ".mindspec/docs/specs/", "docs/specs/"} {
			prefix := filepath.ToSlash(filepath.Join(base, specID, "reviews")) + "/"
			if strings.HasPrefix(name, prefix) {
				rest := strings.TrimPrefix(name, prefix)
				matched = filepath.Base(rest) == "dispositions.jsonl" && strings.Count(rest, "/") == 1
			}
		}
		if !matched {
			continue
		}
		data, err := exec.FileAtRef(head, name)
		if err != nil {
			return fmt.Errorf("%s", "cannot read "+name+": "+err.Error())
		}
		var lines [][]byte
		for _, row := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(row) != "" {
				lines = append(lines, []byte(row))
			}
		}
		if err := collectPlanDeferralIDs(lines, specID, ids); err != nil {
			return fmt.Errorf("%s", name+": "+err.Error())
		}
	}
	return nil
}

func planResolutionsSection(content string) string {
	return resolutionDocumentText(content)
}

func resolutionNamesID(section, id string) bool {
	// An id embedded in a longer id does not name the original obligation.
	for _, word := range strings.FieldsFunc(section, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	}) {
		if word == id {
			return true
		}
	}
	return false
}
