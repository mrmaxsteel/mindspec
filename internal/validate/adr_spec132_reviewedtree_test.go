package validate

import (
	"os"
	"strings"
	"testing"
)

func TestDocs_Spec132Amendments_ReviewedTree(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0041-gate-before-mutate.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	_, amendment, ok := strings.Cut(text, "## Amendment (Spec 132)")
	if !ok {
		t.Fatal("missing amendment")
	}
	_, section, ok := strings.Cut(amendment, "### Reviewed-tree confirmation")
	if !ok {
		t.Fatal("missing reviewed-tree subsection")
	}
	section, _, _ = strings.Cut(section, "\n### ")
	section = strings.Join(strings.Fields(section), " ")
	for _, phrase := range []string{
		"changes Amendment (Spec 129), part (ii)", "panel registrations and ledger-backed rebinds",
		"`mindspec_landed_reviewed_sha` key counts only when it is in R",
		"unregistered key is unverified and confirms nothing", "subject-named first-parent candidate",
		"D is in R (`exact`)", "ancestor of D", "git diff --no-renames --name-only -z r D",
		"no paths or only tool-artifact paths (`reviewed-tree`)", "panel.ToolArtifactPaths",
		"spec directory of the bead's own epic", "review glob is limited to that spec",
		"Another spec's artifacts do not qualify", "both paths must qualify",
		"failed review read confirms nothing", "confirms and never vetoes",
		"tip or binding contradiction still refuses", "revert check still runs after confirmation",
		"provenance string names `exact` or `reviewed-tree`",
		"reachable-fallback path does not evaluate this datum",
	} {
		if !strings.Contains(section, phrase) {
			t.Errorf("reviewed-tree subsection missing %q", phrase)
		}
	}
	_, prior, ok := strings.Cut(text, "### (ii) Non-corroborating panel datum and binding veto")
	if !ok {
		t.Fatal("missing prior panel subsection")
	}
	prior, _, _ = strings.Cut(prior, "\n### ")
	prior = strings.Join(strings.Fields(prior), " ")
	for _, clause := range []string{
		"The set alone never confirms an ancestor or a fallback.",
		"the panel leg is normally non-corroborating.",
	} {
		if !strings.Contains(prior, clause+" See [Amendment (Spec 132)](#amendment-spec-132)") {
			t.Errorf("missing adjacent back-pointer for %q", clause)
		}
	}
}
