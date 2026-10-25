package panel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Keep the previous digest bytes for LF documents.
func frozenDocumentSubjectContent(name string, data []byte, specDir, gate string) ([]byte, bool, error) {
	documentGate := gate == "spec_approve" || gate == "plan_approve"
	if gate == "final_review" && placeholderPath.MatchString(name) && strings.HasPrefix(string(data), "# "+strings.TrimSuffix(path.Base(name), ".md")+":") && isPlaceholder(data) {
		return nil, true, nil
	}
	if name != specDir+"/spec.md" && name != specDir+"/plan.md" {
		return data, false, nil
	}
	if documentGate && name == specDir+"/plan.md" && bytes.Equal(data, []byte(PlanScaffold(path.Base(specDir)))) {
		return nil, true, nil
	}
	body := data
	var fm map[string]any
	if bytes.HasPrefix(data, []byte("---\n")) {
		rest := data[4:]
		end := bytes.Index(rest, []byte("\n---\n"))
		blockEnd, bodyStart := end, end+5
		if bytes.HasPrefix(rest, []byte("---\n")) {
			blockEnd, bodyStart = 0, 4
		}
		if blockEnd < 0 {
			return nil, false, fmt.Errorf("unclosed document frontmatter")
		}
		if err := yaml.Unmarshal(rest[:blockEnd], &fm); err != nil {
			return nil, false, fmt.Errorf("invalid document frontmatter: %w", err)
		}
		body = rest[bodyStart:]
	}

	if documentGate {
		for _, key := range []string{"status", "approved_at", "approved_by"} {
			delete(fm, key)
		}
		if name == specDir+"/plan.md" {
			delete(fm, "bead_ids")
		} else {
			body = frozenStripApproval(body)
			// Appending the approval section supplies its own blank separator.
			// Normalize that end padding when the section is absent as well.
			body = []byte(strings.TrimRight(string(body), "\n"))
		}
	}
	// Keep frontmatter and body in distinct fields to prevent representation
	// collisions with body text that happens to start with serialized YAML.
	if fm == nil {
		fm = map[string]any{}
	}
	if body == nil {
		body = []byte{}
	}
	canonical, err := json.Marshal(struct {
		Frontmatter map[string]any `json:"frontmatter"`
		Body        []byte         `json:"body"`
	}{fm, body})
	if err != nil {
		return nil, false, fmt.Errorf("invalid document frontmatter values: %w", err)
	}
	return canonical, false, nil
}

func checkDocumentSubjectLFCompatibility(t *testing.T, name string, data []byte, dir, gate string) {
	t.Helper()
	if bytes.Contains(data, []byte("\r\n")) {
		t.Fatal("LF fixture contains CRLF")
	}
	got, omit, err := DocumentSubjectContent(name, data, dir, gate)
	want, wantOmit, wantErr := frozenDocumentSubjectContent(name, data, dir, gate)
	if !bytes.Equal(got, want) || omit != wantOmit || fmt.Sprint(err) != fmt.Sprint(wantErr) {
		t.Fatalf("LF digest bytes changed for %s (%s): got %q, %v, %v; want %q, %v, %v", name, gate, got, omit, err, want, wantOmit, wantErr)
	}
}

func TestS133Digest_LFContentCompatibility(t *testing.T) {
	files := trackedMarkdown(t)
	cases := 0
	for name, text := range files {
		if strings.Contains(text, "\r") {
			continue
		}
		for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
			checkDocumentSubjectLFCompatibility(t, name, []byte(text), path.Dir(name), gate)
			cases++
		}
	}
	if cases == 0 {
		t.Fatal("empty LF fixture corpus")
	}
	t.Logf("LF document comparisons: %d", cases)
}

func TestS133Digest_LFEdgeCompatibility(t *testing.T) {
	const dir = ".mindspec/specs/example"
	for _, body := range []string{"Body\n", "Body\r", "Body\n\r", "key: |\n  value\n", ""} {
		for _, fm := range []string{"", "---\n---\n", "---\ntitle: Example\n---\n", "---\nbody: |\n  value\n---\n", "---\n", "---\n[bad\n---\n", "---\ntitle: Example\n---"} {
			for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
				checkDocumentSubjectLFCompatibility(t, dir+"/spec.md", []byte(fm+body), dir, gate)
			}
		}
	}
}
