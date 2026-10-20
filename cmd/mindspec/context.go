package main

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/tokenize"
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
Reads pre-populated fields from bd show --json.

When --max-tokens N is supplied (N >= 0), a token-budgeted
layout is emitted, with an approximate token counter. When --max-tokens
is NOT passed, the legacy layout is used to preserve byte-identical
output for callers that rely on it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]

		maxTokens, _ := cmd.Flags().GetInt("max-tokens")
		if maxTokens < 0 {
			return fmt.Errorf("--max-tokens must be >= 0")
		}
		if cmd.Flags().Changed("max-tokens") {
			// Spec 088 budgeted-bundle path. maxTokens == 0 is the
			// explicit "emit the new layout unbudgeted" opt-in.
			out, err := contextpack.BuildBead(beadID, maxTokens, tokenize.Approx{})
			if err != nil {
				return fmt.Errorf("building bead context: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(out))
			return nil
		}

		// Legacy path — preserve byte-identical output (HC-2).
		rendered, err := contextpack.RenderBeadContext(beadID)
		if err != nil {
			return fmt.Errorf("rendering bead context: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), rendered)
		return nil
	},
}

func init() {
	contextBeadCmd.Flags().Int("max-tokens", 0,
		"Budget for the rendered bundle in approx tokens (0 = unbudgeted; omit to preserve legacy output)")
	contextCmd.AddCommand(contextBeadCmd)
}
