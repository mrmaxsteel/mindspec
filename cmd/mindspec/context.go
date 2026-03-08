package main

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Context generation commands",
	Long:  `Generate bead-scoped context for agent sessions.`,
}

var contextBeadCmd = &cobra.Command{
	Use:   "bead <bead-id>",
	Short: "Generate bead context from bd show",
	Long: `Generate bead-scoped context to stdout.
Reads pre-populated fields from bd show --json (Spec 074).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]

		rendered, err := contextpack.RenderBeadContext(beadID)
		if err != nil {
			return fmt.Errorf("rendering bead context: %w", err)
		}

		fmt.Print(rendered)
		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextBeadCmd)
}
