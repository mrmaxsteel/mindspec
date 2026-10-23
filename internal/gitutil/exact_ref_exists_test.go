package gitutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExactRefExists_PresentAbsentAndBrokenBranch(t *testing.T) {
	root := initGitRepo(t)
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

func TestExactRefExists_OnlyExitOneConfirmsAbsence(t *testing.T) {
	for _, code := range []int{0, 1, 128} {
		t.Run(itoa(code), func(t *testing.T) {
			calls := swapExec(t, "", code)
			exists, err := ExactRefExists("/repo", "refs/heads/spec/demo")
			if exists != (code == 0) || (err != nil) != (code == 128) {
				t.Fatalf("exit %d: exists=%v err=%v", code, exists, err)
			}
			if len(*calls) != 1 || !hasArg((*calls)[0].args, "show-ref") || !hasArg((*calls)[0].args, "--verify") || !hasArg((*calls)[0].args, "--quiet") {
				t.Fatalf("expected exact show-ref: %v", *calls)
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
