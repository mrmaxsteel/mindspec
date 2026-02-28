package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/approve"
	"github.com/mindspec/mindspec/internal/bead"
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve",
	Short: "Approve a spec or plan, resolving its gate and transitioning state",
	Long:  `Validates, updates frontmatter, resolves the Beads human gate, transitions MindSpec state, and emits guidance for the new mode.`,
}

var approveSpecCmd = &cobra.Command{
	Use:   "spec [spec-id]",
	Short: "Approve a spec and transition to Plan Mode",
	Long: `Validates the spec, updates the Approval section to APPROVED,
creates the spec bead and gate (if not already present),
resolves the spec gate in Beads, generates the context pack,
sets state to plan mode, and emits plan mode guidance.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]
		approvedBy, _ := cmd.Flags().GetString("approved-by")

		root, err := findRoot()
		if err != nil {
			return err
		}

		// Preflight Beads (best-effort — gate resolution needs it)
		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: Beads preflight failed: %v (bead creation and gate resolution may fail)\n", err)
		}

		result, err := approve.ApproveSpec(root, specID, approvedBy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Summary
		fmt.Printf("Spec %s approved.\n", specID)
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Commit approval artifacts before continuing (required for clean-tree gates).")
		fmt.Printf("  2. Continue planning for %s.\n", specID)
		fmt.Println()

		// Instruct-tail: emit guidance for plan mode
		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

var approvePlanCmd = &cobra.Command{
	Use:   "plan [spec-id]",
	Short: "Approve a plan and transition toward Implementation Mode",
	Long: `Validates the plan, updates YAML frontmatter to Approved,
creates implementation beads (if not already present),
resolves the plan gate in Beads, and emits guidance.
Run 'mindspec next' after this to claim work and enter Implementation Mode.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]
		approvedBy, _ := cmd.Flags().GetString("approved-by")

		root, err := findRoot()
		if err != nil {
			return err
		}

		// Preflight Beads (best-effort)
		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: Beads preflight failed: %v (bead creation and gate resolution may fail)\n", err)
		}

		result, err := approve.ApprovePlan(root, specID, approvedBy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Summary
		fmt.Printf("Plan %s approved.\n", specID)
		if result.GateID != "" {
			fmt.Printf("Gate resolved: %s\n", result.GateID)
		}
		if len(result.BeadIDs) > 0 {
			fmt.Printf("Created %d implementation beads:\n", len(result.BeadIDs))
			for _, id := range result.BeadIDs {
				fmt.Printf("  - %s\n", id)
			}
		} else {
			fmt.Println("WARNING: No implementation beads were created.")
		}
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Commit approval artifacts now.")
		fmt.Println("  2. Run `mindspec next` to claim work and enter Implementation Mode.")
		fmt.Println("     `mindspec next` requires a clean working tree and will fail if approval changes are uncommitted.")
		fmt.Println()

		// Instruct-tail: emit guidance for current mode
		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

var approveImplCmd = &cobra.Command{
	Use:   "impl [spec-id]",
	Short: "Approve implementation and transition to idle",
	Long: `Verifies review mode is active for the given spec,
transitions state to idle, and emits idle mode guidance.
This is the final human gate in the spec lifecycle.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Print merge summary if a merge was performed.
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
				}
			}
			if result.DiffStat != "" {
				fmt.Printf("\n%s\n", result.DiffStat)
			}
		}
		fmt.Println()

		// Instruct-tail: emit idle guidance
		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

func init() {
	approveSpecCmd.Flags().String("approved-by", "user", "Identity of the approver")
	approvePlanCmd.Flags().String("approved-by", "user", "Identity of the approver")
	approveImplCmd.Flags().Bool("wait", false, "Wait for CI checks to pass then merge PR (only applies to PR strategy)")
	approveCmd.AddCommand(approveSpecCmd)
	approveCmd.AddCommand(approvePlanCmd)
	approveCmd.AddCommand(approveImplCmd)
}
