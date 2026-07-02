package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// walkExclusions are the top-level directory names skipped by the
// dead-manifest workspace walk (spec 091 Req 17, V2-6 binding). A
// stray match inside one of these trees (e.g. internal/foo/** matching
// .worktrees/<wt>/internal/foo/bar.go) would mask a genuinely dead
// manifest, so they are excluded.
var walkExclusions = map[string]struct{}{
	".git":       {},
	".worktrees": {},
	".beads":     {},
}

// walkWorkspaceFn is the package-level seam for the single workspace tree
// enumeration (spec 108 Req 9). It defaults to filepath.WalkDir; tests
// swap it to count invocations and prove the tree is walked exactly once
// per ownership check regardless of domain count.
var walkWorkspaceFn = filepath.WalkDir

// checkOwnershipManifests runs the static-time ownership manifest
// health checks (spec 091): the dead-manifest Warn (Req 17) for every
// EXISTING manifest whose paths glob set resolves to zero files, then
// the three hygiene Warns (Req 20: duplicate-entry, redundant-subpath,
// domain-overlap). All are advisory — none blocks the gate.
//
// Manifest state is loaded via validate.LoadOwnership and glob matching
// via validate.GlobMatch (Bead 1's shared helpers) — doctor must NOT
// reimplement either, or the Warns would lie.
func checkOwnershipManifests(r *Report, root string) {
	domains := canonicalDomains(root)
	if len(domains) == 0 {
		return
	}

	// Enumerate the live workspace file list ONCE per ownership check
	// (spec 108 Req 9), then test every domain's paths globs against this
	// cached list instead of re-walking the tree per domain.
	files := enumerateWorkspaceFiles(root)

	// Per-domain dead-manifest, plus collect loaded paths for the
	// cross-domain hygiene checks.
	loaded := make(map[string]*validate.Ownership, len(domains))
	for _, d := range domains {
		o, err := validate.LoadOwnership(root, d)
		if err != nil {
			// Schema violation (e.g. excluded first segment) — surface
			// it; the manifest cannot be evaluated for liveness.
			r.Checks = append(r.Checks, Check{
				Name:    manifestCheckName(d),
				Status:  Warn,
				Message: fmt.Sprintf("OWNERSHIP.yaml invalid: %v", err),
			})
			continue
		}
		loaded[d] = o
		checkDeadManifest(r, files, d, o)
	}

	checkHygiene(r, domains, loaded)
}

// enumerateWorkspaceFiles walks the workspace tree ONCE via the
// walkWorkspaceFn seam and returns every non-directory file path relative
// to root (slash-separated). The walk skips the walkExclusions trees
// (.git/, .worktrees/, .beads/, V2-6) so a stray match inside those trees
// cannot mask a dead manifest. The result is consumed by every domain's
// dead-manifest check within a single ownership check, replacing the
// former per-domain full walk.
func enumerateWorkspaceFiles(root string) []string {
	var files []string
	_ = walkWorkspaceFn(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate unreadable entries; keep walking
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if _, skip := walkExclusions[rel]; skip {
				return fs.SkipDir
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files
}

// canonicalDomains returns the lexicographically-sorted domain
// directory names under the tier-aware domains enumeration root
// (flat .mindspec/domains → canonical .mindspec/docs/domains → legacy
// docs/domains, spec 106 Req 3) — the same layout validate.LoadOwnership
// reads. Returns nil when the directory is absent.
func canonicalDomains(root string) []string {
	dir := workspace.DomainsDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var domains []string
	for _, e := range entries {
		if e.IsDir() {
			domains = append(domains, e.Name())
		}
	}
	sort.Strings(domains)
	return domains
}

func manifestCheckName(domain string) string {
	return filepath.ToSlash(filepath.Join("docs", "domains", domain, "OWNERSHIP.yaml"))
}

// checkDeadManifest emits the dead-manifest Warn (spec 091 Req 17) when
// an EXISTING manifest's whole paths set resolves to zero files in the
// workspace. A missing manifest (Source() == "missing") NEVER fires it
// — that state is owned solely by the Req 21 missing-OWNERSHIP Warn
// (one state, one Warn). An empty stub (paths: []) trivially resolves
// to zero files and DOES fire, with the suspect glob reported as
// "(empty)".
func checkDeadManifest(r *Report, files []string, domain string, o *validate.Ownership) {
	if o.Source() == "missing" {
		return // covered by the Req 21 missing-OWNERSHIP Warn
	}

	if manifestResolvesAny(files, o.Paths) {
		return
	}

	suspect := "(empty)"
	if len(o.Paths) > 0 {
		suspect = strings.Join(o.Paths, ", ")
	}
	r.Checks = append(r.Checks, Check{
		Name:   manifestCheckName(domain),
		Status: Warn,
		Message: fmt.Sprintf("dead-manifest: paths glob %s resolves to zero files in the workspace — "+
			"run 'mindspec ownership populate %s' to emit an agent prompt", suspect, domain),
	})
}

// manifestResolvesAny reports whether any of the manifest's paths globs
// matches at least one file in the pre-enumerated workspace file list
// (the slash-separated relative paths from enumerateWorkspaceFiles, which
// already excludes the .git/, .worktrees/, and .beads/ trees per V2-6). An
// empty paths set trivially resolves to nothing. Matching delegates to the
// shared validate.GlobMatch.
func manifestResolvesAny(files, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, rel := range files {
		if matchesAnyGlob(paths, rel) {
			return true
		}
	}
	return false
}

// matchesAnyGlob reports whether rel matches at least one pattern,
// delegating each comparison to the shared validate.GlobMatch (doctor
// does NOT reimplement glob matching — this is a thin iteration over
// the exported matcher).
func matchesAnyGlob(patterns []string, rel string) bool {
	for _, p := range patterns {
		if validate.GlobMatch(p, rel) {
			return true
		}
	}
	return false
}

// checkHygiene emits the three advisory hygiene Warns (spec 091 Req 20)
// over the loaded manifests. All are literal-string checks (no glob
// resolution), all advisory, all hand-edit-only.
func checkHygiene(r *Report, domains []string, loaded map[string]*validate.Ownership) {
	// duplicate-entry + redundant-subpath are per-domain.
	for _, d := range domains {
		o, ok := loaded[d]
		if !ok {
			continue
		}
		checkDuplicateEntry(r, d, "paths", o.Paths)
		checkDuplicateEntry(r, d, "exclude", o.Exclude)
		checkRedundantSubpath(r, d, o.Paths)
	}

	// domain-overlap is cross-domain.
	checkDomainOverlap(r, domains, loaded)
}

// checkDuplicateEntry warns when the same literal path appears more
// than once within a single domain's list (paths or exclude).
func checkDuplicateEntry(r *Report, domain, field string, entries []string) {
	seen := make(map[string]bool, len(entries))
	var dupes []string
	for _, e := range entries {
		if seen[e] {
			dupes = append(dupes, e)
		}
		seen[e] = true
	}
	if len(dupes) == 0 {
		return
	}
	r.Checks = append(r.Checks, Check{
		Name:   manifestCheckName(domain),
		Status: Warn,
		Message: fmt.Sprintf("duplicate-entry: %s contains the same path more than once: %s",
			field, strings.Join(dedupe(dupes), ", ")),
	})
}

// checkRedundantSubpath warns when a paths entry is a strict
// glob-subpath of another paths entry in the same domain (the narrower
// entry is fully implied by the wider one). Prefix matching on the
// literal path string after stripping a trailing /** (per Req 20). The
// Warn names BOTH entries and identifies the redundant (narrower) one.
func checkRedundantSubpath(r *Report, domain string, paths []string) {
	for i, narrow := range paths {
		for j, wide := range paths {
			if i == j || narrow == wide {
				continue
			}
			if isStrictSubpath(narrow, wide) {
				r.Checks = append(r.Checks, Check{
					Name:   manifestCheckName(domain),
					Status: Warn,
					Message: fmt.Sprintf("redundant-subpath: %q is implied by %q — the narrower entry %q is noise (or the wider one is wrong)",
						narrow, wide, narrow),
				})
			}
		}
	}
}

// isStrictSubpath reports whether narrow is a strict path-prefix of wide
// after stripping a trailing /** from each. e.g. internal/foo/bar/** is
// a strict subpath of internal/foo/**. Equality is NOT a strict subpath.
func isStrictSubpath(narrow, wide string) bool {
	np := strings.TrimSuffix(narrow, "/**")
	wp := strings.TrimSuffix(wide, "/**")
	if np == wp {
		return false
	}
	return strings.HasPrefix(np, wp+"/")
}

// checkDomainOverlap warns when the same literal path appears in
// paths across two or more domains' manifests. Literal-string
// comparison only (Req 20) — two different glob strings resolving to
// overlapping file sets are NOT flagged (accepted gap, ADR-0036 (i)).
func checkDomainOverlap(r *Report, domains []string, loaded map[string]*validate.Ownership) {
	claimants := make(map[string][]string) // path -> domains claiming it
	for _, d := range domains {
		o, ok := loaded[d]
		if !ok {
			continue
		}
		for _, p := range dedupe(o.Paths) {
			claimants[p] = append(claimants[p], d)
		}
	}

	// Deterministic order over overlapping paths.
	var overlapping []string
	for p, ds := range claimants {
		if len(ds) > 1 {
			overlapping = append(overlapping, p)
		}
	}
	sort.Strings(overlapping)

	for _, p := range overlapping {
		ds := claimants[p]
		sort.Strings(ds)
		r.Checks = append(r.Checks, Check{
			Name:   "OWNERSHIP.yaml domain-overlap",
			Status: Warn,
			Message: fmt.Sprintf("domain-overlap: path %q is claimed by multiple domains: %s — "+
				"decide which domain owns it (or split the path)", p, strings.Join(ds, ", ")),
		})
	}
}

// dedupe returns entries with duplicates removed, order preserved.
func dedupe(entries []string) []string {
	seen := make(map[string]bool, len(entries))
	var out []string
	for _, e := range entries {
		if seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	return out
}
