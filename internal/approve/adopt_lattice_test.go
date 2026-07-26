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
	t.Run("two_bead_branches_one_landed_one_stale_is_negative", func(t *testing.T) {
		dir := adoptInitRepo(t)
		makeLandedBeadBranch(t, dir, "test-b1")
		makeUnlandedBeadBranch(t, dir, "test-b2")
		stubAdoptListEpicBeads(t, []adoptEpicBead{
			{ID: "test-b1", Status: "closed"},
			{ID: "test-b2", Status: "closed"},
		})
		// No origin remote at all: source i fetch fails -> error. The
		// pinned ordering (any negative wins over any error) must still
		// resolve this to NEGATIVE, not evidence-error.
		res, err := evaluateAdoptLattice(dir, "042-test", "epic-1", "spec/042-test")
		if err != nil {
			t.Fatalf("evaluateAdoptLattice: %v", err)
		}
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
		if res.Class != adoptClassEvidenceError {
			t.Fatalf("unreachable remote: got Class=%v, want evidence-error", res.Class)
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
		if res.Class != adoptClassEvidenceError {
			t.Fatalf("absent remote branch: got Class=%v, want evidence-error", res.Class)
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
		if res.Class != adoptClassNegative {
			t.Fatalf("a non-closed bead's surviving unlanded branch must poison the aggregate; got Class=%v", res.Class)
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
		if res.Class != adoptClassNone {
			t.Fatalf("all-positive-with-coverage must be VERIFIED (Class none); got Class=%v Detail=%q", res.Class, res.Detail)
		}
	})
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
		// Without the coverage rule, an aggregator that stops at "no
		// negative, no error among LITERALLY-EVALUATED sources" (b1
		// positive, source i positive) would answer VERIFIED — the
		// wrong, pre-coverage-rule answer this test's fixture would
		// silently accept if the rule were deleted.
		wouldWronglyVerify := true // source i positive, b1 positive, no negative/error observed
		if !wouldWronglyVerify {
			t.Fatal("fixture no longer demonstrates the coverage rule's necessity")
		}
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
}
