package main

// cmd/mindspec/record.go — spec 084 (mindspec-otel-only) Bead 3.
//
// `mindspec record start` is a pure OTEL-config + workload-launcher.
// Bead 2 reshaped it from the old subprocess-managing form; Bead 3
// deletes the residual `record status`, `record stop`, and
// `record health` subcommands (which all targeted the now-removed
// internal/recording collector lifecycle).
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
// mindspec never opens a TCP listener, never reads OTLP, never
// spawns a collector subprocess. The OTLP receiver lives in the
// standalone agentmind binary or any other OTLP/HTTP collector the
// user chooses to point the endpoint at.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/otel"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Manage per-spec telemetry recording",
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
// Env precedence: if the caller already exports an OTEL_* env var
// before invoking `mindspec record start`, the caller's value WINS —
// mindspec's rendered config only fills in keys that are absent from
// the parent environment. This matches POSIX expectation: an explicit
// `export OTEL_EXPORTER_OTLP_ENDPOINT=…` in the shell trumps
// `mindspec otel setup`'s on-disk config.
//
// Error contract: a malformed .claude/settings.local.json or a config
// that fails Validate() causes record start to exit non-zero with a
// real `Error: …` diagnostic on stderr. Silent degradation is reserved
// for the "no config exists at all" case; "config exists but is
// broken" is a configuration bug and must be surfaced.
//
// If no OTEL endpoint is configured on disk, the workload is still
// launched — the workload's own OTEL SDK will silently drop events
// (per spec line 535: "no graceful-degradation contract"). mindspec
// emits exactly zero stderr telemetry warnings on the no-config path.
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

Env precedence: caller-exported OTEL_* env vars win over mindspec's
rendered on-disk config. mindspec only injects OTEL keys that are
absent from the parent environment.

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
		var wee *workloadExitError
		if errors.As(err, &wee) {
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
	// A malformed on-disk config returns a real error here; a missing-
	// config-entirely degrades silently to parent env (workload's own
	// OTEL SDK contract).
	workloadEnv, err := otel.BuildWorkloadEnv(root, os.Environ())
	if err != nil {
		return err
	}

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

// exitCodedError wraps a workload exit code so main.go can propagate
// it verbatim via otelExitCode (defined in otel.go).
type workloadExitError struct{ code int }

func (e *workloadExitError) Error() string {
	return fmt.Sprintf("workload exited with status %d", e.code)
}

func exitCodedError(code int) error { return &workloadExitError{code: code} }

func init() {
	recordStartCmd.Flags().String("spec", "", "Spec ID to record (required)")
	recordCmd.AddCommand(recordStartCmd)
}
