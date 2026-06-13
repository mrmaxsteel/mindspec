package complete

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

// finalRecoveryCommand extracts the command of the FINAL `recovery: `
// line of a guard-failure message (Bead 9 punch-list B9: per-site tests
// assert on the extracted command, not just message substrings).
func finalRecoveryCommand(t *testing.T, msg string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, guard.RecoveryPrefix) {
		t.Fatalf("message does not end with a recovery line: %q", msg)
	}
	return strings.TrimSpace(strings.TrimPrefix(last, guard.RecoveryPrefix))
}

// TestCheckDirtyTreeFnDefaultsToNextCheckDirtyTreeDetail kills mutant
// M6d (Bead 9 punch-list B10): the production binding of the Reqs 6/7
// classification seam MUST be next.CheckDirtyTreeDetail — a one-line
// rebind disables the entire artifact-aware dirty-tree gate with all
// other tests green. Pattern: Bead 3's reflect-Pointer identity pin
// (cmd/mindspec/repair_test.go, internal/approve/impl_test.go).
func TestCheckDirtyTreeFnDefaultsToNextCheckDirtyTreeDetail(t *testing.T) {
	if reflect.ValueOf(checkDirtyTreeFn).Pointer() != reflect.ValueOf(next.CheckDirtyTreeDetail).Pointer() {
		t.Fatal("checkDirtyTreeFn must default to next.CheckDirtyTreeDetail (spec 092 Reqs 6/7, DQ-2)")
	}
}

// TestCompleteGetwdFnDefaultsToOsGetwd kills Bead 7 panel mutant M7:
// the Req 8 context-line seam MUST default to os.Getwd — every test
// swaps the seam in saveAndRestore, so a severed default would go
// undetected without this identity pin.
func TestCompleteGetwdFnDefaultsToOsGetwd(t *testing.T) {
	if reflect.ValueOf(completeGetwdFn).Pointer() != reflect.ValueOf(os.Getwd).Pointer() {
		t.Fatal("completeGetwdFn must default to os.Getwd (spec 092 Req 8)")
	}
}

// saveAndRestore saves all function variables and returns a restore function.
func saveAndRestore(t *testing.T) {
	t.Helper()

	// Spec 092 Req 3c moved an unconditional os.Chdir(repoRoot) INSIDE
	// complete.Run (complete.go) so the terminal mutation survives the
	// bead worktree being removed out from under the process. As a
	// side effect, every Run-calling test leaves the process cwd parked
	// at its own setupTempRoot() temp dir; once that t.TempDir() is
	// removed at test teardown the process cwd is a deleted directory,
	// and the NEXT test that opens with os.Getwd() (e.g.
	// TestPrintResultWarningsRecursStatelessly) fails `getwd: no such
	// file or directory` under CI's serialized `-race` ordering.
	//
	// saveAndRestore runs first in every Run-calling test (before
	// setupTempRoot), so this cwd-restoring cleanup is registered first
	// and — cleanups being LIFO — runs LAST, after every t.TempDir
	// removal, leaving the process cwd at a stable real directory for
	// the next test in the package.
	origWd, wdErr := os.Getwd()
	if wdErr == nil {
		t.Cleanup(func() { _ = os.Chdir(origWd) })
	}

	origClose := closeBeadFn
	origWtList := worktreeListFn
	origRunBD := runBDFn
	origListJSON := listJSONFn
	origResolveTarget := resolveTargetFn
	origFindLocalRoot := findLocalRootFn
	origFetchBeadByID := fetchBeadByIDFn
	origMergeMeta := completeMergeMetadataFn
	origGitEmail := gitUserEmailFn
	origCheckDirty := checkDirtyTreeFn
	origGetwd := completeGetwdFn

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
		checkDirtyTreeFn = origCheckDirty
		completeGetwdFn = origGetwd
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
	// Spec 092 Reqs 6/7: the artifact-aware tree classification shells
	// out to git/bd in production (next.CheckDirtyTreeDetail); default to
	// a clean tree so existing tests stay hermetic.
	checkDirtyTreeFn = func(repoRoot, cwd string) (artifactDirt, userDirt []string, err error) {
		return nil, nil, nil
	}
	// Spec 092 Req 8: pin the context-line cwd so the asserted worktree
	// kind does not depend on where `go test` runs (the repo checkout
	// itself may be a bead worktree).
	completeGetwdFn = func() (string, error) { return "/testcwd", nil }
}

// newMockExec creates a MockExecutor with defaults suitable for complete tests.
func newMockExec() *executor.MockExecutor {
	return &executor.MockExecutor{}
}

// serveRefFromDisk wires a MockExecutor's ref-read seams
// (FileAtRefOrAbsent + TreeDirsAtRef) to resolve against the on-disk
// tree at root, simulating "the diffed ref's tree == this fixture".
// Spec 095 moved the per-bead gates' OWNERSHIP attribution (manifests +
// domain enumeration) onto beadHead; these mock-backed tests build their
// OWNERSHIP fixture on disk, so this makes the ref read resolve to the
// same files. Absent paths classify as claims-nothing (present false,
// nil error) — never an operational error — mirroring the real
// MindspecExecutor.FileAtRefOrAbsent contract.
func serveRefFromDisk(mock *executor.MockExecutor, root string) {
	mock.FileAtRefOrAbsentFn = func(_ref, rel string) ([]byte, bool, error) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return data, true, nil
	}
	mock.TreeDirsAtRefFn = func(_ref, dir string) ([]string, error) {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		var dirs []string
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			}
		}
		return dirs, nil
	}
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
	restoreList := phase.SetListJSONForTest(phaseEpicListJSONStub(specID, epicID, mode))
	t.Cleanup(restoreList)

	restoreRun := phase.SetRunBDForTest(phaseEpicRunBDStub(epicID))
	t.Cleanup(restoreRun)
}

// phaseEpicListJSONStub returns the listJSON stub behind
// stubPhaseEpicInMode as a plain closure so tests can wrap it (e.g.
// with a cwd-sensitivity guard, see
// TestRun_FromInsideBeadWorktree_PhaseIntegrity).
func phaseEpicListJSONStub(specID, epicID, mode string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
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
	}
}

// phaseEpicRunBDStub returns the runBD stub behind stubPhaseEpicInMode
// as a plain closure (see phaseEpicListJSONStub).
func phaseEpicRunBDStub(epicID string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		// queryEpicStatus: bd show <id> --json
		if len(args) >= 1 && args[0] == "show" {
			return json.Marshal([]phase.EpicInfo{{ID: epicID, Status: "open"}})
		}
		return []byte("[]"), nil
	}
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
	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		return nil, []string{"modified-file.go"}, nil
	}

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
	if !strings.Contains(err.Error(), "uncommitted user changes") {
		t.Errorf("error should mention uncommitted user changes: %v", err)
	}
	// Req 6: the block message names the dirty paths.
	if !strings.Contains(err.Error(), "modified-file.go") {
		t.Errorf("error should name the dirty path: %v", err)
	}
	// Req 12: guard failure ends with a recovery line.
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("user-dirt block must end with a recovery line, got: %v", err)
	}
	// Req 8 (mindspec-tjat): worktree-context line naming where the
	// command ran (the pinned /testcwd → main kind) and the checkout
	// this guard evaluated (the bead worktree), preceding the final
	// recovery line.
	wantCtx := "you are in the main worktree (/testcwd); this check evaluated /tmp/worktree-bead-1"
	if !strings.Contains(err.Error(), wantCtx) {
		t.Errorf("user-dirt block missing context line %q, got: %v", wantCtx, err)
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[len(lines)-2], "you are in the ") {
		t.Errorf("context line must immediately precede the final recovery line, got: %v", err)
	}
	// B9 (bead5 R3-1, mutant M5c): the FINAL recovery command is the
	// auto-commit re-run and is never a banned (Req 19) command.
	cmd := finalRecoveryCommand(t, err.Error())
	if guard.IsBannedRecoveryCommand(cmd) {
		t.Errorf("final recovery command is banned (Req 19): %q", cmd)
	}
	if want := `mindspec complete bead-1 "describe what you did"`; cmd != want {
		t.Errorf("final recovery command = %q, want %q", cmd, want)
	}
	// Honest behavior: a blocked completion mutates nothing — no
	// artifact-sync commit, no bead close, no merge.
	if calls := mock.CallsTo("CommitAll"); len(calls) != 0 {
		t.Errorf("expected no CommitAll calls on user-dirt block, got %d", len(calls))
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected no CompleteBead calls on user-dirt block, got %d", len(calls))
	}
}

func TestRun_DirtyTreeWithoutWorktreeSuggestsNext(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		return nil, []string{"hello.go"}, nil
	}

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "mindspec next") {
		t.Fatalf("expected recovery hint to mention `mindspec next`, got: %v", err)
	}
	if !strings.Contains(err.Error(), "hello.go") {
		t.Errorf("error should name the dirty path: %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("no-worktree user-dirt block must end with a recovery line, got: %v", err)
	}
	// Req 8 (mindspec-tjat): with no bead worktree the check evaluated
	// root; the context line still names where the command ran.
	wantCtx := "you are in the main worktree (/testcwd); this check evaluated " + root
	if !strings.Contains(err.Error(), wantCtx) {
		t.Errorf("no-worktree user-dirt block missing context line %q, got: %v", wantCtx, err)
	}
	// M5b (Bead 7 panel): the context line immediately precedes the
	// final recovery line (Req 12 ordering) on the NO-WORKTREE branch
	// too — mirrors TestRun_DirtyTreeRefuses' assertion for the
	// with-worktree branch.
	lines := strings.Split(err.Error(), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[len(lines)-2], "you are in the ") {
		t.Errorf("context line must immediately precede the final recovery line, got: %v", err)
	}
	// B9 (bead5 R3-1, mutant M5c): the FINAL recovery command itself —
	// not just the message body — is `mindspec next`, and it is never a
	// banned (Req 19) command.
	cmd := finalRecoveryCommand(t, err.Error())
	if guard.IsBannedRecoveryCommand(cmd) {
		t.Errorf("final recovery command is banned (Req 19): %q", cmd)
	}
	if cmd != "mindspec next" {
		t.Errorf("final recovery command = %q, want %q", cmd, "mindspec next")
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

// TestRun_LastLifecycleBeadWithOpenBugChildAdvancesToReview is the spec
// 095 ry73 e2e guarantee at the `complete` end: closing the LAST lifecycle
// (task) bead while a non-lifecycle bug child is ALREADY open must derive
// `review` (the open bug is ignored) and persist mindspec_phase=="review"
// via the step-6.5 sync. RED-on-revert: counting the open bug would derive
// `implement`, leaving the spec unable to reach `impl approve` without a
// manual detach + repair.
func TestRun_LastLifecycleBeadWithOpenBugChildAdvancesToReview(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }

	// Last task bead closed + an open bug child filed earlier as an epic
	// child. Lifecycle-only derivation → review.
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed": {{ID: "bead-1", Title: "[IMPL 008-test.1] Done", IssueType: "task"}},
		"open":   {{ID: "bug-7", Title: "follow-up", IssueType: "bug"}},
	})

	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{})
	}

	// Capture the step-6.5 mindspec_phase sync write.
	var syncedPhase string
	origMerge := completeMergeMetadataFn
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if p, ok := updates["mindspec_phase"].(string); ok {
			syncedPhase = p
		}
		return nil
	}
	t.Cleanup(func() { completeMergeMetadataFn = origMerge })

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextMode != state.ModeReview {
		t.Fatalf("expected review mode (open bug must not block), got %s", result.NextMode)
	}
	if syncedPhase != state.ModeReview {
		t.Errorf("step-6.5 sync wrote mindspec_phase=%q, want %q", syncedPhase, state.ModeReview)
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
	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		return nil, []string{"some-user-file.go"}, nil
	}

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
	// The existing auto-commit hint, now the Req 12 recovery line.
	if !strings.Contains(err.Error(), "mindspec complete my-bead") {
		t.Errorf("hint should include bead ID, got: %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("auto-commit hint must be a final recovery line, got: %v", err)
	}
	// B9: the FINAL recovery command carries the bead ID and is never a
	// banned (Req 19) command.
	cmd := finalRecoveryCommand(t, err.Error())
	if guard.IsBannedRecoveryCommand(cmd) {
		t.Errorf("final recovery command is banned (Req 19): %q", cmd)
	}
	if want := `mindspec complete my-bead "describe what you did"`; cmd != want {
		t.Errorf("final recovery command = %q, want %q", cmd, want)
	}
}

// --- Spec 092 Reqs 6/7 (mindspec-i4ad): artifact-aware clean-tree check ---

// TestRun_ArtifactOnlyDirtSucceeds: when the only dirt surviving the
// ADR-0025 normalization is .beads/issues.jsonl, complete.Run folds it
// into a follow-up `chore: sync beads artifact` commit (via the
// executor) and proceeds — never blocking, never requiring --no-verify.
func TestRun_ArtifactOnlyDirtSucceeds(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	var gotRepoRoot, gotCwd string
	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		gotRepoRoot, gotCwd = repoRoot, cwd
		return []string{".beads/issues.jsonl"}, nil, nil
	}

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("artifact-only dirt must never block completion, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}

	// The normalization must target the same checkout being status-checked.
	if gotRepoRoot != "/tmp/worktree-bead-1" || gotCwd != "/tmp/worktree-bead-1" {
		t.Errorf("classification paths: got repoRoot=%q cwd=%q, want both the bead worktree", gotRepoRoot, gotCwd)
	}

	// Req 7: exactly one follow-up commit, through the executor, at the
	// bead worktree, with the DQ-4 message.
	commitCalls := mock.CallsTo("CommitAll")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 CommitAll (artifact sync), got %d", len(commitCalls))
	}
	if path := commitCalls[0].Args[0].(string); path != "/tmp/worktree-bead-1" {
		t.Errorf("artifact-sync commit path: got %q, want bead worktree", path)
	}
	if msg := commitCalls[0].Args[1].(string); msg != "chore: sync beads artifact" {
		t.Errorf("artifact-sync commit msg: got %q, want %q", msg, "chore: sync beads artifact")
	}

	// The terminal mutation still ran.
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 1 {
		t.Fatalf("expected 1 CompleteBead call, got %d", len(calls))
	}
}

// TestRun_ArtifactDirtAfterAutoCommitFollowUpCommit: the i4ad field
// case — a pre-commit hook re-exports the JSONL DURING the auto-commit,
// re-dirtying the tree immediately after. The residual artifact dirt is
// folded into a follow-up commit (DQ-4: follow-up, never amend), so the
// completion succeeds with two commits: impl(...) then the chore sync.
func TestRun_ArtifactDirtAfterAutoCommitFollowUpCommit(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		// Post-auto-commit snapshot: the hook's re-export survives
		// normalization (Dolt state changed since the commit's export).
		return []string{".beads/issues.jsonl"}, nil, nil
	}

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	_, err := Run(root, "bead-1", "", "implement the thing", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("post-auto-commit artifact re-export must never block, got: %v", err)
	}

	commitCalls := mock.CallsTo("CommitAll")
	if len(commitCalls) != 2 {
		t.Fatalf("expected 2 CommitAll calls (auto-commit + follow-up sync), got %d", len(commitCalls))
	}
	first := commitCalls[0].Args[1].(string)
	second := commitCalls[1].Args[1].(string)
	if !strings.Contains(first, "impl(bead-1)") || !strings.Contains(first, "implement the thing") {
		t.Errorf("first commit should be the auto-commit, got %q", first)
	}
	if second != "chore: sync beads artifact" {
		t.Errorf("second commit should be the follow-up artifact sync, got %q", second)
	}
}

// TestRun_ArtifactAndUserDirtBlocks: when BOTH artifact and user dirt
// exist, user dirt blocks — the artifact handling must not mask it. No
// artifact-sync commit, no terminal mutation.
func TestRun_ArtifactAndUserDirtBlocks(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		return []string{".beads/issues.jsonl"}, []string{"main.go", "internal/foo/bar.go"}, nil
	}

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected user dirt to block even alongside artifact dirt")
	}
	for _, p := range []string{"main.go", "internal/foo/bar.go"} {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("block message should name dirty path %q, got: %v", p, err)
		}
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("user-dirt block must end with a recovery line, got: %v", err)
	}
	// B9: the final recovery command is never a banned (Req 19) command.
	if cmd := finalRecoveryCommand(t, err.Error()); guard.IsBannedRecoveryCommand(cmd) {
		t.Errorf("final recovery command is banned (Req 19): %q", cmd)
	}
	if calls := mock.CallsTo("CommitAll"); len(calls) != 0 {
		t.Errorf("artifact handling must not run when user dirt blocks; got %d CommitAll calls", len(calls))
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected no CompleteBead call on block, got %d", len(calls))
	}
}

// TestRun_ArtifactSyncCommitFailureAborts: HC-4 — if the follow-up
// artifact-sync commit fails, the command fails BEFORE the terminal
// mutation (no bead close, no merge).
func TestRun_ArtifactSyncCommitFailureAborts(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	mock.CommitAllErr = fmt.Errorf("commit hook exploded")

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
		t.Fatal("expected error when the artifact-sync commit fails")
	}
	if !strings.Contains(err.Error(), "committing beads artifact sync") {
		t.Errorf("error should name the artifact-sync step, got: %v", err)
	}
	if closed {
		t.Error("bead must not be closed when the pre-terminal artifact sync fails")
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected no CompleteBead call, got %d", len(calls))
	}
}

// TestRun_DirtyTreeCheckErrorPropagates: a classification failure (git
// status unavailable, bd export failure) aborts before any mutation.
func TestRun_DirtyTreeCheckErrorPropagates(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		return nil, nil, fmt.Errorf("normalizing beads export: bd export in /tmp: boom")
	}

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected classification error to propagate")
	}
	if !strings.Contains(err.Error(), "checking working tree") {
		t.Errorf("error should be wrapped as a working-tree check failure, got: %v", err)
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected no CompleteBead call, got %d", len(calls))
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

	// Recorder for the override metadata write. The spec 080
	// mindspec_phase sync also routes through this seam (spec 092
	// Bead 4 testability change), so record the target id per call.
	type metaWrite struct {
		id      string
		updates map[string]interface{}
	}
	var metaCalls []metaWrite
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaCalls = append(metaCalls, metaWrite{id: id, updates: updates})
		return nil
	}
	gitUserEmailFn = func() string { return "override-user@example.invalid" }

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{AllowDocSkew: "doc PR in flight"})
	if err != nil {
		t.Fatalf("expected override to allow completion, got: %v", err)
	}

	// Two metadata writes are expected: the override skew write
	// (this bead 3 feature) and the spec 080 mindspec_phase sync.
	// We assert the override one is present and targets the bead.
	foundOverride := false
	for _, c := range metaCalls {
		m := c.updates
		if reason, ok := m["mindspec_doc_skew_reason"].(string); ok && reason == "doc PR in flight" {
			if c.id != "bead-1" {
				t.Errorf("override metadata should target bead-1, got %q", c.id)
			}
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
// domain, an OWNERSHIP.yaml claiming internal/core/** for that domain
// (spec 091 Req 13 removed the silent loader fallback, so attribution
// requires a real manifest — a manifest-less domain claims nothing),
// a plan.md citing only an execution-domain ADR, and that Accepted
// ADR on disk. Returns the spec ID.
func writeADRDivergenceFixture(t *testing.T, root, specID string) {
	t.Helper()

	specDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}

	coreDir := filepath.Join(root, ".mindspec", "docs", "domains", "core")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("mkdir core domain dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "OWNERSHIP.yaml"),
		[]byte("paths:\n  - internal/core/**\n"), 0o644); err != nil {
		t.Fatalf("write core OWNERSHIP.yaml: %v", err)
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
	// via the fixture's OWNERSHIP.yaml (`internal/core/**`).
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
	// Spec 095: the per-bead gates read OWNERSHIP from beadHead; serve
	// that ref tree from the on-disk fixture so internal/core/foo.go
	// attributes to "core" (→ an uncovered finding seeds the placeholder).
	serveRefFromDisk(mock, root)

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

// --- Spec 091 Bead 5: warnings pipe (Req 22(a) + printing half of 22(b)) ---

// captureWarnOutput swaps the package-level warnWriter seam for a
// buffer and restores it on cleanup.
func captureWarnOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := warnWriter
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = orig })
	return &buf
}

// TestCompleteWarnStreamDefaultsToStderr pins the Req 22(a) stream
// contract: in production, WARN lines go to stderr.
func TestCompleteWarnStreamDefaultsToStderr(t *testing.T) {
	if warnWriter != os.Stderr {
		t.Errorf("warnWriter must default to os.Stderr, got %T", warnWriter)
	}
}

// TestCompletePrintsDocSyncWarningAndProceeds: a diff that produces a
// warning-severity doc-sync issue but NO errors must print
// `WARN <name>: <message>` AND complete successfully (warnings never
// block). Req 22(a), including the HasFailures()==false case.
func TestCompletePrintsDocSyncWarningAndProceeds(t *testing.T) {
	saveAndRestore(t)
	buf := captureWarnOutput(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "091-warn-pipe", "epic-091")
	resolveTargetFn = func(r, flag string) (string, error) { return "091-warn-pipe", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	// cmd/ source + a non-operator doc: the cmd-docs lane emits a
	// SevWarning and no lane emits a SevError.
	mock.ChangedFilesResult = []string{"cmd/mindspec/foo.go", "docs/notes.md"}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("warning-only doc-sync result must not block completion, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "WARN cmd-docs: cmd/ changes without operator-docs update") {
		t.Errorf("expected `WARN cmd-docs: <message>` line, got %q", out)
	}
	// Exact format: line starts with `WARN <name>: ` (no decoration).
	if !strings.HasPrefix(out, "WARN cmd-docs: ") {
		t.Errorf("WARN line must be formatted `WARN <name>: <message>`, got %q", out)
	}
	// Exactly one consumer prints — no double-print.
	if n := strings.Count(out, "WARN cmd-docs:"); n != 1 {
		t.Errorf("expected exactly 1 WARN line per issue per run, got %d in %q", n, out)
	}
}

// TestCompleteNoWarningsPrintsNothing: zero warning-severity issues →
// no WARN line (companion case for Req 22(a)).
func TestCompleteNoWarningsPrintsNothing(t *testing.T) {
	saveAndRestore(t)
	buf := captureWarnOutput(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "091-warn-pipe", "epic-091")
	resolveTargetFn = func(r, flag string) (string, error) { return "091-warn-pipe", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	mock.ChangedFilesResult = nil // empty diff → no issues at all

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("clean doc-sync result must complete, got: %v", err)
	}
	if strings.Contains(buf.String(), "WARN") {
		t.Errorf("no warnings in result → no WARN output, got %q", buf.String())
	}
}

// TestPrintResultWarningsRecursStatelessly pins the HC-2 printing
// half: rendering the SAME warning-carrying result twice prints the
// WARN line BOTH times (no suppression, no dedup) and creates no
// marker/state file anywhere (the rendering path does no persistence).
// It also pins severity-genericity: ANY SevWarning renders, error
// issues never do.
func TestPrintResultWarningsRecursStatelessly(t *testing.T) {
	// Run from an empty dir so any sneaky relative-path persistence
	// would be visible.
	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	r := &validate.Result{}
	r.AddWarning("missing-source-globs", "source_globs not set in .mindspec/config.yaml")
	r.AddError("doc-sync", "errors are not rendered by the warnings pipe")

	var buf bytes.Buffer
	printResultWarnings(&buf, r)
	printResultWarnings(&buf, r) // recurrence: same result, second run

	want := "WARN missing-source-globs: source_globs not set in .mindspec/config.yaml\n"
	if buf.String() != want+want {
		t.Errorf("warning must print on BOTH runs, verbatim:\nwant %q\ngot  %q", want+want, buf.String())
	}
	if strings.Contains(buf.String(), "doc-sync") {
		t.Errorf("SevError issues must not render as WARN lines: %q", buf.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("rendering must persist NO marker/state file (HC-2); found %v", entries)
	}
}

// --- Spec 092 Bead 4: terminal-command cwd safety (mindspec-qxsy) ---

// doomedExec wraps MockExecutor so CompleteBead can reify the side
// effect the real executor has in the field: removing the bead worktree
// the process was invoked from.
type doomedExec struct {
	*executor.MockExecutor
	onCompleteBead func()
}

func (d *doomedExec) CompleteBead(beadID, specBranch, msg string) error {
	if d.onCompleteBead != nil {
		d.onCompleteBead()
	}
	return d.MockExecutor.CompleteBead(beadID, specBranch, msg)
}

// cwdSensitive wraps a bd stub so it fails exactly the way a real bd
// subprocess spawn fails when the process cwd has been deleted. This is
// what makes TestRun_FromInsideBeadWorktree_PhaseIntegrity
// discriminating: without the Req 3c chdir inside Run, every bd call
// after CompleteBead errors, advanceState silently degrades to ModeIdle,
// and the mindspec_phase sync is skipped.
func cwdSensitive(fn func(args ...string) ([]byte, error)) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		if _, err := os.Getwd(); err != nil {
			return nil, fmt.Errorf("simulated bd spawn from deleted cwd: %w", err)
		}
		return fn(args...)
	}
}

// TestRun_FromInsideBeadWorktree_PhaseIntegrity is the spec 092 AC
// "qxsy unit (complete-side phase integrity, Req 3c)": running
// complete.Run with the process cwd INSIDE the bead worktree that
// CompleteBead removes must (a) leave the epic's mindspec_phase metadata
// equal to the child-derived phase, (b) return a mode that is NOT
// falsely idle, and (c) leave the process cwd at the repo root.
func TestRun_FromInsideBeadWorktree_PhaseIntegrity(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	specID := "009-doomed"
	epicID := "epic-doom"

	// Phase stubs: all children closed → derived phase is review. Both
	// channels fail when called from a deleted cwd, like real bd.
	restoreList := phase.SetListJSONForTest(cwdSensitive(phaseEpicListJSONStub(specID, epicID, state.ModeReview)))
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(cwdSensitive(phaseEpicRunBDStub(epicID)))
	t.Cleanup(restoreRun)

	resolveTargetFn = func(r, flag string) (string, error) { return specID, nil }
	closeBeadFn = func(ids ...string) error { return nil }

	// queryAllChildren channel (complete package seam): one closed
	// child → review. Also cwd-sensitive.
	listJSONFn = cwdSensitive(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--status=closed" {
				return json.Marshal([]phase.ChildInfo{{
					ID: "bead-doom", Title: "[" + specID + "] doomed", Status: "closed",
				}})
			}
		}
		return []byte("[]"), nil
	})
	runBDFn = cwdSensitive(func(args ...string) ([]byte, error) { return []byte("[]"), nil })

	// Bead worktree on disk; the process is invoked from INSIDE it.
	beadWt := filepath.Join(root, ".worktrees", "worktree-bead-doom")
	if err := os.MkdirAll(beadWt, 0o755); err != nil {
		t.Fatalf("mkdir bead worktree: %v", err)
	}
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-doom", Path: beadWt, Branch: "bead/bead-doom"},
		}, nil
	}

	// Record every metadata write (the phase sync routes through the
	// seam since spec 092 Bead 4).
	type metaWrite struct {
		id      string
		updates map[string]interface{}
	}
	var metaCalls []metaWrite
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaCalls = append(metaCalls, metaWrite{id: id, updates: updates})
		return nil
	}

	// CompleteBead removes the worktree the process sits in — the
	// field condition mindspec-qxsy pins.
	mock := &doomedExec{
		MockExecutor: newMockExec(),
		onCompleteBead: func() {
			if err := os.RemoveAll(beadWt); err != nil {
				t.Fatalf("removing bead worktree: %v", err)
			}
		},
	}

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(beadWt); err != nil {
		t.Fatalf("chdir into bead worktree: %v", err)
	}

	result, err := Run(root, "bead-doom", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// (b) NOT falsely idle — children derive to review.
	if result.NextMode != state.ModeReview {
		t.Errorf("NextMode: got %q, want %q (a falsely idle mode means the deleted-cwd degradation was not prevented)", result.NextMode, state.ModeReview)
	}

	// (a) mindspec_phase synced to the child-derived phase on the epic.
	foundPhaseSync := false
	for _, c := range metaCalls {
		if v, ok := c.updates["mindspec_phase"].(string); ok && v == state.ModeReview {
			if c.id != epicID {
				t.Errorf("mindspec_phase sync targeted %q, want epic %q", c.id, epicID)
			}
			foundPhaseSync = true
		}
	}
	if !foundPhaseSync {
		t.Errorf("expected a mindspec_phase=%q sync write on the epic; got %v", state.ModeReview, metaCalls)
	}

	// (c) the process cwd is the repo root, not the deleted worktree.
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("process ended in an unresolvable cwd: %v", wdErr)
	}
	realWd, _ := filepath.EvalSymlinks(wd)
	realRoot, _ := filepath.EvalSymlinks(root)
	if realWd != realRoot {
		t.Errorf("process cwd after Run: got %q, want repo root %q", wd, root)
	}
}

// TestFormatResult_ImplementIncludesCdHint is the spec 092 AC "qxsy
// unit (Req 4 FormatResult)": the implement-mode branch emits the same
// `Run: cd <spec-worktree>` hint as the plan/review branches.
func TestFormatResult_ImplementIncludesCdHint(t *testing.T) {
	r := &Result{
		BeadID:          "bead-1",
		BeadClosed:      true,
		WorktreeRemoved: true,
		NextMode:        state.ModeImplement,
		NextBead:        "bead-2",
		NextSpec:        "008-test",
		SpecWorktree:    "/repo/.worktrees/worktree-spec-008-test",
	}
	out := FormatResult(r)
	want := "Run: `cd /repo/.worktrees/worktree-spec-008-test`"
	if !strings.Contains(out, want) {
		t.Errorf("implement branch should contain %q (spec 092 Req 4); got:\n%s", want, out)
	}

	// Without a removed worktree there is nothing to cd back from.
	r.WorktreeRemoved = false
	out = FormatResult(r)
	if strings.Contains(out, "Run: `cd") {
		t.Errorf("cd hint should be omitted when no worktree was removed; got:\n%s", out)
	}
}

// --- Spec 093 Req 2: ADR-divergence repair-first ladder ---

// TestADRDivergenceFailure_RepairFirstLadder forces the ADR-divergence
// gate through the real Run path (same fixture as TestOverrideUnblocks)
// and asserts the spec 093 Req 2 message contract: the repair-first
// triage ladder in the body (OWNERSHIP.yaml fix, then revert, bypass
// flags LAST), the offending file name carried by the findings, and the
// final `recovery:` lines carrying the re-run + bypass commands — the
// per-site mirror of the 092 Req 21 convention test.
func TestADRDivergenceFailure_RepairFirstLadder(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeADRDivergenceFixture(t, root, "093-test")
	stubPhaseEpic(t, "093-test", "epic-093")
	resolveTargetFn = func(r, flag string) (string, error) { return "093-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	// Source touch attributed to the uncovered "core" domain.
	mock.ChangedFilesResult = []string{"internal/core/foo.go"}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{AllowDocSkew: "test setup"})
	if err == nil {
		t.Fatal("expected adr-divergence failure, got nil")
	}
	msg := err.Error()

	// 092 Req 12/21: final recovery line, no banned commands.
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("adr-divergence failure must end with a recovery line: %q", msg)
	}
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, guard.RecoveryPrefix) && guard.IsBannedRecoveryCommand(strings.TrimPrefix(line, guard.RecoveryPrefix)) {
			t.Errorf("recovery line emits a banned command (092 Req 19): %q", line)
		}
	}

	// The findings carry the file name — the ladder is actionable
	// without any skill.
	if !strings.Contains(msg, "adr-divergence:") {
		t.Errorf("failure must keep the adr-divergence label: %q", msg)
	}
	if !strings.Contains(msg, "internal/core/foo.go") {
		t.Errorf("failure must name the offending file: %q", msg)
	}

	// Req 2 AC: repair-first ladder order — OWNERSHIP.yaml fix, then
	// revert, bypass flags LAST.
	iOwnership := strings.Index(msg, "OWNERSHIP.yaml")
	iRevert := strings.Index(msg, "revert it and re-run")
	iBypass := strings.Index(msg, "--override-adr")
	if iOwnership < 0 || iRevert < 0 || iBypass < 0 {
		t.Fatalf("ladder steps missing (OWNERSHIP.yaml=%d revert=%d bypass=%d): %q", iOwnership, iRevert, iBypass, msg)
	}
	if !(iOwnership < iRevert && iRevert < iBypass) {
		t.Errorf("ladder must be repair-first (OWNERSHIP.yaml < revert < bypass), got %d/%d/%d: %q", iOwnership, iRevert, iBypass, msg)
	}
	if !strings.Contains(msg, ".mindspec/docs/domains/<name>/OWNERSHIP.yaml") {
		t.Errorf("ladder must name the OWNERSHIP.yaml path: %q", msg)
	}

	// The bypass-first hint is gone.
	if strings.Contains(msg, "hint: re-run with") {
		t.Errorf("the pre-093 bypass-first hint must be gone: %q", msg)
	}

	// Recovery lines: re-run first, bypass commands last; the final
	// line is the --supersede-adr bypass (extracted command, not just a
	// substring).
	var recoveries []string
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, guard.RecoveryPrefix) {
			recoveries = append(recoveries, strings.TrimPrefix(line, guard.RecoveryPrefix))
		}
	}
	if len(recoveries) != 3 {
		t.Fatalf("want 3 recovery lines (re-run + 2 bypasses), got %d: %q", len(recoveries), msg)
	}
	if !strings.HasPrefix(recoveries[0], "mindspec complete bead-1 ") || strings.Contains(recoveries[0], "--") {
		t.Errorf("first recovery must be the plain re-run: %q", recoveries[0])
	}
	if want := `mindspec complete bead-1 --override-adr "<reason>"`; recoveries[1] != want {
		t.Errorf("second recovery = %q, want %q", recoveries[1], want)
	}
	if want := "mindspec complete bead-1 --supersede-adr ADR-NNNN"; recoveries[2] != want {
		t.Errorf("final recovery = %q, want %q", recoveries[2], want)
	}
	if got := finalRecoveryCommand(t, msg); got != "mindspec complete bead-1 --supersede-adr ADR-NNNN" {
		t.Errorf("finalRecoveryCommand = %q", got)
	}
}

// --- mindspec-aqey / mindspec-perm: per-bead gate anchoring tests ---
//
// The per-bead doc-sync + ADR-divergence gates must measure
// merge-base(specBranch, beadBranch)..beadBranch — the bead's own work
// — regardless of which checkout `mindspec complete` runs from. The
// old code measured MergeBase(specBranch, HEAD) and then diffed
// relative to the ambient checkout, which was wrong on BOTH sides:
// from the repo root the range was main-side drift (false blocks,
// mindspec-aqey — hit live twice on 2026-06-11 at spec-092 Bead 9);
// from the spec worktree the range was empty (vacuous passes,
// mindspec-perm — every spec-092 bead passed this way).

// TestPerBeadGatesAnchorOwnershipRefAndRangeToBeadHead is the spec 095
// regression lock: the per-bead gates anchor BOTH the diff range AND the
// OWNERSHIP-attribution ref to the bead fork/tip — never the ambient
// HEAD. It asserts every ChangedFiles call is fork-sha..bead/bead-1 AND
// every ref read (FileAtRefOrAbsent / TreeDirsAtRef) uses ref
// bead/bead-1. RED-on-revert: anchoring the range OR the ownership ref
// back to ambient HEAD changes a recorded arg and trips the assertion.
func TestPerBeadGatesAnchorOwnershipRefAndRangeToBeadHead(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeADRDivergenceFixture(t, root, "086-doc-sync") // core domain + spec/plan/ADR on disk
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "fork-sha"
	// A source touch in the BEAD diff so attribution (the ref reads)
	// actually runs; any other range is main-side drift.
	mock.ChangedFilesFn = func(base, head string) ([]string, error) {
		if base == "fork-sha" && head == "bead/bead-1" {
			return []string{"internal/core/foo.go"}, nil
		}
		return []string{"internal/contextpack/drift.go"}, nil
	}
	// Record the ref each ref-read seam is called with, then resolve
	// against the on-disk fixture.
	var refReads []string
	mock.FileAtRefOrAbsentFn = func(ref, rel string) ([]byte, bool, error) {
		refReads = append(refReads, ref)
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return data, true, nil
	}
	mock.TreeDirsAtRefFn = func(ref, dir string) ([]string, error) {
		refReads = append(refReads, ref)
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			return nil, nil
		}
		var dirs []string
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			}
		}
		return dirs, nil
	}

	// AllowDocSkew lets doc-sync fall through so the ADR-divergence lane
	// also runs its ref reads; the run may still block on uncovered core
	// (the fixture's ADR covers execution, not core) — that does not
	// matter here. We assert ONLY the anchoring of the recorded args.
	_, _ = Run(root, "bead-1", "", "", mock, CompleteOpts{AllowDocSkew: "anchor probe"})

	cfCalls := mock.CallsTo("ChangedFiles")
	if len(cfCalls) == 0 {
		t.Fatal("expected ChangedFiles calls from the gates")
	}
	for _, c := range cfCalls {
		if c.Args[0] != "fork-sha" || c.Args[1] != "bead/bead-1" {
			t.Errorf("ChangedFiles(%v, %v): per-bead range must be fork-sha..bead/bead-1, never ambient HEAD", c.Args[0], c.Args[1])
		}
	}
	if len(refReads) == 0 {
		t.Fatal("expected OWNERSHIP ref reads (FileAtRefOrAbsent / TreeDirsAtRef) from the gates")
	}
	for _, ref := range refReads {
		if ref != "bead/bead-1" {
			t.Errorf("OWNERSHIP ref read used %q; must anchor to bead/bead-1 (spec 095), never ambient HEAD", ref)
		}
	}
}

// TestPerBeadGatesAnchorToBeadFork_MainDriftDoesNotBlock pins the
// mindspec-aqey false-block case: the bead's own diff is clean, but
// every OTHER measurable range (the ambient HEAD / working-tree drift
// the old code measured) is full of doc-sync violations. The gates
// must pass, and every gate diff must be exactly
// fork-sha..bead/bead-1.
func TestPerBeadGatesAnchorToBeadFork_MainDriftDoesNotBlock(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	// Full spec fixture (spec.md + plan.md + cited ADR) so the
	// ADR-divergence lane RUNS through to its diff — without it,
	// ValidateDivergence no-ops on the missing spec.md and the
	// ChangedFiles assertions below would be vacuous for the ADR lane
	// (PR #132 panel C2 major / mutation M2b).
	writeADRDivergenceFixture(t, root, "086-doc-sync")
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	// Reuse-resolution path: the bead worktree exists and carries the
	// bead branch; beadHead must come from this entry.
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "fork-sha"
	mock.ChangedFilesFn = func(base, head string) ([]string, error) {
		if base == "fork-sha" && head == "bead/bead-1" {
			// The bead's own diff: clean (no files at all).
			return nil, nil
		}
		// ANY other range — notably the old working-tree-vs-base and
		// base..HEAD measurements — sees main-side drift that would
		// trip both gates (doc-less source change). If the gates
		// consult such a range, they false-block and this test fails.
		return []string{"README.md", "SECURITY.md", "internal/contextpack/drift.go"}, nil
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("gates must pass when the BEAD diff is clean despite main-side drift (mindspec-aqey), got: %v", err)
	}

	// The fork point must be computed against the bead branch, never HEAD.
	mbCalls := mock.CallsTo("MergeBase")
	if len(mbCalls) == 0 {
		t.Fatal("expected a MergeBase call for the per-bead gates")
	}
	for _, c := range mbCalls {
		if c.Args[0] != "spec/086-doc-sync" || c.Args[1] != "bead/bead-1" {
			t.Errorf("MergeBase(%v, %v): per-bead gates must anchor to (spec/086-doc-sync, bead/bead-1)", c.Args[0], c.Args[1])
		}
	}

	// Every gate diff must be the bead range — no ambient-HEAD or
	// working-tree measurement may remain. BOTH lanes must have
	// diffed: doc-sync (ValidateDocsRange) and ADR-divergence
	// (ValidateDivergence, reached via the fixture above) each issue
	// exactly one ChangedFiles call, so fewer than two means a lane
	// never measured anything and its anchoring is unpinned. Mutation
	// M2b (head→"HEAD" at the complete.go CheckADRDivergence call)
	// dies on the per-call args check.
	cfCalls := mock.CallsTo("ChangedFiles")
	if len(cfCalls) < 2 {
		t.Fatalf("expected ChangedFiles calls from BOTH gate lanes (doc-sync + adr-divergence), got %d", len(cfCalls))
	}
	for _, c := range cfCalls {
		if c.Args[0] != "fork-sha" || c.Args[1] != "bead/bead-1" {
			t.Errorf("ChangedFiles(%v, %v): per-bead gates must diff fork-sha..bead/bead-1 only", c.Args[0], c.Args[1])
		}
	}
}

// TestPerBeadGateHeadFallsBackToCanonicalBranch pins the e.Branch != ""
// guard in step 2: a worktree entry matched by NAME whose Branch field
// is empty (detached / unreported) must not blank out beadHead — the
// gates fall back to the canonical workspace.BeadBranch name.
func TestPerBeadGateHeadFallsBackToCanonicalBranch(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: ""},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mock := newMockExec()
	mock.MergeBaseResult = "fork-sha"

	if _, err := Run(root, "bead-1", "", "", mock, CompleteOpts{}); err != nil {
		t.Fatalf("clean bead diff must complete, got: %v", err)
	}
	mbCalls := mock.CallsTo("MergeBase")
	if len(mbCalls) == 0 {
		t.Fatal("expected a MergeBase call for the per-bead gates")
	}
	if mbCalls[0].Args[0] != "spec/086-doc-sync" || mbCalls[0].Args[1] != "bead/bead-1" {
		t.Errorf("MergeBase(%v, %v): empty worktree Branch must fall back to the canonical bead branch", mbCalls[0].Args[0], mbCalls[0].Args[1])
	}
}

// TestPerBeadGateMergeBaseErrorNamesRefs pins the failure path: a
// merge-base failure (e.g. the bead branch does not exist) surfaces
// with BOTH anchoring refs named, before any mutation.
func TestPerBeadGateMergeBaseErrorNamesRefs(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	var closed bool
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	mock := newMockExec()
	mock.MergeBaseErr = fmt.Errorf("fatal: not a valid ref: bead/bead-1")

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a merge-base failure to propagate")
	}
	want := "computing merge-base of spec/086-doc-sync and bead/bead-1 for the per-bead gates"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error must name both anchoring refs; want substring %q, got: %v", want, err)
	}
	if closed {
		t.Error("bead must not be closed on a merge-base failure")
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected no CompleteBead call on merge-base failure, got %d", len(calls))
	}
}

// TestPerBeadGatesFireDespiteEmptyAmbientRange pins the mindspec-perm
// vacuous-pass case: simulate the checkout where the OLD code measured
// an empty range (complete run from the spec worktree, HEAD == spec
// tip ⇒ ambient diff empty) while the bead's real diff carries a
// genuine doc-sync violation. The gates must FIRE.
func TestPerBeadGatesFireDespiteEmptyAmbientRange(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	// No worktree entry: beadHead falls back to the canonical
	// workspace.BeadBranch name.
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	var closed bool
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	mock := newMockExec()
	mock.MergeBaseResult = "fork-sha"
	mock.ChangedFilesFn = func(base, head string) ([]string, error) {
		if base == "fork-sha" && head == "bead/bead-1" {
			// The bead's real work: a doc-less source change — a
			// genuine doc-sync violation.
			return []string{"internal/contextpack/foo.go"}, nil
		}
		// Every other range is empty — exactly what the old code saw
		// from the spec worktree and passed vacuously on.
		return nil, nil
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("gates must fire on a real bead-diff violation even when the ambient range is empty (mindspec-perm)")
	}
	if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("error should name the doc-sync lane, got: %v", err)
	}
	if closed {
		t.Error("bead must not be closed when the gate blocks")
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected no CompleteBead call on gate block, got %d", len(calls))
	}
}

// TestPerBeadGateBlockKeepsRecoveryContract pins the honest case under
// the new anchoring: a violation IN the bead diff blocks completion
// with the existing recovery contract (the --allow-doc-skew override
// hint) and performs no terminal mutation.
func TestPerBeadGateBlockKeepsRecoveryContract(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	var closed bool
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	mock := newMockExec()
	mock.MergeBaseResult = "fork-sha"
	// Plain result: every range (there is only the bead range now)
	// carries the doc-less source change.
	mock.ChangedFilesResult = []string{"internal/contextpack/foo.go"}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the doc-sync gate to block a bead-diff violation")
	}
	if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("error should name the doc-sync lane, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--allow-doc-skew") {
		t.Errorf("block must keep the --allow-doc-skew recovery hint, got: %v", err)
	}
	if closed {
		t.Error("bead must not be closed when the gate blocks")
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected no CompleteBead call on gate block, got %d", len(calls))
	}
}
