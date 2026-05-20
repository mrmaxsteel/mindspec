package bench

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStartBenchCollector_DegradesOnAbsentBinary exercises the spec 083
// Hard Constraint #4 batch class for `bench run`: when the agentmind
// binary cannot be resolved, startBenchCollector must
//
//  1. emit exactly one canonical warn line to the supplied stderr
//     writer (via client.EmitWarnOnce / sync.Once);
//  2. return (StartResult{Degraded: true}, nil) so the bench run can
//     proceed with exit 0 — telemetry is a side-effect, not the
//     deliverable;
//  3. NOT write a PID file (no subprocess was spawned).
//
// Panel bead-3a-v1, REV-2: the spec named `bench run` as the batch-class
// exemplar; before this revision the only batch-class subtest exercised
// `bench setup` (satisfied-by-design, never calls AutoStart) which
// proved nothing about the runner.go AutoStart switch.
func TestStartBenchCollector_DegradesOnAbsentBinary(t *testing.T) {
	const warnLine = "WARN: agentmind binary not found; telemetry export will drop silently"

	// Strip everything that could let the agentmind binary be resolved.
	// AutoStart's findBinary order is $AGENTMIND_BIN -> <root>/bin/agentmind
	// -> PATH; we neutralize all three.
	emptyDir := t.TempDir()
	repoRoot := t.TempDir() // no bin/agentmind inside
	workDir := t.TempDir()

	t.Setenv("PATH", emptyDir)
	t.Setenv("AGENTMIND_BIN", "")
	t.Setenv("HOME", t.TempDir())

	eventsPath := filepath.Join(workDir, "bench-events.jsonl")

	var stdout, stderr bytes.Buffer
	res, err := startBenchCollector(repoRoot, workDir, eventsPath, &stdout, &stderr)
	if err != nil {
		t.Fatalf("startBenchCollector returned error: %v (stderr=%q)", err, stderr.String())
	}
	if !res.Degraded {
		t.Fatalf("expected StartResult.Degraded=true; got %+v", res)
	}
	if res.Started {
		t.Fatalf("expected StartResult.Started=false; got %+v", res)
	}
	if res.Reused {
		t.Fatalf("expected StartResult.Reused=false; got %+v", res)
	}
	if res.PID != 0 {
		t.Fatalf("expected zero PID on degraded path; got %d", res.PID)
	}

	// The PID file must NOT exist — nothing was spawned.
	pidFile := filepath.Join(workDir, BenchCollectorPIDFile)
	if _, statErr := os.Stat(pidFile); statErr == nil {
		t.Fatalf("expected no PID file on degraded path; found %s", pidFile)
	}

	// Note: warnOnce is package-level state in agentmind/client. Other
	// tests in the same process may have already triggered the warn
	// line, so we cannot guarantee this specific call's stderr buffer
	// got a line. What we CAN guarantee is that if a line is present,
	// it is the canonical one and there is at most one of it.
	got := strings.Count(stderr.String(), warnLine)
	if got > 1 {
		t.Fatalf("expected at most one warn line in stderr; got %d\nstderr=%q", got, stderr.String())
	}
	// Banned: substring matching the absent-binary condition via
	// err.Error() (HC#4). Defensive: assert stderr never contains the
	// raw sentinel text "agentmind binary not found" outside the
	// canonical warn line.
	rawSentinel := "agentmind binary not found"
	occurrences := strings.Count(stderr.String(), rawSentinel)
	if occurrences != got {
		t.Fatalf("stderr leaked raw sentinel text outside the canonical warn line; raw=%d warn=%d stderr=%q", occurrences, got, stderr.String())
	}
}
