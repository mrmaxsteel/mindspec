package main

import (
	"fmt"
	"time"

	"github.com/mindspec/mindspec/internal/contextpack"
	"github.com/mindspec/mindspec/internal/trace"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Context pack generation commands",
	Long:  `Assemble DDD-informed context packs for agent sessions.`,
}

var contextPackMode string

var contextPackCmd = &cobra.Command{
	Use:   "pack <spec-id>",
	Short: "Generate a context pack for a spec",
	Long: `Generate a context-pack.md file bundling domain docs, ADRs, policies,
and provenance for the specified spec. Content varies by --mode.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		specID := args[0]
		mode := contextPackMode

		// Validate mode
		switch mode {
		case contextpack.ModeSpec, contextpack.ModePlan, contextpack.ModeImplement:
			// valid
		default:
			return fmt.Errorf("invalid mode %q: must be spec, plan, or implement", mode)
		}

		buildStart := time.Now()
		pack, err := contextpack.Build(root, specID, mode)
		if err != nil {
			return fmt.Errorf("building context pack: %w", err)
		}

		// Emit trace with per-section token breakdown
		sectionTokens := make(map[string]any)
		tokensTotal := 0
		rendered := pack.Render()
		for _, s := range pack.Sections {
			t := trace.EstimateTokens(s.Content)
			sectionTokens[s.Heading] = t
			tokensTotal += t
		}
		trace.Emit(trace.NewEvent("contextpack.build").
			WithSpec(specID).
			WithDuration(time.Since(buildStart)).
			WithTokens(trace.EstimateTokens(rendered)).
			WithData(map[string]any{
				"tokens_total":  tokensTotal,
				"sections":      sectionTokens,
				"section_count": len(pack.Sections),
				"mode":          mode,
			}))

		if err := pack.WriteToFile(root, specID); err != nil {
			return fmt.Errorf("writing context pack: %w", err)
		}

		fmt.Printf("Context pack generated: docs/specs/%s/context-pack.md (mode=%s)\n", specID, mode)
		return nil
	},
}

func init() {
	contextPackCmd.Flags().StringVar(&contextPackMode, "mode", "spec", "content tier: spec, plan, or implement")
	contextCmd.AddCommand(contextPackCmd)
}
