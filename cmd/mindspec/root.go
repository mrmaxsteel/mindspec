package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mindspec",
	Short: "MindSpec: Spec-Driven Development and Self-Documentation System",
	Long:  `MindSpec is a spec-driven development + context management framework.`,
}

func init() {
	rootCmd.AddCommand(approveCmd)
	rootCmd.AddCommand(beadCmd)
	rootCmd.AddCommand(completeCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(glossaryCmd)
	rootCmd.AddCommand(instructCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(specInitCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(validateCmd)
}
