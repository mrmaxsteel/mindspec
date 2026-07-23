package adr

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ADR represents a parsed Architecture Decision Record.
type ADR struct {
	ID    string
	Path  string
	Title string
	Date  string
	// Status is the normalized lifecycle status: the first token of the
	// raw **Status**: line value (e.g. "Accepted" for a line reading
	// "Accepted (Finalized in spec 090 Bead 1)"). All known statuses are
	// single words (Proposed/Accepted/Superseded/Deprecated/Withdrawn/
	// Rejected), so comparisons like strings.EqualFold(a.Status,
	// "Accepted") work regardless of trailing provenance qualifiers.
	Status string
	// StatusRaw preserves the full **Status**: line value, including any
	// parenthetical qualifiers, for display paths (show / list) so
	// provenance notes aren't lost by normalization.
	StatusRaw    string
	Domains      []string
	Supersedes   string
	SupersededBy string
	Content      string
}

// ParseADR reads and parses a single ADR file.
func ParseADR(path string) (ADR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ADR{}, err
	}

	content := string(data)
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, ".md")
	// R5(b) (spec 123): ID is the canonical "ADR-<digits>" prefix of the
	// stem, never the full slugged stem — a slugged file
	// "ADR-0001-integrate-at-contracts.md" reports ID "ADR-0001", not the
	// long stem. Shares the digit-walk with workspace.ResolveADRFile via
	// idvalidate.ADRCanonicalPrefix so the two can't disagree about where
	// an ADR's number ends.
	id := idvalidate.ADRCanonicalPrefix(stem)

	a := ADR{
		ID:      id,
		Path:    path,
		Content: content,
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Extract title from heading: # ADR-NNNN: <Title>
		if a.Title == "" && strings.HasPrefix(trimmed, "# ADR-") {
			if idx := strings.Index(trimmed, ": "); idx >= 0 {
				a.Title = strings.TrimSpace(trimmed[idx+2:])
			}
		}

		if strings.Contains(trimmed, "**Date**:") {
			a.Date = extractValue(trimmed, "**Date**:")
		}
		if strings.Contains(trimmed, "**Status**:") {
			a.StatusRaw = extractValue(trimmed, "**Status**:")
			a.Status = normalizeStatus(a.StatusRaw)
		}
		if strings.Contains(trimmed, "**Domain(s)**:") {
			domStr := extractValue(trimmed, "**Domain(s)**:")
			for _, d := range splitTopLevel(domStr) {
				d = strings.ToLower(strings.TrimSpace(d))
				if d != "" {
					a.Domains = append(a.Domains, d)
				}
			}
		}
		if strings.Contains(trimmed, "**Supersedes**:") {
			v := extractValue(trimmed, "**Supersedes**:")
			if v != "n/a" && v != "" {
				a.Supersedes = v
			}
		}
		if strings.Contains(trimmed, "**Superseded-by**:") {
			v := extractValue(trimmed, "**Superseded-by**:")
			if v != "n/a" && v != "" {
				a.SupersededBy = v
			}
		}
	}

	return a, nil
}

// DisplayStatus returns the status string for human-facing output:
// StatusRaw (full provenance, e.g. "Accepted (Finalized in spec 090
// Bead 1)") when available, falling back to the normalized Status for
// ADR values constructed without a raw line (struct literals, mocks).
func (a *ADR) DisplayStatus() string {
	if a.StatusRaw != "" {
		return a.StatusRaw
	}
	return a.Status
}

// normalizeStatus reduces a raw **Status**: line value to its canonical
// single-token form. Authors routinely append provenance qualifiers —
// e.g. "Accepted (Finalized in spec 090 Bead 1)" or "Withdrawn
// (superseded by ADR-0015)" — which previously broke every exact
// strings.EqualFold(status, "Accepted") comparison downstream
// (FilterADRs, plan coverage, adr list --status). The normalized form is
// the first whitespace-delimited token, with any leading parenthetical
// split off and trailing punctuation trimmed.
func normalizeStatus(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Split off a directly-attached qualifier: "Accepted(note)".
	if idx := strings.IndexAny(raw, "([{"); idx >= 0 {
		raw = raw[:idx]
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimRight(fields[0], "(:;,.")
}

// splitTopLevel splits s on commas that are NOT nested inside any of
// ( ) [ ] { } — a depth-tracking variant of strings.Split(s, ",").
// Domain annotations routinely carry parenthesized qualifiers with
// embedded commas, e.g.:
//
//	webapp (`app/`, react navigation native-stack), api, infra
//
// which a naive comma split shreds into broken tokens. Depth never
// goes negative: an unmatched closing bracket is ignored (clamped at
// 0) so a malformed value degrades to the naive behavior rather than
// swallowing the rest of the line. With no brackets present the
// output is identical to strings.Split.
func splitTopLevel(s string) []string {
	var (
		out   []string
		depth int
		start int
	)
	for i, r := range s {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + len(",")
			}
		}
	}
	out = append(out, s[start:])
	return out
}

// extractValue extracts the value after a metadata key in a line.
func extractValue(line, key string) string {
	idx := strings.Index(line, key)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[idx+len(key):])
}

// ScanADRs reads all ADR-*.md files from the ADR directory, sorted by ID.
func ScanADRs(root string) ([]ADR, error) {
	adrDir := workspace.ADRDir(root)
	pattern := filepath.Join(adrDir, "ADR-*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing ADRs: %w", err)
	}

	var adrs []ADR
	for _, path := range matches {
		a, err := ParseADR(path)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
		}
		adrs = append(adrs, a)
	}

	sort.Slice(adrs, func(i, j int) bool {
		return adrs[i].ID < adrs[j].ID
	})

	return adrs, nil
}

// FilterADRs returns ADRs with Status "Accepted" that have at least one
// domain in common with the provided domain list.
func FilterADRs(adrs []ADR, domains []string) []ADR {
	domainSet := make(map[string]bool)
	for _, d := range domains {
		domainSet[strings.ToLower(strings.TrimSpace(d))] = true
	}

	var result []ADR
	for _, a := range adrs {
		if !strings.EqualFold(a.Status, "Accepted") {
			continue
		}
		for _, d := range a.Domains {
			if domainSet[d] {
				result = append(result, a)
				break
			}
		}
	}
	return result
}

// NextID scans existing ADRs under root and returns the next available ID
// (zero-padded to 4 digits).
func NextID(root string) (string, error) {
	maxNum, err := maxADRNum(root)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", maxNum+1), nil
}

// NextIDAcross scans ADRs under EVERY supplied root and returns the next
// available ID over their UNION (the max leading-integer across all roots,
// +1). It exists so `adr create` from a bead/spec worktree — whose WRITE
// target is the worktree-local root but whose READS/validation union the
// worktree branch ADRs with the main-checkout ADRs — allocates an ID that
// cannot collide with a main-only ADR added after the branch diverged
// (mindspec-8lzq). With a single root it is identical to NextID.
func NextIDAcross(roots ...string) (string, error) {
	maxNum := 0
	for _, root := range roots {
		n, err := maxADRNum(root)
		if err != nil {
			return "", err
		}
		if n > maxNum {
			maxNum = n
		}
	}
	return fmt.Sprintf("%04d", maxNum+1), nil
}

// maxADRNum returns the highest ADR number found under root (0 when none).
func maxADRNum(root string) (int, error) {
	adrDir := workspace.ADRDir(root)
	pattern := filepath.Join(adrDir, "ADR-*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, fmt.Errorf("globbing ADRs: %w", err)
	}

	maxNum := 0
	for _, path := range matches {
		base := filepath.Base(path)
		// Extract the leading numeric run after the "ADR-" prefix.
		// Both the legacy bare "ADR-NNNN.md" and the slugged
		// "ADR-NNNN-slug.md" forms carry their numeric ID as the run
		// of digits immediately following "ADR-". Atoi-ing the whole
		// "NNNN-slug" suffix (the prior behavior) failed for every
		// slugged filename, skipping it and leaving maxNum too low —
		// so a directory of only slugged ADRs produced a COLLIDING
		// low NextID. Reading just the leading digit run counts both
		// forms; a non-numeric head (e.g. "ADR-foo.md") yields an
		// empty run and is skipped safely.
		name := strings.TrimSuffix(base, ".md")
		rest := strings.TrimPrefix(name, "ADR-")
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		if end == 0 {
			continue
		}
		n, err := strconv.Atoi(rest[:end])
		if err != nil {
			continue
		}
		if n > maxNum {
			maxNum = n
		}
	}

	return maxNum, nil
}
