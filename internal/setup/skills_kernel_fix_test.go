package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/mrmaxsteel/mindspec/internal/state"
	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

func s134SkillCopies(t *testing.T, name string) map[string]string {
	t.Helper()
	tracked, err := os.ReadFile(filepath.Join(repoRoot(t), ".claude", "skills", name, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"plugin": pluginmindspec.SkillFiles()[name],
		"claude": string(tracked),
	}
}

// s134FreshAgentProblems checks that the cycle adapter starts a fresh agent
// for each step and defers to the orchestrator rules of instruct, and that
// instruct renders dispatch.fresh-agent, which carries the lease rule that
// the old skill text held.
func s134FreshAgentProblems(adapter, instructOut string) []string {
	var problems []string
	for _, want := range []string{"Start a fresh agent with that output as its prompt.", "Follow the orchestrator rules that `mindspec instruct` prints."} {
		if !strings.Contains(adapter, want) {
			problems = append(problems, "adapter lacks: "+want)
		}
	}
	_, section, _ := strings.Cut(instructOut, "## Rules\n\n")
	var row string
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "- dispatch.fresh-agent: ") {
			row = line
		}
	}
	for _, want := range []string{"Do not reuse an agent for a second actor step", "take a lease that a still-running agent of this loop holds", "Start a fresh agent for each actor step"} {
		if !strings.Contains(row, want) {
			problems = append(problems, "instruct dispatch.fresh-agent lacks: "+want)
		}
	}
	return problems
}

func TestS134Skills_CycleLeaseRecovery(t *testing.T) {
	out, err := instruct.Render(&instruct.Context{Mode: state.ModeImplement})
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range s134SkillCopies(t, "ms-bead-cycle") {
		t.Run(name, func(t *testing.T) {
			if problems := s134FreshAgentProblems(body, out); len(problems) != 0 {
				t.Fatal(problems)
			}
			for mutation, pair := range map[string][2]string{
				"adapter reuses agent": {strings.Replace(body, "Start a fresh agent", "Reuse the agent", 1), out},
				"instruct row removed": {body, strings.Replace(out, "- dispatch.fresh-agent: ", "- dispatch.removed: ", 1)},
			} {
				if len(s134FreshAgentProblems(pair[0], pair[1])) == 0 {
					t.Errorf("mutation %s passed the fresh-agent check", mutation)
				}
			}
		})
	}
}

func TestS134Skills_ReadmeAutonomyNavigation(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Governed autonomy\n", "[Governed autonomy](#governed-autonomy)", "[Autonomy](#governed-autonomy)"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing autonomy heading or navigation: %s", want)
		}
	}
	if strings.Contains(strings.ToLower(string(body)), "autonomy ladder") || strings.Contains(string(body), "#the-autonomy-ladder") {
		t.Error("obsolete autonomy ladder heading or anchor")
	}
}
