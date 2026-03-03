package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/approve"
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
transitions state to idle, and emits idle mode guidance.
This is the final human gate in the spec lifecycle.`,
	Args: cobra.ExactArgs(1),
	RunE: approveImplRunE,
}

func init() {
	implApproveCmd.Flags().Bool("wait", false, "Wait for CI checks to pass then merge PR (only applies to PR strategy)")
	implCmd.AddCommand(implApproveCmd)
}

// approveImplRunE is shared between `impl approve` and `approve impl`.
func approveImplRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]

	root, err := findRoot()
	if err != nil {
		return err
	}

	wait, _ := cmd.Flags().GetBool("wait")
	opts := approve.ImplOpts{Wait: wait}

	result, err := approve.ApproveImpl(root, specID, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Implementation for %s approved. Mode: idle.\n", result.SpecID)
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	if result.MergeStrategy != "" {
		fmt.Println()
		fmt.Printf("Merge summary:\n")
		fmt.Printf("  Strategy: %s\n", result.MergeStrategy)
		fmt.Printf("  Branch:   %s → main\n", result.SpecBranch)
		if result.CommitCount > 0 {
			fmt.Printf("  Commits:  %d\n", result.CommitCount)
		}
		if result.PRURL != "" {
			fmt.Printf("  PR:       %s\n", result.PRURL)
			if result.PRMerged {
				fmt.Printf("  Status:   merged\n")
			} else {
				fmt.Printf("\nACTION REQUIRED: Fill in the PR template at %s\n", result.PRURL)
				fmt.Printf("  Update the Summary, Spec, Changes, and Test plan sections.\n")
			}
		}
		if result.DiffStat != "" {
			fmt.Printf("\n%s\n", result.DiffStat)
		}
	}
	fmt.Println()

	if err := emitInstruct(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
	}

	return nil
}
