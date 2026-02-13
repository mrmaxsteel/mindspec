package bead

import (
	"fmt"
	"os/exec"
)

// GateResult holds the result of gate creation.
type GateResult struct {
	ID    string
	Title string
	IsNew bool // true if newly created, false if reused existing
}

// CreateGate creates a human gate bead as a child of the given parent.
// Returns the created gate info.
func CreateGate(title, parentID string) (*GateResult, error) {
	args := []string{"create", title,
		"--type=gate",
		"--priority=0",
		"--json",
	}
	if parentID != "" {
		args = append(args, "--parent="+parentID)
	}

	cmd := execCommand("bd", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd create gate failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd create gate failed: %w", err)
	}

	var info BeadInfo
	if err := parseJSON(out, &info); err != nil {
		return nil, fmt.Errorf("parsing gate creation output: %w", err)
	}

	return &GateResult{
		ID:    info.ID,
		Title: info.Title,
		IsNew: true,
	}, nil
}

// FindGate searches for an open gate matching the given title prefix.
// Returns the gate info or nil if not found.
func FindGate(titlePrefix string) (*BeadInfo, error) {
	items, err := Search(titlePrefix)
	if err != nil {
		return nil, nil // search failure = treat as not found
	}
	if len(items) > 0 {
		return &items[0], nil
	}
	return nil, nil
}

// FindGateAnyStatus searches for a gate matching the title prefix in any status (open or closed).
// Used to check if a gate has been resolved.
func FindGateAnyStatus(titlePrefix string) (*BeadInfo, error) {
	cmd := execCommand("bd", "search", titlePrefix, "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // search failure = treat as not found
	}

	items, err := parseBeadList(out)
	if err != nil {
		return nil, nil
	}
	if len(items) > 0 {
		return &items[0], nil
	}
	return nil, nil
}

// FindOrCreateGate finds an existing open gate or creates a new one.
// Idempotent: returns existing gate if one matches the title prefix.
func FindOrCreateGate(title, parentID string) (*GateResult, error) {
	existing, _ := FindGate(title)
	if existing != nil {
		return &GateResult{
			ID:    existing.ID,
			Title: existing.Title,
			IsNew: false,
		}, nil
	}

	return CreateGate(title, parentID)
}

// ResolveGate resolves (closes) a gate by ID with a reason string.
func ResolveGate(gateID, reason string) error {
	args := []string{"gate", "resolve", gateID}
	if reason != "" {
		args = append(args, "--reason="+reason)
	}

	cmd := execCommand("bd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd gate resolve failed: %s", string(out))
	}
	return nil
}

// IsGateResolved checks whether a gate matching the title prefix is resolved (closed).
// Returns:
//   - true if gate exists and is closed
//   - false if gate exists and is open
//   - true if no gate exists (backward compat: no gate = no block)
func IsGateResolved(titlePrefix string) (bool, error) {
	gate, _ := FindGateAnyStatus(titlePrefix)
	if gate == nil {
		return true, nil // no gate = no block (backward compat)
	}
	return gate.Status == "closed", nil
}

// SpecGateTitle returns the conventional title for a spec approval gate.
func SpecGateTitle(specID string) string {
	return fmt.Sprintf("[GATE spec-approve %s]", specID)
}

// PlanGateTitle returns the conventional title for a plan approval gate.
func PlanGateTitle(specID string) string {
	return fmt.Sprintf("[GATE plan-approve %s]", specID)
}
