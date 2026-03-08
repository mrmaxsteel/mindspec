package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/spf13/cobra"
)

var implCmd = &cobra.Command{
	Use:   "impl",
	Short: "Implementation lifecycle commands",
}

var implApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve implementation and transition to idle",
	Long: `Verifies review mode is active for the given spec,
pushes the spec branch to remote (if available), cleans up
worktrees and branches locally, and transitions state to idle.
This is the final human gate in the spec lifecycle.`,
	Args: cobra.ExactArgs(1),
	RunE: approveImplRunE,
}

func init() {
	implCmd.AddCommand(implApproveCmd)
}

// approveImplRunE is shared between `impl approve` and `approve impl`.
func approveImplRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]

	root, err := findRoot()
	if err != nil {
		return err
	}

	// Auto-cd into the spec worktree so phase resolution finds the correct
	// context. Without this, running from main fails because DiscoverActiveSpecs
	// doesn't find closed epics (review mode).
	specWtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	if info, err := os.Stat(specWtPath); err == nil && info.IsDir() {
		_ = os.Chdir(specWtPath)
	}

	exec := newExecutor(root)
	result, err := approve.ApproveImpl(root, specID, exec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Implementation for %s approved. Mode: idle.\n", result.SpecID)
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	if result.SpecBranch != "" {
		fmt.Println()
		fmt.Printf("Summary:\n")
		fmt.Printf("  Branch:   %s\n", result.SpecBranch)
		if result.CommitCount > 0 {
			fmt.Printf("  Commits:  %d\n", result.CommitCount)
		}
		if result.DiffStat != "" {
			fmt.Printf("\n%s\n", result.DiffStat)
		}
		if result.Pushed {
			fmt.Printf("\nBranch pushed to remote. Create a PR to merge into main:\n")
			fmt.Printf("  gh pr create --head %s --base main --title \"[SPEC %s] <title>\" --body \"<description>\"\n", result.SpecBranch, specID)
		}
	}
	fmt.Println()

	if err := emitInstruct(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
	}

	return nil
}
