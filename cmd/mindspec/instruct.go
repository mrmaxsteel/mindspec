package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/spf13/cobra"
)

var instructCmd = &cobra.Command{
	Use:   "instruct",
	Short: "Emit agent instructions for the current mode and active work",
	Long: `Derives mode from beads state (ADR-0023) and emits mode-appropriate operating
guidance for agent consumption.

If --spec is omitted and exactly one active spec exists, it is auto-selected.
If multiple active specs exist, the command fails with a list of candidates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		specFlag, _ := cmd.Flags().GetString("spec")
		panelState, _ := cmd.Flags().GetBool("panel-state")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		return instruct.RunWithOptions(context.Background(), cwd, format, specFlag, os.Stdout, instruct.Options{
			PanelState: panelState,
		})
	},
}

func init() {
	instructCmd.Flags().String("format", "", "Output format: markdown (default) or json")
	instructCmd.Flags().String("spec", "", "Target spec ID (auto-detected if exactly one active spec)")
	instructCmd.Flags().Bool("panel-state", false, "Append the open-panel-rounds block: per-panel tally, N-1 threshold, and reviewed-HEAD freshness vs the mindspec complete gate")
}
