package hook

import (
	"os"
	"path/filepath"
	"strings"
)

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
	return false
}

// containsWord checks if haystack contains needle as a substring.
func containsWord(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// getCwd returns the current working directory.
var getCwd = os.Getwd

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
