package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan lifecycle commands",
}

var planApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve a plan and transition toward Implementation Mode",
	Long: `Validates the plan, updates YAML frontmatter to Approved,
creates implementation beads (if not already present),
resolves the plan gate in Beads, and automatically claims the first
bead via 'mindspec next'.

Use --no-next to approve without claiming work (e.g. when reviewing
a plan but not starting implementation yet).`,
	Args: cobra.ExactArgs(1),
	RunE: approvePlanRunE,
}

func init() {
	planApproveCmd.Flags().String("approved-by", "user", "Identity of the approver")
	planApproveCmd.Flags().Bool("no-next", false, "Approve without auto-claiming the first bead")
	planCmd.AddCommand(planApproveCmd)
}

// approvePlanRunE is shared between `plan approve` and `approve plan`.
func approvePlanRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]
	approvedBy, _ := cmd.Flags().GetString("approved-by")
	noNext, _ := cmd.Flags().GetBool("no-next")

	root, err := findRoot()
	if err != nil {
		return err
	}

	if err := bead.Preflight(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: Beads preflight failed: %v (bead creation and gate resolution may fail)\n", err)
	}

	result, err := approve.ApprovePlan(root, specID, approvedBy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

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

	// Auto-next: claim the first bead unless --no-next
	if !noNext {
		fmt.Println("--- Claiming next bead ---")
		fmt.Println()
		_ = nextCmd.Flags().Set("spec", specID)
		_ = nextCmd.Flags().Set("force", "true")
		if nextErr := nextCmd.RunE(nextCmd, nil); nextErr != nil {
			fmt.Fprintf(os.Stderr, "warning: auto-next failed: %v\n", nextErr)
			fmt.Fprintf(os.Stderr, "Run `mindspec next --spec %s` manually to claim work.\n", specID)
			// Fall back to plan-mode guidance
			if err := emitInstruct(root); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
			}
		}
		// next already emits instruct — no need to emit again
	} else {
		fmt.Println("Plan approved (--no-next). Run `mindspec next` when ready to start work.")
		fmt.Println()
		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}
	}

	return nil
}
