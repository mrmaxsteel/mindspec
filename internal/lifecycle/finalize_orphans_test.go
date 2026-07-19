package lifecycle

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// stubFinalizeOrphanSeams installs the finalize-orphan predicate's
// injectable seams for a single test and restores them on cleanup.
func stubFinalizeOrphanSeams(t *testing.T,
	branches []string, branchesErr error,
	commitCount int, commitCountErr error,
	diffStat string, diffStatErr error,
	fileAtRef []byte, fileAtRefErr error,
) {
	t.Helper()
	origBranches := localBranchRefsFn
	origCommitCount := finalizeOrphanCommitCountFn
	origDiffStat := finalizeOrphanDiffStatFn
	origNetEffect := finalizeOrphanNetEffectFn
	origFileAtRef := fileAtRefFn
	origRevParseRef := revParseRefFn
	t.Cleanup(func() {
		localBranchRefsFn = origBranches
		finalizeOrphanCommitCountFn = origCommitCount
		finalizeOrphanDiffStatFn = origDiffStat
		finalizeOrphanNetEffectFn = origNetEffect
		fileAtRefFn = origFileAtRef
		revParseRefFn = origRevParseRef
	})

	localBranchRefsFn = func(workdir string) ([]string, error) { return branches, branchesErr }
	finalizeOrphanCommitCountFn = func(workdir, base, head string) (int, error) { return commitCount, commitCountErr }
	finalizeOrphanDiffStatFn = func(workdir, base, head string) (string, error) { return diffStat, diffStatErr }
	// Default the net-effect confirmation to "NOT landed" so the
	// unmerged-carrier tests keep flagging; the landed/error tests
	// override this seam directly.
	finalizeOrphanNetEffectFn = func(workdir, ref, target string) (bool, error) { return false, nil }
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) { return fileAtRef, fileAtRefErr }
	// Default origin/main as resolvable (the common remote workflow); the
	// no-remote-fallback tests override this seam directly.
	revParseRefFn = func(workdir, ref string) (string, error) { return "deadbeefcafe", nil }
}

// (a) an outstanding chore/finalize-<specID> branch is flagged, with stats
// computed against origin/main (never local main — the seams below prove
// the CALL args, not just the values).
func TestFindOutstandingFinalizeBranches_Flagged(t *testing.T) {
	var gotCountBase, gotDiffBase string
	stubFinalizeOrphanSeams(t,
		[]string{"main", "spec/010-test", "chore/finalize-010-test"}, nil,
		3, nil,
		"2 files changed", nil,
		nil, nil,
	)
	// Wrap the count/diff seams to capture the base arg actually passed.
	origCount := finalizeOrphanCommitCountFn
	finalizeOrphanCommitCountFn = func(workdir, base, head string) (int, error) {
		gotCountBase = base
		return origCount(workdir, base, head)
	}
	origDiff := finalizeOrphanDiffStatFn
	finalizeOrphanDiffStatFn = func(workdir, base, head string) (string, error) {
		gotDiffBase = base
		return origDiff(workdir, base, head)
	}

	orphans, err := FindOutstandingFinalizeBranches(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %+v", len(orphans), orphans)
	}
	o := orphans[0]
	if o.Kind != "finalize_branch" {
		t.Errorf("Kind = %q, want finalize_branch", o.Kind)
	}
	if o.SpecID != "010-test" {
		t.Errorf("SpecID = %q, want 010-test", o.SpecID)
	}
	if o.Branch != "chore/finalize-010-test" {
		t.Errorf("Branch = %q, want chore/finalize-010-test", o.Branch)
	}
	if o.CommitCount != 3 {
		t.Errorf("CommitCount = %d, want 3", o.CommitCount)
	}
	if o.DiffStat != "2 files changed" {
		t.Errorf("DiffStat = %q, want %q", o.DiffStat, "2 files changed")
	}
	if gotCountBase != "origin/main" {
		t.Errorf("CommitCount base = %q, want origin/main (never local main)", gotCountBase)
	}
	if gotDiffBase != "origin/main" {
		t.Errorf("DiffStat base = %q, want origin/main (never local main)", gotDiffBase)
	}
	if o.RecoveryCommand() == "" {
		t.Error("RecoveryCommand must not be empty")
	}
}

// (b) no chore/finalize-* branch present → no findings.
func TestFindOutstandingFinalizeBranches_Healthy(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		[]string{"main", "spec/010-test", "bead/mindspec-x.1"}, nil,
		0, nil, "", nil, nil, nil,
	)
	orphans, err := FindOutstandingFinalizeBranches(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("expected no orphans, got %+v", orphans)
	}
}

// (b2) G1 (spec 119 final-review) / spec 121 R4: a carrier branch whose
// content is already net-effect LANDED on origin/main (the benign
// merged-but-undeleted residue finalizeOrphanedSpecBranch deliberately
// leaves behind on success, OR a squash-merged carrier — the squash blind
// spot this bead closes) must NOT be flagged, and the check must be asked
// about origin/main (never local main).
func TestFindOutstandingFinalizeBranches_MergedCarrierNotFlagged(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		[]string{"main", "chore/finalize-010-test"}, nil,
		0, nil, "", nil, nil, nil,
	)
	var gotRef, gotTarget string
	finalizeOrphanNetEffectFn = func(workdir, ref, target string) (bool, error) {
		gotRef, gotTarget = ref, target
		return true, nil // landed
	}

	orphans, err := FindOutstandingFinalizeBranches(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("a landed carrier must NOT be flagged, got %+v", orphans)
	}
	if gotRef != "chore/finalize-010-test" {
		t.Errorf("NetEffectLanded ref = %q, want the carrier branch", gotRef)
	}
	if gotTarget != "origin/main" {
		t.Errorf("NetEffectLanded target = %q, want origin/main (never local main)", gotTarget)
	}
}

// (b3) G1 / spec 121 R4: when a carrier's net-effect landed state CANNOT be
// determined, the branch is never asserted "unmerged" from absence of
// proof — it is skipped and the error is returned. A later provable orphan
// in the same list still survives (the ScanOrphanedClosedBeads mixed-list
// contract).
func TestFindOutstandingFinalizeBranches_NetEffectErrorNotAssertedUnmerged(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		[]string{"chore/finalize-010-err", "chore/finalize-011-real"}, nil,
		2, nil, "1 file changed", nil, nil, nil,
	)
	finalizeOrphanNetEffectFn = func(workdir, ref, target string) (bool, error) {
		if ref == "chore/finalize-010-err" {
			return false, errors.New("simulated missing origin/main")
		}
		return false, nil // 011-real is provably unmerged
	}

	orphans, err := FindOutstandingFinalizeBranches(".")
	if err == nil {
		t.Fatal("expected the net-effect-check error to be returned, got nil")
	}
	for _, o := range orphans {
		if o.Branch == "chore/finalize-010-err" {
			t.Errorf("the net-effect-error branch must NOT be asserted unmerged, got %+v", o)
		}
	}
	if len(orphans) != 1 || orphans[0].Branch != "chore/finalize-011-real" {
		t.Errorf("the later provable orphan must survive the earlier net-effect-check error, got %+v", orphans)
	}
}

// (c) a local-branch enumeration failure propagates as an error.
func TestFindOutstandingFinalizeBranches_PropagatesListError(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, errors.New("simulated git failure"),
		0, nil, "", nil, nil, nil,
	)
	if _, err := FindOutstandingFinalizeBranches("."); err == nil {
		t.Fatal("expected a propagated error on branch-listing failure, got nil")
	}
}

// (d) live-closed epic + main's committed export still shows it open →
// flagged, naming the divergence.
func TestStaleTrackerOnMain_Flagged(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-1","status":"open"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected a finding, got nil")
	}
	if o.Kind != "stale_tracker" {
		t.Errorf("Kind = %q, want stale_tracker", o.Kind)
	}
	if o.SpecID != "010-test" {
		t.Errorf("SpecID = %q, want 010-test", o.SpecID)
	}
	if o.RecoveryCommand() != "mindspec impl approve 010-test" {
		t.Errorf("RecoveryCommand = %q, want %q", o.RecoveryCommand(), "mindspec impl approve 010-test")
	}
}

// (e) agreement (main's export already shows closed) → no finding.
func TestStaleTrackerOnMain_HealthyAgreement(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-1","status":"closed"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != nil {
		t.Errorf("expected no finding on agreement, got %+v", o)
	}
}

// (f) live NOT closed → never a finding, regardless of main's content
// (this predicate only ever fires on the "reverted close" signature).
func TestStaleTrackerOnMain_LiveNotClosed(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-1","status":"open"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != nil {
		t.Errorf("expected no finding when live epic is not closed, got %+v", o)
	}
}

// (g) epic absent from main's committed export → no finding (not this
// predicate's concern; e.g. a brand-new epic never yet exported to main).
func TestStaleTrackerOnMain_EpicAbsentFromMain(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-other","status":"open"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != nil {
		t.Errorf("expected no finding when epic is absent from main's export, got %+v", o)
	}
}

// (h) a genuine git-read failure propagates (distinguished from "no
// finding").
func TestStaleTrackerOnMain_PropagatesReadError(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		nil, errors.New("simulated git show failure"),
	)
	if _, err := StaleTrackerOnMain(".", "010-test", "epic-1", true); err == nil {
		t.Fatal("expected a propagated error on git-read failure, got nil")
	}
}

// (i) R2(c) (spec 121): when origin/main exists, the classifier consults
// IT for the committed export — never possibly-stale local main.
func TestStaleTrackerOnMain_ConsultsOriginMainFirst(t *testing.T) {
	stubFinalizeOrphanSeams(t, nil, nil, 0, nil, "", nil, nil, nil)
	var gotRefs []string
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) {
		gotRefs = append(gotRefs, ref)
		if ref == "origin/main" {
			return []byte(`{"id":"epic-1","status":"open"}` + "\n"), nil
		}
		return []byte(`{"id":"epic-1","status":"open"}` + "\n"), nil
	}
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil || o.Kind != "stale_tracker" {
		t.Fatalf("expected a stale_tracker finding, got %+v", o)
	}
	if len(gotRefs) == 0 || gotRefs[0] != "origin/main" {
		t.Errorf("the FIRST committed-export read must be origin/main, got %v", gotRefs)
	}
}

// (j) R2(c): the no-remote direct workflow (no origin/main ref at all)
// falls back to local main.
func TestStaleTrackerOnMain_NoRemoteFallsBackToLocalMain(t *testing.T) {
	stubFinalizeOrphanSeams(t, nil, nil, 0, nil, "", nil, nil, nil)
	revParseRefFn = func(workdir, ref string) (string, error) {
		return "", fmt.Errorf("rev-parse %s: %w", ref, gitutil.ErrRefNotFound)
	}
	var gotRef string
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) {
		gotRef = ref
		return []byte(`{"id":"epic-1","status":"open"}` + "\n"), nil
	}
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected a finding, got nil")
	}
	if gotRef != "main" {
		t.Errorf("no-remote fallback must read local main, got ref %q", gotRef)
	}
}

// (k) R2(c): a genuine (non-ErrRefNotFound) failure resolving origin/main
// propagates rather than silently falling back to local main.
func TestStaleTrackerOnMain_OriginMainResolveErrorPropagates(t *testing.T) {
	stubFinalizeOrphanSeams(t, nil, nil, 0, nil, "", nil, nil, nil)
	revParseRefFn = func(workdir, ref string) (string, error) {
		return "", errors.New("simulated transient git failure")
	}
	if _, err := StaleTrackerOnMain(".", "010-test", "epic-1", true); err == nil {
		t.Fatal("expected the transient origin/main resolution failure to propagate, got nil")
	}
}

// (l) R2(c): origin/main already agrees (closed) but the DISTINCT local
// main ref still lags — surfaced as a pull_advisory, NEVER the self-looping
// `mindspec impl approve` recovery a stale_tracker finding carries.
func TestStaleTrackerOnMain_PullAdvisoryWhenLocalLags(t *testing.T) {
	stubFinalizeOrphanSeams(t, nil, nil, 0, nil, "", nil, nil, nil)
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) {
		if ref == "origin/main" {
			return []byte(`{"id":"epic-1","status":"closed"}` + "\n"), nil
		}
		return []byte(`{"id":"epic-1","status":"open"}` + "\n"), nil // local main lags
	}
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil || o.Kind != "pull_advisory" {
		t.Fatalf("expected a pull_advisory finding, got %+v", o)
	}
	if strings.Contains(o.RecoveryCommand(), "impl approve") {
		t.Errorf("a pull_advisory must never recommend the self-looping impl-approve recovery, got %q", o.RecoveryCommand())
	}
	if !strings.Contains(o.RecoveryCommand(), "pull") {
		t.Errorf("expected the pull recovery line, got %q", o.RecoveryCommand())
	}
}

// (m) R2(c): origin/main agrees AND local main agrees too — no finding of
// any kind (no phantom pull_advisory when there is nothing to pull).
func TestStaleTrackerOnMain_NoPullAdvisoryWhenLocalAgrees(t *testing.T) {
	stubFinalizeOrphanSeams(t, nil, nil, 0, nil, "", nil, nil, nil)
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) {
		return []byte(`{"id":"epic-1","status":"closed"}` + "\n"), nil
	}
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != nil {
		t.Errorf("expected no finding when both origin/main and local main agree, got %+v", o)
	}
}

// TestFinalizeOrphanNetEffectFn_IsGitutilNetEffectLanded is AC-17's
// lifecycle-side anti-drift pin: the doctor merged-carrier suppression's
// seam default MUST be the identical exported symbol the executor probe
// falls back to (internal/executor's netEffectLandedFn) — never a private
// reimplementation at either site (the 119 AC-12 pattern,
// doctor/lifecycle_integrity_test.go:169 precedent).
func TestFinalizeOrphanNetEffectFn_IsGitutilNetEffectLanded(t *testing.T) {
	if reflect.ValueOf(finalizeOrphanNetEffectFn).Pointer() != reflect.ValueOf(gitutil.NetEffectLanded).Pointer() {
		t.Fatal("finalizeOrphanNetEffectFn must be gitutil.NetEffectLanded (AC-17 anti-drift: the doctor suppression and the executor probe must invoke the identical exported predicate)")
	}
}

// TestFinalizeOrphansSkipsMalformedBranch is spec 120 AC-23 (the reverse-
// derivation gate, round-4 G2): a local chore/finalize-x;evil-shaped
// branch yields NO FinalizeOrphan and one escaped warning/clean skip — no
// raw hostile bytes in any output; a valid chore/finalize-053-foo branch
// still reports byte-identically.
func TestFinalizeOrphansSkipsMalformedBranch(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		[]string{"main", "chore/finalize-x;evil", "chore/finalize-053-foo"}, nil,
		2, nil,
		"1 file changed", nil,
		nil, nil,
	)

	orphans, err := FindOutstandingFinalizeBranches(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected exactly 1 orphan (the malformed branch skipped), got %d: %+v", len(orphans), orphans)
	}
	o := orphans[0]
	if o.SpecID != "053-foo" || o.Branch != "chore/finalize-053-foo" {
		t.Errorf("expected the valid branch to report byte-identically, got %+v", o)
	}
	for _, orphan := range orphans {
		if strings.Contains(orphan.SpecID, ";") || strings.Contains(orphan.Branch, ";") {
			t.Errorf("hostile bytes reached a FinalizeOrphan: %+v", orphan)
		}
	}
}
