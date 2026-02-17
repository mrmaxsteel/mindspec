package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/bootstrap"
	"github.com/mindspec/mindspec/internal/brownfield"
	"github.com/spf13/cobra"
)

type initMode string

const (
	initModeGreenfield       initMode = "greenfield"
	initModeBrownfieldReport initMode = "brownfield-report"
	initModeBrownfieldApply  initMode = "brownfield-apply"
)

func resolveInitMode(brownfieldFlag, reportOnlyFlag, applyFlag bool, archiveMode, resumeRunID string) (initMode, string, error) {
	if resumeRunID != "" && !brownfieldFlag {
		return "", "", fmt.Errorf("--resume requires --brownfield")
	}

	if !brownfieldFlag {
		if reportOnlyFlag || applyFlag || archiveMode != "" {
			return "", "", fmt.Errorf("--report-only, --apply, and --archive require --brownfield")
		}
		return initModeGreenfield, "", nil
	}

	if reportOnlyFlag && applyFlag {
		return "", "", fmt.Errorf("--report-only and --apply are mutually exclusive")
	}
	if archiveMode != "" && !applyFlag {
		return "", "", fmt.Errorf("--archive requires --apply")
	}

	if applyFlag {
		if archiveMode == "" {
			archiveMode = "copy"
		}
		if archiveMode != "copy" && archiveMode != "move" {
			return "", "", fmt.Errorf("invalid --archive value %q: must be copy or move", archiveMode)
		}
		return initModeBrownfieldApply, archiveMode, nil
	}

	// Default brownfield mode is report-only.
	return initModeBrownfieldReport, "", nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap a MindSpec project structure",
	Long: `Creates the required directory structure, starter files, and state
so that 'mindspec doctor' passes and the spec-driven workflow is
immediately usable.

All file creation is additive — existing files are never overwritten.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		brownfieldFlag, _ := cmd.Flags().GetBool("brownfield")
		reportOnlyFlag, _ := cmd.Flags().GetBool("report-only")
		applyFlag, _ := cmd.Flags().GetBool("apply")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		archiveMode, _ := cmd.Flags().GetString("archive")
		resumeRunID, _ := cmd.Flags().GetString("resume")

		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		if dryRun && brownfieldFlag {
			return fmt.Errorf("--dry-run is only valid for greenfield init; use --brownfield --report-only for no-write brownfield mode")
		}

		mode, archive, err := resolveInitMode(brownfieldFlag, reportOnlyFlag, applyFlag, archiveMode, resumeRunID)
		if err != nil {
			return err
		}

		switch mode {
		case initModeBrownfieldReport:
			report, err := brownfield.Run(root, brownfield.RunOptions{
				Apply:  false,
				RunID:  resumeRunID,
				Resume: resumeRunID != "",
			})
			if report != nil {
				if jsonFlag {
					out, jsonErr := report.ToJSON()
					if jsonErr != nil {
						return jsonErr
					}
					fmt.Println(out)
				} else {
					fmt.Println(report.FormatSummary())
					fmt.Println()
					fmt.Println("Mode: report-only (default for --brownfield)")
					fmt.Println("No files were modified.")
				}
			}
			if err != nil {
				return err
			}
			return nil
		case initModeBrownfieldApply:
			report, err := brownfield.Run(root, brownfield.RunOptions{
				Apply:       true,
				ArchiveMode: archive,
				RunID:       resumeRunID,
				Resume:      resumeRunID != "",
			})
			if report != nil {
				if jsonFlag {
					out, jsonErr := report.ToJSON()
					if jsonErr != nil {
						return jsonErr
					}
					fmt.Println(out)
				} else {
					fmt.Println(report.FormatSummary())
					fmt.Println()
				}
			}
			if err != nil {
				return err
			}
			fmt.Printf("Brownfield apply completed (archive=%s).\n", archive)
			return nil
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
	initCmd.Flags().Bool("dry-run", false, "Print what would be created without writing anything")
	initCmd.Flags().Bool("brownfield", false, "Run brownfield onboarding for existing repositories")
	initCmd.Flags().Bool("report-only", false, "Report-only brownfield analysis (no writes)")
	initCmd.Flags().Bool("apply", false, "Apply brownfield migration changes")
	initCmd.Flags().String("archive", "", "Archive mode for --apply: copy or move")
	initCmd.Flags().String("resume", "", "Resume or reuse a brownfield run ID")
	initCmd.Flags().Bool("json", false, "Output brownfield report as JSON")
}
