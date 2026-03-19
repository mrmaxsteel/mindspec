package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/hook"
	"github.com/mrmaxsteel/mindspec/internal/instruct"
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

		// session-start is special: it writes session metadata and
		// runs instruct, rather than the pass/block/warn pattern.
		// Handle it before ParseInput to avoid blocking on stdin
		// (io.ReadAll hangs if the caller doesn't close stdin).
		if name == "session-start" {
			inp := hook.ParseInputNonBlocking(os.Stdin)
			return runSessionStartHook(inp)
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
// 2. Emits mode guidance (fast-path inline for protected branches, subprocess fallback)
func runSessionStartHook(inp *hook.Input) error {
	source := "unknown"
	if inp.Raw != nil {
		if s, ok := inp.Raw["source"].(string); ok && s != "" {
			source = s
		}
	}

	root, err := findRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not find mindspec root")
		return nil
	}

	// Write session metadata (best-effort)
	writeSession := exec.Command(os.Args[0], "state", "write-session", "--source="+source)
	writeSession.Dir = root
	writeSession.Stderr = os.Stderr
	_ = writeSession.Run()

	// Pre-warm dolt server in the background so bd calls don't pay the
	// 5s+ cold-start penalty. Idempotent — no-ops if already running.
	prewarmDolt(root)

	// Fast path: on a protected branch (main/master), emit idle guidance
	// inline without spawning a subprocess or querying beads.
	if output, ok := instruct.RenderIdleIfProtected(root); ok {
		fmt.Print(output)
		return nil
	}

	// Slow path: spawn instruct subprocess with a timeout for non-protected branches.
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "instruct")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintln(os.Stderr, "warning: mindspec instruct timed out (bd/dolt may be slow — try `bd dolt start`)")
		} else {
			fmt.Fprintln(os.Stderr, "mindspec instruct unavailable — run make build")
		}
	}
	return nil
}

// prewarmDolt starts the dolt server in the background if server mode is configured.
// Only relevant for server mode — embedded mode has no server to start.
// Checks env var, metadata.json, and config.yaml (matching beads config priority).
func prewarmDolt(root string) {
	if !isDoltServerMode(root) {
		return
	}
	cmd := exec.Command("bd", "dolt", "start")
	cmd.Dir = root
	_ = cmd.Start() // fire-and-forget; ~300ms
}

// isDoltServerMode checks whether beads is configured for dolt server mode.
// Follows beads config priority: env var > metadata.json > config.yaml > default (embedded).
func isDoltServerMode(root string) bool {
	// 1. Environment variable (highest priority)
	if v := os.Getenv("BEADS_DOLT_SERVER_MODE"); v == "1" || v == "true" {
		return true
	}

	// 2. metadata.json
	if data, err := os.ReadFile(filepath.Join(root, ".beads", "metadata.json")); err == nil {
		var meta map[string]any
		if json.Unmarshal(data, &meta) == nil {
			if mode, ok := meta["dolt_mode"].(string); ok {
				return mode == "server"
			}
		}
	}

	// 3. config.yaml (check for "mode: server" in dolt section)
	if data, err := os.ReadFile(filepath.Join(root, ".beads", "config.yaml")); err == nil {
		// Simple check — avoid pulling in a yaml dependency
		content := string(data)
		if strings.Contains(content, "mode: server") || strings.Contains(content, "mode: \"server\"") {
			return true
		}
	}

	return false // default: embedded
}

func init() {
	hookCmd.Flags().Bool("list", false, "List available hook names")
	hookCmd.Flags().StringVar(&hookFormat, "format", "", "Override protocol auto-detection (claude or copilot)")
	hookCmd.SetUsageTemplate(strings.Replace(hookCmd.UsageTemplate(), "{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}", "{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}", 1))
}
