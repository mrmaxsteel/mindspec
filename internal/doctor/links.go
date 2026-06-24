package doctor

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DanglingLink describes a markdown link whose local target does not resolve
// to an existing file on disk (a 404). File and Resolved are repo-relative
// slash paths; Target is the link target exactly as written in the source.
type DanglingLink struct {
	File     string
	Target   string
	Resolved string
}

// inlineLinkRe matches inline markdown links and images: the `](target)` form.
// The target is everything up to the first whitespace or the closing paren, so
// an optional `"title"` after the target is ignored.
var inlineLinkRe = regexp.MustCompile(`\]\(\s*([^)\s]+)`)

// refDefRe matches a reference-style link definition: `[label]: target` at the
// start of a (trimmed) line.
var refDefRe = regexp.MustCompile(`^\[[^\]]+\]:\s+(\S+)`)

// movedTreeRoots is the set of post-migration flat lifecycle locations the
// link-existence lane scans (Req 5 / AC10) — every markdown link under these,
// PLUS the affected repo-root docs (movedTreeRootDocs), is checked. The flat
// lifecycle children carry the co-located reviews (`<spec-dir>/reviews/…`)
// under `.mindspec/specs`; `project-docs` is the dogfood-eviction destination,
// scanned so the asymmetric depth-change links the eviction introduces are
// gated for 404s alongside the lifecycle trees.
var movedTreeRoots = []string{
	filepath.Join(".mindspec", "specs"),
	filepath.Join(".mindspec", "adr"),
	filepath.Join(".mindspec", "domains"),
	filepath.Join(".mindspec", "core"),
	"project-docs",
}

// movedTreeRootDocs are the repo-root docs that reference INTO the moved trees
// and so must be scanned alongside them (Req 5: README/AGENTS refs).
var movedTreeRootDocs = []string{"README.md", "AGENTS.md", ".mindspec/context-map.md"}

// CheckMovedTreeLinks scans EVERY markdown link in the post-migration flat
// lifecycle tree (.mindspec/{specs,adr,domains,core}) AND the affected root
// docs (README.md, AGENTS.md, .mindspec/context-map.md), resolving each LOCAL
// link target against the filesystem, and returns every link that 404s.
//
// This is the GATING link-existence lane the `migrate layout` mover runs after
// the moves+rewrites and before it finalizes (Req 5): it scans the WHOLE tree,
// not merely the finite set the rewriter touched, so a breaking link the
// rewriter failed to anticipate — and a dangling link the rewriter mis-pointed
// — are BOTH caught at the gate rather than shipped. External links (http(s),
// mailto, protocol-relative), pure `#anchor` targets, and empty targets are
// not filesystem-resolvable and are skipped.
func CheckMovedTreeLinks(root string) ([]DanglingLink, error) {
	var files []string
	for _, rel := range movedTreeRoots {
		abs := filepath.Join(root, rel)
		err := filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				// An absent moved-tree root is fine (e.g. a tree with no
				// domains) — nothing to scan there.
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, rel := range movedTreeRootDocs {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if fileExists(abs) {
			files = append(files, abs)
		}
	}
	sort.Strings(files)

	var dangling []DanglingLink
	for _, abs := range files {
		links, err := extractMarkdownLinks(abs)
		if err != nil {
			return nil, err
		}
		fileDir := filepath.Dir(abs)
		for _, target := range links {
			resolved, ok := resolveLocalLink(root, fileDir, target)
			if !ok {
				continue // external / anchor / empty — not filesystem-checked
			}
			if !pathExists(resolved) {
				relFile, _ := filepath.Rel(root, abs)
				relResolved, _ := filepath.Rel(root, resolved)
				dangling = append(dangling, DanglingLink{
					File:     filepath.ToSlash(relFile),
					Target:   target,
					Resolved: filepath.ToSlash(relResolved),
				})
			}
		}
	}
	return dangling, nil
}

// extractMarkdownLinks returns the raw targets of every inline link/image and
// reference-style link definition in the file at path.
func extractMarkdownLinks(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var targets []string
	for _, m := range inlineLinkRe.FindAllStringSubmatch(string(data), -1) {
		targets = append(targets, m[1])
	}
	for _, line := range strings.Split(string(data), "\n") {
		if m := refDefRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			targets = append(targets, m[1])
		}
	}
	return targets, nil
}

// resolveLocalLink decides whether target is a filesystem-resolvable local
// link and, if so, returns its absolute path. ok is false for external links
// (http(s)/mailto/tel/protocol-relative), pure `#anchor` targets, and empty
// targets. A leading `/` is treated as repo-root-relative.
func resolveLocalLink(root, fileDir, target string) (string, bool) {
	target = strings.TrimSpace(target)
	target = strings.Trim(target, "<>")
	if target == "" || strings.HasPrefix(target, "#") {
		return "", false
	}
	if strings.Contains(target, "://") || strings.HasPrefix(target, "//") ||
		strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "tel:") {
		return "", false
	}
	// Drop any #fragment / ?query suffix.
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	if target == "" {
		return "", false
	}
	target = filepath.FromSlash(target)
	if filepath.IsAbs(target) || strings.HasPrefix(target, string(filepath.Separator)) {
		return filepath.Join(root, strings.TrimPrefix(target, string(filepath.Separator))), true
	}
	return filepath.Join(fileDir, target), true
}

// pathExists reports whether path exists (file OR directory) — a directory
// link target (e.g. a domain folder) is a valid resolution.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// LinksReport runs CheckMovedTreeLinks and renders one Check per dangling
// link (Error), or a single OK check when every link resolves. It is the
// report-surface adapter; the mover consumes CheckMovedTreeLinks directly so
// it can FAIL the run-state machine on any 404 (Req 5).
func LinksReport(root string) []Check {
	dangling, err := CheckMovedTreeLinks(root)
	if err != nil {
		return []Check{{Name: "links", Status: Error, Message: err.Error()}}
	}
	if len(dangling) == 0 {
		return []Check{{Name: "links", Status: OK, Message: "all markdown links resolve"}}
	}
	checks := make([]Check, 0, len(dangling))
	for _, d := range dangling {
		checks = append(checks, Check{
			Name:    d.File,
			Status:  Error,
			Message: "dangling link " + d.Target + " → " + d.Resolved + " (404)",
		})
	}
	return checks
}
