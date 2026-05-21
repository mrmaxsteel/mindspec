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
	origMergeMeta := completeMergeMetadataFn
	origGitEmail := gitUserEmailFn

	t.Cleanup(func() {
		closeBeadFn = origClose
		worktreeListFn = origWtList
		runBDFn = origRunBD
		listJSONFn = origListJSON
		resolveTargetFn = origResolveTarget
		findLocalRootFn = origFindLocalRoot
		fetchBeadByIDFn = origFetchBeadByID
		completeMergeMetadataFn = origMergeMeta
		gitUserEmailFn = origGitEmail
	})

	// Spec 089: phase.EnsureMigrated (wired into complete) shells to
	// `bd` via bead.MergeMetadata when the epic lacks mindspec_phase.
	// CI has no `bd` on PATH, so stub the seam to a no-op for the
	// duration of the test.
	restorePhaseMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		return nil
	})
	t.Cleanup(restorePhaseMerge)

	// Default stubs
	resolveTargetFn = func(root, flag string) (string, error) { return "", fmt.Errorf("no active specs") }
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) { return next.BeadInfo{}, fmt.Errorf("not found") }
	listJSONFn = func(args ...string) ([]byte, error) { return []byte("[]"), nil }
	// Spec 086 Bead 3: keep metadata + git-identity reads inert by
	// default so the existing tests don't shell out to bd or git.
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error { return nil }
	gitUserEmailFn = func() string { return "test@example.invalid" }
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

		// Parent queries for DerivePhase child enumeration.
		// PERF-1: cache.GetChildren now passes
		//   --status=open,in_progress,closed -n 0
		// in a single call. The stub returns a single child matching the
		// requested mode (DerivePhaseFromChildren counts the status).
		isParentQuery := false
		for _, a := range args {
			if a == "--parent" {
				isParentQuery = true
				break
			}
		}

		if isParentQuery {
			switch mode {
			case state.ModeImplement:
				return json.Marshal([]phase.ChildInfo{{
					ID: "stub-bead", Title: "[" + specID + "] stub", Status: "in_progress",
				}})
			case state.ModePlan:
				return json.Marshal([]phase.ChildInfo{{
					ID: "stub-bead", Title: "[" + specID + "] stub", Status: "open",
				}})
			case state.ModeReview:
				return json.Marshal([]phase.ChildInfo{{
					ID: "stub-bead", Title: "[" + specID + "] stub", Status: "closed",
				}})
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

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.BeadClosed {
		t.Error("bead should be closed")
	}
}

// stubChildrenByStatus installs a listJSONFn that returns children keyed by
// status. PERF-1: the cache now issues a single `--status=open,in_progress,closed`
// bd call and filters in-process, so this stub returns the union of all
// requested-status buckets (each stamped with its bucket's status) in one shot.
// Any status not in the map contributes no items.
func stubChildrenByStatus(byStatus map[string][]bead.BeadInfo) {
	listJSONFn = func(args ...string) ([]byte, error) {
		statuses := []string{}
		for _, a := range args {
			const prefix = "--status="
			if strings.HasPrefix(a, prefix) {
				csv := strings.TrimPrefix(a, prefix)
				for _, s := range strings.Split(csv, ",") {
					statuses = append(statuses, strings.TrimSpace(s))
				}
			}
		}
		if len(statuses) == 0 {
			// Default-open fallback (mirrors `bd list` behaviour) — keeps any
			// legacy caller that omitted --status working.
			statuses = []string{"open"}
		}
		var out []bead.BeadInfo
		for _, status := range statuses {
			items, ok := byStatus[status]
			if !ok {
				continue
			}
			for i := range items {
				stamped := items[i]
				stamped.Status = status
				out = append(out, stamped)
			}
		}
		return json.Marshal(out)
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

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	_, err := Run(root, "bead-1", "", "add feature X", mock, CompleteOpts{})
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

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
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

	_, err := Run(root, "my-bead", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "mindspec complete my-bead") {
		t.Errorf("hint should include bead ID, got: %v", err)
	}
}

// --- Spec 086 Bead 3: doc-sync gate + override metadata tests ---

// TestCompleteBlocksOnDocSkew: a bead whose diff touches an
// internal/ Go source file with no doc updates should be rejected
// by the doc-sync gate when no override is requested.
func TestCompleteBlocksOnDocSkew(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	// Source-only diff (internal/<domain>/foo.go) with no doc updates.
	// ValidateDocs runs both the legacy fallback (no domains dir) AND
	// the cmd-docs / source-vs-doc check, raising at least one
	// SevError under "doc-sync".
	mock.ChangedFilesResult = []string{"internal/contextpack/foo.go"}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected doc-sync gate to reject source-only diff")
	}
	if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("error should mention doc-sync: %v", err)
	}
	// Verify MergeBase was called for the gate
	if calls := mock.CallsTo("MergeBase"); len(calls) == 0 {
		t.Error("expected MergeBase call from doc-sync gate")
	}
}

// TestCompleteAllowsOverride: same source-only diff, but with the
// AllowDocSkew opts set, should succeed AND record the override
// metadata on the bead AFTER closeBeadFn returns.
func TestCompleteAllowsOverride(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	mock.ChangedFilesResult = []string{"internal/contextpack/foo.go"}

	// Recorder for the override metadata write.
	var metaCalls []map[string]interface{}
	var metaBeadID string
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaBeadID = id
		metaCalls = append(metaCalls, updates)
		return nil
	}
	gitUserEmailFn = func() string { return "override-user@example.invalid" }

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{AllowDocSkew: "doc PR in flight"})
	if err != nil {
		t.Fatalf("expected override to allow completion, got: %v", err)
	}

	if metaBeadID != "bead-1" {
		t.Errorf("override metadata should target bead-1, got %q", metaBeadID)
	}
	// Two metadata writes are expected: the override skew write
	// (this bead 3 feature) and the spec 080 mindspec_phase sync.
	// We assert the override one is present.
	foundOverride := false
	for _, m := range metaCalls {
		if reason, ok := m["mindspec_doc_skew_reason"].(string); ok && reason == "doc PR in flight" {
			if by, _ := m["mindspec_doc_skew_by"].(string); by != "override-user@example.invalid" {
				t.Errorf("mindspec_doc_skew_by: got %q, want override-user@example.invalid", by)
			}
			if at, _ := m["mindspec_doc_skew_at"].(string); at == "" {
				t.Error("mindspec_doc_skew_at should not be empty")
			}
			foundOverride = true
			break
		}
	}
	if !foundOverride {
		t.Errorf("expected an override metadata write with mindspec_doc_skew_reason; got %v", metaCalls)
	}
}

// --- Spec 087 Bead 3: ADR-divergence override/supersede tests ---

// writeADRDivergenceFixture builds a fixture under root that trips
// the ADR-divergence gate: a spec.md declaring "core" as an impacted
// domain, a plan.md citing only an execution-domain ADR, and that
// Accepted ADR on disk. Returns the spec ID.
func writeADRDivergenceFixture(t *testing.T, root, specID string) {
	t.Helper()

	specDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	specMD := "# Spec " + specID + "\n\n## Impacted Domains\n\n- core\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specMD), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	planMD := "---\nspec_id: " + specID + "\nstatus: Approved\nbead_ids:\n  - bead-1\nadr_citations:\n  - id: ADR-9001\n---\n\n# Plan\n"
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planMD), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}

	adrDir := filepath.Join(root, ".mindspec", "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir adr dir: %v", err)
	}
	adrMD := "# ADR-9001: Exec-only test\n\n" +
		"- **Date**: 2026-01-01\n" +
		"- **Status**: Accepted\n" +
		"- **Domain(s)**: execution\n" +
		"- **Deciders**: test\n" +
		"- **Supersedes**: n/a\n" +
		"- **Superseded-by**: n/a\n\n" +
		"## Decision\nTest fixture.\n"
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-9001.md"), []byte(adrMD), 0o644); err != nil {
		t.Fatalf("write ADR-9001.md: %v", err)
	}
}

// TestOverrideUnblocks: a complete.Run that would be rejected by the
// ADR-divergence gate (uncovered "core" domain touch) succeeds when
// --override-adr "<reason>" is set, AND the
// `mindspec_adr_override_*` metadata is written on the bead AFTER
// CompleteBead returns nil.
func TestOverrideUnblocks(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeADRDivergenceFixture(t, root, "087-test")
	stubPhaseEpic(t, "087-test", "epic-087")
	resolveTargetFn = func(r, flag string) (string, error) { return "087-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	// Source touch that ValidateDivergence will attribute to "core"
	// via the fallback `internal/core/**` (no OWNERSHIP.yaml present).
	mock.ChangedFilesResult = []string{"internal/core/foo.go"}

	// Recorder for metadata writes.
	var metaCalls []map[string]interface{}
	var metaBeadIDs []string
	metaSeenBeforeCompleteBead := false
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if len(mock.CallsTo("CompleteBead")) == 0 {
			metaSeenBeforeCompleteBead = true
		}
		metaBeadIDs = append(metaBeadIDs, id)
		metaCalls = append(metaCalls, updates)
		return nil
	}
	gitUserEmailFn = func() string { return "override-user@example.invalid" }

	// Sanity check: WITHOUT the override flag, the gate must reject.
	{
		probeMock := newMockExec()
		probeMock.MergeBaseResult = "merge-base-sha"
		probeMock.ChangedFilesResult = []string{"internal/core/foo.go"}
		_, err := Run(root, "bead-1", "", "", probeMock, CompleteOpts{AllowDocSkew: "test setup"})
		if err == nil || !strings.Contains(err.Error(), "adr-divergence") {
			t.Fatalf("baseline: expected adr-divergence error without override, got: %v", err)
		}
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{
		AllowDocSkew: "test setup",
		OverrideADR:  "wip — core ADR coming in followup",
	})
	if err != nil {
		t.Fatalf("override should allow completion, got: %v", err)
	}

	// Verify the override metadata was written to the right bead.
	foundOverride := false
	for i, m := range metaCalls {
		reason, ok := m["mindspec_adr_override_reason"].(string)
		if !ok {
			continue
		}
		if reason != "wip — core ADR coming in followup" {
			t.Errorf("override reason: got %q, want verbatim flag value", reason)
		}
		if metaBeadIDs[i] != "bead-1" {
			t.Errorf("override metadata target: got %q, want bead-1", metaBeadIDs[i])
		}
		if by, _ := m["mindspec_adr_override_by"].(string); by != "override-user@example.invalid" {
			t.Errorf("mindspec_adr_override_by: got %q", by)
		}
		if at, _ := m["mindspec_adr_override_at"].(string); at == "" {
			t.Error("mindspec_adr_override_at must not be empty")
		}
		foundOverride = true
		break
	}
	if !foundOverride {
		t.Fatalf("expected an mindspec_adr_override_reason write; got %v", metaCalls)
	}

	// Strict ordering: the metadata-write seam was NEVER invoked
	// before CompleteBead during this Run (panel CONSENSUS revision 4
	// carried forward to spec 087 Bead 3 — the seam check above
	// records every entry-time observation of whether CompleteBead
	// had been called).
	if metaSeenBeforeCompleteBead {
		t.Error("override metadata write occurred before CompleteBead — panel CONSENSUS rev 4 violation")
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 1 {
		t.Errorf("expected 1 CompleteBead call, got %d", len(calls))
	}
}

// TestSupersedeUnblocks: same fixture as TestOverrideUnblocks but with
// --supersede-adr ADR-0099. Asserts (1) the placeholder ADR file
// exists on disk at the user-supplied ID verbatim, (2) the run
// succeeds, (3) the four mindspec_adr_supersede_* keys are written
// on the bead AFTER CompleteBead.
func TestSupersedeUnblocks(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeADRDivergenceFixture(t, root, "087-test")
	stubPhaseEpic(t, "087-test", "epic-087")
	resolveTargetFn = func(r, flag string) (string, error) { return "087-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	mock.ChangedFilesResult = []string{"internal/core/foo.go"}

	var metaCalls []map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaCalls = append(metaCalls, updates)
		return nil
	}
	gitUserEmailFn = func() string { return "supersede-user@example.invalid" }

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{
		AllowDocSkew: "test setup",
		SupersedeADR: "ADR-0099",
	})
	if err != nil {
		t.Fatalf("supersede should allow completion, got: %v", err)
	}

	// The placeholder ADR file MUST exist on disk at the verbatim ID.
	adrPath := filepath.Join(root, ".mindspec", "docs", "adr", "ADR-0099.md")
	data, readErr := os.ReadFile(adrPath)
	if readErr != nil {
		t.Fatalf("expected placeholder ADR at %s: %v", adrPath, readErr)
	}
	body := string(data)
	if !strings.Contains(body, "**Status**: Proposed") {
		t.Errorf("placeholder ADR must carry Status: Proposed, got:\n%s", body)
	}
	if !strings.Contains(body, "**Domain(s)**: core") {
		t.Errorf("placeholder ADR must seed Domain(s) from the uncovered finding (core), got:\n%s", body)
	}

	// All four mindspec_adr_supersede_* keys must be written.
	foundSupersede := false
	for _, m := range metaCalls {
		id, ok := m["mindspec_adr_supersede_id"].(string)
		if !ok {
			continue
		}
		if id != "ADR-0099" {
			t.Errorf("mindspec_adr_supersede_id: got %q, want ADR-0099", id)
		}
		reason, _ := m["mindspec_adr_supersede_reason"].(string)
		if !strings.Contains(reason, "ADR-0099") {
			t.Errorf("mindspec_adr_supersede_reason must reference the new ID, got %q", reason)
		}
		if at, _ := m["mindspec_adr_supersede_at"].(string); at == "" {
			t.Error("mindspec_adr_supersede_at must not be empty")
		}
		if by, _ := m["mindspec_adr_supersede_by"].(string); by != "supersede-user@example.invalid" {
			t.Errorf("mindspec_adr_supersede_by: got %q", by)
		}
		foundSupersede = true
		break
	}
	if !foundSupersede {
		t.Fatalf("expected mindspec_adr_supersede_id write; got %v", metaCalls)
	}
}

// TestSupersedeRejectsExistingID: --supersede-adr against an already
// existing ADR id returns an error containing "already exists" and
// MUST NOT mutate the existing ADR file nor write any metadata.
func TestSupersedeRejectsExistingID(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeADRDivergenceFixture(t, root, "087-test")
	stubPhaseEpic(t, "087-test", "epic-087")
	resolveTargetFn = func(r, flag string) (string, error) { return "087-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	// Pre-seed ADR-9001 collision (it already exists from the fixture).
	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	mock.ChangedFilesResult = []string{"internal/core/foo.go"}

	originalBody, _ := os.ReadFile(filepath.Join(root, ".mindspec", "docs", "adr", "ADR-9001.md"))

	var metaCalls []map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaCalls = append(metaCalls, updates)
		return nil
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{
		AllowDocSkew: "test setup",
		SupersedeADR: "ADR-9001",
	})
	if err == nil {
		t.Fatal("expected error when --supersede-adr collides with existing ADR")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should contain 'already exists', got: %v", err)
	}

	// Existing ADR must not be mutated.
	afterBody, _ := os.ReadFile(filepath.Join(root, ".mindspec", "docs", "adr", "ADR-9001.md"))
	if string(afterBody) != string(originalBody) {
		t.Error("existing ADR was mutated by failed --supersede-adr call")
	}

	// No supersede metadata may have been written (the failure aborts
	// before terminal mutation).
	for _, m := range metaCalls {
		if _, ok := m["mindspec_adr_supersede_id"]; ok {
			t.Errorf("no supersede metadata may be written on collision; got %v", m)
		}
	}
}

// TestOverrideMetadataGoesThroughSeam: the override write MUST flow
// through completeMergeMetadataFn (NOT a direct bead.MergeMetadata
// call). Per spec.md Requirement 12 + the panel revision 8 rename,
// the seam is the only audit-write path.
func TestOverrideMetadataGoesThroughSeam(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeADRDivergenceFixture(t, root, "087-test")
	stubPhaseEpic(t, "087-test", "epic-087")
	resolveTargetFn = func(r, flag string) (string, error) { return "087-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	mock.ChangedFilesResult = []string{"internal/core/foo.go"}

	seamCalls := 0
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if _, ok := updates["mindspec_adr_override_reason"]; ok {
			seamCalls++
		}
		return nil
	}

	if _, err := Run(root, "bead-1", "", "", mock, CompleteOpts{
		AllowDocSkew: "test setup",
		OverrideADR:  "captured via seam",
	}); err != nil {
		t.Fatalf("override should allow completion, got: %v", err)
	}

	if seamCalls != 1 {
		t.Errorf("expected exactly one seam call carrying mindspec_adr_override_reason; got %d", seamCalls)
	}
}

// TestSkewMetadataWrittenAfterSuccess: if the terminal mutation
// (closeBeadFn) returns an error AND the bead is not already-closed,
// the override metadata must NOT be written. The failure itself is
// the audit trail (panel CONSENSUS revision 4).
func TestSkewMetadataWrittenAfterSuccess(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	// closeBeadFn fails with a non-idempotent error.
	closeBeadFn = func(ids ...string) error { return fmt.Errorf("bd close failed: simulated") }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "open"}, nil
	}

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	mock.ChangedFilesResult = []string{"internal/contextpack/foo.go"}

	// Track every metadata call.
	var metaCalls []map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaCalls = append(metaCalls, updates)
		return nil
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{AllowDocSkew: "doc PR in flight"})
	if err == nil {
		t.Fatal("expected closeBeadFn failure to propagate")
	}
	// The metadata write block runs AFTER closeBeadFn returns nil.
	// Since close failed, no override metadata may have been written.
	for _, m := range metaCalls {
		if _, ok := m["mindspec_doc_skew_reason"]; ok {
			t.Errorf("override metadata written despite close failure: %v", m)
		}
	}
}
