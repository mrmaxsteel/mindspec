package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// escapeLines applies termsafe.Escape to each line of a (possibly
// multi-line) block of agent-influenced text — subprocess CombinedOutput
// — while preserving the real newlines that separate genuine lines (R4:
// per-line escaping for line-oriented bodies, never per-message, so a
// hostile line cannot forge additional lines while legitimate multi-line
// structure survives).
func escapeLines(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = termsafe.Escape(l)
	}
	return strings.Join(lines, "\n")
}

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
		// R4: subprocess CombinedOutput is agent-influenced porcelain text —
		// escape per-line before it reaches the error path.
		return fmt.Errorf("mindspec complete %s failed: %v: %s", idrender.Bead(beadID), err, escapeLines(string(out)))
	}
	return nil
}

// checkOrphanedBeads reports closed lifecycle beads that were closed via a bare
// `bd close` instead of `mindspec complete` (bead mindspec-4gsz): their
// bead/<id> branch still exists and is NOT merged into the spec branch, so the
// work is unmerged and ungated and the lifecycle can no longer see it.
//
// It walks the tier-aware specs enumeration root (flat .mindspec/specs →
// canonical .mindspec/docs/specs → legacy docs/specs, spec 106 Req 3) and runs
// the shared lifecycle.FindOrphanedClosedBeads predicate per spec (the same
// trigger `mindspec next` and `mindspec complete` block on). Each orphan is
// reported as Status=Error with the `mindspec complete <id>` recovery line and
// a FixFunc that re-invokes completion under `--fix`. Read-only by default; the
// binary is run only when Fix() calls the FixFunc.
func checkOrphanedBeads(r *Report, root string) {
	specsRoot := workspace.SpecsDir(root)
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
			// R4: beadID is an ID-typed position (idrender.Bead);
			// BeadBranch/SpecBranch carry a "bead/"/"spec/" prefix so
			// they don't validate against the bare idvalidate grammar —
			// escape as free text instead.
			r.Checks = append(r.Checks, Check{
				Name:   fmt.Sprintf("orphaned closed bead: %s", idrender.Bead(beadID)),
				Status: Error,
				Message: fmt.Sprintf("bead %s was closed without `mindspec complete` — its branch %s is unmerged into %s. Run `%s` to recover.",
					idrender.Bead(beadID), termsafe.Escape(o.BeadBranch), termsafe.Escape(o.SpecBranch), o.RecoveryCommand()),
				FixFunc: func() error { return runMindspecCompleteFn(root, beadID) },
			})
		}
	}
}
