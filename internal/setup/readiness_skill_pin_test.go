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
