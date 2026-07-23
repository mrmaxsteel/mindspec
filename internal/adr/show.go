package adr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Show reads and returns a single ADR by ID.
//
// Lookup resolves through workspace.ResolveADRFile: an exact filename match
// ("<id>.md") when the on-disk file is bare, or a slug-tolerant match
// ("<id>-<slug>.md") when it isn't — this lets callers cite a pure ID like
// `ADR-0001` even when the on-disk file is `ADR-0001-descriptive.md`. A
// directory holding BOTH a bare and a slugged file for the same number is a
// collision: ResolveADRFile errors naming both paths rather than silently
// preferring the bare file (spec 123 R5(c)).
//
// SEC-1 (bead mindspec-x1qr): id is validated BEFORE any path/glob
// construction (ResolveADRFile re-validates defensively). Without
// validation, an id containing `*`, `?`, `[`, `]` would inject a glob
// pattern; an id with `/` or `..` would escape the ADR directory.
//
// G2 (final review): a ResolveADRFile "not found" result is translated to
// the Store-level ErrNotFound sentinel so a caller layering Stores
// (OverlayStore) can fall through to another store on a genuine miss while
// still seeing every OTHER error — most importantly a collision, when the
// directory holds both a bare and a slugged file for the same number —
// propagate untranslated, so it can never be silently swallowed by a
// fallback.
func Show(root, id string) (*ADR, error) {
	if err := idvalidate.ADRID(id); err != nil {
		return nil, err
	}
	path, err := workspace.ResolveADRFile(root, id)
	if err != nil {
		if errors.Is(err, workspace.ErrADRNotFound) {
			return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
		}
		return nil, err
	}
	a, err := ParseADR(path)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// escapeLines applies termsafe.Escape to each line of a (possibly
// multi-line) block of agent-writable text — an ADR's ## Decision body —
// while preserving the real newlines that separate genuine lines (R4:
// per-line escaping for line-oriented bodies, never per-message, so a
// hostile line cannot forge additional lines while legitimate multi-line
// structure survives).
func escapeLines(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = termsafe.Escape(l)
	}
	return strings.Join(lines, "\n")
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

	// R4: every field below is agent-writable ADR frontmatter/filename
	// content (ID is the on-disk filename stem, the rest come straight off
	// the **Key**: lines or heading of the .md body) — termsafe.Escape each
	// single-line value; Decision is a multi-line free-text body so it is
	// escaped per-line via escapeLines.
	b.WriteString(fmt.Sprintf("ID:             %s\n", termsafe.Escape(a.ID)))
	b.WriteString(fmt.Sprintf("Title:          %s\n", termsafe.Escape(a.Title)))
	b.WriteString(fmt.Sprintf("Status:         %s\n", termsafe.Escape(a.DisplayStatus())))
	b.WriteString(fmt.Sprintf("Date:           %s\n", termsafe.Escape(a.Date)))
	b.WriteString(fmt.Sprintf("Domain(s):      %s\n", termsafe.Escape(strings.Join(a.Domains, ", "))))

	if a.Supersedes != "" {
		b.WriteString(fmt.Sprintf("Supersedes:     %s\n", termsafe.Escape(a.Supersedes)))
	}
	if a.SupersededBy != "" {
		b.WriteString(fmt.Sprintf("Superseded-by:  %s\n", termsafe.Escape(a.SupersededBy)))
	}

	decision := ExtractDecision(a.Content)
	if decision != "" {
		b.WriteString(fmt.Sprintf("\nDecision:\n%s\n", escapeLines(decision)))
	}

	return b.String()
}

// adrJSON is the JSON representation of an ADR for --json output.
type adrJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	// StatusRaw carries the unnormalized **Status**: line value (with
	// provenance qualifiers) when it differs from the normalized Status.
	StatusRaw    string   `json:"status_raw,omitempty"`
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
	if a.StatusRaw != "" && a.StatusRaw != a.Status {
		j.StatusRaw = a.StatusRaw
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
