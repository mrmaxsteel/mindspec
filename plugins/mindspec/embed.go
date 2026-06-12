// Package pluginmindspec embeds the SKILL.md contents of the plugin's 7
// skills so they can be shipped by `mindspec setup <agent>` alongside the
// core lifecycle skills (defined in internal/setup/claude.go::skillFiles).
//
// Until 2026-06 the plugin lived purely on-disk and was opt-in for projects
// willing to copy plugins/mindspec/skills/ into their .claude/skills/ tree.
// Embedding them here means every `mindspec setup` user gets the full
// autonomous-loop skill set — the lifecycle gates AND the bead/panel/orchestrator
// skills — by default.
package pluginmindspec

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

//go:embed skills/*/SKILL.md
var skillsFS embed.FS

// SkillFiles returns the embedded plugin SKILL.md contents keyed by skill
// directory name (e.g. "ms-bead-cycle"). The map is built fresh on each call;
// callers can mutate the returned map safely.
func SkillFiles() map[string]string {
	out := make(map[string]string)
	_ = fs.WalkDir(skillsFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if path.Base(p) != "SKILL.md" {
			return nil
		}
		// p = "skills/<name>/SKILL.md" — extract <name>.
		rel := strings.TrimPrefix(p, "skills/")
		name := strings.TrimSuffix(rel, "/SKILL.md")
		data, readErr := skillsFS.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		out[name] = string(data)
		return nil
	})
	return out
}

// SkillNames returns the embedded plugin skill names in lexical order.
// Useful for tests and for surfaces (like CLAUDE.md generation) that want a
// deterministic listing.
func SkillNames() []string {
	m := SkillFiles()
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	// embed.FS WalkDir returns entries in lexical order already, but build the
	// slice deterministically regardless of map iteration order.
	sortStrings(names)
	return names
}

// sortStrings is a tiny stand-in for sort.Strings to avoid pulling sort into
// this single-call surface. Insertion sort on N=7 is fine.
func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}
