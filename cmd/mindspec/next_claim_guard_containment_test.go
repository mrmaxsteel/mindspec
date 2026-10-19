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
	f := newScopedNextFixture(t)
	const specID = "129-fixture"
	const customRoot = "custom-trees"
	write := func(path, text string) {
		target := filepath.Join(f.root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(".mindspec/config.yaml", "worktree_root: "+customRoot+"\nrecording:\n  enabled: true\n")
	write(".gitignore", "tracker*\nbd\ncaptured-*\n"+customRoot+"/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n.mindspec/phase.lock\n")
	write(".mindspec/specs/"+specID+"/recording/manifest.json", `{"spec_id":"129-fixture","started_at":"2026-01-01T00:00:00Z","status":"recording","phases":[{"phase":"implement","started_at":"2026-01-01T00:00:00Z"}]}`+"\n")
	honestyGit(t, f.root, "add", ".")
	honestyGit(t, f.root, "commit", "-m", "custom root recording fixture")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	honestyGit(t, f.root, "worktree", "add", customRoot+"/worktree-spec-"+specID, "spec/"+specID)
	// The anchor's own ignore entry is present, isolating recording writes.
	if err := os.WriteFile(filepath.Join(f.root, customRoot, "worktree-spec-"+specID, ".gitignore"), []byte(customRoot+"/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := honestyGit(t, f.root, "status", "--porcelain"); got != "" {
		t.Fatalf("fixture must be clean: %s", got)
	}
	want := next.ComputeEntangleablePaths(f.root, f.root, specID)
	if len(want) != 2 {
		t.Fatalf("expected exactly two recording paths: %v", want)
	}
	out, err := runScopedNext(t, f, "epic-129.1", "--allow-not-ready=MF-1,MF-2,MF-3,MF-4", "--evidence", nextEvidenceFile(t))
	if err != nil {
		t.Fatalf("claim: %v %s", err, out)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimRight(honestyGit(t, f.root, "status", "--porcelain"), "\r\n"), "\n") {
		if len(line) >= 3 {
			got = append(got, line[3:])
		}
	}
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
