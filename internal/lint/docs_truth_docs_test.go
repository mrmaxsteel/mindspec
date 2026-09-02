// docs_truth_docs_test.go — the doc-side half of the docs-truth lint:
// which files are in scope, the inline planned-claim marker grammar and
// its heading-scoped coverage, and extracting candidate `mindspec ...`
// invocations and `/ms-*` skill references out of markdown prose.
// mindspec-ks4u (spec w0-docs-truth Bead 2).
package lint

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// docsScopeRoots are the linted surfaces: the two roots the bead brief
// named, README.md and project-docs. This is NOT "everything outside
// is the immutable historical record" — an earlier version of this
// comment claimed exactly that, and it was false (O2-5/O3-1): this
// very commit modifies CONTRIBUTING.md, a file outside these roots,
// and .mindspec/core/** is live, actively-maintained, publicly-linked
// reference documentation (README.md and project-docs/user/README.md
// both point readers at it as "the complete command reference"), not
// a point-in-time historical record.
//
// A genuinely historical, point-in-time class DOES exist outside these
// roots — .mindspec/specs/**, .mindspec/migrations/**, .mindspec/adr/**,
// docs_archive/, .beads/ — and a spec correctly describing the
// architecture as of when it was written is why THOSE stay unlinted.
// But it is not the only thing excluded, and it is not why the OTHER
// exclusions are unscanned: .mindspec/core/**, AGENTS.md, CLAUDE.md,
// CONTRIBUTING.md, SECURITY.md, BENCH-MOVED.md, and
// plugins/mindspec/** are live, public, contributor/agent-facing
// surfaces this bead's scope simply does not reach yet. That gap
// (including a real `mindspec explore` untruth in .mindspec/core/) is
// tracked separately (mindspec-6ewu) rather than silently absorbed
// into "historical record". Extending docsScopeRoots to cover them is
// a deliberate follow-up, not something this bead does as a side
// effect.
var docsScopeRoots = []string{"README.md", "project-docs"}

// docsScopeExcludeDirs are subtrees under project-docs/** carved out of
// the scanned surface for the same reason as the top-level exclusions,
// even though they weren't named individually in the bead brief:
//
//   - project-docs/user/archive/ — every file in it opens with an
//     explicit "ARCHIVED ... historical context only" marker. It is
//     the same immutable-historical-record class as docs_archive/,
//     just filed under project-docs/ instead of at repo root.
//
// docsScopeExcludeFiles are individual files carved out:
//
//   - project-docs/claims-registry.yaml — the registry itself. It is
//     consumed here as structured config (loadClaimsRegistry), not
//     scanned as prose: its own header comment quotes the marker
//     grammar and a becomes_true_when field legitimately narrates
//     unmarked future-tense command text (`mindspec loop status`
//     exists...) that would otherwise false-positive against the very
//     rule this file defines.
var docsScopeExcludeDirs = []string{
	filepath.Join("project-docs", "user", "archive"),
}

var docsScopeExcludeFiles = []string{
	filepath.Join("project-docs", "claims-registry.yaml"),
}

// listScopedDocs walks repoRoot per docsScopeRoots/Exclude* and returns
// repo-relative paths to every in-scope file (markdown or yaml; the
// lint only meaningfully inspects .md content but a stray non-.md file
// under project-docs/** is deliberately not silently skipped — R1/R2/R3
// simply find nothing to flag in it).
func listScopedDocs(repoRoot string) ([]string, error) {
	var out []string
	for _, root := range docsScopeRoots {
		full := filepath.Join(repoRoot, root)
		info, err := os.Stat(full)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, root)
			continue
		}
		err = filepath.Walk(full, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				return relErr
			}
			if fi.IsDir() {
				for _, ex := range docsScopeExcludeDirs {
					if rel == ex {
						return filepath.SkipDir
					}
				}
				return nil
			}
			for _, ex := range docsScopeExcludeFiles {
				if rel == ex {
					return nil
				}
			}
			if strings.HasSuffix(rel, ".md") {
				out = append(out, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// --- marker grammar + heading-scoped coverage -------------------------

// markerRegex is Bead 1's own grammar, verbatim (project-docs/claims-registry.yaml
// header, and see README.md:145 for the real site this was transcribed
// from): *(planned — claim `<id>`, <free text>)*
var markerRegex = regexp.MustCompile("\\*\\(planned — claim `([a-z0-9][a-z0-9-]*)`[^)]*\\)\\*")

// htmlCommentRe matches an HTML comment span, `(?s)` letting `.` cross
// line boundaries so a comment opened on one line and closed on a
// later one is still recognized as one span. Used by parseDocFile to
// exclude a marker written inside `<!-- ... -->` from df.Markers — see
// that function's doc comment for why (final-gate finding L4-FINAL-1).
var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

type markerHit struct {
	ID   string
	Line int // 1-indexed
}

type headingHit struct {
	Line  int // 1-indexed
	Level int
}

// docFile is a parsed scanned document: raw lines, headings, and marker
// hits, sufficient to answer "is line L, in this file, covered by a
// marker with this id".
type docFile struct {
	Path     string
	Raw      string
	Lines    []string
	Headings []headingHit
	Markers  []markerHit
}

// parseDocFile parses relPath into a docFile. Markers are recognized
// ONLY in TWO reader-invisible positions this file knows how to detect
// (final-gate finding L4-FINAL-1, plus its pre-existing L1-6 sibling):
// a marker written inside a fenced code block, or inside an HTML
// comment, is never added to df.Markers, however the marker grammar
// itself matches, because neither position is visible to a human
// reading the rendered document — exactly the reader/lint divergence
// the heading/non-heading coverage narrowing (F2-r2-1/A2) already
// closed for coverage SPAN, now closed for these two instances of
// marker RECOGNITION too. Without this, a marker hidden in `<!--
// *(planned — claim `x`) --> ` on a heading line grants that hidden
// marker full heading-section authority while being invisible in
// rendered docs — defeating R1/R3 through a channel neither the
// id+token rule nor the narrowed coverage checks.
//
// RESIDUAL, stated rather than silently left (confirm-round finding
// L4-FINAL-1, still open): the underlying requirement — "a marker
// grants coverage only where a reader can actually see it" — is met
// for these two channels, not for the class as a whole. Marker
// recognition here operates on raw Markdown text, not a rendered or
// semantic notion of visibility, so any OTHER way of hiding text from
// a rendered view still works against it: an HTML element carrying a
// `hidden` attribute or `aria-hidden="true"`, CSS `display:none` /
// `visibility:hidden`, an unopened `<details>` section, or zero-width
// Unicode characters splitting the marker's own text, none of which
// this file detects. L4's probe (`<span hidden>*(planned — claim
// `loop-status`...)*</span>` followed by unmarked live-looking prose)
// demonstrates the `hidden`-attribute case concretely: zero R1 findings
// for the unmarked prose, exactly the hazard this comment used to claim
// was closed. Closing the general case requires either a real
// HTML/Markdown rendering-and-visibility pass, or enumerating every
// invisible-markup channel one at a time (Go's regexp package has no
// backreferences, so even the single `hidden`-attribute case can't be
// matched precisely against its own closing tag the way htmlCommentRe
// matches `<!--`/`-->` — only approximated against the NEXT closing
// tag of any name, an approximation that itself would need its own
// false-exclusion audit before shipping). Given zero real-corpus
// exposure to any of these channels today (unlike the fenced-code and
// HTML-comment channels, both found in the real corpus), that is
// tracked as an open gap rather than attempted under this change.
func parseDocFile(repoRoot, relPath string) (*docFile, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, relPath))
	if err != nil {
		return nil, err
	}
	raw := string(data)
	df := &docFile{Path: relPath, Raw: raw, Lines: strings.Split(raw, "\n")}

	inFenceByLine := make([]bool, len(df.Lines))
	inFence := false
	headingRe := regexp.MustCompile(`^(#{1,6})\s`)
	for i, line := range df.Lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			inFenceByLine[i] = true // the delimiter line itself is not prose either
			continue
		}
		inFenceByLine[i] = inFence
		if inFence {
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			df.Headings = append(df.Headings, headingHit{Line: i + 1, Level: len(m[1])})
		}
	}

	htmlCommentSpans := htmlCommentRe.FindAllStringIndex(raw, -1)
	inHTMLComment := func(pos int) bool {
		for _, span := range htmlCommentSpans {
			if pos >= span[0] && pos < span[1] {
				return true
			}
		}
		return false
	}

	for _, m := range markerRegex.FindAllStringSubmatchIndex(raw, -1) {
		line := 1 + strings.Count(raw[:m[0]], "\n")
		if inFenceByLine[line-1] {
			continue // fenced/code content is not reader-visible prose (L1-6)
		}
		if inHTMLComment(m[0]) {
			continue // invisible in rendered docs (L4-FINAL-1)
		}
		id := raw[m[2]:m[3]]
		df.Markers = append(df.Markers, markerHit{ID: id, Line: line})
	}

	return df, nil
}

// headingLevelActiveAt returns the level of the nearest heading at or
// before line, or 0 if line precedes every heading in the file.
func (df *docFile) headingLevelActiveAt(line int) int {
	lvl := 0
	for _, h := range df.Headings {
		if h.Line <= line {
			lvl = h.Level
		} else {
			break
		}
	}
	return lvl
}

// isHeadingLine reports whether line is itself one of df's recorded
// heading lines.
func (df *docFile) isHeadingLine(line int) bool {
	for _, h := range df.Headings {
		if h.Line == line {
			return true
		}
	}
	return false
}

// listItemStartRe matches a line that opens a new markdown list item
// (bulleted or ordered), used by paragraphCoverageEnd below as a block
// boundary distinct from a plain blank line.
var listItemStartRe = regexp.MustCompile(`^\s*([-*+]|\d+\.)\s`)

// tableRowStartRe and blockquoteStartRe are paragraphCoverageEnd's
// other two block-boundary shapes (final-gate finding L1-3): a
// markdown table row and a blockquote line, treated exactly like
// listItemStartRe. Without these, a marker in a table cell covered
// every remaining row of that table, and a marker on a `>` line
// covered the rest of the quote — both containers the registry
// contract's own clause 2 names as narrow-coverage shapes ("a
// sentence, a list item, a table cell"), and the exact smuggle the
// list-item narrowing was built to close, surviving in the two block
// shapes nobody added a boundary for.
//
// tableRowStartRe alone only recognizes a LEADING-PIPE table row
// (`| cell | cell |`). Confirm-round finding L1-C4: GFM also permits
// omitting the leading (and trailing) pipe on every row — a valid
// `Verb | Status` line with no leading `|` — and an HTML `<tr><td>`
// table row, neither of which tableRowStartRe matched, so a marker
// still bled coverage past either shape while clause 2 promised
// otherwise. isTableRowLine below recognizes both, in addition to the
// leading-pipe shape.
var tableRowStartRe = regexp.MustCompile(`^\s*\|`)
var blockquoteStartRe = regexp.MustCompile(`^\s*>`)

// htmlTableRowStartRe is the HTML half of L1-C4's fix: an HTML `<tr>`
// table-row tag, the other container clause 2's "table cell" promise
// covers that tableRowStartRe's markdown-only pattern cannot match.
var htmlTableRowStartRe = regexp.MustCompile(`(?i)^\s*<tr\b`)

// pipeRowCellSepRe matches a `|` surrounded by whitespace with
// non-whitespace on both sides — the cell separator shape of a
// pipe-LESS GFM table row (L1-C4's other markdown gap: `Verb | Status`,
// no leading pipe, is valid GFM and tableRowStartRe's leading-`^\s*\|`
// pattern never matches it). Matched only after stripping inline code
// spans (this file's inlineCodeSpanRe, defined below and also used by
// extractInvocations) — replacing each span with a single non-space
// placeholder character, NOT with nothing, so a cell whose entire
// content is a code span (e.g. a "mindspec loop status | works today"
// row where the whole first cell is inline code) still has
// non-whitespace immediately next to the pipe once stripped; deleting
// the span outright would leave a bare leading space there and
// silently miss the row. A literal shell pipe INSIDE backticks (the
// span itself) is removed either way, so it is never mistaken for
// this shape.
var pipeRowCellSepRe = regexp.MustCompile(`\S\s+\|\s+\S`)

// isTableRowLine reports whether line opens (or is) a markdown or HTML
// table row, in any of the three shapes L1-3/L1-C4 narrow coverage for:
// leading-pipe markdown, pipe-less markdown, or HTML <tr>.
func isTableRowLine(line string) bool {
	if tableRowStartRe.MatchString(line) || htmlTableRowStartRe.MatchString(line) {
		return true
	}
	return pipeRowCellSepRe.MatchString(inlineCodeSpanRe.ReplaceAllString(line, "X"))
}

// isFenceDelimLine reports whether line is a fenced-code-block
// delimiter (``` or ~~~ after leading whitespace), the same test
// parseDocFile itself uses to toggle inFence.
func isFenceDelimLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// coverageEnd returns the exclusive line bound of the region a marker
// at markerLine covers. F2-r2-1 (A2): coverageEnd used to be
// heading-scoped unconditionally, so an inline marker embedded mid-list
// or mid-paragraph (e.g. autonomy.md:115, a single bulleted item)
// inherited the same wide reach as a section-heading marker — up to
// eight unrelated sibling bullets, in the real corpus, silently
// "covered" by a marker that named none of them. The registry
// contract's own placement clause (§2) actually describes two
// different shapes, and they get two different spans:
//
//   - a marker ON a heading line names the whole section that heading
//     introduces — headingCoverageEnd, UNCHANGED from before. This is
//     legitimate and stays wide on purpose: the heading itself is the
//     reader-visible label for everything under it, exactly the "the
//     paragraph introducing the section... that contains it" case the
//     contract already names, and every current wide-span claim in the
//     real corpus (loop-governance's config-key tokens, buried inside
//     the YAML examples several lines below their section's heading
//     marker) depends on exactly this shape.
//   - a marker on any OTHER line names only the sentence/paragraph/list
//     item making the claim — paragraphCoverageEnd, NEW: narrowed to
//     that block, plus (per the contract's other named shape, "the
//     paragraph introducing... the code block that contains it") an
//     immediately-following fenced code block, if the marker's
//     paragraph is followed (across any number of blank lines, but no
//     other content) directly by one. README.md:110 is exactly this
//     shape: an inline paragraph, no heading, that introduces the
//     `loop:` YAML example immediately below it.
//
// A marker never covers a SIBLING list item or a later paragraph in
// the same section anymore unless it is itself heading-placed — this
// is what closes the exploit (see TestR2CoverageDoesNotBleedIntoSiblingListItem).
func (df *docFile) coverageEnd(markerLine int) int {
	if df.isHeadingLine(markerLine) {
		return df.headingCoverageEnd(markerLine)
	}
	return df.paragraphCoverageEnd(markerLine)
}

// headingCoverageEnd is coverageEnd's original, unchanged behavior for
// a marker embedded in a heading line: forward to (but not including)
// the next heading of equal-or-higher level (lower-or-equal number),
// or past EOF if none.
func (df *docFile) headingCoverageEnd(markerLine int) int {
	activeLvl := df.headingLevelActiveAt(markerLine)
	if activeLvl == 0 {
		return len(df.Lines) + 1
	}
	for _, h := range df.Headings {
		if h.Line > markerLine && h.Level <= activeLvl {
			return h.Line
		}
	}
	return len(df.Lines) + 1
}

// paragraphCoverageEnd is coverageEnd's new, narrow behavior for a
// marker NOT on a heading line: the marker's own contiguous block
// (ending at the next blank line, the next list-item-start line, the
// next table row, the next blockquote line, the next heading, or EOF
// — whichever comes first), plus, if that block is immediately
// followed by a fenced code block (skipping only blank lines to find
// it), the whole of that code block too. The table-row and
// blockquote-line boundaries (final-gate finding L1-3) close the same
// smuggle the list-item boundary closes, for the two other container
// shapes the registry contract's clause 2 names ("a sentence, a list
// item, a table cell") but this function did not yet narrow: a marker
// in a table cell used to cover every remaining row of that table,
// and a marker on a `>` line used to cover the rest of the quote.
func (df *docFile) paragraphCoverageEnd(markerLine int) int {
	n := len(df.Lines)

	end := markerLine + 1
	for end <= n {
		line := df.Lines[end-1]
		if strings.TrimSpace(line) == "" || df.isHeadingLine(end) ||
			listItemStartRe.MatchString(line) || isTableRowLine(line) || blockquoteStartRe.MatchString(line) {
			break
		}
		end++
	}

	probe := end
	for probe <= n && strings.TrimSpace(df.Lines[probe-1]) == "" {
		probe++
	}
	if probe > n || !isFenceDelimLine(df.Lines[probe-1]) {
		return end
	}
	for closeLine := probe + 1; closeLine <= n; closeLine++ {
		if isFenceDelimLine(df.Lines[closeLine-1]) {
			return closeLine + 1
		}
	}
	return n + 1 // unterminated fence: defensively cover to EOF
}

// coveredBy reports whether line is within the coverage range of a
// marker in df bearing exactly id. Callers must always pass a specific
// id — an earlier version of this function treated id == "" as "any
// marker, any id", which is the marker-amnesty defect (F2-1/O2-1): a
// marker's heading-scoped span exempted every unresolved
// invocation/skill-ref in its range regardless of the marker's own id
// or registered tokens, so any registered claim could be minted into a
// blanket lint-off pragma anywhere in its section. checkR1/checkR2 no
// longer call this with an empty id; see markerTokenCovers, the
// id-typed and token-matched replacement for that use.
func (df *docFile) coveredBy(line int, id string) (bool, string) {
	for _, m := range df.Markers {
		if m.ID != id {
			continue
		}
		if m.Line <= line && line < df.coverageEnd(m.Line) {
			return true, m.ID
		}
	}
	return false, ""
}

// markerTokenCovers is R1/R2's exemption rule (F2-1/O1-2/O2-1's
// required change): an unresolved invocation/skill-ref at line, in df,
// is exempted by a covering marker ONLY when that marker's id is
// registered in reg AND text is exactly one of that claim's tokens —
// the symmetric partner of R3's existing token-coverage clause (R3
// enforces token-occurrence => marked; this enforces
// exempted-failure => registered token). A marker therefore exempts
// only the specific claim it names, never every unresolved claim
// anywhere in its heading-scoped coverage range. text is the exact
// content-identity string the caller would otherwise report as a
// finding: joinWords(words) for R1, "/"+sref.Name for R2.
func markerTokenCovers(df *docFile, line int, text string, reg *claimsRegistryDoc) bool {
	if reg == nil {
		return false
	}
	for _, m := range df.Markers {
		if !(m.Line <= line && line < df.coverageEnd(m.Line)) {
			continue
		}
		claim, ok := reg.Claims[m.ID]
		if !ok {
			continue
		}
		for _, tok := range claim.Tokens {
			if tok == text {
				return true
			}
		}
	}
	return false
}

// --- invocation extraction ---------------------------------------------

type invocationOccurrence struct {
	Text string // e.g. "loop status" or "domain add|list|show --flag"
	Line int
}

var inlineCodeSpanRe = regexp.MustCompile("`([^`]+)`")

// extractInvocations finds every candidate `mindspec ...` invocation in
// df: inline single-backtick code spans, plus lines inside fenced code
// blocks that start with `mindspec ` (an optional leading `$ ` shell
// prompt is tolerated; a trailing `# comment` is stripped).
func extractInvocations(df *docFile) []invocationOccurrence {
	var out []invocationOccurrence

	inFence := false
	for i, line := range df.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			body := trimmed
			body = strings.TrimPrefix(body, "$ ")
			if body == "mindspec" || strings.HasPrefix(body, "mindspec ") {
				if idx := strings.Index(body, "#"); idx >= 0 {
					body = body[:idx]
				}
				out = append(out, invocationOccurrence{Text: strings.TrimSpace(body), Line: i + 1})
			}
			continue
		}
		for _, m := range inlineCodeSpanRe.FindAllStringSubmatch(line, -1) {
			body := strings.TrimSpace(m[1])
			if body == "mindspec" || strings.HasPrefix(body, "mindspec ") {
				out = append(out, invocationOccurrence{Text: body, Line: i + 1})
			}
		}
	}
	return out
}

// tokenizeInvocation splits an extracted invocation string into
// whitespace-delimited words, normalizing the escaped-pipe form seen in
// markdown tables (`a\|b\|c`) to plain `|`.
func tokenizeInvocation(text string) []string {
	text = strings.ReplaceAll(text, "\\|", "|")
	return strings.Fields(text)
}

// expandAlternatives handles the doc convention of listing several
// subcommands sharing a prefix as one invocation, in either of the two
// observed forms: a single pipe-joined token (`add|list|show`, also
// what the escaped-pipe markdown-table form `add\|list\|show` becomes
// after tokenizeInvocation normalizes it) or a space-separated list
// with bare `|` separators (`create | verify | tally`). Returns one
// word-list per alternative, with any tokens AFTER the alternation
// group (flags, positional words) preserved on every generated
// alternative; if tokens contains no "|" at all, returns the single
// unmodified word-list.
//
// O1-1/O2-4: the previous version got both forms wrong. For the
// space-separated form, it took the first PIPE-BEARING token as the
// split point — but that token is the bare `|` itself, so the word
// before it (the first real alternative) stayed glued into the prefix
// and was never generated as its own candidate; `README.md:77`'s
// `mindspec panel create | verify | tally` therefore checked only
// `panel create`, never `panel verify` or `panel tally`. For the
// glued single-token form, every word after the split point — flags
// and flag VALUES included — was folded into the alternative list
// instead of being carried along on each alternative, so `mindspec
// spec create|verify --title x` generated garbage candidates
// (`spec --title`, `spec x`) and never checked the true claim `spec
// create --title x` / `spec verify --title x`.
//
// # The trailing-bare-pipe panic (W0 Bead 9, mindspec-ng3g) and the
// # alternation-vs-shell-pipe rule
//
// A token list ending in a bare, unpaired "|" — `mindspec panel
// create |` (index out of range [4] with length 4), or just
// `mindspec |` (index out of range [2] with length 2) — panicked: the
// space-separated-form loop consumed the "|" (line "i++; continue")
// assuming a following word always exists, then indexed tokens[i] one
// past the end. `mindspec config show | grep runner` never panicked
// (the "|" there has a following word, "grep"), which is also why the
// prior version's own comment never surfaced the bug.
//
// The fix is the same guard in both places bare "|" is recognized as
// a separator: a "|" only introduces another alternative when a real
// word follows it in the SAME token list (checked before consuming
// it, both at the split-point search below and inside the
// alternative-collecting loop). This is also this function's answer
// to the "how is an alternation pipe told apart from an ordinary
// SHELL pipe" question the bead brief asks to state explicitly:
// alternation requires a word on BOTH sides within one extracted
// invocation's tokens (`create | verify`, `create|verify`); a
// trailing bare "|" with nothing after it is neither shape and is
// left untouched in the tail rather than treated as a separator — it
// most likely means a doc's invocation text was truncated or
// malformed, not that a real alternation or shell pipe was intended.
// A residual, PRE-EXISTING and UNCHANGED limitation this fix does not
// attempt to close (out of this bead's narrow scope — the panic and
// the stated rule, not a rewrite of the alternation model): a genuine
// shell pipe INTO A DIFFERENT PROGRAM whose right-hand side is a
// single bare word with something after IT too (`mindspec config show
// | grep runner`, plugins/mindspec/skills/ms-panel-run/SKILL.md:18 —
// outside docsScopeRoots, so never actually reached by this lint
// today) is syntactically indistinguishable from `create | verify`
// alternation and would still be misread as an alternative between
// "show" and "grep". Closing that would require resolving each
// alternative against the real command tree from inside this
// doc-side-only function, which is a materially larger change than
// "fix the panic and state the rule".
func expandAlternatives(tokens []string) [][]string {
	splitAt := -1
	for i, t := range tokens {
		if !strings.Contains(t, "|") {
			continue
		}
		if t == "|" && (i == 0 || i+1 >= len(tokens)) {
			continue // dangling bare pipe: no left or right neighbor to alternate
		}
		splitAt = i
		break
	}
	if splitAt < 0 {
		return [][]string{tokens}
	}

	// A bare "|" token is a separator, not an alternative — its LEFT
	// neighbor (which would otherwise stay glued into the prefix) is
	// really the first alternative, so the alternation group starts one
	// token earlier in that case. A glued token ("create|verify|tally")
	// already contains every alternative and needs no earlier token.
	groupStart := splitAt
	if tokens[splitAt] == "|" && splitAt > 0 {
		groupStart = splitAt - 1
	}

	var alts []string
	i := groupStart
	if strings.Contains(tokens[i], "|") {
		for _, p := range strings.Split(tokens[i], "|") {
			if p != "" {
				alts = append(alts, p)
			}
		}
		i++
	} else {
		// Space-separated form: consume WORD ("|" WORD)*, but only
		// consume a "|" that has a WORD after it (i+1 < len(tokens)) —
		// a dangling trailing "|" is left for tail below instead of
		// panicking on an out-of-range index or being folded in as a
		// meaningless extra alternative.
		for i < len(tokens) {
			alts = append(alts, tokens[i])
			i++
			if i < len(tokens) && tokens[i] == "|" && i+1 < len(tokens) {
				i++
				continue
			}
			break
		}
	}
	groupEnd := i

	prefix := tokens[:groupStart]
	tail := tokens[groupEnd:]

	out := make([][]string, 0, len(alts))
	for _, a := range alts {
		cand := make([]string, 0, len(prefix)+1+len(tail))
		cand = append(cand, prefix...)
		cand = append(cand, a)
		cand = append(cand, tail...)
		out = append(out, cand)
	}
	return out
}

// --- skill-reference extraction ----------------------------------------

// skillRefRe matches both the slashed form (`/ms-explore`) and the
// slashless prose form ("the ms-fix-cycle skill") — O2-7 found the
// slashless form escaping R2 (and R3's token clause, whose fix-cycle
// token is literally `/ms-fix-cycle`) entirely: a retracted claim can
// be reintroduced by simply dropping the leading slash, in text that
// reads identically to a human reader. Group 1 is the leading-boundary
// alternative (start-of-line, or one non-alnum-non-slash character);
// group 2 is the name, always WITHOUT a leading slash, matching the
// old Name field's format. The optional `/?` between them makes the
// slash itself, when present, part of the boundary rather than the
// name — see extractSkillRefs.
var skillRefRe = regexp.MustCompile(`(^|[^a-zA-Z0-9/])/?(ms-[a-z0-9-]+)`)

type skillRefOccurrence struct {
	Name string // without leading slash
	Line int
}

// extractSkillRefs finds every `/ms-*`- or slashless `ms-*`-shaped
// reference in df, at any position (inline prose, table cells, code
// spans). skillRefRe's leading-boundary group replaces the old manual
// isAlnum/prev=='/' guard: a URL path segment like `.../foo/ms-bar` is
// still not mistaken for a skill reference (the character immediately
// before "ms-bar" is "/", which the boundary group's char class
// excludes, and the character before THAT is alphanumeric, so no
// anchor position produces a match), whether or not the ms- name
// itself happens to be preceded by a slash.
func extractSkillRefs(df *docFile) []skillRefOccurrence {
	var out []skillRefOccurrence
	for i, line := range df.Lines {
		for _, m := range skillRefRe.FindAllStringSubmatchIndex(line, -1) {
			out = append(out, skillRefOccurrence{Name: line[m[4]:m[5]], Line: i + 1})
		}
	}
	return out
}

// extractRetiredSlashRefs finds slash-prefixed occurrences of a
// RETIRED pre-"ms-"-rename name (O2-7 residual: see
// retiredPreRenameNames, docs_truth_skills_test.go). Unlike
// skillRefRe, which matches any `ms-[a-z0-9-]+` shape generically,
// this matches only the exact retired names passed in, via
// alternation — deliberately NOT a generic "any slash-prefixed
// hyphenated word" pattern, which would false-positive on ordinary
// prose slashes sharing no structural signal with a command reference
// (`and/or`, `on/off`, date and fraction notation). A name is only
// reported here if it is BOTH slash-prefixed AND an exact former
// skill/workflow spelling; retired returns nil short-circuits to no
// matches when there is nothing to look for.
func extractRetiredSlashRefs(df *docFile, retired map[string]bool) []skillRefOccurrence {
	if len(retired) == 0 {
		return nil
	}
	names := make([]string, 0, len(retired))
	for n := range retired {
		names = append(names, regexp.QuoteMeta(n))
	}
	re := regexp.MustCompile(`(^|[^a-zA-Z0-9/])/(` + strings.Join(names, "|") + `)([^a-zA-Z0-9-]|$)`)
	var out []skillRefOccurrence
	for i, line := range df.Lines {
		for _, m := range re.FindAllStringSubmatchIndex(line, -1) {
			out = append(out, skillRefOccurrence{Name: line[m[4]:m[5]], Line: i + 1})
		}
	}
	return out
}

// --- claims registry ----------------------------------------------------

// claimEntry mirrors one entry under `claims:` in
// project-docs/claims-registry.yaml.
type claimEntry struct {
	Tokens          []string `yaml:"tokens"`
	Owner           string   `yaml:"owner"`
	Locations       []string `yaml:"locations"`
	BecomesTrueWhen string   `yaml:"becomes_true_when"`
}

type claimsRegistryDoc struct {
	Version int                   `yaml:"version"`
	Claims  map[string]claimEntry `yaml:"claims"`
}

func loadClaimsRegistry(repoRoot string) (*claimsRegistryDoc, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "project-docs", "claims-registry.yaml"))
	if err != nil {
		return nil, err
	}
	var doc claimsRegistryDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// findAllLiteral returns every line number (1-indexed) on which token
// occurs as a literal substring of df.Raw.
func (df *docFile) findAllLiteral(token string) []int {
	var lines []int
	raw := df.Raw
	start := 0
	for {
		idx := strings.Index(raw[start:], token)
		if idx < 0 {
			break
		}
		abs := start + idx
		lines = append(lines, 1+strings.Count(raw[:abs], "\n"))
		start = abs + len(token)
	}
	return lines
}

// joinWords renders a resolved word list back into a plain (unquoted)
// invocation string. Used as the content-identity key for
// knownDocsTruthExceptions (docs_truth_test.go) — callers that want a
// human-readable, quoted form for error output apply %q themselves
// (see truthFinding.String) rather than baking quoting into the value
// that identity matching compares.
func joinWords(words []string) string {
	return strings.Join(words, " ")
}
