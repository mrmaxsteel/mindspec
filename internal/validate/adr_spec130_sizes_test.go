package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Sizes(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	_, amendment, ok := strings.Cut(text, "## Amendment (Spec 130)")
	if !ok {
		t.Fatal("missing Spec 130 amendment")
	}
	for _, phrase := range []string{
		"The default becomes 6/6/4/6 per gate (`spec_approve`/`plan_approve`/`bead`/`final_review`)",
		"`adhoc` defaulting to 6 (R2.1)",
		"this reverses spec 112 R2",
		"`DefaultConfig` holds no `panel.reviewers` list",
	} {
		if !strings.Contains(amendment, phrase) {
			t.Errorf("size amendment missing %q", phrase)
		}
	}
	_, historical, ok := strings.Cut(text, "## Amendment (Spec 126)")
	if !ok {
		t.Fatal("missing Spec 126 heading")
	}
	historical, _, _ = strings.Cut(historical, "## Amendment (Spec 130)")
	if !strings.Contains(historical, "**The shipped default is 6 / `n-1`, unconditionally.** (Size superseded by [Amendment (Spec 130)](#amendment-spec-130).)") {
		t.Error("Spec 126 size bullet lacks its Spec 130 back-pointer")
	}
	changelog, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	_, release, ok := strings.Cut(string(changelog), "## [0.17.0]")
	if !ok {
		t.Fatal("missing 0.17.0 heading")
	}
	release, _, _ = strings.Cut(release, "\n## [")
	for _, phrase := range []string{
		"Default panel size is set per gate",
		"6 slots",
		"bead panels have 4",
	} {
		if !strings.Contains(release, phrase) {
			t.Errorf("0.17.0 panel-size note missing %q", phrase)
		}
	}
}
