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

const (
	planActionCreate      = "create"
	planActionUpdate      = "update"
	planActionMerge       = "merge"
	planActionSplit       = "split"
	planActionArchiveOnly = "archive-only"
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
	ID             string       `json:"id"`
	Action         string       `json:"action"`
	Target         string       `json:"target,omitempty"`
	Sources        []PlanSource `json:"sources"`
	ArchiveTargets []string     `json:"archive_targets,omitempty"`
	Rationale      string       `json:"rationale"`
	Confidence     float64      `json:"confidence"`
	LLMUsed        bool         `json:"llm_used"`
}

// ExtractionEntry captures candidate canonical targets derived per source document.
type ExtractionEntry struct {
	Path             string   `json:"path"`
	SHA256           string   `json:"sha256"`
	Category         string   `json:"category"`
	Rule             string   `json:"rule"`
	Rationale        string   `json:"rationale,omitempty"`
	Confidence       float64  `json:"confidence"`
	RequiresLLM      bool     `json:"requires_llm"`
	CandidateTargets []string `json:"candidate_targets"`
}

// ValidationCheck captures one machine-checked plan validation result.
type ValidationCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// PlanValidation summarizes checks for plan integrity and traceability.
type PlanValidation struct {
	RunID  string            `json:"run_id"`
	Valid  bool              `json:"valid"`
	Checks []ValidationCheck `json:"checks"`
}

// MigrationPlan is the machine-readable plan artifact for review before apply.
type MigrationPlan struct {
	RunID       string          `json:"run_id"`
	GeneratedAt string          `json:"generated_at"`
	LLM         LLMConfig       `json:"llm"`
	Operations  []PlanOperation `json:"operations"`
}

func buildExtraction(root string, report *Report) ([]ExtractionEntry, error) {
	entries := make([]ExtractionEntry, 0, len(report.Classification)+1)
	for _, c := range report.Classification {
		targets := extractionTargets(c)
		entries = append(entries, ExtractionEntry{
			Path:             c.Path,
			SHA256:           c.SHA256,
			Category:         c.Category,
			Rule:             c.Rule,
			Rationale:        c.Rationale,
			Confidence:       c.Confidence,
			RequiresLLM:      c.RequiresLLM,
			CandidateTargets: targets,
		})
	}

	policyPath := filepath.Join(root, "architecture", "policies.yml")
	if data, err := os.ReadFile(policyPath); err == nil {
		sum := sha256.Sum256(data)
		entries = append(entries, ExtractionEntry{
			Path:             filepath.ToSlash(filepath.Join("architecture", "policies.yml")),
			SHA256:           hex.EncodeToString(sum[:]),
			Category:         "policy",
			Rule:             "policy-migration",
			Rationale:        "Legacy policy file migrates to canonical .mindspec/policies.yml with rewritten references.",
			Confidence:       1.0,
			RequiresLLM:      false,
			CandidateTargets: []string{filepath.ToSlash(filepath.Join(".mindspec", "policies.yml"))},
		})
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read legacy policy: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func extractionTargets(c ClassificationEntry) []string {
	lower := strings.ToLower(c.Path)
	base := filepath.Base(lower)

	// A combined context+glossary source can be split into two canonical targets.
	if strings.Contains(base, "context") && strings.Contains(base, "glossary") {
		targets := []string{
			filepath.ToSlash(filepath.Join(".mindspec", "docs", "context-map.md")),
			filepath.ToSlash(filepath.Join(".mindspec", "docs", "glossary.md")),
		}
		sort.Strings(targets)
		return dedupeStrings(targets)
	}

	target, ok := canonicalTarget(c.Path, c.Category)
	if !ok {
		return nil
	}
	return []string{target}
}

func buildMigrationPlan(root string, report *Report, extraction []ExtractionEntry) (*MigrationPlan, error) {
	targetBuckets := map[string][]ExtractionEntry{}
	unassigned := make([]ExtractionEntry, 0)
	sourceTargetCount := make(map[string]int, len(extraction))

	for _, e := range extraction {
		sourceTargetCount[e.Path] = len(e.CandidateTargets)
		if len(e.CandidateTargets) == 0 {
			unassigned = append(unassigned, e)
			continue
		}
		for _, target := range e.CandidateTargets {
			targetBuckets[target] = append(targetBuckets[target], e)
		}
	}

	targets := make([]string, 0, len(targetBuckets))
	for target := range targetBuckets {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	sort.Slice(unassigned, func(i, j int) bool { return unassigned[i].Path < unassigned[j].Path })

	nextID := 1
	newID := func() string {
		id := fmt.Sprintf("op-%03d", nextID)
		nextID++
		return id
	}

	operations := make([]PlanOperation, 0, len(targets)+len(unassigned))
	for _, target := range targets {
		entries := targetBuckets[target]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

		sources := make([]PlanSource, 0, len(entries))
		archiveTargets := make([]string, 0, len(entries))
		minConfidence := 1.0
		llmUsed := false
		hasSplitSource := false

		for _, e := range entries {
			sources = append(sources, PlanSource{
				Path:        e.Path,
				SHA256:      e.SHA256,
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
			if strings.HasPrefix(e.Rule, "llm:") {
				llmUsed = true
			}
			if sourceTargetCount[e.Path] > 1 {
				hasSplitSource = true
			}
		}

		action := planActionCreate
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err == nil {
			action = planActionUpdate
		}
		if len(entries) > 1 {
			action = planActionMerge
		} else if hasSplitSource {
			action = planActionSplit
		}

		rationale := makeOperationRationale(action, target, entries)
		operations = append(operations, PlanOperation{
			ID:             newID(),
			Action:         action,
			Target:         target,
			Sources:        sources,
			ArchiveTargets: dedupeStrings(archiveTargets),
			Rationale:      rationale,
			Confidence:     minConfidence,
			LLMUsed:        llmUsed,
		})
	}

	for _, e := range unassigned {
		operations = append(operations, PlanOperation{
			ID:     newID(),
			Action: planActionArchiveOnly,
			Sources: []PlanSource{{
				Path:        e.Path,
				SHA256:      e.SHA256,
				Category:    e.Category,
				Rule:        e.Rule,
				Rationale:   e.Rationale,
				Confidence:  e.Confidence,
				RequiresLLM: e.RequiresLLM,
			}},
			ArchiveTargets: []string{filepath.ToSlash(filepath.Join("docs_archive", report.RunID, e.Path))},
			Rationale:      fmt.Sprintf("No canonical target mapping for category %q; preserving in archive only.", e.Category),
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

func makeOperationRationale(action, target string, entries []ExtractionEntry) string {
	if len(entries) == 0 {
		return "No source entries were attached to this operation."
	}
	first := entries[0]
	switch action {
	case planActionMerge:
		return fmt.Sprintf("%d sources map to %s and are merged; representative rationale: %s", len(entries), target, firstExplanation(first))
	case planActionSplit:
		return fmt.Sprintf("Source %s is split into multiple canonical targets including %s; rationale: %s", first.Path, target, firstExplanation(first))
	default:
		return fmt.Sprintf("%s maps to %s via rule %q; rationale: %s", first.Path, target, first.Rule, firstExplanation(first))
	}
}

func firstExplanation(e ExtractionEntry) string {
	if strings.TrimSpace(e.Rationale) != "" {
		return e.Rationale
	}
	return "Deterministic classification rule matched source path/content."
}

func validateMigrationPlan(runID string, extraction []ExtractionEntry, plan *MigrationPlan) PlanValidation {
	checks := make([]ValidationCheck, 0, 8)
	valid := true

	if len(extraction) == 0 {
		valid = false
		checks = append(checks, ValidationCheck{Name: "extraction", Status: "error", Message: "no extraction entries"})
	} else {
		checks = append(checks, ValidationCheck{Name: "extraction", Status: "ok", Message: fmt.Sprintf("entries=%d", len(extraction))})
	}

	if len(plan.Operations) == 0 {
		valid = false
		checks = append(checks, ValidationCheck{Name: "operations", Status: "error", Message: "no plan operations"})
	} else {
		checks = append(checks, ValidationCheck{Name: "operations", Status: "ok", Message: fmt.Sprintf("count=%d", len(plan.Operations))})
	}

	seenIDs := map[string]struct{}{}
	validActions := map[string]struct{}{
		planActionCreate:      {},
		planActionUpdate:      {},
		planActionMerge:       {},
		planActionSplit:       {},
		planActionArchiveOnly: {},
	}

	for _, op := range plan.Operations {
		if _, ok := seenIDs[op.ID]; ok {
			valid = false
			checks = append(checks, ValidationCheck{Name: "operation.id", Status: "error", Message: fmt.Sprintf("duplicate id %s", op.ID)})
		}
		seenIDs[op.ID] = struct{}{}

		if _, ok := validActions[op.Action]; !ok {
			valid = false
			checks = append(checks, ValidationCheck{Name: "operation.action", Status: "error", Message: fmt.Sprintf("%s uses unsupported action %q", op.ID, op.Action)})
		}
		if len(op.Sources) == 0 {
			valid = false
			checks = append(checks, ValidationCheck{Name: "operation.sources", Status: "error", Message: fmt.Sprintf("%s has no sources", op.ID)})
		}
		if op.Action == planActionArchiveOnly {
			if op.Target != "" {
				valid = false
				checks = append(checks, ValidationCheck{Name: "operation.target", Status: "error", Message: fmt.Sprintf("%s archive-only must not set target", op.ID)})
			}
		} else if strings.TrimSpace(op.Target) == "" {
			valid = false
			checks = append(checks, ValidationCheck{Name: "operation.target", Status: "error", Message: fmt.Sprintf("%s requires target", op.ID)})
		}
		for _, src := range op.Sources {
			if strings.TrimSpace(src.Path) == "" || strings.TrimSpace(src.SHA256) == "" {
				valid = false
				checks = append(checks, ValidationCheck{Name: "operation.source", Status: "error", Message: fmt.Sprintf("%s contains invalid source entry", op.ID)})
			}
		}
	}

	if valid {
		checks = append(checks, ValidationCheck{Name: "integrity", Status: "ok", Message: "plan validation passed"})
	}

	return PlanValidation{RunID: runID, Valid: valid, Checks: checks}
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
		fmt.Fprintf(&b, "- Operation ID: `%s`\n", op.ID)
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
	path := runDir(root, report.RunID)

	extraction, err := buildExtraction(root, report)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(path, "extraction.json"), extraction); err != nil {
		return err
	}

	plan, err := buildMigrationPlan(root, report, extraction)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(path, "plan.json"), plan); err != nil {
		return err
	}

	validation := validateMigrationPlan(report.RunID, extraction, plan)
	if err := writeJSON(filepath.Join(path, "validation.json"), validation); err != nil {
		return err
	}
	if !validation.Valid {
		return fmt.Errorf("generated migration plan failed validation checks")
	}

	markdown := renderPlanMarkdown(plan)
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Join(path, "plan.md"), err)
	}
	return nil
}

func loadMigrationPlan(root, runID string) (*MigrationPlan, error) {
	path := filepath.Join(runDir(root, runID), "plan.json")
	var plan MigrationPlan
	if err := readJSON(path, &plan); err != nil {
		return nil, fmt.Errorf("load migration plan: %w", err)
	}
	return &plan, nil
}

func verifyApplyPlanAndSourceDrift(root string, report *Report) (*MigrationPlan, error) {
	planRel := filepath.ToSlash(filepath.Join(".mindspec", "migrations", report.RunID, "plan.json"))
	planPath := filepath.Join(root, filepath.FromSlash(planRel))
	if _, err := os.Stat(planPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("migrate apply blocked: missing %s (run 'mindspec migrate plan --run-id %s' first)", planRel, report.RunID)
		}
		return nil, fmt.Errorf("stat %s: %w", planRel, err)
	}

	plan, err := loadMigrationPlan(root, report.RunID)
	if err != nil {
		return nil, err
	}

	expected := map[string]string{}
	for _, op := range plan.Operations {
		for _, src := range op.Sources {
			if src.Path == "" || src.SHA256 == "" {
				return nil, fmt.Errorf("migrate apply blocked: plan contains invalid source entries")
			}
			if prev, ok := expected[src.Path]; ok && prev != src.SHA256 {
				return nil, fmt.Errorf("migrate apply blocked: inconsistent source hash for %s in plan", src.Path)
			}
			expected[src.Path] = src.SHA256
		}
	}
	if len(expected) == 0 {
		return nil, fmt.Errorf("migrate apply blocked: plan has no source entries")
	}

	for _, inv := range report.Inventory {
		if _, ok := expected[inv.Path]; !ok {
			return nil, fmt.Errorf("migrate apply blocked: plan does not cover source %s", inv.Path)
		}
	}

	var drift []string
	paths := make([]string, 0, len(expected))
	for p := range expected {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		abs := filepath.Join(root, filepath.FromSlash(path))
		data, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				drift = append(drift, fmt.Sprintf("%s (missing)", path))
				continue
			}
			return nil, fmt.Errorf("read source %s: %w", path, err)
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != expected[path] {
			drift = append(drift, path)
		}
	}

	if len(drift) > 0 {
		sort.Strings(drift)
		preview := strings.Join(drift[:min(5, len(drift))], ", ")
		if len(drift) > 5 {
			preview += fmt.Sprintf(", and %d more", len(drift)-5)
		}
		return nil, fmt.Errorf(
			"migrate apply blocked: source drift detected for %d file(s): %s; rerun 'mindspec migrate plan'",
			len(drift),
			preview,
		)
	}
	return plan, nil
}

func dedupeStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
