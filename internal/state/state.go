package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mindspec/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

// Valid mode values (used for phase in lifecycle.yaml and mode in hooks).
const (
	ModeIdle      = "idle"
	ModeSpec      = "spec"
	ModePlan      = "plan"
	ModeImplement = "implement"
	ModeReview    = "review"
)

// ValidModes lists all valid mode values.
var ValidModes = []string{ModeIdle, ModeSpec, ModePlan, ModeImplement, ModeReview}

// Session holds transient per-session metadata persisted at .mindspec/session.json.
type Session struct {
	SessionSource    string `json:"sessionSource,omitempty"`
	SessionStartedAt string `json:"sessionStartedAt,omitempty"`
	BeadClaimedAt    string `json:"beadClaimedAt,omitempty"`
}

// Lifecycle holds per-spec lifecycle state, persisted at docs/specs/<id>/lifecycle.yaml.
// This is the authoritative source of truth for a spec's lifecycle phase.
type Lifecycle struct {
	Phase  string `yaml:"phase"`
	EpicID string `yaml:"epic_id,omitempty"`
}

// Focus tracks which spec and bead the user is currently focused on.
// Persisted at .mindspec/focus. This is a cursor, not lifecycle state.
type Focus struct {
	Mode           string `json:"mode"`
	ActiveSpec     string `json:"activeSpec,omitempty"`
	ActiveBead     string `json:"activeBead,omitempty"`
	ActiveWorktree string `json:"activeWorktree,omitempty"`
	SpecBranch     string `json:"specBranch,omitempty"`
	Timestamp      string `json:"timestamp"`
}

// ReadSession loads the session from .mindspec/session.json under root.
// Returns a zero Session (no error) if the file does not exist.
func ReadSession(root string) (*Session, error) {
	path := workspace.SessionPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Session{}, nil
		}
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing session file: %w", err)
	}
	return &s, nil
}

// WriteSessionFile persists the session to .mindspec/session.json under root.
func WriteSessionFile(root string, s *Session) error {
	dir := workspace.MindspecDir(root)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating .mindspec directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}
	data = append(data, '\n')

	return os.WriteFile(workspace.SessionPath(root), data, 0644)
}

// ReadLifecycle loads the lifecycle state from docs/specs/<id>/lifecycle.yaml.
// Returns nil (no error) if the file does not exist.
func ReadLifecycle(specDir string) (*Lifecycle, error) {
	path := filepath.Join(specDir, "lifecycle.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading lifecycle.yaml: %w", err)
	}

	var lc Lifecycle
	if err := yaml.Unmarshal(data, &lc); err != nil {
		return nil, fmt.Errorf("parsing lifecycle.yaml: %w", err)
	}
	return &lc, nil
}

// WriteLifecycle persists the lifecycle state to docs/specs/<id>/lifecycle.yaml.
func WriteLifecycle(specDir string, lc *Lifecycle) error {
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return fmt.Errorf("creating spec directory: %w", err)
	}

	data, err := yaml.Marshal(lc)
	if err != nil {
		return fmt.Errorf("marshaling lifecycle: %w", err)
	}

	return os.WriteFile(filepath.Join(specDir, "lifecycle.yaml"), data, 0644)
}

// ReadFocus loads the focus cursor from .mindspec/focus under root.
// Returns nil (no error) if the file does not exist.
func ReadFocus(root string) (*Focus, error) {
	path := workspace.FocusPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading focus: %w", err)
	}

	var f Focus
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing focus: %w", err)
	}
	return &f, nil
}

// WriteFocus persists the focus cursor to .mindspec/focus under root.
func WriteFocus(root string, f *Focus) error {
	dir := workspace.MindspecDir(root)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating .mindspec directory: %w", err)
	}

	f.Timestamp = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling focus: %w", err)
	}
	data = append(data, '\n')

	return os.WriteFile(workspace.FocusPath(root), data, 0644)
}

// SpecBranch returns the canonical branch name for a spec.
func SpecBranch(specID string) string { return "spec/" + specID }

// SpecWorktreePath returns the canonical worktree path for a spec.
func SpecWorktreePath(root, specID string) string {
	return filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
}

// BeadWorktreePath returns the canonical worktree path for a bead
// nested under its spec's worktree.
func BeadWorktreePath(specWorktree, beadID string) string {
	return filepath.Join(specWorktree, ".worktrees", "worktree-"+beadID)
}

// IsValidMode reports whether mode is a recognized lifecycle mode.
func IsValidMode(mode string) bool {
	for _, m := range ValidModes {
		if m == mode {
			return true
		}
	}
	return false
}
