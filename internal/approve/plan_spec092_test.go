package approve

// Spec 092 Bead 6 — plan-side hardening tests.
//
//   - Req 13a (mindspec-lawq, AC "lawq unit"): buildDesignField cites
//     ADRs by ID + title and inlines NO Decision text, bounding the
//     --design payload by construction.
//   - Req 13b (AC "lawq unit"): any mid-batch `bd create` failure
//     aborts with a structured guard failure naming the failing bead
//     heading, the offending field + byte size (when the cause is
//     Error 1105), the already-created bead IDs, and recovery lines.
//   - Req 15 (mindspec-e6qq, AC "e6qq unit"): a plan with N distinct
//     frontmatter/structure violations produces ONE error listing all
//     N (one bullet each) plus a final recovery line, in a single
//     plan-approve invocation.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

// finalRecoveryCommand extracts the command from the FINAL `recovery: `
// line of a guard-failure message (the line agents paste verbatim),
// failing the test when the message does not end with one.
func finalRecoveryCommand(t *testing.T, msg string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, guard.RecoveryPrefix) {
		t.Fatalf("final line is not a recovery line: %q in:\n%s", last, msg)
	}
	return strings.TrimPrefix(last, guard.RecoveryPrefix)
}

// assertNoBannedRecoveryLines applies the Req 19 per-site check to every
// recovery line in msg (Bead-9 punch-list pattern).
func assertNoBannedRecoveryLines(t *testing.T, msg string) {
	t.Helper()
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, guard.RecoveryPrefix) {
			if cmd := strings.TrimPrefix(line, guard.RecoveryPrefix); guard.IsBannedRecoveryCommand(cmd) {
				t.Errorf("recovery command %q is banned (Req 19); got:\n%s", cmd, msg)
			}
		}
	}
}

// writeLargeADRFixtures creates a canonical-layout project root with
// n ADRs, each carrying a LARGE Decision section, plus a spec dir.
// Returns (specDir, decisionMarker) where decisionMarker is a sentinel
// string present in every ADR's Decision body.
func writeLargeADRFixtures(t *testing.T, n int) (specDir, decisionMarker string) {
	t.Helper()
	tmp := t.TempDir()

	// buildDesignField derives root as specDir/../../.. — with the
	// canonical layout (<root>/.mindspec/docs/specs/<id>) that resolves
	// to <root>/.mindspec, whose docs dir is <root>/.mindspec/docs.
	specDir = filepath.Join(tmp, ".mindspec", "docs", "specs", "092-test")
	adrDir := filepath.Join(tmp, ".mindspec", "docs", "adr")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	decisionMarker = "INLINED-DECISION-SENTINEL"
	bigBody := strings.Repeat("This decision paragraph is deliberately verbose. ", 200) // ~10KB each
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("ADR-%04d", i)
		content := fmt.Sprintf(`# %s: Decision Title %d

- **Status**: Accepted
- **Domain(s)**: core

## Context

Some context.

## Decision

%s %s

## Consequences

Some consequences.
`, id, i, decisionMarker, bigBody)
		if err := os.WriteFile(filepath.Join(adrDir, id+".md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write adr: %v", err)
		}
	}
	return specDir, decisionMarker
}

// TestBuildDesignField_CitesADRsByID is the AC "lawq unit" structural
// half: a plan citing 8 large ADRs produces bead design fields
// containing by-ID citations (`see ADR-NNNN — <title>`) and no inlined
// Decision text.
func TestBuildDesignField_CitesADRsByID(t *testing.T) {
	const n = 8
	specDir, decisionMarker := writeLargeADRFixtures(t, n)

	// Spec 097 R2: the ADR list is built from the plan's structured
	// `adr_citations` IDs (passed in directly), not a spec-prose scrape.
	var adrCitationIDs []string
	for i := 1; i <= n; i++ {
		adrCitationIDs = append(adrCitationIDs, fmt.Sprintf("ADR-%04d", i))
	}

	design := buildDesignField(specDir, "1. Frob the widget", adrCitationIDs)

	// By-ID citations with the ADR's title, one per cited ADR.
	for i := 1; i <= n; i++ {
		want := fmt.Sprintf("see ADR-%04d — Decision Title %d", i, i)
		if !strings.Contains(design, want) {
			t.Errorf("design field should cite by ID + title %q; got:\n%s", want, design)
		}
	}
	// Where to find the full text.
	if !strings.Contains(design, ".mindspec/docs/adr/") {
		t.Errorf("design field should point at .mindspec/docs/adr/ for full text; got:\n%s", design)
	}
	// NO inlined Decision text.
	if strings.Contains(design, decisionMarker) {
		t.Errorf("design field must not inline ADR Decision text; sentinel found in:\n%s", design)
	}
	// Bounded by construction: 8 ADRs × ~10KB Decisions used to inline
	// ~80KB (over Dolt's ~65,535-byte Error-1105 ceiling); citations
	// keep the field small.
	if len(design) > 4096 {
		t.Errorf("design field should be bounded by construction; got %d bytes", len(design))
	}
}

// midBatchPlan is a three-bead plan used by the containment tests.
const midBatchPlan = `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan

## Bead 1: First thing

**Steps**
1. Step one

**Verification**
- [ ] Tests pass

**Depends on**
None

## Bead 2: Second thing

**Steps**
1. Step one

**Verification**
- [ ] Tests pass

**Depends on**
Bead 1

## Bead 3: Third thing

**Steps**
1. Step one

**Verification**
- [ ] Tests pass

**Depends on**
Bead 2
`

// TestCreateImplementationBeads_MidBatchFailureContainment is the AC
// "lawq unit" containment half: a forced mid-batch `bd create` failure
// (here: Dolt Error 1105 on the third bead) aborts with the failing
// bead heading, the offending field + byte size, the already-created
// bead IDs, and recovery lines — never a raw Error 1105 with a silent
// partial set.
func TestCreateImplementationBeads_MidBatchFailureContainment(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	if err := os.WriteFile(planPath, []byte(midBatchPlan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	calls := 0
	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			calls++
			if calls == 3 {
				return nil, fmt.Errorf("running bd create: Error 1105 (HY000): serialized transaction of size 70000 bytes exceeds limit")
			}
			return []byte(fmt.Sprintf(`{"id":"test-bead-%d"}`, calls)), nil
		}
		return nil, nil
	}

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err == nil {
		t.Fatal("expected a mid-batch containment error, got nil")
	}
	// The partial set is returned to the caller.
	if len(beadIDs) != 2 {
		t.Fatalf("expected the 2 already-created bead IDs, got %v", beadIDs)
	}
	msg := err.Error()

	// Names the failing bead heading.
	if !strings.Contains(msg, `"Bead 3: Third thing"`) {
		t.Errorf("error must name the failing bead heading; got:\n%s", msg)
	}
	// Lists the already-created bead IDs.
	if !strings.Contains(msg, "test-bead-1, test-bead-2") {
		t.Errorf("error must list the already-created bead IDs; got:\n%s", msg)
	}
	// 1105 cause → offending field + byte size reported.
	if !strings.Contains(msg, "likely oversized payload: --") || !strings.Contains(msg, "bytes") {
		t.Errorf("error must name the offending field and its byte size for Error 1105; got:\n%s", msg)
	}
	// Recovery: remove the partial set (INT-1: `bd delete --force`, the
	// only command that actually removes beads and thus converges — and
	// --force is mandatory, a bare `bd delete` is a preview-only no-op),
	// then re-run plan approve.
	if !strings.Contains(msg, "recovery: bd delete test-bead-1 test-bead-2 --force") {
		t.Errorf("recovery must remove the partial set via bd delete --force; got:\n%s", msg)
	}
	if !strings.Contains(msg, "recovery: mindspec plan approve 042-test") {
		t.Errorf("recovery must re-run plan approve; got:\n%s", msg)
	}
	// Req 12: per-site HasFinalRecoveryLine.
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
}

// TestCreateImplementationBeads_FirstBeadFailureNoPartialSet: a
// non-1105 failure on the FIRST create still aborts with a structured
// error and a recovery line, explicitly stating no partial set exists
// (and emitting no field-size guess, since the cause is not 1105).
func TestCreateImplementationBeads_FirstBeadFailureNoPartialSet(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	if err := os.WriteFile(planPath, []byte(midBatchPlan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bd daemon not reachable")
	}

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(beadIDs) != 0 {
		t.Fatalf("expected no bead IDs, got %v", beadIDs)
	}
	msg := err.Error()
	if !strings.Contains(msg, `"Bead 1: First thing"`) {
		t.Errorf("error must name the failing bead heading; got:\n%s", msg)
	}
	if !strings.Contains(msg, "no beads were created") {
		t.Errorf("error must state no partial set exists; got:\n%s", msg)
	}
	if strings.Contains(msg, "oversized payload") {
		t.Errorf("non-1105 failure must not guess an oversized field; got:\n%s", msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
}

// TestPlanValidationFailure_AggregatesAllIssues is the AC "e6qq unit"
// helper-level pin: N error-severity issues produce ONE failure with
// one bullet per issue and a single final recovery line.
func TestPlanValidationFailure_AggregatesAllIssues(t *testing.T) {
	vr := &validate.Result{SubCommand: "plan", TargetID: "042-test"}
	vr.AddError("frontmatter-version", "missing required field: version")
	vr.AddError("bead-verification", "Bead 1: First thing: missing verification steps")
	vr.AddError("bead-acceptance-criteria", "Bead 1: First thing: missing per-bead acceptance criteria — each bead must have an **Acceptance Criteria** section")
	vr.AddWarning("bead-depends", "Bead 1: First thing: no 'Depends on' declaration")

	err := planValidationFailure("042-test", vr)
	msg := err.Error()

	if !strings.Contains(msg, "3 issue(s)") {
		t.Errorf("failure must count all error issues; got:\n%s", msg)
	}
	for _, bullet := range []string{
		"- [frontmatter-version] missing required field: version",
		"- [bead-verification]",
		"- [bead-acceptance-criteria]",
	} {
		if !strings.Contains(msg, bullet) {
			t.Errorf("failure must carry bullet %q; got:\n%s", bullet, msg)
		}
	}
	// Warnings are not bulleted as blocking issues.
	if strings.Contains(msg, "[bead-depends]") {
		t.Errorf("warnings must not appear as blocking bullets; got:\n%s", msg)
	}
	// Exactly one recovery line, final.
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("failure must end with a `recovery:` line; got:\n%s", msg)
	}
	if got := strings.Count(msg, guard.RecoveryPrefix); got != 1 {
		t.Errorf("expected exactly one recovery line, got %d in:\n%s", got, msg)
	}
}

// TestApprovePlan_SinglePassAggregatedValidationError is the AC "e6qq
// unit" end-to-end pin: a single plan-approve invocation against a plan
// with multiple distinct violations reports them ALL at once.
func TestApprovePlan_SinglePassAggregatedValidationError(t *testing.T) {
	tmp := t.TempDir()
	specID := "042-test"
	specDir := filepath.Join(tmp, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Plan with multiple distinct violations: a bead missing steps +
	// verification + acceptance criteria. (version is intentionally absent but
	// no longer a violation — it auto-fills to "1" per 098 R3/e6qq.)
	planContent := `---
status: Draft
spec_id: "042-test"
---

# Plan

## ADR Fitness
Evaluated; none applicable.

## Testing Strategy
Unit tests.

## Bead 1: Broken bead

**Steps**

**Verification**

**Depends on**
None
`
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// Stub the phase seams so EnsureMigrated and the gate-consistency
	// check never shell out to bd.
	epicJSON := []byte(`[{"id":"epic-42","title":"[SPEC 042-test] Test","status":"open","issue_type":"epic","metadata":{"spec_num":42,"spec_title":"test","mindspec_phase":"plan"}}]`)
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return epicJSON, nil
			}
		}
		return []byte(`[]`), nil
	})
	defer restoreList()
	restoreRunBD := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return epicJSON, nil
		}
		return []byte(`[]`), nil
	})
	defer restoreRunBD()

	_, err := ApprovePlan(tmp, specID, "tester", nil)
	if err == nil {
		t.Fatal("expected an aggregated validation error, got nil")
	}
	msg := err.Error()

	// All distinct violations are present in the ONE error.
	for _, want := range []string{
		"[bead-steps]",
		"[bead-verification]",
		"[bead-acceptance-criteria]",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("aggregated error must list %s; got:\n%s", want, msg)
		}
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("aggregated error must end with a `recovery:` line; got:\n%s", msg)
	}
	if !strings.Contains(msg, "recovery: mindspec plan approve "+specID) {
		t.Errorf("recovery must re-run plan approve; got:\n%s", msg)
	}
}

// TestCreateImplementationBeads_DeleteRecoveryConverges is the INT-1
// convergence pin: the Req 13b recovery round-trip must actually WORK,
// not just read well.
//
//	(a) a mid-batch create failure leaves a partial set and emits
//	    `recovery: bd delete <ids> --force`;
//	(b) the OLD `bd close` recovery's outcome — the partial set left
//	    CLOSED under the epic — dead-ends the re-run in
//	    handleExistingBeads, and that dead end itself ends with a
//	    recovery line (no recovery-less wall);
//	(c) the NEW recovery's outcome — the partial set DELETED, children
//	    listing empty — lets the re-run succeed and create the full set.
func TestCreateImplementationBeads_DeleteRecoveryConverges(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	if err := os.WriteFile(planPath, []byte(midBatchPlan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	// Children-listing seam: mutable state standing in for the bd DB.
	origList := planListJSONFn
	defer func() { planListJSONFn = origList }()
	listState := []byte(`[]`) // first approval: no children yet
	planListJSONFn = func(args ...string) ([]byte, error) {
		return listState, nil
	}

	// bd-create fake: fail the third create while failThird is set.
	origRun := planRunBDFn
	defer func() { planRunBDFn = origRun }()
	calls := 0
	failThird := true
	planRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			calls++
			if failThird && calls == 3 {
				return nil, fmt.Errorf("running bd create: Error 1105 (HY000): serialized transaction of size 70000 bytes exceeds limit")
			}
			return []byte(fmt.Sprintf(`{"id":"test-bead-%d"}`, calls)), nil
		}
		return nil, nil // dep add etc.
	}

	// (a) First run: mid-batch failure → partial set + delete recovery.
	partial, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err == nil {
		t.Fatal("expected a mid-batch containment error, got nil")
	}
	if len(partial) != 2 {
		t.Fatalf("expected a 2-bead partial set, got %v", partial)
	}
	msg := err.Error()
	if !strings.Contains(msg, "recovery: bd delete test-bead-1 test-bead-2 --force") {
		t.Fatalf("recovery must delete the partial set with --force; got:\n%s", msg)
	}
	assertNoBannedRecoveryLines(t, msg)

	// (b) The old `bd close` outcome: partial set CLOSED under the epic.
	// The re-run is still rejected (Spec 074 supersede safety keeps its
	// teeth), but the rejection now carries its own recovery line.
	listState = []byte(`[{"id":"test-bead-1","status":"closed"},{"id":"test-bead-2","status":"closed"}]`)
	if _, err := createImplementationBeads(planPath, "042-test", "parent-123"); err == nil {
		t.Fatal("re-run with closed leftovers must still be rejected")
	} else if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("the closed-leftovers dead end must end with a `recovery:` line; got:\n%s", err)
	}

	// (c) Simulate the pasted `bd delete test-bead-1 test-bead-2 --force`:
	// the partial set is GONE, so the children listing is empty. The
	// re-run must now SUCCEED and create the full set.
	listState = []byte(`[]`)
	failThird = false
	calls = 0
	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err != nil {
		t.Fatalf("re-run after the emitted recovery must succeed, got: %v", err)
	}
	if len(beadIDs) != 3 {
		t.Fatalf("re-run must create the FULL set (3 beads), got %v", beadIDs)
	}
}

// TestHandleExistingBeads_DeadEndsEndWithRecovery is the second half of
// INT-1: the supersede-safety rejection keeps rejecting closed and
// in_progress children, but each rejection ends with a status-appropriate,
// non-banned recovery line naming the blocking bead (Req 12 + HC-5).
func TestHandleExistingBeads_DeadEndsEndWithRecovery(t *testing.T) {
	origList := planListJSONFn
	defer func() { planListJSONFn = origList }()

	t.Run("closed child", func(t *testing.T) {
		planListJSONFn = func(args ...string) ([]byte, error) {
			return []byte(`[{"id":"bead-done","status":"closed"}]`), nil
		}
		err := handleExistingBeads("epic-123", "version: 1\n")
		if err == nil {
			t.Fatal("closed child must still be rejected")
		}
		msg := err.Error()
		if !guard.HasFinalRecoveryLine(msg) {
			t.Fatalf("rejection must end with a `recovery:` line; got:\n%s", msg)
		}
		cmd := finalRecoveryCommand(t, msg)
		if cmd != "bd delete bead-done --force" {
			t.Errorf("closed-child recovery must name the bead: want %q, got %q", "bd delete bead-done --force", cmd)
		}
		if guard.IsBannedRecoveryCommand(cmd) {
			t.Errorf("recovery command %q is banned (Req 19)", cmd)
		}
		// The prose names BOTH ways into this state, so an agent only
		// deletes in the partial-create case.
		if !strings.Contains(msg, "completed work") || !strings.Contains(msg, "partial") {
			t.Errorf("prose must distinguish completed work from partial-create leftovers; got:\n%s", msg)
		}
	})

	t.Run("in_progress child", func(t *testing.T) {
		planListJSONFn = func(args ...string) ([]byte, error) {
			return []byte(`[{"id":"bead-active","status":"in_progress"}]`), nil
		}
		err := handleExistingBeads("epic-123", "version: 1\n")
		if err == nil {
			t.Fatal("in_progress child must still be rejected")
		}
		msg := err.Error()
		if !guard.HasFinalRecoveryLine(msg) {
			t.Fatalf("rejection must end with a `recovery:` line; got:\n%s", msg)
		}
		cmd := finalRecoveryCommand(t, msg)
		if cmd != "mindspec complete bead-active" {
			t.Errorf("in_progress recovery must complete the active bead: want %q, got %q", "mindspec complete bead-active", cmd)
		}
		if guard.IsBannedRecoveryCommand(cmd) {
			t.Errorf("recovery command %q is banned (Req 19)", cmd)
		}
	})
}

// deprecatedApproveOrder mirrors Bead 8's negative regex from
// internal/instruct/instruct_test.go: the deprecated verb-noun gate
// order (`mindspec approve ...` / `approve <noun>`) must not appear.
var deprecatedApproveOrder = regexp.MustCompile(`(?i)mindspec\s+approve\b|\bapprove\s+(spec|plan|impl)\b`)

// TestApprovePlan_MissingPlanUnapprovedSpec_CanonicalRecovery is the
// INT-2 per-site pin: plan-approve against a spec that was never
// approved (no plan.md) must hint the CANONICAL `mindspec spec approve
// <id>` via a Req 12 recovery line — never the deprecated
// `mindspec approve spec` order or a bare `Run:` hint.
func TestApprovePlan_MissingPlanUnapprovedSpec_CanonicalRecovery(t *testing.T) {
	tmp := t.TempDir()
	specID := "042-test"
	specDir := filepath.Join(tmp, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Draft spec, NO plan.md — the wrong-subcommand path.
	specContent := "---\nstatus: Draft\n---\n\n# Spec\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// No epic anywhere: EnsureMigrated no-ops and specIsApproved falls
	// back to the Draft frontmatter.
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer restoreList()
	restoreRunBD := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer restoreRunBD()

	_, err := ApprovePlan(tmp, specID, "tester", nil)
	if err == nil {
		t.Fatal("expected the unapproved-spec error, got nil")
	}
	msg := err.Error()

	if !strings.Contains(msg, "has not been approved yet") {
		t.Errorf("error must explain the spec is unapproved; got:\n%s", msg)
	}
	// (a) Canonical noun-verb hint.
	if !strings.Contains(msg, "mindspec spec approve "+specID) {
		t.Errorf("error must hint the canonical `mindspec spec approve %s`; got:\n%s", specID, msg)
	}
	// (b) Deprecated verb-noun order must NOT appear.
	if deprecatedApproveOrder.MatchString(msg) {
		t.Errorf("error must not use the deprecated `approve spec` order; got:\n%s", msg)
	}
	// (c) Req 12 final recovery line.
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
	// (d) The extracted recovery command is not banned.
	if cmd := finalRecoveryCommand(t, msg); guard.IsBannedRecoveryCommand(cmd) {
		t.Errorf("recovery command %q is banned (Req 19)", cmd)
	}
}
