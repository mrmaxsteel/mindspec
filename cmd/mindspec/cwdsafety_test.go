package main

// Spec 092 Bead 4 (mindspec-qxsy): coverage for the AC "qxsy unit
// (impl approve)" — invocation from inside the spec worktree ends with
// the process cwd at root, the cd-back NOTE as the last line of stdout,
// and a nil error (exit 0).
//
// Panel R3-1/R3-2: the tests below call the PRODUCTION tail functions
// (implApproveTail / completeTail — the exact code the RunE handlers
// run) rather than simulating their sequence, so deleting the chdir or
// the NOTE call from the production path, or moving the NOTE before the
// instruct tail, fails these tests. The withWorkingDir restore-failure
// half of the impl-approve AC lives in
// internal/executor/executor_test.go.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// chdirIntoDoomed creates root/<rel>, chdirs the process into it, and
// returns its path plus the captured invocation cwd — the entry state
// of a terminal command run from inside a worktree.
func chdirIntoDoomed(t *testing.T, root, rel string) (doomed, invocationCwd string) {
	t.Helper()
	doomed = filepath.Join(root, rel)
	if err := os.MkdirAll(doomed, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", doomed, err)
	}
	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(doomed); err != nil {
		t.Fatalf("chdir into %s: %v", doomed, err)
	}
	invocationCwd = captureInvocationCwd()
	if invocationCwd == "" {
		t.Fatal("captureInvocationCwd returned empty for a valid cwd")
	}
	return doomed, invocationCwd
}

func assertCwdIsRoot(t *testing.T, root string) {
	t.Helper()
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("process ended in unresolvable cwd: %v", wdErr)
	}
	realWd, _ := filepath.EvalSymlinks(wd)
	realRoot, _ := filepath.EvalSymlinks(root)
	if realWd != realRoot {
		t.Errorf("process cwd: got %q, want root %q", wd, root)
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

// TestImplApproveTail_SuccessFromRemovedSpecWorktree exercises the
// production `impl approve` tail with the terminal mutation having
// removed the invocation directory (the spec worktree):
//   - the Req 3b chdir leaves the process at root (kills panel
//     mutation 1b — delete the chdir and the cwd stays deleted),
//   - the Req 4 NOTE is the LAST line of stdout, emitted AFTER the
//     instruct tail (kills mutation 2c — move emitCdBackNote before
//     instructFn and the last line becomes the instruct output),
//   - the tail returns nil → exit 0.
func TestImplApproveTail_SuccessFromRemovedSpecWorktree(t *testing.T) {
	root := t.TempDir()
	specWt, invocationCwd := chdirIntoDoomed(t, root, filepath.Join(".worktrees", "worktree-spec-091-x"))

	// Terminal mutation: FinalizeEpic removes the spec worktree —
	// including the process cwd.
	if err := os.RemoveAll(specWt); err != nil {
		t.Fatalf("removing spec worktree: %v", err)
	}

	var stdout, stderr bytes.Buffer
	instructed := false
	tailErr := implApproveTail(&stdout, &stderr, root, invocationCwd, "091-x", nil,
		&approve.ImplResult{SpecID: "091-x", SpecBranch: "spec/091-x", CommitCount: 2},
		nil,
		func(r string) error {
			instructed = true
			if r != root {
				t.Errorf("instructFn root: got %q, want %q", r, root)
			}
			fmt.Fprintln(&stdout, "instruct guidance line")
			return nil
		})

	if tailErr != nil {
		t.Fatalf("tail returned error on success path: %v", tailErr)
	}
	if !instructed {
		t.Error("instruct tail was not emitted")
	}

	assertCwdIsRoot(t, root)

	want := "NOTE: your shell's working directory was removed — run: cd " + root
	if got := lastLine(stdout.String()); got != want {
		t.Errorf("last stdout line:\n  got  %q\n  want %q", got, want)
	}
	if !strings.Contains(stdout.String(), "Implementation for 091-x approved") {
		t.Errorf("success output missing; stdout:\n%s", stdout.String())
	}
}

// TestImplApproveTail_SuccessNoNoteWhileCwdExists: from a surviving
// invocation directory the tail stays NOTE-free.
func TestImplApproveTail_SuccessNoNoteWhileCwdExists(t *testing.T) {
	root := t.TempDir()
	_, invocationCwd := chdirIntoDoomed(t, root, "alive")

	var stdout, stderr bytes.Buffer
	tailErr := implApproveTail(&stdout, &stderr, root, invocationCwd, "091-x", nil,
		&approve.ImplResult{SpecID: "091-x"},
		nil,
		func(string) error { fmt.Fprintln(&stdout, "instruct guidance line"); return nil })

	if tailErr != nil {
		t.Fatalf("tail returned error: %v", tailErr)
	}
	if strings.Contains(stdout.String(), "working directory was removed") {
		t.Errorf("NOTE emitted while invocation cwd exists; stdout:\n%s", stdout.String())
	}
}

// TestImplApproveTail_ErrorPathEmitsNoteOnStderr (panel R2-3):
// FinalizeEpic can fail AFTER removing the spec worktree. The error
// path must still chdir to root and emit the NOTE — as the LAST line of
// stderr (stdout is the success channel) — before the caller exits 1.
func TestImplApproveTail_ErrorPathEmitsNoteOnStderr(t *testing.T) {
	root := t.TempDir()
	specWt, invocationCwd := chdirIntoDoomed(t, root, filepath.Join(".worktrees", "worktree-spec-091-x"))

	// FinalizeEpic removed the worktree, then failed.
	if err := os.RemoveAll(specWt); err != nil {
		t.Fatalf("removing spec worktree: %v", err)
	}
	approveErr := errors.New("finalize: direct merge conflict")

	var stdout, stderr bytes.Buffer
	tailErr := implApproveTail(&stdout, &stderr, root, invocationCwd, "091-x", nil,
		nil, // result is nil on error
		approveErr,
		func(string) error {
			t.Error("instruct tail must not run on the error path")
			return nil
		})

	if !errors.Is(tailErr, approveErr) {
		t.Errorf("tail must return the approve error for the exit-1 path; got %v", tailErr)
	}

	assertCwdIsRoot(t, root)

	if stdout.Len() != 0 {
		t.Errorf("error path must not write success output to stdout; got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "error: finalize: direct merge conflict") {
		t.Errorf("stderr missing the error line; got:\n%s", stderr.String())
	}
	want := "NOTE: your shell's working directory was removed — run: cd " + root
	if got := lastLine(stderr.String()); got != want {
		t.Errorf("last stderr line:\n  got  %q\n  want %q", got, want)
	}
}

// TestCompleteTail_NoteIsLastStdoutLine exercises the production
// `mindspec complete` tail with the bead worktree (the invocation cwd)
// removed by the terminal mutation: the NOTE is the LAST line of
// stdout, after FormatResult and the instruct tail (panel R3-2 —
// moving emitCdBackNote before instructFn fails this test).
func TestCompleteTail_NoteIsLastStdoutLine(t *testing.T) {
	root := t.TempDir()
	beadWt, invocationCwd := chdirIntoDoomed(t, root, filepath.Join(".worktrees", "worktree-bead-doom"))

	// complete.Run's CompleteBead removed the bead worktree; its Req 3c
	// chdir moved the process to root.
	if err := os.RemoveAll(beadWt); err != nil {
		t.Fatalf("removing bead worktree: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to root: %v", err)
	}

	var stdout, stderr bytes.Buffer
	completeTail(&stdout, &stderr, root, invocationCwd,
		&complete.Result{
			BeadID:          "bead-doom",
			BeadClosed:      true,
			WorktreeRemoved: true,
			NextMode:        state.ModeReview,
			NextSpec:        "009-doomed",
		},
		func(r string) error {
			if r != root {
				t.Errorf("instructFn root: got %q, want %q", r, root)
			}
			fmt.Fprintln(&stdout, "instruct guidance line")
			return nil
		})

	out := stdout.String()
	if !strings.Contains(out, "Bead bead-doom closed.") {
		t.Errorf("FormatResult output missing; stdout:\n%s", out)
	}
	if !strings.Contains(out, "instruct guidance line") {
		t.Errorf("instruct tail missing; stdout:\n%s", out)
	}
	want := "NOTE: your shell's working directory was removed — run: cd " + root
	if got := lastLine(out); got != want {
		t.Errorf("last stdout line:\n  got  %q\n  want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

// TestCompleteTail_NoNoteWhileCwdExists: the happy path (complete run
// from the repo root) stays NOTE-free.
func TestCompleteTail_NoNoteWhileCwdExists(t *testing.T) {
	root := t.TempDir()
	_, invocationCwd := chdirIntoDoomed(t, root, "alive")

	var stdout, stderr bytes.Buffer
	completeTail(&stdout, &stderr, root, invocationCwd,
		&complete.Result{BeadID: "bead-1", BeadClosed: true},
		func(string) error { fmt.Fprintln(&stdout, "instruct guidance line"); return nil })

	if strings.Contains(stdout.String(), "working directory was removed") {
		t.Errorf("NOTE emitted while invocation cwd exists; stdout:\n%s", stdout.String())
	}
}

func TestCompleteOutput_ReportedLedgerFailure(t *testing.T) {
	root := t.TempDir()
	invocationCwd := filepath.Join(root, ".worktrees", "removed-bead")
	recovery := "recovery: git -C '" + root + "' add -- ledger && git -C '" + root + "' commit -m \"chore(panel): ledger\""
	message := "bead is closed and merged; only the panel ledger commit remains: commit failed\n" + recovery
	runErr := &complete.ReportedError{Err: errors.New(message)}
	var stdout, stderr bytes.Buffer
	// Run prints the diagnostic before returning the reported error.
	fmt.Fprintln(&stderr, runErr)
	result := &complete.Result{BeadID: "bead-doom", BeadClosed: true, WorktreeRemoved: true, NextMode: state.ModeReview}
	err := completeOutput(&stdout, &stderr, root, invocationCwd, result, runErr, func(string) error {
		fmt.Fprintln(&stdout, "instruct guidance line")
		return nil
	})
	if !errors.Is(err, runErr) {
		t.Fatalf("reported failure must still exit non-zero: %v", err)
	}
	if strings.Count(stderr.String(), recovery) != 1 || strings.Count(stderr.String(), "bead is closed and merged; only the panel ledger commit remains") != 1 {
		t.Errorf("error and recovery must appear once: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Bead bead-doom closed.") || !strings.Contains(stdout.String(), "instruct guidance line") {
		t.Errorf("missing normal completion summary or guidance: %q", stdout.String())
	}
	if got, want := lastLine(stdout.String()), "NOTE: your shell's working directory was removed — run: cd "+root; got != want {
		t.Errorf("last stdout line: got %q, want %q", got, want)
	}
}

func TestCompleteOutput_UnreportedFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runErr := errors.New("pre-merge failure")
	err := completeOutput(&stdout, &stderr, "", "", nil, runErr, func(string) error {
		t.Fatal("instruct called after a pre-merge failure")
		return nil
	})
	if !errors.Is(err, runErr) || stderr.String() != "error: pre-merge failure\n" || stdout.Len() != 0 {
		t.Fatalf("unexpected error output: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
}

// TestCompleteTail_SuccessBranch_AgreesWithResultNextMode is AC-4's
// success branch (spec 128 bead 5), driven over the REAL production
// completeTail wiring: when result.NextModeIndeterminate is false, the
// tail calls instructFn (never instruct.RenderIndeterminate) and the
// output contains no indeterminate marker — the two branches are
// mutually exclusive, asserted only in their own driving case (round-2
// G11 — a tail that always renders the indeterminate marker would pass
// this branch vacuously, which is exactly why AC-4 requires the SUCCESS
// branch to assert its ABSENCE, not just the failure branch's presence).
func TestCompleteTail_SuccessBranch_AgreesWithResultNextMode(t *testing.T) {
	root := t.TempDir()
	instructFnCalled := false

	var stdout, stderr bytes.Buffer
	completeTail(&stdout, &stderr, root, root,
		&complete.Result{
			BeadID:   "bead-ac4",
			NextMode: state.ModeImplement,
			NextBead: "bead-ac4-next",
			NextSpec: "008-ac4",
		},
		func(r string) error {
			instructFnCalled = true
			fmt.Fprintln(&stdout, "## MindSpec Lifecycle\ninstruct guidance line")
			return nil
		})

	if !instructFnCalled {
		t.Fatal("the success branch must call instructFn (the real per-mode tail), not instruct.RenderIndeterminate")
	}
	out := stdout.String()
	if strings.Contains(out, "cannot determine mode") {
		t.Errorf("success branch output must contain no indeterminate marker; got:\n%s", out)
	}
	resultBeforeGuidance(t, "complete (success branch)", out, "Bead bead-ac4 closed.", "## MindSpec Lifecycle")
}

// modeHeadings maps each real state.Mode* to its instruct template's own
// heading line — used to assert MECHANICAL agreement between
// Result.NextMode and what the REAL tail actually rendered (round-1
// panel G3): a generic callback assertion (as in the test above) cannot
// catch the tail rendering a DIFFERENT mode than the one the result
// summary claimed, since a stub instructFn renders whatever the test
// tells it to, not what production derivation actually computes.
var modeHeadings = map[string]string{
	state.ModeIdle:      "MindSpec — No Active Work",
	state.ModeSpec:      "MindSpec — Spec Mode",
	state.ModePlan:      "MindSpec — Plan Mode",
	state.ModeImplement: "MindSpec — Implementation Mode",
	state.ModeReview:    "MindSpec — Review Mode",
}

// TestCompleteTail_SuccessBranch_RealEmitInstructAgreesWithResultNextMode
// is round-1 panel G3: drives completeTail with the REAL production
// emitInstruct (not a stub) against a real fixture whose epic
// independently derives to state.ModeImplement, and asserts the rendered
// tail's heading equals the ONE heading Result.NextMode names — and no
// other mode's heading appears. This would go RED if an implement result
// were ever paired with review/plan/idle/indeterminate guidance (the
// exact regression a generic stub callback cannot detect).
func TestCompleteTail_SuccessBranch_RealEmitInstructAgreesWithResultNextMode(t *testing.T) {
	root := t.TempDir()
	withTestChdir(t, root)

	epics := []phase.EpicInfo{{
		ID: "epic-ac4-real", Title: "[SPEC 902-ac4-real] AC-4 real fixture", Status: "open",
		IssueType: "epic", Metadata: map[string]interface{}{
			"spec_num": float64(902), "spec_title": "ac4-real",
			"mindspec_phase": state.ModeImplement, "mindspec_phase_semantics": "v2",
		},
	}}
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	}))

	// emitInstruct (the real production instructFn) writes directly to
	// the literal os.Stdout — it does not respect an injected io.Writer,
	// exactly like production's own completeTail(os.Stdout, os.Stderr,
	// ...) call site. A bytes.Buffer passed as completeTail's stdout
	// parameter would therefore capture ONLY FormatResult's own output,
	// silently missing emitInstruct's — so this test redirects the real
	// os.Stdout (captureStdout) and passes it through explicitly,
	// matching production byte-for-byte.
	stderrBuf := &bytes.Buffer{}
	out, err := captureStdout(t, func() error {
		completeTail(os.Stdout, stderrBuf, root, root, &complete.Result{
			BeadID:   "bead-ac4-real",
			NextMode: state.ModeImplement,
			NextSpec: "902-ac4-real",
		}, emitInstruct)
		return nil
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}

	want := modeHeadings[state.ModeImplement]
	if !strings.Contains(out, want) {
		t.Fatalf("rendered tail does not agree with Result.NextMode=%q (want heading %q); got:\n%s", state.ModeImplement, want, out)
	}
	for mode, heading := range modeHeadings {
		if mode == state.ModeImplement {
			continue
		}
		if strings.Contains(out, heading) {
			t.Errorf("rendered tail contains a DIFFERENT mode's heading %q (mode %q) alongside the expected %q — Result.NextMode and the real rendered template disagree:\n%s", heading, mode, want, out)
		}
	}
	if stderrBuf.Len() != 0 {
		t.Errorf("unexpected stderr (a real emitInstruct failure would mean the fixture is wrong): %q", stderrBuf.String())
	}
}

// TestCompleteTail_IndeterminateBranch_NeverCallsInstructFn is AC-4's
// failure branch (spec 128 bead 5, AC-4b's real-wiring companion): when
// result.NextModeIndeterminate is true, completeTail renders the shared
// indeterminate form directly (instruct.RenderIndeterminate) and MUST
// NOT call instructFn at all — a fresh independent re-derivation there
// could disagree with (or paper over, via stale metadata) the exact
// failure the result summary just reported. The output carries the
// indeterminate marker, no idle.md guidance, and ends with a
// copy-pastable final `recovery:` line (ADR-0035).
func TestCompleteTail_IndeterminateBranch_NeverCallsInstructFn(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	completeTail(&stdout, &stderr, root, root,
		&complete.Result{
			BeadID:                "bead-ac4b",
			NextModeIndeterminate: true,
			IndeterminateReason:   "reading post-close child set for epic epic-ac4b: bd list --parent: simulated failure",
			NextSpec:              "008-ac4b",
		},
		func(r string) error {
			t.Fatal("instructFn must NOT be called on the indeterminate branch")
			return nil
		})

	out := stdout.String()
	if !strings.Contains(out, "Bead bead-ac4b closed.") {
		t.Errorf("FormatResult summary missing; stdout:\n%s", out)
	}
	if !strings.Contains(out, "cannot determine mode") {
		t.Errorf("expected the indeterminate marker text, got:\n%s", out)
	}
	if strings.Contains(out, "## Available Actions") {
		t.Errorf("idle.md's Available Actions block must never appear on the indeterminate branch; got:\n%s", out)
	}
	if strings.Contains(out, "No Active Work") {
		t.Errorf("idle.md's heading must never appear on the indeterminate branch; got:\n%s", out)
	}
	// emitCdBackNote appends nothing here (invocation cwd exists), so the
	// tail's own final recovery line stays the true last non-empty line.
	if got := lastLine(strings.TrimRight(out, "\n")); got != "recovery: mindspec doctor" {
		t.Errorf("expected the tail to end with a copy-pastable recovery line, got %q; full output:\n%s", got, out)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

// TestAdoptSpecTail_VerifiedSuccess_EmitsResultThenInstructTail is round-1
// panel G4: `mindspec impl adopt`'s success path was missing its
// post-result instruct tail entirely (every other lifecycle-transitioning
// verb — complete, impl approve — emits one). This drives the REAL
// production adoptSpecTail with a hand-built verified AdoptResult and
// asserts the result line precedes the (injected) guidance, and that the
// non-fatal tail-warning behavior matches completeTail/implApproveTail.
func TestAdoptSpecTail_VerifiedSuccess_EmitsResultThenInstructTail(t *testing.T) {
	var stdout, stderr bytes.Buffer
	instructFnCalled := false
	adoptSpecTail(&stdout, &stderr, "/repo", &approve.AdoptResult{
		SpecID:   "008-adopt-verified",
		EpicID:   "epic-adopt-1",
		Verified: true,
	}, func(root string) error {
		instructFnCalled = true
		if root != "/repo" {
			t.Errorf("instructFn root: got %q, want %q", root, "/repo")
		}
		fmt.Fprintln(&stdout, "## MindSpec Lifecycle\ninstruct guidance line")
		return nil
	})

	if !instructFnCalled {
		t.Fatal("adoptSpecTail must call the instruct tail on a successful adopt (round-1 panel G4)")
	}
	out := stdout.String()
	resultBeforeGuidance(t, "impl adopt (verified)", out, "adopted (landed evidence VERIFIED)", "## MindSpec Lifecycle")
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

// TestAdoptSpecTail_AttestedSuccess_EmitsResultThenInstructTail is G4's
// second success leg (verified and attested are the two DISTINCT success
// shapes AdoptSpec can return; both must carry the tail).
func TestAdoptSpecTail_AttestedSuccess_EmitsResultThenInstructTail(t *testing.T) {
	var stdout, stderr bytes.Buffer
	instructFnCalled := false
	adoptSpecTail(&stdout, &stderr, "/repo", &approve.AdoptResult{
		SpecID:   "008-adopt-attested",
		EpicID:   "epic-adopt-2",
		Verified: false,
		Trigger:  "branch-absent-no-source",
	}, func(root string) error {
		instructFnCalled = true
		fmt.Fprintln(&stdout, "## MindSpec Lifecycle\ninstruct guidance line")
		return nil
	})

	if !instructFnCalled {
		t.Fatal("adoptSpecTail must call the instruct tail on the attested success path too")
	}
	out := stdout.String()
	resultBeforeGuidance(t, "impl adopt (attested)", out, "adopted via attestation", "## MindSpec Lifecycle")
	if !strings.Contains(out, "trigger=branch-absent-no-source") {
		t.Errorf("expected the attestation trigger named in the result line, got:\n%s", out)
	}
}

// TestAdoptSpecTail_InstructFailure_IsNonFatalWarning matches
// completeTail/implApproveTail's contract: a tail-render failure is a
// stderr warning, never a fatal error — the adopt itself already
// succeeded.
func TestAdoptSpecTail_InstructFailure_IsNonFatalWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	adoptSpecTail(&stdout, &stderr, "/repo", &approve.AdoptResult{
		SpecID:   "008-adopt-warn",
		Verified: true,
	}, func(string) error {
		return errors.New("simulated instruct failure")
	})

	if !strings.Contains(stdout.String(), "adopted (landed evidence VERIFIED)") {
		t.Errorf("result line missing from stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: could not emit guidance: simulated instruct failure") {
		t.Errorf("expected a non-fatal stderr warning, got: %q", stderr.String())
	}
}

// TestEmitCdBackNote_SilentWhileInvocationCwdExists: the NOTE is a
// removed-directory recovery channel, not noise on the happy path.
func TestEmitCdBackNote_SilentWhileInvocationCwdExists(t *testing.T) {
	root := t.TempDir()
	alive := filepath.Join(root, "alive")
	if err := os.MkdirAll(alive, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout bytes.Buffer
	emitCdBackNote(&stdout, alive, root)
	if stdout.Len() != 0 {
		t.Errorf("NOTE emitted while invocation cwd exists: %q", stdout.String())
	}
}

// TestEmitCdBackNote_NoOpWithoutCapturedCwd: an empty capture (cwd was
// unresolvable at entry) disables the NOTE instead of pointing the
// agent at a stat of "".
func TestEmitCdBackNote_NoOpWithoutCapturedCwd(t *testing.T) {
	var stdout bytes.Buffer
	emitCdBackNote(&stdout, "", "/some/root")
	if stdout.Len() != 0 {
		t.Errorf("NOTE emitted for empty invocation cwd: %q", stdout.String())
	}
}

// TestCaptureInvocationCwd_DeletedCwdDegradesGracefully: when the
// process starts in an already-deleted directory, the capture+NOTE
// pipeline must never error or emit garbage. Platform behavior differs
// — Linux getcwd fails (capture returns "", NOTE disabled) while macOS
// returns the stale path (capture non-empty, stat fails, NOTE emitted)
// — so the invariant is: the output is either empty or exactly the
// cd-back NOTE pointing at root.
func TestCaptureInvocationCwd_DeletedCwdDegradesGracefully(t *testing.T) {
	base := t.TempDir()
	doomed := filepath.Join(base, "doomed")
	if err := os.MkdirAll(doomed, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(doomed); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.RemoveAll(doomed); err != nil {
		t.Fatalf("remove cwd: %v", err)
	}

	got := captureInvocationCwd()

	root := base
	var stdout bytes.Buffer
	emitCdBackNote(&stdout, got, root)

	out := stdout.String()
	want := "NOTE: your shell's working directory was removed — run: cd " + root + "\n"
	switch {
	case got == "" && out != "":
		t.Errorf("empty capture must disable the NOTE; got %q", out)
	case got != "" && out != want:
		t.Errorf("non-empty capture of a deleted cwd must emit the NOTE:\n  got  %q\n  want %q", out, want)
	}
}
