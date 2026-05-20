package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorOWNERSHIPYamlMissing verifies that checkDomains warns when a
// domain directory lacks OWNERSHIP.yaml and reports OK when present
// (spec-086 Bead 4).
func TestDoctorOWNERSHIPYamlMissing(t *testing.T) {
	t.Run("missing manifest emits Warn", func(t *testing.T) {
		root := t.TempDir()
		domainDir := filepath.Join(root, "docs", "domains", "foo")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Need at least one file so checkDomains iterates into foo/.
		if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# foo"), 0o644); err != nil {
			t.Fatal(err)
		}

		r := &Report{}
		checkDomains(r, root, "docs")

		var found *Check
		for i := range r.Checks {
			if r.Checks[i].Name == "docs/domains/foo/OWNERSHIP.yaml" {
				found = &r.Checks[i]
				break
			}
		}
		if found == nil {
			t.Fatal("expected OWNERSHIP.yaml check, got none")
		}
		if found.Status != Warn {
			t.Errorf("expected Warn status (not Missing/Error per Req 15), got %d", found.Status)
		}
		if !strings.Contains(found.Message, "OWNERSHIP.yaml") {
			t.Errorf("expected message to mention OWNERSHIP.yaml, got %q", found.Message)
		}
	})

	t.Run("present manifest emits OK", func(t *testing.T) {
		root := t.TempDir()
		domainDir := filepath.Join(root, "docs", "domains", "foo")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# foo"), 0o644); err != nil {
			t.Fatal(err)
		}
		manifest := []byte("domain: foo\nattributes: []\n")
		if err := os.WriteFile(filepath.Join(domainDir, "OWNERSHIP.yaml"), manifest, 0o644); err != nil {
			t.Fatal(err)
		}

		r := &Report{}
		checkDomains(r, root, "docs")

		var found *Check
		for i := range r.Checks {
			if r.Checks[i].Name == "docs/domains/foo/OWNERSHIP.yaml" {
				found = &r.Checks[i]
				break
			}
		}
		if found == nil {
			t.Fatal("expected OWNERSHIP.yaml check, got none")
		}
		if found.Status != OK {
			t.Errorf("expected OK status when manifest exists, got %d (msg=%q)", found.Status, found.Message)
		}
	})
}
