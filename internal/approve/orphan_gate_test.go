package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// --- Spec 115 Bead 2: the pre-terminal orphan/obligation refusal gate ---
//
// These tests drive runOrphanObligationGate purely through the package's
// own implXxxFn seams (never a real `bd`/git call) — saveAndRestore
// already defaults every Bead 2 seam to a benign no-op, so each test here
// overrides only the seam(s) its scenario needs.

// approvePanelFixture writes a registered panel (panel.json + verdict
// files) under root/review/<slug>, the SAME on-disk shape
// internal/complete's own panel fixtures use (panel.Scan globs
// review/*/panel.json). specID's spec dir resolves as the legacy
// docs/specs/<id> tree (writeSpecDir), so panelGateRoots's non-flat
// branch includes root itself among the scanned roots.
func approvePanelFixture(t *testing.T, root, slug, beadID string, expectedReviewers int, verdicts map[string]string) {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := panel.Panel{BeadID: &beadID, Spec: "115", Round: 1, ExpectedReviewers: expectedReviewers}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "panel.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, v := range verdicts {
		vd, _ := json.Marshal(map[string]string{"verdict": v})
		if err := os.WriteFile(filepath.Join(dir, name), vd, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// approveOKMock is the baseline MockExecutor for scenarios expected to
// reach FinalizeEpic.
func approveOKMock() *executor.MockExecutor {
	return &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}
}

// --- AC1a: the headline orphan-refusal falsifier ---

func TestApproveImpl_OrphanRefuses(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
		return []lifecycle.Orphan{{BeadID: "bead-x", BeadBranch: "bead/bead-x", SpecBranch: "spec/010-test"}}, nil
	}

	var closed []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			closed = append(closed, args[1])
		}
		return []byte("ok"), nil
	}
	phaseWrites := 0
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		phaseWrites++
		return nil
	}

	mock := approveOKMock()
	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected an orphan refusal")
	}
	msg := err.Error()
	for _, want := range []string{"bead-x", "bead/bead-x", "spec/010-test"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal must name %q: %v", want, msg)
		}
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("refusal must end with a recovery line: %v", msg)
	}
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	if got, want := lines[len(lines)-1], "recovery: mindspec complete bead-x"; got != want {
		t.Errorf("final recovery line = %q, want %q", got, want)
	}
	if phaseWrites != 0 {
		t.Errorf("phase metadata writes = %d, want 0 (no epic close, no phase write on refusal)", phaseWrites)
	}
	if len(closed) != 0 {
		t.Errorf("epic close calls = %v, want none", closed)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must not run on an orphan refusal: %d calls", len(calls))
	}
}

// --- AC1b: the three cleanly-signalled orphan-scan infra errors fail closed ---

func TestApproveImpl_OrphanInfraErrorFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"epic-lookup error", fmt.Errorf("finding epic for spec 010-test: bd show epic: transient failure")},
		{"bd-list error", fmt.Errorf("listing closed beads for epic epic-parent: bd list: transient failure")},
		{"ancestry error", fmt.Errorf("checking ancestry of bead/bead-x: transient git failure")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			writeSpecDir(t, tmp, "010-test")
			os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

			saveAndRestore(t)

			implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
				return nil, tc.err
			}

			var closed []string
			implRunBDCombinedFn = func(args ...string) ([]byte, error) {
				if len(args) >= 2 && args[0] == "close" {
					closed = append(closed, args[1])
				}
				return []byte("ok"), nil
			}
			phaseWrites := 0
			implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
				phaseWrites++
				return nil
			}

			mock := approveOKMock()
			_, err := ApproveImpl(tmp, "010-test", mock)
			if err == nil {
				t.Fatal("expected a fail-closed refusal on an orphan-scan infra error")
			}
			if !guard.HasFinalRecoveryLine(err.Error()) {
				t.Errorf("infra-error refusal must end with a recovery line: %v", err)
			}
			if phaseWrites != 0 {
				t.Errorf("phase metadata writes = %d, want 0", phaseWrites)
			}
			if len(closed) != 0 {
				t.Errorf("epic close calls = %v, want none", closed)
			}
			if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
				t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
			}
		})
	}
}

// --- AC2: exemptions, clean path, epic scope ---

func TestApproveImpl_OrphanExemptions(t *testing.T) {
	t.Run("ancestor branch does not refuse", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		saveAndRestore(t)

		// The orphan scan already exempts a merged-but-undeleted
		// ancestor branch (internal/lifecycle's own contract) — from
		// the gate's seam-level view that is simply "no orphans". The
		// worktree-enum leg sees the SAME branch and independently
		// confirms it as an ancestor.
		implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
			return nil, nil
		}
		implClosedEpicBeadIDsFn = func(specID string) ([]string, error) { return []string{"bead-1"}, nil }
		implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
			return []bead.WorktreeListEntry{{Branch: "bead/bead-1", Path: "/tmp/wt"}}, nil
		}
		implIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return true, nil }

		mock := approveOKMock()
		if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
			t.Fatalf("an ancestor (merged-but-undeleted) branch must not refuse: %v", err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
			t.Errorf("expected FinalizeEpic to run once, got %d", len(calls))
		}
	})

	// (b) Deliberately non-discriminating regression pin (AC2's own
	// wording): a spec with no orphans and no pending obligations
	// proceeds to FinalizeEpic exactly as TestApproveImpl_HappyPath /
	// TestApproveImpl_FinalizeEpicCalled — both unmodified in behavior.
	t.Run("clean path proceeds to FinalizeEpic", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		saveAndRestore(t)

		mock := approveOKMock()
		if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
			t.Fatalf("a clean spec must approve: %v", err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
			t.Errorf("expected FinalizeEpic to run once, got %d", len(calls))
		}
	})

	t.Run("a different spec's orphan does not trigger this spec's refusal", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		saveAndRestore(t)

		implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
			return nil, nil // epic-scoped: another spec's orphan never surfaces here.
		}
		// This spec's own closed-bead set is just bead-1; a DIFFERENT
		// spec's worktree entry must not be considered at all.
		implClosedEpicBeadIDsFn = func(specID string) ([]string, error) { return []string{"bead-1"}, nil }
		implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
			return []bead.WorktreeListEntry{
				{Branch: "bead/bead-other-spec", Path: "/tmp/wt-other"},
			}, nil
		}
		implIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
			t.Fatalf("ancestry must not be checked for a bead outside this spec's closed-epic-bead set: %s", ancestor)
			return false, nil
		}

		mock := approveOKMock()
		if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
			t.Fatalf("a different spec's worktree must not trigger this spec's refusal: %v", err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
			t.Errorf("expected FinalizeEpic to run once, got %d", len(calls))
		}
	})
}

// --- AC2d: a genuinely-deleted branch never false-refuses ---

func TestApproveImpl_DeletedBranchNoRefusal(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	saveAndRestore(t)

	// Genuinely absent (merged-and-cleaned, the normal post-`complete`
	// state): BranchExists -> false inside the orphan scan (no trigger,
	// no orphan), and no worktree exists for it either.
	implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
		return nil, nil
	}
	implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	mock := approveOKMock()
	if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
		t.Fatalf("a genuinely-deleted branch must not refuse: %v", err)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
		t.Errorf("expected FinalizeEpic to run once (clean-path parity), got %d", len(calls))
	}
}

// --- AC3: hatches bypass nothing ---

func TestApproveImpl_HatchDoesNotBypassOrphan(t *testing.T) {
	run := func(t *testing.T, setEnv, setConfigFalse bool) {
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		if setConfigFalse {
			if err := os.WriteFile(filepath.Join(tmp, ".mindspec", "config.yaml"),
				[]byte("enforcement:\n  panel_gate: false\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if setEnv {
			os.Setenv(panel.SkipPanelEnv, "1")
			t.Cleanup(func() { os.Unsetenv(panel.SkipPanelEnv) })
		}

		saveAndRestore(t)
		implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
			return []lifecycle.Orphan{{BeadID: "bead-x", BeadBranch: "bead/bead-x", SpecBranch: "spec/010-test"}}, nil
		}

		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected the orphan refusal to fire despite the hatch")
		}
		if strings.Contains(err.Error(), panel.SkipPanelEnv) {
			t.Errorf("refusal message must never print %s (HC-7): %v", panel.SkipPanelEnv, err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
	}

	t.Run("MINDSPEC_SKIP_PANEL=1", func(t *testing.T) { run(t, true, false) })
	t.Run("enforcement.panel_gate: false", func(t *testing.T) { run(t, false, true) })
}

// --- AC5: advisory slot naming, decoration only ---

func TestApproveImpl_AdvisorySlotNaming(t *testing.T) {
	const beadID = "bead-x"

	setupOrphan := func(t *testing.T) string {
		t.Helper()
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		saveAndRestore(t)
		implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
			return []lifecycle.Orphan{{BeadID: beadID, BeadBranch: "bead/" + beadID, SpecBranch: "spec/010-test"}}, nil
		}
		return tmp
	}
	refusalMessage := func(t *testing.T, tmp string) string {
		t.Helper()
		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected an orphan refusal")
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
		return err.Error()
	}

	t.Run("readable panel names the unresolved slot", func(t *testing.T) {
		tmp := setupOrphan(t)
		approvePanelFixture(t, tmp, "115-orphan-slot", beadID, 3, map[string]string{
			"codex-correctness-round-1.json": "REQUEST_CHANGES",
			"claude-a-round-1.json":          "APPROVE",
			"claude-b-round-1.json":          "APPROVE",
		})
		msg := refusalMessage(t, tmp)
		if !strings.Contains(msg, "codex-correctness") {
			t.Errorf("expected the unresolved slot named in the refusal: %v", msg)
		}
	})

	t.Run("panel directory removed: identical refusal minus the slot line", func(t *testing.T) {
		tmp := setupOrphan(t)
		// No panel written at all — unreadable/missing panel.
		msg := refusalMessage(t, tmp)
		if strings.Contains(msg, "codex-correctness") {
			t.Errorf("no panel is registered — must not name a slot: %v", msg)
		}
		if !strings.Contains(msg, beadID) {
			t.Errorf("base refusal must still fire identically: %v", msg)
		}
	})

	t.Run("no paste-able refutation incantation, no skip-var leak", func(t *testing.T) {
		tmp := setupOrphan(t)
		approvePanelFixture(t, tmp, "115-orphan-slot2", beadID, 3, map[string]string{
			"codex-correctness-round-1.json": "REQUEST_CHANGES",
			"claude-a-round-1.json":          "APPROVE",
			"claude-b-round-1.json":          "APPROVE",
		})
		msg := refusalMessage(t, tmp)
		if strings.Contains(strings.ToLower(msg), "refut") {
			t.Errorf("message must contain no paste-able refutation incantation: %v", msg)
		}
		if strings.Contains(msg, panel.SkipPanelEnv) {
			t.Errorf("message must never print %s: %v", panel.SkipPanelEnv, msg)
		}
	})
}

// --- AC6: the durable-obligation backstop ---

func TestApproveImpl_ObligationBackstop(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		saveAndRestore(t)
		return tmp
	}

	t.Run("branch-less uncovered obligation refuses with restoration-prerequisite recourse", func(t *testing.T) {
		tmp := setup(t)
		implCheckObligationsFn = func(beadID string, getMeta func(string) (map[string]interface{}, error)) error {
			return fmt.Errorf("bead %s carries an unresolved refutation_pending obligation (X@round 1) not yet covered by a durable panel_refuted record", beadID)
		}
		implBranchExistsFn = func(name string) bool { return false }

		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected an obligation refusal")
		}
		msg := err.Error()
		if !strings.Contains(msg, "bead-1") {
			t.Errorf("refusal must name the bead: %v", msg)
		}
		if !guard.HasFinalRecoveryLine(msg) {
			t.Errorf("refusal must end with a recovery line: %v", msg)
		}
		lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
		if got, want := lines[len(lines)-1], "recovery: mindspec complete bead-1"; got != want {
			t.Errorf("final recovery line = %q, want %q", got, want)
		}
		if !strings.Contains(lines[len(lines)-2], "restore the bead/bead-1 branch ref") {
			t.Errorf("branch-less recourse must name the restoration prerequisite before the bare command: %v", msg)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
	})

	t.Run("(slot, round)-covered proceeds", func(t *testing.T) {
		tmp := setup(t)
		implCheckObligationsFn = func(beadID string, getMeta func(string) (map[string]interface{}, error)) error {
			return nil
		}

		mock := approveOKMock()
		if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
			t.Fatalf("a covered obligation must not refuse: %v", err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
			t.Errorf("expected FinalizeEpic to run once, got %d", len(calls))
		}
	})

	t.Run("metadata read error fails closed with branch-exists recourse", func(t *testing.T) {
		tmp := setup(t)
		implCheckObligationsFn = func(beadID string, getMeta func(string) (map[string]interface{}, error)) error {
			return fmt.Errorf("bead %s metadata could not be read to verify its refutation obligations are satisfied", beadID)
		}
		implBranchExistsFn = func(name string) bool { return true }

		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected a fail-closed refusal on a metadata read error (never fail-open)")
		}
		msg := err.Error()
		lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
		if got, want := lines[len(lines)-1], "recovery: mindspec complete bead-1"; got != want {
			t.Errorf("branch-exists recourse must be the single bare command: got %q, want %q (msg: %v)", got, want, msg)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
	})

	t.Run("hatch does not bypass", func(t *testing.T) {
		tmp := setup(t)
		os.Setenv(panel.SkipPanelEnv, "1")
		t.Cleanup(func() { os.Unsetenv(panel.SkipPanelEnv) })
		implCheckObligationsFn = func(beadID string, getMeta func(string) (map[string]interface{}, error)) error {
			return fmt.Errorf("bead %s carries an unresolved refutation_pending obligation", beadID)
		}
		implBranchExistsFn = func(name string) bool { return true }

		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected the obligation refusal to fire despite the hatch")
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
	})
}

// --- AC6e: an unreadable plan-bead enumeration refuses, never silently skips ---

func TestApproveImpl_CorruptPlanRefuses(t *testing.T) {
	assertRefusal := func(t *testing.T, tmp string) {
		t.Helper()
		saveAndRestore(t)
		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected a refusal naming the unreadable plan path")
		}
		if !strings.Contains(err.Error(), "plan bead list could not be read") {
			t.Errorf("refusal must mention the unreadable plan-bead enumeration: %v", err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
	}

	t.Run("missing plan.md", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		assertRefusal(t, tmp)
	})

	// Valid YAML frontmatter with NO bead_ids key: internal/validate's OWN
	// plan-frontmatter parser (used by the ADR-divergence gate, which
	// runs BEFORE this leg) tolerates an absent/empty bead_ids list fine
	// (it only cares about adr_citations) — so this exercises SPECIFICALLY
	// readPlanBeadIDs's stricter "no bead_ids in plan frontmatter" error
	// reaching Leg 3, rather than an unrelated earlier gate's own parse
	// failure preempting it.
	t.Run("corrupt plan.md frontmatter (no bead_ids)", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		specDir := filepath.Join(tmp, "docs", "specs", "010-test")
		if err := os.WriteFile(filepath.Join(specDir, "plan.md"),
			[]byte("---\nstatus: Approved\nspec_id: \"010-test\"\n---\n\n# Plan\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		assertRefusal(t, tmp)
	})
}

// --- AC7b: a settled orphan converges ---

func TestApproveImpl_PassesAfterOrphanSettled(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	saveAndRestore(t)

	// Post-settle state (the bead's branch is now an ancestor of the
	// spec branch): the orphan scan no longer reports it, and the
	// worktree-enum leg's ancestry check agrees.
	implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
		return nil, nil
	}
	implClosedEpicBeadIDsFn = func(specID string) ([]string, error) { return []string{"bead-1"}, nil }
	implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Branch: "bead/bead-1", Path: "/tmp/wt"}}, nil
	}
	implIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return true, nil }

	mock := approveOKMock()
	if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
		t.Fatalf("a post-settle ancestor state must pass the gate: %v", err)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
		t.Errorf("expected FinalizeEpic to run once, got %d", len(calls))
	}
}

// --- AC11 (Fact 1): a whole-store failure aborts before the orphan scan ---

func TestApproveImpl_UnreadableRefStoreAbortsPreScan(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	saveAndRestore(t)

	scanCalled := false
	implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
		scanCalled = true
		return nil, nil
	}
	var closed []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			closed = append(closed, args[1])
		}
		return []byte("ok"), nil
	}
	phaseWrites := 0
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		phaseWrites++
		return nil
	}

	mock := &executor.MockExecutor{
		// exec.MergeBase("main", specBranch), currently at impl.go:294
		// (the spec's Fact-1 pin cited impl.go:249 against the pre-115
		// tree; Bead 2's insertions above it shifted the line), is the
		// FIRST ref-store touch — the equivalent of a real
		// `git merge-base` hitting an unreadable refs/heads (exit 128).
		MergeBaseErr: fmt.Errorf("exit status 128: fatal: unable to read refs/heads: Permission denied"),
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected a whole-store failure to abort ApproveImpl before the orphan scan")
	}
	if scanCalled {
		t.Error("the orphan scan must not run when the pre-scan MergeBase call fails (Fact 1)")
	}
	if phaseWrites != 0 {
		t.Errorf("phase metadata writes = %d, want 0", phaseWrites)
	}
	if len(closed) != 0 {
		t.Errorf("epic close calls = %v, want none", closed)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
	}
}

// --- AC13: the round-7 Option B worktree-enumeration merge-prevention leg ---

func TestApproveImpl_WorktreeEnumRefusesDespiteProbeMiss(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
		saveAndRestore(t)
		// The round-6 transient: the branch-existence probe already
		// missed it, so the orphan scan reports no orphans.
		implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
			return nil, nil
		}
		return tmp
	}

	t.Run("the race: probe miss + enumerated non-ancestor worktree refuses", func(t *testing.T) {
		tmp := setup(t)
		implClosedEpicBeadIDsFn = func(specID string) ([]string, error) { return []string{"bead-1"}, nil }
		implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
			return []bead.WorktreeListEntry{{Branch: "bead/bead-1", Path: "/tmp/wt1"}}, nil
		}
		implIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return false, nil }

		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected the worktree-enum leg to refuse despite the probe miss")
		}
		msg := err.Error()
		if !strings.Contains(msg, "bead-1") || !strings.Contains(msg, "bead/bead-1") {
			t.Errorf("refusal must name the bead and its branch: %v", msg)
		}
		lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
		if got, want := lines[len(lines)-1], "recovery: mindspec complete bead-1"; got != want {
			t.Errorf("final recovery line = %q, want %q", got, want)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
	})

	t.Run("worktree-list error fails closed", func(t *testing.T) {
		tmp := setup(t)
		implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
			return nil, fmt.Errorf("bd worktree list failed: transient")
		}

		mock := approveOKMock()
		_, err := ApproveImpl(tmp, "010-test", mock)
		if err == nil {
			t.Fatal("expected a fail-closed refusal on a worktree-list error")
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
			t.Errorf("FinalizeEpic must not run: %d calls", len(calls))
		}
	})

	t.Run("no false positives: ancestor branch and a different spec's bead do not trigger", func(t *testing.T) {
		tmp := setup(t)
		implClosedEpicBeadIDsFn = func(specID string) ([]string, error) { return []string{"bead-1"}, nil }
		implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
			return []bead.WorktreeListEntry{
				{Branch: "bead/bead-1", Path: "/tmp/wt1"},          // ancestor -> no trigger
				{Branch: "bead/bead-other-spec", Path: "/tmp/wt2"}, // not in the closed set -> no trigger
			}, nil
		}
		implIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
			if ancestor != "bead/bead-1" {
				t.Fatalf("ancestry must not be checked for a bead outside this spec's closed-epic-bead set: %s", ancestor)
			}
			return true, nil
		}

		mock := approveOKMock()
		if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
			t.Fatalf("an ancestor branch and a different spec's worktree must not trigger the leg: %v", err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
			t.Errorf("expected FinalizeEpic to run once, got %d", len(calls))
		}
	})

	// TestReverseGate: spec 120 AC-23 (round-4 reverse-derivation gate at
	// runWorktreeEnumerationLeg): a hostile bead/x;evil-shaped worktree
	// entry is skipped — never matched against the closed-epic-bead set,
	// never embedded as an ID in a refusal — even when the ancestry check
	// would report it unmerged (the worst case for a false trigger).
	t.Run("hostile worktree entry never triggers the enumeration leg", func(t *testing.T) {
		tmp := setup(t)
		implClosedEpicBeadIDsFn = func(specID string) ([]string, error) { return []string{"bead-1"}, nil }
		implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
			return []bead.WorktreeListEntry{
				{Branch: "bead/x;evil", Path: "/tmp/wt-hostile"},
			}, nil
		}
		implIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
			t.Fatalf("ancestry must never be checked for a malformed reverse-derived bead id: %s", ancestor)
			return false, nil
		}

		mock := approveOKMock()
		if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
			t.Fatalf("a hostile worktree entry must never trigger the enumeration leg: %v", err)
		}
		if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
			t.Errorf("expected FinalizeEpic to run once, got %d", len(calls))
		}
	})
}
