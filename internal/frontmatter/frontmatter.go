// Package frontmatter parses YAML frontmatter from markdown documents
// (spec.md, plan.md, and any other doc that uses leading `---` fences).
//
// This is the canonical implementation for MindSpec. Callers must NOT
// re-implement substring/prefix scanning of YAML status fields — they
// will silently drift on quoting, casing, comments, and multi-line values.
// See ARCH-6 (mindspec-npd2) for the consolidation rationale.
package frontmatter

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Status returns the trimmed value of the `status:` key from the YAML
// frontmatter at the head of data. Case is preserved; callers decide
// how to compare (use strings.EqualFold for the Draft/Approved gate).
//
// Returns "" if data has no `---` opener, no closing `---`, no `status`
// field, or fails YAML parsing.
func Status(data []byte) string {
	v, _ := Field(data, "status")
	return v
}

// StatusFromPath is Status but reads from a file path. Returns "" on
// any I/O or parse error (no distinction — frontmatter is a contract,
// "missing" and "malformed" are equally invalid).
func StatusFromPath(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return Status(data)
}

// Field returns the string value of an arbitrary scalar key in the
// frontmatter. `ok` is false when the field is missing, the document
// has no frontmatter, or the value is non-scalar (sequence/mapping).
// Whitespace around the value is trimmed; quotes (single/double) are
// honored by the underlying yaml unmarshaller.
func Field(data []byte, key string) (value string, ok bool) {
	block, _, found := Parse(data)
	if !found {
		return "", false
	}
	var m map[string]any
	if err := yaml.Unmarshal(block, &m); err != nil {
		return "", false
	}
	raw, present := m[key]
	if !present || raw == nil {
		return "", false
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v), true
	case []any, map[string]any:
		return "", false
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v)), true
	}
}

// Parse returns the raw YAML frontmatter block (between the opening
// and closing `---` fences) as a []byte plus the byte offset where
// the body content begins. Returned block is nil and offset is 0 if
// no frontmatter is present (no opening fence, no closing fence, or
// otherwise malformed).
func Parse(data []byte) (block []byte, bodyOffset int, ok bool) {
	if len(data) == 0 {
		return nil, 0, false
	}
	// Split preserving line indices so we can compute byte offsets.
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) == 0 {
		return nil, 0, false
	}
	if strings.TrimRight(string(lines[0]), "\r\n") != "---" {
		return nil, 0, false
	}
	// Find closing fence among subsequent lines.
	var blockBuf bytes.Buffer
	consumed := len(lines[0])
	closerFound := false
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		consumed += len(line)
		stripped := strings.TrimRight(string(line), "\r\n")
		if stripped == "---" {
			closerFound = true
			break
		}
		blockBuf.Write(line)
	}
	if !closerFound {
		return nil, 0, false
	}
	return blockBuf.Bytes(), consumed, true
}
