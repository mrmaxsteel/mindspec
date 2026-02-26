package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mindspec/mindspec/internal/hook"
	"github.com/spf13/cobra"
)

var hookFormat string

var hookCmd = &cobra.Command{
	Use:   "hook <name>",
	Short: "Run a PreToolUse hook by name",
	Long: `Run a MindSpec PreToolUse hook. Reads tool context from stdin (JSON),
checks workflow state, and emits the appropriate response for the caller's
protocol (Claude Code or Copilot, auto-detected from stdin format).

Use --list to see available hook names.
Use --format to override protocol auto-detection.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		list, _ := cmd.Flags().GetBool("list")
		if list {
			for _, name := range hook.Names {
				fmt.Println(name)
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("hook name required (use --list to see available hooks)")
		}

		name := args[0]
		if !isValidHook(name) {
			return fmt.Errorf("unknown hook %q (use --list to see available hooks)", name)
		}

		inp, proto, err := hook.ParseInput(os.Stdin)
		if err != nil {
			// Graceful: parse errors are not fatal, treat as empty input
			inp = &hook.Input{}
		}

		// Override protocol if --format is set
		switch hookFormat {
		case "claude":
			proto = hook.ProtocolClaude
		case "copilot":
			proto = hook.ProtocolCopilot
		case "":
			// auto-detect (already set by ParseInput)
		default:
			return fmt.Errorf("unknown format %q (use claude or copilot)", hookFormat)
		}

		st := hook.ReadState()
		enforce := hook.EnforcementEnabled()

		result := hook.Run(name, inp, st, enforce)
		code := hook.Emit(result, proto)
		if code != 0 {
			os.Exit(code)
		}
		return nil
	},
}

func isValidHook(name string) bool {
	for _, n := range hook.Names {
		if n == name {
			return true
		}
	}
	return false
}

func init() {
	hookCmd.Flags().Bool("list", false, "List available hook names")
	hookCmd.Flags().StringVar(&hookFormat, "format", "", "Override protocol auto-detection (claude or copilot)")
	// Hide from default help since these are internal flags
	hookCmd.SetUsageTemplate(strings.Replace(hookCmd.UsageTemplate(), "{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}", "{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}", 1))
}
