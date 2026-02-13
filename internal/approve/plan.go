package approve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/validate"

	"gopkg.in/yaml.v3"
)

// PlanResult holds the result of plan approval.
type PlanResult struct {
	SpecID   string
	GateID   string // empty if no gate found
	Warnings []string
}

// ApprovePlan validates and approves a plan, resolving its gate and setting state.
func ApprovePlan(root, specID string) (*PlanResult, error) {
	result := &PlanResult{SpecID: specID}

	// Step 1: Validate
	vr := validate.ValidatePlan(root, specID)
	if vr.HasFailures() {
		return nil, fmt.Errorf("plan validation failed:\n%s", vr.FormatText())
	}

	// Step 2: Update plan frontmatter
	planPath := filepath.Join(root, "docs", "specs", specID, "plan.md")
	if err := updatePlanApproval(planPath); err != nil {
		return nil, fmt.Errorf("updating plan approval: %w", err)
	}

	// Step 3: Resolve plan gate (best-effort)
	gateTitle := bead.PlanGateTitle(specID)
	gate, _ := bead.FindGateAnyStatus(gateTitle)
	if gate != nil {
		if gate.Status != "closed" {
			reason := fmt.Sprintf("Plan %s approved via mindspec approve plan", specID)
			if err := bead.ResolveGate(gate.ID, reason); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not resolve plan gate: %v", err))
			} else {
				result.GateID = gate.ID
			}
		} else {
			result.GateID = gate.ID // already resolved
		}
	} else {
		result.Warnings = append(result.Warnings, "no plan gate found (legacy beads — proceeding without gate)")
	}

	// Step 4: Set state to plan mode (approved)
	// Note: implement mode requires a bead ID. The user runs `mindspec next`
	// to claim work and transition to implement mode.
	if err := state.SetMode(root, state.ModePlan, specID, ""); err != nil {
		return nil, fmt.Errorf("setting state: %w", err)
	}

	return result, nil
}

// updatePlanApproval reads a plan file and updates YAML frontmatter with approval fields.
func updatePlanApproval(planPath string) error {
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
	fmMap["approved_by"] = "user"

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
