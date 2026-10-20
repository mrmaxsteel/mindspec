package panel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreate_UnparseablePanelJSONRefuses(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{FileName: "{", "BRIEF.md": "authored brief\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Create(dir, CreateInput{Spec: "900", Target: "main", Round: 2, ExpectedReviewers: 2, ReviewedHeadSHA: "abc"}); err == nil {
		t.Fatal("unparseable panel.json must refuse")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("refusal created files: %v", entries)
	}
	for name, want := range map[string]string{FileName: "{", "BRIEF.md": "authored brief\n"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s changed: %q %v", name, got, err)
		}
	}
}
