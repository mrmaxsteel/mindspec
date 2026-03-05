package instruct

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/contextpack"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

//go:embed templates/*.md
var templateFS embed.FS

// SpecInfo describes an active spec for the ambiguous template.
type SpecInfo struct {
	SpecID string
	Mode   string
}

// Context holds all data needed to render guidance.
type Context struct {
	Mode             string     `json:"mode"`
	ActiveSpec       string     `json:"active_spec"`
	ActiveBead       string     `json:"active_bead"`
	ActiveWorktree   string     `json:"active_worktree"`
	InWorktree       bool       `json:"in_worktree,omitempty"`
	SpecGoal         string     `json:"spec_goal,omitempty"`
	PlanApproved     bool       `json:"plan_approved,omitempty"`
	AvailableSpecs   []string   `json:"available_specs,omitempty"`
	ActiveSpecList   []SpecInfo `json:"active_spec_list,omitempty"`
	BeadPrimer       string     `json:"bead_primer,omitempty"`
	BranchProtection bool       `json:"branch_protection,omitempty"`
	Warnings         []string   `json:"warnings,omitempty"`
}

// JSONOutput is the structured output for --format=json.
type JSONOutput struct {
	Mode       string   `json:"mode"`
	ActiveSpec string   `json:"active_spec"`
	ActiveBead string   `json:"active_bead"`
	Guidance   string   `json:"guidance"`
	Gates      []string `json:"gates"`
	Warnings   []string `json:"warnings"`
}

// BuildContext creates a rendering context from focus state and project root.
func BuildContext(root string, mc *state.Focus) *Context {
	ctx := &Context{
		Mode:           mc.Mode,
		ActiveSpec:     mc.ActiveSpec,
		ActiveBead:     mc.ActiveBead,
		ActiveWorktree: mc.ActiveWorktree,
	}

	// Check if CWD matches the active worktree
	if mc.ActiveWorktree != "" {
		cwd, _ := os.Getwd()
		if cwd != "" {
			cwdAbs, _ := filepath.Abs(cwd)
			wtAbs, _ := filepath.Abs(mc.ActiveWorktree)
			ctx.InWorktree = strings.HasPrefix(cwdAbs, wtAbs)
		}
	}

	// Load config for branch protection setting
	cfg, _ := config.Load(root)
	ctx.BranchProtection = cfg.Enforcement.PreCommitHook

	// Load spec goal if we have an active spec
	if mc.ActiveSpec != "" {
		ctx.SpecGoal = readSpecGoal(root, mc.ActiveSpec)
	}

	// Check plan approval status in plan mode
	if mc.Mode == state.ModePlan && mc.ActiveSpec != "" {
		ctx.PlanApproved = isPlanApproved(root, mc.ActiveSpec)
	}

	// AvailableSpecs removed — the disk directory listing was noise
	// (showed all historical specs, not active ones).

	// Build bead primer for implement mode with active bead (session recovery)
	if mc.Mode == state.ModeImplement && mc.ActiveBead != "" && mc.ActiveSpec != "" {
		primer, err := contextpack.BuildBeadPrimer(root, mc.ActiveSpec, mc.ActiveBead)
		if err == nil {
			ctx.BeadPrimer = contextpack.RenderBeadPrimer(primer)
		}
	}

	// Run cross-validation and collect warnings
	warnings := state.CrossValidate(root, mc)
	for _, w := range warnings {
		ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("[%s] %s", w.Field, w.Message))
	}
	if mc.Mode == state.ModeImplement && mc.ActiveBead != "" && mc.ActiveWorktree == "" {
		ctx.Warnings = append(ctx.Warnings, "[worktree] no active implement worktree is set. Run `mindspec next` before coding or committing.")
	}

	return ctx
}

// Render produces markdown guidance for the given context.
func Render(ctx *Context) (string, error) {
	tmplName := ctx.Mode + ".md"
	tmplPath := "templates/" + tmplName

	data, err := templateFS.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("loading template %s: %w", tmplName, err)
	}

	tmpl, err := template.New(tmplName).Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parsing template %s: %w", tmplName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("rendering template %s: %w", tmplName, err)
	}

	result := buf.String()

	// Append bead primer if available (implement mode with active bead)
	if ctx.BeadPrimer != "" {
		result += "\n---\n\n" + ctx.BeadPrimer
	}

	// Append warnings if any
	if len(ctx.Warnings) > 0 {
		result += "\n---\n\n## Warnings\n\n"
		for _, w := range ctx.Warnings {
			result += fmt.Sprintf("- %s\n", w)
		}
	}

	return result, nil
}

// RenderJSON produces structured JSON output.
func RenderJSON(ctx *Context) (string, error) {
	guidance, err := Render(ctx)
	if err != nil {
		return "", err
	}

	out := JSONOutput{
		Mode:       ctx.Mode,
		ActiveSpec: ctx.ActiveSpec,
		ActiveBead: ctx.ActiveBead,
		Guidance:   guidance,
		Gates:      gatesForMode(ctx.Mode),
		Warnings:   ctx.Warnings,
	}

	if out.Warnings == nil {
		out.Warnings = []string{}
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling JSON: %w", err)
	}

	return string(data), nil
}

// gatesForMode returns the human-in-the-loop gates for a given mode.
func gatesForMode(mode string) []string {
	switch mode {
	case state.ModeSpec:
		return []string{"Spec approval (run mindspec approve spec <id>)"}
	case state.ModePlan:
		return []string{
			"Plan approval (run mindspec approve plan <id>)",
			"ADR divergence (stop and inform user if ADR blocks progress)",
		}
	case state.ModeImplement:
		return []string{
			"ADR divergence (stop immediately if implementation deviates from cited ADR)",
			"Scope expansion (discovered work becomes new beads)",
		}
	case state.ModeReview:
		return []string{
			"Implementation approval (run mindspec approve impl <id>)",
		}
	default:
		return []string{}
	}
}

// isPlanApproved checks whether the plan frontmatter has status: Approved.
func isPlanApproved(root, specID string) bool {
	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		return false
	}

	// Quick scan: find "status:" in YAML frontmatter
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 || string(bytes.TrimSpace(lines[0])) != "---" {
		return false
	}
	for _, line := range lines[1:] {
		trimmed := bytes.TrimSpace(line)
		if string(trimmed) == "---" {
			break
		}
		if bytes.HasPrefix(trimmed, []byte("status:")) {
			val := string(bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("status:"))))
			return val == "Approved" || val == "\"Approved\""
		}
	}
	return false
}

// readSpecGoal extracts the Goal section from a spec file.
func readSpecGoal(root, specID string) string {
	specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return ""
	}

	// Simple extraction: find "## Goal" and take content until next "##"
	lines := bytes.Split(data, []byte("\n"))
	inGoal := false
	var goalLines [][]byte

	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("## Goal")) {
			inGoal = true
			continue
		}
		if inGoal && bytes.HasPrefix(line, []byte("## ")) {
			break
		}
		if inGoal {
			goalLines = append(goalLines, line)
		}
	}

	goal := bytes.TrimSpace(bytes.Join(goalLines, []byte("\n")))
	return string(goal)
}

// listSpecs returns the names of spec directories under the active docs root.
func listSpecs(root string) []string {
	specsDir := filepath.Join(workspace.DocsDir(root), "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil
	}

	var specs []string
	for _, e := range entries {
		if e.IsDir() {
			specs = append(specs, e.Name())
		}
	}
	return specs
}
