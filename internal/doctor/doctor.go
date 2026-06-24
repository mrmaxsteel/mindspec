package doctor

import "fmt"

// Status represents the result of a single health check.
type Status int

const (
	OK      Status = iota
	Missing        // expected artifact is absent
	Error          // something is wrong and needs action
	Warn           // advisory, not a failure
	Fixed          // was broken, auto-repaired by --fix
)

// Check represents a single health check result.
type Check struct {
	Name    string
	Status  Status
	Message string
	FixFunc func() error // if non-nil, --fix can auto-repair this check
}

// Report holds the results of all doctor checks.
type Report struct {
	Checks []Check
}

// HasFailures returns true if any check has Error or Missing status.
func (r *Report) HasFailures() bool {
	for _, c := range r.Checks {
		if c.Status == Error || c.Status == Missing {
			return true
		}
	}
	return false
}

// Fix runs FixFunc on all checks that have one and are in Error or Warn
// status. Fixed checks are updated to Fixed status.
//
// Note for check authors: attaching a FixFunc to a Warn-status check opts
// that check into auto-repair under `mindspec doctor --fix`. Leave FixFunc
// nil on advisory-only warnings that should stay visible until the user
// acts on them manually.
func (r *Report) Fix() {
	for i := range r.Checks {
		c := &r.Checks[i]
		if c.FixFunc != nil && (c.Status == Error || c.Status == Warn) {
			if err := c.FixFunc(); err != nil {
				c.Message += fmt.Sprintf(" (fix failed: %v)", err)
			} else {
				c.Status = Fixed
			}
		}
	}
}

// Options tunes doctor's behavior. Force controls whether `--fix` on the
// beads config-drift check should also replace user-authored values for
// mindspec-required keys (as opposed to only adding missing ones).
//
// DryRunMigration, when true, makes doctor skip its repair checks and
// run only the checkDryRunMigration reporter, which walks all specs and
// reports which would migrate on their next lifecycle command. Writes
// nothing. See ADR-0034 and spec 089 Requirement 11 (the dry-run path
// is reporting-only and exits 0 regardless of how many legacy specs
// are surfaced).
type Options struct {
	Force           bool
	DryRunMigration bool
}

// Run executes all doctor checks against the given project root.
func Run(root string) *Report {
	return RunWithOptions(root, Options{})
}

// RunWithOptions is Run's full-surface variant.
func RunWithOptions(root string, opts Options) *Report {
	r := &Report{}
	if opts.DryRunMigration {
		checkDryRunMigration(r, root)
		return r
	}
	checkDocs(r, root)
	// Spec 106 Bead 4 (Req 8): detect the docs layout, warn when a
	// canonical/legacy tree would flatten on the next `migrate layout`, and
	// ERROR on a dual-layout duplicate spec id (the stale-duplicate read hazard).
	checkLayout(r, root)
	// Spec 091 Bead 4: static-time ownership manifest checks
	// (dead-manifest Req 17 + the three hygiene Warns Req 20) and the
	// missing-source-globs Warn (Req 18). All advisory; none blocks the
	// gate. Run after checkDocs so the missing-OWNERSHIP Warn (Req 21,
	// the "missing" state) is reported before dead-manifest (the
	// existing-but-dead state) — one state, one Warn.
	checkOwnershipManifests(r, root)
	checkSourceGlobs(r, root)
	checkBeads(r, root)
	checkOrphanedBeads(r, root)
	checkBeadsConfigDrift(r, root, opts.Force)
	checkStrayRootJSONL(r, root)
	checkDurabilityRisk(r, root)
	checkBdVersionFloor(r, root)
	checkBdSchemaDrift(r, root)
	checkMultipleBdOnPath(r, root)
	checkBeadsMergeDriver(r, root)
	checkGit(r, root)
	checkHooks(r, root)
	return r
}
