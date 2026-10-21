package validate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// The unlisted-plan-path rule prefers precision over recall. A plan names
// many files only as context (files it reads, cites or must leave alone),
// so a path counts only when the words next to it say to change it. Each
// sentence becomes a list of tokens (words, code spans, paths and clause
// breaks), so a word inside a file name is never a verb or a marker. A
// clause ends at ";", ":", a dash or ", which/so/but/while/because". A
// path is an edit target when no negation comes before it in its clause
// and one of these is true:
//   - an edit verb comes before it with at most three words between, and
//     no stop word ("of", "from", "for", ...) or read-only marker between
//     (other paths, code spans and "new", "own", "file" are not counted);
//   - it is the subject of an edit ("a.go gains a field"), or it follows
//     "In" at the start of an imperative clause ("In a.go, add X");
//   - it has a "(new)" or "new file" tag;
//   - its clause uses passive edit phrasing ("is updated") and has no
//     read-only marker, or it is in a table row whose clause has an edit
//     verb and no marker;
//   - it leads a list item under a "Files" heading or label, or it is in
//     the File column of a table, and no marker is in the item or row;
//   - it starts a unit as a label ("a.go: add the helper") and the clause
//     after the label is an edit instruction;
//   - it starts the clause after "Add two files:".
// Paths in fenced code blocks, paths with a line or anchor reference
// (citations), globs, placeholders and conventional example paths never
// count. A template path whose file name has "<name>" placeholders is
// compared with key_file_paths as a glob.

// planPathTokenSplit splits a sentence into raw candidate tokens. Only
// whitespace, backticks and double quotes separate tokens here, so a glob
// or placeholder token stays whole until it is rejected.
var planPathTokenSplit = regexp.MustCompile("[\\s`\"]+")

// planPathSubSplit splits an accepted raw token on list punctuation.
var planPathSubSplit = regexp.MustCompile(`[,;|()]+`)

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
var planFileExts = wordSet(
	"go", "mod", "sum", "md", "txt", "yaml", "yml", "json", "jsonl", "toml",
	"js", "mjs", "cjs", "ts", "tsx", "jsx", "py", "sh", "bash", "rb", "rs",
	"java", "kt", "c", "h", "cc", "cpp", "swift", "proto", "sql", "html",
	"css", "tmpl", "tpl", "cfg", "ini", "xml", "csv", "svg", "lock",
)

// planPathRootFiles are file names without an extension. As a directory
// segment ("Makefile/go.mod") they mark prose for "one or the other".
var planPathRootFiles = wordSet(
	"Makefile", "Dockerfile", "Gemfile", "Rakefile", "Procfile", "Justfile",
	"Vagrantfile", "README", "LICENSE", "CHANGELOG",
)

// planPathPlaceholderStems are segment names (before any extension) that
// mark a conventional example path such as internal/foo/bar.go or x.go.
var planPathPlaceholderStems = wordSet("foo", "bar", "baz", "qux", "example", "x", "y", "z")

// planPathToolDirs are tool-state trees that a plan names for context but
// that no bead lists as a file it edits.
var planPathToolDirs = []string{".mindspec/", ".beads/"}

// planEditVerbs mark a request to change a file. Generic verbs that
// mostly mean something else in plans ("write the result to", "port the
// logic from", "probes touch these files") are not in the list.
var planEditVerbs = wordSet(
	"create", "creates", "add", "adds", "rewrite", "rewrites",
	"edit", "edits", "modify", "modifies", "update", "updates",
	"change", "changes", "extend", "extends", "rename", "renames",
	"delete", "deletes", "remove", "removes", "move", "moves",
	"implement", "implements", "refactor", "refactors",
	"replace", "replaces", "fix", "fixes", "introduce", "introduces",
	"wire", "wires", "append", "appends", "insert", "inserts",
	"extract", "extracts", "rework", "reworks",
	"rewire", "rewires", "restructure", "restructures",
	"harden", "hardens", "regenerate", "regenerates",
	"register", "registers", "make", "makes",
)

// planArticles before a verb word make it a noun: "the change in a.go".
var planArticles = wordSet("the", "a", "an", "this", "that", "each", "any", "its")

// planSubjectEdits, directly after a path, say the file changes: "a.go
// gains a field".
var planSubjectEdits = wordSet("gains", "gain", "gets", "grows", "learns", "loses")

// planEditParticiples follow "is", "are", "be" or "gets" in passive edit
// phrasing ("a.go is updated"). Before a path they narrate earlier work
// ("fix round 1 added a.go"), so they end the search for a verb.
var planEditParticiples = wordSet(
	"created", "added", "written", "rewritten", "edited", "modified",
	"updated", "changed", "extended", "renamed", "deleted", "removed",
	"moved", "implemented", "refactored", "replaced", "fixed", "introduced",
	"wired", "extracted",
)

// planNegations cancel an edit: before a verb they cancel the verb, and
// before a path in its clause they cancel every edit signal for the path.
var planNegations = wordSet(
	"not", "never", "no", "don't", "doesn't", "didn't", "won't", "cannot",
	"can't", "shouldn't", "mustn't",
)

// planReadOnlyWords mark a clause that names a file as context: a
// negation, a read, or an example. A word matches whole; a hyphenated
// word is one word, so "no-op" or "same-file" is not a marker. Any word
// that ends in "-unchanged" is a marker too.
var planReadOnlyWords = wordSet(
	"not", "don't", "doesn't", "never", "unchanged", "untouched",
	"leave", "leaves", "left", "keep", "keeps", "read", "reads", "reading",
	"inspect", "inspects", "inspection", "unrelated", "already", "example",
	"examples", "mirror", "mirrors", "mirroring", "pattern", "precedent",
	"see", "cf", "e.g", "neither", "nor", "unless", "would", "grep",
	"checked", "fixture", "fixtures", "match", "matches", "matching",
	"nothing", "throwaway", "follow-up",
)

// planAdjacencyStops end the search back from a path to its edit verb:
// a word that makes the path a reference ("the caller of", "tests for",
// "next to the one in", "a link to") rather than the object of the verb.
var planAdjacencyStops = wordSet(
	"of", "from", "for", "as", "on", "next", "beside", "alongside", "via",
	"using", "per", "following", "like", "after", "than", "by", "around",
	"against", "that", "which", "whose", "where", "calls", "called", "uses",
	"such", "sure", "call", "link", "pointer", "reference", "import",
)

// planFreeWords describe the file itself ("its own new test file"), so
// they do not count toward the adjacency limit.
var planFreeWords = wordSet("new", "own", "file", "files")

// planAdjacencyWords is the most words allowed between an edit verb and
// its path ("add a test in a_test.go").
const planAdjacencyWords = 3

// planScanTokens is the most tokens that the search for a path's verb
// reads, so a long run of paths keeps the check linear.
const planScanTokens = 64

// planTokRe finds the tokens of a sentence. The groups are: 1-2 a link's
// text and target, 3 a code span, 4 a clause break, 5 a bare path, and 6
// a word.
var planTokRe = regexp.MustCompile(`\[([^\]\n]*)\]\(([^)\s]*)\)` +
	"|`([^`]*)`" +
	`|([;:](?:\s|$)|\s*[\x{2014}\x{2013}]\s*|\s--\s|,\s+(?:which|so|but|while|because)\b)` +
	"|([^\\s\"()`]*/[^\\s\"()`]*[^\\s\"()`.,;:!?])" +
	`|((?i:e\.g|i\.e)\b|[A-Za-z][A-Za-z'\x{2019}-]*)`)

// planLinkWordRe finds the words of a link's text.
var planLinkWordRe = regexp.MustCompile(`[A-Za-z][A-Za-z'-]*`)

// planNewAfterRe and planNewBeforeRe find a "(new)" tag after a path and
// a "new file" or "**NEW**" tag before it.
var (
	planNewAfterRe  = regexp.MustCompile(`(?i)^[*_]*\s*\(\s*new\b`)
	planNewBeforeRe = regexp.MustCompile(`(?i)\bnew\b[*_]*\s*(?:test\s+)?(?:files?\b)?[*_\s:,]*$`)
)

// planTagWindow is how many bytes before and after a path the "new" tag
// search reads. A tag is at most about 30 bytes, and the bound keeps the
// check linear in the length of a sentence.
const planTagWindow = 64

// planNewTag reports whether a "new" tag stands next to the path at
// s[start:end]. The windows are cut on UTF-8 boundaries.
func planNewTag(s string, start, end int) bool {
	lo, hi := max(0, start-planTagWindow), min(len(s), end+planTagWindow)
	for lo < start && !utf8.RuneStart(s[lo]) {
		lo++
	}
	for hi < len(s) && hi > end && !utf8.RuneStart(s[hi]) {
		hi--
	}
	return planNewAfterRe.MatchString(s[end:hi]) || planNewBeforeRe.MatchString(s[lo:start])
}

// planFilesLabel is a "Files" label: "Files:", "Changed files", "Key
// files", "Files touched", "Files to create/modify".
const planFilesLabel = `(?:key |changed |new |modified )?files?(?: touched| changed| to (?:create|modify|touch|change|edit|add|update|write)(?:\s*/\s*(?:create|modify|touch|change|edit|add|update|write))*)?`

// planFilesHeadingRe matches a line that is only a Files label or heading.
var planFilesHeadingRe = regexp.MustCompile(`(?i)^\s*(?:#+\s*|[-*+]\s+)?[*_]*` + planFilesLabel + `[*_]*\s*:?\s*[*_]*\s*$`)

// planFilesInlineRe matches a Files label at the start of a unit that has
// more text after it.
var planFilesInlineRe = regexp.MustCompile(`(?i)^\s*(?:[-*+]\s+|\d+[.)]\s+)?[*_]*` + planFilesLabel + `[*_]*\s*:\s*[*_]*`)

// planSentenceEnd ends a sentence: ".", "!" or "?" before whitespace.
var planSentenceEnd = regexp.MustCompile(`[.!?]+\s+`)

// planUnitStart marks a line that starts a new unit: a list item, a
// heading, a table row or a block quote.
var planUnitStart = regexp.MustCompile(`^\s*(?:[-*+]\s|\d+[.)]\s|#|\||>)`)

// planListItemRe matches a list item line.
var planListItemRe = regexp.MustCompile(`^\s*(?:[-*+]\s|\d+[.)]\s)`)

func wordSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// planUnit is a list item, heading, table row or paragraph of a plan body.
// filesCtx is set for a list item under a Files heading or label;
// fileCell holds the File column cell of a table row.
type planUnit struct {
	text     string
	filesCtx bool
	fileCell string
}

// planFenceRun returns the fence character and run length when trimmed
// opens or closes a fenced code block.
func planFenceRun(trimmed string) (byte, int) {
	if len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, 0
	}
	c := trimmed[0]
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return c, n
}

// planTableCells splits a table row into trimmed cells.
func planTableCells(row string) []string {
	row = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(row), "|"), "|")
	cells := strings.Split(row, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// planBodyUnits splits a markdown body into units. It skips fenced code
// blocks completely. A fence closes only on a run of the same character
// that is at least as long as the opening run. A list item gets filesCtx
// when it is under a Files heading or label, or nested under a line that
// ends in ":" and asks for an edit ("Add these tests:").
func planBodyUnits(body string) []planUnit {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	const noCtx = -2
	var units []planUnit
	var cur []string
	curFiles := false
	curIndent := 0
	ctxIndent := noCtx // list items indented more than this get filesCtx
	flush := func() {
		if len(cur) == 0 {
			return
		}
		text := strings.Join(cur, " ")
		units = append(units, planUnit{text: text, filesCtx: curFiles})
		cur = nil
		curFiles = false
		if planIntroducesEdits(text) {
			if planListItemRe.MatchString(text) {
				ctxIndent = curIndent
			} else {
				ctxIndent = curIndent - 1
			}
		}
	}
	var fenceChar byte
	fenceLen := 0
	tableFileCol := -1
	prevTable := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimLeft(line, " \t>")
		if fenceLen > 0 {
			if c, n := planFenceRun(trimmed); c == fenceChar && n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
				fenceLen = 0
			}
			continue
		}
		if c, n := planFenceRun(trimmed); n > 0 {
			flush()
			fenceChar, fenceLen = c, n
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
			prevTable = false
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			flush()
			ctxIndent = noCtx
			cells := planTableCells(line)
			u := planUnit{text: line}
			if !prevTable {
				tableFileCol = -1
				for i, c := range cells {
					switch strings.ToLower(strings.Trim(c, "*_ ")) {
					case "file", "files", "file(s)", "path", "paths":
						tableFileCol = i
					}
				}
			} else if tableFileCol >= 0 && tableFileCol < len(cells) {
				u.fileCell = cells[tableFileCol]
			}
			units = append(units, u)
			prevTable = true
			continue
		}
		prevTable = false
		if planFilesHeadingRe.MatchString(line) {
			flush()
			ctxIndent = -1
			continue
		}
		isItem := planListItemRe.MatchString(line)
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			flush()
			ctxIndent = noCtx
		} else if isItem && indent <= ctxIndent {
			ctxIndent = noCtx // a sibling or parent item ends the list
		} else if !isItem && len(cur) == 0 && indent <= ctxIndent {
			ctxIndent = noCtx // a paragraph line ends the list
		}
		if planUnitStart.MatchString(line) {
			flush()
			curFiles = isItem && ctxIndent != noCtx && indent > ctxIndent
			curIndent = indent
		} else if len(cur) == 0 {
			curIndent = indent
		}
		cur = append(cur, line)
	}
	flush()
	return units
}

// planIntroducesEdits reports whether a unit ends in ":" and its last
// clause asks for an edit, so the list nested under it names edit
// targets.
func planIntroducesEdits(text string) bool {
	t := strings.TrimRight(text, " \t*_")
	if !strings.HasSuffix(t, ":") {
		return false
	}
	sents := planSentences(t)
	if len(sents) == 0 {
		return false
	}
	clauses := planClauses(sents[len(sents)-1])
	last := clauses[len(clauses)-1]
	if len(last.toks) == 0 && len(clauses) > 1 {
		last = clauses[len(clauses)-2]
	}
	return last.verb && !last.marker
}

// planSentences splits a unit's text into sentences.
func planSentences(u string) []string {
	// A list marker such as "2." must not end a sentence.
	u = planListItemRe.ReplaceAllString(u, "")
	// Keep abbreviations from ending a sentence.
	u = strings.NewReplacer("e.g.", "e.g", "i.e.", "i.e", "etc.", "etc", "vs.", "vs").Replace(u)
	var out []string
	for _, s := range planSentenceEnd.Split(u, -1) {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// planTok is one token of a clause: a lower-case word, or a code span,
// link target or bare path (span set). paths holds the file paths of a
// span; a span with no paths, such as an identifier, counts as a word.
type planTok struct {
	word   string
	span   bool
	paths  []planPath
	newTag bool
}

// planClause is a clause of a sentence and the break text that ends it.
type planClause struct {
	toks    []planTok
	brk     string
	marker  bool // a read-only word or phrase, or a negated edit verb
	verb    bool // an edit verb that is not negated
	passive bool // passive edit phrasing such as "is updated"
	words   int
}

// planClauses splits a sentence into clauses of tokens. The tokens hold
// the paths as data, so no text in the sentence can stand in for a path.
func planClauses(s string) []planClause {
	var out []planClause
	var cur planClause
	for _, m := range planTokRe.FindAllStringSubmatchIndex(s, -1) {
		group := func(g int) (string, bool) {
			if m[2*g] < 0 {
				return "", false
			}
			return s[m[2*g]:m[2*g+1]], true
		}
		raw, isSpan := group(3)
		if target, ok := group(2); ok {
			// Link text that is one token, such as a file name, is not prose.
			if text, _ := group(1); strings.ContainsAny(strings.TrimSpace(text), " \t") {
				for _, w := range planLinkWordRe.FindAllString(text, -1) {
					cur.toks = append(cur.toks, planTok{word: strings.ToLower(w)})
				}
			}
			raw, isSpan = target, true
		} else if p, ok := group(5); ok {
			raw, isSpan = p, true
		}
		switch {
		case isSpan:
			paths := planTokenPaths(raw)
			cur.toks = append(cur.toks, planTok{span: true, paths: paths,
				newTag: len(paths) > 0 && planNewTag(s, m[0], m[1])})
		case m[8] >= 0:
			cur.brk = s[m[8]:m[9]]
			out = append(out, cur)
			cur = planClause{}
		default:
			w := strings.Trim(strings.ReplaceAll(strings.ToLower(s[m[0]:m[1]]), "\u2019", "'"), "-'")
			cur.toks = append(cur.toks, planTok{word: w})
		}
	}
	out = append(out, cur)
	for i := range out {
		out[i].classify()
	}
	return out
}

// classify sets the word checks of a clause. A negated edit verb counts as
// a read-only marker.
func (c *planClause) classify() {
	words := []string{}
	for j, t := range c.toks {
		if t.span {
			continue
		}
		c.words++
		words = append(words, t.word)
		if planReadOnlyWords[t.word] || strings.HasSuffix(t.word, "-unchanged") {
			c.marker = true
		}
		if planIsVerb(c.toks, j) {
			if planVerbNegated(c.toks, j) {
				c.marker = true
			} else {
				c.verb = true
			}
		}
		if planEditParticiples[t.word] {
			switch planWordAt(c.toks, j-1) {
			case "is", "are", "be", "gets", "newly", "also", "now":
				c.passive = true
			}
		}
	}
	joined := " " + strings.Join(words, " ") + " "
	if strings.Contains(joined, " such as ") || strings.Contains(joined, " out of scope ") {
		c.marker = true
	}
}

// planWordAt returns the word at toks[k], or "" for a span or an index out
// of range.
func planWordAt(toks []planTok, k int) string {
	if k < 0 || k >= len(toks) {
		return ""
	}
	return toks[k].word
}

// planIsVerb reports whether toks[j] is an edit verb and not a noun.
func planIsVerb(toks []planTok, j int) bool {
	return planEditVerbs[planWordAt(toks, j)] && !planArticles[planWordAt(toks, j-1)]
}

// planVerbNegated reports whether the edit verb at toks[j] is directly
// negated: "not change", "never edit", "no need to update".
func planVerbNegated(toks []planTok, j int) bool {
	return planNegations[planWordAt(toks, j-1)] ||
		planWordAt(toks, j-1) == "to" && planWordAt(toks, j-2) == "need" && planWordAt(toks, j-3) == "no"
}

// planIsPath reports whether toks[k] is a span that holds a file path.
func planIsPath(toks []planTok, k int) bool {
	return k >= 0 && k < len(toks) && len(toks[k].paths) > 0
}

// planVerbBefore reports whether an edit verb comes shortly before the
// path at toks[i], with no stop word or marker between. Code spans and
// other paths between ("Edit a.go, b.go and c.go") do not count.
func planVerbBefore(toks []planTok, i int) bool {
	n := 0
	for j := i - 1; j >= max(0, i-planScanTokens) && n <= planAdjacencyWords; j-- {
		w := toks[j].word
		switch {
		case toks[j].span || planFreeWords[w]:
			continue
		case planIsVerb(toks, j):
			return !planVerbNegated(toks, j)
		case planReadOnlyWords[w] || planAdjacencyStops[w] || planEditParticiples[w]:
			return false
		}
		n++
	}
	return false
}

// planVerbAfter reports whether the path at toks[i] is the subject of an
// edit ("a.go gains a field") or follows "In" at the start of an
// imperative clause ("In a.go, add the helper").
func planVerbAfter(toks []planTok, i int) bool {
	k := i + 1
	for k <= i+planScanTokens && (planIsPath(toks, k) || planWordAt(toks, k) == "and") {
		k++
	}
	if planSubjectEdits[planWordAt(toks, k)] {
		return true
	}
	if i > planScanTokens {
		return false
	}
	for j := 1; j < i; j++ {
		if !planIsPath(toks, j) && planWordAt(toks, j) != "and" {
			return false
		}
	}
	switch planWordAt(toks, 0) {
	case "in", "inside", "within":
		return planIsVerb(toks, k) && !planVerbNegated(toks, k)
	}
	return false
}

// planSentenceEditPaths returns the paths that sentence s names in an
// edit context. filesCtx marks the sentence as the lead of a list item
// under a Files heading or label; tableRow marks a table row, where an
// edit verb may follow the path ("| a.go | update the parser |").
func planSentenceEditPaths(s string, filesCtx, tableRow, unitStart bool) []planPath {
	if loc := planFilesInlineRe.FindStringIndex(s); loc != nil {
		s = s[loc[1]:]
		filesCtx = true
	}
	clauses := planClauses(s)
	sentenceMarker := false
	for _, c := range clauses {
		sentenceMarker = sentenceMarker || c.marker
	}
	var out []planPath
	for ci, c := range clauses {
		startsWithPath := planIsPath(c.toks, 0)
		// A unit that opens with a path label names the file it is about
		// when the next clause asks for an edit: "- a.go: add the helper".
		brk := strings.TrimSpace(c.brk)
		label := ci == 0 && unitStart && startsWithPath && !c.marker && c.words <= 3 &&
			(brk == ":" || brk == "--" || brk == "\u2014" || brk == "\u2013") &&
			clauses[ci+1].verb && !clauses[ci+1].marker
		// "Add two files: a.go and b.go" carries the verb past the colon.
		inherit := ci > 0 && strings.HasPrefix(clauses[ci-1].brk, ":") && clauses[ci-1].verb && !clauses[ci-1].marker &&
			startsWithPath && !c.verb && !c.marker && c.words <= 3
		lead := filesCtx && ci == 0 && !sentenceMarker
		clauseEdit := !c.marker && (c.passive || tableRow && c.verb)
		negated := false
		for i, t := range c.toks {
			if planNegations[t.word] {
				negated = true
			}
			if len(t.paths) == 0 || negated {
				continue
			}
			if label || inherit || lead || clauseEdit || t.newTag || planVerbBefore(c.toks, i) || planVerbAfter(c.toks, i) {
				out = append(out, t.paths...)
			}
		}
	}
	return out
}

// planPath is a path named in the plan. A template path ("a/b_<name>.go")
// also has the glob ("a/b_*.go") and a concrete sample of it.
type planPath struct {
	path   string
	glob   string
	sample string
}

// planTemplateRe matches one "<name>" placeholder of a template path.
var planTemplateRe = regexp.MustCompile(`<[A-Za-z0-9_-]+>`)

// planTemplateSentinel stands in for a placeholder while a template path is
// checked as an ordinary path.
const planTemplateSentinel = "qqtmplqq"

// planTokenPaths returns the file paths in a raw token (a code span or a
// whitespace-delimited word), without citations, globs and placeholders.
func planTokenPaths(raw string) []planPath {
	var out []planPath
	for _, part := range planPathTokenSplit.Split(raw, -1) {
		tok := trimPlanPathEdges(part)
		if tok == "" || strings.Contains(tok, "://") {
			continue
		}
		if strings.Contains(tok, "<") {
			if p, ok := planTemplatePath(tok); ok {
				out = append(out, p)
			}
			continue
		}
		if strings.ContainsAny(tok, planPathGlobChars) {
			continue
		}
		for _, piece := range planPathSubSplit.Split(tok, -1) {
			if p, ok := normalizePlanPathToken(piece); ok {
				out = append(out, planPath{path: p})
			}
		}
	}
	return out
}

// planTemplatePath accepts a token whose only placeholders are "<name>"
// parts of its file name, such as internal/a/adr_<id>_test.go. A
// placeholder in a directory, a file name that is only placeholders, or
// any other glob character rejects the token.
func planTemplatePath(tok string) (planPath, bool) {
	names := planTemplateRe.FindAllString(tok, -1)
	conc := planTemplateRe.ReplaceAllString(tok, planTemplateSentinel)
	if len(names) == 0 || strings.ContainsAny(conc, planPathGlobChars) || strings.ContainsAny(conc, ",;|()") {
		return planPath{}, false
	}
	p, ok := normalizePlanPathToken(conc)
	if !ok {
		return planPath{}, false
	}
	dir, file := "", p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		dir, file = p[:i], p[i+1:]
	}
	stem := file
	if dot := strings.LastIndex(stem, "."); dot > 0 {
		stem = stem[:dot]
	}
	if strings.Contains(dir, planTemplateSentinel) || strings.Trim(strings.ReplaceAll(stem, planTemplateSentinel, ""), "_-.") == "" {
		return planPath{}, false
	}
	display := p
	for _, n := range names {
		display = strings.Replace(display, planTemplateSentinel, n, 1)
	}
	return planPath{path: display, glob: strings.ReplaceAll(p, planTemplateSentinel, "*"), sample: p}, true
}

// editContextPlanPaths returns the paths, sorted and without duplicates,
// that body names in an edit context.
func editContextPlanPaths(body string) []planPath {
	seen := map[string]bool{}
	var out []planPath
	add := func(ps []planPath) {
		for _, p := range ps {
			if !seen[p.path] {
				seen[p.path] = true
				out = append(out, p)
			}
		}
	}
	for _, u := range planBodyUnits(body) {
		for i, s := range planSentences(u.text) {
			add(planSentenceEditPaths(s, u.filesCtx && i == 0, strings.HasPrefix(strings.TrimSpace(u.text), "|"), i == 0))
		}
		if u.fileCell == "" {
			continue
		}
		marker := false
		for _, c := range planClauses(u.text) {
			marker = marker || c.marker
		}
		if !marker {
			add(planTokenPaths(u.fileCell))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

// planPathCovered reports whether some key_file_paths entry covers p. A
// template path is covered by an entry that covers its sample, by an entry
// equal to its glob, or by a concrete entry that its glob matches.
func planPathCovered(entries []string, p planPath) bool {
	for _, e := range entries {
		if p.glob == "" {
			if keyFilePathCovers(e, p.path) {
				return true
			}
			continue
		}
		et := strings.TrimPrefix(strings.TrimSpace(e), "./")
		if et == p.glob || keyFilePathCovers(e, p.sample) {
			return true
		}
		if et != "" && !strings.ContainsAny(et, "*?[") && GlobMatch(p.glob, et) {
			return true
		}
	}
	return false
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
		if !planPathCovered(entries, p) {
			missing = append(missing, p.path)
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

// planPathsSkippedMessage is the warning text when the check fails on the
// plan text. The check is advisory, so a failure must not stop the verb.
const planPathsSkippedMessage = "the check for plan file paths missing from key_file_paths was skipped: it could not read the plan text."

// checkUnlistedPlanPaths adds one advisory warning (never an error) that
// lists the plan-body file paths absent from every bead's key_file_paths.
// A panic in the check becomes a single warning that the check was
// skipped.
func checkUnlistedPlanPaths(r *Result, content string, chunks []WorkChunk) {
	checkUnlistedPlanPathsWith(r, func() []string { return UnlistedPlanPaths(content, chunks) })
}

// checkUnlistedPlanPathsWith runs find and adds the warning. A panic in
// find never reaches the caller: it becomes one warning that the check
// was skipped.
func checkUnlistedPlanPathsWith(r *Result, find func() []string) {
	defer func() {
		if recover() != nil {
			r.AddWarning(PlanPathsWarningName, planPathsSkippedMessage)
		}
	}()
	if missing := find(); len(missing) > 0 {
		r.AddWarning(PlanPathsWarningName, UnlistedPlanPathsMessage(missing))
	}
}
