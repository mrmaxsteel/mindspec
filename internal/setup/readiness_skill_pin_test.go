package setup

// readiness_skill_pin_test.go: the readiness contract is binary output.
// `mindspec next <w> --emit-only` prints the readiness review (SR-1 to
// SR-5, the NOT READY first line, the anti-browbeat and independent
// re-derivation rules, each stored reason with its clarification and the
// complete --check line); cmd/mindspec/next_selection_test.go pins it. The
// ms-bead-impl adapter keeps only the staged-rule marker of its old
// sequence, and an old installed copy still refreshes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

func readinessSkillFile(t *testing.T, name string) string {
	t.Helper()
	files := pluginmindspec.SkillFiles()
	content, ok := files[name]
	if !ok {
		t.Fatalf("%s SKILL.md not found among embedded plugin skills", name)
	}
	return content
}

const readinessStagedRule = "<!-- staged-rule: id=sequence.bead-impl-prompt; remove-by=2027-06-30 -->"

// readinessMarkerOnlyProblems returns why content does not carry the
// staged marker alone: the marker must occur once, and no other line may
// name the rule id or restate the readiness sequence.
func readinessMarkerOnlyProblems(content string) []string {
	var problems []string
	if strings.Count(content, readinessStagedRule) != 1 {
		problems = append(problems, "the staged-rule marker must occur exactly once")
	}
	if strings.Count(content, "sequence.bead-impl-prompt") != 1 {
		problems = append(problems, "only the marker may name sequence.bead-impl-prompt")
	}
	for _, removed := range []string{"bead ready-check", "Phase 0", "SR-1", "NOT READY", "mindspec_readiness_override", "Anti-browbeat", "Clarification-handling rule"} {
		if strings.Contains(content, removed) {
			problems = append(problems, "the adapter restates binary output: "+removed)
		}
	}
	return problems
}

func TestMsBeadImplSkill_StagedMarkerOnly(t *testing.T) {
	plugin := readinessSkillFile(t, "ms-bead-impl")
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".claude", "skills", "ms-bead-impl", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"plugin": plugin, "claude": string(raw)} {
		if problems := readinessMarkerOnlyProblems(content); len(problems) != 0 {
			t.Errorf("%s copy: %v", name, problems)
		}
	}
	for name, mutated := range map[string]string{
		"no marker":    strings.Replace(plugin, readinessStagedRule, "", 1),
		"apply line":   plugin + "\nApply `sequence.bead-impl-prompt` before each dispatch.\n",
		"exit line":    plugin + "\nRun `mindspec bead ready-check <id>`; proceed on exit 0; otherwise stop and surface its report.\n",
		"phase 0 text": plugin + "\nPhase 0: review SR-1 to SR-5 and return NOT READY.\n",
	} {
		if len(readinessMarkerOnlyProblems(mutated)) == 0 {
			t.Errorf("mutation %s should turn the marker-only pin red", name)
		}
	}
}

func TestMsBeadImplSkill_OverrideCoverageRefresh(t *testing.T) {
	for _, name := range []string{"ms-bead-impl", "ms-bead-cycle"} {
		t.Run(name, func(t *testing.T) {
			prior, err := historicalSkillsFS.ReadFile("historical_skills/" + name + ".pre129.md")
			if err != nil {
				t.Fatal(err)
			}
			if name == "ms-bead-impl" && !strings.Contains(string(prior), "ONLY if EVERY currently-failing signal is present in that recorded set") {
				t.Fatal("snapshot lacks replaced comparison")
			}
			if name == "ms-bead-cycle" && !strings.Contains(string(prior), "`mindspec next --allow-not-ready`") {
				t.Fatal("snapshot lacks replaced bare flag")
			}
			dir := t.TempDir()
			if err := writeSkill(filepath.Join(dir, name, "SKILL.md"), string(prior)); err != nil {
				t.Fatal(err)
			}
			wanted := readinessSkillFile(t, name)
			var result Result
			if err := installSkills(dir, "skills", map[string]string{name: wanted}, false, &result); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, name, "SKILL.md"))
			if err != nil || string(got) != wanted || len(result.Refreshed) != 1 {
				t.Fatalf("refresh result: %+v, err=%v", result, err)
			}
		})
	}
}
