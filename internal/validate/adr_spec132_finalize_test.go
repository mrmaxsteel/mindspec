package validate

import (
	"os"
	"strings"
	"testing"
)

func TestDocs_Spec132Amendments_Finalize(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0041-gate-before-mutate.md")
	if err != nil {
		t.Fatal(err)
	}
	_, amendment, ok := strings.Cut(string(data), "## Amendment (Spec 132)")
	if !ok {
		t.Fatal("missing amendment")
	}
	_, section, ok := strings.Cut(amendment, "### Implementation approval recovery")
	if !ok {
		t.Fatal("missing finalize subsection")
	}
	section, _, _ = strings.Cut(section, "\n### ")
	text := strings.Join(strings.Fields(section), " ")
	for _, phrase := range []string{
		"reads the epic status after a failure", "epic closed", "epic still open",
		"failed finalize stage", "inner recovery lines stay unchanged and come last",
		"only when the inner error has no recovery line", "mindspec_finalize_tip",
		"after the export commit and all bead auto-merges", "Every run overwrites",
		"pre-finalize routing checkpoint stays separate", "one close", "same carrier branch",
		"--force-with-lease", "git ls-remote", "The executor opens no PR", "finalize-PR automation (§4)", "adopts an open PR for the same carrier head", "a retry opens no second PR while the first is open or merged", "every pre-close gate again",
		"already landed", "epic is closed", "stored phase is done", "every lifecycle child's landed binding",
		"temporary ref", "remote main", "fetch failure", "squash or rebase merge",
		"destruction guard again", "tips are ancestors of the target",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("finalize amendment missing %q", phrase)
		}
	}
}
