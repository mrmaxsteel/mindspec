package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckRuntimeFilesTracked_UnignoredUntracked pins spec 123 R4c/AC-7(i):
// a runtime file that is NOT yet tracked by git but also has no ignore rule
// (the pre-accident state a single `git add .mindspec/` away from becoming
// the tracked-file Error below) gets a Warn, not a silent OK. --fix appends
// the ignore entry via the shared gitutil helper, after which a re-run
// reports OK. RED on pre-spec-123 main (reported "OK: not tracked by git").
func TestCheckRuntimeFilesTracked_UnignoredUntracked(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "session.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No .gitignore at all — untracked and unignored.

	r := &Report{}
	checkRuntimeFilesTracked(r, root)

	c := findCheck(r, ".mindspec/session.json git tracking")
	if c == nil {
		t.Fatal("missing check")
	}
	if c.Status != Warn {
		t.Fatalf("expected Warn, got %v (msg=%q)", c.Status, c.Message)
	}
	if c.FixFunc == nil {
		t.Fatal("expected a FixFunc")
	}
	if err := c.FixFunc(); err != nil {
		t.Fatalf("FixFunc: %v", err)
	}

	r2 := &Report{}
	checkRuntimeFilesTracked(r2, root)
	c2 := findCheck(r2, ".mindspec/session.json git tracking")
	if c2 == nil || c2.Status != OK {
		t.Fatalf("expected OK after --fix, got %+v", c2)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !containsExactLine(string(data), ".mindspec/session.json") {
		t.Errorf(".gitignore missing session.json entry after --fix:\n%s", data)
	}
}

// TestCheckRuntimeFilesTracked_AlreadyIgnored pins AC-7(ii): a runtime file
// that is untracked AND already gitignored reports OK.
func TestCheckRuntimeFilesTracked_AlreadyIgnored(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "focus"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".gitignore"), ".mindspec/focus\n")

	r := &Report{}
	checkRuntimeFilesTracked(r, root)

	c := findCheck(r, ".mindspec/focus git tracking")
	if c == nil {
		t.Fatal("missing check")
	}
	if c.Status != OK {
		t.Fatalf("expected OK, got %v (msg=%q)", c.Status, c.Message)
	}
}

// TestCheckRuntimeFilesTracked_TrackedGuard is a GUARD pinning AC-7(iii): a
// TRACKED runtime file still gets the pre-existing Error + untrack FixFunc,
// unchanged by the R4c ignore-ness addition (it takes precedence over the
// new Warn — a tracked file is the worse, already-happened state).
func TestCheckRuntimeFilesTracked_TrackedGuard(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "session.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".mindspec/session.json")
	runGit(t, root, "commit", "-m", "oops")

	r := &Report{}
	checkRuntimeFilesTracked(r, root)

	c := findCheck(r, ".mindspec/session.json git tracking")
	if c == nil {
		t.Fatal("missing check")
	}
	if c.Status != Error {
		t.Fatalf("expected Error (tracked file), got %v (msg=%q)", c.Status, c.Message)
	}
	if c.FixFunc == nil {
		t.Fatal("expected untrack FixFunc")
	}
	if err := c.FixFunc(); err != nil {
		t.Fatalf("FixFunc: %v", err)
	}

	r2 := &Report{}
	checkRuntimeFilesTracked(r2, root)
	c2 := findCheck(r2, ".mindspec/session.json git tracking")
	if c2 == nil || c2.Status != OK {
		t.Fatalf("expected OK after untrack --fix, got %+v", c2)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !containsExactLine(string(data), ".mindspec/session.json") {
		t.Errorf(".gitignore missing session.json entry after untrack --fix:\n%s", data)
	}
}

func containsExactLine(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}
