package next

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// SelectWork picks a work item from the list.
//
// Total pick semantics (spec 128 R6.1/AC-9, round-1 G10): 0 means unset —
// the OQ-5 default resolution (displayed row 1, bd ready's priority-first
// order); a negative pick is rejected with an error rather than silently
// ignored; any pick >= 1 indexes the DISPLAYED list (FormatWorkList's
// numbering) REGARDLESS of how many items it has — including exactly one,
// where an out-of-range pick (e.g. --pick 2 against a single-item list)
// now errors instead of silently returning the sole item. The prior
// single-item early return that bypassed pick validation is gone
// (red-on-revert: it used to return items[0] for ANY pick value once
// len(items)==1).
func SelectWork(items []BeadInfo, pick int) (BeadInfo, error) {
	if len(items) == 0 {
		return BeadInfo{}, fmt.Errorf("no items to select from")
	}
	if pick < 0 {
		return BeadInfo{}, fmt.Errorf("pick %d is invalid: must be >= 0 (0 selects the default)", pick)
	}
	if pick == 0 {
		// OQ-5 resolution: default to displayed row 1 — bd ready's
		// priority-first order, preserved end-to-end (R6.2).
		return items[0], nil
	}
	if pick > len(items) {
		return BeadInfo{}, fmt.Errorf("pick %d out of range (1-%d)", pick, len(items))
	}
	return items[pick-1], nil
}

// SelectNamedFromReadySet resolves a positional bead ID against a
// spec-scoped ReadySet — the claim path's positional resolution (spec 128
// R6.1). It defers to SelectWorkByName over the FILTERED (displayed) set
// for the normal cases, but first checks whether name identifies an item
// the spec-title filter DROPPED from the raw ready set (R6.3(b)/AC-10): a
// positional bead ID that IS ready but was excluded by mindspec's spec
// filter gets the honest exclusion-reason error instead of
// SelectWorkByName's generic "not found or not ready" — a filtered-out
// bead is a documented reason, not a mystery. An ambiguous match among
// excluded raw items is deliberately NOT resolved here — SelectWorkByName's
// own ambiguity/not-found message on the filtered set stays authoritative
// rather than this function guessing which excluded bead the caller meant.
func SelectNamedFromReadySet(rs ReadySet, name string) (BeadInfo, error) {
	if rs.SpecFlag != "" && rs.ExcludedCount() > 0 {
		if err, excluded := exclusionReasonForName(rs, name); excluded {
			return BeadInfo{}, err
		}
	}
	return SelectWorkByName(rs.Filtered, name)
}

// exclusionReasonForName reports whether name identifies (by exact ID or
// suffix, matching SelectWorkByName's own resolution rules) EXACTLY ONE
// item in rs.Raw that is not present in rs.Filtered — i.e. an item the
// spec-title filter actually dropped (never a recomputation of the
// filter itself; membership is checked against the real, already-computed
// Filtered set).
func exclusionReasonForName(rs ReadySet, name string) (error, bool) {
	if name == "" {
		return nil, false
	}
	kept := map[string]struct{}{}
	for _, item := range rs.Filtered {
		kept[item.ID] = struct{}{}
	}
	var match *BeadInfo
	for i := range rs.Raw {
		item := &rs.Raw[i]
		if _, ok := kept[item.ID]; ok {
			continue // present in the filtered set — not an exclusion case
		}
		if item.ID == name || strings.HasSuffix(item.ID, "-"+name) {
			if match != nil {
				return nil, false // ambiguous among excluded items — defer to SelectWorkByName
			}
			match = item
		}
	}
	if match == nil {
		return nil, false
	}
	return fmt.Errorf("bead %s is ready but excluded by the spec filter: title does not reference spec %s; it cannot be claimed via `mindspec next %s` while excluded",
		idrender.Bead(match.ID), idrender.Spec(rs.SpecFlag), name), true
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
//
// R4: IssueType is a bd `--type` value — not schema-enforced against a
// fixed enum on this side of the bd boundary (a caller invoking `bd
// create --type` directly, outside mindspec's own always-"task" wrapper,
// could set it to anything) — escape it alongside Title.
func FormatWorkList(items []BeadInfo) string {
	var result string
	for i, item := range items {
		result += fmt.Sprintf("  %d. [%s] %s (P%d, %s)\n", i+1, idrender.Bead(item.ID), termsafe.Escape(item.Title), item.Priority, termsafe.Escape(item.IssueType))
	}
	return result
}
