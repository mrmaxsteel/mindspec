package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/spec"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Spec lifecycle commands",
}

var specCreateCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create a new specification and enter Spec Mode",
	Long: `Creates a new spec directory with spec.md from the template,
creates a branch and worktree, sets state to spec mode, and emits guidance.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]
		title, _ := cmd.Flags().GetString("title")

		root, err := findRoot()
		if err != nil {
			return err
		}

		exec := newExecutor(root)
		result, err := spec.Run(root, specID, title, exec)
		if err != nil {
			return err
		}

		specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
		relPath, err := filepath.Rel(root, specPath)
		if err != nil {
			relPath = specPath
		}
		fmt.Printf("Spec initialized: %s\n", filepath.ToSlash(relPath))

		if result.WorktreePath != "" {
			fmt.Printf("Worktree: %s (branch: %s)\n", result.WorktreePath, result.SpecBranch)
			fmt.Printf("\n  cd %s\n\n", result.WorktreePath)
		} else {
			fmt.Println()
		}

		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

var specApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve a spec and transition to Plan Mode",
	Long: `Validates the spec, updates the Approval section to APPROVED,
creates the spec bead and gate (if not already present),
resolves the spec gate in Beads, generates the context pack,
sets state to plan mode, and emits plan mode guidance.`,
	Args: cobra.ExactArgs(1),
	RunE: approveSpecRunE,
}

func init() {
	specCreateCmd.Flags().String("title", "", "Spec title (derived from slug if omitted)")
	specApproveCmd.Flags().String("approved-by", "user", "Identity of the approver")
	specCmd.AddCommand(specCreateCmd)
	specCmd.AddCommand(specApproveCmd)
}

// approveSpecRunE is shared between `spec approve` and `approve spec`.
func approveSpecRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]
	approvedBy, _ := cmd.Flags().GetString("approved-by")

	root, err := findRoot()
	if err != nil {
		return err
	}

	if err := bead.Preflight(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: Beads preflight failed: %v (bead creation and gate resolution may fail)\n", err)
	}

	exec := newExecutor(root)
	result, err := approve.ApproveSpec(root, specID, approvedBy, exec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Spec %s approved.\n", specID)
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Commit approval artifacts before continuing (required for clean-tree gates).")
	fmt.Printf("  2. Continue planning for %s.\n", specID)
	fmt.Println()

	if err := emitInstruct(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
	}

	return nil
}
