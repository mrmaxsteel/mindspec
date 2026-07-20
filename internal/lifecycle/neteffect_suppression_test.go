package lifecycle

// Spec 121 Bead 1 (R4, AC-9/AC-19): real bare-origin fixtures for the
// doctor merged-carrier suppression's net-effect rewiring
// (FindOutstandingFinalizeBranches). Unlike finalize_orphans_test.go's
// seam-stubbed unit tests, these exercise the REAL gitutil.NetEffectLanded
// through the package's unstubbed seam default — no test in this file
// overrides finalizeOrphanNetEffectFn.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func neSuppRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
}

func neSuppWriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// setupSuppressionRepo creates a real repo with a bare "origin" remote and
// main pushed, ready for a chore/finalize-<specID> carrier fixture.
func setupSuppressionRepo(t *testing.T) (dir, origin string) {
	t.Helper()
	dir = t.TempDir()
	neSuppRunGit(t, dir, "init", "-b", "main")
	neSuppWriteFile(t, dir, "README.md", "seed\n")
	neSuppRunGit(t, dir, "add", ".")
	neSuppRunGit(t, dir, "commit", "-m", "init")
	origin = t.TempDir()
	neSuppRunGit(t, origin, "init", "--bare", "-b", "main")
	neSuppRunGit(t, dir, "remote", "add", "origin", origin)
	neSuppRunGit(t, dir, "push", "-u", "origin", "main")
	return dir, origin
}

// TestFindOutstandingFinalizeBranches_RealSquashMergedCarrierSuppressed is
// AC-9's positive half at the REAL doctor consumer: a chore/finalize
// carrier squash-merged into origin/main (SHA ancestry false) must be
// suppressed via the net-effect fallback — the squash blind spot this bead
// closes. RED on today's main.
func TestFindOutstandingFinalizeBranches_RealSquashMergedCarrierSuppressed(t *testing.T) {
	dir, _ := setupSuppressionRepo(t)
	neSuppRunGit(t, dir, "checkout", "-b", "chore/finalize-077-test")
	neSuppWriteFile(t, dir, "carrier.txt", "export\n")
	neSuppRunGit(t, dir, "add", ".")
	neSuppRunGit(t, dir, "commit", "-m", "finalize export")
	neSuppRunGit(t, dir, "checkout", "main")
	neSuppRunGit(t, dir, "merge", "--squash", "chore/finalize-077-test")
	neSuppRunGit(t, dir, "commit", "-m", "squash merge carrier")
	neSuppRunGit(t, dir, "push", "origin", "main")

	orphans, err := FindOutstandingFinalizeBranches(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("a squash-merged carrier must be suppressed, got %+v", orphans)
	}
}

// TestFindOutstandingFinalizeBranches_RealUnmergedCarrierStillFlagged is
// AC-9's negative half: a carrier that was never merged anywhere must
// still be flagged — the normal (unmerged) path stays unchanged.
func TestFindOutstandingFinalizeBranches_RealUnmergedCarrierStillFlagged(t *testing.T) {
	dir, _ := setupSuppressionRepo(t)
	neSuppRunGit(t, dir, "checkout", "-b", "chore/finalize-077-test")
	neSuppWriteFile(t, dir, "carrier.txt", "export\n")
	neSuppRunGit(t, dir, "add", ".")
	neSuppRunGit(t, dir, "commit", "-m", "finalize export")
	neSuppRunGit(t, dir, "checkout", "main")

	orphans, err := FindOutstandingFinalizeBranches(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 1 || orphans[0].Branch != "chore/finalize-077-test" {
		t.Fatalf("a genuinely unmerged carrier must still be flagged, got %+v", orphans)
	}
}

// TestFindOutstandingFinalizeBranches_RealTrueMergeThenRevertFlaggedAgain
// is AC-19(iv) at the DOCTOR consumer: a carrier TRULY (non-squash) merged
// into origin/main — SHA ancestry HOLDS — whose content was subsequently
// REVERTED there must be flagged AGAIN: ancestry alone must NOT suppress it
// forever, unlike the FinalizeEpic probe's deliberately-unchanged
// ancestry-routed behavior for the identical shape (see the executor
// package's own AC-19(iv) fixture, asserting the opposite polarity by
// design — the per-consumer split this bead's Requirement 4 pins). RED on
// today's main (ancestry-only suppression would hide this forever).
func TestFindOutstandingFinalizeBranches_RealTrueMergeThenRevertFlaggedAgain(t *testing.T) {
	dir, _ := setupSuppressionRepo(t)
	neSuppRunGit(t, dir, "checkout", "-b", "chore/finalize-077-test")
	neSuppWriteFile(t, dir, "carrier.txt", "export\n")
	neSuppRunGit(t, dir, "add", ".")
	neSuppRunGit(t, dir, "commit", "-m", "finalize export")
	neSuppRunGit(t, dir, "checkout", "main")
	neSuppRunGit(t, dir, "merge", "--no-ff", "-m", "Merge chore/finalize-077-test", "chore/finalize-077-test")
	neSuppRunGit(t, dir, "revert", "--no-edit", "-m", "1", "HEAD")
	neSuppRunGit(t, dir, "push", "origin", "main")

	orphans, err := FindOutstandingFinalizeBranches(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 1 || orphans[0].Branch != "chore/finalize-077-test" {
		t.Fatalf("a true-merge-then-reverted carrier must be flagged again (ancestry alone must not suppress it), got %+v", orphans)
	}
}

// TestFindOutstandingFinalizeBranches_RealSupersedingExportSuppressed is
// the leg-(b) "later superseding export" fixture at the real consumer: a
// tracker-only carrier bumps an epic's status, and origin/main
// INDEPENDENTLY already carries an equal-or-later status for it (a
// superseding export) — this is a genuine textual conflict at leg (a),
// confined to .beads/issues.jsonl, so leg (b)'s status-total-order
// subsumption applies and the carrier is suppressed.
func TestFindOutstandingFinalizeBranches_RealSupersedingExportSuppressed(t *testing.T) {
	dir, _ := setupSuppressionRepo(t)
	neSuppWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"open"}`+"\n")
	neSuppRunGit(t, dir, "add", ".")
	neSuppRunGit(t, dir, "commit", "-m", "seed tracker export")
	neSuppRunGit(t, dir, "push", "origin", "main")

	neSuppRunGit(t, dir, "checkout", "-b", "chore/finalize-077-test")
	neSuppWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"in_progress"}`+"\n")
	neSuppRunGit(t, dir, "add", ".")
	neSuppRunGit(t, dir, "commit", "-m", "carrier: bump to in_progress")

	neSuppRunGit(t, dir, "checkout", "main")
	neSuppWriteFile(t, dir, ".beads/issues.jsonl", `{"id":"epic-1","status":"closed"}`+"\n")
	neSuppRunGit(t, dir, "add", ".")
	neSuppRunGit(t, dir, "commit", "-m", "main: superseding export closes epic-1")
	neSuppRunGit(t, dir, "push", "origin", "main")

	orphans, err := FindOutstandingFinalizeBranches(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("a carrier whose content a LATER superseding export already satisfies must be suppressed, got %+v", orphans)
	}
}
