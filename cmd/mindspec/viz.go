package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/mindspec/mindspec/internal/viz"
	"github.com/spf13/cobra"
)

var vizCmd = &cobra.Command{
	Use:   "viz",
	Short: "Real-time 3D visualization of agent activity",
	Long: `Launch a local web server that renders agent activity as an interactive
3D force-directed graph with a starfield aesthetic.

Subcommands:
  live    Start OTLP receiver + web UI for real-time visualization
  replay  Replay a recorded NDJSON session file`,
}

var vizLiveCmd = &cobra.Command{
	Use:   "live",
	Short: "Start OTLP receiver + web UI for real-time visualization",
	Long: `Start an OTLP/HTTP receiver and web UI server. Configure Claude Code with:
  export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:<otlp-port>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		otlpPort, _ := cmd.Flags().GetInt("otlp-port")
		uiPort, _ := cmd.Flags().GetInt("ui-port")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		return viz.RunLive(ctx, otlpPort, uiPort)
	},
}

var vizReplayCmd = &cobra.Command{
	Use:   "replay <file.jsonl>",
	Short: "Replay a recorded NDJSON session file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		speed, _ := cmd.Flags().GetFloat64("speed")
		uiPort, _ := cmd.Flags().GetInt("ui-port")

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

		return viz.RunReplay(ctx, filePath, speed, uiPort)
	},
}

func init() {
	vizLiveCmd.Flags().Int("otlp-port", 4318, "Port for OTLP/HTTP receiver")
	vizLiveCmd.Flags().Int("ui-port", 8420, "Port for web UI")

	vizReplayCmd.Flags().Float64("speed", 1, "Replay speed multiplier (1, 5, 10, or 0 for max)")
	vizReplayCmd.Flags().Int("ui-port", 8420, "Port for web UI")

	vizCmd.AddCommand(vizLiveCmd)
	vizCmd.AddCommand(vizReplayCmd)
}
