package bench

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/agentmind/client"
)

// BenchCollectorPIDFile is the basename of the file where the bench
// path persists the spawned agentmind subprocess PID so a later
// teardown step (or a curious operator) can locate and stop it.
const BenchCollectorPIDFile = "bench-agentmind.pid"

// StartResult describes the outcome of starting the bench-side
// agentmind collector. Exactly one of Started, Reused, or Degraded
// is true on a nil-error return.
type StartResult struct {
	// Started is true when AutoStart spawned a new agentmind subprocess.
	// In that case PID is the OS process ID of the new subprocess and
	// the value was written to <workDir>/bench-agentmind.pid.
	Started bool
	// Reused is true when AutoStart detected an existing agentmind
	// instance already listening on the OTLP port and did not spawn.
	Reused bool
	// Degraded is true when the agentmind binary could not be resolved
	// (client.ErrBinaryNotFound). The warn line has been emitted (once
	// per process) and the bench run continues without telemetry —
	// spec 083 Hard Constraint #4 batch class.
	Degraded bool
	// PID is the OS process ID of the spawned agentmind subprocess
	// (zero when Reused or Degraded).
	PID int
	// Consumer is the live NDJSON event-stream consumer attached to
	// the subprocess stdout pipe via `client.ReadEvents`. Non-nil
	// only when Started is true (Reused has no inherited stdout pipe;
	// Degraded has no subprocess at all). Spec 083 Bead 3b's
	// read-side rewire — the consumer forwards each
	// `wire.CollectedEvent` decoded from
	// `Handle.Stdout` to the same on-disk NDJSON file the bench
	// runner aggregations expect.
	Consumer *StreamConsumer
}

// RunStartCollectorForTest is the test-only entry point that lets
// out-of-package tests (e.g., cmd/mindspec/degradation_test.go) exercise
// the bench-side AutoStart degradation switch without going through the
// full `bench run` subprocess path (which requires claude on PATH, a
// clean git tree, and bin/mindspec).
//
// Production code MUST call startBenchCollector directly via Run; this
// function exists solely to let panel REV-2's `batch_bench_run_*`
// subtest assert the per-class contract from cmd/mindspec without
// re-implementing the AutoStart switch.
//
// Returns a non-nil error only when AutoStart returns a non-sentinel
// error (the batch class degrades to nil on ErrBinaryNotFound).
func RunStartCollectorForTest(repoRoot, workDir, eventsPath string, stdout, stderrWriter io.Writer) error {
	_, err := startBenchCollector(repoRoot, workDir, eventsPath, stdout, stderrWriter)
	return err
}

// startBenchCollector launches agentmind for the bench run and applies
// the spec 083 Hard Constraint #4 batch-class degradation contract.
//
// On success (AutoStart returned nil):
//   - If a new subprocess was spawned, persist its PID to
//     <workDir>/bench-agentmind.pid so a later teardown step can stop
//     it (panel bead-3a-v1, REV-4 — the previous code spawned and
//     forgot the PID).
//   - Print the "started (PID …)" or "already running" line on stdout.
//
// On client.ErrBinaryNotFound (detected via errors.Is — substring
// matching on err.Error() is forbidden by HC#4): emit the centralized
// warn line via client.EmitWarnOnce on stderrWriter and return with
// (Degraded:true, nil). The bench continues with exit 0; telemetry is
// a side-effect for the bench command, not the deliverable.
//
// On any other AutoStart error: wrap and return.
func startBenchCollector(repoRoot, workDir, eventsPath string, stdout, stderrWriter io.Writer) (StartResult, error) {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderrWriter == nil {
		stderrWriter = os.Stderr
	}

	handle, err := client.AutoStart(repoRoot, agentMindPort, client.DefaultUIPort, eventsPath)
	switch {
	case err == nil:
		res := StartResult{}
		if handle != nil && handle.PID > 0 {
			res.Started = true
			res.PID = handle.PID
			// REV-4 (panel bead-3a-v1): persist the spawned PID so a
			// later teardown step has a handle to stop the collector.
			// A tiny PID file is sufficient — the bench WorkDir is
			// ephemeral and a single integer is all we need.
			pidFile := filepath.Join(workDir, BenchCollectorPIDFile)
			if writeErr := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", handle.PID)), 0o644); writeErr != nil {
				// Persistence failure must not silently swallow — but
				// it also must not abort the bench. Surface a warning
				// on stderr and continue.
				fmt.Fprintf(stderrWriter, "warning: could not persist agentmind PID to %s: %v\n", pidFile, writeErr)
			}
			// Bead 3b: wire the subprocess stdout pipe through
			// client.ReadEvents into the on-disk NDJSON file the
			// post-run aggregations consume. The io.Reader fed to
			// client.ReadEvents is handle.Stdout (subprocess stdout
			// pipe) — NEVER an os.Open on eventsPath (Hard Constraint
			// #3: outbound channel is stdout-pipe NDJSON, NOT
			// file-tail).
			consumer, consumeErr := ConsumeHandleToFile(handle, eventsPath)
			if consumeErr != nil {
				return StartResult{}, fmt.Errorf("starting bench event-stream consumer: %w", consumeErr)
			}
			res.Consumer = consumer
			fmt.Fprintf(stdout, "AgentMind started (PID %d) → %s\n", handle.PID, eventsPath)
		} else {
			res.Reused = true
			fmt.Fprintf(stdout, "AgentMind already running on :%d\n", agentMindPort)
		}
		fmt.Fprintf(stdout, "Watch live at http://localhost:%d\n", client.DefaultUIPort)
		return res, nil
	case errors.Is(err, client.ErrBinaryNotFound):
		// Batch class: degrade gracefully. EmitWarnOnce is the sync.Once
		// guarded helper that guarantees exactly one warn line per
		// process even if multiple consumers also call it.
		client.EmitWarnOnce(stderrWriter)
		return StartResult{Degraded: true}, nil
	default:
		return StartResult{}, fmt.Errorf("starting AgentMind collector: %w", err)
	}
}
