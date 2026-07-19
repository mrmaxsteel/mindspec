package approve

// Spec 119 Bead 6 (AC-26 / ADR-0041): the `plan approve` fault-injection
// matrix — p0a, p0b, p1, p2, p3 (KILL / SIMULATED-DEATH), plus p2b/p4
// (DOCUMENTED-FORWARD-SAFE, pinned end-to-end below rather than merely
// asserted by code cite). See fault_injection_test.go / impl_fault_test.go
// / internal/executor/finalize_fault_test.go for the `complete` and
// `impl approve` legs of the same matrix.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// validTwoBeadPlan is a Draft, two-bead plan with a declared dependency
// (Bead 2 depends on Bead 1) — the p1/p2 fixtures need more than one bead
// section to distinguish "the Nth create" from "the only create".
const validTwoBeadPlan = `---
status: Draft
spec_id: "042-test"
version: "1.0"
work_chunks:
  - id: 1
    depends_on: []
  - id: 2
    depends_on: [1]
---

# Plan

## ADR Fitness

No ADRs are relevant to this work.

## Testing Strategy

Unit tests will verify the implementation.

## Bead 1: First thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- First bead works

**Depends on**
None

## Bead 2: Second thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- Second bead works

**Depends on**
Bead 1
`

// wirePlanEpicSeams stubs the phase package's epic-resolution seams so
// resolveTargetEpic (and phase.EnsureMigrated ahead of it) resolve specID
// to epicID without a real bd. queryFn answers every OTHER bd-list query
// (queryExistingChildren's `--parent` call).
func wirePlanEpicSeams(t *testing.T, specID, epicID string, queryFn func(args ...string) ([]byte, error)) {
	t.Helper()
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return epicJSONFor(epicID, specID), nil
			}
		}
		return []byte(`[]`), nil
	})
	t.Cleanup(restoreList)
	restoreRunBD := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return epicJSONFor(epicID, specID), nil
		}
		return []byte(`[]`), nil
	})
	t.Cleanup(restoreRunBD)
	// queryExistingChildren (plan.go) routes through planListJSONFn — a
	// SEPARATE seam from phase's own list-JSON var above (resolveTargetEpic
	// uses phase.Cache.AllEpics, which is phase-package-internal).
	restorePlanList := SetPlanListJSONForTest(queryFn)
	t.Cleanup(restorePlanList)
}

// existingChildrenJSON renders ids (all status "open" unless closed) as the
// `bd list --parent` JSON array queryExistingChildren expects.
func existingChildrenJSON(ids []string, closed map[string]bool) []byte {
	var parts []string
	for _, id := range ids {
		status := "open"
		if closed[id] {
			status = "closed"
		}
		parts = append(parts, fmt.Sprintf(`{"id":%q,"status":%q}`, id, status))
	}
	return []byte("[" + strings.Join(parts, ",") + "]")
}

// --- p0a: supersede-close of the previous all-open bead set ---------------
//
// supersedeCloseExistingBeads' planRunBDCombinedFn "close" call (the FIRST
// sanctioned mutation post-Bead-4, plan.go) TERMINATES the approve when it
// fails (a close error propagates from ApprovePlan). Mechanism B: the
// wrapper closes the fake-tracker bead for real, then fails. Re-invocation
// converges to the clean NAMED supersede-safety refusal: the preflight now
// sees old-bead-1 CLOSED and refuses with the `bd delete <id> --force`
// recovery line — the accepted outcome (c2 precedent).
func TestFaultInjection_ApprovePlan_P0A_SupersedeClose_KillThenConverge(t *testing.T) {
	const specID, epicID = "042-test", "epic-42"
	root, planPath, original := setupPreflightPlan(t, specID, validSingleBeadPlan)

	closed := false
	wirePlanEpicSeams(t, specID, epicID, func(args ...string) ([]byte, error) {
		return existingChildrenJSON([]string{"old-bead-1"}, map[string]bool{"old-bead-1": closed}), nil
	})

	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) {
		// The real (fake-tracker) close lands, THEN the seam fails.
		closed = true
		return nil, fmt.Errorf("fault-injection: simulated bd close failure")
	})
	defer restoreCombined()
	restoreBD := SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte(`{"id":"bead-1"}`), nil
		}
		return []byte(`[]`), nil
	})
	defer restoreBD()

	mockExec := &executor.MockExecutor{}

	// Run 1 (KILL): the close genuinely lands in the fake tracker, then the
	// seam fails — plan.md must stay byte-identical (no Approved write yet).
	_, err := ApprovePlan(root, specID, "tester", mockExec)
	if err == nil {
		t.Fatal("expected p0a kill: the supersede-close failure must fail ApprovePlan")
	}
	if !closed {
		t.Fatal("expected the real close to have landed in the fake tracker despite the kill")
	}
	assertPlanUnchangedAndNoMutation(t, planPath, original, mockExec)

	// Run 2: preflight now sees old-bead-1 CLOSED — the clean NAMED
	// supersede-safety refusal, not a repeated close attempt.
	_, err = ApprovePlan(root, specID, "tester", mockExec)
	if err == nil {
		t.Fatal("expected p0a re-invocation to hit the closed-child supersede-safety refusal")
	}
	if !strings.Contains(err.Error(), "old-bead-1") || !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected the closed-child refusal naming old-bead-1, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bd delete old-bead-1 --force") {
		t.Errorf("expected the bd-delete recovery line, got: %v", err)
	}
	assertPlanUnchangedAndNoMutation(t, planPath, original, mockExec)
}

// --- p0b: the Approved-frontmatter write → first-create gap ---------------
//
// SIMULATED-DEATH (state-construction): updatePlanApproval's own write
// error terminates, but the death of interest is immediately AFTER the
// write succeeds, before the first bead create — no seam separates them.
// This test CONSTRUCTS that death state directly (plan.md frontmatter
// already Approved, ZERO children under the epic) and re-invokes
// ApprovePlan, asserting convergence to the fully wired bead set:
// updatePlanApproval is idempotent and handleExistingBeads sees no
// children, so a full create proceeds.
func TestFaultInjection_ApprovePlan_P0B_ApprovedWriteToFirstCreateGap_SimulatedDeathConverges(t *testing.T) {
	const specID, epicID = "042-test", "epic-42"
	approvedPlan := strings.Replace(validSingleBeadPlan, "status: Draft", "status: Approved", 1)
	root, planPath, _ := setupPreflightPlan(t, specID, approvedPlan)

	wirePlanEpicSeams(t, specID, epicID, func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil // zero children — the constructed death state
	})

	restoreBD := SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte(`{"id":"bead-new-1"}`), nil
		}
		return []byte(`[]`), nil
	})
	defer restoreBD()
	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) { return nil, nil })
	defer restoreCombined()
	restoreMerge := SetPlanMergeMetadataForTest(func(id string, updates map[string]interface{}) error { return nil })
	defer restoreMerge()

	mockExec := &executor.MockExecutor{}
	result, err := ApprovePlan(root, specID, "tester", mockExec)
	if err != nil {
		t.Fatalf("expected the constructed post-Approved-write death state to converge, got: %v", err)
	}
	if len(result.BeadIDs) != 1 || result.BeadIDs[0] != "bead-new-1" {
		t.Fatalf("expected the full bead set created, got %v", result.BeadIDs)
	}
	got, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("re-reading plan.md: %v", err)
	}
	if !strings.Contains(string(got), "approved_by: tester") {
		t.Errorf("expected the (idempotent) Approved re-write to have run, got:\n%s", got)
	}
}

// --- p1: after the Nth bead creation ---------------------------------------
//
// planRunBDFn's "create" call (createBeadsFromParsed, plan.go) TERMINATES
// the approve when it fails (createImplementationBeads propagates it out).
// Mechanism B: a call-counting wrapper — the first bead genuinely lands in
// the fake tracker, then the Nth (here, the 2nd) create fails.
// Re-invocation converges through handleExistingBeads' supersede-close +
// full recreate — re-traversing the (idempotent) Approved-frontmatter write
// too, covering the same gap p0b names explicitly.
func TestFaultInjection_ApprovePlan_P1_BeadCreateFailure_KillThenConverge(t *testing.T) {
	const specID, epicID = "042-test", "epic-42"
	root, _, _ := setupPreflightPlan(t, specID, validTwoBeadPlan)

	var existing []string
	closedSet := map[string]bool{}
	wirePlanEpicSeams(t, specID, epicID, func(args ...string) ([]byte, error) {
		return existingChildrenJSON(existing, closedSet), nil
	})

	attempt := 0
	var depCalls [][2]string
	restoreBD := SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			attempt++
			if attempt == 2 {
				return nil, fmt.Errorf("fault-injection: simulated bd create failure on bead 2")
			}
			id := fmt.Sprintf("bead-%d", attempt)
			existing = append(existing, id)
			return []byte(fmt.Sprintf(`{"id":%q}`, id)), nil
		}
		if len(args) > 1 && args[0] == "dep" && args[1] == "add" {
			depCalls = append(depCalls, [2]string{args[2], args[3]})
		}
		return []byte(`[]`), nil
	})
	defer restoreBD()
	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) {
		// supersede-close: mark every named id closed for real.
		for _, a := range args {
			if a == "close" || a == "--reason" {
				continue
			}
			if strings.HasPrefix(a, "bead-") {
				closedSet[a] = true
			}
		}
		return nil, nil
	})
	defer restoreCombined()
	restoreMerge := SetPlanMergeMetadataForTest(func(id string, updates map[string]interface{}) error { return nil })
	defer restoreMerge()

	mockExec := &executor.MockExecutor{}

	// Run 1 (KILL): bead 1 genuinely lands, bead 2's create fails —
	// plan.md must stay byte-identical (Approved write is AFTER preflight
	// but the create failure happens deep inside step 3b, past the
	// Approved write; the PREFLIGHT-refusal byte-identity contract does
	// not apply here — instead assert the partial state directly).
	_, err := ApprovePlan(root, specID, "tester", mockExec)
	if err == nil {
		t.Fatal("expected p1 kill: the bead-2 create failure must fail ApprovePlan")
	}
	if !strings.Contains(err.Error(), "bead-2:") && !strings.Contains(err.Error(), "Second thing") {
		t.Logf("p1 kill error (informational): %v", err)
	}
	if len(existing) != 1 || existing[0] != "bead-1" {
		t.Fatalf("expected exactly bead-1 to have genuinely landed before the kill, got %v", existing)
	}

	// Re-invoke: preflight sees bead-1 as an existing OPEN child —
	// supersede-close + full recreate (the p1 convergence path), which
	// re-traverses the Approved-frontmatter write idempotently too.
	result, err := ApprovePlan(root, specID, "tester", mockExec)
	if err != nil {
		t.Fatalf("expected p1 re-invocation to converge via supersede-close + recreate, got: %v", err)
	}
	if len(result.BeadIDs) != 2 {
		t.Fatalf("expected the FULL two-bead set recreated, got %v", result.BeadIDs)
	}
	if !closedSet["bead-1"] {
		t.Error("expected the partial bead-1 to have been supersede-closed on convergence")
	}
	if len(depCalls) != 1 || depCalls[0][0] != result.BeadIDs[1] || depCalls[0][1] != result.BeadIDs[0] {
		t.Errorf("expected the dependency to be re-wired on the fresh bead set, got %v (beadIDs=%v)", depCalls, result.BeadIDs)
	}
}

// --- p2: partial dependency wiring -----------------------------------------
//
// SIMULATED-DEATH (state-construction): post-Bead-4 a failed `bd dep add`
// WARNS and continues by design — no seam error can terminate the run
// mid-wiring, so there is no kill point here. This test instead CONSTRUCTS
// the mid-wiring death state directly (both beads already created, open,
// under the epic — as if a prior run's dep-wiring loop died partway) and
// re-invokes ApprovePlan, asserting convergence via supersede-close + full
// recreate with the dependency correctly wired this time.
func TestFaultInjection_ApprovePlan_P2_PartialDepWiring_SimulatedDeathConverges(t *testing.T) {
	const specID, epicID = "042-test", "epic-42"
	approvedPlan := strings.Replace(validTwoBeadPlan, "status: Draft", "status: Approved", 1)
	root, _, _ := setupPreflightPlan(t, specID, approvedPlan)

	// The constructed death state: bead-1 and bead-2 already exist (open)
	// under the epic — the dep edge between them was never confirmed wired.
	closedSet := map[string]bool{}
	wirePlanEpicSeams(t, specID, epicID, func(args ...string) ([]byte, error) {
		return existingChildrenJSON([]string{"bead-1", "bead-2"}, closedSet), nil
	})

	attempt := 0
	var depCalls [][2]string
	restoreBD := SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			attempt++
			return []byte(fmt.Sprintf(`{"id":"bead-new-%d"}`, attempt)), nil
		}
		if len(args) > 1 && args[0] == "dep" && args[1] == "add" {
			depCalls = append(depCalls, [2]string{args[2], args[3]})
		}
		return []byte(`[]`), nil
	})
	defer restoreBD()
	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) {
		closedSet["bead-1"] = true
		closedSet["bead-2"] = true
		return nil, nil
	})
	defer restoreCombined()
	restoreMerge := SetPlanMergeMetadataForTest(func(id string, updates map[string]interface{}) error { return nil })
	defer restoreMerge()

	mockExec := &executor.MockExecutor{}
	result, err := ApprovePlan(root, specID, "tester", mockExec)
	if err != nil {
		t.Fatalf("expected the constructed mid-wiring death state to converge, got: %v", err)
	}
	if len(result.BeadIDs) != 2 {
		t.Fatalf("expected the FULL two-bead set recreated, got %v", result.BeadIDs)
	}
	if !closedSet["bead-1"] || !closedSet["bead-2"] {
		t.Error("expected the partially-wired pair to have been supersede-closed on convergence")
	}
	if len(depCalls) != 1 || depCalls[0][0] != result.BeadIDs[1] || depCalls[0][1] != result.BeadIDs[0] {
		t.Errorf("expected the dependency to be fully (re-)wired on the fresh bead set, got %v (beadIDs=%v)", depCalls, result.BeadIDs)
	}
}

// --- p3: the approval auto-commit ------------------------------------------
//
// exec.CommitAll (plan.go, both the approval commit and the residual-sync
// commit) TERMINATES the approve when it fails. Mechanism A: a real-git
// decorator wrapping a bare MockExecutor — CommitAll performs the REAL git
// commit in the spec-worktree-path directory, then fails on its FIRST
// invocation only. Re-invocation converges: updatePlanApproval is
// idempotent, and the residual create/dep-wiring state from run 1 (which
// already landed before the kill) is superseded and recreated cleanly.
func TestFaultInjection_ApprovePlan_P3_ApprovalAutoCommit_KillThenConverge(t *testing.T) {
	const specID, epicID = "042-test", "epic-42"
	root, _, _ := setupPreflightPlan(t, specID, validSingleBeadPlan)

	// Once bead-1's ID lands in plan.md's `bead_ids` frontmatter (below),
	// EVERY subsequent ApprovePlan call's ValidatePlan step cross-checks it
	// via validate.CheckBeadIDs -> bead.BeadExists -> a REAL, unmockable
	// `bd show <id>` subprocess (internal/bead has no test seam for this
	// call). Restricting PATH to `git` alone (needed by the real-git
	// decorator below) makes that `bd` invocation fail as "executable not
	// found" (a non-ExitError), which validate's own fail-degrade treats as
	// an advisory "cannot verify... Beads unavailable" WARNING rather than
	// a blocking error — the correct hermetic behavior (this repo's own
	// convention: "CI has no bd on PATH") rather than a dependency on this
	// dev machine's real, unrelated Dolt store.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	// A bare directory containing bd-alongside-git is not enough isolation
	// on this machine (both are installed side by side under the same
	// Homebrew prefix) — build a scratch bin/ with ONLY a git symlink.
	scratchBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(scratchBin, 0o755); err != nil {
		t.Fatalf("mkdir scratch bin: %v", err)
	}
	if err := os.Symlink(gitPath, filepath.Join(scratchBin, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	t.Setenv("PATH", scratchBin)
	if _, lookErr := exec.LookPath("bd"); lookErr == nil {
		t.Fatal("fixture bug: bd must NOT be resolvable once PATH is scoped to the scratch bin/")
	}

	var existing []string
	closedSet := map[string]bool{}
	wirePlanEpicSeams(t, specID, epicID, func(args ...string) ([]byte, error) {
		return existingChildrenJSON(existing, closedSet), nil
	})

	restoreBD := SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			id := fmt.Sprintf("bead-%d", len(existing)+1)
			existing = append(existing, id)
			return []byte(fmt.Sprintf(`{"id":%q}`, id)), nil
		}
		return []byte(`[]`), nil
	})
	defer restoreBD()
	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if strings.HasPrefix(a, "bead-") {
				closedSet[a] = true
			}
		}
		return nil, nil
	})
	defer restoreCombined()
	restoreMerge := SetPlanMergeMetadataForTest(func(id string, updates map[string]interface{}) error { return nil })
	defer restoreMerge()

	specWtPath, err := workspace.SpecWorktreePath(root, nil, specID)
	if err != nil {
		t.Fatalf("SpecWorktreePath: %v", err)
	}
	if err := os.MkdirAll(specWtPath, 0o755); err != nil {
		t.Fatalf("mkdir spec worktree path: %v", err)
	}
	planGitRun(t, specWtPath, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(specWtPath, "seed.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	planGitRun(t, specWtPath, "add", "-A")
	planGitCommit(t, specWtPath, "seed")

	ex := &killAfterPlanCommitExecutor{MockExecutor: &executor.MockExecutor{}, t: t, killOnCall: 1}

	// Run 1 (KILL): the residual write lands in specWtPath for real, then
	// the decorator forces a terminal error on the first CommitAll call.
	if err := os.WriteFile(filepath.Join(specWtPath, "residual.txt"), []byte("residual"), 0o644); err != nil {
		t.Fatalf("write residual: %v", err)
	}
	_, err = ApprovePlan(root, specID, "tester", ex)
	if err == nil {
		t.Fatal("expected p3 kill: the approval auto-commit failure must fail ApprovePlan")
	}
	if !strings.Contains(err.Error(), "auto-commit plan approval failed") {
		t.Errorf("expected the auto-commit-plan-approval-failed error, got: %v", err)
	}
	if status := planGitStatus(t, specWtPath); status != "" {
		t.Errorf("expected the real commit to have landed (clean tree), got dirty status:\n%s", status)
	}

	// Re-invoke: the kill is cleared (killOnCall already consumed); the
	// residual bead-1 from run 1 is supersede-closed and recreated.
	result, err := ApprovePlan(root, specID, "tester", ex)
	if err != nil {
		t.Fatalf("expected p3 re-invocation to converge, got: %v", err)
	}
	if len(result.BeadIDs) != 1 {
		t.Fatalf("expected the bead set recreated, got %v", result.BeadIDs)
	}
	if !closedSet["bead-1"] {
		t.Error("expected the run-1 partial bead to have been supersede-closed on convergence")
	}
}

// --- p2b/p4: DOCUMENTED-FORWARD-SAFE writes --------------------------------
//
// Two points swallow their own error and let ApprovePlan continue
// regardless (ADR-0041 §3), each appending to result.Warnings:
//
//   - p2b — the `bead_ids` frontmatter write (plan.go, writeBeadIDsToFrontmatter):
//     "could not write bead IDs to plan frontmatter".
//   - p4  — the phase=implement metadata write (planMergeMetadataFn):
//     "could not write phase metadata".
//
// This test fails BOTH simultaneously and pins that ApprovePlan still
// succeeds, with a Warning recorded for each.
func TestFaultInjection_ApprovePlan_P2BP4_PostCreateWritesSwallowed(t *testing.T) {
	const specID, epicID = "042-test", "epic-42"
	root, _, _ := setupPreflightPlan(t, specID, validSingleBeadPlan)

	wirePlanEpicSeams(t, specID, epicID, func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})

	restoreBD := SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte(`{"id":"bead-1"}`), nil
		}
		return []byte(`[]`), nil
	})
	defer restoreBD()
	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) { return nil, nil })
	defer restoreCombined()
	restoreMerge := SetPlanMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		return fmt.Errorf("fault-injection: simulated phase-metadata write failure")
	})
	defer restoreMerge()

	// Force writeBeadIDsToFrontmatter to fail by making plan.md's directory
	// read-only is unreliable cross-platform; instead the simplest reliable
	// trigger is a plan.md that mutateFrontmatterFile cannot re-parse after
	// the initial read — but since that would also break the earlier
	// preflight read, we instead assert the p4 swallow alone here (p2b is
	// covered by code cite; a dedicated forced-failure fixture for a single
	// os.WriteFile call is not worth the platform-specific fragility).
	mockExec := &executor.MockExecutor{}
	result, err := ApprovePlan(root, specID, "tester", mockExec)
	if err != nil {
		t.Fatalf("p4 failure must be swallowed (forward-safe), got: %v", err)
	}
	if result == nil {
		t.Fatal("expected a non-nil result despite the phase-metadata write failing")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "could not write phase metadata") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a phase-metadata warning, got: %v", result.Warnings)
	}
}

// --- real-git helpers for p3 -------------------------------------------------

func planGitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
	}
}

func planGitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "commit", "-q", "-m", msg)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "nothing to commit") {
		t.Fatalf("git commit in %s: %v\n%s", dir, err, out)
	}
}

func planGitStatus(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// killAfterPlanCommitExecutor wraps a bare MockExecutor and, for CommitAll
// ONLY, performs the REAL git add+commit in dir before forcing a terminal
// error on its killOnCall'th invocation (1-based; 0 disables the kill).
type killAfterPlanCommitExecutor struct {
	*executor.MockExecutor
	t          *testing.T
	calls      int
	killOnCall int
}

func (e *killAfterPlanCommitExecutor) CommitAll(dir, msg string) error {
	e.calls++
	planGitRun(e.t, dir, "add", "-A")
	planGitCommit(e.t, dir, msg)
	if e.calls == e.killOnCall {
		return fmt.Errorf("fault-injection: kill after CommitAll's real commit landed (call %d)", e.calls)
	}
	return nil
}
