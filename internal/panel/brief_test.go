package panel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_QuotedMarkersRoundBump(t *testing.T) {
	for _, marker := range []string{briefHeaderOpen, briefHeaderClose} {
		t.Run(marker, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "reviews", "p")
			in := CreateInput{
				Spec: "fixture", Target: "main", Round: 1, ExpectedReviewers: 2,
				Roster: createRoster(2), SubjectDigest: "sha256:fixture",
				ReviewedHeadSHA: "cafefeedcafefeedcafefeedcafefeedcafefeed",
				BriefRegions:    BriefRegions{SpecACs: "## Acceptance Criteria\n\n```text\n" + marker + "\n```\n"},
			}
			for round := 1; round <= 2; round++ {
				in.Round = round
				if err := Create(dir, in); err != nil {
					t.Fatalf("Create round %d: %v", round, err)
				}
			}
			data, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
			if err != nil {
				t.Fatal(err)
			}
			brief := string(data)
			if !strings.Contains(brief, "````text\n"+in.BriefRegions.SpecACs+"````\n") {
				t.Fatalf("quotation changed:\n%s", brief)
			}
			for _, genuine := range []string{briefHeaderOpen, briefHeaderClose} {
				if got := findMarkerLines(brief, genuine); len(got) != 1 {
					t.Fatalf("want one genuine %q marker, got %v", genuine, got)
				}
			}
		})
	}
}

func TestBrief_FenceEscalation(t *testing.T) {
	quote := "## Acceptance Criteria\n````text\nexample\n````\n"
	got := renderBriefRegions(briefHeaderOpen+"\n## Your job\n", BriefRegions{SpecACs: quote})
	if !strings.Contains(got, "`````text\n"+quote+"`````\n") {
		t.Fatalf("outer fence must be longer than the quoted fence:\n%s", got)
	}
}

func TestBrief_EscapeText(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"printable", "- [ ] AC-1: say \"hi\" — ok", "- [ ] AC-1: say \"hi\" — ok"},
		{"controls", "- Escape \x1b[31m \u202e \u200b \xff\n", "- Escape \\x1b[31m \\u202e \\u200b \\xff\n"},
		{"replacement-rune", "valid \ufffd", "valid \ufffd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeBriefText(tc.text)
			if got != tc.want {
				t.Fatalf("escaped text: got %q, want %q", got, tc.want)
			}
			if again := escapeBriefText(got); again != got {
				t.Fatalf("escape is not idempotent: got %q, then %q", got, again)
			}
		})
	}
}
