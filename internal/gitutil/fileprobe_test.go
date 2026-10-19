package gitutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileAtRefTyped(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root, "-c", "user.email=test@example.invalid", "-c", "user.name=test"}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte("committed plan\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "plan.md")
	git("commit", "-m", "fixture")
	git("branch", "spec/129-fixture")
	for _, tc := range []struct {
		name, ref, path string
		want            error
		content         string
	}{
		{"ref absent", "spec/999-absent", "plan.md", ErrRefAbsent, ""},
		{"path absent", "spec/129-fixture", "absent.md", ErrPathNotAtRef, ""},
		{"present", "spec/129-fixture", "plan.md", nil, "committed plan\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := FileAtRefTyped(root, tc.ref, tc.path)
			if !errors.Is(err, tc.want) || string(data) != tc.content {
				t.Fatalf("got %q %v; want %q %v", data, err, tc.content, tc.want)
			}
		})
	}
	for _, stage := range []string{"rev-parse", "ls-tree", "show"} {
		t.Run("failure/"+stage, func(t *testing.T) {
			old := execCommand
			t.Cleanup(func() { execCommand = old })
			execCommand = func(name string, args ...string) *exec.Cmd {
				for _, a := range args {
					if a == stage {
						return exec.Command("sh", "-c", "echo injected-failure >&2; exit 128")
					}
				}
				return exec.Command(name, args...)
			}
			_, err := FileAtRefTyped(root, "spec/129-fixture", "plan.md")
			if err == nil || errors.Is(err, ErrRefAbsent) || errors.Is(err, ErrPathNotAtRef) {
				t.Fatalf("failure was mistaken for absence: %v", err)
			}
		})
	}
	t.Run("absent ref with output is failure", func(t *testing.T) {
		old := execCommand
		t.Cleanup(func() { execCommand = old })
		execCommand = func(string, ...string) *exec.Cmd { return exec.Command("sh", "-c", "echo corrupt >&2; exit 1") }
		_, err := FileAtRefTyped(root, "spec/129-fixture", "plan.md")
		if err == nil || errors.Is(err, ErrRefAbsent) || !strings.Contains(err.Error(), "corrupt") {
			t.Fatalf("nonempty exit-1: %v", err)
		}
	})
	for _, tc := range [][2]string{{"--help", "plan.md"}, {"spec/129-fixture", "--help"}} {
		t.Run("reject/"+strings.Join(tc[:], "/"), func(t *testing.T) {
			old := execCommand
			t.Cleanup(func() { execCommand = old })
			execCommand = func(string, ...string) *exec.Cmd { t.Fatal("option-like operand reached git"); return nil }
			if _, err := FileAtRefTyped(root, tc[0], tc[1]); err == nil {
				t.Fatal("expected option rejection")
			}
		})
	}
}
