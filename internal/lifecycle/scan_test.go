package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
// regression pin: with 118 on-disk spec directories and ONE active epic,
// the aggregate scan performs exactly one all-epics bd query and at most
// one children query — ZERO per-spec-dir epic/status/children subprocesses
// (the shipped Bead-2 wiring issued ~4 uncached bd calls PER DIRECTORY,
// measured at minutes-to-never for a full doctor run).
func TestScanIntegrityFindings_SubprocessBudget(t *testing.T) {
	root := t.TempDir()
	// 118 on-disk spec dirs the scan must be entirely INDEPENDENT of.
	for i := 1; i <= 118; i++ {
		if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs", fmt.Sprintf("%03d-historical", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var epicListCalls, parentListCalls, otherListCalls, showCalls int
	epicJSON := `[
		{"id":"epic-active","title":"[SPEC 118-active] live","status":"in_progress","issue_type":"epic","metadata":{"spec_num":118,"spec_title":"active"}},
		{"id":"epic-done","title":"[SPEC 001-historical] done","status":"closed","issue_type":"epic","metadata":{"spec_num":1,"spec_title":"historical"}}
	]`
	childrenJSON := `[
		{"id":"b1","title":"done bead","status":"closed","issue_type":"task"},
		{"id":"b2","title":"done bead","status":"closed","issue_type":"task"}
	]`
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "--type=epic"):
			epicListCalls++
			return []byte(epicJSON), nil
		case len(args) > 0 && args[0] == "--parent":
			parentListCalls++
			return []byte(childrenJSON), nil
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
	if parentListCalls > 1 {
		t.Errorf("children bd query must run at most once (one ACTIVE epic), ran %d times", parentListCalls)
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
