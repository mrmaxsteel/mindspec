package executor

// Spec 127 bead 6 (R5(d)): end-to-end resumption tests over a REAL git
// repo, driving PRODUCTION CompleteBead through the full conflict →
// preserve → resolve → --resolve-merge lifecycle. These are the
// mechanism-level proof AC-9(v) requires: no raw `git merge` line is
// ever printed, a plain re-run never touches a preserved conflict, and
// --resolve-merge both re-prints steps (still conflicted) and completes
// (resolved+staged), landing the SAME cleanup a fresh merge success
// reaches.

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// TestCompleteBead_ResolveMerge_FullLifecycle drives the whole R5(d)
// resumption arc through production CompleteBead:
//  1. plain invocation conflicts → preserved (not aborted), no raw
//     `git merge` line, --resolve-merge named.
//  2. plain re-run (no flag) while the conflict is preserved → refuses
//     WITHOUT touching anything (AC-9(v)(δ)'s "plain no-flag re-run
//     refuses without aborting").
//  3. --resolve-merge while STILL conflicted → re-prints resolution
//     steps, still refuses, still does not touch the preserved merge.
//  4. operator resolves + stages (no commit).
//  5. --resolve-merge with the index resolved → completes the merge
//     (git commit --no-edit, preserving the original subject) and
//     proceeds through the SAME cleanup a fresh merge success would
//     reach (worktree removal, branch deletion).
func TestCompleteBead_ResolveMerge_FullLifecycle(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}
	// Make the fake's Remove reify a REAL `git worktree remove` so the
	// post-completion branch deletion below (which requires the bead
	// worktree to no longer be checked out) can actually succeed.
	fake.onRemove = func(name string) {
		if name == "worktree-mindspec-x.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
		}
	}

	// 1. Plain invocation: conflicts, preserved.
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	if strings.Contains(err.Error(), "git merge") {
		t.Fatalf("no raw `git merge` line may ever be printed; got:\n%s", err.Error())
	}
	if !strings.Contains(err.Error(), ResolveMergeFlag) {
		t.Fatalf("recovery must name %s; got:\n%s", ResolveMergeFlag, err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the conflict must be PRESERVED (not aborted) after the first attempt")
	}

	// 2. Plain re-run (no --resolve-merge) while the conflict is
	// preserved: must refuse WITHOUT touching the preserved state
	// (AC-9(v)(delta)) — never abort, never checkout, never commit over it.
	preservedMergeHeadBefore := mergeHeadSHA(t, specWtPath)
	err = g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("a plain re-run over a preserved conflict must refuse")
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("a plain re-run must NOT abort the preserved conflict")
	}
	if got := mergeHeadSHA(t, specWtPath); got != preservedMergeHeadBefore {
		t.Fatalf("MERGE_HEAD must be untouched by a plain re-run; was %s, now %s", preservedMergeHeadBefore, got)
	}

	// 3. --resolve-merge while STILL conflicted: re-prints steps, still
	// refuses, still does not touch the preserved merge.
	err = g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("--resolve-merge over an unresolved conflict must still refuse")
	}
	if !strings.Contains(err.Error(), "conflicted") {
		t.Errorf("the still-conflicted refusal should mention the conflict; got:\n%s", err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("--resolve-merge must NOT abort an unresolved conflict")
	}
	if got := mergeHeadSHA(t, specWtPath); got != preservedMergeHeadBefore {
		t.Fatalf("MERGE_HEAD must be untouched while still conflicted; was %s, now %s", preservedMergeHeadBefore, got)
	}

	// 4. Operator resolves and STAGES (no commit) — resumeReadyToComplete.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	// 5. --resolve-merge with the index resolved: completes the merge
	// and proceeds through cleanup.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("--resolve-merge with a resolved index must complete the merge, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be COMPLETE — no MERGE_HEAD should remain")
	}
	if got, readErr := os.ReadFile(specWtPath + "/c.txt"); readErr != nil || string(got) != "resolved\n" {
		t.Errorf("the spec branch tip must carry the resolved content; got %q, err=%v", got, readErr)
	}
	// Cleanup ran (bead worktree/branch removed, same as a fresh-merge
	// success) — proof the merge completion fell through to the SAME
	// post-merge cleanup path, not a special-cased short circuit.
	// (IsAncestor cannot be checked here: cleanup already deleted the
	// bead branch by the time CompleteBead returns.)
	if len(fake.removeCalls) == 0 {
		t.Error("cleanup (worktree removal) must run after the resumed merge completes, same as a fresh merge success")
	}
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must be deleted after the resumed merge completes and cleanup runs")
	}
}

// TestCompleteBead_ResolveMerge_SourceDriftIsIncorporated is AC-9(v) leg
// alpha (bead-6 fix round 1, G1-1's "PinsOldSourceTipAfterDrift" finding):
// the bead branch gains a NEW commit AFTER the conflict begins but BEFORE
// --resolve-merge completes it. Before this fix, the completed commit's
// second parent stayed the OLD (preserved) MERGE_HEAD and the new
// commit's content was silently omitted — this test proves the current
// tip is incorporated (a catch-up merge runs immediately after
// completing the stale resumed merge) before CompleteBead reports
// success.
func TestCompleteBead_ResolveMerge_SourceDriftIsIncorporated(t *testing.T) {
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

	// 1. Plain invocation: conflicts, preserved.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}

	// 2. DRIFT: a new commit lands on the bead branch AFTER the conflict
	// began, touching a path the resolved conflict never touches (so the
	// catch-up merge below is a clean fast-forward-shaped merge, not a
	// second conflict).
	if err := os.WriteFile(beadWtDir+"/drift.txt", []byte("drifted work\n"), 0o644); err != nil {
		t.Fatalf("write drift file: %v", err)
	}
	runGitIn(t, beadWtDir, "add", "drift.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "drift: bead branch advances after the conflict began")
	driftedBeadTip := refHash(t, dir, "bead/mindspec-x.1")

	// 3. Resolve + stage the ORIGINAL conflict (never touches drift.txt).
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	// 4. --resolve-merge completes the PRESERVED (now-stale) merge, then
	// must incorporate the drift before reporting success.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("--resolve-merge with drift must still converge, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("no merge should remain in progress after drift is incorporated")
	}
	if got, readErr := os.ReadFile(specWtPath + "/c.txt"); readErr != nil || string(got) != "resolved\n" {
		t.Errorf("the resolved content must survive, got %q, err=%v", got, readErr)
	}
	if got, readErr := os.ReadFile(specWtPath + "/drift.txt"); readErr != nil || string(got) != "drifted work\n" {
		t.Errorf("AC-9(v)(alpha): the DRIFTED commit's content must be incorporated, not silently omitted; got %q, err=%v", got, readErr)
	}
	isAnc, ancErr := gitutil.IsAncestor(dir, driftedBeadTip, "spec/077-test")
	if ancErr != nil || !isAnc {
		t.Errorf("AC-9(v)(alpha): the drifted bead tip %s must be an ancestor of spec/077-test after convergence (ancErr=%v, isAnc=%v)", driftedBeadTip, ancErr, isAnc)
	}
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must be deleted once the drift is incorporated and the merge fully converges")
	}
}

// TestCompleteBead_ResolveMerge_ForeignPreservedMergeRefuses is bead-6 fix
// round 1's G1-1 fix, both resumption legs: a preserved MERGE_HEAD that
// does NOT correspond to the requested bead branch (a foreign merge —
// e.g. another bead's conflict left in the SAME shared spec worktree, or
// an unrelated operator merge) must never be silently re-diagnosed as
// this request's own conflict, and must never be completed by
// --resolve-merge.
func TestCompleteBead_ResolveMerge_ForeignPreservedMergeRefuses(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)
	_ = beadWtDir

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}

	// Plant a FOREIGN merge directly in the spec worktree: an unrelated
	// branch, never named by this invocation at all.
	runGitIn(t, dir, "branch", "foreign-branch")
	foreignWt := dir + "/.wt-foreign"
	runGitIn(t, dir, "worktree", "add", foreignWt, "foreign-branch")
	if err := os.WriteFile(foreignWt+"/f.txt", []byte("foreign side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, foreignWt, "add", ".")
	runGitIn(t, foreignWt, "commit", "-m", "foreign change")
	if err := os.WriteFile(specWtPath+"/f.txt", []byte("spec side (unrelated path)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "spec-side change to the same path foreign-branch touches")
	_, _ = exec.Command("git", "-C", specWtPath, "merge", "--no-ff", "-m", "Merge foreign-branch", "foreign-branch").CombinedOutput()
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: the foreign merge must be mid-conflict in the spec worktree")
	}

	// STILL-CONFLICTED leg: refuses, names the mismatch, touches nothing.
	preservedBefore := mergeHeadSHA(t, specWtPath)
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("a foreign preserved merge must refuse, not be silently re-diagnosed as this bead's own conflict")
	}
	if !strings.Contains(err.Error(), "does not correspond to the requested source") {
		t.Errorf("refusal must name the binding mismatch; got:\n%s", err.Error())
	}
	if got := mergeHeadSHA(t, specWtPath); got != preservedBefore {
		t.Fatalf("the foreign merge must be untouched; MERGE_HEAD was %s, now %s", preservedBefore, got)
	}

	// READY-TO-COMPLETE leg: resolve + stage the FOREIGN conflict, then
	// invoke THIS bead's --resolve-merge — must still refuse rather than
	// silently completing someone else's merge.
	if err := os.WriteFile(specWtPath+"/f.txt", []byte("resolved foreign content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", "f.txt")
	err = g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("a resolved-and-staged FOREIGN merge must still refuse --resolve-merge for an unrelated bead")
	}
	if !strings.Contains(err.Error(), "does not correspond to the requested source") {
		t.Errorf("refusal must name the binding mismatch; got:\n%s", err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the foreign merge must remain preserved (uncommitted) — this bead's invocation must never complete it")
	}
}

// TestCompleteBead_ResolveMerge_AncestorForeignMergeRefuses is bead-6 fix
// round 2's G1 confirm-round finding: classifyPreservedMergeBinding was
// refuted for a preserved MERGE_HEAD belonging to a DIFFERENT branch that
// merely happens to be an ANCESTOR of the requested source's current tip
// — ancestry alone cannot establish that a preserved merge is this
// bead's own history, merely drifted. Unlike
// TestCompleteBead_ResolveMerge_ForeignPreservedMergeRefuses (whose
// decoy branch is NOT an ancestor of the bead branch — the easy case
// bindingForeign already caught), this fixture makes decoy-src's tip a
// REAL ancestor of the bead branch's current tip (via a genuine,
// unrelated merge on the bead branch itself) — the exact shape that
// used to fall through to bindingDrifted and be silently adopted as
// "this bead's own conflict, just drifted", up to and including being
// COMPLETED by --resolve-merge.
func TestCompleteBead_ResolveMerge_AncestorForeignMergeRefuses(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/077-ancfor")
	runGitIn(t, dir, "branch", "bead/mindspec-ancfor.1")
	runGitIn(t, dir, "branch", "decoy-src")

	specWtPath := dir + "/.worktrees/worktree-spec-077-ancfor"
	if err := os.MkdirAll(dir+"/.worktrees", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-ancfor")

	// decoy-src: an UNRELATED branch that will become the FOREIGN
	// preserved merge's source.
	decoyWt := dir + "/.wt-decoy-src"
	runGitIn(t, dir, "worktree", "add", decoyWt, "decoy-src")
	if err := os.WriteFile(decoyWt+"/shared.txt", []byte("decoy version\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, decoyWt, "add", ".")
	runGitIn(t, decoyWt, "commit", "-m", "decoy work")
	runGitIn(t, dir, "worktree", "remove", "--force", decoyWt)
	decoyTip := refHash(t, dir, "decoy-src")

	// bead/mindspec-ancfor.1: its OWN work, PLUS a REAL (clean, no
	// conflict) merge of decoy-src for unrelated reasons — so decoyTip
	// becomes an ANCESTOR of the bead branch's CURRENT tip, without the
	// bead branch itself BEING decoy-src.
	beadWtDir := dir + "/.wt-bead-ancfor1"
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-ancfor.1")
	if err := os.WriteFile(beadWtDir+"/own.txt", []byte("bead's own work\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead's own work")
	runGitIn(t, beadWtDir, "merge", "--no-ff", "-m", "bead incorporates decoy-src for unrelated reasons", "decoy-src")
	beadTip := refHash(t, dir, "bead/mindspec-ancfor.1")
	if isAnc, ancErr := gitutil.IsAncestor(dir, decoyTip, beadTip); ancErr != nil || !isAnc {
		t.Fatalf("fixture invariant broken: decoyTip must be an ancestor of the bead branch's current tip, isAnc=%v err=%v", isAnc, ancErr)
	}

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-ancfor.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-ancfor.1",
	}}

	// Plant the FOREIGN preserved merge directly in the SPEC worktree:
	// decoy-src conflicts with the spec's own edit to the SAME path —
	// never a merge of the bead branch at all.
	if err := os.WriteFile(specWtPath+"/shared.txt", []byte("spec version\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "spec-side edit to the same path decoy-src touches")
	_, _ = exec.Command("git", "-C", specWtPath, "merge", "--no-ff", "-m", "Merge decoy-src", "decoy-src").CombinedOutput()
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: the decoy merge must be mid-conflict in the spec worktree")
	}
	if got := mergeHeadSHA(t, specWtPath); got != decoyTip {
		t.Fatalf("fixture invariant broken: MERGE_HEAD must be decoyTip; got %s want %s", got, decoyTip)
	}

	specTipBefore := refHash(t, dir, "spec/077-ancfor")

	// STILL-CONFLICTED leg: ancestry alone must not let this be
	// silently re-diagnosed as the bead's own (merely drifted) conflict.
	err := g.CompleteBead("mindspec-ancfor.1", "spec/077-ancfor", "", "", false)
	if err == nil {
		t.Fatal("an ancestor-but-foreign preserved merge must refuse, not be silently trusted as this bead's own drifted conflict")
	}
	if !strings.Contains(err.Error(), "does not correspond to the requested source") {
		t.Errorf("refusal must name the binding mismatch (bindingForeign), not the still-conflicted resolution steps; got:\n%s", err.Error())
	}
	if got := mergeHeadSHA(t, specWtPath); got != decoyTip {
		t.Fatalf("the foreign merge must be untouched; MERGE_HEAD was %s, now %s", decoyTip, got)
	}

	// READY-TO-COMPLETE leg: resolve + stage the FOREIGN conflict (as an
	// operator innocently would, believing the still-conflicted message
	// above described their own bead's conflict), then invoke
	// --resolve-merge — must STILL refuse. Before this fix, ancestry
	// alone classified this bindingDrifted, so completeResumedMerge
	// would COMMIT decoy-src's content under its "Merge decoy-src"
	// subject and report success — silently adopting a foreign merge as
	// this bead's own landing.
	if err := os.WriteFile(specWtPath+"/shared.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", "shared.txt")
	err = g.CompleteBead("mindspec-ancfor.1", "spec/077-ancfor", "", "", true)
	if err == nil {
		t.Fatal("an ancestor-but-foreign preserved merge must refuse --resolve-merge too — completing it would silently adopt decoy-src's content as this bead's own landed merge")
	}
	if !strings.Contains(err.Error(), "does not correspond to the requested source") {
		t.Errorf("refusal must name the binding mismatch; got:\n%s", err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the foreign merge must remain preserved (uncommitted) — this bead's invocation must never complete it")
	}
	if got := refHash(t, dir, "spec/077-ancfor"); got != specTipBefore {
		t.Errorf("spec branch tip must not advance — nothing may be committed over an ancestor-but-foreign preserved merge; was %s, now %s", specTipBefore, got)
	}
}

// TestCompleteBead_ResolveMerge_InvokedFromWrongCheckout is AC-9(v) leg
// gamma (spec text (ii): "resolves its own target worktree and branch,
// so neither operand is scrollback-pinned"): the calling process's cwd is
// a DECOY worktree entirely unrelated to this bead's spec worktree at
// invocation time. CompleteBead must still resolve and complete the
// correct spec worktree — never operate on whatever the process happens
// to be sitting in.
func TestCompleteBead_ResolveMerge_InvokedFromWrongCheckout(t *testing.T) {
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

	// A DECOY worktree, checked out to an unrelated branch, with its own
	// unrelated content — never named by this invocation.
	runGitIn(t, dir, "branch", "decoy-branch")
	decoyWt := dir + "/.wt-decoy"
	runGitIn(t, dir, "worktree", "add", decoyWt, "decoy-branch")
	if err := os.WriteFile(decoyWt+"/decoy.txt", []byte("decoy content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, decoyWt, "add", ".")
	runGitIn(t, decoyWt, "commit", "-m", "decoy change")
	decoyTipBefore := refHash(t, dir, "decoy-branch")

	// 1. Plain invocation from the decoy checkout: conflicts, preserved
	// (proves the FIRST attempt already resolves specWtPath correctly,
	// not the decoy).
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(decoyWt); err != nil {
		t.Fatalf("chdir into decoy worktree: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	if gitutil.MergeInProgress(decoyWt) {
		t.Fatal("the decoy checkout must never enter a merge state — the conflict belongs to specWtPath")
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the conflict must be preserved in the RESOLVED spec worktree, not wherever the process cwd happened to be")
	}

	// 2. Resolve + stage, still invoked from the decoy checkout.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	// 3. --resolve-merge, still invoked from the decoy checkout: must
	// complete the merge in specWtPath.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("--resolve-merge invoked from an unrelated checkout must still complete the correct spec worktree, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be complete in specWtPath")
	}
	if got, readErr := os.ReadFile(specWtPath + "/c.txt"); readErr != nil || string(got) != "resolved\n" {
		t.Errorf("the resolved content must land on the spec branch; got %q, err=%v", got, readErr)
	}

	// The decoy checkout must be entirely untouched throughout.
	if got := refHash(t, dir, "decoy-branch"); got != decoyTipBefore {
		t.Errorf("the decoy branch must never move; was %s, now %s", decoyTipBefore, got)
	}
	if got, readErr := os.ReadFile(decoyWt + "/decoy.txt"); readErr != nil || string(got) != "decoy content\n" {
		t.Errorf("the decoy worktree's content must be untouched; got %q, err=%v", got, readErr)
	}
}

// TestCompleteBead_ResolveMerge_AddStepOperandsExactlyMatchConflictedSet
// is AC-9(v) leg eta: the printed resolution steps' `git add` line
// operands must equal EXACTLY the conflicted-file set (parsed, not
// merely "contains one path") — never a wider -A/./-u form. This needs
// MULTIPLE conflicted files to be a meaningful parity check (a
// single-file case cannot distinguish "the exact set" from "at least one
// path is named").
func TestCompleteBead_ResolveMerge_AddStepOperandsExactlyMatchConflictedSet(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/077-multi")
	runGitIn(t, dir, "branch", "bead/mindspec-multi.1")

	specWtPath := dir + "/.worktrees/worktree-spec-077-multi"
	if err := os.MkdirAll(dir+"/.worktrees", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-multi")
	for _, name := range []string{"c1.txt", "c2.txt"} {
		if err := os.WriteFile(specWtPath+"/"+name, []byte("spec side "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "spec change (two files)")

	beadWtDir := dir + "/.wt-bead-multi1"
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-multi.1")
	for _, name := range []string{"c1.txt", "c2.txt"} {
		if err := os.WriteFile(beadWtDir+"/"+name, []byte("bead side "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead change (two files)")

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-multi.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-multi.1",
	}}

	if err := g.CompleteBead("mindspec-multi.1", "spec/077-multi", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	conflicted := gitutil.ConflictedFiles(specWtPath)
	if len(conflicted) != 2 {
		t.Fatalf("fixture invariant broken: expected 2 conflicted files, got %v", conflicted)
	}

	// A SECOND (still-conflicted) invocation is the one that renders the
	// pinned resolution-step template (resolutionSteps) — the first,
	// fresh-conflict invocation renders the caller's own rich conflict
	// message instead, which carries no "git add" line at all.
	err := g.CompleteBead("mindspec-multi.1", "spec/077-multi", "", "", true)
	if err == nil {
		t.Fatal("expected the still-conflicted refusal, got nil")
	}

	msg := err.Error()
	var addLine string
	for _, line := range strings.Split(msg, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "2. git add ") {
			addLine = strings.TrimPrefix(trimmed, "2. git add ")
			break
		}
	}
	if addLine == "" {
		t.Fatalf("expected a '2. git add <paths>' line in the resolution steps; got:\n%s", msg)
	}
	gotOperands := strings.Fields(addLine)
	if len(gotOperands) != len(conflicted) {
		t.Fatalf("AC-9(v)(eta): add-step operand count = %d, want exactly %d (the conflicted set %v); got operands %v", len(gotOperands), len(conflicted), conflicted, gotOperands)
	}
	wantSet := map[string]bool{}
	for _, f := range conflicted {
		wantSet[f] = true
	}
	for _, op := range gotOperands {
		if !wantSet[op] {
			t.Errorf("AC-9(v)(eta): add-step operand %q is not in the conflicted set %v — the emitted add-step must equal the conflicted set exactly, never a wider form", op, conflicted)
		}
	}
	for _, forbidden := range []string{"-A", ".", "-u"} {
		for _, op := range gotOperands {
			if op == forbidden {
				t.Errorf("AC-9(v)(eta): the emitted add-step must never contain the forbidden whole-index form %q", forbidden)
			}
		}
	}
}

// TestCommitAll_RefusesOverAPreservedMerge is bead-6 fix round 1's F1-1
// fixture: internal/approve's spec.go/plan.go reach the R5(d)(v)
// preserved-merge precondition EXCLUSIVELY through the
// executor.Executor interface's CommitAll method (grep-confirmed: this
// is the one production call site of gitutil.CommitAll, inside
// commitWithExport). Rather than standing up the full ApproveSpec/
// ApprovePlan gate chain (bd epic, spec.md validation, etc.) just to
// reach this call, this drives the EXACT EXPORTED METHOD those two call
// sites invoke directly — proving the reach with a real red-on-revert
// test rather than an architectural (grep-only) argument alone.
func TestCommitAll_RefusesOverAPreservedMerge(t *testing.T) {
	g, _, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)
	_ = beadWtDir

	orig := execBeadExportFn
	t.Cleanup(func() { execBeadExportFn = orig })
	execBeadExportFn = func(workdir string) error { return nil }

	// Plant a real, mid-conflict merge in the spec worktree (never a
	// merge this CommitAll call started).
	_, _ = exec.Command("git", "-C", specWtPath, "merge", "--no-ff", "bead/mindspec-x.1").CombinedOutput()
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: the spec worktree must be mid-conflict")
	}
	headBefore := refHash(t, specWtPath, "HEAD")

	err := g.CommitAll(specWtPath, "chore: approve spec 999-test")
	if err == nil {
		t.Fatal("CommitAll must refuse rather than commit over a preserved merge (the exact two-parent chore: resurrection spec 125 shipped to fix)")
	}
	if !strings.Contains(err.Error(), "already in progress there that this run did not start") {
		t.Errorf("refusal must name the preserved-merge precondition; got: %v", err)
	}
	if got := refHash(t, specWtPath, "HEAD"); got != headBefore {
		t.Errorf("HEAD must not advance — CommitAll must touch nothing on this refusal; was %s, now %s", headBefore, got)
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Error("the preserved merge must remain untouched (never committed, never aborted)")
	}
}

// mergeHeadSHA reads MERGE_HEAD's resolved SHA in workdir (empty if
// absent), used to prove a plain re-run or a still-conflicted
// --resolve-merge invocation never touches the preserved merge state.
func mergeHeadSHA(t *testing.T, workdir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", workdir, "rev-parse", "MERGE_HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// TestResumeAwareMerge_NoMergeStateFailureIsDistinctFromConflict is
// AC-9(v)(zeta)/R5(d)(vi): a merge that fails to even START (a dirty
// worktree blocking the checkout/merge, no MERGE_HEAD ever created) is
// diagnosed as a WORKTREE-STATE refusal — never the word "conflict",
// naming the blocking paths — never the still-conflicted resolution
// steps, which would loop forever over a worktree that was never
// mid-merge.
func TestResumeAwareMerge_NoMergeStateFailureIsDistinctFromConflict(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}

	// Dirty the spec worktree on the SAME path the bead branch also
	// touches, uncommitted — `git merge` refuses to even START (no
	// MERGE_HEAD is ever created), the exact worktree-state class.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("uncommitted local edit\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("expected a worktree-state refusal, got nil")
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: a merge that never started must leave no MERGE_HEAD")
	}
	msg := err.Error()
	if strings.Contains(msg, "conflict") {
		t.Errorf("a no-merge-state failure must never use the word \"conflict\"; got:\n%s", msg)
	}
	if !strings.Contains(msg, "c.txt") {
		t.Errorf("the blocking path (c.txt) must be named; got:\n%s", msg)
	}
}
