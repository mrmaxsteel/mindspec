package phase_test

// Spec 092 Bead 3 (Req 2): the stored-vs-derived consistency warning
// emitted by DerivePhaseWithStatus must end with a
// `recovery: mindspec repair phase <spec-id>` line.
//
// This is the per-site recovery-convention test for the phase package
// (the Req 21 mirror described in
// internal/guard/recovery_convention_test.go). It lives in an EXTERNAL
// test package because internal/guard imports internal/phase — the
// warning in derive.go is hand-formatted for that reason, and this
// test keeps the hand-formatted line aligned with
// guard.HasFinalRecoveryLine and the Req 19 banned-command check.

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func captureStderrPhase(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stderr = orig }()
	fn()
	w.Close()
	os.Stderr = orig
	return <-done
}

func TestConsistencyWarningEndsWithRepairRecoveryLine(t *testing.T) {
	// Stored phase "implement" disagrees with child-derived "review"
	// (all children closed) → the warning fires.
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"b1","title":"bead","status":"closed","issue_type":"task"}]`), nil
	})
	defer restoreList()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "show" {
			return []byte(`[{"id":"epic-1","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"implement","spec_num":10,"spec_title":"test"}}]`), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	var got string
	var err error
	stderr := captureStderrPhase(t, func() {
		got, err = phase.DerivePhaseWithStatus("epic-1", "open")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Stored phase is still trusted by DerivePhase (the heal is the
	// gate's / repair's job, not the read path's).
	if got != "implement" {
		t.Errorf("DerivePhaseWithStatus = %q, want implement (stored trusted)", got)
	}

	warning := strings.TrimRight(stderr, "\n")
	if !strings.Contains(warning, "disagrees with child-derived phase") {
		t.Fatalf("expected the consistency warning to fire; stderr=%q", stderr)
	}
	// Req 2: warning ends with the repair recovery command, resolved to
	// the concrete spec ID so it is copy-pastable.
	if !guard.HasFinalRecoveryLine(warning) {
		t.Errorf("consistency warning must end with a recovery line (Req 2/12): %q", warning)
	}
	if !strings.Contains(warning, "recovery: mindspec repair phase 010-test") {
		t.Errorf("warning missing `recovery: mindspec repair phase 010-test`: %q", warning)
	}
	// Req 19: never a raw bd metadata-update command.
	if guard.IsBannedRecoveryCommand(warning) || strings.Contains(warning, "bd update --metadata") {
		t.Errorf("warning emits a banned raw metadata command: %q", warning)
	}
}

func TestConsistencyWarningSilentWhenPhasesAgree(t *testing.T) {
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"b1","title":"bead","status":"in_progress","issue_type":"task"}]`), nil
	})
	defer restoreList()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "show" {
			return []byte(`[{"id":"epic-1","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"implement","spec_num":10,"spec_title":"test"}}]`), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	stderr := captureStderrPhase(t, func() {
		_, _ = phase.DerivePhaseWithStatus("epic-1", "open")
	})
	if strings.Contains(stderr, "recovery:") || strings.Contains(stderr, "disagrees") {
		t.Errorf("no warning expected when stored and derived agree; stderr=%q", stderr)
	}
}
