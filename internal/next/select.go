package next

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/shellquote"
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

// SelectNamedFromReadySet honors scoped positional intent before empty-set handling.
func SelectNamedFromReadySet(rs ReadySet, name string) (BeadInfo, error) {
	if rs.SpecFlag == "" {
		return SelectWorkByName(rs.Filtered, name)
	}
	if rs.EpicID != "" && name == rs.EpicID {
		return BeadInfo{}, fmt.Errorf("bead %s is the epic of spec %s, not a bead to claim\nrecovery: mindspec next --spec %s", idrender.Bead(rs.EpicID), idrender.Spec(rs.SpecFlag), shellquote.Quote(idrender.Spec(rs.SpecFlag)))
	}
	item, err := SelectWorkByName(rs.Raw, name)
	if err != nil {
		var ambiguous *ambiguousBeadNameError
		if errors.As(err, &ambiguous) {
			return BeadInfo{}, err
		}
		if len(rs.Filtered) == 0 {
			return BeadInfo{}, fmt.Errorf("no ready plan bead for spec %s; positional %s cannot be claimed from an empty ready set\nrecovery: mindspec next --spec %s", idrender.Spec(rs.SpecFlag), idrender.Bead(name), shellquote.Quote(idrender.Spec(rs.SpecFlag)))
		}
		return BeadInfo{}, err
	}
	for _, kept := range rs.Filtered {
		if kept.ID == item.ID {
			return item, nil
		}
	}
	for _, miss := range rs.NearMisses {
		if miss.Bead.ID != item.ID {
			continue
		}
		if miss.Class == "N4" {
			if miss.NoLineage {
				return BeadInfo{}, fmt.Errorf("bead %s has no lineage, so readiness cannot evaluate it; no readiness override applies\nrecovery: bd show %s --json", idrender.Bead(item.ID), shellquote.Quote(idrender.Bead(item.ID)))
			}
			if miss.Lineage == rs.EpicID {
				return item, nil
			}
		}
		return BeadInfo{}, fmt.Errorf("%s", miss.String())
	}
	parent := ""
	if item.Parent != "" {
		parent = " (parent: " + idrender.Bead(item.Parent) + ")"
	}
	return BeadInfo{}, fmt.Errorf("bead %s is bd-ready but is not linked to spec %s%s\nrecovery: mindspec next --spec %s", idrender.Bead(item.ID), idrender.Spec(rs.SpecFlag), parent, shellquote.Quote(idrender.Spec(rs.SpecFlag)))
}

type ambiguousBeadNameError struct{ error }

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
	var matches []BeadInfo
	for i := range items {
		if strings.HasSuffix(items[i].ID, "-"+name) {
			matches = append(matches, items[i])
		}
	}
	if len(matches) > 1 {
		// Final-review round 1 (R5/G4, superseding round-1 panel G3):
		// the prior `e.g. \`mindspec-%s\`` example both hardcoded one
		// project's issue prefix and interpolated the selector into an
		// executable-looking span where termsafe is identity on
		// printable shell metacharacters. Name the REAL candidates
		// instead — each through idrender.Bead's validate-or-quote gate
		// (bd IDs are agent-writable, ADR-0042) — and keep the free-text
		// selector in a %q position only.
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = idrender.Bead(m.ID)
		}
		return BeadInfo{}, &ambiguousBeadNameError{fmt.Errorf("bead name %q is ambiguous across the ready set (matches %s); qualify it with the full bead ID", name, strings.Join(ids, ", "))}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	// Final-review round 1 (R5/G4): the executable-shaped guidance
	// carries NO dynamic operand — the unresolvable selector appears
	// only in the %q position.
	return BeadInfo{}, fmt.Errorf("bead %q is not in the ready set (not found or not ready); run `mindspec next` with no positional to list claimable work", name)
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
