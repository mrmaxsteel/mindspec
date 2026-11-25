package main

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// readinessReviewSignals are the five semantic signals that the agent
// reviews before it edits a file.
var readinessReviewSignals = []string{
	"SR-1: the plan steps and acceptance criteria are implementable without inventing unstated behavior.",
	"SR-2: each claimed acceptance criterion is decidable: a concrete pass/fail check exists or is directly constructible.",
	"SR-3: each helper that this brief says to reuse exists at the stated import path with the stated shape.",
	"SR-4: the work item does not contradict the spec or the landed work of a sibling item.",
	"SR-5: no ambiguity remains that forces a choice between materially different implementations.",
}

// renderReadinessReview returns the readiness review that `next <w>
// --emit-only` prints after the brief. It reads the stored readiness
// attempt of the item and refuses when that record is malformed. A
// present key is always parsed, so a null or non-object value refuses
// as malformed; only an absent key means a first dispatch. Store
// text renders through termsafe.Escape on lines that start with
// "- Ordinal", so no stored text can start a line or pass as a header.
func renderReadinessReview(info next.BeadInfo) (string, error) {
	var record *bead.AttemptRecord
	if raw, ok := info.Metadata[bead.MetaKeyReadinessAttempt]; ok {
		parsed, err := bead.ParseAttemptRecord(raw)
		if err != nil {
			return "", guard.NewFailure(
				fmt.Sprintf("the %s metadata of %s is malformed: %s; no brief was printed", bead.MetaKeyReadinessAttempt, idrender.Bead(info.ID), err.Error()),
				"bd show <bead-id> --json   (the operator repairs or removes the record, then re-run)")
		}
		record = &parsed
	}
	id := idrender.Bead(info.ID)
	var sb strings.Builder
	sb.WriteString("\n## Readiness review (before any edit)\n\n")
	sb.WriteString("Before you edit a file, examine these five signals against the spec and plan text above:\n\n")
	for _, line := range readinessReviewSignals {
		sb.WriteString("- " + line + "\n")
	}
	sb.WriteString("\nIf all five pass, print exactly one line, `Phase 0: READY`, and implement. Make zero commits until the review passes.\n\n")
	fmt.Fprintf(&sb, "If a signal fails, make zero commits. Return a report whose first line is exactly `NOT READY: %s`. Then give the reasons, numbered by ordinal (1, 2, ...; each unique). Tag each reason with its signal (SR-1 to SR-5), quote the span that is wrong or missing verbatim, and ask the question that unblocks it.\n\n", id)
	sb.WriteString("Anti-browbeat rule: each clarification below pairs one original reason with its answer. Judge whether the answer resolves that reason against the cited span. A bare \"it is ready\" never resolves a reason, and an answer never permits new normative behavior. Report NOT READY again for each reason that has no grounded clarification or whose clarification does not resolve it.\n\n")
	sb.WriteString("Independent re-derivation rule: do your own SR-1 to SR-5 review first, as on a first dispatch, and only then read the clarifications. The stored record is supporting evidence, not a closed list: it can omit reasons. Report NOT READY for each unresolved concern that you find, also a concern that the record does not name.\n\n")
	sb.WriteString("### Readiness clarifications\n\n")
	if record == nil {
		sb.WriteString("(none: this is a first dispatch)\n")
	} else {
		for _, entry := range record.Report {
			fmt.Fprintf(&sb, "- Ordinal %d [%s]: reason: %s -- clarification: ", entry.Ordinal, entry.Signal, termsafe.Escape(entry.Reason))
			if c, ok := record.Clarification(entry.Ordinal); ok {
				fmt.Fprintf(&sb, "%s (span: %s)\n", termsafe.Escape(c.Answer), termsafe.Escape(c.Span))
			} else {
				sb.WriteString("(none recorded)\n")
			}
		}
	}
	fmt.Fprintf(&sb, "\nBefore you return, run `mindspec complete %s --check`. Return only when it passes, or return `NOT READY: %s` with zero commits.\n", id, id)
	return sb.String(), nil
}
