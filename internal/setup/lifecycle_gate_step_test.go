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

// lifecycleGateCase drives both the positive pins and the mutation probe
// below from ONE table, keyed by literal name -> expected gate flag and
// expected approve VERB, so the wrong-verb and enforcement-note-removal
// mutations exercise the exact same production predicates the positive
// tests use (no separate, weaker probe-only logic).
type lifecycleGateCase struct {
	name      string // lifecycleSkillFiles() map key
	gate      string // --gate <gate> value
	verb      string // the CORRECT `mindspec <verb...>` approve invocation
	otherVerb string // the OTHER gate's approve invocation (the wrong-verb mutation)
}

var lifecycleGateCases = []lifecycleGateCase{
	{
		name:      "ms-spec-approve",
		gate:      "spec_approve",
		verb:      "mindspec spec approve",
		otherVerb: "mindspec plan approve",
	},
	{
		name:      "ms-plan-approve",
		gate:      "plan_approve",
		verb:      "mindspec plan approve",
		otherVerb: "mindspec spec approve",
	},
}

// notBeforeTallyVerbRE ties "do NOT run `<verb>`" to "until
// `/ms-panel-tally` returns Allow" as a BOUNDED single fragment (not
// three independent strings.Contains calls) — a literal that names the
// WRONG approve verb right after "do NOT run" (e.g. ms-spec-approve
// telling the reader not to run `mindspec plan approve`) must fail this
// check for the gate's correct verb, even though the literal still
// contains "/ms-panel-tally" and "returns Allow" somewhere.
func notBeforeTallyVerbRE(verb string) *regexp.Regexp {
	return regexp.MustCompile(
		`do NOT run\s{1,10}` + "`" + regexp.QuoteMeta(verb) + "`" +
			`\s{1,10}until\s{1,10}` + "`" + `/ms-panel-tally` + "`" +
			`\s{1,10}returns Allow`,
	)
}

func notBeforeTallyVerbPresent(content, verb string) bool {
	return notBeforeTallyVerbRE(verb).MatchString(content)
}

// enforcementNoteRE pins the portable (bead-ID-free — see
// TestLifecycleGateStep_NoBeadIDLeak) enforcement-scope note that must
// appear in BOTH the ms-spec-approve and ms-plan-approve literals: "this
// step is guidance, not a mechanized preflight; binary enforcement of
// these gates is a separate, tracked enhancement." \s+ bridges the
// line-wrap between "tracked" and "enhancement" in the shipped raw
// string literal.
var enforcementNoteRE = regexp.MustCompile(`binary enforcement of these gates is a separate, tracked\s+enhancement`)

func enforcementNotePresent(content string) bool {
	return enforcementNoteRE.MatchString(content)
}

// TestLifecycleGateStep_NamesGateVerbAndEnforcementNote is the
// table-driven positive pin for AC-6's ms-spec-approve/ms-plan-approve
// literal half: each literal names its own `--gate <gate>` flag, binds
// "do NOT run <the CORRECT approve verb>" to the
// "until `/ms-panel-tally` returns Allow" instruction (bounded, in the
// SAME literal — a wrong-verb swap REDs this), cites ADR-0044, and
// carries the portable "binary enforcement ... separate, tracked
// enhancement" note.
func TestLifecycleGateStep_NamesGateVerbAndEnforcementNote(t *testing.T) {
	files := lifecycleSkillFiles()
	for _, tc := range lifecycleGateCases {
		t.Run(tc.name, func(t *testing.T) {
			content, ok := files[tc.name]
			if !ok {
				t.Fatalf("lifecycleSkillFiles() has no %q entry", tc.name)
			}
			if !strings.Contains(content, "--gate "+tc.gate) {
				t.Errorf("%s literal missing the '--gate %s' panel-invocation fragment", tc.name, tc.gate)
			}
			if !notBeforeTallyVerbPresent(content, tc.verb) {
				t.Errorf("%s literal missing the bounded \"do NOT run `%s` ... until `/ms-panel-tally` returns Allow\" fragment", tc.name, tc.verb)
			}
			if !strings.Contains(content, "ADR-0044") {
				t.Errorf("%s literal missing the ADR-0044 citation", tc.name)
			}
			if !enforcementNotePresent(content) {
				t.Errorf("%s literal missing the portable \"binary enforcement of these gates is a separate, tracked enhancement\" note", tc.name)
			}
		})
	}
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

// TestLifecycleGateStep_MutationProbe proves the positive pins above are
// not vacuous by applying the SAME production predicates
// (notBeforeTallyVerbPresent, enforcementNotePresent) to mutated copies
// of the shipped literals — never a standalone strings.Replace/Contains
// check that bypasses those predicates.
func TestLifecycleGateStep_MutationProbe(t *testing.T) {
	files := lifecycleSkillFiles()

	t.Run("wrong_verb_reds_notBeforeTallyVerbPresent", func(t *testing.T) {
		for _, tc := range lifecycleGateCases {
			content, ok := files[tc.name]
			if !ok {
				t.Fatalf("lifecycleSkillFiles() has no %q entry", tc.name)
			}
			if !notBeforeTallyVerbPresent(content, tc.verb) {
				t.Fatalf("precondition failed: %s literal must currently carry the correct-verb fragment for %q", tc.name, tc.verb)
			}

			wrongVerbLiteral := "`" + tc.otherVerb + "`"
			correctVerbLiteral := "`" + tc.verb + "`"
			if !strings.Contains(content, correctVerbLiteral) {
				t.Fatalf("fixture assumption broken: %q not found (backtick-delimited) in %s literal", correctVerbLiteral, tc.name)
			}
			mutated := strings.Replace(content, correctVerbLiteral, wrongVerbLiteral, 1)
			if mutated == content {
				t.Fatalf("fixture assumption broken: swap of %q -> %q had no effect on %s literal", correctVerbLiteral, wrongVerbLiteral, tc.name)
			}

			// Applying the SAME production predicate (for the gate's
			// correct verb) to the mutated content must now fail: the
			// "do NOT run" instruction no longer names this gate's verb.
			if notBeforeTallyVerbPresent(mutated, tc.verb) {
				t.Errorf("mutation probe failed: swapping the approve verb to %q in %s should turn notBeforeTallyVerbPresent(content, %q) false", tc.otherVerb, tc.name, tc.verb)
			}
		}
	})

	t.Run("enforcement_note_removal_reds", func(t *testing.T) {
		for _, name := range []string{"ms-spec-approve", "ms-plan-approve"} {
			content, ok := files[name]
			if !ok {
				t.Fatalf("lifecycleSkillFiles() has no %q entry", name)
			}
			if !enforcementNotePresent(content) {
				t.Fatalf("precondition failed: %s literal must currently carry the enforcement note", name)
			}

			mutated := enforcementNoteRE.ReplaceAllString(content, "")
			if mutated == content {
				t.Fatalf("fixture assumption broken: could not locate the enforcement note to remove from %s literal", name)
			}

			// Applying the SAME production predicate to the mutated
			// content must now fail.
			if enforcementNotePresent(mutated) {
				t.Errorf("mutation probe failed: deleting the enforcement note from %s literal should turn enforcementNotePresent false", name)
			}
		}
	})

	t.Run("gate_flag_removal_reds", func(t *testing.T) {
		for _, tc := range lifecycleGateCases {
			content, ok := files[tc.name]
			if !ok {
				t.Fatalf("lifecycleSkillFiles() has no %q entry", tc.name)
			}
			flag := "--gate " + tc.gate
			if !strings.Contains(content, flag) {
				t.Fatalf("precondition failed: the shipped %s literal must carry the %q fragment", tc.name, flag)
			}
			mutated := strings.Replace(content, flag, "", 1)
			if strings.Contains(mutated, flag) {
				t.Errorf("mutation probe failed: stripping %q from %s should remove it", flag, tc.name)
			}
		}
	})
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
