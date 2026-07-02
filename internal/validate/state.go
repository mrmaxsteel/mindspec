package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
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

	// Check if spec is already approved (drift: state says spec mode but spec is
	// approved). The declared status comes from the YAML frontmatter `status:`
	// field via SpecStatusAt (spec 108 R6 — one approval source of truth),
	// compared case-insensitively, not the `## Approval` prose.
	if strings.EqualFold(SpecStatusAt(specDir), "Approved") {
		warnings = append(warnings, Warning{
			Field:   "mode",
			Message: fmt.Sprintf("State says spec mode but spec %s is already APPROVED. Consider: mindspec state set --mode=plan --spec=%s", s.ActiveSpec, s.ActiveSpec),
		})
	}

	// Check if plan.md exists (drift: molecule says spec mode but agent already started planning).
	// This means the spec-approve gate was skipped — the agent jumped to plan writing
	// without running `mindspec spec approve`.
	planPath := filepath.Join(specDir, "plan.md")
	if _, err := os.Stat(planPath); err == nil {
		warnings = append(warnings, Warning{
			Field:   "mode",
			Message: fmt.Sprintf("SKIPPED GATE: spec-approve gate is still open but plan.md already exists for %s. Run `mindspec spec approve %s` to resolve the gate before continuing plan work.", s.ActiveSpec, s.ActiveSpec),
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
	// Approval status is read from the YAML frontmatter `status:` field via
	// SpecStatusAt (spec 108 R6 — one approval source of truth), compared
	// case-insensitively, not the `## Approval` prose.
	if status := SpecStatusAt(specDir); !strings.EqualFold(status, "Approved") {
		warnings = append(warnings, Warning{
			Field:   "mode",
			Message: fmt.Sprintf("State says plan mode but spec %s has status %q (expected Approved)", s.ActiveSpec, status),
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

// checkBeadStatus calls `bd show <bead> --json` via bead.RunBD (shared tracing)
// to check bead status. Returns warning message or empty string.
//
// PERF-1: routed through bead.RunBD instead of a direct exec.Command so the call
// shares trace instrumentation with the rest of the CLI and could be memoized
// in a future Cache extension. On JSON parse failure we fall back to the same
// "Could not verify bead" warning as before.
func checkBeadStatus(beadID string) string {
	output, err := bead.RunBD("show", beadID, "--json")
	if err != nil {
		// bd not available or bead not found — non-fatal
		return fmt.Sprintf("Could not verify bead %s via bd: %v", beadID, err)
	}

	var items []struct {
		Status string `json:"status"`
	}
	if jerr := json.Unmarshal(output, &items); jerr != nil || len(items) == 0 {
		// JSON parse failure — same fallback as the previous grep-based check.
		return fmt.Sprintf("Could not verify bead %s via bd: %v", beadID, jerr)
	}

	if strings.EqualFold(strings.TrimSpace(items[0].Status), "closed") {
		return fmt.Sprintf("Bead %s appears to be closed, but state says implement mode", beadID)
	}

	return ""
}
