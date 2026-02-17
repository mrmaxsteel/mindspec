package brownfield

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PlanSource captures one source document entry in the migration plan.
type PlanSource struct {
	Path        string  `json:"path"`
	SHA256      string  `json:"sha256"`
	Category    string  `json:"category"`
	Rule        string  `json:"rule"`
	Rationale   string  `json:"rationale,omitempty"`
	Confidence  float64 `json:"confidence"`
	RequiresLLM bool    `json:"requires_llm"`
}

// PlanOperation captures one proposed canonical migration action.
type PlanOperation struct {
	Action         string       `json:"action"`
	Target         string       `json:"target,omitempty"`
	Sources        []PlanSource `json:"sources"`
	ArchiveTargets []string     `json:"archive_targets,omitempty"`
	Rationale      string       `json:"rationale"`
	Confidence     float64      `json:"confidence"`
	LLMUsed        bool         `json:"llm_used"`
}

// MigrationPlan is the machine-readable plan artifact for review before apply.
type MigrationPlan struct {
	RunID       string          `json:"run_id"`
	GeneratedAt string          `json:"generated_at"`
	LLM         LLMConfig       `json:"llm"`
	Operations  []PlanOperation `json:"operations"`
}

func buildMigrationPlan(root string, report *Report) (*MigrationPlan, error) {
	shaByPath := make(map[string]string, len(report.Inventory))
	for _, inv := range report.Inventory {
		shaByPath[inv.Path] = inv.SHA256
	}

	grouped := map[string][]ClassificationEntry{}
	var unknown []ClassificationEntry
	for _, c := range report.Classification {
		target, ok := canonicalTarget(c.Path, c.Category)
		if !ok {
			unknown = append(unknown, c)
			continue
		}
		grouped[target] = append(grouped[target], c)
	}

	targets := make([]string, 0, len(grouped))
	for target := range grouped {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	operations := make([]PlanOperation, 0, len(targets)+len(unknown))
	for _, target := range targets {
		entries := grouped[target]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

		sources := make([]PlanSource, 0, len(entries))
		archiveTargets := make([]string, 0, len(entries))
		minConfidence := 1.0
		llmUsed := false
		for _, e := range entries {
			sources = append(sources, PlanSource{
				Path:        e.Path,
				SHA256:      shaByPath[e.Path],
				Category:    e.Category,
				Rule:        e.Rule,
				Rationale:   e.Rationale,
				Confidence:  e.Confidence,
				RequiresLLM: e.RequiresLLM,
			})
			archiveTargets = append(archiveTargets, filepath.ToSlash(filepath.Join("docs_archive", report.RunID, e.Path)))
			if e.Confidence < minConfidence {
				minConfidence = e.Confidence
			}
			llmUsed = llmUsed || strings.HasPrefix(e.Rule, "llm:")
		}

		action := "create"
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err == nil {
			action = "update"
		}
		rationale := fmt.Sprintf("%s maps to %s via rule %q.", entries[0].Path, target, entries[0].Rule)
		if entries[0].Rationale != "" {
			rationale = fmt.Sprintf("%s LLM rationale: %s", rationale, entries[0].Rationale)
		}
		if len(entries) > 1 {
			action = "merge"
			rationale = fmt.Sprintf(
				"%d source documents are merged into %s because they classify to the same canonical target.",
				len(entries),
				target,
			)
			if entries[0].Rationale != "" {
				rationale = fmt.Sprintf("%s Example LLM rationale: %s", rationale, entries[0].Rationale)
			}
		}

		operations = append(operations, PlanOperation{
			Action:         action,
			Target:         target,
			Sources:        sources,
			ArchiveTargets: archiveTargets,
			Rationale:      rationale,
			Confidence:     minConfidence,
			LLMUsed:        llmUsed,
		})
	}

	sort.Slice(unknown, func(i, j int) bool { return unknown[i].Path < unknown[j].Path })
	for _, e := range unknown {
		operations = append(operations, PlanOperation{
			Action: "drop",
			Sources: []PlanSource{{
				Path:        e.Path,
				SHA256:      shaByPath[e.Path],
				Category:    e.Category,
				Rule:        e.Rule,
				Rationale:   e.Rationale,
				Confidence:  e.Confidence,
				RequiresLLM: e.RequiresLLM,
			}},
			ArchiveTargets: []string{filepath.ToSlash(filepath.Join("docs_archive", report.RunID, e.Path))},
			Rationale:      fmt.Sprintf("No canonical target mapping exists for category %q.", e.Category),
			Confidence:     e.Confidence,
			LLMUsed:        strings.HasPrefix(e.Rule, "llm:"),
		})
	}

	generatedAt := ""
	if ts, err := time.Parse("20060102T150405Z", report.RunID); err == nil {
		generatedAt = ts.UTC().Format(time.RFC3339)
	}

	return &MigrationPlan{
		RunID:       report.RunID,
		GeneratedAt: generatedAt,
		LLM:         report.LLM,
		Operations:  operations,
	}, nil
}

func renderPlanMarkdown(plan *MigrationPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Migration Plan\n\n")
	fmt.Fprintf(&b, "- Run ID: `%s`\n", plan.RunID)
	fmt.Fprintf(&b, "- Generated At: `%s`\n", plan.GeneratedAt)
	fmt.Fprintf(&b, "- LLM Provider: `%s` (available=%t)\n", plan.LLM.Provider, plan.LLM.Available)
	fmt.Fprintf(&b, "- Operations: `%d`\n\n", len(plan.Operations))

	for i, op := range plan.Operations {
		fmt.Fprintf(&b, "## %d. `%s`", i+1, op.Action)
		if op.Target != "" {
			fmt.Fprintf(&b, " -> `%s`", op.Target)
		}
		fmt.Fprintf(&b, "\n\n")
		fmt.Fprintf(&b, "- Confidence: `%.2f`\n", op.Confidence)
		fmt.Fprintf(&b, "- LLM Used: `%t`\n", op.LLMUsed)
		fmt.Fprintf(&b, "- Rationale: %s\n", op.Rationale)
		fmt.Fprintf(&b, "- Sources:\n")
		for _, src := range op.Sources {
			fmt.Fprintf(&b, "  - `%s` (sha256=%s, category=%s, rule=%s)\n", src.Path, src.SHA256, src.Category, src.Rule)
			if src.Rationale != "" {
				fmt.Fprintf(&b, "    - rationale: %s\n", src.Rationale)
			}
		}
		if len(op.ArchiveTargets) > 0 {
			fmt.Fprintf(&b, "- Archive Targets:\n")
			for _, archive := range op.ArchiveTargets {
				fmt.Fprintf(&b, "  - `%s`\n", archive)
			}
		}
		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}

func writePlanArtifacts(root string, report *Report) error {
	plan, err := buildMigrationPlan(root, report)
	if err != nil {
		return err
	}
	path := runDir(root, report.RunID)
	if err := writeJSON(filepath.Join(path, "plan.json"), plan); err != nil {
		return err
	}
	markdown := renderPlanMarkdown(plan)
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Join(path, "plan.md"), err)
	}
	return nil
}

func verifyApplyPlanAndSourceDrift(root string, report *Report) error {
	planRel := filepath.ToSlash(filepath.Join(".mindspec", "migrations", report.RunID, "plan.json"))
	planPath := filepath.Join(root, filepath.FromSlash(planRel))
	if _, err := os.Stat(planPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("migrate apply blocked: missing %s (run 'mindspec migrate plan --run-id %s' first)", planRel, report.RunID)
		}
		return fmt.Errorf("stat %s: %w", planRel, err)
	}

	var drift []string
	for _, inv := range report.Inventory {
		abs := filepath.Join(root, filepath.FromSlash(inv.Path))
		data, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				drift = append(drift, fmt.Sprintf("%s (missing)", inv.Path))
				continue
			}
			return fmt.Errorf("read source %s: %w", inv.Path, err)
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != inv.SHA256 {
			drift = append(drift, inv.Path)
		}
	}

	if len(drift) > 0 {
		sort.Strings(drift)
		preview := strings.Join(drift[:min(5, len(drift))], ", ")
		if len(drift) > 5 {
			preview += fmt.Sprintf(", and %d more", len(drift)-5)
		}
		return fmt.Errorf(
			"migrate apply blocked: source drift detected for %d file(s): %s; rerun 'mindspec migrate plan'",
			len(drift),
			preview,
		)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
