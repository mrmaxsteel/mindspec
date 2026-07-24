package lifecycle

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// TestMain installs a hermetic, always-unavailable default for
// landedBindingMetadataFn across every test in this file (spec 121 Bead 2):
// the production default (bead.GetMetadata) shells out to a real `bd`
// process against whatever repo happens to be the test binary's cwd —
// none of these throwaway fixture dirs are a real bd-tracked repo, and
// relying on `bd show <fixture-bead-id>` to happen to error is
// environment-dependent (it would silently start SUCCEEDING if a fixture
// ID like "bead-one" ever collided with a real tracked issue). Tests that
// exercise the binding leg itself install their own stub and restore this
// default via t.Cleanup.
// landedBindingMetadataFnDefault captures the PRODUCTION default of
// landedBindingMetadataFn before TestMain substitutes the hermetic stub,
// so the spec 125 F3-2 pointer pin
// (TestLandedBindingMetadataFnDefaultPinned) can still assert the real
// default is bead.GetMetadata — the read gate cannot go hollow even
// though every test in this package runs behind the stub.
var landedBindingMetadataFnDefault func(string) (map[string]interface{}, error)

func TestMain(m *testing.M) {
	landedBindingMetadataFnDefault = landedBindingMetadataFn
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	os.Exit(m.Run())
}

// initLandedRepo builds a real throwaway git repo with a spec/<id> branch
// forked from main — the fixture shape FindLandedMerge/MergedUnclosed
// operate over. run is a t.Helper closure for issuing further git commands
// against the same repo.
func initLandedRepo(t *testing.T, specID string) (dir string, run func(args ...string)) {
	t.Helper()
	dir = t.TempDir()
	run = func(args ...string) {
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
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")
	run("checkout", "-b", "spec/"+specID)
	return dir, run
}

// mergeBead creates bead/<id> off the current HEAD, commits one change, and
// merges it (--no-ff, the deterministic gitutil.MergeInto message) back into
// whatever branch is currently checked out.
func mergeBead(t *testing.T, run func(args ...string), dir, beadID, specBranch string) {
	t.Helper()
	run("checkout", "-b", "bead/"+beadID)
	os.WriteFile(filepath.Join(dir, beadID+".txt"), []byte(beadID+"\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work "+beadID)
	run("checkout", specBranch)
	run("merge", "--no-ff", "-m", "Merge bead/"+beadID, "bead/"+beadID)
}

// commitResolvedMerge builds a deterministic 2-parent merge commit whose tree
// is the CURRENT index (a hand-staged conflict resolution), with the given
// first/second parents and the deterministic "Merge <branch>" subject, and
// fast-forwards the current branch to it. It exists because a REAL `git merge`
// conflict followed by `git commit` is not portable across git builds — the
// conflict-merge MERGE_MSG/commit behavior varies (spec 121 CI: some builds
// abort on an empty MERGE_MSG, others produced a merge the first-parent scan
// did not see), whereas `commit-tree` yields the identical 2-parent merge
// shape on every git version. Callers stage the resolution and assert the
// two sides genuinely conflict separately, so the property under test (a
// conflict-RESOLUTION merge is identified) is preserved.
func commitResolvedMerge(t *testing.T, dir, firstParent, secondParent, subject string) string {
	t.Helper()
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	tree := git("write-tree")
	merge := git("commit-tree", tree, "-p", firstParent, "-p", secondParent, "-m", subject)
	git("reset", "--hard", merge)
	return merge
}

// landedMergeDiag renders the git state a FindLandedMerge failure needs to be
// self-explaining when the failure does not reproduce locally (spec 121 CI):
// the git build, and the first-parent merge commits the scan actually sees
// (short SHA, parent list, subject) — so "the merge was never found" (an empty
// or malformed scan) is distinguishable from a net-effect refusal straight
// from the CI log, without a second blind push.
func landedMergeDiag(t *testing.T, dir string) string {
	t.Helper()
	ver, _ := exec.Command("git", "version").CombinedOutput()
	merges, _ := exec.Command("git", "-C", dir, "log", "--first-parent", "--merges",
		"--format=%h parents=[%p] %s", "spec/119-test").CombinedOutput()
	return "DIAG " + strings.TrimSpace(string(ver)) +
		" | first-parent-merges: " + strings.TrimSpace(string(merges))
}

// assertSidesConflict verifies branch1 and branch2 genuinely conflict on a
// three-way merge, using the NON-MUTATING `git merge-tree --write-tree`
// plumbing (exit 1 == conflict) instead of a real `git merge`. A real merge
// leaves runner-dependent in-progress state: on the CI runner a `git merge`
// of two conflicting branches returned non-zero WITHOUT writing MERGE_HEAD,
// so a follow-up `git merge --abort` fatals (spec 121 CI) — and the same
// no-MERGE_HEAD state is what made the old conflict-then-`commit` fixture
// produce a non-merge commit the first-parent scan could not find. merge-tree
// checks the conflict touching neither the working tree, index, nor HEAD.
func assertSidesConflict(t *testing.T, dir, branch1, branch2 string) {
	t.Helper()
	err := exec.Command("git", "-C", dir, "merge-tree", "--write-tree", branch1, branch2).Run()
	exit, ok := err.(*exec.ExitError)
	if err == nil || !ok || exit.ExitCode() != 1 {
		t.Fatalf("test setup: expected %s and %s to conflict (merge-tree exit 1), got err=%v", branch1, branch2, err)
	}
}

func TestFindLandedMerge_Identified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("FindLandedMerge: %v", err)
	}
	if len(landed.SHA) < 7 || len(landed.FirstParent) < 7 || len(landed.SecondParent) < 7 {
		t.Fatalf("expected populated SHAs, got %+v", landed)
	}
}

// TestFindLandedMerge_FreshBranchNotFound pins the AC-10 load-bearing
// property: a freshly-claimed bead branch (zero own commits, trivially an
// ancestor of the spec branch) produces no merge commit at all — so
// FindLandedMerge correctly reports ErrLandedMergeNotFound, never a false
// positive.
func TestFindLandedMerge_FreshBranchNotFound(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/fresh")
	run("checkout", "spec/119-test")

	_, err := FindLandedMerge(dir, "spec/119-test", "fresh")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound, got %v", err)
	}
}

// TestFindLandedMerge_NoBranchAtAllNotFound: a bead that was never even
// branched (no bead/<id> ref ever existed, no merge commit) also reports
// ErrLandedMergeNotFound.
func TestFindLandedMerge_NoBranchAtAllNotFound(t *testing.T) {
	dir, _ := initLandedRepo(t, "119-test")

	_, err := FindLandedMerge(dir, "spec/119-test", "never-existed")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound, got %v", err)
	}
}

// TestFindLandedMerge_BranchDeletedAfterMerge: the common recovery
// scenario — the bead branch was merged AND then deleted (mimicking
// CompleteBead's cleanup). Post-spec-121, the subject text alone is no
// longer sufficient once every OTHER corroboration leg is gone (AC-11):
// this scenario now converges because CompleteBead's merge-time binding
// write (R5(b)) recorded the landed-binding metadata BEFORE the branch was
// deleted — stubbed here via landedBindingMetadataFn to isolate the
// FindLandedMerge-level behavior from a real bd process. FindLandedMerge
// must still identify the merge from the spec branch's own history,
// confirmed by the binding datum.
func TestFindLandedMerge_BranchDeletedAfterMerge(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	beadTip := revParseIn(t, dir, "bead/bead-one")
	run("branch", "-D", "bead/bead-one")

	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(issueID string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_second_parent": beadTip}, nil
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("FindLandedMerge after branch deletion (with a recorded landed-binding): %v", err)
	}
	if landed.SHA == "" {
		t.Fatal("expected a populated merge SHA")
	}
}

// TestFindLandedMerge_BranchDeletedNoBindingNoEvidence is the spec 121
// AC-11 companion: with the branch ALSO deleted and no landed-binding
// recorded (the pre-121 state — a subject-scan candidate exists but every
// corroboration leg is unavailable), FindLandedMerge must NOT identify the
// merge on the subject text alone. It returns a *LandedMergeNoEvidence
// carrying the candidate's SHAs.
func TestFindLandedMerge_BranchDeletedNoBindingNoEvidence(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	run("branch", "-D", "bead/bead-one")

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	var noEvidence *LandedMergeNoEvidence
	if !errors.As(err, &noEvidence) {
		t.Fatalf("expected *LandedMergeNoEvidence, got %v", err)
	}
	if noEvidence.MergeSHA == "" || noEvidence.SecondParent == "" {
		t.Fatalf("expected populated candidate SHAs, got %+v", noEvidence)
	}
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Error("LandedMergeNoEvidence must still satisfy errors.Is(err, ErrLandedMergeNotFound)")
	}
}

// TestFindLandedMerge_SpoofedSubjectOverWrongSecondParentRefuses is spec
// 121 AC-11's spoof fixture: a hand-crafted "Merge bead/<id>" commit whose
// second parent is a NON-EMPTY, unrelated commit (not the real bead's
// work) and no admissible datum confirms it — FindLandedMerge must refuse,
// proving the identification is by DATUM, never by the subject text or
// second-parent non-emptiness alone.
func TestFindLandedMerge_SpoofedSubjectOverWrongSecondParentRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")

	// An unrelated branch with real (non-empty) content stands in for the
	// "wrong bead's work" a spoofed merge subject points at.
	run("checkout", "-b", "unrelated-work")
	os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated content")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/spoofed-bead", "unrelated-work")

	_, err := FindLandedMerge(dir, "spec/119-test", "spoofed-bead")
	var noEvidence *LandedMergeNoEvidence
	if !errors.As(err, &noEvidence) {
		t.Fatalf("expected the spoofed subject to refuse via the no-evidence path, got %v", err)
	}
}

// revParseIn resolves ref's SHA in dir via a plain `git rev-parse`.
func revParseIn(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// TestFindLandedMerge_ReviewedHeadSHACorroborates: a registered panel whose
// reviewed_head_sha equals the merge's second parent corroborates the
// match (still identified).
func TestFindLandedMerge_ReviewedHeadSHACorroborates(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("baseline FindLandedMerge: %v", err)
	}

	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })
	beadID := "bead-one"
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{
			Dir: "/fake/review/one",
			Panel: panel.Panel{
				BeadID:          &beadID,
				ReviewedHeadSHA: landed.SecondParent,
			},
		}}
	}

	got, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("FindLandedMerge with corroborating panel: %v", err)
	}
	if got.SHA != landed.SHA {
		t.Errorf("SHA = %q, want %q", got.SHA, landed.SHA)
	}
}

// TestFindLandedMerge_ReviewedHeadSHAContradicts: a registered panel whose
// reviewed_head_sha is neither equal to nor an ancestor of the merge's
// second parent CONTRADICTS the match — not a positive identification.
func TestFindLandedMerge_ReviewedHeadSHAContradicts(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")

	// A second, UNRELATED branch/commit — neither ancestor nor descendant
	// of bead/bead-one's tip — stands in for a contradicting reviewed_head_sha.
	run("checkout", "main")
	run("checkout", "-b", "unrelated")
	os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated work")
	cmd := exec.Command("git", "-C", dir, "rev-parse", "unrelated")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse unrelated: %v", err)
	}
	unrelatedSHA := string(out)
	if len(unrelatedSHA) > 0 && unrelatedSHA[len(unrelatedSHA)-1] == '\n' {
		unrelatedSHA = unrelatedSHA[:len(unrelatedSHA)-1]
	}

	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })
	beadID := "bead-one"
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{
			Dir: "/fake/review/one",
			Panel: panel.Panel{
				BeadID:          &beadID,
				ReviewedHeadSHA: unrelatedSHA,
			},
		}}
	}

	_, err = FindLandedMerge(dir, "spec/119-test", "bead-one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound on contradiction, got %v", err)
	}
}

func TestMergedUnclosed_BranchDeleted(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	beadTip := revParseIn(t, dir, "bead/bead-one")
	run("branch", "-D", "bead/bead-one")

	// Spec 121: the branch is gone with no panel registered, so the
	// merge-time landed-binding (recorded by CompleteBead's fail-closed
	// write BEFORE that same cleanup ran) is the confirming datum — the
	// post-121 invariant this fixture reproduces.
	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(issueID string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_second_parent": beadTip}, nil
	}

	landed, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if !ok {
		t.Fatal("expected merged-unclosed to be true")
	}
	if landed == nil || landed.SHA == "" {
		t.Fatalf("expected populated landed evidence, got %+v", landed)
	}
}

// TestMergedUnclosed_NoEvidencePropagatesAsError is spec 121's companion to
// TestMergedUnclosed_BranchDeleted: WITHOUT the landed-binding (or any
// other datum), a subject-scan candidate must surface as a genuine error
// (a *LandedMergeNoEvidence, via errors.As) — not silently swallowed into
// (nil, false, nil) — so a caller like internal/complete's reconcile can
// render the R5(c) attested-restore refusal instead of a generic "nothing
// found" message.
func TestMergedUnclosed_NoEvidencePropagatesAsError(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	run("branch", "-D", "bead/bead-one")

	_, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one")
	if ok {
		t.Fatal("expected merged-unclosed to be false")
	}
	var noEvidence *LandedMergeNoEvidence
	if !errors.As(err, &noEvidence) {
		t.Fatalf("expected a *LandedMergeNoEvidence error, got %v", err)
	}
}

func TestMergedUnclosed_BranchSurvivesAsAncestor(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	// bead/bead-one still exists but carries no NEW commits beyond the merge —
	// benign merged-but-undeleted.

	_, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if !ok {
		t.Fatal("expected merged-unclosed to be true for a merged, surviving, non-diverged branch")
	}
}

// TestMergedUnclosed_BranchDivergedNotFlagged pins the anti-false-flag
// requirement: if bead/<id> carries NEW commits landed AFTER the identified
// merge (real still-unmerged work), it must NOT be reported merged-unclosed.
func TestMergedUnclosed_BranchDivergedNotFlagged(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")

	run("checkout", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "one-more.txt"), []byte("more\n"), 0644)
	run("add", ".")
	run("commit", "-m", "more work not yet merged")
	run("checkout", "spec/119-test")

	_, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if ok {
		t.Fatal("a branch carrying new unlanded commits must NOT be flagged merged-unclosed")
	}
}

func TestMergedUnclosed_NoLandedMerge(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/fresh")
	run("checkout", "spec/119-test")

	_, ok, err := MergedUnclosed(dir, "spec/119-test", "fresh")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if ok {
		t.Fatal("expected merged-unclosed to be false when no landed merge exists")
	}
}

// TestReviewedHeadSHAForBead_MalformedSpecBranchProceedsRootOnly is spec
// 120 AC-23's landed.go:163 companion: a malformed spec branch (its
// TrimPrefix-derived specID fails idvalidate.SpecID) makes corroboration
// proceed root-only — landedPanelScanFn is called with ONLY the repo
// root, never a composed (and therefore possibly-hostile) spec-dir root.
func TestReviewedHeadSHAForBead_MalformedSpecBranchProceedsRootOnly(t *testing.T) {
	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })

	var gotRoots []string
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		gotRoots = roots
		return nil
	}

	_, found := reviewedHeadSHAForBead("/repo", "spec/x;evil", "mindspec-x.1")
	if found {
		t.Error("expected no corroboration available for a malformed spec branch")
	}
	if len(gotRoots) != 1 || gotRoots[0] != "/repo" {
		t.Errorf("expected corroboration to scan root-only, got roots=%v", gotRoots)
	}
}

// TestFindLandedMerge_RevertNotIdentified is spec 121 AC-10(i): a
// positively-corroborated candidate merge whose OWN content was later
// reverted on specBranch must NOT be identified — R5(d)'s net-effect
// since-M check gates identification, not corroboration alone.
func TestFindLandedMerge_RevertNotIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	mergeSHA := revParseIn(t, dir, "spec/119-test")
	run("revert", "--no-edit", "-m", "1", mergeSHA)

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound after a revert of the merge's own content, got %v", err)
	}
	if !strings.Contains(err.Error(), "reverted") {
		t.Errorf("expected the refusal to name the revert evidence, got: %v", err)
	}
	// AC-10: this must be a DIFFERENT error shape than the no-evidence
	// path — a *LandedMergeNoEvidence would (mis)invite an attested-restore
	// recovery, which is wrong here (the content was deliberately reverted,
	// not merely uncorroborated).
	var noEvidence *LandedMergeNoEvidence
	if errors.As(err, &noEvidence) {
		t.Error("a reverted merge must not surface as a LandedMergeNoEvidence (that implies attested-restore, which is wrong for a genuine revert)")
	}
}

// TestFindLandedMerge_RevertThenReapplyIdentified is AC-10(ii)'s
// anti-overreach guard (the spec's stated tag deviation — passes both
// before and after this bead's changes): a revert FOLLOWED BY a later
// commit that reintroduces the SAME net content is still identified.
// "Ever reverted ⇒ reject" would be over-rejection.
func TestFindLandedMerge_RevertThenReapplyIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	mergeSHA := revParseIn(t, dir, "spec/119-test")
	run("revert", "--no-edit", "-m", "1", mergeSHA)
	// Reapply: a NEW commit recreating the exact file the bead's merge
	// introduced (same net content, different commit shape than a
	// cherry-pick — see the sibling test below for that shape).
	os.WriteFile(filepath.Join(dir, "bead-one.txt"), []byte("bead-one\n"), 0644)
	run("add", ".")
	run("commit", "-m", "reapply bead-one's change")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("expected revert-then-reapply to still identify the merge, got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the original merge %q", landed.SHA, mergeSHA)
	}
}

// TestFindLandedMerge_CherryPickReapplyIdentified: the OTHER reapply
// shape — a cherry-pick of the bead's own reverted commit (a DIFFERENT
// commit SHA, same net diff) — is still identified: net effect is
// content-based, never SHA-based.
func TestFindLandedMerge_CherryPickReapplyIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	mergeSHA := revParseIn(t, dir, "spec/119-test")
	beadCommitSHA := revParseIn(t, dir, "bead/bead-one")
	run("revert", "--no-edit", "-m", "1", mergeSHA)
	run("cherry-pick", beadCommitSHA)

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("expected a cherry-pick reapply to still identify the merge, got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the original merge %q", landed.SHA, mergeSHA)
	}
}

// TestFindLandedMerge_PerDatumSufficiency pins spec 121 AC-12: each
// admissible datum is INDEPENDENTLY sufficient to positively identify a
// candidate — panel-only, surviving-branch-only, and binding-only each
// close the identification alone, with the other two legs unavailable.
func TestFindLandedMerge_PerDatumSufficiency(t *testing.T) {
	t.Run("panel-only", func(t *testing.T) {
		dir, run := initLandedRepo(t, "119-test")
		mergeBead(t, run, dir, "bead-one", "spec/119-test")
		secondParent := revParseIn(t, dir, "bead/bead-one")
		run("branch", "-D", "bead/bead-one") // surviving-branch leg unavailable

		origScan := landedPanelScanFn
		t.Cleanup(func() { landedPanelScanFn = origScan })
		beadID := "bead-one"
		landedPanelScanFn = func(roots ...string) []panel.Registration {
			return []panel.Registration{{Panel: panel.Panel{BeadID: &beadID, ReviewedHeadSHA: secondParent}}}
		}
		// binding leg unavailable via TestMain's default stub.

		landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
		if err != nil {
			t.Fatalf("panel-only corroboration must be sufficient, got: %v", err)
		}
		if landed == nil {
			t.Fatal("expected a positively identified merge")
		}
	})

	t.Run("surviving-branch-only", func(t *testing.T) {
		dir, run := initLandedRepo(t, "119-test")
		mergeBead(t, run, dir, "bead-one", "spec/119-test")
		// No panel registered (landedPanelScanFn default finds none here);
		// binding leg unavailable via TestMain's default stub.

		landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
		if err != nil {
			t.Fatalf("surviving-branch-only corroboration must be sufficient, got: %v", err)
		}
		if landed == nil {
			t.Fatal("expected a positively identified merge")
		}
	})

	t.Run("binding-only", func(t *testing.T) {
		dir, run := initLandedRepo(t, "119-test")
		mergeBead(t, run, dir, "bead-one", "spec/119-test")
		secondParent := revParseIn(t, dir, "bead/bead-one")
		run("branch", "-D", "bead/bead-one")

		origBinding := landedBindingMetadataFn
		t.Cleanup(func() { landedBindingMetadataFn = origBinding })
		landedBindingMetadataFn = func(issueID string) (map[string]interface{}, error) {
			return map[string]interface{}{"mindspec_landed_second_parent": secondParent}, nil
		}

		landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
		if err != nil {
			t.Fatalf("binding-only corroboration must be sufficient, got: %v", err)
		}
		if landed == nil {
			t.Fatal("expected a positively identified merge")
		}
	})
}

// TestFindLandedMerge_MultiCommitBeadBranchIdentified is spec 121 AC-12's
// multi-commit honest-merge fixture (final-review F2-1(a)): a bead branch
// carrying SEVERAL own commits, merged --no-ff with the deterministic
// message, must reconcile exactly like the single-commit mergeBead() shape —
// identified by FindLandedMerge AND flagged merged-unclosed. Every other R5
// fixture routes through mergeBead()'s exactly-one-commit merge; this pins
// that nothing in the identification (subject scan, corroboration,
// R5(d) net-effect subsumption) silently assumes a single-commit second
// parent.
func TestFindLandedMerge_MultiCommitBeadBranchIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/bead-one")
	for _, f := range []string{"one.txt", "two.txt", "three.txt"} {
		os.WriteFile(filepath.Join(dir, f), []byte(f+"\n"), 0644)
		run("add", ".")
		run("commit", "-m", "work "+f)
	}
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-one", "bead/bead-one")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("a multi-commit bead-branch merge must reconcile, got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the merge %q", landed.SHA, mergeSHA)
	}
	if _, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one"); err != nil || !ok {
		t.Errorf("expected merged-unclosed (ok=true, err=nil), got ok=%v err=%v", ok, err)
	}
}

// TestFindLandedMerge_ConflictResolutionMergeIdentified is spec 121 AC-12's
// conflict-resolution fixture (final-review F2-1(b)), the load-bearing one:
// a REAL conflict resolved at merge time means M's tree matches NEITHER
// parent's version of the conflicted file — and R5(d)'s gate is
// ContentSubsumed(M^1, M, tip), whose merge-tree leg treats a CONFLICT as
// definitive not-landed. An honestly-conflict-resolved-then-landed merge
// refusing here would be the §2(i) permanent-refusal class the whole spec
// forbids (over-refusal of legitimate work). The spec branch is also
// advanced past M with unrelated later work so the subsumption check is
// non-degenerate (tip != M, ours != theirs).
func TestFindLandedMerge_ConflictResolutionMergeIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")

	// A file both sides will edit, committed on the spec branch before the
	// bead forks.
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("base\n"), 0644)
	run("add", ".")
	run("commit", "-m", "seed conflict file")

	run("checkout", "-b", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("bead side\n"), 0644)
	run("add", ".")
	run("commit", "-m", "bead edit")

	run("checkout", "spec/119-test")
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("spec side\n"), 0644)
	run("add", ".")
	run("commit", "-m", "spec edit")

	// The two sides must genuinely CONFLICT (the property under test is a
	// conflict-RESOLUTION merge). Assert that non-mutatingly, then build the
	// resolved merge deterministically — see commitResolvedMerge /
	// assertSidesConflict for why a real `git merge` conflict is not portable.
	assertSidesConflict(t, dir, "bead/bead-one", "spec/119-test")
	// Resolve honestly (content matching NEITHER parent) and commit the merge
	// with the deterministic "Merge bead/bead-one" subject FindLandedMerge's
	// first-parent scan matches, exactly as an operator resolving a
	// gitutil.MergeInto conflict in the spec worktree would.
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("resolved: spec side + bead side\n"), 0644)
	run("add", ".")
	mergeSHA := commitResolvedMerge(t, dir, "spec/119-test", "bead/bead-one", "Merge bead/bead-one")

	// Advance the spec branch with unrelated later work: tip != M, so the
	// R5(d) subsumption evaluates a genuine three-way, not ours==theirs.
	os.WriteFile(filepath.Join(dir, "later.txt"), []byte("later\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated later work")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("an honestly-conflict-resolved merge must reconcile (R5(d) must not over-refuse it), got: %v\n%s", err, landedMergeDiag(t, dir))
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the conflict-resolution merge %q", landed.SHA, mergeSHA)
	}
	if _, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one"); err != nil || !ok {
		t.Errorf("expected merged-unclosed (ok=true, err=nil), got ok=%v err=%v", ok, err)
	}
}

// TestFindLandedMerge_ConflictResolvedRegionRewrittenLaterIdentified is
// spec 121 final-review r2 F2-2r (Probe 1): a conflict-resolution merge M
// whose RESOLVED REGION is later legitimately rewritten on the spec branch
// (honest work built ON M, no revert anywhere) must still be identified.
// Pre-fix, R5(d) collapsed the resulting merge-tree CONFLICT to "reverted
// after landing" — a factually false refusal that deadlocks the reconcile
// path permanently (the branch can be restored, corroboration passes, and
// the subsumption gate refuses again forever — the §2(i) class ADR-0041
// forbids). Post-fix, a CONFLICT at the R5(d) three-way means both sides
// advanced past M^1 on M's own region — landed-then-evolved — and
// identifies; only the CLEAN not-subsumed shape (a genuine backout) refuses.
func TestFindLandedMerge_ConflictResolvedRegionRewrittenLaterIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")

	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("base\n"), 0644)
	run("add", ".")
	run("commit", "-m", "seed conflict file")

	run("checkout", "-b", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("bead side\n"), 0644)
	run("add", ".")
	run("commit", "-m", "bead edit")

	run("checkout", "spec/119-test")
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("spec side\n"), 0644)
	run("add", ".")
	run("commit", "-m", "spec edit")

	assertSidesConflict(t, dir, "bead/bead-one", "spec/119-test")
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("resolved: spec side + bead side\n"), 0644)
	run("add", ".")
	// commit-tree (not a real conflict `commit`): portable across git builds
	// — see commitResolvedMerge.
	mergeSHA := commitResolvedMerge(t, dir, "spec/119-test", "bead/bead-one", "Merge bead/bead-one")

	// The honest later rewrite of the RESOLVED region itself — work built
	// on M, not a revert of it.
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("second edition, superseding the resolution\n"), 0644)
	run("add", ".")
	run("commit", "-m", "later rewrite of the resolved region")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("a landed-then-evolved merge (resolved region honestly rewritten later) must still be identified, got: %v\n%s", err, landedMergeDiag(t, dir))
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the conflict-resolution merge %q", landed.SHA, mergeSHA)
	}
	if _, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one"); err != nil || !ok {
		t.Errorf("expected merged-unclosed (ok=true, err=nil), got ok=%v err=%v", ok, err)
	}
}

// TestFindLandedMerge_CleanMergeContentRewrittenLaterIdentified is F2-2r's
// Probe 2 — the WIDER blast radius: even a plain, clean, single-commit
// mergeBead() merge is over-refused pre-fix once the file M introduced is
// later rewritten on the spec branch (the three-way sees base-absent vs
// two different added contents — a conflict). Honest later evolution of a
// bead's own file is everyday spec-branch life; it must identify.
func TestFindLandedMerge_CleanMergeContentRewrittenLaterIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	// Later honest rewrite of the file the bead's merge introduced.
	os.WriteFile(filepath.Join(dir, "bead-one.txt"), []byte("rewritten by later work built on the bead\n"), 0644)
	run("add", ".")
	run("commit", "-m", "later rewrite of the bead's file")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("a clean merge whose content was honestly rewritten later must still be identified, got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the merge %q", landed.SHA, mergeSHA)
	}
	if _, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one"); err != nil || !ok {
		t.Errorf("expected merged-unclosed (ok=true, err=nil), got ok=%v err=%v", ok, err)
	}
}

// TestFindLandedMerge_EvolvedCleanDivergenceIdentified is spec 125 AC-5's
// core (RED on the spec-init SHA — refused as "reverted after landing"):
// the 8nhe.2 PARTIAL-SUPERSESSION shape. Merge M lands content across TWO
// surfaces; later first-parent commits remove-AND-replace ONE surface (its
// content superseded at a different path) while M's OTHER content remains
// at the tip — explicitly NOT a pure delete/relocate of all of M's paths
// (that is the stated indistinguishable residual, pinned separately by
// TestFindLandedMerge_CleanFullRemovalRefusesResidualFloor). The fixture
// ASSERTS ITS OWN SHAPE-PRECONDITION — today's forward primitive reads it
// as SubsumptionCleanDivergence — so it cannot drift into the already-green
// conflict shape. FindLandedMerge must identify M: content evolved by later
// honest work is EVOLVED, never a revert.
func TestFindLandedMerge_EvolvedCleanDivergenceIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "surface-a.txt"), []byte("alpha payload\n"), 0644)
	os.WriteFile(filepath.Join(dir, "surface-b.txt"), []byte("beta payload\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work bead-one: two surfaces")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-one", "bead/bead-one")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	// Later honest first-parent work: surface-a removed AND replaced at a
	// different path; surface-b (M's other content) remains at the tip.
	run("rm", "surface-a.txt")
	os.WriteFile(filepath.Join(dir, "surface-a2.txt"), []byte("superseding replacement for alpha\n"), 0644)
	run("add", ".")
	run("commit", "-m", "supersede surface-a with surface-a2")

	// AC-5 shape-precondition: TODAY's primitive classifies this fixture
	// as SubsumptionCleanDivergence — the arm this spec sub-classifies.
	outcome, err := gitutil.ContentSubsumedOutcome(dir, mergeSHA+"^1", mergeSHA, "spec/119-test")
	if err != nil {
		t.Fatalf("shape-precondition check: %v", err)
	}
	if outcome != gitutil.SubsumptionCleanDivergence {
		t.Fatalf("fixture shape drifted: want SubsumptionCleanDivergence from the forward check, got %v", outcome)
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("an evolved-content-PRESENT CleanDivergence shape must identify (AC-5, the 8nhe.2 fix), got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the merge %q", landed.SHA, mergeSHA)
	}
}

// TestFindLandedMerge_DifferentRegionLaterWorkIdentified is AC-5's SEPARATE
// sub-case — a GREEN anti-false-ID regression guard, NOT a RED-today
// assertion (plan F1 MINOR): later work on a DIFFERENT region of a file M
// also touched leaves M's own content present, so M identifies. This shape
// resolves to SubsumptionLanded on today's primitive already (asserted, so
// the guard's target is documented); it guards against a per-file-path
// REGRESSION ("M's file was touched later ⇒ reverted"), which would wrongly
// refuse here.
func TestFindLandedMerge_DifferentRegionLaterWorkIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	// A multi-region file seeded on the spec branch BEFORE the bead forks,
	// with enough separation that edits at the two ends never conflict.
	seed := "top: original\n" + strings.Repeat("filler line\n", 10) + "bottom: original\n"
	os.WriteFile(filepath.Join(dir, "shared.txt"), []byte(seed), 0644)
	run("add", ".")
	run("commit", "-m", "seed shared multi-region file")

	run("checkout", "-b", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("top: bead-one's change\n"+strings.Repeat("filler line\n", 10)+"bottom: original\n"), 0644)
	run("add", ".")
	run("commit", "-m", "bead edits the TOP region")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-one", "bead/bead-one")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	// Later honest work on a DIFFERENT region of the same file.
	os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("top: bead-one's change\n"+strings.Repeat("filler line\n", 10)+"bottom: later work\n"), 0644)
	run("add", ".")
	run("commit", "-m", "later work edits the BOTTOM region")

	// Documented guard target: this shape is SubsumptionLanded today (M's
	// top-region change re-applies as a no-op) — already identified; the
	// assertion pins the sub-case against a future per-file-path rule.
	outcome, err := gitutil.ContentSubsumedOutcome(dir, mergeSHA+"^1", mergeSHA, "spec/119-test")
	if err != nil {
		t.Fatalf("shape check: %v", err)
	}
	if outcome != gitutil.SubsumptionLanded {
		t.Fatalf("guard fixture shape drifted: want SubsumptionLanded, got %v", outcome)
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("later work on a DIFFERENT region of M's file must never read as a revert of M, got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the merge %q", landed.SHA, mergeSHA)
	}
}

// TestFindLandedMerge_CleanFullRemovalRefusesResidualFloor is spec 125
// AC-6's residual-floor guard: a pure, full, clean removal of M's paths
// with nothing replacing the content leaves the tip carrying NONE of M's
// introduced content — content-INDISTINGUISHABLE from a `git revert M` —
// so it REFUSES. This is the DELIBERATE false-negative floor R3 states
// (not a bug): any datum that accepted clean full removal would accept
// every real revert too.
func TestFindLandedMerge_CleanFullRemovalRefusesResidualFloor(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	run("rm", "bead-one.txt")
	run("commit", "-m", "remove bead-one's file cleanly and fully")

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("a clean full removal of M's content must refuse (the deliberate residual floor), got: %v", err)
	}
	if !strings.Contains(err.Error(), "no longer present") {
		t.Errorf("expected the refusal to name the content-absence honestly, got: %v", err)
	}
	var noEvidence *LandedMergeNoEvidence
	if errors.As(err, &noEvidence) {
		t.Error("the residual-floor refusal must not surface as LandedMergeNoEvidence (corroboration succeeded; the content is absent)")
	}
}

// TestFindLandedMerge_TrueRevertWithCoincidentalBlobRefuses is the
// G-BLOCK-1 end-to-end guard (RED against a rename-detection-ON
// RevertShape): merge M lands a distinctive blob at path X; M is genuinely
// reverted (X removed); unrelated later first-parent work recreates the
// IDENTICAL blob at a DIFFERENT path Y. A rename-detecting reverse un-apply
// would see "X renamed to Y", classify EVOLVED, and IDENTIFY the reverted
// merge — an unsafe false-positive. With rename detection OFF the merge
// correctly REFUSES.
func TestFindLandedMerge_TrueRevertWithCoincidentalBlobRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	blob := "line one\nline two\nline three\nline four\nline five\nline six\nline seven\nline eight\n"
	run("checkout", "-b", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "X.txt"), []byte(blob), 0644)
	run("add", ".")
	run("commit", "-m", "work bead-one: introduce X")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-one", "bead/bead-one")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	// Genuine revert of M (X removed).
	run("revert", "--no-edit", "-m", "1", mergeSHA)
	// Unrelated later work recreates the identical blob at a DIFFERENT path.
	os.WriteFile(filepath.Join(dir, "Y.txt"), []byte(blob), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated: identical blob at Y")

	// Shape-precondition: the FORWARD check still reads this as
	// CleanDivergence (the arm sub-classified) — pins the fixture drives
	// the revert-shape leg, not the already-green Landed/Conflict arms.
	outcome, err := gitutil.ContentSubsumedOutcome(dir, mergeSHA+"^1", mergeSHA, "spec/119-test")
	if err != nil {
		t.Fatalf("shape-precondition check: %v", err)
	}
	if outcome != gitutil.SubsumptionCleanDivergence {
		t.Fatalf("fixture shape drifted: want SubsumptionCleanDivergence, got %v", outcome)
	}

	_, err = FindLandedMerge(dir, "spec/119-test", "bead-one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("a true revert must refuse even with a coincidental identical blob at another path (G-BLOCK-1), got: %v", err)
	}
	var noEvidence *LandedMergeNoEvidence
	if errors.As(err, &noEvidence) {
		t.Error("the revert refusal must not surface as LandedMergeNoEvidence (corroboration succeeded; the content was reverted)")
	}
}

// TestFindLandedMerge_ConflictOutcomeStillIdentifies is spec 125 AC-6's
// Conflict-arm guard, asserted as BEHAVIOR (not a line-span): a
// SubsumptionConflict outcome still falls through to identify (the
// spec-121 F2-2r contract), and is NEVER routed through the new
// revert-shape sub-classification — spec 125 confines its change to the
// CleanDivergence arm, so the documented pre-existing
// Conflict-hides-revert residual is neither fixed nor worsened here.
func TestFindLandedMerge_ConflictOutcomeStillIdentifies(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	origSub := landedContentSubsumedFn
	t.Cleanup(func() { landedContentSubsumedFn = origSub })
	landedContentSubsumedFn = func(workdir, base, ref, target string) (gitutil.Subsumption, error) {
		return gitutil.SubsumptionConflict, nil
	}
	origRev := landedRevertShapeFn
	t.Cleanup(func() { landedRevertShapeFn = origRev })
	landedRevertShapeFn = func(workdir, mergeSHA, target string) (bool, error) {
		t.Error("a SubsumptionConflict outcome must never be routed through the revert-shape discrimination (spec 125's change is confined to the CleanDivergence arm)")
		return false, nil
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("a SubsumptionConflict outcome must still fall through to identify, got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the merge %q", landed.SHA, mergeSHA)
	}
}

// TestFindLandedMerge_RevertShapeInfraErrorPropagates is the plan-gate
// O2-1 pin at the CONSUMER: when the CleanDivergence arm's reverse check
// fails on infra (e.g. git < 2.38's unsupported --write-tree), the error
// PROPAGATES as a fail-closed infra refusal — never mapped to "identify"
// (a false-positive attestation on an undetermined result) and never
// classified as the not-found/reverted refusal. Both boolean polarities of
// the erroring seam are pinned, so an `if rev {...} else {identify}` that
// swallows the error fails this test in each direction.
func TestFindLandedMerge_RevertShapeInfraErrorPropagates(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")

	origSub := landedContentSubsumedFn
	t.Cleanup(func() { landedContentSubsumedFn = origSub })
	landedContentSubsumedFn = func(workdir, base, ref, target string) (gitutil.Subsumption, error) {
		return gitutil.SubsumptionCleanDivergence, nil
	}
	origRev := landedRevertShapeFn
	t.Cleanup(func() { landedRevertShapeFn = origRev })

	for _, staleBool := range []bool{false, true} {
		simulated := errors.New(`fatal: unknown option '--write-tree'`)
		landedRevertShapeFn = func(workdir, mergeSHA, target string) (bool, error) {
			return staleBool, simulated
		}

		landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
		if err == nil {
			t.Fatalf("staleBool=%v: an infra failure in the reverse check must propagate, got identification %+v", staleBool, landed)
		}
		if !errors.Is(err, simulated) {
			t.Errorf("staleBool=%v: expected the propagated error to wrap the infra failure, got: %v", staleBool, err)
		}
		if errors.Is(err, ErrLandedMergeNotFound) {
			t.Errorf("staleBool=%v: an UNDETERMINED reverse check must not be classified as not-found/reverted, got: %v", staleBool, err)
		}
	}
}

// TestFindLandedMerge_OctopusCandidateExcluded is spec 125 AC-6's
// octopus/parent guard: a >2-parent merge — even one carrying the exact
// bead-naming subject AND a binding that would corroborate its second
// parent — is EXCLUDED as a candidate, never run through corroboration or
// the revert/evolved discrimination (whose M^1/M^2 anchoring is only
// meaningful for a two-parent merge). Both discrimination seams fail the
// test if consulted.
func TestFindLandedMerge_OctopusCandidateExcluded(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	specTip := revParseIn(t, dir, "spec/119-test")

	run("checkout", "-b", "side-one")
	os.WriteFile(filepath.Join(dir, "side-one.txt"), []byte("s1\n"), 0644)
	run("add", ".")
	run("commit", "-m", "side one work")
	sideOne := revParseIn(t, dir, "side-one")

	run("checkout", "-b", "side-two", "spec/119-test")
	os.WriteFile(filepath.Join(dir, "side-two.txt"), []byte("s2\n"), 0644)
	run("add", ".")
	run("commit", "-m", "side two work")
	sideTwo := revParseIn(t, dir, "side-two")

	// Hand-craft a THREE-parent (octopus) merge with the bead-naming
	// subject, and fast-forward the spec branch onto it.
	run("checkout", "spec/119-test")
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	tree := git("write-tree")
	octopus := git("commit-tree", tree, "-p", specTip, "-p", sideOne, "-p", sideTwo, "-m", "Merge bead/octo-one")
	git("reset", "--hard", octopus)

	// A binding that WOULD corroborate the octopus's Parents[1] — so a
	// non-excluding impl (the pre-125 `len(m.Parents) < 2` filter) would
	// confirm the candidate and reach the discrimination, failing below.
	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(issueID string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_second_parent": sideOne}, nil
	}

	origSub := landedContentSubsumedFn
	t.Cleanup(func() { landedContentSubsumedFn = origSub })
	landedContentSubsumedFn = func(workdir, base, ref, target string) (gitutil.Subsumption, error) {
		t.Error("an octopus candidate must never be run through the forward content discrimination")
		return gitutil.SubsumptionLanded, nil
	}
	origRev := landedRevertShapeFn
	t.Cleanup(func() { landedRevertShapeFn = origRev })
	landedRevertShapeFn = func(workdir, mergeSHA, target string) (bool, error) {
		t.Error("an octopus candidate must never be run through the revert-shape discrimination")
		return false, nil
	}

	_, err := FindLandedMerge(dir, "spec/119-test", "octo-one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("an octopus candidate must be excluded (not-found), got: %v", err)
	}
	if sideTwo == "" { // sideTwo is fixture plumbing; keep it referenced
		t.Fatal("fixture: sideTwo tip missing")
	}
}

// TestLandedRevertShapeFnDefaultPinned is spec 125's anti-drift pointer
// pin (the netEffectLandedFn/AC-17 pattern): landedRevertShapeFn's
// production default IS gitutil.RevertShape, so the hermetic fixtures
// above provably exercise the real reverse un-apply primitive and the
// seam cannot be silently rewired off it.
func TestLandedRevertShapeFnDefaultPinned(t *testing.T) {
	if reflect.ValueOf(landedRevertShapeFn).Pointer() != reflect.ValueOf(gitutil.RevertShape).Pointer() {
		t.Fatal("landedRevertShapeFn must default to gitutil.RevertShape (spec 125 anti-drift: the CleanDivergence sub-classification must invoke the real reverse un-apply primitive)")
	}
}

// TestLandedBindingForBead_MalformedValuesTreatedAbsent is the spec 121
// final-review G2-1 provenance-gate unit: the landed-binding values are
// read from AGENT-WRITABLE bd metadata, so anything that is not a
// well-formed git object id (option-like, control bytes, a rev expression,
// too short/long, non-hex) makes the binding datum ABSENT — (nil, false) —
// never a struct carrying the hostile value onward.
func TestLandedBindingForBead_MalformedValuesTreatedAbsent(t *testing.T) {
	orig := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = orig })

	for _, hostile := range []string{
		"--upload-pack=/tmp/evil",    // git option injection
		"deadbee\x1b[2J\x07",         // terminal control bytes
		"HEAD~1",                     // rev expression
		"deadbeefdeadbeef:README.md", // rev expression (path form)
		"abc",                        // too short for an object id
		strings.Repeat("a", 65),      // too long
		"deadbeefzzzz",               // non-hex
	} {
		landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"mindspec_landed_merge_sha":     hostile,
				"mindspec_landed_second_parent": hostile,
			}, nil
		}
		if b, have := landedBindingForBead("bead-one"); have || b != nil {
			t.Errorf("malformed binding value %q must be treated as absent, got %+v", hostile, b)
		}
	}

	// Well-formed values (full and abbreviated hex) still bind.
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     "0123456789abcdef0123456789abcdef01234567",
			"mindspec_landed_second_parent": "0123456",
		}, nil
	}
	if b, have := landedBindingForBead("bead-one"); !have || b == nil {
		t.Error("a well-formed hex binding must remain available")
	}
}

// TestFindLandedMerge_HostileBindingNeverAGitOperand is G2-1's end-to-end
// pin: a crafted mindspec_landed_second_parent/merge_sha (option-like or
// control-byte-bearing) must (i) NEVER reach isAncestorFn — the git-operand
// seam — raw, (ii) never confirm the candidate (the bead does not close on
// a crafted binding: with every other leg unavailable this is the R5(c)
// no-evidence refusal), and (iii) never appear raw in the error render.
func TestFindLandedMerge_HostileBindingNeverAGitOperand(t *testing.T) {
	for _, hostile := range []string{
		"--upload-pack=/tmp/evil",
		"deadbee\x1b[2J\x07forged",
	} {
		t.Run(strings.Map(func(r rune) rune {
			if r < 0x20 || r > 0x7e {
				return '_'
			}
			return r
		}, hostile), func(t *testing.T) {
			dir, run := initLandedRepo(t, "119-test")
			mergeBead(t, run, dir, "bead-one", "spec/119-test")
			run("branch", "-D", "bead/bead-one") // no surviving-branch leg

			origBinding := landedBindingMetadataFn
			t.Cleanup(func() { landedBindingMetadataFn = origBinding })
			landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"mindspec_landed_merge_sha":     hostile,
					"mindspec_landed_second_parent": hostile,
				}, nil
			}

			origAnc := isAncestorFn
			t.Cleanup(func() { isAncestorFn = origAnc })
			var operands []string
			isAncestorFn = func(workdir, anc, desc string) (bool, error) {
				operands = append(operands, anc, desc)
				return origAnc(workdir, anc, desc)
			}

			_, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
			var noEvidence *LandedMergeNoEvidence
			if !errors.As(err, &noEvidence) {
				t.Fatalf("a crafted binding must be treated as ABSENT (no-evidence refusal, never a close), got: %v", err)
			}
			for _, op := range operands {
				if op == hostile {
					t.Fatalf("the crafted binding value reached isAncestorFn as a raw git operand: %q", op)
				}
			}
			if err != nil && strings.ContainsAny(err.Error(), "\x1b\x07") {
				t.Errorf("the error render carries raw control bytes: %q", err.Error())
			}
		})
	}
}

// TestFindLandedMerge_BindingContradictsDifferentMerge: a recorded binding
// naming a DIFFERENT merge SHA whose second parent neither matches nor is
// an ancestor of this candidate's is a CONTRADICTION, not silently
// ignored.
func TestFindLandedMerge_BindingContradictsDifferentMerge(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")

	run("checkout", "main")
	run("checkout", "-b", "unrelated")
	os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated work")
	unrelatedSHA := revParseIn(t, dir, "unrelated")

	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(issueID string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     "0000000000000000000000000000000000dead",
			"mindspec_landed_second_parent": unrelatedSHA,
		}, nil
	}

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected a contradicted binding to refuse, got: %v", err)
	}
	var noEvidence *LandedMergeNoEvidence
	if errors.As(err, &noEvidence) {
		t.Error("a CONTRADICTED binding must not surface as LandedMergeNoEvidence (that implies no datum was even available)")
	}
}

// ---------------------------------------------------------------------------
// Spec 125 Bead 3 — the exact-second-parent OWNERSHIP identity suite
// (R5, AC-2b/AC-2c/AC-2d/AC-2e/AC-2f + the G1-F2 binding-SHA path and the
// G2-2 parser-conservatism pins). Per the plan's RED discipline these are
// RED-against-the-WRONG-impl fixtures: each names its deviation target
// in-test (naive newest-first ancestor-consistent scan / newest-ancestor on
// no-exact-match / topology-only cache trust / newest-anchored content
// check / prefix-substring ownership), and passes ONLY against the
// exact-match-required, ownership-named, fail-closed mechanism.
// ---------------------------------------------------------------------------

// TestParseMergeSubjectBeadBranch pins the G2-2 CONSERVATIVE three-state
// parser contract: (branch, true) whenever ANY bead/… token is present —
// including unrecognized formats, which nominate the token they carry and
// are NEVER collapsed into the no-bead bucket — and ("", false) ONLY when
// genuinely no bead/… token appears (the true anonymous-subject case).
func TestParseMergeSubjectBeadBranch(t *testing.T) {
	cases := []struct {
		subject     string
		wantBranch  string
		wantPresent bool
	}{
		// gitutil.MergeInto's deterministic form.
		{"Merge bead/mindspec-x.1", "bead/mindspec-x.1", true},
		// git's default conflict-recovery forms (quoted target and bare).
		{"Merge branch 'bead/mindspec-x.1' into spec/125-foo", "bead/mindspec-x.1", true},
		{"Merge branch 'bead/mindspec-x.1' into 'spec/125-foo'", "bead/mindspec-x.1", true},
		{"Merge branch 'bead/mindspec-x.1'", "bead/mindspec-x.1", true},
		// Prefix-colliding IDs parse to their FULL token (AC-2f's substrate).
		{"Merge bead/mindspec-8nhe.12", "bead/mindspec-8nhe.12", true},
		{"Merge branch 'bead/mindspec-8nhe.12' into spec/125-foo", "bead/mindspec-8nhe.12", true},
		// Unrecognized formats carrying a bead/… token: PRESENT-and-named
		// (G2-2) — never ("", false).
		{"custom pipeline landed [bead/mindspec-z.9] artifacts", "bead/mindspec-z.9", true},
		{"Revert \"Merge bead/mindspec-z.9\"", "bead/mindspec-z.9", true},
		{"Merge remote-tracking branch 'origin/bead/mindspec-x.1'", "bead/mindspec-x.1", true},
		// Genuinely NO bead token: the anonymous-subject state.
		{"chore: commit remaining spec artifacts", "", false},
		{"Merge branch 'feature/no-bead-here'", "", false},
		{"land the payload work via custom ceremony", "", false},
	}
	for _, c := range cases {
		branch, present := parseMergeSubjectBeadBranch(c.subject)
		if present != c.wantPresent || branch != c.wantBranch {
			t.Errorf("parseMergeSubjectBeadBranch(%q) = (%q, %v), want (%q, %v)",
				c.subject, branch, present, c.wantBranch, c.wantPresent)
		}
	}
}

// mergeBeadDefaultSubject is mergeBead's sibling for spec 125's R5
// conflict-recovery shape: the merge commit carries git's DEFAULT
// recovery-form subject (`Merge branch 'bead/<id>' into <spec>`), written
// explicitly with -m for determinism across git builds — the exact shape
// the retired exact-subject scan could never match.
func mergeBeadDefaultSubject(t *testing.T, run func(args ...string), dir, beadID, specBranch string) {
	t.Helper()
	run("checkout", "-b", "bead/"+beadID)
	os.WriteFile(filepath.Join(dir, beadID+".txt"), []byte(beadID+"\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work "+beadID)
	run("checkout", specBranch)
	run("merge", "--no-ff", "-m", "Merge branch 'bead/"+beadID+"' into "+specBranch, "bead/"+beadID)
}

// TestFindLandedMerge_DefaultSubjectIdentified is the R5 core (RED on the
// spec-init SHA): a merge carrying git's DEFAULT conflict-recovery subject
// — never the exact `Merge bead/<id>` — whose second parent IS the bead's
// landed tip must be identified. Ownership comes from the parsed
// bead-branch name in the default subject; landed-ness from the surviving
// branch tip EQUALING the second parent. The pre-125 exact-subject gate
// finds no candidate here at all.
func TestFindLandedMerge_DefaultSubjectIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-one", "spec/119-test")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("a default-subject merge with an exact second-parent match must identify (R5), got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want %q", landed.SHA, mergeSHA)
	}
}

// TestFindLandedMerge_DescendantMergeNeverMisattributed is AC-2b (RED
// against the naive newest-first ancestor-consistent scan — the deviation
// target named by the AC): bead X lands, then a DESCENDANT bead Y branches
// off spec AFTER X and lands, BOTH with git's default recovery subject, so
// X's tip is an ancestor of Y's tip and both merges are subject-ambiguous
// to any non-parsing scan. FindLandedMerge(X) must return M_X (the
// exact-second-parent match), NEVER M_Y — a naive
// IsAncestor(X_tip, secondParent) newest-first scan returns M_Y here. And
// after a `git revert` of ONLY X's content, FindLandedMerge(X) REFUSES:
// the revert-leg reads M_X's content, not M_Y's (which would still be
// present and would mask the revert).
func TestFindLandedMerge_DescendantMergeNeverMisattributed(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-x", "spec/119-test")
	mX := revParseIn(t, dir, "spec/119-test")
	xTip := revParseIn(t, dir, "bead/bead-x")

	// Y branches off spec AFTER X landed → X_tip is an ancestor of Y_tip.
	mergeBeadDefaultSubject(t, run, dir, "bead-y", "spec/119-test")
	mY := revParseIn(t, dir, "spec/119-test")
	if mX == mY {
		t.Fatal("fixture: expected two distinct merges")
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if err != nil {
		t.Fatalf("X's own exact match exists and its surviving branch tip equals it — must identify, got: %v", err)
	}
	if landed.SHA != mX {
		t.Errorf("SHA = %q, want M_X %q — returning M_Y %q is the naive ancestor-consistent misattribution", landed.SHA, mX, mY)
	}
	if landed.SecondParent != xTip {
		t.Errorf("SecondParent = %q, want X's tip %q", landed.SecondParent, xTip)
	}

	// Revert ONLY X's content: the revert-leg must read M_X (refuse), not
	// M_Y (whose content is still present — identification would mask the
	// revert).
	run("revert", "--no-edit", "-m", "1", mX)
	_, err = FindLandedMerge(dir, "spec/119-test", "bead-x")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("after reverting only X's content, FindLandedMerge(X) must refuse, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no longer present") {
		t.Errorf("expected the revert-leg refusal (anchored on M_X), got: %v", err)
	}
	// Y is untouched by X's revert and must still identify.
	landedY, err := FindLandedMerge(dir, "spec/119-test", "bead-y")
	if err != nil || landedY.SHA != mY {
		t.Errorf("Y must still identify its own merge %q, got %+v err=%v", mY, landedY, err)
	}
}

// TestFindLandedMerge_NoExactMatchAncestorPanelRefuses is AC-2c's first
// half (RED against BOTH tempting impls it names): X lands (default
// subject), then descendant Y lands; X's branch is DELETED, X has NO
// binding, and the only datum is a registered panel's reviewed_head_sha
// recording an EARLIER head of X — an ANCESTOR of X's landed tip, EQUAL to
// no merge's second parent. FindLandedMerge(X) MUST REFUSE:
//   - a newest-first ancestor-consistent scan (subject-blind) returns the
//     newest merge M_Y — the reopened misattribution;
//   - the pre-125 ancestor-TOLERANT reviewed_head_sha leg confirms M_X
//     from the ancestor datum — a positive identification with NO exact
//     match, which R5 forbids.
func TestFindLandedMerge_NoExactMatchAncestorPanelRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")

	// X: two commits — the panel reviews the EARLIER head, the merge lands
	// the LATER tip.
	run("checkout", "-b", "bead/bead-x")
	os.WriteFile(filepath.Join(dir, "x1.txt"), []byte("x1\n"), 0644)
	run("add", ".")
	run("commit", "-m", "x work 1")
	xEarly := revParseIn(t, dir, "bead/bead-x")
	os.WriteFile(filepath.Join(dir, "x2.txt"), []byte("x2\n"), 0644)
	run("add", ".")
	run("commit", "-m", "x work 2")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge branch 'bead/bead-x' into spec/119-test", "bead/bead-x")

	// Descendant Y lands after X — the newest ancestor-consistent decoy.
	mergeBeadDefaultSubject(t, run, dir, "bead-y", "spec/119-test")
	mY := revParseIn(t, dir, "spec/119-test")

	run("branch", "-D", "bead/bead-x")

	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })
	beadID := "bead-x"
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{Panel: panel.Panel{BeadID: &beadID, ReviewedHeadSHA: xEarly}}}
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if err == nil {
		t.Fatalf("an ancestor-only reviewed_head_sha must NEVER positively identify (got %+v; M_Y is %s) — exact-equality corroboration is required", landed, mY)
	}
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected a fail-closed ErrLandedMergeNotFound refusal, got: %v", err)
	}
}

// TestFindLandedMerge_ForgedBindingNoRealMergeRefuses is AC-2c's second
// half (the G1-3 forgery): a binding on X whose recorded SHAs are
// well-formed but match NO real merge on spec — neither the merge-SHA nor
// the second-parent resolves to an exact match — is DISCARDED, never
// followed: identification refuses (fail-closed), it does not fall back to
// trusting the cache or to any ancestor-consistent pick.
func TestFindLandedMerge_ForgedBindingNoRealMergeRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-x", "spec/119-test")
	run("branch", "-D", "bead/bead-x")

	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     "1111111111111111111111111111111111111111",
			"mindspec_landed_second_parent": "2222222222222222222222222222222222222222",
		}, nil
	}

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("a forged binding pointing at no real exact merge must be discarded and identification must refuse, got: %v", err)
	}
}

// TestFindLandedMerge_BindingAtOtherBeadsMergeRefuses is AC-2d (RED
// against the topology-only cache-trust impl it names): X and Z each
// landed a REAL exact merge; X carries a stale/forged binding recording
// Z's merge SHA and Z's landed tip. The binding IS git-corroborated as a
// real two-parent exact merge — topology passes — but that merge's subject
// names bead/Z, not bead/X: the OWNERSHIP check discards it, and
// FindLandedMerge(X) REFUSES rather than returning Z's merge as X's. A
// trust-the-binding's-SHA-blindly impl (topology corroboration without the
// subject-ownership check) returns Z's merge here.
func TestFindLandedMerge_BindingAtOtherBeadsMergeRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-x", "spec/119-test")
	mergeBead(t, run, dir, "bead-z", "spec/119-test")
	mZ := revParseIn(t, dir, "spec/119-test")
	zTip := revParseIn(t, dir, "bead/bead-z")
	run("branch", "-D", "bead/bead-x")
	run("branch", "-D", "bead/bead-z")

	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(issueID string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     mZ,
			"mindspec_landed_second_parent": zTip,
		}, nil
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if err == nil {
		t.Fatalf("a cache pointing at ANOTHER bead's real merge must be discarded on ownership, got identification %+v (Z's merge is %s)", landed, mZ)
	}
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected a fail-closed refusal, got: %v", err)
	}
}

// TestFindLandedMerge_RemergeMaskedRevertRefusesOldestAnchor is AC-2e (RED
// against a newest-anchored impl for EITHER parameter of the R3 check):
// bead X lands (M₁, second parent = X_tip); X's content is REVERTED on
// spec; then the SAME second parent is re-merged as an EMPTY no-op merge
// M₂ (no content reintroduced) — so X's landed content is ABSENT at the
// tip, but M₂'s own first parent is the POST-REVERT state. The
// landed-vs-reverted check MUST be Requirement 3's three-way anchored on
// the OLDEST merge M₁ — ContentSubsumedOutcome(base=M₁^1, ref=M₁,
// target=tip) → CleanDivergence → RevertShape → REFUSE. An impl anchoring
// base or theirs on M₂ reads "no change" (SubsumptionLanded) and
// mis-attests the reverted bead. Recording wrappers pin BOTH parameters.
func TestFindLandedMerge_RemergeMaskedRevertRefusesOldestAnchor(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/bead-x")
	os.WriteFile(filepath.Join(dir, "payload.txt"), []byte("the payload\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work bead-x")
	xTip := revParseIn(t, dir, "bead/bead-x")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-x", "bead/bead-x")
	m1 := revParseIn(t, dir, "spec/119-test")
	m1FirstParent := revParseIn(t, dir, m1+"^1")

	// Revert X's content, then re-merge the SAME second parent as an
	// EMPTY merge (tree = the post-revert tree; commitResolvedMerge
	// fast-forwards spec onto it).
	run("revert", "--no-edit", "-m", "1", m1)
	postRevertTip := revParseIn(t, dir, "spec/119-test")
	m2 := commitResolvedMerge(t, dir, postRevertTip, xTip, "Merge bead/bead-x")

	// Recording wrappers: the R3 check must be anchored on M₁ for BOTH
	// the base and the theirs/ref parameter, and the revert-shape
	// sub-classification on M₁ too — never M₂ (the newest).
	var subBase, subRef string
	origSub := landedContentSubsumedFn
	t.Cleanup(func() { landedContentSubsumedFn = origSub })
	landedContentSubsumedFn = func(workdir, base, ref, target string) (gitutil.Subsumption, error) {
		subBase, subRef = base, ref
		return origSub(workdir, base, ref, target)
	}
	var revAnchor string
	origRev := landedRevertShapeFn
	t.Cleanup(func() { landedRevertShapeFn = origRev })
	landedRevertShapeFn = func(workdir, mergeSHA, target string) (bool, error) {
		revAnchor = mergeSHA
		return origRev(workdir, mergeSHA, target)
	}

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("a revert-then-empty-re-merge (content ABSENT at tip) must classify REVERTED and refuse, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no longer present") {
		t.Errorf("expected the reverted-content refusal, got: %v", err)
	}
	if subRef != m1 {
		t.Errorf("R3 theirs/ref anchored on %q, want the OLDEST merge M₁ %q (newest M₂ is %q — the G2-R4-B1 masked-revert)", subRef, m1, m2)
	}
	if subBase != m1FirstParent {
		t.Errorf("R3 base anchored on %q, want M₁^1 %q (M₂^1 is the post-revert state %q)", subBase, m1FirstParent, postRevertTip)
	}
	if revAnchor != m1 {
		t.Errorf("revert-shape anchored on %q, want the OLDEST merge M₁ %q", revAnchor, m1)
	}
}

// TestFindLandedMerge_RemergeReintroducedNewestNamesSHA is AC-2e's
// positive direction: a revert-then-re-merge that DOES reintroduce X's
// content (the tip carries it again) identifies — and the returned
// *LandedMerge.SHA is the NEWEST same-second-parent exact match M₂
// (nearest the tip), never the oldest, while the content-check still ran
// against M₁ (single anchor, asserted by the masked-revert sibling above).
func TestFindLandedMerge_RemergeReintroducedNewestNamesSHA(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/bead-x")
	os.WriteFile(filepath.Join(dir, "payload.txt"), []byte("the payload\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work bead-x")
	xTip := revParseIn(t, dir, "bead/bead-x")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-x", "bead/bead-x")
	m1 := revParseIn(t, dir, "spec/119-test")

	run("revert", "--no-edit", "-m", "1", m1)
	postRevertTip := revParseIn(t, dir, "spec/119-test")
	// The re-merge REINTRODUCES the content (stage it back before
	// commitResolvedMerge snapshots the index as the merge tree).
	os.WriteFile(filepath.Join(dir, "payload.txt"), []byte("the payload\n"), 0644)
	run("add", ".")
	m2 := commitResolvedMerge(t, dir, postRevertTip, xTip, "Merge bead/bead-x")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if err != nil {
		t.Fatalf("a re-merge that reintroduced the content must identify, got: %v", err)
	}
	if landed.SHA != m2 {
		t.Errorf("SHA = %q, want the NEWEST same-second-parent match M₂ %q (M₁ is %q)", landed.SHA, m2, m1)
	}
	if landed.SecondParent != xTip {
		t.Errorf("SecondParent = %q, want %q", landed.SecondParent, xTip)
	}
}

// TestFindLandedMerge_PrefixCollisionFailsSafe is AC-2f (RED against a
// HasPrefix/Contains ownership impl — the deviation target): beads
// mindspec-8nhe.1 and mindspec-8nhe.12 each land a real exact merge (the
// .12 merge NEWER, so a prefix match would evaluate it first for .1 and
// either misattribute or spuriously contradict). Full branch-name EQUALITY
// resolves each bead ONLY to its own merge.
func TestFindLandedMerge_PrefixCollisionFailsSafe(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "mindspec-8nhe.1", "spec/119-test")
	m1 := revParseIn(t, dir, "spec/119-test")
	mergeBead(t, run, dir, "mindspec-8nhe.12", "spec/119-test")
	m12 := revParseIn(t, dir, "spec/119-test")

	landed1, err := FindLandedMerge(dir, "spec/119-test", "mindspec-8nhe.1")
	if err != nil {
		t.Fatalf("mindspec-8nhe.1 must resolve its own merge (a prefix impl evaluates .12's newer merge and fails), got: %v", err)
	}
	if landed1.SHA != m1 {
		t.Errorf("mindspec-8nhe.1 SHA = %q, want its own merge %q (cross-attributing %q is the prefix collision)", landed1.SHA, m1, m12)
	}
	landed12, err := FindLandedMerge(dir, "spec/119-test", "mindspec-8nhe.12")
	if err != nil {
		t.Fatalf("mindspec-8nhe.12 must resolve its own merge, got: %v", err)
	}
	if landed12.SHA != m12 {
		t.Errorf("mindspec-8nhe.12 SHA = %q, want %q", landed12.SHA, m12)
	}
}

// TestFindLandedMerge_PrefixCollisionUnmatchedRefuses is AC-2f's second
// assertion: a colliding-but-UNMATCHED bead (only mindspec-8nhe.12's merge
// exists; mindspec-8nhe.1 never landed) REFUSES rather than
// false-attributing the longer-named sibling's merge — a collision fails
// SAFE (no positive ID), never a false attribution.
func TestFindLandedMerge_PrefixCollisionUnmatchedRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "mindspec-8nhe.12", "spec/119-test")

	landed, err := FindLandedMerge(dir, "spec/119-test", "mindspec-8nhe.1")
	if err == nil {
		t.Fatalf("a bead with no landed merge must refuse — got %+v (a Contains/HasPrefix ownership impl attributes mindspec-8nhe.12's merge)", landed)
	}
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound, got: %v", err)
	}
}

// TestFindLandedMerge_AnonymousSubjectBindingRefuses pins the G-1
// BLOCKING fix (codex final-review): a merge whose subject names NO bead
// (a wholly-custom subject) is NOT automatically identifiable, EVEN with a
// valid-looking complete-time binding pointing at that real exact merge.
// The binding git-corroborates only that the merge is REAL with this exact
// second parent — NOT that it is THIS bead's — so admitting it would make
// the agent-writable binding an independent OWNERSHIP authority (a
// metadata-forge, below the git-history threat boundary). The automatic
// path FAILS CLOSED; the audited `mindspec reattest` (Bead 4) is the
// correct recovery for an anonymous subject. RED against the removed
// binding-SHA-for-anonymous impl.
func TestFindLandedMerge_AnonymousSubjectBindingRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/bead-x")
	os.WriteFile(filepath.Join(dir, "payload.txt"), []byte("the payload\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work bead-x")
	xTip := revParseIn(t, dir, "bead/bead-x")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "land the payload work via custom ceremony", "bead/bead-x")
	mergeSHA := revParseIn(t, dir, "spec/119-test")
	run("branch", "-D", "bead/bead-x")

	// Sanity: the subject genuinely names no bead (the parser's
	// present==false state), so the subject-scan path cannot own it.
	if _, present := parseMergeSubjectBeadBranch("land the payload work via custom ceremony"); present {
		t.Fatal("fixture: the custom subject must name NO bead")
	}

	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     mergeSHA,
			"mindspec_landed_second_parent": xTip,
		}, nil
	}

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("an anonymous-subject merge must NOT be auto-identified on the binding alone (G-1) — expected a fail-closed refusal, got: %v", err)
	}
}

// TestFindLandedMerge_ForgedBindingAtRealAnonymousMergeRefuses is the G-1
// exploit pin (RED against the removed binding-SHA-for-anonymous impl):
// bead X NEVER landed (no merge of X's own tip exists), but a party who
// can WRITE bd metadata (a metadata-forge — EASIER than a commit-forge,
// so BELOW the documented git-history threat boundary) plants a binding on
// X pointing at some OTHER work's real anonymous-subject merge. The old
// binding-SHA path would positively identify that unrelated merge as X's
// landed merge — an unsafe false-positive. FindLandedMerge MUST refuse.
func TestFindLandedMerge_ForgedBindingAtRealAnonymousMergeRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")

	// Some OTHER work lands via a merge with a wholly-custom subject
	// (naming no bead) — a real two-parent merge, not X's.
	run("checkout", "-b", "other-work")
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other payload\n"), 0644)
	run("add", ".")
	run("commit", "-m", "other work")
	otherTip := revParseIn(t, dir, "other-work")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "custom ceremony landing unrelated work", "other-work")
	anonMerge := revParseIn(t, dir, "spec/119-test")

	// bead X never even branched. Forge a binding on X pointing at the
	// real anonymous merge above.
	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     anonMerge,
			"mindspec_landed_second_parent": otherTip,
		}, nil
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if err == nil {
		t.Fatalf("a forged binding on a never-landed bead pointing at a real anonymous merge must be refused (G-1), got identification %+v", landed)
	}
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected a fail-closed ErrLandedMergeNotFound refusal, got: %v", err)
	}
}

// TestFindLandedMerge_BindingUnrecognizedOtherBeadTokenRefuses is the
// G2-2 REJECT direction: a binding-SHA candidate whose merge subject
// carries a DIFFERENT bead's bead/… token in a format the parser does not
// fully recognize is DISCARDED on ownership — the parser reports it
// PRESENT-and-named (nominating bead/bead-z), never the no-bead state, so
// it can never slip through the names-no-bead binding exception. RED
// against a parser that collapses unrecognized formats into ("", false).
func TestFindLandedMerge_BindingUnrecognizedOtherBeadTokenRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/bead-z")
	os.WriteFile(filepath.Join(dir, "z.txt"), []byte("z\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work bead-z")
	zTip := revParseIn(t, dir, "bead/bead-z")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "custom pipeline landed [bead/bead-z] artifacts", "bead/bead-z")
	mZ := revParseIn(t, dir, "spec/119-test")
	run("branch", "-D", "bead/bead-z")

	origBinding := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = origBinding })
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"mindspec_landed_merge_sha":     mZ,
			"mindspec_landed_second_parent": zTip,
		}, nil
	}

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-x")
	if err == nil {
		t.Fatalf("a binding at a merge whose unrecognized-format subject names ANOTHER bead must be discarded on ownership, got %+v", landed)
	}
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected a fail-closed refusal, got: %v", err)
	}
}

// TestFindLandedMerge_BindingPairInconsistentRefuses is the spec 125
// final-review FIX-2a pin (RED against the pre-fix OR corroboration):
// the binding's two keys are written together from ONE real merge, so
// every PRESENT key must agree with the SAME real merge — a
// present-but-contradictory value in EITHER key is a fail-closed
// contradiction, never ignored because the other key happens to match.
func TestFindLandedMerge_BindingPairInconsistentRefuses(t *testing.T) {
	t.Run("mergeSHA-matches-but-secondParent-contradicts", func(t *testing.T) {
		// The pre-fix hole: binding.mergeSHA names the real merge, so the
		// OR confirmed — even though the recorded second parent
		// CONTRADICTS that same merge's real second parent.
		dir, run := initLandedRepo(t, "119-test")
		mergeBead(t, run, dir, "bead-one", "spec/119-test")
		mergeSHA := revParseIn(t, dir, "spec/119-test")
		run("branch", "-D", "bead/bead-one")

		origBinding := landedBindingMetadataFn
		t.Cleanup(func() { landedBindingMetadataFn = origBinding })
		landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"mindspec_landed_merge_sha":     mergeSHA,
				"mindspec_landed_second_parent": "cccccccccccccccccccccccccccccccccccccccc",
			}, nil
		}

		landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
		if err == nil {
			t.Fatalf("a matching merge SHA must not outvote a CONTRADICTORY recorded second parent (the OR hole), got %+v", landed)
		}
		if !errors.Is(err, ErrLandedMergeNotFound) {
			t.Fatalf("expected a fail-closed refusal, got: %v", err)
		}
		var noEvidence *LandedMergeNoEvidence
		if errors.As(err, &noEvidence) {
			t.Error("a CONTRADICTED binding must not surface as LandedMergeNoEvidence (a datum WAS available — it contradicted)")
		}
	})

	t.Run("secondParent-matches-but-mergeSHA-names-no-merge", func(t *testing.T) {
		// The symmetric hole: binding.secondParent equals the real second
		// parent, but the recorded merge SHA names NO real owned merge —
		// pair-inconsistent, refused (the pre-fix OR confirmed on the
		// second-parent leg alone).
		dir, run := initLandedRepo(t, "119-test")
		mergeBead(t, run, dir, "bead-one", "spec/119-test")
		secondParent := revParseIn(t, dir, "bead/bead-one")
		run("branch", "-D", "bead/bead-one")

		origBinding := landedBindingMetadataFn
		t.Cleanup(func() { landedBindingMetadataFn = origBinding })
		landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"mindspec_landed_merge_sha":     "1111111111111111111111111111111111111111",
				"mindspec_landed_second_parent": secondParent,
			}, nil
		}

		landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
		if err == nil {
			t.Fatalf("a matching second parent must not outvote a merge SHA naming NO real merge, got %+v", landed)
		}
		if !errors.Is(err, ErrLandedMergeNotFound) {
			t.Fatalf("expected a fail-closed refusal, got: %v", err)
		}
	})
}

// TestFindLandedMerge_AmbiguousOwnedSecondParentsRefuses is the spec 125
// final-review FIX-2b pin (RED against the pre-fix newest-first pick):
// two owned merges name the SAME bead but carry DIFFERENT second parents
// — genuine ambiguity about which landing is the bead's tip, the same
// shape ReattestLandedMerge refuses as ReattestStateAmbiguous. The
// surviving branch tip EQUALS the newest merge's second parent, so the
// pre-fix impl positively identified the newest candidate; the fix
// FAILS CLOSED with a *LandedMergeNoEvidence naming the conflict.
func TestFindLandedMerge_AmbiguousOwnedSecondParentsRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	// The branch advances and is merged AGAIN — a second landing with a
	// DIFFERENT second parent.
	run("checkout", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "more.txt"), []byte("more\n"), 0644)
	run("add", ".")
	run("commit", "-m", "more work")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-one", "bead/bead-one")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err == nil {
		t.Fatalf("owned merges with DIFFERENT second parents must fail closed, got %+v (the pre-fix impl silently evaluated the newest)", landed)
	}
	var noEvidence *LandedMergeNoEvidence
	if !errors.As(err, &noEvidence) {
		t.Fatalf("expected *LandedMergeNoEvidence on genuine ownership ambiguity, got %v", err)
	}
	if noEvidence.ConflictingSecondParent == "" {
		t.Error("the ambiguity refusal must name the conflicting second parent")
	}
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Error("the ambiguity refusal must still satisfy errors.Is(err, ErrLandedMergeNotFound)")
	}
}

// TestFindLandedMerge_HostilePanelSHAEscapedInRefusal is the spec 125
// final-review FIX-5 pin for the landed.go site: reviewed_head_sha comes
// from the AGENT-WRITABLE panel.json, and the panel-contradiction refusal
// interpolates it into an error that reaches terminal output — a
// control-byte/ANSI value must be rendered escaped (like the sibling
// binding-contradiction branch), never raw.
func TestFindLandedMerge_HostilePanelSHAEscapedInRefusal(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")

	hostile := "deadbee\x1b[2J\x07f0rged"
	beadID := "bead-one"
	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{
			Dir:   "/fake/review/one",
			Panel: panel.Panel{BeadID: &beadID, ReviewedHeadSHA: hostile},
		}}
	}

	_, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("a contradicting panel SHA must refuse, got %v", err)
	}
	if strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Errorf("the refusal renders the hostile reviewed_head_sha RAW (terminal-injectable): %q", err.Error())
	}
}

// TestLandedBindingMetadataFnDefaultPinned is the spec 125 F3-2 pointer
// pin: the PRE-EXISTING lifecycle read seam landedBindingMetadataFn
// defaults to the real bead.GetMetadata (captured by TestMain before the
// hermetic stub is installed), so the binding-read gate provably exercises
// the real bd read path and cannot be silently rewired off it.
func TestLandedBindingMetadataFnDefaultPinned(t *testing.T) {
	if landedBindingMetadataFnDefault == nil {
		t.Fatal("TestMain did not capture the production default")
	}
	if reflect.ValueOf(landedBindingMetadataFnDefault).Pointer() != reflect.ValueOf(bead.GetMetadata).Pointer() {
		t.Fatal("landedBindingMetadataFn must default to bead.GetMetadata (spec 125 F3-2 anti-drift: the landed-binding read gate must exercise the real bd read path)")
	}
}
