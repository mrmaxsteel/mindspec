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
func Show(root, id string) (*ADR, error) {
	path := filepath.Join(workspace.ADRDir(root), id+".md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s not found", id)
	}

	a, err := ParseADR(path)
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
