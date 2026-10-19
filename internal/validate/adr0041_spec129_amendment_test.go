package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADR0041_Spec129Amendment(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0041-gate-before-mutate.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	_, amendment, ok := strings.Cut(text, "## Amendment (Spec 129)")
	if !ok {
		t.Fatal("missing Spec 129 amendment")
	}
	// Whitespace is irrelevant; each pinned clause states an independent part
	// of the scope, evidence, read-back or residual contract.
	amendment = strings.Join(strings.Fields(amendment), " ")
	for _, phrase := range []string{
		"### (i) Reachable fallback and audited read-back", "### (ii) Non-corroborating panel datum and binding veto",
		"lifecycle.FindLandedMergeReachable", "lifecycle.ReattestLandedMerge", "zero owned two-parent candidates",
		"finder-only: no automatic acceptance", "stay first-parent only", "undirected-subject target gap",
		"MergedUnclosed", "complete", "stale_open", "orphan_hints", "plan approval and lattice adoption",
		"F1, F3, F4 and F5 are all git-derived", "multiple-fallback-candidates", "even with the same second parent",
		"surviving bead tip must equal", "CURRENT tip", "any git or metadata read error is a plain error",
		"F2 is retired", "Registrations are advisory output only", "(not determinable locally)", "not read by automatic read-back",
		"ONE `MergeMetadata`", "mindspec_landed_reattest_merge_sha", "mindspec_landed_reattest_second_parent", "mindspec_landed_reattest_scanned_branch",
		"off-first-parent fallback; F1, F3, F4, F5 passed; advisory registrations outside tip: <n>; panel roots scanned: <roots>",
		"op == \"mindspec reattest\"", "RFC-3339", "non-empty `actor`", "well-formed git object IDs", "validity-qualified no-op",
		"equals both the binding and the single scan-derived candidate", "stale audit that confirms nothing",
		"ErrLandedMergeContradiction", "ErrLandedMergeReverted", "q9ea attested-restore is excluded from fallback exits",
		"--allow-not-ready=MF-3 --evidence <path-to-justification>", "Metadata-forge residual and bounded new reach", "agent-writable metadata", "NEW REACH",
		"SET of all well-formed", "non-corroborating, never a contradiction or a veto", "reach the CLOSE paths", "never a close",
		"complete lands a commit no panel reviewed (OQ-3)", "Binding veto versus audited repair", "prior pair is recorded",
	} {
		if !strings.Contains(amendment, phrase) {
			t.Errorf("amendment missing contract phrase %q", phrase)
		}
	}
	if !strings.Contains(text, "governed by Amendment (Spec 129), part (ii)") {
		t.Error("datum (c) lacks amendment cross-reference")
	}
}
