package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// makeSpecDir creates an empty .mindspec/docs/specs/<specID> directory under
// root so checkOrphanedBeads has a spec to walk.
func makeSpecDir(t *testing.T, root, specID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs", specID), 0o755); err != nil {
		t.Fatal(err)
	}
}

// stubFindOrphans swaps the shared predicate for the duration of a test.
func stubFindOrphans(t *testing.T, fn func(specID, workdir, excludeBeadID string) []lifecycle.Orphan) {
	t.Helper()
	orig := findOrphanedClosedBeadsFn
	t.Cleanup(func() { findOrphanedClosedBeadsFn = orig })
	findOrphanedClosedBeadsFn = fn
}

// stubNormalUnmergedHint swaps the spec 127 R2 hint-derivation seam for
// the duration of a test: these fixtures fabricate a lifecycle.Orphan
// value with NO real underlying git repo, so the real
// lifecycle.EvaluateOrphanHint (real git I/O) would fail closed
// (DestructionEvidenceError) against the fixture's non-existent refs.
// This stub reproduces exactly the pre-bead-4 "normal unmerged" byte-
// identical hint every one of these tests already asserts.
func stubNormalUnmergedHint(t *testing.T) {
	t.Helper()
	orig := evaluateOrphanHintFn
	t.Cleanup(func() { evaluateOrphanHintFn = orig })
	evaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
		return lifecycle.OrphanHint{Outcome: guard.DestructionClean, Lines: []string{"mindspec complete " + beadID}}
	}
}

// An orphaned closed bead is reported as Error with the recovery line.
func TestCheckOrphanedBeads_ReportsError(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "008-test")
	stubNormalUnmergedHint(t)

	stubFindOrphans(t, func(specID, workdir, excludeBeadID string) []lifecycle.Orphan {
		if specID != "008-test" {
			return nil
		}
		return []lifecycle.Orphan{{BeadID: "bead-1", BeadBranch: "bead/bead-1", SpecBranch: "spec/008-test"}}
	})

	r := &Report{}
	checkOrphanedBeads(r, root)

	var found *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "orphaned closed bead") {
			found = &r.Checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an orphaned-bead check, got %+v", r.Checks)
	}
	if found.Status != Error {
		t.Errorf("status = %v, want Error", found.Status)
	}
	if !strings.Contains(found.Message, "mindspec complete bead-1") {
		t.Errorf("message must carry the recovery command; got %q", found.Message)
	}
	if found.FixFunc == nil {
		t.Error("an orphaned-bead check must carry a FixFunc for --fix")
	}
	if !r.HasFailures() {
		t.Error("an orphaned bead must trip HasFailures (Error status)")
	}
}

// No orphans → no orphaned-bead check (read-only, no false-positive).
func TestCheckOrphanedBeads_Clean(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "008-test")

	stubFindOrphans(t, func(specID, workdir, excludeBeadID string) []lifecycle.Orphan { return nil })

	r := &Report{}
	checkOrphanedBeads(r, root)
	for _, c := range r.Checks {
		if strings.Contains(c.Name, "orphaned closed bead") {
			t.Errorf("clean repo must report no orphaned-bead check; got %+v", c)
		}
	}
}

// The FixFunc re-invokes `mindspec complete <id>` for the orphan; --fix flips
// the check to Fixed.
func TestCheckOrphanedBeads_FixInvokesComplete(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "008-test")
	stubNormalUnmergedHint(t)

	stubFindOrphans(t, func(specID, workdir, excludeBeadID string) []lifecycle.Orphan {
		return []lifecycle.Orphan{{BeadID: "bead-9", BeadBranch: "bead/bead-9", SpecBranch: "spec/008-test"}}
	})

	var completed []string
	origRun := runMindspecCompleteFn
	t.Cleanup(func() { runMindspecCompleteFn = origRun })
	runMindspecCompleteFn = func(r, beadID string) error {
		completed = append(completed, beadID)
		return nil
	}

	r := &Report{}
	checkOrphanedBeads(r, root)
	r.Fix()

	if len(completed) != 1 || completed[0] != "bead-9" {
		t.Errorf("FixFunc must run `mindspec complete bead-9`; got %v", completed)
	}
	var c *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "orphaned closed bead") {
			c = &r.Checks[i]
		}
	}
	if c == nil || c.Status != Fixed {
		t.Errorf("after --fix the orphaned-bead check must be Fixed; got %+v", c)
	}
}

// TestCheckOrphanedBeads_FixFuncGatedByOutcome is AC-3(vi) (spec 127
// R2(c)): the FixFunc is attached ONLY for guard.DestructionClean —
// every other outcome carries NO FixFunc, so `doctor --fix` mutates
// nothing there. Output and action derive from the SAME hint value, so
// a hint that renders a destructive line can never ALSO carry the
// `mindspec complete` FixFunc.
func TestCheckOrphanedBeads_FixFuncGatedByOutcome(t *testing.T) {
	outcomes := []guard.DestructionOutcome{
		guard.DestructionAncestor,
		guard.DestructionSuperseded,
		guard.DestructionStaleDeletion,
		guard.DestructionEvidenceError,
	}
	if len(outcomes) != int(guard.DestructionOutcomeCount)-1 {
		t.Fatalf("this table covers %d non-Clean outcomes, want %d (guard.DestructionOutcomeCount - 1) — a new outcome variant needs its own row", len(outcomes), int(guard.DestructionOutcomeCount)-1)
	}
	for _, outcome := range outcomes {
		t.Run(outcome.String(), func(t *testing.T) {
			root := t.TempDir()
			makeSpecDir(t, root, "008-test")

			orig := evaluateOrphanHintFn
			t.Cleanup(func() { evaluateOrphanHintFn = orig })
			evaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
				return lifecycle.OrphanHint{Outcome: outcome, Lines: []string{"git diff main bead/bead-9   (inspect)"}}
			}
			stubFindOrphans(t, func(specID, workdir, excludeBeadID string) []lifecycle.Orphan {
				return []lifecycle.Orphan{{BeadID: "bead-9", BeadBranch: "bead/bead-9", SpecBranch: "spec/008-test"}}
			})

			called := false
			origRun := runMindspecCompleteFn
			t.Cleanup(func() { runMindspecCompleteFn = origRun })
			runMindspecCompleteFn = func(r, beadID string) error { called = true; return nil }

			r := &Report{}
			checkOrphanedBeads(r, root)
			var c *Check
			for i := range r.Checks {
				if strings.Contains(r.Checks[i].Name, "orphaned closed bead") {
					c = &r.Checks[i]
				}
			}
			if c == nil {
				t.Fatal("expected an orphaned-bead check")
			}
			if c.FixFunc != nil {
				t.Errorf("outcome %v must carry NO FixFunc", outcome)
			}
			r.Fix()
			if called {
				t.Errorf("outcome %v: doctor --fix must mutate nothing (runMindspecCompleteFn must never be called)", outcome)
			}
		})
	}
}

// TestCheckOrphanedBeads_MultiLineHintThreadsAllLinesInOrder is bead-4
// fix round 1's MAJOR fix (G1-5/O3-1): the accepted consumer-parity
// narrowing claimed each consumer's own per-package tests compensate for
// the missing cross-package literal-parity test, but
// TestCheckOrphanedBeads_FixFuncGatedByOutcome above (the only test in
// this file that stubs a non-Clean outcome) asserts ONLY that FixFunc is
// nil — it never inspects Check.Message's content at all. This stubs
// the Superseded shape (preserve-tag, delete, adopt-invocation — three
// lines) and asserts every line appears, in order and unmodified, in
// Check.Message — the fidelity check the narrowing's "each package
// independently tests its own plumbing" claim needed and did not have.
// checkOrphanedBeads folds hint.Lines with `strings.Join(hint.Lines,
// "; ")` (orphaned_beads.go), so order and content survive that fold
// exactly, unlike impl.go/adopt.go's separate-recovery-line rendering.
func TestCheckOrphanedBeads_MultiLineHintThreadsAllLinesInOrder(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "008-test")

	wantLines := []string{
		`git tag preserve/bead-9 bead/bead-9   (preserve bead-9's commits before deleting — its content is not guaranteed reachable from spec/008-test)`,
		"git branch -D bead/bead-9",
		`mindspec impl adopt 008-test --reason "<why bead-9's content already reached main outside the lifecycle>"`,
	}
	orig := evaluateOrphanHintFn
	t.Cleanup(func() { evaluateOrphanHintFn = orig })
	evaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
		return lifecycle.OrphanHint{
			Outcome:      guard.DestructionSuperseded,
			EvidenceNote: "bead bead-9's branch bead/bead-9 is superseded — its content already landed in spec/008-test via another route per the shared work-destruction predicate",
			Lines:        append([]string{}, wantLines...),
		}
	}
	stubFindOrphans(t, func(specID, workdir, excludeBeadID string) []lifecycle.Orphan {
		return []lifecycle.Orphan{{BeadID: "bead-9", BeadBranch: "bead/bead-9", SpecBranch: "spec/008-test"}}
	})

	r := &Report{}
	checkOrphanedBeads(r, root)
	var c *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "orphaned closed bead") {
			c = &r.Checks[i]
		}
	}
	if c == nil {
		t.Fatal("expected an orphaned-bead check")
	}

	lastIdx := -1
	for i, want := range wantLines {
		idx := strings.Index(c.Message, want)
		if idx < 0 {
			t.Fatalf("line %d (%q) missing verbatim from Check.Message:\n%s", i, want, c.Message)
		}
		if idx <= lastIdx {
			t.Fatalf("line %d (%q) did not appear AFTER the previous line — order not preserved:\n%s", i, want, c.Message)
		}
		lastIdx = idx
	}
	if c.FixFunc != nil {
		t.Error("a Superseded outcome must carry NO FixFunc")
	}
}

// No specs dir → no-op, no panic.
func TestCheckOrphanedBeads_NoSpecsDir(t *testing.T) {
	root := t.TempDir()
	r := &Report{}
	checkOrphanedBeads(r, root)
	if len(r.Checks) != 0 {
		t.Errorf("missing specs dir must be a no-op; got %+v", r.Checks)
	}
}
