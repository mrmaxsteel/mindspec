package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Valid mode values (used for phase derivation and mode in hooks).
const (
	ModeIdle      = "idle"
	ModeSpec      = "spec"
	ModePlan      = "plan"
	ModeImplement = "implement"
	ModeReview    = "review"
	ModeDone      = "done"
)

// ValidModes lists all valid mode values.
var ValidModes = []string{ModeIdle, ModeSpec, ModePlan, ModeImplement, ModeReview}

// Focus is a data transfer struct for passing state context between packages.
// It is NOT persisted to disk — lifecycle state is derived from beads (ADR-0023).
type Focus struct {
	Mode           string `json:"mode"`
	ActiveSpec     string `json:"activeSpec,omitempty"`
	ActiveBead     string `json:"activeBead,omitempty"`
	ActiveWorktree string `json:"activeWorktree,omitempty"`
	SpecBranch     string `json:"specBranch,omitempty"`
}

// Session holds transient per-session metadata persisted at .mindspec/session.json.
type Session struct {
	SessionSource    string `json:"sessionSource,omitempty"`
	SessionStartedAt string `json:"sessionStartedAt,omitempty"`
	BeadClaimedAt    string `json:"beadClaimedAt,omitempty"`
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
