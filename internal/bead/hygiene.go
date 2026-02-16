package bead

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const defaultRecommendedMax = 15

// HygieneReport summarizes workset health.
type HygieneReport struct {
	Stale          []BeadInfo
	Orphaned       []BeadInfo
	Oversized      []BeadInfo
	TotalOpen      int
	RecommendedMax int
}

// AuditWorkset analyzes open beads for hygiene issues.
func AuditWorkset(staleDays int) (*HygieneReport, error) {
	out, err := RunBD("list", "--status=open", "--json")
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
		// Stale check: updated_at older than threshold
		if b.UpdatedAt != "" {
			updated, err := time.Parse(time.RFC3339, b.UpdatedAt)
			if err == nil && updated.Before(staleThreshold) {
				report.Stale = append(report.Stale, b)
			}
		}

		// Orphan check: no [SPEC or [IMPL prefix
		if !strings.HasPrefix(b.Title, "[SPEC ") && !strings.HasPrefix(b.Title, "[IMPL ") {
			report.Orphaned = append(report.Orphaned, b)
		}

		// Oversized check
		descLen := len(b.Description)
		if strings.HasPrefix(b.Title, "[SPEC ") && descLen > 400 {
			report.Oversized = append(report.Oversized, b)
		} else if descLen > 800 {
			report.Oversized = append(report.Oversized, b)
		}
	}

	return report, nil
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
		fmt.Fprintf(&sb, "Orphaned beads (%d) — no [SPEC] or [IMPL] prefix:\n", len(r.Orphaned))
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
	out, err := RunBD("list", "--status=open", "--json")
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
