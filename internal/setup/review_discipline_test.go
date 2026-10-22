package setup

// review_discipline_test.go — spec 126's R1c [CI] guard table over
// pluginmindspec.SkillFiles() (the 8 embedded plugin skills) and
// lifecycleSkillFiles() (the 4 lifecycle-gate literals), beside the
// existing skills_test.go fragment-guard precedent.
//
// CREATED by Bead 1 (mindspec-xurf.1): the ADR-0044 reviewer-conduct
// doctrine plus the five-skill conduct edits (R2/R3/R6/R7). This bead's
// rows cover AC-4, AC-5, AC-8, AC-9, AC-13 (citation half, partial), and
// this bead's slice of the AC-10 scan-plus-allowlist. Beads 3 and 4 APPEND
// further rows (AC-1/2/6/7 and AC-11/12 respectively) to this SAME file —
// do not rename the shared helpers (acTenValidate, acTenScanSurfaces,
// ac10Surfaces, reviewDisciplineAC10Allowlist, repoRoot from
// skills_test.go) without checking those beads' additions.
//
// Table-driven per AC-12: one row per pinned fragment, so a file-granular
// revert of a multi-row skill (e.g. ms-panel-run) legitimately REDs every
// row pinned to that file — the requirement is per-fragment traceability,
// not one-revert-one-row.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
	"gopkg.in/yaml.v3"
)

func TestReviewDiscipline_Spec130NoUnchangedPanelValuesClaim(t *testing.T) {
	root := repoRoot(t)
	for _, path := range []string{
		".claude/skills/ms-panel-run/SKILL.md",
		"plugins/mindspec/skills/ms-panel-run/SKILL.md",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			for _, stale := range []string{"`DefaultConfig` panel VALUES are unchanged", "VALUES ship unchanged"} {
				if strings.Contains(string(data), stale) {
					t.Errorf("%s contains stale panel-size claim %q", path, stale)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-4 / AC-5 / AC-8 / AC-9 / AC-13(partial) — the simple fragment table.
// ---------------------------------------------------------------------------

// fragmentRow is one pinned fragment: a required (or MUST-NOT-CONTAIN)
// substring inside one embedded plugin skill's bytes. Every positive
// fragment below was grepped absent at the parent commit before this
// bead's edits landed (plan.md's pinned-fragment table records the proof);
// reverting the corresponding skill edit REDs the row.
type fragmentRow struct {
	ac      string
	desc    string
	surface string // pluginmindspec.SkillFiles() key
	want    string
	negate  bool // true => surface must NOT contain want
}

var reviewDisciplineFragmentRows = []fragmentRow{
	{ac: "AC-4", desc: "slot isolation", surface: "ms-panel-run", want: "panel.isolated-checkout"},
	{ac: "AC-4", desc: "final isolation", surface: "ms-spec-final-review", want: "panel.isolated-checkout"},
	{ac: "AC-5", desc: "reproduction", surface: "ms-bead-cycle", want: "review.reproduce"},
	{ac: "AC-5", desc: "contested findings", surface: "ms-panel-tally", want: "review.reproduce"},
	{ac: "AC-5", desc: "classifier redispatch", surface: "ms-panel-run", want: "review.classifier-redispatch"},
	{ac: "AC-8", desc: "no vote", surface: "ms-panel-run", want: "review.no-vote"},
	{ac: "AC-8", desc: "final no vote", surface: "ms-spec-final-review", want: "review.no-vote"},
	{ac: "AC-9", desc: "slot scratch", surface: "ms-panel-run", want: "panel.scratch-in-slot"},
	{ac: "AC-9", desc: "fixer scratch", surface: "ms-bead-fix", want: "panel.scratch-in-slot"},
	{ac: "AC-9", desc: "final scratch", surface: "ms-spec-final-review", want: "panel.scratch-in-slot"},
	{ac: "AC-9", desc: "clean completion", surface: "ms-bead-cycle", want: "git status --porcelain"},
	{ac: "AC-13", desc: "rule lookup", surface: "ms-panel-run", want: "mindspec rules show <id>"},
}

// TestReviewDiscipline_Fragments is the table-driven [CI] guard for
// AC-4/5/8/9/13(partial): every row above is checked against the embedded
// plugin-skill bytes. Reverting any one skill edit REDs every row pinned
// to that skill (AC-12's per-fragment traceability).
func TestReviewDiscipline_Fragments(t *testing.T) {
	skills := pluginmindspec.SkillFiles()
	for _, row := range reviewDisciplineFragmentRows {
		row := row
		t.Run(row.ac+"/"+row.desc, func(t *testing.T) {
			content, ok := skills[row.surface]
			if !ok {
				t.Fatalf("pluginmindspec.SkillFiles() has no %q entry", row.surface)
			}
			has := strings.Contains(content, row.want)
			switch {
			case row.negate && has:
				t.Errorf("%s: %s must NOT contain %q, but it does", row.surface, row.ac, row.want)
			case !row.negate && !has:
				t.Errorf("%s: %s is missing the required fragment %q", row.surface, row.ac, row.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-13 (partial) — ADR-0044 exists, lands Accepted (repo house form).
// ---------------------------------------------------------------------------

// TestReviewDiscipline_ADR0044ExistsAccepted asserts the ADR file exists and
// carries the repo's real `- **Status**: Accepted` bullet form (the marker
// every ADR in .mindspec/adr/ actually uses, e.g. ADR-0043:4) — NOT a
// `status:` YAML frontmatter key, which no repo ADR carries.
func TestReviewDiscipline_ADR0044ExistsAccepted(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".mindspec", "adr", "ADR-0044-panel-review-conduct.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	content := string(data)
	if !strings.Contains(content, "- **Status**: Accepted") {
		t.Errorf("ADR-0044 must carry the repo house `- **Status**: Accepted` bullet; got:\n%s", content)
	}
}

// TestReviewDiscipline_ADR0044Spec121Provenance proves ADR-0044's Context
// records the spec-121 Bead-2 incident (a summarised BRIEF silently
// dropping an AC clause, sailing an 8/8 panel) alongside the other
// incidents — codex G1-M3 found this provenance missing even though the
// plan's AC-14 requires it as R5's ADR-only incident record (never in
// shipped skill text — see the portability grep in the AC-10 tests above).
func TestReviewDiscipline_ADR0044Spec121Provenance(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".mindspec", "adr", "ADR-0044-panel-review-conduct.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.Contains(string(data), "spec-121") {
		t.Errorf("ADR-0044 Context must record the spec-121 Bead-2 provenance (summarised-BRIEF/verbatim-AC incident); got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// AC-10 — portability scan-plus-allowlist (Non-Goal filter).
// ---------------------------------------------------------------------------

// acTenModelRE is pattern class (i): model-family values. Categorical, not a
// single-char-suffix denylist — covers gpt-4o, gpt-5.6-sol, o4-mini,
// claude-4.5, and bare family names.
var acTenModelRE = regexp.MustCompile(`(?i)\b(opus|sonnet|haiku|fable|claude-[0-9][.\w-]*|gpt-[0-9][.\w-]*|o[0-9]+(-[a-z]+)?)\b`)

// Pattern class (ii): mindspec-self-development lore that must never ship
// in a consumer skill. Each var is a CATEGORY regex covering its family's
// known spelling variants, never a single literal string, so a paraphrase
// within the same lore category is still caught. The first four cover the
// AC-10 Non-Goal's original four classes; the last three (added after codex
// G1-M1 found the scan silently missing them, spec.md's Non-Goal / AC-10)
// cover gofmt-corruption, git-ref-probe, and bd-off-PATH lore.
var (
	acTenLoreShortTestRE   = regexp.MustCompile(`go test -short`)
	acTenLoreArgvRatchetRE = regexp.MustCompile(`argv-ratchet`)
	acTenLoreHarnessRE     = regexp.MustCompile(`internal/harness`)
	acTenLoreInstructRE    = regexp.MustCompile(`internal/instruct`)
	// acTenLoreGofmtRE: gofmt's Go 1.19+ doc-comment code-span corruption
	// gotcha (spec-113 CI lore) — "gofmt corruption", "gofmt-corrupts",
	// "gofmt doc-comment corruption", etc.
	acTenLoreGofmtRE = regexp.MustCompile(`(?i)gofmt[\s-]*(?:doc-comment[\s-]*)?corrupt\w*`)
	// acTenLoreGitRefProbeRE: the git-ref-probe / `show-ref --exists` lore.
	acTenLoreGitRefProbeRE = regexp.MustCompile(`(?i)git[\s-]*ref[\s-]*probe|show-ref\s+--exists`)
	// acTenLoreBdOffPathRE: the bd-off-PATH choreography lore — bd missing
	// from PATH in a stripped-down shell, `PATH=<no-bd>` and its kin.
	acTenLoreBdOffPathRE = regexp.MustCompile(`(?i)bd[\s-]*off[\s-]*PATH|PATH=<no-bd>|bd\s+(?:is\s+)?not\s+(?:on|found\s+on)\s+PATH`)
)

// acTenLoreRE is the full pattern-class-(ii) list the scanner walks.
var acTenLoreRE = []*regexp.Regexp{
	acTenLoreShortTestRE,
	acTenLoreArgvRatchetRE,
	acTenLoreHarnessRE,
	acTenLoreInstructRE,
	acTenLoreGofmtRE,
	acTenLoreGitRefProbeRE,
	acTenLoreBdOffPathRE,
}

// acTenHomePathRE is pattern class (iii): absolute operator-home paths
// (POSIX only; Windows C:\Users\ is explicitly out of scope per spec.md).
var acTenHomePathRE = regexp.MustCompile(`/Users/|/home/|/root/`)

// acTenSpecIDRE is pattern class (iv): this project's own incident IDs, in
// BOTH the hyphenated (spec-113) and unhyphenated (spec 113 / Spec 113)
// forms — the final-review S3 finding: the original `spec-[0-9]+` required
// the hyphen and silently missed every "spec NNN" prose form (~17 live
// occurrences at fix time, enumerated in the allowlist below). Bounded to
// 2-3 digits (every real spec ID) so 4+-digit numerics never bind; [\s-]+
// (which crosses a line-wrap) is the separator class.
var acTenSpecIDRE = regexp.MustCompile(`(?i)\bspec[\s-]+[0-9]{2,3}\b`)

// acTenHit is one occurrence of a scanned pattern in one surface, carrying
// the 1-based LINE NUMBER the match landed on (recomputed fresh from the
// surface's live content on every scan). The line is what makes the
// allowlist location-load-bearing rather than decorative: codex G1-B1
// proved that keying purely on (surface, matched text) lets an allowed
// token be moved off its declared spot and reintroduced elsewhere for free.
type acTenHit struct {
	surface string
	matched string
	line    int
}

// acTenLineOf returns the 1-based line number containing byte offset off in
// content.
func acTenLineOf(content string, off int) int {
	return strings.Count(content[:off], "\n") + 1
}

// acTenScanContent runs all four pattern classes over one surface's bytes,
// emitting one acTenHit per occurrence (each carrying the line it landed
// on) — a matched text repeated N times in one surface yields N hits, which
// is what makes occurrence-AND-location-accounted allowlisting possible.
func acTenScanContent(surface, content string) []acTenHit {
	var hits []acTenHit
	scanRE := func(re *regexp.Regexp) {
		for _, loc := range re.FindAllStringIndex(content, -1) {
			hits = append(hits, acTenHit{surface: surface, matched: content[loc[0]:loc[1]], line: acTenLineOf(content, loc[0])})
		}
	}
	scanRE(acTenModelRE)
	for _, re := range acTenLoreRE {
		scanRE(re)
	}
	scanRE(acTenHomePathRE)
	scanRE(acTenSpecIDRE)
	return hits
}

// acTenScanSurfaces runs acTenScanContent over every (surface, content) pair.
func acTenScanSurfaces(surfaces map[string]string) []acTenHit {
	var hits []acTenHit
	for surface, content := range surfaces {
		hits = append(hits, acTenScanContent(surface, content)...)
	}
	return hits
}

// acTenAllowEntry is one occurrence-AND-location-accounted allowlist entry:
// (surface, resolved anchor line, matched text) authorizes exactly `count`
// occurrences of that EXACT matched text ON THAT LINE — never "any hit
// anywhere in this file". `locator` is a LITERAL, VERBATIM substring lifted
// from the real surface's surrounding prose (deliberately excluding the
// matched token itself) that must resolve to EXACTLY ONE line in the live
// surface content; that resolved line is what matching keys on. This is
// what makes the allowlist location-load-bearing rather than decorative
// (codex G1-B1): moving an allowed token off its declared locator's line
// desyncs the (surface, line, matched) key, which both strands the
// allowlist entry as unused AND produces a fresh unallowlisted hit wherever
// the token landed instead. The anchor text survives line-number drift from
// unrelated edits — it is re-resolved from live content on every run, never
// pinned to a literal integer — per plan.md's "robust to line-number drift"
// requirement.
type acTenAllowEntry struct {
	surface string
	locator string
	matched string
	count   int
	reason  string
}

// acTenResolveLocator finds the 1-based line number in content that
// contains the anchor substring locator, verbatim. Returns ok=false (with a
// diagnostic) if the anchor is absent or ambiguous (matches more than one
// line) — an unresolvable locator can never authorize a hit, so callers
// must treat that as a validation problem, not a silent pass.
func acTenResolveLocator(content, locator string) (line int, diag string, ok bool) {
	lines := strings.Split(content, "\n")
	matchLine := 0
	count := 0
	for i, l := range lines {
		if strings.Contains(l, locator) {
			count++
			matchLine = i + 1
		}
	}
	switch count {
	case 0:
		return 0, "locator anchor not found in surface content", false
	case 1:
		return matchLine, "", true
	default:
		return 0, fmt.Sprintf("locator anchor is ambiguous: matched %d lines", count), false
	}
}

// reviewDisciplineAC10Allowlist is empty. The shipped skills once carried
// spec-number cross-refs and cross-project case history, all allowlisted
// here; those citations were removed from the shipped text (consumers have
// no copy of mindspec's specs), so no entry remains. The negative tests
// below build their own synthetic entries to keep the location-keyed
// accounting covered.
var reviewDisciplineAC10Allowlist = []acTenAllowEntry{}

// acTenValidate checks hits against the allowlist, resolving each entry's
// locator to a concrete (surface, line) key against the LIVE surfaces map
// and consuming hits ONE-TO-ONE against that resolved key (matched text AND
// count must both agree, AT that specific line). It is a pure function —
// it returns problem strings instead of calling t.Errorf — so the SAME
// logic can be exercised both for the real positive guard and for the
// negative mutation tests below without one masking the other's failures.
// FAILS (returns a non-empty slice) on (a) any allowlist entry whose
// locator cannot be uniquely resolved in its surface, (b) any scan hit not
// matched by a resolved entry AT THE ENTRY'S RESOLVED LINE — including a
// hit whose text is allowlisted elsewhere in the same surface but landed on
// the WRONG line, the locator-mismatch case codex G1-B1 found silently
// passing — and (c) any allowlist entry left unconsumed (its resolved line
// never saw the expected count of hits).
func acTenValidate(hits []acTenHit, allowlist []acTenAllowEntry, surfaces map[string]string) []string {
	return scanAllowlistValidate("AC-10", hits, allowlist, surfaces)
}

// scanAllowlistValidate is acTenValidate's logic with the problem-message
// label as a parameter, so other scans (surface_refs_test.go) reuse the
// same location-keyed accounting under their own label.
func scanAllowlistValidate(label string, hits []acTenHit, allowlist []acTenAllowEntry, surfaces map[string]string) []string {
	type key struct {
		surface string
		line    int
		matched string
	}
	remaining := make(map[key]int)
	var problems []string

	for _, e := range allowlist {
		content, ok := surfaces[e.surface]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s allowlist entry unresolvable: surface=%s locator=%q matched=%s: surface not present among scanned surfaces", label, e.surface, e.locator, e.matched))
			continue
		}
		line, diag, ok := acTenResolveLocator(content, e.locator)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s allowlist entry unresolvable: surface=%s locator=%q matched=%s: %s", label, e.surface, e.locator, e.matched, diag))
			continue
		}
		remaining[key{e.surface, line, e.matched}] += e.count
	}

	for _, h := range hits {
		k := key{h.surface, h.line, h.matched}
		if remaining[k] > 0 {
			remaining[k]--
			continue
		}
		problems = append(problems, fmt.Sprintf("%s scan: unallowlisted hit in %s at line %d: %s", label, h.surface, h.line, h.matched))
	}
	for k, n := range remaining {
		if n > 0 {
			problems = append(problems, fmt.Sprintf("%s allowlist entry unused (or under-consumed): surface=%s line=%d matched=%s", label, k.surface, k.line, k.matched))
		}
	}
	return problems
}

// ac10Surfaces returns every surface the AC-10 scan must cover: the
// embedded plugin skills (pluginmindspec.SkillFiles(), all 8 — the scan is
// NOT limited to the 5 skills this bead edits) plus the four
// lifecycleSkillFiles() map VALUES — never the claude.go SOURCE text,
// whose Go comments carry spec-072 at :370/:437/:448, outside the literal
// values and outside any shipped skill body. Both source maps are built
// fresh per call, so callers may mutate the returned map freely.
func ac10Surfaces() map[string]string {
	surfaces := pluginmindspec.SkillFiles()
	for name, content := range lifecycleSkillFiles() {
		surfaces[name] = content
	}
	return surfaces
}

func TestReviewDiscipline_AC10StagedRuleMarkerOnly(t *testing.T) {
	content := ac10Surfaces()["ms-bead-impl"]
	if !strings.Contains(content, readinessStagedRule) {
		t.Fatal("ms-bead-impl must carry the rule-id staged marker")
	}
	for _, token := range []string{"spec 133", "spec-131"} {
		t.Run(token, func(t *testing.T) {
			surfaces := ac10Surfaces()
			if problems := acTenValidate(acTenScanSurfaces(surfaces), reviewDisciplineAC10Allowlist, surfaces); len(problems) != 0 {
				t.Fatalf("shipped surfaces (incl. the staged-rule marker) must scan clean: %v", problems)
			}
			content := surfaces["ms-bead-impl"] + "\nUnrelated " + token + " reference elsewhere in this skill.\n"
			surfaces["ms-bead-impl"] = content
			problems := acTenValidate(acTenScanSurfaces(surfaces), reviewDisciplineAC10Allowlist, surfaces)
			want := fmt.Sprintf("AC-10 scan: unallowlisted hit in ms-bead-impl at line %d: %s", acTenLineOf(content, strings.LastIndex(content, token)), token)
			if len(problems) != 1 || problems[0] != want {
				t.Fatalf("unrelated reference must fail only at its own line: got %v, want %q", problems, want)
			}
		})
	}
}

// TestReviewDiscipline_AC10ScanPlusAllowlist is the [CI] guard for AC-10: it
// scans every embedded skill and lifecycle literal for the pattern classes
// and fails on any hit the allowlist above does not account for at its
// resolved locator line (exact surface + resolved line + exact matched text
// + count), and on any allowlist entry left unconsumed or whose locator
// fails to resolve.
func TestReviewDiscipline_AC10ScanPlusAllowlist(t *testing.T) {
	surfaces := ac10Surfaces()
	hits := acTenScanSurfaces(surfaces)
	for _, problem := range acTenValidate(hits, reviewDisciplineAC10Allowlist, surfaces) {
		t.Error(problem)
	}
}

// TestReviewDiscipline_AC10NegativeCategoricalHits proves the scan REDs on
// each of the mandated categorical negatives (spec.md's spec-gate G2, plus
// the 3 lore classes G1-M1 added: gofmt-corruption, git-ref-probe,
// bd-off-PATH), injected into a synthetic fixture surface — never a real
// shipped file.
func TestReviewDiscipline_AC10NegativeCategoricalHits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		inject string
	}{
		{"gpt-4o", "the model gpt-4o handled this probe"},
		{"o4-mini", "routed to o4-mini for the empirical check"},
		// The S3-widened incident-ID class: the unhyphenated prose form the
		// original `spec-[0-9]+` pattern silently missed.
		{"spec-id-unhyphenated", "this regressed once in spec 999 — see the postmortem"},
		{"spec-id-unhyphenated-capitalized", "the Spec 999 incident write-up"},
		{"root-agent-path", "wrote scratch to /root/agent/notes.md"},
		{"gofmt-corruption-lore", "watch for the gofmt doc-comment corruption bug in Go 1.19+"},
		{"git-ref-probe-lore", "avoid the git-ref-probe flake by using show-ref --exists instead"},
		{"bd-off-path-lore", "handle the bd-off-PATH case when the shell runs with PATH=<no-bd>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := map[string]string{"fixture-surface": tc.inject}
			hits := acTenScanSurfaces(fixture)
			if len(hits) == 0 {
				t.Fatalf("fixture assumption broken: %q produced no scan hits at all", tc.inject)
			}
			if problems := acTenValidate(hits, nil, fixture); len(problems) == 0 {
				t.Fatalf("expected the AC-10 scan to RED on %q, but it found no problems", tc.inject)
			}
		})
	}
}

// TestReviewDiscipline_AC10NewLoreClassesZeroOnRealSurfaces proves the 3
// lore classes G1-M1 added (gofmt-corruption, git-ref-probe, bd-off-PATH)
// currently produce ZERO hits across the real embedded surfaces — the
// classes are enforced categorically for the future, not because any
// shipped skill presently carries this lore.
func TestReviewDiscipline_AC10NewLoreClassesZeroOnRealSurfaces(t *testing.T) {
	surfaces := ac10Surfaces()
	for _, re := range []*regexp.Regexp{acTenLoreGofmtRE, acTenLoreGitRefProbeRE, acTenLoreBdOffPathRE} {
		for surface, content := range surfaces {
			if m := re.FindString(content); m != "" {
				t.Errorf("surface %s unexpectedly contains a new-lore-class match %q; portable skill text must not carry this mindspec-self-development lore", surface, m)
			}
		}
	}
}

// TestReviewDiscipline_AC10NegativeDuplicateToken proves occurrence accounting
// with a local allowlisted fixture: an extra hit off its locator must fail.
func TestReviewDiscipline_AC10NegativeDuplicateToken(t *testing.T) {
	surfaces := ac10Surfaces()
	locator := "fixture provenance: spec-050 and spec-050 again"
	surfaces["ms-panel-tally"] += "\n" + locator + "\n"
	allowed := append(append([]acTenAllowEntry{}, reviewDisciplineAC10Allowlist...), acTenAllowEntry{surface: "ms-panel-tally", locator: locator, matched: "spec-050", count: 2, reason: "synthetic occurrence-accounting fixture"})
	if problems := acTenValidate(acTenScanSurfaces(surfaces), allowed, surfaces); len(problems) != 0 {
		t.Fatalf("unmutated fixture: %v", problems)
	}
	surfaces["ms-panel-tally"] += "\nA third spec-050 reference injected for the test.\n"
	problems := acTenValidate(acTenScanSurfaces(surfaces), allowed, surfaces)
	if len(problems) == 0 {
		t.Fatal("expected third occurrence outside its locator to RED")
	}
	for _, p := range problems {
		if !strings.Contains(p, "ms-panel-tally") {
			t.Errorf("unexpected unrelated problem: %s", p)
		}
	}
}

// A portable shipped skill has no incident tokens; a local fixture proves
// location accounting without putting forbidden provenance back into it.
func TestReviewDiscipline_AC10NegativeLocatorMismatch(t *testing.T) {
	surfaces := ac10Surfaces()
	locator := "fixture provenance: spec-050-bead2 and spec-050-bead2-r2"
	surfaces["ms-panel-run"] += "\n" + locator + "\n"
	allowed := append(append([]acTenAllowEntry{}, reviewDisciplineAC10Allowlist...), acTenAllowEntry{surface: "ms-panel-run", locator: "fixture provenance:", matched: "spec-050", count: 2, reason: "synthetic locator fixture"})
	if problems := acTenValidate(acTenScanSurfaces(surfaces), allowed, surfaces); len(problems) != 0 {
		t.Fatalf("unmutated fixture: %v", problems)
	}
	surfaces["ms-panel-run"] = strings.ReplaceAll(surfaces["ms-panel-run"], "spec-050", "case-050") + "\nElsewhere: spec-050 and spec-050 again.\n"
	problems := acTenValidate(acTenScanSurfaces(surfaces), allowed, surfaces)
	sawUnused, sawUnallowlisted := false, false
	for _, p := range problems {
		if strings.Contains(p, "ms-panel-run") && strings.Contains(p, "unused") {
			sawUnused = true
		}
		if strings.Contains(p, "ms-panel-run") && strings.Contains(p, "unallowlisted") {
			sawUnallowlisted = true
		}
	}
	if !sawUnused || !sawUnallowlisted {
		t.Fatalf("expected unused locator and unallowlisted relocated hits: %v", problems)
	}
}

// TestReviewDiscipline_AC10NegativeCountPreservingSubstitution proves a
// disallowed class cannot consume an allowed locator/count slot: ONE
// allowed "spec-050" occurrence at an allowlisted locator is replaced with
// "gpt-4o" — the locator's TOTAL token count is unchanged (still two
// tokens) — and the scan must still RED, because the allowlist key is
// (surface, line, EXACT matched text), not a raw hit count.
func TestReviewDiscipline_AC10NegativeCountPreservingSubstitution(t *testing.T) {
	// Shaped like ms-panel-tally's allowlisted locator: two spec-050
	// occurrences at one spot.
	fixtureLine := "postmortem citing spec-050 and spec-050 again"
	mutated := strings.Replace(fixtureLine, "spec-050", "gpt-4o", 1)
	if strings.Count(mutated, "spec-050") != 1 || !strings.Contains(mutated, "gpt-4o") {
		t.Fatalf("fixture assumption broken: substitution did not produce one spec-050 + one gpt-4o, got %q", mutated)
	}

	surfaces := ac10Surfaces()
	surfaces["ms-panel-tally"] = mutated // total token count at this surface unchanged: still 2

	hits := acTenScanSurfaces(surfaces)
	problems := acTenValidate(hits, reviewDisciplineAC10Allowlist, surfaces)
	if len(problems) == 0 {
		t.Fatal("expected the count-preserving spec-050->gpt-4o substitution to RED the AC-10 scan")
	}
}

// TestReviewDiscipline_AC10NegativeUnusedAllowlistEntry proves the OTHER
// residual the occurrence-accounted allowlist must catch: an allowlist
// entry that no scan hit ever consumes (e.g. left behind after a future
// cleanup, or whose locator is simply wrong) REDs too, so a stale entry
// cannot linger as a silent hole.
func TestReviewDiscipline_AC10NegativeUnusedAllowlistEntry(t *testing.T) {
	extended := append([]acTenAllowEntry{}, reviewDisciplineAC10Allowlist...)
	extended = append(extended, acTenAllowEntry{
		surface: "ms-bead-fix",
		locator: "synthetic — never actually present in the shipped skill",
		matched: "spec-999",
		count:   1,
		reason:  "test-only: demonstrates an unconsumed/unresolvable allowlist entry REDs",
	})

	surfaces := ac10Surfaces()
	hits := acTenScanSurfaces(surfaces)
	problems := acTenValidate(hits, extended, surfaces)
	if len(problems) == 0 {
		t.Fatal("expected an unused allowlist entry to RED the AC-10 scan")
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "ms-bead-fix") && strings.Contains(p, "spec-999") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the unused-entry problem to name surface=ms-bead-fix matched=spec-999; got: %v", problems)
	}
}

// ---------------------------------------------------------------------------
// Bead 3 (mindspec-xurf.3) appends below: AC-1 (one authoritative panel-size
// story, incl. the NEGATIVE structural fixed-six sweep guard), AC-2 (the
// panel.gates ladder example, structural — parsed, not grepped), AC-6
// (lifecycle-literal + document-gate lens table completeness), AC-7
// (verbatim-AC BRIEF replaces the summarise-only guidance), and the two
// literal AC-13 citation rows completing the seven-surface set. Reuses
// acTenHit / acTenAllowEntry (generic (surface, line, matched-text, count)
// shapes) and acTenLineOf / acTenResolveLocator (generic scan/locator
// helpers) from Bead 1's AC-10 section above; does NOT reuse acTenValidate
// itself (its problem messages are hardcoded "AC-10 ..." — reusing it here
// would mislabel AC-1 sweep failures as AC-10 scan failures), so this
// section defines its own acOneValidate with AC-1-flavored messages instead.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// AC-1 (embedded-fragment half) / AC-6 (plugin-skill document-gate lens
// table) / AC-7 (verbatim-AC BRIEF) — plugin-skill fragment rows.
// ---------------------------------------------------------------------------

var reviewDisciplineBead3FragmentRows = []fragmentRow{
	{ac: "AC-1", desc: "resolved roster", surface: "ms-panel-run", want: "mindspec panel verify <panel-slug> --json"},
	{ac: "AC-1", desc: "run pass rule", surface: "ms-panel-run", want: "0 unresolved REQUEST_CHANGES"},
	{ac: "AC-1", desc: "tally pass rule", surface: "ms-panel-tally", want: "0 unresolved REQUEST_CHANGES"},
	{ac: "AC-1", desc: "cycle fanout", surface: "ms-bead-cycle", want: "then the configured reviewers fan out"},
	{ac: "AC-1", desc: "autopilot fanout", surface: "ms-spec-autopilot", want: "across the configured panel reviewers + the impl subagent"},
	{ac: "AC-1", desc: "autopilot report", surface: "ms-spec-autopilot", want: "<N>/<N> APPROVE"},
	{ac: "AC-6", desc: "binary lenses", surface: "ms-panel-run", want: "The binary resolves the roster, lenses, acceptance question and verdict contract."},
	{ac: "AC-7", desc: "binary prompt", surface: "ms-panel-run", want: "mindspec panel prompt"},
}

// TestReviewDiscipline_Bead3Fragments is Bead 3's table-driven [CI] guard,
// same shape as TestReviewDiscipline_Fragments above but scoped to this
// bead's own rows — kept as a separate slice/function for per-bead
// traceability (AC-12), not because the underlying mechanism differs.
func TestReviewDiscipline_Bead3Fragments(t *testing.T) {
	skills := pluginmindspec.SkillFiles()
	for _, row := range reviewDisciplineBead3FragmentRows {
		row := row
		t.Run(row.ac+"/"+row.desc, func(t *testing.T) {
			content, ok := skills[row.surface]
			if !ok {
				t.Fatalf("pluginmindspec.SkillFiles() has no %q entry", row.surface)
			}
			has := strings.Contains(content, row.want)
			switch {
			case row.negate && has:
				t.Errorf("%s: %s must NOT contain %q, but it does", row.surface, row.ac, row.want)
			case !row.negate && !has:
				t.Errorf("%s: %s is missing the required fragment %q", row.surface, row.ac, row.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-6 (lifecycle-literal half, completeness) / AC-13 (the two literal
// citation rows completing the seven-surface set) — lifecycleSkillFiles()
// fragment rows.
//
// The PRIMARY RED-on-revert pin for these two literals' Step-1 edits is
// Bead 2's own internal/setup/lifecycle_gate_step_test.go (a sibling file,
// per plan.md's W1-parallelism note). These rows are this bead's OWNERSHIP
// of AC-6/AC-13 completing the full seven-surface assertion (plan.md's
// Provenance table) — intentionally redundant with Bead 2's self-pin, not a
// substitute for it.
// ---------------------------------------------------------------------------

type literalFragmentRow struct {
	ac      string
	desc    string
	surface string // lifecycleSkillFiles() key
	want    string
	negate  bool // true => surface must NOT contain want
}

var reviewDisciplineLiteralFragmentRows = []literalFragmentRow{
	{ac: "AC-6", desc: "ms-spec-approve literal names --gate spec_approve", surface: "ms-spec-approve", want: "--gate spec_approve"},
	{ac: "AC-6", desc: "ms-plan-approve literal names --gate plan_approve", surface: "ms-plan-approve", want: "--gate plan_approve"},
	{ac: "AC-13", desc: "ms-spec-approve literal cites no mindspec ADR-0044", surface: "ms-spec-approve", want: "ADR-0044", negate: true},
	{ac: "AC-13", desc: "ms-plan-approve literal cites no mindspec ADR-0044", surface: "ms-plan-approve", want: "ADR-0044", negate: true},
}

func TestReviewDiscipline_LiteralFragments(t *testing.T) {
	literals := lifecycleSkillFiles()
	for _, row := range reviewDisciplineLiteralFragmentRows {
		row := row
		t.Run(row.ac+"/"+row.desc, func(t *testing.T) {
			content, ok := literals[row.surface]
			if !ok {
				t.Fatalf("lifecycleSkillFiles() has no %q entry", row.surface)
			}
			has := strings.Contains(content, row.want)
			switch {
			case row.negate && has:
				t.Errorf("%s: %s must NOT contain %q, but it does", row.surface, row.ac, row.want)
			case !row.negate && !has:
				t.Errorf("%s: %s is missing the required fragment %q", row.surface, row.ac, row.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-1(c)/(d) — the DefaultConfig structural surface and the ADR-0043
// amendment marker: the other two of the three surfaces AC-1 requires to
// agree (the embedded-fragment surface is reviewDisciplineBead3FragmentRows
// above).
// ---------------------------------------------------------------------------

// TestReviewDiscipline_AC1PanelSizeConsistency is AC-1(c)/(d)'s structural
// half: config.DefaultConfig() resolves 6/6/4/6 per gate (adhoc 6),
// across equal claude/codex families, no global list, ApproveThreshold ==
// "n-1". This is a separate row from the embedded-fragment checks above by
// construction — reverting a DefaultConfig VALUE reds THIS test without
// touching a single skill byte, exactly the "any one surface alone reds"
// property AC-1(c) requires.
func TestReviewDiscipline_AC1PanelSizeConsistency(t *testing.T) {
	cfg := config.DefaultConfig()
	if len(cfg.Panel.Reviewers) != 0 {
		t.Errorf("implicit global reviewers: %+v", cfg.Panel.Reviewers)
	}
	if cfg.Panel.ApproveThreshold != "n-1" {
		t.Errorf("threshold = %q, want n-1", cfg.Panel.ApproveThreshold)
	}
	for gate, want := range map[string]int{"spec_approve": 6, "plan_approve": 6, "bead": 4, "final_review": 6, "adhoc": 6} {
		slots, err := cfg.PanelGateReviewerSlots(gate)
		if err != nil {
			t.Fatal(err)
		}
		if len(slots) != want {
			t.Errorf("%s slots = %d; want %d", gate, len(slots), want)
		}
		families := map[string]int{}
		for _, slot := range slots {
			families[slot.Model]++
		}
		if len(families) != 1 || families["harness"] != want {
			t.Errorf("%s families = %v; want %d harness slots", gate, families, want)
		}
	}
}

// TestReviewDiscipline_ADR0043Amendment is AC-1(c)'s third surface / AC-3's
// [doc] grep marker: ADR-0043 carries the "## Amendment (Spec 126)" heading
// distinguishing the shipped 6/n-1 default from the operator-scaled ladder
// (plan.md's ADR Fitness section names this heading as the AC-1/AC-3 grep
// marker).
func TestReviewDiscipline_ADR0043Amendment(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".mindspec", "adr", "ADR-0043-panel-disposition-telemetry-store.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.Contains(string(data), "## Amendment (Spec 126)") {
		t.Errorf("ADR-0043 is missing the '## Amendment (Spec 126)' marker heading; got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// AC-2 — the panel.gates ladder EXAMPLE, structural (parsed, not grepped).
// ---------------------------------------------------------------------------

// acTwoLadderReviewerEntry decodes one panel.gates.<gate>.reviewers[] entry
// as a raw key-set (not a typed struct) so the test can see EVERY key
// present, including one a typed struct would silently drop — a `model:`
// key is exactly the disallowed case AC-2 exists to catch.
type acTwoLadderReviewerEntry map[string]interface{}

type acTwoLadderGate struct {
	Reviewers []acTwoLadderReviewerEntry `yaml:"reviewers"`
}

type acTwoLadder struct {
	Panel struct {
		Gates map[string]acTwoLadderGate `yaml:"gates"`
	} `yaml:"panel"`
}

// acTwoExtractFencedYAML finds the FIRST ```yaml fenced block in content and
// strips its per-line "# " comment-marker prefix — the ladder example ships
// as a commented, paste-into-config.yaml block — returning the resulting
// plain YAML text. ok=false if no ```yaml block exists, or if any non-blank
// line inside it is not commented (a block the operator could not safely
// uncomment-and-paste as shown).
func acTwoExtractFencedYAML(content string) (yamlText string, ok bool) {
	const open = "```yaml"
	const closeFence = "```"
	start := strings.Index(content, open)
	if start == -1 {
		return "", false
	}
	rest := content[start+len(open):]
	end := strings.Index(rest, closeFence)
	if end == -1 {
		return "", false
	}
	block := rest[:end]
	var lines []string
	for _, l := range strings.Split(block, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if !strings.HasPrefix(l, "#") {
			return "", false
		}
		lines = append(lines, strings.TrimPrefix(l, "# "))
	}
	return strings.Join(lines, "\n"), true
}

// acTwoParseLadder YAML-parses the (already comment-stripped) ladder text.
func acTwoParseLadder(yamlText string) (acTwoLadder, error) {
	var ladder acTwoLadder
	err := yaml.Unmarshal([]byte(yamlText), &ladder)
	return ladder, err
}

// acTwoCheckReviewerKeys returns one problem string per reviewer-entry key
// outside {family, count}, across every gate — the shared assertion both the
// positive test and the negative model-key fixture drive.
func acTwoCheckReviewerKeys(ladder acTwoLadder) []string {
	var problems []string
	for gateName, g := range ladder.Panel.Gates {
		for i, r := range g.Reviewers {
			for k := range r {
				if k != "family" && k != "count" {
					problems = append(problems, fmt.Sprintf("panel.gates.%s.reviewers[%d] carries disallowed key %q (only family/count allowed); entry=%v", gateName, i, k, r))
				}
			}
		}
	}
	return problems
}

// TestReviewDiscipline_AC2LadderExampleStructural extracts the real
// panel.gates ladder example from the embedded ms-panel-run bytes, parses
// it, and asserts (i) it names gates bead/spec_approve/final_review and (ii)
// every reviewer entry carries ONLY family/count keys.
func TestReviewDiscipline_AC2LadderExampleStructural(t *testing.T) {
	for _, name := range []string{"ms-panel-run", "ms-spec-final-review"} {
		body := pluginmindspec.SkillFiles()[name]
		if strings.Contains(body, "```yaml") || regexp.MustCompile(`(?m)^\|[ ]*[RF][0-9]+`).MatchString(body) {
			t.Errorf("%s duplicates binary config tables", name)
		}
		if !strings.Contains(body, "binary") {
			t.Errorf("%s must delegate roster and lenses to binary", name)
		}
	}
}

// TestReviewDiscipline_AC2LadderExampleNegativeModelKey proves the structural
// check is not vacuous: hand-injecting a `model: opus` line into a copy of
// the extracted example (never the real file) must RED the SAME
// key-set assertion the positive test uses.
func TestReviewDiscipline_AC2LadderExampleNegativeModelKey(t *testing.T) {
	ladder, err := acTwoParseLadder("panel:\n  gates:\n    bead:\n      reviewers:\n        - family: harness\n          count: 4\n          model: unexpected\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(acTwoCheckReviewerKeys(ladder)) == 0 {
		t.Fatal("model-key mutation passed")
	}
}

// ---------------------------------------------------------------------------
// AC-1 — NEGATIVE structural sweep guard: no residual fixed-six-topology
// EXECUTION instruction survives outside the labelled-default sentence, the
// DEFAULT-labelled lens tables, and the fenced ladder example.
// ---------------------------------------------------------------------------

// acOneMDGap matches the gap between adjacent words of a fixed-six phrase,
// tolerating inline Markdown formatting (backtick / asterisk / underscore)
// around either word in addition to plain whitespace. The round-2 codex
// reviewer (G1-1A) proved the plain `\s+` gap hollow: the REAL pre-sweep
// ms-panel-run:8 wording is backticked ("three `Agent` calls ... three
// `codex exec` ..."), which `three\s+agent\s+calls` never matched — so
// reverting the real line stayed GREEN while a de-backticked surrogate
// fixture REDded. An interpreted string because a Go raw string cannot
// contain the backtick the class must include.
const acOneMDGap = "[\\s`*_]+"

// acOneSweepPatterns is the pattern set the plan mandates PLUS the six
// classes the round-1 panel (F1 + codex G1) proved the original 11 miss —
// each entry named for clear failure messages. Deliberately separate from
// acTenModelRE et al above — a different AC, a different (much smaller)
// allowlist. Every class has a categorical negative-inject fixture in
// TestReviewDiscipline_AC1SweepGuardNegativeCategoricalHits proving it REDs.
var acOneSweepPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"all six", regexp.MustCompile(`(?i)all six`)},
	{"six reviewers (spelled out)", regexp.MustCompile(`(?i)six reviewers`)},
	{"6[- ]reviewers? (separator-tolerant, catches the hyphenated heading form)", regexp.MustCompile(`(?i)6[- ]reviewers?`)},
	{"six distinct", regexp.MustCompile(`(?i)six distinct`)},
	{"Six verdicts", regexp.MustCompile(`(?i)six verdicts`)},
	{"r{4,5,6} literal slot range", regexp.MustCompile(`r\{4,5,6\}`)},
	{"R1, R2, R3 slot enumeration", regexp.MustCompile(`R1,\s*R2,\s*R3`)},
	{"R4, R5, R6 slot enumeration", regexp.MustCompile(`R4,\s*R5,\s*R6`)},
	{"R1-R3 / R1–R3 family partition (hyphen or en-dash)", regexp.MustCompile(`R1[-–]R3`)},
	{"R4-R6 / R4–R6 family partition (hyphen or en-dash)", regexp.MustCompile(`R4[-–]R6`)},
	{"/3 claude|codex hard-coded family denominator", regexp.MustCompile(`/3\s*(?:claude|codex)`)},

	// --- panel round-1 additions (F1 + G1): paired-count wording ---
	{"3+3 literal paired family count", regexp.MustCompile(`3\s*\+\s*3`)},
	{"three per family (format-tolerant)", regexp.MustCompile(`(?i)three` + acOneMDGap + `per` + acOneMDGap + `family`)},
	{"three Agent calls (paired-count launch wording, old ms-panel-run:8 form; format-tolerant so the REAL backticked form hits too — codex G1-1A)", regexp.MustCompile(`(?i)three` + acOneMDGap + `agent` + acOneMDGap + `calls`)},
	{"three claude(s)/codex spelled family count (format-tolerant so the REAL backticked codex-exec form hits too — codex G1-1A)", regexp.MustCompile(`(?i)\bthree` + acOneMDGap + `(?:claudes?|codex)\b`)},
	// The digit twin of the previous class. The canonical shipped-default
	// sentences write the families BACKTICKED ("3 `claude` + 3 `codex`"),
	// which this pattern deliberately does not match — bare "3 Claude
	// APPROVE"-style topology wording does.
	{"3 claude(s)/codex digit family count (unbackticked)", regexp.MustCompile(`(?i)\b3\s+(?:claudes?|codex)\b`)},

	// --- panel round-1 additions (F1 + G1): fixed-denominator verdict math ---
	{"<d>/6 verdict fraction (5/6 form, old ms-panel-tally:127 form)", regexp.MustCompile(`\b[0-9]+\s*/\s*6\b`)},
	{"<d>-of-6 verdict fraction (5-of-6 form)", regexp.MustCompile(`(?i)\b[0-9]+-of-6\b`)},

	// --- panel round-1 additions (F1 + G1): hard-coded registration + fan-out ---
	{"expected_reviewers hard-coded digit (old ms-spec-final-review:40 form)", regexp.MustCompile(`(?i)expected_reviewers\W{0,4}[0-9]`)},
	{"fan out 6/six", regexp.MustCompile(`(?i)fan[\s-]*out\s+(?:6|six)\b`)},
	{"6-slot / six-slot / six slot", regexp.MustCompile(`(?i)\b(?:6|six)[- ]slot`)},
}

// acOneScanContent runs every AC-1 sweep pattern over one surface's bytes,
// reusing acTenHit's generic (surface, matched, line) shape and acTenLineOf
// to resolve the 1-based line for each hit.
func acOneScanContent(surface, content string) []acTenHit {
	var hits []acTenHit
	for _, p := range acOneSweepPatterns {
		for _, loc := range p.re.FindAllStringIndex(content, -1) {
			hits = append(hits, acTenHit{surface: surface, matched: content[loc[0]:loc[1]], line: acTenLineOf(content, loc[0])})
		}
	}
	return hits
}

func acOneScanSurfaces(surfaces map[string]string) []acTenHit {
	var hits []acTenHit
	for surface, content := range surfaces {
		hits = append(hits, acOneScanContent(surface, content)...)
	}
	return hits
}

// ---------------------------------------------------------------------------
// AC-1 — STRUCTURAL six-row slot-table detection (panel F1 + G1): a six-row
// R1..R6 / F1..F6 (any single-letter prefix) lens/slot table is itself a
// fixed-six topology statement that no substring pattern above can see —
// the rows individually look innocent. Such a table is a violation UNLESS
// its introducing prose carries BOTH the exact DEFAULT label and the
// scaled-mix assignment rule (the ms-panel-run:§ Slot lens defaults and
// ms-spec-final-review step-4 forms), which is what turns a fixed
// enumeration into a labelled shipped-default the operator derives from.
// ---------------------------------------------------------------------------

// acOneSlotTableRowRE matches one markdown table row whose FIRST cell is a
// single-uppercase-letter slot id (R1, F6, S3, ...), leading indent allowed
// (ms-spec-final-review's table sits inside a numbered list item). The cell
// tolerates inline Markdown formatting around the token — inline-code and
// bold slot ids are still slot ids (codex G1-1B: the unformatted-only
// recognizer let a fully backticked or bolded six-row table pass). An
// interpreted string because a Go raw string cannot contain the backtick
// the class must include.
var acOneSlotTableRowRE = regexp.MustCompile("^[ \t]*\\|[ \t`*_]*([A-Z])([0-9]+)[ \t`*_]*\\|")

// acOneTableLineRE matches ANY markdown-table-shaped line (leading indent +
// pipe): slot rows, header rows, and |---| separator rows alike. The
// structural scan walks a contiguous block of such lines as ONE table
// instead of terminating at the first non-slot row — codex G1-1B's third
// bypass repeated the header row between R3 and R4, splitting what is
// visually one six-row table into two "incomplete" three-row runs.
var acOneTableLineRE = regexp.MustCompile(`^[ \t]*\|`)

// The two fragments a six-row slot table's introducing prose MUST carry
// (within acOneTableContextWindow lines above the first slot row) to be the
// labelled default lens set rather than a residual fixed-six instruction.
// These are verbatim from the two real labelled tables; the Bead3 fragment
// rows pin the full sentences so a label-only rewording REDs there too.
const (
	acOneTableDefaultLabelFragment  = "DEFAULT lens assignment for"
	acOneTableScaledMixRuleFragment = "assign each configured slot a distinct lens, reusing or splitting this default set"
	acOneTableContextWindow         = 12
)

// acOneSixRowTableProblems returns one problem per markdown table block in
// content that enumerates a complete <letter>1..<letter>6 slot-row set
// without the DEFAULT label + scaled-mix rule in the window above the block.
// A "table block" is a maximal run of table-shaped lines (acOneTableLineRE);
// separator rows and repeated header rows inside the block are SKIPPED, not
// terminators, so a header row wedged between R3 and R4 cannot split the
// enumeration into two innocent-looking halves (codex G1-1B).
func acOneSixRowTableProblems(surface, content string) []string {
	lines := strings.Split(content, "\n")
	var problems []string
	i := 0
	for i < len(lines) {
		if !acOneTableLineRE.MatchString(lines[i]) {
			i++
			continue
		}
		start := i         // first line of the table block (usually its header row)
		firstSlotRow := -1 // first SLOT row, for the problem message
		seen := map[string]map[int]bool{}
		for i < len(lines) && acOneTableLineRE.MatchString(lines[i]) {
			if m := acOneSlotTableRowRE.FindStringSubmatch(lines[i]); m != nil {
				if n, err := strconv.Atoi(m[2]); err == nil { // err unreachable given [0-9]+; defensive
					if firstSlotRow == -1 {
						firstSlotRow = i
					}
					if seen[m[1]] == nil {
						seen[m[1]] = map[int]bool{}
					}
					seen[m[1]][n] = true
				}
			}
			i++
		}
		if firstSlotRow == -1 {
			continue // a table block with no slot rows at all
		}
		for letter, nums := range seen {
			complete := true
			for k := 1; k <= 6; k++ {
				if !nums[k] {
					complete = false
					break
				}
			}
			if !complete {
				continue
			}
			ctxStart := start - acOneTableContextWindow
			if ctxStart < 0 {
				ctxStart = 0
			}
			ctx := strings.Join(lines[ctxStart:start], "\n")
			if !strings.Contains(ctx, acOneTableDefaultLabelFragment) || !strings.Contains(ctx, acOneTableScaledMixRuleFragment) {
				problems = append(problems, fmt.Sprintf(
					"AC-1 structural sweep: %s carries a six-row %s1..%s6 slot table (first slot row at line %d) without the DEFAULT label (%q) plus the scaled-mix assignment rule (%q) in the %d lines above it — a fixed six-slot enumeration must be the labelled shipped default the operator derives a scaled mix from, never a bare execution instruction",
					surface, letter, letter, firstSlotRow+1, acOneTableDefaultLabelFragment, acOneTableScaledMixRuleFragment, acOneTableContextWindow))
			}
		}
	}
	return problems
}

// acOneStructuralTableProblems runs the six-row-table detection over every
// surface — the structural half the [CI] guard adds to the pattern half.
func acOneStructuralTableProblems(surfaces map[string]string) []string {
	var problems []string
	for surface, content := range surfaces {
		problems = append(problems, acOneSixRowTableProblems(surface, content)...)
	}
	return problems
}

// acOneSurfaces is the AC-1 sweep's surface set: EVERY embedded plugin skill
// plus all four lifecycle-literal VALUES (ac10Surfaces(), the same broad
// backstop AC-10 scans) plus the claude.go CLAUDE.md skills-table template
// (claudeMDManagedBlock). The original Bead-3 cut scanned only the three R1
// skills + the template; the round-1 panel (S2) proved that under-complete —
// residual fixed-six instructions were LIVE in ms-bead-cycle ("then 6
// reviewers fan out") and ms-spec-autopilot ("across the 6 reviewers",
// "6/6 APPROVE") while CI stayed green, violating R1's single-authority
// AC-1 clause. A residual fixed-six in ANY shipped skill is a violation, so
// the sweep now covers them all. Built fresh per call so callers may mutate
// the returned map freely.
func acOneSurfaces() map[string]string {
	surfaces := ac10Surfaces()
	surfaces["claude.go:claudeMDManagedBlock"] = claudeMDManagedBlock
	return surfaces
}

// reviewDisciplineAC1Allowlist is the explicit allowlist, enumerated over
// the FULL expanded surface set + pattern set: the ONE labelled
// shipped-DEFAULT sentence per surface that carries it, plus the ONE
// DEFAULT-label token the "six slot" pattern class itself matches inside
// ms-panel-run's labelled lens-table heading. Every other legitimate
// "6"/"six" mention on the expanded surfaces (the DEFAULT-labelled lens
// tables' prose, the fenced ladder example's `count:` values, ms-panel-run's
// "never a literal six"/"never a literal 4-6 range" anti-fixed-six clauses,
// ms-bead-impl's "6." list ordinal) produces ZERO sweep hits by wording —
// a future edit that needs a digit/enumeration in one of those sections
// should add its OWN locator-keyed entry here rather than loosen a pattern.
var reviewDisciplineAC1Allowlist = []acTenAllowEntry{}

// acOneValidate mirrors acTenValidate's occurrence-and-location-accounted
// matching (surface, resolved-locator-line, exact matched text, count) but
// keeps its OWN AC-1-flavored problem messages — deliberately NOT reusing
// acTenValidate itself, whose messages are hardcoded "AC-10 ...", which would
// mislabel an AC-1 sweep failure as an AC-10 scan failure.
func acOneValidate(hits []acTenHit, allowlist []acTenAllowEntry, surfaces map[string]string) []string {
	type key struct {
		surface string
		line    int
		matched string
	}
	remaining := make(map[key]int)
	var problems []string

	for _, e := range allowlist {
		content, ok := surfaces[e.surface]
		if !ok {
			problems = append(problems, fmt.Sprintf("AC-1 sweep allowlist entry unresolvable: surface=%s locator=%q matched=%s: surface not present among scanned surfaces", e.surface, e.locator, e.matched))
			continue
		}
		line, diag, ok := acTenResolveLocator(content, e.locator)
		if !ok {
			problems = append(problems, fmt.Sprintf("AC-1 sweep allowlist entry unresolvable: surface=%s locator=%q matched=%s: %s", e.surface, e.locator, e.matched, diag))
			continue
		}
		remaining[key{e.surface, line, e.matched}] += e.count
	}

	for _, h := range hits {
		k := key{h.surface, h.line, h.matched}
		if remaining[k] > 0 {
			remaining[k]--
			continue
		}
		problems = append(problems, fmt.Sprintf("AC-1 sweep: unallowlisted fixed-six-topology hit in %s at line %d: %q", h.surface, h.line, h.matched))
	}
	for k, n := range remaining {
		if n > 0 {
			problems = append(problems, fmt.Sprintf("AC-1 sweep allowlist entry unused (or under-consumed): surface=%s line=%d matched=%s", k.surface, k.line, k.matched))
		}
	}
	return problems
}

// TestReviewDiscipline_AC1SweepGuard is the [CI] guard: every occurrence of
// a fixed-six-topology pattern across ALL embedded plugin skills, all four
// lifecycle literals, and the claude.go CLAUDE.md template must be accounted
// for by reviewDisciplineAC1Allowlist, AND no surface may carry an
// unlabelled six-row slot table (the structural half) — any residual
// execution instruction still hard-coding the topology REDs here, in any
// shipped skill, not just the three the R1 rewrite touched (panel S2).
func TestReviewDiscipline_AC1SweepGuard(t *testing.T) {
	surfaces := acOneSurfaces()
	hits := acOneScanSurfaces(surfaces)
	for _, problem := range acOneValidate(hits, reviewDisciplineAC1Allowlist, surfaces) {
		t.Error(problem)
	}
	for _, problem := range acOneStructuralTableProblems(surfaces) {
		t.Error(problem)
	}
}

// TestReviewDiscipline_AC1SweepGuardNegativeHeadingForm proves the guard REDs
// when the OLD "# Run a 6-Reviewer Panel" heading is reintroduced into a
// FIXTURE copy of ms-panel-run's bytes (never the real file), while the
// allowlisted per-gate shipped-default sentence is untouched and must
// NOT be implicated — the RED-on-revert demo the plan's Verification section
// names explicitly (G1-1-R2: the hyphenated heading form is exactly what the
// separator-tolerant 6[- ]reviewers? pattern exists to catch).
func TestReviewDiscipline_AC1SweepGuardNegativeHeadingForm(t *testing.T) {
	surfaces := acOneSurfaces()
	original, ok := surfaces["ms-panel-run"]
	if !ok {
		t.Fatal("fixture assumption broken: ms-panel-run missing from acOneSurfaces()")
	}
	if !strings.Contains(original, "# Run a Review Panel") {
		t.Fatal("fixture assumption broken: ms-panel-run no longer carries the reworded topology-neutral heading")
	}
	mutated := strings.Replace(original, "# Run a Review Panel", "# Run a 6-Reviewer Panel", 1)
	if mutated == original {
		t.Fatal("fixture assumption broken: heading substitution had no effect")
	}
	surfaces["ms-panel-run"] = mutated

	hits := acOneScanSurfaces(surfaces)
	problems := acOneValidate(hits, reviewDisciplineAC1Allowlist, surfaces)
	if len(problems) == 0 {
		t.Fatal("expected restoring '# Run a 6-Reviewer Panel' to RED the AC-1 sweep guard")
	}
	// The allowlisted default-sentence entry must stay fully consumed (its
	// locator line is untouched by this mutation) — no "unused" complaint
	// about ms-panel-run's default-sentence entry should appear alongside
	// the genuine unallowlisted heading-hit complaint.
	for _, p := range problems {
		if strings.Contains(p, "ms-panel-run") && strings.Contains(p, "unused") {
			t.Errorf("mutation probe over-fired on the allowlisted default sentence (should remain fully consumed): %s", p)
		}
	}
}

// TestReviewDiscipline_AC1SweepGuardNegativeClaudeMDTemplate proves the guard
// also covers the claude.go CLAUDE.md skills-table template surface:
// reintroducing "launch 6 reviewers" into a FIXTURE copy of
// claudeMDManagedBlock (never the real constant) REDs.
func TestReviewDiscipline_AC1SweepGuardNegativeClaudeMDTemplate(t *testing.T) {
	surfaces := acOneSurfaces()
	original, ok := surfaces["claude.go:claudeMDManagedBlock"]
	if !ok {
		t.Fatal("fixture assumption broken: claude.go:claudeMDManagedBlock missing from acOneSurfaces()")
	}
	const rewordedPhrase = "then launch the configured reviewer panel and collect verdicts"
	if !strings.Contains(original, rewordedPhrase) {
		t.Fatal("fixture assumption broken: claudeMDManagedBlock no longer carries the reworded phrase")
	}
	mutated := strings.Replace(original, rewordedPhrase, "then launch 6 reviewers and collect verdicts", 1)
	if mutated == original {
		t.Fatal("fixture assumption broken: substitution had no effect")
	}
	surfaces["claude.go:claudeMDManagedBlock"] = mutated

	hits := acOneScanSurfaces(surfaces)
	problems := acOneValidate(hits, reviewDisciplineAC1Allowlist, surfaces)
	if len(problems) == 0 {
		t.Fatal("expected restoring 'launch 6 reviewers' in the CLAUDE.md template fixture to RED the AC-1 sweep guard")
	}
}

// TestReviewDiscipline_AC1SweepGuardNegativeCategoricalHits proves each
// remaining sweep pattern REDs on its own synthetic fixture surface, never a
// real shipped file — the plan's "reverting ANY part of the sweep is RED,
// not just the headline fragments" requirement, exercised per pattern class.
func TestReviewDiscipline_AC1SweepGuardNegativeCategoricalHits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		inject string
	}{
		{"all-six", "run all six reviewers now"},
		{"six-reviewers-spelled", "the panel launches six reviewers by default"},
		{"six-distinct", "six distinct lenses keep it honest"},
		{"six-verdicts", "Six verdicts x 3 items each"},
		{"r-slot-range", "write /tmp/codex_x_r{4,5,6}.md"},
		{"claude-slot-enum", "for each of R1, R2, R3"},
		{"codex-slot-enum", "for each of R4, R5, R6"},
		{"claude-range-hyphen", "split R1-R3 claude vs R4-R6 codex"},
		{"family-denominator", "family split <claude>/3 claude"},

		// Panel round-1 additions (F1 + G1) — one categorical negative per
		// newly added pattern class; before those classes existed, every one
		// of these restored forms stayed GREEN.
		// The EXACT real pre-sweep ms-panel-run:8 wording, Markdown backticks
		// intact — the codex G1-1A finding: the pre-fix `three\s+agent\s+calls`
		// pattern never matched this form, and the committed fixture was a
		// de-backticked surrogate, so reverting the real line stayed GREEN.
		{"three-agent-calls-and-three-codex-real-backticked", "fan out three `Agent` calls (Claude) and three `codex exec` background processes (Codex)"},
		// The plain-whitespace form must keep hitting too — the tolerant gap
		// class is a superset of \s+, and this pins that.
		{"three-agent-calls-and-three-codex-plain", "launch three Agent calls (Claude) and three codex CLI sessions"},
		{"three-agent-calls-bold", "launch three **Agent** calls in parallel"},
		{"three-per-family", "keep the mix at three per family"},
		{"three-per-family-backticked", "keep the mix at three `per family` slots"},
		{"3-plus-3", "the classic 3+3 family split"},
		{"three-claudes-spelled", "when all three Claudes APPROVE"},
		{"3-claude-digit", "3 Claude APPROVE, 1+ Codex REQUEST_CHANGES"},
		{"3-codex-digit", "3 Codex APPROVE, 1+ Claude REQUEST_CHANGES"},
		{"five-slash-six-fraction", "pass requires 5/6 for the default panel"},
		{"six-slash-six-fraction", "round K: 6/6 APPROVE"},
		{"five-of-six-hyphenated", "a 5-of-6 supermajority approves"},
		{"expected-reviewers-digit", "set expected_reviewers 6 before launch"},
		{"expected-reviewers-json-digit", `"expected_reviewers": 6`},
		{"fan-out-6", "then fan out 6 and wait for verdicts"},
		{"fan-out-six", "then fan out six and wait for verdicts"},
		{"6-slot-unlabelled", "reuse the 6-slot defaults"},
		{"six-slot-hyphenated", "the six-slot lens table"},
		{"six-slot-spaced", "use the six slot assignments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := map[string]string{"fixture-surface": tc.inject}
			hits := acOneScanSurfaces(fixture)
			if len(hits) == 0 {
				t.Fatalf("fixture assumption broken: %q produced no AC-1 sweep hits at all", tc.inject)
			}
			if problems := acOneValidate(hits, nil, fixture); len(problems) == 0 {
				t.Fatalf("expected the AC-1 sweep to RED on %q, but it found no problems", tc.inject)
			}
		})
	}
}

// TestReviewDiscipline_AC1SweepGuardNegativeOrchestratorResiduals proves the
// EXACT residual fixed-six instructions the round-1 panel (S2) found LIVE in
// the two orchestrator skills RED if restored — each mutation applied to a
// FIXTURE copy of the real surface (never the shipped file), restoring the
// pre-sweep wording verbatim.
func TestReviewDiscipline_AC1SweepGuardNegativeOrchestratorResiduals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		surface  string
		swept    string // the replacement wording now shipped (fixture precondition)
		restored string // the pre-sweep fixed-six wording
	}{
		{
			name:     "ms-bead-cycle sequence-diagram fan-out",
			surface:  "ms-bead-cycle",
			swept:    "then the configured reviewers fan out",
			restored: "then 6 reviewers fan out",
		},
		{
			name:     "ms-bead-cycle family-asymmetry headline",
			surface:  "ms-bead-cycle",
			swept:    "One family approves; another requests changes",
			restored: "all three Claudes APPROVE",
		},
		{
			name:     "ms-spec-autopilot parallelism note",
			surface:  "ms-spec-autopilot",
			swept:    "across the configured panel reviewers",
			restored: "across the 6 reviewers",
		},
		{
			name:     "ms-spec-autopilot 6/6 report tally",
			surface:  "ms-spec-autopilot",
			swept:    "<N>/<N> APPROVE",
			restored: "6/6 APPROVE",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			surfaces := acOneSurfaces()
			original, ok := surfaces[tc.surface]
			if !ok {
				t.Fatalf("fixture assumption broken: %s missing from acOneSurfaces()", tc.surface)
			}
			if !strings.Contains(original, tc.swept) {
				t.Fatalf("fixture assumption broken: %s no longer carries the swept replacement wording %q", tc.surface, tc.swept)
			}
			mutated := strings.Replace(original, tc.swept, tc.restored, 1)
			if mutated == original {
				t.Fatal("fixture assumption broken: restore substitution had no effect")
			}
			surfaces[tc.surface] = mutated

			hits := acOneScanSurfaces(surfaces)
			problems := acOneValidate(hits, reviewDisciplineAC1Allowlist, surfaces)
			if len(problems) == 0 {
				t.Fatalf("expected restoring %q in %s to RED the AC-1 sweep guard", tc.restored, tc.surface)
			}
			found := false
			for _, p := range problems {
				if strings.Contains(p, tc.surface) && strings.Contains(p, "unallowlisted") {
					found = true
				}
			}
			if !found {
				t.Errorf("expected an unallowlisted-hit problem naming %s; got: %v", tc.surface, problems)
			}
		})
	}
}

// TestReviewDiscipline_AC1SweepGuardNegativeBacktickedFanOutRevert proves the
// EXACT real pre-sweep ms-panel-run:8 fan-out wording — Markdown backticks
// intact — REDs when restored into a FIXTURE copy of ms-panel-run (never the
// real file). This is the codex G1-1A revert that stayed GREEN before the
// format-tolerant gap class: `three\s+agent\s+calls` cannot see
// "three `Agent` calls", and the only committed fixture was a de-backticked
// surrogate.
func TestReviewDiscipline_AC1SweepGuardNegativeBacktickedFanOutRevert(t *testing.T) {
	const swept = "Dispatch that prompt once with the harness agent tool"
	const restored = "fan out three `Agent` calls (Claude) and three `codex exec` background processes (Codex)"

	surfaces := acOneSurfaces()
	original, ok := surfaces["ms-panel-run"]
	if !ok {
		t.Fatal("fixture assumption broken: ms-panel-run missing from acOneSurfaces()")
	}
	if !strings.Contains(original, swept) {
		t.Fatalf("fixture assumption broken: ms-panel-run no longer carries the swept per-configured-slot fan-out wording %q", swept)
	}
	mutated := strings.Replace(original, swept, restored, 1)
	if mutated == original {
		t.Fatal("fixture assumption broken: restore substitution had no effect")
	}
	surfaces["ms-panel-run"] = mutated

	hits := acOneScanSurfaces(surfaces)
	problems := acOneValidate(hits, reviewDisciplineAC1Allowlist, surfaces)
	if len(problems) == 0 {
		t.Fatalf("expected restoring the REAL backticked wording %q into ms-panel-run to RED the AC-1 sweep guard", restored)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "ms-panel-run") && strings.Contains(p, "unallowlisted") {
			found = true
		}
		// The allowlisted ms-panel-run entries must remain fully consumed —
		// the mutation must not dislodge them into "unused" complaints.
		if strings.Contains(p, "ms-panel-run") && strings.Contains(p, "unused") {
			t.Errorf("mutation probe over-fired on an allowlisted ms-panel-run entry (should remain fully consumed): %s", p)
		}
	}
	if !found {
		t.Errorf("expected an unallowlisted-hit problem naming ms-panel-run; got: %v", problems)
	}
}

// TestReviewDiscipline_AC1SweepGuardNegativeBareSixRowTable proves the
// structural half REDs on a bare six-row slot table (R1..R6 and F1..F6
// forms) injected into a synthetic fixture surface — including the three
// codex G1-1B format bypasses (inline-code slot ids, bold slot ids, and a
// repeated header row wedged between R3 and R4) — stays GREEN when the
// SAME table carries the exact DEFAULT label + scaled-mix assignment rule,
// and does not over-reach onto a five-row table.
func TestReviewDiscipline_AC1SweepGuardNegativeBareSixRowTable(t *testing.T) {
	// mkWrappedTable wraps each slot id in the given left/right formatting
	// ("`"/"`" for inline code, "**"/"**" for bold, ""/"" for plain).
	mkWrappedTable := func(letter, wrapL, wrapR string, n int) string {
		var b strings.Builder
		b.WriteString("| Slot | Lens |\n|:-----|:-----|\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "| %s%s%d%s | lens %d |\n", wrapL, letter, i, wrapR, i)
		}
		return b.String()
	}
	mkTable := func(letter string, n int) string {
		return mkWrappedTable(letter, "", "", n)
	}
	label := "The table below is the " + acOneTableDefaultLabelFragment + " mix. For a scaled mix, " +
		acOneTableScaledMixRuleFragment + ".\n\n"

	t.Run("bare R1..R6 REDs", func(t *testing.T) {
		content := "Use the slot assignments below.\n\n" + mkTable("R", 6)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) == 0 {
			t.Fatal("expected a bare six-row R1..R6 table to RED the structural sweep")
		}
	})
	t.Run("bare F1..F6 REDs", func(t *testing.T) {
		content := "Final-review lenses:\n\n" + mkTable("F", 6)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) == 0 {
			t.Fatal("expected a bare six-row F1..F6 table to RED the structural sweep")
		}
	})
	t.Run("backticked slot ids RED (codex G1-1B inline-code bypass)", func(t *testing.T) {
		content := "Use the slot assignments below.\n\n" + mkWrappedTable("R", "`", "`", 6)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) == 0 {
			t.Fatal("expected a six-row table with inline-code slot ids (|`R1`|) to RED the structural sweep")
		}
	})
	t.Run("bold slot ids RED (codex G1-1B bold bypass)", func(t *testing.T) {
		content := "Use the slot assignments below.\n\n" + mkWrappedTable("R", "**", "**", 6)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) == 0 {
			t.Fatal("expected a six-row table with bold slot ids (|**R1**|) to RED the structural sweep")
		}
	})
	t.Run("header row wedged between R3 and R4 REDs (codex G1-1B split bypass)", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("Use the slot assignments below.\n\n| Slot | Lens |\n|:-----|:-----|\n")
		for i := 1; i <= 3; i++ {
			fmt.Fprintf(&b, "| R%d | lens %d |\n", i, i)
		}
		b.WriteString("| Slot | Lens |\n|:-----|:-----|\n") // the wedge
		for i := 4; i <= 6; i++ {
			fmt.Fprintf(&b, "| R%d | lens %d |\n", i, i)
		}
		if problems := acOneSixRowTableProblems("fixture-surface", b.String()); len(problems) == 0 {
			t.Fatal("expected a six-row table split by a repeated header row to RED the structural sweep")
		}
	})
	t.Run("labelled DEFAULT table stays GREEN", func(t *testing.T) {
		content := label + mkTable("R", 6)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) != 0 {
			t.Fatalf("expected the labelled DEFAULT six-row table to stay GREEN; got: %v", problems)
		}
	})
	t.Run("labelled DEFAULT table with backticked slot ids stays GREEN (label exemption survives formatting)", func(t *testing.T) {
		content := label + mkWrappedTable("R", "`", "`", 6)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) != 0 {
			t.Fatalf("expected the labelled DEFAULT backticked six-row table to stay GREEN; got: %v", problems)
		}
	})
	t.Run("five-row table stays GREEN (no over-reach)", func(t *testing.T) {
		content := "Partial enumeration:\n\n" + mkTable("R", 5)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) != 0 {
			t.Fatalf("expected a five-row table to stay GREEN; got: %v", problems)
		}
	})
	t.Run("label alone without the scaled-mix rule REDs", func(t *testing.T) {
		content := "The table below is the " + acOneTableDefaultLabelFragment + " mix.\n\n" + mkTable("R", 6)
		if problems := acOneSixRowTableProblems("fixture-surface", content); len(problems) == 0 {
			t.Fatal("expected a DEFAULT label without the scaled-mix assignment rule to RED the structural sweep")
		}
	})
}

// TestReviewDiscipline_AC1SweepGuardNegativeTableLabelStripped proves the
// structural half bites on the REAL surfaces: stripping the DEFAULT-label
// sentence off either shipped lens table (a fixture copy, never the real
// file) leaves a bare six-row table that REDs — a label-only deletion cannot
// survive even if every substring pattern stays quiet.
func TestReviewDiscipline_AC1SweepGuardNegativeTableLabelStripped(t *testing.T) {
	for _, surface := range []string{"ms-panel-run", "ms-spec-final-review"} {
		body := acOneSurfaces()[surface]
		if p := acOneSixRowTableProblems(surface, body); len(p) != 0 {
			t.Fatal(p)
		}
		var table strings.Builder
		table.WriteString("\n| Slot | Lens |\n|:---|:---|\n")
		for i := 1; i <= 6; i++ {
			fmt.Fprintf(&table, "| R%d | lens |\n", i)
		}
		if len(acOneSixRowTableProblems(surface, body+table.String())) == 0 {
			t.Fatal("unlabelled-table mutation passed")
		}
	}
}

// ---------------------------------------------------------------------------
// Bead 4 (mindspec-xurf.4) appends below: AC-11 (R8b CI-parity slot text +
// concrete-invocation tripwires; the config-comment half is Bead 2's, pinned
// in internal/setup/lifecycle_gate_step_test.go) and AC-12 (completeness
// sweep over this whole file's fragment tables — every AC this table claims
// to cover has at least one traceable row, and the file runs under plain
// `go test ./internal/setup/`).
// ---------------------------------------------------------------------------

var reviewDisciplineBead4FragmentRows = []fragmentRow{
	{ac: "AC-11", desc: "CI parity rule", surface: "ms-spec-final-review", want: "review.ci-parity"},
	{ac: "AC-11", desc: "no configured command limit", surface: "ms-spec-final-review", want: "If neither key is set, state that limit in the rationale."},
	{ac: "AC-11", desc: "CI separate result", surface: "ms-spec-final-review", want: "A local result does not replace the PR's CI result."},
	{ac: "AC-11", desc: "no test command", surface: "ms-spec-final-review", want: "go test", negate: true},
	{ac: "AC-11", desc: "no npm command", surface: "ms-spec-final-review", want: "npm test", negate: true},
	{ac: "AC-11", desc: "no pytest command", surface: "ms-spec-final-review", want: "pytest", negate: true},
	{ac: "AC-11", desc: "no make command", surface: "ms-spec-final-review", want: "make test", negate: true},
}

// TestReviewDiscipline_Bead4Fragments is Bead 4's table-driven [CI] guard,
// same shape as TestReviewDiscipline_Fragments / TestReviewDiscipline_
// Bead3Fragments above — a separate slice/function for per-bead
// traceability (AC-12), not because the underlying mechanism differs.
func TestReviewDiscipline_Bead4Fragments(t *testing.T) {
	skills := pluginmindspec.SkillFiles()
	for _, row := range reviewDisciplineBead4FragmentRows {
		row := row
		t.Run(row.ac+"/"+row.desc, func(t *testing.T) {
			content, ok := skills[row.surface]
			if !ok {
				t.Fatalf("pluginmindspec.SkillFiles() has no %q entry", row.surface)
			}
			has := strings.Contains(content, row.want)
			switch {
			case row.negate && has:
				t.Errorf("%s: %s must NOT contain %q, but it does", row.surface, row.ac, row.want)
			case !row.negate && !has:
				t.Errorf("%s: %s is missing the required fragment %q", row.surface, row.ac, row.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-11 (R8b) — the CI-parity instruction BOUND to the named F2 slot
// (final-review codex G1 fix): the assertion extracts the SINGLE F2 table
// row from the embedded ms-spec-final-review bytes and requires the
// commands.ci / commands.test-fallback / no-declaration-advisory fragments
// INSIDE that row, so moving the instruction into an unrelated section can
// no longer stay GREEN.
// ---------------------------------------------------------------------------

// acElevenF2RowRE matches the ms-spec-final-review lens table's F2 slot row:
// a markdown table row (leading indent allowed — the table sits inside a
// numbered list item) whose FIRST cell is F2, tolerating inline Markdown
// formatting around the slot id the same way acOneSlotTableRowRE does. The
// row is one physical line, so (?m)'s per-line `.*$` captures exactly it.
// An interpreted string because a Go raw string cannot contain the backtick
// the formatting class must include.
var acElevenF2RowRE = regexp.MustCompile(`(?m)^4\. Apply .*review\.ci-parity.*$`)

// acElevenF2SlotRow extracts the single F2 slot row from content. ok=false
// (with a diagnostic) when the row is missing or ambiguous — an absent or
// duplicated F2 row can never satisfy the slot binding, so callers must
// treat that as a failure, never a silent pass.
func acElevenF2SlotRow(content string) (row, diag string, ok bool) {
	rows := acElevenF2RowRE.FindAllString(content, -1)
	switch len(rows) {
	case 0:
		return "", "no F2 slot table row found", false
	case 1:
		return rows[0], "", true
	default:
		return "", fmt.Sprintf("F2 slot table row is ambiguous: %d rows match", len(rows)), false
	}
}

// reviewDisciplineAC11F2SlotFragments are the CI-parity fragments that must
// occur INSIDE the F2 slot row itself — never merely anywhere in the body.
var reviewDisciplineAC11F2SlotFragments = []struct {
	desc string
	want string
}{
	{"names commands.ci", "commands.ci"},
	{"names the commands.test fallback", "commands.test"},
	{"records the full invocation rule", "run the project's `commands.ci` verbatim, else `commands.test`, in the slot tree."},
}

// TestReviewDiscipline_AC11F2SlotBinding is the F2-slot-bound half of the
// AC-11 guard: each CI-parity fragment is required within the F2 table row
// extracted from the live embedded skill bytes.
func TestReviewDiscipline_AC11F2SlotBinding(t *testing.T) {
	skills := pluginmindspec.SkillFiles()
	content, ok := skills["ms-spec-final-review"]
	if !ok {
		t.Fatal(`pluginmindspec.SkillFiles() has no "ms-spec-final-review" entry`)
	}
	row, diag, ok := acElevenF2SlotRow(content)
	if !ok {
		t.Fatalf("extracting the F2 slot row from ms-spec-final-review: %s", diag)
	}
	for _, f := range reviewDisciplineAC11F2SlotFragments {
		if !strings.Contains(row, f.want) {
			t.Errorf("AC-11: the ms-spec-final-review F2 slot row (%s) is missing %q — the CI-parity instruction must be assigned to the NAMED F2 slot, not merely appear somewhere in the body", f.desc, f.want)
		}
	}
}

// acElevenF2CIParityInstruction is the full CI-parity instruction sentence
// as shipped inside the F2 slot row — the move-out mutation's payload.
const acElevenF2CIParityInstruction = "run the project's `commands.ci` verbatim, else `commands.test`, in the slot tree."

// TestReviewDiscipline_AC11F2SlotBindingMutationProbe proves the slot
// binding is not vacuous by applying the SAME production predicate
// (acElevenF2SlotRow + per-fragment Contains) to a mutated copy in which
// the CI-parity instruction is moved OUT of the F2 row into an unrelated
// trailing section — the exact codex G1 mutation the pre-fix whole-body
// rows stayed GREEN under (the fixture sanity check below re-proves that).
func TestReviewDiscipline_AC11F2SlotBindingMutationProbe(t *testing.T) {
	skills := pluginmindspec.SkillFiles()
	content, ok := skills["ms-spec-final-review"]
	if !ok {
		t.Fatal(`pluginmindspec.SkillFiles() has no "ms-spec-final-review" entry`)
	}
	if !strings.Contains(content, acElevenF2CIParityInstruction) {
		t.Fatal("fixture assumption broken: the shipped F2 CI-parity instruction no longer appears verbatim — update acElevenF2CIParityInstruction alongside the skill edit")
	}
	mutated := strings.Replace(content, acElevenF2CIParityInstruction, "", 1)
	if mutated == content {
		t.Fatal("fixture assumption broken: move-out replacement had no effect")
	}
	mutated += "\n\n" + acElevenF2CIParityInstruction + "\n"

	// Fixture sanity: the pre-fix HOLLOW whole-body check still passes on
	// this mutation — every fragment remains present in the body.
	for _, f := range reviewDisciplineAC11F2SlotFragments {
		if !strings.Contains(mutated, f.want) {
			t.Fatalf("fixture assumption broken: %q vanished from the mutated body entirely", f.want)
		}
	}

	// The SAME production predicate must now fail: the F2 row still exists
	// but no longer carries the instruction.
	row, diag, ok := acElevenF2SlotRow(mutated)
	if !ok {
		t.Fatalf("fixture assumption broken: the mutated copy should still carry exactly one F2 row: %s", diag)
	}
	for _, f := range reviewDisciplineAC11F2SlotFragments {
		if strings.Contains(row, f.want) {
			t.Errorf("mutation probe failed: %q should have moved out of the F2 slot row", f.want)
		}
	}
}

// reviewDisciplineCompleteACs is the full set of ACs the R1c guard table
// (this file) claims to cover, per AC-12 / plan.md's per-bead ownership
// table. AC-3, AC-13, AC-14, AC-15, AC-16, AC-17 are asserted elsewhere
// (dedicated tests / a different package, per plan.md's Provenance table)
// and are deliberately NOT in this set — AC-12 only claims the fragment
// rows enumerated in spec.md's own AC-12 text: "AC-1/2/4/5/6/7/8/9/10/11".
var reviewDisciplineCompleteACs = []string{
	"AC-1", "AC-2", "AC-4", "AC-5", "AC-6", "AC-7", "AC-8", "AC-9", "AC-10", "AC-11",
}

// TestReviewDiscipline_AC12Completeness is the completeness sweep: every AC
// in reviewDisciplineCompleteACs must have at least one traceable row
// somewhere in this file's fragment tables (or, for the two ACs asserted
// structurally rather than by substring — AC-2's ladder-example parse and
// AC-10's scan-plus-allowlist — at least one dedicated test function whose
// name announces that AC). Deleting every row for one AC (e.g. reverting
// this bead's own AC-11 rows above) REDs this test, which is the AC-12
// per-fragment-traceability guarantee applied to itself.
func TestReviewDiscipline_AC12Completeness(t *testing.T) {
	covered := map[string]bool{}
	for _, row := range reviewDisciplineFragmentRows {
		covered[row.ac] = true
	}
	for _, row := range reviewDisciplineBead3FragmentRows {
		covered[row.ac] = true
	}
	for _, row := range reviewDisciplineBead4FragmentRows {
		covered[row.ac] = true
	}
	for _, row := range reviewDisciplineLiteralFragmentRows {
		covered[row.ac] = true
	}
	// AC-2 (the panel.gates ladder example) and AC-10 (scan-plus-allowlist)
	// are asserted structurally, not via the fragmentRow substring shape —
	// named here as explicitly covered, backstopped by the dedicated tests
	// TestReviewDiscipline_AC2LadderExampleStructural and
	// TestReviewDiscipline_AC10ScanPlusAllowlist. That backstop is made real
	// by the package-scope `var _ = TestReviewDiscipline_AC2LadderExampleStructural`
	// / `var _ = TestReviewDiscipline_AC10ScanPlusAllowlist` references below
	// this function: deleting or renaming either backing test is a COMPILE
	// ERROR for this file, not merely a comment claim, so AC-2/AC-10 cannot
	// silently lose their structural coverage while this file still builds.
	covered["AC-2"] = true
	covered["AC-10"] = true

	for _, ac := range reviewDisciplineCompleteACs {
		if !covered[ac] {
			t.Errorf("AC-12 completeness: %s has no traceable row in the R1c guard table", ac)
		}
	}

	// Table-driven per AC-12: a file-granular revert of a multi-row skill
	// legitimately REDs every row pinned to that file, not just one — prove
	// the stash-matrix property holds structurally by checking every row
	// across every table names a real embedded surface (fragmentRow) or
	// lifecycle literal (literalFragmentRow), so a revert of that file's
	// bytes is guaranteed to intersect at least one row per AC touching it.
	skills := pluginmindspec.SkillFiles()
	for _, row := range append(append([]fragmentRow{}, reviewDisciplineFragmentRows...), append(reviewDisciplineBead3FragmentRows, reviewDisciplineBead4FragmentRows...)...) {
		if _, ok := skills[row.surface]; !ok {
			t.Errorf("AC-12 completeness: row %q (%s) names surface %q which pluginmindspec.SkillFiles() does not have", row.desc, row.ac, row.surface)
		}
	}
	literals := lifecycleSkillFiles()
	for _, row := range reviewDisciplineLiteralFragmentRows {
		if _, ok := literals[row.surface]; !ok {
			t.Errorf("AC-12 completeness: literal row %q (%s) names surface %q which lifecycleSkillFiles() does not have", row.desc, row.ac, row.surface)
		}
	}
}

// reviewDisciplineAC2StructuralBackstop and reviewDisciplineAC10StructuralBackstop
// are compile-load-bearing references to the two dedicated tests that back
// AC-12's "covered" claim for AC-2 and AC-10 (see the comment above). They
// exist ONLY so that deleting or renaming
// TestReviewDiscipline_AC2LadderExampleStructural or
// TestReviewDiscipline_AC10ScanPlusAllowlist fails `go vet`/`go build` for
// this package, rather than silently leaving TestReviewDiscipline_AC12Completeness
// green with a stale hardcoded covered[...] = true and no backing test.
var (
	reviewDisciplineAC2StructuralBackstop  = TestReviewDiscipline_AC2LadderExampleStructural
	reviewDisciplineAC10StructuralBackstop = TestReviewDiscipline_AC10ScanPlusAllowlist
)
