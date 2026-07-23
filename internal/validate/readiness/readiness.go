package readiness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// --- bd-less hermeticity seam (spec 124 plan-gate G2-BDLESS-ENGINE-SEAM) ---
//
// The two package-level func vars below are the COMPLETE bd/lineage
// read-set EvaluateReadiness performs. Every one of MF-1..MF-4 reaches
// internal/bead only through these vars, so engine unit tests can swap
// both to deterministic in-memory fixtures and consult no real bd
// process — hermetic under `bd` absent from PATH, never a t.Skip (spec
// 124 plan-gate F3-1, the spec-119 lesson). MF-3's landed-merge leg
// (internal/lifecycle.FindLandedMerge) is a GIT read, not bd, and is
// exercised over real temp git repos in tests, not faked.
var (
	// findEpicForBeadFn resolves beadID's owning epic + spec via bd/phase
	// lineage (the internal/complete/complete.go:369 fail-closed pattern
	// — NEVER cwd). A real lookup error propagates as-is; this package
	// never degrades to a cwd-derived resolution.
	findEpicForBeadFn = phase.FindEpicForBead

	// fetchBeadRecordFn reads a bead's OWN bd record: its description
	// (MF-2's harvest context and MF-4's description scan) and its
	// "blocks" dependency edges, each carrying the dependency's own bd
	// status (MF-3). One `bd show <id> --json` read serves both needs.
	fetchBeadRecordFn = fetchBeadRecordReal

	// findLandedMergeFn is MF-3's landed-merge decision seam (FX-1). It
	// defaults to lifecycle.FindLandedMerge, which is a GIT read but ALSO
	// performs a TRANSITIVE bd read: FindLandedMerge -> landedBindingForBead
	// -> bead.GetMetadata (internal/lifecycle/landed.go), the merge-time
	// landed-binding corroboration leg. That transitive bd read is not
	// covered by the fetchBeadRecordFn/findEpicForBeadFn seams above, so
	// per the plan's F3-1 "route EVERY bd read through the injectable seam"
	// requirement the WHOLE landed-merge decision is seamed HERE — the
	// AC-3 real-repo tests keep the real function (git+bd both exercised
	// end-to-end over real temp repos), while a bd-less test that must
	// exercise the metadata-corroborated landed path swaps this var to a
	// deterministic in-memory return, consulting no real bd.
	findLandedMergeFn = lifecycle.FindLandedMerge
)

// dependencyEdge is one "blocks" dependency edge read off a bead's bd
// record: the dependency's bead ID and its own bd status.
type dependencyEdge struct {
	ID     string
	Status string
}

// beadRecord is the subset of a bead's bd record EvaluateReadiness needs.
type beadRecord struct {
	Description  string
	Dependencies []dependencyEdge
}

// fetchBeadRecordReal is the real, bd-backed default for fetchBeadRecordFn.
func fetchBeadRecordReal(beadID string) (*beadRecord, error) {
	// Class-2 consumer boundary (ADR-0042 Section1): beadID feeds a `bd
	// show` argv build directly — validated before any bd spawn.
	if err := idvalidate.BeadID(beadID); err != nil {
		return nil, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}
	out, err := bead.RunBD("show", beadID, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd show %s failed: %w", idrender.Bead(beadID), err)
	}
	var items []struct {
		Description  string `json:"description"`
		Dependencies []struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			DependencyType string `json:"dependency_type"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd show %s output: %w", idrender.Bead(beadID), err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("bead %s not found", idrender.Bead(beadID))
	}
	rec := &beadRecord{Description: items[0].Description}
	for _, d := range items[0].Dependencies {
		if strings.EqualFold(d.DependencyType, "blocks") {
			rec.Dependencies = append(rec.Dependencies, dependencyEdge{ID: d.ID, Status: d.Status})
		}
	}
	return rec, nil
}

// EvaluateReadiness evaluates the four mechanical-floor signals (MF-1..
// MF-4, spec 124 R2) for beadID and returns the resulting Report. It never
// mutates anything: no bd write, no git write, no file write on any path
// (spec 124 R1).
//
// Owning-spec resolution is LINEAGE-authoritative (bead -> epic -> spec via
// findEpicForBeadFn), never cwd-derived — a real lookup error refuses here
// rather than degrading to a cwd fallback.
func EvaluateReadiness(root, beadID string) (*Report, error) {
	if err := idvalidate.BeadID(beadID); err != nil {
		return nil, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}

	_, specID, err := findEpicForBeadFn(beadID)
	if err != nil {
		return nil, fmt.Errorf("resolving owning spec for %s: %w", idrender.Bead(beadID), err)
	}
	if specID == "" {
		return nil, fmt.Errorf("bead %s has no discoverable owning spec lineage", idrender.Bead(beadID))
	}

	n, err := beadIndex(beadID)
	if err != nil {
		return nil, err
	}

	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return nil, fmt.Errorf("resolving spec dir for %s: %w", idrender.Spec(specID), err)
	}

	planContent, err := os.ReadFile(filepath.Join(specDir, "plan.md"))
	if err != nil {
		return nil, fmt.Errorf("reading plan.md for spec %s: %w", idrender.Spec(specID), err)
	}
	specContent, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil {
		return nil, fmt.Errorf("reading spec.md for spec %s: %w", idrender.Spec(specID), err)
	}

	fm, err := validate.ParsePlanFrontmatter(string(planContent))
	if err != nil {
		return nil, fmt.Errorf("parsing plan frontmatter for spec %s: %w", idrender.Spec(specID), err)
	}

	rec, err := fetchBeadRecordFn(beadID)
	if err != nil {
		return nil, fmt.Errorf("reading bd record for %s: %w", idrender.Bead(beadID), err)
	}

	sectionRaw, sectionFound := extractBeadSectionRaw(string(planContent), n)

	var chunk *validate.WorkChunk
	for i := range fm.WorkChunks {
		if fm.WorkChunks[i].ID == n {
			chunk = &fm.WorkChunks[i]
			break
		}
	}

	ownSpecNum := leadingSpecNumber(specID)

	report := &Report{BeadID: beadID}
	report.Signals = append(report.Signals, evaluateMF1(sectionRaw, sectionFound, chunk, n))
	report.Signals = append(report.Signals, evaluateMF2(rec.Description, sectionRaw, string(specContent), ownSpecNum))
	report.Signals = append(report.Signals, evaluateMF3(root, specID, rec.Dependencies))
	report.Signals = append(report.Signals, evaluateMF4(rec.Description, sectionRaw))
	return report, nil
}

// beadIndexRe extracts the trailing dotted epic-child index (e.g. ".1" in
// "mindspec-8nhe.1") — the bead's N, mapped positionally to
// work_chunks[N-1] / the Nth "## Bead" section (spec 097 R3).
var beadIndexRe = regexp.MustCompile(`\.([0-9]+)$`)

func beadIndex(beadID string) (int, error) {
	m := beadIndexRe.FindStringSubmatch(beadID)
	if m == nil {
		return 0, fmt.Errorf("bead %s has no dotted epic-child index (expected <epic>.<N>)", idrender.Bead(beadID))
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 0, fmt.Errorf("bead %s has an invalid epic-child index", idrender.Bead(beadID))
	}
	return n, nil
}

// leadingSpecNumber parses the leading digit run of specID (e.g. 124 from
// "124-impl-readiness-gate") — MF-2's foreign-citation exclusion needs this
// to tell "the spec 123 AC-17 pattern" (a citation of a DIFFERENT spec)
// from a genuine same-spec reference.
func leadingSpecNumber(specID string) int {
	i := 0
	for i < len(specID) && specID[i] >= '0' && specID[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(specID[:i])
	return n
}

// --- MF-1: plan section concrete-by-structure (spec 124 R2 MF-1) ---

const scaffoldACPlaceholder = "<Specific, measurable criterion for this bead>"
const scaffoldKeyFilePathPlaceholder = "path/to/file.go"

func evaluateMF1(sectionRaw string, sectionFound bool, chunk *validate.WorkChunk, n int) Signal {
	if !sectionFound {
		return Signal{
			ID:       SignalPlanSection,
			Pass:     false,
			Detail:   fmt.Sprintf("plan.md has no \"## Bead %d\" section", n),
			Recovery: fmt.Sprintf("add a \"## Bead %d\" section to plan.md with a concrete Acceptance Criteria entry", n),
		}
	}
	acEntries, _ := extractNamedBlock(sectionRaw, "**Acceptance Criteria**")
	if !hasConcreteACEntry(acEntries) {
		return Signal{
			ID:       SignalPlanSection,
			Pass:     false,
			Detail:   fmt.Sprintf("\"## Bead %d\"'s Acceptance Criteria block has no entry beyond the scaffold placeholder %q", n, scaffoldACPlaceholder),
			Recovery: fmt.Sprintf("edit plan.md's \"## Bead %d\" Acceptance Criteria block: replace the placeholder with a concrete, measurable criterion", n),
		}
	}
	if chunk == nil {
		return Signal{
			ID:       SignalPlanSection,
			Pass:     false,
			Detail:   fmt.Sprintf("plan.md frontmatter has no work_chunks entry with id: %d", n),
			Recovery: fmt.Sprintf("add a work_chunks entry with id: %d and non-empty key_file_paths to plan.md's frontmatter", n),
		}
	}
	if len(chunk.KeyFilePaths) == 0 {
		return Signal{
			ID:       SignalPlanSection,
			Pass:     false,
			Detail:   fmt.Sprintf("work_chunks[%d].key_file_paths is empty", n),
			Recovery: fmt.Sprintf("declare the files this bead touches in work_chunks id: %d's key_file_paths", n),
		}
	}
	hasConcretePath := false
	for _, p := range chunk.KeyFilePaths {
		t := strings.TrimSpace(p)
		if t == scaffoldKeyFilePathPlaceholder {
			return Signal{
				ID:       SignalPlanSection,
				Pass:     false,
				Detail:   fmt.Sprintf("work_chunks[%d].key_file_paths still carries the scaffold placeholder %q", n, scaffoldKeyFilePathPlaceholder),
				Recovery: fmt.Sprintf("replace the placeholder path in work_chunks id: %d's key_file_paths with real file paths", n),
			}
		}
		if t != "" {
			hasConcretePath = true
		}
	}
	// FX-3: a non-empty slice whose elements are all blank/whitespace
	// ("", "  ") is NOT a concrete files-in-scope declaration — it must
	// FAIL exactly like an empty slice would.
	if !hasConcretePath {
		return Signal{
			ID:       SignalPlanSection,
			Pass:     false,
			Detail:   fmt.Sprintf("work_chunks[%d].key_file_paths has no non-blank concrete path element", n),
			Recovery: fmt.Sprintf("declare the files this bead touches in work_chunks id: %d's key_file_paths (blank entries do not count)", n),
		}
	}
	return Signal{ID: SignalPlanSection, Pass: true}
}

var boldHeaderRe = regexp.MustCompile(`^\*\*[^*]+\*\*$`)

// extractBeadSectionRaw returns the raw text of the Nth "## Bead " section
// in content (the heading line through, but not including, the next H2
// heading), and whether such a section exists.
func extractBeadSectionRaw(content string, n int) (string, bool) {
	lines := strings.Split(content, "\n")
	var buf []string
	count := 0
	capturing := false
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## Bead ") {
			if capturing {
				break
			}
			count++
			if count == n {
				capturing = true
				found = true
			}
			if capturing {
				buf = append(buf, line)
			}
			continue
		}
		if capturing && strings.HasPrefix(line, "## ") {
			break
		}
		if capturing {
			buf = append(buf, line)
		}
	}
	return strings.Join(buf, "\n"), found
}

// extractNamedBlock returns the "- " list-item lines immediately following
// a bold header line matching headerText (e.g. "**Acceptance Criteria**"),
// stopping at the next bold header or H2/H3 heading.
func extractNamedBlock(sectionRaw, headerText string) ([]string, bool) {
	lines := strings.Split(sectionRaw, "\n")
	idx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == headerText {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, false
	}
	var out []string
	for _, l := range lines[idx+1:] {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") || boldHeaderRe.MatchString(trimmed) {
			break
		}
		if strings.HasPrefix(trimmed, "-") {
			out = append(out, trimmed)
		}
	}
	return out, true
}

func stripListMarker(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[ ]") || strings.HasPrefix(s, "[x]") || strings.HasPrefix(s, "[X]") {
		s = strings.TrimSpace(s[3:])
	}
	return s
}

func hasConcreteACEntry(entries []string) bool {
	for _, e := range entries {
		stripped := stripListMarker(e)
		if stripped != "" && stripped != scaffoldACPlaceholder {
			return true
		}
	}
	return false
}

// --- MF-2: claimed Rs/ACs resolve, by an exact rule (spec 124 R2 MF-2) ---

// claimTokenRe matches an R<n> or AC-<n> base token, plus an optional
// directly-appended single lowercase sub-letter (e.g. "R5a", "AC-2b").
// A following parenthetical ("R5(b)", "AC-9(i)") is detected separately
// (extractLineClaims) since regexp (RE2) has no lookahead.
var claimTokenRe = regexp.MustCompile(`\b(R|AC-)([0-9]+)([a-z])?\b`)

// foreignSpecRe matches a "spec <digits>" / "Spec <digits>" citation —
// MF-2's foreign-citation exclusion: a token on a line naming a DIFFERENT
// spec number is a citation, never a claim.
var foreignSpecRe = regexp.MustCompile(`(?i)\bspec\s+([0-9]+)\b`)

// romanEnumeratorRe matches the lowercase-Roman-numeral clause-enumerator
// sequence built ONLY from i/v/x (i, ii, iii, iv, v, vi, vii, viii, ix, x,
// and further compositions) — the FORM the spec pins as always a clause
// enumerator, never a sub-letter, even when the same single letter (e.g.
// "d", "c", "l", "m") would also be a technically-valid Roman numeral
// symbol: those are explicitly classified as sub-letters by the spec, so
// this pattern deliberately excludes them (the closed i/v/x set is the
// FORM the spec names, not "is this string Roman-numeral-valid").
var romanEnumeratorRe = regexp.MustCompile(`^x{0,3}(?:ix|iv|v?i{0,3})$`)

type claimToken struct {
	base        string
	letter      string
	subLettered bool
	raw         string
}

// classifyParenthetical classifies a parenthetical's content by FORM (spec
// 124 plan preamble rule 3, plan-gate F2-1/G2-ENUMERATOR-COLLISION):
// a lowercase-Roman-numeral-sequence content is a clause ENUMERATOR
// (isEnumerator=true, degrades to the base token); a single non-Roman
// alphabetic letter is a SUB-LETTER (subLetter=that letter); anything else
// is unrecognized (recognized=false — the caller falls back to treating
// the token as a plain base claim, parenthetical ignored).
func classifyParenthetical(content string) (isEnumerator bool, subLetter string, recognized bool) {
	if content == "" {
		return false, "", false
	}
	if romanEnumeratorRe.MatchString(content) {
		return true, "", true
	}
	if len(content) == 1 && content[0] >= 'a' && content[0] <= 'z' {
		return false, content, true
	}
	return false, "", false
}

// harvestClaims extracts the bead's OWN claimed R<n>/AC-<n> tokens from
// text (bd description or plan section), applying the three plan-level
// harvest rules: code-span/fence exclusion, foreign-citation exclusion
// (per line), and clause-enumerator normalization.
func harvestClaims(text string, ownSpecNum int) []claimToken {
	stripped := stripCodeSpans(text)
	var out []claimToken
	for _, line := range strings.Split(stripped, "\n") {
		if lineHasForeignCitation(line, ownSpecNum) {
			continue
		}
		out = append(out, extractLineClaims(line)...)
	}
	return out
}

func lineHasForeignCitation(line string, ownSpecNum int) bool {
	for _, m := range foreignSpecRe.FindAllStringSubmatch(line, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil && n != ownSpecNum {
			return true
		}
	}
	return false
}

func extractLineClaims(line string) []claimToken {
	var out []claimToken
	for _, m := range claimTokenRe.FindAllStringSubmatchIndex(line, -1) {
		prefix := line[m[2]:m[3]]
		numStr := line[m[4]:m[5]]
		base := prefix + numStr
		letter := ""
		if m[6] >= 0 {
			letter = line[m[6]:m[7]]
		}
		raw := line[m[0]:m[1]]
		end := m[1]
		if letter == "" && end < len(line) && line[end] == '(' {
			closeIdx := strings.IndexByte(line[end:], ')')
			if closeIdx > 0 {
				content := line[end+1 : end+closeIdx]
				if isEnumerator, subLetter, recognized := classifyParenthetical(content); recognized {
					raw = line[m[0] : end+closeIdx+1]
					if isEnumerator {
						out = append(out, claimToken{base: base, raw: raw})
					} else {
						out = append(out, claimToken{base: base, letter: subLetter, subLettered: true, raw: raw})
					}
					continue
				}
			}
		}
		if letter != "" {
			out = append(out, claimToken{base: base, letter: letter, subLettered: true, raw: raw})
		} else {
			out = append(out, claimToken{base: base, raw: raw})
		}
	}
	return out
}

// extractSpecTokens scans specContent for every R<n>/AC-<n> occurrence
// (bare or sub-lettered), returning the set of distinct base strings seen
// (for letterless-claim resolution: base equality only — "AC-1" and
// "AC-19" are distinct bases by construction, so a numeric-prefix claim
// never resolves via a longer number) and the set of exact base+letter
// strings seen (for sub-lettered-claim exact resolution).
func extractSpecTokens(specContent string) (bases map[string]bool, exact map[string]bool) {
	bases = map[string]bool{}
	exact = map[string]bool{}
	for _, m := range claimTokenRe.FindAllStringSubmatch(specContent, -1) {
		prefix, numStr, letter := m[1], m[2], m[3]
		base := prefix + numStr
		bases[base] = true
		if letter != "" {
			exact[base+letter] = true
		}
	}
	return bases, exact
}

func evaluateMF2(description, sectionRaw, specContent string, ownSpecNum int) Signal {
	claims := harvestClaims(description, ownSpecNum)
	claims = append(claims, harvestClaims(sectionRaw, ownSpecNum)...)

	specBases, specExact := extractSpecTokens(specContent)

	seen := map[string]bool{}
	var dangling []claimToken
	for _, c := range claims {
		key := c.base + "\x00" + c.letter
		if seen[key] {
			continue
		}
		seen[key] = true
		if c.subLettered {
			if !specExact[c.base+c.letter] {
				dangling = append(dangling, c)
			}
		} else if !specBases[c.base] {
			dangling = append(dangling, c)
		}
	}
	if len(dangling) > 0 {
		names := make([]string, 0, len(dangling))
		for _, d := range dangling {
			names = append(names, d.raw)
		}
		sort.Strings(names)
		joined := strings.Join(names, ", ")
		return Signal{
			ID:       SignalTokens,
			Pass:     false,
			Detail:   fmt.Sprintf("claimed token(s) do not resolve in spec.md: %s", joined),
			Recovery: fmt.Sprintf("resolve the dangling claim(s) %s against spec.md, or remove them from the bead's plan section / bd description", joined),
		}
	}
	return Signal{ID: SignalTokens, Pass: true}
}

// --- MF-3: dependencies closed AND landed-merged (spec 124 R2 MF-3) ---

func evaluateMF3(root, specID string, deps []dependencyEdge) Signal {
	if len(deps) == 0 {
		return Signal{ID: SignalDependencies, Pass: true}
	}
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return Signal{
			ID:       SignalDependencies,
			Pass:     false,
			Detail:   fmt.Sprintf("could not resolve the spec branch for %s: %v", idrender.Spec(specID), err),
			Recovery: "fix the spec ID, then re-run `mindspec bead ready-check`",
		}
	}
	for _, dep := range deps {
		if err := idvalidate.BeadID(dep.ID); err != nil {
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency id %s is malformed", idrender.Bead(dep.ID)),
				Recovery: fmt.Sprintf("fix the malformed dependency edge naming %s", idrender.Bead(dep.ID)),
			}
		}
		if !strings.EqualFold(strings.TrimSpace(dep.Status), "closed") {
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s is not closed in bd (status=%s)", dep.ID, dep.Status),
				Recovery: fmt.Sprintf("mindspec complete %s", dep.ID),
			}
		}
		_, lmErr := findLandedMergeFn(root, specBranch, dep.ID)
		if lmErr == nil {
			continue
		}
		var noEvidence *lifecycle.LandedMergeNoEvidence
		switch {
		case errors.As(lmErr, &noEvidence):
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s: a candidate merge exists but no admissible datum corroborates it (%v)", dep.ID, lmErr),
				Recovery: fmt.Sprintf("verify and re-attest the landed merge for %s, then re-run `mindspec bead ready-check`", dep.ID),
			}
		case errors.Is(lmErr, lifecycle.ErrLandedMergeNotFound):
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s is closed but not landed-merged into %s", dep.ID, specBranch),
				Recovery: fmt.Sprintf("mindspec complete %s", dep.ID),
			}
		default:
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s: could not evaluate landed-merge state: %v", dep.ID, lmErr),
				Recovery: fmt.Sprintf("investigate the landed-merge evaluation failure for %s, then re-run `mindspec bead ready-check`", dep.ID),
			}
		}
	}
	return Signal{ID: SignalDependencies, Pass: true}
}

// --- MF-4: no genuine blocking marker (spec 124 R2 MF-4) ---

var fencedCodeBlockRe = regexp.MustCompile("(?s)```.*?```")

// stripCodeSpans blanks out fenced code blocks and inline code spans,
// preserving line breaks (so line-based scans downstream keep their line
// numbers/boundaries) and overall byte layout, so a token appearing only
// inside a code span/fence is invisible to the token and blocking-region
// scans (spec 124 R2 MF-2/MF-4's shared code-span exclusion rule).
func stripCodeSpans(s string) string {
	s = fencedCodeBlockRe.ReplaceAllStringFunc(s, blankPreservingNewlines)
	s = stripInlineCodeSpans(s)
	return s
}

// stripInlineCodeSpans blanks CommonMark inline code spans of ANY backtick
// run length (FX-1 regression: the prior single-backtick regex left the
// PAYLOAD of a multi-backtick span — e.g. a double-backtick “ “code“ “
// — visible, so a TBD / AC-<n> / unchecked-`[ ]` inside it was not
// excluded, producing a false MF-2/MF-4 refusal). Per CommonMark, an
// opening run of N backticks is closed by the next run of EXACTLY N
// backticks; an opening run with no matching closer is literal. The whole
// span (both delimiter runs + payload) is replaced with spaces, newlines
// preserved so line-based scans keep their boundaries.
func stripInlineCodeSpans(s string) string {
	b := []byte(s)
	out := make([]byte, len(b))
	copy(out, b)
	i := 0
	for i < len(b) {
		if b[i] != '`' {
			i++
			continue
		}
		// Measure the opening backtick run.
		openStart := i
		j := i
		for j < len(b) && b[j] == '`' {
			j++
		}
		runLen := j - openStart
		// Search for a closing run of EXACTLY runLen backticks.
		matchEnd := -1
		k := j
		for k < len(b) {
			if b[k] != '`' {
				k++
				continue
			}
			runStart := k
			for k < len(b) && b[k] == '`' {
				k++
			}
			if k-runStart == runLen {
				matchEnd = k
				break
			}
			// A different-length run cannot close this span; keep scanning
			// from AFTER it (k already advanced past it).
		}
		if matchEnd == -1 {
			// No closing run: the opening backticks are literal. Skip past
			// them so a later, longer/shorter run can still open a span.
			i = j
			continue
		}
		for p := openStart; p < matchEnd; p++ {
			if out[p] != '\n' {
				out[p] = ' '
			}
		}
		i = matchEnd
	}
	return string(out)
}

func blankPreservingNewlines(match string) string {
	var b strings.Builder
	for _, r := range match {
		if r == '\n' {
			b.WriteRune('\n')
		} else {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

var tbdTokenRe = regexp.MustCompile(`(?i)\bTBD\b`)
var openQuestionRe = regexp.MustCompile(`(?i)\bOPEN QUESTION\b`)

// findBlockingToken finds the first line (outside code spans/fences) in
// text carrying a genuine TBD/OPEN QUESTION marker, returning that ORIGINAL
// (unstripped) line as evidence — so a hostile byte sequence sitting on
// the same line (e.g. embedded in a bd description) flows into the
// Signal's Detail exactly as written, to be escaped once at Render time
// (spec 124 AC-8), rather than being silently dropped by only naming the
// bare token.
func findBlockingToken(text string) (bool, string) {
	stripped := stripCodeSpans(text)
	strippedLines := strings.Split(stripped, "\n")
	origLines := strings.Split(text, "\n")
	for i, l := range strippedLines {
		if tbdTokenRe.MatchString(l) || openQuestionRe.MatchString(l) {
			orig := l
			if i < len(origLines) {
				orig = origLines[i]
			}
			return true, strings.TrimSpace(orig)
		}
	}
	return false, ""
}

var blockingHeaderRe = regexp.MustCompile(`^\*\*(Blocking Questions|Open Questions|Blocked on)\*\*$`)
var uncheckedItemRe = regexp.MustCompile(`^-\s*\[\s?\]`)

// findBlockingChecklistItem finds an unchecked "- [ ]" item under a
// designated blocking-region header inside sectionRaw. Both the header
// detection and the item scan run over the code-span-stripped text (so a
// header/marker named only inside a code span never opens/extends a
// blocking region); the returned evidence snippet is read back from the
// ORIGINAL (unstripped) line so any code-span content elsewhere on that
// same line still renders faithfully.
func findBlockingChecklistItem(sectionRaw string) (bool, string) {
	stripped := stripCodeSpans(sectionRaw)
	strippedLines := strings.Split(stripped, "\n")
	origLines := strings.Split(sectionRaw, "\n")
	inBlock := false
	for i, l := range strippedLines {
		trimmed := strings.TrimSpace(l)
		if blockingHeaderRe.MatchString(trimmed) {
			inBlock = true
			continue
		}
		if inBlock {
			if boldHeaderRe.MatchString(trimmed) || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
				inBlock = false
				continue
			}
			if uncheckedItemRe.MatchString(trimmed) {
				orig := ""
				if i < len(origLines) {
					orig = strings.TrimSpace(origLines[i])
				}
				return true, orig
			}
		}
	}
	return false, ""
}

func evaluateMF4(description, sectionRaw string) Signal {
	combined := description + "\n" + sectionRaw
	if found, line := findBlockingToken(combined); found {
		return Signal{
			ID:       SignalBlocking,
			Pass:     false,
			Detail:   fmt.Sprintf("a genuine blocking marker appears in the bead's plan section / bd description: %s", line),
			Recovery: "resolve the TBD/OPEN QUESTION marker (or remove it) in the bead's plan section / bd description",
		}
	}
	if found, item := findBlockingChecklistItem(sectionRaw); found {
		return Signal{
			ID:       SignalBlocking,
			Pass:     false,
			Detail:   fmt.Sprintf("an unchecked item under a blocking-region header: %s", item),
			Recovery: "resolve (check off, or remove) the unchecked item under the blocking-region header, then re-run",
		}
	}
	return Signal{ID: SignalBlocking, Pass: true}
}
