package approve

// Spec 127 bead 3 — AC-1 (adopt happy path) and AC-2 (adopt refusal
// legs) driven through the full AdoptSpec entrypoint, real git
// throughout, bd stubbed via this package's in-package seams.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// realAdoptCommitExecutor wraps a bare MockExecutor and, for CommitPaths
// ONLY, performs a REAL `git add -- <paths>` + commit in path — the
// killAfterPlanCommitExecutor house pattern (plan_fault_test.go) applied
// to adopt's finalize-export commit (G1-B3-01: production now confines
// the commit to the export artifact via exec.CommitPaths, never
// exec.CommitAll's `git add -A`). The .beads/issues.jsonl content itself
// is written by adoptStubExport (standing in for production's
// bead.Export refresh, which these tests never shell out to); this
// executor only performs the real stage+commit so there is a real,
// inspectable finalize-export delta to assert against.
type realAdoptCommitExecutor struct {
	*executor.MockExecutor
	t     *testing.T
	calls int
}

func (e *realAdoptCommitExecutor) CommitPaths(path, msg string, paths []string) error {
	e.calls++
	// Delegates to the REAL gitutil.CommitPaths rather than re-implementing
	// similar-but-subtly-different add+commit logic here: a hand-rolled
	// double that merely approximates the real scoped-commit behavior is
	// exactly the drift class this bead's own fix (confining the commit
	// via a pathspec on BOTH `git add` and `git commit`, G1-B3-01) could
	// silently go untested against if this fake used a plain `git commit`
	// with no pathspec.
	return gitutil.CommitPaths(path, msg, paths)
}

// adoptStubExport installs a test double for adoptExportBeadsFn that
// writes a deterministic .beads/issues.jsonl into root — standing in for
// production's bead.Export refresh (a real `bd export`), which these
// tests never shell out to. Returns the call counter so a test can
// assert how many times the export stage actually ran (e.g. an
// idempotent resume must not re-export/re-commit once stage 2 already
// completed).
func adoptStubExport(t *testing.T) *int {
	t.Helper()
	calls := 0
	orig := adoptExportBeadsFn
	adoptExportBeadsFn = func(root string) error {
		calls++
		beadsDir := filepath.Join(root, ".beads")
		if err := os.MkdirAll(beadsDir, 0o755); err != nil {
			return err
		}
		content := fmt.Sprintf(`{"id":"epic-1","status":"closed","adopt_call":%d}`+"\n", calls)
		return os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(content), 0o644)
	}
	t.Cleanup(func() { adoptExportBeadsFn = orig })
	return &calls
}

// wireAdoptSeams stubs adoptFindEpicFn/adoptRunBDCombinedFn/
// adoptGetMetadataFn/adoptMergeMetadataFn so AdoptSpec never shells to a
// real bd: epicID is fixed, "close" is recorded (never actually run),
// and a single in-memory map backs BOTH the metadata read (get) and
// write (merge) — a realistic bd-metadata-store double that lets a test
// call AdoptSpec twice and observe the SECOND call see what the FIRST
// call wrote (G1-B3-03's idempotency fixtures rely on exactly this).
//
// Also defaults the G1-B3-02 review-state precondition to a SATISFIED
// gate (review phase, zero open lifecycle children) — the realistic
// #218 shape: the epic's own lifecycle work already completed through
// the normal bead-complete flow; only main's final merge is missing.
// Tests that specifically exercise gate FAILURE (plan/implement phase,
// an open lifecycle bead) override adoptDerivePhaseDetailFn/
// adoptLifecycleChildIDsFn/adoptReadBeadStatusFn again after calling
// this helper.
func wireAdoptSeams(t *testing.T, epicID string) (closedCalls *int, metadata map[string]interface{}) {
	t.Helper()
	metadata = map[string]interface{}{}
	closed := 0

	origFindEpic := adoptFindEpicFn
	adoptFindEpicFn = func(specID string) (string, error) { return epicID, nil }
	t.Cleanup(func() { adoptFindEpicFn = origFindEpic })

	origPhase := adoptDerivePhaseDetailFn
	adoptDerivePhaseDetailFn = func(id string) (phase.PhaseDetail, error) {
		return phase.PhaseDetail{EpicID: id, Stored: "", Derived: state.ModeReview}, nil
	}
	t.Cleanup(func() { adoptDerivePhaseDetailFn = origPhase })

	origLifecycle := adoptLifecycleChildIDsFn
	adoptLifecycleChildIDsFn = func(id string) ([]string, error) { return nil, nil }
	t.Cleanup(func() { adoptLifecycleChildIDsFn = origLifecycle })

	origReadStatus := adoptReadBeadStatusFn
	adoptReadBeadStatusFn = func(id string) (string, error) { return "closed", nil }
	t.Cleanup(func() { adoptReadBeadStatusFn = origReadStatus })

	origCombined := adoptRunBDCombinedFn
	adoptRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "close" {
			closed++
		}
		return nil, nil
	}
	t.Cleanup(func() { adoptRunBDCombinedFn = origCombined })

	origGet := adoptGetMetadataFn
	adoptGetMetadataFn = func(issueID string) (map[string]interface{}, error) {
		out := make(map[string]interface{}, len(metadata))
		for k, v := range metadata {
			out[k] = v
		}
		return out, nil
	}
	t.Cleanup(func() { adoptGetMetadataFn = origGet })

	origMerge := adoptMergeMetadataFn
	adoptMergeMetadataFn = func(issueID string, updates map[string]interface{}) error {
		for k, v := range updates {
			metadata[k] = v
		}
		return nil
	}
	t.Cleanup(func() { adoptMergeMetadataFn = origMerge })

	return &closed, metadata
}

// TestAdoptSpec_HappyPath is AC-1: verified evidence via surviving,
// positively-evidenced bead branches (the "surviving bead branches
// covering the epic" form of R1b's evidence-source disjunction; the
// remote-spec-ref form is exercised by
// TestEvaluateAdoptLattice_Table/all_positive_with_coverage_is_verified
// in adopt_lattice_test.go — both forms of the disjunction are covered
// across this bead's suite).
func TestAdoptSpec_HappyPath(t *testing.T) {
	dir := adoptInitRepo(t)
	makeLandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, metadata := wireAdoptSeams(t, "epic-1")
	adoptStubExport(t)

	preTip := adoptRefHash(t, dir, "main")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "landed via an external GH merge"})
	if err != nil {
		t.Fatalf("AdoptSpec: unexpected error: %v", err)
	}
	if !result.Verified {
		t.Fatal("expected a VERIFIED adopt result")
	}
	if *closedCalls != 1 {
		t.Fatalf("expected the epic to be closed exactly once, got %d", *closedCalls)
	}

	// R1(f): no third terminal shape — the durable done-state markers
	// must equal a normal `impl approve`'s marker-for-marker (impl.go's
	// MUTATION (2/3): "mindspec_phase": "done", "mindspec_done": true).
	if metadata["mindspec_phase"] != "done" {
		t.Errorf("mindspec_phase = %v, want %q (impl approve parity)", metadata["mindspec_phase"], "done")
	}
	if metadata["mindspec_done"] != true {
		t.Errorf("mindspec_done = %v, want true (impl approve parity)", metadata["mindspec_done"])
	}
	// Adopt-specific audit keys.
	if metadata["mindspec_adopt_evidence"] != "verified" {
		t.Errorf("mindspec_adopt_evidence = %v, want %q", metadata["mindspec_adopt_evidence"], "verified")
	}
	if metadata["mindspec_adopt_reason"] != "landed via an external GH merge" {
		t.Errorf("mindspec_adopt_reason = %v, want the supplied reason", metadata["mindspec_adopt_reason"])
	}
	for _, k := range []string{"mindspec_adopt_actor", "mindspec_adopt_at", "mindspec_adopt_op"} {
		if s, ok := metadata[k].(string); !ok || s == "" {
			t.Errorf("expected a non-empty %s, got %v", k, metadata[k])
		}
	}
	if _, present := metadata["mindspec_adopt_attest_trigger"]; present {
		t.Error("a VERIFIED adopt must not record an attestation trigger")
	}

	// No spec->main merge: exactly ONE new commit landed on main, with
	// exactly one parent (never a merge), touching only the finalize
	// export artifact.
	newCount := adoptCommitCount(t, dir, preTip, "main")
	if newCount != 1 {
		t.Fatalf("expected exactly 1 new commit on main, got %d", newCount)
	}
	parents := adoptParentCount(t, dir, "main")
	if parents != 1 {
		t.Fatalf("adopt's finalize-export commit must have exactly 1 parent (never a merge), got %d", parents)
	}
	changed := adoptChangedPaths(t, dir, preTip, "main")
	if len(changed) != 1 || changed[0] != ".beads/issues.jsonl" {
		t.Fatalf("main-side delta must be confined to the finalize-export artifact, got %v", changed)
	}
}

// TestAdoptSpec_ParityWithNormalApproveImpl is R1(f)/G1-B3-06/O2-2's
// REAL comparison fixture — the prior shipped test asserted two
// independently hand-written literals ("done"/true) that were never
// coupled to ApproveImpl's actual output; a mutation adding a third key
// to ApproveImpl's real MUTATION(2/3) write left it green. This test
// drives a REAL ApproveImpl call (impl_test.go's own
// saveAndRestore/writePlanWithBeads harness — the exact fixture
// TestApproveImpl_HappyPath uses) and a REAL AdoptSpec call over an
// independent fixture, captures BOTH calls' ACTUAL metadata writes, and
// diffs them.
//
// The compared key set is DERIVED from ApproveImpl's own captured
// output (never a hand-listed subset, the defect class bead 2's fixtures
// were repeatedly reworked to stop committing): every key ApproveImpl's
// done-state write touches must be present in AdoptSpec's terminal
// metadata with an EQUAL value. AdoptSpec's own audit-only keys
// (mindspec_adopt_*) are correctly ABSENT from ApproveImpl's write and
// therefore never checked — R1(f) requires parity on the shared
// terminal-state markers, not identity of the whole map.
func TestAdoptSpec_ParityWithNormalApproveImpl(t *testing.T) {
	// --- side A: a real ApproveImpl run, capturing its ACTUAL write.
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	if err := os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			payload := []map[string]string{{"status": "closed"}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	approveMetadata := map[string]interface{}{}
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		for k, v := range updates {
			approveMetadata[k] = v
		}
		return nil
	}

	approveExec := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}
	if _, err := ApproveImpl(tmp, "010-test", approveExec); err != nil {
		t.Fatalf("ApproveImpl: unexpected error: %v", err)
	}
	if len(approveMetadata) == 0 {
		t.Fatal("ApproveImpl never wrote any phase metadata — fixture is broken, this test would pass vacuously")
	}
	if approveMetadata["mindspec_phase"] != "done" || approveMetadata["mindspec_done"] != true {
		t.Fatalf("fixture sanity: ApproveImpl's final captured write = %v, want a done-state map (setup drifted from TestApproveImpl_HappyPath's own fixture)", approveMetadata)
	}

	// --- side B: a real AdoptSpec run over an INDEPENDENT fixture,
	// capturing ITS actual write.
	dir := adoptInitRepo(t)
	makeLandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	_, adoptMetadata := wireAdoptSeams(t, "epic-1")
	adoptStubExport(t)
	adoptExec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}
	if _, err := AdoptSpec(dir, "042-test", adoptExec, AdoptOpts{Reason: "landed via an external GH merge"}); err != nil {
		t.Fatalf("AdoptSpec: unexpected error: %v", err)
	}

	// --- the actual comparison, derived from side A's real output.
	for k, wantV := range approveMetadata {
		gotV, ok := adoptMetadata[k]
		if !ok {
			t.Errorf("ApproveImpl's done-state write sets %q = %v, but AdoptSpec's terminal metadata never sets it — R1(f) parity broken", k, wantV)
			continue
		}
		if gotV != wantV {
			t.Errorf("done-state parity broken at key %q: ApproveImpl wrote %v, AdoptSpec wrote %v", k, wantV, gotV)
		}
	}
}

// TestAdoptSpec_DoesNotCommitOperatorUnrelatedWork is G1-B3-01: the
// finalize-export commit must be CONFINED to the export artifact
// (exec.CommitPaths, staging only ".beads/issues.jsonl") — never
// `git add -A`, which would silently sweep an operator's unrelated dirty
// work in root into the terminal adoption commit. Reproduces G1's exact
// adversary shape (an untracked file present before an otherwise-
// verified AdoptSpec call), plus the tracked-modified and staged
// variants the ruling additionally required.
func TestAdoptSpec_DoesNotCommitOperatorUnrelatedWork(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
		wireAdoptSeams(t, "epic-1")
		adoptStubExport(t)
		return dir
	}

	assertConfined := func(t *testing.T, dir, preTip string) {
		t.Helper()
		changed := adoptChangedPaths(t, dir, preTip, "main")
		if len(changed) != 1 || changed[0] != ".beads/issues.jsonl" {
			t.Fatalf("main-side delta must be confined to the finalize-export artifact even with a dirty workdir, got %v", changed)
		}
	}

	t.Run("untracked_file", func(t *testing.T) {
		dir := setup(t)
		unrelated := filepath.Join(dir, "operator-unrelated.txt")
		const content = "operator's own in-progress work\n"
		if err := os.WriteFile(unrelated, []byte(content), 0o644); err != nil {
			t.Fatalf("write unrelated file: %v", err)
		}
		preTip := adoptRefHash(t, dir, "main")
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "dirty workdir"})
		if err != nil {
			t.Fatalf("AdoptSpec: unexpected error: %v", err)
		}
		if !result.Verified {
			t.Fatal("expected a VERIFIED adopt result")
		}
		assertConfined(t, dir, preTip)

		status := adoptGitStatusPorcelain(t, dir)
		if !strings.Contains(status, "?? operator-unrelated.txt") {
			t.Fatalf("expected operator-unrelated.txt to remain untracked (git status --porcelain):\n%s", status)
		}
		got, err := os.ReadFile(unrelated)
		if err != nil || string(got) != content {
			t.Fatalf("operator-unrelated.txt content must be byte-identical afterward, got %q err=%v", got, err)
		}
	})

	t.Run("tracked_modified_file", func(t *testing.T) {
		dir := setup(t)
		readmePath := filepath.Join(dir, "README.md")
		const content = "operator's own unrelated edit\n"
		if err := os.WriteFile(readmePath, []byte(content), 0o644); err != nil {
			t.Fatalf("write README: %v", err)
		}
		preTip := adoptRefHash(t, dir, "main")
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "dirty workdir"})
		if err != nil {
			t.Fatalf("AdoptSpec: unexpected error: %v", err)
		}
		if !result.Verified {
			t.Fatal("expected a VERIFIED adopt result")
		}
		assertConfined(t, dir, preTip)

		status := adoptGitStatusPorcelain(t, dir)
		if !strings.Contains(status, " M README.md") {
			t.Fatalf("expected README.md to remain modified-but-uncommitted (git status --porcelain):\n%s", status)
		}
		got, err := os.ReadFile(readmePath)
		if err != nil || string(got) != content {
			t.Fatalf("README.md's working-tree content must be byte-identical afterward, got %q err=%v", got, err)
		}
	})

	t.Run("staged_file", func(t *testing.T) {
		dir := setup(t)
		stagedPath := filepath.Join(dir, "operator-staged.txt")
		if err := os.WriteFile(stagedPath, []byte("operator's own staged work\n"), 0o644); err != nil {
			t.Fatalf("write staged file: %v", err)
		}
		adoptGitRun(t, dir, "add", "operator-staged.txt")
		preTip := adoptRefHash(t, dir, "main")
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "dirty workdir"})
		if err != nil {
			t.Fatalf("AdoptSpec: unexpected error: %v", err)
		}
		if !result.Verified {
			t.Fatal("expected a VERIFIED adopt result")
		}
		assertConfined(t, dir, preTip)

		status := adoptGitStatusPorcelain(t, dir)
		if !strings.Contains(status, "A  operator-staged.txt") {
			t.Fatalf("expected operator-staged.txt to remain STAGED but uncommitted — never swept into adopt's own commit (git status --porcelain):\n%s", status)
		}
	})

	t.Run("clean_workdir_control", func(t *testing.T) {
		// The negative control: with nothing else dirty, the delta is
		// (trivially) still confined — TestAdoptSpec_HappyPath already
		// covers this path in full; repeated here only so the four
		// fixtures the ruling named (tracked-modified, untracked, staged,
		// clean) are all present in ONE place, by name.
		dir := setup(t)
		preTip := adoptRefHash(t, dir, "main")
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "clean workdir"})
		if err != nil {
			t.Fatalf("AdoptSpec: unexpected error: %v", err)
		}
		if !result.Verified {
			t.Fatal("expected a VERIFIED adopt result")
		}
		assertConfined(t, dir, preTip)
	})
}

// TestAdoptSpec_ReviewStateGate is G1-B3-02: the review-state
// precondition, equivalent in strength to ApproveImpl's own review/done
// phase gate. Fixtures for plan, implement, review (via the HappyPath /
// AttestationEscape tests' default wiring), and done, plus the explicit
// open-lifecycle-bead refusal.
func TestAdoptSpec_ReviewStateGate(t *testing.T) {
	t.Run("plan_phase_refuses", func(t *testing.T) {
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
		closedCalls, _ := wireAdoptSeams(t, "epic-1")
		adoptDerivePhaseDetailFn = func(id string) (phase.PhaseDetail, error) {
			return phase.PhaseDetail{EpicID: id, Stored: "", Derived: state.ModePlan}, nil
		}
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "spec still in plan phase"})
		if err == nil {
			t.Fatal("expected a refusal when the spec's phase is plan")
		}
		if !strings.Contains(err.Error(), "review-state spec") {
			t.Errorf("expected the refusal to name the review-state precondition, got: %v", err)
		}
		if *closedCalls != 0 {
			t.Fatal("a refusal must perform no mutation")
		}
	})

	t.Run("implement_phase_refuses", func(t *testing.T) {
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
		closedCalls, _ := wireAdoptSeams(t, "epic-1")
		adoptDerivePhaseDetailFn = func(id string) (phase.PhaseDetail, error) {
			return phase.PhaseDetail{EpicID: id, Stored: "", Derived: state.ModeImplement}, nil
		}
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "spec still in implement phase"})
		if err == nil {
			t.Fatal("expected a refusal when the spec's phase is implement")
		}
		if !strings.Contains(err.Error(), "review-state spec") {
			t.Errorf("expected the refusal to name the review-state precondition, got: %v", err)
		}
		if *closedCalls != 0 {
			t.Fatal("a refusal must perform no mutation")
		}
	})

	t.Run("done_phase_passes_the_gate", func(t *testing.T) {
		// "done" satisfies adoptPhaseGateOK exactly like ApproveImpl's own
		// implGateOK — a resumable-terminal-state allowance. This fixture
		// pins ONLY the gate check itself; adoptFinalize's own separate
		// idempotency semantics (G1-B3-03) govern what actually happens
		// next, covered by the dedicated already-done/already-adopted
		// tests above.
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
		closedCalls, metadata := wireAdoptSeams(t, "epic-1")
		adoptStubExport(t)
		adoptDerivePhaseDetailFn = func(id string) (phase.PhaseDetail, error) {
			return phase.PhaseDetail{EpicID: id, Stored: "", Derived: state.ModeDone}, nil
		}
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "done phase satisfies the gate"})
		if err != nil {
			t.Fatalf("AdoptSpec: unexpected error: %v", err)
		}
		if !result.Verified {
			t.Fatal("expected a VERIFIED adopt result")
		}
		if *closedCalls != 1 {
			t.Fatalf("expected the epic closed exactly once, got %d", *closedCalls)
		}
		if metadata["mindspec_adopt_reason"] != "done phase satisfies the gate" {
			t.Errorf("expected the adopt audit reason to be recorded, got %v", metadata["mindspec_adopt_reason"])
		}
	})

	t.Run("open_lifecycle_bead_refuses_even_with_landed_branch_evidence", func(t *testing.T) {
		// The G1-B3-02 adversary shape: an open lifecycle bead whose
		// branch is ALREADY landed in main (positive evidence) must still
		// refuse — landed content does not excuse an unclosed lifecycle
		// bead. This is the explicit check that never trusts a
		// stale/incorrect stored-phase cache.
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
		closedCalls, _ := wireAdoptSeams(t, "epic-1")
		adoptLifecycleChildIDsFn = func(id string) ([]string, error) { return []string{"test-b1"}, nil }
		adoptReadBeadStatusFn = func(id string) (string, error) { return "open", nil }
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "open lifecycle bead present"})
		if err == nil {
			t.Fatal("expected a refusal over an open lifecycle bead, even with landed branch evidence")
		}
		if !strings.Contains(err.Error(), "open lifecycle bead") {
			t.Errorf("expected the refusal to name the open lifecycle bead, got: %v", err)
		}
		if !strings.Contains(err.Error(), "test-b1") {
			t.Errorf("expected the refusal to name the specific bead, got: %v", err)
		}
		if *closedCalls != 0 {
			t.Fatal("a refusal must perform no mutation: the epic must not be closed")
		}
	})
}

// TestAdoptSpec_ReviewStateGate_RealClassificationAllowsNonLifecycleFollowup
// is G1-B3-02's integration proof, using the REAL internal/phase
// classification (phase.DerivePhaseDetail / phase.LifecycleChildIDsForEpic,
// wired through phase's OWN bd-stub seams — never this package's local
// override) rather than a hand-simulated seam: an epic whose lifecycle
// child is closed but which ALSO carries an OPEN non-lifecycle follow-up
// (a `bug` filed against the spec epic post-implementation) must still
// satisfy the review-state gate and the open-lifecycle-bead check — the
// follow-up is correctly excluded from BOTH by the real classifier,
// proving the "allowed non-lifecycle follow-up" claim against genuine
// classification logic, not a fixture that merely asserts it.
func TestAdoptSpec_ReviewStateGate_RealClassificationAllowsNonLifecycleFollowup(t *testing.T) {
	dir := adoptInitRepo(t)
	makeLandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, metadata := wireAdoptSeams(t, "epic-1")
	adoptStubExport(t)

	// Route through the REAL phase package functions for this one test.
	origPhase := adoptDerivePhaseDetailFn
	adoptDerivePhaseDetailFn = phase.DerivePhaseDetail
	t.Cleanup(func() { adoptDerivePhaseDetailFn = origPhase })
	origLifecycle := adoptLifecycleChildIDsFn
	adoptLifecycleChildIDsFn = phase.LifecycleChildIDsForEpic
	t.Cleanup(func() { adoptLifecycleChildIDsFn = origLifecycle })

	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		children := []phase.ChildInfo{
			{ID: "test-b1", Status: "closed", IssueType: "task"},
			{ID: "bug-1", Status: "open", IssueType: "bug"},
		}
		return json.Marshal(children)
	})
	t.Cleanup(restoreList)

	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}
	result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "real classification excludes the open non-lifecycle follow-up"})
	if err != nil {
		t.Fatalf("AdoptSpec: unexpected error (an open non-lifecycle follow-up must never block): %v", err)
	}
	if !result.Verified {
		t.Fatal("expected a VERIFIED adopt result")
	}
	if *closedCalls != 1 {
		t.Fatalf("expected the epic closed exactly once, got %d", *closedCalls)
	}
	if metadata["mindspec_adopt_reason"] == nil {
		t.Error("expected the adopt audit reason to be recorded")
	}
}

// TestAdoptSpec_NoReasonRefusesBeforeMutation is AC-2(i).
func TestAdoptSpec_NoReasonRefusesBeforeMutation(t *testing.T) {
	dir := adoptInitRepo(t)
	closedCalls, _ := wireAdoptSeams(t, "epic-1")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{})
	if err == nil {
		t.Fatal("expected a refusal with no --reason")
	}
	if !strings.Contains(err.Error(), "--reason") {
		t.Errorf("expected the refusal to name --reason, got: %v", err)
	}
	if *closedCalls != 0 {
		t.Fatal("no --reason must perform NO mutation: the epic must not be closed")
	}
	if exec.calls != 0 {
		t.Fatal("no --reason must perform NO mutation: CommitPaths must never be called")
	}
}

// TestAdoptSpec_BranchAbsentNoSourceRefusesAndNamesAttestation is
// AC-2(ii).
func TestAdoptSpec_BranchAbsentNoSourceRefusesAndNamesAttestation(t *testing.T) {
	dir := adoptInitRepo(t)
	stubAdoptListEpicBeads(t, []adoptEpicBead{})
	closedCalls, _ := wireAdoptSeams(t, "epic-1")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "no evidence anywhere"})
	if err == nil {
		t.Fatal("expected a refusal with no ref-bearing evidence source")
	}
	if !strings.Contains(err.Error(), "inspect") && !strings.Contains(err.Error(), "git log") {
		t.Errorf("expected the refusal to name inspection, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--attest-unverified") {
		t.Errorf("expected the refusal to name the attestation flag, got: %v", err)
	}
	if *closedCalls != 0 {
		t.Fatal("a refusal must perform no mutation")
	}
}

// TestAdoptSpec_AttestationEscape is AC-2(iii): two fixtures — a
// no-source trigger, and a second that attests past a NEGATIVE
// aggregate — each asserting the marker records which trigger state was
// exercised.
func TestAdoptSpec_AttestationEscape(t *testing.T) {
	t.Run("no_source_trigger", func(t *testing.T) {
		dir := adoptInitRepo(t)
		stubAdoptListEpicBeads(t, []adoptEpicBead{})
		_, metadata := wireAdoptSeams(t, "epic-1")
		adoptStubExport(t)
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "attesting with no evidence", AttestUnverified: true})
		if err != nil {
			t.Fatalf("AdoptSpec with --attest-unverified: unexpected error: %v", err)
		}
		if result.Verified {
			t.Fatal("an attested adopt must never report Verified")
		}
		if metadata["mindspec_adopt_evidence"] != "attested" {
			t.Errorf("mindspec_adopt_evidence = %v, want %q", metadata["mindspec_adopt_evidence"], "attested")
		}
		if metadata["mindspec_adopt_attest_trigger"] != "no-source" {
			t.Errorf("mindspec_adopt_attest_trigger = %v, want %q", metadata["mindspec_adopt_attest_trigger"], "no-source")
		}
	})

	t.Run("attest_past_negative_trigger", func(t *testing.T) {
		dir := adoptInitRepo(t)
		makeUnlandedBeadBranch(t, dir, "test-b1")
		// Status "open" (not "closed"): a CLOSED bead's stale surviving
		// branch is the composite-incident shape (R1g), which AdoptSpec
		// intercepts with its own interim refusal BEFORE ever reaching
		// the lattice — this fixture wants the plain per-bead-negative
		// lattice path instead.
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "open"}})
		_, metadata := wireAdoptSeams(t, "epic-1")
		adoptStubExport(t)
		exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

		result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "attesting past a stale bead branch", AttestUnverified: true})
		if err != nil {
			t.Fatalf("AdoptSpec with --attest-unverified: unexpected error: %v", err)
		}
		if result.Verified {
			t.Fatal("an attested adopt must never report Verified")
		}
		if metadata["mindspec_adopt_attest_trigger"] != "negative" {
			t.Errorf("mindspec_adopt_attest_trigger = %v, want %q (attesting past NEGATIVE evidence is a distinct, more suspicious trigger than no-source)", metadata["mindspec_adopt_attest_trigger"], "negative")
		}
	})
}

// TestAdoptSpec_BranchPresentCurrentNamesNormalPath is AC-2(iv).
func TestAdoptSpec_BranchPresentCurrentNamesNormalPath(t *testing.T) {
	dir := adoptInitRepo(t)
	adoptGitRun(t, dir, "branch", "spec/042-test", "main")
	closedCalls, _ := wireAdoptSeams(t, "epic-1")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "branch still here"})
	if err == nil {
		t.Fatal("expected a refusal when the local spec branch is present and current")
	}
	if !strings.Contains(err.Error(), "mindspec impl approve 042-test") {
		t.Errorf("expected the refusal to name the normal `impl approve` path, got: %v", err)
	}
	if strings.Contains(err.Error(), "git branch -D") {
		t.Error("a current-branch refusal must never suggest deleting the branch")
	}
	if *closedCalls != 0 {
		t.Fatal("a refusal must perform no mutation")
	}
}

// TestAdoptSpec_BranchPresentStaleNamesDeletionAndRerun is AC-2(v): the
// #218 step-2 recreated-branch shape. The deletion hint must be
// constructor-derived (never a bare string) and the message must NOT say
// the normal path applies.
func TestAdoptSpec_BranchPresentStaleNamesDeletionAndRerun(t *testing.T) {
	dir := adoptInitRepo(t)
	makeStaleRecreatedBranch(t, dir, "spec/042-test")

	closedCalls, _ := wireAdoptSeams(t, "epic-1")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "recreated stale branch"})
	if err == nil {
		t.Fatal("expected a refusal for a stale spec branch")
	}
	msg := err.Error()
	if !strings.Contains(msg, "git branch -D spec/042-test") {
		t.Errorf("expected a constructor-derived `git branch -D spec/042-test` deletion hint, got: %v", msg)
	}
	if strings.Contains(msg, "the normal path applies") {
		t.Error("a stale-branch refusal must never say the normal path applies")
	}
	if !strings.Contains(msg, "mindspec impl adopt 042-test") {
		t.Errorf("expected the refusal to name an adopt re-run, got: %v", msg)
	}
	if *closedCalls != 0 {
		t.Fatal("a refusal must perform no mutation")
	}
}

// TestAdoptSpec_EvidenceErrorNamesRetryFirstThenAttestation is AC-2(vi).
func TestAdoptSpec_EvidenceErrorNamesRetryFirstThenAttestation(t *testing.T) {
	dir := adoptInitRepo(t)
	errBranch := makeUnlandedBeadBranch(t, dir, "test-b1")
	forceAdoptEvalError(t, map[string]error{errBranch: errors.New("simulated probe failure")})
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, _ := wireAdoptSeams(t, "epic-1")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "forced evidence error"})
	if err == nil {
		t.Fatal("expected a fail-closed refusal on a forced evidence error")
	}
	msg := err.Error()
	if !strings.Contains(msg, adoptMarkerEvidenceError) {
		t.Errorf("expected the evidence-error marker in the message, got: %v", msg)
	}
	retryIdx := strings.Index(msg, "retry once")
	attestIdx := strings.Index(msg, "--attest-unverified")
	if retryIdx < 0 || attestIdx < 0 || retryIdx > attestIdx {
		t.Errorf("expected retry to be named BEFORE the attestation escape, got: %v", msg)
	}
	if *closedCalls != 0 {
		t.Fatal("a refusal must perform no mutation")
	}
}

// TestAdoptSpec_CompositeIncidentInterimRefusal is AC-2(viii)'s bead-3
// slice: adopt refuses before mutation over the composite incident state
// (a closed bead's stale bead branch still present), with the INTERIM
// inspection-first wording and NO destructive command (bead 4 upgrades
// this to the full R2-derived hint).
func TestAdoptSpec_CompositeIncidentInterimRefusal(t *testing.T) {
	dir := adoptInitRepo(t)
	makeUnlandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, _ := wireAdoptSeams(t, "epic-1")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "composite incident"})
	if err == nil {
		t.Fatal("expected a refusal over the composite incident state")
	}
	msg := err.Error()
	if strings.Contains(msg, "mindspec complete") {
		t.Error("the orphan-present refusal must never suggest mindspec complete")
	}
	for _, floorCmd := range []string{"git branch -D", "rm -rf", "git reset"} {
		if strings.Contains(msg, floorCmd) {
			t.Errorf("bead 3's INTERIM orphan-present refusal must carry no destructive command, found %q in: %v", floorCmd, msg)
		}
	}
	if !strings.Contains(msg, "git diff") {
		t.Errorf("expected an inspection-first git diff command, got: %v", msg)
	}
	if *closedCalls != 0 {
		t.Fatal("a refusal must perform no mutation")
	}
}

// TestAdoptSpec_NoRefusalMutatesEpicRefsOrMetadata is a cross-cutting
// no-mutation guard over every refusal leg above: `git for-each-ref`
// before/after must be byte-identical (O2-8's observables), proven here
// for the stale-branch leg (the one leg whose refusal RENDERS a
// destructive command — the sharpest test of "renders, never runs").
func TestAdoptSpec_NoRefusalMutatesEpicRefsOrMetadata(t *testing.T) {
	dir := adoptInitRepo(t)
	makeStaleRecreatedBranch(t, dir, "spec/042-test")
	wireAdoptSeams(t, "epic-1")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	before := adoptForEachRef(t, dir)
	if _, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "stale branch present"}); err == nil {
		t.Fatal("expected a refusal")
	}
	after := adoptForEachRef(t, dir)
	if before != after {
		t.Fatalf("a refusal that RENDERS a `git branch -D` recovery line must never RUN it:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if exec.calls != 0 {
		t.Fatal("a refusal must never call CommitPaths")
	}
}

// TestAdoptSpec_SecondCallRefusesAndPreservesFirstAuditPayload is
// G1-B3-03: adopt is a ONE-SHOT terminal transition. A second successful
// call must refuse rather than silently repeating (and thereby
// overwriting) the first call's recorded reason/actor/timestamp/
// evidence-class, and must perform NO further mutation (no second epic
// close, no second commit).
func TestAdoptSpec_SecondCallRefusesAndPreservesFirstAuditPayload(t *testing.T) {
	dir := adoptInitRepo(t)
	makeLandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, metadata := wireAdoptSeams(t, "epic-1")
	adoptStubExport(t)
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	if _, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "first run's genuine reason"}); err != nil {
		t.Fatalf("first AdoptSpec call: unexpected error: %v", err)
	}
	if *closedCalls != 1 {
		t.Fatalf("expected the epic closed exactly once after the first call, got %d", *closedCalls)
	}
	firstReason := metadata["mindspec_adopt_reason"]
	firstActor := metadata["mindspec_adopt_actor"]
	firstAt := metadata["mindspec_adopt_at"]
	preTip := adoptRefHash(t, dir, "main")

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "a SECOND, different reason that must never land"})
	if err == nil {
		t.Fatal("expected the second AdoptSpec call to refuse")
	}
	if !strings.Contains(err.Error(), "already adopted") {
		t.Errorf("expected the refusal to name the already-adopted state, got: %v", err)
	}
	if *closedCalls != 1 {
		t.Fatalf("the second call must NOT close the epic again, got %d total closes", *closedCalls)
	}
	if metadata["mindspec_adopt_reason"] != firstReason {
		t.Errorf("mindspec_adopt_reason changed from %v to %v — the first run's audit payload must be preserved verbatim", firstReason, metadata["mindspec_adopt_reason"])
	}
	if metadata["mindspec_adopt_actor"] != firstActor || metadata["mindspec_adopt_at"] != firstAt {
		t.Errorf("mindspec_adopt_actor/at changed — the first run's audit payload must be preserved verbatim")
	}
	// No new commit landed on main.
	newCount := adoptCommitCount(t, dir, preTip, "main")
	if newCount != 0 {
		t.Fatalf("the second call must not create a new commit, got %d new commits", newCount)
	}
}

// TestAdoptSpec_ResumesInterruptedFinalizeWithoutRerunningStage1 is
// G1-B3-03's crash-recovery leg: a PRIOR run that completed stage 1
// (close + audit metadata) but never reached stage 2 (export + commit,
// e.g. a CommitPaths failure) must be RESUMABLE — a re-run performs
// ONLY the missing stage 2, never re-closing the epic or re-merging
// (and thereby overwriting) stage 1's audit metadata.
func TestAdoptSpec_ResumesInterruptedFinalizeWithoutRerunningStage1(t *testing.T) {
	dir := adoptInitRepo(t)
	makeLandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, metadata := wireAdoptSeams(t, "epic-1")
	adoptStubExport(t)

	// Simulate an INTERRUPTED prior run: stage 1 already completed
	// (recorded reason/actor/timestamp/evidence class present) but stage
	// 2 never ran (no export-committed marker, and main never got the
	// finalize-export commit).
	metadata["mindspec_phase"] = "done"
	metadata["mindspec_done"] = true
	metadata["mindspec_adopt_reason"] = "the original, already-recorded reason"
	metadata["mindspec_adopt_actor"] = "original-actor@original-host via mindspec"
	metadata["mindspec_adopt_at"] = "2020-01-01T00:00:00Z"
	metadata["mindspec_adopt_op"] = "mindspec impl adopt"
	metadata["mindspec_adopt_evidence"] = "verified"

	preTip := adoptRefHash(t, dir, "main")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	if _, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "a resume-time reason, must not overwrite the original"}); err != nil {
		t.Fatalf("AdoptSpec (resume): unexpected error: %v", err)
	}

	// Stage 1 must NOT have re-run: no close call, original audit payload
	// untouched.
	if *closedCalls != 0 {
		t.Fatalf("a resumed run must not re-close the epic (stage 1 already completed on the interrupted prior run), got %d closes", *closedCalls)
	}
	if metadata["mindspec_adopt_reason"] != "the original, already-recorded reason" {
		t.Errorf("mindspec_adopt_reason = %v, want the ORIGINAL reason preserved verbatim (resume must never re-run stage 1)", metadata["mindspec_adopt_reason"])
	}
	if metadata["mindspec_adopt_at"] != "2020-01-01T00:00:00Z" {
		t.Errorf("mindspec_adopt_at = %v, want the ORIGINAL timestamp preserved verbatim", metadata["mindspec_adopt_at"])
	}

	// Stage 2 MUST have run: the export-committed marker is now set, and
	// exactly one new commit landed on main touching only the export
	// artifact.
	if metadata[adoptExportCommittedMetaKey] != true {
		t.Error("expected the export-committed marker to be set after the resume completes stage 2")
	}
	if exec.calls != 1 {
		t.Fatalf("expected stage 2's CommitPaths to run exactly once on resume, got %d calls", exec.calls)
	}
	newCount := adoptCommitCount(t, dir, preTip, "main")
	if newCount != 1 {
		t.Fatalf("expected exactly 1 new commit on main from the resumed stage 2, got %d", newCount)
	}
	changed := adoptChangedPaths(t, dir, preTip, "main")
	if len(changed) != 1 || changed[0] != ".beads/issues.jsonl" {
		t.Fatalf("the resumed stage-2 commit must still be confined to the export artifact, got %v", changed)
	}
}

// TestAdoptSpec_AlreadyDoneViaNormalPathRefuses is G1-B3-03: an epic that
// already reached "done" via the NORMAL `impl approve` path (mindspec_done
// present, but no adopt audit marker) must be refused — adopt is not a
// resume mechanism for a terminal shape it did not itself produce, and
// must never overwrite it or perform a redundant close/commit.
func TestAdoptSpec_AlreadyDoneViaNormalPathRefuses(t *testing.T) {
	dir := adoptInitRepo(t)
	makeLandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, metadata := wireAdoptSeams(t, "epic-1")
	adoptStubExport(t)

	metadata["mindspec_phase"] = "done"
	metadata["mindspec_done"] = true
	// Deliberately NO mindspec_adopt_reason: this is the normal-path
	// done-state shape, never adopt's.

	preTip := adoptRefHash(t, dir, "main")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t}

	_, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "trying to adopt an already normally-done spec"})
	if err == nil {
		t.Fatal("expected a refusal over an already-done-via-normal-path epic")
	}
	if !strings.Contains(err.Error(), "already reached the done state via the normal path") {
		t.Errorf("expected the refusal to name the normal-path done state, got: %v", err)
	}
	if *closedCalls != 0 {
		t.Fatal("a refusal must perform no mutation: the epic must not be closed")
	}
	if exec.calls != 0 {
		t.Fatal("a refusal must perform no mutation: CommitPaths must never be called")
	}
	newCount := adoptCommitCount(t, dir, preTip, "main")
	if newCount != 0 {
		t.Fatalf("a refusal must create no new commit, got %d", newCount)
	}
}

// --- small git-plumbing assertions used above --------------------------

func adoptCommitCount(t *testing.T, dir, base, head string) int {
	t.Helper()
	out, err := adoptGitRunAllowFail(dir, "rev-list", "--count", base+".."+head)
	if err != nil {
		t.Fatalf("rev-list --count %s..%s: %v\n%s", base, head, err, out)
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n); err != nil {
		t.Fatalf("parsing rev-list count %q: %v", out, err)
	}
	return n
}

func adoptParentCount(t *testing.T, dir, ref string) int {
	t.Helper()
	out, err := adoptGitRunAllowFail(dir, "rev-list", "--parents", "-n", "1", ref)
	if err != nil {
		t.Fatalf("rev-list --parents -n1 %s: %v\n%s", ref, err, out)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	return len(fields) - 1
}

func adoptChangedPaths(t *testing.T, dir, base, head string) []string {
	t.Helper()
	out, err := adoptGitRunAllowFail(dir, "diff", "--name-only", base, head)
	if err != nil {
		t.Fatalf("diff --name-only %s %s: %v\n%s", base, head, err, out)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func adoptForEachRef(t *testing.T, dir string) string {
	t.Helper()
	out, err := adoptGitRunAllowFail(dir, "for-each-ref")
	if err != nil {
		t.Fatalf("for-each-ref: %v\n%s", err, out)
	}
	return string(out)
}
