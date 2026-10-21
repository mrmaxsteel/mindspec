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
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	run("switch", "-c", "spec/132-test")
	run("switch", "-c", "bead/"+mergeDoctorBead)
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
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
		return []byte(`[]`), nil
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
