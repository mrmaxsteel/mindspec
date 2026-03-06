package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/bootstrap"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap a new MindSpec project (greenfield only)",
	Long: `Creates the required directory structure, starter files, and state
so that 'mindspec doctor' passes and the spec-driven workflow is
immediately usable.

All file creation is additive — existing files are never overwritten.

Use 'mindspec migrate' to onboard an existing brownfield repository.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		result, err := bootstrap.Run(root, dryRun)
		if err != nil {
			return err
		}

		if dryRun {
			fmt.Println("Dry run — no files written.")
			fmt.Println()
		}

		fmt.Print(result.FormatSummary())

		if !dryRun && len(result.Created) > 0 {
			fmt.Println("\nProject bootstrapped. Run 'mindspec doctor' to verify.")
		}

		return nil
	},
}

func init() {
	initCmd.Flags().Bool("dry-run", false, "Print what would be created without writing files")
}
