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
resolves the spec gate in Beads, sets state to plan mode,
and emits plan mode guidance.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]

		root, err := findRoot()
		if err != nil {
			return err
		}

		// Preflight Beads (best-effort — gate resolution needs it)
		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: Beads preflight failed: %v (gate resolution may fail)\n", err)
		}

		result, err := approve.ApproveSpec(root, specID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Summary
		fmt.Printf("Spec %s approved.\n", specID)
		if result.GateID != "" {
			fmt.Printf("Gate resolved: %s\n", result.GateID)
		}
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
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
resolves the plan gate in Beads, and emits guidance.
Run 'mindspec next' after this to claim work and enter Implementation Mode.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]

		root, err := findRoot()
		if err != nil {
			return err
		}

		// Preflight Beads (best-effort)
		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: Beads preflight failed: %v (gate resolution may fail)\n", err)
		}

		result, err := approve.ApprovePlan(root, specID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Summary
		fmt.Printf("Plan %s approved.\n", specID)
		if result.GateID != "" {
			fmt.Printf("Gate resolved: %s\n", result.GateID)
		}
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
		fmt.Println()
		fmt.Println("Run `mindspec next` to claim work and enter Implementation Mode.")
		fmt.Println()

		// Instruct-tail: emit guidance for current mode
		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

func init() {
	approveCmd.AddCommand(approveSpecCmd)
	approveCmd.AddCommand(approvePlanCmd)
}
