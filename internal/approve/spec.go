package approve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/contextpack"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/validate"
)

// SpecResult holds the result of spec approval.
type SpecResult struct {
	SpecID   string
	GateID   string // empty if no gate found
	Warnings []string
}

// ApproveSpec validates and approves a spec, resolving its gate and setting state.
func ApproveSpec(root, specID, approvedBy string) (*SpecResult, error) {
	result := &SpecResult{SpecID: specID}

	// Step 1: Validate
	vr := validate.ValidateSpec(root, specID)
	if vr.HasFailures() {
		return nil, fmt.Errorf("spec validation failed:\n%s", vr.FormatText())
	}

	// Step 2: Update spec frontmatter (Approval section)
	specPath := filepath.Join(root, "docs", "specs", specID, "spec.md")
	if err := updateSpecApproval(specPath, approvedBy); err != nil {
		return nil, fmt.Errorf("updating spec approval: %w", err)
	}

	// Step 3: Create spec bead + gate (best-effort, idempotent)
	beadResult, err := bead.CreateSpecBead(root, specID)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not create spec bead: %v", err))
	} else if beadResult.GateID != "" {
		// Gate was created or already exists
	}

	// Step 4: Resolve spec gate (best-effort)
	gateTitle := bead.SpecGateTitle(specID)
	gate, _ := bead.FindGateAnyStatus(gateTitle)
	if gate != nil {
		if gate.Status != "closed" {
			reason := fmt.Sprintf("Spec %s approved via mindspec approve spec", specID)
			if err := bead.ResolveGate(gate.ID, reason); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not resolve spec gate: %v", err))
			} else {
				result.GateID = gate.ID
			}
		} else {
			result.GateID = gate.ID // already resolved
		}
	} else {
		result.Warnings = append(result.Warnings, "no spec gate found (legacy beads — proceeding without gate)")
	}

	// Step 5: Generate context pack (best-effort)
	pack, err := contextpack.Build(root, specID, contextpack.ModePlan)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not generate context pack: %v", err))
	} else {
		if err := pack.WriteToFile(root, specID); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not write context pack: %v", err))
		}
	}

	// Step 6: Set state to plan mode
	if err := state.SetMode(root, state.ModePlan, specID, ""); err != nil {
		return nil, fmt.Errorf("setting state: %w", err)
	}

	// Step 7: Emit recording phase marker (best-effort)
	if err := recording.EmitPhaseMarker(root, specID, "spec", "plan"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not emit recording marker: %v", err))
	}
	if err := recording.UpdatePhase(root, specID, "spec", "plan"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not update recording phase: %v", err))
	}

	return result, nil
}

// updateSpecApproval reads a spec file and updates the ## Approval section.
func updateSpecApproval(specPath, approvedBy string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find the ## Approval section
	approvalStart := -1
	approvalEnd := len(lines) // default to end of file
	for i, line := range lines {
		if strings.HasPrefix(line, "## Approval") {
			approvalStart = i
			continue
		}
		if approvalStart >= 0 && strings.HasPrefix(line, "## ") {
			approvalEnd = i
			break
		}
	}

	if approvalStart == -1 {
		return fmt.Errorf("no ## Approval section found in spec")
	}

	// Build new approval section
	today := time.Now().UTC().Format("2006-01-02")
	newApproval := []string{
		"## Approval",
		"",
		"- **Status**: APPROVED",
		fmt.Sprintf("- **Approved By**: %s", approvedBy),
		fmt.Sprintf("- **Approval Date**: %s", today),
		"- **Notes**: Approved via mindspec approve spec",
	}

	// Splice: lines before approval + new approval + lines after approval section
	var result []string
	result = append(result, lines[:approvalStart]...)
	result = append(result, newApproval...)
	result = append(result, lines[approvalEnd:]...)

	output := strings.Join(result, "\n")
	return os.WriteFile(specPath, []byte(output), 0644)
}
