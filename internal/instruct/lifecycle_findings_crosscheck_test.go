package instruct

// AC-15 cross-consumer proof: `mindspec doctor` and the generated
// `mindspec instruct` idle guidance must surface the IDENTICAL finalize-
// orphan finding text from ONE planted fixture. This test drives the REAL
// shared aggregate (lifecycle.ScanIntegrityFindings — left UNSTUBBED in
// both consumers, final-review F1) and the REAL internal/lifecycle
// predicates over a real throwaway git repo, stubbing only the bd process
// layer (phase.SetListJSONForTest / phase.SetRunBDForTest) so no live `bd`
// is required. Since neither consumer re-derives or reformats the finding
// text (both call lifecycle.FinalizeOrphan.FullMessage() verbatim), byte
// equality here is the load-bearing half of AC-15 that the AC-12 identity
// pin (in each package's own test file) cannot show on its own — this test
// exercises the ACTUAL scan over ACTUAL git state, not just seam pointers.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// buildFinalizeOrphanFixture creates a real git repo with a surviving
// chore/finalize-119-test branch that is provably unmerged relative to the
// materialized origin/main ref (FindOutstandingFinalizeBranches' trigger,
// G1-refined) and a main branch whose committed .beads/issues.jsonl shows
// epic-1 as "open" (the stale-tracker trigger, once bd's live state — the
// stubbed epic list — reports it closed).
func buildFinalizeOrphanFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
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
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, ".beads", "issues.jsonl"),
		[]byte(`{"id":"epic-1","status":"open"}`+"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")

	run("checkout", "-b", "chore/finalize-119-test")
	if err := os.WriteFile(filepath.Join(dir, "carrier.txt"), []byte("stranded work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "stranded finalize work")
	run("checkout", "main")
	// G1 (spec 119 final-review): FindOutstandingFinalizeBranches now
	// confirms un-mergedness via IsAncestor(branch, origin/main) before
	// flagging. Materialize the remote-tracking ref at main's tip so the
	// carrier (one commit ahead) is PROVABLY unmerged rather than
	// unverifiable (no network involved — locally available truth only).
	run("update-ref", "refs/remotes/origin/main", "main")

	if err := os.MkdirAll(filepath.Join(dir, ".mindspec", "specs", "119-test"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// stubFixtureBDLayer serves the shared bd epic enumeration BOTH consumers'
// aggregate scans issue: one live epic, epic-1, CLOSED, with spec metadata
// resolving to spec 119-test (SpecIDFromMetadata(119, "test")). Closed
// epics take no children query, but the `--parent` case is served
// defensively.
func stubFixtureBDLayer(t *testing.T) {
	t.Helper()
	epicJSON := `[{"id":"epic-1","title":"[SPEC 119-test] fixture epic","status":"closed","issue_type":"epic","metadata":{"spec_num":119,"spec_title":"test","mindspec_phase":"done"}}]`
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return []byte(epicJSON), nil
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte(epicJSON), nil
	}))
}

func TestLifecycleFindings_FinalizeOrphan_SameTextInDoctorAndInstruct(t *testing.T) {
	root := buildFinalizeOrphanFixture(t)
	stubFixtureBDLayer(t)

	report := doctor.RunLifecycleIntegrityChecksForTest(root)
	var doctorMessages []string
	for _, c := range report.Checks {
		doctorMessages = append(doctorMessages, c.Message)
	}

	instructMessages := collectLifecycleFindings(root, phase.NewCache())

	if len(doctorMessages) < 2 {
		t.Fatalf("expected doctor to surface the finalize-branch AND stale-tracker findings, got %v", doctorMessages)
	}
	if len(instructMessages) < 2 {
		t.Fatalf("expected instruct to surface the finalize-branch AND stale-tracker findings, got %v", instructMessages)
	}

	for _, want := range doctorMessages {
		found := false
		for _, got := range instructMessages {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("doctor finding %q not found byte-identical in instruct output %v", want, instructMessages)
		}
	}
}
