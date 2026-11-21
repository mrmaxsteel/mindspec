package panel

import "testing"

func TestS134Finalize_ArtifactPaths(t *testing.T) {
	for _, layout := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		t.Run(layout, func(t *testing.T) {
			specDir := "/repo/" + layout + "/001-own"
			for _, name := range []string{layout + "/001-own/spec.md", layout + "/001-own/reviews/old/x.go", "review/recorded/x.go", "reviews/recorded/x_test.go", ".beads/issues.jsonl"} {
				if !FinalizeArtifactPath(name, specDir, "review/recorded/", "reviews/recorded/") {
					t.Errorf("artifact refused: %s", name)
				}
			}
			for _, name := range []string{layout + "/002-other/x.go", layout + "/001-own-extra/x.go", "review/evil/x.go", "reviews/evil/x_test.go", "review/recorded-extra/x.go"} {
				if FinalizeArtifactPath(name, specDir, "review/recorded/", "reviews/recorded/") {
					t.Errorf("unregistered path allowed: %s", name)
				}
			}
		})
	}
}
