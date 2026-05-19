package adr

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCreateRejectsSupersedesTraversal is the SEC-1 regression test
// (bead mindspec-x1qr). Before the fix, `--supersedes "../../../tmp/poisoned"`
// caused internal/adr/create.go to mutate /tmp/poisoned.md. After the fix,
// every traversal/glob/separator/literal path must be rejected before any
// filesystem operation runs.
func TestCreateRejectsSupersedesTraversal(t *testing.T) {
	root := setupCreateEnv(t)

	// Bait file the pre-fix attack would have mutated.
	poison := filepath.Join(t.TempDir(), "sec1-poisoned.md")
	if err := os.WriteFile(poison, []byte("**Superseded-by**: n/a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalContent, err := os.ReadFile(poison)
	if err != nil {
		t.Fatal(err)
	}

	cases := []string{
		"../../../tmp/sec1-poisoned",
		"../../etc/passwd",
		"ADR-0001/../../../tmp/poisoned",
		"/etc/passwd",
		"ADR-0001*",     // glob injection in adr.Show fallback
		"ADR-*",         // glob
		"ADR-0001?",     // glob
		"ADR-0001[a-z]", // glob char class
		"ADR-0001{a,b}", // brace expansion
		"ADR-0001\\foo", // backslash
		".",
		"..",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := Create(root, "test", CreateOpts{Supersedes: c})
			if err == nil {
				t.Errorf("Create(--supersedes=%q) should have failed", c)
			}
		})
	}

	// Verify the bait file was not touched by any of the attempts.
	after, err := os.ReadFile(poison)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(originalContent) {
		t.Fatalf("CRITICAL: bait file was mutated despite validation\nbefore: %q\nafter:  %q", originalContent, after)
	}
}

// TestShowRejectsGlobInjection guards against `id` containing glob
// metacharacters reaching filepath.Glob in internal/adr/show.go.
func TestShowRejectsGlobInjection(t *testing.T) {
	root := setupCreateEnv(t)

	bad := []string{
		"ADR-*",
		"ADR-0001*",
		"../../etc/passwd",
		"ADR-0001?",
		"ADR-0001[abc]",
		"ADR-0001{x,y}",
		"ADR-0001\\foo",
		".",
		"..",
		"",
	}
	for _, c := range bad {
		t.Run(c, func(t *testing.T) {
			if _, err := Show(root, c); err == nil {
				t.Errorf("Show(%q) should have failed", c)
			}
		})
	}
}

// TestSupersedeRejectsTraversal exercises the lower-level Supersede call
// in case it's invoked directly (FileStore.Supersede).
func TestSupersedeRejectsTraversal(t *testing.T) {
	root := setupCreateEnv(t)

	cases := []struct{ oldID, newID string }{
		{"../../../tmp/x", "ADR-0099"},
		{"ADR-0001", "../evil"},
		{"ADR-0001*", "ADR-0099"},
		{"", "ADR-0099"},
		{"ADR-0001", ""},
	}
	for _, c := range cases {
		t.Run(c.oldID+"|"+c.newID, func(t *testing.T) {
			if err := Supersede(root, c.oldID, c.newID); err == nil {
				t.Errorf("Supersede(%q, %q) should have failed", c.oldID, c.newID)
			}
		})
	}
}

// TestCopyDomainsRejectsTraversal guards the read-arbitrary-file primitive.
func TestCopyDomainsRejectsTraversal(t *testing.T) {
	root := setupCreateEnv(t)

	bad := []string{
		"../../../tmp/x",
		"ADR-0001/../../../etc",
		"/etc/passwd",
		"ADR-*",
		"",
	}
	for _, c := range bad {
		t.Run(c, func(t *testing.T) {
			if _, err := CopyDomains(root, c); err == nil {
				t.Errorf("CopyDomains(%q) should have failed", c)
			}
		})
	}
}
