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
	"strings"

	"gopkg.in/yaml.v3"
)

// Ownership describes which source-tree paths a domain owns for
// doc-sync attribution. ManifestPath is the absolute path to the
// OWNERSHIP.yaml that produced this value; an empty ManifestPath
// signals the fallback "internal/<domain>/**" heuristic.
type Ownership struct {
	Paths        []string // glob patterns (e.g. "internal/foo/**")
	Exclude      []string // glob patterns subtracted from Paths
	ManifestPath string   // absolute path; "" signals fallback
}

// excludedFirstSegments enumerates the first-path-segment prefixes
// that an OWNERSHIP.yaml entry may NOT use. These trees are out of
// doc-sync scope per Hard Constraint 5 of spec 086.
var excludedFirstSegments = map[string]struct{}{
	"viz":       {},
	"agentmind": {},
	"bench":     {},
}

// loadOwnership reads .mindspec/docs/domains/<domain>/OWNERSHIP.yaml
// and returns the parsed Ownership. When the manifest file does not
// exist, it returns the fallback Ownership with Paths set to
// "internal/<domain>/**" and an empty ManifestPath (no error). On
// schema violation (an entry whose first path segment is in
// excludedFirstSegments) it returns a descriptive error.
func loadOwnership(root, domain string) (*Ownership, error) {
	manifestPath := filepath.Join(root, ".mindspec", "docs", "domains", domain, "OWNERSHIP.yaml")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Ownership{
				Paths:        []string{"internal/" + domain + "/**"},
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
func attributeDomain(root, sourcePath string, domains []string) (string, *Ownership, error) {
	for _, d := range domains {
		o, err := loadOwnership(root, d)
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
// patterns via globMatch.
func matchesAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if globMatch(p, path) {
			return true
		}
	}
	return false
}

// globMatch reports whether path matches pattern. The implementation
// supports the small dialect described in spec 086 plan Bead 1 step
// 6:
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
func globMatch(pattern, path string) bool {
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
