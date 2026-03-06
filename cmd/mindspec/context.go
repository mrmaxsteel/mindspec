package main

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Context generation commands",
	Long:  `Generate bead-scoped context primers for agent sessions.`,
}

var contextBeadCmd = &cobra.Command{
	Use:   "bead <spec-id>",
	Short: "Generate a bead context primer",
	Long: `Generate a bead-scoped context primer to stdout.
Includes bead scope, spec requirements, plan work chunk, ADR decisions,
and domain overviews — everything an agent needs to start a bead.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		specID := args[0]
		beadID, _ := cmd.Flags().GetString("bead")
		if beadID == "" {
			return fmt.Errorf("--bead is required")
		}

		primer, err := contextpack.BuildBeadPrimer(root, specID, beadID)
		if err != nil {
			return fmt.Errorf("building bead primer: %w", err)
		}

		fmt.Print(contextpack.RenderBeadPrimer(primer))
		return nil
	},
}

func init() {
	contextBeadCmd.Flags().String("bead", "", "Bead ID to generate primer for (required)")
	contextCmd.AddCommand(contextBeadCmd)
}
