package next

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// --- AC-11 (R6.4): bd stderr passthrough with safe rendering ---
//
// These tests exercise the FULL production chain — bead.RunBD (the real
// binding of runBDFn) shelling out to a PATH-shimmed fake `bd` — rather
// than mocking runBDFn's return value, so they pin the actual seam AC-11
// names ("a failing bd shell-out ... produces a mindspec error containing
// that stderr line") end to end: internal/bead's tracedOutput/wrapBDError
// stderr capture THROUGH internal/next's QueryReady render-boundary
// escaping.

// writeFakeBD writes an executable POSIX shell script named "bd" into a
// fresh temp dir and prepends that dir to PATH for the duration of the
// calling test, so bead.RunBD's `exec.Command("bd", ...)` resolves to the
// fixture instead of any real bd on the host. Restored via t.Cleanup.
func writeFakeBD(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-bd PATH shim uses a POSIX shell script")
	}
	dir := t.TempDir()
	bdPath := filepath.Join(dir, "bd")
	if err := os.WriteFile(bdPath, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("writing fake bd fixture: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setting PATH: %v", err)
	}
}

// TestQueryReady_AC11ContentLeg pins AC-11's content leg: a fake bd
// exiting 1 with the #146 schema-error line on stderr produces a
// QueryReady error containing that line — not the bare, content-free
// "exit status 1" the pre-fix code discarded stderr down to
// (internal/next/beads.go:38,54 before this bead, per plan.md's LIVE
// residue #3).
func TestQueryReady_AC11ContentLeg(t *testing.T) {
	origRunBD := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRunBD })
	readyJSONFn = bead.ReadyJSON

	writeFakeBD(t, `echo 'Error 1105 (HY000): column "depends_on_id" could not be found' >&2
exit 1`)

	_, err := QueryReady()
	if err == nil {
		t.Fatal("expected an error from the failing fake bd")
	}
	if !strings.Contains(err.Error(), "depends_on_id") {
		t.Errorf("expected the bd schema-error line to survive into QueryReady's error, got: %v", err)
	}
}

// TestQueryReadyForEpic_AC11ContentLeg is TestQueryReady_AC11ContentLeg's
// sibling for the epic-scoped query path (the other half of plan.md's
// LIVE residue #3 citation).
func TestQueryReadyForEpic_AC11ContentLeg(t *testing.T) {
	origRunBD := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRunBD })
	readyJSONFn = bead.ReadyJSON

	writeFakeBD(t, `echo 'Error 1105 (HY000): column "depends_on_id" could not be found' >&2
exit 1`)

	_, err := QueryReadyForEpic("mindspec-9cyu")
	if err == nil {
		t.Fatal("expected an error from the failing fake bd")
	}
	if !strings.Contains(err.Error(), "depends_on_id") {
		t.Errorf("expected the bd schema-error line to survive into QueryReadyForEpic's error, got: %v", err)
	}
}

// TestQueryReady_AC11RenderingLeg pins AC-11's rendering leg (round-2 G4,
// ADR-0042): the fake bd's stderr is multi-line, with the legitimate
// schema-error line PLUS a second, control-bearing line (ESC + CR). The
// schema-error text must still survive, but no raw control byte may
// reach the rendered error — an unsafe raw-passthrough implementation
// (e.g. reverting renderBDError to a bare %w wrap of the already-
// stderr-carrying error) passes the content leg above while failing this
// one, which is the whole point of asserting them separately.
func TestQueryReady_AC11RenderingLeg(t *testing.T) {
	origRunBD := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRunBD })
	readyJSONFn = bead.ReadyJSON

	writeFakeBD(t, `printf 'Error 1105 (HY000): column "depends_on_id" could not be found\n' >&2
printf 'noise\033[31mmore-noise\r\n' >&2
exit 1`)

	_, err := QueryReady()
	if err == nil {
		t.Fatal("expected an error from the failing fake bd")
	}
	msg := err.Error()
	if !strings.Contains(msg, "depends_on_id") {
		t.Errorf("schema-error text must survive rendering, got: %q", msg)
	}
	if strings.ContainsAny(msg, "\x1b\r\x00") {
		t.Errorf("a raw control byte reached the rendered error: %q", msg)
	}
}

// TestFetchBeadByID_AC11RenderingLeg is round-2 panel's red-on-revert pin
// for FetchBeadByID's own render leg — distinct from
// TestFetchBeadAsOf_IsUnsupportedFlagErrorClassifiesThroughWrapper below,
// which pins the CHAIN but would stay green even if the bdRenderedError
// wrapper were dropped from FetchBeadByID's call site (reverting to a
// bare %w of err). This test fails specifically on that regression: a
// fake bd emitting a control-bearing stderr line must not leak the raw
// control byte into FetchBeadByID's rendered error text.
func TestFetchBeadByID_AC11RenderingLeg(t *testing.T) {
	origRunBD := runBDFn
	t.Cleanup(func() { runBDFn = origRunBD })
	runBDFn = bead.RunBD

	writeFakeBD(t, `printf 'Error 1105 (HY000): column "depends_on_id" could not be found\n' >&2
printf 'noise\033[31mmore-noise\r\n' >&2
exit 1`)

	_, err := FetchBeadByID("mindspec-abc")
	if err == nil {
		t.Fatal("expected an error from the failing fake bd")
	}
	msg := err.Error()
	if !strings.Contains(msg, "depends_on_id") {
		t.Errorf("schema-error text must survive rendering, got: %q", msg)
	}
	if strings.ContainsAny(msg, "\x1b\r\x00") {
		t.Errorf("a raw control byte reached FetchBeadByID's rendered error: %q", msg)
	}
}

// TestRenderBDError_EscapesEveryLineIndividually is a focused unit test
// of the render-boundary helper itself: each line of a multi-line error
// is escaped independently (the DirtyTreeFailure house pattern), so a
// clean line stays byte-identical while a control-bearing line is fully
// quoted rather than leaking its raw bytes.
func TestRenderBDError_EscapesEveryLineIndividually(t *testing.T) {
	err := &multilineErr{msg: "exit status 1: clean schema line\nhostile\x1b[31mline\rwith control bytes"}

	got := renderBDError(err)

	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 rendered lines, got %d: %q", len(lines), got)
	}
	if lines[0] != "exit status 1: clean schema line" {
		t.Errorf("clean line must render byte-identical, got: %q", lines[0])
	}
	if strings.ContainsAny(lines[1], "\x1b\r") {
		t.Errorf("control-bearing line must not leak raw control bytes, got: %q", lines[1])
	}
}

type multilineErr struct{ msg string }

func (e *multilineErr) Error() string { return e.msg }

// --- Round-1 panel G1: bdRenderedError must preserve the errors.As/Is
// chain through the render boundary, not just get the escaped text right.

// TestBdRenderedError_UnwrapExposesUnderlyingError is the direct unit
// pin: Error() is the escaped text, Unwrap() is the ORIGINAL error
// unchanged.
func TestBdRenderedError_UnwrapExposesUnderlyingError(t *testing.T) {
	underlying := errors.New("exit status 1: hostile\x1b[31mline")
	wrapped := &bdRenderedError{err: underlying}

	if got := wrapped.Unwrap(); got != underlying {
		t.Errorf("Unwrap() = %v, want the exact underlying error %v", got, underlying)
	}
	if strings.ContainsRune(wrapped.Error(), 0x1b) {
		t.Errorf("Error() must render the escaped text, got: %q", wrapped.Error())
	}
}

// TestQueryReady_ErrorsAsReachesExitError pins that errors.As can still
// find the underlying *exec.ExitError through QueryReady's returned
// error — the round-0 regression G1 caught: %s-interpolating
// renderBDError into the wrapping fmt.Errorf severed the chain entirely.
func TestQueryReady_ErrorsAsReachesExitError(t *testing.T) {
	origRunBD := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRunBD })
	readyJSONFn = bead.ReadyJSON

	writeFakeBD(t, `echo 'Error 1105 (HY000): column "depends_on_id" could not be found' >&2
exit 1`)

	_, err := QueryReady()
	if err == nil {
		t.Fatal("expected an error from the failing fake bd")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected errors.As to reach *exec.ExitError through QueryReady's error, got: %v (chain broken)", err)
	}
}

// TestFetchBeadAsOf_IsUnsupportedFlagErrorClassifiesThroughWrapper pins
// the PRODUCTION consumer of this chain: internal/complete's
// defaultVerifyCommitted calls bead.IsUnsupportedFlagError(err, "as-of")
// on FetchBeadAsOf's returned error to detect a pre-1.0.4 bd and fall
// back gracefully. That classification must still work now that the
// error is wrapped in bdRenderedError for display.
func TestFetchBeadAsOf_IsUnsupportedFlagErrorClassifiesThroughWrapper(t *testing.T) {
	origRunBD := runBDFn
	t.Cleanup(func() { runBDFn = origRunBD })
	runBDFn = bead.RunBD

	writeFakeBD(t, `echo 'Error: unknown flag: --as-of' >&2
exit 1`)

	_, err := FetchBeadAsOf("mindspec-abc", "HEAD")
	if err == nil {
		t.Fatal("expected an error from the failing fake bd")
	}
	if !bead.IsUnsupportedFlagError(err, "as-of") {
		t.Errorf("expected IsUnsupportedFlagError to classify through the bdRenderedError wrapper, got: %v", err)
	}
}

// recordRowcapBD observes the final process argv, after the bead waist.
func recordRowcapBD(t *testing.T) func(string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ROWCAP_ARGV\"\nprintf '[]'\n"
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ROWCAP_ARGV", log)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func(verb string) {
		t.Helper()
		raw, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(strings.TrimSpace(string(raw)), "\n")
		caps := 0
		for i, a := range args {
			if a == "-n" {
				if i+1 == len(args) || args[i+1] != "0" {
					t.Fatalf("bad cap: %q", args)
				}
				caps++
			}
		}
		if len(args) == 0 || args[0] != verb || caps != 1 {
			t.Fatalf("expected %s and exactly one -n 0, got %q", verb, args)
		}
	}
}

func TestQueryReady_Uncapped(t *testing.T) {
	check := recordRowcapBD(t)
	for _, query := range []func() ([]BeadInfo, error){QueryReady, func() ([]BeadInfo, error) { return QueryReadyForEpic("epic-129") }} {
		if _, err := query(); err != nil {
			t.Fatal(err)
		}
		check("ready")
	}
}
func TestResolveActiveBead_Uncapped(t *testing.T) {
	stubEpicForSpec(t, 129, "fixture", "epic-129")
	check := recordRowcapBD(t)
	if _, err := ResolveActiveBead(t.TempDir(), "129-fixture"); err != nil {
		t.Fatal(err)
	}
	check("list")
}
