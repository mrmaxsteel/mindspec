package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/complete"
	"github.com/mindspec/mindspec/internal/guard"
	"github.com/spf13/cobra"
)

var completeCmd = &cobra.Command{
	Use:   "complete [bead-id]",
	Short: "Close a bead, remove its worktree, and advance state",
	Long: `Orchestrates the full bead close-out:
  1. Validates all changes are committed (clean worktree)
  2. Closes the bead via bd close
  3. Removes the worktree via bd worktree remove
  4. Advances state (next bead, plan, or idle)

The bead ID is auto-resolved from state if not provided.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		// CWD guard: prefer running from the bead worktree.
		// If we're in main but an active worktree exists, auto-chdir there.
		if guard.IsMainCWD(root) {
			if wtPath := guard.ActiveWorktreePath(root); wtPath != "" {
				if err := os.Chdir(wtPath); err != nil {
					return fmt.Errorf("could not switch to active worktree %s: %w", wtPath, err)
				}
				fmt.Fprintf(os.Stderr, "note: switched to worktree %s\n", wtPath)
			}
		}

		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		var beadID string
		if len(args) > 0 {
			beadID = args[0]
		}

		result, err := complete.Run(root, beadID)
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