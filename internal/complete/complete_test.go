package complete

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
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
	origResolveTarget := resolveTargetFn
	origFindLocalRoot := findLocalRootFn
	origFetchBeadByID := fetchBeadByIDFn
	origFetchBeadAsOf := fetchBeadAsOfFn
	origMergeMeta := completeMergeMetadataFn
	origGetMeta := completeGetMetadataFn
	origGitEmail := gitUserEmailFn
	origCheckDirty := checkDirtyTreeFn
	origGetwd := completeGetwdFn
	origPostCloseAttempts := postCloseReadAttempts
	origPostCloseBackoff := postCloseReadBackoff
	origDoltCommit := doltCommitFn
	origVerifyCommitted := verifyCommittedFn
	origFindOrphans := findOrphanedClosedBeadsFn
	origFindEpicForBead := findEpicForBeadFn
	origResolveSpecPrefix := resolveSpecPrefixFn
	origMergedUnclosed := mergedUnclosedFn
	origBeadScopeGetMeta := beadScopeGetMetadataFn
	origBeadScopeChangedFiles := beadScopeChangedFilesFn

	t.Cleanup(func() {
		findOrphanedClosedBeadsFn = origFindOrphans
		closeBeadFn = origClose
		worktreeListFn = origWtList
		runBDFn = origRunBD
		resolveTargetFn = origResolveTarget
		findLocalRootFn = origFindLocalRoot
		fetchBeadByIDFn = origFetchBeadByID
		fetchBeadAsOfFn = origFetchBeadAsOf
		completeMergeMetadataFn = origMergeMeta
		completeGetMetadataFn = origGetMeta
		gitUserEmailFn = origGitEmail
		checkDirtyTreeFn = origCheckDirty
		completeGetwdFn = origGetwd
		postCloseReadAttempts = origPostCloseAttempts
		postCloseReadBackoff = origPostCloseBackoff
		doltCommitFn = origDoltCommit
		verifyCommittedFn = origVerifyCommitted
		findEpicForBeadFn = origFindEpicForBead
		resolveSpecPrefixFn = origResolveSpecPrefix
		mergedUnclosedFn = origMergedUnclosed
		beadScopeGetMetadataFn = origBeadScopeGetMeta
		beadScopeChangedFilesFn = origBeadScopeChangedFiles
	})

	// Spec 089: phase.EnsureMigrated (wired into complete) shells to
	// `bd` via bead.MergeMetadata when the epic lacks mindspec_phase.
	// CI has no `bd` on PATH, so stub the seam to a no-op for the
	// duration of the test.
	restorePhaseMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		return nil
	})
	t.Cleanup(restorePhaseMerge)

	// Spec 107 wave 1 (mindspec-oexu.3): the post-close children read now runs
	// through internal/phase (phase.FetchChildren), and the spec→epic resolution
	// shares the same phase list-JSON seam. Default it to an empty result so a
	// Run test that reaches advanceState without an explicit phase stub stays
	// hermetic (no `bd` shell-out); stubPhaseEpic / stubChildrenByStatus / the
	// perf-budget test override it and restore back to this default (LIFO),
	// while this cleanup restores the production seam.
	restorePhaseList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restorePhaseList)

	// Default stubs
	resolveTargetFn = func(root, flag string) (string, error) { return "", fmt.Errorf("no active specs") }
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }
	// Spec 096 final-review (mindspec-2u0u): the default post-close re-read
	// AFFIRMS closed (case a) so happy-path tests get a verified close. The
	// close-verify tests override this to drive cases (b)/(c) and the
	// retry seam; the close-FAILURE tests override it to exercise the
	// already-closed-detection branch (closeBeadFn returns an error).
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "closed"}, nil
	}
	// Spec 086 Bead 3: keep metadata + git-identity reads inert by
	// default so the existing tests don't shell out to bd or git.
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error { return nil }
	// Spec 114 R2/Bead 2: default the metadata READ seam to a clean empty
	// map (no recorded refutation_pending) so reconcilePendingRefutations
	// no-ops for every pre-existing test (§6 fail-open preserved) — the
	// durable-obligation tests override this to drive pending/pending-fails
	// scenarios.
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
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
	// Spec 096 final-review (mindspec-2u0u): no-op the post-close re-read
	// backoff so no test ever sleeps. Tests that exercise the retry seam
	// set postCloseReadAttempts explicitly; the default count is left
	// intact here so existing single-read tests behave unchanged.
	postCloseReadBackoff = func(attempt int) {}
	// Spec 098 Req 2 (mindspec-9n2h): default the post-close durability
	// seams to SUCCESS so happy-path tests get a verified, durable close
	// (forced `bd dolt commit` no-op success; committed-state verify
	// confirms closed). The close-verify tests override these to drive the
	// commit-failure / committed-mismatch RED paths.
	doltCommitFn = func() error { return nil }
	verifyCommittedFn = func(beadID string) error { return nil }
	// bead mindspec-4gsz: keep the bd_close lifecycle-bypass guard inert by
	// default (no orphaned siblings) so existing happy-path tests don't shell
	// out to bd/git or false-block. The guard's own tests override this.
	findOrphanedClosedBeadsFn = func(specID, workdir, excludeBeadID string) []lifecycle.Orphan { return nil }
	// Spec 119 R1 (Bead 1): default the lineage-authoritative spec
	// resolution seam to a genuine "no lineage" result — the typed
	// phase.ErrNoEpicLineage sentinel (final-review finding A: a NON-
	// sentinel error now fails CLOSED instead of falling back) — so every
	// pre-existing test falls through to the (already-stubbed)
	// resolveTargetFn-based fallback path unchanged. The AC-1/AC-2
	// lineage tests override this to drive the new authoritative path;
	// the fail-closed tests override it with a real (non-sentinel) error.
	findEpicForBeadFn = func(beadID string) (string, string, error) {
		return "", "", fmt.Errorf("test: %w", phase.ErrNoEpicLineage)
	}
	// resolveSpecPrefixFn defaults to the real resolver: production hints
	// are almost always already-full spec IDs, which it passes through
	// with no bd call at all (only a bare numeric prefix would shell
	// out) — safe to leave live for every existing test.
	resolveSpecPrefixFn = resolve.ResolveSpecPrefix
	// Spec 119 R4 (Bead 1): default the landed-merge-commit-identity
	// predicate to "not merged-unclosed" so every pre-existing test's
	// worktree-matching behavior is unchanged. The reconcile-matrix tests
	// override this to drive the merged-unclosed / branch-less paths.
	mergedUnclosedFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, bool, error) {
		return nil, false, nil
	}
	// Spec 119 R11 (Bead 5): default the advisory bead-scope seams to a
	// clean empty-metadata read (no declared file_paths baseline — the
	// advisory check silently no-ops) so every pre-existing test stays
	// hermetic (no real `bd show` shell-out) and prints no unexpected
	// WARN. The AC-22 tests override beadScopeGetMetadataFn to drive the
	// cross-domain WARN.
	beadScopeGetMetadataFn = func(id string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
	beadScopeChangedFilesFn = func(exec executor.Executor, base, head string) ([]string, error) {
		return exec.ChangedFiles(base, head)
	}
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

// chdirIntoRoot points the process cwd at root for the test. Spec 107 wave 1
// (mindspec-oexu.3): advanceState now re-reads children via phase.FetchChildren,
// which derives the child-status breadth (bead.AllStatuses) from the cwd project
// root — mirroring complete.Run, which chdir's to the repo root before advancing
// state. The direct advanceState tests chdir into their fixture root so a custom
// status declared in the fixture's .beads/config.yaml is honored. saveAndRestore
// registers the cwd-restoring cleanup.
func chdirIntoRoot(t *testing.T, root string) {
	t.Helper()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir into fixture root: %v", err)
	}
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

// --- Spec 119 R1 (Bead 1): lineage-authoritative spec resolution (AC-1/AC-2) ---

// TestRun_LineageAuthoritative_IgnoresMisleadingCwdResolution (AC-1): the
// bead's parent-epic lineage resolves the spec authoritatively even when
// cwd-derived resolution (resolveTargetFn — standing in for "invoked from a
// DIFFERENT spec's worktree, or from a main checkout with ambiguous active
// specs") would answer differently. resolveTargetFn must never even be
// consulted. RED-on-revert: the pre-119 code tries resolveTargetFn FIRST and
// only falls back to bead lineage when it ERRORS — since this fixture makes
// it SUCCEED (with the wrong answer), reverting this bead's change makes
// resolveCalled become true and the downstream phase stubs (set up for
// "119-correct") no longer match, either flipping the assertion or erroring.
func TestRun_LineageAuthoritative_IgnoresMisleadingCwdResolution(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "119-correct", "epic-x")
	mock := newMockExec()

	findEpicForBeadFn = func(beadID string) (string, string, error) {
		return "epic-x", "119-correct", nil
	}
	var resolveCalled bool
	resolveTargetFn = func(r, flag string) (string, error) {
		resolveCalled = true
		return "119-wrong", nil // simulates a different spec's worktree / main
	}
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("lineage-authoritative resolution should succeed, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if resolveCalled {
		t.Error("cwd-based resolveTargetFn must NOT be consulted when the bead's lineage resolves authoritatively (AC-1)")
	}
}

// TestRun_LineageSpecHintMismatchRefuses (AC-2): an explicit --spec naming a
// DIFFERENT spec than the bead's lineage refuses in preflight, naming BOTH
// spec IDs, with ZERO executor calls (byte-identical state).
func TestRun_LineageSpecHintMismatchRefuses(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	findEpicForBeadFn = func(beadID string) (string, string, error) {
		return "epic-x", "119-correct", nil
	}

	_, err := Run(root, "bead-1", "119-wrong", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a refusal on --spec/lineage mismatch")
	}
	if !strings.Contains(err.Error(), "119-correct") || !strings.Contains(err.Error(), "119-wrong") {
		t.Errorf("error must name BOTH spec IDs, got: %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected ZERO executor calls before the preflight refusal, got %d: %+v", len(mock.Calls), mock.Calls)
	}
}

// TestRun_LineageSpecHintMatchProceeds (AC-2): a --spec hint that MATCHES
// the bead's lineage proceeds normally.
func TestRun_LineageSpecHintMatchProceeds(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "119-correct", "epic-x")
	mock := newMockExec()

	findEpicForBeadFn = func(beadID string) (string, string, error) {
		return "epic-x", "119-correct", nil
	}
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	result, err := Run(root, "bead-1", "119-correct", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("a matching --spec hint must proceed, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
}

// TestRun_LineageUnavailable_FallsBackToCwdResolution: when the lineage
// lookup SUCCEEDS but answers "no epic lineage" (e.g. the bead genuinely
// does not exist yet — the typed phase.ErrNoEpicLineage sentinel), the
// pre-119 cwd/hint-based resolution is still consulted — every pre-119
// resolution path is preserved for this degenerate case. (A real lookup
// ERROR, by contrast, fails closed — see the FailsClosed tests below.)
func TestRun_LineageUnavailable_FallsBackToCwdResolution(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	findEpicForBeadFn = func(beadID string) (string, string, error) {
		return "", "", fmt.Errorf("bead not found: %w", phase.ErrNoEpicLineage)
	}
	var resolveCalled bool
	resolveTargetFn = func(r, flag string) (string, error) {
		resolveCalled = true
		return "008-test", nil
	}
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("fallback resolution must succeed, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if !resolveCalled {
		t.Error("expected the cwd-based fallback resolveTargetFn to be consulted when lineage is unavailable")
	}
}

// TestRun_LineageLookupErrorFailsClosed (spec 119 final-review finding A):
// a REAL lineage-lookup error — a transient bd/Dolt failure, NOT the typed
// phase.ErrNoEpicLineage "genuinely epic-less" sentinel — must refuse
// fail-closed BEFORE the migration and any mutation, with a named retry
// recovery line. It must NEVER silently fall back to cwd-derived
// resolution (which would reintroduce the zty3/R2 wrong-spec bug on a
// transient failure).
func TestRun_LineageLookupErrorFailsClosed(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	findEpicForBeadFn = func(beadID string) (string, string, error) {
		return "", "", fmt.Errorf("bd show %s failed: dolt lock contention", beadID)
	}
	var resolveCalled bool
	resolveTargetFn = func(r, flag string) (string, error) {
		resolveCalled = true
		return "119-wrong", nil
	}
	var migrationCalled bool
	restorePhaseMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		migrationCalled = true
		return nil
	})
	t.Cleanup(restorePhaseMerge)

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a fail-closed refusal on a transient lineage-lookup error")
	}
	if !strings.Contains(err.Error(), "epic lineage") {
		t.Errorf("refusal should name the lineage failure, got: %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("fail-closed lineage refusal must end with a recovery line, got: %v", err)
	}
	if resolveCalled {
		t.Error("cwd-based resolveTargetFn must NEVER be consulted on a lineage-lookup ERROR (fail-closed, not fail-open)")
	}
	if migrationCalled {
		t.Error("the refusal must fire BEFORE the ADR-0034 migration (no metadata write)")
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected ZERO executor calls (zero mutations) on the fail-closed refusal, got %d: %+v", len(mock.Calls), mock.Calls)
	}
}

// TestRun_LineageDependentEpicLookupErrorFailsClosed (finding A, inner
// swallow): a failure while resolving the bead's DEPENDENT parent epic
// (phase.FindEpicForBead's cache.FindEpic leg — previously swallowed into
// "no epic found") now propagates as a real error, and complete fails
// closed on it identically: zero mutations, resolveTargetFn never called.
func TestRun_LineageDependentEpicLookupErrorFailsClosed(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	findEpicForBeadFn = func(beadID string) (string, string, error) {
		// The exact wrapped shape phase.FindEpicForBeadWithCache returns
		// when the parent-epic `bd show` fails (NOT the no-lineage sentinel).
		return "", "", fmt.Errorf("resolving parent epic epic-x for bead %s: bd show epic-x failed: connection refused", beadID)
	}
	var resolveCalled bool
	resolveTargetFn = func(r, flag string) (string, error) {
		resolveCalled = true
		return "119-wrong", nil
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a fail-closed refusal on a dependent-epic lookup error")
	}
	if resolveCalled {
		t.Error("cwd-based resolveTargetFn must NEVER be consulted on a dependent-epic lookup ERROR")
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected ZERO executor calls on the fail-closed refusal, got %d: %+v", len(mock.Calls), mock.Calls)
	}
}

// TestCompleteRunMalformedLineageRefusesConvergently is spec 120 AC-4
// (R2/D1 x spec-119 lineage): with findEpicForBeadFn stubbed to RETURN the
// new malformed-metadata error (the value the D1-checked derivation
// produces for a hostile-titled epic — the stub CANNOT hold "an epic",
// round-3 F2), Run refuses BEFORE any mutation (no executor call
// recorded), the error is NOT errors.Is(…, phase.ErrNoEpicLineage) (no cwd
// fallback), the refusal text is clean by the triple with the hostile
// value escaped-only, and the final recovery line names
// `mindspec repair spec-title <epic-id> …` with a validated epic ID. The
// test then APPLIES the lever (re-stubs to return a valid specID,
// modelling the repaired title), re-runs, and asserts preflight passes —
// convergence proven at execution level.
func TestCompleteRunMalformedLineageRefusesConvergently(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	const epicID = "mindspec-hostile-epic"
	const beadID = "mindspec-hostile-epic.1"

	malformedErr := fmt.Errorf("%w: %v", phase.ErrMalformedSpecMetadata, "invalid spec ID")
	findEpicForBeadFn = func(bid string) (string, string, error) {
		return epicID, "", malformedErr
	}
	var resolveCalled bool
	resolveTargetFn = func(r, flag string) (string, error) {
		resolveCalled = true
		return "119-wrong", nil
	}

	_, err := Run(root, beadID, "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a fail-closed refusal on malformed epic-lineage metadata")
	}
	if errors.Is(err, phase.ErrNoEpicLineage) {
		t.Errorf("malformed-metadata error must NOT be errors.Is ErrNoEpicLineage (no cwd fallback), got: %v", err)
	}
	if resolveCalled {
		t.Error("cwd-based resolveTargetFn must NEVER be consulted on malformed epic-lineage metadata")
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected ZERO executor calls on the malformed-lineage refusal, got %d: %+v", len(mock.Calls), mock.Calls)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("malformed-lineage refusal must end with a recovery line, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mindspec repair spec-title "+epicID) {
		t.Errorf("refusal must name the repair spec-title lever with the validated epic ID, got: %v", err)
	}
	assertCleanText(t, err.Error())

	// Apply the lever: re-stub findEpicForBeadFn as if the title had been
	// repaired to a valid one, modelling `mindspec repair spec-title`
	// having run. The re-run's preflight must now pass (proceed past the
	// lineage step) instead of refusing again.
	findEpicForBeadFn = func(bid string) (string, string, error) {
		return epicID, "120-repaired-title", nil
	}
	resolveTargetFn = func(r, flag string) (string, error) {
		resolveCalled = true
		return "120-repaired-title", nil
	}
	_, err2 := Run(root, beadID, "", "", mock, CompleteOpts{})
	if err2 != nil && strings.Contains(err2.Error(), "malformed spec metadata") {
		t.Errorf("re-run after applying the repair lever must not still refuse on malformed metadata, got: %v", err2)
	}
}

// TestCompleteRunMalformedLineage_HostileEpicIDEmbedsPlaceholder is the
// AC-6 epic-ID-embed subtest: when the lineage lookup's own returned
// epicID is ITSELF malformed (a hostile bd-sourced value — bd ids are
// agent-writable, round 9), the refusal must never embed it raw in an
// executable recovery line — it falls back to the "<epic-id>" placeholder
// precedent (derive.go's specIDForEpicWithCache), same discipline this
// package's other recovery lines already apply.
func TestCompleteRunMalformedLineage_HostileEpicIDEmbedsPlaceholder(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	const hostileEpicID = "x;evil"
	malformedErr := fmt.Errorf("%w: %v", phase.ErrMalformedSpecMetadata, "invalid spec ID")
	findEpicForBeadFn = func(bid string) (string, string, error) {
		return hostileEpicID, "", malformedErr
	}

	_, err := Run(root, "mindspec-x.1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a refusal on malformed epic-lineage metadata")
	}
	if strings.Contains(err.Error(), hostileEpicID) {
		t.Errorf("refusal must NEVER embed the hostile epic id raw, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mindspec repair spec-title <epic-id>") {
		t.Errorf("refusal must fall back to the <epic-id> placeholder, got: %v", err)
	}
}

// TestCompleteRunRejectsInvalidBeadIDArg is spec 120 AC-6 (R3 beadID
// ingress): a malformed beadID argument to complete.Run refuses with the
// `bd ready` lever, BEFORE any lineage lookup (findEpicForBeadFn is never
// consulted) or mutation; a dotted-child bead ID is accepted (reaches the
// lineage lookup unchanged).
func TestCompleteRunRejectsInvalidBeadIDArg(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	hostileBeadIDs := []string{
		"--help",
		"x;evil",
		"x\x00\x1b[31m\nrecovery: forged",
	}
	for _, hostile := range hostileBeadIDs {
		var lineageCalled bool
		findEpicForBeadFn = func(bid string) (string, string, error) {
			lineageCalled = true
			return "", "", fmt.Errorf("test: %w", phase.ErrNoEpicLineage)
		}
		_, err := Run(root, hostile, "", "", mock, CompleteOpts{})
		if err == nil {
			t.Errorf("Run(%q) accepted a hostile bead ID", hostile)
			continue
		}
		if lineageCalled {
			t.Errorf("Run(%q): lineage lookup must never run for a malformed beadID", hostile)
		}
		if len(mock.Calls) != 0 {
			t.Errorf("Run(%q): expected ZERO executor calls, got %d", hostile, len(mock.Calls))
		}
		if !guard.HasFinalRecoveryLine(err.Error()) {
			t.Errorf("Run(%q): refusal must end with a recovery line, got: %v", hostile, err)
		}
		if !strings.Contains(err.Error(), "bd ready") {
			t.Errorf("Run(%q): refusal must name the `bd ready` lever, got: %v", hostile, err)
		}
		assertCleanText(t, err.Error())
	}

	// A dotted-child bead ID is accepted (reaches the lineage lookup).
	var lineageCalled bool
	findEpicForBeadFn = func(bid string) (string, string, error) {
		lineageCalled = true
		return "", "", fmt.Errorf("test: %w", phase.ErrNoEpicLineage)
	}
	resolveTargetFn = func(r, flag string) (string, error) { return "", fmt.Errorf("no active specs") }
	_, _ = Run(root, "mindspec-9cyu.1", "", "", mock, CompleteOpts{})
	if !lineageCalled {
		t.Error("Run(mindspec-9cyu.1) should reach the lineage lookup (clean dotted-child bead ID)")
	}
}

// TestRun_DocSyncRefusesAfterCommitAllBeforeTerminalMutations (spec 119
// final-review finding B): the call-order contract of the artifact-
// materialization subphase (ADR-0041 §1). A doc-sync refusal fires AFTER
// the optional user CommitAll (the local bead-branch commit whose tip the
// gate must measure) but BEFORE every lifecycle-affecting mutation: no
// `bd close`, no CompleteBead (merge/branch/worktree cleanup).
func TestRun_DocSyncRefusesAfterCommitAllBeforeTerminalMutations(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "086-doc-sync", "epic-086")
	resolveTargetFn = func(r, flag string) (string, error) { return "086-doc-sync", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	var closeCalled bool
	closeBeadFn = func(ids ...string) error { closeCalled = true; return nil }

	mock := newMockExec()
	mock.MergeBaseResult = "merge-base-sha"
	// Source-only diff with no doc updates → doc-sync SevError.
	mock.ChangedFilesResult = []string{"internal/contextpack/foo.go"}

	_, err := Run(root, "bead-1", "", "did the work", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the doc-sync gate to refuse")
	}
	if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("error should mention doc-sync: %v", err)
	}
	// AFTER CommitAll: the user commit was materialized (forward-
	// reconcilable local bead-branch commit, retained on refusal).
	if calls := mock.CallsTo("CommitAll"); len(calls) != 1 {
		t.Errorf("expected exactly 1 CommitAll (artifact materialization) BEFORE the doc-sync refusal, got %d", len(calls))
	}
	// BEFORE every lifecycle-affecting mutation.
	if closeCalled {
		t.Error("bd close must NOT run after a doc-sync refusal")
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("expected ZERO CompleteBead (merge/cleanup) calls after a doc-sync refusal, got %d", len(calls))
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

// stubChildrenByStatus installs a phase-seam listJSON stub that returns children
// keyed by status. Spec 107 wave 1 (mindspec-oexu.3): the children query now runs
// through internal/phase (complete.advanceState → phase.FetchChildren, and the
// impl-only guard → phase.DerivePhase), so this stubs phase.SetListJSONForTest
// rather than complete's own seam. It COMPOSES onto whatever phase stub is already
// installed (typically stubPhaseEpic): `--parent` children queries are served
// here, and every other query (`--type=epic` epic resolution, `bd show`) is
// delegated back to the previous stub so both concerns share the one seam. The
// query is a single `--status=<comma-joined>` call, so the stub returns the union
// of all requested-status buckets (each stamped with its bucket's status); any
// status not in the map contributes no items. Callers pair this with stubPhaseEpic,
// whose t.Cleanup restores the seam to the saveAndRestore default.
func stubChildrenByStatus(byStatus map[string][]bead.BeadInfo) {
	prev := phase.ListJSONForTest()
	phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		isParent := false
		for _, a := range args {
			if a == "--parent" {
				isParent = true
				break
			}
		}
		if !isParent {
			// Epic resolution (--type=epic) and other queries stay with the
			// stub stubPhaseEpic installed.
			return prev(args...)
		}
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
	})
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

	chdirIntoRoot(t, root)
	mode, nextBead := advanceState("epic-123")
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

	chdirIntoRoot(t, root)
	mode, nextBead := advanceState("epic-123")
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

	chdirIntoRoot(t, root)
	mode, nextBead := advanceState("epic-123")
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

	chdirIntoRoot(t, root)
	mode, nextBead := advanceState("epic-123")
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

	chdirIntoRoot(t, root)
	mode, _ := advanceState("epic-123")
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

	chdirIntoRoot(t, root)
	mode, _ := advanceState("epic-123")
	// Only the `closed` bead is seen → all closed → review.
	if mode != state.ModeReview {
		t.Errorf("undeclared custom status must not be iterated: got mode %q, want %q", mode, state.ModeReview)
	}
}

func TestAdvanceState_NoEpic(t *testing.T) {
	saveAndRestore(t)

	setupTempRoot(t)
	// Empty epicID (spec→epic resolution found nothing) → idle. Spec 107 wave 1:
	// advanceState now takes the epicID resolved once by Run; "" short-circuits
	// to idle without any bd query (ADR-0023: no lifecycle.yaml needed).
	mode, nextBead := advanceState("")
	if mode != state.ModeIdle {
		t.Errorf("mode: got %q, want %q", mode, state.ModeIdle)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

// TestRun_CompletePerfPairSubprocessBudget is the spec 107 wave 1
// (mindspec-oexu.3) perf-pair contract at the Run level. It drives a full
// complete over a single phase list-JSON seam that serves BOTH the spec→epic
// resolution (`--type=epic`) and the children query (`--parent`), and asserts:
//
//   - AC5: exactly ONE `bd list --parent` for the post-close children query
//     (was ~5 per-status calls in the old queryAllChildren loop).
//   - AC6: the immutable spec→epic mapping is resolved once — at most one
//     `bd list --type=epic` across the whole run (migrate + guard + phase-sync +
//     advanceState previously each built a throwaway cache).
//   - Freshness: the post-close read reflects bd state mutated MID-RUN. The
//     stub flips to an all-closed child set inside closeBeadFn; a memoized /
//     pre-close read would derive implement, so review proves the read is fresh.
func TestRun_CompletePerfPairSubprocessBudget(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	mock := newMockExec()

	const specID = "107-perf"
	const epicID = "epic-107"

	resolveTargetFn = func(r, flag string) (string, error) { return specID, nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	var (
		closed               bool
		epicListCalls        int
		parentCallsPostClose int
	)

	// Mid-run mutation: from the close onward, the epic's child set is
	// all-closed. The post-close children read must observe this (review),
	// not the pre-close in_progress view (implement).
	closeBeadFn = func(ids ...string) error {
		closed = true
		return nil
	}

	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		isEpic, isParent := false, false
		for _, a := range args {
			switch a {
			case "--type=epic":
				isEpic = true
			case "--parent":
				isParent = true
			}
		}
		switch {
		case isEpic:
			epicListCalls++
			return json.Marshal([]phase.EpicInfo{{
				ID: epicID, Title: "[SPEC " + specID + "] Perf", Status: "open", IssueType: "epic",
				Metadata: map[string]interface{}{"spec_num": float64(107), "spec_title": "perf"},
			}})
		case isParent:
			if closed {
				parentCallsPostClose++
				return json.Marshal([]phase.ChildInfo{
					{ID: "mindspec-b1", Title: "[IMPL 107-perf.1] Done", Status: "closed", IssueType: "task"},
				})
			}
			return json.Marshal([]phase.ChildInfo{
				{ID: "mindspec-b1", Title: "[IMPL 107-perf.1] WIP", Status: "in_progress", IssueType: "task"},
			})
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)

	// bd show for the epic (FindEpic → stored-phase read): no mindspec_phase,
	// so derivation falls through to the child set.
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 1 && args[0] == "show" {
			return json.Marshal([]phase.EpicInfo{{ID: epicID, Status: "open"}})
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	// advanceState issues `bd ready` only in implement; the post-close phase
	// derives review, so this is defensive.
	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{})
	}

	result, err := Run(root, "mindspec-b1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !closed {
		t.Fatal("closeBeadFn was never invoked — the mid-run mutation never fired")
	}
	// AC5: exactly one bd list --parent for the post-close children query.
	if parentCallsPostClose != 1 {
		t.Errorf("post-close children query must issue exactly one `bd list --parent`, got %d", parentCallsPostClose)
	}
	// AC6: spec→epic resolved once — at most one bd list --type=epic.
	if epicListCalls > 1 {
		t.Errorf("spec→epic resolution must issue at most one `bd list --type=epic`, got %d", epicListCalls)
	}
	// Freshness: the post-close read reflected the mid-run all-closed mutation.
	if result.NextMode != state.ModeReview {
		t.Errorf("post-close children read must reflect bd state mutated mid-run (want %q), got %q", state.ModeReview, result.NextMode)
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

// --- Spec 096 Bead 2 (mindspec-2u0u): close-leg persistence verification ---
//
// After closeBeadFn returns nil, complete.Run re-reads the bead status
// and decides across three cases:
//   (a) re-read affirms closed       → proceed, BeadClosed: true
//   (b) re-read affirms open/in_prog → HARD error (silent close-loss)
//   (c) re-read itself errors         → tolerate, warn, proceed

// (b) The headline RED proof: closeBeadFn returns nil but the re-read
// AFFIRMS the bead is still in_progress (fetchErr == nil). This is the
// real silent close-loss bug — complete MUST return a non-zero error and
// MUST NOT report BeadClosed: true. RED if reverted to the unconditional
// BeadClosed: true.
func TestRun_CloseReturnsNilButReReadShowsInProgress(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "096-lifecycle", "epic-096")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "096-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	// Close "succeeds" but never persisted.
	closeBeadFn = func(ids ...string) error { return nil }
	// Re-read SUCCEEDS (fetchErr == nil) and shows the bead still open.
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "in_progress"}, nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a non-zero error when close returned nil but the re-read affirms the bead is still in_progress (silent close-loss)")
	}
	if !strings.Contains(err.Error(), "did NOT persist") {
		t.Errorf("error should name the silent close-loss, got: %v", err)
	}
	if result != nil && result.BeadClosed {
		t.Errorf("BeadClosed must NOT be true on an unpersisted close, got result=%+v", result)
	}
}

// (c) The closeverify finding: closeBeadFn returns nil but the re-read
// FETCH errors on EVERY bounded attempt (a genuine close-loss correlated
// with persistent Dolt lock contention). The OLD behavior tolerated this
// and proceeded to merge + worktree removal on an UNVERIFIED close —
// exit 0 with the bead potentially still in_progress, re-exposing the
// silent close-loss class. The fix: a RECOVERABLE soft-block — non-zero
// error, worktree KEPT (CompleteBead never called), BeadClosed NOT set.
// RED on revert to the old tolerate+proceed (which would return err==nil
// and call CompleteBead). Drives the retry loop the full count.
func TestRun_CloseReturnsNilButReReadPersistentlyErrors(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "096-lifecycle", "epic-096")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "096-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	closeBeadFn = func(ids ...string) error { return nil }
	// Re-read ERRORS on every attempt — the read never verifies.
	var reads int
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		reads++
		return next.BeadInfo{}, fmt.Errorf("dolt: persistent read lock contention")
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("a persistent post-close re-read failure must NOT silently proceed to merge — expected a recoverable error")
	}
	if !strings.Contains(err.Error(), "could not be VERIFIED") {
		t.Errorf("error should name the unverified close, got: %v", err)
	}
	// Recoverable: an ADR-0035 `recovery:` line must be present.
	if !strings.Contains(err.Error(), "recovery: ") {
		t.Errorf("error must carry a recovery line (recoverable soft-block), got: %v", err)
	}
	if result != nil && result.BeadClosed {
		t.Errorf("BeadClosed must NOT be set on an unverified close, got result=%+v", result)
	}
	// The worktree must be KEPT: the bead→spec merge step must not run.
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("CompleteBead (merge + worktree removal) must NOT run on an unverified close, got %d calls", len(calls))
	}
	// The retry seam must have been exercised the full bounded count.
	if reads != postCloseReadAttempts {
		t.Errorf("expected %d bounded re-read attempts, got %d", postCloseReadAttempts, reads)
	}
}

// transient-then-closed: the re-read errors on the first attempt(s) and
// then returns "closed" — a transient Dolt lock that clears. complete
// MUST converge to case (a) and PROCEED (no false-block). RED if the
// retry is removed: a single error would wrongly trip the case-(c)
// soft-block on a genuinely-closed bead.
func TestRun_CloseReReadTransientThenClosed(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "096-lifecycle", "epic-096")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "096-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	closeBeadFn = func(ids ...string) error { return nil }
	var reads int
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		reads++
		if reads < 2 {
			return next.BeadInfo{}, fmt.Errorf("dolt: transient read timeout")
		}
		return next.BeadInfo{ID: id, Status: "closed"}, nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("a transient re-read error that resolves to closed must NOT false-block completion, got: %v", err)
	}
	if result == nil || !result.BeadClosed {
		t.Errorf("BeadClosed must be true once the retried re-read confirms closed, got result=%+v", result)
	}
	if reads < 2 {
		t.Errorf("expected the re-read to be retried past the first transient error, got %d reads", reads)
	}
}

// transient-then-in_progress: the re-read errors first, then returns
// in_progress — the real silent close-loss surfacing AFTER a transient
// hiccup. complete MUST converge to case (b) and HARD-fail (the bug
// caught), never tolerate it.
func TestRun_CloseReReadTransientThenInProgress(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "096-lifecycle", "epic-096")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "096-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	closeBeadFn = func(ids ...string) error { return nil }
	var reads int
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		reads++
		if reads < 2 {
			return next.BeadInfo{}, fmt.Errorf("dolt: transient read timeout")
		}
		return next.BeadInfo{ID: id, Status: "in_progress"}, nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("a re-read that resolves to in_progress after a transient error must HARD-fail (silent close-loss)")
	}
	if !strings.Contains(err.Error(), "did NOT persist") {
		t.Errorf("error should name the silent close-loss, got: %v", err)
	}
	if result != nil && result.BeadClosed {
		t.Errorf("BeadClosed must NOT be true on an unpersisted close, got result=%+v", result)
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("CompleteBead must NOT run when the close did not persist, got %d calls", len(calls))
	}
}

// (a) No-regression: closeBeadFn returns nil AND the re-read affirms
// closed → completion proceeds normally with BeadClosed: true.
func TestRun_CloseReturnsNilAndReReadConfirmsClosed(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "096-lifecycle", "epic-096")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "096-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	closeBeadFn = func(ids ...string) error { return nil }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		// Mixed-case + whitespace to pin the EqualFold/TrimSpace predicate.
		return next.BeadInfo{ID: id, Status: " Closed "}, nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("a confirmed-closed re-read must complete, got: %v", err)
	}
	if result == nil || !result.BeadClosed {
		t.Errorf("BeadClosed must be true on a confirmed-closed re-read, got result=%+v", result)
	}
}

// --- Spec 098 Bead 2 (mindspec-9n2h): forced `bd dolt commit` + committed-
// state verify after the session re-read affirms closed (the 2u0u recurrence)
// ---
//
// Even when closeBeadFn returns nil AND the session re-read says "closed"
// (case (a)), that session read does NOT prove the close PERSISTED. complete
// MUST force `bd dolt commit` (durability) and then a committed-state verify
// before proceeding to merge. On a doltCommit failure OR a committed-mismatch,
// it returns a recoverable ADR-0035 error, keeps the worktree (CompleteBead
// 0×), and leaves BeadClosed unset — never `closed` + exit 0.

// TestRun_VerifyCommittedNotClosedBlocks is the headline RED-on-revert proof:
// closeBeadFn→nil, the session re-read fetchBeadByIDFn→"closed" (the case-(a)
// affirm path), doltCommitFn→ok, but verifyCommittedFn→not-closed. Reverting
// the new forced-commit + committed-verify step makes this case (a) proceed
// (err==nil, CompleteBead called) — the unverified close slips through. With
// the step present it MUST hard-fail recoverably.
func TestRun_VerifyCommittedNotClosedBlocks(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "098-lifecycle", "epic-098")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "098-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	closeBeadFn = func(ids ...string) error { return nil }
	// Session re-read AFFIRMS closed (case (a)).
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "closed"}, nil
	}
	// Forced commit succeeds, but the committed-state verify shows NOT closed.
	doltCommitFn = func() error { return nil }
	verifyCommittedFn = func(beadID string) error {
		return fmt.Errorf("committed-state re-read shows status %q (not closed)", "in_progress")
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("a session re-read of closed that fails committed-state verification must NOT proceed — expected a recoverable error (the 2u0u recurrence)")
	}
	if !strings.Contains(err.Error(), "did NOT confirm the close persisted") {
		t.Errorf("error should name the unverified committed close, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery: ") {
		t.Errorf("error must carry a recovery line (recoverable soft-block), got: %v", err)
	}
	if got := finalRecoveryCommand(t, err.Error()); got != "mindspec complete bead-1" {
		t.Errorf("recovery command: got %q, want %q", got, "mindspec complete bead-1")
	}
	if result != nil && result.BeadClosed {
		t.Errorf("BeadClosed must NOT be set on an unverified close, got result=%+v", result)
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("CompleteBead (merge + worktree removal) must NOT run on an unverified close, got %d calls", len(calls))
	}
}

// TestRun_DoltCommitFailureBlocks: closeBeadFn→nil, session re-read→"closed",
// but the forced `bd dolt commit` (durability step) ERRORS. complete MUST
// hard-fail recoverably, keep the worktree, and leave BeadClosed unset — never
// proceed to merge on a close whose durability could not be forced.
func TestRun_DoltCommitFailureBlocks(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "098-lifecycle", "epic-098")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "098-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	closeBeadFn = func(ids ...string) error { return nil }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "closed"}, nil
	}
	doltCommitFn = func() error { return fmt.Errorf("bd dolt commit failed: dolt: write lock contention") }
	// verifyCommittedFn must NOT even be reached when the commit fails.
	var verifyCalled bool
	verifyCommittedFn = func(beadID string) error { verifyCalled = true; return nil }

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("a forced `bd dolt commit` failure after close must NOT proceed — expected a recoverable error")
	}
	if !strings.Contains(err.Error(), "force") && !strings.Contains(err.Error(), "DURABLE") {
		t.Errorf("error should name the failed durability commit, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery: ") {
		t.Errorf("error must carry a recovery line, got: %v", err)
	}
	if verifyCalled {
		t.Error("verifyCommittedFn must NOT run when the forced commit fails")
	}
	if result != nil && result.BeadClosed {
		t.Errorf("BeadClosed must NOT be set when the forced commit fails, got result=%+v", result)
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("CompleteBead must NOT run when the forced commit fails, got %d calls", len(calls))
	}
}

// TestRun_DoltCommitAndVerifyHappyPath: closeBeadFn→nil, session re-read→
// "closed", doltCommitFn→ok, verifyCommittedFn→ok ⇒ completion proceeds with
// BeadClosed: true and the merge runs (CompleteBead called). Confirms the new
// durability gate does not false-block a genuinely-durable close, and pins the
// ordering (commit BEFORE verify).
func TestRun_DoltCommitAndVerifyHappyPath(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "098-lifecycle", "epic-098")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "098-lifecycle", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	closeBeadFn = func(ids ...string) error { return nil }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "closed"}, nil
	}
	var commitCalled, verifyCalled bool
	doltCommitFn = func() error { commitCalled = true; return nil }
	verifyCommittedFn = func(beadID string) error {
		if !commitCalled {
			t.Error("verifyCommittedFn must run AFTER the forced `bd dolt commit`")
		}
		verifyCalled = true
		return nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("a durable, committed-verified close must complete, got: %v", err)
	}
	if result == nil || !result.BeadClosed {
		t.Errorf("BeadClosed must be true on a durable, verified close, got result=%+v", result)
	}
	if !commitCalled || !verifyCalled {
		t.Errorf("both the forced commit and the committed verify must run (commit=%v verify=%v)", commitCalled, verifyCalled)
	}
}

// TestDoltCommitFnDefaultsToBeadDoltCommit pins the production binding of the
// spec 098 Req 2 durability seam to bead.DoltCommit — every test swaps the
// seam in saveAndRestore, so a severed default would go undetected without
// this identity pin (pattern: TestCheckDirtyTreeFnDefaultsToNextCheckDirtyTreeDetail).
func TestDoltCommitFnDefaultsToBeadDoltCommit(t *testing.T) {
	if reflect.ValueOf(doltCommitFn).Pointer() != reflect.ValueOf(bead.DoltCommit).Pointer() {
		t.Fatal("doltCommitFn must default to bead.DoltCommit (spec 098 Req 2, mindspec-9n2h)")
	}
}

// TestVerifyCommittedFnDefaultsToDefault pins the production binding of the
// committed-state verifier seam.
func TestVerifyCommittedFnDefaultsToDefault(t *testing.T) {
	if reflect.ValueOf(verifyCommittedFn).Pointer() != reflect.ValueOf(defaultVerifyCommitted).Pointer() {
		t.Fatal("verifyCommittedFn must default to defaultVerifyCommitted (spec 098 Req 2, mindspec-9n2h)")
	}
}

// TestFetchBeadAsOfFnDefaultsToNextFetchBeadAsOf pins the production
// binding of the committed-state read seam introduced by bead mindspec-uopd.
func TestFetchBeadAsOfFnDefaultsToNextFetchBeadAsOf(t *testing.T) {
	if reflect.ValueOf(fetchBeadAsOfFn).Pointer() != reflect.ValueOf(next.FetchBeadAsOf).Pointer() {
		t.Fatal("fetchBeadAsOfFn must default to next.FetchBeadAsOf (bead mindspec-uopd)")
	}
}

// fakeBDExitError synthesizes a genuine *exec.ExitError whose Stderr field
// carries stderrMsg, mirroring what os/exec.Cmd.Output() produces when a
// real `bd` subprocess exits non-zero (the same code path RunBD/tracedOutput
// uses in production). Building a REAL *exec.ExitError — rather than a
// hand-rolled fake — exercises bead.IsUnsupportedFlagError's errors.As
// type-switch exactly as it runs against a real bd failure.
func fakeBDExitError(t *testing.T, stderrMsg string) error {
	t.Helper()
	cmd := exec.Command("sh", "-c", `printf '%s' "$FAKE_BD_STDERR" 1>&2; exit 1`)
	cmd.Env = append(os.Environ(), "FAKE_BD_STDERR="+stderrMsg)
	_, err := cmd.Output()
	if err == nil {
		t.Fatal("fakeBDExitError: expected the synthetic command to fail")
	}
	return err
}

// captureStderr redirects os.Stderr for the duration of fn and returns
// whatever fn wrote to it. Used to assert on the mindspec-uopd graceful-
// degradation warning line without depending on a test-only writer seam
// (defaultVerifyCommitted intentionally writes straight to os.Stderr, like
// the neighboring HC-3 self-heal line at step 3 of Run).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stderr: %v", err)
	}
	return buf.String()
}

// TestDefaultVerifyCommitted table-drives the four mindspec-uopd cases: the
// committed-state (--as-of HEAD) read is now the PRIMARY verifier, with a
// graceful fallback to the pre-existing same-read path (fetchBeadByIDFn)
// only when the --as-of invocation fails with bd's unknown-flag signature
// (an older, pre-1.0.4 bd). A hard read failure or a not-closed status in
// EITHER path must still error — this is a direct unit test of
// defaultVerifyCommitted, not routed through Run/verifyCommittedFn (every
// Run-level test swaps verifyCommittedFn wholesale and never exercises this
// function's internals).
func TestDefaultVerifyCommitted(t *testing.T) {
	origAsOf := fetchBeadAsOfFn
	origFetch := fetchBeadByIDFn
	t.Cleanup(func() {
		fetchBeadAsOfFn = origAsOf
		fetchBeadByIDFn = origFetch
	})

	const downgradeMarker = "event=complete.committed_read_downgraded"

	tests := []struct {
		name           string
		asOfFn         func(id, ref string) (next.BeadInfo, error)
		sameReadFn     func(id string) (next.BeadInfo, error)
		wantErr        bool
		wantErrSubstr  string
		wantDowngraded bool
	}{
		{
			// (a) --as-of read shows closed → pass. The same-read fallback
			// must NEVER be consulted when --as-of itself succeeds.
			name: "as-of read closed passes without fallback",
			asOfFn: func(id, ref string) (next.BeadInfo, error) {
				if ref != "HEAD" {
					t.Errorf("as-of ref = %q, want %q", ref, "HEAD")
				}
				return next.BeadInfo{ID: id, Status: "closed"}, nil
			},
			sameReadFn: func(id string) (next.BeadInfo, error) {
				t.Fatal("same-read fallback must not run when --as-of succeeds")
				return next.BeadInfo{}, nil
			},
			wantErr: false,
		},
		{
			// (b) --as-of read shows open → error. A definitive committed
			// read of "not closed" is a real failure, not a fallback trigger.
			name: "as-of read open errors without fallback",
			asOfFn: func(id, ref string) (next.BeadInfo, error) {
				return next.BeadInfo{ID: id, Status: "in_progress"}, nil
			},
			sameReadFn: func(id string) (next.BeadInfo, error) {
				t.Fatal("same-read fallback must not run on a definitive as-of read")
				return next.BeadInfo{}, nil
			},
			wantErr:       true,
			wantErrSubstr: `status "in_progress" (not closed)`,
		},
		{
			// (c) --as-of unsupported (older bd) → falls back to the
			// same-read path, which shows closed → pass, with the
			// downgrade warning logged.
			name: "as-of unsupported falls back to closed same-read",
			asOfFn: func(id, ref string) (next.BeadInfo, error) {
				return next.BeadInfo{}, fmt.Errorf("bd show %s --as-of %s failed: %w",
					id, ref, fakeBDExitError(t, "Error: unknown flag: --as-of\n"))
			},
			sameReadFn: func(id string) (next.BeadInfo, error) {
				return next.BeadInfo{ID: id, Status: "closed"}, nil
			},
			wantErr:        false,
			wantDowngraded: true,
		},
		{
			// (d) --as-of unsupported AND the same-read fallback shows
			// not-closed → error (never proceed on an unverified close in
			// either path).
			name: "as-of unsupported and fallback not-closed errors",
			asOfFn: func(id, ref string) (next.BeadInfo, error) {
				return next.BeadInfo{}, fmt.Errorf("bd show %s --as-of %s failed: %w",
					id, ref, fakeBDExitError(t, "Error: unknown flag: --as-of\n"))
			},
			sameReadFn: func(id string) (next.BeadInfo, error) {
				return next.BeadInfo{ID: id, Status: "open"}, nil
			},
			wantErr:        true,
			wantErrSubstr:  `status "open" (not closed)`,
			wantDowngraded: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fetchBeadAsOfFn = tc.asOfFn
			fetchBeadByIDFn = tc.sameReadFn

			var err error
			stderrOut := captureStderr(t, func() {
				err = defaultVerifyCommitted("bead-uopd-1")
			})

			if tc.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tc.wantErrSubstr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr)) {
				t.Errorf("error = %v, want substring %q", err, tc.wantErrSubstr)
			}
			if tc.wantDowngraded && !strings.Contains(stderrOut, downgradeMarker) {
				t.Errorf("expected a committed-read-downgraded warning on stderr, got: %q", stderrOut)
			}
			if !tc.wantDowngraded && strings.Contains(stderrOut, downgradeMarker) {
				t.Errorf("did not expect a committed-read-downgraded warning on stderr, got: %q", stderrOut)
			}
		})
	}
}

// TestDefaultVerifyCommitted_AsOfHardReadFailureNeverFallsBack pins that a
// genuine --as-of read error (bead not found, Dolt lock contention, ...) —
// as opposed to bd's specific unknown-flag signature — is NEVER treated as
// an unsupported-flag signal. Falling back here would mask a real read
// failure behind the weaker same-read path.
func TestDefaultVerifyCommitted_AsOfHardReadFailureNeverFallsBack(t *testing.T) {
	origAsOf := fetchBeadAsOfFn
	origFetch := fetchBeadByIDFn
	t.Cleanup(func() {
		fetchBeadAsOfFn = origAsOf
		fetchBeadByIDFn = origFetch
	})

	fetchBeadAsOfFn = func(id, ref string) (next.BeadInfo, error) {
		return next.BeadInfo{}, fmt.Errorf("bd show %s --as-of %s failed: %w",
			id, ref, fakeBDExitError(t, "Error: dolt: database is locked\n"))
	}
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		t.Fatal("same-read fallback must not run on a genuine (non-unknown-flag) as-of read error")
		return next.BeadInfo{}, nil
	}

	err := defaultVerifyCommitted("bead-uopd-2")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "committed-state re-read (--as-of HEAD) failed") {
		t.Errorf("error should name the failed --as-of read, got: %v", err)
	}
}

func TestRun_AutoCommitUsesExecutor(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	_, err := Run(root, "bead-1", "", "add feature X", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify CommitAll was called via executor, targeting the MATCHED bead
	// worktree — never the root/main checkout (final-review r2 F2).
	commitCalls := mock.CallsTo("CommitAll")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 CommitAll call, got %d", len(commitCalls))
	}
	if path := commitCalls[0].Args[0].(string); path != "/tmp/worktree-bead-1" {
		t.Errorf("CommitAll path: got %q, want the matched bead worktree", path)
	}
	msg := commitCalls[0].Args[1].(string)
	if !strings.Contains(msg, "impl(bead-1)") || !strings.Contains(msg, "add feature X") {
		t.Errorf("CommitAll msg: got %q", msg)
	}
}

// TestRun_CommitMsgNoWorktreeRefusesMainCommit (final-review r2, F2): a
// `complete --commit-msg` invocation whose bead worktree is missing (no
// worktree-list match) while the bead/<id> ref still exists (so the R4
// reconcile path does NOT engage) must REFUSE — the pre-fix behavior fell
// back to CommitAll at root, i.e. a user-work commit on the main checkout
// with no branch guard. The refusal must land BEFORE any mutation: zero
// CommitAll calls, no bd close, no terminal CompleteBead.
func TestRun_CommitMsgNoWorktreeRefusesMainCommit(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	// No matching worktree — the F2 gap's trigger (a pruned worktree, with
	// the bead branch itself still present so reconcile detection stays
	// inert; MockExecutor's RevParseRef defaults to "found").
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closed := false
	closeBeadFn = func(ids ...string) error { closed = true; return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	_, err := Run(root, "bead-1", "", "add feature X", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a refusal — --commit-msg with no bead worktree must never commit on the main checkout")
	}
	msg := err.Error()
	if !strings.Contains(msg, "main checkout") || !strings.Contains(msg, "refusing") {
		t.Errorf("refusal must name the main-checkout hazard; got:\n%s", msg)
	}
	if !strings.Contains(msg, "mindspec next") {
		t.Errorf("refusal must carry the worktree-recreating `mindspec next` recovery; got:\n%s", msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("refusal must end with a `recovery:` line; got:\n%s", msg)
	}
	// NO commit was created anywhere — least of all on main.
	if calls := mock.CallsTo("CommitAll"); len(calls) != 0 {
		t.Errorf("CommitAll must never run on the refusal path, got %d calls", len(calls))
	}
	if closed {
		t.Error("bd close must not run after the --commit-msg refusal")
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("no terminal mutation allowed after the refusal, got %d CompleteBead calls", len(calls))
	}
}

// TestRun_WorktreeListErrorPropagates (final-review r2, F2): a failure
// enumerating worktrees is an infra error, not "no worktree" — it must
// surface as a preflight error (pre-fix it was swallowed, leaving
// wtPath == "" and routing the --commit-msg CommitAll to the main
// checkout). Nothing may mutate.
func TestRun_WorktreeListErrorPropagates(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return nil, fmt.Errorf("simulated bd worktree list failure")
	}
	closed := false
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	_, err := Run(root, "bead-1", "", "add feature X", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the worktree-list failure to propagate")
	}
	if !strings.Contains(err.Error(), "listing bead worktrees") {
		t.Errorf("error must name the worktree enumeration, got: %v", err)
	}
	if calls := mock.CallsTo("CommitAll"); len(calls) != 0 {
		t.Errorf("CommitAll must never run after a worktree-list failure, got %d calls", len(calls))
	}
	if closed {
		t.Error("bd close must not run after a worktree-list failure")
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

	// Req 7 + Spec 119 R3: exactly one follow-up commit, through the
	// executor, at the bead worktree, with the DQ-4 message — staged via
	// the EXPLICIT lifecycle-artifact pathspec (CommitPaths), never an
	// `add -A` equivalent (AC-4).
	commitCalls := mock.CallsTo("CommitPaths")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 CommitPaths (artifact sync), got %d", len(commitCalls))
	}
	if path := commitCalls[0].Args[0].(string); path != "/tmp/worktree-bead-1" {
		t.Errorf("artifact-sync commit path: got %q, want bead worktree", path)
	}
	if msg := commitCalls[0].Args[1].(string); msg != "chore: sync beads artifact" {
		t.Errorf("artifact-sync commit msg: got %q, want %q", msg, "chore: sync beads artifact")
	}
	if paths := commitCalls[0].Args[2].([]string); len(paths) != 1 || paths[0] != ".beads/issues.jsonl" {
		t.Errorf("artifact-sync pathspec: got %v, want [.beads/issues.jsonl]", paths)
	}
	if calls := mock.CallsTo("CommitAll"); len(calls) != 0 {
		t.Errorf("artifact-only sync must never use CommitAll (add -A equivalent), got %d calls", len(calls))
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

	// Spec 119 R3: the auto-commit (step 2.5) still uses CommitAll (the
	// user's own work); the follow-up artifact-only sync now goes
	// through the pathspec-scoped CommitPaths (AC-4).
	autoCommitCalls := mock.CallsTo("CommitAll")
	if len(autoCommitCalls) != 1 {
		t.Fatalf("expected 1 CommitAll (the user's auto-commit), got %d", len(autoCommitCalls))
	}
	first := autoCommitCalls[0].Args[1].(string)
	if !strings.Contains(first, "impl(bead-1)") || !strings.Contains(first, "implement the thing") {
		t.Errorf("auto-commit message wrong, got %q", first)
	}

	syncCalls := mock.CallsTo("CommitPaths")
	if len(syncCalls) != 1 {
		t.Fatalf("expected 1 CommitPaths (follow-up artifact sync), got %d", len(syncCalls))
	}
	second := syncCalls[0].Args[1].(string)
	if second != "chore: sync beads artifact" {
		t.Errorf("follow-up commit message wrong, got %q", second)
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
	mock.CommitPathsErr = fmt.Errorf("commit hook exploded")

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

// TestRun_ArtifactDirt_NoWorktreeRefusesMainCommit (Spec 119 R3, AC-3): when
// no bead worktree matched (wtPath == "", falling back to root — the main
// repo root per Run's own contract), artifact dirt must NOT be committed
// there. Refuses with a named re-invocation command; zero commits.
//
// mindspec-lc12.1 fix-up (panel finding #1): the recovery command must be
// one that actually converges. Resolution (root/wtPath/checkPath) is
// cwd-INDEPENDENT, so "cd into the spec worktree and re-run `mindspec
// complete`" reaches this exact same refusal again — an infinite loop.
// The recovery must instead be `mindspec next`, which detects the
// in-progress bead with a missing worktree and recreates it (see
// internal/next/guard.go), after which a subsequent `mindspec complete`
// finds a real wtPath. Pinned here so a regression back to the "cd ... and
// re-run `mindspec complete`" text fails this test.
func TestRun_ArtifactDirt_NoWorktreeRefusesMainCommit(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
		return []string{".beads/issues.jsonl"}, nil, nil
	}
	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil } // no match → checkPath == root

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a refusal rather than committing artifact dirt on the main checkout")
	}
	if !strings.Contains(err.Error(), "main checkout") {
		t.Errorf("error should name the main-checkout refusal, got: %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("refusal must carry a recovery line, got: %v", err)
	}
	recovery := finalRecoveryCommand(t, err.Error())
	if !strings.HasPrefix(recovery, "mindspec next") {
		t.Errorf("recovery command must be a convergent `mindspec next` re-run, got: %q", recovery)
	}
	if strings.Contains(recovery, "cd into") {
		t.Errorf("recovery command must not tell the caller to cd-and-rerun `mindspec complete` — that loops forever (cwd-independent resolution), got: %q", recovery)
	}
	if len(mock.CallsTo("CommitPaths")) != 0 || len(mock.CallsTo("CommitAll")) != 0 {
		t.Error("expected ZERO commits when refusing the main-checkout artifact sync")
	}
}

// TestRun_ArtifactDirt_ResidualDirtNamedInWarning (Spec 119 R3, AC-4): a
// path that becomes dirty AFTER the artifact-dirt scan (simulated via
// exec.Status returning an extra, unrelated dirty entry post-commit) is
// EXCLUDED from the pathspec-scoped tracker commit and named in a warning —
// never silently swept in.
func TestRun_ArtifactDirt_ResidualDirtNamedInWarning(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	mock.StatusFn = func(workdir string) (string, error) {
		return " M unrelated-race-file.go\n", nil
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
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	w.Close()
	os.Stderr = origStderr
	var captured bytes.Buffer
	io.Copy(&captured, r)

	if err != nil {
		t.Fatalf("residual dirt must never block completion, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if calls := mock.CallsTo("CommitPaths"); len(calls) != 1 {
		t.Fatalf("expected 1 CommitPaths call, got %d", len(calls))
	} else if paths := calls[0].Args[2].([]string); len(paths) != 1 || paths[0] != ".beads/issues.jsonl" {
		t.Errorf("expected the pathspec to be EXACTLY [.beads/issues.jsonl], got %v (the residual file must be excluded)", paths)
	}
	if !strings.Contains(captured.String(), "unrelated-race-file.go") {
		t.Errorf("expected the residual dirty path to be named in a warning, got stderr: %q", captured.String())
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
	// Spec 096 Req 2: the post-close re-read must AFFIRM closed (case a)
	// so the close-verification step emits no WARN of its own — this test
	// pins that an empty doc-sync result prints nothing, so the re-read
	// has to land on the silent (confirmed-closed) path.
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "closed"}, nil
	}
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

	// Spec 107 wave 1 (mindspec-oexu.3): advanceState's post-close children read
	// now runs through phase.FetchChildren on the phase seam stubbed above
	// (cwdSensitive review → one closed child), so there is no separate complete
	// package seam to stub. The runBD seam still fails from a deleted cwd like
	// real bd.
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
	if !strings.Contains(msg, ".mindspec/domains/<name>/OWNERSHIP.yaml") {
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

// TestResolveBeadWorktree_GatesListedBranch is the spec 120 final-review
// G1-1 regression: a bd worktree-list row accepted on a NAME-only match
// used to promote its unvalidated Branch field straight into beadHead —
// which then reaches git rev-parse/merge-base/diff/show argv. The listed
// branch may only replace the validated canonical branch when it is
// itself a well-formed bead branch ("bead/" + idvalidate.BeadID); any
// other value falls back to the canonical waist-composed branch.
func TestResolveBeadWorktree_GatesListedBranch(t *testing.T) {
	const expectedName = "worktree-mindspec-x.1"
	const canonical = "bead/mindspec-x.1"

	cases := []struct {
		name     string
		entries  []bead.WorktreeListEntry
		wantPath string
		wantHead string
	}{
		{
			name:     "no matching row: canonical branch, no path",
			entries:  []bead.WorktreeListEntry{{Name: "other", Branch: "bead/mindspec-zz.9", Path: "/p0"}},
			wantPath: "",
			wantHead: canonical,
		},
		{
			name:     "branch match with foreign name: path adopted, canonical head",
			entries:  []bead.WorktreeListEntry{{Name: "renamed-dir", Branch: canonical, Path: "/p1"}},
			wantPath: "/p1",
			wantHead: canonical,
		},
		{
			name:     "name match with empty branch: canonical head",
			entries:  []bead.WorktreeListEntry{{Name: expectedName, Branch: "", Path: "/p2"}},
			wantPath: "/p2",
			wantHead: canonical,
		},
		{
			name:     "name match with VALID re-anchored bead branch: promoted",
			entries:  []bead.WorktreeListEntry{{Name: expectedName, Branch: "bead/mindspec-other", Path: "/p3"}},
			wantPath: "/p3",
			wantHead: "bead/mindspec-other",
		},
		{
			name:     "name match with Branch=main: falls back to canonical (never reaches git argv)",
			entries:  []bead.WorktreeListEntry{{Name: expectedName, Branch: "main", Path: "/p4"}},
			wantPath: "/p4",
			wantHead: canonical,
		},
		{
			name:     "name match with option-like branch: falls back to canonical",
			entries:  []bead.WorktreeListEntry{{Name: expectedName, Branch: "--upload-pack=/tmp/evil", Path: "/p5"}},
			wantPath: "/p5",
			wantHead: canonical,
		},
		{
			name:     "name match with metachar bead branch: falls back to canonical",
			entries:  []bead.WorktreeListEntry{{Name: expectedName, Branch: "bead/mindspec-x;evil", Path: "/p6"}},
			wantPath: "/p6",
			wantHead: canonical,
		},
		{
			name:     "name match with control-byte branch: falls back to canonical",
			entries:  []bead.WorktreeListEntry{{Name: expectedName, Branch: "bead/mindspec-x\nevil", Path: "/p7"}},
			wantPath: "/p7",
			wantHead: canonical,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotHead string
			// The fallback legs warn on stderr (ADR-0042 degrade policy);
			// capture so test output stays clean.
			_ = captureStderr(t, func() {
				gotPath, gotHead = resolveBeadWorktree(tc.entries, expectedName, canonical)
			})
			if gotPath != tc.wantPath {
				t.Errorf("wtPath = %q, want %q", gotPath, tc.wantPath)
			}
			if gotHead != tc.wantHead {
				t.Errorf("beadHead = %q, want %q", gotHead, tc.wantHead)
			}
		})
	}
}
