package complete

// Spec 092 Bead 6 — incident amendment (2026-06-11 merge-driver
// incident, panel j3-recurrence): complete.Run must PROPAGATE a
// CompleteBead failure as a non-zero exit with an explicit
// closed-but-unmerged recovery hint, instead of the old
// `Warning: bead cleanup: ...` warn-and-continue that exited 0 and
// hid the inconsistency.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// TestRun_BlocksOnOrphanedSibling (bead mindspec-4gsz): if another sibling
// bead under the epic was closed without `mindspec complete` (its branch is
// unmerged into the spec branch), complete.Run blocks BEFORE any mutation and
// points at `mindspec complete <sibling>`. The guard excludes the very bead in
// flight (chicken-and-egg) — verified by the predicate receiving beadID as the
// exclude argument.
func TestRun_BlocksOnOrphanedSibling(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	var gotExclude string
	findOrphanedClosedBeadsFn = func(specID, workdir, excludeBeadID string) []lifecycle.Orphan {
		gotExclude = excludeBeadID
		return []lifecycle.Orphan{{BeadID: "sib-1", BeadBranch: "bead/sib-1", SpecBranch: "spec/008-test"}}
	}
	// Spec 127 R2: no real underlying repo — stub the "normal unmerged"
	// outcome, matching this test's own byte-identical assertion below.
	evaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
		return lifecycle.OrphanHint{Outcome: guard.DestructionClean, Lines: []string{"mindspec complete " + beadID}}
	}

	// A close stub that, if reached, would record a mutation — it must NOT be.
	closed := false
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	_, err := Run(root, "bead-self", "", "", newMockExec(), CompleteOpts{})
	if err == nil {
		t.Fatal("expected a block when an orphaned sibling exists")
	}
	if closed {
		t.Error("Run must block BEFORE any mutation (close must not run)")
	}
	if gotExclude != "bead-self" {
		t.Errorf("predicate exclude arg = %q, want bead-self (the bead in flight)", gotExclude)
	}
	msg := err.Error()
	if !strings.Contains(msg, "sib-1") {
		t.Errorf("error must name the orphaned sibling; got:\n%s", msg)
	}
	if !strings.Contains(msg, "mindspec complete sib-1") {
		t.Errorf("error must point at the sibling recovery command; got:\n%s", msg)
	}
}

// setupCompleteBeadFailure wires the minimal happy-path stubs up to the
// CompleteBead call, with the executor's CompleteBead failing with
// completeErr. Returns the mock and a pointer to the recorded
// mindspec_phase metadata writes (which must stay empty on failure).
func setupCompleteBeadFailure(t *testing.T, completeErr error) (root string, mock *executor.MockExecutor, phaseWrites *[]string) {
	t.Helper()
	saveAndRestore(t)

	root = setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }

	writes := []string{}
	phaseWrites = &writes
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if v, ok := updates["mindspec_phase"]; ok {
			writes = append(writes, fmt.Sprintf("%s=%v", id, v))
		}
		return nil
	}

	mock = newMockExec()
	mock.CompleteBeadErr = completeErr
	return root, mock, phaseWrites
}

// TestRun_CompleteBeadFailurePropagatesClosedButUnmerged: a CompleteBead
// failure that already carries Req 12 recovery lines (the executor's
// conflict-abort failures) surfaces as a non-zero Run error that names
// the closed-but-unmerged state and keeps the executor's recovery lines
// final.
func TestRun_CompleteBeadFailurePropagatesClosedButUnmerged(t *testing.T) {
	execErr := guard.NewFailure(
		"merge conflict: could not merge bead/bead-1 into spec/008-test\nconflicted files:\n  .beads/issues.jsonl",
		"cd /repo/.worktrees/worktree-spec-008-test",
		"git merge --no-ff bead/bead-1",
		"mindspec complete bead-1",
	)
	root, mock, phaseWrites := setupCompleteBeadFailure(t, execErr)

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected an error when CompleteBead fails (no warn-and-continue)")
	}
	if result != nil {
		t.Errorf("expected nil result on failure, got %+v", result)
	}
	msg := err.Error()

	// The closed-but-unmerged state is stated, not hidden.
	if !strings.Contains(msg, "CLOSED in Dolt") {
		t.Errorf("error must state the bead is already CLOSED in Dolt; got:\n%s", msg)
	}
	// Convergence is explicit: re-running complete after resolution.
	if !strings.Contains(msg, "mindspec complete bead-1") {
		t.Errorf("error must name the converging re-run command; got:\n%s", msg)
	}
	if !strings.Contains(msg, "idempotent") {
		t.Errorf("error must state the close step is idempotent (reconvergence); got:\n%s", msg)
	}
	// The executor's conflict detail (conflicted files) is preserved.
	if !strings.Contains(msg, ".beads/issues.jsonl") {
		t.Errorf("error must preserve the executor's conflicted-file detail; got:\n%s", msg)
	}
	// Req 12: per-site HasFinalRecoveryLine.
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
	// B9: the final recovery command is never a banned (Req 19) command.
	if cmd := finalRecoveryCommand(t, msg); guard.IsBannedRecoveryCommand(cmd) {
		t.Errorf("final recovery command is banned (Req 19): %q", cmd)
	}

	// No post-terminal mutations ran: the mindspec_phase sync (step 6.5)
	// must not have fired.
	if len(*phaseWrites) != 0 {
		t.Errorf("mindspec_phase must not be synced after a failed completion; got writes: %v", *phaseWrites)
	}
}

// TestRun_CompleteBeadPlainFailureGainsRecoveryLine: a CompleteBead
// failure WITHOUT recovery lines (e.g. the ancestor safety check) still
// surfaces non-zero, and Run appends the converging re-run as the
// final recovery line.
func TestRun_CompleteBeadPlainFailureGainsRecoveryLine(t *testing.T) {
	execErr := errors.New("bead branch bead/bead-1 is NOT merged into spec/008-test — aborting cleanup to prevent data loss")
	root, mock, _ := setupCompleteBeadFailure(t, execErr)

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected an error when CompleteBead fails")
	}
	msg := err.Error()
	if !strings.Contains(msg, "CLOSED in Dolt") {
		t.Errorf("error must state the bead is already CLOSED in Dolt; got:\n%s", msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	if got := lines[len(lines)-1]; got != "recovery: mindspec complete bead-1" {
		t.Errorf("final recovery line = %q, want %q", got, "recovery: mindspec complete bead-1")
	}
	// B9: the final recovery command is never a banned (Req 19) command.
	if cmd := finalRecoveryCommand(t, msg); guard.IsBannedRecoveryCommand(cmd) {
		t.Errorf("final recovery command is banned (Req 19): %q", cmd)
	}
}

// --- Spec 121 R6 (mindspec-tpjn): orphan-preflight convergence (ADR-0041 §2(i)) ---

// captureStdout redirects os.Stdout for the duration of fn and returns
// whatever fn wrote to it. The step-1.6 WARN-demotion prints via
// fmt.Printf (stdout), mirroring the existing "Warning: bead ... already
// closed" line (complete.go step 4) — captureStderr (above, this
// package) exists for the stderr-writing seams; this is its stdout twin.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return buf.String()
}

// runOrphanFixture runs complete.Run for beadID against a minimal happy
// path (no worktree, no next-ready bead) with the orphan-preflight seams
// stubbed as given, and reports whether the bead was closed (i.e. the run
// PROCEEDED past step 1.6) plus everything Run printed to stdout.
func runOrphanFixture(t *testing.T, specID, beadID string, otherOrphans []lifecycle.Orphan, selfOrphaned func(id string) (bool, error)) (closed bool, gotExclude string, stdout string) {
	t.Helper()
	saveAndRestore(t)
	root := setupTempRoot(t)
	stubPhaseEpic(t, specID, "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return specID, nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return nil, fmt.Errorf("no results") }

	findOrphanedClosedBeadsFn = func(sid, workdir, excludeBeadID string) []lifecycle.Orphan {
		gotExclude = excludeBeadID
		return otherOrphans
	}
	// Spec 127 R2: these fixtures fabricate lifecycle.Orphan values with
	// no real underlying repo — evaluateOrphanHintFn's real
	// implementation would do real git I/O and fail closed
	// (DestructionEvidenceError) against non-existent refs. Stub the
	// "normal unmerged" outcome, matching every assertion in this file
	// (byte-identical to Orphan.RecoveryCommand()'s pre-existing text).
	evaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
		return lifecycle.OrphanHint{Outcome: guard.DestructionClean, Lines: []string{"mindspec complete " + beadID}}
	}
	isBeadSelfOrphanedFn = func(sid, workdir, id string) (bool, error) {
		return selfOrphaned(id)
	}

	var closedID string
	closeBeadFn = func(ids ...string) error { closedID = ids[0]; return nil }

	var runErr error
	stdout = captureStdout(t, func() {
		_, runErr = Run(root, beadID, "", "", newMockExec(), CompleteOpts{})
	})
	if runErr == nil {
		if closedID != beadID {
			t.Errorf("Run succeeded but never closed %s (closed %q instead)", beadID, closedID)
		}
		return true, gotExclude, stdout
	}
	if closedID != "" {
		t.Errorf("Run must not mutate (close) before/without proceeding past the orphan preflight; closed=%q, err=%v", closedID, runErr)
	}
	return false, gotExclude, runErr.Error()
}

// TestRun_MultiOrphanConvergence is AC-13: three mutually-orphaned closed
// siblings A, B, C. `mindspec complete bead-a` proceeds (recovers A) emitting a
// WARN naming BOTH B and C; `mindspec complete bead-b` then proceeds (WARN
// naming C); `mindspec complete bead-c` then proceeds — the whole sequence
// converges via nothing but `mindspec complete` invocations (ADR-0041
// §2(i), the WARN-demotion this bead adds). RED on today's `main`: the
// pre-121 preflight refuses unconditionally on ANY other-orphan finding,
// so `complete A` here would error instead of proceeding.
func TestRun_MultiOrphanConvergence(t *testing.T) {
	// Step 1: complete A. B and C are still orphaned; A is itself the
	// orphan being recovered.
	closedA, excludeA, outA := runOrphanFixture(t, "008-test", "bead-a",
		[]lifecycle.Orphan{
			{BeadID: "bead-b", BeadBranch: "bead/bead-b", SpecBranch: "spec/008-test"},
			{BeadID: "bead-c", BeadBranch: "bead/bead-c", SpecBranch: "spec/008-test"},
		},
		func(id string) (bool, error) { return id == "bead-a", nil },
	)
	if !closedA {
		t.Fatalf("expected complete A to PROCEED (WARN-demotion); got: %s", outA)
	}
	if excludeA != "bead-a" {
		t.Errorf("exclude arg = %q, want %q", excludeA, "bead-a")
	}
	if !strings.Contains(outA, "bead-b") || !strings.Contains(outA, "bead-c") {
		t.Errorf("expected the WARN to name BOTH B and C; got stdout:\n%s", outA)
	}
	if !strings.Contains(strings.ToLower(outA), "warn") {
		t.Errorf("expected a WARN-level line (demoted from refusal); got stdout:\n%s", outA)
	}

	// Step 2: complete B. Only C remains orphaned; B is itself the orphan
	// being recovered.
	closedB, excludeB, outB := runOrphanFixture(t, "008-test", "bead-b",
		[]lifecycle.Orphan{
			{BeadID: "bead-c", BeadBranch: "bead/bead-c", SpecBranch: "spec/008-test"},
		},
		func(id string) (bool, error) { return id == "bead-b", nil },
	)
	if !closedB {
		t.Fatalf("expected complete B to PROCEED (WARN-demotion); got: %s", outB)
	}
	if excludeB != "bead-b" {
		t.Errorf("exclude arg = %q, want %q", excludeB, "bead-b")
	}
	if !strings.Contains(outB, "bead-c") {
		t.Errorf("expected the WARN to name C; got stdout:\n%s", outB)
	}

	// Step 3: complete C. No orphans remain at all — the normal,
	// unconverged path (findOrphanedClosedBeadsFn returns nil).
	closedC, _, outC := runOrphanFixture(t, "008-test", "bead-c", nil,
		func(id string) (bool, error) { return false, nil },
	)
	if !closedC {
		t.Fatalf("expected complete C to proceed with zero orphans remaining; got: %s", outC)
	}
}

// TestRun_AllOrphansRefusalNamesEverySibling is AC-14: a NON-orphaned bead
// C invoked while A and B are both orphaned refuses naming BOTH A and B
// with the full recovery sequence (not just orphans[0], the pre-121
// shape) — then, after A and B are completed, `complete C` proceeds. RED
// on today's `main`: the pre-121 message names only orphans[0] (A), never
// B.
func TestRun_AllOrphansRefusalNamesEverySibling(t *testing.T) {
	closed, exclude, msg := runOrphanFixture(t, "008-test", "bead-c",
		[]lifecycle.Orphan{
			{BeadID: "bead-a", BeadBranch: "bead/bead-a", SpecBranch: "spec/008-test"},
			{BeadID: "bead-b", BeadBranch: "bead/bead-b", SpecBranch: "spec/008-test"},
		},
		func(id string) (bool, error) { return false, nil }, // C is NOT itself orphaned
	)
	if closed {
		t.Fatal("expected complete C to be REFUSED — nothing mutated")
	}
	if exclude != "bead-c" {
		t.Errorf("exclude arg = %q, want %q", exclude, "bead-c")
	}
	if !strings.Contains(msg, "bead-a") {
		t.Errorf("refusal must name sibling A; got:\n%s", msg)
	}
	if !strings.Contains(msg, "bead-b") {
		t.Errorf("refusal must name sibling B; got:\n%s", msg)
	}
	if !strings.Contains(msg, "mindspec complete bead-a") {
		t.Errorf("refusal must name A's recovery command; got:\n%s", msg)
	}
	if !strings.Contains(msg, "mindspec complete bead-b") {
		t.Errorf("refusal must name B's recovery command; got:\n%s", msg)
	}
	if !strings.Contains(msg, "mindspec complete bead-c") {
		t.Errorf("refusal must name the converging re-run of C itself; got:\n%s", msg)
	}

	// After A and B are completed, complete C proceeds (no orphans left).
	closedC, _, outC := runOrphanFixture(t, "008-test", "bead-c", nil,
		func(id string) (bool, error) { return false, nil },
	)
	if !closedC {
		t.Fatalf("expected complete C to proceed once A and B are recovered; got: %s", outC)
	}
}

// TestRun_SelfOrphanDeterminationErrorRetainsRefusal is the infra-error
// retention subtest ADR-0041 §2(i) requires: when the self-orphan
// determination itself ERRORS (an infra/ancestry failure, not a genuine
// answer), the invoked bead is treated as NOT self-orphaned and the
// all-orphans refusal is RETAINED — a retryable preflight refusal with
// NOTHING mutated — never a demotion to WARN on unproven evidence.
func TestRun_SelfOrphanDeterminationErrorRetainsRefusal(t *testing.T) {
	determinationErr := errors.New("checking ancestry of bead/bead-a: simulated infra failure")

	tests := []struct {
		name       string
		selfAnswer bool
	}{
		{"false answer with error", false},
		// S2-1 (panel MINOR, load-bearing discriminator): the production
		// gate is `selfErr == nil && selfOrphaned` — a regression that
		// dropped the `selfErr == nil` half would read THIS true answer
		// as self-orphaned and wrongly WARN-demote instead of retaining
		// the refusal. Stubbing (true, err) proves the error check
		// itself gates the decision, not merely the bool's zero value
		// (the previous false-only fixture could not distinguish the
		// two).
		{"true answer with error", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			closed, _, msg := runOrphanFixture(t, "008-test", "bead-a",
				[]lifecycle.Orphan{
					{BeadID: "bead-b", BeadBranch: "bead/bead-b", SpecBranch: "spec/008-test"},
				},
				func(id string) (bool, error) { return tc.selfAnswer, determinationErr },
			)
			if closed {
				t.Fatal("expected the refusal to be RETAINED on a determination error — nothing mutated")
			}
			if !strings.Contains(msg, "bead-b") {
				t.Errorf("refusal must still name the orphaned sibling B; got:\n%s", msg)
			}
			if !strings.Contains(msg, "mindspec complete bead-b") {
				t.Errorf("refusal must still carry B's recovery command; got:\n%s", msg)
			}
		})
	}
}

// TestRun_MultiLineHintThreadsAllLinesInOrder is bead-4 fix round 2's
// ITEM 1 (G1-5 held open across fix round 1): round 1 added the
// analogous multi-line fidelity test to internal/approve
// (implOrphanRefusal) and internal/doctor (checkOrphanedBeads), but this
// package — the THIRD consumer of the same lifecycle.OrphanHint
// derivation — had no equivalent. Every existing orphan test in this
// file stubs evaluateOrphanHintFn with the single-line Clean shape
// (Lines: []string{"mindspec complete " + beadID}); TestRun_MultiOrphanConvergence
// and TestRun_AllOrphansRefusalNamesEverySibling both cover BREADTH over
// multiple orphaned SIBLINGS, never fidelity over a single hint's
// MULTIPLE LINES. This stubs the Superseded shape (preserve-tag,
// delete, adopt-invocation — three lines) for a single non-self-orphaned
// sibling and asserts every line appears, in order and unmodified,
// inside Run's returned error — the fidelity check the round-1
// amendment's "internal/complete's pre-existing coverage compensates"
// claim asserted and did not have.
//
// complete.go's renderOrphanRecoverySegment folds hint.Lines with
// `strings.Join(hint.Lines, "; ")` (byte-identical fold to
// internal/doctor's orphaned_beads.go), so order and content survive
// that fold exactly — unlike internal/approve's separate-recovery-line
// (guard.NewFailure) rendering.
func TestRun_MultiLineHintThreadsAllLinesInOrder(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return nil, fmt.Errorf("no results") }

	wantLines := []string{
		`git tag preserve/bead-x bead/bead-x   (preserve bead-x's commits before deleting — its content is not guaranteed reachable from spec/008-test)`,
		"git branch -D bead/bead-x",
		`mindspec impl adopt 008-test --reason "<why bead-x's content already reached main outside the lifecycle>"`,
	}
	findOrphanedClosedBeadsFn = func(sid, workdir, excludeBeadID string) []lifecycle.Orphan {
		return []lifecycle.Orphan{{BeadID: "bead-x", BeadBranch: "bead/bead-x", SpecBranch: "spec/008-test"}}
	}
	evaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
		return lifecycle.OrphanHint{
			Outcome:      guard.DestructionSuperseded,
			EvidenceNote: "bead bead-x's branch bead/bead-x is superseded — its content already landed in spec/008-test via another route per the shared work-destruction predicate",
			Lines:        append([]string{}, wantLines...),
		}
	}
	// bead-c is NOT itself orphaned — this drives the all-orphans refusal
	// path (the else branch below the self-orphan WARN-demotion), the
	// same path TestRun_AllOrphansRefusalNamesEverySibling exercises.
	isBeadSelfOrphanedFn = func(sid, workdir, id string) (bool, error) { return false, nil }
	closeBeadFn = func(ids ...string) error { t.Fatal("must not mutate — this is a preflight refusal"); return nil }

	_, err := Run(root, "bead-c", "", "", newMockExec(), CompleteOpts{})
	if err == nil {
		t.Fatal("expected a refusal naming the orphaned sibling")
	}
	msg := err.Error()

	lastIdx := -1
	for i, want := range wantLines {
		idx := strings.Index(msg, want)
		if idx < 0 {
			t.Fatalf("line %d (%q) missing verbatim from Run's error:\n%s", i, want, msg)
		}
		if idx <= lastIdx {
			t.Fatalf("line %d (%q) did not appear AFTER the previous line — order not preserved:\n%s", i, want, msg)
		}
		lastIdx = idx
	}
}

// TestRun_FidelityAcrossAllOutcomes is bead-4 fix round 5's answer to
// G1-5A: TestRun_MultiLineHintThreadsAllLinesInOrder above proves Run's
// line-threading fidelity only for guard.DestructionSuperseded. G1
// reproduced the gap directly: an outcome-conditional truncation
// dropping the final hint line ONLY when hint.Outcome ==
// guard.DestructionAncestor (or, separately,
// guard.DestructionEvidenceError) left the Superseded-only fixture
// green in renderOrphanRecoverySegment. This table drives the all-
// orphans refusal path over the FULL closed outcome set — asserted
// against guard.DestructionOutcomeCount, never a literal count (bead 2
// shipped five stale written counts; a sixth outcome variant must red
// this table's length assertion before it reds anything else) — with a
// FABRICATED three-line hint per outcome: real DeriveOrphanHint output
// is single-line for several outcomes, and this table's job is
// renderOrphanRecoverySegment's OWN `strings.Join(hint.Lines, "; ")`
// fold, never the derivation's own content.
func TestRun_FidelityAcrossAllOutcomes(t *testing.T) {
	outcomes := make([]guard.DestructionOutcome, 0, int(guard.DestructionOutcomeCount))
	for o := guard.DestructionOutcome(0); o < guard.DestructionOutcomeCount; o++ {
		outcomes = append(outcomes, o)
	}
	if len(outcomes) != int(guard.DestructionOutcomeCount) {
		t.Fatalf("this table covers %d outcomes, want %d (guard.DestructionOutcomeCount) — a new outcome variant needs its own row", len(outcomes), int(guard.DestructionOutcomeCount))
	}

	for _, outcome := range outcomes {
		t.Run(outcome.String(), func(t *testing.T) {
			saveAndRestore(t)
			root := setupTempRoot(t)
			stubPhaseEpic(t, "008-test", "mol-parent-1")

			resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
			worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
			runBDFn = func(args ...string) ([]byte, error) { return nil, fmt.Errorf("no results") }

			wantLines := []string{
				fmt.Sprintf("synthetic recovery line 1 for %s", outcome),
				fmt.Sprintf("synthetic recovery line 2 for %s", outcome),
				fmt.Sprintf("synthetic recovery line 3 for %s", outcome),
			}
			findOrphanedClosedBeadsFn = func(sid, workdir, excludeBeadID string) []lifecycle.Orphan {
				return []lifecycle.Orphan{{BeadID: "bead-x", BeadBranch: "bead/bead-x", SpecBranch: "spec/008-test"}}
			}
			evaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
				return lifecycle.OrphanHint{
					Outcome:      outcome,
					EvidenceNote: fmt.Sprintf("synthetic evidence note for %s", outcome),
					Lines:        append([]string{}, wantLines...),
				}
			}
			// bead-c is NOT itself orphaned — this drives the all-orphans
			// refusal path (the else branch below the self-orphan
			// WARN-demotion), the same path
			// TestRun_MultiLineHintThreadsAllLinesInOrder exercises.
			isBeadSelfOrphanedFn = func(sid, workdir, id string) (bool, error) { return false, nil }
			closeBeadFn = func(ids ...string) error { t.Fatal("must not mutate — this is a preflight refusal"); return nil }

			_, err := Run(root, "bead-c", "", "", newMockExec(), CompleteOpts{})
			if err == nil {
				t.Fatal("expected a refusal naming the orphaned sibling")
			}
			msg := err.Error()

			lastIdx := -1
			for i, want := range wantLines {
				idx := strings.Index(msg, want)
				if idx < 0 {
					t.Fatalf("outcome %s: line %d (%q) missing verbatim from Run's error:\n%s", outcome, i, want, msg)
				}
				if idx <= lastIdx {
					t.Fatalf("outcome %s: line %d (%q) did not appear AFTER the previous line — order not preserved:\n%s", outcome, i, want, msg)
				}
				lastIdx = idx
			}
		})
	}
}
