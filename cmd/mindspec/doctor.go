package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of the current workspace",
	Long: `Validates project structure, documentation health, and Beads hygiene.

Use --fix to auto-repair fixable issues (e.g. tracked runtime files).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fix, _ := cmd.Flags().GetBool("fix")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return fmt.Errorf("workspace not found: %w", err)
		}

		fmt.Printf("Workspace Root: %s\n", root)

		report := doctor.Run(root)

		if fix {
			report.Fix()
		}

		for _, c := range report.Checks {
			fmt.Printf("%s: %s", c.Name, statusTag(c.Status))
			if c.Message != "" {
				fmt.Printf(" %s", c.Message)
			}
			fmt.Println()
		}

		if report.HasFailures() {
			os.Exit(1)
		}
		return nil
	},
}

func statusTag(s doctor.Status) string {
	switch s {
	case doctor.OK:
		return "[OK]"
	case doctor.Missing:
		return "[MISSING]"
	case doctor.Error:
		return "[ERROR]"
	case doctor.Warn:
		return "[WARN]"
	case doctor.Fixed:
		return "[FIXED]"
	default:
		return "[UNKNOWN]"
	}
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Auto-repair fixable issues")
}
