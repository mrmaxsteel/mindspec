package setup

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPanelSkills_NoHandEditInstructions(t *testing.T) {
	root := repoRoot(t)
	for _, base := range []string{"plugins/mindspec/skills", ".claude/skills"} {
		for _, name := range []string{"ms-panel-run", "ms-panel-tally"} {
			t.Run(base+"/"+name, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(root, base, name, "SKILL.md"))
				if err != nil {
					t.Fatal(err)
				}
				text := string(data)
				for _, forbidden := range []string{"there is no CLI verb", "set `\"abandoned\": true`", "sets by hand", "directly in `panel.json`", "append one entry to the `refutations` array", "disposition-manifest.json", "n-1", "N−1", "N − 1"} {
					if strings.Contains(text, forbidden) {
						t.Errorf("forbidden hand-edit/manifest/threshold instruction %q", forbidden)
					}
				}
				// The ms-panel-tally adapter runs the Next block that panel tally
				// prints; the verbs and the pass rule are binary output there.
				if name == "ms-panel-run" {
					for _, want := range []string{"mindspec panel refute", "mindspec panel defer", "mindspec panel abandon", "mindspec panel tally", "writes the coverage manifest", "fresh and intact", "all roster slots present", "0 REJECT", "0 hard_block", "0 INVALID", "0 unresolved REQUEST_CHANGES"} {
						if !strings.Contains(text, want) {
							t.Errorf("missing panel rule %q", want)
						}
					}
					if !strings.Contains(text, "mindspec panel submit") {
						t.Error("reviewers must use panel submit")
					}
				}
				for _, pattern := range []string{`ADR-[0-9]+`, `(?i)spec[- ]+[0-9]+`, `§`, `\bR[0-9]+(?:\.[0-9]+|[a-z])\b`, `\b[a-z_]+\.go\b`, `\bmindspec-[a-z0-9]{4}\b`, `#[0-9]{2,}`} {
					if regexp.MustCompile(pattern).MatchString(text) {
						t.Errorf("product skill contains provenance reference matching %s", pattern)
					}
				}
			})
		}
	}
}
