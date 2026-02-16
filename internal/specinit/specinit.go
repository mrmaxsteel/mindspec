package specinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

// specIDPattern matches NNN-kebab-case where NNN is 3+ digits.
var specIDPattern = regexp.MustCompile(`^\d{3,}-[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Run creates a new spec directory with a spec.md from the template,
// then sets state to spec mode. If title is empty, it is derived from
// the slug portion of specID (e.g. "010-spec-init-cmd" → "Spec Init Cmd").
func Run(root, specID, title string) error {
	if !specIDPattern.MatchString(specID) {
		return fmt.Errorf("invalid spec ID %q: must match NNN-kebab-case (e.g. 010-my-feature)", specID)
	}

	specDir := workspace.SpecDir(root, specID)
	if _, err := os.Stat(specDir); err == nil {
		return fmt.Errorf("spec directory already exists: %s", specDir)
	}

	if title == "" {
		title = titleFromSlug(specID)
	}

	// Read template
	templatePath := filepath.Join(root, "docs", "templates", "spec.md")
	tmpl, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading spec template: %w", err)
	}

	// Fill placeholders
	content := strings.Replace(string(tmpl), "<ID>", specID, 1)
	content = strings.Replace(content, "<Title>", title, 1)

	// Create directory and write spec
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return fmt.Errorf("creating spec directory: %w", err)
	}
	specPath := filepath.Join(specDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing spec file: %w", err)
	}

	// Pour the spec-lifecycle formula (best-effort — don't fail if beads not initialized)
	s := &state.State{
		Mode:       state.ModeSpec,
		ActiveSpec: specID,
	}
	if err := bead.Preflight(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: beads not available, skipping molecule creation: %v\n", err)
	} else {
		molID, stepMap, err := pourFormula(specID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not pour formula: %v\n", err)
		} else {
			s.ActiveMolecule = molID
			s.StepMapping = stepMap
			// Mark the spec step as in_progress
			if stepID, ok := stepMap["spec"]; ok {
				if _, err := bead.RunBDCombined("update", stepID, "--status=in_progress"); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not start spec step: %v\n", err)
				}
			}
		}
	}

	// Write state
	if err := state.Write(root, s); err != nil {
		return fmt.Errorf("setting state: %w", err)
	}

	// Start recording (best-effort)
	if wrote, err := recording.EnsureOTLP(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not configure OTLP: %v\n", err)
	} else if wrote {
		fmt.Fprintln(os.Stderr, "OTLP telemetry enabled. Restart Claude Code to begin recording.")
	}

	if err := recording.StartRecording(root, specID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start recording: %v\n", err)
	}

	return nil
}

// pourResult represents the JSON output from `bd mol pour --json`.
type pourResult struct {
	NewEpicID string            `json:"new_epic_id"`
	IDMapping map[string]string `json:"id_mapping"`
}

// pourFormula pours the spec-lifecycle formula and returns the molecule ID
// and a step mapping (formula step ID → beads issue ID).
func pourFormula(specID string) (string, map[string]string, error) {
	out, err := bead.RunBD("mol", "pour", "spec-lifecycle",
		"--var", "spec_id="+specID, "--json")
	if err != nil {
		return "", nil, fmt.Errorf("bd mol pour failed: %w", err)
	}

	var result pourResult
	if err := json.Unmarshal(out, &result); err != nil {
		return "", nil, fmt.Errorf("parsing pour output: %w", err)
	}

	// Build a clean step mapping: strip the formula prefix from keys
	// id_mapping keys are like "spec-lifecycle.spec" → we want just "spec"
	stepMap := make(map[string]string)
	prefix := "spec-lifecycle."
	for k, v := range result.IDMapping {
		shortKey := strings.TrimPrefix(k, prefix)
		stepMap[shortKey] = v
	}

	return result.NewEpicID, stepMap, nil
}

// titleFromSlug derives a title from a spec ID slug.
// "010-spec-init-cmd" → "Spec Init Cmd"
func titleFromSlug(specID string) string {
	// Strip leading numeric prefix (e.g. "010-")
	slug := specID
	for i, c := range slug {
		if c == '-' {
			slug = slug[i+1:]
			break
		}
		if c < '0' || c > '9' {
			break
		}
	}

	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
