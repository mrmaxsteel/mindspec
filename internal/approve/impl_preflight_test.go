package approve

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestApproveImpl_OneHasRemoteDecision(t *testing.T) {
	for _, path := range []string{"push", "direct", "missing-branch"} {
		t.Run(path, func(t *testing.T) {
			root, mock := approvePreflightFixture(t)
			probes := 0
			remote := path == "push"
			implHasRemoteFn = func() (bool, error) { probes++; return remote, nil }
			implSpecBranchExistsFn = func(string, string) (bool, error) { return path != "missing-branch", nil }
			_, err := ApproveImpl(root, "010-test", mock)
			if (err != nil) != (path == "missing-branch") {
				t.Fatalf("approval error = %v", err)
			}
			if probes != 1 {
				t.Fatalf("remote decisions = %d, want 1", probes)
			}
			if path == "missing-branch" {
				if len(mock.CallsTo("FinalizeEpic")) != 0 {
					t.Fatal("missing branch finalized")
				}
			} else {
				assertPreflightRemote(t, mock, remote)
				calls := mock.CallsTo("FinalizeEpic")
				if len(calls) != 1 || calls[0].Args[6] != remote {
					t.Fatalf("finalize decision = %+v", calls)
				}
			}
		})
	}
}

func stubPreflightPhase(t *testing.T, stored string) {
	t.Helper()
	epic := phase.EpicInfo{ID: "epic-parent", Title: "[SPEC 010-test] Test", Status: "open", IssueType: "epic",
		Metadata: map[string]interface{}{"spec_num": 10, "spec_title": "test", "mindspec_phase": stored, "mindspec_phase_semantics": "v2"}}
	children := []phase.ChildInfo{{ID: "bead-1", Status: "closed", IssueType: "task"}}
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg == "--type=epic" {
				return json.Marshal([]phase.EpicInfo{epic})
			}
		}
		return json.Marshal(children)
	}))
	t.Cleanup(phase.SetRunBDForTest(func(...string) ([]byte, error) { return json.Marshal([]phase.EpicInfo{epic}) }))
}

// assertEarlyRefusal checks both lifecycle state and all write seams.
func assertEarlyRefusal(t *testing.T, root string, runner executor.Executor, want string) {
	t.Helper()
	closes, writes, placeholders := 0, 0, 0
	implRunBDCombinedFn = func(...string) ([]byte, error) { closes++; return nil, nil }
	implPhaseMetadataFn = func(string, map[string]interface{}) error { writes++; return nil }
	orig := implCreateWithIDFn
	t.Cleanup(func() { implCreateWithIDFn = orig })
	implCreateWithIDFn = func(string, string, string, adr.CreateOpts) (string, error) {
		placeholders++
		return "", errors.New("unexpected placeholder write")
	}
	epicBefore, err := phase.NewCache().FindEpic("epic-parent")
	if err != nil || epicBefore == nil || epicBefore.Status != "open" {
		t.Fatalf("epic before refusal: %+v %v", epicBefore, err)
	}
	before, err := phase.DerivePhaseDetail("epic-parent")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ApproveImpl(root, "010-test", runner, ImplOpts{SupersedeADR: "ADR-0099"})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want %q refusal, got %v", want, err)
	}
	after, readErr := phase.DerivePhaseDetail("epic-parent")
	if readErr != nil || before.Stored != after.Stored || before.Derived != after.Derived {
		t.Fatalf("phase changed: before=%+v after=%+v err=%v", before, after, readErr)
	}
	epicAfter, readErr := phase.NewCache().FindEpic("epic-parent")
	if readErr != nil || epicAfter == nil || epicAfter.Status != "open" {
		t.Fatalf("epic after refusal: %+v %v", epicAfter, readErr)
	}
	if closes != 0 || writes != 0 || placeholders != 0 {
		t.Fatalf("mutations: close=%d phase=%d placeholder=%d", closes, writes, placeholders)
	}
	var mock *executor.MockExecutor
	switch e := runner.(type) {
	case *executor.MockExecutor:
		mock = e
	case *earlyGuardExecutor:
		mock = e.MockExecutor
	}
	if mock == nil || len(mock.CallsTo("FinalizeEpic")) != 0 {
		t.Fatal("early refusal called FinalizeEpic")
	}
}

func TestApproveImpl_HasRemoteProbeErrorRefuses(t *testing.T) {
	for _, stored := range []string{"review", "implement"} {
		t.Run(stored, func(t *testing.T) {
			root, mock := approvePreflightFixture(t)
			stubPreflightPhase(t, stored)
			implHasRemoteFn = func() (bool, error) { return false, errors.New("read failed\x1b[31m") }
			branchReads := 0
			implSpecBranchExistsFn = func(string, string) (bool, error) { branchReads++; return true, nil }
			assertEarlyRefusal(t, root, mock, "cannot tell whether a git remote is configured:")
			_, err := ApproveImpl(root, "010-test", mock, ImplOpts{SupersedeADR: "ADR-0099"})
			if err == nil {
				t.Fatal("probe error passed")
			}
			for _, pattern := range []string{`ADR-[0-9]`, `q9ea`, `[Ss]pec [0-9]{3}`} {
				if regexp.MustCompile(pattern).MatchString(err.Error()) {
					t.Fatalf("refusal matches %q: %s", pattern, err)
				}
			}
			if !strings.HasSuffix(err.Error(), "recovery: git remote -v") || strings.ContainsRune(err.Error(), '\x1b') {
				t.Fatalf("unsafe refusal: %q", err.Error())
			}
			if branchReads != 0 || len(mock.CallsTo("PreflightFinalizeMerges")) != 0 {
				t.Fatal("probe error reached later preflight")
			}
		})
	}
}

type earlyGuardWorktrees []bead.WorktreeListEntry

func (w earlyGuardWorktrees) List() ([]bead.WorktreeListEntry, error) { return w, nil }
func (earlyGuardWorktrees) Create(string, string) error               { return errors.New("unexpected create") }
func (earlyGuardWorktrees) Remove(string) error                       { return errors.New("unexpected remove") }

type earlyGuardExecutor struct {
	*executor.MockExecutor
	real *executor.MindspecExecutor
}

func (e *earlyGuardExecutor) PreflightFinalizeMerges(epic, spec, branch string, allow []string, reason string, remote bool) error {
	_ = e.MockExecutor.PreflightFinalizeMerges(epic, spec, branch, allow, reason, remote)
	return e.real.PreflightFinalizeMerges(epic, spec, branch, allow, reason, remote)
}

func preflightGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func earlyGuardFixture(t *testing.T, kind string, remote bool) (string, *earlyGuardExecutor) {
	t.Helper()
	root, mock := approvePreflightFixture(t)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	// Use flat lifecycle paths. The common approval helper writes legacy paths.
	if err := os.Rename(filepath.Join(root, "docs", "specs"), filepath.Join(root, ".mindspec", "specs")); err != nil {
		t.Fatal(err)
	}
	// The exempt phase migration takes this lock even when it writes no metadata.
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "phase.lock"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	preflightGit(t, root, "init", "-b", "main")
	preflightGit(t, root, "config", "user.name", "test")
	preflightGit(t, root, "config", "user.email", "test@example.invalid")
	preflightGit(t, root, "add", ".")
	preflightGit(t, root, "commit", "-m", "flat base")
	preflightGit(t, root, "branch", "spec/010-test")
	source, target := "spec/010-test", "main"
	var worktrees earlyGuardWorktrees
	if remote {
		source, target = "bead/bead-1", "spec/010-test"
		preflightGit(t, root, "branch", source)
		worktrees = earlyGuardWorktrees{{Name: "worktree-bead-1", Branch: source}}
	}
	preflightGit(t, root, "switch", source)
	file := "feature.txt"
	if kind == "layout" {
		file = ".mindspec/docs/specs/old/spec.md"
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, file), []byte("feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	preflightGit(t, root, "add", ".")
	preflightGit(t, root, "commit", "-m", "source work")
	preflightGit(t, root, "switch", target)
	if kind == "destruction" {
		preflightGit(t, root, "merge", "--squash", source)
		preflightGit(t, root, "commit", "-m", "work landed by another route")
	}
	implHasRemoteFn = func() (bool, error) { return remote, nil }
	return root, &earlyGuardExecutor{MockExecutor: mock, real: &executor.MindspecExecutor{Root: root, WorktreeOps: worktrees}}
}

func testEarlyGuard(t *testing.T, kind, stored string) {
	t.Helper()
	for _, remote := range []bool{true, false} {
		path := "direct"
		if remote {
			path = "push"
		}
		t.Run(path, func(t *testing.T) {
			root, e := earlyGuardFixture(t, kind, remote)
			stubPreflightPhase(t, stored)
			before := preflightGit(t, root, "show-ref")
			want := "layout regression"
			if kind == "destruction" {
				want = "content already landed"
			}
			assertEarlyRefusal(t, root, e, want)
			if got := preflightGit(t, root, "show-ref"); got != before {
				t.Fatal("preflight moved git refs")
			}
			if got := preflightGit(t, root, "status", "--porcelain"); got != "" {
				t.Fatalf("preflight changed files: %s", got)
			}
			assertPreflightRemote(t, e.MockExecutor, remote)
		})
	}
}
func TestApproveImpl_EarlyLayoutGuardRefuses(t *testing.T) { testEarlyGuard(t, "layout", "review") }
func TestApproveImpl_EarlyDestructionRefuses(t *testing.T) {
	testEarlyGuard(t, "destruction", "review")
}
func TestApproveImpl_EarlyGuardBeforePhaseReconcile(t *testing.T) {
	for _, kind := range []string{"layout", "destruction"} {
		t.Run(kind, func(t *testing.T) { testEarlyGuard(t, kind, "implement") })
	}
}

// This adapter keeps approval's other I/O fake and runs the real push probe.
type pushProbeExecutor struct {
	*executor.MockExecutor
	root string
}

func (e *pushProbeExecutor) PushProbe(branch string) error {
	_ = e.MockExecutor.PushProbe(branch)
	return (&executor.MindspecExecutor{Root: e.root}).PushProbe(branch)
}

func testApprovePushProbe(t *testing.T, output, url string, fails, helper bool) {
	t.Helper()
	root, mock := approvePreflightFixture(t)
	implHasRemoteFn = func() (bool, error) { return true, nil }
	gitDir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\npush) printf '%s' \"$PROBE_OUTPUT\"; exit \"$PROBE_EXIT\";;\nremote) printf '%s' \"$PROBE_URL\";;\n*) exit 97;;\nesac\n"
	if err := os.WriteFile(filepath.Join(gitDir, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", gitDir)
	t.Setenv("PROBE_OUTPUT", output)
	t.Setenv("PROBE_URL", url)
	code := "0"
	if fails {
		code = "1"
	}
	t.Setenv("PROBE_EXIT", code)
	writes := 0
	implRunBDCombinedFn = func(...string) ([]byte, error) { writes++; return nil, nil }
	implPhaseMetadataFn = func(string, map[string]interface{}) error { writes++; return nil }
	_, err := ApproveImpl(root, "010-test", &pushProbeExecutor{mock, root})
	if (err != nil) != fails {
		t.Fatalf("probe failure=%v, approval error=%v", fails, err)
	}
	if fails {
		if writes != 0 || len(mock.CallsTo("FinalizeEpic")) != 0 {
			t.Fatal("failed probe mutated lifecycle")
		}
		if strings.Contains(err.Error(), "credential helper") != helper {
			t.Fatalf("helper hint mismatch: %s", err)
		}
	} else if len(mock.CallsTo("FinalizeEpic")) != 1 {
		t.Fatal("successful probe did not finalize")
	}
}
func TestApproveImpl_PushProbeAuthFailureNamesHelper(t *testing.T) {
	testApprovePushProbe(t, "fatal: Authentication failed", "https://example.invalid/repo.git", true, true)
}
func TestApproveImpl_PushProbeNonFastForwardNoHelperHint(t *testing.T) {
	testApprovePushProbe(t, "rejected (non-fast-forward)", "https://example.invalid/repo.git", true, false)
	t.Run("ssh-auth", func(t *testing.T) {
		testApprovePushProbe(t, "Authentication failed", "ssh://example.invalid/repo.git", true, false)
	})
}
func TestApproveImpl_PushProbeFileRemoteNoHelperPasses(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	preflightGit(t, root, "init", "-b", "main")
	preflightGit(t, root, "config", "user.name", "test")
	preflightGit(t, root, "config", "user.email", "test@example.invalid")
	preflightGit(t, root, "config", "credential.helper", "")
	preflightGit(t, root, "add", ".")
	preflightGit(t, root, "commit", "-m", "spec work")
	preflightGit(t, root, "branch", "spec/010-test")
	remote := t.TempDir()
	preflightGit(t, remote, "init", "--bare", "-b", "main")
	preflightGit(t, root, "remote", "add", "origin", "file://"+remote)
	implHasRemoteFn = func() (bool, error) { return true, nil }
	if _, err := ApproveImpl(root, "010-test", &pushProbeExecutor{mock, root}); err != nil {
		t.Fatal(err)
	}
	if len(mock.CallsTo("FinalizeEpic")) != 1 {
		t.Fatal("successful file probe did not finalize")
	}
	if got := preflightGit(t, remote, "for-each-ref", "--format=%(refname)"); got != "" {
		t.Fatalf("dry run published refs: %s", got)
	}
}
