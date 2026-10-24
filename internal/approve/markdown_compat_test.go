package approve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func frozenUpsertSpecApprovalSection(content, approvedBy string, now time.Time) string {
	lines := strings.Split(content, "\n")

	approvalStart := -1
	approvalEnd := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(line, "## Approval") {
			approvalStart = i
			continue
		}
		if approvalStart >= 0 && strings.HasPrefix(line, "## ") {
			approvalEnd = i
			break
		}
	}

	newApproval := []string{
		"## Approval",
		"",
		"- **Status**: APPROVED",
		fmt.Sprintf("- **Approved By**: %s", approvedBy),
		fmt.Sprintf("- **Approval Date**: %s", now.Format("2006-01-02")),
		"- **Notes**: Approved via mindspec approve spec",
	}

	if approvalStart == -1 {
		base := strings.TrimRight(content, "\n")
		if base == "" {
			return strings.Join(newApproval, "\n") + "\n"
		}
		return base + "\n\n" + strings.Join(newApproval, "\n") + "\n"
	}

	var result []string
	result = append(result, lines[:approvalStart]...)
	result = append(result, newApproval...)
	result = append(result, lines[approvalEnd:]...)
	return strings.Join(result, "\n")
}

func frozenExtractBeadSectionContents(content string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentHeading string
	var currentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## Bead ") {
			// Save previous section
			if currentHeading != "" {
				result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			}
			currentHeading = strings.TrimPrefix(line, "## ")
			currentLines = nil
			continue
		}
		// A non-bead ## heading ends the current bead section
		if strings.HasPrefix(line, "## ") && currentHeading != "" {
			result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			currentHeading = ""
			currentLines = nil
			continue
		}
		if currentHeading != "" {
			currentLines = append(currentLines, line)
		}
	}
	if currentHeading != "" {
		result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
	}

	return result
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

func TestBeadSectionInlineHTMLText(t *testing.T) {
	for name, prefix := range map[string]string{
		"heading":         "## Bead 1: handle `<id>` input\nbody1\n",
		"script span":     "Text uses `<script>` here.\n\n## Bead 1: plain\nbody1\n",
		"tag prefix span": "Use `<pre-delete-SHA>` in prose.\n\n## Bead 1: plain\nbody1\n",
		"comment span":    "Text uses `<!--` here.\n\n## Bead 1: plain\nbody1\n",
	} {
		t.Run(name, func(t *testing.T) {
			text := prefix + "\n## Bead 2: plain\nbody2\n"
			if got, want := extractBeadSectionContents(text), frozenExtractBeadSectionContents(text); !reflect.DeepEqual(got, want) {
				t.Fatalf("lost bead section: got %#v, want %#v", got, want)
			}
		})
	}
}

func TestBeadSectionTrackedPlans(t *testing.T) {
	plans := 0
	for name, text := range trackedMarkdown(t) {
		if !strings.HasPrefix(name, ".mindspec/specs/") || !strings.HasSuffix(name, "/plan.md") {
			continue
		}
		plans++
		got, want := extractBeadSectionContents(text), frozenExtractBeadSectionContents(text)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("bead sections differ: %s (got %d, want %d)", name, len(got), len(want))
		}
		if strings.Contains(name, "/101-") && len(got) != 4 {
			t.Errorf("tracked inline-heading control has %d sections, want 4", len(got))
		}
	}
	if plans == 0 {
		t.Fatal("tracked plan corpus is empty")
	}
	t.Logf("plans=%d", plans)
}

func TestMarkdownCorpusCompatibility(t *testing.T) {
	counts := map[string]int{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for name, text := range trackedMarkdown(t) {
		counts["files"]++
		if planResolutionsSection(text) != frozenPlanResolutionsSection(text) {
			if name != ".mindspec/specs/133-document-final-gates/plan.md" {
				t.Errorf("unexpected resolution section difference: %s", name)
			}
			t.Logf("planResolutionsSection DIFF %s: hide markup and link continuations", name)
			counts["planResolutionsSection"]++
		}
		if upsertSpecApprovalSection(text, "operator", now) != frozenUpsertSpecApprovalSection(text, "operator", now) {
			if name != ".mindspec/domains/workflow/interfaces.md" {
				t.Errorf("unexpected approval edit difference: %s", name)
			}
			t.Logf("upsertSpecApprovalSection DIFF %s: keep the Approval guidance section", name)
			counts["upsertSpecApprovalSection"]++
		}
		if !reflect.DeepEqual(extractBeadSectionContents(text), frozenExtractBeadSectionContents(text)) {
			if name != ".mindspec/core/USAGE.md" {
				t.Errorf("unexpected bead section difference: %s", name)
			}
			t.Logf("extractBeadSectionContents DIFF %s: ignore example headings inside a fence", name)
			counts["extractBeadSectionContents"]++
		}
	}
	t.Logf("corpus=%v", counts)
}

func frozenPlanResolutionsSection(content string) string {
	inSection, fenced := false, false
	var section []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
		}
		if !fenced {
			if trimmed == "## Resolutions" {
				inSection = true
				continue
			}
			if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
				inSection = false
			}
		}
		if inSection {
			section = append(section, line)
		}
	}
	return strings.Join(section, "\n")
}
