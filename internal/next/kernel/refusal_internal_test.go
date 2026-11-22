package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134Paths_OwnRecovery(t *testing.T) {
	for _, kind := range []string{"step-type", "pre-kernel", "typed"} {
		t.Run(kind, func(t *testing.T) {
			cause := errors.New("original")
			var original error
			switch kind {
			case "step-type":
				root := t.TempDir()
				if err := os.Mkdir(filepath.Join(root, ".beads"), 0700); err != nil {
					t.Fatal(err)
				}
				pathWrite(t, filepath.Join(root, ".beads", "config.yaml"), "types.custom: gate\n")
				original = RequireStepType(root)
			case "pre-kernel":
				original = ErrPreKernel
			case "typed":
				original = termsafe.NewWrappedFailure(cause, "José\x1b\nrecovery: forged", "inspect 'José'")
			}
			if original == nil {
				t.Fatal("fixture did not refuse")
			}
			wrapped := fmt.Errorf("untrusted context\x1b: %w", original)
			err := pathFailure(pathFailure(wrapped, "", "generic"), "context: ", "generic")
			if err.Error() != "context: "+original.Error() || !errors.Is(err, wrapped) || !errors.Is(err, original) {
				t.Fatalf("own recovery or cause changed: got %q want %q", err.Error(), "context: "+original.Error())
			}
			if kind == "typed" {
				var failure *termsafe.Failure
				if !errors.Is(err, cause) || !errors.As(err, &failure) {
					t.Fatal("typed cause lost")
				}
			}
		})
	}
}

func TestS134Store_SizeRecoveryByKey(t *testing.T) {
	for _, tc := range []struct{ key, recovery string }{
		{KeyRecord, "submit a shorter record"},
		{KeyReturns, "submit a shorter record"},
		{KeyFindings, "shorten an unpinned verdict for a new review attempt, or halt the spec"},
		{KeyRead, "halt the spec"},
		{"other", "halt the spec"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			step := "test-1.1.review.r1.verify"
			_, err := CheckMetadataSize(step, map[string]any{tc.key: strings.Repeat("x", MetadataLimit)})
			if err == nil || !strings.Contains(err.Error(), "key "+tc.key+" size ") {
				t.Fatalf("fixture did not exceed bound: %v", err)
			}
			parts := strings.Split(err.Error(), "\n")
			if len(parts) != 2 || parts[1] != "recovery: "+tc.recovery {
				t.Fatalf("wrong key recovery: %q", err.Error())
			}
		})
	}
	t.Run("record-read", func(t *testing.T) {
		err := recordSizeFailure("test-1.1.review.r1.verify", MetadataLimit+1)
		if !strings.HasSuffix(err.Error(), "\nrecovery: submit a shorter record") || strings.Count(err.Error(), "recovery:") != 1 {
			t.Fatalf("oversized record recovery: %v", err)
		}
	})
}

func TestS134Paths_GitRootEnvironment(t *testing.T) {
	base := t.TempDir()
	top, other := filepath.Join(base, "top"), filepath.Join(base, "other")
	pathGit(t, base, "init", "-q", top)
	pathGit(t, base, "init", "-q", other)
	root := filepath.Join(top, "project")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0700); err != nil {
		t.Fatal(err)
	}
	inside, outside := filepath.Join(top, "record.json"), filepath.Join(base, "outside.json")
	pathWrite(t, inside, "{}")
	pathWrite(t, outside, "{}")
	for _, mode := range []string{"foreign-dir", "same-dir", "relative-dir", "work-tree", "common-dir", "index-file", "all"} {
		t.Run(mode, func(t *testing.T) {
			values := map[string]string{"GIT_DIR": "", "GIT_WORK_TREE": "", "GIT_COMMON_DIR": "", "GIT_INDEX_FILE": ""}
			switch mode {
			case "foreign-dir":
				values["GIT_DIR"] = filepath.Join(other, ".git")
			case "same-dir":
				values["GIT_DIR"] = filepath.Join(top, ".git")
			case "relative-dir":
				values["GIT_DIR"] = ".git"
			case "work-tree":
				values["GIT_WORK_TREE"] = root
			case "common-dir":
				values["GIT_COMMON_DIR"] = filepath.Join(other, ".git")
			case "index-file":
				values["GIT_INDEX_FILE"] = filepath.Join(other, "index")
			case "all":
				values = map[string]string{"GIT_DIR": filepath.Join(other, ".git"), "GIT_WORK_TREE": other, "GIT_COMMON_DIR": filepath.Join(other, ".git"), "GIT_INDEX_FILE": filepath.Join(other, "index")}
			}
			for key, value := range values {
				t.Setenv(key, value)
				if value == "" {
					if err := os.Unsetenv(key); err != nil {
						t.Fatal(err)
					}
				}
			}
			_, err := readRecordFile(State{Root: root}, "test-1.1.review.r1.lens", inside)
			assertPathFailure(t, err, "outside every worktree")
			if text, err := readRecordFile(State{Root: root}, "test-1.1.review.r1.lens", outside); err != nil || text != "{}" {
				t.Fatalf("outside control refused: %q %v", text, err)
			}
			roots, err := repositoryRoots(root)
			if err != nil || roots[len(roots)-1] != top {
				t.Fatalf("cwd did not select top: %v %v", roots, err)
			}
			for key, value := range values {
				if os.Getenv(key) != value {
					t.Fatalf("root query changed caller environment %s", key)
				}
			}
		})
	}
}

func TestS134Paths_GitEnvironmentScope(t *testing.T) {
	dir := t.TempDir()
	pathWrite(t, filepath.Join(dir, "git"), "#!/bin/sh\nprintf '%s|%s|%s|%s' \"${GIT_DIR+set}\" \"${GIT_WORK_TREE+set}\" \"${GIT_COMMON_DIR+set}\" \"${GIT_INDEX_FILE+set}\"\n")
	if err := os.Chmod(filepath.Join(dir, "git"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE"} {
		t.Setenv(key, "caller-value")
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"worktree", "list", "--porcelain", "-z"}, "|||"},
		{[]string{"rev-parse", "--show-toplevel"}, "|||"},
		{[]string{"diff", "--name-only"}, "set|set|set|set"},
		{[]string{"rev-parse", "HEAD"}, "set|set|set|set"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			data, err := actorGitBytes(dir, tc.args...)
			if err != nil || string(data) != tc.want {
				t.Fatalf("Git environment scope: %q %v, want %q", data, err, tc.want)
			}
		})
	}
}
