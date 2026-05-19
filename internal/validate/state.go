package validate

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Warning represents a drift warning from cross-validation.
type Warning struct {
	Field   string
	Message string
}

// CrossValidate checks focus state against artifact state and returns warnings for any drift.
func CrossValidate(root string, s *state.Focus) []Warning {
	var warnings []Warning

	switch s.Mode {
	case state.ModeSpec:
		warnings = append(warnings, validateSpecMode(root, s)...)
	case state.ModePlan:
		warnings = append(warnings, validatePlanMode(root, s)...)
	case state.ModeImplement:
		warnings = append(warnings, validateImplementMode(root, s)...)
	case state.ModeReview:
		warnings = append(warnings, validateReviewMode(root, s)...)
	}

	return warnings
}

func validateSpecMode(root string, s *state.Focus) []Warning {
	var warnings []Warning

	if s.ActiveSpec == "" {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: "State is in spec mode but no activeSpec is set",
		})
		return warnings
	}

	specDir, err := workspace.SpecDir(root, s.ActiveSpec)
	if err != nil {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: fmt.Sprintf("Invalid activeSpec ID: %v", err),
		})
		return warnings
	}
	specPath := filepath.Join(specDir, "spec.md")
	if _, statErr := os.Stat(specPath); os.IsNotExist(statErr) {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: fmt.Sprintf("Spec file not found: %s", specPath),
		})
		return warnings
	}

	// Check if spec is already approved (drift: state says spec mode but spec is approved)
	if status := readSpecApprovalStatus(specPath); status == "APPROVED" {
		warnings = append(warnings, Warning{
			Field:   "mode",
			Message: fmt.Sprintf("State says spec mode but spec %s is already APPROVED. Consider: mindspec state set --mode=plan --spec=%s", s.ActiveSpec, s.ActiveSpec),
		})
	}

	// Check if plan.md exists (drift: molecule says spec mode but agent already started planning).
	// This means the spec-approve gate was skipped — the agent jumped to plan writing
	// without running `mindspec approve spec`.
	planPath := filepath.Join(specDir, "plan.md")
	if _, err := os.Stat(planPath); err == nil {
		warnings = append(warnings, Warning{
			Field:   "mode",
			Message: fmt.Sprintf("SKIPPED GATE: spec-approve gate is still open but plan.md already exists for %s. Run `mindspec approve spec %s` to resolve the gate before continuing plan work.", s.ActiveSpec, s.ActiveSpec),
		})
	}

	return warnings
}

func validatePlanMode(root string, s *state.Focus) []Warning {
	var warnings []Warning

	if s.ActiveSpec == "" {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: "State is in plan mode but no activeSpec is set",
		})
		return warnings
	}

	specDir, err := workspace.SpecDir(root, s.ActiveSpec)
	if err != nil {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: fmt.Sprintf("Invalid activeSpec ID: %v", err),
		})
		return warnings
	}
	specPath := filepath.Join(specDir, "spec.md")
	if status := readSpecApprovalStatus(specPath); status != "APPROVED" {
		warnings = append(warnings, Warning{
			Field:   "mode",
			Message: fmt.Sprintf("State says plan mode but spec %s has status %q (expected APPROVED)", s.ActiveSpec, status),
		})
	}

	planPath := filepath.Join(specDir, "plan.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: fmt.Sprintf("State says plan mode but no plan.md found at %s", planPath),
		})
	}

	return warnings
}

func validateImplementMode(root string, s *state.Focus) []Warning {
	var warnings []Warning

	if s.ActiveSpec == "" {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: "State is in implement mode but no activeSpec is set",
		})
	}

	if s.ActiveBead == "" {
		warnings = append(warnings, Warning{
			Field:   "activeBead",
			Message: "State is in implement mode but no activeBead is set",
		})
		return warnings
	}

	// Check plan is approved via frontmatter
	specDir, err := workspace.SpecDir(root, s.ActiveSpec)
	if err != nil {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: fmt.Sprintf("Invalid activeSpec ID: %v", err),
		})
		return warnings
	}
	planPath := filepath.Join(specDir, "plan.md")
	if planStatus := frontmatter.StatusFromPath(planPath); !strings.EqualFold(planStatus, "Approved") {
		warnings = append(warnings, Warning{
			Field:   "mode",
			Message: fmt.Sprintf("State says implement mode but plan.md is not in Approved status (got %q; expected Approved)", planStatus),
		})
	}

	// Check bead status via bd show (non-fatal if bd unavailable)
	if beadWarning := checkBeadStatus(s.ActiveBead); beadWarning != "" {
		warnings = append(warnings, Warning{
			Field:   "activeBead",
			Message: beadWarning,
		})
	}

	return warnings
}

func validateReviewMode(root string, s *state.Focus) []Warning {
	var warnings []Warning

	if s.ActiveSpec == "" {
		warnings = append(warnings, Warning{
			Field:   "activeSpec",
			Message: "State is in review mode but no activeSpec is set",
		})
	}

	return warnings
}

// readSpecApprovalStatus extracts the Status field from the Approval section of a spec.
func readSpecApprovalStatus(specPath string) string {
	f, err := os.Open(specPath)
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inApproval := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## Approval") {
			inApproval = true
			continue
		}
		if inApproval && strings.HasPrefix(line, "## ") {
			break
		}
		if inApproval && strings.Contains(line, "**Status**:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "unknown"
}

// checkBeadStatus shells out to bd show to check bead status. Returns warning message or empty string.
func checkBeadStatus(beadID string) string {
	cmd := exec.Command("bd", "show", beadID)
	output, err := cmd.Output()
	if err != nil {
		// bd not available or bead not found — non-fatal
		return fmt.Sprintf("Could not verify bead %s via bd: %v", beadID, err)
	}

	out := string(output)
	if strings.Contains(out, "status: closed") || strings.Contains(out, "Status: closed") {
		return fmt.Sprintf("Bead %s appears to be closed, but state says implement mode", beadID)
	}

	return ""
}
