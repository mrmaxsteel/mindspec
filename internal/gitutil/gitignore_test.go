package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureGitignoreEntries_New writes both runtime entries into a fresh
// (absent) .gitignore.
func TestEnsureGitignoreEntries_New(t *testing.T) {
	root := t.TempDir()
	if err := EnsureGitignoreEntries(root, RuntimeIgnoreEntries...); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	for _, e := range RuntimeIgnoreEntries {
		if !hasExactLine(string(data), e) {
			t.Errorf("expected exact line %q in .gitignore:\n%s", e, data)
		}
	}
}

// TestEnsureGitignoreEntries_Idempotent pins byte-idempotence: a second call
// with the entries already present is a true no-op.
func TestEnsureGitignoreEntries_Idempotent(t *testing.T) {
	root := t.TempDir()
	if err := EnsureGitignoreEntries(root, RuntimeIgnoreEntries...); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err := EnsureGitignoreEntries(root, RuntimeIgnoreEntries...); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if string(first) != string(second) {
		t.Errorf("second call changed .gitignore:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestEnsureGitignoreEntries_LeadingSpaceNotMistaken pins FX-2: a
// pre-existing line with LEADING whitespace (" .mindspec/session.json") is a
// DIFFERENT pattern git does NOT honor, so it must NOT satisfy presence —
// the real, unindented entry must still be appended so `git check-ignore`
// passes. A TrimSpace-based match (the bug) would treat the leading-space
// line as the required entry and leave .gitignore converged-but-unsafe.
func TestEnsureGitignoreEntries_LeadingSpaceNotMistaken(t *testing.T) {
	root := t.TempDir()
	runGitignoreTestGit(t, root, "init", "-q")

	// Seed a .gitignore whose only session.json line is INDENTED (invalid
	// to git) plus an unrelated line.
	seed := "node_modules/\n .mindspec/session.json\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignoreEntries(root, ".mindspec/session.json"); err != nil {
		t.Fatalf("EnsureGitignoreEntries: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Errorf("prior content not preserved:\n%s", content)
	}
	if !hasExactLine(content, ".mindspec/session.json") {
		t.Errorf("expected the exact (unindented) entry to be appended; got:\n%s", content)
	}

	// The ground truth: git must now actually ignore the file.
	cmd := exec.Command("git", "check-ignore", "--quiet", "--", ".mindspec/session.json")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Errorf("git check-ignore still misses .mindspec/session.json after ensure (err=%v):\n%s", err, content)
	}
}

func hasExactLine(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSuffix(line, "\r") == want {
			return true
		}
	}
	return false
}

func runGitignoreTestGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}
