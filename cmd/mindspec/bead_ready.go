package main

// bead_ready.go is a NEW file with its OWN init() registering on the
// existing beadCmd family (spec 124 plan preamble: `cmd/mindspec/bead.go`
// is edited by NO bead — each new verb ships as its own file with its own
// init(), the cobra pattern bead.go itself uses).

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
	"github.com/spf13/cobra"
)

var beadReadyCheckCmd = &cobra.Command{
	Use:   "ready-check <bead-id>",
	Short: "Evaluate a bead's mechanical readiness floor (read-only)",
	Long: `Evaluates the four mechanical readiness signals (spec 124, MF-1..MF-4)
for a bead: its plan section is concrete-by-structure; its claimed R/AC
tokens resolve in spec.md; its declared dependencies are closed AND
landed-merged; and it carries no genuine blocking marker.

This is a pure read: no bd write, no git write, no file write on any
path. Exit 0 when all four signals pass; on any FAIL, exit non-zero
with one recovery: line per failing signal.`,
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

		report, err := readiness.EvaluateReadiness(root, beadID)
		if err != nil {
			return err
		}

		msg := readiness.Render(report)
		if report.AllPass() {
			fmt.Println(msg)
			return nil
		}
		return guard.NewFailure(msg, report.RecoveryCommands()...)
	},
}

func init() {
	beadCmd.AddCommand(beadReadyCheckCmd)
}
