// Package idrender provides forced-safe rendering for the ID-typed
// positions covered by internal/idvalidate: spec IDs and bead IDs.
//
// Unlike free-text fields, an ID-typed render position cannot rely on
// termsafe.Escape alone: Escape is the IDENTITY on every printable ASCII
// string, so a hostile-but-printable value shaped like an ID (e.g.
// "120-x;evil") passes Escape completely unchanged — it never gets
// quoted, and it never gets rejected, because it isn't a control-byte or
// non-ASCII injection. What makes it hostile is that it is NOT a
// well-formed ID: rendering it unquoted alongside genuine IDs invites
// visual confusion (it can be mistaken for a real, validated identifier)
// even though it forges no extra terminal lines.
//
// idrender closes that gap by keying safety off the SAME grammar the
// waist enforces (see idvalidate.SpecID / idvalidate.BeadID), not off
// character class: a value that PASSES the corresponding validator is a
// genuine ID and renders byte-identically (including every existing
// dotted-child and legacy shape idvalidate accepts — see ADR-0042 and
// spec 120-trust-boundary-render-audit R1). A value that FAILS
// validation — whether it is control-byte-hostile or merely
// printable-malformed — is forced through strconv.Quote, which visibly
// marks it as NOT a bare identifier and, as a side effect, also closes
// the control-byte/line-forging class that termsafe.Escape exists for.
//
// The empty string is treated as identity rather than routed through
// the validators: it is the legitimate "no ID" sentinel used at several
// call sites (e.g. spec-mode state lines rendering "spec=" with no
// value), and idvalidate.SpecID/BeadID both reject the empty string as
// invalid. Quoting it would turn today's "spec=" into "spec=\"\"" for a
// case that was never malformed — see spec 120 AC-24's empty-sentinel
// discipline.
package idrender

import (
	"strconv"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

// Spec renders a spec ID for display. If id is empty, or passes
// idvalidate.SpecID, it is returned byte-identically. Otherwise id is
// forced through strconv.Quote so a malformed-but-printable value can
// never be mistaken for — or forge output around — a genuine ID.
func Spec(id string) string {
	if id == "" {
		return id
	}
	if err := idvalidate.SpecID(id); err != nil {
		return strconv.Quote(id)
	}
	return id
}

// Bead renders a bead ID for display. If id is empty, or passes
// idvalidate.BeadID, it is returned byte-identically. Otherwise id is
// forced through strconv.Quote so a malformed-but-printable value can
// never be mistaken for — or forge output around — a genuine ID.
func Bead(id string) string {
	if id == "" {
		return id
	}
	if err := idvalidate.BeadID(id); err != nil {
		return strconv.Quote(id)
	}
	return id
}
