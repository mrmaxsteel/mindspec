package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/githooks"
	"github.com/mrmaxsteel/mindspec/internal/safeio"
	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

const (
	mindspecMarkerBegin = "<!-- BEGIN mindspec:managed -->"
	mindspecMarkerEnd   = "<!-- END mindspec:managed -->"
	// Legacy marker for detecting old-format blocks during upgrades.
	mindspecMarkerLegacy = "<!-- mindspec:managed -->"
)

// Result tracks what the setup operation created, skipped, or found existing.
type Result struct {
	Created      []string
	Skipped      []string
	Refreshed    []string           // mindspec-owned skill files refreshed in place to canonical content (Req 19)
	Removed      []string           // stale mindspec-owned skill dirs removed (Req 18)
	Notices      []string           // user-modified files left in place (provenance HC-6)
	BeadsRan     bool               // true if bd setup <agent> was run
	BeadsMsg     string             // output/error from bd setup <agent>
	BeadsConfig  *bead.ConfigResult // result of EnsureBeadsConfig (or ScanBeadsConfig in check mode) after chained bd setup
	BeadsScan    bool               // true when BeadsConfig came from a read-only scan (check mode)
	BeadsConfErr error              // non-nil if the beads-config step failed (non-fatal)
}

// FormatSummary returns a human-readable summary.
func (r *Result) FormatSummary() string {
	var sb strings.Builder

	if len(r.Created) > 0 {
		sb.WriteString("Created:\n")
		for _, p := range r.Created {
			sb.WriteString("  + ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if len(r.Refreshed) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Refreshed (canonical content):\n")
		for _, p := range r.Refreshed {
			sb.WriteString("  ~ ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if len(r.Removed) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Removed (stale mindspec skill):\n")
		for _, p := range r.Removed {
			sb.WriteString("  x ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if len(r.Skipped) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Already present:\n")
		for _, p := range r.Skipped {
			sb.WriteString("  - ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if len(r.Notices) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Notices (user-modified, left in place):\n")
		for _, p := range r.Notices {
			sb.WriteString("  ! ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if r.BeadsRan {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Beads: ran 'bd setup claude'\n")
		if r.BeadsMsg != "" {
			sb.WriteString("  ")
			sb.WriteString(r.BeadsMsg)
			sb.WriteString("\n")
		}
	}

	if r.BeadsConfig != nil {
		if summary := r.BeadsConfig.FormatSummary(); summary != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			if r.BeadsScan {
				// Prefix a dry-run-style header so check mode doesn't read like
				// mutation. Trim the helper's own header to avoid duplication.
				sb.WriteString("Beads config (check-mode preview — no writes):\n")
				sb.WriteString(trimFirstLine(summary))
			} else {
				sb.WriteString(summary)
			}
		}
	}
	if r.BeadsConfErr != nil {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "Beads config: %v\n", r.BeadsConfErr)
	}

	return sb.String()
}

// trimFirstLine drops the first line (including its trailing newline). Used
// when splicing a setup-specific header in front of the shared ConfigResult
// FormatSummary output.
func trimFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// RunClaude sets up Claude Code integration at root.
// If check is true, reports what would be created without writing.
func RunClaude(root string, check bool) (*Result, error) {
	r := &Result{}

	// 1. settings.json (merge hooks)
	if err := ensureSettings(root, check, r); err != nil {
		return nil, err
	}

	// 2. Skills (.claude/skills/<name>/SKILL.md) — create new, refresh
	// previously-shipped (provenance-gated), skip user-modified with a
	// notice (Reqs 18-19, HC-6).
	if err := installSkills(filepath.Join(root, ".claude", "skills"), filepath.Join(".claude", "skills"), claudeSkillFiles(), check, r); err != nil {
		return nil, err
	}

	// 3. CLAUDE.md (append with marker)
	if err := ensureClaudeMD(root, check, r); err != nil {
		return nil, err
	}

	// 4. Install/upgrade git hooks (pre-commit, post-checkout)
	if !check {
		if err := githooks.InstallAll(root); err != nil {
			return nil, fmt.Errorf("installing git hooks: %w", err)
		}
	}

	// 5. Optionally chain bd setup claude
	if !check {
		chainBeadsSetup(root, "claude", r)
	}

	// 6. Surface .beads/config.yaml drift. Runs after chainBeadsSetup so
	// projects that hadn't run `bd init` yet get the config created here too
	// (bd setup scaffolds .beads/ by this point). Check mode scans read-only
	// so --check still reports pending drift without writing.
	applyBeadsConfig(root, check, r)

	return r, nil
}

// applyBeadsConfig runs the config step on r. Shared by RunClaude, RunCodex,
// and RunCopilot so the four entry points can't drift. Non-fatal: errors are
// recorded on Result but never returned.
func applyBeadsConfig(root string, check bool, r *Result) {
	if !bead.HasBeadsDir(root) {
		return
	}
	var cr *bead.ConfigResult
	var err error
	if check {
		cr, err = bead.ScanBeadsConfig(root)
		r.BeadsScan = true
	} else {
		cr, err = bead.EnsureBeadsConfig(root, false)
	}
	if err != nil {
		r.BeadsConfErr = err
		return
	}
	r.BeadsConfig = cr
}

// ensureSettings creates or merges .claude/settings.json with MindSpec hooks.
func ensureSettings(root string, check bool, r *Result) error {
	return ensureSettingsWith(root, check, r, wantedHooks())
}

// ensureSettingsWith is ensureSettings with an explicit wanted set, so tests
// can exercise the merge machinery against hook shapes that wantedHooks()
// does not carry yet (e.g. the PreToolUse pre-complete gate entry).
func ensureSettingsWith(root string, check bool, r *Result, wanted map[string][]map[string]any) error {
	relPath := filepath.Join(".claude", "settings.json")
	absPath := filepath.Join(root, relPath)

	if fileExists(absPath) {
		// Read existing, check if hooks already present
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", relPath, err)
		}

		var settings map[string]any
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", relPath, err)
		}

		hooks, _ := settings["hooks"].(map[string]any)
		if hooks == nil {
			hooks = make(map[string]any)
		}

		anyChanged := false
		for event, entries := range wanted {
			existing, _ := hooks[event].([]any)
			for _, entry := range entries {
				var changed bool
				existing, changed = mergeWantedEntry(existing, entry)
				if changed {
					anyChanged = true
				}
			}
			hooks[event] = existing
		}

		// Remove stale mindspec-owned entries: hooks mindspec installed in
		// the past but no longer wants (e.g. the spec-072 retired PreToolUse
		// guard hooks, including their legacy `mindspec instruct` form).
		// Staleness = mindspec-owned AND not in the current wanted set; the
		// keep-list is derived from `wanted` itself, so an entry merged in
		// above can never be stripped by this same pass.
		if removeStaleMindspecEntries(hooks, wanted) {
			anyChanged = true
		}

		if !anyChanged {
			r.Skipped = append(r.Skipped, relPath)
			return nil
		}

		r.Created = append(r.Created, relPath+" (merged hooks)")
		if !check {
			settings["hooks"] = hooks
			out, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling %s: %w", relPath, err)
			}
			if err := safeio.WriteFileNoSymlink(absPath, append(out, '\n'), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	} else {
		r.Created = append(r.Created, relPath)
		if !check {
			settings := map[string]any{
				"hooks": wanted,
			}
			if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", relPath, err)
			}
			out, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling %s: %w", relPath, err)
			}
			if err := safeio.WriteFileNoSymlink(absPath, append(out, '\n'), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	}

	return nil
}

// wantedHooks returns the hook configuration MindSpec needs.
func wantedHooks() map[string][]map[string]any {
	return map[string][]map[string]any{
		"SessionStart": {
			{
				"matcher": "",
				"hooks": []map[string]any{
					{
						"type":          "command",
						"command":       "mindspec hook session-start",
						"statusMessage": "Loading mode guidance...",
					},
				},
			},
		},
		// The PreToolUse `mindspec hook pre-complete` panel-gate entry was
		// RETIRED (Spec 102 R1): the heuristic command-string matcher is gone
		// and the in-binary `mindspec complete` gate (Spec 099) is the single
		// authoritative enforcement point. Omitting it from the wanted set
		// removes it from existing installs via removeStaleMindspecEntries
		// (the spec-072 retired-hook precedent).
	}
}

// Hook-entry ownership markers. An entry is mindspec-owned iff one of its
// hook commands prefix-matches mindspecOwnedCmdPrefix OR contains
// mindspecLegacyCmdMarker. Ownership is decided by command CONTENT, never by
// matcher: for shared matchers like PreToolUse "Bash", user lint/guard hooks
// routinely collide on the matcher string, and matcher-keyed identity would
// silently overwrite them (spec 093 Req 7, HC-6).
//
// The `mindspec instruct` arm is retained deliberately: the spec-072 retired
// guard hooks include that legacy form, and instruct-form entries are never
// in the wanted set, so wanted-set-derived staleness removes them (N1).
const (
	mindspecOwnedCmdPrefix  = "mindspec hook "
	mindspecLegacyCmdMarker = "mindspec instruct"
)

// isMindspecOwned reports whether a hook entry is mindspec-owned, judged by
// the content of its hook commands (see the ownership-marker comment above).
func isMindspecOwned(entry map[string]any) bool {
	for _, cmd := range entryCommands(entry) {
		if strings.HasPrefix(cmd, mindspecOwnedCmdPrefix) || strings.Contains(cmd, mindspecLegacyCmdMarker) {
			return true
		}
	}
	return false
}

// entryCommands extracts the ordered hook command strings from an entry.
// Handles both the JSON-decoded shape ([]any of map[string]any) and the
// wantedHooks() literal shape ([]map[string]any).
func entryCommands(entry map[string]any) []string {
	var cmds []string
	appendCmd := func(hm map[string]any) {
		if cmd, ok := hm["command"].(string); ok {
			cmds = append(cmds, cmd)
		}
	}
	switch hooksList := entry["hooks"].(type) {
	case []any:
		for _, h := range hooksList {
			if hm, ok := h.(map[string]any); ok {
				appendCmd(hm)
			}
		}
	case []map[string]any:
		for _, hm := range hooksList {
			appendCmd(hm)
		}
	}
	return cmds
}

// entryEqualsWanted reports whether an existing entry already carries the
// wanted entry's identity: same matcher and the same ordered hook commands.
func entryEqualsWanted(entry, want map[string]any) bool {
	entryMatcher, _ := entry["matcher"].(string)
	wantMatcher, _ := want["matcher"].(string)
	if entryMatcher != wantMatcher {
		return false
	}
	entryCmds := entryCommands(entry)
	wantCmds := entryCommands(want)
	if len(entryCmds) != len(wantCmds) {
		return false
	}
	for i := range entryCmds {
		if entryCmds[i] != wantCmds[i] {
			return false
		}
	}
	return true
}

// mergeWantedEntry merges one wanted hook entry into the existing entries
// for its event. Only mindspec-owned entries are candidates for in-place
// update; user entries are NEVER replaced — when a user entry shares the
// wanted matcher, mindspec's entry is APPENDED alongside it (HC-6).
// Returns the (possibly grown) slice and whether anything changed.
func mergeWantedEntry(existing []any, want map[string]any) ([]any, bool) {
	wantMatcher, _ := want["matcher"].(string)
	for i, e := range existing {
		m, ok := e.(map[string]any)
		if !ok || !isMindspecOwned(m) {
			continue
		}
		if matcher, _ := m["matcher"].(string); matcher != wantMatcher {
			continue
		}
		// Mindspec-owned entry on the wanted matcher: update in place if
		// its command content drifted; otherwise it is already current.
		if entryEqualsWanted(m, want) {
			return existing, false
		}
		existing[i] = want
		return existing, true
	}
	// No mindspec-owned entry on this matcher — append, leaving any user
	// entries sharing the matcher untouched.
	return append(existing, want), true
}

// removeStaleMindspecEntries removes mindspec-owned hook entries that are
// not in the current wanted set. Staleness = mindspec-owned AND absent from
// `wanted`; the keep-list is DERIVED from the wanted set passed in (the same
// one ensureSettings just merged), never a hardcoded whitelist — so a newly
// wanted entry can never be added and stripped in the same pass, and the
// next new hook cannot re-create that bug. Non-mindspec (user) entries are
// never touched. A duplicate owned copy of a wanted entry beyond the first
// is also removed, keeping re-runs idempotent. Returns true if any entries
// were removed.
func removeStaleMindspecEntries(hooks map[string]any, wanted map[string][]map[string]any) bool {
	removedAny := false
	for event, raw := range hooks {
		entries, ok := raw.([]any)
		if !ok || len(entries) == 0 {
			continue
		}
		wantedForEvent := wanted[event]
		matchedWanted := make([]bool, len(wantedForEvent))
		var kept []any
		for _, e := range entries {
			m, ok := e.(map[string]any)
			if !ok || !isMindspecOwned(m) {
				kept = append(kept, e)
				continue
			}
			keep := false
			for i, w := range wantedForEvent {
				if !matchedWanted[i] && entryEqualsWanted(m, w) {
					matchedWanted[i] = true
					keep = true
					break
				}
			}
			if keep {
				kept = append(kept, e)
			}
		}
		if len(kept) == len(entries) {
			continue
		}
		removedAny = true
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	return removedAny
}

// ensureClaudeMD creates or appends the MindSpec block to CLAUDE.md.
func ensureClaudeMD(root string, check bool, r *Result) error {
	return ensureManagedDoc(root, "CLAUDE.md", claudeMDFull, claudeMDAppendBlock, check, r)
}

// ensureManagedDoc creates or refreshes a MindSpec-managed markdown document at
// root/relPath. It is the single write path shared by ensureClaudeMD (CLAUDE.md),
// ensureAgentsMD (AGENTS.md), and ensureCopilotInstructions
// (.github/copilot-instructions.md): fullContent is written verbatim when the
// file is absent, appendBlock is the managed body used to replace an existing
// BEGIN/END block or to append a fresh one. EVERY write and append routes
// through safeio so a symlink planted at relPath is refused (ErrSymlinkRefused)
// rather than followed. The managed-block-presence check is folded in here,
// so no standalone helper is needed to detect an existing BEGIN/END block.
func ensureManagedDoc(root, relPath, fullContent, appendBlock string, check bool, r *Result) error {
	absPath := filepath.Join(root, relPath)

	if !fileExists(absPath) {
		r.Created = append(r.Created, relPath)
		if !check {
			if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", relPath, err)
			}
			if err := safeio.WriteFileNoSymlink(absPath, []byte(fullContent), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
		return nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", relPath, err)
	}
	content := string(data)

	switch {
	case strings.Contains(content, mindspecMarkerBegin):
		// Has BEGIN/END markers — replace managed block in place.
		updated := replaceManagedBlock(content, appendBlock)
		if updated == content {
			r.Skipped = append(r.Skipped, relPath+" (MindSpec block present)")
			return nil
		}
		r.Created = append(r.Created, relPath+" (updated MindSpec block)")
		if !check {
			if err := safeio.WriteFileNoSymlink(absPath, []byte(updated), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	case strings.Contains(content, mindspecMarkerLegacy):
		r.Skipped = append(r.Skipped, relPath+" (MindSpec block present — legacy marker)")
	default:
		r.Created = append(r.Created, relPath+" (appended MindSpec block)")
		if !check {
			block := "\n" + mindspecMarkerBegin + "\n" + appendBlock + mindspecMarkerEnd + "\n"
			f, err := safeio.OpenAppendNoSymlink(absPath, 0o644)
			if err != nil {
				return fmt.Errorf("opening %s: %w", relPath, err)
			}
			_, writeErr := f.WriteString(block)
			closeErr := f.Close()
			if writeErr != nil {
				return fmt.Errorf("writing to %s: %w", relPath, writeErr)
			}
			if closeErr != nil {
				return fmt.Errorf("closing %s: %w", relPath, closeErr)
			}
		}
	}

	return nil
}

// chainBeadsSetup runs `bd setup <agent>` if beads is installed, sharing one
// body across the claude and codex entry points (they differ only by the agent
// identifier). The bd subprocess inherits the caller's CWD unless we pin it —
// without cmd.Dir, test processes (whose CWD is the package source directory)
// cause bd to scaffold integration files into the repo's source tree.
func chainBeadsSetup(root, agent string, r *Result) {
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return
	}

	cmd := exec.Command(bdPath, "setup", agent)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	r.BeadsRan = true
	if err != nil {
		r.BeadsMsg = fmt.Sprintf("warning: %v", err)
	} else if len(out) > 0 {
		r.BeadsMsg = strings.TrimSpace(string(out))
	}
}

// replaceManagedBlock replaces the content between BEGIN and END markers.
// Returns the original string unchanged if the new content matches.
func replaceManagedBlock(content, newBlock string) string {
	beginIdx := strings.Index(content, mindspecMarkerBegin)
	if beginIdx == -1 {
		return content
	}
	endIdx := strings.Index(content, mindspecMarkerEnd)
	if endIdx == -1 {
		return content
	}
	endIdx += len(mindspecMarkerEnd)
	// Include trailing newline if present
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}
	replacement := mindspecMarkerBegin + "\n" + newBlock + mindspecMarkerEnd + "\n"
	return content[:beginIdx] + replacement + content[endIdx:]
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// skillFiles returns the SKILL.md contents keyed by skill directory name.
// Shared across Codex and Copilot setup (both use .agents/skills/).
//
// The returned map merges two sources:
//   - The 4 lifecycle gate skills inlined below (ms-spec-create,
//     ms-spec-approve, ms-plan-approve, ms-impl-approve).
//   - The 8 plugin skills embedded from plugins/mindspec/skills/ via
//     pluginmindspec.SkillFiles() (ms-bead-* and ms-panel-* and ms-spec-*).
//
// Lifecycle skills always win on key collision (they're the canonical
// authority); the plugin merge is additive.
func skillFiles() map[string]string {
	out := pluginmindspec.SkillFiles()
	for name, content := range lifecycleSkillFiles() {
		out[name] = content
	}
	return out
}

// lifecycleSkillFiles returns the 4 spec-lifecycle gate skills as raw-string
// literals. These are the canonical authority — they win on key collision
// with the plugin-embedded skills in skillFiles(). Each carries the
// `managed-by: mindspec` provenance marker so the in-place refresh
// (refreshManagedSkill) can tell a shipped file from a user-modified one.
func lifecycleSkillFiles() map[string]string {
	return map[string]string{
		"ms-spec-create": `---
name: ms-spec-create
description: Create a new MindSpec specification
managed-by: mindspec
---

# Spec Create

1. Ask the user for a spec ID (check ` + "`.mindspec/specs/`" + ` for next available number)
2. Run ` + "`mindspec spec create <id>`" + ` in the terminal (optionally with ` + "`--title \"...\"`" + `)
3. If it fails, show the error and help the user fix it
4. On success: begin drafting the spec (the init output includes guidance)
5. As soon as the spec is scaffolded, automatically run the ms-spec-grill skill
   to grill the author — this auto-invoke is the DEFAULT and fires unless the
   author explicitly opts out. Do NOT merely reference ms-spec-grill; invoke it.
   **Headless guard**: if this session is headless/non-interactive (no human
   available to answer one-at-a-time — e.g. running under an orchestrator,
   ` + "`ms-spec-autopilot`" + `, or a test harness), do NOT enter the grill
   loop. Instead add ` + "`- [ ] grill deferred: headless session — run /ms-spec-grill interactively before approval.`" + `
   to the spec's Open Questions section and proceed. Interactive sessions
   still grill by default.
`,

		"ms-spec-approve": `---
name: ms-spec-approve
description: Approve a spec and transition to Plan Mode
managed-by: mindspec
---

# Spec Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec spec approve <id>`" + ` in the terminal (validates, closes the spec-approve gate, generates context pack, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: immediately begin planning (the approval is the authorization)
`,

		"ms-plan-approve": `---
name: ms-plan-approve
description: Approve a plan and transition toward Implementation Mode
managed-by: mindspec
---

# Plan Approval

1. Identify the active spec/plan via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec plan approve <id>`" + ` in the terminal (validates, closes the plan-approve gate, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: run ` + "`mindspec next`" + ` to claim the first bead and enter Implementation Mode
`,

		"ms-impl-approve": `---
name: ms-impl-approve
description: Approve implementation and close out the spec lifecycle
managed-by: mindspec
---

# Implementation Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. If not in review mode, run ` + "`mindspec complete`" + ` first to transition
3. Run ` + "`mindspec impl approve <id>`" + ` in the terminal (verifies review mode, transitions to idle, emits guidance)
4. If approval fails, show the error and help the user resolve it
5. On success: run the session close protocol:
   - ` + "`bd sync`" + `
   - ` + "`git add`" + ` all changed files (state, specs, recordings, beads)
   - ` + "`git commit`" + `
   - ` + "`bd sync`" + `
   - ` + "`git push`" + `
`,
	}
}

// claudeSkillFiles returns skill contents for .claude/skills/<name>/SKILL.md.
// Uses the same content as the shared skillFiles() but placed in Claude's native path.
func claudeSkillFiles() map[string]string {
	return skillFiles()
}

// claudeMDManagedBlock is the canonical content placed between BEGIN/END markers.
// Used for both new files and appends, ensuring idempotent updates.
const claudeMDManagedBlock = `
**IMPORTANT**: You MUST read and follow [AGENTS.md](AGENTS.md) as your primary behavioral instructions. AGENTS.md is the canonical source of project conventions, workflow rules, and development guidance shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Skills

### Spec lifecycle gates

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-spec-create`" + ` | Create a new specification (enters Spec Mode) |
| ` + "`/ms-spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms-plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms-impl-approve`" + ` | Approve implementation → Idle |

### Bead lifecycle

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-bead-impl`" + ` | Stage the impl prompt (Phase A) + dispatch the subagent (Phase B) |
| ` + "`/ms-bead-fix`" + ` | Dispatch a fix-up subagent with the consolidated change list |

### Review panel

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-panel-run`" + ` | Step 0 writes the panel dir + BRIEF + ` + "`panel.json`" + `; then launch 6 reviewers and collect verdicts |
| ` + "`/ms-panel-tally`" + ` | Single decision authority: decision matrix, artifact gates, consolidation, halt-recovery |

### Orchestrators

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-bead-cycle`" + ` | Single bead end-to-end: pick+claim → impl → panel → fix → re-panel → merge |
| ` + "`/ms-spec-autopilot`" + ` | Whole spec: cycle every bead until the spec is done |
| ` + "`/ms-spec-final-review`" + ` | Final panel of the whole spec branch vs main, before ` + "`/ms-impl-approve`" + ` |

## Bead-loop guardrails (mindspec)

See **AGENTS.md § Bead-loop guardrails (mindspec)** for the canonical orchestrator rules and subagent prompt fences (only the cycle runs ` + "`mindspec complete`" + `, after the panel gate passes; never raw ` + "`git merge bead/<id>`" + `; one ` + "`git push`" + ` at end-of-spec; subagents make exactly one commit, tests must PASS). Surviving skills reference that section rather than re-stating it.
`

// claudeMDFull is written when CLAUDE.md doesn't exist.
var claudeMDFull = "# CLAUDE.md — MindSpec\n" + mindspecMarkerBegin + "\n" + claudeMDManagedBlock + mindspecMarkerEnd + "\n"

// claudeMDAppendBlock is the same managed content, used when appending to existing files.
var claudeMDAppendBlock = claudeMDManagedBlock
