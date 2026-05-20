package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/agentmind/client"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var agentmindCmd = &cobra.Command{
	Use:     "agentmind",
	Aliases: []string{"viz"},
	Short:   "AgentMind — real-time 3D visualization of agent activity",
	Long: `Launch a local web server that renders agent activity as an interactive
3D force-directed graph with a starfield aesthetic.

Subcommands:
  serve         Start OTLP receiver + web UI for real-time visualization
  replay        Replay a recorded NDJSON session file
  setup         Configure agent telemetry export to AgentMind`,
}

// agentmindServeCmd is a thin cobra re-exec wrapper around the
// standalone `agentmind` binary (spec 083 Phase 4b, Bead 4). It
// reconstructs the equivalent `agentmind serve …` argv from the
// flags cobra parsed for us and execs the binary via
// `client.RunStandalone`.
//
// Per spec 083 Hard Constraint #4 (interactive class): on
// `errors.Is(err, client.ErrBinaryNotFound)` we MUST exit non-zero.
// The canonical warn line is emitted exactly once per process via
// `client.EmitWarnOnce` to keep parity with the batch class and to
// give users one consistent diagnostic.
var agentmindServeCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"live"},
	Short:   "Start OTLP receiver + web UI for real-time visualization",
	Long: `Start an OTLP/HTTP receiver and web UI server. Configure Claude Code with:
  export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:<otlp-port>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		otlpPort, _ := cmd.Flags().GetInt("otlp-port")
		uiPort, _ := cmd.Flags().GetInt("ui-port")
		output, _ := cmd.Flags().GetString("output")
		bind, _ := cmd.Flags().GetString("bind")

		runArgs := []string{
			"serve",
			"--otlp-port", strconv.Itoa(otlpPort),
			"--ui-port", strconv.Itoa(uiPort),
			"--bind", bind,
		}
		if strings.TrimSpace(output) != "" {
			runArgs = append(runArgs, "--output", output)
		}

		// Write the AgentMind identity lockfile per the contract in
		// internal/recording/lockfile.go. The standalone agentmind
		// binary does not yet own this write (see spec 083 — the
		// contract docstring notes "When AgentMind is extracted into
		// its own binary, this file...MUST be copied/re-exported"),
		// so the mindspec wrapper continues to own it until the
		// sibling binary takes over.
		//
		// Spec 083 Bead 5: the lockfile helpers moved from the deleted
		// `internal/agentmind/` package to `internal/recording/`
		// (mindspec owns the contract; the agentmind binary honors it).
		token, tokErr := recording.NewToken()
		if tokErr != nil {
			return fmt.Errorf("generating agentmind token: %w", tokErr)
		}
		lf := recording.Lockfile{
			PID:       os.Getpid(),
			OTLPPort:  otlpPort,
			UIPort:    uiPort,
			Token:     token,
			StartedAt: time.Now().UTC(),
		}
		if err := recording.WriteLockfile(lf); err != nil {
			return fmt.Errorf("writing agentmind lockfile: %w", err)
		}
		defer func() { _ = recording.RemoveLockfile() }()

		return runStandaloneWithInteractiveDegradation(cmd, runArgs)
	},
}

// agentmindReplayCmd resolves the recording file path on the
// mindspec side (so the spec-id → file-path lookup keeps using
// mindspec's workspace layout) and execs the standalone agentmind
// binary with the resolved file as the positional argument.
//
// Per spec 083 Hard Constraint #4 (interactive class): exits
// non-zero when the binary is absent.
var agentmindReplayCmd = &cobra.Command{
	Use:   "replay [file.jsonl]",
	Short: "Replay a recorded NDJSON session file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID, _ := cmd.Flags().GetString("spec")
		phase, _ := cmd.Flags().GetString("phase")
		speed, _ := cmd.Flags().GetFloat64("speed")
		uiPort, _ := cmd.Flags().GetInt("ui-port")

		// Resolve file path — stays in mindspec because it depends
		// on mindspec's workspace/spec layout (per spec 083 Scope:
		// "Recording-directory ownership stays in mindspec").
		var filePath string
		if len(args) > 0 {
			filePath = args[0]
		} else if specID != "" {
			if err := validate.SpecID(specID); err != nil {
				return err
			}
			root, err := findRoot()
			if err != nil {
				return fmt.Errorf("finding project root: %w", err)
			}
			filePath, err = recording.EventsPath(root, specID)
			if err != nil {
				return err
			}
			if _, statErr := os.Stat(filePath); statErr != nil {
				specDir, sdErr := workspace.SpecDir(root, specID)
				if sdErr != nil {
					return sdErr
				}
				filePath = filepath.Join(specDir, "recording", "events.ndjson")
				if _, statErr := os.Stat(filePath); statErr != nil {
					return fmt.Errorf("no recording found for spec %s", specID)
				}
			}
		} else {
			return fmt.Errorf("provide a file path or use --spec <id>")
		}

		if speed <= 0 {
			speed = 0
		}

		runArgs := []string{
			"replay",
			filePath,
			"--speed", strconv.FormatFloat(speed, 'f', -1, 64),
			"--ui-port", strconv.Itoa(uiPort),
		}
		if strings.TrimSpace(phase) != "" {
			runArgs = append(runArgs, "--phase", phase)
		}

		return runStandaloneWithInteractiveDegradation(cmd, runArgs)
	},
}

// runStandaloneWithInteractiveDegradation invokes
// client.RunStandalone(runArgs) and applies the interactive-class
// graceful-degradation contract from spec 083 Hard Constraint #4:
//
//   - On `errors.Is(err, client.ErrBinaryNotFound)`: emit the
//     canonical warn line via `client.EmitWarnOnce`, suppress
//     cobra's usage printout, and return an error that produces a
//     non-zero exit code. A user-invoked UI command that exits 0
//     with no UI is a UX bug per the spec.
//   - On `*exec.ExitError`: propagate so the subprocess's non-zero
//     exit reaches the user, suppress cobra usage (the failure is
//     in the child, not in our CLI parsing).
//   - On any other error: return as-is (cobra surfaces it with its
//     default "Error:" prefix).
func runStandaloneWithInteractiveDegradation(cmd *cobra.Command, runArgs []string) error {
	err := client.RunStandalone(runArgs)
	if err == nil {
		return nil
	}
	if errors.Is(err, client.ErrBinaryNotFound) {
		client.EmitWarnOnce(os.Stderr)
		cmd.SilenceUsage = true
		return fmt.Errorf("interactive command requires the agentmind binary; install it or set $AGENTMIND_BIN: %w", err)
	}
	if _, isExit := err.(*exec.ExitError); isExit {
		cmd.SilenceUsage = true
	}
	return err
}

var agentmindSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure agent telemetry export for AgentMind",
}

// agentmindSetupCodexCmd configures Codex OTEL export to AgentMind by
// editing ~/.codex/config.toml — that path stays in mindspec because it
// composes with mindspec's recording layout (spec 083 Scope: "agentmind
// setup stays in mindspec for this spec because it knows mindspec's
// .claude/ layout"; the same reasoning applies to ~/.codex/config.toml).
//
// Spec 083 Bead 5: the `--session` path (Codex JSONL → NDJSON
// conversion) moved to the standalone agentmind binary alongside the
// rest of internal/viz/. When `--session` is set, this command
// re-execs `agentmind setup codex --session …` via
// `client.RunStandalone` and applies the interactive-class
// degradation contract (Hard Constraint #4): exits non-zero if the
// binary is absent. The conversion produces user-visible NDJSON the
// caller asked for, so a silent no-op would be a correctness bug.
var agentmindSetupCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Configure Codex OTEL export, or convert a Codex session JSONL fallback",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionPath, _ := cmd.Flags().GetString("session")
		outputPath, _ := cmd.Flags().GetString("output")
		configPath, _ := cmd.Flags().GetString("config")
		force, _ := cmd.Flags().GetBool("force")

		if strings.TrimSpace(sessionPath) != "" {
			runArgs := []string{"setup", "codex", "--session", sessionPath}
			if strings.TrimSpace(outputPath) != "" {
				runArgs = append(runArgs, "--output", outputPath)
			}
			return runStandaloneWithInteractiveDegradation(cmd, runArgs)
		}

		if configPath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolving home directory: %w", err)
			}
			configPath = recording.DefaultCodexConfigPath(homeDir)
		} else {
			configPath = filepath.Clean(configPath)
		}

		result, err := recording.EnsureCodexOTLP(configPath, force)
		if err != nil {
			return err
		}

		if result.Conflict {
			fmt.Fprintf(os.Stderr, "warning: Codex OTEL endpoint already set to %q (expected %q) — not overriding\n",
				result.ExistingEndpoint, result.ExpectedEndpoint)
			fmt.Fprintln(os.Stderr, "Re-run with --force to replace the existing endpoint.")
			return nil
		}

		if result.Changed {
			fmt.Printf("Configured Codex OTEL export for AgentMind in %s\n", result.ConfigPath)
			return nil
		}

		fmt.Printf("Codex OTEL export already configured for AgentMind in %s\n", result.ConfigPath)
		return nil
	},
}

func init() {
	agentmindServeCmd.Flags().Int("otlp-port", 4318, "Port for OTLP/HTTP receiver")
	agentmindServeCmd.Flags().Int("ui-port", 8420, "Port for web UI")
	agentmindServeCmd.Flags().String("output", "", "Write events to NDJSON file (append mode)")
	agentmindServeCmd.Flags().String("bind", "127.0.0.1", "Address to bind to (use 0.0.0.0 for all interfaces)")

	agentmindReplayCmd.Flags().Float64("speed", 1, "Replay speed multiplier (1, 5, 10, or 0 for max)")
	agentmindReplayCmd.Flags().Int("ui-port", 8420, "Port for web UI")
	agentmindReplayCmd.Flags().String("spec", "", "Spec ID to replay (resolves via active docs root to specs/<id>/recording/events.ndjson)")
	agentmindReplayCmd.Flags().String("phase", "", "Filter replay to a specific phase (e.g., plan, implement)")

	agentmindSetupCodexCmd.Flags().String("config", "", "Path to Codex config.toml (default: ~/.codex/config.toml)")
	agentmindSetupCodexCmd.Flags().Bool("force", false, "Replace an existing non-AgentMind OTEL endpoint")
	agentmindSetupCodexCmd.Flags().String("session", "", "Path to Codex session JSONL to convert for fallback replay")
	agentmindSetupCodexCmd.Flags().StringP("output", "o", "", "Output NDJSON file path for --session (default: <input>-agentmind.ndjson)")
	agentmindSetupCmd.AddCommand(agentmindSetupCodexCmd)

	agentmindCmd.AddCommand(agentmindServeCmd)
	agentmindCmd.AddCommand(agentmindReplayCmd)
	agentmindCmd.AddCommand(agentmindSetupCmd)
}
