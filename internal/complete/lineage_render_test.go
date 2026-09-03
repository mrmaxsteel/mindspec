package complete

// Spec 128 final-review round 1 (R2 F1/G3): hostile-stderr pins for the
// two internal/complete render sinks the bead-9 sweep left raw — the
// lineage-lookup refusal (Run's default lineageErr branch, whose sibling
// at the malformed-metadata branch already escaped) and
// completeSyncPhase's non-timeout surfacing.

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

var hostileLineageErr = errors.New("bd show failed: exit status 1: Error 1105 (HY000): column \"depends_on_id\" could not be found\nnoise\x1b[31mmore-noise\r\nrecovery: forged")

// TestRun_LineageFailureRefusalHostileEscaped drives Run into the
// genuine lineage-lookup-failure refusal with a control-bearing chained
// error and pins the escape at the sink.
func TestRun_LineageFailureRefusalHostileEscaped(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	mock := newMockExec()
	findEpicForBeadFn = func(beadID string) (string, string, error) {
		return "", "", hostileLineageErr
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the lineage-lookup failure to refuse")
	}
	msg := err.Error()
	if !strings.Contains(msg, "epic lineage failed") {
		t.Fatalf("expected the lineage refusal, got: %v", err)
	}
	if strings.ContainsAny(msg, "\x1b\r\x00") {
		t.Errorf("a raw control byte survived into the lineage refusal: %q", msg)
	}
	for _, line := range strings.Split(msg, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line survived: %q", msg)
		}
	}
}

// TestCompleteSyncPhase_WriteFailureHostileEscaped extends the recorded
// no-manufactured-recovery disposition (complete_phaselock_test.go) with
// the escape half: a hostile merge-write failure renders with no raw
// control byte and no forged recovery line, while still surfacing the
// diagnostic.
func TestCompleteSyncPhase_WriteFailureHostileEscaped(t *testing.T) {
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	origMerge := completeMergeMetadataFn
	completeMergeMetadataFn = func(string, map[string]interface{}) error {
		return hostileLineageErr
	}
	t.Cleanup(func() { completeMergeMetadataFn = origMerge })
	root := t.TempDir()

	out := captureStderr(t, func() {
		completeSyncPhase(root, "epic-1", "plan", "mindspec-bead1")
	})
	if !strings.Contains(out, "depends_on_id") {
		t.Errorf("the bd diagnostic must survive: %q", out)
	}
	if strings.ContainsAny(out, "\x1b\r\x00") {
		t.Errorf("a raw control byte survived into completeSyncPhase's surfacing: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line survived: %q", out)
		}
	}
}
