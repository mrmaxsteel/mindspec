package spec

import (
	"os"
	"path/filepath"
	"testing"
)

// TestList_FlatTree pins AC5 (spec 106): `spec list` enumerates the FLAT specs
// root (.mindspec/specs) on a flat tree, returning the same inventory it would
// on a canonical tree, via the Bead-1 tier-aware accessor (Req 3).
func TestList_FlatTree(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"001-alpha", "002-beta"} {
		dir := filepath.Join(root, ".mindspec", "specs", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nstatus: Draft\n---\n# " + id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	specs, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(specs) != 2 || specs[0].SpecID != "001-alpha" || specs[1].SpecID != "002-beta" {
		t.Fatalf("flat enumeration = %+v, want [001-alpha 002-beta]", specs)
	}
	// Status proves the flat spec.md was actually read (not just the dir name).
	if specs[0].Status != "Draft" {
		t.Errorf("specs[0].Status = %q, want Draft (flat spec.md must be read)", specs[0].Status)
	}
}
