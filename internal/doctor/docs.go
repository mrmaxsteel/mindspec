package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// domainFiles are the expected files within each domain directory.
var domainFiles = []string{"overview.md", "architecture.md", "interfaces.md", "runbook.md"}

func checkDocs(r *Report, root string) {
	docsRel := docsRootRel(root)

	requiredDirs := []struct {
		path string // relative to project root
		name string // display name
	}{
		{filepath.Join(docsRel, "domains"), filepath.ToSlash(filepath.Join(docsRel, "domains")) + "/"},
		{filepath.Join(docsRel, "specs"), filepath.ToSlash(filepath.Join(docsRel, "specs")) + "/"},
	}

	// Check required directories
	for _, d := range requiredDirs {
		p := filepath.Join(root, d.path)
		if dirExists(p) {
			r.Checks = append(r.Checks, Check{Name: d.name, Status: OK})
		} else {
			r.Checks = append(r.Checks, Check{
				Name:    d.name,
				Status:  Missing,
				Message: fmt.Sprintf("create %s directory", d.path),
			})
		}
	}

	// Domain subdirectory checks
	checkDomains(r, root, docsRel)

	// Detect stale focus/lifecycle files (ADR-0023)
	checkStaleFocusLifecycle(r, root)

	// Migration metadata checks (only when migration artifacts are present).
	checkMigrationMetadata(r, root)
}

func checkDomains(r *Report, root, docsRel string) {
	domainsDir := filepath.Join(root, docsRel, "domains")
	if !dirExists(domainsDir) {
		return // already reported as missing in requiredDirs
	}

	// Discover domains from disk rather than a hardcoded list.
	entries, err := os.ReadDir(domainsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		domain := entry.Name()
		domainDir := filepath.Join(domainsDir, domain)

		for _, f := range domainFiles {
			fp := filepath.Join(domainDir, f)
			name := filepath.ToSlash(filepath.Join(docsRel, "domains", domain, f))
			if fileExists(fp) {
				r.Checks = append(r.Checks, Check{Name: name, Status: OK})
			} else {
				r.Checks = append(r.Checks, Check{
					Name:    name,
					Status:  Warn,
					Message: "file not found",
				})
			}
		}
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func docsRootRel(root string) string {
	rel, err := filepath.Rel(root, workspace.DocsDir(root))
	if err != nil {
		return "docs"
	}
	return filepath.ToSlash(rel)
}

// checkStaleFocusLifecycle detects stale .mindspec/focus and lifecycle.yaml files (ADR-0023).
func checkStaleFocusLifecycle(r *Report, root string) {
	// Check for stale focus file
	focusPath := filepath.Join(root, ".mindspec", "focus")
	if fileExists(focusPath) {
		r.Checks = append(r.Checks, Check{
			Name:    "Stale focus file",
			Status:  Warn,
			Message: "stale .mindspec/focus detected; lifecycle state is now derived from beads (ADR-0023). Safe to delete.",
		})
	}

	// Check for stale lifecycle.yaml files in spec directories
	specsDir := filepath.Join(workspace.DocsDir(root), "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return
	}

	var stale []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		lcPath := filepath.Join(specsDir, e.Name(), "lifecycle.yaml")
		if fileExists(lcPath) {
			stale = append(stale, e.Name())
		}
	}

	if len(stale) > 0 {
		r.Checks = append(r.Checks, Check{
			Name:    "Stale lifecycle.yaml files",
			Status:  Warn,
			Message: fmt.Sprintf("%d specs have stale lifecycle.yaml: %s. Lifecycle state is now derived from beads (ADR-0023).", len(stale), strings.Join(stale, ", ")),
		})
	}
}
