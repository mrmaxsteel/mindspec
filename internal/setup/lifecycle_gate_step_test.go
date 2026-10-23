package setup

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var sixReviewersRE = regexp.MustCompile(`(?i)6[- ]reviewers?`)

func documentGuidanceError(content, gate, verb string) error {
	command := "`mindspec panel create <slug> --spec <id> --gate " + gate + " --target spec/<id>`"
	panel := strings.Index(content, command)
	approval := strings.Index(content, "`mindspec "+verb+" approve <id>`")
	if panel < 0 || approval < panel {
		return fmt.Errorf("panel with spec target must precede approval")
	}
	for _, required := range []string{
		"do NOT run `mindspec " + verb + " approve` until `/ms-panel-tally` returns PASS",
		"`mindspec " + verb + " approve` refuses without a passing document panel",
		"a gate, a schema, a trust boundary or a merge path",
		"reviewers from more than one family",
		"mindspec config resolve --gate " + gate,
	} {
		normalized := strings.Join(strings.Fields(content), " ")
		if !strings.Contains(normalized, required) {
			return fmt.Errorf("missing %q", required)
		}
	}
	if strings.Contains(content, "guidance, not a mechanized") || strings.Contains(content, "separate, tracked") {
		return fmt.Errorf("stale enforcement guidance")
	}
	return nil
}

func TestLifecycleGateStep_DocumentPanelBeforeApproval(t *testing.T) {
	for _, tc := range []struct{ name, gate, verb string }{
		{"ms-spec-approve", "spec_approve", "spec"}, {"ms-plan-approve", "plan_approve", "plan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := lifecycleSkillFiles()[tc.name]
			if err := documentGuidanceError(body, tc.gate, tc.verb); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []struct{ name, from, to string }{
				{"wrong_target", "--target spec/<id>", "--target <ref>"},
				{"wrong_gate", "--gate " + tc.gate, "--gate adhoc"},
				{"wrong_verb", "do NOT run\n   `mindspec " + tc.verb + " approve`", "do NOT run `mindspec other approve`"},
				{"no_refusal", "refuses without a passing document panel", "accepts without a panel"},
				{"missing_change_kind", "a trust boundary", "a boundary"},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					mutated := strings.Replace(body, mutation.from, mutation.to, 1)
					if mutated == body {
						t.Fatal("mutation did not fire")
					}
					if err := documentGuidanceError(mutated, tc.gate, tc.verb); err == nil {
						t.Fatal("wrong guidance passed")
					}
				})
			}
		})
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
