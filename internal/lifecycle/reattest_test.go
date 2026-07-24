package lifecycle

// reattest_test.go — spec 125 Bead 4: the R4 git-corroborated re-attest
// engine (AC-7 happy path + idempotent no-op, AC-8 fail-closed +
// non-circular legs, AC-9 audited contradiction overwrite, AC-11's
// re-attest-half seam pin). Every fixture is a real throwaway git repo
// (initLandedRepo and friends from landed_test.go); every bd-touching
// leg runs hermetically through the seams — the write seam
// (reattestBindingFn) is force-stubbed for the whole package by init()
// below so no test can accidentally shell out to a real `bd`, and the
// production default is captured FIRST for the pointer pin.

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// reattestBindingFnDefault captures the PRODUCTION default of
// reattestBindingFn before the hermetic stub below replaces it — package
// var initialization runs before TestMain — so the AC-11 pointer pin can
// assert the real default is bead.MergeMetadata even though every test
// runs behind the stub (the landed_test.go TestMain pattern).
var reattestBindingFnDefault = reattestBindingFn

func init() {
	reattestBindingFn = func(string, map[string]interface{}) error {
		return errors.New("hermetic default: reattestBindingFn not stubbed by this test")
	}
}

// reattestRecorder installs a recording write stub and returns the
// captured writes. Restored via t.Cleanup.
type reattestWrite struct {
	beadID  string
	updates map[string]interface{}
}

func installReattestRecorder(t *testing.T) *[]reattestWrite {
	t.Helper()
	var writes []reattestWrite
	orig := reattestBindingFn
	t.Cleanup(func() { reattestBindingFn = orig })
	reattestBindingFn = func(beadID string, updates map[string]interface{}) error {
		cp := make(map[string]interface{}, len(updates))
		for k, v := range updates {
			cp[k] = v
		}
		writes = append(writes, reattestWrite{beadID: beadID, updates: cp})
		return nil
	}
	return &writes
}

// stubBindingRead points the shared read seam (landedBindingMetadataFn —
// the same one FindLandedMerge consults) at a fixed metadata map.
func stubBindingRead(t *testing.T, meta map[string]interface{}) {
	t.Helper()
	orig := landedBindingMetadataFn
	t.Cleanup(func() { landedBindingMetadataFn = orig })
	landedBindingMetadataFn = func(string) (map[string]interface{}, error) {
		return meta, nil
	}
}

func str(t *testing.T, m map[string]interface{}, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("audit key %q missing or not a string: %#v", key, m[key])
	}
	return v
}

// TestReattest_HappyPathFleetState is AC-7 (RED today — no such surface
// exists before this bead): today's fleet state — closed bead, real
// bead→spec merge carrying git's DEFAULT conflict-recovery subject,
// branch DELETED, no binding. Explicit invocation DERIVES the binding
// from the independent exact-second-parent scan and writes SHAs matching
// the real merge (rev-parse-verified) in ONE call together with the full
// audit record; FindLandedMerge subsequently identifies; a re-run is a
// byte-identical no-op (no second write, no audit churn — the
// time-bearing reattest_at key never dirties a converged state).
func TestReattest_HappyPathFleetState(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-one", "spec/125-test")
	run("branch", "-D", "bead/bead-one")
	mergeSHA := revParseIn(t, dir, "spec/125-test")
	secondParent := revParseIn(t, dir, mergeSHA+"^2")
	firstParent := revParseIn(t, dir, mergeSHA+"^1")

	writes := installReattestRecorder(t)

	res, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor@host via test")
	if err != nil {
		t.Fatalf("ReattestLandedMerge: %v", err)
	}
	if !res.Wrote {
		t.Fatal("expected a write on the bare fleet state")
	}
	if res.MergeSHA != mergeSHA || res.SecondParent != secondParent || res.FirstParent != firstParent {
		t.Errorf("derived identity = %s/%s/%s, want %s/%s/%s",
			res.MergeSHA, res.FirstParent, res.SecondParent, mergeSHA, firstParent, secondParent)
	}
	if len(*writes) != 1 {
		t.Fatalf("writes = %d, want exactly 1", len(*writes))
	}
	w := (*writes)[0]
	if w.beadID != "bead-one" {
		t.Errorf("write bead = %q, want bead-one", w.beadID)
	}
	// The binding: ONLY the scan-derived SHAs (rev-parse-verified above).
	if got := str(t, w.updates, "mindspec_landed_merge_sha"); got != mergeSHA {
		t.Errorf("mindspec_landed_merge_sha = %q, want %q", got, mergeSHA)
	}
	if got := str(t, w.updates, "mindspec_landed_second_parent"); got != secondParent {
		t.Errorf("mindspec_landed_second_parent = %q, want %q", got, secondParent)
	}
	// The audit record, inspectable in the same write (AC-7): actor,
	// RFC3339 timestamp, invoking operation, corroborating datum,
	// before-values (empty when previously absent), scanned branch.
	if got := str(t, w.updates, "mindspec_landed_reattest_actor"); got != "test-actor@host via test" {
		t.Errorf("audit actor = %q", got)
	}
	if at := str(t, w.updates, "mindspec_landed_reattest_at"); at == "" {
		t.Error("audit timestamp missing")
	} else if _, perr := time.Parse(time.RFC3339, at); perr != nil {
		t.Errorf("audit timestamp %q is not RFC3339: %v", at, perr)
	}
	if got := str(t, w.updates, "mindspec_landed_reattest_op"); got != "mindspec reattest" {
		t.Errorf("audit op = %q, want %q", got, "mindspec reattest")
	}
	corr := str(t, w.updates, "mindspec_landed_reattest_corroboration")
	if !strings.Contains(corr, "(a)") || !strings.Contains(corr, mergeSHA) {
		t.Errorf("audit corroboration must name datum (a) and the derived merge, got %q", corr)
	}
	if got := str(t, w.updates, "mindspec_landed_reattest_prior_merge_sha"); got != "" {
		t.Errorf("prior merge sha = %q, want empty (previously absent)", got)
	}
	if got := str(t, w.updates, "mindspec_landed_reattest_prior_second_parent"); got != "" {
		t.Errorf("prior second parent = %q, want empty (previously absent)", got)
	}
	if got := str(t, w.updates, "mindspec_landed_reattest_scanned_branch"); got != "spec/125-test" {
		t.Errorf("audit scanned branch = %q, want spec/125-test", got)
	}

	// FindLandedMerge subsequently identifies the branch-deleted bead from
	// the re-attested binding (the MF-3 contract).
	stubBindingRead(t, w.updates)
	landed, ferr := FindLandedMerge(dir, "spec/125-test", "bead-one")
	if ferr != nil {
		t.Fatalf("FindLandedMerge after reattest: %v", ferr)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("FindLandedMerge SHA = %q, want %q", landed.SHA, mergeSHA)
	}

	// Idempotent re-run: already-correct binding → convergent NO-OP,
	// nothing written at all (byte-identical metadata, no audit churn).
	res2, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor@host via test")
	if err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if res2.Wrote {
		t.Error("re-run must be a no-op (Wrote=false)")
	}
	if !strings.Contains(res2.Corroboration, "(d)") {
		t.Errorf("no-op corroboration must include datum (d) existing binding, got %q", res2.Corroboration)
	}
	if len(*writes) != 1 {
		t.Errorf("re-run wrote again (writes=%d) — the no-op must not churn the audit", len(*writes))
	}
}

// TestReattest_RefusesTrulyBare is AC-8(i)'s no-corroboration leg: no
// merge on the scanned branch names the bead at all — refuse, write
// NOTHING (metadata byte-identical), and name the audited ADR-0035
// mindspec-q9ea human attested-restore exit BY NAME.
func TestReattest_RefusesTrulyBare(t *testing.T) {
	dir, _ := initLandedRepo(t, "125-test")
	writes := installReattestRecorder(t)

	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	if !errors.Is(err, ErrReattestRefused) {
		t.Fatalf("expected ErrReattestRefused, got %v", err)
	}
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateNoOwnedMerge {
		t.Fatalf("expected %s refusal, got %v", ReattestStateNoOwnedMerge, err)
	}
	if !strings.Contains(refusal.Detail, "q9ea") {
		t.Errorf("truly-bare refusal must name the q9ea attested-restore exit, got %q", refusal.Detail)
	}
	if len(*writes) != 0 {
		t.Errorf("refusal wrote %d times — a refusal must write NOTHING", len(*writes))
	}
}

// TestReattest_RefusesAnonymousMerge: a REAL two-parent merge exists but
// its subject names NO bead — the explicit path still requires the
// subject-ownership nominator, so operator invocation alone cannot
// nominate an anonymous merge (the non-circularity floor: with no
// nominator and no assertable pair, there is nothing to corroborate).
func TestReattest_RefusesAnonymousMerge(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	// A real branch-and-merge whose merge subject is wholly custom.
	run("checkout", "-b", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "work.txt"), []byte("work\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work")
	run("checkout", "spec/125-test")
	run("merge", "--no-ff", "-m", "land the payload work", "bead/bead-one")
	run("branch", "-D", "bead/bead-one")

	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateNoOwnedMerge {
		t.Fatalf("an anonymous-subject merge must NOT be nominable even under explicit invocation, got %v", err)
	}
	if len(*writes) != 0 {
		t.Errorf("anonymous merge attested (%d writes) — ownership nomination bypassed", len(*writes))
	}
}

// TestReattest_RefusesDescendantMerge is AC-8(ii)'s descendant leg: bead
// X never landed its own merge; a DESCENDANT bead Y (branched after X's
// work reached spec via Y's line) landed with a default subject. An
// ancestor-consistent or topology-only impl attests M_Y for X; the
// ownership rule refuses (M_Y's subject names Y, not X).
func TestReattest_RefusesDescendantMerge(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-y", "spec/125-test")
	run("branch", "-D", "bead/bead-y")

	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-x", "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateNoOwnedMerge {
		t.Fatalf("another bead's merge must never be attested for bead-x, got %v", err)
	}
	if len(*writes) != 0 {
		t.Errorf("descendant/other-bead merge attested (%d writes)", len(*writes))
	}
}

// TestReattest_RefusesDecoyContradictingSurvivingTip is AC-8(ii)'s decoy
// leg: a crafted merge whose subject names the bead but whose second
// parent contradicts the bead's SURVIVING branch tip (datum (b)) —
// refused, nothing written.
func TestReattest_RefusesDecoyContradictingSurvivingTip(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	// The bead's real (surviving, unmerged) branch.
	run("checkout", "-b", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "real.txt"), []byte("real\n"), 0644)
	run("add", ".")
	run("commit", "-m", "real work")
	// An unrelated commit to serve as the decoy's second parent.
	run("checkout", "main")
	run("checkout", "-b", "unrelated")
	os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated")
	unrelatedSHA := revParseIn(t, dir, "unrelated")
	// The decoy merge on spec, subject naming the bead, second parent the
	// unrelated commit.
	run("checkout", "spec/125-test")
	specTip := revParseIn(t, dir, "spec/125-test")
	commitResolvedMerge(t, dir, specTip, unrelatedSHA, "Merge bead/bead-one")

	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateTipContradiction {
		t.Fatalf("expected %s refusal on the decoy, got %v", ReattestStateTipContradiction, err)
	}
	if len(*writes) != 0 {
		t.Errorf("decoy attested (%d writes)", len(*writes))
	}
}

// TestReattest_RefusesAmbiguousSecondParents: two owned merges DISAGREE
// on the landed tip (different second parents) — genuine ambiguity, and
// with no surviving write-time ground truth the engine refuses rather
// than guessing the newest (spec 125 R4 fail-closed rule; note this is
// deliberately STRICTER than FindLandedMerge, which evaluates the newest
// candidate under its corroboration data).
func TestReattest_RefusesAmbiguousSecondParents(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBead(t, run, dir, "bead-one", "spec/125-test")
	// The branch advances and is merged AGAIN — second landing with a
	// DIFFERENT second parent.
	run("checkout", "bead/bead-one")
	os.WriteFile(filepath.Join(dir, "more.txt"), []byte("more\n"), 0644)
	run("add", ".")
	run("commit", "-m", "more work")
	run("checkout", "spec/125-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-one", "bead/bead-one")
	run("branch", "-D", "bead/bead-one")

	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateAmbiguous {
		t.Fatalf("expected %s refusal, got %v", ReattestStateAmbiguous, err)
	}
	if len(*writes) != 0 {
		t.Errorf("ambiguous state attested (%d writes)", len(*writes))
	}
}

// TestReattest_PanelShaContradictionRefuses: a registered panel's
// reviewed_head_sha (datum (c)) that does not EQUAL the owned match's
// second parent contradicts — refused; and an EQUAL one corroborates
// (named in the audit datum).
func TestReattest_PanelShaContradictionRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-one", "spec/125-test")
	run("branch", "-D", "bead/bead-one")
	mergeSHA := revParseIn(t, dir, "spec/125-test")
	secondParent := revParseIn(t, dir, mergeSHA+"^2")
	// An unrelated commit as the contradicting panel SHA.
	run("checkout", "main")
	run("checkout", "-b", "unrelated")
	os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated")
	unrelatedSHA := revParseIn(t, dir, "unrelated")
	run("checkout", "spec/125-test")

	beadID := "bead-one"
	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{
			Dir:   "/fake/review/one",
			Panel: panel.Panel{BeadID: &beadID, ReviewedHeadSHA: unrelatedSHA},
		}}
	}

	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStatePanelContradiction {
		t.Fatalf("expected %s refusal, got %v", ReattestStatePanelContradiction, err)
	}
	if len(*writes) != 0 {
		t.Errorf("panel-contradicted candidate attested (%d writes)", len(*writes))
	}

	// The agreeing panel corroborates and is named as datum (c).
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{
			Dir:   "/fake/review/one",
			Panel: panel.Panel{BeadID: &beadID, ReviewedHeadSHA: secondParent},
		}}
	}
	res, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	if err != nil {
		t.Fatalf("agreeing panel must corroborate: %v", err)
	}
	if !strings.Contains(res.Corroboration, "(c)") {
		t.Errorf("corroboration must name datum (c), got %q", res.Corroboration)
	}
}

// TestReattest_SurvivingTipCorroborates: a surviving branch whose tip
// EQUALS the derived second parent agrees (datum (b), named in the
// audit) — the write proceeds.
func TestReattest_SurvivingTipCorroborates(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-one", "spec/125-test")

	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	if err != nil {
		t.Fatalf("ReattestLandedMerge: %v", err)
	}
	if !strings.Contains(res.Corroboration, "(b)") {
		t.Errorf("corroboration must name datum (b) surviving tip, got %q", res.Corroboration)
	}
	if len(*writes) != 1 {
		t.Errorf("writes = %d, want 1", len(*writes))
	}
}

// TestReattest_ContradictoryBindingOverwrittenAudited is AC-9: a bead
// carrying a stale/contradictory binding — the engine overwrites ONLY
// with the git-corroborated exact identity (G3-1) and records the PRIOR
// values in the same inspectable audit write, so a wrong backfill is
// reconstructable BY INSPECTION of the before/after values. It never
// silently keeps the contradiction and never writes an uncorroborated
// replacement.
func TestReattest_ContradictoryBindingOverwrittenAudited(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-one", "spec/125-test")
	run("branch", "-D", "bead/bead-one")
	mergeSHA := revParseIn(t, dir, "spec/125-test")
	secondParent := revParseIn(t, dir, mergeSHA+"^2")

	staleSHA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	staleSP := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	stubBindingRead(t, map[string]interface{}{
		"mindspec_landed_merge_sha":     staleSHA,
		"mindspec_landed_second_parent": staleSP,
	})

	writes := installReattestRecorder(t)
	res, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	if err != nil {
		t.Fatalf("ReattestLandedMerge: %v", err)
	}
	if !res.Wrote {
		t.Fatal("a contradictory binding must be overwritten, not kept")
	}
	if len(*writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(*writes))
	}
	w := (*writes)[0]
	if got := str(t, w.updates, "mindspec_landed_merge_sha"); got != mergeSHA {
		t.Errorf("overwrite merge sha = %q, want the git-corroborated %q", got, mergeSHA)
	}
	if got := str(t, w.updates, "mindspec_landed_second_parent"); got != secondParent {
		t.Errorf("overwrite second parent = %q, want %q", got, secondParent)
	}
	if got := str(t, w.updates, "mindspec_landed_reattest_prior_merge_sha"); got != staleSHA {
		t.Errorf("audit prior merge sha = %q, want the stale %q (reconstructability)", got, staleSHA)
	}
	if got := str(t, w.updates, "mindspec_landed_reattest_prior_second_parent"); got != staleSP {
		t.Errorf("audit prior second parent = %q, want the stale %q", got, staleSP)
	}
	if res.PriorMergeSHA != staleSHA || res.PriorSecondParent != staleSP {
		t.Errorf("result prior = %q/%q, want %q/%q", res.PriorMergeSHA, res.PriorSecondParent, staleSHA, staleSP)
	}
}

// TestReattest_RevertedRefuses: the bead's landed content was genuinely
// reverted on spec after landing — Requirement 3's discrimination
// (CleanDivergence + RevertShape) classifies REVERTED, and the engine
// refuses rather than re-attesting reverted work.
func TestReattest_RevertedRefuses(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBead(t, run, dir, "bead-one", "spec/125-test")
	mergeSHA := revParseIn(t, dir, "spec/125-test")
	run("revert", "--no-edit", "-m", "1", mergeSHA)
	run("branch", "-D", "bead/bead-one")

	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateReverted {
		t.Fatalf("expected %s refusal, got %v", ReattestStateReverted, err)
	}
	if len(*writes) != 0 {
		t.Errorf("reverted content attested (%d writes)", len(*writes))
	}
}

// TestReattest_MaskedRevertOldestAnchored mirrors AC-2e onto the
// re-attest surface (plan Step 1's same-second-parent rule): M₁ lands,
// the content is reverted, then the SAME second parent is re-merged as
// an EMPTY no-op M₂ — the content is ABSENT at the tip, but M₂'s own
// first parent is the post-revert state. The engine must anchor the R3
// check on the OLDEST merge M₁ and refuse; a newest-anchored impl reads
// "no change" and mis-attests.
func TestReattest_MaskedRevertOldestAnchored(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	run("checkout", "-b", "bead/bead-x")
	os.WriteFile(filepath.Join(dir, "payload.txt"), []byte("the payload\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work bead-x")
	xTip := revParseIn(t, dir, "bead/bead-x")
	run("checkout", "spec/125-test")
	run("merge", "--no-ff", "-m", "Merge bead/bead-x", "bead/bead-x")
	m1 := revParseIn(t, dir, "spec/125-test")
	m1FirstParent := revParseIn(t, dir, m1+"^1")
	run("revert", "--no-edit", "-m", "1", m1)
	postRevertTip := revParseIn(t, dir, "spec/125-test")
	commitResolvedMerge(t, dir, postRevertTip, xTip, "Merge bead/bead-x")
	run("branch", "-D", "bead/bead-x")

	// Recording wrappers pin the M₁ anchoring for BOTH R3 parameters.
	var subBase, subRef string
	origSub := landedContentSubsumedFn
	t.Cleanup(func() { landedContentSubsumedFn = origSub })
	landedContentSubsumedFn = func(workdir, base, ref, target string) (gitutil.Subsumption, error) {
		subBase, subRef = base, ref
		return origSub(workdir, base, ref, target)
	}

	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-x", "test-actor")
	var refusal *ReattestRefusal
	if !errors.As(err, &refusal) || refusal.State != ReattestStateReverted {
		t.Fatalf("masked revert must refuse as %s (newest-anchored impls mis-attest), got %v", ReattestStateReverted, err)
	}
	if subRef != m1 {
		t.Errorf("R3 theirs/ref anchored on %q, want the OLDEST merge M₁ %q", subRef, m1)
	}
	if subBase != m1FirstParent {
		t.Errorf("R3 base anchored on %q, want M₁^1 %q", subBase, m1FirstParent)
	}
	if len(*writes) != 0 {
		t.Errorf("masked revert attested (%d writes)", len(*writes))
	}
}

// TestReattest_WriteFailurePropagates: a failed binding write is an
// ERROR (never a silent success and never a refusal-shaped state).
func TestReattest_WriteFailurePropagates(t *testing.T) {
	dir, run := initLandedRepo(t, "125-test")
	mergeBeadDefaultSubject(t, run, dir, "bead-one", "spec/125-test")
	run("branch", "-D", "bead/bead-one")

	orig := reattestBindingFn
	t.Cleanup(func() { reattestBindingFn = orig })
	reattestBindingFn = func(string, map[string]interface{}) error {
		return errors.New("injected write failure")
	}
	_, err := ReattestLandedMerge(dir, "spec/125-test", "bead-one", "test-actor")
	if err == nil || errors.Is(err, ErrReattestRefused) {
		t.Fatalf("expected a propagated write error, got %v", err)
	}
	if !strings.Contains(err.Error(), "injected write failure") {
		t.Errorf("write failure not propagated: %v", err)
	}
}

// TestReattest_EmptyActorRefused: the audit record is mandatory — an
// empty acting identity is a programmer/caller error, not a write with
// a hole in the audit.
func TestReattest_EmptyActorRefused(t *testing.T) {
	writes := installReattestRecorder(t)
	_, err := ReattestLandedMerge(t.TempDir(), "spec/125-test", "bead-one", "  ")
	if err == nil {
		t.Fatal("expected an error on empty actor")
	}
	if len(*writes) != 0 {
		t.Errorf("empty-actor call wrote (%d writes)", len(*writes))
	}
}

// TestReattestSeamDefaultsPinned is AC-11(i)'s re-attest half: the
// PRODUCTION default of reattestBindingFn is the real bead.MergeMetadata
// (pointer equality — the netEffectLandedFn anti-drift pattern), so the
// hermetic tests above provably exercise the real bd write path and the
// seam cannot be silently rewired. The read half (landedBindingMetadataFn
// → bead.GetMetadata) is pinned by TestLandedBindingMetadataFnDefaultPinned
// in landed_test.go (spec 125 plan F3-2); the executor halves
// (mergeBindingFn/mergeBindingReadFn/locateLandedMergeFn) by Bead 1's
// pins in internal/executor/merge_binding_test.go.
func TestReattestSeamDefaultsPinned(t *testing.T) {
	if reflect.ValueOf(reattestBindingFnDefault).Pointer() != reflect.ValueOf(bead.MergeMetadata).Pointer() {
		t.Error("reattestBindingFn production default must be bead.MergeMetadata (AC-11(i) re-attest half) — the re-attest write gate went hollow")
	}
	if reflect.ValueOf(reattestNowFn).Pointer() != reflect.ValueOf(time.Now).Pointer() {
		t.Error("reattestNowFn production default must be time.Now — the audit timestamp source drifted")
	}
}
