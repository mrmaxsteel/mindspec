package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Bootstrap agent-specific configuration",
	Long: `Sets up agent-specific configuration files (hooks, slash commands,
instruction file pointers) for a particular coding agent.

Supported agents:
  claude    Claude Code (.claude/settings.json, commands, CLAUDE.md)
  codex     OpenAI Codex CLI (AGENTS.md, git hooks)
  copilot   GitHub Copilot (.github/copilot-instructions.md, prompts)

Run 'mindspec setup <agent>' after 'mindspec init' to complete onboarding.`,
}

var setupClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Set up Claude Code integration (hooks, skills, CLAUDE.md)",
	Long: `Creates Claude Code-specific configuration:

  - .claude/settings.json with SessionStart and PreToolUse hooks
  - .claude/skills/ms-*/SKILL.md workflow skills (spec-init, approve, etc.)
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

var setupCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Set up GitHub Copilot integration (instructions, prompt files)",
	Long: `Creates GitHub Copilot-specific configuration:

  - .github/copilot-instructions.md with pointer to AGENTS.md and MindSpec guidance
  - .github/hooks/mindspec.json with sessionStart and preToolUse hooks
  - .github/prompts/*.prompt.md workflow prompt files (spec-approve, plan-approve, etc.)

This command is idempotent — re-running it skips existing items.
Use --check to see what would be created without writing files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetBool("check")

		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		result, err := setup.RunCopilot(root, checkFlag)
		if err != nil {
			return err
		}

		if checkFlag {
			fmt.Println("Check mode — no files written.")
			fmt.Println()
		}

		fmt.Print(result.FormatSummary())

		if !checkFlag && len(result.Created) > 0 {
			fmt.Println("\nGitHub Copilot integration configured.")
		} else if !checkFlag && len(result.Created) == 0 {
			fmt.Println("\nGitHub Copilot integration already configured.")
		}

		return nil
	},
}

var setupCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Set up OpenAI Codex CLI integration (AGENTS.md, skills, git hooks)",
	Long: `Creates OpenAI Codex CLI-specific configuration:

  - AGENTS.md with MindSpec guidance block
  - .agents/skills/ms-*/SKILL.md workflow skills (spec-init, approve, etc.)
  - Git hooks (pre-commit branch enforcement)

If beads (bd) is installed, also runs 'bd setup codex'.

This command is idempotent — re-running it skips existing items.
Use --check to see what would be created without writing files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetBool("check")

		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		result, err := setup.RunCodex(root, checkFlag)
		if err != nil {
			return err
		}

		if checkFlag {
			fmt.Println("Check mode — no files written.")
			fmt.Println()
		}

		fmt.Print(result.FormatSummary())

		if !checkFlag && len(result.Created) > 0 {
			fmt.Println("\nCodex CLI integration configured.")
		} else if !checkFlag && len(result.Created) == 0 {
			fmt.Println("\nCodex CLI integration already configured.")
		}

		return nil
	},
}

func init() {
	setupClaudeCmd.Flags().Bool("check", false, "Report what would be created without writing files")
	setupCodexCmd.Flags().Bool("check", false, "Report what would be created without writing files")
	setupCopilotCmd.Flags().Bool("check", false, "Report what would be created without writing files")
	setupCmd.AddCommand(setupClaudeCmd)
	setupCmd.AddCommand(setupCodexCmd)
	setupCmd.AddCommand(setupCopilotCmd)
}
