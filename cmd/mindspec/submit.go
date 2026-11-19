package main

import (
	"encoding/json"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/spf13/cobra"
)

var submitCmd = &cobra.Command{Use: "submit <step-or-decision>", Short: "Submit evidence or answer a kernel decision", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	state, err := kernelCommandState("", args[0])
	if err != nil {
		return err
	}
	options := kernel.SubmitOptions{Step: args[0]}
	options.Lease, _ = cmd.Flags().GetString("lease")
	options.Artifact, _ = cmd.Flags().GetString("artifact")
	options.Commit, _ = cmd.Flags().GetString("commit")
	options.Answer, _ = cmd.Flags().GetString("answer")
	var out kernel.Output
	if options.Answer != "" {
		out, err = kernel.Answer(state, options.Step, options.Answer)
	} else {
		out, err = kernel.Submit(state, options)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
}}

func init() {
	submitCmd.Flags().String("lease", "", "Current lease token")
	submitCmd.Flags().String("artifact", "", "Artifact path held by a commit")
	submitCmd.Flags().String("commit", "", "Commit that holds the implementation or fix")
	submitCmd.Flags().String("answer", "", "Listed option for a decision")
	submitCmd.MarkFlagsMutuallyExclusive("artifact", "commit", "answer")
	rootCmd.AddCommand(submitCmd)
}
