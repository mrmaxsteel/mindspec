package approve

import (
	"fmt"
	"os"
	"path/filepath"
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

// runBDFn is a package-level variable for testability.
var runBDCombinedFn = bead.RunBDCombined

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

	// Step 2: Resolve and enforce molecule binding before mutating artifacts.
	meta, err := specmeta.EnsureFullyBound(root, specID)
	if err != nil {
		return nil, fmt.Errorf("resolving molecule binding for %s: %w", specID, err)
	}

	// Step 3: Update spec frontmatter + markdown Approval section.
	specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
	if err := updateSpecApproval(specPath, approvedBy); err != nil {
		return nil, fmt.Errorf("updating spec approval: %w", err)
	}

	// Step 3b: Auto-commit approval changes so downstream branches see them.
	if s, sErr := state.Read(root); sErr == nil && s.ActiveWorktree != "" {
		commitMsg := fmt.Sprintf("chore: approve spec %s", specID)
		if err := gitops.CommitAll(s.ActiveWorktree, commitMsg); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not auto-commit spec approval: %v", err))
		}
	}

	// Step 4: Close spec-approve step in molecule (best-effort).
	stepID := strings.TrimSpace(meta.StepMapping["spec-approve"])
	if stepID == "" {
		return nil, fmt.Errorf("spec %s is missing step_mapping.spec-approve; re-run `mindspec spec-init %s` or repair frontmatter", specID, specID)
	}
	if _, err := runBDCombinedFn("close", stepID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not close spec-approve step: %v", err))
	} else {
		result.GateID = stepID
	}

	// Step 5: Set state to plan mode while retaining molecule metadata.
	if err := state.SetModeWithMetadata(root, state.ModePlan, specID, "", meta.MoleculeID, meta.StepMapping); err != nil {
		return nil, fmt.Errorf("setting state: %w", err)
	}

	// Step 6: Emit recording phase marker (best-effort)
	if err := recording.EmitPhaseMarker(root, specID, "spec", "plan"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not emit recording marker: %v", err))
	}
	if err := recording.UpdatePhase(root, specID, "spec", "plan"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not update recording phase: %v", err))
	}

	return result, nil
}

// updateSpecApproval reads a spec file and updates canonical frontmatter fields
// plus the markdown ## Approval section.
func updateSpecApproval(specPath, approvedBy string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}

	now := time.Now().UTC()
	content, err := upsertSpecFrontmatterApproval(string(data), approvedBy, now)
	if err != nil {
		return err
	}
	content = upsertSpecApprovalSection(content, approvedBy, now)

	return os.WriteFile(specPath, []byte(content), 0644)
}

func upsertSpecFrontmatterApproval(content, approvedBy string, now time.Time) (string, error) {
	fm, body, hasFrontmatter := splitFrontmatter(content)

	fmMap := map[string]interface{}{}
	if hasFrontmatter && strings.TrimSpace(fm) != "" {
		if err := yaml.Unmarshal([]byte(fm), &fmMap); err != nil {
			return "", fmt.Errorf("parsing frontmatter: %w", err)
		}
	}

	fmMap["status"] = "Approved"
	fmMap["approved_at"] = now.Format(time.RFC3339)
	fmMap["approved_by"] = approvedBy

	fmBytes, err := yaml.Marshal(fmMap)
	if err != nil {
		return "", fmt.Errorf("marshaling frontmatter: %w", err)
	}

	if hasFrontmatter {
		return "---\n" + string(fmBytes) + "---\n" + body, nil
	}
	return "---\n" + string(fmBytes) + "---\n" + content, nil
}

func upsertSpecApprovalSection(content, approvedBy string, now time.Time) string {
	lines := strings.Split(content, "\n")

	approvalStart := -1
	approvalEnd := len(lines)
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

	newApproval := []string{
		"## Approval",
		"",
		"- **Status**: APPROVED",
		fmt.Sprintf("- **Approved By**: %s", approvedBy),
		fmt.Sprintf("- **Approval Date**: %s", now.Format("2006-01-02")),
		"- **Notes**: Approved via mindspec approve spec",
	}

	if approvalStart == -1 {
		base := strings.TrimRight(content, "\n")
		if base == "" {
			return strings.Join(newApproval, "\n") + "\n"
		}
		return base + "\n\n" + strings.Join(newApproval, "\n") + "\n"
	}

	var result []string
	result = append(result, lines[:approvalStart]...)
	result = append(result, newApproval...)
	result = append(result, lines[approvalEnd:]...)
	return strings.Join(result, "\n")
}

func splitFrontmatter(content string) (frontmatter string, body string, hasFrontmatter bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", content, false
	}

	fmEnd := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fmEnd = i
			break
		}
	}
	if fmEnd == -1 {
		return "", content, false
	}

	return strings.Join(lines[1:fmEnd], "\n"), strings.Join(lines[fmEnd+1:], "\n"), true
}
