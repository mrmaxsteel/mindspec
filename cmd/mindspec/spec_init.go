package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/specinit"
	"github.com/spf13/cobra"
)

var specInitCmd = &cobra.Command{
	Use:   "spec-init [spec-id]",
	Short: "Initialize a new specification and enter Spec Mode",
	Long: `Creates a new spec directory with spec.md from the template,
sets state to spec mode, and emits spec mode guidance.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]
		title, _ := cmd.Flags().GetString("title")

		root, err := findRoot()
		if err != nil {
			return err
		}

		if err := specinit.Run(root, specID, title); err != nil {
			return err
		}

		fmt.Printf("Spec initialized: docs/specs/%s/spec.md\n\n", specID)

		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

func init() {
	specInitCmd.Flags().String("title", "", "Spec title (derived from slug if omitted)")
}
