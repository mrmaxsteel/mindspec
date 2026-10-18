package instruct

// run_mode_error_test.go: spec 128 Bead 3 round-1 panel G4 (MAJOR) — the
// bead's ResolveModeWithCache contract change (a genuine discovery error
// now returns "" instead of ModeIdle, R1.4) left run.go's `mode, _ :=`
// call site swallowing that error, so an empty Mode reached Render's
// ""+".md" template lookup and failed with an accidental, confusing
// error instead of graceful guidance. Disclosed scope addition (run.go
// is in no bead's declared file list): the fix falls back to the same
// no-state handling already used for a resolution failure just above it.
//
// Bead 5 (R2.3) superseded that interim "no-state" fallback: a genuine
// bd/Dolt discovery failure now renders the honest indeterminate form,
// never idle — presenting a failure to derive state as a derived idle
// state was exactly the class of bug R1.4/R2.3 exist to close. This
// file's test was updated in lockstep (never rendering idle here is the
// bead 5 fix, not a regression of G4's original accidental-crash fix,
// which is separately pinned by the "loading template" absence check).

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// TestRun_ModeResolutionFailure_RendersIndeterminate is G4's red-on-revert
// pin, updated for bead 5 (R2.3): with an explicit --spec pass-through
// (so ResolveTargetWithCache succeeds trivially without any bd call) and
// bd itself unreachable for the subsequent mode derivation, `mindspec
// instruct` must render the honest indeterminate form — never the
// accidental "loading template .md" failure an empty Mode would
// otherwise cause, and never a fabricated idle template either.
func TestRun_ModeResolutionFailure_RendersIndeterminate(t *testing.T) {
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
		t.Fatalf("Run surfaced the accidental empty-mode template-load failure instead of the indeterminate form:\n%s", out)
	}
	if strings.Contains(out, "No Active Work") {
		t.Errorf("a genuine discovery failure must never render idle.md (spec 128 R2.3); got:\n%s", out)
	}
	if !strings.Contains(out, "cannot determine mode") {
		t.Errorf("expected the indeterminate marker text, got:\n%s", out)
	}
	if !strings.Contains(out, wantErr.Error()) {
		t.Errorf("expected the real discovery-failure reason %q in the output, got:\n%s", wantErr.Error(), out)
	}
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "recovery: mindspec doctor") {
		t.Errorf("expected the output to end with a copy-pastable recovery line, got:\n%s", out)
	}
}
