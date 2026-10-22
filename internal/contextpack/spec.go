package contextpack

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SpecMeta holds metadata extracted from a spec file.
type SpecMeta struct {
	SpecID  string
	Goal    string
	Domains []string
}

// ParseSpec reads a spec.md file and extracts the goal and impacted domains.
func ParseSpec(specDir string) (*SpecMeta, error) {
	specPath := filepath.Join(specDir, "spec.md")
	f, err := os.Open(specPath)
	if err != nil {
		return nil, fmt.Errorf("opening spec: %w", err)
	}
	defer f.Close()

	meta := &SpecMeta{
		SpecID: filepath.Base(specDir),
	}

	scanner := bufio.NewScanner(f)
	var section string
	var goalLines []string
	sawTopBullet := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			heading = strings.TrimSpace(heading)

			// If we were collecting goal lines, stop
			if section == "goal" {
				meta.Goal = strings.TrimSpace(strings.Join(goalLines, "\n"))
			}

			switch strings.ToLower(heading) {
			case "goal":
				section = "goal"
				goalLines = nil
			case "impacted domains":
				section = "domains"
				sawTopBullet = false
			default:
				section = ""
			}
			continue
		}

		switch section {
		case "goal":
			goalLines = append(goalLines, line)
		case "domains":
			if d, ok := impactedDomainBullet(line, sawTopBullet); ok {
				if !isIndented(line) {
					sawTopBullet = true
				}
				meta.Domains = append(meta.Domains, d...)
			}
		}
	}

	// Handle goal at end of file
	if section == "goal" && meta.Goal == "" {
		meta.Goal = strings.TrimSpace(strings.Join(goalLines, "\n"))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading spec: %w", err)
	}

	return meta, nil
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

// impactedDomainBullet parses one `## Impacted Domains` line. An indented
// sub-bullet under a top-level bullet (for example a glob list under
// `- **workflow**:`) describes its parent domain and is not a domain. The
// domain is the text before the first colon, without emphasis or code
// fences. A leading run of backtick code spans joined by commas gives one
// domain per span.
func impactedDomainBullet(line string, underTopBullet bool) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- ") {
		return nil, false
	}
	if underTopBullet && isIndented(line) {
		return nil, false
	}
	bullet := strings.TrimPrefix(trimmed, "- ")
	domain := bullet
	if idx := strings.Index(bullet, ":"); idx >= 0 {
		domain = strings.TrimSpace(bullet[:idx])
	}
	domain = strings.TrimSpace(domain)
	if spans := leadingCodeSpans(domain); len(spans) > 1 {
		return spans, true
	}
	// Strip markdown emphasis (double asterisks) and inline code fences
	// that authors commonly wrap around domain identifiers.
	domain = strings.TrimPrefix(domain, "**")
	domain = strings.TrimSuffix(domain, "**")
	domain = strings.Trim(domain, "`")
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, true
	}
	return []string{domain}, true
}

// leadingCodeSpans returns the code spans at the start of s that are joined
// only by commas, "and" or whitespace. It stops at any other text.
func leadingCodeSpans(s string) []string {
	var out []string
	rest := s
	for {
		rest = strings.TrimSpace(rest)
		rest = strings.TrimPrefix(rest, ",")
		rest = strings.TrimSpace(rest)
		if len(out) > 0 && strings.HasPrefix(rest, "and ") {
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "and "))
		}
		if !strings.HasPrefix(rest, "`") {
			return out
		}
		end := strings.Index(rest[1:], "`")
		if end < 0 {
			return out
		}
		if span := strings.ToLower(strings.TrimSpace(rest[1 : 1+end])); span != "" {
			out = append(out, span)
		}
		rest = rest[2+end:]
	}
}
