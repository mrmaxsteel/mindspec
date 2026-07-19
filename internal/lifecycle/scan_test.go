package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// stubScanGitSeams pins the scan's git-side seams to a healthy state:
// no finalize carriers, and a main-committed export served from memory.
func stubScanGitSeams(t *testing.T, branches []string, committedJSONL string) {
	t.Helper()
	origBranches := localBranchRefsFn
	origIsAncestor := finalizeOrphanIsAncestorFn
	origFileAtRef := fileAtRefFn
	origCommitCount := finalizeOrphanCommitCountFn
	origDiffStat := finalizeOrphanDiffStatFn
	t.Cleanup(func() {
		localBranchRefsFn = origBranches
		finalizeOrphanIsAncestorFn = origIsAncestor
		fileAtRefFn = origFileAtRef
		finalizeOrphanCommitCountFn = origCommitCount
		finalizeOrphanDiffStatFn = origDiffStat
	})
	localBranchRefsFn = func(workdir string) ([]string, error) { return branches, nil }
	finalizeOrphanIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return false, nil }
	finalizeOrphanCommitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }
	finalizeOrphanDiffStatFn = func(workdir, base, head string) (string, error) { return "1 file changed", nil }
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) { return []byte(committedJSONL), nil }
}

// TestScanIntegrityFindings_SubprocessBudget is the final-review F1
// regression pin, tightened per r2 F1/G2: with 118 on-disk spec
// directories, the aggregate scan performs EXACTLY one all-epics bd query
// and EXACTLY one global open-beads query — an O(1) budget with ZERO
// per-spec-dir and ZERO per-epic subprocesses (the shipped Bead-2 wiring
// issued ~4 uncached bd calls PER DIRECTORY, measured at minutes-to-never
// for a full doctor run). The exact ==1 on the open-beads query is the
// mutation pin for the stale-OPEN leg's ENUMERATION: disabling the leg
// drops the query to 0 and this test goes RED (the positive-detection
// pins live in the StaleOpen fixtures below).
func TestScanIntegrityFindings_SubprocessBudget(t *testing.T) {
	root := t.TempDir()
	// 118 on-disk spec dirs the scan must be entirely INDEPENDENT of.
	for i := 1; i <= 118; i++ {
		if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs", fmt.Sprintf("%03d-historical", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var epicListCalls, openListCalls, parentListCalls, otherListCalls, showCalls int
	epicJSON := `[
		{"id":"epic-active","title":"[SPEC 118-active] live","status":"in_progress","issue_type":"epic","metadata":{"spec_num":118,"spec_title":"active"}},
		{"id":"epic-done","title":"[SPEC 001-historical] done","status":"closed","issue_type":"epic","metadata":{"spec_num":1,"spec_title":"historical"}}
	]`
	// The global open/in_progress enumeration returns the active epic
	// itself plus one open child; neither yields a finding here (the epic
	// is parentless so it is filtered in-process, and the child's
	// MergedUnclosed errors against the non-git root — best-effort skip).
	openJSON := `[
		{"id":"epic-active","title":"[SPEC 118-active] live","status":"in_progress","issue_type":"epic"},
		{"id":"b3","title":"open bead","status":"open","issue_type":"task","parent":"epic-active"}
	]`
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "--type=epic"):
			epicListCalls++
			return []byte(epicJSON), nil
		case len(args) > 0 && args[0] == "--parent":
			parentListCalls++
			return []byte("[]"), nil
		case strings.Contains(joined, "--status=open,in_progress"):
			openListCalls++
			return []byte(openJSON), nil
		default:
			otherListCalls++
			return []byte("[]"), nil
		}
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		showCalls++
		return []byte("[]"), nil
	}))
	// Healthy git side: no carriers; main's export agrees (epic-done closed).
	stubScanGitSeams(t, []string{"main", "spec/118-active"}, `{"id":"epic-done","status":"closed"}`+"\n")

	findings := ScanIntegrityFindings(root, phase.NewCache())

	if len(findings.FinalizeBranches)+len(findings.StaleOpen)+len(findings.StaleTrackers) != 0 {
		t.Errorf("healthy fixture must yield zero findings, got %+v", findings)
	}
	if epicListCalls != 1 {
		t.Errorf("all-epics bd query must run EXACTLY once, ran %d times", epicListCalls)
	}
	if openListCalls != 1 {
		t.Errorf("global open-beads bd query must run EXACTLY once (the stale-OPEN leg's enumeration), ran %d times", openListCalls)
	}
	if parentListCalls != 0 {
		t.Errorf("zero per-epic children bd queries allowed (global enumeration replaced them), ran %d", parentListCalls)
	}
	if otherListCalls != 0 || showCalls != 0 {
		t.Errorf("zero per-dir/per-spec bd queries allowed, got %d other list + %d show calls", otherListCalls, showCalls)
	}
}

// TestScanIntegrityFindings_FindsDivergence: the aggregate reports all
// three finding kinds from the tracker-driven enumeration — an unmerged
// finalize carrier, and a live-closed epic whose committed status on main
// is still open — with the SAME single-home message text the individual
// predicates produce.
func TestScanIntegrityFindings_FindsDivergence(t *testing.T) {
	root := t.TempDir()

	epicJSON := `[{"id":"epic-1","title":"[SPEC 119-test] fixture","status":"closed","issue_type":"epic","metadata":{"spec_num":119,"spec_title":"test"}}]`
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return []byte(epicJSON), nil
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	stubScanGitSeams(t, []string{"main", "chore/finalize-119-test"}, `{"id":"epic-1","status":"open"}`+"\n")

	findings := ScanIntegrityFindings(root, phase.NewCache())

	if len(findings.FinalizeBranches) != 1 || findings.FinalizeBranches[0].Branch != "chore/finalize-119-test" {
		t.Errorf("expected the unmerged carrier finding, got %+v", findings.FinalizeBranches)
	}
	if len(findings.StaleTrackers) != 1 {
		t.Fatalf("expected the stale-tracker finding, got %+v", findings.StaleTrackers)
	}
	// Message parity with the single-home predicate.
	want, err := StaleTrackerOnMain(root, "119-test", "epic-1", true)
	if err != nil || want == nil {
		t.Fatalf("predicate cross-check failed: %v %v", want, err)
	}
	if findings.StaleTrackers[0].FullMessage() != want.FullMessage() {
		t.Errorf("aggregate stale-tracker text %q must equal predicate text %q", findings.StaleTrackers[0].FullMessage(), want.FullMessage())
	}
}

// TestScanIntegrityFindings_UnreadableExportNeverGuesses: when main's
// committed export cannot be read, live-closed epics yield NO stale-tracker
// assertion (cannot check ≠ divergent), and the other legs still run.
func TestScanIntegrityFindings_UnreadableExportNeverGuesses(t *testing.T) {
	root := t.TempDir()

	epicJSON := `[{"id":"epic-1","title":"[SPEC 119-test] fixture","status":"closed","issue_type":"epic","metadata":{"spec_num":119,"spec_title":"test"}}]`
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return []byte(epicJSON), nil
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	stubScanGitSeams(t, []string{"main"}, "")
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) {
		return nil, fmt.Errorf("simulated git show failure")
	}

	findings := ScanIntegrityFindings(root, phase.NewCache())
	if len(findings.StaleTrackers) != 0 {
		t.Errorf("an unreadable export must never be asserted divergent, got %+v", findings.StaleTrackers)
	}
}

// stubScanBDLayer serves the two bd queries the aggregate issues — the
// epic enumeration and the global open/in_progress enumeration — from
// in-memory JSON, failing the test on any OTHER bd traffic (the O(1)
// budget holds even in the positive fixtures).
func stubScanBDLayer(t *testing.T, epicJSON, openJSON string) {
	t.Helper()
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "--type=epic"):
			return []byte(epicJSON), nil
		case strings.Contains(joined, "--status=open,in_progress") && (len(args) == 0 || args[0] != "--parent"):
			return []byte(openJSON), nil
		default:
			t.Errorf("unexpected bd list traffic from the aggregate scan: %v", args)
			return []byte("[]"), nil
		}
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		t.Errorf("unexpected bd show traffic from the aggregate scan: %v", args)
		return []byte("[]"), nil
	}))
}

// TestScanIntegrityFindings_StaleOpenActiveEpic (final-review r2, F1/G2)
// is the aggregate-level POSITIVE pin for the stale-OPEN leg: an ACTIVE
// (in_progress) epic with an OPEN child whose bead/<id> work provably
// landed on the spec branch (a real-git landed merge, MergedUnclosed
// true) MUST surface through ScanIntegrityFindings.StaleOpen with the
// single-home Message() text. Pre-r2 no test exercised this leg through
// the aggregate — disabling it entirely kept every lifecycle/doctor/
// instruct suite green. This test goes RED if the leg is removed.
func TestScanIntegrityFindings_StaleOpenActiveEpic(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")

	stubScanBDLayer(t,
		`[{"id":"epic-1","title":"[SPEC 119-test] fixture","status":"in_progress","issue_type":"epic","metadata":{"spec_num":119,"spec_title":"test"}}]`,
		`[
			{"id":"epic-1","title":"[SPEC 119-test] fixture","status":"in_progress","issue_type":"epic"},
			{"id":"one","title":"landed but still open","status":"open","issue_type":"task","parent":"epic-1"}
		]`,
	)
	// Healthy git side otherwise: no carriers, and main's export is not
	// consulted for a non-closed epic.
	stubScanGitSeams(t, []string{"main", "spec/119-test"}, "")

	findings := ScanIntegrityFindings(dir, phase.NewCache())

	if len(findings.StaleOpen) != 1 {
		t.Fatalf("the aggregate MUST report the landed-but-open bead, got %+v", findings.StaleOpen)
	}
	got := findings.StaleOpen[0]
	if got.BeadID != "one" || got.SpecBranch != "spec/119-test" || got.LandedSHA == "" {
		t.Errorf("finding fields wrong: %+v", got)
	}
	// Message parity with the single-home predicate (AC-15/P8): the
	// aggregate result must be byte-identical to FindStaleOpenBeads' for
	// the same repo state.
	stubStaleOpenSeams(t, "epic-1", nil, []bead.BeadInfo{{ID: "one", Status: "open"}}, nil)
	want, err := FindStaleOpenBeads("119-test", dir)
	if err != nil || len(want) != 1 {
		t.Fatalf("predicate cross-check failed: %v %v", want, err)
	}
	if got.Message() != want[0].Message() {
		t.Errorf("aggregate stale-open text %q must equal predicate text %q", got.Message(), want[0].Message())
	}
}

// TestScanIntegrityFindings_StaleOpenUnderClosedEpic (final-review r2,
// F1/G3): a CLOSED epic with an OPEN child whose work fully landed MUST
// still be reported — the child's tracker status and landed-git state are
// the trigger, never the parent epic's status (Requirement 5/AC-10; the
// per-active-epic children query this replaced silently dropped exactly
// this case, a regression vs the pre-perf all-specs doctor). The
// committed export AGREES the epic is closed, so no stale-tracker finding
// muddies the assertion.
func TestScanIntegrityFindings_StaleOpenUnderClosedEpic(t *testing.T) {
	// R4: RecoveryCommand() now idrender.Bead's the BeadID field (spec 120
	// Bead 5 fix-up) — a valid, idvalidate.BeadID-conformant id is required
	// here so the byte-identical render path is exercised rather than the
	// forced-quote path a bare placeholder like "one" would trigger.
	const beadID = "mindspec-9if1"
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, beadID, "spec/119-test")

	stubScanBDLayer(t,
		`[{"id":"epic-1","title":"[SPEC 119-test] fixture","status":"closed","issue_type":"epic","metadata":{"spec_num":119,"spec_title":"test"}}]`,
		`[{"id":"`+beadID+`","title":"landed but still open","status":"open","issue_type":"task","parent":"epic-1"}]`,
	)
	stubScanGitSeams(t, []string{"main", "spec/119-test"}, `{"id":"epic-1","status":"closed"}`+"\n")

	findings := ScanIntegrityFindings(dir, phase.NewCache())

	if len(findings.StaleTrackers) != 0 {
		t.Errorf("agreeing committed export must yield no stale-tracker finding, got %+v", findings.StaleTrackers)
	}
	if len(findings.StaleOpen) != 1 {
		t.Fatalf("a stale-OPEN child under a CLOSED epic MUST be reported, got %+v", findings.StaleOpen)
	}
	got := findings.StaleOpen[0]
	if got.BeadID != beadID || got.SpecBranch != "spec/119-test" || got.LandedSHA == "" {
		t.Errorf("finding fields wrong: %+v", got)
	}
	if want := "mindspec complete " + beadID; got.RecoveryCommand() != want {
		t.Errorf("RecoveryCommand = %q, want %q", got.RecoveryCommand(), want)
	}
}
