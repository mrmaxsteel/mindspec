package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindspec/mindspec/internal/trace"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Valid mode values.
const (
	ModeIdle      = "idle"
	ModeExplore   = "explore"
	ModeSpec      = "spec"
	ModePlan      = "plan"
	ModeImplement = "implement"
	ModeReview    = "review"
)

// ValidModes lists all valid mode values.
var ValidModes = []string{ModeIdle, ModeExplore, ModeSpec, ModePlan, ModeImplement, ModeReview}

// State represents the MindSpec workflow state persisted at .mindspec/state.json.
type State struct {
	Mode           string            `json:"mode"`
	ActiveSpec     string            `json:"activeSpec"`
	ActiveBead     string            `json:"activeBead"`
	ActiveWorktree string            `json:"activeWorktree,omitempty"`
	SpecBranch     string            `json:"specBranch,omitempty"`
	ActiveMolecule string            `json:"activeMolecule,omitempty"`
	StepMapping    map[string]string `json:"stepMapping,omitempty"`
	NeedsClear     bool              `json:"needs_clear,omitempty"`
	LastUpdated    string            `json:"lastUpdated"`
}

// ErrNoState is returned when .mindspec/state.json does not exist.
var ErrNoState = errors.New("no .mindspec/state.json found")

// Read loads the state from .mindspec/state.json under root.
func Read(root string) (*State, error) {
	path := workspace.StatePath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoState
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &s, nil
}

// Write persists the state to .mindspec/state.json under root.
// Creates the .mindspec/ directory if it doesn't exist.
// If root is inside a git worktree, also propagates state to the main
// worktree so enforcement hooks (which read from main's CWD) stay current.
func Write(root string, s *State) error {
	dir := workspace.MindspecDir(root)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating .mindspec directory: %w", err)
	}

	s.LastUpdated = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	data = append(data, '\n')

	path := workspace.StatePath(root)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	// Dual-write: if we're in a worktree, also write to the main worktree.
	if mainRoot, ok := mainWorktreeRoot(root); ok && mainRoot != root {
		mainDir := workspace.MindspecDir(mainRoot)
		if err := os.MkdirAll(mainDir, 0755); err == nil {
			mainPath := workspace.StatePath(mainRoot)
			// Best-effort — don't fail the primary write if this fails.
			_ = os.WriteFile(mainPath, data, 0644)
		}
	}

	return nil
}

// ClearNeedsClear reads state, sets NeedsClear to false, and writes it back.
// Used by the SessionStart hook after a context clear.
func ClearNeedsClear(root string) error {
	s, err := Read(root)
	if err != nil {
		return err
	}
	s.NeedsClear = false
	return Write(root, s)
}

// mainWorktreeRoot returns the main worktree's root path if the given root
// is inside a git worktree. Returns ("", false) if root is the main worktree
// or detection fails.
func mainWorktreeRoot(root string) (string, bool) {
	// In a git worktree, .git is a file containing "gitdir: <path>".
	// In the main worktree, .git is a directory.
	gitPath := filepath.Join(root, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		// This is the main worktree — no propagation needed.
		return "", false
	}

	// .git is a file — read it to find the main repo.
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", false
	}
	// Format: "gitdir: /path/to/main/.git/worktrees/<name>\n"
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", false
	}
	gitdir := strings.TrimPrefix(line, "gitdir: ")

	// Walk up from gitdir to find the main .git directory.
	// gitdir is typically: <main>/.git/worktrees/<name>
	// We need: <main>
	idx := strings.Index(gitdir, filepath.Join(".git", "worktrees"))
	if idx <= 0 {
		return "", false
	}
	mainRoot := gitdir[:idx-1] // strip trailing separator

	// Verify the main root has .mindspec/
	if _, err := os.Stat(workspace.MindspecDir(mainRoot)); err != nil {
		return "", false
	}

	return mainRoot, true
}

// SetMode validates inputs and writes a new state. Emits a trace event on transition.
func SetMode(root, mode, spec, bead string) error {
	return SetModeWithMetadata(root, mode, spec, bead, "", nil)
}

// SetModeWithMetadata validates inputs and writes a new state.
// If molecule metadata is provided, it is written into state.
// Otherwise, metadata is preserved across transitions for the same active spec.
func SetModeWithMetadata(root, mode, spec, bead, moleculeID string, stepMapping map[string]string) error {
	// Read previous state for trace event
	prevMode := "none"
	var prev *State
	if p, err := Read(root); err == nil {
		prev = p
		prevMode = p.Mode
	}
	trace.Emit(trace.NewEvent("state.transition").
		WithSpec(spec).
		WithData(map[string]any{
			"from":    prevMode,
			"to":      mode,
			"spec_id": spec,
		}))
	if !isValidMode(mode) {
		return fmt.Errorf("invalid mode %q: must be one of %v", mode, ValidModes)
	}

	if mode == ModeSpec || mode == ModePlan || mode == ModeImplement || mode == ModeReview {
		if spec == "" {
			return fmt.Errorf("mode %q requires --spec", mode)
		}
		specDir := workspace.SpecDir(root, spec)
		if _, err := os.Stat(specDir); os.IsNotExist(err) {
			return fmt.Errorf("spec directory does not exist: %s", specDir)
		}
	}

	if mode == ModeImplement && bead == "" {
		return fmt.Errorf("mode %q requires --bead", mode)
	}

	s := &State{
		Mode:       mode,
		ActiveSpec: spec,
		ActiveBead: bead,
	}
	if spec != "" && mode != ModeIdle {
		if moleculeID != "" {
			s.ActiveMolecule = moleculeID
			s.StepMapping = copyStepMapping(stepMapping)
		} else if prev != nil && prev.ActiveSpec == spec {
			s.ActiveMolecule = prev.ActiveMolecule
			s.StepMapping = copyStepMapping(prev.StepMapping)
		}
		// Preserve worktree/branch bindings across transitions for the same spec.
		if prev != nil && prev.ActiveSpec == spec {
			s.ActiveWorktree = prev.ActiveWorktree
			s.SpecBranch = prev.SpecBranch
		}
	}

	return Write(root, s)
}

func copyStepMapping(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isValidMode(mode string) bool {
	for _, m := range ValidModes {
		if m == mode {
			return true
		}
	}
	return false
}
