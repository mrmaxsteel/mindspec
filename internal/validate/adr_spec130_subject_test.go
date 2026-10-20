package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Subject(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	_, amendment, ok := strings.Cut(string(data), "## Amendment (Spec 130)")
	if !ok {
		t.Fatal("missing spec 130 amendment")
	}
	text := strings.Join(strings.Fields(amendment), " ")
	for _, phrase := range []string{
		"§4 — Subject-tree staleness (R3)", "`panel.Subject(gate, specDir)`", "`<specs>/<spec>/`", "`spec_approve`", "`plan_approve`", "`bead`", "`final_review`", "`adhoc`", "an empty gate",
		"`<specs>/*/reviews/**`", "`.mindspec/reviews/**`", "`review/**`", "`.beads/issues.jsonl`", "Other `reviews` directories remain", "`subject_digest`", "sha256 of the filtered recursive `ls-tree` listing",
		"Equal SHAs are fresh with no subject read", "one `GateIO.SubjectTree` call", "Equal digests are fresh; different digests are stale", "empty filtered listing is Block", "refuses create", "original stale message byte-for-byte", "`complete`, `panel verify`, `panel tally` and instruct panel state share this decision",
		"The recorded `gate` is no longer decision-inert. It selects the subject roots through `panel.Subject` and can change Allow/Block. This supersedes the Spec 112 amendment's decision-inert claim for staleness only. `ApproveThreshold()` stays independent of `gate`.",
		"Decision Details — subprocess cap (R3.3)", "three on a SHA mismatch", "rev-parse, subject-tree read, porcelain", "two-subprocess cap", "leaf performs no subprocess I/O",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("subject amendment missing %q", phrase)
		}
	}
}
