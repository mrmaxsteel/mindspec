package complete

// Spec 128 Bead 6, round-1 panel fix round: dedicated MockExecutor-level
// tests for resolveMaterializationDiscriminator's G2 (closed stage-value
// set) and G3 (error-propagation) fixes, plus one Run()-level test for
// G3's third discard site (the S-stage's pre-publish marker read).
// These complement the real-git fixture legs in
// bead6_fixture_legs_test.go, which exercise the same machinery
// end-to-end but cannot as precisely target a single injected error at
// a single call site.

import (
	"fmt"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// --- G2: the Mindspec-Stage discriminator is a CLOSED set ---------------

// TestResolveMaterializationDiscriminator_UnknownStageClearsAndProceeds:
// a present marker whose trailer names a value OUTSIDE
// {materialization, artifact-sync} must be treated identically to a
// foreign/no-trailer marker — cleared (CAS against the observed OID)
// and resumeStage=="" (a normal run, re-running the panel and content
// gates), never resumed as if it were a recognized stage.
func TestResolveMaterializationDiscriminator_UnknownStageClearsAndProceeds(t *testing.T) {
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-fr1.1", "bead/mindspec-fr1.1", "/tmp/root"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return "marker-oid", "some-unrecognized-value", true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		if ref == beadBranch {
			return "tip-not-marker", nil
		}
		return "", fmt.Errorf("unexpected RevParseRef(%s)", ref)
	}
	var clearedWith string
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		clearedWith = expectedOID
		return nil
	}

	stage, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if stage != "" {
		t.Errorf("expected resumeStage==\"\" (normal run) for an unknown stage value, got %q", stage)
	}
	if clearedWith != "marker-oid" {
		t.Errorf("expected ClearStageMarker to CAS against the observed marker OID, got %q", clearedWith)
	}
}

// TestResolveMaterializationDiscriminator_EmptyStageClearsAndProceeds:
// stage=="" with present==true (gitutil's own encoding for "no trailer
// at all" OR "an ambiguous duplicate-trailer commit", per
// ReadStageTrailer) takes the identical foreign-clear path.
func TestResolveMaterializationDiscriminator_EmptyStageClearsAndProceeds(t *testing.T) {
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-fr1.2", "bead/mindspec-fr1.2", "/tmp/root"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return "marker-oid-2", "", true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		return "tip-not-marker", nil
	}
	var cleared bool
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		cleared = true
		if expectedOID != "marker-oid-2" {
			t.Errorf("expected CAS against marker-oid-2, got %q", expectedOID)
		}
		return nil
	}

	stage, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if stage != "" {
		t.Errorf("expected resumeStage==\"\", got %q", stage)
	}
	if !cleared {
		t.Error("expected the empty-stage marker to be cleared")
	}
}

// --- G3: error-discards in resolveMaterializationDiscriminator ----------

// TestResolveMaterializationDiscriminator_BeadTipReadErrorPropagates:
// a RevParseRef failure resolving the bead branch's current tip must
// propagate as a real error — never be silently treated as "clear the
// marker and proceed as a normal run", which would destroy write-ahead
// recovery evidence on a transient read failure.
func TestResolveMaterializationDiscriminator_BeadTipReadErrorPropagates(t *testing.T) {
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-fr1.3", "bead/mindspec-fr1.3", "/tmp/root"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return "marker-oid-3", stageMaterialization, true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		return "", fmt.Errorf("transient: cannot resolve %s", ref)
	}
	var cleared bool
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		cleared = true
		return nil
	}

	_, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err == nil {
		t.Fatal("expected the bead-tip RevParseRef failure to propagate as an error")
	}
	if cleared {
		t.Error("a transient tip-read failure must NEVER clear the marker (would destroy recovery evidence)")
	}
}

// TestResolveMaterializationDiscriminator_ParentResolutionErrorPropagates:
// once the marker is confirmed present with a known stage and the tip
// doesn't equal the marker OID itself, resolving the marker's own first
// parent (oid^) — to test the "unfinished advance" leg — must propagate
// a failure there too, rather than falling through to FOREIGN handling
// and deleting the marker.
func TestResolveMaterializationDiscriminator_ParentResolutionErrorPropagates(t *testing.T) {
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-fr1.4", "bead/mindspec-fr1.4", "/tmp/root"
	const markerOID = "marker-oid-4"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return markerOID, stageArtifactSync, true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		switch ref {
		case beadBranch:
			// tip != markerOID, so the discriminator proceeds to resolve
			// the marker's own parent next.
			return "some-other-tip", nil
		case markerOID + "^":
			return "", fmt.Errorf("transient: cannot resolve %s^", markerOID)
		}
		return "", fmt.Errorf("unexpected RevParseRef(%s)", ref)
	}
	var cleared bool
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		cleared = true
		return nil
	}

	_, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err == nil {
		t.Fatal("expected the marker-parent RevParseRef failure to propagate as an error")
	}
	if cleared {
		t.Error("a transient parent-read failure must NEVER be reclassified as FOREIGN (would destroy recovery evidence)")
	}
}

// --- G3 item 1: the S-stage's own pre-publish marker read -----------

// TestRun_SStageMarkerReadErrorAbortsBeforePublish: a ReadStageMarker
// failure in the S-stage build block (checking expectedOld before
// publishing D) must abort the run — never silently degrade to CREATE
// semantics (expectedOld==""), which could race a marker the M-stage
// itself just published.
func TestRun_SStageMarkerReadErrorAbortsBeforePublish(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	mock.MaterializeStageCommitFn = func(wtPath, tree, parent, stage, subject string) (string, error) {
		return "c-sha", nil
	}
	mock.SnapshotWorktreeTreeFn = func(wtPath string, exclude []string) (string, error) {
		if len(exclude) > 0 {
			return "tree-without-artifact", nil
		}
		return "tree-with-artifact", nil
	}
	mock.ChangedFilesTreeFn = func(workdir, base, treeOID string) ([]string, error) {
		if treeOID == "tree-with-artifact" {
			return []string{".beads/issues.jsonl"}, nil
		}
		return nil, nil
	}
	readStageCalls := 0
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		readStageCalls++
		if readStageCalls == 1 {
			// The discriminator's own pre-panel check: no marker exists yet.
			return "", "", false, nil
		}
		// The S-stage's pre-publish check: a transient read failure.
		return "", "", false, fmt.Errorf("disk read exploded")
	}
	var published bool
	mock.PublishStageMarkerFn = func(beadID, oid, expectedOld string) error {
		published = true
		return nil
	}

	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		return []string{".beads/issues.jsonl"}, nil, nil
	}

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	var closed bool
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the S-stage marker-read failure to abort Run")
	}
	if published {
		t.Error("PublishStageMarker must never be called after a marker-read failure (no silent CREATE-semantics fallback)")
	}
	if closed {
		t.Error("expected no bead close after the S-stage marker-read failure")
	}
}
