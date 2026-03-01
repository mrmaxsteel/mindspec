package hook

import (
	"os"
	"os/exec"
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
	_, stripped := parseEnvPrefixes(cmd)
	return stripped
}

// parseEnvPrefixes parses and removes leading VAR=val prefixes from a command.
// It returns both the parsed env map and the remaining command string.
func parseEnvPrefixes(cmd string) (map[string]string, string) {
	env := map[string]string{}
	stripped := strings.TrimSpace(cmd)
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
		env[varName] = prefix[eqIdx+1:]
		stripped = strings.TrimSpace(stripped[idx+1:])
	}
	return env, stripped
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

// getGitBranch resolves the current git branch in workdir (or current dir if
// workdir is empty). Function var for testability.
var getGitBranch = func(workdir string) (string, error) {
	args := []string{}
	if workdir != "" {
		args = append(args, "-C", workdir)
	}
	args = append(args, "rev-parse", "--abbrev-ref", "HEAD")
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// protectedGitWrite returns operation + branch when command is a protected-branch
// git commit/merge that should be blocked at hook layer.
func protectedGitWrite(rawCmd, strippedCmd, cwd string) (operation, branch string, block bool) {
	env, _ := parseEnvPrefixes(rawCmd)
	if env["MINDSPEC_ALLOW_MAIN"] == "1" {
		return "", "", false
	}

	workdir, subcommand, ok := parseGitCommand(strippedCmd)
	if !ok {
		return "", "", false
	}
	if subcommand != "commit" && subcommand != "merge" {
		return "", "", false
	}

	branchWorkdir := workdir
	if branchWorkdir == "" {
		branchWorkdir = cwd
	} else if !filepath.IsAbs(branchWorkdir) && cwd != "" {
		branchWorkdir = filepath.Clean(filepath.Join(cwd, branchWorkdir))
	}

	current, err := getGitBranch(branchWorkdir)
	if err != nil {
		return "", "", false
	}
	if current == "main" || current == "master" {
		return subcommand, current, true
	}
	return "", "", false
}

// parseGitCommand extracts git workdir + subcommand from a command string.
func parseGitCommand(cmd string) (workdir, subcommand string, ok bool) {
	fields := strings.Fields(cmd)
	if len(fields) < 2 || filepath.Base(fields[0]) != "git" {
		return "", "", false
	}

	for i := 1; i < len(fields); i++ {
		tok := fields[i]
		switch tok {
		case "-C":
			if i+1 >= len(fields) {
				return "", "", false
			}
			workdir = fields[i+1]
			i++
			continue
		case "--git-dir", "--work-tree", "-c":
			if i+1 < len(fields) {
				i++
			}
			continue
		}

		if strings.HasPrefix(tok, "-C") && len(tok) > 2 {
			workdir = strings.TrimPrefix(tok, "-C")
			continue
		}
		if strings.HasPrefix(tok, "--git-dir=") ||
			strings.HasPrefix(tok, "--work-tree=") ||
			strings.HasPrefix(tok, "-c") {
			continue
		}
		if strings.HasPrefix(tok, "-") {
			continue
		}
		return workdir, tok, true
	}
	return workdir, "", false
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
