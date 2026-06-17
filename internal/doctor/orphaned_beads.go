package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// findOrphanedClosedBeadsFn is the shared bd_close lifecycle-bypass predicate
// (also used by `mindspec next` and `mindspec complete`). Injectable so the
// doctor check is unit-testable without a live repo or `bd`.
var findOrphanedClosedBeadsFn = lifecycle.FindOrphanedClosedBeads

// runMindspecCompleteFn re-invokes `mindspec complete <beadID>` to recover one
// orphaned bead during `mindspec doctor --fix`. Injectable for tests; the
// default locates the running mindspec binary and runs it in the project root.
var runMindspecCompleteFn = defaultRunMindspecComplete

func defaultRunMindspecComplete(root, beadID string) error {
	bin, err := os.Executable()
	if err != nil || bin == "" {
		bin = "mindspec"
	}
	cmd := exec.Command(bin, "complete", beadID)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mindspec complete %s failed: %v: %s", beadID, err, string(out))
	}
	return nil
}

// checkOrphanedBeads reports closed lifecycle beads that were closed via a bare
// `bd close` instead of `mindspec complete` (bead mindspec-4gsz): their
// bead/<id> branch still exists and is NOT merged into the spec branch, so the
// work is unmerged and ungated and the lifecycle can no longer see it.
//
// It walks .mindspec/docs/specs/ and runs the shared
// lifecycle.FindOrphanedClosedBeads predicate per spec (the same trigger
// `mindspec next` and `mindspec complete` block on). Each orphan is reported as
// Status=Error with the `mindspec complete <id>` recovery line and a FixFunc
// that re-invokes completion under `--fix`. Read-only by default; the binary is
// run only when Fix() calls the FixFunc.
func checkOrphanedBeads(r *Report, root string) {
	specsRoot := filepath.Join(root, ".mindspec", "docs", "specs")
	entries, err := os.ReadDir(specsRoot)
	if err != nil {
		// No specs dir = nothing to check.
		return
	}

	names := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			names = append(names, ent.Name())
		}
	}
	sort.Strings(names)

	for _, specID := range names {
		// The ancestry check runs in the project root, which holds the
		// bead/<id> and spec/<id> refs. excludeBeadID is empty — doctor is a
		// neutral observer, not itself completing any bead.
		for _, o := range findOrphanedClosedBeadsFn(specID, root, "") {
			beadID := o.BeadID
			r.Checks = append(r.Checks, Check{
				Name:   fmt.Sprintf("orphaned closed bead: %s", beadID),
				Status: Error,
				Message: fmt.Sprintf("bead %s was closed without `mindspec complete` — its branch %s is unmerged into %s. Run `%s` to recover.",
					beadID, o.BeadBranch, o.SpecBranch, o.RecoveryCommand()),
				FixFunc: func() error { return runMindspecCompleteFn(root, beadID) },
			})
		}
	}
}
