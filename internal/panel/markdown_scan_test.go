package panel

import (
	"strings"
	"testing"
)

func TestHeadingBlocks(t *testing.T) {
	for _, hidden := range []string{
		"    ## Hidden\n",
		"- item\n  ## Hidden\n",
		"> ## Hidden\n",
		"- item\n    ```\n  ## Hidden\n    ```\n",
		"> - item\n>   ~~~\n>   ## Hidden\n>   ~~~\n",
		"<!--\n## Hidden\n-->\n",
		"<script>\n## Hidden\n</script>\n",
		"<script>\n\n## Hidden\n</script>\n",
		"<?\n\n## Hidden\n?>\n",
		"---\n# metadata\n## Hidden\n---\n",
	} {
		t.Run(hidden, func(t *testing.T) {
			text := hidden + "\n## Visible ###\nbody\n"
			var headings []MarkdownLine
			for _, line := range MarkdownLines(text) {
				if line.Level > 0 {
					headings = append(headings, line)
				}
			}
			if len(headings) != 1 || headings[0].Level != 2 || headings[0].Heading != "Visible" {
				t.Fatalf("wrong headings: %+v", headings)
			}
			line := headings[0]
			if text[line.Offset:line.End] != "## Visible ###\n" {
				t.Fatalf("wrong source span: %+v", line)
			}
		})
	}
}

func TestFenceContainerCannotChangeInsideCode(t *testing.T) {
	for _, text := range []string{
		"## Visible\n```\n> ```\nhidden\n```\n",
		"## Visible\n- ```\n  - ```\n  hidden\n  ```\n",
		"## Visible\n```\n    ```\nhidden\n```\n",
	} {
		for _, line := range MarkdownLines(text + "\ncontrol\n") {
			if strings.Contains(line.Text, "hidden") {
				t.Fatalf("code became visible: %+v", line)
			}
		}
		lines := MarkdownLines(text + "\ncontrol\n")
		if len(lines) == 0 || lines[len(lines)-1].Text != "control" {
			t.Fatalf("control hidden: %+v", lines)
		}
	}
}

func TestOrderedSiblingsKeepSection(t *testing.T) {
	text := "## Section\n1. first\n2. second\n3. third\n\n## Next\ncontrol\n"
	boundaries := 0
	second := false
	for _, line := range MarkdownLines(text) {
		if line.Level > 0 {
			boundaries++
		}
		second = second || line.Text == "second"
	}
	if boundaries != 2 || !second {
		t.Fatalf("ordered siblings changed the section: %+v", MarkdownLines(text))
	}
}
