package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifest writes an OWNERSHIP.yaml for domain under the canonical
// .mindspec/docs/domains/<domain>/ layout (the layout
// validate.LoadOwnership reads).
func writeManifest(t *testing.T, root, domain, content string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "docs", "domains", domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// touchFile creates an (empty) file at rel under root, making parents.
func touchFile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func manifestWarn(r *Report, domain string) *Check {
	name := manifestCheckName(domain)
	for i := range r.Checks {
		if r.Checks[i].Name == name && strings.Contains(r.Checks[i].Message, "dead-manifest") {
			return &r.Checks[i]
		}
	}
	return nil
}

// TestDeadManifest exercises spec 091 Req 17.
func TestDeadManifest(t *testing.T) {
	t.Run("fires for paths resolving to zero files", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, "foo", "paths:\n  - internal/foo/**\n")
		// no internal/foo/ directory present

		r := &Report{}
		checkOwnershipManifests(r, root)

		c := manifestWarn(r, "foo")
		if c == nil {
			t.Fatal("expected dead-manifest Warn for domain foo")
		}
		if c.Status != Warn {
			t.Errorf("dead-manifest must be Warn, got %d", c.Status)
		}
		if !strings.Contains(c.Message, "internal/foo/**") {
			t.Errorf("message must name the suspect glob, got %q", c.Message)
		}
	})

	t.Run("clears once a manifest path matches a file", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, "foo", "paths:\n  - internal/foo/**\n")
		touchFile(t, root, "internal/foo/bar.go")

		r := &Report{}
		checkOwnershipManifests(r, root)

		if c := manifestWarn(r, "foo"); c != nil {
			t.Errorf("expected NO dead-manifest Warn once a file matches, got %q", c.Message)
		}
	})

	t.Run("empty stub fires with (empty) suspect", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, "foo", "paths: []\n")

		r := &Report{}
		checkOwnershipManifests(r, root)

		c := manifestWarn(r, "foo")
		if c == nil {
			t.Fatal("expected dead-manifest Warn for an empty stub")
		}
		if !strings.Contains(c.Message, "(empty)") {
			t.Errorf("empty stub suspect must be (empty), got %q", c.Message)
		}
	})

	t.Run("does NOT fire for a missing manifest", func(t *testing.T) {
		root := t.TempDir()
		// domain dir exists but NO OWNERSHIP.yaml
		ghostDir := filepath.Join(root, ".mindspec", "docs", "domains", "ghost")
		if err := os.MkdirAll(ghostDir, 0o755); err != nil {
			t.Fatal(err)
		}

		r := &Report{}
		checkOwnershipManifests(r, root)

		if c := manifestWarn(r, "ghost"); c != nil {
			t.Errorf("dead-manifest must NOT fire for a missing manifest, got %q", c.Message)
		}
	})

	t.Run("V2-6: glob matching ONLY an excluded-tree file still fires", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, "foo", "paths:\n  - internal/foo/**\n")
		// The only matching file lives under an excluded tree.
		touchFile(t, root, ".worktrees/wt1/internal/foo/bar.go")

		r := &Report{}
		checkOwnershipManifests(r, root)

		c := manifestWarn(r, "foo")
		if c == nil {
			t.Fatal("expected dead-manifest Warn — excluded-tree match must not mask a dead manifest")
		}
	})
}

// TestHygieneWarns exercises spec 091 Req 20: duplicate-entry,
// redundant-subpath, domain-overlap. All advisory.
func TestHygieneWarns(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "a", "paths:\n  - internal/a/**\n  - internal/a/**\n  - internal/a/sub/**\n  - internal/shared/**\n")
	writeManifest(t, root, "b", "paths:\n  - internal/b/**\n  - internal/shared/**\n")

	r := &Report{}
	checkOwnershipManifests(r, root)

	wantSubstrings := []string{"duplicate-entry", "redundant-subpath", "domain-overlap"}
	for _, want := range wantSubstrings {
		if findCheckContaining(r, want) == nil {
			t.Errorf("expected a %s Warn", want)
		}
	}

	// redundant-subpath names both entries.
	sub := findCheckContaining(r, "redundant-subpath")
	if sub != nil {
		if !strings.Contains(sub.Message, "internal/a/sub/**") || !strings.Contains(sub.Message, "internal/a/**") {
			t.Errorf("redundant-subpath must name both entries, got %q", sub.Message)
		}
	}

	// domain-overlap names both claimants and the path.
	ov := findCheckContaining(r, "domain-overlap")
	if ov != nil {
		if !strings.Contains(ov.Message, "internal/shared/**") || !strings.Contains(ov.Message, "a") || !strings.Contains(ov.Message, "b") {
			t.Errorf("domain-overlap must name the path and both domains, got %q", ov.Message)
		}
	}
}

// TestHygieneWarns_DoNotBlock asserts that hygiene/dead-manifest Warns
// never flip the report to a failure (advisory only, Req 20 / Req 17).
func TestHygieneWarns_DoNotBlock(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "a", "paths:\n  - internal/a/**\n  - internal/a/**\n  - internal/shared/**\n")
	writeManifest(t, root, "b", "paths:\n  - internal/shared/**\n")

	r := &Report{}
	checkOwnershipManifests(r, root)

	if r.HasFailures() {
		t.Error("hygiene/dead-manifest Warns must not block the gate (HasFailures should be false)")
	}
}

// TestDuplicateEntry_InExclude covers the exclude-list duplicate case.
func TestDuplicateEntry_InExclude(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "a", "paths:\n  - internal/a/**\nexclude:\n  - internal/a/gen/**\n  - internal/a/gen/**\n")

	r := &Report{}
	checkOwnershipManifests(r, root)

	c := findCheckContaining(r, "duplicate-entry")
	if c == nil {
		t.Fatal("expected duplicate-entry Warn for a duplicated exclude path")
	}
	if !strings.Contains(c.Message, "exclude") {
		t.Errorf("duplicate-entry must name the field (exclude), got %q", c.Message)
	}
}

// TestOwnershipFixer_ScaffoldsStub proves the missing-OWNERSHIP check is
// fixable (Req 8/15): --fix writes the empty stub and surfaces the
// populate prompt; it never overwrites an existing manifest.
func TestOwnershipFixer_ScaffoldsStub(t *testing.T) {
	root := t.TempDir()
	// Canonical domain dir with the four standard docs but no manifest.
	domainDir := filepath.Join(root, ".mindspec", "docs", "domains", "foo")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range domainFiles {
		if err := os.WriteFile(filepath.Join(domainDir, f), []byte("# foo"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	r := &Report{}
	checkDomains(r, root, ".mindspec/docs")

	ownerName := ".mindspec/docs/domains/foo/OWNERSHIP.yaml"
	c := findCheck(r, ownerName)
	if c == nil {
		t.Fatalf("expected OWNERSHIP.yaml check at %s", ownerName)
	}
	if c.Status != Warn {
		t.Fatalf("expected Warn for missing manifest, got %d", c.Status)
	}
	// Req 21 message: names the fix command, no stale "falls back" claim.
	if !strings.Contains(c.Message, "run 'mindspec doctor --fix' to scaffold a default manifest") {
		t.Errorf("missing-OWNERSHIP Warn must name the --fix remedy, got %q", c.Message)
	}
	if strings.Contains(c.Message, "falls back") {
		t.Errorf("missing-OWNERSHIP Warn must NOT carry the stale 'falls back' claim, got %q", c.Message)
	}
	if c.FixFunc == nil {
		t.Fatal("missing-OWNERSHIP check must be fixable")
	}

	r.Fix()

	stubPath := filepath.Join(domainDir, "OWNERSHIP.yaml")
	got, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("fixer did not write the stub: %v", err)
	}
	if !strings.Contains(string(got), "paths: []") {
		t.Errorf("stub must contain paths: [], got:\n%s", got)
	}
	if strings.Contains(string(got), "- internal/foo/**") {
		t.Errorf("stub must NOT pre-fill paths, got:\n%s", got)
	}
	// Req 15: --fix surfaces the populate prompt via the check message.
	// The prompt is BuildPopulatePrompt(domain); assert on its
	// domain-specific opening line.
	if !strings.Contains(c.Message, "Populate .mindspec/docs/domains/foo/OWNERSHIP.yaml") {
		t.Errorf("fix output must surface the populate prompt, got %q", c.Message)
	}
}

// TestOwnershipFixer_NeverOverwrites covers spec 091 Req 8's
// no-overwrite carve-out (including --fix --force, exercised via the
// FixFunc which is force-independent).
func TestOwnershipFixer_NeverOverwrites(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, ".mindspec", "docs", "domains", "foo")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range domainFiles {
		if err := os.WriteFile(filepath.Join(domainDir, f), []byte("# foo"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifestPath := filepath.Join(domainDir, "OWNERSHIP.yaml")
	hand := []byte("paths:\n  - cmd/foo-cli/**\n")
	if err := os.WriteFile(manifestPath, hand, 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(manifestPath)

	// A present manifest reports OK and has no FixFunc, so a full --fix
	// run leaves it untouched. Directly exercise the fixer to prove the
	// no-overwrite guard regardless of dispatch.
	fix := makeOwnershipFixFunc(&Report{Checks: []Check{{}}}, 0, manifestPath, "foo")
	if err := fix(); err != nil {
		t.Fatal(err)
	}

	after, _ := os.ReadFile(manifestPath)
	if string(before) != string(after) {
		t.Errorf("existing manifest must be byte-identical after fix.\nbefore=%q\nafter=%q", before, after)
	}
}
