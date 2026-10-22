package validate

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func panelDocRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot find repository")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
func readPanelDoc(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(panelDocRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func requirePanelDoc(t *testing.T, body string, fragments ...string) {
	t.Helper()
	for _, f := range fragments {
		if !strings.Contains(body, f) {
			t.Errorf("missing %q", f)
		}
	}
}
func retiredPanelClaimProblems(body string) []string {
	var problems []string
	for _, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, retired := range []string{"panel.substitution", "claude_sub_on_quota"} {
			if strings.Contains(line, retired) && !strings.Contains(lower, "retired") {
				problems = append(problems, line)
			}
		}
		if regexp.MustCompile(`(?i)two model families|second model famil|second family|3 claude \+ 3 codex`).MatchString(line) {
			problems = append(problems, line)
		}
	}
	return problems
}

func TestUserDocs_NoRetiredPanelClaims(t *testing.T) {
	for _, path := range []string{"README.md", "plugins/mindspec/README.md", "project-docs/user/guides/review-panels.md"} {
		t.Run(path, func(t *testing.T) {
			body := readPanelDoc(t, path)
			for _, p := range retiredPanelClaimProblems(body) {
				t.Error(p)
			}
			requirePanelDoc(t, body, "one harness family", "panel", "runner")
			if path == "plugins/mindspec/README.md" {
				requirePanelDoc(t, body, "on_quota", "panel.runners", "panel submit", "config.local.yaml")
				if regexp.MustCompile("(?im)^.*(?:require|need|PATH).*`?codex`?.*$|^.*`?codex`? CLI.*$").MatchString(body) {
					t.Error("requires a named reviewer CLI")
				}
			}
			if strings.HasSuffix(path, "review-panels.md") {
				requirePanelDoc(t, body, "mindspec panel submit", "panel.runners", "on_quota: harness", "no submission", "Exit 0", "Exit 1", "Exit 2", "harness draft", "quota handoff", "handoff_pending", "--dispatch", "--replace", "Windows")
				for _, status := range []string{"final", "damaged", "void", "draft", "running", "quota_wall", "launch_failed", "malformed", "unsubmitted", "missing"} {
					requirePanelDoc(t, body, "| `"+status+"` |")
				}
			}
			for _, wrong := range []string{"panel.substitution enables quota fallback", "claude_sub_on_quota: true", "two model families", "3 Claude + 3 Codex", "Codex additionally serves as the second model family on review panels.", "SECOND MODEL FAMILY", "second family", "SECOND FAMILY"} {
				if len(retiredPanelClaimProblems(body+"\n"+wrong)) == 0 {
					t.Fatalf("scanner accepted %q", wrong)
				}
			}
		})
	}
	t.Run("interfaces", func(t *testing.T) {
		body := readPanelDoc(t, ".mindspec/domains/workflow/interfaces.md")
		_, section, ok := strings.Cut(body, "### Panel Artifact Schema\n")
		if !ok {
			t.Fatal("missing artifact schema section")
		}
		section, _, _ = strings.Cut(section, "\n### ")
		requirePanelDoc(t, section, "verdict schema v2", "mindspec panel submit", "latest submission", "exits 0", "exits 1", "exits 2")
		for _, status := range []string{"final", "damaged", "void", "draft", "running", "quota_wall", "launch_failed", "malformed", "unsubmitted", "missing"} {
			requirePanelDoc(t, section, "`"+status+"`")
		}
	})
	t.Run("changelog", func(t *testing.T) {
		body := readPanelDoc(t, "CHANGELOG.md")
		_, body, ok := strings.Cut(body, "## [0.18.0]")
		if !ok {
			t.Fatal("missing 0.18.0 section")
		}
		body, _, _ = strings.Cut(body, "\n## [")
		requirePanelDoc(t, body, "schema v2 with no opt-out", "without a submission no longer counts", "latest submission replaces", "No tool runner ships", "family needs a runner", "panel.substitution` is retired", "on_quota", "exits 0", "exits 1", "exits 2", "falsifiability, scope-honesty, contradiction and feasibility", "first v2 round", "--round <N+1> --reviewers <n>", "model-only reviewer", "`config resolve` and `doctor` warn")
		requirePanelDoc(t, body, "# before: create now refuses this entry\nreviewers: [{family: codex, model: m1}]", "# after\nrunners: [{family: codex, kind: process, argv: [codex, \"{prompt_file}\"], prompt: file, output: stdout}]\nreviewers: [{family: codex, model: m1}]")
	})
}
