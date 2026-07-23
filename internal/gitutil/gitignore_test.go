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

// TestEnsureGitignoreEntries_NegationDefeated is the G1 final-review pin: a
// .gitignore that already contains the exact entry line, followed LATER by
// a negation rule that un-ignores it, must not be reported as converged.
// git applies patterns in file order with last-match-wins, so
// ".mindspec/session.json" followed by "!.mindspec/session.json" leaves the
// path genuinely NOT ignored even though the line-presence check alone
// would say it is. EnsureGitignoreEntries must detect this via `git
// check-ignore` and append the plain entry again so the LAST match in the
// file re-ignores it.
func TestEnsureGitignoreEntries_NegationDefeated(t *testing.T) {
	root := t.TempDir()
	runGitignoreTestGit(t, root, "init", "-q")

	seed := ".mindspec/session.json\n!.mindspec/session.json\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ground truth BEFORE the fix runs: the negation defeats the entry.
	before := exec.Command("git", "check-ignore", "--quiet", "--", ".mindspec/session.json")
	before.Dir = root
	if err := before.Run(); err == nil {
		t.Fatalf("test setup invalid: .mindspec/session.json is already ignored before EnsureGitignoreEntries runs")
	}

	if err := EnsureGitignoreEntries(root, ".mindspec/session.json"); err != nil {
		t.Fatalf("EnsureGitignoreEntries: %v", err)
	}

	// The ground truth: git must now actually ignore the file, despite the
	// original line already being present.
	after := exec.Command("git", "check-ignore", "--quiet", "--", ".mindspec/session.json")
	after.Dir = root
	if err := after.Run(); err != nil {
		data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
		t.Errorf("git check-ignore still misses .mindspec/session.json after ensure (err=%v):\n%s", err, data)
	}
}

// TestEnsureGitignoreEntries_TrackedFileNoIndex is the round-2 final-review
// FIX A pin: `git check-ignore` WITHOUT --no-index reports a path as "not
// ignored" whenever that path is ALREADY TRACKED in the index, regardless
// of any matching .gitignore rule. Before FIX A, checkIgnored used
// `--quiet` alone, so a tracked runtime file (e.g. accidentally committed
// before the ignore rule existed) would be reported not-ignored on EVERY
// call even though the rule is genuinely present and would otherwise be
// honored — causing EnsureGitignoreEntries to re-append a duplicate entry
// line on every single call (non-idempotent, unbounded .gitignore growth).
// With --no-index, checkIgnored evaluates the ignore RULE only, independent
// of tracked-status, so a tracked-but-rule-covered file converges to a true
// no-op. The entry is deliberately NOT the last line in the seeded
// .gitignore (a second, unrelated line follows it) so this test exercises
// checkIgnored's tracked-vs-rule determination directly, rather than being
// (correctly) short-circuited by the separate FIX B
// last-line-in-file skip.
func TestEnsureGitignoreEntries_TrackedFileNoIndex(t *testing.T) {
	root := t.TempDir()
	runGitignoreTestGit(t, root, "init", "-q")
	runGitignoreTestGit(t, root, "config", "user.email", "test@example.com")
	runGitignoreTestGit(t, root, "config", "user.name", "Test")

	entry := ".mindspec/session.json"
	seed := entry + "\nnode_modules/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, entry), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Force-track the runtime file, as if it had been committed before
	// the ignore rule existed (or before `mindspec init` ran at all).
	runGitignoreTestGit(t, root, "add", "-f", "--", entry, ".gitignore")
	runGitignoreTestGit(t, root, "commit", "-q", "-m", "seed tracked runtime file")

	// Ground truth: plain `git check-ignore` (no --no-index) reports the
	// tracked path as NOT ignored despite the rule being present — this
	// is exactly the trap FIX A closes via --no-index internally.
	plain := exec.Command("git", "check-ignore", "--quiet", "--", entry)
	plain.Dir = root
	if err := plain.Run(); err == nil {
		t.Fatalf("test setup invalid: plain git check-ignore unexpectedly reports %q ignored while tracked", entry)
	}

	first, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if err := EnsureGitignoreEntries(root, entry); err != nil {
			t.Fatalf("call %d: EnsureGitignoreEntries: %v", i, err)
		}
		got, err := os.ReadFile(filepath.Join(root, ".gitignore"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(first) {
			t.Fatalf("call %d: .gitignore changed for an already-tracked, already-rule-covered file (non-idempotent — checkIgnored missing --no-index):\nbefore:\n%s\nafter:\n%s", i, first, got)
		}
	}
}

// TestEnsureGitignoreEntries_DeeperNegationHonest is the round-2
// final-review FIX B pin: a DEEPER .gitignore (e.g. .mindspec/.gitignore)
// containing a negation for the runtime file takes precedence over the
// root .gitignore's plain rule regardless of root-file line order — the
// root file simply cannot reach in and override it. EnsureGitignoreEntries
// must not discover this on every call and re-append an identical
// duplicate line forever (it can never fix the deeper file from here); it
// appends the root entry AT MOST ONCE (when first missing) and, once
// present as the last line of the root file, must stop growing the file on
// subsequent calls even though `git check-ignore` still (rightly) reports
// the path as not-ignored.
func TestEnsureGitignoreEntries_DeeperNegationHonest(t *testing.T) {
	root := t.TempDir()
	runGitignoreTestGit(t, root, "init", "-q")

	entry := ".mindspec/session.json"
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A deeper .gitignore negates the runtime file; deeper files win
	// over root regardless of order.
	if err := os.WriteFile(filepath.Join(root, ".mindspec", ".gitignore"), []byte("!session.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ground truth: even with --no-index, the deeper negation wins.
	groundTruth := func() bool {
		cmd := exec.Command("git", "check-ignore", "--quiet", "--no-index", "--", entry)
		cmd.Dir = root
		return cmd.Run() == nil
	}
	if groundTruth() {
		t.Fatalf("test setup invalid: %q reported ignored despite deeper negation", entry)
	}

	// First call: entry is missing, so it's appended exactly once.
	if err := EnsureGitignoreEntries(root, entry); err != nil {
		t.Fatalf("first call: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	n := strings.Count(string(first), entry+"\n")
	if n != 1 {
		t.Fatalf("expected exactly one occurrence of %q after first call, got %d:\n%s", entry, n, first)
	}
	if groundTruth() {
		t.Fatalf("test setup invalid: deeper negation should still defeat the root entry after first call")
	}

	// Repeated calls must NOT keep re-appending: the deeper negation is
	// beyond the root file's reach, so once the entry is present (as the
	// last line of the root file) a further identical append cannot
	// help and must not happen.
	for i := 0; i < 3; i++ {
		if err := EnsureGitignoreEntries(root, entry); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		got, err := os.ReadFile(filepath.Join(root, ".gitignore"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(first) {
			t.Fatalf("call %d: .gitignore kept growing despite an unreachable deeper negation (no loop guard):\nafter first call:\n%s\nafter call %d:\n%s", i, first, i, got)
		}
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
