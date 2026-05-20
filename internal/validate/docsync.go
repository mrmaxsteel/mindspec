package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// classifiedChanges groups a diff's changed files by category so doc-sync
// lanes can reason about source, doc, and the raw full list together.
// Package-private: no external consumers (spec-086 panel CONSENSUS Minor 8).
type classifiedChanges struct {
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
	changes := classifiedChanges{All: changed, Source: sourceChanges, Docs: docChanges}

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
	checkInternalPackages(r, root, sourceChanges, docChanges)
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

// listDomainDirs returns the lexicographically-sorted list of domain
// directory names under .mindspec/docs/domains/ in the given root.
// Returns an empty slice (no error) when the domains directory is
// missing — callers fall back to the legacy "internal/<domain>/**"
// heuristic via attributeDomain's per-domain loadOwnership fallback.
func listDomainDirs(root string) ([]string, error) {
	dir := filepath.Join(root, ".mindspec", "docs", "domains")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading domains dir %s: %w", dir, err)
	}
	domains := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			domains = append(domains, e.Name())
		}
	}
	sort.Strings(domains)
	return domains, nil
}

// checkInternalPackages errors when internal/ packages changed without
// the corresponding domain docs being updated in the same diff.
// Attribution uses the Bead-1 ownership machinery (loadOwnership +
// attributeDomain): each changed source path is resolved to its
// owning domain via .mindspec/docs/domains/<domain>/OWNERSHIP.yaml,
// or via the "internal/<domain>/**" fallback when the manifest is
// absent. The error message NAMES the manifest file that decided
// ownership (or "<fallback: internal/<domain>/**>") so the operator
// knows which OWNERSHIP.yaml to edit.
func checkInternalPackages(r *Result, root string, source, docs []string) {
	domains, err := listDomainDirs(root)
	if err != nil {
		r.AddError("internal-docs", fmt.Sprintf("cannot enumerate domain dirs: %v", err))
		return
	}

	// Group source files by attributed domain, retaining the
	// manifest/fallback marker that decided ownership.
	type attribution struct {
		manifest string // o.ManifestPath, or "" → use fallback marker
		files    []string
	}
	byDomain := map[string]*attribution{}

	// Legacy fallback when there are no domain manifests at all:
	// preserve the pre-Bead-1 internal/<pkg>/ heuristic so we still
	// surface drift on bare checkouts.
	if len(domains) == 0 {
		pkgs := map[string][]string{}
		for _, f := range source {
			if !strings.HasPrefix(f, "internal/") {
				continue
			}
			parts := strings.SplitN(f, "/", 3)
			if len(parts) < 2 {
				continue
			}
			pkgs[parts[1]] = append(pkgs[parts[1]], f)
		}
		if len(pkgs) == 0 {
			return
		}
		hasDomainDocs := false
		for _, f := range docs {
			if strings.HasPrefix(f, "docs/domains/") || strings.HasPrefix(f, ".mindspec/docs/domains/") {
				hasDomainDocs = true
				break
			}
		}
		if hasDomainDocs {
			return
		}
		names := make([]string, 0, len(pkgs))
		for p := range pkgs {
			names = append(names, p)
		}
		sort.Strings(names)
		for _, p := range names {
			r.AddError("internal-docs", fmt.Sprintf(
				"internal sources in domain %q changed (%s) but no doc updates under %s/; ownership decided by <fallback: internal/%s/**>",
				p, strings.Join(pkgs[p], ", "),
				filepath.Join(".mindspec", "docs", "domains", p),
				p,
			))
		}
		return
	}

	for _, f := range source {
		// Only consider files that could plausibly be owned by a
		// domain. attributeDomain returns "" when nothing matches —
		// in that case the file is silently skipped (it is not the
		// internal-docs lane's job to police unmapped trees).
		domain, o, derr := attributeDomain(root, f, domains)
		if derr != nil {
			r.AddError("internal-docs", fmt.Sprintf("attributing %s: %v", f, derr))
			continue
		}
		if domain == "" {
			continue
		}
		manifest := ""
		if o != nil {
			manifest = o.ManifestPath
		}
		a, ok := byDomain[domain]
		if !ok {
			a = &attribution{manifest: manifest}
			byDomain[domain] = a
		}
		a.files = append(a.files, f)
	}

	if len(byDomain) == 0 {
		return
	}

	// Walk domains in sorted order for deterministic emit.
	domainNames := make([]string, 0, len(byDomain))
	for d := range byDomain {
		domainNames = append(domainNames, d)
	}
	sort.Strings(domainNames)

	for _, domain := range domainNames {
		a := byDomain[domain]
		hasDomainDocs := false
		mindspecPrefix := ".mindspec/docs/domains/" + domain + "/"
		legacyPrefix := "docs/domains/" + domain + "/"
		for _, f := range docs {
			if strings.HasPrefix(f, mindspecPrefix) || strings.HasPrefix(f, legacyPrefix) {
				hasDomainDocs = true
				break
			}
		}
		if hasDomainDocs {
			continue
		}
		marker := a.manifest
		if marker == "" {
			marker = fmt.Sprintf("<fallback: internal/%s/**>", domain)
		}
		r.AddError("internal-docs", fmt.Sprintf(
			"internal sources in domain %q changed (%s) but no doc updates under %s/; ownership decided by %s",
			domain, strings.Join(a.files, ", "),
			filepath.Join(".mindspec", "docs", "domains", domain),
			marker,
		))
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
// .mindspec/docs/specs/<id>/spec.md file is accompanied in the same
// diff by at least one supporting artifact: the sibling plan.md, any
// other file under .mindspec/docs/specs/<id>/, or any ADR file under
// .mindspec/docs/adr/**.md. A spec.md change made in isolation is
// rejected with the "spec-artifact-sync" lane error so the doctrine
// that "a spec change is never atomic" is enforced by the gate.
//
// NOTE on ADR-sibling matching (panel CONSENSUS Minor 9): any
// modification under .mindspec/docs/adr/**.md currently satisfies the
// sibling requirement. This is deliberately loose — spec edits in
// practice routinely add or cite ADRs as the load-bearing artifact,
// and the gate's purpose here is to prevent zero-companion spec.md
// commits, not to police ADR-citation graphs. A stricter "cited ADR"
// check is deferred to spec 087's ADR-divergence lane.
func validateSpecArtifactSync(r *Result, changes classifiedChanges) {
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

	// Sort touched spec IDs for deterministic emit order (panel
	// CONSENSUS Major 6).
	ids := make([]string, 0, len(touched))
	for id := range touched {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
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
			r.AddError("spec-artifact-sync", fmt.Sprintf(
				"spec %s/spec.md change requires plan.md, ADR (.mindspec/docs/adr/**.md), or sibling artifact (.mindspec/docs/specs/%s/**) update in same diff",
				id, id,
			))
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
