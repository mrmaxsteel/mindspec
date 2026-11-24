package main

import (
	"errors"
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

var briefCmd = &cobra.Command{
	Use: "brief <step>", Short: "Print the brief of an enrolled actor step", SilenceUsage: true,
	Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return termsafe.NewFailure("brief requires exactly one actor step", "mindspec brief <step>")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := kernelCommandState("", args[0])
		if err != nil {
			var failure termsafe.RecoveryFailure
			if errors.As(err, &failure) {
				return err
			}
			return termsafe.NewWrappedFailure(err, "cannot read brief state: "+err.Error(), "mindspec next-action")
		}
		body, err := kernel.Brief(s, args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), body)
		return err
	},
}

func init() {
	briefCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return termsafe.NewWrappedFailure(err, "invalid brief flags: "+err.Error(), "mindspec brief <step>")
	})
	rootCmd.AddCommand(briefCmd)
}
