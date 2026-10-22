package validate

import (
	"os"
	"strings"
	"testing"
)

func TestDocs_Spec132Amendments_Preflight(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0041-gate-before-mutate.md")
	if err != nil {
		t.Fatal(err)
	}
	_, amendment, ok := strings.Cut(string(data), "## Amendment (Spec 132)")
	if !ok {
		t.Fatal("missing amendment")
	}
	_, section, ok := strings.Cut(amendment, "### Implementation approval preflight")
	if !ok {
		t.Fatal("missing preflight subsection")
	}
	section, _, _ = strings.Cut(section, "\n### ")
	text := strings.Join(strings.Fields(section), " ")
	for _, phrase := range []string{
		"one error-returning has-remote decision", "before the spec-branch existence switch",
		"An error never selects a local merge", "same candidate helper",
		"every pending bead-to-spec merge", "direct path it also checks the spec-to-main merge",
		"before the placeholder ADR write, the stale-phase reconcile write, and the epic close",
		"git push --dry-run --no-verify", "same remote and refspec", "GIT_TERMINAL_PROMPT=0",
		"Only a failed probe refuses", "missing credential helper alone does not refuse",
		"only for an authentication error on an HTTP(S) remote",
		"receives the has-remote value as a parameter and does not probe again",
		"live guards before each merge", "current tips after artifact commits and bead auto-merges",
		"Both early and live guards must pass",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("preflight amendment missing %q", phrase)
		}
	}
}
