package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/domain"
	"github.com/mrmaxsteel/mindspec/internal/ownership"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// domainFiles are the expected files within each domain directory.
var domainFiles = []string{"overview.md", "architecture.md", "interfaces.md", "runbook.md"}

// docsMappedCheck is the "is this domain mapped in context-map.md"
// predicate the unmapped-domain check consumes. It defaults to the exported
// domain.HasEntry — the SAME helper scaffold.Add's context-map backfill
// consumes (mirrored there as its own scaffoldMappedCheck seam var) — never
// a private reimplementation, so the emission (writer) side and this
// detection side cannot silently disagree about what "mapped" means (spec
// 123 R3/AC-4). See TestDocsMappedCheckIsSharedHelper for the identity pin.
var docsMappedCheck = domain.HasEntry

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

	// Requirement 3 (spec 123, #207): missing-context-map + unmapped-domain.
	checkContextMap(r, root, docsRel)

	// Domain subdirectory checks
	checkDomains(r, root, docsRel)

	// Detect stale focus/lifecycle files (ADR-0023)
	checkStaleFocusLifecycle(r, root)

	// Migration metadata checks (only when migration artifacts are present).
	checkMigrationMetadata(r, root)
}

// checkContextMap implements spec 123 Requirement 3: (a) missing-context-map
// — context-map.md absent at the layout-resolved path → Missing, with a
// --fix that scaffolds the Requirement-1 skeleton (mechanical, ZFC-safe:
// structure only, no invented content); (b) unmapped-domain — a directory
// under domains/ whose name has no corresponding entry heading in
// context-map.md → Warn naming the domain with the recovery line `mindspec
// domain add <name>`. The two states are mutually exclusive per domain: when
// context-map.md is entirely absent, every domain is trivially "unmapped"
// for the same reason the file itself is Missing, so the unmapped-domain
// scan only runs once the file exists — a second lane doesn't pile on
// redundant Warns for the one root cause the Missing finding already names.
func checkContextMap(r *Report, root, docsRel string) {
	cmPath := workspace.ContextMapPath(root)
	cmName := contextMapDisplayName(root, docsRel, cmPath)

	data, err := os.ReadFile(cmPath)
	if err != nil {
		// Only a genuinely-absent file is Missing + auto-scaffoldable. Any
		// OTHER read failure (e.g. an existing but mode-000 unreadable file,
		// or a permission-denied parent) is a concrete Error with NO scaffold
		// fixer: scaffoldContextMap would no-op on an existing file
		// (fileExists=true) yet Report.Fix() would still flip the check to
		// Fixed while the read error persists — a "reports success without
		// fixing" bug. Surfacing the real error with no fixer keeps the
		// finding honest and actionable (FX-3).
		if !os.IsNotExist(err) {
			r.Checks = append(r.Checks, Check{
				Name:    cmName,
				Status:  Error,
				Message: fmt.Sprintf("cannot read %s: %v", cmName, err),
			})
			return
		}
		r.Checks = append(r.Checks, Check{
			Name:    cmName,
			Status:  Missing,
			Message: fmt.Sprintf("create %s — run 'mindspec doctor --fix' to scaffold the skeleton", cmName),
			FixFunc: func() error {
				return scaffoldContextMap(cmPath)
			},
		})
		return
	}

	r.Checks = append(r.Checks, Check{Name: cmName, Status: OK})
	checkUnmappedDomains(r, root, docsRel, string(data))
}

// checkUnmappedDomains warns on every domains/ directory that has no
// corresponding entry heading in content (the docsMappedCheck predicate) —
// the Requirement-3(b) unmapped-domain check. Recovery is `mindspec domain
// add <name>` (the Requirement-2 backfill), not `doctor --fix`: no FixFunc
// is attached, since populating a domain's context-map entry is the same
// scaffolding action `domain add` already owns.
func checkUnmappedDomains(r *Report, root, docsRel, content string) {
	domainsDir := filepath.Join(root, docsRel, "domains")
	entries, err := os.ReadDir(domainsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if docsMappedCheck(content, name) {
			continue
		}
		r.Checks = append(r.Checks, Check{
			Name:   "context-map.md (" + name + ")",
			Status: Warn,
			Message: fmt.Sprintf("domain %q has no context-map entry — run 'mindspec domain add %s' to backfill it",
				name, name),
		})
	}
}

// scaffoldContextMap writes the Requirement-1 skeleton at cmPath if absent.
// Mechanical and ZFC-safe: structure only (title, heading, separator), no
// invented domain content. Idempotent — never overwrites an existing file.
func scaffoldContextMap(cmPath string) error {
	if fileExists(cmPath) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cmPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cmPath, []byte(domain.ContextMapSkeleton()), 0o644)
}

// contextMapDisplayName returns the repo-relative display name for the
// context-map.md check, falling back to a docsRel-joined path if the
// resolved cmPath is somehow not under root (should not happen in practice).
func contextMapDisplayName(root, docsRel, cmPath string) string {
	rel, err := filepath.Rel(root, cmPath)
	if err != nil {
		rel = filepath.Join(docsRel, "context-map.md")
	}
	return filepath.ToSlash(rel)
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

// docsRootRel returns the repo-relative directory that holds the specs/ and
// domains/ enumeration roots, resolved tier-aware (spec 106 Req 3): .mindspec
// (flat), .mindspec/docs (canonical), or docs (legacy). It is derived from the
// PARENT of the Bead-1 specs enumeration root so the doctor docs scans probe
// the right tree on a flat project, not the canonical/legacy DocsDir. On a
// canonical/legacy tree with no flat tree this is byte-identical to the
// pre-spec Rel(root, DocsDir(root)).
func docsRootRel(root string) string {
	rel, err := filepath.Rel(root, filepath.Dir(workspace.SpecsDir(root)))
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
	specsDir := workspace.SpecsDir(root) // tier-aware enumeration root (spec 106 Req 3)
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
