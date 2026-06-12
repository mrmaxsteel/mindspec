package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/ownership"
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

		// OWNERSHIP.yaml check (spec-086 Bead 4; rewritten by spec 091
		// Bead 4). Warn (not Missing) per spec 086 Requirement 15:
		// existing repos must not start failing `mindspec doctor` on day
		// one when the manifest is absent. Spec 091 Req 13 removed the
		// silent internal/<domain>/** fallback — a missing manifest now
		// claims NOTHING — so the Warn no longer mentions a fallback; it
		// names the remedies instead (Req 21). This Warn is the SOLE
		// coverage for the Source()=="missing" state; existing manifests
		// that resolve to zero files are covered by the separate
		// dead-manifest Warn (Req 17). The check is Fixable: --fix writes
		// the Req 8 empty stub via internal/ownership.RenderStub and
		// surfaces the populate prompt (Req 15).
		ownerPath := filepath.Join(domainDir, "OWNERSHIP.yaml")
		ownerName := filepath.ToSlash(filepath.Join(docsRel, "domains", domain, "OWNERSHIP.yaml"))
		if fileExists(ownerPath) {
			r.Checks = append(r.Checks, Check{Name: ownerName, Status: OK})
		} else {
			r.Checks = append(r.Checks, Check{
				Name:   ownerName,
				Status: Warn,
				Message: "missing OWNERSHIP.yaml; this domain claims no source " +
					"paths until the manifest exists — run 'mindspec doctor " +
					"--fix' to scaffold a default manifest, then 'mindspec " +
					"ownership populate " + domain + "' to populate it",
				FixFunc: makeOwnershipFixFunc(r, len(r.Checks), ownerPath, domain),
			})
		}
	}
}

// makeOwnershipFixFunc returns a FixFunc that writes the Req 8 empty
// stub for a missing OWNERSHIP.yaml and surfaces the populate prompt
// (Req 15). The fixer is idempotent and NEVER overwrites an existing
// manifest — including under --fix --force (the documented carve-out:
// manifest content is operator/agent cognition; spec 091 Req 8). The
// stub is rendered by internal/ownership.RenderStub (the single
// templating helper, Bead 3) — doctor does NOT reimplement it.
//
// idx is the index of this check in r.Checks; the FixFunc rewrites
// r.Checks[idx].Message to carry the populate prompt so cmd/doctor
// prints it after Fix() runs (a FixFunc has no other output channel).
// r.Checks is fully built before Report.Fix runs, so the index is
// stable by the time the closure executes.
func makeOwnershipFixFunc(r *Report, idx int, ownerPath, domain string) func() error {
	return func() error {
		// Idempotent / no-overwrite: never touch an existing manifest.
		if fileExists(ownerPath) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(ownerPath), 0o755); err != nil {
			return err
		}
		stub := ownership.RenderStub("mindspec doctor --fix")
		if err := os.WriteFile(ownerPath, stub, 0o644); err != nil {
			return err
		}
		// Surface the populate prompt for this scaffolded domain (Req 15).
		r.Checks[idx].Message = "scaffolded empty-stub OWNERSHIP.yaml — populate it:\n" +
			ownership.BuildPopulatePrompt(domain)
		return nil
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
