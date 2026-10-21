package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExecutor_LsFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init")
	if err := os.MkdirAll(filepath.Join(root, "tracked"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked", "file"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "tracked/file")
	for _, name := range []string{"-dash", "literal*", ":(glob)magic"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		git("--literal-pathspecs", "add", "--", name)
	}
	e := NewMindspecExecutor(root)
	for path, want := range map[string]bool{"tracked": true, "tracked/file": true, "absent": false, "track*": false, "-dash": true, "literal*": true, ":(glob)magic": true} {
		got, err := e.LsFiles(root, path)
		if err != nil || got != want {
			t.Fatalf("%s: %v %v; want %v", path, got, err, want)
		}
	}
	if _, err := e.LsFiles(t.TempDir(), "x"); err == nil {
		t.Fatal("git failure hidden")
	}
	m := &MockExecutor{LsFilesResult: true}
	if got, err := m.LsFiles(root, "tracked"); !got || err != nil || len(m.Calls) != 1 || m.Calls[0].Method != "LsFiles" {
		t.Fatalf("mock: %+v %v %v", m.Calls, got, err)
	}
}
