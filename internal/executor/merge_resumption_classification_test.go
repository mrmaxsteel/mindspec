package executor

// Spec 127 bead-6 fix round 4, item 3: two fixtures round 3 left as
// "residual coverage" without a dedicated test.
//
//  1. classifyPreservedMergeBinding's OWN classification failure — an
//     ancestry/RevParseRef error, distinct from a MISSING marker
//     (bindingIndeterminateRefusal is already exercised incidentally via
//     the marker-absence path in merge_resumption_marker_test.go, but the
//     classification call itself failing had no dedicated fixture).
//  2. A merge-source marker that is PRESENT but MISMATCHED — the
//     discriminator's central case: absent means "cannot tell"
//     (bindingIndeterminateRefusal); mismatched means the recorded
//     evidence POSITIVELY CONTRADICTS the claim (the tool recorded
//     starting a merge from tip X and is now looking at a preserved merge
//     at tip Y) — a confident bindingForeign refusal, never sharing
//     bindingIndeterminateRefusal's "cannot verify" code path.

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// TestClassifyPreservedMergeBinding_AncestryCheckFailureIsIndeterminate is
// item 3's first fixture: classifyPreservedMergeBinding's OWN ancestry
// check failing (a genuine git error, not "not an ancestor" and not "ref
// not found") must surface as the fail-closed bindingIndeterminateRefusal
// leg — never silently treated as bindingExact/bindingDrifted/bindingForeign,
// and distinguishable from the marker-ABSENT indeterminate path (this
// fixture never even reaches the marker check at all: the ancestry check
// fails first).
func TestClassifyPreservedMergeBinding_AncestryCheckFailureIsIndeterminate(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}
	fake.onRemove = func(name string) {
		if name == "worktree-mindspec-x.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
		}
	}

	// Plain invocation: conflicts, preserved. MERGE_HEAD != expectedSource's
	// tip is not even required here — the injected failure fires before
	// classifyPreservedMergeBinding ever reaches the equality/ancestor
	// comparison result, so the ordinary single-commit conflict fixture is
	// enough.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected the initial conflict")
	}

	// DRIFT: advance the bead branch so preservedTip != expectedSource's
	// current tip — otherwise classifyPreservedMergeBinding short-circuits
	// on the bindingExact equality check before ever calling
	// classifyIsAncestorFn at all.
	if err := os.WriteFile(beadWtDir+"/drift.txt", []byte("drift\n"), 0o644); err != nil {
		t.Fatalf("write drift: %v", err)
	}
	runGitIn(t, beadWtDir, "add", "drift.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "drift")

	origAncestor := classifyIsAncestorFn
	t.Cleanup(func() { classifyIsAncestorFn = origAncestor })
	simulated := errors.New("simulated ancestry-check infra failure")
	classifyIsAncestorFn = func(string, string, string) (bool, error) {
		return false, simulated
	}

	specTipBefore := refHash(t, dir, "spec/077-test")
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("an ancestry-check failure must refuse, never silently classify")
	}
	if !strings.Contains(err.Error(), "could not verify") {
		t.Errorf("expected the fail-closed bindingIndeterminateRefusal phrasing (\"could not verify\"), got:\n%s", err.Error())
	}
	if strings.Contains(err.Error(), "does not correspond to the requested source") {
		t.Errorf("an ancestry-check INFRA failure must never be phrased as the confident bindingForeign refusal, got:\n%s", err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the preserved conflict must remain untouched")
	}
	if got := refHash(t, dir, "spec/077-test"); got != specTipBefore {
		t.Errorf("nothing may be committed on an indeterminate classification; spec tip was %s, now %s", specTipBefore, got)
	}
}

// TestClassifyPreservedMergeBinding_MismatchedMarkerIsConfidentForeignRefusal
// is item 3's second, more central fixture: the merge-source marker EXISTS
// (this tool DID start a real merge attempt of this bead branch in this
// worktree at some point) but its recorded tip does not match the
// CURRENTLY preserved MERGE_HEAD — the discriminator's central case. This
// is constructed by having the tool's own first attempt record the marker
// at tip T1, then an operator action OUTSIDE the tool (a manual abort +
// manual `git merge`, never gitutil.MergeInto/MergeBranch) leaves a
// DIFFERENT tip T2 preserved without ever touching the marker — the
// recorded evidence (T1) now positively CONTRADICTS what is actually
// preserved (T2), rather than merely being silent about it. Must assert
// bindingForeign's own confident refusal (foreignMergeRefusal's "does not
// correspond to the requested source" phrasing) — never
// bindingIndeterminateRefusal's "could not verify"/"could not confirm"
// phrasing, which is reserved for when the marker cannot be read at all.
func TestClassifyPreservedMergeBinding_MismatchedMarkerIsConfidentForeignRefusal(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}
	fake.onRemove = func(name string) {
		if name == "worktree-mindspec-x.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
		}
	}

	// 1. The TOOL's own first attempt: conflicts on c.txt, and — as a
	// side effect of gitutil.MergeInto itself — records the merge-source
	// marker for bead/mindspec-x.1 at T1 (the bead branch's tip right now).
	t1 := refHash(t, dir, "bead/mindspec-x.1")
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected the initial conflict")
	}
	if got := mergeHeadSHA(t, specWtPath); got != t1 {
		t.Fatalf("fixture invariant broken: MERGE_HEAD must be T1 (%s); got %s", t1, got)
	}
	if matched, err := mergeSourceMarkerMatches(specWtPath, "bead/mindspec-x.1", t1); err != nil || !matched {
		t.Fatalf("fixture invariant broken: the marker must be recorded at T1 by gitutil.MergeInto itself; matched=%v err=%v", matched, err)
	}

	// 2. Operator action OUTSIDE the tool: abort the preserved conflict by
	// hand.
	if err := gitutil.AbortMerge(specWtPath); err != nil {
		t.Fatalf("aborting the preserved conflict: %v", err)
	}

	// 3. The bead branch advances to T2 (still touching c.txt, so a
	// manual re-merge below conflicts the same way).
	if err := os.WriteFile(beadWtDir+"/c.txt", []byte("bead side v2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", "c.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "bead advances to T2")
	t2 := refHash(t, dir, "bead/mindspec-x.1")

	// 4. The operator, BYPASSING the tool entirely, hand-merges T2 —
	// never through gitutil.MergeInto/MergeBranch, so the marker (still
	// recording T1) is NEVER updated. This conflicts again on c.txt.
	_, _ = exec.Command("git", "-C", specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-x.1", "bead/mindspec-x.1").CombinedOutput()
	if got := mergeHeadSHA(t, specWtPath); got != t2 {
		t.Fatalf("fixture invariant broken: MERGE_HEAD must now be T2 (%s); got %s", t2, got)
	}
	if matched, err := mergeSourceMarkerMatches(specWtPath, "bead/mindspec-x.1", t2); err != nil {
		t.Fatalf("fixture invariant broken: reading the marker must not error (it exists, just stale): %v", err)
	} else if matched {
		t.Fatal("fixture invariant broken: the marker must NOT match T2 — it still records the tool's ORIGINAL attempt at T1")
	}

	// 5. The bead branch advances ONCE MORE, to T3 — so preservedTip (T2)
	// is a strict ANCESTOR of expectedSource's CURRENT tip (T3), never
	// equal to it (an equal preservedTip==sourceTip would short-circuit to
	// bindingExact before the marker is ever consulted at all).
	if err := os.WriteFile(beadWtDir+"/unrelated.txt", []byte("more work\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", "unrelated.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "bead advances to T3")
	t3 := refHash(t, dir, "bead/mindspec-x.1")
	if isAnc, ancErr := gitutil.IsAncestor(dir, t2, t3); ancErr != nil || !isAnc {
		t.Fatalf("fixture invariant broken: T2 must be an ancestor of T3, isAnc=%v err=%v", isAnc, ancErr)
	}

	// Resolve the (manually-produced) conflict and stage it, then invoke
	// --resolve-merge.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	specTipBefore := refHash(t, dir, "spec/077-test")
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("a stale, mismatched marker must never let this be completed as bead/mindspec-x.1's own drifted landing")
	}
	if !strings.Contains(err.Error(), "does not correspond to the requested source") {
		t.Errorf("expected the CONFIDENT bindingForeign refusal (foreignMergeRefusal's phrasing), got:\n%s", err.Error())
	}
	if strings.Contains(err.Error(), "could not verify") || strings.Contains(err.Error(), "could not confirm") {
		t.Errorf("a MISMATCHED marker must never share bindingIndeterminateRefusal's \"cannot tell\" phrasing — the evidence here positively CONTRADICTS the claim; got:\n%s", err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the preserved (foreign) merge must remain untouched")
	}
	if got := refHash(t, dir, "spec/077-test"); got != specTipBefore {
		t.Errorf("nothing may be committed over a merge this tool must refuse; spec tip was %s, now %s", specTipBefore, got)
	}
}
