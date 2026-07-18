package approve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/validate"
)

// planLintDoubleAssignedFiles is the Spec 119 R11 (mindspec-jli8)
// double-assignment plan-lint: it flags any single file path referenced
// in TWO OR MORE beads' `**Steps**` lists — the unambiguous "double
// assignment" case (spec-118's plan panel had to manually catch exactly
// this: a helper assigned to both Bead 1 and Bead 2's steps). It is
// advisory only (AC-23) — a WARNING appended to PlanResult.Warnings,
// never a plan-approve failure — because a genuinely shared file is not
// itself a bug; it is the SIGNAL a human/panel should look at to decide
// whether the two beads collide at merge or whether the overlap is
// benign (e.g. two beads editing disjoint functions in the same file,
// exactly this spec's own Bead 4 / Bead 5 adjacency over
// internal/approve/plan.go).
//
// Path extraction reuses validate.ExtractPathRefs — the SAME
// backtick-free path-reference regex the plan validator already applies
// elsewhere — rather than introducing a second path-matching pattern.
// Deliberately a NEW, free-standing function (kept in its own file) so it
// stays disjoint from Bead 4's preflight-hoist edits inside ApprovePlan /
// createImplementationBeads in plan.go proper.
func planLintDoubleAssignedFiles(sections []validate.BeadSection) []string {
	fileToBeads := map[string][]string{}
	for _, bs := range sections {
		seenInBead := map[string]bool{}
		for _, line := range bs.StepLines {
			for _, p := range validate.ExtractPathRefs(line) {
				if seenInBead[p] {
					continue
				}
				seenInBead[p] = true
				fileToBeads[p] = append(fileToBeads[p], bs.Heading)
			}
		}
	}

	paths := make([]string, 0, len(fileToBeads))
	for p := range fileToBeads {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var warnings []string
	for _, p := range paths {
		beads := fileToBeads[p]
		if len(beads) < 2 {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"plan-lint: %s is assigned to multiple beads' step lists: %s",
			p, strings.Join(beads, ", ")))
	}
	return warnings
}
