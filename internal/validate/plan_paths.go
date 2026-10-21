package validate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// The unlisted-plan-path rule prefers precision over recall. A plan names
// many files only as context (files it reads, cites or must leave alone),
// so a path counts only when its sentence or list item says to change it:
// the unit has an edit verb and no negation or read-only marker. Paths in
// fenced code blocks, paths with a line or anchor reference (citations),
// globs, placeholders and conventional example paths never count.

// planPathTokenSplit splits a sentence into raw candidate tokens. Only
// whitespace, backticks and double quotes separate tokens here, so a glob
// or placeholder token stays whole until it is rejected.
var planPathTokenSplit = regexp.MustCompile("[\\s`\"]+")

// planPathSubSplit splits an accepted raw token on list punctuation.
var planPathSubSplit = regexp.MustCompile(`[,;|()]+`)

// planMarkdownLinkRe matches a markdown link "[text](target)".
var planMarkdownLinkRe = regexp.MustCompile(`\[([^\]\n]*)\]\(([^)\s]*)\)`)

// planPathPrefixRe matches a "label:path" token such as "file:internal/a.go".
var planPathPrefixRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*:(.+)$`)

// planFilePathRe matches a repo-relative file path: two or more segments
// separated by "/", with a final segment that ends in a file extension.
var planFilePathRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+\.([A-Za-z0-9]+)$`)

// planPathGlobChars mark a glob or placeholder token. The whole token is
// rejected, so no suffix of it can count as a path.
const planPathGlobChars = "[]{}*?<>$=@~\\"

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

// planPathRootFiles are file names without an extension. As a directory
// segment ("Makefile/go.mod") they mark prose for "one or the other".
var planPathRootFiles = map[string]bool{
	"Makefile": true, "Dockerfile": true, "Gemfile": true, "Rakefile": true,
	"Procfile": true, "Justfile": true, "Vagrantfile": true,
	"README": true, "LICENSE": true, "CHANGELOG": true,
}

// planPathPlaceholderStems are segment names (before any extension) that
// mark a conventional example path such as internal/foo/bar.go or x.go.
var planPathPlaceholderStems = map[string]bool{
	"foo": true, "bar": true, "baz": true, "qux": true,
	"example": true, "x": true, "y": true, "z": true,
}

// planPathToolDirs are tool-state trees that a plan names for context but
// that no bead lists as a file it edits.
var planPathToolDirs = []string{".mindspec/", ".beads/"}

// planEditVerbs mark a unit that asks for a file change. Words match
// whole and case-insensitively.
var planEditVerbs = wordSet(
	"create", "creates", "creating",
	"add", "adds", "adding",
	"write", "writes", "edit", "edits", "editing",
	"modify", "modifies", "update", "updates",
	"change", "changes", "extend", "extends",
	"rename", "renames", "delete", "deletes",
	"remove", "removes", "move", "moves",
)

// planReadOnlyWords mark a unit that names a file as context: a negation,
// a read, or an example.
var planReadOnlyWords = wordSet(
	"not", "don't", "doesn't", "never", "no", "unchanged", "untouched",
	"leave", "leaves", "left", "keep", "keeps", "read", "reads", "inspect",
	"inspects", "inspection", "unrelated", "already", "example", "examples",
	"like", "mirror", "mirrors", "pattern", "precedent", "see", "cf",
	"existing", "today", "currently", "eg", "neither", "nor", "unless",
	"would", "same", "matching", "match", "matches", "grep", "checked",
	"fixture", "fixtures",
)

// planEditPhrases are edit markers that are not single words. A bare
// "new" is too common in prose to count; "(new)" after a path does.
var planEditPhrases = regexp.MustCompile(`(?i)\(\s*new\s*\)`)

// planReadOnlyPhrases are read-only markers that are not single words.
var planReadOnlyPhrases = regexp.MustCompile(`(?i)\be\.g\b|\bsuch as\b|\bout of scope\b|\bfollow-up\b`)

// planWordRe finds words for the verb and marker checks. A hyphenated
// word is one word for the verb check ("genuinely-new" is not "new"), and
// each of its parts counts for the marker check ("byte-unchanged").
var planWordRe = regexp.MustCompile("[A-Za-z][A-Za-z'\u2019-]*")

// planSentenceEnd ends a sentence: ".", "!" or "?" before whitespace.
var planSentenceEnd = regexp.MustCompile(`[.!?]+\s+`)

// planUnitStart marks a line that starts a new unit: a list item, a
// heading, a table row or a block quote.
var planUnitStart = regexp.MustCompile(`^\s*(?:[-*+]\s|\d+[.)]\s|#|\||>)`)

func wordSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// planBodyUnits splits a markdown body into sentences. It skips fenced
// code blocks completely. A list item, heading, table row or paragraph is
// a unit, and each unit is split into sentences.
func planBodyUnits(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	var units []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			units = append(units, strings.Join(cur, " "))
			cur = nil
		}
	}
	fence := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimLeft(line, " \t>")
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			flush()
			fence = trimmed[:3]
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if planUnitStart.MatchString(line) {
			flush()
		}
		cur = append(cur, line)
	}
	flush()

	var sentences []string
	for _, u := range units {
		// Keep abbreviations from ending a sentence.
		u = strings.NewReplacer("e.g.", "e.g", "i.e.", "i.e", "etc.", "etc", "vs.", "vs").Replace(u)
		for _, s := range planSentenceEnd.Split(u, -1) {
			if strings.TrimSpace(s) != "" {
				sentences = append(sentences, s)
			}
		}
	}
	return sentences
}

// planSentenceIsEdit reports whether a sentence asks for a file change: it
// has an edit verb and no negation or read-only marker.
func planSentenceIsEdit(s string) bool {
	if planReadOnlyPhrases.MatchString(s) {
		return false
	}
	edit := planEditPhrases.MatchString(s)
	for _, w := range planWordRe.FindAllString(strings.ToLower(s), -1) {
		w = strings.Trim(strings.ReplaceAll(w, "\u2019", "'"), "-'")
		for _, part := range strings.Split(w, "-") {
			if planReadOnlyWords[part] {
				return false
			}
		}
		if planEditVerbs[w] {
			edit = true
		}
	}
	return edit
}

// planSentencePaths returns the file paths a sentence names, without the
// ones that are citations (a line or anchor reference).
func planSentencePaths(s string) []string {
	s = planMarkdownLinkRe.ReplaceAllString(s, " $1 $2 ")
	var out []string
	for _, raw := range planPathTokenSplit.Split(s, -1) {
		tok := trimPlanPathEdges(raw)
		if tok == "" || strings.Contains(tok, "://") || strings.ContainsAny(tok, planPathGlobChars) {
			continue
		}
		for _, piece := range planPathSubSplit.Split(tok, -1) {
			if p, ok := normalizePlanPathToken(piece); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

// trimPlanPathEdges removes wrapping punctuation and balanced markdown
// emphasis ("**", "__", "*", "_") from both ends of a token.
func trimPlanPathEdges(tok string) string {
	for {
		before := tok
		tok = strings.Trim(tok, "()'\u2018\u2019,;|")
		tok = strings.TrimRight(tok, ".!?:")
		for _, em := range []string{"**", "__", "*", "_"} {
			if len(tok) > 2*len(em) && strings.HasPrefix(tok, em) && strings.HasSuffix(tok, em) {
				tok = tok[len(em) : len(tok)-len(em)]
				break
			}
		}
		if tok == before {
			return tok
		}
	}
}

// normalizePlanPathToken returns the repo-relative file path in tok, or
// false when tok is not one or is a citation, an absolute path, a
// placeholder or a tool-state path.
func normalizePlanPathToken(tok string) (string, bool) {
	if m := planPathPrefixRe.FindStringSubmatch(tok); m != nil {
		tok = m[1] // "file:internal/a.go"; "C:/x" becomes absolute below
	}
	if i := strings.IndexAny(tok, ":#"); i >= 0 {
		if len(tok) > i+1 {
			return "", false // "a.go:41", "a.go#L3", "a.md#anchor": a citation
		}
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
		lower := strings.ToLower(seg)
		stem := lower
		if dot := strings.Index(lower, "."); dot > 0 {
			stem = lower[:dot]
		}
		if planPathPlaceholderStems[stem] {
			return "", false
		}
		if lower == "path" && i+1 < len(segs) && strings.ToLower(segs[i+1]) == "to" {
			return "", false
		}
		if i < len(segs)-1 {
			// A directory segment that is a file name ("spec.md/plan.md",
			// "Makefile/go.mod") is prose for "one or the other".
			if planPathRootFiles[seg] {
				return "", false
			}
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

// editContextPlanPaths returns the sorted, deduplicated file paths that
// body names in an edit context.
func editContextPlanPaths(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range planBodyUnits(body) {
		if !planSentenceIsEdit(s) {
			continue
		}
		for _, p := range planSentencePaths(s) {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
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
// text after the frontmatter) names in an edit context but that no
// work_chunks[].key_file_paths entry covers. It returns nil when the plan
// declares no work_chunks: such a plan has no per-bead file lists to
// compare against.
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
	for _, p := range editContextPlanPaths(body) {
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
	return fmt.Sprintf("the plan says to create or change %d file(s) that are in no bead's key_file_paths: %s. "+
		"A bead's implementer gets only its key_file_paths as its file list. "+
		"Add each path to the key_file_paths of the bead that changes it.",
		len(paths), strings.Join(esc, ", "))
}

// checkUnlistedPlanPaths adds one advisory warning (never an error) that
// lists the plan-body file paths absent from every bead's key_file_paths.
func checkUnlistedPlanPaths(r *Result, content string, chunks []WorkChunk) {
	if missing := UnlistedPlanPaths(content, chunks); len(missing) > 0 {
		r.AddWarning(PlanPathsWarningName, UnlistedPlanPathsMessage(missing))
	}
}
