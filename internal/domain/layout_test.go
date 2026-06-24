package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestList_FlatTree pins AC5 (spec 106): `domain list` enumerates the FLAT
// domains root (.mindspec/domains) on a flat tree, via the Bead-1 tier-aware
// accessor (Req 3).
func TestList_FlatTree(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"core", "workflow"} {
		if err := os.MkdirAll(filepath.Join(root, ".mindspec", "domains", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 || entries[0].Name != "core" || entries[1].Name != "workflow" {
		t.Fatalf("flat domain enumeration = %+v, want [core workflow]", entries)
	}
}

// TestShow_FlatTree pins AC5 (spec 106): `domain show` reads a domain's
// overview from the FLAT .mindspec/domains/<d> root.
func TestShow_FlatTree(t *testing.T) {
	root := t.TempDir()
	dDir := filepath.Join(root, ".mindspec", "domains", "workflow")
	if err := os.MkdirAll(dDir, 0o755); err != nil {
		t.Fatal(err)
	}
	overview := "# workflow\n\n## What This Domain Owns\n\nthe lifecycle gates\n"
	if err := os.WriteFile(filepath.Join(dDir, "overview.md"), []byte(overview), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Show(root, "workflow")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if info.Name != "workflow" {
		t.Errorf("Name = %q, want workflow", info.Name)
	}
	if !strings.Contains(info.Owns, "the lifecycle gates") {
		t.Errorf("Owns should read the flat overview.md; got %q", info.Owns)
	}
}
