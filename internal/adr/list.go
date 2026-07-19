package adr

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// ListOpts configures ADR listing.
type ListOpts struct {
	Status string
	Domain string
}

// List returns ADRs matching the given filters.
func List(root string, opts ListOpts) ([]ADR, error) {
	adrs, err := ScanADRs(root)
	if err != nil {
		return nil, err
	}

	var result []ADR
	for _, a := range adrs {
		if opts.Status != "" && !strings.EqualFold(a.Status, opts.Status) {
			continue
		}
		if opts.Domain != "" {
			found := false
			target := strings.ToLower(strings.TrimSpace(opts.Domain))
			for _, d := range a.Domains {
				if d == target {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, a)
	}

	return result, nil
}

// FormatTable renders a list of ADRs as a columnar table.
func FormatTable(adrs []ADR) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("%-10s %-12s %-20s %s\n", "ID", "Status", "Domain(s)", "Title"))
	b.WriteString(fmt.Sprintf("%-10s %-12s %-20s %s\n", "──────────", "────────────", "────────────────────", "─────"))

	for _, a := range adrs {
		// R4: ID/Status/Domains/Title all originate from agent-writable ADR
		// filename/frontmatter/heading content — escape every field.
		domains := termsafe.Escape(strings.Join(a.Domains, ", "))
		b.WriteString(fmt.Sprintf("%-10s %-12s %-20s %s\n", termsafe.Escape(a.ID), termsafe.Escape(a.DisplayStatus()), domains, termsafe.Escape(a.Title)))
	}

	return b.String()
}
