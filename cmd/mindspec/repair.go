package main

// Spec 092 (agent-contract-hardening) Bead 3, Req 19:
// `mindspec repair phase <spec-id>` is the ONLY phase-metadata fix
// mindspec ever tells a user or agent to run. Raw
// `bd update <id> --metadata '{...}'` REPLACES the entire metadata map
// (silently wiping mindspec_migrated_at, doc-skew audit keys, and
// ADR-override keys when pasted), so it is banned from all emitted
// output (HC-5); every recovery line that needs a phase fix emits this
// subcommand instead.

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

// repairMergeMetadataFn is the test seam for the Req 19 merge-write.
// It MUST stay bead.MergeMetadata (read-merge-write) in production —
// merge semantics are the whole point of the subcommand.
var repairMergeMetadataFn = bead.MergeMetadata

// repairGetMetadataFn is the test seam `repair spec-title` uses to read
// the epic's existing spec_num metadata before composing the checked
// SpecIDFromMetadata derivation. Defaults to bead.GetMetadata.
var repairGetMetadataFn = bead.GetMetadata

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair mindspec lifecycle state",
}

var repairPhaseCmd = &cobra.Command{
	Use:   "phase <spec-id>",
	Short: "Re-derive the spec's lifecycle phase and merge-write it to the epic",
	Long: `Re-derives the lifecycle phase for the given spec from its child bead
statuses (bead statuses are the ground truth — ADR-0023 §3/§5; the
epic's mindspec_phase metadata is a cache of that derivation, see the
ADR-0034 amendment) and writes the derived value back to the epic's
mindspec_phase metadata.

The write is a MERGE (read-merge-write, the same path lifecycle
commands use): unrelated metadata keys such as mindspec_migrated_at,
doc-skew audit keys, and ADR-override keys are preserved. Never repair
the phase with a raw bd metadata update — that path REPLACES the
entire metadata map and silently wipes every unrelated key.

Use this when a lifecycle gate or warning reports that the stored
phase disagrees with the child-derived phase and the bead states are
already correct.`,
	Args: cobra.ExactArgs(1),
	RunE: repairPhaseRunE,
}

var repairSpecTitleCmd = &cobra.Command{
	Use:   "spec-title <epic-id> <title>",
	Short: "Repair a spec epic's spec_title metadata (the D1 malformed-metadata lever)",
	Long: `Merge-writes a corrected spec_title onto the given epic's metadata (Spec
120 R3/AC-8) — the convergent recovery lever every D1 malformed-lineage
refusal names (see "mindspec repair spec-title <epic-id> <title>" in
those refusal messages).

The write is a MERGE (bead.MergeMetadata, the same HC-5 discipline as
'mindspec repair phase'): unrelated metadata keys are preserved, and a
raw 'bd update --metadata' replace is never emitted.

Both arguments are validated before any bd MUTATION: <epic-id> is gated
as a well-formed bead ID before any bd invocation, and the replacement
<title> must slugify (via the same derivation 'mindspec next'/'mindspec
complete' use) to a well-formed spec ID — that slug convergence check
runs after the epic's spec_num is read but before the metadata merge, so
a title whose slug still fails validation is refused with no write
attempted, and the lever is guaranteed to converge once applied.`,
	Args: cobra.ExactArgs(2),
	RunE: repairSpecTitleRunE,
}

func init() {
	repairCmd.AddCommand(repairPhaseCmd)
	repairCmd.AddCommand(repairSpecTitleCmd)
}

// repairSpecTitleRunE implements `mindspec repair spec-title <epic-id>
// <title>` (spec 120 R3, AC-8). ADR-0035 discipline: every refusal below
// carries a single convergent lever, the hostile value escaped/quoted
// only, and a final recovery line.
func repairSpecTitleRunE(cmd *cobra.Command, args []string) error {
	epicID := args[0]
	newTitle := args[1]

	// Own-arg gate (round-3 O3): validate epicID BEFORE any bd argv
	// embed — a malformed epic ID never reaches a bd spawn.
	if err := idvalidate.BeadID(epicID); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("%s is not a valid epic ID: %v", termsafe.Escape(epicID), err),
			"bd ready   (pick a listed epic ID and re-run)",
		)
	}

	// Read the epic's existing spec_num so the checked SpecIDFromMetadata
	// derivation (D1) can be evaluated against the SAME number the
	// corrected title would pair with — this verb repairs spec_title
	// only, never spec_num.
	meta, err := repairGetMetadataFn(epicID)
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: reading epic %s metadata failed: %v", epicID, err),
			fmt.Sprintf("bd show %s", epicID),
		)
	}
	var specNum int
	switch v := meta["spec_num"].(type) {
	case float64:
		specNum = int(v)
	case int:
		specNum = v
	}
	if specNum <= 0 {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: epic %s has no spec_num metadata to pair with the new title", epicID),
			fmt.Sprintf("bd show %s", epicID),
		)
	}

	// Refuse a replacement title whose slug still fails the corrected
	// idvalidate.SpecID — the whole point of the lever is to converge,
	// so a still-hostile title is rejected before any write.
	newSpecID, err := phase.SpecIDFromMetadata(specNum, newTitle)
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: replacement title %s slugifies to an invalid spec ID: %v", termsafe.Escape(newTitle), err),
			fmt.Sprintf("choose a title whose slug is alphanumeric/hyphen only, then re-run: mindspec repair spec-title %s \"<corrected-title>\"", epicID),
		)
	}

	// HC-5-safe merge-write: preserves every unrelated metadata key
	// (mindspec_phase, mindspec_migrated_at, doc-skew/ADR-override audit
	// keys, spec_num) — never a raw `bd update --metadata` replace.
	if err := repairMergeMetadataFn(epicID, map[string]interface{}{
		"spec_title": newTitle,
	}); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: writing spec_title to epic %s failed: %v", epicID, err),
			fmt.Sprintf("mindspec repair spec-title %s %q", epicID, newTitle),
		)
	}

	fmt.Fprintf(os.Stderr, "event=lifecycle.spec_title_repaired epic=%s spec_id=%s\n", epicID, newSpecID)
	fmt.Printf("spec_title for epic %s repaired to %q (derives spec id %s; unrelated metadata keys preserved).\n",
		epicID, newTitle, newSpecID)
	return nil
}

func repairPhaseRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]

	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		msg := fmt.Sprintf("repair phase: no lifecycle epic found for spec %s", idrender.Spec(specID))
		if err != nil {
			msg += fmt.Sprintf(": %v", err)
		}
		return guard.NewFailure(msg, "mindspec spec list")
	}

	detail, err := phase.DerivePhaseDetail(epicID)
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair phase: deriving phase for spec %s (epic %s): %v", specID, epicID, err),
			fmt.Sprintf("bd show %s", epicID),
		)
	}
	if detail.Derived == "" {
		return guard.NewFailure(
			fmt.Sprintf("repair phase: could not derive a phase for spec %s from epic %s children", specID, epicID),
			fmt.Sprintf("bd show %s", epicID),
		)
	}

	// Req 19: merge-write via bead.MergeMetadata — preserves every
	// unrelated metadata key on the epic.
	if err := repairMergeMetadataFn(epicID, map[string]interface{}{
		"mindspec_phase": detail.Derived,
	}); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair phase: writing mindspec_phase=%s to epic %s failed: %v", detail.Derived, epicID, err),
			fmt.Sprintf("mindspec repair phase %s", specID),
		)
	}

	fmt.Fprintf(os.Stderr, "event=lifecycle.phase_repaired spec=%s epic=%s stored=%s derived=%s\n",
		specID, epicID, detail.Stored, detail.Derived)
	if detail.Stored == detail.Derived {
		fmt.Printf("Phase for %s already consistent: %s (merge-write refreshed; unrelated metadata keys preserved).\n",
			specID, detail.Derived)
	} else {
		fmt.Printf("Phase for %s repaired: %q -> %q (epic %s; unrelated metadata keys preserved).\n",
			specID, detail.Stored, detail.Derived, epicID)
	}
	return nil
}
