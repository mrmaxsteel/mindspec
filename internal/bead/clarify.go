// Package bead. clarify.go implements spec 124 (impl-readiness-gate)
// Bead 3 / R8's append-only readiness-attempt record: the ONE-write-
// per-bead carrier under the dedicated bead.MetaKeyReadinessAttempt
// metadata key (readiness.go) that no mechanical MF-1..MF-4 signal ever
// scans (the layer boundary, AC-12, pinned by Bead 1's own seeded-record
// invariance tests).
//
// WriteAttemptRecord is the ONLY writer of this key — there is no
// update/finalize API (R8e derive-don't-write): the terminal
// READY/escalated disposition is DERIVED from whether a subsequent
// re-dispatch succeeds, never a second write here.
package bead

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
)

// ReportEntry is one ordinal-keyed reason copied verbatim from the
// original Phase-0 `NOT READY: <bead-id>` report (spec 124 R8b): the
// subagent's SR-tagged, ordinal-numbered, span-quoting reason. The
// attempt record carries the FULL original report so a later reader
// (the dispatch ingress, a human auditor, `bd show`) never needs the
// long-gone subagent transcript to know what each clarification answers.
type ReportEntry struct {
	Ordinal int    `json:"ordinal"`
	Signal  string `json:"signal"`
	Reason  string `json:"reason"`
}

// ClarificationEntry is one grounded answer to a cited report ordinal
// (spec 124 R8b): a concrete answer plus the authoritative source span
// it cites (in spec.md / plan.md / landed code). WriteAttemptRecord
// checks only that Span is non-empty — a PRESENCE check. Whether the
// cited span actually SUPPORTS the answer is the fresh Phase-0
// subagent's semantic judgment on re-dispatch (ADR-0040: the binary
// validates structure, the model judges meaning) — never verified here.
type ClarificationEntry struct {
	Ordinal int    `json:"ordinal"`
	Reason  string `json:"reason"`
	Answer  string `json:"answer"`
	Span    string `json:"span"`
}

// AttemptRecord is the append-only readiness-attempt record (spec 124
// R8b/e): the original ordinal-keyed NOT-READY report plus the grounded
// clarification entries. Written EXACTLY ONCE per bead by
// WriteAttemptRecord — the categorical, restart-proof, per-bead cap
// (R8d): the presence of ANY prior attempt record on a bead means the
// single clarification round is already consumed.
type AttemptRecord struct {
	Report         []ReportEntry        `json:"report"`
	Clarifications []ClarificationEntry `json:"clarifications"`
}

// WriteAttemptRecord validates and writes beadID's readiness-attempt
// record via the existing MergeMetadata helper, under the dedicated
// MetaKeyReadinessAttempt key — exactly once per bead, ever.
//
// It REFUSES (fail-closed, zero write) when:
//   - beadID is malformed (idvalidate.BeadID)
//   - beadID ALREADY carries a MetaKeyReadinessAttempt record — the
//     categorical, restart-proof, per-bead cap (R8d): the presence of
//     ANY prior attempt record forces escalation to plan/spec revision,
//     never a second `bead clarify`
//   - the record's own Report is empty (nothing to clarify against), or
//     carries a non-positive or duplicate ordinal
//   - any Clarifications entry cites an ordinal absent from Report
//   - any Clarifications entry carries an empty/whitespace-only Span
//     (a PRESENCE check only — R8b: whether the span SUPPORTS the
//     answer is the fresh Phase-0 subagent's judgment, never verified
//     here)
//
// On success it performs exactly ONE MergeMetadata write. There is no
// update/finalize surface: the terminal disposition (READY-after-
// clarification vs escalated-to-revision) is DERIVED from the
// subsequent re-dispatch outcome, never a second write (R8e).
//
// THREAT BOUNDARY of the per-bead cap (FX-3 / codex-G1). The cap
// prevents the ACCIDENTAL, naive unbounded clarify↔re-dispatch loop —
// an orchestrator that keeps re-answering the same NOT READY without
// escalating — and it SURVIVES orchestrator restart / context loss
// because the marker is durable bd state, not transcript memory. It does
// NOT defend against a deliberate out-of-band DELETION of the
// MetaKeyReadinessAttempt record (e.g. a manual `bd update --metadata`
// stripping it): that is a conscious operator reset — analogous to
// `--allow-not-ready` for the mechanical floor — which also DESTROYS the
// append-only audit trail (and is therefore detectable in the record's
// absence), and is out of the cap's guarantee by design.
//
// SINGLE-WRITER assumption (FX-4 / codex-G2, spec 124 R8d). The
// existence check below (GetMetadata) and the write (MergeMetadata) are
// check-then-act — NOT atomic against a second concurrent writer racing
// the same bead's metadata. This is sound because spec 124 assumes a
// SINGLE writer per bead: `/ms-bead-cycle` is serial per bead by design
// ("Don't claim multiple beads at once"), so exactly one orchestrator
// ever drives one bead's clarify at a time. Two concurrent orchestrators
// racing MergeMetadata on ONE bead (an ordinary lost-update window) is
// explicitly out of scope for the cap's guarantee and is not defended
// against here; the serial-per-bead cycle is the enforced invariant that
// makes the single-writer assumption hold in practice.
func WriteAttemptRecord(beadID string, record AttemptRecord) error {
	// Class-2 consumer boundary (ADR-0042 §1): beadID feeds a `bd show`/
	// `bd update` argv build via GetMetadata/MergeMetadata below —
	// validated BEFORE any bd spawn.
	if err := idvalidate.BeadID(beadID); err != nil {
		return fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}

	if err := validateAttemptRecord(record); err != nil {
		return err
	}

	existing, err := GetMetadata(beadID)
	if err != nil {
		return fmt.Errorf("reading existing metadata for %s: %w", idrender.Bead(beadID), err)
	}
	if _, ok := existing[MetaKeyReadinessAttempt]; ok {
		return fmt.Errorf(
			"bead %s already carries a readiness-attempt record — the one-round-per-bead clarification cap is consumed",
			idrender.Bead(beadID))
	}

	return MergeMetadata(beadID, map[string]interface{}{
		MetaKeyReadinessAttempt: record,
	})
}

// validateAttemptRecord is the record-shape half of WriteAttemptRecord's
// refusal contract (spec 124 R8b/d), factored out so it is directly
// unit-testable without a bd round-trip.
func validateAttemptRecord(record AttemptRecord) error {
	if len(record.Report) == 0 {
		return fmt.Errorf("the attempt record's report is empty — nothing to clarify against")
	}
	ordinals := map[int]bool{}
	for _, e := range record.Report {
		if e.Ordinal <= 0 {
			return fmt.Errorf("report entry has a non-positive ordinal: %d", e.Ordinal)
		}
		if ordinals[e.Ordinal] {
			return fmt.Errorf("report entry ordinal %d appears more than once", e.Ordinal)
		}
		ordinals[e.Ordinal] = true
	}
	for _, c := range record.Clarifications {
		if !ordinals[c.Ordinal] {
			return fmt.Errorf("clarification cites ordinal %d, which is absent from the recorded report", c.Ordinal)
		}
		if strings.TrimSpace(c.Span) == "" {
			return fmt.Errorf("clarification for ordinal %d carries no source span — every clarification must cite an authoritative span (spec 124 R8b)", c.Ordinal)
		}
	}
	return nil
}
