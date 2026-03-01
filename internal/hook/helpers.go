package hook

import (
	"os"
	"path/filepath"
	"strings"
)

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// hasPathPrefix checks if path starts with prefix, handling trailing slashes.
func hasPathPrefix(path, prefix string) bool {
	if prefix == "" {
		return false
	}
	// Normalize: ensure prefix ends with /
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix)
}

// stripEnvPrefixes removes leading VAR=val prefixes from a command string.
func stripEnvPrefixes(cmd string) string {
	stripped := cmd
	for {
		// Match pattern: UPPERCASE_CHARS=VALUE SPACE
		idx := strings.Index(stripped, " ")
		if idx < 0 {
			break
		}
		prefix := stripped[:idx]
		eqIdx := strings.Index(prefix, "=")
		if eqIdx < 0 {
			break
		}
		varName := prefix[:eqIdx]
		if !isEnvVarName(varName) {
			break
		}
		stripped = stripped[idx+1:]
	}
	return stripped
}

func isEnvVarName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// isAllowedCommand checks if a command starts with an allowed prefix.
var allowedPrefixes = []string{
	"cd ",
	"mindspec ",
	"./bin/mindspec ",
	"bd ",
	"make ",
	"git ",
	"go ",
}

func isAllowedCommand(cmd string) bool {
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	// Handle absolute paths: extract the binary name from the first token.
	// e.g. "/usr/local/bin/mindspec state" → "mindspec state" matches "mindspec ".
	if idx := strings.Index(cmd, " "); idx > 0 {
		binary := filepath.Base(cmd[:idx])
		rest := cmd[idx:]
		for _, prefix := range allowedPrefixes {
			trimmed := strings.TrimSpace(strings.TrimSuffix(prefix, " "))
			if trimmed == "" {
				continue
			}
			// Match "mindspec" from prefix "mindspec " against base of absolute path
			if binary == trimmed || binary == filepath.Base(trimmed) {
				_ = rest // binary matches; allow the command
				return true
			}
		}
	}
	return false
}

// containsWord checks if haystack contains needle as a substring.
func containsWord(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// getCwd returns the current working directory.
var getCwd = os.Getwd

// outsideActiveWorktree returns true if CWD is outside the active worktree,
// meaning the agent is doing unrelated work and spec/plan mode restrictions
// should not apply. Also returns true if CWD is within any .worktrees/
// directory (even with no ActiveWorktree set), since that means the agent
// is working in a scoped worktree context.
// Returns false if there's no active worktree and CWD is not in a worktree.
func outsideActiveWorktree(st *HookState) bool {
	cwd, err := getCwd()
	if err != nil || cwd == "" {
		return false
	}
	// Check nested worktree first: CWD is in a worktree spawned FROM the
	// active worktree (e.g. bead worktree inside spec worktree). That's
	// scoped implementation work, not spec/plan editing.
	if st != nil && st.ActiveWorktree != "" {
		nestedWt := st.ActiveWorktree + "/.worktrees/"
		if strings.HasPrefix(cwd, nestedWt) {
			return true
		}
	}
	// If CWD is inside any .worktrees/ directory, the agent is in a
	// scoped worktree — not subject to the global mode restriction.
	if strings.Contains(cwd, "/.worktrees/") {
		// But if CWD is inside the *active* worktree (and not nested),
		// that's the spec's own worktree — restrictions should apply.
		if st != nil && st.ActiveWorktree != "" && hasPathPrefix(cwd, st.ActiveWorktree) {
			return false
		}
		return true
	}
	// No ActiveWorktree set and CWD not in a worktree — conservative.
	if st == nil || st.ActiveWorktree == "" {
		return false
	}
	return !hasPathPrefix(cwd, st.ActiveWorktree)
}

// isCodeFile returns true if the path looks like a code file (not documentation).
func isCodeFile(path string) bool {
	if path == "" {
		return false
	}

	// Documentation paths — always allowed
	docPrefixes := []string{
		".mindspec/docs/",
		"docs/",
		".mindspec/",
		".claude/",
		".github/",
	}
	for _, prefix := range docPrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
		// Handle absolute paths: check if the path contains /prefix as a segment.
		// e.g. "/Users/x/project/.mindspec/focus" contains "/.mindspec/"
		if strings.Contains(path, "/"+prefix) {
			return false
		}
	}

	// Documentation file extensions/names — always allowed
	docFiles := []string{
		"GLOSSARY.md",
		"AGENTS.md",
		"CLAUDE.md",
		"README.md",
		"CHANGELOG.md",
		"LICENSE",
	}
	base := filepath.Base(path)
	for _, name := range docFiles {
		if base == name {
			return false
		}
	}

	// Markdown files are generally docs
	if strings.HasSuffix(path, ".md") {
		return false
	}

	// Everything else is considered code
	return true
}
