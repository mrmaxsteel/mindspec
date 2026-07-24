package main

// bead_clarify.go is a NEW file with its OWN init() registering on the
// existing beadCmd family (spec 124 plan preamble: cmd/mindspec/bead.go
// is edited by NO bead — each new verb ships as its own file with its
// own init(), the cobra pattern bead.go itself uses).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

// maxClarifyRecordBytes caps the --file read (FX-2 / codex-G4): a
// readiness-attempt record is tiny (a handful of ordinal-keyed reasons +
// answers + spans), so 1 MiB is orders of magnitude of headroom while
// refusing an oversize file rather than OOM-ing the process on an 8 GiB
// input (an operator-local self-DoS).
const maxClarifyRecordBytes = 1 << 20 // 1 MiB

var beadClarifyCmd = &cobra.Command{
	Use:   "clarify <bead-id>",
	Short: "Record a grounded readiness clarification for a NOT-READY bead (spec 124 R8)",
	Long: `Writes the append-only readiness-attempt record for a bead: the
original ordinal-keyed Phase-0 "NOT READY: <bead-id>" report, plus one
grounded clarification entry per cited ordinal (each carrying a concrete
answer and an authoritative source span).

The record is a SINGLE JSON file (--file), never free prose on the
command line:

  {
    "report": [
      {"ordinal": 1, "signal": "SR-2", "reason": "..."}
    ],
    "clarifications": [
      {"ordinal": 1, "reason": "...", "answer": "...", "span": "spec.md §R7: \"...\""}
    ]
  }

This performs exactly ONE write per bead. The record's mere presence is
the categorical, restart-proof cap on the clarification loop (spec 124
R8d): once a bead carries an attempt record, this command refuses a
second write, regardless of whether the new attempt cites the same or
different reasons — escalate to plan/spec revision instead.

There is NO --finalize or update surface (R8e, derive-don't-write): the
terminal READY/escalated disposition is derived from whether the
subsequent re-dispatch succeeds, never a second write here.

This writes ONLY the dedicated bd metadata key the mechanical readiness
floor (MF-1..MF-4) never scans — a clarification can never flip a
mechanical PASS/FAIL (spec 124 R8e). The blunt bypass for a known-
acceptable mechanical FAIL is "mindspec next --allow-not-ready", not
this command.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]

		// R3-shaped explicit-ingress gate (ADR-0042): a malformed beadID
		// refuses HERE, before any file read or evaluation.
		if err := idvalidate.BeadID(beadID); err != nil {
			return guard.NewFailure(
				fmt.Sprintf("%s is not a valid bead ID: %v", termsafe.Escape(beadID), err),
				"bd ready   (pick a listed bead ID and re-run)",
			)
		}

		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return guard.NewFailure(
				"--file <record.json> is required: the clarification record (original report + clarification entries) is a single JSON file, never free prose on the command line",
				fmt.Sprintf("mindspec bead clarify %s --file <record.json>", idrender.Bead(beadID)),
			)
		}

		f, err := os.Open(filePath)
		if err != nil {
			return guard.NewFailure(
				fmt.Sprintf("could not read %s: %v", termsafe.Escape(filePath), err),
				"pass a valid --file path to a readiness-attempt record JSON file",
			)
		}
		defer f.Close()

		// FX-2 (codex-G4): cap the read so an oversize --file (a self-DoS
		// vector) is refused cleanly rather than read into memory whole.
		// Read one byte past the cap to distinguish "exactly at the cap"
		// from "over the cap".
		data, err := io.ReadAll(io.LimitReader(f, maxClarifyRecordBytes+1))
		if err != nil {
			return guard.NewFailure(
				fmt.Sprintf("could not read %s: %v", termsafe.Escape(filePath), err),
				"pass a valid --file path to a readiness-attempt record JSON file",
			)
		}
		if len(data) > maxClarifyRecordBytes {
			return guard.NewFailure(
				fmt.Sprintf("%s exceeds the %d-byte readiness-attempt record cap — a clarification record is tiny (ordinal-keyed reasons + answers + spans)", termsafe.Escape(filePath), maxClarifyRecordBytes),
				"shrink the record file to just the cited reasons + their clarifications and re-run",
			)
		}

		// FX-2 (codex-G4): DisallowUnknownFields so a typo'd/malformed
		// record (a misspelled "clarifcations", a stray field) is rejected
		// deterministically, not silently dropped and mis-parsed.
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		var record bead.AttemptRecord
		if err := dec.Decode(&record); err != nil {
			return guard.NewFailure(
				fmt.Sprintf("%s is not valid JSON for a readiness-attempt record: %v", termsafe.Escape(filePath), err),
				"fix the record file's JSON shape ({\"report\": [...], \"clarifications\": [...]}) and re-run",
			)
		}

		if err := bead.WriteAttemptRecord(beadID, record); err != nil {
			return guard.NewFailure(
				err.Error(),
				"escalate to plan/spec revision (the categorical per-bead clarification cap is consumed), or fix the record file and re-run",
			)
		}

		fmt.Printf("Readiness-attempt record written for %s.\n", idrender.Bead(beadID))
		return nil
	},
}

func init() {
	beadClarifyCmd.Flags().String("file", "", "Path to the readiness-attempt record JSON file (required)")
	beadCmd.AddCommand(beadClarifyCmd)
}
