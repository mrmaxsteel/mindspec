package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/spf13/cobra"
)

var beadCmd = &cobra.Command{
	Use:   "bead",
	Short: "Beads integration commands for spec and implementation bead lifecycle",
	Long:  `Create spec beads, plan beads, manage worktrees, and audit workset hygiene.`,
}

var beadSpecCmd = &cobra.Command{
	Use:   "spec [spec-id]",
	Short: "Create a spec bead from an approved specification",
	Long:  `Reads an approved spec, extracts metadata, and creates a Beads issue with a structured description. Idempotent: returns existing bead if already created.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]

		root, err := findRoot()
		if err != nil {
			return err
		}

		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		result, err := bead.CreateSpecBead(root, specID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Spec bead: %s\n", result.Bead.ID)
		if result.GateID != "" {
			fmt.Printf("Spec gate: %s (resolve via `mindspec approve spec %s`)\n", result.GateID, specID)
		}
		return nil
	},
}

var beadPlanCmd = &cobra.Command{
	Use:        "plan [spec-id]",
	Short:      "Create implementation beads from an approved plan",
	Long:       `Reads an approved plan with work_chunks, creates one bead per chunk, wires dependencies, and writes bead IDs into plan frontmatter.`,
	Deprecated: "use /plan-approve workflow instead, which calls this automatically",
	Args:       cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]

		root, err := findRoot()
		if err != nil {
			return err
		}

		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		result, err := bead.CreatePlanBeads(root, specID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Write bead IDs back to plan frontmatter
		planPath := fmt.Sprintf("%s/docs/specs/%s/plan.md", root, specID)
		if err := bead.WriteGeneratedBeadIDs(planPath, result); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write bead IDs to plan: %v\n", err)
		}

		fmt.Printf("Molecule parent: %s\n", result.MolParentID)
		if result.PlanGateID != "" {
			fmt.Printf("Plan gate: %s (resolve via `mindspec approve plan %s`)\n", result.PlanGateID, specID)
		}
		fmt.Println("Implementation beads created:")
		for chunkID, beadID := range result.ChunkBeads {
			fmt.Printf("  chunk %d → %s\n", chunkID, beadID)
		}
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

func init() {
	beadWorktreeCmd.Flags().Bool("create", false, "Create a new worktree for the bead")
	beadHygieneCmd.Flags().Int("stale-days", 7, "Days without update before a bead is considered stale")
	beadHygieneCmd.Flags().Bool("fix", false, "Auto-close beads marked as done (dry-run by default)")
	beadHygieneCmd.Flags().Bool("yes", false, "Execute fix actions (requires --fix)")

	beadCmd.AddCommand(beadSpecCmd)
	beadCmd.AddCommand(beadPlanCmd)
	beadCmd.AddCommand(beadWorktreeCmd)
	beadCmd.AddCommand(beadHygieneCmd)
}
