package main

import (
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:    "approve",
	Short:  "Approve a spec or plan, resolving its gate and transitioning state",
	Long:   `Validates, updates frontmatter, resolves the Beads human gate, transitions MindSpec state, and emits guidance for the new mode.`,
	Hidden: true,
}

var approveSpecCmd = &cobra.Command{
	Use:    "spec [spec-id]",
	Short:  "Approve a spec and transition to Plan Mode",
	Args:   cobra.ExactArgs(1),
	RunE:   approveSpecRunE,
	Hidden: true,
}

var approvePlanCmd = &cobra.Command{
	Use:    "plan [spec-id]",
	Short:  "Approve a plan and transition toward Implementation Mode",
	Args:   cobra.ExactArgs(1),
	RunE:   approvePlanRunE,
	Hidden: true,
}

var approveImplCmd = &cobra.Command{
	Use:    "impl [spec-id]",
	Short:  "Approve implementation and transition to idle",
	Args:   cobra.ExactArgs(1),
	RunE:   approveImplRunE,
	Hidden: true,
}

func init() {
	approveSpecCmd.Flags().String("approved-by", "user", "Identity of the approver")
	approvePlanCmd.Flags().String("approved-by", "user", "Identity of the approver")
	approveImplCmd.Flags().String("allow-doc-skew", "", "Override the doc-sync gate with a recorded reason (records reason+by+at on spec epic metadata)")
	approveImplCmd.Flags().String("override-adr", "", "Override the ADR-divergence gate with a recorded reason (records mindspec_adr_override_* on spec epic metadata)")
	approveImplCmd.Flags().String("supersede-adr", "", "Pre-create a placeholder ADR (Status: Proposed) at the supplied ID and bypass the divergence gate (records mindspec_adr_supersede_* on spec epic metadata)")
	approveCmd.AddCommand(approveSpecCmd)
	approveCmd.AddCommand(approvePlanCmd)
	approveCmd.AddCommand(approveImplCmd)
}
