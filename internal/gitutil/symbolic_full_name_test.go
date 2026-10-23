package gitutil

import (
	"errors"
	"strings"
	"testing"
)

func TestSymbolicFullName_AnnotatedTagObjectIgnoresSameNamedBranch(t *testing.T) {
	root := initGitRepo(t)
	reviewed := runGitOut(t, root, "rev-parse", "HEAD")
	runGit(t, root, "tag", "-a", "review-pin", "-m", "reviewed", reviewed)
	target := runGitOut(t, root, "rev-parse", "refs/tags/review-pin")
	runGit(t, root, "branch", target, reviewed)
	name, err := SymbolicFullName(root, target)
	if err != nil || name != "" {
		t.Fatalf("annotated-tag object must be a pin, got name=%q err=%v", name, err)
	}
}

func TestSymbolicFullName_Exit128RequiresConfirmedAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		name, objects string
		probeExit     int
		wantPin       bool
	}{
		{"ambiguous", strings.Repeat("a", 40) + "\n" + strings.Repeat("b", 40) + "\n", 0, true},
		{"one object", strings.Repeat("a", 40) + "\n", 0, false},
		{"no objects", "", 0, false},
		{"broken repository", "", 128, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := swapExecFunc(t, func(_ string, args []string) (string, int) {
				if hasArg(args, "--disambiguate=abcd") {
					return tc.objects, tc.probeExit
				}
				return "", 128
			})
			name, err := SymbolicFullName("/repo", "abcd")
			if tc.wantPin {
				if err != nil || name != "" {
					t.Fatalf("confirmed ambiguous prefix must be an object pin: name=%q err=%v", name, err)
				}
			} else if err == nil || errors.Is(err, ErrRefNotFound) {
				t.Fatalf("structural git error must propagate: name=%q err=%v", name, err)
			}
			if len(*calls) != 2 {
				t.Fatalf("expected symbolic lookup and ambiguity confirmation, got %d calls", len(*calls))
			}
		})
	}
}

func TestSymbolicFullName_RejectsOptionLikeTarget(t *testing.T) {
	calls := swapExec(t, "", 0)
	if _, err := SymbolicFullName("/repo", "--help"); err == nil || len(*calls) != 0 {
		t.Fatalf("option-like target reached git: calls=%v err=%v", *calls, err)
	}
}
