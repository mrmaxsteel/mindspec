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

// docsScopeRoots are the linted surfaces per the bead brief: the live,
// forward-facing docs. Everything else (specs, migrations, ADRs,
// docs_archive/, .beads/) is the immutable historical record and is
// never linted or modified — a spec correctly describes the
// architecture as of when it was written.
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

// coveredBy reports whether line is within the coverage range of any
// marker in df matching id (any id, if id == "").
func (df *docFile) coveredBy(line int, id string) (bool, string) {
	for _, m := range df.Markers {
		if id != "" && m.ID != id {
			continue
		}
		if m.Line <= line && line < df.coverageEnd(m.Line) {
			return true, m.ID
		}
	}
	return false, ""
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
// observed forms: a single pipe-joined token (`add|list|show`) or a
// space-separated list with bare `|` separators (`create | verify |
// tally`). Returns one word-list per alternative; if tokens contains no
// "|" at all, returns the single unmodified word-list.
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
	prefix := tokens[:splitAt]
	var alts []string
	for _, t := range tokens[splitAt:] {
		for _, p := range strings.Split(t, "|") {
			if p != "" {
				alts = append(alts, p)
			}
		}
	}
	var out [][]string
	for _, a := range alts {
		cand := make([]string, 0, len(prefix)+1)
		cand = append(cand, prefix...)
		cand = append(cand, a)
		out = append(out, cand)
	}
	return out
}

// --- skill-reference extraction ----------------------------------------

var skillRefRe = regexp.MustCompile(`/ms-[a-z0-9-]+`)

type skillRefOccurrence struct {
	Name string // without leading slash
	Line int
}

// extractSkillRefs finds every `/ms-*`-shaped reference in df, at any
// position (inline prose, table cells, code spans) provided the
// character immediately before the slash is not itself alphanumeric or
// another slash (so a URL path segment like `.../foo/ms-bar` is not
// mistaken for a skill reference — no such case is known in the
// scanned corpus, but the guard costs nothing).
func extractSkillRefs(df *docFile) []skillRefOccurrence {
	var out []skillRefOccurrence
	for i, line := range df.Lines {
		for _, loc := range skillRefRe.FindAllStringIndex(line, -1) {
			start := loc[0]
			if start > 0 {
				prev := line[start-1]
				if isAlnum(prev) || prev == '/' {
					continue
				}
			}
			out = append(out, skillRefOccurrence{Name: line[start+1 : loc[1]], Line: i + 1})
		}
	}
	return out
}

func isAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
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
