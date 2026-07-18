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
//
// SkipLocalEnv, when true, skips the developer-local-environment checks
// (Beads merge driver, bd version floor, bd schema drift, multiple-bd-
// on-PATH, and stale hooks) that a fresh CI checkout structurally cannot
// satisfy: `actions/checkout` never populates a checkout-local
// `merge.beads.driver` git config value (that's a per-clone/per-worktree
// setting `mindspec setup` writes, not tracked state), and a CI runner
// has no local `bd` install or git hooks to probe. Running those checks
// unconditionally in CI produced a false-positive "Beads merge driver"
// Error on every fresh checkout (Spec 119 lc12.2 panel finding). The
// repo-integrity / lifecycle-divergence checks (stale-OPEN, finalize-
// orphan, orphaned-closed, tracker/git divergence, docs/layout/ownership,
// beads durable-state) are NOT skipped — those are exactly what CI
// SHOULD gate on. See `mindspec doctor --ci` and the CI wiring in
// `.github/workflows/ci.yml`.
type Options struct {
	Force           bool
	DryRunMigration bool
	SkipLocalEnv    bool
}

// RunWithOptions executes all doctor checks against the given project root.
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
	// Spec 119 Bead 2 (R5/R7), final-review F1: the stale-OPEN cross-check
	// (inverse of checkOrphanedBeads), finalize-orphan surfacing, and the
	// stale-committed-tracker check — ONE shared cache-aware aggregate
	// scan (lifecycle.ScanIntegrityFindings), the identical exported
	// symbol the generated `mindspec instruct` guidance invokes
	// (P8/AC-12/AC-15).
	checkLifecycleIntegrity(r, root)
	checkBeadsConfigDrift(r, root, opts.Force)
	checkStrayRootJSONL(r, root)
	checkDurabilityRisk(r, root)
	checkGit(r, root)
	// Developer-local-environment checks: a fresh CI checkout has no local
	// `bd` install, no git hooks, and no checkout-local merge.beads.driver
	// config (see Options.SkipLocalEnv doc comment), so `mindspec doctor
	// --ci` skips these rather than false-failing the build on every clean
	// checkout.
	if !opts.SkipLocalEnv {
		checkBdVersionFloor(r, root)
		checkBdSchemaDrift(r, root)
		checkMultipleBdOnPath(r, root)
		checkBeadsMergeDriver(r, root)
		checkHooks(r, root)
	}
	return r
}
