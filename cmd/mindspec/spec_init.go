package main

import (
	"github.com/spf13/cobra"
)

var specInitCmd = &cobra.Command{
	Use:    "spec-init [spec-id]",
	Short:  "Initialize a new specification and enter Spec Mode",
	Long:   `Alias for 'mindspec spec create'. Use 'mindspec spec create' instead.`,
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	// RunE is wired in init() to reuse specCreateCmd.RunE so future
	// spec-create changes propagate to this hidden alias automatically.
}

func init() {
	specInitCmd.RunE = specCreateCmd.RunE
	// The shared RunE reads cmd.Flags().GetString("title"); cobra passes the
	// invoked command, so the alias must register its own --title flag.
	specInitCmd.Flags().String("title", "", "Spec title (derived from slug if omitted)")
}
