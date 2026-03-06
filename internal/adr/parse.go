package adr

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ADR represents a parsed Architecture Decision Record.
type ADR struct {
	ID           string
	Path         string
	Title        string
	Date         string
	Status       string
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
	id := strings.TrimSuffix(base, ".md")

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
			a.Status = extractValue(trimmed, "**Status**:")
		}
		if strings.Contains(trimmed, "**Domain(s)**:") {
			domStr := extractValue(trimmed, "**Domain(s)**:")
			for _, d := range strings.Split(domStr, ",") {
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

// NextID scans existing ADRs and returns the next available ID (zero-padded to 4 digits).
func NextID(root string) (string, error) {
	adrDir := workspace.ADRDir(root)
	pattern := filepath.Join(adrDir, "ADR-*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing ADRs: %w", err)
	}

	if len(matches) == 0 {
		return "0001", nil
	}

	maxNum := 0
	for _, path := range matches {
		base := filepath.Base(path)
		// Extract number from ADR-NNNN.md
		name := strings.TrimSuffix(base, ".md")
		numStr := strings.TrimPrefix(name, "ADR-")
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if n > maxNum {
			maxNum = n
		}
	}

	return fmt.Sprintf("%04d", maxNum+1), nil
}
