package main

// bead_ready.go is a NEW file with its OWN init() registering on the
// existing beadCmd family (spec 124 plan preamble: `cmd/mindspec/bead.go`
// is edited by NO bead — each new verb ships as its own file with its own
// init(), the cobra pattern bead.go itself uses).

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
	"github.com/spf13/cobra"
)

var readyCheckEvaluateFn = readiness.EvaluateReadiness
var readyCheckReadMarkerFn = bead.ReadOverrideMarker
var readyCheckBeadFn = next.FetchBeadByID

var beadReadyCheckCmd = &cobra.Command{
	Use:   "ready-check <bead-id>",
	Short: "Evaluate a bead's mechanical readiness floor (read-only)",
	Long: `Evaluates the four mechanical readiness signals (MF-1..MF-4)
for a bead: its plan section is concrete-by-structure; its claimed R/AC
tokens resolve in spec.md; its declared dependencies are closed AND
landed-merged; and it carries no genuine blocking marker.

This is a pure read: no bd write, no git write, no file write on any
path. Exit 0 when all four signals pass or every failure is covered by a
validated schema-v2 override marker. Otherwise exit non-zero, naming the
uncovered signals and the release/re-claim recovery.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]

		// R3-shaped explicit-ingress gate (ADR-0042): a malformed beadID
		// refuses HERE, before any composition or evaluation.
		if err := idvalidate.BeadID(beadID); err != nil {
			return guard.NewFailure(
				fmt.Sprintf("%s is not a valid bead ID: %v", termsafe.Escape(beadID), err),
				"bd ready   (pick a listed bead ID and re-run)",
			)
		}

		root, err := findRoot()
		if err != nil {
			return err
		}

		report, err := readyCheckEvaluateFn(root, beadID)
		if err != nil {
			return err
		}

		inspectRecovery := fmt.Sprintf("bd show %s --json", idrender.Bead(beadID))
		marker, validity, err := readyCheckReadMarkerFn(beadID)
		if err != nil {
			return guard.NewFailure(fmt.Sprintf("could not read readiness override metadata for %s: %s", idrender.Bead(beadID), termsafe.Escape(err.Error())),
				inspectRecovery)
		}
		coverage := readiness.ApplyOverride(report, marker, validity)
		if len(coverage.Uncovered) == 0 {
			fmt.Println(coverage.Message)
			return nil
		}
		msg := coverage.Message + "\nuncovered: " + strings.Join(coverage.Uncovered, ",")
		commands := report.RecoveryCommands()
		if indeterminate := next.IndeterminateReadiness(report); len(indeterminate) > 0 {
			return guard.NewFailure(msg+"\n"+strings.Join(indeterminate, "\n"), commands...)
		}
		info, err := readyCheckBeadFn(beadID)
		if err != nil {
			return guard.NewFailure(fmt.Sprintf("could not read claim status for %s: %s", idrender.Bead(beadID), termsafe.Escape(err.Error())), inspectRecovery)
		}
		if strings.EqualFold(info.Status, "in_progress") {
			commands = append(commands, fmt.Sprintf("mindspec release %s", idrender.Bead(beadID)))
		}
		commands = append(commands, fmt.Sprintf("mindspec next %s --allow-not-ready=%s --evidence <path-to-justification>", idrender.Bead(beadID), strings.Join(coverage.Failing, ",")))
		return guard.NewFailure(msg, commands...)
	},
}

func init() {
	beadCmd.AddCommand(beadReadyCheckCmd)
}
