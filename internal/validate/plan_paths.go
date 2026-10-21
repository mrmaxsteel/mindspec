package validate

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// The unlisted-plan-path rule prefers precision over recall. A plan names
// many files only as context (files it reads, cites or must leave alone),
// so a path counts only when its own clause says to change it. A sentence
// is split into clauses at ";", ":", dashes and ", which/so/but/while/
// because". Paths and code spans are masked before the word checks, so a
// word inside a file name is never a verb or a marker. A path is an edit
// target when one of these is true:
//   - an edit verb comes shortly before it in its clause, with only filler
//     words between, and no negation directly before the verb;
//   - its clause has an edit verb and no negation or read-only marker;
//   - it has a "(new)" tag;
//   - it leads a list item under a "Files" heading or label, or it is in
//     the File column of a table, and its clause has no read-only marker.
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

// planEditVerbs mark a request to change a file. Words match whole and
// case-insensitively, after paths and code spans are masked.
var planEditVerbs = wordSet(
	"create", "creates", "creating",
	"add", "adds", "adding",
	"write", "writes", "rewrite", "rewrites",
	"edit", "edits", "editing",
	"modify", "modifies", "update", "updates", "updating",
	"change", "changes", "extend", "extends",
	"rename", "renames", "delete", "deletes",
	"remove", "removes", "move", "moves",
	"implement", "implements", "refactor", "refactors",
	"replace", "replaces", "fix", "fixes",
	"introduce", "introduces", "wire", "wires", "rewire", "rewires",
	"port", "ports", "split", "splits",
	"append", "appends", "insert", "inserts", "extract", "extracts",
	"make", "makes", "touch", "touches", "restructure", "restructures",
	"rework", "reworks", "harden", "hardens", "export", "exports",
	"register", "registers", "sweep", "sweeps",
	"regenerate", "regenerates", "route", "routes",
)

// planPassiveEdit matches passive edit phrasing such as "is created" or
// "gets a new". Past-tense narration ("Bead 1 added") does not match.
var planPassiveEdit = regexp.MustCompile(`(?i)\b(?:is|are|be|gets?)\s+(?:(?:newly|also|now)\s+)?(?:created|added|written|rewritten|edited|modified|updated|changed|extended|renamed|deleted|removed|moved|implemented|refactored|replaced|fixed|introduced|wired|ported|split|extracted)\b|\bgets?\s+an?\s+new\b`)

// planNegations, directly before an edit verb, cancel it.
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
	"see", "cf", "eg", "neither", "nor", "unless", "would", "grep",
	"checked", "fixture", "fixtures", "match", "matches", "matching",
	"nothing", "throwaway",
)

// planAdjacencyStops end the search back from a path to its edit verb:
// a word that makes the path a reference ("the caller of", "next to the
// one in") rather than the object of the verb.
var planAdjacencyStops = wordSet(
	"of", "from", "next", "beside", "alongside", "via", "using", "per",
	"following", "like", "after", "than", "by", "around", "against",
	"that", "which", "whose", "where", "calls", "called", "uses", "such",
)

// planAdjacencyWords is the most filler words allowed between an edit
// verb and its path ("adds its own pin test file").
const planAdjacencyWords = 6

// planReadOnlyPhrases are read-only markers that are not single words.
var planReadOnlyPhrases = regexp.MustCompile(`(?i)\be\.g\b|\bsuch as\b|\bout of scope\b|\bfollow-up\b`)

// planSlotMark wraps the index of a masked path or code span.
const planSlotMark = "\x01"

// planSlotRe finds a masked slot.
var planSlotRe = regexp.MustCompile(planSlotMark + `(\d+)` + planSlotMark)

// planItemRe finds the words and slots of a masked clause.
var planItemRe = regexp.MustCompile(planSlotMark + `\d+` + planSlotMark + "|[A-Za-z][A-Za-z'\u2019-]*")

// planCodeSpanRe matches an inline code span.
var planCodeSpanRe = regexp.MustCompile("`[^`]*`")

// planBarePathRe matches a token outside code spans that may be a path.
var planBarePathRe = regexp.MustCompile(`[^\s"()` + "`" + `]*/[^\s"()` + "`" + `]*`)

// planNewTagRes mark a slot as a new or changed file: "x.go (new)",
// "**NEW** x.go" and "New file: x.go".
var planNewTagRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)` + planSlotMark + `(\d+)` + planSlotMark + `[*_]*\s*[*_]*\(\s*new\b`),
	regexp.MustCompile(`(?i)\(\s*new\b[^()]*?` + planSlotMark + `(\d+)` + planSlotMark),
	regexp.MustCompile(`(?i)\*\*(?:new|modified|changed|updated)\*\*\s*` + planSlotMark + `(\d+)` + planSlotMark),
	regexp.MustCompile(`(?i)\bnew\s+(?:test\s+)?files?\s*[*_]*\s*:\s*[*_]*\s*` + planSlotMark + `(\d+)` + planSlotMark),
	regexp.MustCompile(`(?i)\bnew\s+(?:test\s+)?(?:file\s+)?[*_]*\s*` + planSlotMark + `(\d+)` + planSlotMark),
}

// planClauseSplit ends a clause: ";" or ":" before a space, a dash, or
// ", which/so/but/while/because".
var planClauseSplit = regexp.MustCompile(`[;:](?:\s|$)|\s*[\x{2014}\x{2013}]\s*|\s--\s|,\s+(?:which|so|but|while|because)\b`)

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
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
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
	masked, slots := maskPlanText(sents[len(sents)-1])
	slotPaths := make([][]planPath, len(slots))
	clauses := planClauseSplit.Split(strings.TrimRight(strings.TrimSpace(masked), ":"), -1)
	c := planClauseInfo(clauses[len(clauses)-1], slotPaths)
	return c.firstVerb >= 0 && !c.hasMarker
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

// maskPlanText replaces link targets, code spans and path-like tokens with
// numbered slots, so a word inside a file name or identifier is never read
// as a verb or a marker. It returns the masked text and the slot contents.
func maskPlanText(s string) (string, []string) {
	var slots []string
	slot := func(raw string) string {
		slots = append(slots, raw)
		return " " + planSlotMark + strconv.Itoa(len(slots)-1) + planSlotMark + " "
	}
	s = planMarkdownLinkRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := planMarkdownLinkRe.FindStringSubmatch(m)
		text := sub[1]
		// Link text that is one token, such as a file name, is not prose.
		if !strings.ContainsAny(strings.TrimSpace(text), " \t") {
			text = ""
		}
		return text + slot(sub[2])
	})
	s = planCodeSpanRe.ReplaceAllStringFunc(s, func(m string) string {
		return slot(strings.Trim(m, "`"))
	})
	s = planBarePathRe.ReplaceAllStringFunc(s, func(m string) string {
		core := strings.TrimRight(m, ".,;:!?")
		return slot(core) + m[len(core):]
	})
	return s, slots
}

// planItem is a word (lower case) or a slot index of a masked clause.
type planItem struct {
	word string
	slot int
}

// planAbbrevRe matches "e.g" and "i.e", which become the words "eg" and
// "ie".
var planAbbrevRe = regexp.MustCompile(`(?i)\b(?:e\.g|i\.e)\b`)

func planClauseItems(clause string) []planItem {
	var items []planItem
	clause = planAbbrevRe.ReplaceAllStringFunc(clause, func(m string) string { return strings.ReplaceAll(m, ".", "") })
	for _, m := range planItemRe.FindAllString(clause, -1) {
		if strings.HasPrefix(m, planSlotMark) {
			n, _ := strconv.Atoi(strings.Trim(m, planSlotMark))
			items = append(items, planItem{slot: n})
			continue
		}
		w := strings.Trim(strings.ReplaceAll(strings.ToLower(m), "\u2019", "'"), "-'")
		items = append(items, planItem{word: w, slot: -1})
	}
	return items
}

func planIsMarker(w string) bool {
	return planReadOnlyWords[w] || strings.HasSuffix(w, "-unchanged")
}

// planVerbNegated reports whether the edit verb at items[j] is directly
// negated: "not change", "never edit", "no edit", "no need to update".
func planVerbNegated(items []planItem, j int) bool {
	if planNegations[planWordAt(items, j-1)] {
		return true
	}
	return planWordAt(items, j-1) == "to" && planWordAt(items, j-2) == "need" && planWordAt(items, j-3) == "no"
}

// planWordAt returns the word at items[k], or "" for a slot or an index
// out of range.
func planWordAt(items []planItem, k int) string {
	if k < 0 || k >= len(items) || items[k].slot >= 0 {
		return ""
	}
	return items[k].word
}

// planIsEditVerb reports whether items[j] is an edit verb. "fix" before
// "round" or "up" is a noun ("fix round 2").
func planIsEditVerb(items []planItem, j int) bool {
	w := planWordAt(items, j)
	if !planEditVerbs[w] {
		return false
	}
	if w == "fix" || w == "fixes" {
		switch planWordAt(items, j+1) {
		case "round", "rounds", "up", "pass", "cycle", "commit":
			return false
		}
	}
	return true
}

// planVerbBefore reports whether an edit verb comes shortly before the
// slot at items[i], with only filler words between it and the slot.
func planVerbBefore(items []planItem, i int, slotPaths [][]planPath) bool {
	n := 0
	for j := i - 1; j >= 0; j-- {
		it := items[j]
		if it.slot >= 0 {
			if len(slotPaths[it.slot]) > 0 {
				continue // a list of paths: "Edit a.go, b.go and c.go"
			}
			n++
		} else {
			if planIsEditVerb(items, j) {
				return !planVerbNegated(items, j)
			}
			if planIsMarker(it.word) || planAdjacencyStops[it.word] || planPastEdits[it.word] {
				return false
			}
			n++
		}
		if n > planAdjacencyWords {
			return false
		}
	}
	return false
}

// planSubjectEdits, directly after a path, say the file changes:
// "a.go gains a field".
var planSubjectEdits = wordSet("gains", "gain", "gets", "grows", "learns", "loses")

// planVerbAfter reports whether the slot at items[i] is the subject of an
// edit ("a.go gains a field") or follows "In" at the start of an
// imperative clause ("In a.go, add the helper").
func planVerbAfter(items []planItem, i int, slotPaths [][]planPath) bool {
	k := i + 1
	for k < len(items) && items[k].slot >= 0 && len(slotPaths[items[k].slot]) > 0 {
		k++
	}
	next := planWordAt(items, k)
	if planSubjectEdits[next] {
		return true
	}
	j := i - 1
	for j >= 0 && (items[j].slot >= 0 && len(slotPaths[items[j].slot]) > 0 || planWordAt(items, j) == "and") {
		j--
	}
	if j != 0 {
		return false
	}
	switch planWordAt(items, 0) {
	case "in", "inside", "within":
		return planIsEditVerb(items, k) && !planVerbNegated(items, k)
	}
	return false
}

// planPastEdits are past-tense edit verbs. They narrate earlier work
// ("Bead 1 added x.go"), so they end the search for an edit verb.
var planPastEdits = wordSet(
	"added", "created", "wrote", "written", "edited", "modified", "updated",
	"changed", "extended", "renamed", "deleted", "removed", "moved",
	"implemented", "refactored", "replaced", "fixed", "introduced", "shipped",
	"landed", "merged",
)

// planClause holds the word checks of one masked clause.
type planClause struct {
	items     []planItem
	passive   bool
	hasMarker bool
	firstVerb int // index of the first edit verb that is not negated, or -1
	words     int
}

// planClauseInfo runs the word checks on a masked clause. A negated edit
// verb counts as a read-only marker.
func planClauseInfo(clause string, slotPaths [][]planPath) planClause {
	c := planClause{
		items:     planClauseItems(clause),
		passive:   planPassiveEdit.MatchString(clause),
		hasMarker: planReadOnlyPhrases.MatchString(clause),
		firstVerb: -1,
	}
	for j, it := range c.items {
		if it.slot >= 0 {
			continue
		}
		c.words++
		if planIsMarker(it.word) {
			c.hasMarker = true
		}
		if planIsEditVerb(c.items, j) {
			if planVerbNegated(c.items, j) {
				c.hasMarker = true
			} else if c.firstVerb < 0 {
				c.firstVerb = j
			}
		}
	}
	return c
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
	masked, slots := maskPlanText(s)
	slotPaths := make([][]planPath, len(slots))
	for i, raw := range slots {
		slotPaths[i] = planTokenPaths(raw)
	}
	tagged := map[int]bool{}
	for _, re := range planNewTagRes {
		for _, m := range re.FindAllStringSubmatch(masked, -1) {
			n, _ := strconv.Atoi(m[1])
			tagged[n] = true
		}
	}

	sentenceMarker := planClauseInfo(masked, slotPaths).hasMarker
	var out []planPath
	start := 0
	bounds := append(planClauseSplit.FindAllStringIndex(masked, -1), []int{len(masked), len(masked)})
	prevColonEdit, prevHadPath := false, false
	for ci, b := range bounds {
		clause := masked[start:b[0]]
		delim := masked[b[0]:b[1]]
		start = b[1]
		c := planClauseInfo(clause, slotPaths)
		items, passive, hasMarker, firstVerb := c.items, c.passive, c.hasMarker, c.firstVerb
		hasVerb := passive || firstVerb >= 0
		// Under a Files heading, the first clause names a file to change,
		// unless a marker anywhere in the sentence says otherwise.
		lead := filesCtx && ci == 0 && !hasMarker && !sentenceMarker
		// "Add two files: a.go and b.go" carries the verb past the colon
		// into a short list of paths.
		inherit := prevColonEdit && !hasVerb && !hasMarker && (!prevHadPath || c.words <= 3) &&
			len(items) > 0 && items[0].slot >= 0 && len(slotPaths[items[0].slot]) > 0
		// A unit that opens with a path and then ":" or a dash names the
		// file it is about: "- a.go: add the helper".
		if ci == 0 && unitStart && !hasMarker && c.words <= 3 && len(items) > 0 &&
			items[0].slot >= 0 && len(slotPaths[items[0].slot]) > 0 && delim != "" &&
			!strings.HasPrefix(strings.TrimSpace(delim), ";") && !strings.HasPrefix(strings.TrimSpace(delim), ",") {
			lead = true
		}
		for i, it := range items {
			if it.slot < 0 || len(slotPaths[it.slot]) == 0 {
				continue
			}
			// A clause-wide edit needs no marker in the clause and an
			// active verb before the path (any verb in a table row), or
			// passive edit phrasing anywhere in the clause.
			clauseEdit := !hasMarker && (passive || (firstVerb >= 0 && (firstVerb < i || tableRow)))
			if clauseEdit || inherit || lead || tagged[it.slot] || planVerbBefore(items, i, slotPaths) || planVerbAfter(items, i, slotPaths) {
				out = append(out, slotPaths[it.slot]...)
			}
		}
		prevColonEdit = hasVerb && !hasMarker && strings.HasPrefix(delim, ":")
		prevHadPath = false
		for _, it := range items {
			if it.slot >= 0 && len(slotPaths[it.slot]) > 0 {
				prevHadPath = true
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
		if u.fileCell != "" {
			masked, _ := maskPlanText(u.text)
			marker := planReadOnlyPhrases.MatchString(masked)
			for _, it := range planClauseItems(masked) {
				if it.slot < 0 && planIsMarker(it.word) {
					marker = true
				}
			}
			if !marker {
				_, slots := maskPlanText(u.fileCell)
				for _, raw := range slots {
					add(planTokenPaths(raw))
				}
			}
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

// checkUnlistedPlanPaths adds one advisory warning (never an error) that
// lists the plan-body file paths absent from every bead's key_file_paths.
func checkUnlistedPlanPaths(r *Result, content string, chunks []WorkChunk) {
	if missing := UnlistedPlanPaths(content, chunks); len(missing) > 0 {
		r.AddWarning(PlanPathsWarningName, UnlistedPlanPathsMessage(missing))
	}
}
