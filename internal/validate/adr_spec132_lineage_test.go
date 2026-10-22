package validate

import (
	"os"
	"strings"
	"testing"
)

func TestDocs_Spec132Amendments_Lineage(t *testing.T) {
	raw, err := os.ReadFile("../../.mindspec/adr/ADR-0023.md")
	if err != nil {
		t.Fatal(err)
	}
	_, text, ok := strings.Cut(string(raw), "#### Amendment (spec 132-readiness-followon)")
	if !ok {
		t.Fatal("missing lineage amendment")
	}
	text, _, _ = strings.Cut(text, "\n#### ")
	text = strings.Join(strings.Fields(text), " ")
	for _, phrase := range []string{"Only one `parent-child` dependency on an epic", "A `blocks` dependency is not lineage", "bead-title fallback is removed", "`spec_id` first, `spec_num` plus `spec_title` second", "`[SPEC <id>]` title third", "A lettered ID resolves to its own spec", "`%03d` derivation applies only to the second source", "including closed epics", "Doctor reports duplicate bindings and title-only beads"} {
		if !strings.Contains(text, phrase) {
			t.Errorf("missing amendment rule %q", phrase)
		}
	}
}
