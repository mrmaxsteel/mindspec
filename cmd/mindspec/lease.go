package main

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/spf13/cobra"
)

var leaseCmd = &cobra.Command{Use: "lease <step>", Short: "Lease a kernel actor step", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	state, err := kernelCommandState("", args[0])
	if err != nil {
		return err
	}
	options := kernel.LeaseOptions{Step: args[0]}
	options.Renew, _ = cmd.Flags().GetString("renew")
	options.Release, _ = cmd.Flags().GetString("release")
	options.Take, _ = cmd.Flags().GetBool("take")
	options.Reason, _ = cmd.Flags().GetString("reason")
	result, err := kernel.Lease(state, options, kernel.LeaseDeps{EnsureWorktree: func(root, item, spec string) (string, error) {
		return next.EnsureWorktree(root, item, spec, newExecutor(root))
	}})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Token)
	return err
}}

func init() {
	leaseCmd.Flags().String("renew", "", "Renew with the current lease token")
	leaseCmd.Flags().String("release", "", "Release with the current lease token")
	leaseCmd.Flags().Bool("take", false, "Revoke the current lease and issue a new generation")
	leaseCmd.Flags().String("reason", "", "Reason for taking the lease")
	leaseCmd.MarkFlagsMutuallyExclusive("renew", "release", "take")
	rootCmd.AddCommand(leaseCmd)
}
