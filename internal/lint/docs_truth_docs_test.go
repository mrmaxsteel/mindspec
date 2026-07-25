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

func parseDocFile(repoRoot, relPath string) (*docFile, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, relPath))
	if err != nil {
		return nil, err
	}
	raw := string(data)
	df := &docFile{Path: relPath, Raw: raw, Lines: strings.Split(raw, "\n")}

	inFence := false
	headingRe := regexp.MustCompile(`^(#{1,6})\s`)
	for i, line := range df.Lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			df.Headings = append(df.Headings, headingHit{Line: i + 1, Level: len(m[1])})
		}
	}

	for _, m := range markerRegex.FindAllStringSubmatchIndex(raw, -1) {
		line := 1 + strings.Count(raw[:m[0]], "\n")
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

// coverageEnd returns the exclusive line bound of the section a marker
// at markerLine covers: forward to (but not including) the next
// heading of equal-or-higher level (lower-or-equal number), or past
// EOF if none — per project-docs/claims-registry.yaml's contract §2
// ("a marker covers content up to the next heading of equal or higher
// level").
func (df *docFile) coverageEnd(markerLine int) int {
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
func expandAlternatives(tokens []string) [][]string {
	splitAt := -1
	for i, t := range tokens {
		if strings.Contains(t, "|") {
			splitAt = i
			break
		}
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
		// Space-separated form: consume WORD ("|" WORD)*.
		for {
			alts = append(alts, tokens[i])
			i++
			if i < len(tokens) && tokens[i] == "|" {
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

// --- claims registry ----------------------------------------------------

// claimEntry mirrors one entry under `claims:` in
// project-docs/claims-registry.yaml.
type claimEntry struct {
	Tokens    []string `yaml:"tokens"`
	Owner     string   `yaml:"owner"`
	Locations []string `yaml:"locations"`
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
