package validate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// planPathTokenDelims splits plan body text into candidate tokens. Markdown
// and prose punctuation that can wrap a path (backticks, quotes, brackets,
// parentheses, commas, pipes) act as separators.
var planPathTokenDelims = regexp.MustCompile("[\\s`'\"()\\[\\]{},;|]+")

// planFilePathRe matches a repo-relative file path: two or more segments
// separated by "/", with a final segment that ends in a file extension.
var planFilePathRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+\.([A-Za-z0-9]+)$`)

// planFileExts is the set of file extensions that mark a token as a file
// path. A fixed set keeps Go qualified names ("internal/panel.Scan") and
// prose ("and/or.") out of the result.
var planFileExts = map[string]bool{
	"go": true, "mod": true, "sum": true, "md": true, "txt": true,
	"yaml": true, "yml": true, "json": true, "jsonl": true, "toml": true,
	"js": true, "mjs": true, "cjs": true, "ts": true, "tsx": true, "jsx": true,
	"py": true, "sh": true, "bash": true, "rb": true, "rs": true,
	"java": true, "kt": true, "c": true, "h": true, "cc": true, "cpp": true,
	"swift": true, "proto": true, "sql": true, "html": true, "css": true,
	"tmpl": true, "tpl": true, "cfg": true, "ini": true, "xml": true,
	"csv": true, "svg": true, "lock": true,
}

// planPathToolDirs are tool-state trees that a plan names for context but
// that no bead lists as a file it edits.
var planPathToolDirs = []string{".mindspec/", ".beads/"}

// ExtractPlanFilePaths returns the sorted, deduplicated repo-relative file
// paths named in text. It prefers precision over recall: it skips URLs,
// absolute and home-relative paths, globs, placeholders, and paths with
// ".." segments. A trailing line reference (":41" or "#L41") and trailing
// sentence punctuation are removed, and a leading "./" is dropped.
func ExtractPlanFilePaths(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tok := range planPathTokenDelims.Split(text, -1) {
		p, ok := normalizePlanPathToken(tok)
		if !ok || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func normalizePlanPathToken(tok string) (string, bool) {
	if tok == "" || strings.Contains(tok, "://") {
		return "", false
	}
	if strings.ContainsAny(tok, "*<>?$=@~\\") {
		return "", false
	}
	if i := strings.IndexAny(tok, ":#"); i >= 0 {
		tok = tok[:i]
	}
	tok = strings.TrimRight(tok, ".!")
	if strings.HasPrefix(tok, "/") {
		return "", false
	}
	tok = strings.TrimPrefix(tok, "./")
	m := planFilePathRe.FindStringSubmatch(tok)
	if m == nil || !planFileExts[strings.ToLower(m[1])] {
		return "", false
	}
	segs := strings.Split(tok, "/")
	for i, seg := range segs {
		if strings.Trim(seg, ".") == "" {
			return "", false // ".", "..", "..." and empty segments
		}
		// A directory segment with a file extension ("spec.md/plan.md")
		// is prose for "one or the other", not a path.
		if i < len(segs)-1 {
			if dot := strings.LastIndex(seg, "."); dot > 0 && planFileExts[strings.ToLower(seg[dot+1:])] {
				return "", false
			}
		}
	}
	for _, dir := range planPathToolDirs {
		if strings.HasPrefix(tok, dir) {
			return "", false
		}
	}
	return tok, true
}

// keyFilePathCovers reports whether a key_file_paths entry covers path p:
// an exact match, a directory prefix ("internal/foo" or "internal/foo/"),
// a glob that GlobMatch accepts, or a listed path that p abbreviates by
// its trailing segments ("approve/plan.go" for "internal/approve/plan.go").
func keyFilePathCovers(entry, p string) bool {
	e := strings.TrimPrefix(strings.TrimSpace(entry), "./")
	if e == "" {
		return false
	}
	if e == p || strings.HasSuffix(e, "/"+p) {
		return true
	}
	if strings.ContainsAny(e, "*?[") {
		return GlobMatch(e, p)
	}
	dir := strings.TrimSuffix(e, "/")
	return dir != "" && strings.HasPrefix(p, dir+"/")
}

// UnlistedPlanPaths returns the sorted file paths that the plan body (the
// text after the frontmatter) names but that no work_chunks[].key_file_paths
// entry covers. It returns nil when the plan declares no work_chunks: such a
// plan has no per-bead file lists to compare against.
func UnlistedPlanPaths(content string, chunks []WorkChunk) []string {
	if len(chunks) == 0 {
		return nil
	}
	body := content
	if _, off, ok := frontmatter.Parse([]byte(content)); ok {
		body = content[off:]
	}
	var entries []string
	for _, c := range chunks {
		entries = append(entries, c.KeyFilePaths...)
	}
	var missing []string
	for _, p := range ExtractPlanFilePaths(body) {
		covered := false
		for _, e := range entries {
			if keyFilePathCovers(e, p) {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, p)
		}
	}
	return missing
}

// PlanPathsWarningName is the issue name of the unlisted-plan-path warning.
const PlanPathsWarningName = "plan-paths-unlisted"

// UnlistedPlanPathsMessage renders the warning text for paths. Each path is
// escaped because it comes from agent-writable plan text.
func UnlistedPlanPathsMessage(paths []string) string {
	esc := make([]string, len(paths))
	for i, p := range paths {
		esc[i] = termsafe.Escape(p)
	}
	return fmt.Sprintf("the plan text names %d file path(s) that are in no bead's key_file_paths: %s. "+
		"A bead gets only its key_file_paths as its file list, so its implementer may not change a file that is not listed. "+
		"Add each path to the key_file_paths of the bead that creates or edits it, or remove the path from the plan text. "+
		"If no bead changes the file, you can ignore this warning.",
		len(paths), strings.Join(esc, ", "))
}

// checkUnlistedPlanPaths adds one advisory warning (never an error) that
// lists the plan-body file paths absent from every bead's key_file_paths.
func checkUnlistedPlanPaths(r *Result, content string, chunks []WorkChunk) {
	if missing := UnlistedPlanPaths(content, chunks); len(missing) > 0 {
		r.AddWarning(PlanPathsWarningName, UnlistedPlanPathsMessage(missing))
	}
}
