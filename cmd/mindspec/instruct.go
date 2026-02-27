package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/guard"
	"github.com/mindspec/mindspec/internal/instruct"
	"github.com/mindspec/mindspec/internal/resolve"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/trace"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var instructCmd = &cobra.Command{
	Use:   "instruct",
	Short: "Emit agent instructions for the current mode and active work",
	Long: `Derives mode from the target spec's molecule state (ADR-0015) and emits
mode-appropriate operating guidance for agent consumption.

If --spec is omitted and exactly one active spec exists, it is auto-selected.
If multiple active specs exist, the command fails with a list of candidates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		specFlag, _ := cmd.Flags().GetString("spec")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		// CWD redirect: if running from main with an active worktree,
		// emit ONLY the redirect message — no normal guidance.
		if wtPath := guard.ActiveWorktreePath(root); wtPath != "" && guard.IsMainCWD(root) {
			msg := fmt.Sprintf("# MindSpec — CWD Redirect\n\nYou are in the main worktree. Run:\n\n  cd %s\n\nThen run `mindspec instruct` for mode-appropriate guidance.\n", wtPath)
			if format == "json" {
				fmt.Printf(`{"redirect":true,"worktree_path":%q,"message":"Switch to worktree"}`, wtPath)
				fmt.Println()
			} else {
				fmt.Print(msg)
			}
			return nil
		}

		// Resolve target spec (ADR-0015 targeting rules)
		specID, resolveErr := resolve.ResolveTarget(root, specFlag)

		// Build state for instruct.BuildContext
		var s *state.State
		if resolveErr != nil {
			// If ambiguous, surface the error directly
			if _, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
				return resolveErr
			}
			// Other errors: fall back to mode-cache
			mc, mcErr := state.ReadModeCache(root)
			if mcErr != nil {
				return handleNoState(root, format)
			}
			s = &state.State{
				Mode:       mc.Mode,
				ActiveSpec: mc.ActiveSpec,
				ActiveBead: mc.ActiveBead,
			}
		} else {
			// Derive mode from molecule
			mode, modeErr := resolve.ResolveMode(root, specID)
			mc, _ := state.ReadModeCache(root)
			if modeErr != nil {
				// Fallback: use mode-cache mode but resolved specID
				if mc != nil {
					s = &state.State{
						Mode:       mc.Mode,
						ActiveSpec: specID,
						ActiveBead: mc.ActiveBead,
					}
				} else {
					s = &state.State{
						Mode:       state.ModeIdle,
						ActiveSpec: specID,
					}
				}
			} else {
				activeBead := ""
				if mc != nil {
					activeBead = mc.ActiveBead
				}
				s = &state.State{
					Mode:       mode,
					ActiveSpec: specID,
					ActiveBead: activeBead,
				}
			}
		}

		ctx := instruct.BuildContext(root, s)

		// Add worktree check when an active worktree is set.
		mc, _ := state.ReadModeCache(root)
		if mc != nil && mc.ActiveWorktree != "" {
			if warning := instruct.CheckWorktree(mc.ActiveWorktree); warning != "" {
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

// handleNoState provides a graceful fallback when no state exists.
func handleNoState(root, format string) error {
	s := &state.State{Mode: state.ModeIdle}
	ctx := instruct.BuildContext(root, s)
	ctx.Warnings = append(ctx.Warnings,
		"[state] No active state found. Run `mindspec spec-init` to start a new spec.",
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
	instructCmd.Flags().String("spec", "", "Target spec ID (auto-detected if exactly one active spec)")
}
