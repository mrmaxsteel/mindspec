package executor

// Spec 127 bead-6 fix round 3: BLOCKING 1's acceptance tests
// (S1-1/G1-3/O1-1/F1-1's panel review). completeDriftedResumedMerge's
// ResetSoft call can fail AFTER its CommitTreeMerge already succeeded,
// leaving the branch at the un-collapsed two-merge topology with
// MERGE_HEAD already cleared — the exact ambiguous shape spec 125's
// FindLandedMerge refuses. Before this fix, a BARE re-invocation would
// silently no-op ("already up to date") over that stranded shape and
// report apparent success. These tests fault-inject the ResetSoft
// failure via resetSoftFn (merge_resumption.go's fault-injection seam)
// and prove: (1) the stranding itself surfaces as a loud error, never
// silent success; (2) a re-invocation repairs the collapse and converges
// once the underlying failure clears; (3) a repair attempt that ITSELF
// fails is also loud and distinguishable, never silently retried as a
// fresh merge.

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// driftedResolveMergeFixture drives CompleteBead through a conflict,
// drift, and resolved-conflict setup identical to
// TestCompleteBead_ResolveMerge_SourceDriftIsIncorporated, stopping just
// before the --resolve-merge invocation that will trigger
// completeDriftedResumedMerge's collapse — shared by both tests below.
func driftedResolveMergeFixture(t *testing.T) (g *MindspecExecutor, dir, specWtPath, beadWtDir, driftedBeadTip string) {
	t.Helper()
	var fake *fakeWorktreeOps
	g, fake, dir = newRepoExecutor(t)
	specWtPath, beadWtDir = setupConflictingSpecAndBead(t, dir)

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

	// 1. Plain invocation: conflicts, preserved (gitutil.MergeInto writes
	// the merge-start marker for bead/mindspec-x.1 here).
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}

	// 2. DRIFT: a new commit lands on the bead branch after the conflict
	// began, on a path the resolved conflict never touches (a clean
	// catch-up, not a second conflict).
	if err := os.WriteFile(beadWtDir+"/drift.txt", []byte("drifted work\n"), 0o644); err != nil {
		t.Fatalf("write drift file: %v", err)
	}
	runGitIn(t, beadWtDir, "add", "drift.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "drift: bead branch advances after the conflict began")
	driftedBeadTip = refHash(t, dir, "bead/mindspec-x.1")

	// 3. Resolve + stage the ORIGINAL conflict.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	return g, dir, specWtPath, beadWtDir, driftedBeadTip
}

// TestCompleteDriftedResumedMerge_ResetSoftFailureStrandsThenRepairs is
// BLOCKING 1's central acceptance test: a ResetSoft failure immediately
// after a successful CommitTreeMerge must surface loudly on the
// invocation that hits it, and a LATER bare re-invocation must repair the
// stranded topology and converge — never silently no-op over it.
func TestCompleteDriftedResumedMerge_ResetSoftFailureStrandsThenRepairs(t *testing.T) {
	g, dir, specWtPath, _, driftedBeadTip := driftedResolveMergeFixture(t)

	// Fault-inject: fail exactly the FIRST ResetSoft call (S1-1's real-git
	// repro) — every call after that is the real gitutil.ResetSoft, so
	// the LATER repair invocation can actually converge.
	origResetSoft := resetSoftFn
	t.Cleanup(func() { resetSoftFn = origResetSoft })
	failNext := true
	resetSoftFn = func(workdir, target string) error {
		if failNext {
			failNext = false
			return errors.New("simulated ref-lock contention")
		}
		return origResetSoft(workdir, target)
	}

	// 4. --resolve-merge: C1 (the stale resumed merge) and C2 (the clean
	// catch-up) both land for real, CommitTreeMerge succeeds, but the
	// injected ResetSoft failure leaves the branch STRANDED at C2 —
	// MERGE_HEAD already cleared, the collapsed commit dangling.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("the injected ResetSoft failure must surface as an error, never silent success")
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: MERGE_HEAD must already be cleared by the catch-up merge commit")
	}
	strandedTip := refHash(t, dir, "spec/077-test")
	merges, err := gitutil.FirstParentMerges(dir, "spec/077-test")
	if err != nil {
		t.Fatalf("FirstParentMerges: %v", err)
	}
	if len(merges) < 2 || merges[0].SHA != strandedTip || len(merges[0].Parents) != 2 || merges[0].Parents[1] != driftedBeadTip {
		t.Fatalf("fixture invariant broken: expected the stranded topology (C2 at the tip, second parent = drifted tip); got %+v", merges)
	}
	if merges[1].SHA != merges[0].Parents[0] || len(merges[1].Parents) != 2 {
		t.Fatalf("fixture invariant broken: expected C1 to be C2's immediate first parent, both two-parent merges; got %+v", merges)
	}
	if merges[0].Subject != merges[1].Subject {
		t.Fatalf("fixture invariant broken: C1 and C2 must share the identical seeded subject (the exact ambiguity spec 125's FindLandedMerge refuses); got %q vs %q", merges[0].Subject, merges[1].Subject)
	}

	// 5. Bare re-invocation (still --resolve-merge, matching how an
	// operator would retry): resumeAwareMerge's resumeNoMergeInProgress
	// leg now repairs the stranded topology FIRST, before ever reaching
	// attemptFreshMerge (whose mergeFn() would otherwise silently no-op
	// "already up to date" over it). ResetSoft succeeds this time
	// (failNext already consumed).
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("the re-invocation must repair the stranded collapse and converge, got: %v", err)
	}

	// The topology is now collapsed: the tip is exactly ONE merge commit
	// (not the two-merge shape above), its second parent the drifted
	// tip, its subject the original seeded form.
	collapsed, err := gitutil.FirstParentMerges(dir, "spec/077-test")
	if err != nil {
		t.Fatalf("FirstParentMerges (post-repair): %v", err)
	}
	if len(collapsed) < 1 || len(collapsed[0].Parents) != 2 || collapsed[0].Parents[1] != driftedBeadTip {
		t.Fatalf("post-repair: expected a single collapsed merge whose second parent is the drifted tip; got %+v", collapsed)
	}
	if collapsed[0].Parents[0] != merges[1].Parents[0] {
		t.Errorf("post-repair: the collapsed merge's first parent must be the ORIGINAL pre-resumption tip (never C1); got %s want %s", collapsed[0].Parents[0], merges[1].Parents[0])
	}
	wantSubject := "Merge bead/mindspec-x.1"
	if collapsed[0].Subject != wantSubject {
		t.Errorf("post-repair subject = %q, want %q", collapsed[0].Subject, wantSubject)
	}
	if got, readErr := os.ReadFile(specWtPath + "/c.txt"); readErr != nil || string(got) != "resolved\n" {
		t.Errorf("the resolved content must survive the repair; got %q, err=%v", got, readErr)
	}
	if got, readErr := os.ReadFile(specWtPath + "/drift.txt"); readErr != nil || string(got) != "drifted work\n" {
		t.Errorf("the drifted content must survive the repair; got %q, err=%v", got, readErr)
	}
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must be deleted once the repaired merge fully converges")
	}
}

// TestRepairStrandedDriftCollapse_RepairFailureIsLoudNotSilent is
// BLOCKING 1's second acceptance leg: when the repair's OWN ResetSoft
// ALSO fails, that must be a loud, distinguishable failure
// (*strandedCollapseError) — never silently retried as a fresh merge,
// and the branch must be left exactly where it was (still stranded, not
// further corrupted), so a LATER retry (once the underlying failure
// clears) can still converge.
func TestRepairStrandedDriftCollapse_RepairFailureIsLoudNotSilent(t *testing.T) {
	g, dir, _, _, _ := driftedResolveMergeFixture(t)

	origResetSoft := resetSoftFn
	t.Cleanup(func() { resetSoftFn = origResetSoft })
	resetSoftFn = func(workdir, target string) error {
		return errors.New("simulated persistent ref-lock contention")
	}

	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("expected the first ResetSoft failure to surface")
	}
	strandedTip := refHash(t, dir, "spec/077-test")

	// Bare re-invocation: the repair ALSO fails (ResetSoft is still
	// stubbed to always fail) — must be loud and distinguishable, never
	// a silent no-op success laundering the stranded topology.
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("a repair attempt that itself fails must never report success")
	}
	var sce *strandedCollapseError
	if !errors.As(err, &sce) {
		t.Errorf("expected a *strandedCollapseError naming the stranded topology, got %T: %v", err, err)
	}
	if got := refHash(t, dir, "spec/077-test"); got != strandedTip {
		t.Errorf("a failed repair attempt must not move the branch; was %s, now %s", strandedTip, got)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must survive a repair failure — no cleanup runs on a refusal")
	}
}

// TestDetectStrandedDriftTopology_IndeterminateWhenBuriedByLaterCommit is
// bead-6 fix round 5's acceptance test for S1-3/O1-3/G1-3/F1-1's shared
// confirm-round finding: a single, ordinary commit landing on the branch
// between an interrupted ResetSoft and a later retry buries the stranded
// C1/C2 pair one commit below the new tip. Before this fix, that silently
// defeated detection (the pre-round-5 code read literally "HEAD"'s tree,
// which had moved past the stranded pair, so the dangling-object proof
// never matched) and a bare retry reported apparent success
// ("already up to date") over the still-uncollapsed, ambiguous topology —
// exactly the class of bug S1-1's original BLOCKING finding was about.
// This test proves the retry now refuses loudly instead, and — just as
// importantly — that it does NOT blindly `git reset --soft` over the
// intervening commit (which would silently discard it).
func TestDetectStrandedDriftTopology_IndeterminateWhenBuriedByLaterCommit(t *testing.T) {
	g, dir, specWtPath, _, driftedBeadTip := driftedResolveMergeFixture(t)
	_ = driftedBeadTip

	origResetSoft := resetSoftFn
	t.Cleanup(func() { resetSoftFn = origResetSoft })
	failNext := true
	resetSoftFn = func(workdir, target string) error {
		if failNext {
			failNext = false
			return errors.New("simulated ref-lock contention")
		}
		return origResetSoft(workdir, target)
	}

	// 4. --resolve-merge: strands C1+C2 exactly as
	// TestCompleteDriftedResumedMerge_ResetSoftFailureStrandsThenRepairs
	// does — the dangling proof object for this exact tree/parents/message
	// now sits in the object store, unreferenced.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("expected the injected ResetSoft failure to surface")
	}
	strandedTip := refHash(t, dir, "spec/077-test")

	// 5. BURY it: an ordinary, unrelated commit lands on the spec branch —
	// standing in for another bead's own merge, or any routine auto-commit
	// — before anyone retries this bead's own completion.
	if err := os.WriteFile(specWtPath+"/unrelated.txt", []byte("unrelated later work\n"), 0o644); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}
	runGitIn(t, specWtPath, "add", "unrelated.txt")
	runGitIn(t, specWtPath, "commit", "-m", "an unrelated commit lands on top of the stranded pair")
	buriedHead := refHash(t, dir, "spec/077-test")
	if buriedHead == strandedTip {
		t.Fatal("fixture invariant broken: the unrelated commit must move the tip past the stranded pair")
	}

	// 6. Bare retry: must refuse loudly — never silently collapse (which
	// would discard the unrelated commit) and never silently report
	// success (which would launder the still-uncollapsed topology).
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("a retry over a stranded pair buried under a later commit must never report success")
	}
	var indeterminate *strandedTopologyIndeterminateError
	if !errors.As(err, &indeterminate) {
		t.Errorf("expected a *strandedTopologyIndeterminateError, got %T: %v", err, err)
	}

	// Nothing was touched: the unrelated commit survives, the stranded
	// pair beneath it is untouched, and the bead branch (not yet
	// cleaned up) still exists.
	if got := refHash(t, dir, "spec/077-test"); got != buriedHead {
		t.Errorf("a refused, indeterminate retry must not move the branch; was %s, now %s", buriedHead, got)
	}
	if got, readErr := os.ReadFile(specWtPath + "/unrelated.txt"); readErr != nil || string(got) != "unrelated later work\n" {
		t.Errorf("the unrelated commit's content must survive untouched; got %q, err=%v", got, readErr)
	}
	merges, ferr := gitutil.FirstParentMerges(dir, "spec/077-test")
	if ferr != nil {
		t.Fatalf("FirstParentMerges: %v", ferr)
	}
	if len(merges) < 2 || merges[0].SHA != strandedTip || len(merges[0].Parents) != 2 {
		t.Fatalf("the stranded C1/C2 pair beneath the unrelated commit must remain exactly as it was, unrewritten; got %+v", merges)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must survive a refused, indeterminate retry — no cleanup runs on a refusal")
	}
}

// TestDetectStrandedDriftTopology_IndeterminateWhenBuriedByLaterMerge is
// bead-6 fix round 6's acceptance test for O1-4/S1-4/S3-4/F1-2/G1-1's
// shared confirm-round finding: round 5's own STATED RESIDUAL 2(b) claimed
// closure for "another bead's own merge... landing on the branch between
// the interruption and a retry", but detectStrandedDriftTopology only ever
// inspected gitutil.FirstParentMerges' two nearest-HEAD entries — a
// genuinely intervening MERGE commit (as opposed to the sibling test's
// ORDINARY commit, which FirstParentMerges never returns at all) became
// the new nearest entry, its subject differed from the stranded pair's own
// subject, and the fixed merges[0]/merges[1] destructuring rejected the
// whole topology on that mismatch before ever reaching the genuinely
// stranded C1/C2 pair one slot further down — a bare retry silently
// reported success ("already up to date") over the still-uncollapsed,
// ambiguous topology. This test proves the retry now refuses loudly
// instead, exactly as the ordinary-commit sibling test does, and that the
// intervening merge's own content survives untouched.
func TestDetectStrandedDriftTopology_IndeterminateWhenBuriedByLaterMerge(t *testing.T) {
	g, dir, specWtPath, _, driftedBeadTip := driftedResolveMergeFixture(t)
	_ = driftedBeadTip

	origResetSoft := resetSoftFn
	t.Cleanup(func() { resetSoftFn = origResetSoft })
	failNext := true
	resetSoftFn = func(workdir, target string) error {
		if failNext {
			failNext = false
			return errors.New("simulated ref-lock contention")
		}
		return origResetSoft(workdir, target)
	}

	// 4. --resolve-merge: strands C1+C2 exactly as the sibling tests above.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("expected the injected ResetSoft failure to surface")
	}
	strandedTip := refHash(t, dir, "spec/077-test")

	// 5. BURY it under a GENUINE MERGE, not an ordinary commit — standing
	// in for another bead's own merge landing on the same spec branch
	// before anyone retries this bead's own completion (a routine event
	// in mindspec's own multi-bead-per-spec-branch workflow).
	runGitIn(t, specWtPath, "checkout", "-b", "other-topic")
	if err := os.WriteFile(specWtPath+"/other.txt", []byte("another bead's own work\n"), 0o644); err != nil {
		t.Fatalf("write other-topic file: %v", err)
	}
	runGitIn(t, specWtPath, "add", "other.txt")
	runGitIn(t, specWtPath, "commit", "-m", "other-topic: unrelated work")
	runGitIn(t, specWtPath, "checkout", "spec/077-test")
	runGitIn(t, specWtPath, "merge", "--no-ff", "-m", "Merge bead/mindspec-other.1 into spec/077-test", "other-topic")
	buriedHead := refHash(t, dir, "spec/077-test")
	if buriedHead == strandedTip {
		t.Fatal("fixture invariant broken: the intervening merge must move the tip past the stranded pair")
	}

	// 6. Bare retry: must refuse loudly — never silently collapse (which
	// would discard the intervening merge) and never silently report
	// success (which would launder the still-uncollapsed topology, the
	// exact overclaim round 5's own STATED RESIDUAL 2(b) made for this
	// shape).
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("a retry over a stranded pair buried under a later merge must never report success")
	}
	var indeterminate *strandedTopologyIndeterminateError
	if !errors.As(err, &indeterminate) {
		t.Errorf("expected a *strandedTopologyIndeterminateError, got %T: %v", err, err)
	}

	// Nothing was touched: the intervening merge survives, the stranded
	// pair beneath it is untouched, and the bead branch (not yet cleaned
	// up) still exists.
	if got := refHash(t, dir, "spec/077-test"); got != buriedHead {
		t.Errorf("a refused, indeterminate retry must not move the branch; was %s, now %s", buriedHead, got)
	}
	if got, readErr := os.ReadFile(specWtPath + "/other.txt"); readErr != nil || string(got) != "another bead's own work\n" {
		t.Errorf("the intervening merge's content must survive untouched; got %q, err=%v", got, readErr)
	}
	merges, ferr := gitutil.FirstParentMerges(dir, "spec/077-test")
	if ferr != nil {
		t.Fatalf("FirstParentMerges: %v", ferr)
	}
	if len(merges) < 3 {
		t.Fatalf("expected the intervening merge PLUS the stranded C1/C2 pair (3 entries); got %+v", merges)
	}
	if merges[0].SHA != buriedHead {
		t.Fatalf("expected the intervening merge to be the newest entry; got %+v", merges[0])
	}
	if merges[1].SHA != strandedTip || len(merges[1].Parents) != 2 {
		t.Fatalf("the stranded pair's own top (C2) must remain exactly as it was, one slot below the intervening merge; got %+v", merges[1])
	}
	if merges[2].SHA != merges[1].Parents[0] || len(merges[2].Parents) != 2 {
		t.Fatalf("C1 must remain C2's immediate first parent, unrewritten; got %+v", merges[2])
	}
	if merges[1].Subject != merges[2].Subject {
		t.Fatalf("the stranded pair must still share its own identical subject, distinct from the intervening merge's; got %q vs %q (intervening: %q)", merges[1].Subject, merges[2].Subject, merges[0].Subject)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must survive a refused, indeterminate retry — no cleanup runs on a refusal")
	}

	// The downstream backstop: spec 125's own FindLandedMerge must still
	// fail closed against this exact ambiguous shape, independent of
	// whatever detectStrandedDriftTopology itself concludes.
	if _, err := lifecycle.FindLandedMerge(dir, "spec/077-test", "mindspec-x.1"); err == nil {
		t.Fatal("FindLandedMerge must still refuse the ambiguous two-merge topology buried under an intervening merge")
	}
}

// TestRepairStrandedDriftCollapse_AcceptedResidualWhenProofExternallyPruned
// is bead-6 fix round 5's acceptance test for STATED RESIDUAL 2(c) —
// completeDriftedResumedMerge's own doc comment — the ONE sub-case this
// round disclosed rather than closed: an external `git gc --prune=now`
// removing the dangling proof object between the interruption and a
// retry, with NOTHING else having landed on top of the stranded pair.
// This is deliberately NOT distinguishable from STATED RESIDUAL 1's own
// legitimately-produced multi-invocation chain by any signal available
// here (see MergeSourceMarkerRef's doc comment for why bead 6 stopped
// chasing an "unforgeable" replacement for the identical reason) — so
// this test pins the ACCEPTED behavior (a no-op fresh merge reports
// success, exactly as it does for the legitimate chain), proves the
// topology is left genuinely uncollapsed (never silently rewritten on
// shape alone), and proves the downstream backstop actually holds:
// lifecycle.FindLandedMerge, asked to identify this bead's own landed
// merge afterward, still fails closed and refuses to pick a side — no
// caller anywhere is ever told a false landed-merge identity.
//
// SPEC 127 FINAL CONFIRM ROUND (G1-2). The finding held against this test
// was that the product "reports completion without the uniquely
// attestable landed identity". The two legs below are what that reading
// was missing, and they are now DERIVED here rather than left to be
// re-argued from the prose:
//
//	(a) completion is NOT identity-less. The retry's ensureLandedBinding
//	    records a durable landed-merge binding, and the identity it
//	    records is UNIQUE by construction rather than chosen: the bead's
//	    own tip is the exact second parent of exactly ONE merge on the
//	    spec branch (C2), so the write side has no candidate set to pick
//	    from. C1, the stranded pair's lower half, merged the PRE-drift
//	    tip and is therefore not a candidate at all.
//	(b) the read side's refusal is a DIFFERENT question, and this test
//	    now pins which one. FindLandedMerge nominates candidates by merge
//	    SUBJECT, so it sees both C1 and C2 under this bead's name, finds
//	    they disagree on the second parent, and returns spec 125 FIX-2b's
//	    ambiguity refusal — BEFORE it ever consults the binding written
//	    at (a). The refusal is "two same-subject merges disagree", never
//	    "nothing identified this bead's landing".
//
// So no caller is told a false identity (the original claim, unchanged),
// AND a true one is durably recorded. What remains — the READ side
// short-circuiting on subject-nominated ambiguity ahead of a write-time
// binding that would resolve it — is a read-path design question for
// internal/lifecycle, not a completion-path defect, and is left to a
// tracked follow-up rather than changed inside a confirm round.
func TestRepairStrandedDriftCollapse_AcceptedResidualWhenProofExternallyPruned(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	g, dir, specWtPath, _, driftedBeadTip := driftedResolveMergeFixture(t)

	origResetSoft := resetSoftFn
	t.Cleanup(func() { resetSoftFn = origResetSoft })
	failNext := true
	resetSoftFn = func(workdir, target string) error {
		if failNext {
			failNext = false
			return errors.New("simulated ref-lock contention")
		}
		return origResetSoft(workdir, target)
	}

	// 4. --resolve-merge: strand C1+C2, exactly as the sibling tests above.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("expected the injected ResetSoft failure to surface")
	}
	strandedTip := refHash(t, dir, "spec/077-test")

	// 5. PRUNE: an external `git gc --prune=now` (nothing mindspec itself
	// ever invokes) genuinely deletes the now-unreachable dangling proof
	// object, in the window before anyone retries.
	runGitIn(t, specWtPath, "reflog", "expire", "--expire=now", "--all")
	runGitIn(t, specWtPath, "gc", "--prune=now", "-q")

	// 6. Bare retry: ACCEPTED to report success here (indistinguishable
	// from the legitimate multi-invocation chain), but must NOT silently
	// rewrite the topology — the two-merge C1/C2 shape must survive
	// exactly as it was. The binding writes are captured (leg (a) of the
	// G1-2 note above): reporting success is only acceptable if the
	// landed identity is durably recorded on the way past.
	var bindings []map[string]interface{}
	mergeBindingFn = func(_ string, meta map[string]interface{}) error {
		bindings = append(bindings, meta)
		return nil
	}
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("the accepted residual reports success (a no-op fresh merge), got: %v", err)
	}
	finalTip := refHash(t, dir, "spec/077-test")
	if finalTip != strandedTip {
		t.Fatalf("the topology must NOT be rewritten on shape alone once the proof is gone: tip moved from %s to %s", strandedTip, finalTip)
	}
	merges, ferr := gitutil.FirstParentMerges(dir, "spec/077-test")
	if ferr != nil {
		t.Fatalf("FirstParentMerges: %v", ferr)
	}
	if len(merges) < 2 || merges[0].SHA != strandedTip || len(merges[0].Parents) != 2 || merges[0].Parents[1] != driftedBeadTip {
		t.Fatalf("the un-collapsed C1/C2 pair must survive exactly as it was; got %+v", merges)
	}

	// (a) The landed identity IS recorded, and it is UNIQUE rather than
	// chosen: exactly one merge on the spec branch carries the bead's own
	// tip as its exact second parent, and that is the merge the binding
	// names. Derived from the repo, never from the SHAs this test already
	// happens to hold.
	owned, oerr := gitutil.ExactSecondParentMerges(dir, "spec/077-test", driftedBeadTip)
	if oerr != nil {
		t.Fatalf("ExactSecondParentMerges: %v", oerr)
	}
	if len(owned) != 1 {
		t.Fatalf("the write side must have exactly ONE candidate — the bead tip is the exact second "+
			"parent of one merge, so there is nothing to pick between; got %d: %+v", len(owned), owned)
	}
	if len(bindings) != 1 {
		t.Fatalf("completion must record the landed-merge binding before cleanup, got %d writes: %+v", len(bindings), bindings)
	}
	gotSHA, _ := bindings[0]["mindspec_landed_merge_sha"].(string)
	gotParent, _ := bindings[0]["mindspec_landed_second_parent"].(string)
	if gotSHA != owned[0].SHA || gotParent != driftedBeadTip {
		t.Errorf("the recorded binding must name the one merge the bead's tip identifies; got sha=%s "+
			"secondParent=%s, want sha=%s secondParent=%s", gotSHA, gotParent, owned[0].SHA, driftedBeadTip)
	}

	// (b) The downstream backstop: spec 125's own FindLandedMerge must
	// still fail closed against this exact ambiguous shape — never attest
	// a false landed-merge identity just because this residual reported
	// "success". And it must refuse for the REASON the residual's safety
	// argument depends on: two same-subject owned merges disagreeing on
	// the landed tip (FIX-2b), which the read side reaches BEFORE it
	// consults the binding recorded at (a) — not "nothing identified this
	// bead at all", which would be a different, weaker statement.
	_, findErr := lifecycle.FindLandedMerge(dir, "spec/077-test", "mindspec-x.1")
	if findErr == nil {
		t.Fatal("FindLandedMerge must still refuse the ambiguous two-merge topology — the accepted residual's own safety depends on this backstop holding")
	}
	var noEvidence *lifecycle.LandedMergeNoEvidence
	if !errors.As(findErr, &noEvidence) || noEvidence.ConflictingSecondParent == "" {
		t.Fatalf("the refusal must be the conflicting-second-parent ambiguity, got %T: %v", findErr, findErr)
	}
	if errors.Is(findErr, lifecycle.ErrLandedMergeNoCandidate) {
		t.Error("the refusal must never be the zero-candidate sentinel: a merge naming this bead " +
			"plainly exists, and that sentinel is the one a caller may read as a licence to delete")
	}
	if noEvidence.SecondParent != driftedBeadTip {
		t.Errorf("the refusal must name the drift-side landing as its candidate; got %s, want %s",
			noEvidence.SecondParent, driftedBeadTip)
	}
}

// TestDanglingCollapsedMergeExists_MessageMismatchIsNotProof is bead-6 fix
// round 5's acceptance test for G1-3's confirm-round finding (the
// false-positive half): a SEPARATE CommitTreeMerge call sharing the exact
// tree and ordered parents of a would-be-collapsed pair, but carrying an
// UNRELATED message, must never be accepted as proof of that pair's own
// genuine interruption.
func TestDanglingCollapsedMergeExists_MessageMismatchIsNotProof(t *testing.T) {
	dir := t.TempDir()
	runGitIn(t, dir, "init", "-q")
	runGitIn(t, dir, "commit", "--allow-empty", "-m", "root")
	parent1 := refHash(t, dir, "HEAD")
	runGitIn(t, dir, "checkout", "-b", "side")
	if err := os.WriteFile(dir+"/side.txt", []byte("side\n"), 0o644); err != nil {
		t.Fatalf("write side file: %v", err)
	}
	runGitIn(t, dir, "add", "side.txt")
	runGitIn(t, dir, "commit", "-m", "side work")
	parent2 := refHash(t, dir, "HEAD")
	tree, err := gitutil.TreeSHA(dir, "HEAD")
	if err != nil {
		t.Fatalf("TreeSHA: %v", err)
	}

	// An UNRELATED CommitTreeMerge call: exact tree and ordered parents,
	// deliberately DIFFERENT message.
	sha, err := gitutil.CommitTreeMerge(dir, tree, parent1, parent2, "an unrelated operator-created object")
	if err != nil {
		t.Fatalf("CommitTreeMerge: %v", err)
	}
	// Read back the object's OWN stored message (rather than assuming the
	// literal -m argument survives byte-for-byte — git's own commit-message
	// normalization is CommitMessageBody's own concern, not this test's;
	// see its doc comment) so the two assertions below differ ONLY in
	// whether the message argument matches, isolating exactly what this
	// fix checks.
	ownMessage, err := gitutil.CommitMessageBody(dir, sha)
	if err != nil {
		t.Fatalf("CommitMessageBody: %v", err)
	}

	proven, err := gitutil.DanglingCollapsedMergeExists(dir, tree, parent1, parent2, "Merge bead/mindspec-x.1")
	if err != nil {
		t.Fatalf("DanglingCollapsedMergeExists: %v", err)
	}
	if proven {
		t.Fatal("an object with the exact tree/parents but an UNRELATED message must never be accepted as proof")
	}

	// The SAME object, looked up with its OWN actual (stored) message, IS
	// proof — confirms this is a message check, not a false negative on
	// tree/parents.
	proven, err = gitutil.DanglingCollapsedMergeExists(dir, tree, parent1, parent2, ownMessage)
	if err != nil {
		t.Fatalf("DanglingCollapsedMergeExists: %v", err)
	}
	if !proven {
		t.Fatal("the same object, queried with its own actual message, must be found")
	}
}
