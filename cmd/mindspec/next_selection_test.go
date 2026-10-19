package main

// next_selection_test.go pins spec 128 Bead 8's CLI-level selection ACs
// end to end against a real bd + git sandbox (reusing
// next_ready_gate_test.go's nextReadyGateSetupSandbox / Sandbox helpers,
// since bd's actual priority-respecting `bd ready` ordering is the thing
// under test):
//
//   - AC-9:  pick/emit parity + total pick semantics (single-ready-item
//     out-of-range pick errors).
//   - AC-10: a positional bead ID present in the raw ready set but
//     excluded by the spec filter names the exclusion reason, never the
//     false "not found or not ready".
//   - AC-10b: a non-empty raw ready set that filters to empty is a loud
//     error, never "No ready work found".
//   - AC-15: explicit --spec scope honesty — a nonexistent full-ID spec
//     and an existing draft spec both refuse without ever falling back
//     to the global, unscoped `bd ready` query.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/harness"
	"github.com/spf13/cobra"
)

// nextSelectionTwoReadyBeads creates an epic with two ready (unclaimed,
// undepended) beads titled to reference specID, with DETERMINISTIC
// differing priorities (bead1 higher priority than bead2) so `bd ready`'s
// priority-first order is stable across the test's several invocations.
func nextSelectionTwoReadyBeads(t *testing.T, sb *harness.Sandbox, specID string) (epicID, bead1, bead2 string) {
	t.Helper()
	epicID = sb.CreateSpecEpic(specID)
	bead1 = sb.CreateBead("["+specID+"] Alpha", "task", epicID)
	bead2 = sb.CreateBead("["+specID+"] Beta", "task", epicID)
	if out, err := sb.Run("bd", "update", bead1, "--priority", "1"); err != nil {
		t.Fatalf("bd update %s --priority 1: %v\n%s", bead1, err, out)
	}
	if out, err := sb.Run("bd", "update", bead2, "--priority", "3"); err != nil {
		t.Fatalf("bd update %s --priority 3: %v\n%s", bead2, err, out)
	}
	return epicID, bead1, bead2
}

// TestNextSelection_EmitOnlyPickParity_DifferentBeadsPerPick pins AC-9's
// core: --emit-only --pick 1 and --emit-only --pick 2 emit DIFFERENT
// beads, in bd ready's priority-first order, and --pick 3 (out of range)
// errors on the emit-only path.
func TestNextSelection_EmitOnlyPickParity_DifferentBeadsPerPick(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "978-selection-fixture"
	_, bead1, bead2 := nextSelectionTwoReadyBeads(t, sb, specID)

	out1, err := sb.Run("mindspec", "next", "--spec", specID, "--pick", "1", "--emit-only")
	if err != nil {
		t.Fatalf("--pick 1 --emit-only: %v\n%s", err, out1)
	}
	out2, err := sb.Run("mindspec", "next", "--spec", specID, "--pick", "2", "--emit-only")
	if err != nil {
		t.Fatalf("--pick 2 --emit-only: %v\n%s", err, out2)
	}
	// Both outputs list the full ready set (both bead IDs), so the
	// parity check is on the rendered PRIMER's own "**Bead**: <id>" line
	// — the actually-SELECTED bead — not on mere presence anywhere in
	// the ready-work listing.
	primer1 := "**Bead**: " + bead1
	primer2 := "**Bead**: " + bead2
	if !strings.Contains(out1, primer1) {
		t.Errorf("expected --pick 1 to emit the higher-priority bead %s (displayed row 1), got:\n%s", bead1, out1)
	}
	if !strings.Contains(out2, primer2) {
		t.Errorf("expected --pick 2 to emit %s, got:\n%s", bead2, out2)
	}
	if strings.Contains(out1, primer2) || strings.Contains(out2, primer1) {
		t.Errorf("--pick 1 and --pick 2 must emit DIFFERENT beads as the SELECTED primer:\npick1=%s\npick2=%s", out1, out2)
	}

	outRange, err := sb.Run("mindspec", "next", "--spec", specID, "--pick", "3", "--emit-only")
	if err == nil {
		t.Fatalf("expected --pick 3 (out of range on a 2-item ready set) to error, got exit 0:\n%s", outRange)
	}
	if !strings.Contains(outRange, "out of range") {
		t.Errorf("expected an 'out of range' error, got:\n%s", outRange)
	}
}

const nextSelectionParitySpecMD = `---
title: Selection Parity Fixture
status: Approved
---
# Selection Parity Fixture

## Acceptance Criteria
- [ ] AC-1 — bead 1 satisfies the readiness floor.
- [ ] AC-2 — bead 2 satisfies the readiness floor.
`

// nextSelectionParityPlanMD wires TWO independent (no depends_on between
// them) work chunks so both beads are simultaneously ready — mirroring
// next_ready_gate_test.go's nextReadyGatePositivePlanMD shape (a
// confirmed-passing readiness-floor fixture) so this test exercises real
// `bd ready`/readiness-gate plumbing rather than a bare epic+bead
// shortcut that skips the plan.md the readiness floor requires.
const nextSelectionParityPlanMD = `---
status: Draft
spec_id: 977-selection-parity
version: "1"
adr_citations: []
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/fixture/selection_parity_one.go
  - id: 2
    depends_on: []
    key_file_paths:
      - internal/fixture/selection_parity_two.go
---
# Plan: 977-selection-parity

## ADR Fitness

No existing ADRs are impacted; this is a self-contained test fixture.

## Testing Strategy

Unit tests via ` + "`go test`" + `.

## Bead 1: Alpha

**Steps**
1. Create internal/fixture/selection_parity_one.go
2. Implement the fixture function
3. Add a unit test

**Verification**
- [ ] ` + "`go test ./internal/fixture/`" + ` passes

**Acceptance Criteria**
- [ ] AC-1 — bead 1 satisfies the readiness floor.

**Depends on**
None

## Bead 2: Beta

**Steps**
1. Create internal/fixture/selection_parity_two.go
2. Implement the fixture function
3. Add a unit test

**Verification**
- [ ] ` + "`go test ./internal/fixture/`" + ` passes

**Acceptance Criteria**
- [ ] AC-2 — bead 2 satisfies the readiness floor.

**Depends on**
None
`

// TestNextSelection_ClaimPickMatchesEmitOnlyPick pins AC-9's cross-path
// parity: the claim path's --pick 1 claims the SAME bead --emit-only
// --pick 1 named. Uses a real plan-approve-backed two-bead fixture (both
// independently ready, matching next_ready_gate_test.go's confirmed-
// passing shape) since the readiness floor's plan.md/spec.md read is a
// hard error a bare epic+bead fixture can't satisfy.
func TestNextSelection_ClaimPickMatchesEmitOnlyPick(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "977-selection-parity"
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, specID, nextSelectionParitySpecMD, nextSelectionParityPlanMD)
	bead1 := epicID + ".1"
	bead2 := epicID + ".2"
	if out, err := sb.Run("bd", "update", bead1, "--priority", "1"); err != nil {
		t.Fatalf("bd update %s --priority 1: %v\n%s", bead1, err, out)
	}
	if out, err := sb.Run("bd", "update", bead2, "--priority", "3"); err != nil {
		t.Fatalf("bd update %s --priority 3: %v\n%s", bead2, err, out)
	}

	outEmit, err := sb.Run("mindspec", "next", "--spec", specID, "--pick", "1", "--emit-only")
	if err != nil {
		t.Fatalf("--pick 1 --emit-only: %v\n%s", err, outEmit)
	}
	if !strings.Contains(outEmit, "**Bead**: "+bead1) {
		t.Fatalf("expected --pick 1 --emit-only to name %s, got:\n%s", bead1, outEmit)
	}

	outClaim, err := sb.Run("mindspec", "next", "--spec", specID, "--pick", "1")
	if err != nil {
		t.Fatalf("--pick 1 (claim path): %v\n%s", err, outClaim)
	}
	if !strings.Contains(outClaim, bead1) {
		t.Errorf("expected the claim path's --pick 1 to claim %s (matching the emit-only preview), got:\n%s", bead1, outClaim)
	}

	status, _ := nextReadyGateBDStatus(t, sb, bead1)
	if status != "in_progress" {
		t.Errorf("expected %s to be claimed (in_progress) after --pick 1, got status %q", bead1, status)
	}
}

// TestNextSelection_SingleReadyItem_OutOfRangePickErrors is AC-9's
// pinned red-on-revert: an explicit out-of-range --pick against exactly
// ONE ready bead must error, never silently select the sole item.
func TestNextSelection_SingleReadyItem_OutOfRangePickErrors(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "976-selection-single"
	epicID := sb.CreateSpecEpic(specID)
	sb.CreateBead("["+specID+"] Solo", "task", epicID)

	out, err := sb.Run("mindspec", "next", "--spec", specID, "--pick", "2", "--emit-only")
	if err == nil {
		t.Fatalf("expected --pick 2 against a single-item ready set to error, got exit 0:\n%s", out)
	}
	if !strings.Contains(out, "out of range") {
		t.Errorf("expected an 'out of range' error, got:\n%s", out)
	}
}

// TestNextSelection_PositionalExcludedByFilter_NamesReason pins AC-10: a
// positional bead ID present in the raw ready set but excluded by
// mindspec's spec-title filter gets an error naming the exclusion
// reason — never the false "not found or not ready".
func TestNextSelection_PositionalExcludedByFilter_NamesReason(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "975-selection-excluded"
	epicID := sb.CreateSpecEpic(specID)
	// Title deliberately does NOT reference specID.
	beadID := sb.CreateBead("Unrelated work item", "task", epicID)

	out, err := sb.Run("mindspec", "next", "--spec", specID, beadID)
	if err == nil {
		t.Fatalf("expected the positional claim of a filter-excluded bead to refuse, got exit 0:\n%s", out)
	}
	if strings.Contains(out, "not found or not ready") {
		t.Errorf("AC-10: the false 'not found or not ready' message must not appear for a filter-excluded positional bead, got:\n%s", out)
	}
	if !strings.Contains(out, beadID) || !strings.Contains(out, "excluded") {
		t.Errorf("expected the refusal to name the excluded bead and the reason, got:\n%s", out)
	}
}

// TestNextSelection_AllExcludedByFilter_LoudError pins AC-10b: a
// non-empty raw ready set that the spec filter drops to EMPTY is a loud
// error naming the exclusions — never the silent "No ready work found"
// success line — on both the claim and --emit-only paths.
func TestNextSelection_AllExcludedByFilter_LoudError(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "974-selection-all-excluded"
	epicID := sb.CreateSpecEpic(specID)
	sb.CreateBead("Unrelated work one", "task", epicID)
	sb.CreateBead("Unrelated work two", "task", epicID)

	outClaim, err := sb.Run("mindspec", "next", "--spec", specID)
	if err == nil {
		t.Fatalf("expected the claim path to refuse when every ready bead is filter-excluded, got exit 0:\n%s", outClaim)
	}
	if strings.Contains(outClaim, "No ready work found") {
		t.Errorf("AC-10b: 'No ready work found' must not appear when the raw ready set was non-empty but fully filtered, got:\n%s", outClaim)
	}

	outEmit, err := sb.Run("mindspec", "next", "--spec", specID, "--emit-only")
	if err == nil {
		t.Fatalf("expected --emit-only to refuse the same way, got exit 0:\n%s", outEmit)
	}
	if strings.Contains(outEmit, "No ready work found") {
		t.Errorf("AC-10b: 'No ready work found' must not appear on --emit-only either, got:\n%s", outEmit)
	}
}

// TestNextSelection_ExplicitSpecScopeHonesty_NonexistentSpec pins
// AC-15(i): an explicit --spec naming a full spec ID that resolves to no
// epic and no on-disk draft refuses, on BOTH paths, without ever
// claiming/emitting a bead from a DIFFERENT, globally-ready epic — the
// removed silent global `bd ready` fallback would otherwise hand back
// exactly that other bead.
func TestNextSelection_ExplicitSpecScopeHonesty_NonexistentSpec(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	// A genuinely different, globally-ready epic — if the bogus --spec
	// silently fell back to global `bd ready`, THIS is the bead it would
	// hand back.
	otherEpic := sb.CreateSpecEpic("973-other-spec")
	otherBead := sb.CreateBead("[973-other-spec] Unrelated", "task", otherEpic)

	bogusSpec := "999-does-not-exist-anywhere"

	outClaim, err := sb.Run("mindspec", "next", "--spec", bogusSpec)
	if err == nil {
		t.Fatalf("expected the claim path to refuse for a nonexistent --spec, got exit 0:\n%s", outClaim)
	}
	if !strings.Contains(outClaim, bogusSpec) {
		t.Errorf("expected the refusal to name the nonexistent spec, got:\n%s", outClaim)
	}
	if strings.Contains(outClaim, otherBead) {
		t.Errorf("a nonexistent --spec must never fall back to a DIFFERENT spec's ready bead (AC-15(i)), got:\n%s", outClaim)
	}

	outEmit, err := sb.Run("mindspec", "next", "--spec", bogusSpec, "--emit-only")
	if err == nil {
		t.Fatalf("expected --emit-only to refuse the same way, got exit 0:\n%s", outEmit)
	}
	if strings.Contains(outEmit, otherBead) {
		t.Errorf("--emit-only must never fall back to a DIFFERENT spec's ready bead either, got:\n%s", outEmit)
	}

	status, _ := nextReadyGateBDStatus(t, sb, otherBead)
	if status != "open" {
		t.Errorf("the other spec's bead must remain untouched (open) after the refused invocations, got status %q", status)
	}
}

// TestNextSelection_ExplicitSpecScopeHonesty_ExistingDraft pins
// AC-15(ii): an existing on-disk draft spec (no epic yet) refuses with
// the honest no-epic/not-approved message, never a false "doesn't exist"
// claim, and never selects a bead from another spec.
func TestNextSelection_ExplicitSpecScopeHonesty_ExistingDraft(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	otherEpic := sb.CreateSpecEpic("972-other-spec")
	otherBead := sb.CreateBead("[972-other-spec] Unrelated", "task", otherEpic)

	draftSpecID := "971-draft-fixture"
	sb.WriteFile(".mindspec/specs/"+draftSpecID+"/spec.md", "# draft\n")
	sb.Commit("setup: draft spec directory")
	if out, err := sb.Run("git", "branch", "spec/"+draftSpecID); err != nil {
		t.Fatalf("git branch spec/%s: %v\n%s", draftSpecID, err, out)
	}

	out, err := sb.Run("mindspec", "next", "--spec", draftSpecID)
	if err == nil {
		t.Fatalf("expected the claim path to refuse for a draft (no-epic) --spec, got exit 0:\n%s", out)
	}
	if !strings.Contains(out, "no epic") {
		t.Errorf("expected the honest no-epic/not-approved message, got:\n%s", out)
	}
	if strings.Contains(out, otherBead) {
		t.Errorf("a draft --spec must never select a bead from ANOTHER spec (AC-15(ii)), got:\n%s", out)
	}

	status, _ := nextReadyGateBDStatus(t, sb, otherBead)
	if status != "open" {
		t.Errorf("the other spec's bead must remain untouched (open) after the refused invocation, got status %q", status)
	}
}

// --- Round-1 panel G1: the spec-mandated pinned-line tests were missing ---

// TestNextSelection_EmitOnlyDefaultPick_PrintsLineAndSelectsRow1 pins
// AC-9's "the no-pick default path prints the self-describing line" on
// the --emit-only display path: with >=2 ready beads and neither --pick
// nor a positional given, the EXACT OQ-5 default line is printed, and
// the primer that's actually emitted is displayed row 1 (the
// higher-priority bead).
func TestNextSelection_EmitOnlyDefaultPick_PrintsLineAndSelectsRow1(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "968-selection-default-emit"
	_, bead1, _ := nextSelectionTwoReadyBeads(t, sb, specID)

	out, err := sb.Run("mindspec", "next", "--spec", specID, "--emit-only")
	if err != nil {
		t.Fatalf("default-pick --emit-only: %v\n%s", err, out)
	}
	const wantLine = "Defaulting to item 1 (bd ready priority order). Use --pick=N or a bead ID to select a specific item."
	if !strings.Contains(out, wantLine) {
		t.Errorf("expected the exact OQ-5 default line, got:\n%s", out)
	}
	if !strings.Contains(out, "**Bead**: "+bead1) {
		t.Errorf("expected the default pick to select displayed row 1 (%s), got:\n%s", bead1, out)
	}
}

// nextSelectionDefaultClaimSpecMD/PlanMD mirror
// nextSelectionParitySpecMD/PlanMD (a confirmed-passing two-independent-
// bead readiness fixture) under a fresh spec ID so this test's real
// claim doesn't collide with TestNextSelection_ClaimPickMatchesEmitOnlyPick's.
const nextSelectionDefaultClaimSpecID = "967-selection-default-claim"

// TestNextSelection_ClaimDefaultPick_PrintsLineAndClaimsRow1 pins AC-9's
// default-line assertion on the CLAIM path: with >=2 ready beads and no
// selector given, the exact OQ-5 default line prints AND the bead
// actually claimed is displayed row 1 (the higher-priority bead) —
// closing the round-1 G1(a) gap (grep 'Defaulting' in *_test.go
// previously returned zero).
func TestNextSelection_ClaimDefaultPick_PrintsLineAndClaimsRow1(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	planMD := strings.ReplaceAll(nextSelectionParityPlanMD, "977-selection-parity", nextSelectionDefaultClaimSpecID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, nextSelectionDefaultClaimSpecID, nextSelectionParitySpecMD, planMD)
	bead1 := epicID + ".1"
	bead2 := epicID + ".2"
	if out, err := sb.Run("bd", "update", bead1, "--priority", "1"); err != nil {
		t.Fatalf("bd update %s --priority 1: %v\n%s", bead1, err, out)
	}
	if out, err := sb.Run("bd", "update", bead2, "--priority", "3"); err != nil {
		t.Fatalf("bd update %s --priority 3: %v\n%s", bead2, err, out)
	}

	out, err := sb.Run("mindspec", "next", "--spec", nextSelectionDefaultClaimSpecID)
	if err != nil {
		t.Fatalf("default-pick claim: %v\n%s", err, out)
	}
	const wantLine = "Defaulting to item 1 (bd ready priority order). Use --pick=N or a bead ID to select a specific item."
	if !strings.Contains(out, wantLine) {
		t.Errorf("expected the exact OQ-5 default line, got:\n%s", out)
	}
	if !strings.Contains(out, bead1) {
		t.Errorf("expected the default pick to claim displayed row 1 (%s), got:\n%s", bead1, out)
	}

	status, _ := nextReadyGateBDStatus(t, sb, bead1)
	if status != "in_progress" {
		t.Errorf("expected %s (displayed row 1) to be claimed by the no-selector default path, got status %q", bead1, status)
	}
	statusOther, _ := nextReadyGateBDStatus(t, sb, bead2)
	if statusOther != "open" {
		t.Errorf("expected %s (displayed row 2) to remain untouched by the default pick, got status %q", bead2, statusOther)
	}
}

// TestNextSelection_PositionalEmitOnly_BypassesSpecFilter pins the round-1
// G1(b) gap: R6.3(a)'s one documented exception. `--spec <s> <bead-id>
// --emit-only` against a ready bead whose title does NOT reference the
// spec must still proceed (R6.1's pinned bypass — positional --emit-only
// never constructs the ready set, never applies the spec filter), print
// ONLY the pinned bypass line, with no exclusion report and no false
// readiness claim.
func TestNextSelection_PositionalEmitOnly_BypassesSpecFilter(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "966-selection-bypass"
	epicID := sb.CreateSpecEpic(specID)
	// Title deliberately does NOT reference specID — this bead would be
	// EXCLUDED by the spec-title filter on every OTHER path.
	beadID := sb.CreateBead("Unrelated work item", "task", epicID)

	out, err := sb.Run("mindspec", "next", "--spec", specID, beadID, "--emit-only")
	if err != nil {
		t.Fatalf("expected the positional --emit-only preview to proceed despite the title not referencing the spec, got: %v\n%s", err, out)
	}
	const wantBypassLine = "emit-only: bead fetched by ID; readiness not checked."
	if !strings.Contains(out, wantBypassLine) {
		t.Errorf("expected the exact pinned bypass line, got:\n%s", out)
	}
	if strings.Contains(out, "excluded") {
		t.Errorf("positional --emit-only must report NO exclusions (R6.3(a)'s documented exception — it never constructs the ready set), got:\n%s", out)
	}
	if strings.Contains(out, "is ready") || strings.Contains(out, "Ready work") {
		t.Errorf("positional --emit-only must never claim readiness — 'readiness not checked' is the only readiness-shaped statement, got:\n%s", out)
	}
	if !strings.Contains(out, "**Bead**: "+beadID) {
		t.Errorf("expected the primer to still be emitted for the fetched bead, got:\n%s", out)
	}
}

// --- Round-1 panel G2: --pick < 0 boundary, validated before ANY side effect ---

// TestNextSelection_NegativePick_PositionalConflict_ZeroSideEffects pins
// that positional + --pick=-1 refuses on BOTH paths before ANY fetch or
// claim, and before the (unrelated) positional-vs-positive-pick mutual-
// exclusion message — the negative-pick check runs first and
// unconditionally.
func TestNextSelection_NegativePick_PositionalConflict_ZeroSideEffects(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "965-selection-negative-pick"
	epicID := sb.CreateSpecEpic(specID)
	beadID := sb.CreateBead("["+specID+"] Solo", "task", epicID)

	outClaim, err := sb.Run("mindspec", "next", beadID, "--pick=-1")
	if err == nil {
		t.Fatalf("expected positional + --pick=-1 to refuse on the claim path, got exit 0:\n%s", outClaim)
	}
	if !strings.Contains(outClaim, "pick") || !strings.Contains(outClaim, "invalid") {
		t.Errorf("expected the invalid-pick refusal, got:\n%s", outClaim)
	}
	statusClaim, _ := nextReadyGateBDStatus(t, sb, beadID)
	if statusClaim != "open" {
		t.Errorf("expected zero side effects (bead unclaimed) after the refused claim-path invocation, got status %q", statusClaim)
	}

	outEmit, err := sb.Run("mindspec", "next", beadID, "--pick=-1", "--emit-only")
	if err == nil {
		t.Fatalf("expected positional + --pick=-1 to refuse on --emit-only too, got exit 0:\n%s", outEmit)
	}
	if strings.Contains(outEmit, "emit-only: bead fetched by ID") {
		t.Errorf("the invalid-pick refusal must preempt --emit-only's positional bypass entirely — no bypass line, got:\n%s", outEmit)
	}
	statusEmit, _ := nextReadyGateBDStatus(t, sb, beadID)
	if statusEmit != "open" {
		t.Errorf("expected zero side effects after the refused --emit-only invocation, got status %q", statusEmit)
	}
}

// TestNextSelection_NegativePick_EmptyReadySet_NeverSucceeds pins that an
// invalid --pick refuses even when the (would-be) ready set is entirely
// empty — never the silent "No ready work found" success path, on
// either the claim or --emit-only path.
func TestNextSelection_NegativePick_EmptyReadySet_NeverSucceeds(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	specID := "964-selection-negative-empty"
	sb.CreateSpecEpic(specID) // epic with ZERO children — genuinely empty ready set

	outClaim, err := sb.Run("mindspec", "next", "--spec", specID, "--pick=-1")
	if err == nil {
		t.Fatalf("expected --pick=-1 against an empty ready set to refuse (claim path), got exit 0:\n%s", outClaim)
	}
	if strings.Contains(outClaim, "No ready work found") {
		t.Errorf("an invalid pick must refuse as an invalid-pick error, never render as the unrelated 'No ready work found' success line, got:\n%s", outClaim)
	}
	if !strings.Contains(outClaim, "invalid") {
		t.Errorf("expected the invalid-pick refusal, got:\n%s", outClaim)
	}

	outEmit, err := sb.Run("mindspec", "next", "--spec", specID, "--pick=-1", "--emit-only")
	if err == nil {
		t.Fatalf("expected --pick=-1 to refuse on --emit-only too, got exit 0:\n%s", outEmit)
	}
	if strings.Contains(outEmit, "No ready work found") || strings.Contains(outEmit, "no ready work found") {
		t.Errorf("an invalid pick must never render as any 'no ready work' outcome, got:\n%s", outEmit)
	}
	if !strings.Contains(outEmit, "invalid") {
		t.Errorf("expected the invalid-pick refusal, got:\n%s", outEmit)
	}
}

// TestNextSelection_PositionalEmitOnly_HostileIDRendersEscaped pins
// round-1 panel G3's audit of the emit-only positional fetch-error wrap:
// a hostile positional ID never leaks a raw control byte, whether it
// fails idvalidate.BeadID inside FetchBeadByID or in the outer
// "fetching bead %s" wrap this bead's fix now routes through
// idrender.Bead.
func TestNextSelection_PositionalEmitOnly_HostileIDRendersEscaped(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	hostile := "mindspec-\x1b[31mFORGED\x1b[0m"

	out, err := sb.Run("mindspec", "next", hostile, "--emit-only")
	if err == nil {
		t.Fatalf("expected a hostile positional ID to refuse, got exit 0:\n%s", out)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("raw ESC byte survived into the rendered refusal: %q", out)
	}
}

// --- Round-2 panel residual: R6.3(a)'s partial-exclusion NOTICE is
// actually PRINTED, not just content-pinned at the unit level. ---
//
// internal/next.ReadySet.ExclusionNotice() was already unit-tested for
// its CONTENT (internal/next/readyset_test.go), but nothing at the CLI
// level asserted the two next.go call sites
// (fmt.Fprintln(os.Stderr, notice)) actually run — deleting both would
// have stayed green. These fixtures mix a plan-approved (readiness-
// satisfying) two-bead set with a THIRD, bare, title-unreferenced bead
// under the SAME epic so the filter partially excludes (not fully —
// AC-10b's loud-error path is a different, already-covered case) while
// the claim/preview still succeeds on the remaining, matching bead.

const nextSelectionNoticeClaimSpecID = "963-selection-notice-claim"
const nextSelectionNoticeEmitSpecID = "962-selection-notice-emit"

// TestNextSelection_ClaimPath_PartialExclusionNoticePrinted pins the
// claim-path call site: with a partial filter exclusion, the "excluded
// by spec filter" notice is actually printed to the command's output
// (not merely computable), and the claim of the remaining, matching bead
// still succeeds.
func TestNextSelection_ClaimPath_PartialExclusionNoticePrinted(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	planMD := strings.ReplaceAll(nextSelectionParityPlanMD, "977-selection-parity", nextSelectionNoticeClaimSpecID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, nextSelectionNoticeClaimSpecID, nextSelectionParitySpecMD, planMD)
	bead1 := epicID + ".1"
	bead2 := epicID + ".2"
	if out, err := sb.Run("bd", "update", bead1, "--priority", "1"); err != nil {
		t.Fatalf("bd update %s --priority 1: %v\n%s", bead1, err, out)
	}
	if out, err := sb.Run("bd", "update", bead2, "--priority", "3"); err != nil {
		t.Fatalf("bd update %s --priority 3: %v\n%s", bead2, err, out)
	}
	// A THIRD, bare, ready bead under the SAME epic whose title does NOT
	// reference the spec — the spec-title filter drops it (partial
	// exclusion: 1 of 3 raw items), while bead1/bead2 still pass.
	excludedBead := sb.CreateBead("Unrelated work item", "task", epicID)

	out, err := sb.Run("mindspec", "next", "--spec", nextSelectionNoticeClaimSpecID)
	if err != nil {
		t.Fatalf("expected the claim to succeed on the remaining matching bead despite the partial exclusion: %v\n%s", err, out)
	}
	if !strings.Contains(out, "excluded by spec filter") {
		t.Errorf("expected the partial-exclusion notice to be PRINTED (not merely computable) on the claim path, got:\n%s", out)
	}
	if !strings.Contains(out, bead1) {
		t.Errorf("expected the default pick to still claim %s (the higher-priority MATCHING bead), got:\n%s", bead1, out)
	}
	status, _ := nextReadyGateBDStatus(t, sb, bead1)
	if status != "in_progress" {
		t.Errorf("expected %s to be claimed despite the partial exclusion, got status %q", bead1, status)
	}
	excludedStatus, _ := nextReadyGateBDStatus(t, sb, excludedBead)
	if excludedStatus != "open" {
		t.Errorf("the filter-excluded bead must remain untouched (open), got status %q", excludedStatus)
	}
}

// TestNextSelection_EmitOnlyPath_PartialExclusionNoticePrinted pins the
// --emit-only call site the same way, on a bare (no plan.md) epic+bead
// fixture — --emit-only never runs the readiness gate, so no plan
// approval is needed to get a clean success here.
func TestNextSelection_EmitOnlyPath_PartialExclusionNoticePrinted(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)
	epicID, bead1, _ := nextSelectionTwoReadyBeads(t, sb, nextSelectionNoticeEmitSpecID)
	sb.CreateBead("Unrelated work item", "task", epicID)

	out, err := sb.Run("mindspec", "next", "--spec", nextSelectionNoticeEmitSpecID, "--emit-only")
	if err != nil {
		t.Fatalf("expected the --emit-only preview to succeed despite the partial exclusion: %v\n%s", err, out)
	}
	if !strings.Contains(out, "excluded by spec filter") {
		t.Errorf("expected the partial-exclusion notice to be PRINTED (not merely computable) on the --emit-only path, got:\n%s", out)
	}
	if !strings.Contains(out, "**Bead**: "+bead1) {
		t.Errorf("expected the default pick to still preview %s (the higher-priority MATCHING bead), got:\n%s", bead1, out)
	}
}

func TestNext_101ReadyBeads_AllVisible(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	rows := make([]map[string]interface{}, 101)
	for i := range rows {
		rows[i] = map[string]interface{}{"id": fmt.Sprintf("epic-129.%d", i+1), "title": fmt.Sprintf("Ready row %d", i+1), "status": "open", "issue_type": "task"}
	}
	for name, n := range map[string]int{"all": 101, "capped": 100} {
		raw, err := json.Marshal(rows[:n])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
case "$1" in
 ready)
  previous=''; unlimited=no
  for arg in "$@"; do
   if { [ "$previous" = '-n' ] || [ "$previous" = '--limit' ]; } && [ "$arg" = '0' ]; then unlimited=yes; fi
   previous="$arg"
  done
  if [ "$unlimited" = yes ]; then cat "$ROWCAP_FIXTURE/all"; else cat "$ROWCAP_FIXTURE/capped"; fi
  ;;
 show) printf '[{"id":"%s","title":"Selected row","description":"Fixture work"}]' "$2" ;;
 *) printf 'unexpected fake bd command: %s\n' "$1" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ROWCAP_FIXTURE", dir)
	// Pin the fake's cap semantics independently of the production waist's spelling.
	for _, limit := range [][]string{nil, {"-n", "0"}, {"--limit", "0"}} {
		args := append([]string{"ready"}, limit...)
		raw, err := exec.Command(filepath.Join(dir, "bd"), args...).Output()
		if err != nil {
			t.Fatal(err)
		}
		var got []map[string]interface{}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		want := 101
		if limit == nil {
			want = 100
		}
		if len(got) != want {
			t.Fatalf("capping fake with %q returned %d rows, want %d", args, len(got), want)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, pick := range []int{0, 101} {
		// A file avoids pipe-buffer deadlocks while the 101-item menu is written.
		f, err := os.CreateTemp(dir, "stdout-")
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stdout
		os.Stdout = f
		// Fresh flags keep the real CLI RunE independent of other CLI tests.
		cmd := &cobra.Command{Use: "next", RunE: nextCmd.RunE}
		cmd.Flags().Int("pick", 0, "")
		cmd.Flags().String("spec", "", "")
		cmd.Flags().Bool("force", false, "")
		cmd.Flags().Bool("emit-only", false, "")
		cmd.Flags().Bool("allow-not-ready", false, "")
		cmd.SetArgs([]string{"--emit-only", fmt.Sprintf("--pick=%d", pick)})
		err = cmd.Execute()
		os.Stdout = orig
		if closeErr := f.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		raw, readErr := os.ReadFile(f.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		out := string(raw)
		if err != nil {
			t.Fatalf("pick %d: %v\n%s", pick, err, out)
		}
		if !strings.Contains(out, "Ready work (101 items):") {
			t.Fatalf("101 rows must be visible:\n%s", out)
		}
		for i := 1; i <= 101; i++ {
			if !strings.Contains(out, fmt.Sprintf("epic-129.%d", i)) {
				t.Fatalf("row %d missing", i)
			}
		}
		if pick == 101 && !strings.Contains(out, "**Bead**: epic-129.101") {
			t.Fatalf("pick 101 did not emit last bead:\n%s", out)
		}
	}
}
