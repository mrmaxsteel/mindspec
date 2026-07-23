package doctor

// Spec 119 lc12.2 fix-up: a fresh `actions/checkout` never populates a
// checkout-local `merge.beads.driver` git config value (that's a per-clone
// setting `mindspec setup` writes, not tracked repo state), so running
// `mindspec doctor` unconditionally in CI produced a false-positive "Beads
// merge driver" Error on every clean checkout — breaking the build for
// everyone once merged. Options.SkipLocalEnv (wired to `mindspec doctor
// --ci`) skips that check plus the other developer-local-environment
// checks (bd version floor, bd schema drift, multiple-bd-on-PATH, stale
// hooks), while leaving every repo-integrity / lifecycle-divergence check
// gating the build unconditionally.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/domain"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// TestRunWithOptions_SkipLocalEnv_OmitsDevEnvChecks pins that SkipLocalEnv
// suppresses exactly the developer-local-environment checks, using a fresh-
// checkout-shaped fixture (merge=beads attribute present, no driver
// configured) that DOES trip "Beads merge driver" when SkipLocalEnv is
// false — proving the fixture is genuinely load-bearing, not accidentally
// already-healthy.
func TestRunWithOptions_SkipLocalEnv_OmitsDevEnvChecks(t *testing.T) {
	root := beadsRoot(t, true)
	writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
	// No `git config merge.beads.driver` — the exact fresh-checkout state:
	// .gitattributes is tracked and checked out, but merge.beads.driver is
	// local git config that a plain `actions/checkout` never sets.
	writeExecutable(t, filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh"))
	// Round out the fixture so it is otherwise healthy: empty domains/specs
	// enumeration roots, a durable Beads state file, and (spec 123 R1/R3) a
	// context-map.md skeleton so the new missing-context-map check doesn't
	// itself turn this "otherwise healthy" fixture unhealthy.
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "domains"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".mindspec", "context-map.md"), domain.ContextMapSkeleton())
	writeFile(t, filepath.Join(root, ".beads", "issues.jsonl"), "")

	// Regression pin: without SkipLocalEnv, this fixture DOES fail.
	full := RunWithOptions(root, Options{})
	if c := findCheck(full, "Beads merge driver"); c == nil || c.Status != Error {
		t.Fatalf("fixture must reproduce the fresh-checkout false-positive without --ci; got %+v", c)
	}
	if !full.HasFailures() {
		t.Fatal("expected the unscoped run to have failures (sanity check on the fixture)")
	}

	// With SkipLocalEnv, none of the developer-local-environment checks
	// are present at all.
	ci := RunWithOptions(root, Options{SkipLocalEnv: true})
	for _, name := range []string{
		"Beads merge driver",
		"bd version floor",
		"bd schema drift",
		"bd on PATH",
		"Claude Code hooks",
		"Copilot hooks",
		"git pre-commit hook",
	} {
		if c := findCheck(ci, name); c != nil {
			t.Errorf("SkipLocalEnv=true must omit the %q check entirely; got %+v", name, c)
		}
	}
	if ci.HasFailures() {
		t.Errorf("a healthy repo-integrity fixture must exit clean under --ci; checks: %+v", ci.Checks)
	}
}

// TestRunWithOptions_SkipLocalEnv_StillFailsOnRealDivergence pins the other
// half of the CI-doctor contract: --ci must NOT become a blanket pass — a
// genuine repo-integrity / lifecycle-divergence finding (here, a stale-OPEN
// bead surfaced via the shared lifecycle predicate seam) still trips
// HasFailures with SkipLocalEnv=true, exactly as it would without it.
func TestRunWithOptions_SkipLocalEnv_StillFailsOnRealDivergence(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{
			StaleOpen: []lifecycle.StaleOpenBead{{BeadID: "stale-one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}},
		}
	})

	report := RunWithOptions(root, Options{SkipLocalEnv: true})
	if !report.HasFailures() {
		t.Fatalf("a real stale-open divergence must still fail under --ci; checks: %+v", report.Checks)
	}
	if c := findCheck(report, "stale-open bead: stale-one"); c == nil || c.Status != Error {
		t.Errorf("expected the stale-open bead check to survive SkipLocalEnv; got %+v", c)
	}
}
