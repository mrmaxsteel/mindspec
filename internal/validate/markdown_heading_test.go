package validate

import (
	"strings"
	"testing"
)

func TestSectionHeadingBlocks(t *testing.T) {
	text := "## Goal\nkeep\n\n- item\n    ```\n    ## Hidden\n    ```\n\n> ## Hidden\n\n    ## Hidden\n\n<!--\n## Hidden\n-->\n\n## Scope\ncontrol\n"
	sections := parseSections(text)
	if len(sections) != 2 || !strings.Contains(sections["Goal"], "## Hidden") || sections["Scope"] != "control\n" {
		t.Fatalf("wrong sections: %#v", sections)
	}
}

func TestScopeSubheadingBlocks(t *testing.T) {
	for _, hidden := range []string{
		"```\n### In Scope\n### Out of Scope\n```\n",
		"    ### In Scope\n    ### Out of Scope\n",
		"- item\n  ### In Scope\n  ### Out of Scope\n",
		"> ### In Scope\n> ### Out of Scope\n",
		"<!--\n### In Scope\n### Out of Scope\n-->\n",
	} {
		r := &Result{}
		checkScopeSubsections(r, map[string]string{"Scope": hidden})
		if len(r.Issues) != 2 || r.Issues[0].Name != "scope-in-missing" || r.Issues[1].Name != "scope-out-missing" {
			t.Fatalf("hidden subheadings counted: %+v", r.Issues)
		}
		r = &Result{}
		checkScopeSubsections(r, map[string]string{"Scope": hidden + "\n### In Scope\nwork\n\n### Out of Scope\nother work\n"})
		if r.HasFailures() {
			t.Fatalf("visible control failed: %+v", r.Issues)
		}
	}
}
