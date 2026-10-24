package approve

import (
	"strings"
	"testing"
	"time"
)

func TestApprovalHeadingBlocks(t *testing.T) {
	for _, text := range []string{
		"```\n## Approval\nkeep this code\n```\n",
		"- item\n    ```\n    ## Approval\n    keep this code\n    ```\n",
		"> ## Approval\n> keep this quote\n",
		"    ## Approval\n    keep this code\n",
		"<!--\n## Approval\nkeep this comment\n-->\n",
		"## Approval details\nkeep this section\n",
		"   ## Approval\nkeep this section\n",
		"## Approval ##\nkeep this section\n",
	} {
		updated := upsertSpecApprovalSection(text, "operator", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		if !strings.HasPrefix(updated, text) || !strings.Contains(updated[len(text):], "## Approval\n") {
			t.Fatalf("hidden heading changed the edit span: %q", updated)
		}
	}
}

func TestBeadContentHeadingBlocks(t *testing.T) {
	text := "## Bead 1: Work\nkeep\n\n- item\n    ```\n    ## Bead 2: Hidden\n    ```\n\n> ## Appendix\n\n    ## Appendix\n\n<!--\n## Appendix\n-->\n\n## Appendix\nend\n"
	sections := extractBeadSectionContents(text)
	if len(sections) != 1 || !strings.Contains(sections["Bead 1: Work"], "## Bead 2: Hidden") || !strings.Contains(sections["Bead 1: Work"], "-->") || strings.Contains(sections["Bead 1: Work"], "\nend") {
		t.Fatalf("wrong bead content: %#v", sections)
	}
}
