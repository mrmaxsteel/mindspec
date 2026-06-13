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

	"github.com/mrmaxsteel/mindspec/internal/executor"
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
	manifestPath := filepath.Join(root, ".mindspec", "docs", "domains", domain, "OWNERSHIP.yaml")

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

// domainManifestRelPath returns the repo-relative (forward-slash) path
// to a domain's OWNERSHIP.yaml, the form git refs address. It is kept
// separate from LoadOwnership's filepath.Join so the on-disk and
// ref-anchored loaders cannot drift on the manifest location.
func domainManifestRelPath(domain string) string {
	return ".mindspec/docs/domains/" + domain + "/OWNERSHIP.yaml"
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
	relPath := domainManifestRelPath(domain)

	data, present, err := exec.FileAtRefOrAbsent(ref, relPath)
	if err != nil {
		return nil, fmt.Errorf("reading OWNERSHIP.yaml for domain %q at ref %q: %w", domain, ref, err)
	}
	if !present {
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
		return nil, fmt.Errorf("parsing %s:%s: %w", ref, relPath, err)
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
		ManifestPath: ref + ":" + relPath,
	}, nil
}

// listDomainDirsAtRef is the ref-anchored sibling of listDomainDirs
// (spec 095): it enumerates the domain directories under
// .mindspec/docs/domains/ in ref's tree via the executor, so a
// branch-only domain dir is discovered from the diffed ref. An absent
// domains/ tree at a valid ref yields an empty slice (no error); an
// operational git failure (invalid ref) is a hard error.
func listDomainDirsAtRef(exec executor.Executor, ref string) ([]string, error) {
	dirs, err := exec.TreeDirsAtRef(ref, ".mindspec/docs/domains")
	if err != nil {
		return nil, fmt.Errorf("listing domain dirs at ref %q: %w", ref, err)
	}
	sort.Strings(dirs)
	return dirs, nil
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
	for _, d := range domains {
		o, err := loadOwnershipForRef(exec, root, ownerRef, d)
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
