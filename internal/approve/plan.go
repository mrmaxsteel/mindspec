package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

// planRunBDCombinedFn is a package-level variable for testability.
var planRunBDCombinedFn = bead.RunBDCombined

// planRunBDFn is for JSON-returning bd commands (stdout only, no stderr mixing).
var planRunBDFn = bead.RunBD

// planListJSONFn wraps bead.ListJSON for testability.
var planListJSONFn = bead.ListJSON

// SetPlanRunBDForTest swaps planRunBDFn for testing and returns a restore function.
func SetPlanRunBDForTest(fn func(args ...string) ([]byte, error)) func() {
	orig := planRunBDFn
	planRunBDFn = fn
	return func() { planRunBDFn = orig }
}

// SetPlanRunBDCombinedForTest swaps planRunBDCombinedFn for testing and returns a restore function.
func SetPlanRunBDCombinedForTest(fn func(args ...string) ([]byte, error)) func() {
	orig := planRunBDCombinedFn
	planRunBDCombinedFn = fn
	return func() { planRunBDCombinedFn = orig }
}

// PlanResult holds the result of plan approval.
type PlanResult struct {
	SpecID   string
	GateID   string // empty if no gate found
	BeadIDs  []string
	Warnings []string
}

// ApprovePlan validates and approves a plan, creating beads and setting state.
func ApprovePlan(root, specID, approvedBy string, exec executor.Executor) (*PlanResult, error) {
	result := &PlanResult{SpecID: specID}

	// Step 1: Validate (SpecDir is worktree-aware per ADR-0022)
	vr := validate.ValidatePlan(root, specID)
	if vr.HasFailures() {
		// If plan.md doesn't exist, check if the spec needs approval first.
		// This guides agents that pick the wrong approve subcommand.
		specDir := workspace.SpecDir(root, specID)
		planPath := filepath.Join(specDir, "plan.md")
		if _, statErr := os.Stat(planPath); os.IsNotExist(statErr) {
			specPath := filepath.Join(specDir, "spec.md")
			if specData, readErr := os.ReadFile(specPath); readErr == nil {
				if !strings.Contains(string(specData), "status: Approved") {
					return nil, fmt.Errorf("spec %s has not been approved yet — no plan.md exists.\nRun: mindspec approve spec %s", specID, specID)
				}
			}
		}
		return nil, fmt.Errorf("plan validation failed:\n%s", vr.FormatText())
	}

	// Step 2: Find epic ID via beads metadata query (ADR-0023).
	specDir := workspace.SpecDir(root, specID)
	var parentID string
	epicID, epicErr := phase.FindEpicBySpecID(specID)
	if epicErr == nil {
		parentID = epicID
	}

	// Step 3: Update plan frontmatter
	planPath := filepath.Join(specDir, "plan.md")
	if err := updatePlanApproval(planPath, approvedBy); err != nil {
		return nil, fmt.Errorf("updating plan approval: %w", err)
	}

	// Step 3b: Auto-create implementation beads from plan sections
	if parentID != "" {
		beadIDs, err := createImplementationBeads(planPath, specID, parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to create implementation beads: %w\n\nThe plan has been approved but beads were NOT created.\nFix the issue and re-run: mindspec bead create-from-plan %s", err, specID)
		} else if len(beadIDs) > 0 {
			result.BeadIDs = beadIDs
			if err := writeBeadIDsToFrontmatter(planPath, beadIDs); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not write bead IDs to plan frontmatter: %v", err))
			}
		} else {
			result.Warnings = append(result.Warnings, "plan has no '## Bead N:' sections; no implementation beads were created. Add bead sections to the plan or create beads manually.")
		}
	} else {
		result.Warnings = append(result.Warnings, "no epic found for spec via beads metadata; skipping bead auto-creation")
	}

	// Step 4: Auto-commit plan approval + bead_ids so implementation
	// worktrees that branch from spec/<id> contain the approved artifacts.
	specWtPath := state.SpecWorktreePath(root, specID)
	commitMsg := fmt.Sprintf("chore: approve plan for %s", specID)
	if err := exec.CommitAll(specWtPath, commitMsg); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not auto-commit plan approval: %v", err))
	}

	// Step 5: HandoffEpic — notify executor that beads are ready for dispatch.
	// For GitExecutor this is a no-op. Other executors may use this to schedule work.
	if parentID != "" && len(result.BeadIDs) > 0 {
		if err := exec.HandoffEpic(parentID, specID, result.BeadIDs); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("handoff epic failed: %v", err))
		}
	}

	// Step 6: Emit recording phase marker (best-effort)
	if err := recording.EmitPhaseMarker(root, specID, "plan", "plan-approved"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not emit recording marker: %v", err))
	}
	if err := recording.UpdatePhase(root, specID, "plan", "plan-approved"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not update recording phase: %v", err))
	}

	return result, nil
}

// updatePlanApproval reads a plan file and updates YAML frontmatter with approval fields.
func updatePlanApproval(planPath, approvedBy string) error {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("reading plan: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find frontmatter boundaries
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("no frontmatter found")
	}

	fmEndIdx := -1
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			fmEndIdx = i + 1
			break
		}
	}
	if fmEndIdx == -1 {
		return fmt.Errorf("unclosed frontmatter")
	}

	// Extract and parse frontmatter
	fmLines := lines[1:fmEndIdx]
	var activeFmLines []string
	for _, line := range fmLines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			activeFmLines = append(activeFmLines, line)
		}
	}

	fmContent := strings.Join(activeFmLines, "\n")
	var fmMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmContent), &fmMap); err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}
	if fmMap == nil {
		fmMap = make(map[string]interface{})
	}

	// Update approval fields
	now := time.Now().UTC()
	fmMap["status"] = "Approved"
	fmMap["approved_at"] = now.Format(time.RFC3339)
	fmMap["approved_by"] = approvedBy

	// Re-marshal
	newFm, err := yaml.Marshal(fmMap)
	if err != nil {
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	// Splice back
	body := strings.Join(lines[fmEndIdx+1:], "\n")
	output := "---\n" + strings.TrimRight(string(newFm), "\n") + "\n---\n" + body

	return os.WriteFile(planPath, []byte(output), 0644)
}

// beadNumRe matches "Bead N" where N is the bead number in the heading.
var beadNumRe = regexp.MustCompile(`^Bead\s+(\d+)`)

// createImplementationBeads parses plan.md for ## Bead sections, creates child
// beads under the lifecycle epic, and wires inter-bead dependencies.
// Each bead is populated with description, acceptance criteria, design, and metadata
// so agents can work from `bd show <id>` alone (Spec 074).
// Returns the ordered list of created bead IDs.
func createImplementationBeads(planPath, specID, parentID string) ([]string, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("reading plan: %w", err)
	}
	planContent := string(data)

	sections := validate.ParseBeadSections(planContent)
	if len(sections) == 0 {
		return nil, nil
	}

	// --- Re-approval safeguard: close-and-recreate existing beads (Spec 074) ---
	if err := handleExistingBeads(parentID, planContent); err != nil {
		return nil, err
	}

	// --- Assemble shared context from spec.md ---
	specDir := filepath.Dir(planPath)
	specContent := readFileOrEmpty(filepath.Join(specDir, "spec.md"))

	requirements := contextpack.ExtractSection(specContent, "Requirements")
	acceptanceCriteria := contextpack.ExtractSection(specContent, "Acceptance Criteria")

	// Build design field: spec requirements + ADR decision snapshots
	design := buildDesignField(specDir, specContent, requirements)

	// --- Extract raw bead section content from plan.md ---
	sectionContent := extractBeadSectionContents(planContent)

	// Map from bead number (from heading) to created bead ID for dependency wiring.
	numToID := make(map[int]string)
	var beadIDs []string

	for _, sec := range sections {
		title := fmt.Sprintf("[%s] %s", specID, sec.Heading)

		// Get the raw work chunk for this bead
		workChunk := sectionContent[sec.Heading]

		// Extract file paths from the work chunk
		filePaths := contextpack.ExtractFilePathsFromText(workChunk)

		// Build metadata JSON
		metadataJSON := buildBeadMetadata(specID, filePaths)

		args := []string{
			"create",
			"--title", title,
			"--type", "task",
			"--parent", parentID,
			"--description", workChunk,
			"--acceptance", acceptanceCriteria,
			"--design", design,
			"--metadata", metadataJSON,
			"--json",
		}
		out, err := planRunBDFn(args...)
		if err != nil {
			return beadIDs, fmt.Errorf("creating bead for %q: %w", sec.Heading, err)
		}

		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &created); err != nil {
			return beadIDs, fmt.Errorf("parsing create output for %q: %w", sec.Heading, err)
		}

		beadIDs = append(beadIDs, created.ID)

		// Extract bead number from heading for dependency resolution.
		if m := beadNumRe.FindStringSubmatch(sec.Heading); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil {
				numToID[n] = created.ID
			}
		}
	}

	// Wire dependencies: parse "Depends on" text for "Bead N" references.
	depRe := regexp.MustCompile(`(?i)bead\s+(\d+)`)
	for i, sec := range sections {
		if sec.DependsOn == "" {
			continue
		}
		depText := strings.ToLower(sec.DependsOn)
		if depText == "none" || depText == "nothing" || depText == "n/a" {
			continue
		}

		matches := depRe.FindAllStringSubmatch(sec.DependsOn, -1)
		for _, m := range matches {
			depNum, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			depID, ok := numToID[depNum]
			if !ok {
				continue
			}
			// Wire: beadIDs[i] depends on depID
			if _, err := planRunBDFn("dep", "add", beadIDs[i], depID); err != nil {
				// Best-effort: don't fail the whole operation for a dep wiring issue
				continue
			}
		}
	}

	return beadIDs, nil
}

// handleExistingBeads checks if beads already exist under the epic (re-approval).
// If any are in_progress or closed, returns an error. If all are open, closes them.
func handleExistingBeads(parentID, planContent string) error {
	out, err := planListJSONFn("--parent", parentID)
	if err != nil {
		return nil // Can't query — proceed with creation (first approval case)
	}

	var children []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if json.Unmarshal(out, &children) != nil || len(children) == 0 {
		return nil // No existing children — first approval
	}

	// Check for in-progress or closed beads
	for _, c := range children {
		status := strings.ToLower(c.Status)
		if status == "in_progress" || status == "closed" {
			return fmt.Errorf("cannot re-approve plan: bead %s is %s — close or complete active work first", c.ID, c.Status)
		}
	}

	// All open — close them with supersede reason
	version := extractPlanVersion(planContent)
	reason := fmt.Sprintf("superseded by plan v%s", version)
	var ids []string
	for _, c := range children {
		ids = append(ids, c.ID)
	}
	if len(ids) > 0 {
		// Close via bd close with reason
		args := append([]string{"close"}, ids...)
		args = append(args, "--reason", reason)
		if _, err := planRunBDCombinedFn(args...); err != nil {
			return fmt.Errorf("closing superseded beads: %w", err)
		}
	}

	return nil
}

// extractPlanVersion reads the version field from plan frontmatter.
func extractPlanVersion(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "version:") {
			v := strings.TrimPrefix(trimmed, "version:")
			v = strings.TrimSpace(v)
			v = strings.Trim(v, `"'`)
			return v
		}
	}
	return "unknown"
}

// buildDesignField assembles the design field content: spec requirements + ADR decision snapshots.
func buildDesignField(specDir, specContent, requirements string) string {
	var parts []string

	if requirements != "" {
		parts = append(parts, "## Requirements\n\n"+requirements)
	}

	// Parse ADR IDs from the spec's ADR Touchpoints section
	touchpoints := contextpack.ExtractSection(specContent, "ADR Touchpoints")
	adrIDs := parseADRIDs(touchpoints)

	if len(adrIDs) > 0 {
		// specDir is e.g. .mindspec/docs/specs/074-slug; root is 3 levels up
		root := filepath.Join(specDir, "..", "..", "..")
		store := adr.NewFileStore(root)

		var decisions []string
		for _, id := range adrIDs {
			a, err := store.Get(id)
			if err != nil {
				continue
			}
			decision := contextpack.ExtractSection(a.Content, "Decision")
			if decision != "" {
				decisions = append(decisions, fmt.Sprintf("### %s\n\n%s", id, decision))
			}
		}
		if len(decisions) > 0 {
			parts = append(parts, "## ADR Decisions\n\n"+strings.Join(decisions, "\n\n"))
		}
	}

	return strings.Join(parts, "\n\n")
}

// adrIDRe matches ADR IDs like "ADR-0023" in markdown links or plain text.
var adrIDRe = regexp.MustCompile(`ADR-(\d{4})`)

// parseADRIDs extracts ADR IDs (e.g., "ADR-0023") from the ADR Touchpoints section text.
func parseADRIDs(touchpoints string) []string {
	matches := adrIDRe.FindAllString(touchpoints, -1)
	seen := make(map[string]bool)
	var ids []string
	for _, id := range matches {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// extractBeadSectionContents extracts the raw markdown content for each ## Bead section.
// Returns a map from heading text (e.g., "Bead 1: Populate Fields") to section content.
func extractBeadSectionContents(content string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentHeading string
	var currentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## Bead ") {
			// Save previous section
			if currentHeading != "" {
				result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			}
			currentHeading = strings.TrimPrefix(line, "## ")
			currentLines = nil
			continue
		}
		// A non-bead ## heading ends the current bead section
		if strings.HasPrefix(line, "## ") && currentHeading != "" {
			result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			currentHeading = ""
			currentLines = nil
			continue
		}
		if currentHeading != "" {
			currentLines = append(currentLines, line)
		}
	}
	if currentHeading != "" {
		result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
	}

	return result
}

// buildBeadMetadata constructs the metadata JSON string for a bead.
func buildBeadMetadata(specID string, filePaths []string) string {
	meta := map[string]interface{}{
		"spec_id":    specID,
		"file_paths": filePaths,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Sprintf(`{"spec_id":"%s"}`, specID)
	}
	return string(data)
}

// readFileOrEmpty reads a file and returns its content, or empty string on error.
func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// writeBeadIDsToFrontmatter adds the bead_ids list to the plan's YAML frontmatter.
func writeBeadIDsToFrontmatter(planPath string, beadIDs []string) error {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("reading plan: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("no frontmatter found")
	}

	fmEndIdx := -1
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			fmEndIdx = i + 1
			break
		}
	}
	if fmEndIdx == -1 {
		return fmt.Errorf("unclosed frontmatter")
	}

	// Extract and parse frontmatter
	fmLines := lines[1:fmEndIdx]
	var activeFmLines []string
	for _, line := range fmLines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			activeFmLines = append(activeFmLines, line)
		}
	}

	fmContent := strings.Join(activeFmLines, "\n")
	var fmMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmContent), &fmMap); err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}
	if fmMap == nil {
		fmMap = make(map[string]interface{})
	}

	// Convert []string to []interface{} for YAML
	ids := make([]interface{}, len(beadIDs))
	for i, id := range beadIDs {
		ids[i] = id
	}
	fmMap["bead_ids"] = ids

	newFm, err := yaml.Marshal(fmMap)
	if err != nil {
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	body := strings.Join(lines[fmEndIdx+1:], "\n")
	output := "---\n" + strings.TrimRight(string(newFm), "\n") + "\n---\n" + body

	return os.WriteFile(planPath, []byte(output), 0644)
}

// CreateBeadsFromPlan is a recovery function that creates implementation beads
// from an already-approved plan. Use this when plan-approve failed to create
// beads (e.g., bd was unreachable, CWD issue, etc.).
func CreateBeadsFromPlan(root, specID string) (*PlanResult, error) {
	result := &PlanResult{SpecID: specID}

	epicID, epicErr := phase.FindEpicBySpecID(specID)
	if epicErr != nil || epicID == "" {
		return nil, fmt.Errorf("spec %s has no epic in beads; cannot create beads", specID)
	}

	specDir := workspace.SpecDir(root, specID)
	planPath := filepath.Join(specDir, "plan.md")
	beadIDs, err := createImplementationBeads(planPath, specID, epicID)
	if err != nil {
		return nil, fmt.Errorf("creating beads: %w", err)
	}

	result.BeadIDs = beadIDs
	if len(beadIDs) > 0 {
		if err := writeBeadIDsToFrontmatter(planPath, beadIDs); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not write bead IDs to plan frontmatter: %v", err))
		}
	}

	return result, nil
}
