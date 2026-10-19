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

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
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
	if !strings.HasPrefix(res.Corroboration, "off-first-parent fallback; F1, F3, F4, F5 passed;") {
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

func TestReadBack_ForgedBindingNoAudit(t *testing.T) {
	f := newFallbackFixture(t)
	stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.second})
	requireOffFirstParent(t, f, 1)
	acceptFallback(t, f)
}

func TestReattest_FallbackRecordsAuditOnNoOp(t *testing.T) {
	TestReadBack_ForgedBindingNoAudit(t)
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
	for _, leg := range []string{"matching-second-round", "stale-with-tip", "stale-with-moved-tip", "stale-alone", "stale-with-binding"} {
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
			case "stale-with-moved-tip":
				moveSurvivingTip(t, f)
			case "stale-alone":
				run("branch", "-D", "bead/"+fallbackBead)
			case "stale-with-binding":
				run("branch", "-D", "bead/"+fallbackBead)
				stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.second})
			}
			got, err := FindLandedMerge(dir, fallbackSpec, fallbackBead)
			switch leg {
			case "stale-with-moved-tip":
				if !errors.Is(err, ErrLandedMergeContradiction) || !strings.Contains(err.Error(), "surviving branch") {
					t.Fatalf("wrong contradiction datum: %v", err)
				}
			case "stale-alone":
				var no *LandedMergeNoEvidence
				if !errors.As(err, &no) || no.MergeSHA != f.merge {
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
