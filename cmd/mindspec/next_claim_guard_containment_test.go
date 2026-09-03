package main

// next_claim_guard_containment_test.go — spec 128 (lifecycle-verb-state-
// honesty) Bead 7, ROUND-2 PANEL FIX: real end-to-end regressions for
// canonical-toplevel containment (G1, hard block) and the custom
// worktree_root recording divergence (G2). Reuses the
// next_ready_gate_test.go / next_claim_guard_test.go sandbox precedent.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next"
)

// TestNextCmd_ClaimGuard_EnumerationPin_CustomWorktreeRootRecording is
// round-2 panel G2's mandated custom-worktree_root pin. R3's round-2
// finding, precisely: workspace.SpecDir's own worktree-existence probe
// (workspace.go's DefaultWorktreesDir) hardcodes `.worktrees` regardless
// of a configured custom cfg.WorktreeRoot. So with a custom root
// configured, recording enabled, and a spec worktree PRESENT at the
// custom location, SpecDir's probe still looks under the hardcoded
// `.worktrees` (never finds anything there) and falls back to
// resolving the spec's recording dir at repoRoot itself —
// recording.EventsPath/ManifestPath therefore resolve UNDER repoRoot,
// not the custom worktree. The pre-round-2 samePath(anchor, cwd) gate
// got this exactly backwards: `anchor` (claimAnchorRoot, which IS
// cfg-aware) correctly found the custom worktree and returned IT, so a
// claim invoked from repoRoot (cwd == repoRoot, exactly where the write
// actually lands) had samePath(customWorktree, repoRoot) == false — the
// recording class was wrongly EXCLUDED despite the write being fully
// observable, and a pre-dirty manifest.json at repoRoot would have been
// silently waved through as ignoredDirt then read-modify-written.
//
// This fixture proves the round-2 fix closes that gap by construction:
// ComputeEntangleablePaths trusts recording's OWN resolved answer
// (wherever SpecDir actually sends it) and gates purely on whether that
// resolved target is CONTAINED in cwd's observed tree — irrespective of
// what claimAnchorRoot would have said, and irrespective of the
// decoy custom-root spec worktree SpecDir's probe never even looks at.
func TestNextCmd_ClaimGuard_EnumerationPin_CustomWorktreeRootRecording(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	specID := "981-claim-guard-custom-worktree-root"
	const customRoot = "custom-trees"

	sb.WriteFile(".mindspec/config.yaml", "worktree_root: "+customRoot+"\nrecording:\n  enabled: true\n")
	// The sandbox's default .gitignore hardcodes `.worktrees/` — swap in
	// the custom root's entry so the .gitignore class stays quiescent
	// and this fixture isolates G2's recording divergence.
	sb.WriteFile(".gitignore", ".beads/\n.harness/\n"+customRoot+"/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n.mindspec/phase.lock\n")
	// Seed the manifest where SpecDir's hardcoded-.worktrees probe
	// ACTUALLY resolves it under this config (its flat-default fallback
	// at repoRoot) — matching R3's finding precisely, not where a
	// naive reading of "custom worktree_root" might suggest.
	sb.WriteFile(".mindspec/specs/"+specID+"/recording/manifest.json",
		`{"spec_id":"`+specID+`","started_at":"2026-01-01T00:00:00Z","status":"recording","phases":[{"phase":"implement","started_at":"2026-01-01T00:00:00Z"}]}`+"\n")
	// The spec 124 readiness gate reads plan.md to EVALUATE readiness at
	// all (a missing plan.md is a hard evaluation error, not a
	// bypassable failed signal — --allow-not-ready below only bypasses
	// FAILED signals). Seed a minimal plan at the SAME repoRoot-relative
	// location SpecDir resolves under this config, so the claim reaches
	// the entanglement guard rather than erroring out beforehand; this
	// bead's specific readiness signals are irrelevant to what's under
	// test, hence --allow-not-ready rather than a fully valid plan.
	sb.WriteFile(".mindspec/specs/"+specID+"/spec.md", nextReadyGatePositiveSpecMD)
	sb.WriteFile(".mindspec/specs/"+specID+"/plan.md", strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", specID))
	sb.Commit("setup: custom worktree_root + recording enabled + seeded manifest")
	// config.Load's process-wide cache (see EnumerationPin_RecordingEnabled's
	// comment) must be reset so this TEST PROCESS's own
	// next.ComputeEntangleablePaths call below sees the fresh config.
	config.ResetCache()

	epicID := sb.CreateSpecEpic(specID)
	beadID := sb.CreateBead("[SPEC "+specID+"] Bead 1: custom worktree_root fixture", "task", epicID)

	// The decoy: a spec worktree genuinely PRESENT at the custom root.
	// claimAnchorRoot (cfg-aware) would find it for .gitignore purposes;
	// SpecDir's hardcoded probe never does, for recording purposes —
	// the exact divergence this fixture pins.
	specBranch := "spec/" + specID
	if out, err := sb.Run("git", "branch", specBranch); err != nil {
		t.Fatalf("git branch %s: %v\n%s", specBranch, err, out)
	}
	specWtRel := customRoot + "/worktree-spec-" + specID
	if out, err := sb.Run("git", "worktree", "add", specWtRel, specBranch); err != nil {
		t.Fatalf("git worktree add: %v\n%s", out, err)
	}

	if before := dirtyPorcelainPaths(t, sb); len(before) != 0 {
		t.Fatalf("fixture sanity: sandbox root must be CLEAN before the claim, got: %v", before)
	}

	// Predict BEFORE the claim runs (same discipline as the
	// GitignoreEntryAbsent pin — the write itself would change what a
	// post-claim prediction reads). The claim is invoked FROM sb.Root
	// (repoRoot), NOT the decoy custom worktree — exactly R3's finding's
	// shape.
	want := next.ComputeEntangleablePaths(sb.Root, sb.Root, specID)
	if len(want) != 2 {
		t.Fatalf("fixture sanity: expected exactly 2 predicted recording paths, got %v — G2 regression (the pre-dirty manifest at repoRoot would be silently waved through as ignoredDirt)", want)
	}

	// This bead has no plan-approve-derived readiness signals (no
	// acceptance criteria, no work_chunks) since plan approve was
	// bypassed — --allow-not-ready proceeds past that gate, which is
	// orthogonal to the entanglement mechanism under test here.
	out, err := sb.Run("mindspec", "next", beadID, "--allow-not-ready")
	if err != nil {
		t.Fatalf("expected the claim to succeed, got: %v\n%s", err, out)
	}

	got := dirtyPorcelainPaths(t, sb)
	assertPathsMatchEnumeration(t, got, want)
}

// TestNextCmd_ClaimGuard_SubdirectoryInvocationRefusesByteIdentical is
// round-2 panel G1/R6's mandated real-claim regression #1: with a
// genuinely dirty, entry-absent .gitignore in the spec worktree, a
// claim invoked from a SUBDIRECTORY of that worktree must still
// REFUSE — the pre-fix samePath(anchor, cwd) lexical-equality gate
// disarmed the class here even though the dirt is fully observable
// (porcelain is toplevel-relative regardless of cwd).
func TestNextCmd_ClaimGuard_SubdirectoryInvocationRefusesByteIdentical(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	specID := "980-claim-guard-subdir-invocation"
	planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", specID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, specID, nextReadyGatePositiveSpecMD, planMD)
	beadID := epicID + ".1"
	specWtDir := filepath.Join(sb.Root, ".worktrees", "worktree-spec-"+specID)
	gitCommandInDir(t, specWtDir, "config", "user.email", "test@mindspec.dev")
	gitCommandInDir(t, specWtDir, "config", "user.name", "MindSpec Test")

	// Dirty .gitignore, entry absent — WITHOUT committing.
	if err := os.WriteFile(filepath.Join(specWtDir, ".gitignore"), []byte(".beads/\n.harness/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n.mindspec/phase.lock\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A subdirectory of the spec worktree — an operator's shell being one
	// level deep (e.g. inside internal/) when they run `mindspec next` is
	// entirely ordinary.
	subdir := filepath.Join(specWtDir, "internal")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	statusBefore, _ := nextReadyGateBDStatus(t, sb, beadID)
	branchesBefore := sb.ListBranches("bead/")
	worktreesBefore := sb.ListWorktrees()
	gitignoreBefore, _ := os.ReadFile(filepath.Join(specWtDir, ".gitignore"))

	out, err := runMindspecInDir(t, sb, subdir, "next", beadID)
	if err == nil {
		t.Fatalf("expected the claim to REFUSE (dirty, entry-absent .gitignore, subdirectory invocation, round-2 panel G1); got exit 0:\n%s", out)
	}
	if !strings.Contains(out, "cannot claim work") {
		t.Errorf("expected the preserved DirtyTreeFailure contract, got:\n%s", out)
	}

	statusAfter, _ := nextReadyGateBDStatus(t, sb, beadID)
	if statusAfter != statusBefore {
		t.Errorf("bd status changed across the refused invocation: before=%q after=%q", statusBefore, statusAfter)
	}
	if !equalStringSlices(branchesBefore, sb.ListBranches("bead/")) {
		t.Errorf("bead branch list changed across the refused invocation: before=%v after=%v", branchesBefore, sb.ListBranches("bead/"))
	}
	if !equalStringSlices(worktreesBefore, sb.ListWorktrees()) {
		t.Errorf("worktree list changed across the refused invocation: before=%v after=%v", worktreesBefore, sb.ListWorktrees())
	}
	gitignoreAfter, _ := os.ReadFile(filepath.Join(specWtDir, ".gitignore"))
	if string(gitignoreBefore) != string(gitignoreAfter) {
		t.Error(".gitignore bytes changed across the refused invocation (a refusal must be a total no-op)")
	}
}

// TestNextCmd_ClaimGuard_SymlinkAliasInvocationRefusesByteIdentical is
// round-2 panel G1/R6's mandated real-claim regression #2: same dirty,
// entry-absent .gitignore, but the claim is invoked through a SYMLINK
// ALIAS of the spec worktree rather than the worktree's own lexical
// path — the pre-fix gate evaded entirely here (a different string,
// the identical tree).
func TestNextCmd_ClaimGuard_SymlinkAliasInvocationRefusesByteIdentical(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on windows")
	}
	sb := nextReadyGateSetupSandbox(t)

	specID := "979-claim-guard-symlink-invocation"
	planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", specID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, specID, nextReadyGatePositiveSpecMD, planMD)
	beadID := epicID + ".1"
	specWtDir := filepath.Join(sb.Root, ".worktrees", "worktree-spec-"+specID)
	gitCommandInDir(t, specWtDir, "config", "user.email", "test@mindspec.dev")
	gitCommandInDir(t, specWtDir, "config", "user.name", "MindSpec Test")

	if err := os.WriteFile(filepath.Join(specWtDir, ".gitignore"), []byte(".beads/\n.harness/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n.mindspec/phase.lock\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	alias := filepath.Join(t.TempDir(), "spec-worktree-alias")
	if err := os.Symlink(specWtDir, alias); err != nil {
		t.Fatalf("creating symlink alias: %v", err)
	}

	statusBefore, _ := nextReadyGateBDStatus(t, sb, beadID)
	branchesBefore := sb.ListBranches("bead/")
	worktreesBefore := sb.ListWorktrees()
	gitignoreBefore, _ := os.ReadFile(filepath.Join(specWtDir, ".gitignore"))

	out, err := runMindspecInDir(t, sb, alias, "next", beadID)
	if err == nil {
		t.Fatalf("expected the claim to REFUSE (dirty, entry-absent .gitignore, symlink-alias invocation, round-2 panel G1); got exit 0:\n%s", out)
	}
	if !strings.Contains(out, "cannot claim work") {
		t.Errorf("expected the preserved DirtyTreeFailure contract, got:\n%s", out)
	}

	statusAfter, _ := nextReadyGateBDStatus(t, sb, beadID)
	if statusAfter != statusBefore {
		t.Errorf("bd status changed across the refused invocation: before=%q after=%q", statusBefore, statusAfter)
	}
	if !equalStringSlices(branchesBefore, sb.ListBranches("bead/")) {
		t.Errorf("bead branch list changed across the refused invocation: before=%v after=%v", branchesBefore, sb.ListBranches("bead/"))
	}
	if !equalStringSlices(worktreesBefore, sb.ListWorktrees()) {
		t.Errorf("worktree list changed across the refused invocation: before=%v after=%v", worktreesBefore, sb.ListWorktrees())
	}
	gitignoreAfter, _ := os.ReadFile(filepath.Join(specWtDir, ".gitignore"))
	if string(gitignoreBefore) != string(gitignoreAfter) {
		t.Error(".gitignore bytes changed across the refused invocation (a refusal must be a total no-op)")
	}
	target, err := os.Readlink(alias)
	if err != nil || target != specWtDir {
		t.Errorf("symlink alias target changed across the refused invocation: got %q, err %v", target, err)
	}
}
