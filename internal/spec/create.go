package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/hooks"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/templates"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// specIDPattern matches NNN-kebab-case where NNN is 3+ digits.
var specIDPattern = regexp.MustCompile(`^\d{3,}-[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Result holds the output of a spec-init operation.
type Result struct {
	SpecDir      string // Path to the spec directory
	WorktreePath string // Path to the created worktree (empty if not created)
	SpecBranch   string // Name of the spec branch (empty if not created)
}

// Run creates a new spec directory with a spec.md from the template,
// then sets state to spec mode. If title is empty, it is derived from
// the slug portion of specID (e.g. "010-spec-init-cmd" → "Spec Init Cmd").
//
// ADR-0006 (zero-on-main): the workspace is created FIRST, then spec files
// are written into the workspace — never to the main worktree.
//
// The exec parameter provides workspace creation and git operations;
// enforcement content (spec files, hooks, recording) stays here.
func Run(root, specID, title string, exec executor.Executor) (*Result, error) {
	if !specIDPattern.MatchString(specID) {
		return nil, fmt.Errorf("invalid spec ID %q: must match NNN-kebab-case (e.g. 010-my-feature)", specID)
	}

	if title == "" {
		title = titleFromSlug(specID)
	}

	// --- Phase 1: Create workspace (branch + worktree via executor) ---

	ws, err := exec.InitSpecWorkspace(specID)
	if err != nil {
		return nil, fmt.Errorf("creating spec workspace: %w", err)
	}

	result := &Result{
		WorktreePath: ws.Path,
		SpecBranch:   ws.Branch,
	}

	// --- Phase 2: Write spec files into the workspace (not main) ---

	// Check for existing spec dir in the workspace.
	specDir := workspace.SpecDir(ws.Path, specID)
	if _, err := os.Stat(specDir); err == nil {
		return nil, fmt.Errorf("spec directory already exists: %s", specDir)
	}
	result.SpecDir = specDir

	// Fill placeholders and write spec.md.
	content := strings.Replace(templates.Spec(), "<ID>", specID, 1)
	content = strings.Replace(content, "<Title>", title, 1)

	if err := os.MkdirAll(specDir, 0755); err != nil {
		return nil, fmt.Errorf("creating spec directory: %w", err)
	}
	specPath := filepath.Join(specDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing spec file: %w", err)
	}

	// --- Phase 3: Auto-commit spec files to the branch ---
	// Note: Epic creation moved to `spec approve` per ADR-0023 (epic = approval gate).
	commitMsg := fmt.Sprintf("chore: initialize spec %s", specID)
	if err := exec.CommitAll(ws.Path, commitMsg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-commit spec files: %v\n", err)
	}

	// --- Phase 4: Hooks + recording ---
	// Note: No focus file written per ADR-0023 (beads is single state authority).

	// Install git hooks (best-effort, ensures Layer 1 enforcement).
	if err := hooks.InstallAll(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not install git hooks: %v\n", err)
	}

	// Start recording in the workspace (best-effort).
	if wrote, err := recording.EnsureOTLP(ws.Path); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not configure OTLP: %v\n", err)
	} else if wrote {
		fmt.Fprintln(os.Stderr, "OTLP telemetry enabled. Restart Claude Code to begin recording.")
	}

	if err := recording.StartRecording(ws.Path, specID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start recording: %v\n", err)
	}

	return result, nil
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
