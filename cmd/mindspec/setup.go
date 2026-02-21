package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Bootstrap agent-specific configuration",
	Long: `Sets up agent-specific configuration files (hooks, slash commands,
instruction file pointers) for a particular coding agent.

Currently supported agents:
  claude    Claude Code (.claude/settings.json, commands, CLAUDE.md)

Run 'mindspec setup claude' after 'mindspec init' to complete onboarding.`,
}

var setupClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Set up Claude Code integration (hooks, slash commands, CLAUDE.md)",
	Long: `Creates Claude Code-specific configuration:

  - .claude/settings.json with SessionStart and PreToolUse hooks
  - .claude/commands/*.md slash commands (spec-init, spec-approve, etc.)
  - CLAUDE.md with pointer to AGENTS.md and MindSpec guidance

If beads (bd) is installed, also runs 'bd setup claude'.

This command is idempotent — re-running it skips existing items.
Use --check to see what would be created without writing files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetBool("check")

		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		result, err := setup.RunClaude(root, checkFlag)
		if err != nil {
			return err
		}

		if checkFlag {
			fmt.Println("Check mode — no files written.")
			fmt.Println()
		}

		fmt.Print(result.FormatSummary())

		if !checkFlag && len(result.Created) > 0 {
			fmt.Println("\nClaude Code integration configured.")
		} else if !checkFlag && len(result.Created) == 0 {
			fmt.Println("\nClaude Code integration already configured.")
		}

		return nil
	},
}

func init() {
	setupClaudeCmd.Flags().Bool("check", false, "Report what would be created without writing files")
	setupCmd.AddCommand(setupClaudeCmd)
}
