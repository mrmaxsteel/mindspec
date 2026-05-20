package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrmaxsteel/agentmind/client"
	"github.com/mrmaxsteel/mindspec/internal/bench"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
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
		eventsPath, err := recording.EventsPath(root, specID)
		if err != nil {
			return err
		}
		eventCount := countLines(eventsPath)

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

// recordStartCmd is the explicit telemetry-as-output entry point for spec
// 083 Hard Constraint #4 (Test C). The recording IS the deliverable here,
// so when the agentmind binary cannot be resolved, this command:
//
//   - emits the canonical absent-binary warn line exactly once via
//     `client.EmitWarnOnce`, and
//   - returns a non-zero exit with a clear error wrapping
//     `client.ErrBinaryNotFound`.
//
// Detection uses `errors.Is(err, client.ErrBinaryNotFound)` — Hard
// Constraint #4 prohibits substring matching on error text.
var recordStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start telemetry recording for a spec (telemetry-as-output)",
	Long: `Start telemetry recording for the given spec ID.

Recording is telemetry-as-output: the agentmind binary collects OTLP
events emitted by Claude Code (or another agent runtime) and writes
NDJSON to .mindspec/specs/<id>/recording/events.ndjson. If the
agentmind binary is absent, this command exits non-zero — a silent
empty recording would be a correctness violation, not graceful
degradation (spec 083 Hard Constraint #4).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		specID, _ := cmd.Flags().GetString("spec")
		if specID == "" {
			return fmt.Errorf("--spec is required")
		}
		if err := validate.SpecID(specID); err != nil {
			return err
		}

		root, err := findLocalRoot()
		if err != nil {
			return err
		}

		if !recording.IsEnabled(root) {
			return fmt.Errorf("recording is disabled (set recording.enabled: true in .mindspec/config.yaml to enable)")
		}

		if err := recording.StartRecording(root, specID); err != nil {
			if errors.Is(err, client.ErrBinaryNotFound) {
				// Telemetry-as-output class: emit the canonical warn line
				// (sync.Once guarded — exactly one per process) AND fail
				// with non-zero exit.
				client.EmitWarnOnce(os.Stderr)
				return fmt.Errorf("recording requires the agentmind binary; install it or set $AGENTMIND_BIN: %w", err)
			}
			return err
		}

		fmt.Printf("Recording started for spec %s\n", specID)
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
	recordStartCmd.Flags().String("spec", "", "Spec ID to start recording for (required)")
	recordStatusCmd.Flags().String("spec", "", "Spec ID (defaults to active spec)")
	recordStopCmd.Flags().String("spec", "", "Spec ID (defaults to active spec)")
	recordCollectCmd.Flags().Int("port", 4318, "Port for OTLP/HTTP receiver")
	recordCollectCmd.Flags().String("output", "", "Output NDJSON file path")

	recordCmd.AddCommand(recordStartCmd)
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
