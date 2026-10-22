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
		"spec directory of the spec branch that the caller names",
		"On the close path, this is the bead's own spec", "review glob is limited to that spec",
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
	data, err = os.ReadFile("../../.mindspec/domains/workflow/architecture.md")
	if err != nil {
		t.Fatal(err)
	}
	_, ownership, ok := strings.Cut(string(data), "### `FindLandedMerge`: two-source ownership identity (R5)")
	if !ok {
		t.Fatal("missing finder ownership section")
	}
	ownership, _, _ = strings.Cut(ownership, "\n### ")
	ownership = strings.Join(strings.Fields(ownership), " ")
	for _, phrase := range []string{
		"Surviving tips and bindings require exact equality",
		"An exact member confirms. A reviewed ancestor also confirms",
		"its diff to the second parent is empty or contains only allowed tool artifacts",
		"See [Reviewed-tree confirmation](#reviewed-tree-confirmation)",
		"A set with neither match confirms nothing and never vetoes another leg",
		"With no corroboration for an owned candidate, the function refuses",
	} {
		if !strings.Contains(ownership, phrase) {
			t.Errorf("finder ownership section missing %q", phrase)
		}
	}
}
