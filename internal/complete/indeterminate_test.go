package complete

// indeterminate_test.go — spec 128 (lifecycle-verb-state-honesty) bead 5,
// AC-4b: a failing post-close child read renders the indeterminate form
// in both the result summary (FormatResult) and the instruct tail —
// never a fabricated `plan` derived from a nil child set — and step-6.5
// skips its phase-sync write entirely rather than writing a value
// derived from the failed read.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// TestCompleteFetchChildrenFnDefaultsToPhaseFetchChildren is the
// reflect-Pointer identity pin (matching this file's established
// checkDirtyTreeFn/completeGetwdFn pattern): the AC-4b seam MUST default
// to phase.FetchChildren — a severed default would silently disable the
// seam every other test in this file swaps.
func TestCompleteFetchChildrenFnDefaultsToPhaseFetchChildren(t *testing.T) {
	if reflect.ValueOf(completeFetchChildrenFn).Pointer() != reflect.ValueOf(phase.FetchChildren).Pointer() {
		t.Fatal("completeFetchChildrenFn must default to phase.FetchChildren (spec 128 bead 5 AC-4b)")
	}
}

// TestAdvanceState_ChildReadFailure_PropagatesError is the direct unit
// leg: advanceState must return a non-nil error (never a silently
// discarded one degrading to a derivation over a nil child set) when the
// post-close children read fails.
func TestAdvanceState_ChildReadFailure_PropagatesError(t *testing.T) {
	saveAndRestore(t)

	wantErr := errors.New("bd list --parent: transient flake")
	completeFetchChildrenFn = func(epicID string) ([]phase.ChildInfo, error) {
		return nil, wantErr
	}

	mode, nextBead, err := advanceState("epic-123")
	if err == nil {
		t.Fatal("expected a non-nil error from advanceState on a failed child read")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("advanceState error does not wrap the underlying read failure: %v", err)
	}
	if mode != "" {
		t.Errorf("mode must be empty on error (never a fabricated derivation), got %q", mode)
	}
	if nextBead != "" {
		t.Errorf("nextBead must be empty on error, got %q", nextBead)
	}
}

// TestFormatResult_Indeterminate pins AC-4b's result-summary half: the
// indeterminate form names the real reason and never renders any of the
// mode-specific branches (no "Mode: plan", no fabricated next bead).
func TestFormatResult_Indeterminate(t *testing.T) {
	r := &Result{
		BeadID:                "bead-1",
		BeadClosed:            true,
		WorktreeRemoved:       true,
		NextModeIndeterminate: true,
		IndeterminateReason:   "reading post-close child set for epic epic-123: bd list --parent: transient flake",
	}
	out := FormatResult(r)
	if !strings.Contains(out, "cannot determine mode") {
		t.Errorf("expected the indeterminate marker text, got:\n%s", out)
	}
	if !strings.Contains(out, r.IndeterminateReason) {
		t.Errorf("expected the real reason in the output, got:\n%s", out)
	}
	if strings.Contains(out, "Mode: plan") || strings.Contains(out, "Mode: implement") || strings.Contains(out, "Mode: review") || strings.Contains(out, "Mode: idle") {
		t.Errorf("indeterminate result must never render a fabricated mode; got:\n%s", out)
	}
}

// TestFormatResult_PlanRewording pins R2.3's ModePlan rewording (round-4/
// round-5 tally): the message must fit BOTH surviving triggers (the
// empty-lifecycle-subset fallback and the all-open branch) and must
// never claim "blocked" — true for neither trigger.
func TestFormatResult_PlanRewording(t *testing.T) {
	r := &Result{
		BeadID:     "bead-1",
		BeadClosed: true,
		NextMode:   state.ModePlan,
		NextSpec:   "008-test",
	}
	out := FormatResult(r)
	if strings.Contains(out, "blocked") {
		t.Errorf("ModePlan message must not claim beads are blocked (true for neither surviving trigger); got:\n%s", out)
	}
	if !strings.Contains(out, "Mode: plan") {
		t.Errorf("expected the plan mode marker, got:\n%s", out)
	}
}

// TestRun_AC4b_PostCloseChildReadFailure_RendersIndeterminate drives the
// REAL result path (Run, not a hand-built Result) with
// completeFetchChildrenFn failing after a successful bead close/merge:
// the result renders indeterminate, NextMode/NextBead stay at their zero
// values (no fabricated `plan`), and step-6.5's phase-sync write never
// fires (no completeMergeMetadataFn call carrying mindspec_phase derived
// from the failed read).
func TestRun_AC4b_PostCloseChildReadFailure_RendersIndeterminate(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	specWtDir := filepath.Join(root, ".worktrees", "worktree-spec-008-test")
	if err := os.MkdirAll(specWtDir, 0755); err != nil {
		t.Fatalf("mkdir spec worktree dir: %v", err)
	}

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}

	var closedID string
	closeBeadFn = func(ids ...string) error {
		closedID = ids[0]
		return nil
	}

	// The AC-4b seam: the post-close children read fails. Nothing else
	// in the fixture needs to change — advanceState's own `bd ready`
	// call is unreachable once the child read itself errors.
	wantReadErr := errors.New("bd list --parent: dolt daemon unreachable")
	completeFetchChildrenFn = func(epicID string) ([]phase.ChildInfo, error) {
		return nil, wantReadErr
	}

	var mergeMetaCalls []map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		mergeMetaCalls = append(mergeMetaCalls, updates)
		return nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if closedID != "bead-1" {
		t.Fatalf("closed ID: got %q, want %q (the bead close/merge must still succeed before advanceState runs)", closedID, "bead-1")
	}
	if !result.NextModeIndeterminate {
		t.Fatal("expected Result.NextModeIndeterminate=true on a failed post-close child read")
	}
	if !strings.Contains(result.IndeterminateReason, wantReadErr.Error()) {
		t.Errorf("IndeterminateReason does not carry the real failure: got %q", result.IndeterminateReason)
	}
	if result.NextMode != "" {
		t.Errorf("NextMode must stay empty (no fabricated mode), got %q", result.NextMode)
	}
	if result.NextBead != "" {
		t.Errorf("NextBead must stay empty, got %q", result.NextBead)
	}

	// AC-4b's no-fabricated-write clause: no mindspec_phase sync write
	// carrying a value derived from the failed (nil) child set.
	for _, updates := range mergeMetaCalls {
		if v, ok := updates["mindspec_phase"]; ok {
			t.Errorf("step-6.5 must skip its phase-sync write entirely on indeterminate; got mindspec_phase=%v", v)
		}
	}

	out := FormatResult(result)
	if !strings.Contains(out, "cannot determine mode") {
		t.Errorf("FormatResult must render the indeterminate form; got:\n%s", out)
	}
	if strings.Contains(out, "plan") {
		t.Errorf("FormatResult must never fabricate `plan` from the failed read; got:\n%s", out)
	}
}
