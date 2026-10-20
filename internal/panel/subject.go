package panel

import (
	"crypto/sha256"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// SubjectRoots is derived from the gate and workspace spec directory, never
// from panel.json. Paths are repository-relative and slash-separated.
type SubjectRoots struct {
	Roots      []string
	Exclusions []string
}

// Subject derives the R3.1 subject for flat, canonical and legacy spec layouts.
// specDir is the workspace-resolved spec directory, absolute or repo-relative.
func Subject(gate, specDir string) SubjectRoots {
	dir := filepath.ToSlash(filepath.Clean(specDir))
	// Worktrees can be nested: use the last lifecycle path anchor.
	specs := ".mindspec/specs"
	for _, candidate := range []string{".mindspec/docs/specs", ".mindspec/specs", "docs/specs"} {
		if strings.HasPrefix(dir, candidate+"/") || dir == candidate {
			specs = candidate
			break
		}
		if strings.HasSuffix(dir, "/"+candidate) {
			dir = candidate
			specs = candidate
			break
		}
		if i := strings.LastIndex(dir, "/"+candidate+"/"); i >= 0 {
			dir = dir[i+1:]
			specs = candidate
			break
		}
	}
	roots := []string{"."}
	if gate == "spec_approve" || gate == "plan_approve" {
		roots = []string{dir + "/"}
	}
	return SubjectRoots{Roots: roots, Exclusions: []string{specs + "/*/reviews/**", ".mindspec/reviews/**", "review/**", ".beads/issues.jsonl"}}
}

// PanelSpecDir prefers the co-located spec directory; fallback is supplied by
// the caller's workspace resolver for repo-root and adhoc registrations.
func PanelSpecDir(panelDir, fallback string) string {
	dir := PanelDirScanRoot(panelDir)
	if filepath.Base(filepath.Dir(panelDir)) == "reviews" && filepath.Base(filepath.Dir(dir)) == "specs" {
		return dir
	}
	return fallback
}

// SubjectTreeReader adapts the executor's full-tree primitive to GateIO. Root
// selection is done in Go, equivalent to ls-tree path operands; exclusions
// must be done in Go because ls-tree rejects exclude pathspec magic.
func SubjectTreeReader(lsTree func(workdir, ref string) (string, error)) func(string, string, SubjectRoots) (string, error) {
	return func(workdir, ref string, roots SubjectRoots) (string, error) {
		listing, err := lsTree(workdir, ref)
		if err != nil {
			return "", err
		}
		return SubjectDigest(listing, roots)
	}
}

// SubjectDigest hashes the exact retained ls-tree lines. Git's quoted path
// syntax is decoded only for matching, never rewritten in the hashed listing.
// Empty and malformed listings fail closed.
func SubjectDigest(listing string, subject SubjectRoots) (string, error) {
	var filtered strings.Builder
	for _, line := range strings.SplitAfter(listing, "\n") {
		if line == "" {
			continue
		}
		metadata, name, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		if !ok || len(strings.Fields(metadata)) != 3 || name == "" {
			return "", fmt.Errorf("malformed subject tree listing")
		}
		if strings.HasPrefix(name, "\"") {
			var err error
			name, err = strconv.Unquote(name)
			if err != nil {
				return "", fmt.Errorf("malformed subject tree path: %w", err)
			}
		}
		included := false
		for _, root := range subject.Roots {
			if root == "." || strings.HasPrefix(name, strings.TrimSuffix(root, "/")+"/") || name == root {
				included = true
				break
			}
		}
		if !included {
			continue
		}
		excluded := false
		for _, pattern := range subject.Exclusions {
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				parts := strings.Split(name, "/")
				depth := len(strings.Split(prefix, "/"))
				if len(parts) > depth {
					excluded, _ = path.Match(prefix, strings.Join(parts[:depth], "/"))
				}
			} else {
				excluded = name == pattern
			}
			if excluded {
				break
			}
		}
		if !excluded {
			filtered.WriteString(line)
		}
	}
	if filtered.Len() == 0 {
		return "", fmt.Errorf("empty subject tree listing")
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(filtered.String()))), nil
}
