package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/spf13/cobra"
)

var completeCmd = &cobra.Command{
	Use:   "complete <bead-id> [commit message...]",
	Short: "Close a bead, remove its worktree, and advance state",
	Long: `Orchestrates the full bead close-out:
  1. Auto-commits changes if a commit message is provided
  2. Validates all changes are committed (clean worktree)
  3. Closes the bead via bd close
  4. Removes the worktree via bd worktree remove
  5. Advances state (next bead, plan, or idle)

Usage:
  mindspec complete <bead-id> "describe what you did"    # auto-commit + close
  mindspec complete <bead-id>                            # close (tree must be clean)

The bead ID is required as the first argument.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		specID, _ := cmd.Flags().GetString("spec")

		// Resolve --spec prefix (e.g. "079" → "079-location-agnostic-commands")
		if specID != "" {
			resolved, err := resolve.ResolveSpecPrefix(specID)
			if err != nil {
				return fmt.Errorf("resolving --spec prefix: %w", err)
			}
			specID = resolved
		}

		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		// Parse args: first arg is always bead ID, remaining args are commit message.
		beadID := args[0]
		var commitMsg string
		if len(args) > 1 {
			commitMsg = strings.Join(args[1:], " ")
		}

		// Auto-chdir to spec worktree or main root before calling Run.
		if specID != "" {
			specWtPath := state.SpecWorktreePath(root, specID)
			if fi, err := os.Stat(specWtPath); err == nil && fi.IsDir() {
				os.Chdir(specWtPath)
			} else {
				os.Chdir(root)
			}
		}

		exec := newExecutor(root)
		result, err := complete.Run(root, beadID, specID, commitMsg, exec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		fmt.Print(complete.FormatResult(result))

		// Instruct-tail: emit guidance for the new mode
		fmt.Println() // separator between summary and guidance
		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}
		return nil
	},
}

func init() {
	completeCmd.Flags().String("spec", "", "Target spec ID when multiple specs are active")
}
