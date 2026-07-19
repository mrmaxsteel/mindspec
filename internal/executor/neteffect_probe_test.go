package executor

// Spec 121 Bead 1 (R4, AC-8/AC-9/AC-17/AC-19): real bare-origin fixtures for
// the FinalizeEpic probe's net-effect fallback — the squash-merge blind
// spot fix (mindspec-3xqm item 1). These extend the bug-wu7t fixtures in
// finalize_orphan_test.go (setupOrphanOrigin's --no-ff merge shape); here
// the spec branch's commits are discarded from origin/main's history
// entirely (squash), the shape SHA ancestry cannot see.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// setupSquashMergedOrigin mirrors setupOrphanOrigin but SQUASH-merges
// spec/077-test into main (locally and on origin) instead of a --no-ff
// merge commit, so the spec branch's own SHAs never appear in origin/main's
// history at all — the shape SHA ancestry alone cannot detect.
func setupSquashMergedOrigin(t *testing.T, dir string) string {
	t.Helper()
	origin := t.TempDir()
	runGitIn(t, origin, "init", "--bare", "-b", "main")
	runGitIn(t, dir, "remote", "add", "origin", origin)
	runGitIn(t, dir, "push", "-u", "origin", "main")

	runGitIn(t, dir, "checkout", "-b", "spec/077-test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "spec change")
	runGitIn(t, dir, "checkout", "main")
	runGitIn(t, dir, "merge", "--squash", "spec/077-test")
	runGitIn(t, dir, "commit", "-m", "squash merge spec/077-test")
	runGitIn(t, dir, "push", "origin", "main")
	return origin
}

// TestFinalizeEpic_Probe_SquashMergedRoutesToCarrier is AC-8's core: a
// squash-merged spec branch (SHA ancestry false) must still route to the
// chore/finalize-<specID> carrier via the net-effect fallback, instead of
// pushing the epic-close commit onto the now-dead spec branch (the pre-121
// bug). RED on today's main (the documented blind spot).
func TestFinalizeEpic_Probe_SquashMergedRoutesToCarrier(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	origin := setupSquashMergedOrigin(t, dir)
	fake.listEntries = nil

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantBranch := "chore/finalize-077-test"
	if result.FinalizeBranch != wantBranch {
		t.Fatalf("FinalizeBranch = %q, want %q (a squash-merged spec branch must route to the carrier)", result.FinalizeBranch, wantBranch)
	}
	if !branchExistsIn(t, origin, wantBranch) {
		t.Fatalf("%s must exist on the remote", wantBranch)
	}
}

// TestFinalizeEpic_Probe_SquashThenRevertNotRouted is AC-19(i) at the probe:
// a squash-merge whose content was SUBSEQUENTLY REVERTED on origin/main
// must NOT be routed to the carrier — main's current content no longer
// carries the work, so pushing the epic-close commit on the normal spec
// branch push path is correct (the branch is genuinely not landed).
func TestFinalizeEpic_Probe_SquashThenRevertNotRouted(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	origin := setupSquashMergedOrigin(t, dir)
	runGitIn(t, dir, "revert", "--no-edit", "HEAD")
	runGitIn(t, dir, "push", "origin", "main")
	fake.listEntries = nil

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalizeBranch != "" {
		t.Errorf("FinalizeBranch = %q, want empty (a squash-then-reverted branch must NOT route to the carrier)", result.FinalizeBranch)
	}
	if branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Error("no chore/finalize branch should exist when the squashed content was reverted")
	}
}

// TestFinalizeEpic_Probe_SquashThenUnrelatedLaterChangesStillRoutes is
// AC-19(iii): a squash-merge followed by UNRELATED later changes on
// origin/main must STILL route to the carrier.
func TestFinalizeEpic_Probe_SquashThenUnrelatedLaterChangesStillRoutes(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	setupSquashMergedOrigin(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("unrelated"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "unrelated later change")
	runGitIn(t, dir, "push", "origin", "main")
	fake.listEntries = nil

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalizeBranch != "chore/finalize-077-test" {
		t.Errorf("FinalizeBranch = %q, want chore/finalize-077-test (unrelated later main changes must not un-land the squash)", result.FinalizeBranch)
	}
}

// TestFinalizeEpic_Probe_TrueMergeThenRevertStaysAncestryRouted is AC-19(iv)
// at the PROBE: a TRUE (non-squash) merge, later reverted on origin/main,
// keeps SHA ancestry true — the probe's per-consumer contract deliberately
// leaves this ancestry-routed (the spent-carrier justification: the spec
// branch is a spent PR carrier regardless of later history), UNCHANGED
// from today. This pins the per-consumer split against the DOCTOR
// suppression, which — unlike this probe — is asserted to un-suppress in
// exactly this shape (see internal/lifecycle's own AC-19(iv) fixture).
func TestFinalizeEpic_Probe_TrueMergeThenRevertStaysAncestryRouted(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	origin := setupOrphanOrigin(t, dir, true) // true --no-ff merge, already merged
	runGitIn(t, dir, "revert", "--no-edit", "-m", "1", "HEAD")
	runGitIn(t, dir, "push", "origin", "main")
	fake.listEntries = nil

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalizeBranch != "chore/finalize-077-test" {
		t.Errorf("FinalizeBranch = %q, want chore/finalize-077-test (ancestry alone still routes at the probe, unchanged)", result.FinalizeBranch)
	}
	if !branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Fatal("chore/finalize-077-test must exist on the remote")
	}
}

// TestFinalizeEpic_Probe_NetEffectInfraFailureWarnsAndFailsOpen is the
// per-consumer INFRA-POSTURE subtest (panel F2) for the probe: when the
// net-effect fallback errors (ancestry already false), the probe WARNS
// naming itself undetermined and proceeds on the ancestry answer alone
// (false) — the DELIBERATE fail-open spec 121 R4 pins, never a hard
// failure of `impl approve`.
func TestFinalizeEpic_Probe_NetEffectInfraFailureWarnsAndFailsOpen(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	origin := setupSquashMergedOrigin(t, dir)
	fake.listEntries = nil

	orig := netEffectLandedFn
	t.Cleanup(func() { netEffectLandedFn = orig })
	sentinel := errors.New("simulated old-git net-effect infra failure")
	netEffectLandedFn = func(workdir, ref, target string) (bool, error) {
		return false, sentinel
	}

	var result FinalizeResult
	var err error
	stderr := captureStderrAround(t, func() {
		result, err = g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	})
	if err != nil {
		t.Fatalf("a net-effect infra failure must fail OPEN (never a hard `impl approve` failure), got: %v", err)
	}
	if result.FinalizeBranch != "" {
		t.Errorf("FinalizeBranch = %q, want empty (fail-open: proceed on ancestry alone)", result.FinalizeBranch)
	}
	if !strings.Contains(stderr, "net-effect probe undetermined") {
		t.Errorf("expected the undetermined-probe warning on stderr, got:\n%s", stderr)
	}
	if branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Error("no chore/finalize branch should be created on the fail-open path")
	}
}

// TestNetEffectLandedFn_IsGitutilNetEffectLanded is AC-17's executor-side
// anti-drift pin: the probe's seam default MUST be the identical exported
// symbol the doctor merged-carrier suppression falls back to
// (internal/lifecycle's finalizeOrphanNetEffectFn) — never a private
// reimplementation at either site (the 119 AC-12 pattern,
// doctor/lifecycle_integrity_test.go:169 precedent).
func TestNetEffectLandedFn_IsGitutilNetEffectLanded(t *testing.T) {
	if reflect.ValueOf(netEffectLandedFn).Pointer() != reflect.ValueOf(gitutil.NetEffectLanded).Pointer() {
		t.Fatal("netEffectLandedFn must be gitutil.NetEffectLanded (AC-17 anti-drift: the probe and the doctor suppression must invoke the identical exported predicate)")
	}
}
