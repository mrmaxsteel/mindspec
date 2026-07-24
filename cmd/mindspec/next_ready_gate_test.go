package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/harness"
)

// next_ready_gate_test.go pins spec 124 (impl-readiness-gate) Bead 2's
// AC-4: `mindspec next` gates the claim on Bead 1's mechanical readiness
// floor BEFORE any mutation, with a durable `--allow-not-ready` override
// and `--force` orthogonality.
//
// This drives the REAL mindspec binary end-to-end (buildMindspecBinary,
// the spec 084 Bead 3 / Bead-1-AC-10 precedent) against a real bd + git
// sandbox (internal/harness.NewSandbox) — no fake-bd shim substitutes for
// `plan approve`/`next`'s own bd/git mutation chain (claim, branch,
// worktree), which cannot be faithfully faked without reimplementing bd.
// Per the repo's no-skip-gating convention for real-bd/real-git flows, a
// missing `bd`/`git` on PATH is a hard failure (t.Fatalf), never t.Skip.

// nextReadyGateBDStatus runs `bd show <id> --json` in the sandbox and
// returns the bead's status field plus its raw metadata map (for the
// AC-4 override-marker assertion).
func nextReadyGateBDStatus(t *testing.T, sb *harness.Sandbox, beadID string) (status string, metadata map[string]interface{}) {
	t.Helper()
	out, err := sb.Run("bd", "show", beadID, "--json")
	if err != nil {
		t.Fatalf("bd show %s: %v\n%s", beadID, err, out)
	}
	var records []struct {
		Status   string                 `json:"status"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(out), &records); err != nil {
		t.Fatalf("parsing bd show %s --json: %v\nraw: %s", beadID, err, out)
	}
	if len(records) == 0 {
		t.Fatalf("bd show %s --json returned no records", beadID)
	}
	return records[0].Status, records[0].Metadata
}

// nextReadyGateSetupSandbox builds the mindspec binary, puts it first on
// the live process PATH (restored via t.Cleanup — the buildMindspecBinary
// / harness.Sandbox.Env() precedent from
// TestBeadReadyCheck_TemporalFlow_RealPlanApproveAndComplete), and returns
// a fresh sandbox with bd/git required on PATH (hard failure, never
// t.Skip, if absent).
func nextReadyGateSetupSandbox(t *testing.T) *harness.Sandbox {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real-bd/real-git end-to-end flow under -short")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Fatalf("real bd required for this AC-4 gate-before-mutate test (no-skip-gating, spec 124 plan-gate F3-1): %v", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("real git required: %v", err)
	}

	binPath := buildMindspecBinary(t)
	binDir := filepath.Dir(binPath)
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	return harness.NewSandbox(t)
}

// nextReadyGateApproveSingleBeadPlan sets up a spec branch + worktree,
// writes specMD/planMD (a single-work-chunk plan), commits, and runs the
// real `mindspec plan approve` path — returning the epic ID `next` claims
// beads under.
func nextReadyGateApproveSingleBeadPlan(t *testing.T, sb *harness.Sandbox, specID, specMD, planMD string) string {
	t.Helper()
	epicID := sb.CreateSpecEpic(specID)

	specBranch := "spec/" + specID
	if out, err := sb.Run("git", "branch", specBranch); err != nil {
		t.Fatalf("git branch %s: %v\n%s", specBranch, err, out)
	}
	specWtDir := ".worktrees/worktree-spec-" + specID
	if out, err := sb.Run("git", "worktree", "add", specWtDir, specBranch); err != nil {
		t.Fatalf("git worktree add: %v\n%s", out, err)
	}

	sb.WriteFile(specWtDir+"/.mindspec/specs/"+specID+"/spec.md", specMD)
	sb.WriteFile(specWtDir+"/.mindspec/specs/"+specID+"/plan.md", planMD)

	specWtAbs := filepath.Join(sb.Root, specWtDir)
	if out, err := exec.Command("git", "-C", specWtAbs, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add (spec worktree): %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", specWtAbs, "commit", "-m", "setup: draft plan").CombinedOutput(); err != nil {
		t.Fatalf("git commit (spec worktree): %v\n%s", err, out)
	}
	sb.Commit("setup: spec worktree state")

	if out, err := sb.Run("mindspec", "plan", "approve", specID); err != nil {
		t.Fatalf("mindspec plan approve %s: %v\n%s", specID, err, out)
	}
	return epicID
}

const nextReadyGateNegativeSpecMD = `---
title: Readiness Gate Negative Fixture
status: Approved
---
# Readiness Gate Negative Fixture

## Acceptance Criteria
- [ ] AC-1 — placeholder criterion never claimed by the bead below.
`

const nextReadyGateNegativePlanMD = `---
status: Draft
spec_id: 996-readiness-gate-negative
version: "1"
adr_citations: []
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - path/to/file.go
---
# Plan: 996-readiness-gate-negative

## ADR Fitness

No existing ADRs are impacted; this is a self-contained test fixture.

## Testing Strategy

Unit tests via ` + "`go test`" + `.

## Bead 1: Negative gate fixture

**Steps**
1. Step one

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- <Specific, measurable criterion for this bead>

**Depends on**
None
`

const nextReadyGatePositiveSpecMD = `---
title: Readiness Gate Positive Fixture
status: Approved
---
# Readiness Gate Positive Fixture

## Acceptance Criteria
- [ ] AC-1 — bead 1 satisfies the readiness floor.
`

const nextReadyGatePositivePlanMD = `---
status: Draft
spec_id: 995-readiness-gate-positive
version: "1"
adr_citations: []
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/fixture/gate_positive.go
---
# Plan: 995-readiness-gate-positive

## ADR Fitness

No existing ADRs are impacted; this is a self-contained test fixture.

## Testing Strategy

Unit tests via ` + "`go test`" + `.

## Bead 1: Positive gate fixture

**Steps**
1. Create internal/fixture/gate_positive.go
2. Implement the fixture function
3. Add a unit test

**Verification**
- [ ] ` + "`go test ./internal/fixture/`" + ` passes

**Acceptance Criteria**
- [ ] AC-1 — bead 1 satisfies the readiness floor.

**Depends on**
None
`

// TestNextReadyGate_NegativeBead_RefusalIsZeroMutationAndForceOrthogonal
// pins AC-4's refusal half: against a bead that fails the mechanical
// floor, `mindspec next` exits non-zero and a full state audit (bd
// status of the selected bead, `git branch --list 'bead/*'`, worktree
// list) is byte-identical to pre-invocation; `--force` alone does NOT
// bypass the gate (still refused, still zero-mutation).
func TestNextReadyGate_NegativeBead_RefusalIsZeroMutationAndForceOrthogonal(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	epicID := nextReadyGateApproveSingleBeadPlan(t, sb,
		"996-readiness-gate-negative", nextReadyGateNegativeSpecMD, nextReadyGateNegativePlanMD)
	beadID := epicID + ".1"

	statusBefore, _ := nextReadyGateBDStatus(t, sb, beadID)
	branchesBefore := sb.ListBranches("bead/")
	worktreesBefore := sb.ListWorktrees()

	out, err := sb.Run("mindspec", "next", beadID)
	if err == nil {
		t.Fatalf("expected `mindspec next %s` to refuse (mechanical floor FAIL), got exit 0:\n%s", beadID, out)
	}
	if !strings.Contains(out, "MF-1") {
		t.Errorf("expected the refusal to name MF-1, got:\n%s", out)
	}

	statusAfter, _ := nextReadyGateBDStatus(t, sb, beadID)
	branchesAfter := sb.ListBranches("bead/")
	worktreesAfter := sb.ListWorktrees()

	if statusBefore != statusAfter {
		t.Errorf("bd status changed across the refused invocation: before=%q after=%q", statusBefore, statusAfter)
	}
	if !equalStringSlices(branchesBefore, branchesAfter) {
		t.Errorf("`git branch --list 'bead/*'` changed across the refused invocation: before=%v after=%v", branchesBefore, branchesAfter)
	}
	if !equalStringSlices(worktreesBefore, worktreesAfter) {
		t.Errorf("worktree list changed across the refused invocation: before=%v after=%v", worktreesBefore, worktreesAfter)
	}

	// --force alone gains no readiness authority: still refused, still
	// zero-mutation.
	outForce, errForce := sb.Run("mindspec", "next", beadID, "--force")
	if errForce == nil {
		t.Fatalf("expected `mindspec next %s --force` to STILL refuse (force is orthogonal to readiness), got exit 0:\n%s", beadID, outForce)
	}
	if !strings.Contains(outForce, "MF-1") {
		t.Errorf("expected the --force refusal to also name MF-1, got:\n%s", outForce)
	}

	statusAfterForce, _ := nextReadyGateBDStatus(t, sb, beadID)
	branchesAfterForce := sb.ListBranches("bead/")
	worktreesAfterForce := sb.ListWorktrees()
	if statusBefore != statusAfterForce {
		t.Errorf("bd status changed across the --force-refused invocation: before=%q after=%q", statusBefore, statusAfterForce)
	}
	if !equalStringSlices(branchesBefore, branchesAfterForce) {
		t.Errorf("branch list changed across the --force-refused invocation: before=%v after=%v", branchesBefore, branchesAfterForce)
	}
	if !equalStringSlices(worktreesBefore, worktreesAfterForce) {
		t.Errorf("worktree list changed across the --force-refused invocation: before=%v after=%v", worktreesBefore, worktreesAfterForce)
	}
}

// TestNextReadyGate_AllowNotReady_ClaimsAndRecordsDurableMarker pins
// AC-4's override half: `--allow-not-ready` proceeds past the failing
// floor, stderr names every failing signal, and a durable override
// marker naming those signals is written to the bead (visible via
// `bd show`).
func TestNextReadyGate_AllowNotReady_ClaimsAndRecordsDurableMarker(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	epicID := nextReadyGateApproveSingleBeadPlan(t, sb,
		"994-readiness-gate-override", nextReadyGateNegativeSpecMD,
		strings.ReplaceAll(nextReadyGateNegativePlanMD, "996-readiness-gate-negative", "994-readiness-gate-override"))
	beadID := epicID + ".1"

	out, err := sb.Run("mindspec", "next", beadID, "--allow-not-ready")
	if err != nil {
		t.Fatalf("expected `mindspec next %s --allow-not-ready` to succeed, got: %v\n%s", beadID, err, out)
	}
	if !strings.Contains(out, "MF-1") {
		t.Errorf("expected the override warning to name the failing MF-1 signal, got:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "warning") {
		t.Errorf("expected an override warning in the output, got:\n%s", out)
	}

	status, metadata := nextReadyGateBDStatus(t, sb, beadID)
	if status != "in_progress" {
		t.Errorf("expected bead %s to be claimed (in_progress) after --allow-not-ready, got status %q", beadID, status)
	}
	marker, ok := metadata["mindspec_readiness_override"]
	if !ok {
		t.Fatalf("expected a mindspec_readiness_override metadata key on %s after --allow-not-ready; got metadata %v", beadID, metadata)
	}
	markerJSON, err := json.Marshal(marker)
	if err != nil {
		t.Fatalf("marshaling marker for inspection: %v", err)
	}
	if !strings.Contains(string(markerJSON), "MF-1") {
		t.Errorf("expected the override marker to name MF-1, got: %s", markerJSON)
	}

	worktreeName := "worktree-" + beadID
	if !sb.WorktreeExists(worktreeName) {
		t.Errorf("expected a worktree for %s to have been created after --allow-not-ready", beadID)
	}
	branches := sb.ListBranches("bead/")
	found := false
	for _, b := range branches {
		if strings.Contains(b, beadID) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a bead/%s branch to exist after --allow-not-ready, got branches %v", beadID, branches)
	}
}

// TestNextReadyGate_PositiveBead_ClaimsWithNoPromptNoRefusal pins the
// zero-friction half: against a bead that satisfies every mechanical
// signal, `mindspec next` claims normally — no refusal, no
// `--allow-not-ready` needed, and the output carries no NOT-READY / MF-x
// FAIL noise.
func TestNextReadyGate_PositiveBead_ClaimsWithNoPromptNoRefusal(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	epicID := nextReadyGateApproveSingleBeadPlan(t, sb,
		"995-readiness-gate-positive", nextReadyGatePositiveSpecMD, nextReadyGatePositivePlanMD)
	beadID := epicID + ".1"

	out, err := sb.Run("mindspec", "next", beadID)
	if err != nil {
		t.Fatalf("expected `mindspec next %s` to claim normally, got: %v\n%s", beadID, err, out)
	}
	for _, unwanted := range []string{"NOT READY", "MF-1: FAIL", "MF-2: FAIL", "MF-3: FAIL", "MF-4: FAIL", "allow-not-ready"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("expected no readiness-refusal noise in the positive-bead claim output, found %q in:\n%s", unwanted, out)
		}
	}

	status, metadata := nextReadyGateBDStatus(t, sb, beadID)
	if status != "in_progress" {
		t.Errorf("expected bead %s to be claimed (in_progress), got status %q", beadID, status)
	}
	if _, ok := metadata["mindspec_readiness_override"]; ok {
		t.Errorf("expected NO readiness override marker on a bead that never failed the floor; got metadata %v", metadata)
	}
}

// installMetadataFailingBD drops a fake `bd` executable into a temp dir
// and prepends it to the live process PATH (restored via t.Cleanup). The
// fake forwards EVERY bd call to the real bd binary UNLESS the
// FAKE_BD_FAIL_METADATA env var is set AND the call carries `--metadata`,
// in which case it exits non-zero — so the sandbox's pinned bd shim
// (which resolves the real bd via findRealBinary, picking up this fake as
// the earliest bd on PATH) forwards metadata writes to a controllable
// failure. The sentinel gate lets all of setup's plan-approve metadata
// writes pass through; the test flips it on ONLY around the
// `next --allow-not-ready` invocation whose marker write must fail.
//
// This must run BEFORE nextReadyGateSetupSandbox so the fake sits ahead of
// the real bd when the sandbox composes its shim.
func installMetadataFailingBD(t *testing.T) {
	t.Helper()
	realBD, err := exec.LookPath("bd")
	if err != nil {
		t.Fatalf("real bd required to build the metadata-failing fake: %v", err)
	}

	fakeDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ -n \"$FAKE_BD_FAIL_METADATA\" ]; then\n" +
		"  for arg in \"$@\"; do\n" +
		"    if [ \"$arg\" = \"--metadata\" ]; then\n" +
		"      echo \"fake bd: metadata write forced to fail (spec 124 AC-4 fail-closed test)\" >&2\n" +
		"      exit 1\n" +
		"    fi\n" +
		"  done\n" +
		"fi\n" +
		"exec \"" + realBD + "\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(fakeDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake bd: %v", err)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", fakeDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
}

// TestNextReadyGate_AllowNotReady_MarkerWriteFailsClosed pins FX-1: on the
// `--allow-not-ready` proceed path the durable override marker is a
// GUARANTEE, not best-effort. With the marker write FORCED to fail, the
// command REFUSES (exit non-zero naming the marker-write failure), the
// bead is NOT claimed, and no worktree is created — never the prior
// swallow-and-continue partial state (claimed + worktree, no marker).
func TestNextReadyGate_AllowNotReady_MarkerWriteFailsClosed(t *testing.T) {
	installMetadataFailingBD(t)

	sb := nextReadyGateSetupSandbox(t)

	epicID := nextReadyGateApproveSingleBeadPlan(t, sb,
		"993-readiness-gate-marker-fail", nextReadyGateNegativeSpecMD,
		strings.ReplaceAll(nextReadyGateNegativePlanMD, "996-readiness-gate-negative", "993-readiness-gate-marker-fail"))
	beadID := epicID + ".1"

	statusBefore, _ := nextReadyGateBDStatus(t, sb, beadID)
	branchesBefore := sb.ListBranches("bead/")
	worktreesBefore := sb.ListWorktrees()

	// Flip the marker-write failure on ONLY for this invocation (setup's
	// plan-approve metadata writes above ran with it off).
	os.Setenv("FAKE_BD_FAIL_METADATA", "1")
	defer os.Unsetenv("FAKE_BD_FAIL_METADATA")

	out, err := sb.Run("mindspec", "next", beadID, "--allow-not-ready")
	if err == nil {
		t.Fatalf("expected `mindspec next %s --allow-not-ready` to REFUSE when the override marker write fails, got exit 0:\n%s", beadID, out)
	}
	if !strings.Contains(strings.ToLower(out), "marker") {
		t.Errorf("expected the refusal to name the override marker-write failure, got:\n%s", out)
	}

	// Nothing claimed, no branch, no worktree — fail-closed, not the
	// swallow-and-continue partial state.
	statusAfter, metadata := nextReadyGateBDStatus(t, sb, beadID)
	if statusAfter != statusBefore {
		t.Errorf("bead status changed despite the fail-closed refusal: before=%q after=%q (expected the bead to remain UNCLAIMED)", statusBefore, statusAfter)
	}
	if statusAfter == "in_progress" {
		t.Errorf("bead %s was claimed despite the marker-write failure — fail-closed violated", beadID)
	}
	if _, ok := metadata["mindspec_readiness_override"]; ok {
		t.Errorf("expected NO override marker after the fail-closed refusal; got metadata %v", metadata)
	}
	if !equalStringSlices(branchesBefore, sb.ListBranches("bead/")) {
		t.Errorf("bead branch list changed despite the fail-closed refusal: before=%v after=%v", branchesBefore, sb.ListBranches("bead/"))
	}
	if !equalStringSlices(worktreesBefore, sb.ListWorktrees()) {
		t.Errorf("worktree list changed despite the fail-closed refusal: before=%v after=%v", worktreesBefore, sb.ListWorktrees())
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// installClaimFailingBD mirrors installMetadataFailingBD: a fake `bd`
// forwards every call to the real bd UNLESS the FAKE_BD_FAIL_CLAIM env
// var is set AND the call carries `--claim`, in which case it exits
// non-zero — forcing ClaimBead to fail AFTER the Step 4.6 override
// marker landed (the G3-OVERRIDE-ORPHAN window, spec 124 final-review
// r1). Must run BEFORE nextReadyGateSetupSandbox so the fake sits ahead
// of the real bd when the sandbox composes its shim.
func installClaimFailingBD(t *testing.T) {
	t.Helper()
	realBD, err := exec.LookPath("bd")
	if err != nil {
		t.Fatalf("real bd required to build the claim-failing fake: %v", err)
	}

	fakeDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ -n \"$FAKE_BD_FAIL_CLAIM\" ]; then\n" +
		"  for arg in \"$@\"; do\n" +
		"    if [ \"$arg\" = \"--claim\" ]; then\n" +
		"      echo \"fake bd: claim forced to fail (spec 124 G3-OVERRIDE-ORPHAN rollback test)\" >&2\n" +
		"      exit 1\n" +
		"    fi\n" +
		"  done\n" +
		"fi\n" +
		"exec \"" + realBD + "\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(fakeDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake bd: %v", err)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", fakeDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
}

// TestNextReadyGate_AllowNotReady_ClaimFailureRollsBackMarker pins the
// G3-OVERRIDE-ORPHAN fix (spec 124 final-review r1): the override marker
// is written BEFORE ClaimBead (AC-4 fail-closed), so when ClaimBead then
// FAILS, the marker must be ROLLED BACK — a failed claim leaves the bead
// unclaimed with NO orphaned override marker authorizing a claim this
// run never made, and no branch/worktree. RED-on-revert: removing the
// rollback call from cmd/mindspec/next.go's claim-failure branch leaves
// the marker behind and turns this red.
func TestNextReadyGate_AllowNotReady_ClaimFailureRollsBackMarker(t *testing.T) {
	installClaimFailingBD(t)

	sb := nextReadyGateSetupSandbox(t)

	epicID := nextReadyGateApproveSingleBeadPlan(t, sb,
		"992-readiness-gate-claim-fail", nextReadyGateNegativeSpecMD,
		strings.ReplaceAll(nextReadyGateNegativePlanMD, "996-readiness-gate-negative", "992-readiness-gate-claim-fail"))
	beadID := epicID + ".1"

	statusBefore, _ := nextReadyGateBDStatus(t, sb, beadID)
	branchesBefore := sb.ListBranches("bead/")
	worktreesBefore := sb.ListWorktrees()

	// Flip the claim failure on ONLY for this invocation (the marker
	// write itself must SUCCEED — the orphan window under test is
	// marker-landed-then-claim-lost).
	os.Setenv("FAKE_BD_FAIL_CLAIM", "1")
	defer os.Unsetenv("FAKE_BD_FAIL_CLAIM")

	out, err := sb.Run("mindspec", "next", beadID, "--allow-not-ready")
	if err == nil {
		t.Fatalf("expected `mindspec next %s --allow-not-ready` to fail when the claim fails, got exit 0:\n%s", beadID, out)
	}
	if !strings.Contains(strings.ToLower(out), "rolled back") {
		t.Errorf("expected the failure output to report the override-marker rollback, got:\n%s", out)
	}

	// The bead is unclaimed, the marker is GONE, and no branch/worktree
	// was created — the failed claim left no orphaned override.
	statusAfter, metadata := nextReadyGateBDStatus(t, sb, beadID)
	if statusAfter != statusBefore {
		t.Errorf("bead status changed despite the failed claim: before=%q after=%q", statusBefore, statusAfter)
	}
	if statusAfter == "in_progress" {
		t.Errorf("bead %s claimed despite the forced claim failure", beadID)
	}
	if _, ok := metadata["mindspec_readiness_override"]; ok {
		t.Errorf("ORPHANED override marker left behind after the failed claim (G3-OVERRIDE-ORPHAN); got metadata %v", metadata)
	}
	if !equalStringSlices(branchesBefore, sb.ListBranches("bead/")) {
		t.Errorf("bead branch list changed despite the failed claim: before=%v after=%v", branchesBefore, sb.ListBranches("bead/"))
	}
	if !equalStringSlices(worktreesBefore, sb.ListWorktrees()) {
		t.Errorf("worktree list changed despite the failed claim: before=%v after=%v", worktreesBefore, sb.ListWorktrees())
	}
}
