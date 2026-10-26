package panel

import (
	"bytes"
	"strings"
	"testing"
)

func TestDocumentSubjectHeadingBlocks(t *testing.T) {
	approval := "## Approval\n\n- **Status**: DRAFT\n- **Approved By**: -\n- **Approval Date**: -\n- **Notes**: -\n"
	for _, text := range []string{
		"```\n" + approval + "```\n",
		"- item\n    ```\n    " + strings.ReplaceAll(strings.TrimSuffix(approval, "\n"), "\n", "\n    ") + "\n    ```\n",
		"> " + strings.ReplaceAll(approval, "\n", "\n> "),
		"    " + strings.ReplaceAll(approval, "\n", "\n    "),
		"<!--\n" + approval + "-->\n",
	} {
		if !bytes.Equal(stripApproval([]byte(text)), []byte(text)) {
			t.Fatalf("hidden approval became an own write: %q", text)
		}
	}
	if got := stripApproval([]byte(approval + "## Appendix\nkeep\n")); string(got) != "## Appendix\nkeep\n" {
		t.Fatalf("visible control did not strip: %q", got)
	}
}

func TestAcceptanceHeadingBlocks(t *testing.T) {
	for _, hidden := range []string{
		"- item\n    ```\n    ## Acceptance Criteria\n    hidden\n    ```\n",
		"> ## Acceptance Criteria\n> hidden\n",
		"    ## Acceptance Criteria\n    hidden\n",
		"<!--\n## Acceptance Criteria\nhidden\n-->\n",
	} {
		if got := AcceptanceCriteriaSection(hidden); got != "no acceptance criteria section found" {
			t.Fatalf("hidden heading counted: %q", got)
		}
		control := "## Acceptance Criteria\nvisible\n"
		if got := AcceptanceCriteriaSection(hidden + "\n" + control + "\n## Appendix\n"); got != control+"\n" {
			t.Fatalf("visible control missing: %q", got)
		}
	}
}
