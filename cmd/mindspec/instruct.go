package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/guard"
	"github.com/mindspec/mindspec/internal/instruct"
	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/resolve"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/trace"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var instructCmd = &cobra.Command{
	Use:   "instruct",
	Short: "Emit agent instructions for the current mode and active work",
	Long: `Derives mode from lifecycle.yaml and emits mode-appropriate operating
guidance for agent consumption.

If --spec is omitted and exactly one active spec exists, it is auto-selected.
If multiple active specs exist, the command fails with a list of candidates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		specFlag, _ := cmd.Flags().GetString("spec")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		localRoot, err := workspace.FindLocalRoot(cwd)
		if err != nil {
			return err
		}
		// mainRoot resolves worktrees back to the main repo (for guard, spec lookup).
		mainRoot, _ := workspace.FindRoot(cwd)
		if mainRoot == "" {
			mainRoot = localRoot
		}

		// CWD redirect: if running from main with an active worktree,
		// emit ONLY the redirect message — no normal guidance.
		if wtPath := guard.ActiveWorktreePath(mainRoot); wtPath != "" && guard.IsMainCWD(mainRoot) {
			msg := fmt.Sprintf("# MindSpec — CWD Redirect\n\nYou are in the main worktree. Run:\n\n  cd %s\n\nThen run `mindspec instruct` for mode-appropriate guidance.\n", wtPath)
			if format == "json" {
				fmt.Printf(`{"redirect":true,"worktree_path":%q,"message":"Switch to worktree"}`, wtPath)
				fmt.Println()
			} else {
				fmt.Print(msg)
			}
			return nil
		}

		// Protected branch check: main/master → always idle (focus file is stale).
		if specFlag == "" {
			branch, _ := gitops.CurrentBranch()
			cfg, cfgErr := config.Load(mainRoot)
			if cfgErr != nil {
				cfg = config.DefaultConfig()
			}
			if branch != "" && cfg.IsProtectedBranch(branch) {
				return handleNoState(mainRoot, format)
			}
		}

		// ADR-0023: derive state from beads, not focus files.
		// First try resolver for spec targeting, then use phase context.
		specID, resolveErr := resolve.ResolveTarget(mainRoot, specFlag)

		var mc *state.Focus
		if resolveErr != nil {
			if ambErr, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
				return handleAmbiguous(mainRoot, format, ambErr)
			}
			// Try phase context for beads-derived state.
			ctx, ctxErr := phase.ResolveContext(mainRoot)
			if ctxErr != nil || ctx == nil || ctx.Phase == "" {
				return handleNoState(mainRoot, format)
			}
			mc = &state.Focus{
				Mode:       ctx.Phase,
				ActiveSpec: ctx.SpecID,
				ActiveBead: ctx.BeadID,
			}
		} else {
			// Derive mode from beads
			mode, _ := resolve.ResolveMode(mainRoot, specID)
			// Try to find active bead via phase context
			ctx, _ := phase.ResolveContextFromDir(mainRoot, localRoot)
			activeBead := ""
			if ctx != nil {
				activeBead = ctx.BeadID
			}
			mc = &state.Focus{
				Mode:       mode,
				ActiveSpec: specID,
				ActiveBead: activeBead,
			}
		}

		// ADR-0023: ActiveWorktree is no longer stored in focus files.
		// Resolve it from git worktree list if we have an active bead.
		if mc.ActiveBead != "" && mc.ActiveWorktree == "" {
			mc.ActiveWorktree = resolveBeadWorktree(mc.ActiveBead)
		}

		ctx := instruct.BuildContext(mainRoot, mc)

		// Add worktree check when an active worktree is set.
		if mc.ActiveWorktree != "" {
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
			WithSpec(mc.ActiveSpec).
			WithTokens(trace.EstimateTokens(output)).
			WithData(map[string]any{
				"tokens_total": trace.EstimateTokens(output),
				"mode":         mc.Mode,
				"template":     mc.Mode + ".md",
			}))
		fmt.Print(output)
		return nil
	},
}

// handleNoState provides a graceful fallback when no state exists.
func handleNoState(root, format string) error {
	mc := &state.Focus{Mode: state.ModeIdle}
	ctx := instruct.BuildContext(root, mc)
	ctx.Warnings = append(ctx.Warnings,
		"[state] No active state found. Run `mindspec spec create <slug>` to start a new spec.",
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

// handleAmbiguous falls back to idle guidance with a warning listing the ambiguous specs.
func handleAmbiguous(root, format string, ambErr *resolve.ErrAmbiguousTarget) error {
	mc := &state.Focus{Mode: state.ModeIdle}
	ctx := instruct.BuildContext(root, mc)
	ctx.Warnings = append(ctx.Warnings,
		"[resolve] "+ambErr.Error()+"Use `mindspec instruct --spec <id>` to target a specific spec.",
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

// resolveBeadWorktree finds the worktree path for a bead by checking
// git worktree list for a matching bead branch or worktree name.
func resolveBeadWorktree(beadID string) string {
	entries, err := bead.WorktreeList()
	if err != nil {
		return ""
	}
	wtName := "worktree-" + beadID
	branchName := "bead/" + beadID
	for _, e := range entries {
		if e.Name == wtName || e.Branch == branchName {
			return e.Path
		}
	}
	return ""
}

func init() {
	instructCmd.Flags().String("format", "", "Output format: markdown (default) or json")
	instructCmd.Flags().String("spec", "", "Target spec ID (auto-detected if exactly one active spec)")
}
