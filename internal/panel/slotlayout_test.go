package panel

import "testing"

func TestNewSlotLayout_TrailingSlash(t *testing.T) {
	mainRoot := t.TempDir()
	t.Run("same_layout", func(t *testing.T) {
		want, err := NewSlotLayout(mainRoot, ".mindspec-scratch", "spec", "panel", 1, "R1")
		if err != nil {
			t.Fatalf("root without trailing slash: %v", err)
		}
		got, err := NewSlotLayout(mainRoot, ".mindspec-scratch/", "spec", "panel", 1, "R1")
		if err != nil {
			t.Fatalf("root with one trailing slash: %v", err)
		}
		if got != want {
			t.Fatalf("layout with trailing slash = %+v, want %+v", got, want)
		}
	})
	for _, tc := range []struct {
		name string
		root string
	}{
		{"empty", ""},
		{"absolute", "/scratch"},
		{"slash", "/"},
		{"dot", "."},
		{"dot_slash", "./"},
		{"parent", ".."},
		{"parent_component", "a/../b"},
		{"dot_component", "a/./b"},
		{"empty_component", "a//b"},
		{"two_trailing_slashes", ".mindspec-scratch//"},
		{"nul", "a\x00b"},
		{"backslash", "a\\b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := NewSlotLayout(mainRoot, tc.root, "spec", "panel", 1, "R1"); err == nil {
				t.Fatalf("root %q accepted: %+v", tc.root, got)
			}
		})
	}
}
