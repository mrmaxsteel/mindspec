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

func TestFilledBriefStubBody(t *testing.T) {
	for _, tc := range []struct {
		body   string
		refuse bool
	}{
		{"<!--TODO(skill): fill -->", true}, {"<!--  todo(skill): fill -->", true}, {"<!--\nToDo(skill): fill -->", true},
		{"Mention TODO(skill) in prose.", false}, {"Mention todo(skill) in prose.", false},
	} {
		file := filepath.Join(t.TempDir(), "BRIEF.md")
		text := briefHeaderOpen + "\nQuoted <!-- TODO(skill): example -->\n" + briefHeaderClose + "\n\n" + tc.body
		if err := os.WriteFile(file, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := ReadFilledBrief(file)
		if (err != nil) != tc.refuse {
			t.Fatalf("body %q: %v", tc.body, err)
		}
	}
}

func TestReadFilledBriefSymlink(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "body.md"), filepath.Join(dir, "BRIEF.md")
	if err := os.WriteFile(target, []byte("Review the implementation."), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if _, err := ReadFilledBrief(link); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("expected regular-file refusal, got %v", err)
	}
}
