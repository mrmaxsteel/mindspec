package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/validate"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate spec, plan, and documentation artifacts",
	Long:  `Checks that specs, plans, and documentation meet MindSpec quality standards.`,
}

var validateSpecCmd = &cobra.Command{
	Use:   "spec [spec-id]",
	Short: "Validate a specification document",
	Long:  `Checks structural quality of a spec.md file: required sections, acceptance criteria quality, open questions.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		specID := args[0]

		root, err := findRoot()
		if err != nil {
			return err
		}

		result := validate.ValidateSpec(root, specID)
		return outputResult(result, format)
	},
}

var validatePlanCmd = &cobra.Command{
	Use:   "plan [spec-id]",
	Short: "Validate a plan document",
	Long:  `Checks structural quality of a plan.md file: YAML frontmatter, bead sections, verification steps, ADR citations.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		specID := args[0]

		root, err := findRoot()
		if err != nil {
			return err
		}

		result := validate.ValidatePlan(root, specID)
		return outputResult(result, format)
	},
}

var validateDocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Check doc-sync compliance",
	Long:  `Compares changed source files against documentation updates to detect missing doc-sync.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		diffRef, _ := cmd.Flags().GetString("diff")

		root, err := findRoot()
		if err != nil {
			return err
		}

		result := validate.ValidateDocs(root, diffRef)
		return outputResult(result, format)
	},
}

func init() {
	validateCmd.PersistentFlags().String("format", "", "Output format: text (default) or json")
	validateDocsCmd.Flags().String("diff", "HEAD~1", "Git ref to diff against")

	validateCmd.AddCommand(validateSpecCmd)
	validateCmd.AddCommand(validatePlanCmd)
	validateCmd.AddCommand(validateDocsCmd)
}

func outputResult(result *validate.Result, format string) error {
	if format == "json" {
		out, err := result.ToJSON()
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(result.FormatText())
	}

	if result.HasFailures() {
		os.Exit(1)
	}
	return nil
}
