package lifecycle

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
func TestMain(m *testing.M) {
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

	// The merge must genuinely CONFLICT (raw exec — run() would t.Fatal).
	cmd := exec.Command("git", "-C", dir, "merge", "--no-ff", "-m", "Merge bead/bead-one", "bead/bead-one")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("test setup: expected a real merge conflict, merge succeeded: %s", out)
	}
	// Resolve honestly (content matching NEITHER parent) and commit the
	// merge; --no-edit preserves MERGE_MSG's deterministic
	// "Merge bead/bead-one" subject, exactly as an operator resolving a
	// gitutil.MergeInto conflict in the spec worktree would.
	os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("resolved: spec side + bead side\n"), 0644)
	run("add", ".")
	run("commit", "--no-edit")
	mergeSHA := revParseIn(t, dir, "spec/119-test")

	// Advance the spec branch with unrelated later work: tip != M, so the
	// R5(d) subsumption evaluates a genuine three-way, not ours==theirs.
	os.WriteFile(filepath.Join(dir, "later.txt"), []byte("later\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated later work")

	landed, err := FindLandedMerge(dir, "spec/119-test", "bead-one")
	if err != nil {
		t.Fatalf("an honestly-conflict-resolved merge must reconcile (R5(d) must not over-refuse it), got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("SHA = %q, want the conflict-resolution merge %q", landed.SHA, mergeSHA)
	}
	if _, ok, err := MergedUnclosed(dir, "spec/119-test", "bead-one"); err != nil || !ok {
		t.Errorf("expected merged-unclosed (ok=true, err=nil), got ok=%v err=%v", ok, err)
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
