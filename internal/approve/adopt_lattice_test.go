package approve

// Spec 127 bead 3 — AC-2(ix): the aggregation lattice, fixtured as a
// table over evaluateAdoptLattice directly (adopt_test.go drives the
// full AdoptSpec entrypoint and its rendered refusal wording; this file
// pins the lattice's SEMANTICS, real-git throughout, with only the bd
// list-epic-beads leg stubbed — bd realism is not this file's job, the
// git evidence primitives are).

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// stubAdoptListEpicBeads overrides adoptListEpicBeadsFn to return a fixed
// bead list regardless of epicID — bypassing bd entirely.
func stubAdoptListEpicBeads(t *testing.T, beads []adoptEpicBead) {
	t.Helper()
	orig := adoptListEpicBeadsFn
	t.Cleanup(func() { adoptListEpicBeadsFn = orig })
	adoptListEpicBeadsFn = func(root, epicID string) ([]adoptEpicBead, error) {
		return beads, nil
	}
}

// forceAdoptEvalError overrides adoptEvaluateAgainstMainFn so any ref in
// forced returns (evidenceError, guard.DestructionEvidenceError, the
// mapped error) while every OTHER ref falls through to the REAL
// evaluator — a seam-forced probe error over an otherwise-real git
// evaluation (the R4 target-drift backstop fixture's own "seam/hook-
// forced" technique, not a repo-corruption trick).
func forceAdoptEvalError(t *testing.T, forced map[string]error) {
	t.Helper()
	orig := adoptEvaluateAgainstMainFn
	t.Cleanup(func() { adoptEvaluateAgainstMainFn = orig })
	adoptEvaluateAgainstMainFn = func(root, ref string) (evidenceState, guard.DestructionOutcome, error) {
		if err, ok := forced[ref]; ok {
			return evidenceError, guard.DestructionEvidenceError, err
		}
		return orig(root, ref)
	}
}

// makeLandedBeadBranch creates bead/<id> off main, adds a commit, and
// merges it into main — an ordinary DestructionAncestor/Superseded shape
// (positive evidence), branch left behind (as a real merged-but-
// undeleted branch would be).
func makeLandedBeadBranch(t *testing.T, dir, id string) string {
	t.Helper()
	branch := "bead/" + id
	adoptGitRun(t, dir, "checkout", "-q", "-b", branch)
	adoptWriteFile(t, dir, id+".txt", "landed work for "+id+"\n")
	adoptCommit(t, dir, "work for "+id)
	adoptGitRun(t, dir, "checkout", "-q", "main")
	adoptGitRun(t, dir, "merge", "-q", "--no-ff", "-m", "Merge "+branch, branch)
	return branch
}

// makeUnlandedBeadBranch creates bead/<id> off main with a novel commit
// that is NEVER merged into main — an ordinary DestructionClean shape
// (negative evidence: not yet landed).
func makeUnlandedBeadBranch(t *testing.T, dir, id string) string {
	t.Helper()
	branch := "bead/" + id
	adoptGitRun(t, dir, "checkout", "-q", "-b", branch)
	adoptWriteFile(t, dir, id+".txt", "unlanded work for "+id+"\n")
	adoptCommit(t, dir, "work for "+id)
	adoptGitRun(t, dir, "checkout", "-q", "main")
	return branch
}

// TestEvaluateAdoptLattice_Table is AC-2(ix): every row asserts its own
// refusal-class marker, and the poisoned-cache row is evaluated at the
// freshly fetched remote tip, never the stale local cache.
func TestEvaluateAdoptLattice_Table(t *testing.T) {
	seenClasses := map[adoptRefusalClass]bool{} // O2-3: coverage assertion below
	t.Run("two_bead_branches_one_landed_one_stale_is_negative", func(t *testing.T) {
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		makeUnlandedBeadBranch(t, dir, "test-b2")
		stubAdoptListEpicBeads(t, []adoptEpicBead{
			{ID: "test-b1", Status: "closed"},
			{ID: "test-b2", Status: "closed"},
		})
		// No origin remote at all: source (i) is ABSENT, not an erroring
		// fetch (G1-B3-07/F1-O3 comment fix — the prior wording here
		// stated the opposite of the mechanism: with no "origin"
		// configured, evaluateAdoptLattice's srcIAbsent carve-out means
		// FetchRemoteBranchIn is never even called, so this fixture
		// cannot and does not exercise "negative beats a literal source-i
		// fetch error" — it only proves negative beats a merely-ABSENT
		// source. See TestEvaluateAdoptLattice_NegativeBeatsGenuineSourceIError
		// below for the real fetch-error-vs-negative ordering fixture no
		// prior test in this table exercised.
		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassNegative || res.Marker != adoptMarkerNegative || res.Trigger != adoptTriggerNegative {
			t.Fatalf("got Class=%v Marker=%q Trigger=%q, want negative/%s/%s", res.Class, res.Marker, res.Trigger, adoptMarkerNegative, adoptTriggerNegative)
		}
	})

	t.Run("one_landed_one_probe_error_is_evidence_error_distinct_from_negative", func(t *testing.T) {
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		errBranch := makeUnlandedBeadBranch(t, dir, "test-b2")
		forceAdoptEvalError(t, map[string]error{errBranch: errors.New("simulated probe failure for b2")})
		stubAdoptListEpicBeads(t, []adoptEpicBead{
			{ID: "test-b1", Status: "closed"},
			{ID: "test-b2", Status: "closed"},
		})
		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassEvidenceError || res.Marker != adoptMarkerEvidenceError || res.Trigger != adoptTriggerError {
			t.Fatalf("got Class=%v Marker=%q Trigger=%q, want evidence-error/%s/%s", res.Class, res.Marker, res.Trigger, adoptMarkerEvidenceError, adoptTriggerError)
		}
		if res.Marker == adoptMarkerNegative {
			t.Fatal("evidence-error marker must be textually distinct from the negative marker")
		}
	})

	t.Run("corroborated_spec_ref_positive_plus_bead_branch_negative_is_sources_conflict", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		adoptGitRun(t, dir, "branch", specBranch, "main")
		adoptBareOrigin(t, dir)
		adoptGitRun(t, dir, "push", "-q", "origin", specBranch)
		makeUnlandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "open"}})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassSourcesConflict || res.Marker != adoptMarkerSourcesConflict {
			t.Fatalf("got Class=%v Marker=%q, want sources-conflict/%s", res.Class, res.Marker, adoptMarkerSourcesConflict)
		}
	})

	t.Run("partial_coverage_one_of_n_landed_rest_uncovered_is_unavailable_never_verified", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		// spec branch == main (ancestor -> positive), pushed to a real
		// origin so source i is positive.
		adoptGitRun(t, dir, "branch", specBranch, "main")
		adoptBareOrigin(t, dir)
		adoptGitRun(t, dir, "push", "-q", "origin", specBranch)
		makeLandedBeadBranch(t, dir, "test-b1")
		// b2 has NO surviving branch and no merge commit anywhere naming
		// it -> FindLandedMerge legitimately reports not-found.
		stubAdoptListEpicBeads(t, []adoptEpicBead{
			{ID: "test-b1", Status: "closed"},
			{ID: "test-b2", Status: "closed"},
		})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassCoverageUnavailable || res.Marker != adoptMarkerCoverageUnavailable || res.Trigger != adoptTriggerNoSource {
			t.Fatalf("got Class=%v Marker=%q Trigger=%q, want coverage-unavailable/%s/%s", res.Class, res.Marker, res.Trigger, adoptMarkerCoverageUnavailable, adoptTriggerNoSource)
		}
		if res.Class == adoptClassNone {
			t.Fatal("one-of-N surviving-and-landed must never be a VERIFIED adopt (B-r4-5)")
		}
	})

	t.Run("poisoned_cache_evaluated_at_fresh_fetch_tip_never_stale_cache", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		adoptGitRun(t, dir, "branch", specBranch, "main")
		origin := adoptBareOrigin(t, dir)
		adoptGitRun(t, dir, "push", "-q", "origin", specBranch)
		// Populate dir's local origin/spec/042-test tracking ref at the
		// LANDED tip (spec branch == main here).
		adoptGitRun(t, dir, "fetch", "-q", "origin", specBranch)
		staleCachedTip := adoptRefHash(t, dir, "origin/"+specBranch)
		mainTip := adoptRefHash(t, dir, "main")
		if staleCachedTip != mainTip {
			t.Fatalf("setup: cached tip %s should equal main %s before poisoning", staleCachedTip, mainTip)
		}

		// A SEPARATE clone advances origin's spec branch to a genuinely
		// UNLANDED tip, without dir ever re-fetching — dir's local cache
		// is now stale (poisoned) relative to origin's true state.
		clone := t.TempDir()
		adoptGitRun(t, clone, "clone", "-q", origin, ".")
		adoptGitRun(t, clone, "checkout", "-q", specBranch)
		adoptWriteFile(t, clone, "unlanded.txt", "novel unlanded content\n")
		adoptCommit(t, clone, "novel unlanded work")
		adoptGitRun(t, clone, "push", "-q", "origin", specBranch)

		stubAdoptListEpicBeads(t, []adoptEpicBead{})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassNegative || res.Marker != adoptMarkerNegative {
			t.Fatalf("got Class=%v Marker=%q, want negative/%s (evaluated at the fresh fetch tip)", res.Class, res.Marker, adoptMarkerNegative)
		}
		// Prove the fetch actually ran and updated the local cache to
		// origin's true (unlanded) tip — the property the row exists to
		// verify, not merely the aggregate's class.
		freshTip := adoptRefHash(t, dir, "origin/"+specBranch)
		if freshTip == staleCachedTip {
			t.Fatal("the local tracking ref must have been updated by a fresh fetch, not left at the pre-poisoning cached tip")
		}
	})

	t.Run("unreachable_remote_variant_is_error", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		adoptGitRun(t, dir, "branch", specBranch, "main")
		adoptGitRun(t, dir, "remote", "add", "origin", "/nonexistent/path/that/does/not/exist")
		stubAdoptListEpicBeads(t, []adoptEpicBead{})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassEvidenceError {
			t.Fatalf("unreachable remote: got Class=%v, want evidence-error", res.Class)
		}
		if res.Marker != adoptMarkerEvidenceError {
			t.Fatalf("unreachable remote: got Marker=%q, want %q (O2-4: each row asserts its own marker)", res.Marker, adoptMarkerEvidenceError)
		}
	})

	t.Run("absent_remote_branch_variant_is_error", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		// origin exists but was never given this spec branch.
		adoptBareOrigin(t, dir)
		stubAdoptListEpicBeads(t, []adoptEpicBead{})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassEvidenceError {
			t.Fatalf("absent remote branch: got Class=%v, want evidence-error", res.Class)
		}
		if res.Marker != adoptMarkerEvidenceError {
			t.Fatalf("absent remote branch: got Marker=%q, want %q (O2-4: each row asserts its own marker)", res.Marker, adoptMarkerEvidenceError)
		}
	})

	t.Run("non_closed_beads_surviving_unlanded_branch_poisons_allstatuses_enumeration", func(t *testing.T) {
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		makeUnlandedBeadBranch(t, dir, "test-b2")
		// b2 is "open", NOT "closed" — a closed-only enumeration (the
		// forbidden ClosedEpicBeadIDs shape) would never even see it and
		// would wrongly green this row.
		stubAdoptListEpicBeads(t, []adoptEpicBead{
			{ID: "test-b1", Status: "closed"},
			{ID: "test-b2", Status: "open"},
		})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassNegative {
			t.Fatalf("a non-closed bead's surviving unlanded branch must poison the aggregate; got Class=%v", res.Class)
		}
		if res.Marker != adoptMarkerNegative {
			t.Fatalf("got Marker=%q, want %q (O2-4: each row asserts its own marker)", res.Marker, adoptMarkerNegative)
		}
	})

	t.Run("all_positive_with_coverage_is_verified", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		adoptGitRun(t, dir, "branch", specBranch, "main")
		adoptBareOrigin(t, dir)
		adoptGitRun(t, dir, "push", "-q", "origin", specBranch)
		makeLandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		seenClasses[res.Class] = true
		if res.Class != adoptClassNone {
			t.Fatalf("all-positive-with-coverage must be VERIFIED (Class none); got Class=%v Detail=%q", res.Class, res.Detail)
		}
	})

	// O2-3: a coverage assertion tying this table's rows to
	// adoptRefusalClass's own cardinality (the identical convention this
	// file's header comment cites for bead 1's DestructionOutcome, but
	// had not itself applied) — every class from adoptClassNone through
	// adoptClassCoverageUnavailable must be exercised by at least one
	// row above. Verified by mutation: adding a 6th, unused class value
	// to the enum leaves this assertion RED until a new row exercises
	// it, rather than silently staying green.
	if len(seenClasses) != int(adoptRefusalClassCount) {
		var missing []string
		for c := adoptRefusalClass(0); c < adoptRefusalClassCount; c++ {
			if !seenClasses[c] {
				missing = append(missing, fmt.Sprintf("%d", c))
			}
		}
		t.Fatalf("table exercises %d of %d refusal classes; missing class value(s): %v — every adoptRefusalClass value needs its own row", len(seenClasses), int(adoptRefusalClassCount), missing)
	}
}

// TestEvaluateAdoptLattice_NegativeBeatsGenuineSourceIError is the real
// fixture the G1-B3-07/F1(O3) comment fix names as missing: no row in
// TestEvaluateAdoptLattice_Table pairs a LITERALLY ERRORING source (i)
// (a configured-but-unreachable origin — never the absent-remote
// carve-out, which skips the fetch entirely) with a genuinely NEGATIVE
// bead branch, even though the code's `if anyNegative {...} if
// anyError {...}` ordering implements exactly this priority by
// construction. This pins it empirically.
func TestEvaluateAdoptLattice_NegativeBeatsGenuineSourceIError(t *testing.T) {
	dir := adoptInitRepo(t)
	specBranch := "spec/042-test"
	// A CONFIGURED but unreachable origin: source (i) genuinely ERRORS
	// (never absent — RemoteExistsIn reports true; the fetch itself
	// fails).
	adoptGitRun(t, dir, "remote", "add", "origin", "/nonexistent/path/that/does/not/exist")
	makeUnlandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})

	res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
	if err != nil {
		t.Fatalf("evaluateAdoptLattice: %v", err)
	}
	if res.Class != adoptClassNegative || res.Marker != adoptMarkerNegative || res.Trigger != adoptTriggerNegative {
		t.Fatalf("got Class=%v Marker=%q Trigger=%q, want negative/%s/%s (negative must win over a genuinely erroring source i, not merely an absent one)", res.Class, res.Marker, res.Trigger, adoptMarkerNegative, adoptTriggerNegative)
	}
}

// TestEvaluateAdoptLattice_MalformedGitConfigIsEvidenceErrorNeverNoSource
// is G1-B3-04, real-git (mirroring G1's own reproduction — a lab with
// one invalid line in .git/config): a structurally malformed/unreadable
// git config makes `git config --get remote.origin.url` exit 128 (a
// genuine failure), not the clean exit-1 "not configured" case
// RemoteExistsIn's own absence path expects. Before the fix, a
// bool-returning RemoteExistsIn folded BOTH exits into `false`, so this
// exact scenario was audited identically to "no origin remote is
// configured" (srcIAbsent) — losing source (i) silently instead of
// erroring. The (bool, error) helpers must surface this as evidence-
// error, never no-source (and never VERIFIED).
func TestEvaluateAdoptLattice_MalformedGitConfigIsEvidenceErrorNeverNoSource(t *testing.T) {
	dir := adoptInitRepo(t)
	stubAdoptListEpicBeads(t, []adoptEpicBead{})

	cfgPath := filepath.Join(dir, ".git", "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading .git/config: %v", err)
	}
	corrupted := string(data) + "[bad\n"
	if err := os.WriteFile(cfgPath, []byte(corrupted), 0o644); err != nil {
		t.Fatalf("writing corrupted .git/config: %v", err)
	}

	res, evalErr := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
	if evalErr != nil {
		t.Fatalf("evaluateAdoptLattice: %v", evalErr)
	}
	if res.Class != adoptClassEvidenceError {
		t.Fatalf("a malformed .git/config must be evidence-error, never no-source/verified; got Class=%v Detail=%q", res.Class, res.Detail)
	}
	if res.Marker != adoptMarkerEvidenceError {
		t.Fatalf("got Marker=%q, want %q", res.Marker, adoptMarkerEvidenceError)
	}
	if res.Trigger != adoptTriggerError {
		t.Fatalf("got Trigger=%q, want %q (never no-source, even though the underlying git failure superficially resembles 'nothing configured')", res.Trigger, adoptTriggerError)
	}
}

// TestEvaluateAdoptLattice_BeadBranchExistsErrorIsEvidenceError is
// G1-B3-04's beads-loop counterpart, seam-forced (this file's own
// forceAdoptEvalError house style): a structural BranchExistsIn failure
// probing a SURVIVING bead's branch must fold into anyError, never be
// silently skipped as "no surviving branch" (which — before the fix —
// is exactly what a bool-returning BranchExistsIn made indistinguishable
// from a genuine structural failure).
func TestEvaluateAdoptLattice_BeadBranchExistsErrorIsEvidenceError(t *testing.T) {
	dir := adoptInitRepo(t)
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})

	origExists := adoptBranchExistsFn
	t.Cleanup(func() { adoptBranchExistsFn = origExists })
	adoptBranchExistsFn = func(root, name string) (bool, error) {
		if name == "bead/test-b1" {
			return false, errors.New("simulated structural git failure probing branch existence")
		}
		return origExists(root, name)
	}

	res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
	if err != nil {
		t.Fatalf("evaluateAdoptLattice: %v", err)
	}
	if res.Class != adoptClassEvidenceError || res.Marker != adoptMarkerEvidenceError || res.Trigger != adoptTriggerError {
		t.Fatalf("got Class=%v Marker=%q Trigger=%q, want evidence-error/%s/%s — a BranchExistsIn failure must never be silently read as 'no surviving branch'", res.Class, res.Marker, res.Trigger, adoptMarkerEvidenceError, adoptTriggerError)
	}
}

// TestEvaluateAdoptLattice_RuleDeletionRedOnRevert proves each pinned
// rule is load-bearing by deleting it and observing the SAME fixtures
// flip green-when-they-must-not — the AC-2(ix) preamble's own claim
// ("deleting the poison, coverage, or corroboration rule REDs its row").
// Rather than editing production source at test time, this re-derives
// each rule's ABSENCE directly: a hand-rolled aggregation that omits the
// rule, run over the identical fixture, is asserted to disagree with the
// real evaluateAdoptLattice — i.e. the real function's answer is NOT the
// answer you would get without that rule, so the rule is doing real work.
func TestEvaluateAdoptLattice_RuleDeletionRedOnRevert(t *testing.T) {
	t.Run("without_the_epic_coverage_rule_partial_coverage_would_wrongly_verify", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		adoptGitRun(t, dir, "branch", specBranch, "main")
		adoptBareOrigin(t, dir)
		adoptGitRun(t, dir, "push", "-q", "origin", specBranch)
		makeLandedBeadBranch(t, dir, "test-b1")
		stubAdoptListEpicBeads(t, []adoptEpicBead{
			{ID: "test-b1", Status: "closed"},
			{ID: "test-b2", Status: "closed"}, // uncovered
		})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		if res.Class == adoptClassNone {
			t.Fatal("coverage rule must have fired; got VERIFIED with an uncovered bead")
		}

		// S3-1: make the counterfactual REAL rather than a hardcoded
		// `true` literal standing in for it. Re-derive source (i) and
		// test-b1's state using the SAME real primitives
		// evaluateAdoptLattice itself calls — a rule-omitting
		// aggregation that stops at "no negative, no error among
		// LITERALLY-EVALUATED sources" (never reaching the coverage
		// clause, so it never even looks at test-b2, which has no
		// surviving branch) — and assert THAT computation would answer
		// positive/no-error over THIS exact fixture, matching the rigor
		// of the fetch-route sibling subtest below (which calls
		// evaluateAgainstMain directly rather than asserting a literal).
		remoteExists, remoteErr := adoptRemoteExistsFn(dir, adoptRemote)
		if remoteErr != nil {
			t.Fatalf("adoptRemoteExistsFn: %v", remoteErr)
		}
		if !remoteExists {
			t.Fatal("fixture setup: expected a configured origin remote")
		}
		fetchedSHA, fetchErr := adoptFetchRemoteBranchFn(dir, adoptRemote, specBranch)
		if fetchErr != nil {
			t.Fatalf("adoptFetchRemoteBranchFn: %v", fetchErr)
		}
		srcIState, _, srcIErr := adoptEvaluateAgainstMainFn(dir, fetchedSHA)
		if srcIErr != nil {
			t.Fatalf("evaluating source i: %v", srcIErr)
		}
		b1State, _, b1Err := adoptEvaluateAgainstMainFn(dir, "bead/test-b1")
		if b1Err != nil {
			t.Fatalf("evaluating bead/test-b1: %v", b1Err)
		}
		if srcIState == evidenceNegative || srcIState == evidenceError || b1State == evidenceNegative || b1State == evidenceError {
			t.Fatal("fixture no longer demonstrates the coverage rule's necessity: a literally-evaluated-sources-only scan would ALSO see a negative/error here, so omitting the coverage rule would not wrongly verify — the real counterfactual computation disagrees with this test's own premise")
		}
		// The rule-omitting aggregation (source i positive, test-b1
		// positive, test-b2 never examined) would therefore answer
		// VERIFIED — genuinely disagreeing with the real
		// evaluateAdoptLattice's non-None result asserted above.
	})

	t.Run("without_the_fetch_route_the_poisoned_cache_would_wrongly_stay_positive", func(t *testing.T) {
		dir := adoptInitRepo(t)
		specBranch := "spec/042-test"
		adoptGitRun(t, dir, "branch", specBranch, "main")
		origin := adoptBareOrigin(t, dir)
		adoptGitRun(t, dir, "push", "-q", "origin", specBranch)
		adoptGitRun(t, dir, "fetch", "-q", "origin", specBranch)
		staleCachedTip := adoptRefHash(t, dir, "origin/"+specBranch)

		clone := t.TempDir()
		adoptGitRun(t, clone, "clone", "-q", origin, ".")
		adoptGitRun(t, clone, "checkout", "-q", specBranch)
		adoptWriteFile(t, clone, "unlanded.txt", "novel unlanded content\n")
		adoptCommit(t, clone, "novel unlanded work")
		adoptGitRun(t, clone, "push", "-q", "origin", specBranch)

		stubAdoptListEpicBeads(t, []adoptEpicBead{})

		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
		if res.Class != adoptClassNegative {
			t.Fatalf("the real evaluator (fetch-then-evaluate) must be negative; got %v", res.Class)
		}
		// Without the fetch route (evaluating the STALE local cache
		// instead), the same fixture's cached tip is still the OLD
		// landed tip — which would evaluate positive, not negative.
		st, _, evalErr := evaluateAgainstMain(dir, staleCachedTip)
		if evalErr != nil {
			t.Fatalf("evaluating the stale cached tip directly: %v", evalErr)
		}
		if st != evidencePositive {
			t.Fatal("fixture no longer demonstrates the poisoned-cache trap: the stale cached tip must itself evaluate positive")
		}
	})
}

// TestEvaluateAdoptLattice_BranchlessBeadCoveredViaFindLandedMerge is
// O1-2: every existing fixture that reaches VERIFIED uses only beads
// WITH a surviving branch; the epic-coverage clause's SECOND
// evidentiary route — a bead with NO surviving branch, covered instead
// via lifecycle.FindLandedMerge over the corroborated spec ref — had
// zero coverage anywhere in this file. This drives that exact route to
// VERIFIED for real: bead/test-b1 is merged into main (a real merge
// commit naming it by subject), then DELETED (no surviving branch), and
// a review/test-b1-r1/panel.json records reviewed_head_sha == the
// bead's tip AT MERGE TIME (the merge commit's second parent) — the
// confirmation leg FindLandedMerge needs when the branch itself is
// gone.
func TestEvaluateAdoptLattice_BranchlessBeadCoveredViaFindLandedMerge(t *testing.T) {
	dir := adoptInitRepo(t)
	specBranch := "spec/042-test"

	adoptGitRun(t, dir, "checkout", "-q", "-b", "bead/test-b1")
	adoptWriteFile(t, dir, "test-b1.txt", "landed work for test-b1\n")
	adoptCommit(t, dir, "work for test-b1")
	beadTip := adoptRefHash(t, dir, "bead/test-b1")
	adoptGitRun(t, dir, "checkout", "-q", "main")
	adoptGitRun(t, dir, "merge", "-q", "--no-ff", "-m", "Merge bead/test-b1", "bead/test-b1")
	adoptGitRun(t, dir, "branch", "-D", "bead/test-b1") // no surviving branch

	adoptGitRun(t, dir, "branch", specBranch, "main")
	adoptBareOrigin(t, dir)
	adoptGitRun(t, dir, "push", "-q", "origin", specBranch)

	adoptWriteFile(t, dir, "review/test-b1-r1/panel.json", fmt.Sprintf(
		`{"bead_id":"test-b1","spec":"042-test","target":"bead/test-b1","round":1,"expected_reviewers":3,"reviewed_head_sha":%q}`,
		beadTip,
	))

	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})

	res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
	if err != nil {
		t.Fatalf("evaluateAdoptLattice: %v", err)
	}
	if res.Class != adoptClassNone {
		t.Fatalf("a branchless bead corroborated via a confirmed FindLandedMerge result must be VERIFIED; got Class=%v Detail=%q", res.Class, res.Detail)
	}

	// Converse (O1-2): WITHOUT the panel.json confirmation, FindLandedMerge
	// fail-closes (no confirmation leg available) and the SAME fixture must
	// NOT verify.
	if err := os.RemoveAll(filepath.Join(dir, "review")); err != nil {
		t.Fatalf("removing review/: %v", err)
	}
	res2, err2 := evaluateAdoptLattice(dir, "042-test", "epic-1", specBranch)
	if err2 != nil {
		t.Fatalf("evaluateAdoptLattice (no confirmation): %v", err2)
	}
	if res2.Class == adoptClassNone {
		t.Fatal("without ANY confirmation leg, FindLandedMerge must fail closed (coverage-unavailable), not silently verify")
	}
}

// TestListEpicBeads_AllStatusesBreadth pins F-r5-2's status-set
// resolution directly, using the REAL listEpicBeads (not the
// adoptListEpicBeadsFn stub the lattice table above uses): the enumerated
// --status= argv must include a project custom status, and a
// present-but-unparseable config.yaml must be an ERROR, never a silently
// narrowed enumeration — never bead.AllStatuses' own missing-file
// degrade.
func TestListEpicBeads_AllStatusesBreadth(t *testing.T) {
	t.Run("custom_status_is_included_in_the_enumeration", func(t *testing.T) {
		dir := adoptInitRepo(t)
		adoptWriteFile(t, dir, ".beads/config.yaml", `status.custom: "triaged"`+"\n")
		var capturedArgs []string
		orig := adoptListBDFn
		t.Cleanup(func() { adoptListBDFn = orig })
		adoptListBDFn = func(args ...string) ([]byte, error) {
			capturedArgs = append([]string(nil), args...)
			return []byte(`[]`), nil
		}
		if _, err := listEpicBeads(dir, "epic-1"); err != nil {
			t.Fatalf("listEpicBeads: %v", err)
		}
		found := false
		for _, a := range capturedArgs {
			if a == "--status=open,in_progress,blocked,closed,triaged" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a --status= argument naming the custom status 'triaged', got args=%v", capturedArgs)
		}
	})

	t.Run("missing_config_is_the_legitimate_no_customs_state", func(t *testing.T) {
		dir := adoptInitRepo(t)
		orig := adoptListBDFn
		t.Cleanup(func() { adoptListBDFn = orig })
		adoptListBDFn = func(args ...string) ([]byte, error) { return []byte(`[]`), nil }
		if _, err := listEpicBeads(dir, "epic-1"); err != nil {
			t.Fatalf("listEpicBeads with no config.yaml: unexpected error: %v", err)
		}
	})

	t.Run("present_but_unparseable_config_is_evidence_error_never_narrowed", func(t *testing.T) {
		dir := adoptInitRepo(t)
		adoptWriteFile(t, dir, ".beads/config.yaml", "{ not: valid: yaml: [")
		orig := adoptListBDFn
		t.Cleanup(func() { adoptListBDFn = orig })
		called := false
		adoptListBDFn = func(args ...string) ([]byte, error) {
			called = true
			return []byte(`[]`), nil
		}
		if _, err := listEpicBeads(dir, "epic-1"); err == nil {
			t.Fatal("expected an error from an unparseable config.yaml, got nil")
		}
		if called {
			t.Fatal("bd must never be invoked when the status-set resolution itself failed (never a silently narrowed enumeration)")
		}
	})

	// O1-1: a WELL-FORMED YAML document whose status.custom value is a
	// shape bead.CustomStatuses' splitCustomList (internal/bead/config.go)
	// cannot extract anything from — a mapping, a scalar bool/number, or a
	// list containing a non-string entry — must ALSO be evidence-error,
	// never silently degrade to bead.AllStatuses' built-ins-only result
	// (which resolveAdoptStatusSet's own file-level parse guard does NOT
	// catch, since the YAML itself parses fine).
	for _, tc := range []struct {
		name string
		yaml string
	}{
		{"mapping_value", "status.custom:\n  weird: mapping\n"},
		{"scalar_bool_value", "status.custom: true\n"},
		{"scalar_number_value", "status.custom: 42\n"},
		{"list_with_non_string_entry", "status.custom:\n  - triaged\n  - 7\n"},
	} {
		t.Run("unrecognized_custom_status_shape_is_evidence_error_never_narrowed/"+tc.name, func(t *testing.T) {
			dir := adoptInitRepo(t)
			adoptWriteFile(t, dir, ".beads/config.yaml", tc.yaml)
			orig := adoptListBDFn
			t.Cleanup(func() { adoptListBDFn = orig })
			called := false
			adoptListBDFn = func(args ...string) ([]byte, error) {
				called = true
				return []byte(`[]`), nil
			}
			if _, err := listEpicBeads(dir, "epic-1"); err == nil {
				t.Fatalf("expected an error from status.custom shape %q, got nil", tc.yaml)
			}
			if called {
				t.Fatal("bd must never be invoked when the status-set resolution itself failed (never a silently narrowed enumeration)")
			}
		})
	}
}

// TestEvaluateAdoptLattice_CustomStatusPoisonsAndResolutionFailureErrors
// is AC-2(ix)'s custom-status pair, run through the REAL listEpicBeads
// (adoptListEpicBeadsFn left at its production default) with only
// adoptListBDFn stubbed — so the lattice's own AllStatuses breadth (not
// a test double standing in for it) is what resolves the custom-status
// bead into the enumeration.
func TestEvaluateAdoptLattice_CustomStatusPoisonsAndResolutionFailureErrors(t *testing.T) {
	dir := adoptInitRepo(t)
	adoptWriteFile(t, dir, ".beads/config.yaml", `status.custom: "triaged"`+"\n")
	makeUnlandedBeadBranch(t, dir, "test-b1")

	orig := adoptListBDFn
	t.Cleanup(func() { adoptListBDFn = orig })
	adoptListBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]map[string]string{{"id": "test-b1", "status": "triaged"}})
	}

	res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
	if err != nil {
		t.Fatalf("evaluateAdoptLattice: %v", err)
	}
	if res.Class != adoptClassNegative {
		t.Fatalf("a custom-status bead's surviving unlanded branch must poison the aggregate; got Class=%v", res.Class)
	}
	if res.Marker != adoptMarkerNegative {
		t.Fatalf("got Marker=%q, want %q (O2-4: each row asserts its own marker)", res.Marker, adoptMarkerNegative)
	}

	// Same fixture, but the status-set resolution itself is now forced to
	// fail (an unparseable config.yaml) -> evidence-error, never a
	// silently narrowed enumeration that would just drop the
	// custom-status bead and (wrongly) resolve some other way.
	adoptWriteFile(t, dir, ".beads/config.yaml", "{ not: valid: yaml: [")
	res2, err2 := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
	if err2 != nil {
		t.Fatalf("evaluateAdoptLattice: %v", err2)
	}
	if res2.Class != adoptClassEvidenceError {
		t.Fatalf("a forced status-set resolution failure must be evidence-error; got Class=%v", res2.Class)
	}
	if res2.Marker != adoptMarkerEvidenceError {
		t.Fatalf("got Marker=%q, want %q (O2-4: each row asserts its own marker)", res2.Marker, adoptMarkerEvidenceError)
	}
}
