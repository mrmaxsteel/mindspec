package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/cleanup"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup [spec-id]",
	Short: "Clean up worktree and branch after a spec lifecycle completes",
	Long: `Removes the spec worktree and branch after implementation approval.

If a PR was created, checks its status first:
  - merged: cleans up worktree and branch
  - open: refuses (merge the PR first, or use --force)
  - closed without merge: refuses (re-open or use --force)

Use --force to skip PR status checks and clean up unconditionally.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		root, err := findRoot()
		if err != nil {
			return err
		}

		exec := newExecutor(root)
		result, err := cleanup.Run(root, specID, force, exec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if result.WorktreeRemoved {
			fmt.Printf("Removed worktree for %s\n", specID)
		}
		if result.BranchDeleted {
			fmt.Printf("Deleted branch spec/%s\n", specID)
		}
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
		if !result.WorktreeRemoved && !result.BranchDeleted {
			fmt.Println("Nothing to clean up.")
		}

		return nil
	},
}

func init() {
	cleanupCmd.Flags().Bool("force", false, "Skip PR status checks and force cleanup")
}
