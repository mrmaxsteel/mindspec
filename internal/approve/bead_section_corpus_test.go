package approve

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

func beadSectionCorpus() map[string]string {
	return map[string]string{
		"trailing spaces": "## Bead 1: a  \nbody1\n\n## Bead 2: b\nbody2\n",
		"CRLF":            "## Bead 1: a\r\nbody1\r\n\r\n## Bead 2: b\r\nbody2\r\n",
		"closing hashes":  "## Bead 1: a ##\nbody1\n\n## Bead 2: b ###\nbody2\n",
		"fenced example":  "## Bead 1: a\nbody1\n```md\n## Bead 9: example\nexample body\n```\n\n## Bead 2: b\nbody2\n",
	}
}

func TestBeadSectionCorpusKeys(t *testing.T) {
	for name, text := range beadSectionCorpus() {
		t.Run(name, func(t *testing.T) {
			sections := validate.ParseBeadSections(text)
			if len(sections) != 2 {
				t.Fatalf("got %d sections, want 2 real sections", len(sections))
			}
			contents := extractBeadSectionContents(text)
			for i, section := range sections {
				body, ok := contents[section.Heading]
				if !ok || !strings.Contains(body, fmt.Sprintf("body%d", i+1)) {
					t.Errorf("heading %q lost its body: %q", section.Heading, body)
				}
			}
			if name == "fenced example" && !strings.Contains(contents[sections[0].Heading], "## Bead 9: example") {
				t.Error("example text was removed from its parent section")
			}
		})
	}
}

func TestBeadSectionCorpusCreation(t *testing.T) {
	for name, text := range beadSectionCorpus() {
		t.Run(name, func(t *testing.T) {
			var descriptions []string
			original := planRunBDFn
			t.Cleanup(func() { planRunBDFn = original })
			planRunBDFn = func(args ...string) ([]byte, error) {
				if len(args) == 0 || args[0] != "create" {
					t.Fatalf("unexpected call: %q", args)
				}
				for i, arg := range args {
					if arg == "--description" && i+1 < len(args) {
						descriptions = append(descriptions, args[i+1])
					}
				}
				return []byte(fmt.Sprintf(`{"id":"task-%d"}`, len(descriptions))), nil
			}
			sections := validate.ParseBeadSections(text)
			ids, _, err := createBeadsFromParsed(t.TempDir(), "example", "parent", text, sections, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"body1", "body2"}
			if name == "fenced example" {
				want[0] += "\n```md\n## Bead 9: example\nexample body\n```"
			}
			if len(ids) != 2 || !reflect.DeepEqual(descriptions, want) {
				t.Fatalf("created descriptions=%q, ids=%v; want %q", descriptions, ids, want)
			}
		})
	}
}

func TestBeadSectionMissingContent(t *testing.T) {
	for _, text := range []string{"## Bead 1: a\nbody1\n", "## Bead 1: a\nbody1\n\n## Bead 2: missing\n"} {
		t.Run(text, func(t *testing.T) {
			original := planRunBDFn
			t.Cleanup(func() { planRunBDFn = original })
			calls := 0
			planRunBDFn = func(args ...string) ([]byte, error) {
				calls++
				return []byte(`{"id":"task-unexpected"}`), nil
			}
			sections := []validate.BeadSection{{Heading: "Bead 1: a"}, {Heading: "Bead 2: missing"}}
			_, _, err := createBeadsFromParsed(t.TempDir(), "example", "parent", text, sections, nil)
			if err == nil || !strings.Contains(err.Error(), "has no extracted content") || !strings.Contains(err.Error(), "Bead 2: missing") || calls != 0 {
				t.Fatalf("missing section did not refuse before creation: err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestBeadSectionEmptyPreflight(t *testing.T) {
	forbidBDCalls(t)
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("unexpected epic query")
	}))
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(path, []byte("## Bead 1: empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolvePlanApprovePreflight(dir, path, "example")
	if err == nil || !strings.Contains(err.Error(), "has no extracted content") {
		t.Fatalf("empty section was not refused in preflight: %v", err)
	}
}

func TestS133DocGate_BeadSectionCorpus(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, shape := range []string{"trailing spaces", "CRLF", "closing hashes", "fenced example"} {
			t.Run(policy+"/"+shape, func(t *testing.T) {
				f := s133Document(t, policy, "plan_approve", "absent")
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				switch shape {
				case "trailing spaces":
					content = strings.Replace(content, "## Bead 1: First thing", "## Bead 1: First thing  ", 1)
				case "CRLF":
					content = strings.ReplaceAll(content, "\n", "\r\n")
				case "closing hashes":
					content = strings.Replace(content, "## Bead 1: First thing", "## Bead 1: First thing ##", 1)
				case "fenced example":
					content += "\n```md\n## Bead 9: example\nexample body\n```\n"
				}
				f.writePlan(content)
				f.commit("section shape")
				f.seed("plan_approve", "pass")
				original := planRunBDFn
				t.Cleanup(func() { planRunBDFn = original })
				description := ""
				planRunBDFn = func(args ...string) ([]byte, error) {
					if len(args) > 0 && args[0] == "create" {
						for i, arg := range args {
							if arg == "--description" && i+1 < len(args) {
								description = args[i+1]
							}
						}
					}
					return original(args...)
				}
				_, err := f.approve("plan_approve")
				if err != nil || f.creates != 1 || !strings.Contains(description, "Step one") || !strings.Contains(description, "First bead works") {
					t.Fatalf("section text lost: err=%v creates=%d description=%q", err, f.creates, description)
				}
				if shape == "fenced example" && !strings.Contains(description, "## Bead 9: example") {
					t.Error("example text lost from real bead")
				}
			})
		}
	}
}
