package complete

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// saveAndRestore saves all function variables and returns a restore function.
func saveAndRestore(t *testing.T) {
	t.Helper()
	origClose := closeBeadFn
	origWtList := worktreeListFn
	origRunBD := runBDFn
	origListJSON := listJSONFn
	origResolveTarget := resolveTargetFn
	origFindLocalRoot := findLocalRootFn
	origFetchBeadByID := fetchBeadByIDFn

	t.Cleanup(func() {
		closeBeadFn = origClose
		worktreeListFn = origWtList
		runBDFn = origRunBD
		listJSONFn = origListJSON
		resolveTargetFn = origResolveTarget
		findLocalRootFn = origFindLocalRoot
		fetchBeadByIDFn = origFetchBeadByID
	})

	// Default stubs
	resolveTargetFn = func(root, flag string) (string, error) { return "", fmt.Errorf("no active specs") }
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) { return next.BeadInfo{}, fmt.Errorf("not found") }
	listJSONFn = func(args ...string) ([]byte, error) { return []byte("[]"), nil }
}

// newMockExec creates a MockExecutor with defaults suitable for complete tests.
func newMockExec() *executor.MockExecutor {
	return &executor.MockExecutor{}
}

// setupTempRoot creates a temp dir with .mindspec/.
func setupTempRoot(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	return tmp
}

// stubPhaseEpic stubs phase functions so FindEpicBySpecID returns epicID
// and DerivePhase returns "implement" (at least one in_progress child).
func stubPhaseEpic(t *testing.T, specID, epicID string) {
	stubPhaseEpicInMode(t, specID, epicID, state.ModeImplement)
}

// stubPhaseEpicInMode stubs phase functions for a specific lifecycle mode.
func stubPhaseEpicInMode(t *testing.T, specID, epicID, mode string) {
	t.Helper()
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		// Epic type queries (FindEpicBySpecID, DiscoverActiveSpecs)
		for _, a := range args {
			if a == "--type=epic" {
				epics := []phase.EpicInfo{{
					ID: epicID, Title: "[SPEC " + specID + "] Test", Status: "open",
					IssueType: "epic", Metadata: map[string]interface{}{},
				}}
				var num int
				var title string
				if idx := strings.Index(specID, "-"); idx > 0 {
					fmt.Sscanf(specID[:idx], "%d", &num)
					title = specID[idx+1:]
				}
				if num > 0 && title != "" {
					epics[0].Metadata["spec_num"] = float64(num)
					epics[0].Metadata["spec_title"] = title
				}
				return json.Marshal(epics)
			}
		}

		// Parent queries for DerivePhase child enumeration
		isParentQuery := false
		var status string
		for _, a := range args {
			if a == "--parent" {
				isParentQuery = true
			}
			if strings.HasPrefix(a, "--status=") {
				status = strings.TrimPrefix(a, "--status=")
			}
		}

		if isParentQuery {
			switch mode {
			case state.ModeImplement:
				if status == "in_progress" {
					return json.Marshal([]phase.ChildInfo{{
						ID: "stub-bead", Title: "[" + specID + "] stub", Status: "in_progress",
					}})
				}
			case state.ModePlan:
				if status == "open" {
					return json.Marshal([]phase.ChildInfo{{
						ID: "stub-bead", Title: "[" + specID + "] stub", Status: "open",
					}})
				}
			case state.ModeReview:
				if status == "closed" {
					return json.Marshal([]phase.ChildInfo{{
						ID: "stub-bead", Title: "[" + specID + "] stub", Status: "closed",
					}})
				}
			}
		}

		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)

	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		// queryEpicStatus: bd show <id> --json
		if len(args) >= 1 && args[0] == "show" {
			return json.Marshal([]phase.EpicInfo{{ID: epicID, Status: "open"}})
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
}

func TestRun_HappyPath(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	// Create spec worktree dir so executor's CompleteBead merge path is found
	specWtDir := filepath.Join(root, ".worktrees", "worktree-spec-008-test")
	os.MkdirAll(specWtDir, 0755)

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

	// Next ready bead exists; children mix (closed + open) → implement phase.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed": {{ID: "bead-1", Title: "[IMPL 008-test.1] Done"}},
		"open":   {{ID: "bead-2", Title: "[IMPL 008-test.2] Next chunk"}},
	})
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []bead.BeadInfo{
				{ID: "bead-2", Title: "[IMPL 008-test.2] Next chunk"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}

	result, err := Run(root, "bead-1", "", "", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if closedID != "bead-1" {
		t.Errorf("closed ID: got %q, want %q", closedID, "bead-1")
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if !result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved=true")
	}
	if result.NextMode != state.ModeImplement {
		t.Errorf("NextMode: got %q, want %q", result.NextMode, state.ModeImplement)
	}
	if result.NextBead != "bead-2" {
		t.Errorf("NextBead: got %q, want %q", result.NextBead, "bead-2")
	}

	// Verify executor was called with CompleteBead
	completeCalls := mock.CallsTo("CompleteBead")
	if len(completeCalls) != 1 {
		t.Fatalf("expected 1 CompleteBead call, got %d", len(completeCalls))
	}
	if completeCalls[0].Args[0] != "bead-1" {
		t.Errorf("CompleteBead beadID: got %q, want %q", completeCalls[0].Args[0], "bead-1")
	}

	// ADR-0023: no focus file written — state derived from beads.
	focusPath := filepath.Join(root, ".mindspec", "focus")
	if _, statErr := os.Stat(focusPath); statErr == nil {
		t.Error("expected no focus file to be written")
	}
}

func TestRun_DirtyTreeRefuses(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	mock.IsTreeCleanErr = fmt.Errorf("workspace has uncommitted changes:\n M modified-file.go")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}

	_, err := Run(root, "bead-1", "", "", mock)
	if err == nil {
		t.Fatal("expected error for dirty worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error should mention uncommitted changes: %v", err)
	}
}

func TestRun_DirtyTreeWithoutWorktreeSuggestsNext(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	mock.IsTreeCleanErr = fmt.Errorf("workspace has uncommitted changes:\n M hello.go")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	_, err := Run(root, "bead-1", "", "", mock)
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "mindspec next") {
		t.Fatalf("expected recovery hint to mention `mindspec next`, got: %v", err)
	}
}

func TestRun_NoWorktree(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	// No worktrees at all
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return nil, nil
	}

	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return nil, fmt.Errorf("no results") }

	result, err := Run(root, "bead-1", "", "", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.BeadClosed {
		t.Error("bead should be closed")
	}
}

// stubChildrenByStatus installs a listJSONFn that returns children keyed by
// their --status=<name> filter. Each returned child's Status field is
// overwritten to match the requested filter so that phase.DerivePhaseFromChildren
// counts them correctly. Any status not in the map returns [].
func stubChildrenByStatus(byStatus map[string][]bead.BeadInfo) {
	listJSONFn = func(args ...string) ([]byte, error) {
		for _, a := range args {
			const prefix = "--status="
			if strings.HasPrefix(a, prefix) {
				status := strings.TrimPrefix(a, prefix)
				if items, ok := byStatus[status]; ok {
					stamped := make([]bead.BeadInfo, len(items))
					for i := range items {
						stamped[i] = items[i]
						stamped[i].Status = status
					}
					return json.Marshal(stamped)
				}
			}
		}
		return []byte("[]"), nil
	}
}

// writeBeadsCustomStatus creates .beads/config.yaml under root with the
// given custom-status declaration so queryAllChildren's bead.AllStatuses
// lookup will iterate it. Pass "" to skip custom statuses entirely (useful
// for tests that only care about built-in ones).
func writeBeadsCustomStatus(t *testing.T, root, customLine string) {
	t.Helper()
	if customLine == "" {
		return
	}
	dir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "status.custom: \"" + customLine + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAdvanceState_NextReady(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	// One closed + one open child → DerivePhaseFromChildren returns implement.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed": {{ID: "done-bead", Title: "[IMPL 001-test-spec.1] Done"}},
		"open":   {{ID: "next-bead", Title: "[IMPL 001-test-spec.2] Next"}},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			return json.Marshal([]bead.BeadInfo{
				{ID: "next-bead", Title: "[IMPL 001-test-spec.2] Next"},
			})
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, nextBead := advanceState(root, "001-test-spec")
	if mode != state.ModeImplement {
		t.Errorf("mode: got %q, want %q", mode, state.ModeImplement)
	}
	if nextBead != "next-bead" {
		t.Errorf("nextBead: got %q, want %q", nextBead, "next-bead")
	}
}

func TestAdvanceState_BlockedChildren(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	// All children open, none closed/in_progress → plan mode.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"open": {{ID: "blocked-bead", Title: "[001-test-spec] Bead 3: Blocked"}},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			return json.Marshal([]bead.BeadInfo{})
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, nextBead := advanceState(root, "001-test-spec")
	if mode != state.ModePlan {
		t.Errorf("mode: got %q, want %q", mode, state.ModePlan)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

func TestAdvanceState_AllDone(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	// All children closed → review mode.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed": {
			{ID: "done-1", Title: "[IMPL 001-test-spec.1] Done"},
			{ID: "done-2", Title: "[IMPL 001-test-spec.2] Done"},
		},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{})
	}

	mode, nextBead := advanceState(root, "001-test-spec")
	if mode != state.ModeReview {
		t.Errorf("mode: got %q, want %q", mode, state.ModeReview)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

// TestAdvanceState_InProgressBeadHoldsImplementPhase pins the fix for the
// premature review-mode bug. If any remaining bead is in_progress (e.g. a
// parallel agent has it claimed) when another bead completes, advanceState
// must stay in implement — not flip the spec to review. Earlier code queried
// only `--status=open` and silently missed in_progress beads.
func TestAdvanceState_InProgressBeadHoldsImplementPhase(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	// One closed (just completed) + one in_progress (peer claim) → implement.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed":      {{ID: "just-closed", Title: "[IMPL 001-test-spec.1] Just closed"}},
		"in_progress": {{ID: "claimed-bead", Title: "[IMPL 001-test-spec.2] Claimed by peer"}},
	})

	// No ready beads (the in_progress one is already claimed; nothing unblocked).
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			return json.Marshal([]bead.BeadInfo{})
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, nextBead := advanceState(root, "001-test-spec")
	if mode != state.ModeImplement {
		t.Errorf("in_progress bead must hold phase in implement: got %q, want %q (not review)", mode, state.ModeImplement)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty when no ready bead, got %q", nextBead)
	}
}

// TestAdvanceState_CustomResolvedStatusHoldsPhase confirms queryAllChildren
// reads custom statuses from .beads/config.yaml at runtime instead of
// hardcoding specific strings. A child in any project-declared custom status
// must prevent a premature flip to review.
func TestAdvanceState_CustomResolvedStatusHoldsPhase(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeBeadsCustomStatus(t, root, "resolved")
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	// One closed, one in a custom `resolved` status. Derivation treats any
	// non-closed/in_progress status as open, so this is closed + open → implement.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed":   {{ID: "done-1", Title: "[IMPL 001-test-spec.1] Done"}},
		"resolved": {{ID: "gate-1", Title: "[GATE] pending resolution"}},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			return json.Marshal([]bead.BeadInfo{})
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, _ := advanceState(root, "001-test-spec")
	if mode == state.ModeReview {
		t.Errorf("custom-status child must prevent review flip: got %q", mode)
	}
}

// TestAdvanceState_UndeclaredCustomStatusIsNotIterated is the negative case:
// if a project doesn't declare a status in status.custom, queryAllChildren
// must not iterate it (a bead sitting in an unknown status would be missed,
// which is the correct behaviour — it is literally an unknown status to the
// project and deserves human attention, not a silent special case).
func TestAdvanceState_UndeclaredCustomStatusIsNotIterated(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	// No .beads/config.yaml — only built-in statuses are iterated.
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed":     {{ID: "done-1", Title: "[IMPL 001-test-spec.1] Done"}},
		"undeclared": {{ID: "mystery", Title: "[???] Undeclared status"}},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{})
	}

	mode, _ := advanceState(root, "001-test-spec")
	// Only the `closed` bead is seen → all closed → review.
	if mode != state.ModeReview {
		t.Errorf("undeclared custom status must not be iterated: got mode %q, want %q", mode, state.ModeReview)
	}
}

func TestAdvanceState_NoEpic(t *testing.T) {
	saveAndRestore(t)

	setupTempRoot(t)
	// No epic found for spec → idle (ADR-0023: no lifecycle.yaml needed).
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil // no epics
	})
	t.Cleanup(restore)

	mode, nextBead := advanceState("", "test")
	if mode != state.ModeIdle {
		t.Errorf("mode: got %q, want %q", mode, state.ModeIdle)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

func TestRun_AdvancesToImplementWhenNextBeadReady(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }

	// One just-closed + one open child → phase derives to implement.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed": {{ID: "bead-1", Title: "[IMPL 008-test.1] Done"}},
		"open":   {{ID: "bead-2", Title: "[IMPL 008-test.2] Next"}},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []bead.BeadInfo{
				{ID: "bead-2", Title: "[IMPL 008-test.2] Next"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected")
	}

	result, err := Run(root, "bead-1", "", "", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextMode != state.ModeImplement {
		t.Fatalf("expected implement mode, got %s", result.NextMode)
	}
	if result.NextBead != "bead-2" {
		t.Errorf("expected next bead bead-2, got %s", result.NextBead)
	}
}

func TestRun_AdvancesToReviewWhenNoMoreBeads(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }

	// All children closed → review mode.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed": {{ID: "bead-1", Title: "[IMPL 008-test.1] Done"}},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{})
	}

	result, err := Run(root, "bead-1", "", "", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextMode != state.ModeReview {
		t.Fatalf("expected review mode, got %s", result.NextMode)
	}
}

func TestFormatResult_Implement(t *testing.T) {
	r := &Result{
		BeadID:          "bead-1",
		BeadClosed:      true,
		WorktreeRemoved: true,
		NextMode:        state.ModeImplement,
		NextBead:        "bead-2",
		NextSpec:        "008-test",
	}
	out := FormatResult(r)
	if !strings.Contains(out, "bead-1") {
		t.Errorf("should mention closed bead: %s", out)
	}
	if !strings.Contains(out, "bead-2") {
		t.Errorf("should mention next bead: %s", out)
	}
	if !strings.Contains(out, "Worktree removed") {
		t.Errorf("should mention worktree removal: %s", out)
	}
}

func TestFormatResult_Review(t *testing.T) {
	r := &Result{
		BeadID:     "bead-last",
		BeadClosed: true,
		NextMode:   state.ModeReview,
		NextSpec:   "test-spec",
	}
	out := FormatResult(r)
	if !strings.Contains(out, "review") {
		t.Errorf("should mention review: %s", out)
	}
	if !strings.Contains(out, "mindspec instruct") {
		t.Errorf("should mention mindspec instruct: %s", out)
	}
}

func TestRun_CloseFailsNonIdempotent(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	// closeBeadFn fails with a non-"already closed" error
	closeBeadFn = func(ids ...string) error {
		return fmt.Errorf("bd close failed: network error")
	}

	// fetchBeadByIDFn says bead is still open — not an idempotent case
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: "bead-1", Status: "open"}, nil
	}

	_, err := Run(root, "bead-1", "", "", mock)
	if err == nil {
		t.Fatal("expected error when close fails and bead is not closed")
	}
	if !strings.Contains(err.Error(), "closing bead") {
		t.Errorf("expected 'closing bead' error, got: %v", err)
	}
}

func TestRun_AutoCommitUsesExecutor(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	_, err := Run(root, "bead-1", "", "add feature X", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify CommitAll was called via executor
	commitCalls := mock.CallsTo("CommitAll")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 CommitAll call, got %d", len(commitCalls))
	}
	msg := commitCalls[0].Args[1].(string)
	if !strings.Contains(msg, "impl(bead-1)") || !strings.Contains(msg, "add feature X") {
		t.Errorf("CommitAll msg: got %q", msg)
	}
}

func TestRun_ImplOnlyGuardRejectsPlanPhase(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	// Stub phase to return "plan" mode (open children, none in_progress)
	stubPhaseEpicInMode(t, "008-test", "mol-parent-1", state.ModePlan)
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	_, err := Run(root, "bead-1", "", "", mock)
	if err == nil {
		t.Fatal("expected error from impl-only guard")
	}
	if !strings.Contains(err.Error(), "implementation beads only") {
		t.Errorf("expected impl-only guard message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "'plan' phase") {
		t.Errorf("expected error to mention plan phase, got: %v", err)
	}
}

func TestRun_ImplOnlyGuardAllowsReview(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpicInMode(t, "008-test", "mol-parent-1", state.ModeReview)
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	result, err := Run(root, "bead-1", "", "", mock)
	if err != nil {
		t.Fatalf("expected success in review phase, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
}

func TestRun_DirtyTreeHintIncludesBeadID(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	mock.IsTreeCleanErr = fmt.Errorf("workspace has uncommitted changes")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-my-bead", Path: "/tmp/wt", Branch: "bead/my-bead"},
		}, nil
	}

	_, err := Run(root, "my-bead", "", "", mock)
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "mindspec complete my-bead") {
		t.Errorf("hint should include bead ID, got: %v", err)
	}
}
