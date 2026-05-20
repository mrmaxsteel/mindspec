package validate

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// ClassifiedChanges groups a diff's changed files by category so doc-sync
// lanes can reason about source, doc, and the raw full list together.
type ClassifiedChanges struct {
	All    []string
	Source []string
	Docs   []string
}

// ValidateDocs checks for doc-sync compliance by comparing changed source files
// against documentation updates in the same diff.
func ValidateDocs(root, diffRef string, exec executor.Executor) *Result {
	r := &Result{SubCommand: "docs"}

	if diffRef == "" {
		diffRef = "HEAD~1"
	}

	changed, err := getChangedFiles(exec, diffRef)
	if err != nil {
		r.AddError("git-diff", fmt.Sprintf("cannot get changed files: %v", err))
		return r
	}

	if len(changed) == 0 {
		return r // no changes, all good
	}

	sourceChanges, docChanges := classifyChanges(changed)
	changes := ClassifiedChanges{All: changed, Source: sourceChanges, Docs: docChanges}

	// Spec-artifact sync runs BEFORE the source-empty early-return so a
	// spec.md-only diff (which classifies as docs-only) still gates on
	// having a plan.md / ADR / sibling artifact in the same diff.
	validateSpecArtifactSync(r, changes)

	if len(sourceChanges) == 0 {
		return r // only doc changes, spec-artifact lane already ran
	}

	// Check if any doc files were also changed
	if len(docChanges) == 0 {
		r.AddError("doc-sync", "source files changed but no documentation files updated")
	}

	// Check specific mapping heuristics
	checkInternalPackages(r, sourceChanges, docChanges)
	checkCmdChanges(r, sourceChanges, docChanges)

	return r
}

// getChangedFiles returns the list of changed files between the working tree
// and ref, routing through the Executor boundary instead of shelling out.
func getChangedFiles(exec executor.Executor, ref string) ([]string, error) {
	files, err := exec.ChangedFiles("", ref)
	if err != nil {
		return nil, fmt.Errorf("changed files for %s: %w", ref, err)
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
		strings.HasPrefix(path, "CLAUDE.md") ||
		strings.HasPrefix(path, "AGENTS.md")
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

	// Check if any domain docs files were changed
	hasDomainDocs := false
	for _, f := range docs {
		if strings.HasPrefix(f, "docs/domains/") || strings.HasPrefix(f, ".mindspec/docs/domains/") {
			hasDomainDocs = true
			break
		}
	}

	if !hasDomainDocs && len(pkgs) > 0 {
		names := make([]string, 0, len(pkgs))
		for pkg := range pkgs {
			names = append(names, pkg)
		}
		r.AddError("internal-docs", fmt.Sprintf("internal packages changed (%s) but no domain docs files updated", strings.Join(names, ", ")))
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
		// Existing operator-docs accept set (preserved):
		if f == "CLAUDE.md" || strings.Contains(f, "CONVENTIONS.md") {
			hasRelevantDoc = true
			break
		}
		// Spec-086 additive operator-docs accept set (Requirement 10):
		// any user-facing doc or the core USAGE manual also satisfies the lane.
		if strings.HasPrefix(f, ".mindspec/docs/user/") ||
			f == ".mindspec/docs/core/USAGE.md" {
			hasRelevantDoc = true
			break
		}
	}

	if !hasRelevantDoc {
		r.AddWarning("cmd-docs", "cmd/ changes without operator-docs update (one of CLAUDE.md, CONVENTIONS.md, .mindspec/docs/user/**, .mindspec/docs/core/USAGE.md)")
	}
}

// validateSpecArtifactSync enforces that any modification to a
// .mindspec/docs/specs/<id>/spec.md file is accompanied in the same diff by
// at least one supporting artifact: the sibling plan.md, any other file
// under .mindspec/docs/specs/<id>/, or any ADR file under
// .mindspec/docs/adr/**.md. A spec.md change made in isolation is rejected
// with the "spec-doc-sync" lane error so the doctrine that "a spec change
// is never atomic" is enforced by the gate.
func validateSpecArtifactSync(r *Result, changes ClassifiedChanges) {
	// Collect spec IDs whose spec.md was touched in this diff.
	touched := make(map[string]bool)
	for _, f := range changes.All {
		if id := specMDID(f); id != "" {
			touched[id] = true
		}
	}
	if len(touched) == 0 {
		return
	}

	for id := range touched {
		prefix := ".mindspec/docs/specs/" + id + "/"
		specMD := prefix + "spec.md"
		hasCompanion := false
		for _, f := range changes.All {
			if f == specMD {
				continue
			}
			if strings.HasPrefix(f, prefix) {
				hasCompanion = true
				break
			}
			if strings.HasPrefix(f, ".mindspec/docs/adr/") && strings.HasSuffix(f, ".md") {
				hasCompanion = true
				break
			}
		}
		if !hasCompanion {
			r.AddError("spec-doc-sync", "spec.md change requires plan.md, ADR, or sibling artifact update in same diff")
		}
	}
}

// specMDID returns the spec ID iff path is .mindspec/docs/specs/<id>/spec.md.
// Returns "" otherwise.
func specMDID(path string) string {
	const prefix = ".mindspec/docs/specs/"
	const suffix = "/spec.md"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimSuffix(rest, suffix)
	// Reject nested paths — must be exactly one segment.
	if rest == "" || strings.Contains(rest, "/") {
		return ""
	}
	return rest
}
