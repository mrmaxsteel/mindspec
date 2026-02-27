package specinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/hooks"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/specmeta"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/templates"
	"github.com/mindspec/mindspec/internal/workspace"
)

// specIDPattern matches NNN-kebab-case where NNN is 3+ digits.
var specIDPattern = regexp.MustCompile(`^\d{3,}-[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

var (
	preflightFn      = bead.Preflight
	pourFormulaFn    = pourFormula
	runBDCombined    = bead.RunBDCombined
	writeSpecMeta    = specmeta.Write
	loadConfigFn     = config.Load
	createBranchFn   = gitops.CreateBranch
	branchExistsFn   = gitops.BranchExists
	worktreeCreateFn = bead.WorktreeCreate
	ensureGitignore  = gitops.EnsureGitignoreEntry
)

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
// ADR-0006 (zero-on-main): the worktree is created FIRST, then spec files
// are written into the worktree — never to the main worktree.
func Run(root, specID, title string) (*Result, error) {
	if !specIDPattern.MatchString(specID) {
		return nil, fmt.Errorf("invalid spec ID %q: must match NNN-kebab-case (e.g. 010-my-feature)", specID)
	}

	if title == "" {
		title = titleFromSlug(specID)
	}

	// --- Phase 1: Create worktree (before any file writes) ---

	cfg, cfgErr := loadConfigFn(root)
	if cfgErr != nil {
		return nil, fmt.Errorf("could not load config (required for worktree creation): %w", cfgErr)
	}

	specBranch := "spec/" + specID
	wtName := "worktree-spec-" + specID
	wtPath := cfg.WorktreePath(root, wtName)

	// Ensure .worktrees/ dir exists and is gitignored.
	if err := os.MkdirAll(filepath.Join(root, cfg.WorktreeRoot), 0755); err != nil {
		return nil, fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := ensureGitignore(root, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create spec branch from HEAD if it doesn't exist.
	if !branchExistsFn(specBranch) {
		if err := createBranchFn(specBranch, "HEAD"); err != nil {
			return nil, fmt.Errorf("creating branch %s: %w", specBranch, err)
		}
	}

	// Create worktree via beads (sets up .beads/redirect for shared DB).
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	if err := worktreeCreateFn(relWtPath, specBranch); err != nil {
		return nil, fmt.Errorf("creating worktree: %w", err)
	}

	result := &Result{
		WorktreePath: wtPath,
		SpecBranch:   specBranch,
	}

	// --- Phase 2: Write spec files into the worktree (not main) ---

	// Check for existing spec dir in the worktree.
	specDir := workspace.SpecDir(wtPath, specID)
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

	// --- Phase 3: Molecule setup (beads) ---

	s := &state.State{
		Mode:           state.ModeSpec,
		ActiveSpec:     specID,
		ActiveWorktree: wtPath,
		SpecBranch:     specBranch,
	}
	if err := preflightFn(root); err != nil {
		return nil, fmt.Errorf("creating lifecycle molecule requires beads to be available: %w", err)
	}

	// Ensure the spec-lifecycle formula exists (self-healing for projects
	// bootstrapped before the formula was included in mindspec init).
	if err := ensureFormula(root); err != nil {
		return nil, fmt.Errorf("ensuring spec-lifecycle formula: %w", err)
	}

	molID, stepMap, err := pourFormulaFn(specID)
	if err != nil {
		return nil, fmt.Errorf("pouring spec-lifecycle molecule: %w", err)
	}

	s.ActiveMolecule = molID
	s.StepMapping = stepMap

	// Rename the parent epic to follow [SPEC <id>] convention.
	epicTitle := fmt.Sprintf("[SPEC %s] %s", specID, title)
	if _, err := runBDCombined("update", molID, "--title="+epicTitle); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not rename parent epic: %v\n", err)
	}
	// Mark the spec step as in_progress.
	if stepID, ok := stepMap["spec"]; ok {
		if _, err := runBDCombined("update", stepID, "--status=in_progress"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not start spec step: %v\n", err)
		}
	}
	// Write molecule binding into spec frontmatter in the WORKTREE (ADR-0015).
	meta := &specmeta.Meta{
		MoleculeID:  molID,
		StepMapping: stepMap,
	}
	if err := writeSpecMeta(specDir, meta); err != nil {
		return nil, fmt.Errorf("writing molecule binding to spec frontmatter: %w", err)
	}

	// --- Phase 3b: Auto-commit spec files to the branch ---
	// Without this, downstream worktrees (bead branches) that branch from
	// spec/<id> would not contain spec.md or its molecule frontmatter.
	commitMsg := fmt.Sprintf("chore: initialize spec %s", specID)
	if err := gitops.CommitAll(wtPath, commitMsg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-commit spec files: %v\n", err)
	}

	// --- Phase 4: State + hooks + recording ---

	// Write state to main root (enforcement hooks read this).
	if err := state.Write(root, s); err != nil {
		return nil, fmt.Errorf("setting state: %w", err)
	}

	// Also write state to worktree root so commands work from either location.
	if err := state.Write(wtPath, s); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write state to worktree: %v\n", err)
	}

	// Install pre-commit hook (best-effort, ensures Layer 1 enforcement).
	if err := hooks.InstallPreCommit(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not install pre-commit hook: %v\n", err)
	}

	// Start recording in the worktree (best-effort).
	if wrote, err := recording.EnsureOTLP(wtPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not configure OTLP: %v\n", err)
	} else if wrote {
		fmt.Fprintln(os.Stderr, "OTLP telemetry enabled. Restart Claude Code to begin recording.")
	}

	if err := recording.StartRecording(wtPath, specID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start recording: %v\n", err)
	}

	return result, nil
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

// ensureFormula writes the spec-lifecycle formula to .beads/formulas/ if it
// does not already exist. This handles projects bootstrapped before the formula
// was included in `mindspec init`.
func ensureFormula(root string) error {
	formulaPath := filepath.Join(root, ".beads", "formulas", "spec-lifecycle.formula.toml")
	if _, err := os.Stat(formulaPath); err == nil {
		return nil // already exists
	}
	if err := os.MkdirAll(filepath.Dir(formulaPath), 0755); err != nil {
		return fmt.Errorf("creating formulas directory: %w", err)
	}
	return os.WriteFile(formulaPath, []byte(templates.SpecLifecycleFormula()), 0644)
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
