package panel

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// TrackerArtifactPath is the co-managed tracker export.
const TrackerArtifactPath = ".beads/issues.jsonl"

// ToolArtifactPaths is the single home for co-managed tool artifacts (R4.6).
// specDir is the workspace-resolved specs root or spec directory.
func ToolArtifactPaths(specDir string) []string {
	specs := strings.TrimSuffix(Subject("", specDir).Exclusions[0], "/*/reviews/**")
	return []string{TrackerArtifactPath, specs + "/*/reviews/*/dispositions.jsonl"}
}

// IsToolArtifactPath matches repository-relative paths with stdlib path.Match.
func IsToolArtifactPath(name, specDir string) bool {
	for _, pattern := range ToolArtifactPaths(specDir) {
		if matched, _ := path.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// ResolveLedgerArtifacts returns exact repository-relative local ledger paths.
// Registration presence, including a malformed panel.json, follows Scan's rule.
// HEAD registrations apply only to deleted or staged-deleted ledger paths.
// Git evidence is supplied by the caller so panel remains a filesystem leaf.
func ResolveLedgerArtifacts(root string, headPaths, deletedPaths []string) []string {
	roots := []string{root, filepath.Join(root, ".mindspec")}
	for _, specs := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		dirs, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(specs), "*"))
		roots = append(roots, dirs...)
	}
	ledgers := map[string]bool{}
	for _, reg := range Scan(roots...) {
		rel, err := filepath.Rel(root, filepath.Join(reg.Dir, "dispositions.jsonl"))
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			ledgers[filepath.ToSlash(rel)] = true
		}
	}
	head := map[string]bool{}
	for _, name := range headPaths {
		head[name] = true
	}
	for _, name := range deletedPaths {
		if isLedgerLocation(name) && head[path.Join(path.Dir(name), FileName)] {
			ledgers[name] = true
		}
	}
	paths := make([]string, 0, len(ledgers))
	for name := range ledgers {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths
}

// The roots mirror the supported spec layouts and Scan's two segment names.
// In particular repository reviews/ is readable by Scan, though create uses review/.
func isLedgerLocation(name string) bool {
	for _, base := range []string{"", ".mindspec/", ".mindspec/specs/*/", ".mindspec/docs/specs/*/", "docs/specs/*/"} {
		for _, seg := range reviewSegments {
			if matched, _ := path.Match(base+seg+"/*/dispositions.jsonl", name); matched {
				return true
			}
		}
	}
	return false
}
