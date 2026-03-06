package resolve

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ErrAmbiguousTarget is returned when multiple active specs exist and no --spec was provided.
type ErrAmbiguousTarget struct {
	Active []SpecStatus
}

func (e *ErrAmbiguousTarget) Error() string {
	var sb strings.Builder
	sb.WriteString("multiple active specs found; use --spec to target one:\n")
	for _, s := range e.Active {
		sb.WriteString(fmt.Sprintf("  %s  (mode: %s)\n", s.SpecID, s.Mode))
	}
	return sb.String()
}

// ResolveTarget determines which spec to operate on.
//
// Resolution order:
//  1. If specFlag is provided (from --spec), use it directly.
//  2. Derive from beads context (worktree path + beads query).
//  3. Query active specs; if exactly one, auto-select it.
//  4. If multiple active specs exist, return ErrAmbiguousTarget.
//  5. If nothing found, return an error.
func ResolveTarget(root, specFlag string) (string, error) {
	// Explicit target
	if specFlag != "" {
		return specFlag, nil
	}

	// Worktree-derived context: if CWD is inside a spec/bead worktree,
	// resolve directly from the path — no need to query all active specs.
	// Skip this for main worktree (it would call DiscoverActiveSpecs
	// internally, which we do below via ActiveSpecs anyway).
	kind, _, _ := workspace.DetectWorktreeContext(root)
	if kind != workspace.WorktreeMain {
		ctx, err := phase.ResolveContextFromDir(root, root)
		if err == nil && ctx != nil && ctx.SpecID != "" {
			return ctx.SpecID, nil
		}
	}

	// Query active specs (main worktree or worktree context didn't resolve)
	active, err := ActiveSpecs(root)
	if err == nil {
		switch len(active) {
		case 1:
			return active[0].SpecID, nil
		default:
			if len(active) > 1 {
				return "", &ErrAmbiguousTarget{Active: active}
			}
		}
	}

	return "", fmt.Errorf("no active specs found; use --spec flag")
}
