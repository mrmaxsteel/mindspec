package instruct

// run_mode_error_test.go: spec 128 Bead 3 round-1 panel G4 (MAJOR) — the
// bead's ResolveModeWithCache contract change (a genuine discovery error
// now returns "" instead of ModeIdle, R1.4) left run.go's `mode, _ :=`
// call site swallowing that error, so an empty Mode reached Render's
// ""+".md" template lookup and failed with an accidental, confusing
// error instead of graceful guidance. Disclosed scope addition (run.go
// is in no bead's declared file list): the fix falls back to the same
// no-state handling already used for a resolution failure just above it.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// TestRun_ModeResolutionFailure_FallsBackGracefully is G4's red-on-revert
// pin: with an explicit --spec pass-through (so ResolveTargetWithCache
// succeeds trivially without any bd call) and bd itself unreachable for
// the subsequent mode derivation, `mindspec instruct` must render the
// graceful no-state fallback — never the accidental "loading template
// .md" failure an empty Mode would otherwise cause.
func TestRun_ModeResolutionFailure_FallsBackGracefully(t *testing.T) {
	root := setupRunTestProject(t)

	wantErr := errors.New("bd: transient flake")
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "042-fake-spec", &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "loading template") {
		t.Fatalf("Run surfaced the accidental empty-mode template-load failure instead of falling back gracefully:\n%s", out)
	}
	if !strings.Contains(out, "No Active Work") {
		t.Errorf("expected the graceful no-state fallback (idle template heading), got:\n%s", out)
	}
}
