package gitutil

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestLsTree_ArgvAndRefGate(t *testing.T) {
	original := execCommand
	t.Cleanup(func() { execCommand = original })
	calls := 0
	const listing = "100644 blob abc123\tfile.go\n"
	execCommand = func(name string, args ...string) *exec.Cmd {
		calls++
		want := []string{"-C", "/repo/spec", "ls-tree", "-r", "--full-tree", "HEAD", "--"}
		if name != "git" || !reflect.DeepEqual(args, want) {
			t.Fatalf("command %s %v, want git %v", name, args, want)
		}
		return exec.Command("printf", "%s", listing)
	}
	if _, err := LsTree("/repo/spec", "--help"); err == nil || calls != 0 {
		t.Fatalf("option-like ref reached git: calls=%d err=%v", calls, err)
	}
	if got, err := LsTree("/repo/spec", "HEAD"); err != nil || got != listing || calls != 1 {
		t.Fatalf("listing=%q calls=%d err=%v", got, calls, err)
	}
	execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "printf 'fatal: invalid tree reference\\n' >&2; exit 1")
	}
	if got, err := LsTree("/repo/spec", "HEAD"); err == nil || got != "" ||
		!strings.Contains(err.Error(), "ls-tree HEAD:") || !strings.Contains(err.Error(), "fatal: invalid tree reference") {
		t.Fatalf("read failure hidden: %q %v", got, err)
	}
}
