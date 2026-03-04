package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/validate"
	"github.com/mindspec/mindspec/internal/workspace"
	"gopkg.in/yaml.v3"
)

// specRunBDFn is a package-level variable for testability.
var specRunBDFn = bead.RunBD

// SetSpecRunBDForTest swaps specRunBDFn for testing and returns a restore function.
func SetSpecRunBDForTest(fn func(args ...string) ([]byte, error)) func() {
	orig := specRunBDFn
	specRunBDFn = fn
	return func() { specRunBDFn = orig }
}

// SpecResult holds the result of spec approval.
type SpecResult struct {
	SpecID   string
	Warnings []string
}

// ApproveSpec validates and approves a spec, updating lifecycle and setting state.
func ApproveSpec(root, specID, approvedBy string) (*SpecResult, error) {
	result := &SpecResult{SpecID: specID}

	// Step 1: Validate (SpecDir is worktree-aware per ADR-0022)
	vr := validate.ValidateSpec(root, specID)
	if vr.HasFailures() {
		return nil, fmt.Errorf("spec validation failed:\n%s", vr.FormatText())
	}

	specDir := workspace.SpecDir(root, specID)

	// Step 2: Update spec frontmatter + markdown Approval section.
	specPath := filepath.Join(specDir, "spec.md")
	if err := updateSpecApproval(specPath, approvedBy); err != nil {
		return nil, fmt.Errorf("updating spec approval: %w", err)
	}

	// Step 3: Create lifecycle epic in beads (ADR-0023).
	// Epic creation = spec approval gate. The epic's existence is the durable record.
	specNum, specTitle := parseSpecID(specID)
	if specNum > 0 {
		if err := phase.CheckSpecNumberCollision(specNum); err != nil {
			return nil, fmt.Errorf("spec number collision: %w", err)
		}
		metadata := fmt.Sprintf(`{"spec_num":%d,"spec_title":"%s"}`, specNum, specTitle)
		epicTitle := fmt.Sprintf("[SPEC %s] %s", specID, titleFromSpecID(specID))
		out, epicErr := specRunBDFn("create", "--title", epicTitle, "--type=epic", "--metadata", metadata, "--json")
		if epicErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not create lifecycle epic: %v", epicErr))
		} else {
			var created struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(out, &created) == nil && created.ID != "" {
				// Push to Dolt so other agents see the epic immediately
				_, _ = specRunBDFn("dolt", "push")
			}
		}
	}

	// Step 3b: Scaffold plan.md if it doesn't exist, so the agent has the exact
	// structure that validation requires (frontmatter, sections, bead template).
	planPath := filepath.Join(specDir, "plan.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		scaffold := scaffoldPlan(specID)
		if err := os.WriteFile(planPath, []byte(scaffold), 0644); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not scaffold plan.md: %v", err))
		}
	}

	// Step 3c: Auto-commit approval changes so downstream branches see them.
	specWtPath := state.SpecWorktreePath(root, specID)
	commitMsg := fmt.Sprintf("chore: approve spec %s", specID)
	if err := gitops.CommitAll(specWtPath, commitMsg); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not auto-commit spec approval: %v", err))
	}

	// Step 4: Emit recording phase marker (best-effort)
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

	trimmed := strings.TrimRight(string(fmBytes), "\n")
	if hasFrontmatter {
		return "---\n" + trimmed + "\n---\n" + body, nil
	}
	return "---\n" + trimmed + "\n---\n" + content, nil
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

// scaffoldPlan generates a plan.md skeleton with the exact structure that
// validation expects, so the agent only needs to fill in content.
func scaffoldPlan(specID string) string {
	return fmt.Sprintf(`---
status: Draft
spec_id: %s
version: "1"
---
# Plan: %s

## ADR Fitness

No ADRs are relevant to this work. (Update this section if ADRs apply.)

## Testing Strategy

Unit tests will verify the implementation.

## Bead 1: <Title>

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] `+"`make test`"+` passes

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| (map spec criteria) | Bead 1 verification |
`, specID, specID)
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

// parseSpecID extracts spec_num and spec_title from a spec ID like "060-eliminate-focus-lifecycle".
func parseSpecID(specID string) (int, string) {
	dashIdx := strings.Index(specID, "-")
	if dashIdx < 0 {
		return 0, ""
	}
	numStr := specID[:dashIdx]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, ""
	}
	return num, specID[dashIdx+1:]
}

// titleFromSpecID derives a human-readable title from a spec ID slug.
func titleFromSpecID(specID string) string {
	dashIdx := strings.Index(specID, "-")
	if dashIdx < 0 {
		return specID
	}
	slug := specID[dashIdx+1:]
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
