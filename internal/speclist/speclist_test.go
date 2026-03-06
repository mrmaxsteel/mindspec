package speclist

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestList_ScansSpecDirs(t *testing.T) {
	root := setupTestRoot(t,
		specDir("001-alpha", "Draft"),
		specDir("002-beta", "Approved"),
	)

	// Stub beads to return no epics
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	specs, err := List(root)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}

	if specs[0].SpecID != "001-alpha" {
		t.Errorf("specs[0].SpecID = %q, want %q", specs[0].SpecID, "001-alpha")
	}
	if specs[0].Status != "Draft" {
		t.Errorf("specs[0].Status = %q, want %q", specs[0].Status, "Draft")
	}
	if specs[0].Phase != "—" {
		t.Errorf("specs[0].Phase = %q, want %q", specs[0].Phase, "—")
	}

	if specs[1].SpecID != "002-beta" {
		t.Errorf("specs[1].SpecID = %q, want %q", specs[1].SpecID, "002-beta")
	}
	if specs[1].Status != "Approved" {
		t.Errorf("specs[1].Status = %q, want %q", specs[1].Status, "Approved")
	}
}

func TestList_SortsBySpecID(t *testing.T) {
	root := setupTestRoot(t,
		specDir("010-zulu", "Draft"),
		specDir("003-alpha", "Draft"),
		specDir("007-mike", "Approved"),
	)

	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	specs, err := List(root)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(specs) != 3 {
		t.Fatalf("expected 3 specs, got %d", len(specs))
	}

	want := []string{"003-alpha", "007-mike", "010-zulu"}
	for i, w := range want {
		if specs[i].SpecID != w {
			t.Errorf("specs[%d].SpecID = %q, want %q", i, specs[i].SpecID, w)
		}
	}
}

func TestList_EmptySpecsDir(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0755)

	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	specs, err := List(root)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 specs, got %d", len(specs))
	}
}

func TestReadFrontmatterStatus(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "draft",
			content: "---\nstatus: Draft\n---\n# Spec",
			want:    "Draft",
		},
		{
			name:    "approved",
			content: "---\nstatus: Approved\napproved_at: \"2026-01-01\"\n---\n# Spec",
			want:    "Approved",
		},
		{
			name:    "no frontmatter",
			content: "# Spec\nSome content",
			want:    "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "spec.md")
			os.WriteFile(path, []byte(tt.content), 0644)

			got := readFrontmatterStatus(path)
			if got != tt.want {
				t.Errorf("readFrontmatterStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

type specDirOpt struct {
	id     string
	status string
}

func specDir(id, status string) specDirOpt {
	return specDirOpt{id: id, status: status}
}

func setupTestRoot(t *testing.T, specs ...specDirOpt) string {
	t.Helper()
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	for _, s := range specs {
		dir := filepath.Join(specsDir, s.id)
		os.MkdirAll(dir, 0755)
		content := "---\nstatus: " + s.status + "\n---\n# Spec " + s.id + "\n"
		os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0644)
	}

	return root
}
