package panel

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Frozen functions retain the released byte contract.
func frozenStripApproval(body []byte) []byte {
	text := string(body)
	start, end := -1, len(text)
	frozenScanMarkdownLines(text, func(offset int, line string) {
		if start < 0 && line == "## Approval" {
			start = offset
			return
		}
		if start >= 0 && offset > start && strings.HasPrefix(line, "## ") && end == len(text) {
			end = offset
		}
	})
	if start < 0 {
		return body
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(text[start:end], "\n")[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		matched := false
		for _, prefix := range []string{"- **Status**:", "- **Approved By**:", "- **Approval Date**:", "- **Notes**:"} {
			if strings.HasPrefix(line, prefix) && !seen[prefix] {
				seen[prefix] = true
				matched = true
				break
			}
		}
		if !matched {
			return body
		}
	}
	if len(seen) != 4 {
		return body
	}
	return []byte(text[:start] + text[end:])
}

func frozenScanMarkdownLines(s string, fn func(int, string)) {
	var fenceByte byte
	fenceLength := 0
	for offset := 0; offset < len(s); {
		end := strings.IndexByte(s[offset:], '\n')
		if end < 0 {
			end = len(s)
		} else {
			end += offset
		}
		line := strings.TrimSuffix(s[offset:end], "\r")
		trimmed := strings.TrimSpace(line)
		run := 0
		if len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			for run < len(trimmed) && trimmed[run] == trimmed[0] {
				run++
			}
		}
		if fenceLength == 0 {
			if run >= 3 {
				fenceByte, fenceLength = trimmed[0], run
			} else {
				fn(offset, line)
			}
		} else if run >= fenceLength && trimmed[0] == fenceByte && strings.TrimSpace(trimmed[run:]) == "" {
			fenceLength = 0
		}
		offset = end + 1
	}
}

func trackedMarkdown(t *testing.T) map[string]string {
	t.Helper()
	out, err := exec.Command("git", "-C", "../..", "ls-files", "-z", "--", ".mindspec/").Output()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, name := range strings.Split(string(out), "\x00") {
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(data)
	}
	if len(files) == 0 {
		t.Fatal("tracked Markdown corpus is empty")
	}
	return files
}

const compatApproval = "## Approval\n\n- **Status**: APPROVED\n- **Approved By**: u\n- **Approval Date**: d\n- **Notes**: n\n"

func TestApprovalDigestReleasedCompatibility(t *testing.T) {
	cases := map[string]string{"plain": "# T\n\n" + compatApproval}
	for _, bullet := range []string{"-", "- ", "*", "1."} {
		cases["empty item "+bullet] = "## Intro\n" + bullet + "\n\nbody\n\n" + compatApproval
	}
	for name, tail := range map[string]string{"rule": "\n---\n\ntail\n", "comment": "\n<!-- c -->\n\ntail\n", "heading": "\n# Heading\n\ntail\n", "next section": "\n## Next\n\ntail\n"} {
		cases[name] = "# T\n\n## A\n\nx\n\n" + compatApproval + tail
	}
	cases["inline tag"] = "An input <id> is required.\n\n" + compatApproval
	cases["unclosed comment"] = "<!--\n" + compatApproval
	cases["CRLF"] = strings.ReplaceAll(cases["plain"], "\n", "\r\n")
	cases["unicode blank"] = compatApproval + "\u00a0\n"
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if got, want := stripApproval([]byte(text)), frozenStripApproval([]byte(text)); !bytes.Equal(got, want) {
				t.Fatalf("digest input changed: got %q, want %q", got, want)
			}
		})
	}
	if bytes.Equal(frozenStripApproval([]byte(compatApproval)), []byte(compatApproval)) {
		t.Fatal("released control did not strip")
	}
}

func frozenAcceptanceCriteriaSection(spec string) string {
	start, end := -1, len(spec)
	frozenScanMarkdownLines(spec, func(offset int, line string) {
		if start < 0 {
			if strings.TrimSpace(line) == "## Acceptance Criteria" {
				start = offset
			}
			return
		}
		if end == len(spec) && (strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ")) {
			end = offset
		}
	})
	if start < 0 {
		return "no acceptance criteria section found"
	}
	return spec[start:end]
}

func TestMarkdownCorpusCompatibility(t *testing.T) {
	counts := map[string]int{}
	specFiles := 0
	for name, text := range trackedMarkdown(t) {
		counts["files"]++
		if strings.HasPrefix(name, ".mindspec/specs/") && strings.Count(name, "/") == 3 {
			specFiles++
		}
		if !bytes.Equal(stripApproval([]byte(text)), frozenStripApproval([]byte(text))) {
			t.Errorf("stripApproval differs: %s", name)
			counts["stripApproval"]++
		}
		if AcceptanceCriteriaSection(text) != frozenAcceptanceCriteriaSection(text) {
			t.Errorf("AcceptanceCriteriaSection differs: %s", name)
			counts["AcceptanceCriteriaSection"]++
		}
		type row struct {
			Offset int
			Text   string
		}
		var got, want []row
		scanMarkdownLines(text, func(offset int, text string) { got = append(got, row{offset, text}) })
		frozenScanMarkdownLines(text, func(offset int, text string) { want = append(want, row{offset, text}) })
		if !reflect.DeepEqual(got, want) {
			t.Errorf("brief scan differs: %s", name)
			counts["brief scan"]++
		}
	}
	if specFiles == 0 {
		t.Fatal("spec corpus is empty")
	}
	t.Logf("corpus=%v spec documents=%d", counts, specFiles)
}

func TestBriefMarkerReleasedCompatibility(t *testing.T) {
	for _, prefix := range []string{"-\n\n", "Text uses `<script>` and `<!--`.\n\n", "- item\n  ```\n  code\n\n  more\n  ```\n\n"} {
		text := prefix + briefHeaderOpen + "\nheader\n" + briefHeaderClose + "\n"
		for _, marker := range []string{briefHeaderOpen, briefHeaderClose} {
			var want []int
			frozenScanMarkdownLines(text, func(offset int, line string) {
				if line == marker {
					want = append(want, offset)
				}
			})
			if got := findMarkerLines(text, marker); len(want) != 1 || !reflect.DeepEqual(got, want) {
				t.Fatalf("marker scan changed: got %v, want %v", got, want)
			}
		}
	}
}

func TestAcceptanceConservativeBoundary(t *testing.T) {
	text := "## Acceptance Criteria\n\n- item\nlazy continuation\n\n## Appendix\nrest\n"
	if got, want := AcceptanceCriteriaSection(text), frozenAcceptanceCriteriaSection(text); got != want {
		t.Fatalf("conservative gate boundary changed the section: got %q, want %q", got, want)
	}
}
