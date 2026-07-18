// Package validate.
//
// ownership.go provides per-domain OWNERSHIP.yaml resolution backing
// doc-sync attribution (ADR-0031). The schema rejects `viz/`,
// `agentmind/`, `bench/` first-segment entries at load time per Hard
// Constraint 5 of spec 086.
package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"gopkg.in/yaml.v3"
)

// Ownership describes which source-tree paths a domain owns for
// doc-sync attribution. ManifestPath identifies the OWNERSHIP.yaml that
// produced this value: an absolute on-disk path for the working-tree
// loader (LoadOwnership), or a ref-qualified `<ref>:<rel-path>` form for
// the ref-anchored loader (LoadOwnershipAtRef, spec 095). It is empty
// ONLY when the manifest is absent (on disk or at the ref), in which
// case the domain claims NOTHING (Paths is empty — spec 091 Req 13
// removed the silent "internal/<domain>/**" fallback heuristic). Use
// Source to distinguish the three post-load states; Source keys off
// ManifestPath == "" so both loaders share one decision point.
type Ownership struct {
	Paths        []string // glob patterns (e.g. "internal/foo/**")
	Exclude      []string // glob patterns subtracted from Paths
	ManifestPath string   // on-disk absolute path or "<ref>:<rel-path>"; "" when absent
}

// Source reports where this Ownership's claims came from. It is
// DERIVED from the existing fields (not a stored field — spec 091
// panel D2 resolution), so there is exactly one decision point and
// no loader path-specific assignment. Three states (spec 091 Req 13):
//
//	OWNERSHIP.yaml absent on disk            → "missing"    (ManifestPath "", Paths empty)
//	file exists, paths: [] (empty stub)      → "empty-stub" (ManifestPath set, Paths empty)
//	file exists, paths: [...] non-empty      → "manifest"   (ManifestPath set, Paths non-empty)
//
// Doc-sync uses Source only for diagnostic Warn text; the gate's
// pass/fail rule does not branch on it.
func (o *Ownership) Source() string {
	if o.ManifestPath == "" {
		return "missing"
	}
	if len(o.Paths) == 0 {
		return "empty-stub"
	}
	return "manifest"
}

// excludedFirstSegments enumerates the first-path-segment prefixes
// that an OWNERSHIP.yaml entry may NOT use. These trees are out of
// doc-sync scope per Hard Constraint 5 of spec 086.
var excludedFirstSegments = map[string]struct{}{
	"viz":       {},
	"agentmind": {},
	"bench":     {},
}

// LoadOwnership reads .mindspec/docs/domains/<domain>/OWNERSHIP.yaml
// and returns the parsed Ownership. When the manifest file does not
// exist, it returns an Ownership that claims NOTHING — empty Paths,
// empty ManifestPath, Source() == "missing" (no error). The silent
// fallback that synthesized "internal/<domain>/**" claims for a
// missing manifest was removed by spec 091 Req 13 (ZFC correction;
// see ADR-0036). On schema violation (an entry whose first path
// segment is in excludedFirstSegments) it returns a descriptive
// error.
//
// Exported so internal/doctor can reuse the loader (spec 091) —
// doctor must NOT reimplement manifest loading.
func LoadOwnership(root, domain string) (*Ownership, error) {
	// Tier-aware manifest location (spec 106 Req 3/6): flat
	// .mindspec/domains/<d> → canonical .mindspec/docs/domains/<d> → legacy
	// docs/domains/<d>, via the Bead-1 enumeration-root accessor.
	manifestPath := filepath.Join(workspace.DomainsDir(root), domain, "OWNERSHIP.yaml")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Ownership{
				Paths:        []string{},
				Exclude:      nil,
				ManifestPath: "",
			}, nil
		}
		return nil, fmt.Errorf("reading OWNERSHIP.yaml for domain %q: %w", domain, err)
	}

	var parsed struct {
		Paths   []string `yaml:"paths"`
		Exclude []string `yaml:"exclude"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", manifestPath, err)
	}

	for _, entry := range parsed.Paths {
		if err := checkExcludedSegment(entry); err != nil {
			return nil, err
		}
	}
	for _, entry := range parsed.Exclude {
		if err := checkExcludedSegment(entry); err != nil {
			return nil, err
		}
	}

	return &Ownership{
		Paths:        parsed.Paths,
		Exclude:      parsed.Exclude,
		ManifestPath: manifestPath,
	}, nil
}

// domainManifestRelPaths returns the repo-relative (forward-slash) candidate
// paths to a domain's OWNERSHIP.yaml across the three layouts, in
// read-precedence order: flat (.mindspec/domains/<d>/), canonical
// (.mindspec/docs/domains/<d>/), legacy (docs/domains/<d>/). git refs address
// the repo-relative form, so the ref-anchored loader must try each — a flat
// ref emits the flat path while historical refs and forks emit the
// canonical/legacy paths forever (spec 106 Req 6, PERMANENT multi-prefix
// posture, decoupled from the filesystem read-tier lifecycle). Kept separate
// from LoadOwnership's filepath.Join so the on-disk and ref-anchored loaders
// stay aligned on the manifest location across layouts.
func domainManifestRelPaths(domain string) []string {
	return []string{
		".mindspec/domains/" + domain + "/OWNERSHIP.yaml",      // flat
		".mindspec/docs/domains/" + domain + "/OWNERSHIP.yaml", // canonical
		"docs/domains/" + domain + "/OWNERSHIP.yaml",           // legacy
	}
}

// LoadOwnershipAtRef is the ref-anchored sibling of LoadOwnership (spec
// 095 / mindspec-vvs9; ADR-0031 amend): it reads a domain's
// OWNERSHIP.yaml from the SAME git ref the gate diffs — via the
// executor, not os.ReadFile on the ambient working tree — so an
// OWNERSHIP claim committed on a branch satisfies its own gate with no
// override.
//
// Outcome classification (ADR-0036 amend — operational-error ≠ absent):
//
//   - manifest ABSENT in ref's tree → claims-nothing Ownership
//     (ManifestPath "", Source() == "missing"), NO error — treated
//     identically to absent-on-disk.
//   - OPERATIONAL git/executor failure (invalid ref, git error) →
//     propagated as a hard error. It is NEVER collapsed to
//     claims-nothing: a transient git glitch must not silently
//     un-attribute a file and un-gate doc-drift.
//
// The present/absent split is made at the executor boundary
// (FileAtRefOrAbsent), because `git show <ref>:<path>` returns a generic
// error for BOTH missing-path and bad-ref. The HC-5
// excluded-first-segment schema check and the YAML parse run
// byte-identically on the ref bytes. ManifestPath is ref-qualified
// (`<ref>:<rel-path>`) since it is no longer an on-disk path.
func LoadOwnershipAtRef(exec executor.Executor, ref, domain string) (*Ownership, error) {
	// Try the three layout candidates in read-precedence order, first-present
	// wins (spec 106 Req 6). On a canonical/legacy ref the flat candidate is
	// simply absent (present=false, no error) and the canonical/legacy one is
	// used — byte-identical to the pre-spec single-path read. An OPERATIONAL
	// failure (bad ref) errors on the FIRST probe and is propagated, never
	// collapsed to claims-nothing (ADR-0036 amend).
	var data []byte
	var foundRel string
	for _, relPath := range domainManifestRelPaths(domain) {
		d, present, err := exec.FileAtRefOrAbsent(ref, relPath)
		if err != nil {
			return nil, fmt.Errorf("reading OWNERSHIP.yaml for domain %q at ref %q: %w", domain, ref, err)
		}
		if present {
			data = d
			foundRel = relPath
			break
		}
	}
	if foundRel == "" {
		return &Ownership{
			Paths:        []string{},
			Exclude:      nil,
			ManifestPath: "",
		}, nil
	}

	var parsed struct {
		Paths   []string `yaml:"paths"`
		Exclude []string `yaml:"exclude"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing %s:%s: %w", ref, foundRel, err)
	}

	for _, entry := range parsed.Paths {
		if err := checkExcludedSegment(entry); err != nil {
			return nil, err
		}
	}
	for _, entry := range parsed.Exclude {
		if err := checkExcludedSegment(entry); err != nil {
			return nil, err
		}
	}

	return &Ownership{
		Paths:        parsed.Paths,
		Exclude:      parsed.Exclude,
		ManifestPath: ref + ":" + foundRel,
	}, nil
}

// domainsTreeRoots are the repo-relative domains enumeration roots across the
// three layouts (flat, canonical, legacy). The ref-anchored enumerator unions
// over all three so a branch in ANY layout is enumerable (spec 106 Req 6).
var domainsTreeRoots = []string{
	".mindspec/domains",      // flat
	".mindspec/docs/domains", // canonical
	"docs/domains",           // legacy
}

// listDomainDirsAtRef is the ref-anchored sibling of listDomainDirs
// (spec 095): it enumerates the domain directories under the domains
// enumeration root in ref's tree via the executor, so a branch-only domain dir
// is discovered from the diffed ref. Spec 106 Req 6: it unions the three
// layout roots (flat/canonical/legacy), deduping by name, so a flat ref AND a
// canonical/legacy ref both enumerate correctly. An absent domains/ tree at a
// valid ref yields an empty slice (no error — TreeDirsAtRef returns empty for
// an absent dir at a valid ref); an operational git failure (invalid ref) is a
// hard error.
func listDomainDirsAtRef(exec executor.Executor, ref string) ([]string, error) {
	seen := map[string]struct{}{}
	for _, root := range domainsTreeRoots {
		dirs, err := exec.TreeDirsAtRef(ref, root)
		if err != nil {
			return nil, fmt.Errorf("listing domain dirs at ref %q: %w", ref, err)
		}
		for _, d := range dirs {
			seen[d] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out, nil
}

// resolveDomains enumerates domain directory names for the gate. When
// ownerRef is non-empty it reads from the diffed ref (spec 095);
// otherwise it preserves the working-tree read (the `mindspec validate
// docs` CLI / doctor path). The dispatch lives here so every gate seam
// chooses the tree the same way.
func resolveDomains(exec executor.Executor, root, ownerRef string) ([]string, error) {
	if ownerRef == "" {
		return listDomainDirs(root)
	}
	return listDomainDirsAtRef(exec, ownerRef)
}

// loadOwnershipForRef reads a single domain's OWNERSHIP from the diffed
// ref when ownerRef is non-empty (spec 095), else from the on-disk
// root. Companion of resolveDomains: both gates pick the same tree.
func loadOwnershipForRef(exec executor.Executor, root, ownerRef, domain string) (*Ownership, error) {
	if ownerRef == "" {
		return LoadOwnership(root, domain)
	}
	return LoadOwnershipAtRef(exec, ownerRef, domain)
}

// loadOwnershipForRefFn is the package-level seam through which every
// hoisted per-domain manifest load routes (spec 108 R7). It defaults to
// loadOwnershipForRef; the per-run ownershipCache reads through it, so a
// test can swap it for a counter and assert that a multi-file gate run
// loads each domain's manifest a number of times that is a function of
// domain count only — independent of the number of changed files.
var loadOwnershipForRefFn = loadOwnershipForRef

// ownershipCache memoizes per-domain Ownership loads within a single gate
// run so each candidate domain's OWNERSHIP.yaml is read from disk (and, at
// a ref, git-show'd up to three times in LoadOwnershipAtRef) AT MOST ONCE,
// regardless of how many changed files are attributed against it (spec 108
// R7). It generalizes the "load every manifest once, attribute in memory"
// pattern checkUnclaimedSource already uses (docsync.go) to the three
// per-(file × domain) attribution sites (ValidateDivergence,
// checkInternalPackages, normalizeImpactedDomains). Loads route through
// the loadOwnershipForRefFn seam so a test can count them.
//
// Both the success AND the error outcome are memoized: a broken manifest
// is read once and then re-surfaces the SAME (nil, err) on every
// subsequent lookup. This is byte-identical to the pre-cache behavior,
// where attributeDomain re-loaded the manifest per file and re-emitted the
// same per-file attribution error each time — the cache changes the number
// of reads, never the diagnostics.
//
// A cache is scoped to ONE gate-run call; it is never shared across runs,
// so it holds no cross-run staleness. Access is single-threaded within a
// run (the file loops are sequential), so the plain map needs no
// synchronization.
type ownershipCache struct {
	exec     executor.Executor
	root     string
	ownerRef string
	entries  map[string]ownershipCacheEntry
}

type ownershipCacheEntry struct {
	own *Ownership
	err error
}

func newOwnershipCache(exec executor.Executor, root, ownerRef string) *ownershipCache {
	return &ownershipCache{
		exec:     exec,
		root:     root,
		ownerRef: ownerRef,
		entries:  map[string]ownershipCacheEntry{},
	}
}

// get returns the domain's Ownership, loading (and memoizing) it through
// the loadOwnershipForRefFn seam on first request and returning the
// memoized outcome thereafter.
func (c *ownershipCache) get(domain string) (*Ownership, error) {
	if e, ok := c.entries[domain]; ok {
		return e.own, e.err
	}
	o, err := loadOwnershipForRefFn(c.exec, c.root, c.ownerRef, domain)
	c.entries[domain] = ownershipCacheEntry{own: o, err: err}
	return o, err
}

// checkExcludedSegment returns an error when entry's first path
// segment is in excludedFirstSegments.
func checkExcludedSegment(entry string) error {
	segment := strings.SplitN(entry, "/", 2)[0]
	if _, bad := excludedFirstSegments[segment]; bad {
		return fmt.Errorf("OWNERSHIP.yaml entry %q has excluded first segment %q (viz/agentmind/bench trees are out of doc-sync scope)", entry, segment)
	}
	return nil
}

// attributeDomain returns the owning domain name for a changed
// source-file path. It iterates domains in lexicographic order of the
// domain directory name (caller responsibility — pass a sorted slice)
// and returns the FIRST domain whose Ownership matches via Paths
// minus Exclude. Returns ("", nil, nil) when no domain claims the
// file.
//
// ownerRef selects the tree the per-domain OWNERSHIP manifest is read
// from: a non-empty ref reads from the diffed ref (spec 095 /
// mindspec-vvs9), while "" preserves the on-disk working-tree read
// (`mindspec validate docs` CLI). exec is the executor backing the ref
// read; it is unused (and may be nil) when ownerRef == "".
func attributeDomain(exec executor.Executor, root, ownerRef, sourcePath string, domains []string) (string, *Ownership, error) {
	return attributeDomainCached(newOwnershipCache(exec, root, ownerRef), sourcePath, domains)
}

// attributeDomainCached is attributeDomain over a caller-supplied
// ownershipCache, so a gate that attributes many files reuses ONE per-run
// cache instead of re-loading every domain's manifest per file (spec 108
// R7). Semantics are identical to attributeDomain: iterate domains in the
// given (caller-sorted) order, return the FIRST whose Paths match minus
// Exclude, or ("", nil, nil) when none claims the file; a load error
// propagates exactly as before. attributeDomain itself is now the
// single-file convenience wrapper that allocates a one-shot cache.
func attributeDomainCached(cache *ownershipCache, sourcePath string, domains []string) (string, *Ownership, error) {
	for _, d := range domains {
		o, err := cache.get(d)
		if err != nil {
			return "", nil, err
		}
		if !matchesAny(o.Paths, sourcePath) {
			continue
		}
		if matchesAny(o.Exclude, sourcePath) {
			continue
		}
		return d, o, nil
	}
	return "", nil, nil
}

// matchesAny returns true when path matches at least one pattern in
// patterns via GlobMatch.
func matchesAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if GlobMatch(p, path) {
			return true
		}
	}
	return false
}

// GlobMatch reports whether path matches pattern. Exported so
// internal/doctor can reuse the matcher (spec 091) — doctor must NOT
// reimplement glob matching. The implementation supports the small
// dialect described in spec 086 plan Bead 1 step 6:
//
//   - leading `**/`   — matches zero or more leading path segments
//   - trailing `/**`  — matches the directory itself OR any
//     descendant file
//   - mid-path `**`   — matches zero or more path segments anywhere
//     in the middle of the pattern
//   - `?`             — matches a single non-`/` character (per
//     path/filepath.Match)
//   - `\*`            — escaped literal asterisk (per
//     path/filepath.Match)
//
// Segment-level matching delegates to path/filepath.Match.
func GlobMatch(pattern, path string) bool {
	// Special-case: trailing /** matches the directory itself.
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if path == prefix {
			return true
		}
	}

	patSegs := strings.Split(pattern, "/")
	pathSegs := strings.Split(path, "/")
	return globMatchSegments(patSegs, pathSegs)
}

// globMatchSegments matches pattern segments against path segments,
// treating a literal "**" segment as a multi-segment wildcard
// (matches zero or more path segments).
func globMatchSegments(pat, path []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// Try consuming 0..len(path) path segments.
			rest := pat[1:]
			if len(rest) == 0 {
				return true // trailing ** matches anything (including empty).
			}
			for i := 0; i <= len(path); i++ {
				if globMatchSegments(rest, path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		ok, err := filepath.Match(pat[0], path[0])
		if err != nil || !ok {
			return false
		}
		pat = pat[1:]
		path = path[1:]
	}
	return len(path) == 0
}

// ResolveCandidateDomains resolves the domain-NAME set OWNERSHIP
// attribution should consult for specDir: the spec's declared `##
// Impacted Domains` entries, normalized to owning-domain names (spec 100
// R1, normalizeImpactedDomains) or — when the spec declares none (missing
// spec.md, or a spec.md with no Impacted-Domains section) — every domain
// directory discovered at ownerRef (or the on-disk working tree when
// ownerRef is "").
//
// This is the EXACT candidate-domain fallback ValidateDivergence performs
// internally for its own per-file attribution loop (declared-domains ||
// full-enumeration); exported here — additively, ValidateDivergence's own
// inline logic is left untouched so its pinned error-message tests keep
// passing byte-for-byte — so a second advisory gate consulting the SAME
// OWNERSHIP data (Spec 119 R11's bead-scope WARN,
// internal/complete/bead_scope.go) reuses it instead of re-deriving the
// same fallback with its own drift risk.
//
// A missing spec.md is not an error: it degrades to the domain-directory
// enumeration fallback, same as an empty declared-domains list would.
func ResolveCandidateDomains(exec executor.Executor, root, specDir, ownerRef string) ([]string, error) {
	var declared []string
	if meta, err := contextpack.ParseSpec(specDir); err == nil {
		declared = meta.Domains
	} else if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
		return nil, fmt.Errorf("loading spec metadata: %w", err)
	}

	normalized, normErrs := normalizeImpactedDomains(exec, root, ownerRef, declared)
	if len(normErrs) > 0 {
		return nil, fmt.Errorf("resolving impacted domains: %s", strings.Join(normErrs, "; "))
	}
	if len(normalized) > 0 {
		sort.Strings(normalized)
		return normalized, nil
	}

	disc, derr := resolveDomains(exec, root, ownerRef)
	if derr != nil {
		return nil, derr
	}
	sort.Strings(disc)
	return disc, nil
}

// AttributeChangedFileDomains attributes each of paths to its owning
// domain among candidateDomains (a domain-NAME set, typically
// ResolveCandidateDomains' return value), reusing the SAME per-run
// ownershipCache + attributeDomainCached machinery ValidateDivergence
// uses — so a caller attributing many files loads each candidate domain's
// OWNERSHIP.yaml at most once (spec 108 R7 discipline), never
// re-implementing OWNERSHIP.yaml glob matching.
//
// Returns a path -> domain map. A path attributed to NO candidate domain
// (unowned, excluded via the viz/agentmind/bench first-segment rule, or a
// non-source process artifact per isProcessArtifact — mirroring
// ValidateDivergence's identical skip so the two gates agree on what
// counts as governable source) is OMITTED from the map entirely — callers
// distinguish "unowned" from "owned by domain X" via ok, not by an
// empty-string value.
func AttributeChangedFileDomains(exec executor.Executor, root, ownerRef string, paths, candidateDomains []string) (map[string]string, error) {
	sortedDomains := append([]string(nil), candidateDomains...)
	sort.Strings(sortedDomains)

	cache := newOwnershipCache(exec, root, ownerRef)
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		seg := p
		if idx := strings.Index(p, "/"); idx >= 0 {
			seg = p[:idx]
		}
		if _, bad := excludedFirstSegments[seg]; bad {
			continue
		}
		if isProcessArtifact(p) {
			continue
		}
		domain, _, err := attributeDomainCached(cache, p, sortedDomains)
		if err != nil {
			return nil, fmt.Errorf("attributing %s: %w", p, err)
		}
		if domain != "" {
			out[p] = domain
		}
	}
	return out, nil
}
