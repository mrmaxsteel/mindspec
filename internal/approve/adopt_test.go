package approve

// Spec 127 bead 3 — AC-1 (adopt happy path) and AC-2 (adopt refusal
// legs) driven through the full AdoptSpec entrypoint, real git
// throughout, bd stubbed via this package's in-package seams.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// realAdoptCommitExecutor wraps a bare MockExecutor and, for CommitAll
// ONLY, performs a REAL git add+commit in path — the
// killAfterPlanCommitExecutor house pattern (plan_fault_test.go) applied
// to adopt's finalize-export commit. It writes a deterministic
// .beads/issues.jsonl change first (standing in for production's
// bead.Export refresh, which this test never shells out to) so there is
// a real, inspectable finalize-export delta to assert against.
type realAdoptCommitExecutor struct {
	*executor.MockExecutor
	t     *testing.T
	calls int
}

func (e *realAdoptCommitExecutor) CommitAll(path, msg string) error {
	e.calls++
	beadsDir := filepath.Join(path, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`{"id":"epic-1","status":"closed","adopt_call":%d}`+"\n", e.calls)
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(content), 0o644); err != nil {
		return err
	}
	adoptGitRun(e.t, path, "add", "-A")
	out, err := adoptGitRunAllowFail(path, "commit", "-q", "-m", msg)
	if err != nil && !strings.Contains(string(out), "nothing to commit") {
		return fmt.Errorf("git commit: %w\n%s", err, out)
	}
	return nil
}

// wireAdoptSeams stubs adoptFindEpicFn/adoptRunBDCombinedFn/
// adoptMergeMetadataFn so AdoptSpec never shells to a real bd: epicID is
// fixed, "close" is recorded (never actually run), and metadata merges
// into an in-memory map the test can inspect afterward.
func wireAdoptSeams(t *testing.T, epicID string) (closedCalls *int, metadata map[string]interface{}) {
	t.Helper()
	metadata = map[string]interface{}{}
	closed := 0

	origFindEpic := adoptFindEpicFn
	adoptFindEpicFn = func(specID string) (string, error) { return epicID, nil }
	t.Cleanup(func() { adoptFindEpicFn = origFindEpic })

	origCombined := adoptRunBDCombinedFn
	adoptRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "close" {
			closed++
		}
		return nil, nil
	}
	t.Cleanup(func() { adoptRunBDCombinedFn = origCombined })

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
		t.Fatal("no --reason must perform NO mutation: CommitAll must never be called")
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
		t.Fatal("a refusal must never call CommitAll")
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
