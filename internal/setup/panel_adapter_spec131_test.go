package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var adapterSkillNames = []string{"ms-panel-run", "ms-panel-tally", "ms-bead-fix", "ms-bead-cycle", "ms-spec-final-review", "ms-bead-impl"}

func eachAdapterCopy(t *testing.T, check func(*testing.T, string, string)) {
	t.Helper()
	for _, base := range []string{"plugins/mindspec/skills", ".claude/skills"} {
		for _, name := range adapterSkillNames {
			t.Run(base+"/"+name, func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join(repoRoot(t), base, name, "SKILL.md"))
				if err != nil {
					t.Fatal(err)
				}
				check(t, name, string(raw))
			})
		}
	}
}
func requireAdapterText(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestPanelSkills_AdapterShape(t *testing.T) {
	eachAdapterCopy(t, func(t *testing.T, name, body string) {
		if strings.Contains(name, "ms-panel-") {
			if n := strings.Count(body, "\n"); n > 80 {
				t.Errorf("%d lines, limit 80", n)
			}
		}
		switch name {
		case "ms-panel-run":
			requireAdapterText(t, body, "runner: claude-code-workflow", "`/ms-panel` once", "{slug, spec, gate, target, bead_id, round}", "runner: claude-code-skills", "runner: external", "mindspec panel create", "mindspec panel verify <panel-slug> --json", "mindspec panel checkout <panel-slug> --slot <slot> --dispatch", "--replace", "mindspec panel prompt", "mindspec panel submit", "effective model", "cannot run that model, report", "model equals family", "harness default model", "verdict label", "after each process or agent ends", "On exit 0, invoke `/ms-panel-tally`")
		case "ms-panel-tally":
			requireAdapterText(t, body, "review.reproduce", "review.no-vote", "panel.artifact-gate", "--data - <<'JSON'", "mindspec panel disposition check", "replaced", "--json", "MINDSPEC_SKIP_PANEL=1", "mindspec panel rebind", "mindspec panel defer")
			if strings.Contains(body, "confidence") {
				t.Error("retired verdict field")
			}
		case "ms-spec-final-review":
			requireAdapterText(t, body, "mindspec panel checkout <panel-slug> --slot <slot>", "review.ci-parity", "commands.ci", "commands.test")
		case "ms-bead-cycle":
			requireAdapterText(t, body, "review.reproduce", "review.no-vote", "panel.fix-after-final", "panel.one-fixer", "Use one panel slug", "for all rounds", "Set `--round <R>` to the next round.")
		case "ms-bead-impl":
			requireAdapterText(t, body, "sequence.bead-impl-prompt", "review.derive", "lifecycle.complete-by-orchestrator")
		}
		canonical, err := os.ReadFile(filepath.Join(repoRoot(t), "plugins/mindspec/skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if body != string(canonical) {
			t.Error("installed skill differs from canonical")
		}
	})
}

type adapterActions struct{ process, harness string }

var statusActions = map[string]adapterActions{
	"missing":       {"`panel launch` in the background", "redispatch"},
	"running":       {"wait", "if the agent returned and the slot is still running, redispatch once, then report"},
	"draft":         {"`panel launch` once more, then report", "redispatch once, then report"},
	"unsubmitted":   {"`panel launch` once more, then report", "redispatch once, then report"},
	"void":          {"`panel launch` once more, then report", "redispatch once, then report"},
	"launch_failed": {"`panel launch` once more, then report", "redispatch once, then report"},
	"malformed":     {"`panel launch` once more, then report", "redispatch once, then report"},
	"quota_wall":    {"report", "with `handoff_pending`: redispatch; else report"},
	"damaged":       {"`panel recover`, then report", "report"},
}

func adapterStatusProblems(body string) []string {
	rows := map[string][]string{}
	var problems []string
	for _, line := range strings.Split(body, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 5 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}
		status := strings.Trim(strings.TrimSpace(cells[1]), "`")
		if _, exists := rows[status]; exists {
			problems = append(problems, "duplicate "+status)
		}
		rows[status] = []string{strings.TrimSpace(cells[2]), strings.TrimSpace(cells[3])}
	}
	if len(rows) != len(statusActions) {
		problems = append(problems, fmt.Sprintf("%d status rows, want %d", len(rows), len(statusActions)))
	}
	for status, want := range statusActions {
		got := rows[status]
		if len(got) != 2 || got[0] != want.process || !strings.Contains(got[1], want.harness) {
			problems = append(problems, "wrong actions for "+status)
		}
	}
	return problems
}
func TestPanelRun_StatusTable(t *testing.T) {
	eachAdapterCopy(t, func(t *testing.T, name, body string) {
		if name != "ms-panel-run" {
			return
		}
		for _, p := range adapterStatusProblems(body) {
			t.Error(p)
		}
		requireAdapterText(t, body, "one retry per slot and round", "next non-final, non-running status")
		// Wrong actions, duplicate and missing rows must fail the same predicate.
		for _, wrong := range []string{
			strings.Replace(body, "redispatch once, then report", "report without redispatch", 1),
			body + "\n| `missing` | wait | wait |\n",
			strings.Replace(body, "| `damaged` |", "| `unknown` |", 1),
		} {
			if len(adapterStatusProblems(wrong)) == 0 {
				t.Fatal("status table accepted a wrong mutation")
			}
		}
	})
}
func adapterSurfaceProblems(body string) []string {
	var problems []string
	// Only install-target values and path tokens may contain the harness name.
	body = strings.ReplaceAll(body, "runner: claude-code-workflow", "runner: adapter")
	body = strings.ReplaceAll(body, "runner: claude-code-skills", "runner: adapter")
	body = strings.ReplaceAll(body, ".claude/", ".adapter/")
	for _, pattern := range []string{`(?i)/tmp|codex|copilot|gemini|claude`, `ADR-[0-9]|(?i:spec[- ]+[0-9]{3})|\bR[0-9]+\.[0-9]|#[0-9]{3}|\bmindspec-[a-z0-9]{4}(?:\.[0-9]+)?\b|§`, `(?i)lens defaults|size ladder|failure detection`} {
		if regexp.MustCompile(pattern).MatchString(body) {
			problems = append(problems, pattern)
		}
	}
	return problems
}
func TestSkills_NoFamilyNoTmp(t *testing.T) {
	eachAdapterCopy(t, func(t *testing.T, name, body string) {
		for _, p := range adapterSurfaceProblems(body) {
			t.Errorf("%s matches %s", name, p)
		}
		for _, wrong := range []string{"/TMP/probe", "CoDeX", "claude agent", "ADR-0044", "spec 131", "R12.3", "#253", "mindspec-zadp.9", "§ conduct"} {
			if len(adapterSurfaceProblems(body+"\n"+wrong)) == 0 {
				t.Fatalf("scanner missed %q", wrong)
			}
		}
	})
}
func TestBeadFix_FixerRules(t *testing.T) {
	eachAdapterCopy(t, func(t *testing.T, name, body string) {
		if name != "ms-bead-fix" {
			return
		}
		requireAdapterText(t, body, "mindspec rules --scope fixer", "complete stdout as `fixer_rules`", "mindspec panel checkout <panel-slug> --fixer", "target's own worktree", "Never dispatch the fixer in a slot tree")
		block := regexp.MustCompile("(?s)```text\\n(.*?)\\n```").FindStringSubmatch(body)
		if len(block) != 2 {
			t.Fatal("missing fixer prompt template")
		}
		requireAdapterText(t, block[1], "Fixer rules (verbatim stdout):\n<fixer_rules>", "Scratch directory: <absolute fixer directory>", "Target worktree: <absolute target worktree>")
		if strings.Contains(block[1], "mindspec rules --scope fixer") {
			t.Error("prompt contains command in place of rule output")
		}
	})
}
