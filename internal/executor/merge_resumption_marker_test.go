package executor

// Spec 127 bead-6 fix round 3: BLOCKING 2's acceptance test (G1-1's
// confirm-round finding). classifyPreservedMergeBinding's
// ancestor-but-not-equal leg used to trust mergeSubjectNamesSource's
// MERGE_MSG-subject check ALONE — but MERGE_MSG is a plain file git
// itself invites an operator to hand-edit before finishing a merge. An
// operator who rewrites its first line to falsely claim "Merge
// <expectedSource>" over a preserved FOREIGN merge (one that merely
// happens to be an ancestor of expectedSource's current tip) got it
// classified bindingDrifted and silently completed as this bead's own
// landing.
//
// This mirrors TestCompleteBead_ResolveMerge_AncestorForeignMergeRefuses'
// fixture exactly (a decoy branch made a real ancestor of the bead
// branch's current tip via an unrelated merge, then preserved as a
// FOREIGN conflict in the spec worktree) but ADDS the hand-edit: MERGE_MSG's
// subject is rewritten to falsely claim the bead branch's own identity
// before --resolve-merge runs. The fix (mergeSourceMarkerMatches,
// gitutil.MergeSourceMarkerRef) must still refuse — the subject's claim
// is no longer sufficient by itself; nothing in this fixture ever ran a
// real gitutil.MergeInto/MergeBranch for the bead branch in this spec
// worktree, so no corroborating marker exists at all, and the refusal
// must be the fail-closed "cannot verify" leg, never a silent
// bindingDrifted completion.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// rewriteMergeMsgSubject hand-edits workdir's MERGE_MSG first line to
// newSubject — exactly the "ordinary operator action" G1's finding says
// must never be sufficient, alone, to forge a preserved merge's
// identity. Uses `git rev-parse --git-path MERGE_MSG` (the same
// resolution gitutil.MergeMsgSubject itself uses) so this works
// correctly against a linked worktree's MERGE_MSG, not the shared
// repository's own.
func rewriteMergeMsgSubject(t *testing.T, workdir, newSubject string) {
	t.Helper()
	out, err := exec.Command("git", "-C", workdir, "rev-parse", "--git-path", "MERGE_MSG").Output()
	if err != nil {
		t.Fatalf("resolving MERGE_MSG path: %v", err)
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(workdir, path)
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading MERGE_MSG before rewrite: %v", err)
	}
	lines := strings.SplitN(string(existing), "\n", 2)
	rest := ""
	if len(lines) > 1 {
		rest = "\n" + lines[1]
	}
	if err := os.WriteFile(path, []byte(newSubject+rest), 0o644); err != nil {
		t.Fatalf("hand-editing MERGE_MSG: %v", err)
	}
}

// TestCompleteBead_ResolveMerge_HandEditedMergeMessageCannotForgeBinding
// is BLOCKING 2's acceptance test: hand-editing MERGE_MSG's subject to
// falsely claim the bead branch's own identity over a preserved,
// genuinely-foreign (but ancestor) merge must NOT flip the classification
// to bindingDrifted — the merge-start marker (producer-written evidence,
// not unforgeable; see gitutil.MergeSourceMarkerRef's doc comment) must
// still refuse to corroborate it.
func TestCompleteBead_ResolveMerge_HandEditedMergeMessageCannotForgeBinding(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/077-forge")
	runGitIn(t, dir, "branch", "bead/mindspec-forge.1")
	runGitIn(t, dir, "branch", "decoy-src-forge")

	specWtPath := dir + "/.worktrees/worktree-spec-077-forge"
	if err := os.MkdirAll(dir+"/.worktrees", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-forge")

	// decoy-src-forge: an UNRELATED branch that becomes the FOREIGN
	// preserved merge's real source.
	decoyWt := dir + "/.wt-decoy-src-forge"
	runGitIn(t, dir, "worktree", "add", decoyWt, "decoy-src-forge")
	if err := os.WriteFile(decoyWt+"/shared.txt", []byte("decoy version\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, decoyWt, "add", ".")
	runGitIn(t, decoyWt, "commit", "-m", "decoy work")
	runGitIn(t, dir, "worktree", "remove", "--force", decoyWt)
	decoyTip := refHash(t, dir, "decoy-src-forge")

	// bead/mindspec-forge.1: its own work PLUS a real, clean merge of
	// decoy-src-forge for unrelated reasons — decoyTip becomes an
	// ANCESTOR of the bead branch's current tip without the bead branch
	// itself BEING decoy-src-forge.
	beadWtDir := dir + "/.wt-bead-forge1"
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-forge.1")
	if err := os.WriteFile(beadWtDir+"/own.txt", []byte("bead's own work\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead's own work")
	runGitIn(t, beadWtDir, "merge", "--no-ff", "-m", "bead incorporates decoy-src-forge for unrelated reasons", "decoy-src-forge")
	beadTip := refHash(t, dir, "bead/mindspec-forge.1")
	if isAnc, ancErr := gitutil.IsAncestor(dir, decoyTip, beadTip); ancErr != nil || !isAnc {
		t.Fatalf("fixture invariant broken: decoyTip must be an ancestor of the bead branch's current tip, isAnc=%v err=%v", isAnc, ancErr)
	}

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-forge.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-forge.1",
	}}

	// Plant the FOREIGN preserved merge directly in the SPEC worktree:
	// decoy-src-forge conflicts with the spec's own edit — never a merge
	// of the bead branch, and never through gitutil.MergeInto, so NO
	// merge-start marker for "bead/mindspec-forge.1" is ever recorded in
	// this worktree.
	if err := os.WriteFile(specWtPath+"/shared.txt", []byte("spec version\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "spec-side edit to the same path decoy-src-forge touches")
	_, _ = exec.Command("git", "-C", specWtPath, "merge", "--no-ff", "-m", "Merge decoy-src-forge", "decoy-src-forge").CombinedOutput()
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: the decoy merge must be mid-conflict in the spec worktree")
	}
	if got := mergeHeadSHA(t, specWtPath); got != decoyTip {
		t.Fatalf("fixture invariant broken: MERGE_HEAD must be decoyTip; got %s want %s", got, decoyTip)
	}

	// THE FORGERY: hand-edit MERGE_MSG's subject to falsely claim this is
	// the bead branch's own merge — exactly what mergeSubjectNamesSource
	// alone (pre-round-3) would have trusted as sufficient.
	rewriteMergeMsgSubject(t, specWtPath, "Merge bead/mindspec-forge.1")
	if named, err := mergeSubjectNamesSource(specWtPath, "bead/mindspec-forge.1"); err != nil || !named {
		t.Fatalf("fixture invariant broken: the hand-edited subject must (falsely) name the bead branch; named=%v err=%v", named, err)
	}

	specTipBefore := refHash(t, dir, "spec/077-forge")

	// STILL-CONFLICTED leg: the forged subject alone must not flip this
	// to bindingDrifted.
	err := g.CompleteBead("mindspec-forge.1", "spec/077-forge", "", "", false)
	if err == nil {
		t.Fatal("a hand-edited subject must not let a foreign ancestor merge be trusted as this bead's own drifted conflict")
	}
	if got := mergeHeadSHA(t, specWtPath); got != decoyTip {
		t.Fatalf("the foreign merge must be untouched; MERGE_HEAD was %s, now %s", decoyTip, got)
	}

	// READY-TO-COMPLETE leg: resolve + stage the FOREIGN conflict (as an
	// operator innocently would), then invoke --resolve-merge. Before
	// round 3, the hand-edited subject alone would have classified this
	// bindingDrifted and completeResumedMerge would have COMMITTED
	// decoy-src-forge's content under the forged subject, reporting
	// success. The marker — producer-written evidence, not unforgeable —
	// must refuse to corroborate it.
	if err := os.WriteFile(specWtPath+"/shared.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", "shared.txt")
	err = g.CompleteBead("mindspec-forge.1", "spec/077-forge", "", "", true)
	if err == nil {
		t.Fatal("a hand-edited MERGE_MSG subject must never be sufficient, alone, to complete a foreign merge as this bead's own landing")
	}
	if !strings.Contains(err.Error(), "could not confirm") && !strings.Contains(err.Error(), "could not verify") {
		t.Errorf("the refusal must name the identity-verification failure (not silently re-trust the forged subject); got:\n%s", err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the foreign merge must remain preserved (uncommitted) — the forged subject must never let this be completed")
	}
	if got := refHash(t, dir, "spec/077-forge"); got != specTipBefore {
		t.Errorf("spec branch tip must not advance — nothing may be committed over a merge this tool cannot verify; was %s, now %s", specTipBefore, got)
	}
}
