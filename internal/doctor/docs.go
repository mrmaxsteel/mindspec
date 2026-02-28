package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
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

	// Spec lifecycle checks (ADR-0020)
	checkSpecLifecycles(r, root)

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

// checkSpecLifecycles warns on active spec directories that lack a lifecycle.yaml.
func checkSpecLifecycles(r *Report, root string) {
	specsDir := filepath.Join(workspace.DocsDir(root), "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return // specs dir missing is already reported
	}

	var missing []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specDir := filepath.Join(specsDir, e.Name())
		if !fileExists(filepath.Join(specDir, "spec.md")) {
			continue
		}
		lc, err := state.ReadLifecycle(specDir)
		if err != nil || lc == nil {
			missing = append(missing, e.Name())
		}
	}

	if len(missing) == 0 {
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:    "Spec lifecycle files",
		Status:  Warn,
		Message: fmt.Sprintf("%d specs missing lifecycle.yaml: %s", len(missing), strings.Join(missing, ", ")),
	})
}
