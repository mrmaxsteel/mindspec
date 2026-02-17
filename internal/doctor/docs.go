package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/glossary"
	"github.com/mindspec/mindspec/internal/workspace"
)

// expectedDomains and their expected files.
var expectedDomains = []string{"core", "context-system", "workflow"}
var domainFiles = []string{"overview.md", "architecture.md", "interfaces.md", "runbook.md"}

func checkDocs(r *Report, root string) {
	docsRel := docsRootRel(root)

	requiredDirs := []struct {
		path string // relative to project root
		name string // display name
	}{
		{filepath.Join(docsRel, "core"), filepath.ToSlash(filepath.Join(docsRel, "core")) + "/"},
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

	// GLOSSARY.md
	checkGlossary(r, root)

	// context-map.md
	cmPath := workspace.ContextMapPath(root)
	cmName := filepath.ToSlash(filepath.Join(docsRel, "context-map.md"))
	if fileExists(cmPath) {
		r.Checks = append(r.Checks, Check{Name: cmName, Status: OK})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    cmName,
			Status:  Missing,
			Message: fmt.Sprintf("create %s", cmName),
		})
	}

	// Domain subdirectory checks
	checkDomains(r, root, docsRel)

	// Migration metadata checks (only when brownfield artifacts are present).
	checkMigrationMetadata(r, root)
}

func checkGlossary(r *Report, root string) {
	glossaryName := relSlash(root, workspace.GlossaryPath(root))
	entries, err := glossary.Parse(root)
	if err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    glossaryName,
			Status:  Missing,
			Message: fmt.Sprintf("create %s", glossaryName),
		})
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:    glossaryName,
		Status:  OK,
		Message: fmt.Sprintf("(%d terms)", len(entries)),
	})

	// Check for broken links using parsed entries
	var broken []string
	for _, e := range entries {
		if e.FilePath == "" {
			continue
		}
		fullPath := filepath.Join(root, e.FilePath)
		if !fileExists(fullPath) {
			broken = append(broken, fmt.Sprintf("%s -> %s", e.Term, e.Target))
		}
	}

	if len(broken) > 0 {
		r.Checks = append(r.Checks, Check{
			Name:    "Glossary links",
			Status:  Error,
			Message: fmt.Sprintf("%d broken: %s", len(broken), strings.Join(broken, "; ")),
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "Glossary links",
			Status:  OK,
			Message: "all links verified",
		})
	}
}

func checkDomains(r *Report, root, docsRel string) {
	domainsDir := filepath.Join(root, docsRel, "domains")
	if !dirExists(domainsDir) {
		return // already reported as missing in requiredDirs
	}

	for _, domain := range expectedDomains {
		domainDir := filepath.Join(domainsDir, domain)
		if !dirExists(domainDir) {
			r.Checks = append(r.Checks, Check{
				Name:    filepath.ToSlash(filepath.Join(docsRel, "domains", domain)) + "/",
				Status:  Warn,
				Message: "domain directory not found",
			})
			continue
		}

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

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
