package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bench"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Manage per-spec telemetry recording",
}

var recordStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show recording status for the active spec",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findLocalRoot()
		if err != nil {
			return err
		}

		specID, _ := cmd.Flags().GetString("spec")
		if specID == "" {
			// ADR-0023: derive active spec from beads, not focus file.
			ctx, ctxErr := phase.ResolveContextFromDir(root, root)
			if ctxErr != nil || ctx == nil || ctx.SpecID == "" {
				return fmt.Errorf("no active spec — use --spec to specify one")
			}
			specID = ctx.SpecID
		}

		if !recording.IsEnabled(root) {
			fmt.Println("Recording disabled (set recording.enabled: true in .mindspec/config.yaml to enable)")
			return nil
		}

		if !recording.HasRecording(root, specID) {
			fmt.Printf("No recording found for spec %s\n", specID)
			return nil
		}

		m, err := recording.ReadManifest(root, specID)
		if err != nil {
			return err
		}

		health, _ := recording.HealthCheck(root, specID)
		var healthStr string
		switch health {
		case recording.HealthAlive:
			healthStr = "alive"
		case recording.HealthDead:
			healthStr = "dead"
		default:
			healthStr = "not running"
		}

		// Count events
		eventCount := countLines(recording.EventsPath(root, specID))

		// Current phase
		currentPhase := "unknown"
		if len(m.Phases) > 0 {
			currentPhase = m.Phases[len(m.Phases)-1].Phase
		}

		// Elapsed time
		var elapsed string
		if t, err := time.Parse(time.RFC3339, m.StartedAt); err == nil {
			elapsed = time.Since(t).Truncate(time.Second).String()
		}

		fmt.Printf("Spec:       %s\n", m.SpecID)
		fmt.Printf("Status:     %s\n", m.Status)
		fmt.Printf("Collector:  %s (PID %d, port %d)\n", healthStr, m.CollectorPID, m.CollectorPort)
		fmt.Printf("Events:     %d\n", eventCount)
		fmt.Printf("Phase:      %s\n", currentPhase)
		if elapsed != "" {
			fmt.Printf("Elapsed:    %s\n", elapsed)
		}
		return nil
	},
}

var recordStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop recording for the active spec",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findLocalRoot()
		if err != nil {
			return err
		}

		specID, _ := cmd.Flags().GetString("spec")
		if specID == "" {
			ctx, ctxErr := phase.ResolveContextFromDir(root, root)
			if ctxErr != nil || ctx == nil || ctx.SpecID == "" {
				return fmt.Errorf("no active spec — use --spec to specify one")
			}
			specID = ctx.SpecID
		}

		if err := recording.StopRecording(root, specID); err != nil {
			return err
		}
		fmt.Printf("Recording stopped for spec %s\n", specID)
		return nil
	},
}

var recordHealthCmd = &cobra.Command{
	Use:    "health",
	Short:  "Check and restart dead collector (for SessionStart hook)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findLocalRoot()
		if err != nil {
			return nil // silent exit if no project root
		}

		// ADR-0023: derive active spec from beads, not focus file.
		ctx, ctxErr := phase.ResolveContextFromDir(root, root)
		if ctxErr != nil || ctx == nil || ctx.SpecID == "" || ctx.Phase == state.ModeIdle {
			return nil // no active spec
		}

		return recording.RestartIfDead(root, ctx.SpecID)
	},
}

var recordCollectCmd = &cobra.Command{
	Use:    "collect",
	Short:  "Run OTLP collector (internal — used by recording subsystem)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Deprecated: use 'mindspec agentmind serve --output <path>' instead")
		port, _ := cmd.Flags().GetInt("port")
		output, _ := cmd.Flags().GetString("output")

		if output == "" {
			return fmt.Errorf("--output is required")
		}

		collector := bench.NewCollectorAppend(port, output)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		return collector.Run(ctx)
	},
}

func init() {
	recordStatusCmd.Flags().String("spec", "", "Spec ID (defaults to active spec)")
	recordStopCmd.Flags().String("spec", "", "Spec ID (defaults to active spec)")
	recordCollectCmd.Flags().Int("port", 4318, "Port for OTLP/HTTP receiver")
	recordCollectCmd.Flags().String("output", "", "Output NDJSON file path")

	recordCmd.AddCommand(recordStatusCmd)
	recordCmd.AddCommand(recordStopCmd)
	recordCmd.AddCommand(recordHealthCmd)
	recordCmd.AddCommand(recordCollectCmd)
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count
}
