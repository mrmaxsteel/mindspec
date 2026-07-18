package instruct

// AC-15 cross-consumer proof: `mindspec doctor` and the generated
// `mindspec instruct` idle guidance must surface the IDENTICAL finalize-
// orphan finding text from ONE planted fixture. This test drives the REAL
// internal/lifecycle predicates (lifecycle.FindOutstandingFinalizeBranches
// / lifecycle.StaleTrackerOnMain — left UNSTUBBED in both consumers) over a
// real throwaway git repo, stubbing only the small epic-resolution glue
// each consumer already owns privately (doctor's
// findEpicForFinalizeCheckFn/findEpicStatusFn, instruct's
// instructFindEpicBySpecIDFn/instructFindEpicStatusFn) so no live `bd` is
// required. Since neither consumer re-derives or reformats the finding
// text (both call lifecycle.FinalizeOrphan.FullMessage() verbatim), byte
// equality here is the load-bearing half of AC-15 that the AC-12 identity
// pin (in each package's own test file) cannot show on its own — this test
// exercises the ACTUAL predicate over ACTUAL git state, not just seam
// pointers.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
)

// buildFinalizeOrphanFixture creates a real git repo with a surviving
// chore/finalize-119-test branch (FindOutstandingFinalizeBranches' trigger)
// and a main branch whose committed .beads/issues.jsonl shows epic-1 as
// "open" (StaleTrackerOnMain's trigger, once the caller reports
// liveClosed=true). It also creates the flat spec-enumeration directory
// both checkFinalizeOrphans and collectLifecycleFindings walk.
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

func TestLifecycleFindings_FinalizeOrphan_SameTextInDoctorAndInstruct(t *testing.T) {
	root := buildFinalizeOrphanFixture(t)

	// Doctor's own epic-resolution glue, stubbed (no live bd required).
	// The core predicates (findOutstandingFinalizeBranchesFn,
	// staleTrackerOnMainFn) are left at their REAL lifecycle defaults.
	t.Cleanup(doctor.SetFindEpicForFinalizeCheckForTest(func(specID string) (string, error) {
		if specID == "119-test" {
			return "epic-1", nil
		}
		return "", nil
	}))
	t.Cleanup(doctor.SetFindEpicStatusForTest(func(epicID string) (string, error) { return "closed", nil }))

	// Instruct's own epic-resolution glue, stubbed identically.
	origFindEpic := instructFindEpicBySpecIDFn
	origStatus := instructFindEpicStatusFn
	t.Cleanup(func() {
		instructFindEpicBySpecIDFn = origFindEpic
		instructFindEpicStatusFn = origStatus
	})
	instructFindEpicBySpecIDFn = func(specID string) (string, error) {
		if specID == "119-test" {
			return "epic-1", nil
		}
		return "", nil
	}
	instructFindEpicStatusFn = func(epicID string) (string, error) { return "closed", nil }

	report := doctor.RunFinalizeOrphanChecksForTest(root)
	var doctorMessages []string
	for _, c := range report.Checks {
		doctorMessages = append(doctorMessages, c.Message)
	}

	instructMessages := collectLifecycleFindings(root)

	if len(doctorMessages) == 0 {
		t.Fatalf("expected doctor to surface at least one finalize-orphan finding, got none")
	}
	if len(instructMessages) == 0 {
		t.Fatalf("expected instruct to surface at least one finalize-orphan finding, got none")
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
