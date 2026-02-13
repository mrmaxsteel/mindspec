package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/instruct"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/trace"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var instructCmd = &cobra.Command{
	Use:   "instruct",
	Short: "Emit agent instructions for the current mode and active work",
	Long:  `Reads .mindspec/state.json and emits mode-appropriate operating guidance for agent consumption.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		s, err := state.Read(root)
		if err != nil {
			if err == state.ErrNoState {
				return handleNoState(root, format)
			}
			return err
		}

		ctx := instruct.BuildContext(root, s)

		// Add worktree check for implement mode
		if s.Mode == state.ModeImplement {
			if warning := instruct.CheckWorktree(s.ActiveBead); warning != "" {
				ctx.Warnings = append(ctx.Warnings, "[worktree] "+warning)
			}
		}

		if format == "json" {
			output, err := instruct.RenderJSON(ctx)
			if err != nil {
				return err
			}
			fmt.Println(output)
			return nil
		}

		output, err := instruct.Render(ctx)
		if err != nil {
			return err
		}
		trace.Emit(trace.NewEvent("instruct.render").
			WithSpec(s.ActiveSpec).
			WithTokens(trace.EstimateTokens(output)).
			WithData(map[string]any{
				"tokens_total": trace.EstimateTokens(output),
				"mode":         s.Mode,
				"template":     s.Mode + ".md",
			}))
		fmt.Print(output)
		return nil
	},
}

// handleNoState provides a graceful fallback when state.json is missing.
func handleNoState(root, format string) error {
	// Fall back to idle mode with a warning
	s := &state.State{Mode: state.ModeIdle}
	ctx := instruct.BuildContext(root, s)
	ctx.Warnings = append(ctx.Warnings,
		"[state] No .mindspec/state.json found. Run `mindspec state set --mode=<mode> --spec=<id>` to initialize.",
	)

	if format == "json" {
		output, err := instruct.RenderJSON(ctx)
		if err != nil {
			return err
		}
		fmt.Println(output)
		return nil
	}

	output, err := instruct.Render(ctx)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func init() {
	instructCmd.Flags().String("format", "", "Output format: markdown (default) or json")
}
