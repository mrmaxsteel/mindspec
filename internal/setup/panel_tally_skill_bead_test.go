package setup

// panel_tally_skill_bead_test.go — spec 127 final review (mindspec-tyi3):
// the ms-panel-tally stale-verdict rule's re-panel invocation must carry
// `--bead <bead-id>`.
//
// `mindspec complete`'s gate selects a bead's panel with panel.ForBead,
// which matches ONLY registrations whose bead_id equals the bead being
// completed. `panel create`'s `--bead` flag is the sole input that sets it.
// So a documented re-panel argv without `--bead` registers `bead_id: null`,
// panel.ForBead selects nothing, and internal/complete's empty-selection
// leg is the documented fail-open — the bead completes UNGATED against the
// exact stale verdicts the stale-verdict rule exists to invalidate. The
// omission shipped in BOTH the canonical plugin skill and the tracked
// installed copy, byte-identically, so neither one could be used to catch
// the other.
//
// This asserts the SHIPPED content (pluginmindspec.SkillFiles(), the same
// surface `mindspec setup <agent>` installs from), the canonical file on
// disk, and the tracked .claude/skills/ copy — and pins the three against
// each other, so a future fix applied to only one side REDs instead of
// silently stranding the others.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

// staleVerdictRepanelLine returns the single line of an ms-panel-tally
// SKILL.md carrying the stale-verdict rule's re-panel invocation.
func staleVerdictRepanelLine(t *testing.T, label, content string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "mindspec panel create") && strings.Contains(line, "--round <N+1>") {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s: expected exactly 1 re-panel invocation line, found %d — this test's own locator is stale, fix it before trusting the assertions below", label, len(found))
	}
	return found[0]
}

func TestMsPanelTallySkill_StaleVerdictRepanelBindsBead(t *testing.T) {
	shipped, ok := pluginmindspec.SkillFiles()["ms-panel-tally"]
	if !ok {
		t.Fatal("ms-panel-tally SKILL.md not found among embedded plugin skills")
	}

	root := repoRoot(t)
	readFile := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		return string(data)
	}
	const (
		canonicalRel = "plugins/mindspec/skills/ms-panel-tally/SKILL.md"
		installedRel = ".claude/skills/ms-panel-tally/SKILL.md"
	)

	surfaces := map[string]string{
		"embedded (mindspec setup installs this)": shipped,
		canonicalRel: readFile(canonicalRel),
		installedRel: readFile(installedRel),
	}

	lines := map[string]string{}
	for label, content := range surfaces {
		line := staleVerdictRepanelLine(t, label, content)
		lines[label] = line
		if !strings.Contains(line, "--bead <bead-id>") {
			t.Errorf("%s: the stale-verdict re-panel invocation omits `--bead <bead-id>` — an agent following it registers a bead_id:null panel that mindspec complete's ForBead selection cannot see, fail-opening the gate:\n  %s", label, line)
		}
	}

	// Parity: the same instruction on every surface. A fix applied to one
	// copy only is exactly how the installed copy came to lag canonical.
	for label, line := range lines {
		if line != lines[canonicalRel] {
			t.Errorf("%s's re-panel invocation line diverges from %s:\n  %s\n  %s", label, canonicalRel, line, lines[canonicalRel])
		}
	}
}
