package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mindspec/mindspec/internal/hook"
	"github.com/spf13/cobra"
)

var hookFormat string

var hookCmd = &cobra.Command{
	Use:   "hook <name>",
	Short: "Run a hook by name",
	Long: `Run a MindSpec hook. Reads context from stdin (JSON) or environment,
checks workflow state, and emits the appropriate response.

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
			inp = &hook.Input{}
		}

		switch hookFormat {
		case "claude":
			proto = hook.ProtocolClaude
		case "copilot":
			proto = hook.ProtocolCopilot
		case "":
			// auto-detect
		default:
			return fmt.Errorf("unknown format %q (use claude or copilot)", hookFormat)
		}

		// session-start is special: it writes session metadata and
		// runs instruct, rather than the pass/block/warn pattern.
		if name == "session-start" {
			return runSessionStartHook(inp)
		}

		st := hook.ReadState()
		result := hook.Run(name, inp, st, true)
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

// runSessionStartHook handles the session-start hook:
// 1. Writes session metadata (source + timestamp) from stdin JSON
// 2. Runs `mindspec instruct` and prints its output
func runSessionStartHook(inp *hook.Input) error {
	// Extract source from stdin JSON (Claude Code sends {"source": "startup|clear|resume|compact"})
	source := "unknown"
	if inp.Raw != nil {
		if s, ok := inp.Raw["source"].(string); ok && s != "" {
			source = s
		}
	}

	// Find root for running commands
	root, err := findRoot()
	if err != nil {
		// Can't find root — still try instruct
		fmt.Fprintln(os.Stderr, "warning: could not find mindspec root")
	}

	// Write session metadata (best-effort)
	if root != "" {
		writeSession := exec.Command(os.Args[0], "state", "write-session", "--source="+source)
		writeSession.Dir = root
		writeSession.Stderr = os.Stderr
		_ = writeSession.Run()
	}

	// Run instruct and emit its output
	instruct := exec.Command(os.Args[0], "instruct")
	if root != "" {
		instruct.Dir = root
	}
	instruct.Stdout = os.Stdout
	instruct.Stderr = os.Stderr
	if err := instruct.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mindspec instruct unavailable — run make build")
	}
	return nil
}

func init() {
	hookCmd.Flags().Bool("list", false, "List available hook names")
	hookCmd.Flags().StringVar(&hookFormat, "format", "", "Override protocol auto-detection (claude or copilot)")
	hookCmd.SetUsageTemplate(strings.Replace(hookCmd.UsageTemplate(), "{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}", "{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}", 1))
}
