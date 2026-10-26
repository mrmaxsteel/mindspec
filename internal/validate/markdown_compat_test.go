package validate

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func frozenParseSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentHeading string
	var currentBody []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if currentHeading != "" {
				sections[currentHeading] = strings.Join(currentBody, "\n")
			}
			currentHeading = strings.TrimPrefix(line, "## ")
			currentHeading = strings.TrimSpace(currentHeading)
			currentBody = nil
		} else if currentHeading != "" {
			currentBody = append(currentBody, line)
		}
	}
	// Save last section
	if currentHeading != "" {
		sections[currentHeading] = strings.Join(currentBody, "\n")
	}

	return sections
}

func frozenCheckScopeSubsections(r *Result, sections map[string]string) {
	scope, exists := sections["Scope"]
	if !exists {
		return // already flagged by required section check
	}

	if !strings.Contains(scope, "### In Scope") {
		r.AddError("scope-in-missing", "Scope section missing '### In Scope' subsection")
	}
	if !strings.Contains(scope, "### Out of Scope") {
		r.AddError("scope-out-missing", "Scope section missing '### Out of Scope' subsection")
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

func TestValidateSpecListFenceBlank(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"080-epic-lifecycle-metadata", "084-mindspec-otel-only", "088-context-pack-budgeter"} {
		t.Run(id, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, ".mindspec/specs", id, "spec.md"))
			if err != nil {
				t.Fatal(err)
			}
			sections := parseSections(string(data))
			for _, name := range []string{"Scope", "Acceptance Criteria", "Approval"} {
				if strings.TrimSpace(sections[name]) == "" {
					t.Errorf("lost section %s", name)
				}
			}
			result := ValidateSpec(root, id)
			for _, issue := range result.Issues {
				if issue.Name == "section-missing" {
					t.Errorf("false missing section: %+v", issue)
				}
			}
		})
	}
}

func TestMarkdownCorpusCompatibility(t *testing.T) {
	counts := map[string]int{}
	for name, text := range trackedMarkdown(t) {
		counts["files"]++
		got, want := parseSections(text), frozenParseSections(text)
		if !reflect.DeepEqual(got, want) {
			if name != ".mindspec/core/USAGE.md" && name != ".mindspec/specs/013-proof-runner/spec.md" {
				t.Errorf("unexpected section difference: %s", name)
			}
			t.Logf("parseSections DIFF %s: keep example headings inside their fence", name)
			counts["parseSections"]++
		}
		a, b := &Result{}, &Result{}
		checkScopeSubsections(a, map[string]string{"Scope": text})
		frozenCheckScopeSubsections(b, map[string]string{"Scope": text})
		if !reflect.DeepEqual(a.Issues, b.Issues) {
			t.Errorf("scope subheadings differ: %s", name)
			counts["checkScopeSubsections"]++
		}
	}
	t.Logf("corpus=%v", counts)
}
