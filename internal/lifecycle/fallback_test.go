package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const fallbackSpec = "spec/129-test"
const fallbackBead = "bead-one"

type fallbackFixture struct {
	dir    string
	run    func(...string)
	merge  string
	second string
}

func writeFallbackFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// carry239 makes earlier spec landings reachable solely through main's second
// parent, then fast-forwards the spec. The spec retains the real merge object.
func carry239(t *testing.T, f fallbackFixture) {
	t.Helper()
	f.run("checkout", "main")
	f.run("merge", "--no-ff", "-m", "sibling PR", fallbackSpec)
	f.run("checkout", fallbackSpec)
	f.run("merge", "--ff-only", "main")
}

func newFallbackFixture(t *testing.T) fallbackFixture {
	t.Helper()
	dir, run := initLandedRepo(t, "129-test")
	mergeBead(t, run, dir, fallbackBead, fallbackSpec)
	f := fallbackFixture{dir: dir, run: run, merge: revParseIn(t, dir, "HEAD"), second: revParseIn(t, dir, "bead/"+fallbackBead)}
	carry239(t, f)
	return f
}

func registerFallbackPanel(t *testing.T, root, slug, sha string, round int) string {
	t.Helper()
	dir := filepath.Join(root, "reviews", slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	id := fallbackBead
	data, err := json.Marshal(panel.Panel{BeadID: &id, ReviewedHeadSHA: sha, Round: round})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "panel.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func requireOffFirstParent(t *testing.T, f fallbackFixture, count int) {
	t.Helper()
	got, err := FindLandedMergeReachable(f.dir, fallbackSpec, fallbackBead)
	var off *LandedMergeOffFirstParent
	if got != nil || !errors.As(err, &off) || !errors.Is(err, ErrLandedMergeNotFound) || errors.Is(err, ErrLandedMergeNoCandidate) || len(off.Candidates) != count {
		t.Fatalf("want %d off-first-parent candidates, got %+v %v", count, got, err)
	}
	for _, m := range off.Candidates {
		for _, text := range []string{fallbackBead, fallbackSpec, m.SHA, m.Parents[1], "recovery: mindspec reattest " + fallbackBead} {
			if !strings.Contains(err.Error(), text) {
				t.Errorf("diagnostic %q missing %q", err, text)
			}
		}
	}
}

func acceptFallback(t *testing.T, f fallbackFixture) map[string]interface{} {
	t.Helper()
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res == nil || !res.Wrote || len(*writes) != 1 {
		t.Fatalf("accept: %+v %v writes=%v", res, err, *writes)
	}
	meta := (*writes)[0].updates
	if !auditMatches(meta, fallbackSpec, f.merge, f.second) {
		t.Fatalf("invalid audit written: %+v", meta)
	}
	if !strings.HasPrefix(res.Corroboration, "off-first-parent fallback; single-candidate, branch-tip, not-reverted and read checks passed;") {
		t.Fatalf("missing provenance: %q", res.Corroboration)
	}
	stubBindingRead(t, meta)
	requireReadBack(t, f)
	return meta
}

func requireReadBack(t *testing.T, f fallbackFixture) {
	t.Helper()
	got, err := FindLandedMergeReachable(f.dir, fallbackSpec, fallbackBead)
	if err != nil || got == nil || got.SHA != f.merge || got.SecondParent != f.second {
		t.Fatalf("read-back: %+v %v", got, err)
	}
}

func TestReviewedSHAClass_Table(t *testing.T) {
	for _, tc := range []struct {
		sha   string
		class reviewedSHAClass
	}{
		{"", reviewedSHAEmpty}, {"design-review-no-code", reviewedSHAPlaceholder}, {"HEAD", reviewedSHAPlaceholder},
		{"abcdef0", reviewedSHAObjectID}, {strings.Repeat("A", 64), reviewedSHAObjectID},
		{"--output=/tmp/x", reviewedSHAMalformed}, {"HEAD~1", reviewedSHAMalformed}, {"main^", reviewedSHAMalformed},
		{"a..b", reviewedSHAMalformed}, {"main@{1}", reviewedSHAMalformed}, {"main:file", reviewedSHAMalformed},
		{" abcdef0", reviewedSHAMalformed}, {"abcdef0\n", reviewedSHAMalformed}, {"abc\x1bdef", reviewedSHAMalformed},
	} {
		t.Run(fmt.Sprintf("%q", tc.sha), func(t *testing.T) {
			if got := classifyReviewedSHA(tc.sha); got != tc.class {
				t.Fatalf("class=%s want %s", got, tc.class)
			}
		})
	}
}

func TestFindLandedMergeReachable_OffFirstParentNamesOnly(t *testing.T) {
	f := newFallbackFixture(t)
	registerFallbackPanel(t, f.dir, "round-one", f.second, 1)
	requireOffFirstParent(t, f, 1)
}

func TestReattest_FallbackAccepts239Shape(t *testing.T) {
	f := newFallbackFixture(t)
	registerFallbackPanel(t, f.dir, "round-one", f.second, 1)
	acceptFallback(t, f)
}

func TestReattest_FallbackAcceptsDefaultMergeSubject(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	mergeBeadDefaultSubject(t, run, dir, fallbackBead, fallbackSpec)
	f := fallbackFixture{dir, run, revParseIn(t, dir, "HEAD"), revParseIn(t, dir, "bead/"+fallbackBead)}
	carry239(t, f)
	requireOffFirstParent(t, f, 1)
	got, err := FindLandedMergeReachable(dir, fallbackSpec, fallbackBead)
	var off *LandedMergeOffFirstParent
	if got != nil || !errors.As(err, &off) || len(off.Candidates) != 1 || off.Candidates[0].SHA != f.merge || off.Candidates[0].Parents[1] != f.second {
		t.Fatalf("default subject lost scan-derived candidate: %+v %v", got, err)
	}
	// Pins one atomic binding+audit write and the successful audited read-back.
	acceptFallback(t, f)
}

func TestReattest_FallbackSameTupleNoOp(t *testing.T) {
	f := newFallbackFixture(t)
	acceptFallback(t, f)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res.Wrote || len(*writes) != 0 {
		t.Fatalf("no-op: %+v %v writes=%v", res, err, *writes)
	}
	requireReadBack(t, f)
}

// Spec 129 R4.2: a valid audit must equal both binding and candidate to no-op.
func TestReattest_FallbackRewritesWhenBindingDriftsFromValidAudit(t *testing.T) {
	f := newFallbackFixture(t)
	meta := acceptFallback(t, f)
	meta["mindspec_landed_merge_sha"] = strings.Repeat("c", 40)

	requireOffFirstParent(t, f, 1)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res == nil || !res.Wrote || len(*writes) != 1 {
		t.Fatalf("drifted binding must be rewritten: %+v %v writes=%v", res, err, *writes)
	}
	if !auditMatches((*writes)[0].updates, fallbackSpec, f.merge, f.second) {
		t.Fatalf("rewrite did not restore a valid matching tuple: %+v", (*writes)[0].updates)
	}
}

// Spec 129 R4.2: second-parent binding drift cannot converge on a valid audit.
func TestReattest_FallbackRewritesWhenBindingSecondParentDrifts(t *testing.T) {
	f := newFallbackFixture(t)
	meta := acceptFallback(t, f)
	meta["mindspec_landed_second_parent"] = strings.Repeat("c", 40)

	requireOffFirstParent(t, f, 1)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res == nil || !res.Wrote || len(*writes) != 1 {
		t.Fatalf("drifted second-parent binding must be rewritten: %+v %v writes=%v", res, err, *writes)
	}
	if !auditMatches((*writes)[0].updates, fallbackSpec, f.merge, f.second) {
		t.Fatalf("rewrite did not restore a valid matching tuple: %+v", (*writes)[0].updates)
	}
}

// Spec 129 R4.2 F4: partially superseded content is not a reverted landing.
func TestReattest_FallbackAcceptedThenEvolvedStaysIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	run("checkout", "-b", "bead/"+fallbackBead)
	writeFallbackFile(t, dir, "surface-a.txt", "alpha payload\n")
	writeFallbackFile(t, dir, "surface-b.txt", "beta payload\n")
	run("add", ".")
	run("commit", "-m", "work bead-one: two surfaces")
	run("checkout", fallbackSpec)
	run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
	f := fallbackFixture{dir: dir, run: run, merge: revParseIn(t, dir, "HEAD"), second: revParseIn(t, dir, "bead/"+fallbackBead)}
	carry239(t, f)
	acceptFallback(t, f)

	// Later first-parent work supersedes one surface and retains the other.
	if err := os.Rename(filepath.Join(dir, "surface-a.txt"), filepath.Join(dir, "surface-a2.txt")); err != nil {
		t.Fatal(err)
	}
	writeFallbackFile(t, dir, "surface-a2.txt", "superseding replacement for alpha\n")
	run("add", ".")
	run("commit", "-m", "supersede surface-a with surface-a2")

	outcome, err := gitutil.ContentSubsumedOutcome(dir, f.merge+"^1", f.merge, fallbackSpec)
	if err != nil || outcome != gitutil.SubsumptionCleanDivergence {
		t.Fatalf("shape precondition: want SubsumptionCleanDivergence, got %v %v", outcome, err)
	}
	requireReadBack(t, f)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	var refusal *ReattestRefusal
	if errors.As(err, &refusal) || err != nil || res == nil {
		t.Fatalf("evolved fallback landing refused: %+v %v writes=%v", res, err, *writes)
	}
}

// Spec 129 R4.2 F4: content edited in place is a landed-then-evolved landing.
func TestReattest_FallbackAcceptedThenConflictEvolvedStaysIdentified(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	run("checkout", "-b", "bead/"+fallbackBead)
	writeFallbackFile(t, dir, "surface-a.txt", "alpha payload\n")
	run("add", ".")
	run("commit", "-m", "work bead-one")
	run("checkout", fallbackSpec)
	run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
	f := fallbackFixture{dir: dir, run: run, merge: revParseIn(t, dir, "HEAD"), second: revParseIn(t, dir, "bead/"+fallbackBead)}
	carry239(t, f)
	acceptFallback(t, f)

	if err := os.WriteFile(filepath.Join(dir, "surface-a.txt"), []byte("alpha payload, edited later by honest work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "evolve surface-a in place")

	outcome, err := gitutil.ContentSubsumedOutcome(dir, f.merge+"^1", f.merge, fallbackSpec)
	if err != nil || outcome != gitutil.SubsumptionConflict {
		t.Fatalf("shape precondition: want SubsumptionConflict, got %v %v", outcome, err)
	}
	requireReadBack(t, f)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	var refusal *ReattestRefusal
	if errors.As(err, &refusal) || err != nil || res == nil {
		t.Fatalf("conflict-evolved fallback landing refused: %+v %v writes=%v", res, err, *writes)
	}
}

func TestReattest_FallbackMalformedSameTupleRewrites(t *testing.T) {
	for _, key := range []string{reattestKeyOp, reattestKeyAt, reattestKeyActor} {
		t.Run(key, func(t *testing.T) {
			f := newFallbackFixture(t)
			meta := acceptFallback(t, f)
			delete(meta, key)
			requireOffFirstParent(t, f, 1)
			acceptFallback(t, f)
			writes := installReattestRecorder(t)
			res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
			if err != nil || res.Wrote || len(*writes) != 0 {
				t.Fatalf("repaired audit not converged: %+v %v", res, err)
			}
		})
	}
}

func TestReattest_FallbackAcceptsSingleDirRebasedRound(t *testing.T) {
	f := newFallbackFixture(t)
	registerFallbackPanel(t, f.dir, "rebased", strings.Repeat("a", 40), 1)
	registerFallbackPanel(t, f.dir, "rebased", f.second, 2)
	acceptFallback(t, f)
}

func TestReattest_FallbackAccepts233Descendant(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	run("checkout", "-b", "bead/"+fallbackBead)
	writeFallbackFile(t, dir, "reviewed", "reviewed")
	run("add", ".")
	run("commit", "-m", "reviewed")
	reviewed := revParseIn(t, dir, "HEAD")
	writeFallbackFile(t, dir, "materialized", "later completion content")
	run("add", ".")
	run("commit", "-m", "materialization")
	second := revParseIn(t, dir, "HEAD")
	run("checkout", fallbackSpec)
	run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
	f := fallbackFixture{dir, run, revParseIn(t, dir, "HEAD"), second}
	carry239(t, f)
	registerFallbackPanel(t, dir, "reviewed", reviewed, 1)
	acceptFallback(t, f)
}

// Refresh and a long-form sibling merge must not become extra candidates.
func topologyWithRefresh(t *testing.T, refresh, sibling, own bool) fallbackFixture {
	t.Helper()
	dir, run := initLandedRepo(t, "129-test")
	run("checkout", "-b", "bead/"+fallbackBead)
	writeFallbackFile(t, dir, "bead-one.txt", "work")
	run("add", ".")
	run("commit", "-m", "work")
	if refresh {
		run("checkout", fallbackSpec)
		writeFallbackFile(t, dir, "spec.txt", "spec work")
		run("add", ".")
		run("commit", "-m", "spec advance")
		run("checkout", "bead/"+fallbackBead)
		run("merge", "--no-ff", "-m", "Merge branch 'spec/129-test' into bead/bead-one", fallbackSpec)
	}
	second := revParseIn(t, dir, "bead/"+fallbackBead)
	if sibling {
		run("checkout", "-b", "bead/sibling", fallbackSpec)
		run("merge", "--no-ff", "-m", "Merge branch 'bead/bead-one' into bead/sibling", "bead/"+fallbackBead)
	}
	run("checkout", fallbackSpec)
	merge := ""
	if own {
		run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
		merge = revParseIn(t, dir, "HEAD")
	}
	if sibling {
		run("merge", "--no-ff", "-m", "Merge bead/sibling", "bead/sibling")
	}
	return fallbackFixture{dir, run, merge, second}
}

func TestFindLandedMergeReachable_RefreshedThenOff(t *testing.T) {
	f := topologyWithRefresh(t, true, false, true)
	carry239(t, f)
	requireOffFirstParent(t, f, 1)
	acceptFallback(t, f)
}
func TestFindLandedMergeReachable_LongFormSiblingExcluded(t *testing.T) {
	f := topologyWithRefresh(t, false, true, true)
	carry239(t, f)
	requireOffFirstParent(t, f, 1)
	acceptFallback(t, f)
}
func TestFindLandedMergeReachable_OnlyViaSibling(t *testing.T) {
	f := topologyWithRefresh(t, false, true, false)
	carry239(t, f)
	_, err := FindLandedMergeReachable(f.dir, fallbackSpec, fallbackBead)
	if !errors.Is(err, ErrLandedMergeNoCandidate) {
		t.Fatalf("explicit sibling target admitted: %v", err)
	}
	writes := installReattestRecorder(t)
	_, err = ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateNoOwnedMerge || len(*writes) != 0 {
		t.Fatalf("sibling-only reattest: %v writes=%v", err, *writes)
	}
}
func TestFindLandedMergeReachable_FirstParentRefresh(t *testing.T) {
	f := topologyWithRefresh(t, true, false, true)
	old := reachableMergesFn
	t.Cleanup(func() { reachableMergesFn = old })
	reachableMergesFn = func(string, string) ([]gitutil.MergeCommit, error) {
		t.Fatal("fallback ran despite primary candidate")
		return nil, nil
	}
	requireReadBack(t, f)
}
func TestFindLandedMergeReachable_Unreachable(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	run("checkout", "-b", "unreachable")
	mergeBead(t, run, dir, fallbackBead, "unreachable")
	_, err := FindLandedMergeReachable(dir, fallbackSpec, fallbackBead)
	if !errors.Is(err, ErrLandedMergeNoCandidate) {
		t.Fatalf("unreachable merge nominated: %v", err)
	}
}
func TestMergedUnclosed_IgnoresFallback(t *testing.T) {
	f := newFallbackFixture(t)
	for _, audited := range []bool{false, true} {
		if audited {
			acceptFallback(t, f)
		}
		m, ok, err := MergedUnclosed(f.dir, fallbackSpec, fallbackBead)
		if m != nil || ok || err != nil {
			t.Fatalf("fallback licensed close (audited=%v): %+v %v %v", audited, m, ok, err)
		}
	}
}

// AC-11 (xi): the executor's merge-time binding names the #239 landing.
func TestReattest_FallbackRecordsAuditOnNoOp(t *testing.T) {
	f := newFallbackFixture(t)
	stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.second})
	requireOffFirstParent(t, f, 1)
	acceptFallback(t, f)
}

// AC-11 (ix): a forged binding names the short-form X-to-Y sibling merge.
func TestReadBack_ForgedBindingNoAudit(t *testing.T) {
	f := siblingFallbackFixture(t)
	meta := map[string]interface{}{
		"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.second,
	}
	stubBindingRead(t, meta)
	requireOffFirstParent(t, f, 1)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res == nil || !res.Wrote || len(*writes) != 1 {
		t.Fatalf("binding without audit must write exactly once: %+v %v writes=%v", res, err, *writes)
	}
	w := (*writes)[0]
	if w.beadID != fallbackBead || w.updates["mindspec_landed_merge_sha"] != meta["mindspec_landed_merge_sha"] ||
		w.updates["mindspec_landed_second_parent"] != meta["mindspec_landed_second_parent"] {
		t.Fatalf("audit write changed the existing binding: %+v", w)
	}
	if !auditMatches(w.updates, fallbackSpec, f.merge, f.second) ||
		w.updates[reattestKeyPriorMerge] != f.merge || w.updates[reattestKeyPriorSecond] != f.second {
		t.Fatalf("missing audited tuple or prior pair: %+v", w.updates)
	}
	stubBindingRead(t, w.updates)
	requireReadBack(t, f)
	res, err = ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res == nil || res.Wrote || len(*writes) != 1 {
		t.Fatalf("audited sibling landing did not converge: %+v %v writes=%v", res, err, *writes)
	}
}

func TestReadBack_FullForgedAuditResidual(t *testing.T) {
	f := siblingFallbackFixture(t)
	unlanded := outsideReviewedCommit(t, f)
	stubBindingRead(t, map[string]interface{}{
		"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.second,
		reattestKeyMerge: f.merge, reattestKeySecond: f.second, reattestKeyScannedBranch: fallbackSpec,
		reattestKeyOp: reattestOp, reattestKeyActor: "agent-writable", reattestKeyAt: "2026-10-19T00:00:00Z",
	})
	registerFallbackPanel(t, f.dir, "unlanded", unlanded, 2)
	requireReadBack(t, f)
}

func TestReadBack_MalformedAudit(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{reattestKeyOp, "other"}, {reattestKeyAt, "yesterday"}, {reattestKeyActor, ""}, {reattestKeyScannedBranch, "spec/other"},
		{reattestKeyMerge, ""}, {reattestKeySecond, "not-hex"},
		{reattestKeyMerge, strings.Repeat("a", 40)}, {reattestKeySecond, strings.Repeat("b", 40)},
		{"mindspec_landed_merge_sha", strings.Repeat("c", 40)}, {"mindspec_landed_second_parent", strings.Repeat("d", 40)},
	} {
		t.Run(tc.key, func(t *testing.T) {
			f := newFallbackFixture(t)
			meta := acceptFallback(t, f)
			meta[tc.key] = tc.value
			requireOffFirstParent(t, f, 1)
		})
	}
}

func TestReadBack_AcceptedThenReverted(t *testing.T) {
	f := newFallbackFixture(t)
	acceptFallback(t, f)
	f.run("revert", "--no-edit", "-m", "1", f.merge)
	_, err := FindLandedMergeReachable(f.dir, fallbackSpec, fallbackBead)
	if !errors.Is(err, ErrLandedMergeReverted) || !strings.Contains(err.Error(), "reverted or cleanly removed after landing") {
		t.Fatalf("revert accepted/wrong sentence: %v", err)
	}
	writes := installReattestRecorder(t)
	_, err = ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateReverted || len(*writes) != 0 {
		t.Fatalf("reverted reattest: %v writes=%v", err, *writes)
	}
	if !refusal.Fallback {
		t.Fatal("reverted fallback refusal must set Fallback=true")
	}
}

func TestReadBack_AcceptedThenLaterReviewStaysPass(t *testing.T) {
	f := newFallbackFixture(t)
	acceptFallback(t, f)
	registerFallbackPanel(t, f.dir, "later", strings.Repeat("a", 40), 1)
	old := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = old })
	landedPanelScanFn = func(...string) []panel.Registration { t.Fatal("read-back read a registration"); return nil }
	requireReadBack(t, f)
}

func moveSurvivingTip(t *testing.T, f fallbackFixture) {
	t.Helper()
	f.run("checkout", "bead/"+fallbackBead)
	writeFallbackFile(t, f.dir, "unlanded", "new tip")
	f.run("add", ".")
	f.run("commit", "-m", "unlanded")
	f.run("checkout", fallbackSpec)
}
func TestReattest_FallbackRefusesSurvivingTipContradiction(t *testing.T) {
	f := newFallbackFixture(t)
	moveSurvivingTip(t, f)
	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateTipContradiction || !refusal.Fallback || len(*writes) != 0 {
		t.Fatalf("surviving tip: %v writes=%v", err, *writes)
	}
}
func TestReadBack_SurvivingTipMovedFails(t *testing.T) {
	f := newFallbackFixture(t)
	acceptFallback(t, f)
	moveSurvivingTip(t, f)
	_, err := FindLandedMergeReachable(f.dir, fallbackSpec, fallbackBead)
	if !errors.Is(err, ErrLandedMergeContradiction) || !strings.Contains(err.Error(), "surviving branch") {
		t.Fatalf("tip contradiction lost: %v", err)
	}
}

func injectReachableReadError(t *testing.T) error {
	t.Helper()
	old := reachableMergesFn
	t.Cleanup(func() { reachableMergesFn = old })
	injected := errors.New("injected git read failure")
	reachableMergesFn = func(string, string) ([]gitutil.MergeCommit, error) { return nil, injected }
	return injected
}
func requirePlainReadError(t *testing.T, err, want error) {
	t.Helper()
	var off *LandedMergeOffFirstParent
	if !errors.Is(err, want) || errors.Is(err, ErrLandedMergeNotFound) || errors.As(err, &off) {
		t.Fatalf("infrastructure error misclassified: %v", err)
	}
}
func TestReattest_FallbackGitReadErrorRefuses(t *testing.T) {
	f := newFallbackFixture(t)
	injected := injectReachableReadError(t)
	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	requirePlainReadError(t, err, injected)
	if len(*writes) != 0 {
		t.Fatal("F5 failure wrote metadata")
	}
}
func TestReadBack_GitReadErrorIsPlainError(t *testing.T) {
	f := newFallbackFixture(t)
	acceptFallback(t, f)
	injected := injectReachableReadError(t)
	_, err := FindLandedMergeReachable(f.dir, fallbackSpec, fallbackBead)
	requirePlainReadError(t, err, injected)
}
func TestReadBack_MetadataReadErrorIsPlainError(t *testing.T) {
	f := newFallbackFixture(t)
	acceptFallback(t, f)
	old := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = old })
	injected := errors.New("injected metadata read failure")
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) { return nil, injected }
	_, err := FindLandedMergeReachable(f.dir, fallbackSpec, fallbackBead)
	requirePlainReadError(t, err, injected)
	writes := installReattestRecorder(t)
	_, err = ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	requirePlainReadError(t, err, injected)
	if len(*writes) != 0 {
		t.Fatal("metadata read failure wrote")
	}
}

func TestLandedMerge_TypedSentinelsWrapNotFound(t *testing.T) {
	for _, err := range []error{ErrLandedMergeContradiction, ErrLandedMergeReverted} {
		if !errors.Is(err, ErrLandedMergeNotFound) || errors.Is(err, ErrLandedMergeNoCandidate) {
			t.Fatalf("sentinel chain: %v", err)
		}
	}
}

func TestReviewedHeadSHASet_UntrackedIgnored(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	writeFallbackFile(t, dir, ".gitignore", "reviews/\n")
	sha := revParseIn(t, dir, "HEAD")
	registration := registerFallbackPanel(t, dir, "ignored", sha, 2)
	run("check-ignore", filepath.Join(registration, "panel.json"))
	set := reviewedHeadSHAForBead(dir, fallbackSpec, fallbackBead)
	if _, ok := set[sha]; !ok {
		t.Fatalf("ignored registration absent from set: %v", set)
	}
}

// Freeze timestamps for byte-for-byte comparisons of complete metadata writes.
func freezeReattestTime(t *testing.T) {
	t.Helper()
	old := reattestNowFn
	t.Cleanup(func() { reattestNowFn = old })
	reattestNowFn = func() time.Time { return time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC) }
}

func compareAdvisoryWrites(t *testing.T, f fallbackFixture, add func(), expected []string, count int) {
	t.Helper()
	freezeReattestTime(t)
	writes := installReattestRecorder(t)
	base, baseErr := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	if baseErr != nil || !base.Wrote || len(*writes) != 1 {
		t.Fatalf("baseline: %+v %v", base, baseErr)
	}
	before := (*writes)[0].updates
	add()
	var emitted []string
	res, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor", func(line string) {
		if len(*writes) != 1 {
			t.Fatal("advisory emitted after metadata write")
		}
		emitted = append(emitted, line)
	})
	if err != nil || err != baseErr || res.Wrote != base.Wrote || len(*writes) != 2 {
		t.Fatalf("advisory changed exit/write: %+v %v writes=%v", res, err, *writes)
	}
	after := (*writes)[1].updates
	// The provenance count is intentionally advisory-dependent. All binding,
	// audit tuple and prior-value keys must remain byte-identical.
	for key, value := range before {
		if key != reattestKeyCorroboration && !reflect.DeepEqual(after[key], value) {
			t.Errorf("advisory changed %s: %v -> %v", key, value, after[key])
		}
	}
	if !strings.Contains(str(t, after, reattestKeyCorroboration), fmt.Sprintf("advisory registrations outside tip: %d; panel roots scanned:", count)) {
		t.Fatalf("count/provenance: %v", after)
	}
	text := strings.Join(emitted, "\n")
	for _, want := range append(expected, "candidate "+f.merge+" reached "+fallbackSpec+"'s tip off the first-parent chain; its subject names no target", "advisory: panel roots scanned: "+f.dir) {
		if !strings.Contains(text, want) {
			t.Errorf("output %q missing %q", text, want)
		}
	}
	if strings.ContainsAny(text, "\x1b\x07") {
		t.Fatalf("raw terminal control in advisory: %q", text)
	}
}

// The short subject does not name a target: X can reach the spec inside Y.
func siblingFallbackFixture(t *testing.T) fallbackFixture {
	t.Helper()
	dir, run := initLandedRepo(t, "129-test")
	run("checkout", "-b", "bead/"+fallbackBead)
	writeFallbackFile(t, dir, "bead-one.txt", "bead-one\n")
	run("add", ".")
	run("commit", "-m", "X round one")
	second := revParseIn(t, dir, "HEAD")
	run("checkout", "-b", "bead/sibling", fallbackSpec)
	run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
	merge := revParseIn(t, dir, "HEAD")
	run("checkout", fallbackSpec)
	run("merge", "--no-ff", "-m", "Merge bead/sibling", "bead/sibling")
	return fallbackFixture{dir, run, merge, second}
}

func outsideReviewedCommit(t *testing.T, f fallbackFixture) string {
	t.Helper()
	f.run("checkout", "-b", "later-review", f.second)
	writeFallbackFile(t, f.dir, "later-round", "unlanded round")
	f.run("add", ".")
	f.run("commit", "-m", "round two never landed")
	sha := revParseIn(t, f.dir, "HEAD")
	f.run("checkout", fallbackSpec)
	return sha
}

func TestReattest_FallbackAcceptsLaterUnlandedRoundWithAdvisory(t *testing.T) {
	f := siblingFallbackFixture(t)
	sha := outsideReviewedCommit(t, f)
	dir := filepath.Join(f.dir, "reviews", "round-two")
	compareAdvisoryWrites(t, f, func() {
		registerFallbackPanel(t, f.dir, "round-one", f.second, 1)
		registerFallbackPanel(t, f.dir, "round-two", sha, 2)
	}, []string{"advisory: registration " + dir + " (round 2) reviewed " + sha + ", which is outside " + fallbackSpec + "'s tip history"}, 1)
}

func TestReattest_FallbackAcceptsAmendedAfterPanelWithAdvisory(t *testing.T) {
	f := newFallbackFixture(t)
	// A reviewed commit replaced by an amended sibling with the same tree is
	// outside tip history even though its content matches the landing.
	f.run("checkout", "-b", "pre-amend", f.second+"^1")
	writeFallbackFile(t, f.dir, "bead-one.txt", "bead-one\n")
	f.run("add", ".")
	f.run("commit", "-m", "pre-amend reviewed message")
	sha := revParseIn(t, f.dir, "HEAD")
	f.run("checkout", fallbackSpec)
	compareAdvisoryWrites(t, f, func() { registerFallbackPanel(t, f.dir, "amended", sha, 1) }, []string{"reviewed " + sha + ", which is outside " + fallbackSpec + "'s tip history"}, 1)
}

func TestReattest_FallbackMalformedReviewedSHANeverGitOperand(t *testing.T) {
	for _, sha := range []string{"--output=/tmp/x", "HEAD~1", "main^", "a..b", "bad\x1b[2J\x07"} {
		t.Run(fmt.Sprintf("%q", sha), func(t *testing.T) {
			f := newFallbackFixture(t)
			old := isAncestorFn
			t.Cleanup(func() { isAncestorFn = old })
			isAncestorFn = func(string, string, string) (bool, error) {
				t.Fatal("malformed registration reached git ancestry seam")
				return false, nil
			}
			compareAdvisoryWrites(t, f, func() { registerFallbackPanel(t, f.dir, "malformed", sha, 2) }, []string{"advisory: registration " + filepath.Join(f.dir, "reviews", "malformed") + " carries a malformed reviewed_head_sha; ignored"}, 0)
		})
	}
}

func TestReattest_FallbackSkipsEmptyAndPlaceholderSHA(t *testing.T) {
	for _, sha := range []string{"", "design-review-no-code"} {
		t.Run(sha, func(t *testing.T) {
			f := newFallbackFixture(t)
			old := isAncestorFn
			t.Cleanup(func() { isAncestorFn = old })
			isAncestorFn = func(string, string, string) (bool, error) {
				t.Fatal("empty/placeholder reached git")
				return false, nil
			}
			compareAdvisoryWrites(t, f, func() { registerFallbackPanel(t, f.dir, "skipped", sha, 1) }, nil, 0)
			lines, _, _ := advisoryRegistrationReport(f.dir, fallbackSpec, fallbackBead)
			if len(lines) != 1 || strings.Contains(lines[0], "registration ") {
				t.Fatalf("empty/placeholder emitted advisory: %v", lines)
			}
		})
	}
}

func TestReattest_FallbackGarbageCollectedSHAAdvisory(t *testing.T) {
	f := newFallbackFixture(t)
	sha := strings.Repeat("f", 40)
	compareAdvisoryWrites(t, f, func() { registerFallbackPanel(t, f.dir, "gc", sha, 2) }, []string{"reviewed " + sha + ", (not determinable locally)"}, 1)
}

func TestReattest_FallbackVisibilityChangesAdvisoryOnly(t *testing.T) {
	f := newFallbackFixture(t)
	sha := outsideReviewedCommit(t, f)
	// A second checkout shares git history but has no review files.
	other := filepath.Join(t.TempDir(), "bead-checkout")
	f.run("worktree", "add", "--detach", other, fallbackSpec)
	freezeReattestTime(t)
	specDir := filepath.Join(f.dir, ".mindspec", "specs", "129-test")
	registerFallbackPanel(t, specDir, "local-only", sha, 2)
	writeFallbackFile(t, f.dir, ".gitignore", ".mindspec/specs/*/reviews/**\n")
	f.run("check-ignore", filepath.Join(specDir, "reviews", "local-only", "panel.json"))
	writes := installReattestRecorder(t)
	var reports [2][]string
	for i, root := range []string{f.dir, other} {
		res, err := ReattestLandedMerge(root, fallbackSpec, fallbackBead, "test-actor", func(line string) { reports[i] = append(reports[i], line) })
		if err != nil || !res.Wrote {
			t.Fatalf("checkout %s: %+v %v", root, res, err)
		}
		if !strings.Contains(strings.Join(reports[i], "\n"), "advisory: panel roots scanned: "+root) {
			t.Fatalf("missing invoking roots: %v", reports[i])
		}
	}
	if len(*writes) != 2 {
		t.Fatalf("writes=%v", *writes)
	}
	for key, value := range (*writes)[0].updates {
		if key != reattestKeyCorroboration && !reflect.DeepEqual((*writes)[1].updates[key], value) {
			t.Errorf("visibility changed %s", key)
		}
	}
	if !strings.Contains(strings.Join(reports[0], "\n"), "which is outside") || strings.Contains(strings.Join(reports[1], "\n"), "which is outside") {
		t.Fatalf("visibility reports: %v", reports)
	}
}

// Two independent merges with the same second parent must NEVER be grouped.
// The empty sibling is made before removal of C, so grouping it would mask
// the real landing's later revert (AC-11 x).
func emptySiblingMaskFixture(t *testing.T) fallbackFixture {
	t.Helper()
	dir, run := initLandedRepo(t, "129-test")
	run("checkout", "-b", "bead/"+fallbackBead)
	writeFallbackFile(t, dir, "C", "content C\n")
	run("add", ".")
	run("commit", "-m", "X introduces C")
	second := revParseIn(t, dir, "HEAD")
	run("checkout", "-b", "bead/sibling", fallbackSpec)
	writeFallbackFile(t, dir, "C", "content C\n")
	run("add", ".")
	run("commit", "-m", "Y independently introduces C")
	yContent := revParseIn(t, dir, "HEAD")
	run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
	run("revert", "--no-edit", yContent)
	run("checkout", fallbackSpec)
	run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
	merge := revParseIn(t, dir, "HEAD")
	run("revert", "--no-edit", "-m", "1", merge)
	run("checkout", "bead/sibling")
	run("merge", "--no-ff", "-m", "refresh from spec", fallbackSpec)
	run("checkout", fallbackSpec)
	run("merge", "--ff-only", "bead/sibling")
	f := fallbackFixture{dir, run, merge, second}
	carry239(t, f)
	return f
}

func TestReattest_FallbackRefusesMultipleCandidates(t *testing.T) {
	f := emptySiblingMaskFixture(t)
	requireOffFirstParent(t, f, 2)
	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateMultipleFallbackCandidates || len(*writes) != 0 {
		t.Fatalf("multiple same-parent candidates: %v writes=%v", err, *writes)
	}
}

func addSecondCandidate(t *testing.T, f fallbackFixture) string {
	t.Helper()
	f.run("checkout", "-b", "second-sibling", f.merge+"^1")
	f.run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead+" into "+fallbackSpec, "bead/"+fallbackBead)
	merge := revParseIn(t, f.dir, "HEAD")
	f.run("merge", "--no-ff", "-m", "carry spec", fallbackSpec)
	f.run("checkout", fallbackSpec)
	f.run("merge", "--ff-only", "second-sibling")
	carry239(t, f)
	return merge
}

func TestReadBack_AcceptedThenSecondCandidate(t *testing.T) {
	f := newFallbackFixture(t)
	acceptFallback(t, f)
	addSecondCandidate(t, f)
	requireOffFirstParent(t, f, 2)
	// A matching audit for the scan's first candidate cannot defeat F1 either.
	candidates, err := reachableCandidates(f.dir, fallbackSpec, "bead/"+fallbackBead)
	if err != nil || len(candidates) != 2 {
		t.Fatalf("candidate fixture: %+v %v", candidates, err)
	}
	m := candidates[0]
	stubBindingRead(t, map[string]interface{}{
		"mindspec_landed_merge_sha": m.SHA, "mindspec_landed_second_parent": m.Parents[1],
		reattestKeyMerge: m.SHA, reattestKeySecond: m.Parents[1], reattestKeyScannedBranch: fallbackSpec,
		reattestKeyOp: reattestOp, reattestKeyActor: "test-actor", reattestKeyAt: "2026-10-19T00:00:00Z",
	})
	requireOffFirstParent(t, f, 2)
}
func TestReadBack_StaleAuditAfterExecutorRewrite(t *testing.T) {
	f := newFallbackFixture(t)
	meta := acceptFallback(t, f)
	secondMerge := addSecondCandidate(t, f)
	// MergeMetadata semantics: replace only the binding pair, retain old audit.
	meta["mindspec_landed_merge_sha"] = secondMerge
	meta["mindspec_landed_second_parent"] = f.second
	requireOffFirstParent(t, f, 2)
	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateMultipleFallbackCandidates || len(*writes) != 0 {
		t.Fatalf("stale audit repaired ambiguously: %v writes=%v", err, *writes)
	}
}

func TestFindLandedMerge_PanelSet(t *testing.T) {
	for _, leg := range []string{"matching-second-round", "whitespace-padded-second", "stale-with-tip", "stale-with-moved-tip", "stale-alone", "stale-with-binding", "stale-with-contradictory-binding", "exact-with-contradictory-binding"} {
		t.Run(leg, func(t *testing.T) {
			dir, run := initLandedRepo(t, "129-test")
			mergeBead(t, run, dir, fallbackBead, fallbackSpec)
			f := fallbackFixture{dir, run, revParseIn(t, dir, "HEAD"), revParseIn(t, dir, "bead/"+fallbackBead)}
			stale := revParseIn(t, dir, f.second+"^1")
			registerFallbackPanel(t, dir, "a-stale", stale, 1)
			switch leg {
			case "matching-second-round":
				registerFallbackPanel(t, dir, "z-second", f.second, 2)
				run("branch", "-D", "bead/"+fallbackBead)
			case "whitespace-padded-second":
				registerFallbackPanel(t, dir, "z-second", " "+f.second, 2)
				run("branch", "-D", "bead/"+fallbackBead)
				set := reviewedHeadSHAForBead(dir, fallbackSpec, fallbackBead)
				if _, ok := set[f.second]; ok {
					t.Errorf("set builder admitted a whitespace-padded second parent: %v", set)
				}
				if _, ok := set[" "+f.second]; ok {
					t.Fatal("set builder retained whitespace in its key")
				}
			case "stale-with-moved-tip":
				moveSurvivingTip(t, f)
			case "stale-alone":
				run("branch", "-D", "bead/"+fallbackBead)
			case "stale-with-binding":
				run("branch", "-D", "bead/"+fallbackBead)
				stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.second})
			case "stale-with-contradictory-binding", "exact-with-contradictory-binding":
				run("branch", "-D", "bead/"+fallbackBead)
				stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": stale})
				if leg == "exact-with-contradictory-binding" {
					registerFallbackPanel(t, dir, "z-second", f.second, 2)
				}
			}
			got, err := FindLandedMerge(dir, fallbackSpec, fallbackBead)
			switch leg {
			case "stale-with-contradictory-binding", "exact-with-contradictory-binding":
				if got != nil || !errors.Is(err, ErrLandedMergeContradiction) || !strings.Contains(err.Error(), "landed-binding") {
					t.Fatalf("panel set pre-empted landed-binding veto: %+v %v", got, err)
				}
			case "stale-with-moved-tip":
				if !errors.Is(err, ErrLandedMergeContradiction) || !strings.Contains(err.Error(), "surviving branch") {
					t.Fatalf("wrong contradiction datum: %v", err)
				}
			case "stale-alone", "whitespace-padded-second":
				var no *LandedMergeNoEvidence
				if got != nil || !errors.As(err, &no) || no.MergeSHA != f.merge {
					t.Fatalf("nonmatching set confirmed/lost candidate: %+v %v", got, err)
				}
			default:
				if err != nil || got == nil || got.SHA != f.merge {
					t.Fatalf("set pre-empted exact datum: %+v %v", got, err)
				}
			}
		})
	}
}

func TestReattest_PanelSetNonCorroborating(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	mergeBead(t, run, dir, fallbackBead, fallbackSpec)
	merge := revParseIn(t, dir, "HEAD")
	second := revParseIn(t, dir, "bead/"+fallbackBead)
	registerFallbackPanel(t, dir, "stale", revParseIn(t, dir, second+"^1"), 1)
	run("branch", "-D", "bead/"+fallbackBead)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || !res.Wrote || len(*writes) != 1 || !strings.Contains(res.Corroboration, "(c) panel set non-corroborating") {
		t.Fatalf("non-corroborating reattest: %+v %v writes=%v", res, err, *writes)
	}
	stubBindingRead(t, (*writes)[0].updates)
	got, err := FindLandedMerge(dir, fallbackSpec, fallbackBead)
	if err != nil || got.SHA != merge {
		t.Fatalf("repaired binding did not confirm: %+v %v", got, err)
	}
}

func TestReattest_PanelSetRepairsContradictoryPrimaryBinding(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	mergeBead(t, run, dir, fallbackBead, fallbackSpec)
	merge := revParseIn(t, dir, "HEAD")
	second := revParseIn(t, dir, "bead/"+fallbackBead)
	registerFallbackPanel(t, dir, "stale", revParseIn(t, dir, second+"^1"), 1)
	run("branch", "-D", "bead/"+fallbackBead)
	priorMerge, priorSecond := strings.Repeat("a", 40), strings.Repeat("b", 40)
	stubBindingRead(t, map[string]interface{}{
		"mindspec_landed_merge_sha": priorMerge, "mindspec_landed_second_parent": priorSecond,
	})
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res == nil || !res.Wrote || len(*writes) != 1 {
		t.Fatalf("primary repair must write exactly once without refusal: %+v %v writes=%v", res, err, *writes)
	}
	w := (*writes)[0]
	if w.beadID != fallbackBead || !auditMatches(w.updates, fallbackSpec, merge, second) {
		t.Fatalf("repair did not write scan-derived binding and valid audit together: %+v", w)
	}
	if w.updates[reattestKeyPriorMerge] != priorMerge || w.updates[reattestKeyPriorSecond] != priorSecond {
		t.Fatalf("contradictory prior pair not audited: %+v", w.updates)
	}
	corroboration, _ := w.updates[reattestKeyCorroboration].(string)
	if !strings.Contains(corroboration, "(c) panel set non-corroborating") || corroboration != res.Corroboration {
		t.Fatalf("audit lost non-corroborating panel provenance: %+v", w.updates)
	}
	stubBindingRead(t, w.updates)
	got, err := FindLandedMerge(dir, fallbackSpec, fallbackBead)
	if err != nil || got == nil || got.SHA != merge || got.SecondParent != second {
		t.Fatalf("primary repaired binding read-back: %+v %v", got, err)
	}
}

func TestMergedUnclosed_PanelSet(t *testing.T) {
	dir, run := initLandedRepo(t, "129-test")
	mergeBead(t, run, dir, fallbackBead, fallbackSpec)
	second := revParseIn(t, dir, "bead/"+fallbackBead)
	registerFallbackPanel(t, dir, "stale", revParseIn(t, dir, second+"^1"), 1)
	got, ok, err := MergedUnclosed(dir, fallbackSpec, fallbackBead)
	if err != nil || !ok || got == nil {
		t.Fatalf("stale set vetoed surviving tip: %+v %v %v", got, ok, err)
	}
	run("branch", "-D", "bead/"+fallbackBead)
	got, ok, err = MergedUnclosed(dir, fallbackSpec, fallbackBead)
	var no *LandedMergeNoEvidence
	if got != nil || ok || !errors.As(err, &no) {
		t.Fatalf("stale set licensed close: %+v %v %v", got, ok, err)
	}
}

func TestReattest_RepairsContradictoryBindingOnFallback(t *testing.T) {
	f := newFallbackFixture(t)
	prior := strings.Repeat("a", 40)
	stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": prior, "mindspec_landed_second_parent": prior})
	registerFallbackPanel(t, f.dir, "stale", strings.Repeat("b", 40), 1)
	requireOffFirstParent(t, f, 1)
	meta := acceptFallback(t, f)
	if meta[reattestKeyPriorMerge] != prior || meta[reattestKeyPriorSecond] != prior {
		t.Fatalf("prior binding not audited: %+v", meta)
	}
}

func TestFindLandedMergeReachable_PrimaryNonIdentificationNeverFallsBack(t *testing.T) {
	for _, contradiction := range []bool{false, true} {
		t.Run(fmt.Sprintf("binding-contradiction=%v", contradiction), func(t *testing.T) {
			dir, run := initLandedRepo(t, "129-test")
			mergeBead(t, run, dir, fallbackBead, fallbackSpec)
			merge := revParseIn(t, dir, "HEAD")
			second := revParseIn(t, dir, merge+"^2")
			run("branch", "-D", "bead/"+fallbackBead)
			if contradiction {
				stubBindingRead(t, map[string]interface{}{
					"mindspec_landed_merge_sha":     merge,
					"mindspec_landed_second_parent": revParseIn(t, dir, second+"^1"),
				})
			}
			check := func(got *LandedMerge, err error) {
				t.Helper()
				var no *LandedMergeNoEvidence
				var off *LandedMergeOffFirstParent
				if got != nil || errors.As(err, &off) {
					t.Fatalf("primary result replaced with fallback: %+v %v", got, err)
				}
				if contradiction {
					if !errors.Is(err, ErrLandedMergeContradiction) || !strings.Contains(err.Error(), "landed-binding") {
						t.Fatalf("lost primary binding contradiction: %v", err)
					}
				} else if !errors.As(err, &no) || no.MergeSHA != merge || no.SecondParent != second {
					t.Fatalf("lost primary NoEvidence candidate: %v", err)
				}
			}
			check(FindLandedMerge(dir, fallbackSpec, fallbackBead))
			old := reachableMergesFn
			t.Cleanup(func() { reachableMergesFn = old })
			reachableMergesFn = func(string, string) ([]gitutil.MergeCommit, error) {
				t.Fatal("fallback ran despite primary non-identification")
				return nil, nil
			}
			check(FindLandedMergeReachable(dir, fallbackSpec, fallbackBead))
		})
	}
}

func TestFindLandedMergeReachable_SubjectMatcherRejectsOtherTargetsAndPrefixes(t *testing.T) {
	for _, tc := range []struct {
		bead, subject string
		octopus       bool
	}{
		{fallbackBead, "Merge bead/" + fallbackBead + " into bead/other", false},
		{fallbackBead, "Merge bead/" + fallbackBead + " into spec/other-spec", false},
		{fallbackBead, "Merge branch 'bead/" + fallbackBead + "' into spec/other-spec", false},
		{fallbackBead, "Merge branch 'bead/" + fallbackBead + "'", false},
		{fallbackBead, "Merge bead/" + fallbackBead, true},
		{fallbackBead, "Merge bead/" + fallbackBead + ".10", false},
		{fallbackBead, "Merge bead/" + fallbackBead + "-x", false},
		{fallbackBead + ".1", "Merge bead/" + fallbackBead + ".10", false},
		{fallbackBead + ".1", "Merge bead/" + fallbackBead + ".1-x", false},
	} {
		t.Run(tc.subject+"/for="+tc.bead, func(t *testing.T) {
			f := siblingFallbackFixture(t)
			// Reuse a real reachable merge through the scan seam, varying
			// its subject or parent count. A matching audit grants nothing.
			m := gitutil.MergeCommit{SHA: f.merge, Parents: []string{revParseIn(t, f.dir, f.merge+"^1"), f.second}, Subject: tc.subject}
			if tc.octopus {
				m.Parents = append(m.Parents, strings.Repeat("e", 40))
			}
			oldPrimary, oldReachable := firstParentMergesFn, reachableMergesFn
			t.Cleanup(func() { firstParentMergesFn, reachableMergesFn = oldPrimary, oldReachable })
			firstParentMergesFn = func(string, string) ([]gitutil.MergeCommit, error) { return nil, nil }
			reachableMergesFn = func(string, string) ([]gitutil.MergeCommit, error) { return []gitutil.MergeCommit{m}, nil }
			stubBindingRead(t, map[string]interface{}{
				"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.second,
				reattestKeyMerge: f.merge, reattestKeySecond: f.second,
				reattestKeyScannedBranch: fallbackSpec, reattestKeyOp: reattestOp,
				reattestKeyActor: "test-actor", reattestKeyAt: "2026-10-19T00:00:00Z",
			})
			candidates, err := reachableCandidates(f.dir, fallbackSpec, "bead/"+tc.bead)
			if err != nil || len(candidates) != 0 {
				t.Errorf("subject matcher admitted %q for %s: %+v %v", tc.subject, tc.bead, candidates, err)
			}
			got, err := FindLandedMergeReachable(f.dir, fallbackSpec, tc.bead)
			if got != nil || !errors.Is(err, ErrLandedMergeNoCandidate) {
				t.Errorf("finder admitted excluded subject: %+v %v", got, err)
			}
			writes := installReattestRecorder(t)
			res, err := ReattestLandedMerge(f.dir, fallbackSpec, tc.bead, "test-actor")
			var refusal *ReattestRefusal
			if res != nil || !errors.As(err, &refusal) || refusal.State != ReattestStateNoOwnedMerge || len(*writes) != 0 {
				t.Fatalf("reattest admitted excluded subject: %+v %v writes=%v", res, err, *writes)
			}
		})
	}
}

func TestFindLandedMerge_BindingContradictionEscapesBranch(t *testing.T) {
	const branch = "spec/129-test\x1b[2J\x07\u009b31m"
	merge, second := strings.Repeat("a", 40), strings.Repeat("b", 40)
	oldPrimary, oldTip := firstParentMergesFn, landedRevParseRefFn
	t.Cleanup(func() { firstParentMergesFn, landedRevParseRefFn = oldPrimary, oldTip })
	firstParentMergesFn = func(string, string) ([]gitutil.MergeCommit, error) {
		return []gitutil.MergeCommit{{SHA: merge, Parents: []string{"first", second}, Subject: "Merge bead/" + fallbackBead}}, nil
	}
	landedRevParseRefFn = func(string, string) (string, error) { return "", gitutil.ErrRefNotFound }
	stubBindingRead(t, map[string]interface{}{
		"mindspec_landed_merge_sha": strings.Repeat("c", 40), "mindspec_landed_second_parent": strings.Repeat("d", 40),
	})
	got, err := FindLandedMerge(t.TempDir(), branch, fallbackBead)
	if got != nil || !errors.Is(err, ErrLandedMergeContradiction) || !strings.Contains(err.Error(), "landed-binding") {
		t.Fatalf("lost binding contradiction: %+v %v", got, err)
	}
	message := err.Error()
	if strings.ContainsFunc(message, unicode.IsControl) || !strings.Contains(message, termsafe.Escape(branch)) || !strings.Contains(message, termsafe.Escape(fallbackBead)) {
		t.Errorf("binding contradiction contains unsafe or missing ID/branch: %q", message)
	}
}

func TestReattest_FallbackAdvisoryEscapesDirectoriesAndRoots(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprintf("malformed=%v", malformed), func(t *testing.T) {
			f := siblingFallbackFixture(t)
			sha := outsideReviewedCommit(t, f)
			if malformed {
				sha = "bad\x1b[2J\x07\u009b31m"
			}
			root := filepath.Join(t.TempDir(), "root\x1b\x07\u009b31m")
			f.run("worktree", "add", "--detach", root, fallbackSpec)
			dir := registerFallbackPanel(t, root, "panel\x1b[2J\x07\u009b31m", sha, 2)
			lines, count, roots := advisoryRegistrationReport(root, fallbackSpec, fallbackBead)
			wantCount := 1
			want := "advisory: registration " + termsafe.Escape(dir) + " (round 2) reviewed " + sha + ", which is outside " + fallbackSpec + "'s tip history"
			if malformed {
				wantCount = 0
				want = "advisory: registration " + termsafe.Escape(dir) + " carries a malformed reviewed_head_sha; ignored"
			}
			if count != wantCount || len(lines) != 2 || lines[0] != want || !strings.Contains(roots, termsafe.Escape(root)) {
				t.Errorf("advisory content/count: lines=%q count=%d roots=%q", lines, count, roots)
			}
			assertSafe := func(origin string, values []string) {
				t.Helper()
				for _, line := range values {
					if strings.ContainsFunc(line, unicode.IsControl) {
						t.Errorf("%s contains raw control byte: %q", origin, line)
					}
				}
			}
			assertSafe("returned advisory", append(lines, roots))
			writes := installReattestRecorder(t)
			var emitted []string
			res, err := ReattestLandedMerge(root, fallbackSpec, fallbackBead, "test-actor", func(line string) {
				if len(*writes) != 0 {
					t.Fatal("report callback ran after the write")
				}
				emitted = append(emitted, line)
			})
			if err != nil || res == nil || !res.Wrote || len(*writes) != 1 {
				t.Fatalf("hostile path changed acceptance: %+v %v writes=%v", res, err, *writes)
			}
			if !reflect.DeepEqual(res.Lines, emitted) || len(emitted) != 3 || emitted[1] != want {
				t.Errorf("returned/callback reports differ or lost registration: result=%q callback=%q", res.Lines, emitted)
			}
			assertSafe("returned reattest", res.Lines)
			assertSafe("reports callback", emitted)
		})
	}
}

func TestFindLandedMerge_PanelSetNonCorroboratingDiagnostic(t *testing.T) {
	const branch = "spec/129-test\x1b[2J\x07\u009b31m"
	const merge = "merge\x1b[2J\x07"
	const second = "second\x1b[2J\x07"
	oldPrimary, oldTip, oldPanel := firstParentMergesFn, landedRevParseRefFn, landedPanelScanFn
	t.Cleanup(func() { firstParentMergesFn, landedRevParseRefFn, landedPanelScanFn = oldPrimary, oldTip, oldPanel })
	firstParentMergesFn = func(string, string) ([]gitutil.MergeCommit, error) {
		return []gitutil.MergeCommit{{SHA: merge, Parents: []string{"first", second}, Subject: "Merge bead/" + fallbackBead}}, nil
	}
	landedRevParseRefFn = func(string, string) (string, error) { return "", gitutil.ErrRefNotFound }
	id := fallbackBead
	landedPanelScanFn = func(...string) []panel.Registration {
		return []panel.Registration{{Panel: panel.Panel{BeadID: &id, ReviewedHeadSHA: strings.Repeat("a", 40)}}}
	}
	got, err := FindLandedMerge(t.TempDir(), branch, fallbackBead)
	var no *LandedMergeNoEvidence
	if got != nil || !errors.As(err, &no) {
		t.Fatalf("lost non-corroborating NoEvidence: %+v %v", got, err)
	}
	message := err.Error()
	for _, value := range []string{fallbackBead, branch, merge, second} {
		if strings.Count(message, termsafe.Escape(value)) != 1 {
			t.Errorf("diagnostic must render %q escaped exactly once: %q", value, message)
		}
	}
	if strings.ContainsFunc(message, unicode.IsControl) || !strings.Contains(message, "panel set non-corroborating") {
		t.Errorf("unsafe or incomplete non-corroboration diagnostic: %q", message)
	}
}

func TestReadBack_StaleAuditSingleCandidate(t *testing.T) {
	f := siblingFallbackFixture(t)
	meta := acceptFallback(t, f)
	// Model the executor replacing only the binding pair. Keep M1's valid
	// audit, but make M2 the ONLY scan candidate so F1 cannot mask equality.
	// Replace the spec history in this throwaway repo, leaving M1 reachable
	// only from the old sibling branch, outside the scanned spec tip.
	f.run("checkout", "-b", "replacement", f.merge+"^1")
	f.run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead+" into "+fallbackSpec, "bead/"+fallbackBead)
	m2 := fallbackFixture{f.dir, f.run, revParseIn(t, f.dir, "HEAD"), f.second}
	f.run("branch", "-f", fallbackSpec, m2.merge)
	f.run("checkout", fallbackSpec)
	carry239(t, m2)
	meta["mindspec_landed_merge_sha"] = m2.merge
	meta["mindspec_landed_second_parent"] = m2.second
	if meta[reattestKeyMerge] == m2.merge {
		t.Fatal("fixture must carry different M1 and M2 audit tuples")
	}
	candidates, err := reachableCandidates(m2.dir, fallbackSpec, "bead/"+fallbackBead)
	if err != nil || len(candidates) != 1 || candidates[0].SHA != m2.merge {
		t.Fatalf("stale-audit fixture must pass F1: %+v %v", candidates, err)
	}
	// Show all validity and candidate/binding checks succeed with just the
	// audit pair replaced. The real read must reject the retained M1 pair.
	fresh := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		fresh[k] = v
	}
	fresh[reattestKeyMerge], fresh[reattestKeySecond] = m2.merge, m2.second
	if !auditMatches(fresh, fallbackSpec, m2.merge, m2.second) {
		t.Fatal("fixture has a failure beyond the stale audit pair")
	}
	requireOffFirstParent(t, m2, 1)
	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(m2.dir, fallbackSpec, fallbackBead, "test-actor")
	if err != nil || res == nil || !res.Wrote || len(*writes) != 1 || !auditMatches((*writes)[0].updates, fallbackSpec, m2.merge, m2.second) {
		t.Fatalf("single stale audit was not repaired: %+v %v writes=%v", res, err, *writes)
	}
}
