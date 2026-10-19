package main

import (
	"encoding/json"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"

	"github.com/spf13/cobra"
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
// The legacy sandbox helpers below support the existing long integration
// fixtures. The scoped-override fixtures use a local fake tracker and real
// temporary git worktrees; no live tracker is consulted.
// Legacy integrations build the real binary (buildMindspecBinary,
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
	f := newScopedNextFixture(t)
	f.rows[0]["description"] = "AC-999"
	f.write(t)
	out, err := runScopedNext(t, f, "epic-129.1", "--allow-not-ready=MF-1,MF-2,MF-3,MF-4", "--evidence", nextEvidenceFile(t))
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	requireText(t, out, "warning", "MF-2", "MF-1 named but passing — not recorded")
	m, v, err := bead.ReadOverrideMarker("epic-129.1")
	if err != nil || v != bead.MarkerValid || !reflect.DeepEqual(m.Signals, []string{"MF-2"}) {
		t.Fatalf("durable v2 marker: %+v %s %v", m, v, err)
	}
	if f.status(t, "epic-129.1") != "in_progress" {
		t.Fatal("not claimed")
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

// TestNextReadyGate_AllowNotReady_MarkerWriteFailsClosed pins FX-1: on the
// `--allow-not-ready` proceed path the durable override marker is a
// GUARANTEE, not best-effort. With the marker write FORCED to fail, the
// command REFUSES (exit non-zero naming the marker-write failure), the
// bead is NOT claimed, and no worktree is created — never the prior
// swallow-and-continue partial state (claimed + worktree, no marker).
func TestNextReadyGate_AllowNotReady_MarkerWriteFailsClosed(t *testing.T) {
	f := newScopedNextFixture(t)
	f.rows[0]["description"] = "AC-999"
	f.write(t)
	t.Setenv("SCOPED_FAIL_METADATA", "1")
	before := f.audit(t)
	_, err := runScopedNext(t, f, "epic-129.1", "--allow-not-ready=MF-2", "--evidence", nextEvidenceFile(t))
	if err == nil {
		t.Fatal("marker write failure claimed")
	}
	requireText(t, err.Error(), "recording the --allow-not-ready override marker")
	assertScopedNoMutation(t, f, before, true)
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

// TestNextReadyGate_AllowNotReady_ClaimFailureRollsBackMarker pins the
// G3-OVERRIDE-ORPHAN fix (spec 124 final-review r1): the override marker
// is written BEFORE ClaimBead (AC-4 fail-closed), so when ClaimBead then
// FAILS, the marker must be ROLLED BACK — a failed claim leaves the bead
// unclaimed with NO orphaned override marker authorizing a claim this
// run never made, and no branch/worktree. RED-on-revert: removing the
// rollback call from cmd/mindspec/next.go's claim-failure branch leaves
// the marker behind and turns this red.
func TestNextReadyGate_AllowNotReady_ClaimFailureRollsBackMarker(t *testing.T) {
	f := newScopedNextFixture(t)
	f.rows[0]["description"] = "AC-999"
	f.write(t)
	t.Setenv("SCOPED_FAIL_CLAIM", "1")
	before := scopedAudit(t, f)
	out, err := runScopedNext(t, f, "epic-129.1", "--allow-not-ready=MF-2", "--evidence", nextEvidenceFile(t))
	if err == nil {
		t.Fatal("claim failure ignored")
	}
	requireText(t, out, "rolled back")
	_, v, e := bead.ReadOverrideMarker("epic-129.1")
	if e != nil || v != bead.MarkerAbsent {
		t.Fatalf("orphan marker %s %v", v, e)
	}
	// The local JSON is rewritten by rollback; compare decoded tracker data.
	var old, now any
	data, e := os.ReadFile(f.path)
	if e != nil {
		t.Fatal(e)
	}
	raw := strings.Split(before, "\nAUDIT-GIT\n")[0]
	if e := json.Unmarshal([]byte(raw), &old); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(data, &now); e != nil {
		t.Fatal(e)
	}
	// Metadata's empty map is an acceptable representation of no keys.
	row := now.(map[string]any)["rows"].([]any)[0].(map[string]any)
	delete(row, "metadata")
	if !reflect.DeepEqual(old, now) {
		t.Fatalf("rollback changed tracker: before=%v after=%v", old, now)
	}
	if f.status(t, "epic-129.1") != "open" {
		t.Fatal("claim succeeded")
	}
	if strings.Contains(honestyGit(t, f.root, "branch", "--list", "bead/*"), "bead/") {
		t.Fatal("claim failure created branch")
	}
}

// Scoped command tests use only a local tracker fake and temporary git repo.
func nextEvidenceFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "why.md")
	if err := os.WriteFile(path, []byte("operator accepts these precise failures"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func newScopedNextFixture(t *testing.T) *nextHonestyFixture {
	t.Helper()
	f := newNextHonestyFixture(t)
	f.rows = f.rows[:1]
	f.write(t)
	if exec.Command("git", "-C", f.root, "show-ref", "--verify", "--quiet", "refs/heads/spec/129-fixture").Run() != nil {
		honestyGit(t, f.root, "branch", "spec/129-fixture")
	}
	// Extend the existing fake locally, without changing the shared fixture.
	script := strings.Replace(nextHonestyFakeBD, `elif a[0]=="update":`, `elif a[0]=="update":
 if os.environ.get("SCOPED_FAIL_METADATA") and arg("--metadata"): sys.exit("forced metadata write failure")
 if os.environ.get("SCOPED_FAIL_CLAIM") and "--claim" in a: sys.exit("forced claim failure")`, 1)
	script = strings.Replace(script, ` if arg("--parent"): row["parent"]=arg("--parent")`, ` if arg("--parent"): row["parent"]=arg("--parent")
 if arg("--status"): row["status"]=arg("--status")
 if arg("--unset-metadata"): row.setdefault("metadata",{}).pop(arg("--unset-metadata"),None)`, 1)
	if err := os.WriteFile(filepath.Join(f.root, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	return f
}
func runScopedNext(t *testing.T, f *nextHonestyFixture, args ...string) (string, error) {
	t.Helper()
	out, err := os.CreateTemp(f.root, "captured-")
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, out
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	cmd := &cobra.Command{Use: "next", RunE: nextCmd.RunE, SilenceErrors: true, SilenceUsage: true}
	registerNextFlags(cmd)
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), runErr
}
func scopedAudit(t *testing.T, f *nextHonestyFixture) string {
	t.Helper()
	data, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\nAUDIT-GIT\n" + honestyGit(t, f.root, "for-each-ref", "--format=%(refname) %(objectname)") + honestyGit(t, f.root, "worktree", "list", "--porcelain") + honestyGit(t, f.root, "status", "--porcelain")
}
func assertScopedNoMutation(t *testing.T, f *nextHonestyFixture, before string, allowMetadataAttempt bool) {
	t.Helper()
	if f.audit(t) != before {
		t.Fatal("refusal changed tracker, refs, worktrees or porcelain")
	}
	raw, err := os.ReadFile(f.path + ".argv")
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		if len(args) > 0 && args[0] == "update" {
			if allowMetadataAttempt && strings.Contains(line, "--metadata") {
				continue
			}
			t.Fatalf("refusal performed a tracker mutation: %s", line)
		}
	}
}
func TestNext_BareAllowNotReady_Refuses(t *testing.T) {
	for _, leg := range []string{"MF2-and-MF3", "nothing-failing", "nothing-failing-missing-branch", "foreign-dotted", "indeterminate", "selection"} {
		t.Run(leg, func(t *testing.T) {
			f := newScopedNextFixture(t)
			args := []string{"epic-129.1", "--allow-not-ready"}
			want := "recovery: mindspec next epic-129.1"
			switch leg {
			case "foreign-dotted":
				f.rows = []map[string]interface{}{nextHonestyRow("other-epic.1", "[129] foreign dotted bead", "", "task", "open")}
				f.write(t)
				args[0] = "other-epic.1"
				want = "recovery: mindspec bead ready-check other-epic.1"
				report, err := readiness.EvaluateReadiness(f.root, args[0])
				if err != nil || !report.AllPass() {
					t.Fatalf("foreign fixture must pass every signal: %+v %v", report, err)
				}
			case "nothing-failing-missing-branch":
				honestyGit(t, f.root, "branch", "-D", "spec/129-fixture")
			case "MF2-and-MF3":
				f.rows[0]["description"] = "AC-999"
				f.rows = append(f.rows, nextHonestyRow("epic-129.2", "dependency", "epic-129", "task", "open"))
				f.deps["epic-129.1"] = []map[string]string{{"id": "epic-129.2", "status": "open", "dependency_type": "blocks"}}
				f.write(t)
				want += " --allow-not-ready=MF-2,MF-3 --evidence <path-to-justification>"
			case "indeterminate":
				f.deps["epic-129.1"] = []map[string]string{{"id": "external-dep", "status": "closed", "dependency_type": "blocks"}}
				f.write(t)
				want = "recovery: mindspec bead ready-check epic-129.1   (the standalone per-signal report)"
			case "selection":
				args = []string{"--allow-not-ready"}
				want = "recovery: mindspec next <bead-id> --allow-not-ready=<ids> --evidence <path-to-justification>"
			}
			before := f.audit(t)
			_, err := runScopedNext(t, f, args...)
			if err == nil {
				t.Fatal("bare flag accepted")
			}
			requireText(t, err.Error(), "requires =<ids>")
			lines := strings.Split(err.Error(), "\n")
			if got := lines[len(lines)-1]; got != want {
				t.Fatalf("final recovery = %q, want %q", got, want)
			}
			if leg == "indeterminate" {
				requireText(t, err.Error(), "MF-3 cannot be overridden", "fix the read first")
				if strings.Contains(err.Error(), "--allow-not-ready=") {
					t.Fatal("indeterminate refusal suggested an override")
				}
			}
			assertScopedNoMutation(t, f, before, false)
		})
	}
}
func TestNext_AllowNotReadySpaceSeparated_Refuses(t *testing.T) {
	for _, tc := range []struct{ token, signals string }{
		{"MF-3", "MF-3"},
		{"mf-3", "MF-3"},
		{"MF-4,MF-2,MF-4", "MF-2,MF-4"},
		{"MF-9", "<ids>"},
		{"MF-3;rm -rf ~", "<ids>"},
	} {
		t.Run(tc.token, func(t *testing.T) {
			if strings.Contains(tc.token, ";") && len(guard.FindFloorMatches(tc.token)) == 0 {
				t.Fatal("destructive-token fixture did not fire the floor detector")
			}
			f := newScopedNextFixture(t)
			before := f.audit(t)
			_, err := runScopedNext(t, f, "--allow-not-ready", tc.token)
			if err == nil {
				t.Fatal("space-separated flag accepted")
			}
			requireText(t, err.Error(), "requires =<ids>")
			want := "recovery: mindspec next <bead-id> --allow-not-ready=" + tc.signals + " --evidence <path-to-justification>"
			lines := strings.Split(err.Error(), "\n")
			if got := lines[len(lines)-1]; got != want {
				t.Errorf("final recovery = %q, want %q", got, want)
			}
			if matches := guard.FindFloorMatches(err.Error()); len(matches) != 0 {
				t.Errorf("bare refusal contains destructive floor matches: %+v", matches)
			}
			assertScopedNoMutation(t, f, before, false)
			raw, err := os.ReadFile(f.path + ".argv")
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if len(raw) != 0 {
				t.Fatalf("signal token triggered tracker lookup: %s", raw)
			}
		})
	}
}
func TestNext_InvalidScopedSignalRefuses(t *testing.T) {
	for _, value := range []string{"MF-9", "all", ""} {
		t.Run(value, func(t *testing.T) {
			f := newScopedNextFixture(t)
			before := f.audit(t)
			_, err := runScopedNext(t, f, "epic-129.1", "--allow-not-ready="+value)
			if err == nil {
				t.Fatal("invalid signal accepted")
			}
			want := "unknown readiness signal " + value
			last := "recovery: mindspec bead ready-check epic-129.1"
			if value == "" {
				want = "requires =<ids>"
				last = "recovery: mindspec next epic-129.1 --allow-not-ready=<ids> --evidence <path-to-justification>"
			}
			requireText(t, err.Error(), want)
			lines := strings.Split(err.Error(), "\n")
			if got := lines[len(lines)-1]; got != last {
				t.Fatalf("final recovery = %q, want %q", got, last)
			}
			assertScopedNoMutation(t, f, before, false)
			raw, err := os.ReadFile(f.path + ".argv")
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if len(raw) != 0 {
				t.Fatalf("invalid signal triggered tracker lookup: %s", raw)
			}
		})
	}
}

func TestNext_UnscopedEmptySetShowsIgnoredDirt(t *testing.T) {
	for _, blocking := range []bool{false, true} {
		t.Run(fmt.Sprint(blocking), func(t *testing.T) {
			f := newScopedNextFixture(t)
			f.rows = nil
			f.epics[0]["status"] = "closed"
			f.write(t)
			if target, err := resolve.ResolveTarget(f.root, ""); err == nil || target != "" {
				t.Fatalf("fixture resolved target %q: %v", target, err)
			}
			path := filepath.Join(f.root, "unrelated.txt")
			if err := os.WriteFile(path, []byte("operator dirt"), 0600); err != nil {
				t.Fatal(err)
			}
			if blocking {
				file := filepath.Join(f.root, ".gitignore")
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, append(data, []byte("operator-note\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			blocked, ignored, err := next.CheckDirtyTreeProportional(f.root, f.root, "")
			if err != nil || len(ignored) == 0 || (blocking && len(blocked) == 0) {
				t.Fatalf("dirt fixture not armed: blocking=%v ignored=%v err=%v", blocked, ignored, err)
			}
			before := f.audit(t)
			out, err := runScopedNext(t, f)
			if err != nil {
				t.Fatalf("empty-set dirt must never block: %v", err)
			}
			requireText(t, out, "No ready work found.", next.IgnoredDirtNotice([]string{"unrelated.txt"}))
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "operator dirt" {
				t.Fatalf("unrelated dirt changed: %q %v", data, err)
			}
			assertScopedNoMutation(t, f, before, false)
		})
	}
}
func TestNext_ScopedFlagNothingFailing_ClaimsWithoutMarker(t *testing.T) {
	f := newScopedNextFixture(t)
	out, err := runScopedNext(t, f, "epic-129.1", "--allow-not-ready=MF-3")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	requireText(t, out, "MF-3 named but passing — not recorded")
	if f.status(t, "epic-129.1") != "in_progress" {
		t.Fatal("normal claim did not run")
	}
	_, v, err := bead.ReadOverrideMarker("epic-129.1")
	if err != nil || v != bead.MarkerAbsent {
		t.Fatalf("passing signal wrote marker: %s %v", v, err)
	}
}
func TestNext_N4PositionalOverridesMF1(t *testing.T) {
	f := newScopedNextFixture(t)
	f.rows = []map[string]interface{}{nextHonestyRow("epic-129.4", "follow-up", "epic-129", "bug", "open")}
	f.write(t)
	out, err := runScopedNext(t, f, "epic-129.4", "--spec", "129-fixture", "--allow-not-ready=MF-1", "--evidence", nextEvidenceFile(t))
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	m, v, err := bead.ReadOverrideMarker("epic-129.4")
	if err != nil || v != bead.MarkerValid || !reflect.DeepEqual(m.Signals, []string{"MF-1"}) || f.status(t, "epic-129.4") != "in_progress" {
		t.Fatalf("N4 did not claim with MF1-only v2 marker: %+v %s %v", m, v, err)
	}
}
func TestNext_NoLineageBugRefuses(t *testing.T) {
	f := newScopedNextFixture(t)
	f.rows = []map[string]interface{}{nextHonestyRow("mindspec-bug", "orphan", "", "bug", "open")}
	// Suppress active-target resolution without altering readiness's real lineage resolver.
	f.epics[0]["status"] = "closed"
	f.write(t)
	before := f.audit(t)
	_, err := runScopedNext(t, f, "mindspec-bug")
	if err == nil {
		t.Fatal("no-lineage bug claimed")
	}
	requireText(t, err.Error(), "evaluating readiness for mindspec-bug", "no epic")
	assertScopedNoMutation(t, f, before, false)
}
func TestNext_MissingSpecBranchNeverClaims(t *testing.T) {
	f := newScopedNextFixture(t)
	honestyGit(t, f.root, "branch", "-D", "spec/129-fixture")
	before := f.audit(t)
	_, err := runScopedNext(t, f, "epic-129.1", "--spec", "129-fixture")
	if err == nil {
		t.Fatal("missing branch claimed")
	}
	requireText(t, err.Error(), "spec branch spec/129-fixture", "nothing was claimed")
	assertScopedNoMutation(t, f, before, false)
}
func TestNext_UnscopedLineageRecordingDirtRefuses(t *testing.T) {
	for _, file := range []string{"manifest.json", "events.ndjson"} {
		t.Run(file, func(t *testing.T) {
			f := newScopedNextFixture(t)
			f.epics[0]["status"] = "closed"
			f.write(t)
			if err := os.WriteFile(filepath.Join(f.root, ".mindspec/config.yaml"), []byte("recording:\n  enabled: true\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := recording.StartRecording(f.root, "129-fixture"); err != nil {
				t.Fatal(err)
			}
			honestyGit(t, f.root, "add", ".")
			honestyGit(t, f.root, "commit", "-m", "recording fixture")
			manifest, err := recording.ManifestPath(f.root, "129-fixture")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(filepath.Dir(manifest), file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if file == "manifest.json" {
				var m map[string]interface{}
				if err := json.Unmarshal(data, &m); err != nil {
					t.Fatal(err)
				}
				m["operator_note"] = "uncommitted user annotation"
				data, err = json.Marshal(m)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				data = append(data, []byte("{\"operator_note\":\"uncommitted user annotation\"}\n")...)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if target, err := resolve.ResolveTarget(f.root, ""); err == nil || target != "" {
				t.Fatalf("fixture resolved target %q: %v", target, err)
			}
			before := f.audit(t)
			out, runErr := runScopedNext(t, f, "epic-129.1")
			if runErr == nil {
				t.Errorf("unscoped claim accepted protected dirty recording file: %s", out)
			} else {
				requireText(t, runErr.Error(), "uncommitted user changes", file)
			}
			if strings.Contains(out, "unrelated dirty path") {
				t.Errorf("protected path called unrelated: %s", out)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(data) {
				t.Error("user recording bytes changed across refusal")
			}
			assertScopedNoMutation(t, f, before, false)
		})
	}
}

func TestNext_UnscopedResolvedLineageMismatchRefuses(t *testing.T) {
	f := newScopedNextFixture(t)
	// The resolver and selection see S. A read-only tracker response changes
	// at the gate boundary, modelling lineage drift since the selection read.
	for _, file := range []string{"spec.md", "plan.md"} {
		data, err := os.ReadFile(filepath.Join(f.root, ".mindspec/specs/129-fixture", file))
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(f.root, ".mindspec/specs/130-lineage")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	honestyGit(t, f.root, "add", ".")
	honestyGit(t, f.root, "commit", "-m", "second lineage spec")
	honestyGit(t, f.root, "branch", "spec/130-lineage")
	path := filepath.Join(f.root, "bd")
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script = []byte(strings.Replace(string(script), `d=json.load(open(p))`, `d=json.load(open(p))
if os.environ.get("SCOPED_LINEAGE_CHANGED"):
 d["epics"][0]["title"]="[SPEC 130] lineage"
 d["epics"][0]["metadata"].update(spec_num=130,spec_title="lineage")`, 1))
	if err := os.WriteFile(path, script, 0755); err != nil {
		t.Fatal(err)
	}
	if target, err := resolve.ResolveTarget(f.root, ""); err != nil || target != "129-fixture" {
		t.Fatalf("resolver = %q, %v", target, err)
	}
	old := nextGateReadinessFn
	t.Cleanup(func() { nextGateReadinessFn = old })
	reached := false
	nextGateReadinessFn = func(root, id string, opts next.GateOptions) (*next.GateResult, error) {
		reached = true
		t.Setenv("SCOPED_LINEAGE_CHANGED", "1")
		return next.GateReadiness(root, id, opts)
	}
	before := f.audit(t)
	_, err = runScopedNext(t, f, "epic-129.1")
	if !reached || err == nil {
		t.Fatalf("resolved-scope mismatch did not refuse at the gate: reached=%v err=%v", reached, err)
	}
	requireText(t, err.Error(), "bead epic-129.1 resolves to spec 130-lineage by lineage but is being claimed for spec 129-fixture; nothing was claimed")
	assertScopedNoMutation(t, f, before, false)
}

func TestNext_ForeignDottedTitleRefuses(t *testing.T) {
	f := newScopedNextFixture(t)
	f.rows = []map[string]interface{}{nextHonestyRow("other-epic.1", "[129] foreign dotted bead", "", "task", "open")}
	f.epics[0]["status"] = "closed"
	f.write(t)
	before := f.audit(t)
	_, err := runScopedNext(t, f, "other-epic.1")
	if err == nil {
		t.Fatal("foreign dotted bead claimed")
	}
	requireText(t, err.Error(), "bead other-epic.1 is not a plan bead of spec 129-fixture (no epic-129.<N> ID); mindspec next claims only plan beads")
	assertScopedNoMutation(t, f, before, false)
}

func TestNext_UnscopedDispatchesIntoLineageSpec(t *testing.T) {
	f := newScopedNextFixture(t)
	// A closed epic has no active target; lineage can still resolve it.
	f.epics[0]["status"] = "closed"
	f.write(t)
	base := strings.TrimSpace(honestyGit(t, f.root, "rev-parse", "spec/129-fixture"))
	honestyGit(t, f.root, "commit", "--allow-empty", "-m", "HEAD ahead of spec")
	head := strings.TrimSpace(honestyGit(t, f.root, "rev-parse", "HEAD"))
	if base == head {
		t.Fatal("fixture needs distinct tips")
	}
	oldGate := nextGateReadinessFn
	defer func() { nextGateReadinessFn = oldGate }()
	nextGateReadinessFn = func(root, id string, opts next.GateOptions) (*next.GateResult, error) {
		if opts.ScopeSpec != "" {
			t.Fatalf("fixture unexpectedly resolved target spec %q", opts.ScopeSpec)
		}
		return next.GateReadiness(root, id, opts)
	}
	called := false
	nextEnsureWorktreeFn = func(root, id, spec string, e executor.Executor) (string, error) {
		called = true
		if spec != "129-fixture" {
			t.Fatalf("dispatch spec %q; wanted lineage", spec)
		}
		// Exercise the real EnsureWorktree/DispatchBead base selection.
		mock := &executor.MockExecutor{}
		if _, err := next.EnsureWorktree(root, id, spec, mock); err != nil {
			t.Fatal(err)
		}
		calls := mock.CallsTo("DispatchBead")
		if len(calls) != 1 || calls[0].Args[0] != id || calls[0].Args[1] != spec {
			t.Fatalf("DispatchBead argv: %+v", calls)
		}
		realExec := executor.NewMindspecExecutor(root)
		realExec.WorktreeOps = localNextWorktrees{t, root}
		return next.EnsureWorktree(root, id, spec, realExec)
	}
	out, err := runScopedNext(t, f, "epic-129.1")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !called {
		t.Fatal("EnsureWorktree never called")
	}
	tip := strings.TrimSpace(honestyGit(t, f.root, "rev-parse", "bead/epic-129.1"))
	if tip != base || tip == head {
		t.Fatalf("branch cut from HEAD: got %s want %s (HEAD %s)", tip, base, head)
	}
	requireText(t, out, "129-fixture")
}
func TestNext_ReattestRecoveryScopedFormParses(t *testing.T) {
	sourcePath, err := filepath.Abs("reattest.go")
	if err != nil {
		t.Fatal(err)
	}
	f := newScopedNextFixture(t)
	reached := false
	old := nextGateReadinessFn
	defer func() { nextGateReadinessFn = old }()
	sentinel := fmt.Errorf("stopped at readiness")
	nextGateReadinessFn = func(root, id string, opts next.GateOptions) (*next.GateResult, error) {
		reached = true
		if id != "epic-129.1" || !opts.FlagGiven || !reflect.DeepEqual(opts.NamedSignals, []string{"MF-3"}) || opts.EvidencePath == "" {
			t.Fatalf("parsed options: %s %+v", id, opts)
		}
		if _, err := os.Stat(opts.EvidencePath); err != nil {
			t.Fatal(err)
		}
		return nil, sentinel
	}
	// Read the actual recovery literal so production wording drift is tested.
	source, err := parser.ParseFile(token.NewFileSet(), sourcePath, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var forms []string
	ast.Inspect(source, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(value, "mindspec next <dependent>") {
			forms = append(forms, value)
		}
		return true
	})
	if len(forms) != 1 {
		t.Fatalf("expected one dependent recovery literal, got %v", forms)
	}
	form := strings.ReplaceAll(forms[0], "<dependent>", "epic-129.1")
	form = strings.ReplaceAll(form, "<path-to-justification>", nextEvidenceFile(t))
	args := strings.Fields(form)
	if len(args) < 3 || args[0] != "mindspec" || args[1] != "next" {
		t.Fatalf("unexpected recovery form: %q", form)
	}
	before := f.audit(t)
	_, err = runScopedNext(t, f, args[2:]...)
	if !reached || err != sentinel {
		t.Fatalf("recovery did not reach GateReadiness: %v reached %v", err, reached)
	}
	assertScopedNoMutation(t, f, before, false)
}

// Real git worktrees behind the executor's existing I/O seam; no bd process.
type localNextWorktrees struct {
	t    *testing.T
	root string
}

func (w localNextWorktrees) Create(name, branch string) error {
	honestyGit(w.t, w.root, "worktree", "add", name, branch)
	return nil
}
func (w localNextWorktrees) Remove(name string) error {
	entries, _ := w.List()
	for _, entry := range entries {
		if entry.Name == name || entry.Branch == "bead/"+name || entry.Name == "worktree-"+name {
			honestyGit(w.t, w.root, "worktree", "remove", entry.Path)
			return nil
		}
	}
	return nil
}
func (w localNextWorktrees) List() ([]bead.WorktreeListEntry, error) {
	raw := honestyGit(w.t, w.root, "worktree", "list", "--porcelain")
	var entries []bead.WorktreeListEntry
	for _, block := range strings.Split(strings.TrimSpace(raw), "\n\n") {
		var e bead.WorktreeListEntry
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				e.Path = strings.TrimPrefix(line, "worktree ")
				e.Name = filepath.Base(e.Path)
			}
			if strings.HasPrefix(line, "branch ") {
				e.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func scopedTrackerUpdates(t *testing.T, f *nextHonestyFixture) int {
	t.Helper()
	raw, err := os.ReadFile(f.path + ".argv")
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		if len(args) > 0 && args[0] == "update" {
			n++
		}
	}
	return n
}

func TestNext_BareAllowNotReady_DottedMFBeadRecovery(t *testing.T) {
	f := newScopedNextFixture(t)
	f.rows = []map[string]interface{}{nextHonestyRow("mf-abc.1", "[129] dotted bead", "", "task", "open")}
	f.write(t)
	before := f.audit(t)
	_, err := runScopedNext(t, f, "--allow-not-ready", "mf-abc.1")
	if err == nil {
		t.Fatal("bare flag accepted")
	}
	lines := strings.Split(err.Error(), "\n")
	if got, want := lines[len(lines)-1], "recovery: mindspec bead ready-check mf-abc.1"; got != want {
		t.Errorf("dotted bead recovery = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), "--allow-not-ready=mf-abc.1") {
		t.Error("bead ID echoed as signal authority")
	}
	assertScopedNoMutation(t, f, before, false)
	raw, err := os.ReadFile(f.path + ".argv")
	if err != nil {
		t.Fatal(err)
	}
	lookedUp := false
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		if reflect.DeepEqual(args, []string{"show", "mf-abc.1", "--json"}) {
			lookedUp = true
		}
	}
	if !lookedUp {
		t.Fatalf("dotted bead did not receive read-only recovery lookup: %s", raw)
	}
}
