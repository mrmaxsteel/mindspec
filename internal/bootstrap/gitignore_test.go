package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_GitignoreAppendsRuntimeEntries pins spec 123 AC-5 (R4a): a repo
// with a pre-existing, unrelated .gitignore gets exactly the two runtime
// entries appended, with every prior byte and line preserved (order
// included); `git check-ignore` matches both; a second `mindspec init`
// leaves the file byte-identical. RED on pre-spec-123 main: the .gitignore
// manifest item was create-only (Skipped when the file already existed),
// so `git check-ignore` missed both entries entirely.
func TestRun_GitignoreAppendsRuntimeEntries(t *testing.T) {
	root := t.TempDir()
	original := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, original) {
		t.Errorf("prior .gitignore bytes not preserved at the head; got:\n%s", content)
	}
	for _, entry := range []string{".mindspec/session.json", ".mindspec/focus"} {
		if !strings.Contains(content, entry) {
			t.Errorf(".gitignore missing runtime entry %q; got:\n%s", entry, content)
		}
	}

	// git check-ignore must match both (needs an actual git repo).
	runGit(t, root, "init", "-q")
	for _, entry := range []string{".mindspec/session.json", ".mindspec/focus"} {
		cmd := exec.Command("git", "check-ignore", "--quiet", "--", entry)
		cmd.Dir = root
		if err := cmd.Run(); err != nil {
			t.Errorf("git check-ignore %s: not ignored (err=%v)", entry, err)
		}
	}

	afterFirst := content

	// Second init: byte-identical (idempotent).
	if _, err := Run(root, false); err != nil {
		t.Fatalf("second Run() error: %v", err)
	}
	data2, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != afterFirst {
		t.Errorf("second Run() changed .gitignore; before:\n%s\nafter:\n%s", afterFirst, data2)
	}
}

// TestRun_GitignoreCreatedFromScratchAlreadyIgnores is a sanity guard: the
// greenfield create-from-scratch path (no pre-existing .gitignore) already
// contains both runtime entries, so the R4a ensure-on-existing-file call is
// a true no-op immediately after — never a double append.
func TestRun_GitignoreCreatedFromScratchAlreadyIgnores(t *testing.T) {
	root := t.TempDir()

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, entry := range []string{".mindspec/session.json", ".mindspec/focus"} {
		if strings.Count(content, entry) != 1 {
			t.Errorf("expected exactly one occurrence of %q, got %d in:\n%s",
				entry, strings.Count(content, entry), content)
		}
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}
