package adr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Show reads and returns a single ADR by ID.
//
// Lookup first tries an exact filename match ("<id>.md"). If that fails, it
// falls back to a slug-tolerant match: any file named "<id>-<slug>.md" in the
// ADR directory, provided it's unambiguous. This lets plans cite a pure ID
// like `ADR-0001` even when the on-disk file is `ADR-0001-descriptive.md`.
func Show(root, id string) (*ADR, error) {
	dir := workspace.ADRDir(root)
	path := filepath.Join(dir, id+".md")
	if _, err := os.Stat(path); err == nil {
		a, err := ParseADR(path)
		if err != nil {
			return nil, err
		}
		return &a, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	matches, err := filepath.Glob(filepath.Join(dir, id+"-*.md"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s not found", id)
	}
	if len(matches) > 1 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = filepath.Base(m)
		}
		return nil, fmt.Errorf("%s is ambiguous: matches %s", id, strings.Join(names, ", "))
	}

	a, err := ParseADR(matches[0])
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ExtractDecision extracts the content of the ## Decision section.
func ExtractDecision(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inDecision := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Decision" {
			inDecision = true
			continue
		}
		if inDecision && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if inDecision {
			result = append(result, line)
		}
	}

	text := strings.TrimSpace(strings.Join(result, "\n"))
	return text
}

// FormatSummary produces a human-readable summary of an ADR.
func FormatSummary(a *ADR) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("ID:             %s\n", a.ID))
	b.WriteString(fmt.Sprintf("Title:          %s\n", a.Title))
	b.WriteString(fmt.Sprintf("Status:         %s\n", a.Status))
	b.WriteString(fmt.Sprintf("Date:           %s\n", a.Date))
	b.WriteString(fmt.Sprintf("Domain(s):      %s\n", strings.Join(a.Domains, ", ")))

	if a.Supersedes != "" {
		b.WriteString(fmt.Sprintf("Supersedes:     %s\n", a.Supersedes))
	}
	if a.SupersededBy != "" {
		b.WriteString(fmt.Sprintf("Superseded-by:  %s\n", a.SupersededBy))
	}

	decision := ExtractDecision(a.Content)
	if decision != "" {
		b.WriteString(fmt.Sprintf("\nDecision:\n%s\n", decision))
	}

	return b.String()
}

// adrJSON is the JSON representation of an ADR for --json output.
type adrJSON struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Date         string   `json:"date"`
	Domains      []string `json:"domains"`
	Supersedes   string   `json:"supersedes,omitempty"`
	SupersededBy string   `json:"superseded_by,omitempty"`
	Decision     string   `json:"decision"`
}

// FormatJSON produces a JSON representation of an ADR.
func FormatJSON(a *ADR) (string, error) {
	j := adrJSON{
		ID:           a.ID,
		Title:        a.Title,
		Status:       a.Status,
		Date:         a.Date,
		Domains:      a.Domains,
		Supersedes:   a.Supersedes,
		SupersededBy: a.SupersededBy,
		Decision:     ExtractDecision(a.Content),
	}
	if j.Domains == nil {
		j.Domains = []string{}
	}

	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
