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
//
// Round-1 panel G2/F2a: phase.Cache does not memoize errors, so a first
// FindEpicBySpecIDWithCache failure may be a transient bd flake rather
// than a genuine "no epic" result. Before concluding anything about
// drafts or idle, this probes bd health directly and — if bd is in fact
// healthy — RE-RUNS the epic lookup against the now-warm cache. Without
// this, a recovered epic lookup's result was silently discarded by the
// draft/idle fallback below (empirically: a bd stub failing exactly the
// first call, with a real in_progress epic, fabricated idle instead of
// the epic's real derived phase).
func ResolveModeWithCache(c *phase.Cache, root, specID string) (string, error) {
	epicID, err := phase.FindEpicBySpecIDWithCache(c, specID)
	if err != nil || epicID == "" {
		if _, epicsErr := c.AllEpics(); epicsErr != nil {
			return "", fmt.Errorf("querying epics for spec %s: %w", idrender.Spec(specID), epicsErr)
		}
		epicID, err = phase.FindEpicBySpecIDWithCache(c, specID)
	}

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
// DiscoverDraftSpecs. It builds its candidate specID set from the UNION of
// (a) every spec directory under workspace.SpecsDir and (b) every local
// `spec/<id>` branch (round-1 panel G3, MAJOR): `spec create` commits the
// spec directory only inside the spec workspace it creates, never to the
// invoking checkout's own working tree, so an on-disk-only scan can never
// enumerate a genuinely fresh draft from a different checkout (e.g. main)
// — the branch is the one signal both checkouts share. Each candidate is
// then filtered through draftSpecStatusWithCache (dir-or-tree content +
// branch, no beads epic). A genuine bd/discovery/git failure encountered
// while probing any candidate propagates immediately rather than being
// read as "not a draft" — never fabricate a result from a failed read.
func DiscoverDraftSpecsWithCache(c *phase.Cache, root string) ([]DraftSpec, error) {
	candidates := map[string]bool{}

	specsDir := workspace.SpecsDir(root)
	entries, err := os.ReadDir(specsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("scanning specs directory %s: %w", specsDir, err)
	}
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
		candidates[specID] = true
	}

	branches, err := gitutil.LocalBranchRefs(root)
	if err != nil {
		return nil, fmt.Errorf("listing local branches: %w", err)
	}
	for _, b := range branches {
		specID, ok := strings.CutPrefix(b, workspace.SpecBranchPrefix)
		if !ok || idvalidate.SpecID(specID) != nil {
			continue
		}
		candidates[specID] = true
	}

	specIDs := make([]string, 0, len(candidates))
	for specID := range candidates {
		specIDs = append(specIDs, specID)
	}
	sort.Strings(specIDs)

	var drafts []DraftSpec
	for _, specID := range specIDs {
		isDraft, branch, draftErr := draftSpecStatusWithCache(c, root, specID)
		if draftErr != nil {
			return nil, draftErr
		}
		if isDraft {
			drafts = append(drafts, DraftSpec{SpecID: specID, SpecBranch: branch})
		}
	}
	return drafts, nil
}

// specTreeCandidatePaths returns the git-tree-relative candidate paths for
// specID's spec.md, one per SpecDir layout tier (flat, canonical, legacy —
// mirroring workspace.SpecDir's own precedence list minus its worktree-
// nested tiers, which do not apply when probing a branch's own root tree).
// Forward-slash joined regardless of OS: these are git object-path
// arguments, not filesystem paths.
func specTreeCandidatePaths(specID string) []string {
	return []string{
		".mindspec/specs/" + specID + "/spec.md",
		".mindspec/docs/specs/" + specID + "/spec.md",
		"docs/specs/" + specID + "/spec.md",
	}
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
//
// Ordering (round-1 panel G6): the epic check runs FIRST — memoized/free
// after the AllEpics health probe above, it short-circuits the common case
// (a shipped spec already has an epic) before any git subprocess runs at
// all, rather than after paying for a branch check + dir stat.
//
// Reach (round-1 panel G3, MAJOR): `spec create` writes the spec directory
// ONLY into the spec workspace (internal/spec/create.go) and commits it
// there — never to the invoking (e.g. main) checkout's own working tree —
// so a plain os.Stat probe of THIS checkout can never see a genuinely
// fresh draft from main. Once the branch is confirmed to exist, this falls
// back to a git-tree-scoped check of the BRANCH'S OWN committed tree
// (refs/objects are shared across worktrees of one repository, so the
// draft is visible the instant spec create's auto-commit lands, with no
// working-tree copy needed) before the plain on-disk check, which stays
// first since it is free relative to a git subprocess and already covers
// the from-inside-the-spec-worktree case (AC-1(i)).
func draftSpecStatusWithCache(c *phase.Cache, root, specID string) (isDraft bool, branch string, err error) {
	if _, epicsErr := c.AllEpics(); epicsErr != nil {
		return false, "", fmt.Errorf("querying epics for spec %s: %w", idrender.Spec(specID), epicsErr)
	}

	if epicID, epicErr := phase.FindEpicBySpecIDWithCache(c, specID); epicErr == nil && epicID != "" {
		return false, "", nil // has an epic — not a draft
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

	dir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return false, "", err
	}
	if _, statErr := os.Stat(dir); statErr == nil {
		return true, branch, nil
	} else if !os.IsNotExist(statErr) {
		return false, "", fmt.Errorf("checking spec directory %s: %w", dir, statErr)
	}

	// Round-2 panel G1/ADV-1: probe the FULLY QUALIFIED ref
	// ("refs/heads/<branch>"), never the short branch name — git's
	// ambiguous-short-name resolution prefers refs/tags, so a same-named
	// tag (reachable in any clone via a normal fetch) would silently
	// shadow the branch BranchExistsIn just confirmed, and cat-file would
	// resolve against the TAG's tree instead. "refs/heads/<branch>" is
	// exactly the ref BranchExistsIn verified above, so this closes the
	// gap between what was confirmed and what gets probed.
	qualifiedRef := "refs/heads/" + branch
	for _, rel := range specTreeCandidatePaths(specID) {
		exists, treeErr := gitutil.FileExistsAtRef(root, qualifiedRef, rel)
		if treeErr != nil {
			return false, "", fmt.Errorf("checking %s at %s: %w", rel, qualifiedRef, treeErr)
		}
		if exists {
			return true, branch, nil
		}
	}
	return false, "", nil
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
