// Package executor_test hosts spec 125 Bead 3's cross-package MF-3 e2e
// (AC-1b + AC-2, R6(a)): the full pinned conflict-recovery miss shape
// driven through PRODUCTION CompleteBead and then through
// internal/lifecycle.FindLandedMerge — the exact contract spec 124's MF-3
// readiness signal consumes.
//
// This file is DELIBERATELY an external `_test` package (the plan's
// F3-1/G1-F1 mechanism note): from here the unexported binding seams
// (mergeBindingFn, mergeBindingReadFn, landedBindingMetadataFn) are
// UNREACHABLE, so the test cannot stub them even by accident — it drives
// the PRODUCTION seam defaults (bead.MergeMetadata / bead.GetMetadata)
// end-to-end against a stateful fake-`bd`-on-PATH (the internal/approve
// plan_fault_test.go scratch-bin/ + t.Setenv("PATH", …) pattern), which is
// STRONGER than seam stubbing: the real bd-CLI write and read code paths
// run, persisting to the same store across processes. The executor→
// lifecycle import here is TEST-ONLY — the executor-must-not-import-
// lifecycle PRODUCTION boundary stands untouched.
//
// Spec-119 no-skip gating: the fake bd IS this test's declared
// environment, so it FATALs — never skips — when the shim is absent or
// not runnable; it never skips on a missing real bd (it needs none).
package executor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// fakeBDScript is the stateful fake `bd` (plan-gate F3-1/G1-F1). It
// persists `bd update --metadata` payloads under $FAKE_BD_STORE and plays
// them back to `bd show <id> --json`, so bead.MergeMetadata's write and
// bead.GetMetadata's read observe the SAME store — a real end-to-end run
// of the production seam defaults. Per plan-gate F3-R2-1 it ALSO exits 0
// for every other bd verb the production path shells (`bd export`,
// `bd worktree create/list/remove`) — without these the e2e reds opaquely
// inside commit/cleanup plumbing instead of exercising the contract; the
// `worktree list` arm additionally emits valid empty JSON because
// bead.WorktreeList parses its stdout.
const fakeBDScript = `#!/bin/sh
# Stateful fake bd for spec 125 Bead 3's landed e2e (see landed_e2e_test.go).
case "$1" in
  export)
    exit 0
    ;;
  worktree)
    case "$2" in
      list) printf '[]\n' ;;
      create|remove) : ;;
    esac
    exit 0
    ;;
  update)
    # bd update <id> --metadata <json>  (replace semantics, like real bd)
    if [ "$3" = "--metadata" ] && [ -n "$FAKE_BD_STORE" ]; then
      printf '[{"metadata":%s}]' "$4" > "$FAKE_BD_STORE/$2.json"
    fi
    exit 0
    ;;
  show)
    # bd show <id> --json
    if [ -n "$FAKE_BD_STORE" ] && [ -f "$FAKE_BD_STORE/$2.json" ]; then
      cat "$FAKE_BD_STORE/$2.json"
    else
      printf '[{"metadata":{}}]\n'
    fi
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`

// installFakeBD writes the shim into a scratch bin/, prepends it to PATH,
// and points $FAKE_BD_STORE at a scratch store dir. FATAL — never skip —
// when the shim does not resolve first on PATH or is not runnable
// (spec-119 no-skip gating: the shim IS the declared environment).
func installFakeBD(t *testing.T) {
	t.Helper()
	scratchBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(scratchBin, 0o755); err != nil {
		t.Fatalf("mkdir scratch bin: %v", err)
	}
	shim := filepath.Join(scratchBin, "bd")
	if err := os.WriteFile(shim, []byte(fakeBDScript), 0o755); err != nil {
		t.Fatalf("write fake bd shim: %v", err)
	}
	t.Setenv("FAKE_BD_STORE", t.TempDir())
	t.Setenv("PATH", scratchBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	resolved, err := exec.LookPath("bd")
	if err != nil || resolved != shim {
		t.Fatalf("fake bd shim must resolve first on PATH (resolved %q, err %v) — FATAL, not skip (spec 119)", resolved, err)
	}
	if out, runErr := exec.Command("bd", "export").CombinedOutput(); runErr != nil {
		t.Fatalf("fake bd shim is not runnable (bd export: %v, %s) — FATAL, not skip (spec 119)", runErr, out)
	}
}

// gitE2E runs a git command in dir, fataling on failure.
func gitE2E(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitE2ERaw(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %s (%v)", args, out, err)
	}
	return strings.TrimSpace(string(out))
}

func gitE2ERaw(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	return cmd.CombinedOutput()
}

// TestLandedE2E_ConflictRecoveryBindsAndFindLandedMergeIdentifies is
// AC-1b + AC-2 (RED on the spec-init SHA: the exact-subject scan missed
// the default-subject recovery merge, the silent-nil swallow wrote no
// binding, and FindLandedMerge fail-closed on the branch-deleted bead —
// the 755/757 fleet state). The full R6(a) chain, all through PRODUCTION
// code and the PRODUCTION seam defaults:
//
//  1. a 2nd bead under a spec hits a REAL add/add conflict on the tracked
//     .beads/issues.jsonl → CompleteBead refuses; its printed recovery
//     line carries `-m "Merge <beadBranch>"` (AC-1b's message half);
//  2. an operator following the PRE-fix line verbatim (no -m) resolves and
//     commits — landing git's DEFAULT subject;
//  3. the recovery re-run of production CompleteBead (bead already an
//     ancestor, no-op merge) persists the binding through the REAL
//     bead.MergeMetadata → fake-bd store (Bead 1's write half), then
//     deletes the branch;
//  4. with the branch DELETED and the binding the ONLY datum,
//     lifecycle.FindLandedMerge POSITIVELY identifies the merge — the
//     exact *LandedMerge contract spec 124's MF-3 consumes (AC-2).
func TestLandedE2E_ConflictRecoveryBindsAndFindLandedMergeIdentifies(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	installFakeBD(t)

	const beadID = "mindspec-e2e1.2"
	const specBranch = "spec/125-e2e"
	beadBranch := "bead/" + beadID

	dir := t.TempDir()
	gitE2E(t, dir, "init", "-b", "main")
	gitE2E(t, dir, "config", "user.email", "test@example.com")
	gitE2E(t, dir, "config", "user.name", "test")
	gitE2E(t, dir, "commit", "--allow-empty", "-m", "root")
	gitE2E(t, dir, "branch", specBranch)
	gitE2E(t, dir, "branch", beadBranch)

	// Spec worktree at the canonical path CompleteBead derives, with a
	// prior bead's already-merged .beads/issues.jsonl export committed
	// (the spec-side half of the add/add conflict).
	specWt := filepath.Join(dir, ".worktrees", "worktree-spec-125-e2e")
	if err := os.MkdirAll(filepath.Dir(specWt), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gitE2E(t, dir, "worktree", "add", specWt, specBranch)
	t.Cleanup(func() { _, _ = gitE2ERaw(dir, "worktree", "remove", "--force", specWt) })
	if err := os.MkdirAll(filepath.Join(specWt, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specWt, ".beads", "issues.jsonl"), []byte("spec-side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitE2E(t, specWt, "add", ".")
	gitE2E(t, specWt, "commit", "-m", "chore: commit remaining spec artifacts")

	// The 2nd bead's own commits — the SAME tracked path (bead-side half
	// of the add/add conflict) plus a deliverable — authored via a
	// TEMPORARY worktree that is removed before CompleteBead runs, so the
	// branch survives unchecked-out (the real fleet shape by cleanup time)
	// and the shim's no-op `bd worktree remove` cannot strand a checkout
	// that would block branch deletion.
	beadWt := filepath.Join(dir, ".wt-bead-e2e")
	gitE2E(t, dir, "worktree", "add", beadWt, beadBranch)
	if err := os.MkdirAll(filepath.Join(beadWt, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadWt, ".beads", "issues.jsonl"), []byte("bead-side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadWt, "e2e.txt"), []byte("bead work\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitE2E(t, beadWt, "add", ".")
	gitE2E(t, beadWt, "commit", "-m", "bead work")
	gitE2E(t, dir, "worktree", "remove", "--force", beadWt)

	// Production callers invoke the executor from the repo root (gitutil's
	// BranchExists/DeleteBranch operate on $PWD).
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// PRODUCTION executor: default WorktreeOps (bd worktree … via the
	// shim) and default binding seams (bead.MergeMetadata/GetMetadata via
	// the shim) — nothing stubbed.
	g := executor.NewMindspecExecutor(dir)

	// 1. First run: CompleteBead's own MergeInto hits the real add/add
	// conflict and refuses; the printed recovery line must carry -m so a
	// verbatim-following operator produces an identifiable subject
	// (AC-1b's message half, Bead 1's beadToSpecConflictFailure fix).
	err = g.CompleteBead(beadID, specBranch, "")
	if err == nil {
		t.Fatal("expected the bead→spec add/add conflict to refuse")
	}
	wantRecovery := `git merge --no-ff -m "Merge ` + beadBranch + `" ` + beadBranch
	if !strings.Contains(err.Error(), wantRecovery) {
		t.Errorf("conflict recovery must print %q (AC-1b), got:\n%v", wantRecovery, err)
	}
	if _, refErr := gitE2ERaw(dir, "rev-parse", "--verify", "refs/heads/"+beadBranch); refErr != nil {
		t.Fatal("fixture: the conflicted bead branch must survive the refusal")
	}

	// 2. Operator recovery following the PRE-fix line verbatim (no -m):
	// the merge conflicts, the operator resolves and commits, and git
	// writes its DEFAULT subject — the exact miss shape 755/757 fleet
	// beads are stranded in.
	_, _ = gitE2ERaw(specWt, "merge", "--no-ff", beadBranch) // conflicts by construction
	if err := os.WriteFile(filepath.Join(specWt, ".beads", "issues.jsonl"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	gitE2E(t, specWt, "add", ".")
	gitE2E(t, specWt, "commit", "--no-edit")

	mergeSHA := gitE2E(t, dir, "rev-parse", specBranch)
	beadTip := gitE2E(t, dir, "rev-parse", beadBranch)
	subject := gitE2E(t, dir, "log", "--format=%s", "-1", mergeSHA)
	if subject == "Merge "+beadBranch {
		t.Fatalf("fixture: the recovery merge must land git's DEFAULT subject, got the exact form %q", subject)
	}

	// 3. Recovery re-run of PRODUCTION CompleteBead: already-ancestor
	// no-op merge; the binding is persisted via the REAL
	// bead.MergeMetadata (bd update through the shim), then cleanup
	// deletes the branch.
	if err := g.CompleteBead(beadID, specBranch, ""); err != nil {
		t.Fatalf("the recovery re-run must converge, got: %v", err)
	}

	// Binding durably visible through the SAME bd surface `bd show` reads
	// (the REAL bead.GetMetadata, no seam): AC-1b's write half.
	meta, err := bead.GetMetadata(beadID)
	if err != nil {
		t.Fatalf("bead.GetMetadata through the fake bd store: %v", err)
	}
	if got, _ := meta["mindspec_landed_merge_sha"].(string); got != mergeSHA {
		t.Errorf("mindspec_landed_merge_sha = %q, want the rev-parse-verified merge %q", got, mergeSHA)
	}
	if got, _ := meta["mindspec_landed_second_parent"].(string); got != beadTip {
		t.Errorf("mindspec_landed_second_parent = %q, want the bead tip %q", got, beadTip)
	}

	// Branch deleted — the binding is now the ONLY datum for this bead.
	if _, refErr := gitE2ERaw(dir, "rev-parse", "--verify", "refs/heads/"+beadBranch); refErr == nil {
		t.Fatal("the bead branch must be deleted after the converged recovery re-run")
	}

	// 4. AC-2 — the MF-3 contract: FindLandedMerge, through its OWN
	// production defaults (landedBindingMetadataFn = bead.GetMetadata →
	// the shim), POSITIVELY identifies the branch-DELETED bead's merge —
	// not *LandedMergeNoEvidence, not ErrLandedMergeNotFound. Ownership
	// is nominated by the DEFAULT subject's parsed branch name AND the
	// binding names the merge; landed-ness is the exact second parent.
	landed, err := lifecycle.FindLandedMerge(dir, specBranch, beadID)
	if err != nil {
		t.Fatalf("FindLandedMerge must positively identify the branch-deleted bead (the MF-3 contract), got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("LandedMerge.SHA = %q, want %q", landed.SHA, mergeSHA)
	}
	if landed.SecondParent != beadTip {
		t.Errorf("LandedMerge.SecondParent = %q, want %q", landed.SecondParent, beadTip)
	}
	if landed.FirstParent == "" {
		t.Error("LandedMerge.FirstParent must be populated (the M^1..M evidence range)")
	}
}
