package specmeta

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Meta holds the YAML frontmatter fields for a spec's molecule binding.
type Meta struct {
	MoleculeID  string            `yaml:"molecule_id,omitempty"`
	StepMapping map[string]string `yaml:"step_mapping,omitempty"`
	Status      string            `yaml:"status,omitempty"`
	ApprovedAt  string            `yaml:"approved_at,omitempty"`
	ApprovedBy  string            `yaml:"approved_by,omitempty"`
}

var runBDFn = bead.RunBD

var lifecycleStepOrder = []string{"spec", "spec-approve", "plan", "plan-approve", "implement", "review"}

// Read parses the YAML frontmatter from a spec.md file under specDir.
// Returns a zero Meta (no error) if the spec has no frontmatter.
func Read(specDir string) (*Meta, error) {
	specPath := filepath.Join(specDir, "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading spec: %w", err)
	}

	fm, _ := extractFrontmatter(string(data))
	if fm == "" {
		return &Meta{}, nil
	}

	var m Meta
	if err := yaml.Unmarshal([]byte(fm), &m); err != nil {
		return nil, fmt.Errorf("parsing spec frontmatter: %w", err)
	}
	return &m, nil
}

// Write updates or inserts YAML frontmatter in a spec.md file.
// Only writes the molecule binding fields; preserves everything else in the file.
func Write(specDir string, m *Meta) error {
	specPath := filepath.Join(specDir, "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}

	content := string(data)
	existingFM, body := extractFrontmatter(content)

	// Merge: parse existing frontmatter as a map, overlay our fields
	merged := make(map[string]interface{})
	if existingFM != "" {
		if err := yaml.Unmarshal([]byte(existingFM), &merged); err != nil {
			return fmt.Errorf("parsing existing frontmatter: %w", err)
		}
	}

	if m.MoleculeID != "" {
		merged["molecule_id"] = m.MoleculeID
	}
	if len(m.StepMapping) > 0 {
		merged["step_mapping"] = m.StepMapping
	}
	if strings.TrimSpace(m.Status) != "" {
		merged["status"] = m.Status
	}
	if strings.TrimSpace(m.ApprovedAt) != "" {
		merged["approved_at"] = m.ApprovedAt
	}
	if strings.TrimSpace(m.ApprovedBy) != "" {
		merged["approved_by"] = m.ApprovedBy
	}

	fmBytes, err := yaml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n")
	sb.WriteString(body)

	return os.WriteFile(specPath, []byte(sb.String()), 0644)
}

// ReadForSpec is a convenience that reads spec meta given a project root and spec ID.
func ReadForSpec(root, specID string) (*Meta, error) {
	return Read(workspace.SpecDir(root, specID))
}

// WriteForSpec is a convenience that writes spec meta given a project root and spec ID.
func WriteForSpec(root, specID string, m *Meta) error {
	return Write(workspace.SpecDir(root, specID), m)
}

// Backfill attempts to find the molecule for a spec by searching Beads
// for the [SPEC <specID>] title convention. If found, writes the binding
// into the spec frontmatter and returns the Meta. Returns nil Meta with
// no error if no molecule is found.
func Backfill(root, specID string) (*Meta, error) {
	if err := bead.Preflight(root); err != nil {
		return nil, fmt.Errorf("beads not available: %w", err)
	}

	molID, err := findMoleculeByConvention(specID)
	if err != nil {
		return nil, fmt.Errorf("searching for molecule: %w", err)
	}
	if molID == "" {
		return nil, nil
	}

	m := &Meta{MoleculeID: molID}
	if err := WriteForSpec(root, specID, m); err != nil {
		return nil, fmt.Errorf("writing backfill binding: %w", err)
	}
	return m, nil
}

// EnsureBound returns the spec's molecule binding, performing lazy backfill
// if the spec lacks a molecule_id. Returns nil Meta (no error) if no
// molecule can be found at all.
func EnsureBound(root, specID string) (*Meta, error) {
	m, err := ReadForSpec(root, specID)
	if err != nil {
		return nil, err
	}
	if m.MoleculeID != "" {
		return m, nil
	}

	// Lazy backfill
	return Backfill(root, specID)
}

// EnsureFullyBound returns a spec's molecule binding, recovering missing fields when possible.
// It guarantees both molecule_id and step_mapping are present and internally consistent.
func EnsureFullyBound(root, specID string) (*Meta, error) {
	m, err := EnsureBound(root, specID)
	if err != nil {
		return nil, err
	}
	if m == nil || m.MoleculeID == "" {
		return nil, fmt.Errorf("spec %s has no molecule binding; run `mindspec spec-init %s` or restore spec frontmatter molecule_id", specID, specID)
	}

	needsRecovery := len(m.StepMapping) == 0
	if !needsRecovery {
		for _, key := range lifecycleStepOrder {
			if strings.TrimSpace(m.StepMapping[key]) == "" {
				needsRecovery = true
				break
			}
		}
	}

	if needsRecovery {
		recovered, err := recoverStepMapping(m.MoleculeID, specID)
		if err != nil {
			return nil, fmt.Errorf("recovering step mapping for %s (%s): %w", specID, m.MoleculeID, err)
		}
		m.StepMapping = recovered
	}

	// Keep the parent molecule ID mirrored in step_mapping for compatibility.
	if m.StepMapping == nil {
		m.StepMapping = map[string]string{}
	}
	m.StepMapping["spec-lifecycle"] = m.MoleculeID

	if err := WriteForSpec(root, specID, m); err != nil {
		return nil, fmt.Errorf("persisting molecule binding for %s: %w", specID, err)
	}

	return m, nil
}

// findMoleculeByConvention searches Beads for an epic titled "[SPEC <specID>]".
func findMoleculeByConvention(specID string) (string, error) {
	prefix := "[SPEC " + specID + "]"
	out, err := runBDFn("search", prefix, "--json")
	if err != nil {
		return "", fmt.Errorf("bd search failed: %w", err)
	}

	var results []bead.BeadInfo
	if err := json.Unmarshal(out, &results); err != nil {
		return "", fmt.Errorf("parsing search results: %w", err)
	}

	for _, r := range results {
		if r.IssueType == "epic" && strings.HasPrefix(r.Title, prefix) {
			return r.ID, nil
		}
	}
	return "", nil
}

func recoverStepMapping(moleculeID, specID string) (map[string]string, error) {
	out, err := runBDFn("mol", "show", moleculeID, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd mol show failed: %w", err)
	}

	var payload struct {
		Issues []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parsing mol show output: %w", err)
	}

	titleToStep := map[string]string{
		"Write spec " + specID:   "spec",
		"Approve spec " + specID: "spec-approve",
		"Write plan " + specID:   "plan",
		"Approve plan " + specID: "plan-approve",
		"Implement " + specID:    "implement",
		"Review " + specID:       "review",
	}

	stepMap := make(map[string]string, len(lifecycleStepOrder)+1)
	for _, issue := range payload.Issues {
		if step, ok := titleToStep[issue.Title]; ok {
			stepMap[step] = issue.ID
		}
	}
	stepMap["spec-lifecycle"] = moleculeID

	var missing []string
	for _, key := range lifecycleStepOrder {
		if strings.TrimSpace(stepMap[key]) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("could not resolve molecule steps %v from molecule %s", missing, moleculeID)
	}

	return stepMap, nil
}

// extractFrontmatter splits a markdown file into frontmatter (without delimiters)
// and body. If no frontmatter is present, returns ("", fullContent).
func extractFrontmatter(content string) (string, string) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	// First line must be "---"
	if !scanner.Scan() {
		return "", content
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return "", content
	}

	var fmLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			// Found closing delimiter
			fm := strings.Join(fmLines, "\n")
			// Body is everything after the closing ---
			var bodyLines []string
			for scanner.Scan() {
				bodyLines = append(bodyLines, scanner.Text())
			}
			body := strings.Join(bodyLines, "\n")
			// Preserve leading newline after frontmatter
			if body != "" {
				body = "\n" + body
			}
			return fm, body
		}
		fmLines = append(fmLines, line)
	}

	// No closing delimiter — treat entire content as body (no frontmatter)
	return "", content
}
