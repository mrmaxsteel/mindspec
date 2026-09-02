// Package resolve provides spec targeting and active-spec discovery.
package resolve

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// SpecStatus holds the resolved state for a single spec.
type SpecStatus struct {
	SpecID string `json:"spec_id"`
	Mode   string `json:"mode"`
	Active bool   `json:"active"`
}

// ActiveSpecs discovers all specs with active lifecycles by querying beads for open epics.
// Returns specs sorted by spec ID for deterministic output. Constructs a fresh
// phase.Cache; hot-path callers should use ActiveSpecsWithCache.
func ActiveSpecs(root string) ([]SpecStatus, error) {
	return ActiveSpecsWithCache(phase.NewCache(), root)
}

// ActiveSpecsWithCache is the cache-aware variant of ActiveSpecs.
func ActiveSpecsWithCache(c *phase.Cache, root string) ([]SpecStatus, error) {
	activeSpecs, err := phase.DiscoverActiveSpecsWithCache(c)
	if err != nil {
		return nil, err
	}

	result := make([]SpecStatus, 0, len(activeSpecs))
	for _, as := range activeSpecs {
		if as.Phase == "" || as.Phase == "done" || as.Phase == state.ModeIdle {
			continue
		}
		result = append(result, SpecStatus{SpecID: as.SpecID, Mode: as.Phase, Active: true})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SpecID < result[j].SpecID
	})

	return result, nil
}

// ResolveMode returns the current lifecycle phase for a spec by querying beads.
// Returns ModeIdle if no epic exists for the spec and it is not an on-disk
// draft. Constructs a fresh phase.Cache; hot-path callers should use
// ResolveModeWithCache.
func ResolveMode(root, specID string) (string, error) {
	return ResolveModeWithCache(phase.NewCache(), root, specID)
}

// ResolveModeWithCache is the cache-aware variant of ResolveMode.
//
// OQ-2 (spec 128): a spec with no epic is not automatically idle — it may be
// a pre-epic DRAFT (spec directory + `spec/<id>` branch exist, no lifecycle
// epic created yet — the epic is created at `spec approve`, the durable
// approval record). ModeSpec is returned for a draft.
//
// Honesty (R1.4, the named hazard): a genuine bd/discovery FAILURE (bd
// unreachable, Dolt down) is never conflated with "bd succeeded, no epic
// found" — draftSpecStatusWithCache probes bd health directly and
// propagates a failure rather than falling through to idle.
func ResolveModeWithCache(c *phase.Cache, root, specID string) (string, error) {
	epicID, err := phase.FindEpicBySpecIDWithCache(c, specID)
	if err != nil || epicID == "" {
		isDraft, _, draftErr := draftSpecStatusWithCache(c, root, specID)
		if draftErr != nil {
			return "", draftErr
		}
		if isDraft {
			return state.ModeSpec, nil
		}
		return state.ModeIdle, nil
	}
	derivedPhase, err := phase.DerivePhaseWithCache(c, epicID)
	if err != nil {
		return "", err
	}
	return derivedPhase, nil
}

// DraftSpec identifies an on-disk pre-epic spec draft (OQ-2): a spec
// directory and its `spec/<id>` branch both exist, but beads has no
// lifecycle epic for it yet.
type DraftSpec struct {
	SpecID     string
	SpecBranch string
}

// DiscoverDraftSpecs scans the specs directory for pre-epic drafts.
// Constructs a fresh phase.Cache; hot-path callers should use
// DiscoverDraftSpecsWithCache.
func DiscoverDraftSpecs(root string) ([]DraftSpec, error) {
	return DiscoverDraftSpecsWithCache(phase.NewCache(), root)
}

// DiscoverDraftSpecsWithCache is the cache-aware variant of
// DiscoverDraftSpecs. It enumerates every spec directory under
// workspace.SpecsDir and keeps only those that pass draftSpecStatusWithCache
// (spec dir + branch on disk, no beads epic). A genuine bd/discovery
// failure encountered while probing any candidate propagates immediately
// rather than being read as "not a draft" — never fabricate a result from a
// failed read.
func DiscoverDraftSpecsWithCache(c *phase.Cache, root string) ([]DraftSpec, error) {
	specsDir := workspace.SpecsDir(root)
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning specs directory %s: %w", specsDir, err)
	}

	var drafts []DraftSpec
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specID := e.Name()
		// Enumerated from an agent-creatable directory name (ADR-0042 §1
		// gate-all-ids) — validate-and-skip rather than composing a
		// branch/path from an unvalidated entry, matching spec.List's
		// convention (internal/spec/list.go).
		if idvalidate.SpecID(specID) != nil {
			continue
		}
		isDraft, branch, draftErr := draftSpecStatusWithCache(c, root, specID)
		if draftErr != nil {
			return nil, draftErr
		}
		if isDraft {
			drafts = append(drafts, DraftSpec{SpecID: specID, SpecBranch: branch})
		}
	}

	sort.Slice(drafts, func(i, j int) bool {
		return drafts[i].SpecID < drafts[j].SpecID
	})
	return drafts, nil
}

// draftSpecStatusWithCache reports whether specID is a pre-epic draft
// (OQ-2): its spec directory and `spec/<id>` branch both exist, and beads
// has been confirmed (via a direct AllEpics health probe, memoized and
// therefore free when the caller already queried) to have no epic for it.
//
// A genuine bd/discovery failure propagates as an error rather than being
// read as "not a draft" — the same distinction ResolveModeWithCache draws:
// bd succeeding with zero matches is honestly different from bd failing to
// answer at all.
func draftSpecStatusWithCache(c *phase.Cache, root, specID string) (isDraft bool, branch string, err error) {
	if _, epicsErr := c.AllEpics(); epicsErr != nil {
		return false, "", fmt.Errorf("querying epics for spec %s: %w", idrender.Spec(specID), epicsErr)
	}

	dir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return false, "", err
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		if os.IsNotExist(statErr) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("checking spec directory %s: %w", dir, statErr)
	}

	branch, err = workspace.SpecBranch(specID)
	if err != nil {
		return false, "", err
	}
	branchExists, err := gitutil.BranchExistsIn(root, branch)
	if err != nil {
		return false, "", fmt.Errorf("checking branch %s existence: %w", branch, err)
	}
	if !branchExists {
		return false, "", nil
	}

	epicID, epicErr := phase.FindEpicBySpecIDWithCache(c, specID)
	if epicErr == nil && epicID != "" {
		return false, "", nil // has an epic — not a draft
	}
	return true, branch, nil
}

// ResolveSpecBranch and ResolveWorktree were thin shims around the
// state.* path helpers. Both helpers moved to internal/workspace and
// had no remaining callers in the tree, so the shims were removed
// (ARCH-7 / mindspec-c8q0). Use workspace.SpecBranch and
// workspace.SpecWorktreePath directly.

// FormatActiveList produces a human-readable list of active specs.
func FormatActiveList(specs []SpecStatus) string {
	if len(specs) == 0 {
		return "No active specs found.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Active specs (%d):\n", len(specs)))
	for _, s := range specs {
		sb.WriteString(fmt.Sprintf("  %s  phase=%s\n", idrender.Spec(s.SpecID), s.Mode))
	}
	return sb.String()
}
