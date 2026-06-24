package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckDocs_FlatTree pins AC5 (spec 106): the doctor docs scan probes the
// FLAT .mindspec/{specs,domains} roots on a flat tree — via the tier-aware
// docsRootRel (Req 3) — rather than the canonical/legacy DocsDir. RED-on-revert:
// a DocsDir-based docsRootRel would probe docs/{specs,domains} and report the
// flat dirs Missing.
func TestCheckDocs_FlatTree(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"specs", filepath.Join("domains", "workflow")} {
		if err := os.MkdirAll(filepath.Join(root, ".mindspec", sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A domain file so checkDomains iterates into workflow/.
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "domains", "workflow", "overview.md"), []byte("# wf"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkDocs(r, root)

	statusOf := func(name string) (Status, bool) {
		for _, c := range r.Checks {
			if c.Name == name {
				return c.Status, true
			}
		}
		return Missing, false
	}

	// The required-dir checks must be named with the FLAT .mindspec/ root and
	// report OK (the dirs are present).
	for _, name := range []string{".mindspec/specs/", ".mindspec/domains/"} {
		st, ok := statusOf(name)
		if !ok {
			t.Errorf("expected flat required-dir check %q, not found", name)
			continue
		}
		if st != OK {
			t.Errorf("check %q status = %d, want OK (flat dir present)", name, st)
		}
	}

	// The flat domain doc check must be present under the flat root and OK.
	if st, ok := statusOf(".mindspec/domains/workflow/overview.md"); !ok || st != OK {
		t.Errorf("flat domain doc check .mindspec/domains/workflow/overview.md: ok=%v status=%d, want OK", ok, st)
	}
}
