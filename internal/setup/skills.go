package setup

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// historicalSkillsFS holds byte-exact snapshots of SKILL.md contents MindSpec
// shipped in prior versions. Provenance refresh (Req 19) and stale-skill
// cleanup (Req 18) byte-match an on-disk file against these snapshots to tell
// a MindSpec-shipped file from a user-authored one (HC-6): only files that
// match a shipped snapshot are refreshed or removed; anything else is left in
// place with a notice.
//
//go:embed historical_skills/*.md
var historicalSkillsFS embed.FS

// previouslyShippedSkills returns, for each skill name, the set of byte-exact
// SKILL.md contents MindSpec has shipped over time. The CURRENT canonical
// content is added by the caller (installSkills); this function returns only
// the historical (pre-093 and pre-marker) snapshots:
//
//   - Every embedded historical_skills/<name>.md (the pre-093 plugin skills,
//     including the four now removed, and the removed ms-spec-status).
//   - The marker-less variant of each lifecycle skill: pre-093 lifecycle
//     skills shipped without the `managed-by: mindspec` frontmatter line, so
//     stripping that line from the current canonical content reconstructs the
//     exact prior bytes — letting existing installs that carry the deprecated
//     noun-verb wording be refreshed in place.
func previouslyShippedSkills() map[string][]string {
	out := make(map[string][]string)

	entries, _ := historicalSkillsFS.ReadDir("historical_skills")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		data, err := historicalSkillsFS.ReadFile("historical_skills/" + e.Name())
		if err != nil {
			continue
		}
		out[name] = append(out[name], string(data))
	}

	// Marker-less prior variant of each lifecycle skill (Req 19b).
	for name, content := range lifecycleSkillFiles() {
		if prior := stripManagedByMarker(content); prior != content {
			out[name] = append(out[name], prior)
		}
	}

	return out
}

// stripManagedByMarker removes the single `managed-by: mindspec` frontmatter
// line (and its trailing newline) from a SKILL.md body, reconstructing the
// pre-marker shipped bytes.
func stripManagedByMarker(content string) string {
	return strings.Replace(content, "managed-by: mindspec\n", "", 1)
}

// removedSkills lists skill names MindSpec previously shipped but no longer
// wants (Req 16/18: ms-spec-status deleted; ms-bead-next/-merge/-prep and
// ms-panel-create folded into surviving skills). installSkills removes their
// on-disk dirs iff the SKILL.md byte-matches a shipped snapshot.
var removedSkills = []string{
	"ms-spec-status",
	"ms-bead-next",
	"ms-bead-merge",
	"ms-bead-prep",
	"ms-panel-create",
}

// installSkills writes the wanted skill set under skillsDir, then cleans up
// removed skills. skillsRel is the repo-relative form of skillsDir, used for
// Result reporting.
//
// Per-skill disposition:
//   - absent → create.
//   - present and byte-identical to the canonical content → skip (current).
//   - present and byte-matches a previously-shipped snapshot → refresh in
//     place to canonical (Req 19b).
//   - present and matches nothing shipped → user-modified: leave it, notice
//     (HC-6).
//
// In check mode nothing is written; the Result still records what WOULD
// happen.
func installSkills(skillsDir, skillsRel string, wanted map[string]string, check bool, r *Result) error {
	shipped := previouslyShippedSkills()

	for name, content := range wanted {
		relPath := filepath.Join(skillsRel, name, "SKILL.md")
		absPath := filepath.Join(skillsDir, name, "SKILL.md")

		if !fileExists(absPath) {
			r.Created = append(r.Created, relPath)
			if !check {
				if err := writeSkill(absPath, content); err != nil {
					return err
				}
			}
			continue
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", relPath, err)
		}
		current := string(data)

		switch {
		case current == content:
			// Already canonical.
			r.Skipped = append(r.Skipped, relPath)
		case matchesShipped(current, content, shipped[name]):
			// Previously-shipped content → refresh in place (Req 19b).
			r.Refreshed = append(r.Refreshed, relPath)
			if !check {
				if err := writeSkill(absPath, content); err != nil {
					return err
				}
			}
		default:
			// User-modified: leave it, notice (HC-6).
			r.Notices = append(r.Notices, relPath+" (user-modified — left in place; delete it to receive the canonical version)")
		}
	}

	cleanupRemovedSkills(skillsDir, skillsRel, check, r)
	return nil
}

// matchesShipped reports whether current matches any content MindSpec has
// shipped for this skill — the canonical content or any historical snapshot.
func matchesShipped(current, canonical string, history []string) bool {
	if current == canonical {
		return true
	}
	for _, h := range history {
		if current == h {
			return true
		}
	}
	return false
}

// cleanupRemovedSkills removes the on-disk dirs of skills MindSpec no longer
// ships, but only when the SKILL.md byte-matches a shipped snapshot
// (provenance discipline, mirroring githooks.CleanStaleGitHooks). A
// user-modified file under a removed skill dir is left in place with a notice
// (HC-6).
func cleanupRemovedSkills(skillsDir, skillsRel string, check bool, r *Result) {
	shipped := previouslyShippedSkills()
	for _, name := range removedSkills {
		dir := filepath.Join(skillsDir, name)
		skillPath := filepath.Join(dir, "SKILL.md")
		relPath := filepath.Join(skillsRel, name)
		if !fileExists(skillPath) {
			continue
		}
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		if matchesShipped(string(data), "", shipped[name]) {
			r.Removed = append(r.Removed, relPath)
			if !check {
				_ = os.RemoveAll(dir)
			}
		} else {
			r.Notices = append(r.Notices, relPath+" (retired skill, user-modified — left in place; remove it manually if unused)")
		}
	}
}

// writeSkill writes a single SKILL.md, creating its parent dir.
func writeSkill(absPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("creating dir for %s: %w", absPath, err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", absPath, err)
	}
	return nil
}
