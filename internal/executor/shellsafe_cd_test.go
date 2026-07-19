package executor

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// TestExecutableCdRendersShellSafe is AC-12 (this package's slice of the
// sink table): the two executor conflict-failure recoveries
// (beadToSpecConflictFailure's specWtPath cd, directMergeConflictFailure's
// root-only cd) route through the single shell-safe emitter — a
// space-bearing path is POSIX single-quoted and round-trips a real shell;
// a clean path renders byte-identical to today; the root-only sink NEVER
// refuses (it has no error return in the first place — it's a rendering
// helper, not a gate).
func TestExecutableCdRendersShellSafe(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available for the round-trip assertion")
	}

	t.Run("beadToSpecConflictFailure: space-bearing spec worktree path is quoted and round-trips sh -c", func(t *testing.T) {
		specWt := "/tmp/spec worktree with spaces"
		err := beadToSpecConflictFailure("bead/x", "spec/x", specWt, "mindspec complete x", errors.New("merge failed"))
		msg := err.Error()
		wantLine := "cd '" + specWt + "'"
		if !strings.Contains(msg, wantLine) {
			t.Fatalf("recovery message missing quoted cd line %q; got:\n%s", wantLine, msg)
		}
		assertShellRoundTrips(t, " with spaces")
	})

	t.Run("beadToSpecConflictFailure: single-quote-bearing spec worktree path is escaped and round-trips sh -c", func(t *testing.T) {
		specWt := "/tmp/spec's worktree"
		err := beadToSpecConflictFailure("bead/x", "spec/x", specWt, "mindspec complete x", errors.New("merge failed"))
		msg := err.Error()
		// containment.ShellSafe escapes an embedded single quote via the
		// standard close-quote/backslash-quote/reopen-quote technique;
		// assert the message carries EXACTLY that rendering (not a
		// hand-rolled approximation of it).
		wantLine := "cd " + containment.ShellSafe(specWt)
		if !strings.Contains(msg, wantLine) {
			t.Fatalf("recovery message missing single-quote-escaped cd line %q; got:\n%s", wantLine, msg)
		}
		assertShellRoundTrips(t, "'s worktree")
	})

	t.Run("beadToSpecConflictFailure: clean path renders byte-identical", func(t *testing.T) {
		specWt := "/tmp/spec-worktree-clean"
		err := beadToSpecConflictFailure("bead/x", "spec/x", specWt, "mindspec complete x", errors.New("merge failed"))
		msg := err.Error()
		wantLine := "cd " + specWt
		if !strings.Contains(msg, wantLine) {
			t.Fatalf("recovery message missing unquoted cd line %q; got:\n%s", wantLine, msg)
		}
		if strings.Contains(msg, "cd '"+specWt) {
			t.Fatalf("clean path must NOT be quoted; got:\n%s", msg)
		}
	})

	t.Run("directMergeConflictFailure: root-only sink is quote-emitted and never refuses (no error-returning gate)", func(t *testing.T) {
		root := "/tmp/root with spaces"
		err := directMergeConflictFailure(root, "spec/x", errors.New("merge failed"))
		if err == nil {
			t.Fatal("directMergeConflictFailure always returns a failure describing the conflict — this is not the refusal being tested")
		}
		msg := err.Error()
		wantLine := "cd '" + root + "'"
		if !strings.Contains(msg, wantLine) {
			t.Fatalf("recovery message missing quoted root cd line %q; got:\n%s", wantLine, msg)
		}
		assertShellRoundTrips(t, " with spaces")
	})

	t.Run("directMergeConflictFailure: clean root renders byte-identical", func(t *testing.T) {
		root := "/tmp/root-clean"
		err := directMergeConflictFailure(root, "spec/x", errors.New("merge failed"))
		msg := err.Error()
		wantLine := "cd " + root
		if !strings.Contains(msg, wantLine) {
			t.Fatalf("recovery message missing unquoted cd line %q; got:\n%s", wantLine, msg)
		}
	})
}

// assertShellRoundTrips creates a real directory (a fresh t.TempDir()
// with suffix appended to its name — e.g. " with spaces" or "'s
// worktree") and confirms `sh -c "<containment.EmitCd(real)> && pwd"`
// actually lands there.
//
// This calls the REAL emitter (containment.EmitCd) rather than
// re-deriving the quoting rule locally: an earlier version of this
// helper hand-rolled its own close-quote/backslash-quote/reopen-quote
// string concatenation to build the cd line, which meant the test was
// only proving that ITS OWN reimplementation of the quoting rule
// round-trips a shell — never that the actual production emitter's
// output does (F panel finding, R5 fix-up round). Routing through
// containment.EmitCd closes that gap: a regression in ShellSafe/
// EmitCd's escaping now fails HERE, not just silently in production.
func assertShellRoundTrips(t *testing.T, suffix string) {
	t.Helper()
	// A synthetic path in the caller's asserted message may not exist
	// (e.g. "/tmp/spec worktree with spaces"); create a real temp dir
	// carrying the same quoting-relevant character(s) instead, since
	// `sh -c` would otherwise fail with "no such file or directory" for
	// reasons unrelated to quoting.
	real := t.TempDir() + suffix
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", real, err)
	}
	realLine := containment.EmitCd(real)
	out, err := exec.Command("sh", "-c", realLine+" && pwd").CombinedOutput()
	if err != nil {
		t.Fatalf("sh -c %q failed: %v\noutput: %s", realLine, err, out)
	}
	got := strings.TrimSpace(string(out))
	// Resolve symlinks isn't needed here: t.TempDir() is already resolved.
	if got != real && got != strings.TrimSuffix(real, "/") {
		t.Errorf("shell round-trip: sh -c %q landed in %q, want %q", realLine, got, real)
	}
}
