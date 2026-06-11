package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// approveCmd is the hidden deprecated verb-noun alias (`mindspec
// approve <noun>`). It stays hidden per spec 092 DQ-3; the canonical
// commands are the noun-verb forms (`mindspec spec approve <id>` etc.,
// see approvalGatesBlock in root.go). Its subcommands keep working for
// backward compatibility; bare or unknown targets error with the
// canonical commands (spec 092 Req 10b).
var approveCmd = &cobra.Command{
	Use:    "approve",
	Short:  "Deprecated alias — approval gates use the noun-verb order (spec/plan/impl approve)",
	Long:   "Deprecated verb-noun alias kept for backward compatibility. Use the canonical noun-verb commands:\n\n" + approvalGatesBlock,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		msg := "missing approval target"
		if len(args) > 0 {
			msg = fmt.Sprintf("unknown approval target %q", args[0])
		}
		return fmt.Errorf("%s — approval gates use the noun-verb order:\n%s", msg, approvalGatesBlock)
	},
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
