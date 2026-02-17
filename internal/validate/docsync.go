package validate

import (
	"fmt"
	"os/exec"
	"strings"
)

// ValidateDocs checks for doc-sync compliance by comparing changed source files
// against documentation updates in the same diff.
func ValidateDocs(root, diffRef string) *Result {
	r := &Result{SubCommand: "docs"}

	if diffRef == "" {
		diffRef = "HEAD~1"
	}

	changed, err := getChangedFiles(diffRef)
	if err != nil {
		r.AddError("git-diff", fmt.Sprintf("cannot get changed files: %v", err))
		return r
	}

	if len(changed) == 0 {
		return r // no changes, all good
	}

	sourceChanges, docChanges := classifyChanges(changed)

	if len(sourceChanges) == 0 {
		return r // only doc changes, all good
	}

	// Check if any doc files were also changed
	if len(docChanges) == 0 {
		r.AddWarning("doc-sync", "source files changed but no documentation files updated")
	}

	// Check specific mapping heuristics
	checkInternalPackages(r, sourceChanges, docChanges)
	checkCmdChanges(r, sourceChanges, docChanges)

	return r
}

// getChangedFiles runs git diff --name-only and returns the list of changed files.
func getChangedFiles(ref string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", ref).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s: %w", ref, err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// ParseChangedFiles parses a newline-separated list of file paths.
// Exported for testing without shelling out to git.
func ParseChangedFiles(output string) []string {
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// classifyChanges splits files into source and doc categories.
func classifyChanges(files []string) (source, docs []string) {
	for _, f := range files {
		if isDocFile(f) {
			docs = append(docs, f)
		} else if isSourceFile(f) {
			source = append(source, f)
		}
	}
	return
}

// isDocFile returns true for documentation files.
func isDocFile(path string) bool {
	return strings.HasPrefix(path, "docs/") ||
		strings.HasPrefix(path, ".mindspec/docs/") ||
		path == ".mindspec/policies.yml" ||
		strings.HasPrefix(path, "CLAUDE.md") ||
		strings.HasPrefix(path, "AGENTS.md") ||
		strings.HasPrefix(path, "GLOSSARY.md") ||
		strings.HasPrefix(path, "architecture/")
}

// isSourceFile returns true for Go source files.
func isSourceFile(path string) bool {
	return (strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/")) &&
		strings.HasSuffix(path, ".go") &&
		!strings.HasSuffix(path, "_test.go")
}

// checkInternalPackages warns if internal/ packages changed without domain doc updates.
func checkInternalPackages(r *Result, source, docs []string) {
	// Collect unique package directories from source changes
	pkgs := make(map[string]bool)
	for _, f := range source {
		if strings.HasPrefix(f, "internal/") {
			parts := strings.SplitN(f, "/", 3)
			if len(parts) >= 2 {
				pkgs[parts[1]] = true
			}
		}
	}

	if len(pkgs) == 0 {
		return
	}

	// Check if any docs/domains/ files were changed
	hasDomainDocs := false
	for _, f := range docs {
		if strings.HasPrefix(f, "docs/domains/") {
			hasDomainDocs = true
			break
		}
	}

	if !hasDomainDocs && len(pkgs) > 0 {
		names := make([]string, 0, len(pkgs))
		for pkg := range pkgs {
			names = append(names, pkg)
		}
		r.AddWarning("internal-docs", fmt.Sprintf("internal packages changed (%s) but no docs/domains/ files updated", strings.Join(names, ", ")))
	}
}

// checkCmdChanges warns if cmd/ files changed without CLAUDE.md or CONVENTIONS.md updates.
func checkCmdChanges(r *Result, source, docs []string) {
	hasCmdChanges := false
	for _, f := range source {
		if strings.HasPrefix(f, "cmd/") {
			hasCmdChanges = true
			break
		}
	}

	if !hasCmdChanges {
		return
	}

	hasRelevantDoc := false
	for _, f := range docs {
		if f == "CLAUDE.md" || strings.Contains(f, "CONVENTIONS.md") {
			hasRelevantDoc = true
			break
		}
	}

	if !hasRelevantDoc {
		r.AddWarning("cmd-docs", "cmd/ files changed but neither CLAUDE.md nor CONVENTIONS.md updated")
	}
}
