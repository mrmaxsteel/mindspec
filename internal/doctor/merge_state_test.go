package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

const mergeDoctorBead = "mindspec-test.1"

func mergeDoctorRepo(t *testing.T, open bool, marker ...string) (string, func(...string) string) {
	t.Helper()
	return mergeDoctorRepoWithPaths(t, open, []string{"a"}, marker...)
}

func mergeDoctorRepoWithPaths(t *testing.T, open bool, paths []string, marker ...string) (string, func(...string) string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false", "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-b", "main")
	for _, path := range paths {
		if err := os.WriteFile(filepath.Join(root, path), []byte("before\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run("add", ".")
	run("commit", "-m", "initial")
	run("switch", "-c", "spec/132-test")
	run("switch", "-c", "bead/"+mergeDoctorBead)
	for _, path := range paths {
		if err := os.WriteFile(filepath.Join(root, path), []byte("after\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run("commit", "-am", "change")
	run("switch", "spec/132-test")
	run("merge", "--no-ff", "-m", "Merge bead/"+mergeDoctorBead, "bead/"+mergeDoctorBead)
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "--type=epic") {
			return []byte(`[{"id":"mindspec-test","issue_type":"epic","status":"in_progress","metadata":{"spec_num":132,"spec_title":"test"}}]`), nil
		}
		if open {
			metadata := `{}`
			if len(marker) > 0 {
				metadata = fmt.Sprintf(`{"mindspec_merge_only":%q,"mindspec_merge_only_sha":%q}`, marker[0], marker[0])
			}
			return []byte(fmt.Sprintf(`[{"id":%q,"parent":"mindspec-test","status":"open","metadata":%s}]`, mergeDoctorBead, metadata)), nil
		}
		return []byte(fmt.Sprintf(`[{"id":%q,"parent":"mindspec-test","status":"closed"}]`, mergeDoctorBead)), nil
	}))
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "bd"), []byte("#!/bin/sh\nprintf '[]\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	stubScanIntegrity(t, lifecycle.ScanIntegrityFindings)
	return root, run
}

func TestDoctor_StaleOpenSeverityFromGitState(t *testing.T) {
	for _, marker := range []string{"", "pending", strings.Repeat("a", 40)} {
		t.Run("marker="+marker, func(t *testing.T) {
			root, _ := mergeDoctorRepo(t, true, marker)
			r := &Report{}
			checkLifecycleIntegrity(r, root)
			if len(r.Checks) != 1 {
				t.Fatalf("want one finding, got %+v", r.Checks)
			}
			if r.Checks[0].Status != Info {
				t.Errorf("status = %v, want Info", r.Checks[0].Status)
			}
			if r.HasFailures() {
				t.Errorf("a valid merge must be informational: %+v", r.Checks)
			}
			if r.Checks[0].FixFunc != nil {
				t.Error("valid merge must have no automatic close")
			}
			if !strings.Contains(r.Checks[0].Message, "recovery: mindspec complete "+mergeDoctorBead) {
				t.Errorf("missing recovery: %s", r.Checks[0].Message)
			}
		})
	}
}

func TestDoctor_RevertedMergeWithUserChangesOmitsCheckout(t *testing.T) {
	root, run := mergeDoctorRepo(t, true)
	run("revert", "-m", "1", "--no-edit", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "user"), []byte("keep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	if len(r.Checks) != 1 || strings.Contains(r.Checks[0].Message, " checkout ") || !strings.Contains(r.Checks[0].Message, " revert '") {
		t.Fatalf("unsafe recovery: %+v", r.Checks)
	}
}

func TestDoctor_RevertedMergeOnlyOneFinding(t *testing.T) {
	root, run := mergeDoctorRepo(t, true)
	merge := run("rev-parse", "HEAD")
	run("revert", "-m", "1", "--no-edit", merge)
	reverted := run("rev-parse", "HEAD")
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	if len(r.Checks) != 1 {
		t.Fatalf("want one reverted finding, got %+v", r.Checks)
	}
	c := r.Checks[0]
	if c.Status != Error || !strings.Contains(c.Name, "reverted bead merge") || c.FixFunc != nil || !r.HasFailures() {
		t.Fatalf("wrong finding: %+v", c)
	}
	if !strings.Contains(c.Message, merge) || !strings.Contains(c.Message, "revert '"+reverted+"'") {
		t.Errorf("missing full identities and recovery: %s", c.Message)
	}
}

func TestDoctor_PresenceProbeErrorKeepsError(t *testing.T) {
	root, run := mergeDoctorRepo(t, true)
	merge := &lifecycle.LandedMerge{
		SHA:          run("rev-parse", "HEAD"),
		FirstParent:  run("rev-parse", "HEAD^1"),
		SecondParent: run("rev-parse", "HEAD^2"),
	}
	if presence, err := lifecycle.CheckMergePresence(root, "spec/132-test", ".mindspec/specs/132-test", merge); err != nil || len(presence.ChangedPaths) == 0 {
		t.Fatalf("presence probe must pass before fault injection: %+v, %v", presence, err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	script := `#!/bin/sh
for arg do
    if [ "$arg" = "--no-renames" ]; then
        printf 'injected presence probe failure\n' >&2
        exit 1
    fi
done
exec "$MERGE_DOCTOR_REAL_GIT" "$@"
`
	if err := os.WriteFile(filepath.Join(fake, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MERGE_DOCTOR_REAL_GIT", realGit)
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err = lifecycle.CheckMergePresence(root, "spec/132-test", ".mindspec/specs/132-test", merge)
	if err == nil || !strings.Contains(err.Error(), "diff paths") {
		t.Fatalf("presence probe must fail: %v", err)
	}
	landed, ok, err := lifecycle.MergedUnclosed(root, "spec/132-test", mergeDoctorBead)
	if err != nil || !ok || landed.SHA != merge.SHA {
		t.Fatalf("fallback must still identify the landed merge: %+v, %v, %v", landed, ok, err)
	}
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	if len(r.Checks) != 1 || r.Checks[0].Status != Error || !r.HasFailures() {
		t.Fatalf("presence probe error must keep an Error finding: %+v", r.Checks)
	}
	if !strings.Contains(r.Checks[0].Message, merge.SHA) {
		t.Fatalf("finding must identify the landed merge: %+v", r.Checks[0])
	}
}

func TestDoctor_NamingMergeWithoutBranchIsError(t *testing.T) {
	root, run := mergeDoctorRepo(t, true, "pending")
	merge := run("rev-parse", "HEAD")
	secondParent := run("rev-parse", "HEAD^2")
	run("branch", "-D", "bead/"+mergeDoctorBead)
	fake := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '[{\"metadata\":{\"mindspec_landed_merge_sha\":\"%s\",\"mindspec_landed_second_parent\":\"%s\"}}]'\n", merge, secondParent)
	if err := os.WriteFile(filepath.Join(fake, "bd"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	merges, err := lifecycle.FindNamingMerges(root, "spec/132-test", mergeDoctorBead)
	if err != nil || len(merges) != 1 {
		t.Fatalf("want one naming merge: %+v, %v", merges, err)
	}
	valid, err := lifecycle.FindValidMerge(root, "spec/132-test", mergeDoctorBead)
	if err != nil || valid != nil {
		t.Fatalf("deleted branch must have no valid merge: %+v, %v", valid, err)
	}
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	if len(r.Checks) != 1 || r.Checks[0].Status != Error || !r.HasFailures() {
		t.Fatalf("naming merge without a valid merge must stay Error: %+v", r.Checks)
	}
}

func TestDoctor_PartialRevertIsError(t *testing.T) {
	root, run := mergeDoctorRepoWithPaths(t, true, []string{"a", "b"})
	merge := run("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("commit", "-am", "restore one path")
	if got := run("show", "HEAD:b"); got != "after" {
		t.Fatalf("second path must retain merged content: %q", got)
	}
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	if len(r.Checks) != 1 {
		t.Fatalf("want one partial-revert finding: %+v", r.Checks)
	}
	c := r.Checks[0]
	if c.Status != Error || !strings.Contains(c.Name, "reverted bead merge") || c.FixFunc != nil || !r.HasFailures() {
		t.Fatalf("partial revert must be an Error with no automatic fix: %+v", c)
	}
	if !strings.Contains(c.Message, merge) || !strings.Contains(c.Message, "':(literal)a'") {
		t.Fatalf("partial-revert finding must name the merge and restored path: %s", c.Message)
	}
}

func TestDoctor_FixSkipsRevertedMergeOnly(t *testing.T) {
	root, run := mergeDoctorRepo(t, true)
	run("revert", "-m", "1", "--no-edit", "HEAD")
	assertDoctorFixReadOnly(t, root, run, Error)
}

func TestDoctor_ClosedRevertedNoFinding(t *testing.T) {
	root, run := mergeDoctorRepo(t, false)
	run("revert", "-m", "1", "--no-edit", "HEAD")
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	if len(r.Checks) != 0 {
		t.Fatalf("closed bead must have no merge-state finding: %+v", r.Checks)
	}
}

func TestDoctor_FixSkipsStaleOpenInfo(t *testing.T) {
	root, run := mergeDoctorRepo(t, true)
	assertDoctorFixReadOnly(t, root, run, Info)
}

func assertDoctorFixReadOnly(t *testing.T, root string, run func(...string) string, want Status) {
	t.Helper()
	original := runMindspecCompleteFn
	t.Cleanup(func() { runMindspecCompleteFn = original })
	runMindspecCompleteFn = func(string, string) error { t.Fatal("doctor must leave this bead open"); return nil }
	before := run("show-ref")
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	r.Fix()
	if len(r.Checks) != 1 || r.Checks[0].Status != want || r.Checks[0].FixFunc != nil {
		t.Fatalf("fix changed finding: %+v", r.Checks)
	}
	if after := run("show-ref"); before != after {
		t.Fatal("doctor changed git refs")
	}
	if status := run("status", "--porcelain"); status != "" {
		t.Fatalf("doctor wrote checkout: %s", status)
	}
}

func TestDoctor_InfoNeverRunsFix(t *testing.T) {
	r := &Report{Checks: []Check{{Status: Info, FixFunc: func() error { t.Fatal("Info must never run a fix"); return nil }}}}
	r.Fix()
	if r.HasFailures() || r.Checks[0].Status != Info {
		t.Fatal("Info changed status or exit result")
	}
}
