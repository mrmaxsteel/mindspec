package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifest writes an OWNERSHIP.yaml under
// root/.mindspec/docs/domains/<domain>/OWNERSHIP.yaml with the given
// raw YAML body. It fails the test if any I/O step fails.
func writeManifest(t *testing.T, root, domain, body string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "docs", "domains", domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write OWNERSHIP.yaml: %v", err)
	}
}

func TestOwnershipMultiMatchFirstWins(t *testing.T) {
	root := t.TempDir()
	// alpha and beta both claim the same path; alpha is
	// lexicographically earlier and must win.
	writeManifest(t, root, "alpha", "paths:\n  - internal/foo/**\n")
	writeManifest(t, root, "beta", "paths:\n  - internal/foo/**\n")

	domains := []string{"alpha", "beta"} // already sorted
	owner, o, err := attributeDomain(root, "internal/foo/bar.go", domains)
	if err != nil {
		t.Fatalf("attributeDomain err: %v", err)
	}
	if owner != "alpha" {
		t.Fatalf("expected alpha to win first-match, got %q", owner)
	}
	if o == nil || o.ManifestPath == "" {
		t.Fatalf("expected non-fallback Ownership for alpha, got %+v", o)
	}
	if !strings.Contains(o.ManifestPath, filepath.Join("alpha", "OWNERSHIP.yaml")) {
		t.Fatalf("manifest path should point at alpha, got %q", o.ManifestPath)
	}
}

func TestOwnershipRejectsExcludedTrees(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "naughty", "paths:\n  - viz/foo/**\n")
	_, err := loadOwnership(root, "naughty")
	if err == nil {
		t.Fatalf("expected load error for viz/ entry; got nil")
	}
	if !strings.Contains(err.Error(), "viz") {
		t.Fatalf("error should name offending segment, got: %v", err)
	}
	if !strings.Contains(err.Error(), "viz/foo/**") {
		t.Fatalf("error should name offending entry, got: %v", err)
	}

	// Also reject when the violator appears in `exclude:`.
	root2 := t.TempDir()
	writeManifest(t, root2, "naughty2", "paths:\n  - internal/foo/**\nexclude:\n  - agentmind/inner/**\n")
	_, err = loadOwnership(root2, "naughty2")
	if err == nil || !strings.Contains(err.Error(), "agentmind") {
		t.Fatalf("expected error naming agentmind exclude entry; got: %v", err)
	}

	root3 := t.TempDir()
	writeManifest(t, root3, "naughty3", "paths:\n  - bench/v2/foo/**\n")
	if _, err := loadOwnership(root3, "naughty3"); err == nil || !strings.Contains(err.Error(), "bench") {
		t.Fatalf("expected error naming bench entry; got: %v", err)
	}
}

func TestOwnershipFallback(t *testing.T) {
	root := t.TempDir()
	// Domain dir exists on disk (we create it) but no OWNERSHIP.yaml
	// is present. loadOwnership must return the fallback
	// Ownership with empty ManifestPath.
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "domains", "freshdomain"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	o, err := loadOwnership(root, "freshdomain")
	if err != nil {
		t.Fatalf("loadOwnership err: %v", err)
	}
	if o.ManifestPath != "" {
		t.Fatalf("expected empty ManifestPath for fallback, got %q", o.ManifestPath)
	}
	if len(o.Paths) != 1 || o.Paths[0] != "internal/freshdomain/**" {
		t.Fatalf("expected fallback paths [internal/freshdomain/**], got %v", o.Paths)
	}

	// attributeDomain with the fallback should still match files
	// inside that internal/<domain>/ tree.
	owner, o2, err := attributeDomain(root, "internal/freshdomain/sub/file.go", []string{"freshdomain"})
	if err != nil {
		t.Fatalf("attributeDomain err: %v", err)
	}
	if owner != "freshdomain" {
		t.Fatalf("expected fallback to attribute to freshdomain, got %q", owner)
	}
	if o2.ManifestPath != "" {
		t.Fatalf("expected fallback ownership, got manifest %q", o2.ManifestPath)
	}

	// A file that no domain claims yields the empty result.
	owner3, _, err := attributeDomain(root, "cmd/other/main.go", []string{"freshdomain"})
	if err != nil {
		t.Fatalf("attributeDomain err: %v", err)
	}
	if owner3 != "" {
		t.Fatalf("expected no-match for cmd/, got %q", owner3)
	}
}

func TestGlobMatchBasics(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// Leading **/
		{"leading-deep", "**/foo.go", "internal/x/y/foo.go", true},
		{"leading-one", "**/foo.go", "internal/foo.go", true},
		{"leading-bare", "**/foo.go", "foo.go", true},

		// Trailing /**
		{"trailing-descendant", "internal/foo/**", "internal/foo/bar/baz.go", true},
		{"trailing-self", "internal/foo/**", "internal/foo", true},
		{"trailing-miss", "internal/foo/**", "internal/bar/baz.go", false},

		// Mid-path **
		{"mid-deep", "internal/**/foo.go", "internal/x/y/foo.go", true},
		{"mid-zero-segments", "internal/**/foo.go", "internal/foo.go", true},

		// ? single-char wildcard
		{"q-one", "foo?.go", "foo1.go", true},
		{"q-two", "foo?.go", "foo12.go", false},

		// Escaped *
		{"escaped-star-match", `foo\*.go`, "foo*.go", true},
		{"escaped-star-nomatch", `foo\*.go`, "foobar.go", false},

		// Clear no-match
		{"no-match", "internal/foo/**", "cmd/bar.go", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := globMatch(tc.pattern, tc.path)
			if got != tc.want {
				t.Fatalf("globMatch(%q, %q) = %v; want %v", tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}
