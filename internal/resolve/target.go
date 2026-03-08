package resolve

import (
	"fmt"
	"strings"
	"unicode"

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

// ResolveSpecPrefix resolves a numeric prefix (e.g. "077") to a full spec ID
// (e.g. "077-execution-layer-interface"). If the input already contains a hyphen,
// it is assumed to be a full spec ID and returned as-is.
func ResolveSpecPrefix(prefix string) (string, error) {
	// If it contains a hyphen, treat as full spec ID — pass through.
	if strings.Contains(prefix, "-") {
		return prefix, nil
	}

	// Must be numeric-only to be a prefix.
	for _, r := range prefix {
		if !unicode.IsDigit(r) {
			return prefix, nil
		}
	}

	// Pad to 3 digits for matching.
	padded := fmt.Sprintf("%03s", prefix)

	// Query active specs first.
	activeSpecs, err := phase.DiscoverActiveSpecs()
	if err == nil {
		var matches []string
		for _, as := range activeSpecs {
			if len(as.SpecID) >= 3 && as.SpecID[:3] == padded {
				matches = append(matches, as.SpecID)
			}
		}
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			// Fall through to error.
		default:
			return "", fmt.Errorf("ambiguous spec prefix %q matches multiple specs: %s", prefix, strings.Join(matches, ", "))
		}
	}

	return "", fmt.Errorf("no spec found matching prefix %q", prefix)
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
	// Explicit target — resolve prefix if needed.
	if specFlag != "" {
		return ResolveSpecPrefix(specFlag)
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
