package specinit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Run creates a new spec directory with a spec.md from the template,
// then sets state to spec mode. If title is empty, it is derived from
// the slug portion of specID (e.g. "010-spec-init-cmd" → "Spec Init Cmd").
func Run(root, specID, title string) error {
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

	// Set state to spec mode
	s := &state.State{
		Mode:       state.ModeSpec,
		ActiveSpec: specID,
	}
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
