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

func TestMarkdownListFenceBlank(t *testing.T) {
	for name, text := range map[string]string{
		"list":    "- item\n  ```\n  code\n\n  more\n  ```\n\n## After\n",
		"ordered": "8. Emit:\n   ```\n   a\n\n   b\n   ```\n9. next\n\n## After\n",
		"nested":  "> - item\n>   ```\n>   a\n>\n>   b\n>   ```\n\n## After\n",
	} {
		t.Run(name, func(t *testing.T) {
			lines := MarkdownLines(text)
			if len(lines) == 0 || lines[len(lines)-1].Heading != "After" {
				t.Fatalf("lost heading after fence: %+v", lines)
			}
			for _, line := range lines {
				if line.Text == "more" || line.Text == "b" {
					t.Errorf("code became visible: %+v", line)
				}
			}
		})
	}
}

func TestMarkdownFenceEndsWithContainer(t *testing.T) {
	for _, text := range []string{"> ```\n> code\n## After\n", "> ```\n> code\n\n## After\n", "- ```\n  code\n## After\n"} {
		lines := MarkdownLines(text)
		if len(lines) == 0 || lines[len(lines)-1].Heading != "After" {
			t.Fatalf("container fence hid heading: %+v", lines)
		}
	}
}

func TestMarkdownEmptyListItems(t *testing.T) {
	for _, marker := range []string{"-", "- ", "*", "+", "1.", "1.   "} {
		lines := MarkdownLines("## Before\n" + marker + "\n\nbody\n\n## After\n")
		var headings []string
		for _, line := range lines {
			if line.Level > 0 {
				headings = append(headings, line.Heading)
			}
		}
		if strings.Join(headings, ",") != "Before,After" {
			t.Errorf("empty item %q lost headings: %+v", marker, lines)
		}
	}
}

func TestMarkdownInlineHTMLText(t *testing.T) {
	for _, text := range []string{"## Bead 1: handle `<id>` input\n", "Text uses `<script>` here.\n\n", "`<pre-delete-SHA>` is a code span.\n\n", "An input <id> is required.\n\n", "<https://example.test>\n\n", "Text uses `<!--` here.\n\n", "## Bead 1: handle `<!--` input\n"} {
		lines := MarkdownLines(text + "## After\n")
		if len(lines) == 0 || lines[len(lines)-1].Heading != "After" {
			t.Errorf("inline text hid heading: %+v", lines)
		}
		if len(lines) > 0 && strings.HasPrefix(text, "##") && lines[0].Heading != strings.TrimSuffix(strings.TrimPrefix(text, "## "), "\n") {
			t.Errorf("heading text lost: %+v", lines)
		}
	}
}

func TestMarkdownThematicBreakKeepsEarlierHeading(t *testing.T) {
	text := "## Before\n\n```\nexample\n```\n\n---\n\n## After\n"
	var headings []string
	for _, line := range MarkdownLines(text) {
		if line.Level > 0 {
			headings = append(headings, line.Heading)
		}
	}
	if strings.Join(headings, ",") != "Before,After" {
		t.Fatalf("thematic break removed a heading: %v", headings)
	}
}

func TestMarkdownHTMLBlockOpeners(t *testing.T) {
	for _, block := range []string{
		"<div>\n## Hidden\n</div>\n",
		"   <SCRIPT>\n\n## Hidden\n</SCRIPT>\n",
		"<?target\n\n## Hidden\n?>\n",
		"<!DOCTYPE\n## Hidden\n>\n",
		"<![CDATA[\n## Hidden\n]]>\n",
		"<custom attr='value'>\n## Hidden\n",
	} {
		var headings []string
		for _, line := range MarkdownLines(block + "\n## After\n") {
			if line.Level > 0 {
				headings = append(headings, line.Heading)
			}
		}
		if strings.Join(headings, ",") != "After" {
			t.Errorf("wrong headings for %q: %v", block, headings)
		}
	}
	for _, text := range []string{"text\n<custom>\n", "<not a tag\n", "<div/garbage>\n", "</custom attr>\n", "<pre-delete-SHA>\n\n"} {
		lines := MarkdownLines(text + "## After\n")
		if len(lines) == 0 || lines[len(lines)-1].Heading != "After" {
			t.Errorf("text hid heading: %+v", lines)
		}
	}
}

func TestMarkdownEmptyListItemHeading(t *testing.T) {
	for _, tc := range []struct{ marker, indent string }{{"-", "  "}, {"- ", "  "}, {"1.", "   "}} {
		t.Run(tc.marker, func(t *testing.T) {
			text := tc.marker + "\n" + tc.indent + "## Resolutions\n" + tc.indent + "plan-one\n\n## After\n"
			var headings []string
			for _, line := range MarkdownLines(text) {
				if line.Level > 0 {
					headings = append(headings, line.Heading)
				}
			}
			if strings.Join(headings, ",") != "After" {
				t.Fatalf("list-contained heading became top-level: %v", headings)
			}
		})
	}
}

func TestMarkdownLazyContinuationScope(t *testing.T) {
	text := "## Resolutions\n> Context.\nlazy text\nhidden-id\n\nvisible-id\n"
	var visible []string
	for _, line := range MarkdownLines(text) {
		visible = append(visible, line.Text)
		if line.Level > 0 && line.Heading != "Resolutions" {
			t.Errorf("lazy text ended the section: %+v", line)
		}
	}
	got := strings.Join(visible, "\n")
	if strings.Contains(got, "hidden-id") || !strings.Contains(got, "visible-id") {
		t.Fatalf("lazy paragraph scope changed: %q", got)
	}
}
