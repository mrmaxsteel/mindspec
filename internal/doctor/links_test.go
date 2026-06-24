package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFileAt(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCheckMovedTreeLinks_AllResolve asserts the link-existence lane reports
// zero 404s on a well-formed flat tree, leaving symmetric `../../adr/…`
// spec→ADR links untouched (AC10 green half).
func TestCheckMovedTreeLinks_AllResolve(t *testing.T) {
	root := t.TempDir()
	writeFileAt(t, root, ".mindspec/specs/000-x/spec.md", "# Spec\n[adr](../../adr/ADR-0001.md)\n[core](../../core/USAGE.md)\n")
	writeFileAt(t, root, ".mindspec/adr/ADR-0001.md", "# ADR\n[u](../core/USAGE.md)\n")
	writeFileAt(t, root, ".mindspec/core/USAGE.md", "# Usage\n[cm](../context-map.md)\n")
	writeFileAt(t, root, ".mindspec/domains/foo/overview.md", "# foo\n[a](architecture.md)\n")
	writeFileAt(t, root, ".mindspec/domains/foo/architecture.md", "# arch\n")
	writeFileAt(t, root, ".mindspec/context-map.md", "# CM\n[foo](domains/foo/overview.md)\n")
	writeFileAt(t, root, "README.md", "# P\n[s](.mindspec/specs/000-x/spec.md)\nExternal: [x](https://example.com)\nAnchor: [y](#section)\n")

	dangling, err := CheckMovedTreeLinks(root)
	if err != nil {
		t.Fatalf("CheckMovedTreeLinks: %v", err)
	}
	if len(dangling) != 0 {
		t.Errorf("expected zero dangling links, got %+v", dangling)
	}
}

// TestCheckMovedTreeLinks_ReportsDangling asserts a 404 (a moved-tree link
// pointing at a nonexistent file) is reported (AC10 fail half), and that
// external/anchor links are NOT flagged.
func TestCheckMovedTreeLinks_ReportsDangling(t *testing.T) {
	root := t.TempDir()
	writeFileAt(t, root, ".mindspec/specs/000-x/spec.md", "# Spec\n[gone](../../adr/ADR-9999.md)\n[ok](../../core/USAGE.md)\n[ext](https://x.test)\n")
	writeFileAt(t, root, ".mindspec/core/USAGE.md", "# Usage\n")

	dangling, err := CheckMovedTreeLinks(root)
	if err != nil {
		t.Fatalf("CheckMovedTreeLinks: %v", err)
	}
	if len(dangling) != 1 {
		t.Fatalf("expected exactly 1 dangling link, got %+v", dangling)
	}
	if dangling[0].Target != "../../adr/ADR-9999.md" {
		t.Errorf("unexpected dangling target: %+v", dangling[0])
	}
}

// TestLinksReport_RendersChecks asserts the report adapter renders an Error
// check per dangling link and an OK check when clean.
func TestLinksReport_RendersChecks(t *testing.T) {
	root := t.TempDir()
	writeFileAt(t, root, ".mindspec/core/A.md", "# A\n[bad](./missing.md)\n")

	checks := LinksReport(root)
	if len(checks) != 1 || checks[0].Status != Error {
		t.Fatalf("expected 1 Error check, got %+v", checks)
	}

	writeFileAt(t, root, ".mindspec/core/missing.md", "# now exists\n")
	checks = LinksReport(root)
	if len(checks) != 1 || checks[0].Status != OK {
		t.Fatalf("expected 1 OK check after fix, got %+v", checks)
	}
}

// TestCheckMovedTreeLinks_NonReadmeRootDocBrokenLink is the bead-3jq7
// regression: a broken local link in a tracked repo-root doc that is NOT
// README.md or AGENTS.md (here BENCH-MOVED.md) is now caught by the 404 gate.
// Before the fix the lane scanned only {README,AGENTS} among root docs, so the
// spec-106 flatten broke two BENCH-MOVED.md links and the gate reported GREEN.
func TestCheckMovedTreeLinks_NonReadmeRootDocBrokenLink(t *testing.T) {
	root := t.TempDir()
	// A clean flat lifecycle tree.
	writeFileAt(t, root, ".mindspec/specs/000-x/spec.md", "# Spec\n[adr](../../adr/ADR-0001.md)\n")
	writeFileAt(t, root, ".mindspec/adr/ADR-0001.md", "# ADR\n")
	// A NON-README/AGENTS repo-root doc with one broken local link and one good
	// link (the good link proves the scan is not flagging everything).
	writeFileAt(t, root, "BENCH-MOVED.md",
		"# Bench moved\n[good](.mindspec/adr/ADR-0001.md)\n[gone](.mindspec/docs/adr/ADR-0001.md)\n[ext](https://x.test)\n")

	dangling, err := CheckMovedTreeLinks(root)
	if err != nil {
		t.Fatalf("CheckMovedTreeLinks: %v", err)
	}
	if len(dangling) != 1 {
		t.Fatalf("expected exactly 1 dangling link from BENCH-MOVED.md, got %+v", dangling)
	}
	if dangling[0].File != "BENCH-MOVED.md" || dangling[0].Target != ".mindspec/docs/adr/ADR-0001.md" {
		t.Errorf("unexpected dangling link: %+v", dangling[0])
	}
}
