package setup

// lifecycle_gate_step_test.go — spec 126 Bead 2's OWN [CI] pin for the
// R4/AC-6 literal-half edits this bead lands in claude.go:
// lifecycleSkillFiles()'s ms-spec-approve/ms-plan-approve raw-string
// literals gain a pre-approve panel step (Step 1), and the CLAUDE.md
// skills-table template (claudeMDManagedBlock) loses its "6 reviewers"
// topology commitment (also Step 1, folded panel finding F1-3).
//
// Deliberately a SEPARATE file from Bead 1's review_discipline_test.go (a
// parallel W1 bead touching a disjoint set of SKILL.md files) so the two
// beads share zero files. Bead 3's later R1c table rows re-assert the same
// fragments as part of its own completeness sweep — that is redundant
// coverage, not this file's job: AC-6 OWNERSHIP (the lens-table half plus
// the R1c rows) stays with Bead 3; this file only pins the Step-1 literal
// edits this bead itself lands, so a revert of THIS bead's work REDs in
// THIS bead, not three beads downstream.
//
// Reverting any Step-1 literal edit in claude.go REDs one or more of the
// tests below.

import (
	"regexp"
	"strings"
	"testing"
)

// sixReviewersRE is this bead's own local copy of the case-insensitive,
// separator-tolerant "6 reviewers"/"6-reviewers" pattern (matches
// singular "reviewer" too). Bead 3's later AC-1 sweep guard defines the
// SAME pattern as its standing row over the full skill-byte union; this
// bead's copy pins only the ONE surface (claudeMDManagedBlock) this bead
// itself edits, and does not depend on Bead 3 having landed yet.
var sixReviewersRE = regexp.MustCompile(`(?i)6[- ]reviewers?`)

// TestLifecycleGateStep_SpecApproveNamesGateAndTallyAllow pins the
// ms-spec-approve literal half of AC-6: the shipped SKILL text names
// `--gate spec_approve`, instructs NOT running the approve verb before a
// `/ms-panel-tally` Allow, and cites ADR-0044.
func TestLifecycleGateStep_SpecApproveNamesGateAndTallyAllow(t *testing.T) {
	files := lifecycleSkillFiles()
	content, ok := files["ms-spec-approve"]
	if !ok {
		t.Fatal("lifecycleSkillFiles() has no \"ms-spec-approve\" entry")
	}

	if !strings.Contains(content, "--gate spec_approve") {
		t.Error("ms-spec-approve literal missing the '--gate spec_approve' panel-invocation fragment")
	}
	if !notBeforeTallyAllowPresent(content) {
		t.Error("ms-spec-approve literal missing the not-before-tally-Allow instruction")
	}
	if !strings.Contains(content, "ADR-0044") {
		t.Error("ms-spec-approve literal missing the ADR-0044 citation")
	}
}

// TestLifecycleGateStep_PlanApproveNamesGateAndTallyAllow is the
// ms-plan-approve mirror of the above.
func TestLifecycleGateStep_PlanApproveNamesGateAndTallyAllow(t *testing.T) {
	files := lifecycleSkillFiles()
	content, ok := files["ms-plan-approve"]
	if !ok {
		t.Fatal("lifecycleSkillFiles() has no \"ms-plan-approve\" entry")
	}

	if !strings.Contains(content, "--gate plan_approve") {
		t.Error("ms-plan-approve literal missing the '--gate plan_approve' panel-invocation fragment")
	}
	if !notBeforeTallyAllowPresent(content) {
		t.Error("ms-plan-approve literal missing the not-before-tally-Allow instruction")
	}
	if !strings.Contains(content, "ADR-0044") {
		t.Error("ms-plan-approve literal missing the ADR-0044 citation")
	}
}

// notBeforeTallyAllowPresent isolates the "do not run the approve verb
// until /ms-panel-tally returns Allow" instruction, so both positive tests
// and the mutation probe below share the identical predicate.
func notBeforeTallyAllowPresent(content string) bool {
	return strings.Contains(content, "do NOT run") &&
		strings.Contains(content, "/ms-panel-tally") &&
		strings.Contains(content, "returns Allow")
}

// TestLifecycleGateStep_NoBeadIDLeak is the MUST-NOT-CONTAIN half of AC-6:
// the tracked mechanization-enhancement bead ID mindspec-0pij lives in the
// plan, the spec, and ADR-0044 only — NEVER in the shipped literal (a
// bead-ID leak that no AC-10 scan class catches). The shipped text says
// only the portable "binary enforcement of these gates is a separate,
// tracked enhancement" phrasing.
func TestLifecycleGateStep_NoBeadIDLeak(t *testing.T) {
	files := lifecycleSkillFiles()
	for _, name := range []string{"ms-spec-approve", "ms-plan-approve"} {
		content, ok := files[name]
		if !ok {
			t.Fatalf("lifecycleSkillFiles() has no %q entry", name)
		}
		if strings.Contains(content, "mindspec-0pij") {
			t.Errorf("%s literal leaks the tracked bead ID mindspec-0pij — this pointer must stay in the plan/spec/ADR only, never in the shipped literal", name)
		}
	}
}

// TestLifecycleGateStep_MutationProbe proves the two positive pins above
// are not vacuous: stashing (simulated here by string-surgery on an
// in-memory copy) the Step-1 literal edit turns the SAME predicates red.
// The bead's own verification also runs the real `git stash` version of
// this demonstration; this probe pins the same fact at the Go level so it
// survives independently of any one manual verification run.
func TestLifecycleGateStep_MutationProbe(t *testing.T) {
	files := lifecycleSkillFiles()
	content, ok := files["ms-spec-approve"]
	if !ok {
		t.Fatal("lifecycleSkillFiles() has no \"ms-spec-approve\" entry")
	}
	if !strings.Contains(content, "--gate spec_approve") {
		t.Fatal("precondition failed: the shipped ms-spec-approve literal must carry the --gate spec_approve fragment")
	}

	mutated := strings.Replace(content, "--gate spec_approve", "", 1)
	if strings.Contains(mutated, "--gate spec_approve") {
		t.Fatal("mutation probe failed: stripping the fragment should remove it")
	}
}

// TestLifecycleGateStep_ClaudeMDTemplateRewordedTopologyNeutral pins the
// Step-1 folded finding (F1-3): the CLAUDE.md skills-table template
// (claudeMDManagedBlock, seeded to every consumer CLAUDE.md's Review panel
// table row) must NOT match the case-insensitive, separator-tolerant
// `6[- ]reviewers?` pattern — the same pattern Bead 3's later AC-1 sweep
// guard applies as its standing row. Restoring "launch 6 reviewers" (or
// "launch 6-reviewers") in claudeMDManagedBlock REDs this test.
func TestLifecycleGateStep_ClaudeMDTemplateRewordedTopologyNeutral(t *testing.T) {
	if sixReviewersRE.MatchString(claudeMDManagedBlock) {
		t.Errorf("claudeMDManagedBlock still matches the topology-committed '6[- ]reviewers?' pattern — the ms-panel-run table row must read topology-neutral (e.g. \"launch the configured reviewer panel\"):\n%s", claudeMDManagedBlock)
	}
	// Precondition-style sanity: the table row this spec touches is still
	// present (renamed/deleted rows are a different regression, out of
	// this test's scope), and it still credibly describes the same step.
	if !strings.Contains(claudeMDManagedBlock, "ms-panel-run") {
		t.Fatal("precondition failed: claudeMDManagedBlock no longer mentions ms-panel-run at all")
	}
}

// TestLifecycleGateStep_ClaudeMDTemplateMutationProbe proves the reword
// pin above is not vacuous: restoring the old "6 reviewers" phrasing in a
// copy of the template turns the SAME pattern red.
func TestLifecycleGateStep_ClaudeMDTemplateMutationProbe(t *testing.T) {
	if sixReviewersRE.MatchString(claudeMDManagedBlock) {
		t.Fatal("precondition failed: claudeMDManagedBlock must currently be reworded topology-neutral")
	}
	reverted := strings.Replace(claudeMDManagedBlock,
		"then launch the configured reviewer panel and collect verdicts",
		"then launch 6 reviewers and collect verdicts", 1)
	if reverted == claudeMDManagedBlock {
		t.Fatal("fixture assumption broken: could not locate the reworded phrase to revert")
	}
	if !sixReviewersRE.MatchString(reverted) {
		t.Fatal("mutation probe failed: reverting to '6 reviewers' should turn the pattern red")
	}
}
