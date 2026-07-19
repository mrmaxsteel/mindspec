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

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// templateFuncs are the helpers available to every instruct template.
// `termsafe` is the spec 116 safe-set/quote rule (internal/termsafe.Escape),
// applied at the RENDER SINK for agent-writable display strings (spec 119
// final-review O2) — e.g. templates/idle.md's lifecycle findings, which
// carry spec-dir/branch/bead names. The underlying finding strings stay
// canonical (the doctor/instruct AC-15 wording parity is asserted on the
// shared predicate text; both consumers escape at their own sinks).
// `shellsafe` is the R5 single shell-safe cd emitter's quoting primitive
// (containment.ShellSafe, ADR-0042 §4) — templates/implement.md's
// "Run `cd {{.ActiveWorktree | shellsafe}}`" line routes through it so a
// space-bearing worktree_root still renders a shell-safe, round-trippable
// cd line (AC-12).
var templateFuncs = template.FuncMap{
	"termsafe":  termsafe.Escape,
	"shellsafe": containment.ShellSafe,
}

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
	// PanelState is the rendered open-panel-rounds block (Spec 093
	// Req 14). Empty when no panel is registered — Render appends
	// nothing, preserving the zero-cost-when-no-panel contract (Req 15).
	// Populated by the explicit `--panel-state` flag or the implement
	// mode SessionStart auto-include.
	PanelState string `json:"panel_state,omitempty"`
	// LifecycleFindings carries stale-OPEN and finalize-orphan guidance
	// (Spec 119 R7): each string is a fully rendered "<message> Run
	// `<recovery command>`." line built from the SAME exported
	// internal/lifecycle predicates `mindspec doctor` consumes
	// (P8/AC-12/AC-15) — never a re-derived copy. Populated in idle mode
	// only; templates/idle.md renders it verbatim.
	LifecycleFindings []string `json:"lifecycle_findings,omitempty"`
}

// JSONOutput is the structured output for --format=json.
type JSONOutput struct {
	Mode       string   `json:"mode"`
	ActiveSpec string   `json:"active_spec"`
	ActiveBead string   `json:"active_bead"`
	Guidance   string   `json:"guidance"`
	Gates      []string `json:"gates"`
	Warnings   []string `json:"warnings"`
	// PanelState carries the rendered open-panel-rounds block (Spec 093
	// Req 14) when requested via --panel-state; omitted otherwise.
	PanelState string `json:"panel_state,omitempty"`
	// LifecycleFindings carries stale-OPEN and finalize-orphan guidance
	// (Spec 119 R7); populated in idle mode only.
	LifecycleFindings []string `json:"lifecycle_findings,omitempty"`
}

// BuildContext creates a rendering context from focus state and project root.
// Constructs a fresh phase.Cache; hot-path callers should use BuildContextWithCache.
func BuildContext(root string, mc *state.Focus) *Context {
	return BuildContextWithCache(phase.NewCache(), root, mc)
}

// BuildContextWithCache is the cache-aware variant of BuildContext.
func BuildContextWithCache(c *phase.Cache, root string, mc *state.Focus) *Context {
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

	// Check plan approval status in plan mode. Case-insensitive match (via
	// strings.EqualFold on the frontmatter status; ARCH-6 / mindspec-npd2):
	// status: approved (lowercase) now counts as approved.
	if mc.Mode == state.ModePlan && mc.ActiveSpec != "" {
		if specDir, err := workspace.SpecDir(root, mc.ActiveSpec); err == nil {
			planPath := filepath.Join(specDir, "plan.md")
			ctx.PlanApproved = strings.EqualFold(frontmatter.StatusFromPath(planPath), "Approved")
		}
	}

	// AvailableSpecs removed — the disk directory listing was noise
	// (showed all historical specs, not active ones).

	// Build bead context for implement mode with active bead (session recovery)
	if mc.Mode == state.ModeImplement && mc.ActiveBead != "" {
		rendered, err := contextpack.RenderBeadContext(mc.ActiveBead)
		if err == nil {
			ctx.BeadPrimer = rendered
		}
	}

	// Run cross-validation and collect warnings
	warnings := validate.CrossValidate(root, mc)
	for _, w := range warnings {
		ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("[%s] %s", w.Field, w.Message))
	}
	if mc.Mode == state.ModeImplement && mc.ActiveBead != "" && mc.ActiveWorktree == "" {
		ctx.Warnings = append(ctx.Warnings, "[worktree] no active implement worktree is set. Run `mindspec next` before coding or committing.")
	}

	// Spec 119 Bead 2 (R7): surface stale-OPEN and finalize-orphan
	// findings in idle guidance — the same good moment `mindspec doctor`
	// is naturally reached for, between lifecycle tasks. Scoped to idle
	// mode so active spec/plan/implement/review renders don't pay the
	// scan cost on every instruct call. Final-review F1: the INVOCATION's
	// phase.Cache is threaded into the shared aggregate scan, whose bd
	// cost is one (commonly already-memoized) epic list plus one children
	// query per ACTIVE epic — never a per-spec-dir subprocess fan-out.
	if mc.Mode == state.ModeIdle {
		ctx.LifecycleFindings = collectLifecycleFindings(root, c)
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

	tmpl, err := template.New(tmplName).Funcs(templateFuncs).Parse(string(data))
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

	// Append panel-state block if present (Spec 093 Req 14/15). Empty
	// when no panel is registered → nothing appended.
	if ctx.PanelState != "" {
		result += "\n---\n\n" + ctx.PanelState
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

// IsProtectedCheckout reports whether the current branch is protected
// (main/master per config). Git/config-only — no beads queries. Split out
// of RenderIdleIfProtected (final-review F1) so Run's protected-branch
// fast path can DECIDE without rendering: previously it invoked
// RenderIdleIfProtected, discarded the output, and re-rendered via
// handleNoState — paying the idle lifecycle scan twice per invocation.
func IsProtectedCheckout(root string) bool {
	branch, err := gitutil.CurrentBranch()
	if err != nil || branch == "" {
		return false
	}
	cfg, err := config.Load(root)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	return cfg.IsProtectedBranch(branch)
}

// RenderIdleIfProtected checks if the current branch is protected (main/master)
// and returns rendered idle guidance if so. Returns ("", false) if the branch is
// not protected or on error. This avoids beads queries for the common main-branch case.
func RenderIdleIfProtected(root string) (string, bool) {
	if !IsProtectedCheckout(root) {
		return "", false
	}
	mc := &state.Focus{Mode: state.ModeIdle}
	ctx := BuildContext(root, mc)
	output, err := Render(ctx)
	if err != nil {
		return "", false
	}
	return output, true
}

// RenderJSON produces structured JSON output.
func RenderJSON(ctx *Context) (string, error) {
	guidance, err := Render(ctx)
	if err != nil {
		return "", err
	}

	out := JSONOutput{
		Mode:              ctx.Mode,
		ActiveSpec:        ctx.ActiveSpec,
		ActiveBead:        ctx.ActiveBead,
		Guidance:          guidance,
		Gates:             gatesForMode(ctx.Mode),
		Warnings:          ctx.Warnings,
		PanelState:        ctx.PanelState,
		LifecycleFindings: ctx.LifecycleFindings,
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
		return []string{"Spec approval (run mindspec spec approve <id>)"}
	case state.ModePlan:
		return []string{
			"Plan approval (run mindspec plan approve <id>)",
			"ADR divergence (stop and inform user if ADR blocks progress)",
		}
	case state.ModeImplement:
		return []string{
			"ADR divergence (stop immediately if implementation deviates from cited ADR)",
			"Scope expansion (discovered work becomes new beads)",
		}
	case state.ModeReview:
		return []string{
			"Implementation approval (run mindspec impl approve <id>)",
		}
	default:
		return []string{}
	}
}

// readSpecGoal extracts the Goal section from a spec file.
func readSpecGoal(root, specID string) string {
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return ""
	}
	specPath := filepath.Join(specDir, "spec.md")
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

// listSpecs returns the names of spec directories under the tier-aware specs
// enumeration root (flat .mindspec/specs → canonical .mindspec/docs/specs →
// legacy docs/specs, spec 106 Req 3).
func listSpecs(root string) []string {
	specsDir := workspace.SpecsDir(root)
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
