package executor

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Spec 106 Bead 4 (Req 9): the DIRECTIONAL merge-time layout-fingerprint
// HARD-FAIL, installed in front of the REAL local merge seams in this package
// (CompleteBead's and FinalizeEpic's gitutil.MergeInto for bead→spec, and
// FinalizeEpic's direct gitutil.MergeBranch for the no-remote spec→main path).
//
// Honest coverage scope: the REMOTE-PR path (FinalizeEpic pushes the spec
// branch for a PR when a remote exists — see mindspec_executor.go) does NOT
// local-merge, so this in-binary guard never runs there. Cross-layout
// protection on the PR path relies on the Bead-3 branches-not-worktrees
// pre-flatten precondition + PR review, NOT on this guard. The guard therefore
// covers the LOCAL-merge seams only, and this is stated rather than overclaimed.

// layoutAtRef fingerprints the docs layout of ref's tree, read THROUGH the
// executor's own git seam (TreeDirsAtRef / BlobExistsAtRef) and classified by
// the shared Bead-1 workspace signature helper (ClassifyLayout) — one source
// of truth, so the merge guard and the on-disk DetectLayout can never drift.
//
// Bead 2 (spec 118): flat, canonical, and legacy are each derived
// INDEPENDENTLY by descending the tier's own wrapper — NO `!Flat &&
// !Canonical` gating — so every marker is observed regardless of the others
// (AC-9, AC-11). This supersedes the pre-Bead-2 shortcut that treated the
// bare .mindspec child name "docs" as a canonical marker
// (workspace.LayoutMarkersFromMindspecChildren, now flat-only — see its
// doc comment) and treated bare `docs` wrapper existence as a legacy marker;
// neither a bare wrapper nor a bare `docs` child name marks a tier anymore.
func (g *MindspecExecutor) layoutAtRef(ref string) (workspace.Layout, error) {
	flat, err := g.tierMarkerAtRef(ref, ".mindspec", ".mindspec/context-map.md")
	if err != nil {
		return "", err
	}
	canonical, err := g.tierMarkerAtRef(ref, ".mindspec/docs", ".mindspec/docs/context-map.md")
	if err != nil {
		return "", err
	}
	legacy, err := g.tierMarkerAtRef(ref, "docs", "docs/context-map.md")
	if err != nil {
		return "", err
	}
	return workspace.ClassifyLayout(workspace.LayoutMarkers{
		Flat:      flat,
		Canonical: canonical,
		Legacy:    legacy,
	}), nil
}

// tierMarkerAtRef reports whether a single docs tier is marked as populated
// at ref: an immediate child of wrapperPath matching the shared
// workspace.IsLifecycleChildName predicate (specs/adr/domains/core) — which
// TreeDirsAtRef already restricts to TREE entries, so a same-named regular
// file never counts — OR contextMapPath existing as a BLOB at ref (checked
// with the type-aware BlobExistsAtRef, never FileAtRef/`git show`, which also
// succeeds for a tree). It mirrors the filesystem tier probes
// (workspace.flatTreePresent / canonicalTreePresent / legacyTreePresent) but
// reads through git instead of the filesystem, so an absent or bare wrapper
// (empty TreeDirsAtRef result, no context-map blob) reports false with no
// error (Bead 2 / AC-9, AC-11, AC-16, AC-17, AC-18, AC-23).
func (g *MindspecExecutor) tierMarkerAtRef(ref, wrapperPath, contextMapPath string) (bool, error) {
	children, err := g.TreeDirsAtRef(ref, wrapperPath)
	if err != nil {
		return false, err
	}
	for _, name := range children {
		if workspace.IsLifecycleChildName(name) {
			return true, nil
		}
	}
	return g.BlobExistsAtRef(ref, contextMapPath)
}

// mergeLayoutRegression reports whether merging a sourceLayout tree onto a
// targetLayout tree is the REGRESSION direction the directional guard blocks
// (Req 9; extended by Bead 2 / spec 118 AC-10, AC-12, AC-22): a
// canonical/legacy/MIXED source onto a FLAT target — the merge that would
// resurrect the pre-flatten .mindspec/docs/... (or root docs/...) paths on
// top of an already-flattened tree. A mixed source already coexists with a
// flat lifecycle tree of its own (ClassifyLayout only reports Mixed when a
// flat marker is present alongside a canonical/legacy one), so it carries
// the exact same regression risk as a pure canonical/legacy source and is
// blocked identically. EVERY other combination is allowed: same-layout
// (flat→flat, canonical→canonical), the flat→canonical/legacy MIGRATION
// direction (the flatten landing itself — so Bead 5's own move-merge and the
// eventual flat-spec→canonical-main merge are NOT blocked), and a greenfield
// source. The rule is precise: block ⟺ source is canonical/legacy/mixed AND
// target is flat.
func mergeLayoutRegression(sourceLayout, targetLayout workspace.Layout) bool {
	if targetLayout != workspace.LayoutFlat {
		return false
	}
	switch sourceLayout {
	case workspace.LayoutCanonical, workspace.LayoutLegacy, workspace.LayoutMixed:
		return true
	default:
		return false
	}
}

// guardMergeLayout HARD-FAILS the REGRESSION merge direction (canonical/legacy
// source → flat target) with a forward-only rebase recovery line (ADR-0023),
// BEFORE the merge runs, mutating nothing. It is DIRECTIONAL — the
// flat→canonical migration direction and same-layout merges pass. It is EXEMPT
// inside a recorded IN-PROGRESS migration run (recoveryActive, computed at the
// call site via workspace.MigrationRecoveryActive — Bead-1's in-flight-run-id
// scoping, NOT a stale/completed run record), where a transient cross-layout
// merge is part of the recovery path. A layout read failure at either ref is
// non-fatal (fail-open: the guard never blocks a legitimate merge on a
// transient git error), mirroring the panel gate's deliberate fail-open posture.
//
// layoutAt is injected (g.layoutAtRef in production) so the directional logic
// is unit-testable without a real two-layout repo.
func guardMergeLayout(sourceRef, targetRef string, layoutAt func(string) (workspace.Layout, error), recoveryActive bool) error {
	if recoveryActive {
		return nil
	}
	sourceLayout, err := layoutAt(sourceRef)
	if err != nil {
		return nil
	}
	targetLayout, err := layoutAt(targetRef)
	if err != nil {
		return nil
	}
	if !mergeLayoutRegression(sourceLayout, targetLayout) {
		return nil
	}
	return mergeLayoutRegressionFailure(sourceRef, targetRef, sourceLayout, targetLayout)
}

// mergeLayoutRegressionFailure is the guard.NewFailure for a blocked layout
// regression: it names the offending refs and their layouts, states the
// forward-only rationale, and ends with a `rebase onto the post-flatten target`
// recovery command (the ADR-0035 final-recovery-line contract).
func mergeLayoutRegressionFailure(sourceRef, targetRef string, sourceLayout, targetLayout workspace.Layout) error {
	msg := fmt.Sprintf(
		"layout regression: refusing to merge %s (%s layout) into %s (%s layout) — "+
			"this would resurrect the pre-flatten .mindspec/docs/... paths on top of the already-flattened tree. "+
			"The layout flatten is forward-only (ADR-0023): rebase the source onto the post-flatten target instead of merging it back.",
		sourceRef, sourceLayout, targetRef, targetLayout)
	return guard.NewFailure(msg,
		fmt.Sprintf("git rebase %s %s", targetRef, sourceRef),
	)
}
