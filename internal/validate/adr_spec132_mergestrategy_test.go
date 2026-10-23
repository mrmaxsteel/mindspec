package validate

import (
	"os"
	"strings"
	"testing"
)

func TestDocs_Spec132Amendments_MergeStrategy(t *testing.T) {
	for _, path := range []string{
		"../../.mindspec/adr/ADR-0006.md",
		"../../.mindspec/adr/ADR-0019.md",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			_, note, ok := strings.Cut(string(data), "**Removal note (2026-10-22, Spec 132):**")
			if !ok {
				t.Fatal("missing dated removal note")
			}
			note, _, _ = strings.Cut(strings.TrimSpace(note), "\n\n")
			note = strings.Join(strings.Fields(note), " ")
			for _, phrase := range []string{
				"The `merge_strategy` config key is removed.",
				"Legacy configs still load; the loader ignores the key.",
				"`impl approve` pushes when a git remote exists, otherwise it merges locally.",
				"decisions in this ADR still apply.",
			} {
				if !strings.Contains(note, phrase) {
					t.Errorf("removal note lacks %q", phrase)
				}
			}
		})
	}
}
