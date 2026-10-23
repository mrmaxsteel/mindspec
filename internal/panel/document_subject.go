package panel

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"gopkg.in/yaml.v3"
)

// IsLifecycleGate reports whether a panel reviews a lifecycle approval.
func IsLifecycleGate(gate string) bool {
	return gate == "spec_approve" || gate == "plan_approve" || gate == "final_review"
}

// CheckLifecycleTarget requires approval panels to follow their spec branch.
func CheckLifecycleTarget(gate, spec, target string) error {
	if IsLifecycleGate(gate) && target != "spec/"+spec {
		return fmt.Errorf("gate %s requires target spec/%s; recorded target is %s", termsafe.Escape(gate), termsafe.Escape(spec), termsafe.Escape(target))
	}
	return nil
}

// DocumentSubjectTreeReader selects semantic hashing for lifecycle panels.
// The callbacks keep Git access in the caller's execution layer.
func DocumentSubjectTreeReader(gate, specDir string, lsTree func(string, string) (string, error), read func(string, string, string) ([]byte, error)) func(string, string, SubjectRoots) (string, error) {
	if !IsLifecycleGate(gate) {
		return SubjectTreeReader(lsTree)
	}
	return func(workdir, ref string, roots SubjectRoots) (string, error) {
		listing, err := lsTree(workdir, ref)
		if err != nil {
			return "", err
		}
		return DocumentSubjectDigest(listing, roots, specDir, gate, func(name string) ([]byte, error) { return read(workdir, ref, name) })
	}
}

// DocumentSubjectDigest hashes the subject with approval-owned content removed.
// All remaining YAML keys and values are compared semantically. Other files
// retain their Git object identity and mode. The reader must read the same tree
// as listing. A failed read or malformed frontmatter refuses the digest.
func DocumentSubjectDigest(listing string, roots SubjectRoots, specDir, gate string, read func(string) ([]byte, error)) (string, error) {
	if strings.TrimSpace(listing) == "" {
		return "", fmt.Errorf("empty subject tree listing")
	}
	specDir = strings.TrimSuffix(Subject("spec_approve", specDir).Roots[0], "/")
	var normalized strings.Builder
	for _, line := range strings.SplitAfter(listing, "\n") {
		if line == "" {
			continue
		}
		meta, quoted, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || quoted == "" {
			return "", fmt.Errorf("malformed subject tree listing")
		}
		name := quoted
		if strings.HasPrefix(name, "\"") {
			var err error
			name, err = strconv.Unquote(name)
			if err != nil {
				return "", err
			}
		}
		if !SubjectIncludes(name, roots) {
			continue
		}
		isDoc := name == specDir+"/spec.md" || name == specDir+"/plan.md"
		isADR := gate == "final_review" && placeholderPath.MatchString(name)
		if fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755") && (isDoc || isADR) {
			data, err := read(name)
			if err != nil {
				return "", err
			}
			data, omit, err := DocumentSubjectContent(name, data, specDir, gate)
			if err != nil {
				return "", err
			}
			if omit {
				if fields[0] != "100644" {
					normalized.WriteString(line)
				}
				continue
			}
			fmt.Fprintf(&normalized, "%s blob sha256:%x\t%s\n", fields[0], sha256.Sum256(data), quoted)
		} else {
			normalized.WriteString(line)
		}
	}
	// A subject made only of excluded own writes is a valid empty subject.
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(normalized.String()))), nil
}

// SubjectIncludes applies the same root and exclusion rules as SubjectDigest.
func SubjectIncludes(name string, roots SubjectRoots) bool {
	included := false
	for _, root := range roots.Roots {
		if root == "." || name == root || strings.HasPrefix(name, strings.TrimSuffix(root, "/")+"/") {
			included = true
			break
		}
	}
	return included && len(RebindPaths([]string{name}, roots)) == 1
}

// DocumentSubjectContent is also used for uncommitted subject comparisons.
func DocumentSubjectContent(name string, data []byte, specDir, gate string) ([]byte, bool, error) {
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
			body = stripApproval(body)
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

func stripApproval(body []byte) []byte {
	text := string(body)
	start, end := -1, len(text)
	scanMarkdownLines(text, func(offset int, line string) {
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

var placeholderPath = regexp.MustCompile(`^(\.mindspec/adr|\.mindspec/docs/adr|docs/adr)/ADR-[0-9]{4,}(-[a-z0-9]+)*\.md$`)
var placeholderHeader = regexp.MustCompile(`^# (ADR-[0-9]{4,}(?:-[a-z0-9]+)*): Placeholder for (ADR-[0-9]{4,}(?:-[a-z0-9]+)*)\n`)

func isPlaceholder(data []byte) bool {
	text := string(data)
	m := placeholderHeader.FindStringSubmatch(text)
	if len(m) != 3 || m[1] != m[2] {
		return false
	}
	value := func(prefix string) string {
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimPrefix(line, prefix)
			}
		}
		return ""
	}
	date := value("- **Date**: ")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return false
	}
	want := strings.ReplaceAll(placeholderTemplate, "NNNN", strings.TrimPrefix(m[1], "ADR-"))
	want = strings.ReplaceAll(want, "<Title>", "Placeholder for "+m[1])
	want = strings.ReplaceAll(want, "<YYYY-MM-DD>", date)
	want = strings.Replace(want, "<comma-separated list>", value("- **Domain(s)**: "), 1)
	return text == want
}
