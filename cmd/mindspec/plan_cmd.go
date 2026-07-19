package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan lifecycle commands",
	// Spec 092 Req 10b: typos of the deprecated `approve` verb suggest
	// the noun-verb command families.
	SuggestFor: []string{"approve", "aprove"},
}

var planApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve a plan and transition toward Implementation Mode",
	Long: `Validates the plan, updates YAML frontmatter to Approved,
and creates implementation beads (if not already present).

After approval, run /clear then 'mindspec next' to claim your first bead.`,
	Args: cobra.ExactArgs(1),
	RunE: approvePlanRunE,
}

func init() {
	planApproveCmd.Flags().String("approved-by", "user", "Identity of the approver")
	planCmd.AddCommand(planApproveCmd)
}

// approvePlanRunE is shared between `plan approve` and `approve plan`.
func approvePlanRunE(cmd *cobra.Command, args []string) error {
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
	result, err := approve.ApprovePlan(root, specID, approvedBy, exec)
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
			fmt.Printf("  - %s\n", idrender.Bead(id))
		}
	} else {
		fmt.Println("WARNING: No implementation beads were created.")
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	fmt.Println()

	// Spec 080: stop after approval — do NOT auto-claim beads.
	fmt.Println("Next steps:")
	fmt.Println("  1. Run /clear to reset your context")
	fmt.Printf("  2. Run `mindspec next` to claim your first bead\n")
	fmt.Println()
	if err := emitInstruct(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
	}

	return nil
}
