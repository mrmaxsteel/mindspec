package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/viz"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var agentmindCmd = &cobra.Command{
	Use:     "agentmind",
	Aliases: []string{"viz"},
	Short:   "AgentMind — real-time 3D visualization of agent activity",
	Long: `Launch a local web server that renders agent activity as an interactive
3D force-directed graph with a starfield aesthetic.

Subcommands:
  serve   Start OTLP receiver + web UI for real-time visualization
  replay  Replay a recorded NDJSON session file
  setup   Configure agent telemetry export to AgentMind`,
}

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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		return viz.RunLive(ctx, otlpPort, uiPort, output)
	},
}

var agentmindReplayCmd = &cobra.Command{
	Use:   "replay [file.jsonl]",
	Short: "Replay a recorded NDJSON session file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID, _ := cmd.Flags().GetString("spec")
		phase, _ := cmd.Flags().GetString("phase")
		speed, _ := cmd.Flags().GetFloat64("speed")
		uiPort, _ := cmd.Flags().GetInt("ui-port")

		// Resolve file path
		var filePath string
		if len(args) > 0 {
			filePath = args[0]
		} else if specID != "" {
			root, err := findRoot()
			if err != nil {
				return fmt.Errorf("finding project root: %w", err)
			}
			filePath = recording.EventsPath(root, specID)
			if _, err := os.Stat(filePath); err != nil {
				// Also try resolving via workspace
				specDir := workspace.SpecDir(root, specID)
				filePath = specDir + "/recording/events.ndjson"
				if _, err := os.Stat(filePath); err != nil {
					return fmt.Errorf("no recording found for spec %s", specID)
				}
			}
		} else {
			return fmt.Errorf("provide a file path or use --spec <id>")
		}

		if speed <= 0 {
			speed = 0
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		return viz.RunReplay(ctx, filePath, speed, uiPort, phase)
	},
}

var agentmindSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure agent telemetry export for AgentMind",
}

var agentmindSetupCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Configure Codex OTEL export to AgentMind (OTLP/HTTP localhost:4318)",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		force, _ := cmd.Flags().GetBool("force")

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

	agentmindReplayCmd.Flags().Float64("speed", 1, "Replay speed multiplier (1, 5, 10, or 0 for max)")
	agentmindReplayCmd.Flags().Int("ui-port", 8420, "Port for web UI")
	agentmindReplayCmd.Flags().String("spec", "", "Spec ID to replay (resolves to docs/specs/<id>/recording/events.ndjson)")
	agentmindReplayCmd.Flags().String("phase", "", "Filter replay to a specific phase (e.g., plan, implement)")

	agentmindSetupCodexCmd.Flags().String("config", "", "Path to Codex config.toml (default: ~/.codex/config.toml)")
	agentmindSetupCodexCmd.Flags().Bool("force", false, "Replace an existing non-AgentMind OTEL endpoint")
	agentmindSetupCmd.AddCommand(agentmindSetupCodexCmd)

	agentmindCmd.AddCommand(agentmindServeCmd)
	agentmindCmd.AddCommand(agentmindReplayCmd)
	agentmindCmd.AddCommand(agentmindSetupCmd)
}
