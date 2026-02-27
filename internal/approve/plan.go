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

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/specmeta"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/validate"
	"github.com/mindspec/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

// planRunBDCombinedFn is a package-level variable for testability.
var planRunBDCombinedFn = bead.RunBDCombined

// planRunBDFn is for JSON-returning bd commands (stdout only, no stderr mixing).
var planRunBDFn = bead.RunBD

// PlanResult holds the result of plan approval.
type PlanResult struct {
	SpecID   string
	GateID   string // empty if no gate found
	BeadIDs  []string
	Warnings []string
}

// ApprovePlan validates and approves a plan, resolving its gate and setting state.
func ApprovePlan(root, specID, approvedBy string) (*PlanResult, error) {
	result := &PlanResult{SpecID: specID}

	// Step 1: Validate
	vr := validate.ValidatePlan(root, specID)
	if vr.HasFailures() {
		return nil, fmt.Errorf("plan validation failed:\n%s", vr.FormatText())
	}

	// Step 2: Resolve and enforce molecule binding before mutating artifacts.
	meta, err := specmeta.EnsureFullyBound(root, specID)
	if err != nil {
		return nil, fmt.Errorf("resolving molecule binding for %s: %w", specID, err)
	}

	// Step 2: Update plan frontmatter
	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
	if err := updatePlanApproval(planPath, approvedBy); err != nil {
		return nil, fmt.Errorf("updating plan approval: %w", err)
	}

	// Step 2b: Auto-create implementation beads from plan sections
	implementStepID := strings.TrimSpace(meta.StepMapping["implement"])
	if implementStepID != "" {
		beadIDs, err := createImplementationBeads(planPath, specID, implementStepID)
		if err != nil {
			return nil, fmt.Errorf("failed to create implementation beads: %w\n\nThe plan has been approved but beads were NOT created.\nFix the issue and re-run: mindspec bead create-from-plan %s", err, specID)
		} else if len(beadIDs) > 0 {
			result.BeadIDs = beadIDs
			if err := writeBeadIDsToFrontmatter(planPath, beadIDs); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not write bead IDs to plan frontmatter: %v", err))
			}
		} else {
			// Plan has no ## Bead sections — warn loudly
			result.Warnings = append(result.Warnings, "plan has no '## Bead N:' sections; no implementation beads were created. Add bead sections to the plan or create beads manually with: bd create --title '[SPEC] Bead N: ...' --parent "+implementStepID)
		}
	} else {
		result.Warnings = append(result.Warnings, "no implement step in molecule; skipping bead auto-creation")
	}

	// Step 2c: Auto-commit plan approval + bead_ids so implementation
	// worktrees that branch from spec/<id> contain the approved artifacts.
	if s, sErr := state.Read(root); sErr == nil && s.ActiveWorktree != "" {
		commitMsg := fmt.Sprintf("chore: approve plan for %s", specID)
		if err := gitops.CommitAll(s.ActiveWorktree, commitMsg); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not auto-commit plan approval: %v", err))
		}
	}

	// Step 3: Close plan-approve step in molecule (best-effort)
	stepID := strings.TrimSpace(meta.StepMapping["plan-approve"])
	if stepID == "" {
		return nil, fmt.Errorf("spec %s is missing step_mapping.plan-approve; re-run `mindspec spec-init %s` or repair frontmatter", specID, specID)
	}
	if _, err := planRunBDCombinedFn("close", stepID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not close plan-approve step: %v", err))
	} else {
		result.GateID = stepID
	}

	// Step 5: Set state to plan mode (approved)
	// Note: implement mode requires a bead ID. The user runs `mindspec next`
	// to claim work and transition to implement mode.
	if err := state.SetModeWithMetadata(root, state.ModePlan, specID, "", meta.MoleculeID, meta.StepMapping); err != nil {
		return nil, fmt.Errorf("setting state: %w", err)
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
	output := "---\n" + string(newFm) + "---\n" + body

	return os.WriteFile(planPath, []byte(output), 0644)
}

// beadNumRe matches "Bead N" where N is the bead number in the heading.
var beadNumRe = regexp.MustCompile(`^Bead\s+(\d+)`)

// createImplementationBeads parses plan.md for ## Bead sections, creates child
// beads under the implement molecule step, and wires inter-bead dependencies.
// Returns the ordered list of created bead IDs.
func createImplementationBeads(planPath, specID, parentID string) ([]string, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("reading plan: %w", err)
	}

	sections := validate.ParseBeadSections(string(data))
	if len(sections) == 0 {
		return nil, nil
	}

	// Map from bead number (from heading) to created bead ID for dependency wiring.
	numToID := make(map[int]string)
	var beadIDs []string

	for _, sec := range sections {
		title := fmt.Sprintf("[%s] %s", specID, sec.Heading)
		args := []string{"create", "--title", title, "--type", "task", "--parent", parentID, "--json"}
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
	output := "---\n" + string(newFm) + "---\n" + body

	return os.WriteFile(planPath, []byte(output), 0644)
}

// CreateBeadsFromPlan is a recovery function that creates implementation beads
// from an already-approved plan. Use this when plan-approve failed to create
// beads (e.g., bd was unreachable, CWD issue, etc.).
func CreateBeadsFromPlan(root, specID string) (*PlanResult, error) {
	result := &PlanResult{SpecID: specID}

	meta, err := specmeta.EnsureFullyBound(root, specID)
	if err != nil {
		return nil, fmt.Errorf("resolving molecule binding for %s: %w", specID, err)
	}

	implementStepID := strings.TrimSpace(meta.StepMapping["implement"])
	if implementStepID == "" {
		return nil, fmt.Errorf("spec %s has no implement step in molecule; cannot create beads", specID)
	}

	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
	beadIDs, err := createImplementationBeads(planPath, specID, implementStepID)
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
