package panel

import (
	"path"
	"strings"
)

// ToolArtifactPaths is the single home for co-managed tool artifacts (R4.6).
// specDir is the workspace-resolved specs root or spec directory.
func ToolArtifactPaths(specDir string) []string {
	specs := strings.TrimSuffix(Subject("", specDir).Exclusions[0], "/*/reviews/**")
	return []string{".beads/issues.jsonl", specs + "/*/reviews/*/dispositions.jsonl"}
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
