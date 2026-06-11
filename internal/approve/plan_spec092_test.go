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
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

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

	var touchpoints []string
	for i := 1; i <= n; i++ {
		touchpoints = append(touchpoints, fmt.Sprintf("- [ADR-%04d](../../adr/ADR-%04d.md)", i, i))
	}
	specContent := "# Spec\n\n## ADR Touchpoints\n" + strings.Join(touchpoints, "\n") + "\n\n## Requirements\n1. Frob the widget\n"

	design := buildDesignField(specDir, specContent, "1. Frob the widget")

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
	// Recovery: remove the partial set, then re-run plan approve.
	if !strings.Contains(msg, "recovery: bd close test-bead-1 test-bead-2") {
		t.Errorf("recovery must remove the partial set; got:\n%s", msg)
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
	// Plan with multiple distinct violations: missing version (frontmatter),
	// a bead missing steps + verification + acceptance criteria.
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
		"[frontmatter-version]",
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
