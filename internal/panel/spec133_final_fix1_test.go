package panel

import (
	"bytes"
	"strings"
	"testing"
)

func TestS133Fix1_ApprovalMatcherParity(t *testing.T) {
	approval := "## Approval\n\n- **Status**: Draft\n- **Approved By**: -\n- **Approval Date**: -\n- **Notes**: -\n"
	for name, body := range map[string]string{
		"matching":   approval,
		"CRLF":       strings.ReplaceAll(approval, "\n", "\r\n"),
		"extra":      approval + "Reviewed text.\n",
		"duplicate":  approval + "- **Notes**: extra\n",
		"incomplete": "## Approval\n- **Status**: Draft\n",
		"fenced":     "```\n" + approval + "```\n",
		"appendix":   approval + "\n## Appendix\nKeep.\n",
	} {
		t.Run(name, func(t *testing.T) {
			start, end, line, matches := ApprovalSection([]byte(body))
			var want []byte
			if start >= 0 && matches {
				want = []byte(body[:start] + body[end:])
			} else {
				want = []byte(body)
			}
			if got := stripApproval([]byte(body)); !bytes.Equal(got, want) {
				t.Fatalf("digest and exported matcher differ: %q != %q", got, want)
			}
			if name == "extra" && line != "Reviewed text." {
				t.Fatalf("wrong first line: %q", line)
			}
			if name == "duplicate" && line != "- **Notes**: extra" {
				t.Fatalf("wrong duplicate line: %q", line)
			}
			if name == "incomplete" && (matches || line != "## Approval") {
				t.Fatalf("incomplete section accepted: %q", line)
			}
			if name == "matching" && !matches {
				t.Fatal("matching section refused")
			}
		})
	}
}
