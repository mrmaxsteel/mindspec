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
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/spf13/cobra"
)

// repairMergeMetadataFn is the test seam for the Req 19 merge-write.
// It MUST stay bead.MergeMetadata (read-merge-write) in production —
// merge semantics are the whole point of the subcommand.
var repairMergeMetadataFn = bead.MergeMetadata

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

func init() {
	repairCmd.AddCommand(repairPhaseCmd)
}

func repairPhaseRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]

	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		msg := fmt.Sprintf("repair phase: no lifecycle epic found for spec %s", specID)
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
