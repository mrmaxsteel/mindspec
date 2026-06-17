package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// An orphaned closed bead is reported as Error with the recovery line.
func TestCheckOrphanedBeads_ReportsError(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "008-test")

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

// No specs dir → no-op, no panic.
func TestCheckOrphanedBeads_NoSpecsDir(t *testing.T) {
	root := t.TempDir()
	r := &Report{}
	checkOrphanedBeads(r, root)
	if len(r.Checks) != 0 {
		t.Errorf("missing specs dir must be a no-op; got %+v", r.Checks)
	}
}
