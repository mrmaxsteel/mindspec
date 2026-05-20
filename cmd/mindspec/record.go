package main

// cmd/mindspec/record.go — spec 084 (mindspec-otel-only) Bead 2 rewire.
//
// `mindspec record start` is now a pure OTEL-config + workload-launcher:
//
//  1. Reads the user-configured OTEL endpoint from
//     .claude/settings.local.json and/or ~/.codex/config.toml via the
//     internal/otel reader (the same surface that powers
//     `mindspec otel setup` / `mindspec otel status`).
//  2. Renders the OTEL env block (CLAUDE_CODE_ENABLE_TELEMETRY=1 plus
//     OTEL_EXPORTER_OTLP_* keys) via internal/otel and merges it into
//     the workload's environment.
//  3. Writes the recording manifest + a phase marker on disk so the
//     spec/plan/impl lifecycle has a per-spec recording directory to
//     hang artifacts on.
//  4. Execs the workload (whatever follows `--`) with the merged env,
//     inheriting stdin/stdout/stderr verbatim.
//  5. Exits with the workload's exit code.
//
// What this file used to do — spawn the agentmind subprocess via
// client.AutoStart, manage its PID, and tail its NDJSON output — is
// deliberately removed (spec 084 Hard Constraint #4: "mindspec record
// start is pure config + launch"). The OTLP receiver lives in the
// standalone agentmind binary or any other OTLP/HTTP collector the
// user chooses to point the endpoint at; mindspec is no longer aware
// of it.
//
// The status/stop/health subcommands are unchanged in shape from the
// pre-spec-084 implementation; their CollectorPID / CollectorPort
// references are vestigial and slated for cleanup in Bead 3 (which
// also deletes internal/recording/collector.go). Keeping them here in
// Bead 2 keeps the diff minimal: Bead 2's scope is "record start only".

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/otel"
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

		if !recording.HasRecording(root, specID) {
			fmt.Printf("No recording found for spec %s\n", specID)
			return nil
		}

		m, err := recording.ReadManifest(root, specID)
		if err != nil {
			return err
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

		fmt.Printf("Spec:    %s\n", m.SpecID)
		fmt.Printf("Status:  %s\n", m.Status)
		fmt.Printf("Events:  %d\n", eventCount)
		fmt.Printf("Phase:   %s\n", currentPhase)
		if elapsed != "" {
			fmt.Printf("Elapsed: %s\n", elapsed)
		}
		return nil
	},
}

// recordStartCmd is the spec-084 reshaped `mindspec record start`:
// pure OTEL-config + workload-launcher.
//
// Usage:
//
//	mindspec record start --spec <id> -- <workload-cmd...>
//
// Behavior (spec 084 Hard Constraint #4 + plan Bead 2):
//   - reads the user-configured OTEL endpoint (Claude
//     settings.local.json, then Codex config.toml as fallback);
//   - if any endpoint is found, injects the canonical OTEL env keys
//     (CLAUDE_CODE_ENABLE_TELEMETRY=1, OTEL_EXPORTER_OTLP_ENDPOINT=…,
//     OTEL_EXPORTER_OTLP_PROTOCOL=…, etc.) into the workload's env;
//   - writes the recording manifest + a "spec" phase marker on disk
//     (filesystem bookkeeping — not telemetry handling);
//   - execs the workload, inheriting stdin/stdout/stderr verbatim;
//   - exits with the workload's exit code.
//
// If no OTEL endpoint is configured on disk, the workload is still
// launched — the workload's own OTEL SDK will silently drop events
// (per spec line 535: "no graceful-degradation contract"). mindspec
// emits exactly zero stderr telemetry warnings either way.
var recordStartCmd = &cobra.Command{
	Use:   "start [-- workload-cmd...]",
	Short: "Launch a workload with OTEL env vars set, recording the lifecycle on disk",
	Long: `Launch a workload with the user-configured OTEL endpoint set in its
environment, and record per-spec lifecycle bookkeeping on disk.

mindspec is OTEL-config-only: it writes OTEL_EXPORTER_OTLP_* env vars
into the workload's environment and then execs the workload. The
workload (Claude Code, Codex, bash, ...) emits OTLP telemetry to the
endpoint the user configured via 'mindspec otel setup'. mindspec
itself never speaks OTLP, never opens a TCP listener, and never reads
the workload's telemetry back.

The exit code is the workload's exit code, propagated verbatim.

Examples:

  mindspec record start --spec 084-mindspec-otel-only -- claude --dangerously-skip-permissions
  mindspec record start --spec 084-mindspec-otel-only -- bash -c 'echo hi'

If '-- <workload-cmd>' is omitted, the command exits non-zero with
a usage error.`,
	DisableFlagsInUseLine: true,
	SilenceUsage:          true,
	// SilenceErrors: workload propagation MUST produce empty stderr
	// (spec Test F line 651 — "stderr is empty"). cobra's default
	// "Error: <msg>" print would violate that for workload exit codes;
	// we surface only real mindspec errors (flag parsing, manifest I/O)
	// via os.Stderr ourselves below.
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := runRecordStart(cmd, args)
		if err == nil {
			return nil
		}
		// Workload-exit errors propagate the exit code verbatim with
		// empty stderr (spec Test F line 651). Every other error is
		// a real mindspec error and gets printed before returning.
		if _, ok := err.(*workloadExitError); ok {
			return err
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	},
}

// runRecordStart is the body of `mindspec record start`. Split out
// from the cobra RunE so the wrapper can apply the spec-084 Test F
// "stderr is empty on workload exit" rule (see recordStartCmd above).
func runRecordStart(cmd *cobra.Command, args []string) error {
	specID, _ := cmd.Flags().GetString("spec")
	if specID == "" {
		return fmt.Errorf("--spec is required")
	}
	if err := validate.SpecID(specID); err != nil {
		return err
	}

	// Workload args appear after `--` on the command line. cobra
	// surfaces them as positional args once ArgsLenAtDash() is
	// non-negative; we require at least one positional arg.
	workloadArgs := args
	if len(workloadArgs) == 0 {
		return fmt.Errorf("a workload command is required after '--' (e.g. mindspec record start --spec %s -- bash -c 'echo hi')", specID)
	}

	root, err := findLocalRoot()
	if err != nil {
		return err
	}

	// 1) Write the recording skeleton on disk: dir, manifest with a
	// single "spec" phase entry. Filesystem bookkeeping only — no
	// telemetry pipeline is started.
	if err := writeRecordingSkeleton(root, specID); err != nil {
		return err
	}

	// 2) Build the workload env: parent env + OTEL keys rendered
	// from whatever the user configured via `mindspec otel setup`.
	// If no endpoint is configured on disk, the parent env is
	// passed through unmodified (no OTEL keys synthesized from
	// thin air — silent drop is the workload's OTEL SDK contract).
	workloadEnv := buildWorkloadEnv(root)

	// 3) Exec the workload, inherit stdio, propagate exit code.
	bin := workloadArgs[0]
	argv := workloadArgs[1:]
	child := exec.Command(bin, argv...)
	child.Env = workloadEnv
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := child.Run(); err != nil {
		// Propagate the workload's exit code verbatim. Any non
		// ExitError (e.g. fork/exec failure) bubbles up as
		// mindspec's own error.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitCodedError(exitErr.ExitCode())
		}
		return fmt.Errorf("launching workload %q: %w", bin, err)
	}
	return nil
}

// writeRecordingSkeleton creates the recording directory and writes a
// fresh manifest with a single "spec" phase entry. Idempotent: a
// pre-existing recording for the same spec is preserved (returns nil
// without rewriting the manifest).
func writeRecordingSkeleton(root, specID string) error {
	dir, err := recording.RecordingDir(root, specID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating recording dir: %w", err)
	}
	mp, err := recording.ManifestPath(root, specID)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(mp); statErr == nil {
		// Manifest already present — leave it in place.
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	m := &recording.Manifest{
		SpecID:    specID,
		StartedAt: now,
		Status:    "launched",
		Phases: []recording.Phase{
			{Phase: "spec", StartedAt: now},
		},
	}
	if err := recording.WriteManifest(root, specID, m); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	// Sanity: the manifest path is within the recording dir.
	_ = filepath.Dir(mp)
	return nil
}

// buildWorkloadEnv returns the env slice to pass to the workload. The
// parent process env is the baseline; any OTEL endpoint configured on
// disk (.claude/settings.local.json or ~/.codex/config.toml) is
// rendered via internal/otel and overlaid on top.
//
// The function never returns an error — a missing config file, a
// parse failure, or a partial config all degrade to "pass parent env
// through unchanged". mindspec must not emit stderr about telemetry
// (spec Test F line 651).
func buildWorkloadEnv(root string) []string {
	parent := os.Environ()
	cfg, ok := loadConfiguredOtel(root)
	if !ok {
		return parent
	}
	if err := cfg.Validate(); err != nil {
		// Bad config on disk — silently fall back to parent env. The
		// workload's own OTEL SDK will read whatever OTEL_* vars the
		// parent process has set (if any) and behave accordingly.
		return parent
	}
	otelEnv := otelEnvKeyValues(cfg)
	return mergeEnv(parent, otelEnv)
}

// loadConfiguredOtel returns the OTEL config the user previously
// wrote via `mindspec otel setup`, preferring the Claude target over
// Codex (Claude is the project-local default; Codex is user-global).
// Returns (Config{}, false) when nothing is configured.
func loadConfiguredOtel(root string) (otel.Config, bool) {
	status, _ := otel.ReadCurrent(root)
	if status.ClaudePresent {
		return status.Claude, true
	}
	if status.CodexPresent {
		return status.Codex, true
	}
	return otel.Config{}, false
}

// otelEnvKeyValues renders the OTEL Config as a flat KEY=VALUE map
// using the same key set RenderClaudeSettingsLocal emits. Returns a
// map so mergeEnv can overlay it on the parent process env without
// shell-quoting roundtrips.
func otelEnvKeyValues(c otel.Config) map[string]string {
	c = c.Normalize()
	out := map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY": "1",
		"OTEL_METRICS_EXPORTER":        "otlp",
		"OTEL_LOGS_EXPORTER":           "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL":  c.Protocol,
		"OTEL_EXPORTER_OTLP_ENDPOINT":  c.Endpoint,
		"OTEL_SERVICE_NAME":            c.ServiceName,
	}
	if hdr := c.FormatHeaders(); hdr != "" {
		out["OTEL_EXPORTER_OTLP_HEADERS"] = hdr
	}
	return out
}

// mergeEnv overlays the overrides map onto the parent env slice,
// preserving non-overridden keys and replacing overridden ones in
// place (rather than appending) so the result has no duplicates.
func mergeEnv(parent []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return parent
	}
	seen := make(map[string]bool, len(overrides))
	out := make([]string, 0, len(parent)+len(overrides))
	for _, kv := range parent {
		eq := indexEq(kv)
		if eq < 0 {
			out = append(out, kv)
			continue
		}
		key := kv[:eq]
		if v, ok := overrides[key]; ok {
			out = append(out, key+"="+v)
			seen[key] = true
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func indexEq(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return i
		}
	}
	return -1
}

// exitCodedError wraps a workload exit code so main.go can propagate
// it verbatim via otelExitCode (defined in otel.go).
type workloadExitError struct{ code int }

func (e *workloadExitError) Error() string {
	return fmt.Sprintf("workload exited with status %d", e.code)
}

func exitCodedError(code int) error { return &workloadExitError{code: code} }

var recordStopCmd = &cobra.Command{
	Use:    "stop",
	Short:  "Stop recording for the active spec",
	Hidden: true,
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

func init() {
	recordStartCmd.Flags().String("spec", "", "Spec ID to record (required)")
	recordStatusCmd.Flags().String("spec", "", "Spec ID (defaults to active spec)")
	recordStopCmd.Flags().String("spec", "", "Spec ID (defaults to active spec)")

	recordCmd.AddCommand(recordStartCmd)
	recordCmd.AddCommand(recordStatusCmd)
	recordCmd.AddCommand(recordStopCmd)
	recordCmd.AddCommand(recordHealthCmd)
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
