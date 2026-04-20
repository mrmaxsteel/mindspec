package bead

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	defaultRecommendedMax  = 15
	oversizedDescThreshold = 800
)

// HygieneReport summarizes workset health.
type HygieneReport struct {
	Stale          []BeadInfo
	Orphaned       []BeadInfo
	Oversized      []BeadInfo
	TotalOpen      int
	RecommendedMax int
}

// AuditWorkset analyzes open beads for hygiene issues. A bead is classified
// as mindspec-owned when it carries explicit metadata (`mindspec_phase` or
// `spec_num`/`spec_id`) written by spec-create or plan-approve — we no longer
// infer ownership from title-prefix substrings like `[SPEC ` / `[IMPL `,
// which would misclassify any bead whose title drifted from the convention
// or whose tooling wrote a different prefix.
func AuditWorkset(staleDays int) (*HygieneReport, error) {
	out, err := ListJSON("--status=open")
	if err != nil {
		return nil, fmt.Errorf("listing open beads: %w", err)
	}

	var beads []BeadInfo
	if err := json.Unmarshal(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing beads list: %w", err)
	}

	report := &HygieneReport{
		TotalOpen:      len(beads),
		RecommendedMax: defaultRecommendedMax,
	}

	now := time.Now()
	staleThreshold := now.AddDate(0, 0, -staleDays)

	for _, b := range beads {
		if b.UpdatedAt != "" {
			updated, err := time.Parse(time.RFC3339, b.UpdatedAt)
			if err == nil && updated.Before(staleThreshold) {
				report.Stale = append(report.Stale, b)
			}
		}

		if !hasMindspecMetadata(b) {
			report.Orphaned = append(report.Orphaned, b)
		}

		if len(b.Description) > oversizedDescThreshold {
			report.Oversized = append(report.Oversized, b)
		}
	}

	return report, nil
}

// hasMindspecMetadata reports whether a bead was created by mindspec and
// therefore belongs to a spec lifecycle. Presence of any mindspec-written
// metadata key with a non-nil, non-empty-string value is sufficient.
//
// The `v != ""` check looks odd against an `interface{}`: it's comparing the
// *string value* "" to whatever type the JSON decoder produced. Go's interface
// equality treats values of different types as unequal, so `spec_num: 7`
// (float64) passes the check, as does `mindspec_done: false` (bool) — both
// are legitimate presence signals. Only the literal empty string would be
// treated as "absent", which is the intent: a metadata key explicitly set to
// an empty string should not count as ownership.
//
// `mindspec_done` is included so epics predating Spec 080 are not
// misreported as orphans.
func hasMindspecMetadata(b BeadInfo) bool {
	if b.Metadata == nil {
		return false
	}
	for _, key := range []string{"mindspec_phase", "spec_id", "spec_num", "mindspec_done"} {
		if v, ok := b.Metadata[key]; ok && v != nil && v != "" {
			return true
		}
	}
	return false
}

// FormatReport produces a human-readable hygiene report.
func FormatReport(r *HygieneReport) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Workset Hygiene Report\n")
	fmt.Fprintf(&sb, "=====================\n\n")
	fmt.Fprintf(&sb, "Open beads: %d / %d recommended max\n\n", r.TotalOpen, r.RecommendedMax)

	if len(r.Stale) > 0 {
		fmt.Fprintf(&sb, "Stale beads (%d):\n", len(r.Stale))
		for _, b := range r.Stale {
			fmt.Fprintf(&sb, "  - %s: %s (updated: %s)\n", b.ID, b.Title, b.UpdatedAt)
			fmt.Fprintf(&sb, "    Fix: bd update %s --notes=\"still active\" OR bd close %s\n", b.ID, b.ID)
		}
		sb.WriteString("\n")
	}

	if len(r.Orphaned) > 0 {
		fmt.Fprintf(&sb, "Orphaned beads (%d) — no mindspec metadata (spec_id / mindspec_phase):\n", len(r.Orphaned))
		for _, b := range r.Orphaned {
			fmt.Fprintf(&sb, "  - %s: %s\n", b.ID, b.Title)
		}
		sb.WriteString("\n")
	}

	if len(r.Oversized) > 0 {
		fmt.Fprintf(&sb, "Oversized descriptions (%d):\n", len(r.Oversized))
		for _, b := range r.Oversized {
			fmt.Fprintf(&sb, "  - %s: %s (%d chars)\n", b.ID, b.Title, len(b.Description))
		}
		sb.WriteString("\n")
	}

	if len(r.Stale) == 0 && len(r.Orphaned) == 0 && len(r.Oversized) == 0 {
		sb.WriteString("No issues found.\n")
	}

	return sb.String()
}

// FixHygiene closes beads that have status "done".
// If dryRun is true, returns what would be done without executing.
func FixHygiene(dryRun bool) ([]string, error) {
	out, err := ListJSON("--status=open")
	if err != nil {
		return nil, fmt.Errorf("listing open beads: %w", err)
	}

	var beads []BeadInfo
	if err := json.Unmarshal(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing beads list: %w", err)
	}

	var actions []string
	for _, b := range beads {
		if strings.EqualFold(b.Status, "done") {
			action := fmt.Sprintf("close %s (%s)", b.ID, b.Title)
			if dryRun {
				actions = append(actions, "[dry-run] would "+action)
			} else {
				if _, err := RunBDCombined("update", b.ID, "--status=closed"); err != nil {
					return actions, fmt.Errorf("closing %s: %w", b.ID, err)
				}
				actions = append(actions, action)
			}
		}
	}

	if len(actions) == 0 {
		actions = append(actions, "no beads with 'done' status found")
	}

	return actions, nil
}
