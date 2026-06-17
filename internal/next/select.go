package next

import (
	"fmt"
	"strings"
)

// SelectWork picks a work item from the list.
// If exactly one item, it is returned directly (auto-claim).
// If multiple, it prints a numbered list and returns the selected index.
// The pick parameter selects a specific item (1-based); 0 means default to first.
func SelectWork(items []BeadInfo, pick int) (BeadInfo, error) {
	if len(items) == 0 {
		return BeadInfo{}, fmt.Errorf("no items to select from")
	}

	if len(items) == 1 {
		return items[0], nil
	}

	// Multiple items — validate pick or default to first
	if pick > 0 {
		if pick > len(items) {
			return BeadInfo{}, fmt.Errorf("pick %d out of range (1-%d)", pick, len(items))
		}
		return items[pick-1], nil
	}

	// Default: return first item
	return items[0], nil
}

// SelectWorkByName selects the bead named by `name` from the already-fetched
// ready `items` slice. It is the claim-path counterpart to SelectWork for when
// the caller supplied a positional bead ID: the positional names a SPECIFIC
// bead the caller intends, so it must resolve to exactly that bead or fail —
// it MUST NOT fall through to items[0] (spec 101 R1 / mindspec-mfe0). A name
// that is not in the ready set (not ready, or not found) returns a clear error.
func SelectWorkByName(items []BeadInfo, name string) (BeadInfo, error) {
	// Empty name can never name a SPECIFIC bead — and an empty suffix needle
	// ("-"+"" == "-") would spuriously match any ID ending in "-". The caller
	// already guards `named != ""`; this is defense-in-depth so the unit is
	// safe in isolation.
	if name == "" {
		return BeadInfo{}, fmt.Errorf("empty bead name cannot be resolved against the ready set")
	}

	// Exact match wins, and is checked across ALL items FIRST. A positional ID
	// names a SPECIFIC bead, so a full-ID input must resolve to its exact bead
	// even if an earlier item suffix-matches the same trailing segment — exact
	// intent is never shadowed by a looser suffix match.
	for _, item := range items {
		if item.ID == name {
			return item, nil
		}
	}

	// No exact hit — fall back to suffix-aware matching so a SHORT form ("xxxx")
	// resolves against whatever issue-prefix the ready set carries, with no
	// hardcoded prefix literal (the prefix is project-derived, written once into
	// .beads/config.yaml). The needle's leading "-" anchors the match to a whole
	// hyphen-delimited trailing segment (so "xxx" never matches "...-yyxxx").
	//
	// The project carries a primary issue-prefix, but a second hyphenated
	// namespace can coexist in the FULL issue store (e.g. `bd mol` molecule IDs
	// like "mindspec-mol-qmq" alongside "mindspec-qmq"), so a post-prefix suffix
	// is NOT guaranteed globally unique. We therefore do not assume uniqueness:
	// if more than one ready item suffix-matches, the short form is genuinely
	// ambiguous and we error (telling the caller to qualify with the full ID)
	// rather than silently claim an order-dependent arbitrary bead.
	var match *BeadInfo
	ambiguous := false
	for i := range items {
		if strings.HasSuffix(items[i].ID, "-"+name) {
			if match != nil {
				ambiguous = true
				break
			}
			match = &items[i]
		}
	}
	if ambiguous {
		return BeadInfo{}, fmt.Errorf("bead name %q is ambiguous across the ready set; qualify it with the full bead ID (e.g. `mindspec-%s`)", name, name)
	}
	if match != nil {
		return *match, nil
	}

	return BeadInfo{}, fmt.Errorf("bead %q is not in the ready set (not found or not ready); it cannot be claimed via `mindspec next %s`", name, name)
}

// FormatWorkList returns a formatted numbered list of work items for display.
func FormatWorkList(items []BeadInfo) string {
	var result string
	for i, item := range items {
		result += fmt.Sprintf("  %d. [%s] %s (P%d, %s)\n", i+1, item.ID, item.Title, item.Priority, item.IssueType)
	}
	return result
}
