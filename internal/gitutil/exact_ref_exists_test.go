package gitutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExactRefExists_PresentAbsentAndBrokenBranch(t *testing.T) {
	root := initGitRepo(t)
	requireShowRefExists(t, root)
	runGit(t, root, "branch", "spec/demo")
	for _, tc := range []struct {
		ref  string
		want bool
	}{
		{"refs/heads/spec/demo", true}, {"refs/heads/absent", false},
	} {
		exists, err := ExactRefExists(root, tc.ref)
		if err != nil || exists != tc.want {
			t.Fatalf("%s: exists=%v err=%v", tc.ref, exists, err)
		}
	}
	// A same-named tag cannot supply an absent branch.
	runGit(t, root, "tag", "spec/absent")
	if exists, err := ExactRefExists(root, "refs/heads/spec/absent"); err != nil || exists {
		t.Fatalf("tag shadowed branch: %v %v", exists, err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git/refs/heads/spec/demo"), []byte(strings.Repeat("1", 40)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Git may report either existence or a corrupt-ref error; neither is absence.
	exists, err := ExactRefExists(root, "refs/heads/spec/demo")
	if !exists && err == nil {
		t.Fatal("broken branch was reported absent")
	}
	if err != nil && errors.Is(err, ErrRefNotFound) {
		t.Fatalf("broken branch classified as missing: %v", err)
	}
}

func TestExactRefExists_OnlyExitTwoConfirmsAbsence(t *testing.T) {
	for _, code := range []int{0, 2, 1, 128, 129, 42} {
		t.Run(itoa(code), func(t *testing.T) {
			calls := swapExec(t, "", code)
			exists, err := ExactRefExists("/repo", "refs/heads/spec/demo")
			if exists != (code == 0) || (err != nil) != (code != 0 && code != 2) {
				t.Fatalf("exit %d: exists=%v err=%v", code, exists, err)
			}
			if code == 129 && !strings.Contains(err.Error(), "confirming the spec branch is absent needs git 2.43 or later") {
				t.Errorf("old Git error must explain the version requirement: %v", err)
			}
			if len(*calls) != 1 || strings.Join((*calls)[0].args, " ") != "-C /repo show-ref --exists refs/heads/spec/demo" {
				t.Fatalf("expected exact show-ref: %v", *calls)
			}
		})
	}
}

func requireShowRefExists(t *testing.T, root string) {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "show-ref", "--exists", "refs/heads/main").CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 129 {
		t.Skip("git show-ref --exists requires git 2.43 or later")
	}
	if err != nil {
		t.Fatalf("probe git show-ref --exists: %v: %s", err, out)
	}
}

func TestExactRefExists_CorruptAndDanglingBranches(t *testing.T) {
	for _, tc := range []struct {
		name, contents string
		present        bool
	}{
		{"empty", "", false},
		{"truncated", "1234\n", false},
		{"garbage", "garbage\n", false},
		// A dangling symref exists; the caller must reject its unreadable tip.
		{"dangling-symbolic", "ref: refs/heads/nowhere\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initGitRepo(t)
			requireShowRefExists(t, root)
			runGit(t, root, "branch", "spec/demo")
			if err := os.WriteFile(filepath.Join(root, ".git/refs/heads/spec/demo"), []byte(tc.contents), 0600); err != nil {
				t.Fatal(err)
			}
			exists, err := ExactRefExists(root, "refs/heads/spec/demo")
			if exists != tc.present || (err == nil) != tc.present || errors.Is(err, ErrRefNotFound) {
				t.Fatalf("branch must be present or return an error: exists=%v err=%v", exists, err)
			}
		})
	}
}

func TestExactRefExists_RejectsUnqualifiedAndOptionLikeNames(t *testing.T) {
	calls := swapExec(t, "", 0)
	for _, ref := range []string{"spec/demo", "--help"} {
		if _, err := ExactRefExists("/repo", ref); err == nil {
			t.Fatalf("invalid exact ref accepted: %s", ref)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("invalid ref reached git: %v", *calls)
	}
}
