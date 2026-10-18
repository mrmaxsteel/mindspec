package resolve

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
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
		sb.WriteString(fmt.Sprintf("  %s  (mode: %s)\n", idrender.Spec(s.SpecID), s.Mode))
	}
	return sb.String()
}

// ResolveSpecPrefix resolves a numeric prefix (e.g. "077") to a full spec ID
// (e.g. "077-execution-layer-interface"). If the input already contains a hyphen,
// it is assumed to be a full spec ID and returned as-is.
func ResolveSpecPrefix(prefix string) (string, error) {
	return ResolveSpecPrefixWithCache(phase.NewCache(), prefix)
}

// ResolveSpecPrefixWithCache is the cache-aware variant of ResolveSpecPrefix.
//
// R3 explicit-ingress early gate (ADR-0042, spec 120 AC-7): validates its
// RESULT before returning — both the hyphen pass-through below and the
// prefix-resolved value — so a hostile `--spec` value refuses cleanly at
// the CLI surface, before any composition (`SpecBranch`/`SpecWorktreePath`/
// etc.) ever sees it. Every live spec ID (including letter-suffixed forms
// like "008b-human-gates") passes byte-identically.
func ResolveSpecPrefixWithCache(c *phase.Cache, prefix string) (string, error) {
	// If it contains a hyphen, treat as full spec ID — pass through,
	// gated.
	if strings.Contains(prefix, "-") {
		if err := idvalidate.SpecID(prefix); err != nil {
			return "", fmt.Errorf("%q is not a valid spec ID: %w; run `mindspec spec list` and re-run with a listed ID", prefix, err)
		}
		return prefix, nil
	}

	// Must be numeric-only to be a prefix.
	for _, r := range prefix {
		if !unicode.IsDigit(r) {
			if err := idvalidate.SpecID(prefix); err != nil {
				return "", fmt.Errorf("%q is not a valid spec ID: %w; run `mindspec spec list` and re-run with a listed ID", prefix, err)
			}
			return prefix, nil
		}
	}

	// Pad to 3 digits for matching.
	padded := fmt.Sprintf("%03s", prefix)

	// Query active specs first.
	activeSpecs, err := phase.DiscoverActiveSpecsWithCache(c)
	if err == nil {
		var matches []string
		for _, as := range activeSpecs {
			if len(as.SpecID) >= 3 && as.SpecID[:3] == padded {
				matches = append(matches, as.SpecID)
			}
		}
		switch len(matches) {
		case 1:
			resolved := matches[0]
			if err := idvalidate.SpecID(resolved); err != nil {
				// Defense in depth: a matched active-spec SpecID should
				// already be well-formed (D1-checked), but no id operand
				// is trusted by provenance (round 9).
				return "", fmt.Errorf("resolved spec id %q for prefix %q is invalid: %w; run `mindspec spec list` and re-run with a listed ID", resolved, prefix, err)
			}
			return resolved, nil
		case 0:
			// Fall through to error.
		default:
			return "", fmt.Errorf("ambiguous spec prefix %q matches multiple specs: %s", prefix, strings.Join(matches, ", "))
		}
	}

	return "", fmt.Errorf("no spec found matching prefix %q", prefix)
}

// IsDraftSpecWithCache reports whether specID is an on-disk pre-epic
// draft (OQ-2) — exported so callers outside this package (internal/next's
// R6.7 scope-resolution refusal, spec 128 Bead 8) can draw the same
// draft/nonexistent distinction ResolveSpecPrefixProven and
// ResolveModeWithCache already use, without re-deriving draft detection a
// second time.
func IsDraftSpecWithCache(c *phase.Cache, root, specID string) (bool, error) {
	isDraft, _, err := draftSpecStatusWithCache(c, root, specID)
	return isDraft, err
}

// ResolveSpecPrefixProven is ResolveSpecPrefix's root-aware sibling used
// by `mindspec next`'s explicit --spec scope resolution (spec 128 R6.7):
// the full-ID (hyphenated) passthrough branch additionally PROVES the
// resolved spec ID names an actual spec — an epic-bearing one, or a
// legitimate on-disk pre-epic draft (spec create's committed spec
// directory + branch, OQ-2) — rather than accepting any syntactically
// valid hyphenated string unchecked. The numeric-prefix branch already
// proves existence via DiscoverActiveSpecsWithCache and is untouched.
//
// A genuinely nonexistent spec ID refuses HERE, before mindspec next ever
// issues a ready query, so the caller never reaches the (removed) silent
// global `bd ready` fallback (AC-15(i)). An existing draft passes through
// UNCHANGED — its own honest "no epic; not approved" refusal is produced
// downstream, by the epic-lookup step that actually needs an epic
// (AC-15(ii)); this function's own refusal is reserved for the
// genuinely-nonexistent case so the two messages never collide.
//
// ResolveSpecPrefix/ResolveSpecPrefixWithCache themselves are UNCHANGED —
// internal/complete's resolveSpecPrefixFn seam and cmd/mindspec/complete.go
// keep today's syntax-only passthrough; this is a new, narrower entry
// point for callers that both have a root AND need the existence
// guarantee.
func ResolveSpecPrefixProven(root, prefix string) (string, error) {
	c := phase.NewCache()
	resolved, err := ResolveSpecPrefixWithCache(c, prefix)
	if err != nil {
		return "", err
	}
	if !strings.Contains(prefix, "-") {
		return resolved, nil // numeric branch already proved existence above
	}
	if epicID, epicErr := phase.FindEpicBySpecIDWithCache(c, resolved); epicErr == nil && epicID != "" {
		return resolved, nil // has an epic
	}
	isDraft, _, draftErr := draftSpecStatusWithCache(c, root, resolved)
	if draftErr != nil {
		return "", draftErr
	}
	if isDraft {
		return resolved, nil // exists as a draft — the honest no-epic refusal is the caller's epic-lookup step
	}
	return "", fmt.Errorf("no spec found matching %q; run `mindspec spec list` and re-run with a listed ID", prefix)
}

// ResolveTarget determines which spec to operate on.
//
// Resolution order:
//  1. If specFlag is provided (from --spec), use it directly.
//  2. Derive from beads context (worktree path + beads query).
//  3. Query active specs; if exactly one, auto-select it.
//  4. If multiple active specs exist, return ErrAmbiguousTarget.
//  5. If nothing found, return an error.
//
// Constructs a fresh phase.Cache; hot-path callers should use ResolveTargetWithCache.
func ResolveTarget(root, specFlag string) (string, error) {
	return ResolveTargetWithCache(phase.NewCache(), root, specFlag)
}

// ResolveTargetWithCache is the cache-aware variant of ResolveTarget. It shares
// phase.Cache with subsequent ResolveModeWithCache / ResolveContextWithCache /
// instruct.BuildContextWithCache calls so a single warm `mindspec instruct` run
// makes at most 3 bd subprocess calls.
func ResolveTargetWithCache(c *phase.Cache, root, specFlag string) (string, error) {
	// Explicit target — resolve prefix if needed.
	if specFlag != "" {
		return ResolveSpecPrefixWithCache(c, specFlag)
	}

	// Worktree-derived context: if CWD is inside a spec/bead worktree,
	// resolve directly from the path — no need to query all active specs.
	// Skip this for main worktree (it would call DiscoverActiveSpecs
	// internally, which we do below via ActiveSpecs anyway).
	kind, _, _ := workspace.DetectWorktreeContext(root)
	if kind != workspace.WorktreeMain {
		ctx, err := phase.ResolveContextFromDirWithCache(c, root, root)
		if err == nil && ctx != nil && ctx.SpecID != "" {
			return ctx.SpecID, nil
		}
	}

	// Query active specs (main worktree or worktree context didn't resolve)
	active, err := ActiveSpecsWithCache(c, root)
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
