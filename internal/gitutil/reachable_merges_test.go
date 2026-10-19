package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReachableMerges(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("checkout", "-b", "spec/129-test")
	run("checkout", "-b", "bead/x")
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "x")
	run("commit", "-m", "work")
	second := run("rev-parse", "HEAD")
	run("checkout", "spec/129-test")
	first := run("rev-parse", "HEAD")
	run("merge", "--no-ff", "-m", "Merge bead/x", "bead/x")
	inner := run("rev-parse", "HEAD")
	run("checkout", "main")
	run("merge", "--no-ff", "-m", "outer merge", "spec/129-test")
	outer := run("rev-parse", "HEAD")
	run("checkout", "spec/129-test")
	run("merge", "--ff-only", "main")
	merges, err := ReachableMerges(dir, "spec/129-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 2 || merges[0].SHA != outer || merges[1].SHA != inner || merges[1].Subject != "Merge bead/x" || !reflect.DeepEqual(merges[1].Parents, []string{first, second}) {
		t.Fatalf("reachable merge order/identity = %+v", merges)
	}
	primary, err := FirstParentMerges(dir, "spec/129-test")
	if err != nil || len(primary) != 1 || primary[0].SHA != outer {
		t.Fatalf("first-parent scope: %+v %v", primary, err)
	}
	if _, err := ReachableMerges(dir, "--all"); err == nil {
		t.Fatal("option-like ref admitted")
	}
	if _, err := ReachableMerges(dir, "missing-ref"); err == nil {
		t.Fatal("missing-ref failure hidden")
	}
}
