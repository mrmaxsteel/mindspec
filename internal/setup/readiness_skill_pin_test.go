package setup

// readiness_skill_pin_test.go — spec 124 (impl-readiness-gate) Bead 3:
// content pins over the SHIPPED plugin skill content
// (pluginmindspec.SkillFiles(), the same surface `mindspec setup <agent>`
// installs from — the spec-123 AC-17 pattern, internal/setup/
// adhoc_panel_skill_test.go's precedent) so the prose layer can never
// silently detach from the binary (ADR-0040 no-restate, R7b).
//
// AC-5: the unconditional dispatch-ingress invocation, reachable from
// BOTH the prompt-path and manual-fallback dispatch flows, plus the
// override-marker proceed-with-warning rule.
// AC-6: the Phase 0 contract — all five SR signal IDs, the zero-commit
// rule, the "NOT READY: <bead-id>" ordinal-numbered verbatim-span shape,
// and the R8c anti-browbeat clarification-handling rule.
// AC-7: the ms-bead-cycle NOT-READY routing — no panel round, the
// max_consecutive_impl_failures exclusion, never /ms-bead-fix, the
// ACCEPT halt-and-surface disposition.
// AC-13: the bounded-loop contract — both dispositions, the grounded
// reason-keyed clarification requirement, the categorical per-bead cap,
// and the clarify-vs---allow-not-ready distinction.

import (
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

func readinessSkillFile(t *testing.T, name string) string {
	t.Helper()
	files := pluginmindspec.SkillFiles()
	content, ok := files[name]
	if !ok {
		t.Fatalf("%s SKILL.md not found among embedded plugin skills", name)
	}
	return content
}

// --- AC-5: unconditional dispatch ingress ---

func TestMsBeadImplSkill_IngressPrecedesPhaseAAndNamesBothPaths(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")

	ingressIdx := strings.Index(content, "## Ingress — readiness re-check")
	if ingressIdx < 0 {
		t.Fatal("expected an '## Ingress — readiness re-check' section in the shipped ms-bead-impl SKILL.md")
	}
	phaseAIdx := strings.Index(content, "## Phase A — stage the prompt")
	if phaseAIdx < 0 {
		t.Fatal("expected a '## Phase A — stage the prompt' section")
	}
	if ingressIdx >= phaseAIdx {
		t.Errorf("the ingress section must precede Phase A (so prompt-path/manual-fallback dispatch cannot skip it); ingress at %d, Phase A at %d", ingressIdx, phaseAIdx)
	}

	if !strings.Contains(content, "mindspec bead ready-check") {
		t.Error("expected the ready-check invocation in the shipped SKILL.md")
	}
	if !strings.Contains(content, "prompt-path") {
		t.Error("expected the ingress section to name the prompt-path dispatch flow")
	}
	if !strings.Contains(content, "manual") {
		t.Error("expected the ingress section to name the manual-fallback dispatch flow")
	}
	if !strings.Contains(content, "mindspec_readiness_override") {
		t.Error("expected the override-marker name in the shipped SKILL.md")
	}
	if !strings.Contains(content, "proceed") || !strings.Contains(content, "warning") {
		t.Error("expected the override-honoring proceed-with-warning rule in the shipped SKILL.md")
	}
}

// --- AC-6: Phase 0 contract ---

func TestMsBeadImplSkill_Phase0ContractShipped(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")

	if !strings.Contains(content, "Phase 0 — readiness review") {
		t.Fatal("expected a 'Phase 0 — readiness review' block in the shipped SKILL.md")
	}
	for _, sr := range []string{"SR-1", "SR-2", "SR-3", "SR-4", "SR-5"} {
		if !strings.Contains(content, sr) {
			t.Errorf("expected signal ID %s in the shipped Phase 0 block", sr)
		}
	}
	if !strings.Contains(content, "zero commits") {
		t.Error("expected the zero-commit rule in the shipped Phase 0 block")
	}
	if !strings.Contains(content, "NOT READY: <bead-id>") {
		t.Error("expected the exact 'NOT READY: <bead-id>' first-line contract in the shipped Phase 0 block")
	}
	if !strings.Contains(content, "ordinal") {
		t.Error("expected the ordinal-numbering contract in the shipped Phase 0 block")
	}
	if !strings.Contains(content, "verbatim") {
		t.Error("expected the verbatim-span-quoting contract in the shipped Phase 0 block")
	}
	if !mutationProbeClarificationRulePresent(content) {
		t.Error("expected the R8c clarification-handling (anti-browbeat) rule in the shipped Phase 0 block")
	}
	// FX-1 (codex-G3): the anti-browbeat rule can only FUNCTION if the
	// re-dispatch injection pairs, per ordinal, the ORIGINAL cited reason
	// with its clarification — otherwise Phase 0 never sees the reason it
	// is supposed to judge as resolved. Pin that pairing (both the ingress
	// instruction and the Readiness Clarification render template).
	if !mutationProbeReasonClarificationPairingPresent(content) {
		t.Error("expected the re-dispatch injection to PAIR the original reason with its clarification per ordinal (FX-1) — the anti-browbeat rule cannot function otherwise")
	}
}

// mutationProbeClarificationRulePresent isolates the exact anti-browbeat
// substring this test (and its own mutation-probe companion below) both
// check, so the two can never silently drift apart.
func mutationProbeClarificationRulePresent(content string) bool {
	return strings.Contains(content, "re-reported NOT READY") &&
		strings.Contains(content, "Clarification-handling rule")
}

// mutationProbeReasonClarificationPairingPresent isolates the FX-1
// per-ordinal reason↔clarification pairing requirement: the ingress
// instruction must say the ORIGINAL cited reason is rendered ALONGSIDE
// its clarification, and the Readiness Clarification render template must
// carry both a `reason:` and a `clarification:` field on one line.
func mutationProbeReasonClarificationPairingPresent(content string) bool {
	return strings.Contains(content, "original cited NOT-READY reason") &&
		strings.Contains(content, "reason: <original verbatim reason>") &&
		strings.Contains(content, "clarification: <answer>")
}

// TestMsBeadImplSkill_ClarificationRuleMutationProbe (recorded mutation
// probe, plan-gate step-5 verification item): stripping the
// clarification-handling rule out of a copy of the shipped content turns
// the SAME predicate TestMsBeadImplSkill_Phase0ContractShipped relies on
// red — proving the pin is not vacuously true (e.g. matching on a
// generic word present elsewhere for unrelated reasons).
func TestMsBeadImplSkill_ClarificationRuleMutationProbe(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeClarificationRulePresent(content) {
		t.Fatal("precondition failed: the shipped content must carry the clarification-handling rule")
	}

	start := strings.Index(content, "**Clarification-handling rule")
	if start < 0 {
		t.Fatal("could not locate the clarification-handling rule paragraph to mutate")
	}
	end := strings.Index(content[start:], "\n\n")
	if end < 0 {
		end = len(content) - start
	}
	mutated := content[:start] + content[start+end:]

	if mutationProbeClarificationRulePresent(mutated) {
		t.Fatal("mutation probe failed: stripping the clarification-handling paragraph should turn the pin red")
	}
}

// TestMsBeadImplSkill_ReasonPairingMutationProbe (FX-1 codex-G3): dropping
// the ORIGINAL-reason rendering from the Readiness Clarification template
// turns the pairing pin red — proving the pin genuinely requires the
// reason to be rendered alongside the clarification (not merely that some
// generic "reason" word appears somewhere).
func TestMsBeadImplSkill_ReasonPairingMutationProbe(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeReasonClarificationPairingPresent(content) {
		t.Fatal("precondition failed: the shipped content must pair the original reason with its clarification")
	}
	// Simulate a regression that drops the original-reason field from the
	// render template, keeping only the answer.
	mutated := strings.Replace(content, "reason: <original verbatim reason> — ", "", 1)
	mutated = strings.Replace(mutated, "the original cited NOT-READY reason", "the clarification answer", 1)
	if mutationProbeReasonClarificationPairingPresent(mutated) {
		t.Fatal("mutation probe failed: dropping the original-reason rendering should turn the pairing pin red")
	}
}

// --- AC-7: ms-bead-cycle NOT-READY routing ---

func TestMsBeadCycleSkill_NotReadyRoutingShipped(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-cycle")

	if !strings.Contains(content, "NOT READY") {
		t.Fatal("expected 'NOT READY' documented in the shipped ms-bead-cycle SKILL.md")
	}
	if !strings.Contains(content, "No panel round is consumed") {
		t.Error("expected the no-panel-round rule")
	}
	if !strings.Contains(content, "max_consecutive_impl_failures") {
		t.Error("expected the max_consecutive_impl_failures exclusion named")
	}
	if !strings.Contains(content, "Never routed to `/ms-bead-fix`") {
		t.Error("expected the never-routed-to-fix rule")
	}
	if !strings.Contains(content, "ACCEPT") || !strings.Contains(content, "halt") {
		t.Error("expected the ACCEPT halt-and-surface disposition")
	}
}

// --- AC-13: bounded-loop + audit contract ---

func TestMsBeadCycleSkill_BoundedLoopContractShipped(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-cycle")

	if !strings.Contains(content, "ACCEPT") {
		t.Error("expected the ACCEPT disposition documented")
	}
	if !strings.Contains(content, "DISAGREE") {
		t.Error("expected the DISAGREE/clarify disposition documented")
	}
	if !strings.Contains(content, "ordinal, verbatim reason, concrete answer, authoritative source span") {
		t.Error("expected the grounded reason-keyed clarification requirement (ordinal + verbatim reason + answer + source span)")
	}
	if !strings.Contains(content, "categorical and durable") {
		t.Error("expected the categorical/durable per-bead cap wording")
	}
	if !strings.Contains(content, "bead clarify") {
		t.Error("expected the bead clarify verb invocation documented")
	}
	if !strings.Contains(content, "--allow-not-ready") {
		t.Error("expected the clarify-vs---allow-not-ready distinction to name --allow-not-ready")
	}
	if !strings.Contains(content, "never interchangeable") {
		t.Error("expected the clarify-vs---allow-not-ready distinction to be stated explicitly")
	}
}

// --- ms-spec-autopilot: ACCEPTed-NOT-READY halt row ---

func TestMsSpecAutopilotSkill_AcceptedNotReadyHaltRow(t *testing.T) {
	content := readinessSkillFile(t, "ms-spec-autopilot")
	if !strings.Contains(content, "NOT READY") {
		t.Fatal("expected the ACCEPTed NOT READY halt row in the shipped ms-spec-autopilot SKILL.md")
	}
	if !strings.Contains(content, "do NOT proceed to the next bead") && !strings.Contains(content, "do not proceed to the next bead") {
		t.Error("expected the halt row to say not to proceed to the next bead")
	}
}

// --- Final-review r1 G1/G3-OVERRIDE-COVERAGE: stale-override coverage ---

// mutationProbeOverrideCoveragePresent isolates the override-coverage
// requirement (final-review r1 G1-STALE-OVERRIDE-COVERAGE /
// G3-OVERRIDE-COVERAGE / O1-1): the ingress must compare the FRESH
// ready-check's currently-failing signals against the marker's recorded
// `signals` set, proceed ONLY on full coverage, and STOP (re-gate) on
// any uncovered new failure.
func mutationProbeOverrideCoveragePresent(content string) bool {
	return strings.Contains(content, "ONLY if EVERY currently-failing signal is present in that recorded set") &&
		strings.Contains(content, "PRESENCE alone is NOT authority to dispatch") &&
		strings.Contains(content, "naming each uncovered signal")
}

// TestMsBeadImplSkill_OverrideCoverageCheckShipped pins that a stale
// override marker cannot bypass NEW failing signals: dispatch on the
// FAIL-with-marker branch requires the marker's recorded signal set to
// COVER every currently-failing signal from the fresh ready-check, and
// an uncovered failure re-blocks (NOT READY routing).
func TestMsBeadImplSkill_OverrideCoverageCheckShipped(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeOverrideCoveragePresent(content) {
		t.Error("expected the shipped ms-bead-impl ingress to require override-marker COVERAGE of every currently-failing signal (stale marker must not bypass new failures)")
	}
	if !strings.Contains(content, "STOP and treat this exactly like the FAIL-no-override branch") {
		t.Error("expected the uncovered-signal outcome to re-gate via the FAIL-no-override branch (NOT READY routing)")
	}
}

// TestMsBeadImplSkill_OverrideCoverageMutationProbe proves the coverage
// pin is not vacuous: stripping the coverage comparison out of a copy of
// the shipped content turns the SAME predicate red.
func TestMsBeadImplSkill_OverrideCoverageMutationProbe(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeOverrideCoveragePresent(content) {
		t.Fatal("precondition failed: the shipped content must carry the override-coverage rule")
	}
	// Simulate the pre-fix regression: proceed on marker PRESENCE alone.
	mutated := strings.Replace(content,
		"ONLY if EVERY currently-failing signal is present in that recorded set", "when the marker is present", 1)
	if mutationProbeOverrideCoveragePresent(mutated) {
		t.Fatal("mutation probe failed: dropping the coverage comparison should turn the pin red")
	}
}

// --- Final-review r1 G2-R8-01: ALL cited reasons rendered on re-dispatch ---

// mutationProbeAllReasonsRenderedPresent isolates the render-ALL-reasons
// requirement (final-review r1 G2-R8-01): the re-dispatch injection
// iterates the record's `report` array (every originally-cited reason,
// never a subset), and a report ordinal with no paired clarification is
// still rendered — `clarification: (none recorded)` — so the fresh
// Phase 0 re-reports NOT READY for the unaddressed reason.
func mutationProbeAllReasonsRenderedPresent(content string) bool {
	return strings.Contains(content, "EVERY originally-cited reason is rendered, never a subset") &&
		strings.Contains(content, "clarification: (none recorded)")
}

// TestMsBeadImplSkill_AllCitedReasonsRenderedShipped pins the G2-R8-01
// fix: the anti-browbeat rule cannot be evaded by clarifying only some
// reasons, because every cited reason — clarified or not — reaches the
// re-dispatched Phase 0, and an unaddressed one re-reports NOT READY.
func TestMsBeadImplSkill_AllCitedReasonsRenderedShipped(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeAllReasonsRenderedPresent(content) {
		t.Error("expected the shipped ms-bead-impl re-dispatch injection to render ALL originally-cited reasons from the record's report array, including unclarified ones as 'clarification: (none recorded)'")
	}
	if !strings.Contains(content, "re-reports NOT READY for it") {
		t.Error("expected the unaddressed-reason outcome (Phase 0 re-reports NOT READY) stated at the injection site")
	}
	if !strings.Contains(content, "fail closed to the NOT-READY routing") {
		t.Error("expected the malformed-record ingress to fail CLOSED (no dispatch on a partial injection)")
	}
}

// --- Final-review r2 G2-R2-R8-REPORT-OMISSION: independent re-derivation ---

// mutationProbeIndependentRederivationPresent isolates the r2 fix: the
// R8 reason-pairing bijection in `bead clarify` is closed only over the
// operator-supplied record's `report` array, so a reason omitted from
// BOTH arrays passes validation — the record can never prove itself
// complete. The real anti-browbeat backstop is therefore the
// re-dispatched Phase 0 RE-DERIVING its own SR-1..SR-5 reasons
// independently: the injected clarifications are supplementary evidence,
// NOT a closed checklist of the only reasons to consider, and a
// clarification only helps if it resolves a reason Phase 0 independently
// still finds. Both the Phase 0 rule and the ingress-side "never the
// complete cited-set" framing must be present.
func mutationProbeIndependentRederivationPresent(content string) bool {
	return strings.Contains(content, "Independent re-derivation rule") &&
		strings.Contains(content, "NOT a closed checklist of the only reasons to consider") &&
		strings.Contains(content, "a reason YOU independently still find") &&
		strings.Contains(content, "complete cited-set")
}

// TestMsBeadImplSkill_IndependentRederivationShipped pins the
// G2-R2-R8-REPORT-OMISSION fix: omitting an originally-cited reason from
// the clarification record cannot silently drop it, because the
// re-dispatched Phase 0 is instructed to re-derive its own reasons from
// a fresh review and re-report NOT READY for any genuinely-unresolved
// concern — even one absent from the injected record.
func TestMsBeadImplSkill_IndependentRederivationShipped(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeIndependentRederivationPresent(content) {
		t.Error("expected the shipped ms-bead-impl Phase 0 block to carry the independent re-derivation rule (clarifications are supplementary evidence, never a closed checklist / the complete cited-set)")
	}
	if !strings.Contains(content, "including a concern that appears nowhere in the injected record") {
		t.Error("expected the re-derivation rule to cover concerns absent from the injected record entirely")
	}
}

// TestMsBeadImplSkill_IndependentRederivationMutationProbe proves the pin
// is not vacuous: reverting to the closed-checklist framing (or dropping
// the rule paragraph) turns the SAME predicate red.
func TestMsBeadImplSkill_IndependentRederivationMutationProbe(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeIndependentRederivationPresent(content) {
		t.Fatal("precondition failed: the shipped content must carry the independent re-derivation rule")
	}
	// Simulate the pre-fix regression: Phase 0 framed as "address the
	// reasons in the clarification record" (a closed checklist).
	mutated := strings.Replace(content,
		"NOT a closed checklist of the only reasons to consider",
		"the checklist of reasons to address", 1)
	if mutationProbeIndependentRederivationPresent(mutated) {
		t.Fatal("mutation probe failed: reverting to the closed-checklist framing should turn the pin red")
	}
	// Dropping the whole rule paragraph must also go red.
	start := strings.Index(content, "**Independent re-derivation rule")
	if start < 0 {
		t.Fatal("could not locate the independent re-derivation paragraph to mutate")
	}
	end := strings.Index(content[start:], "\n\n")
	if end < 0 {
		end = len(content) - start
	}
	mutated2 := content[:start] + content[start+end:]
	if mutationProbeIndependentRederivationPresent(mutated2) {
		t.Fatal("mutation probe failed: stripping the re-derivation paragraph should turn the pin red")
	}
}

// TestMsBeadImplSkill_AllReasonsRenderedMutationProbe proves the pin is
// not vacuous: dropping the unaddressed-reason rendering turns the SAME
// predicate red.
func TestMsBeadImplSkill_AllReasonsRenderedMutationProbe(t *testing.T) {
	content := readinessSkillFile(t, "ms-bead-impl")
	if !mutationProbeAllReasonsRenderedPresent(content) {
		t.Fatal("precondition failed: the shipped content must carry the render-all-reasons rule")
	}
	mutated := strings.ReplaceAll(content, "clarification: (none recorded)", "")
	if mutationProbeAllReasonsRenderedPresent(mutated) {
		t.Fatal("mutation probe failed: dropping the unaddressed-reason rendering should turn the pin red")
	}
	mutated2 := strings.Replace(content,
		"EVERY originally-cited reason is rendered, never a subset", "the clarified reasons are rendered", 1)
	if mutationProbeAllReasonsRenderedPresent(mutated2) {
		t.Fatal("mutation probe failed: dropping the report-array iteration rule should turn the pin red")
	}
}
