package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/spf13/cobra"
)

var beadCmd = &cobra.Command{
	Use:   "bead",
	Short: "Beads integration commands for worktree management and workset hygiene",
	Long:  `Manage worktrees and audit workset hygiene. Implementation beads are created via 'mindspec approve plan'.`,
}

var beadSpecCmd = &cobra.Command{
	Use:        "spec [spec-id]",
	Short:      "Deprecated: lifecycle managed via beads (ADR-0023)",
	Deprecated: "use 'mindspec spec create' which creates a lifecycle epic",
	Args:       cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Lifecycle state is derived from beads (ADR-0023).")
		fmt.Fprintln(os.Stderr, "Use 'mindspec spec create' to create a new spec with lifecycle tracking.")
		return nil
	},
}

var beadPlanCmd = &cobra.Command{
	Use:        "plan [spec-id]",
	Short:      "Deprecated: lifecycle managed via beads (ADR-0023)",
	Deprecated: "use 'mindspec approve plan' which creates implementation beads",
	Args:       cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Implementation beads are created automatically by 'mindspec approve plan'.")
		fmt.Fprintln(os.Stderr, "Use 'mindspec bead create-from-plan' to manually create beads from a plan.")
		return nil
	},
}

var beadWorktreeCmd = &cobra.Command{
	Use:        "worktree [bead-id]",
	Short:      "Show or create a worktree for a bead",
	Long:       `Without --create, shows the worktree path for a bead. With --create, creates a git worktree at ../worktree-<bead-id>.`,
	Deprecated: "use 'mindspec next' which creates worktrees automatically, or 'bd worktree' for direct access",
	Args:       cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		create, _ := cmd.Flags().GetBool("create")

		root, err := findRoot()
		if err != nil {
			return err
		}

		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		if create {
			wtName := "worktree-" + beadID
			branchName := "bead/" + beadID
			if err := bead.WorktreeCreate(wtName, branchName); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Worktree created: %s\n", wtName)
		} else {
			entries, err := bead.WorktreeList()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(2)
			}
			expectedName := "worktree-" + beadID
			expectedBranch := "bead/" + beadID
			found := false
			for _, e := range entries {
				if e.Name == expectedName || e.Branch == expectedBranch {
					fmt.Println(e.Path)
					found = true
					break
				}
			}
			if !found {
				fmt.Println("No worktree found for bead", beadID)
			}
		}
		return nil
	},
}

var beadHygieneCmd = &cobra.Command{
	Use:   "hygiene",
	Short: "Audit the active workset for hygiene issues",
	Long:  `Reports stale beads, orphaned beads, oversized descriptions, and total open count. Use --fix --yes to auto-close done beads.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		staleDays, _ := cmd.Flags().GetInt("stale-days")
		fix, _ := cmd.Flags().GetBool("fix")
		yes, _ := cmd.Flags().GetBool("yes")

		root, err := findRoot()
		if err != nil {
			return err
		}

		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		report, err := bead.AuditWorkset(staleDays)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}

		fmt.Print(bead.FormatReport(report))

		if fix {
			dryRun := !yes
			actions, err := bead.FixHygiene(dryRun)
			if err != nil {
				fmt.Fprintf(os.Stderr, "fix error: %v\n", err)
				os.Exit(2)
			}
			if dryRun {
				fmt.Println("\nDry run (use --yes to execute):")
			} else {
				fmt.Println("\nActions taken:")
			}
			for _, a := range actions {
				fmt.Printf("  %s\n", a)
			}
		}

		return nil
	},
}

var beadCreateFromPlanCmd = &cobra.Command{
	Use:   "create-from-plan [spec-id]",
	Short: "Create implementation beads from an approved plan's ## Bead sections",
	Long: `Parses the plan.md for ## Bead N: sections and creates corresponding
beads in Beads, parented to the spec's lifecycle epic.
Use this to recover when plan-approve failed to create beads.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]

		root, err := findRoot()
		if err != nil {
			return err
		}

		if err := bead.Preflight(root); err != nil {
			return fmt.Errorf("beads preflight failed: %w", err)
		}

		result, err := approve.CreateBeadsFromPlan(root, specID)
		if err != nil {
			return err
		}

		if len(result.BeadIDs) == 0 {
			fmt.Println("No ## Bead sections found in plan.")
			return nil
		}

		fmt.Printf("Created %d implementation beads:\n", len(result.BeadIDs))
		for _, id := range result.BeadIDs {
			fmt.Printf("  - %s\n", id)
		}
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}

		return nil
	},
}

func init() {
	beadWorktreeCmd.Flags().Bool("create", false, "Create a new worktree for the bead")
	beadHygieneCmd.Flags().Int("stale-days", 7, "Days without update before a bead is considered stale")
	beadHygieneCmd.Flags().Bool("fix", false, "Auto-close beads marked as done (dry-run by default)")
	beadHygieneCmd.Flags().Bool("yes", false, "Execute fix actions (requires --fix)")

	beadCmd.AddCommand(beadSpecCmd)
	beadCmd.AddCommand(beadPlanCmd)
	beadCmd.AddCommand(beadWorktreeCmd)
	beadCmd.AddCommand(beadHygieneCmd)
	beadCmd.AddCommand(beadCreateFromPlanCmd)
}
