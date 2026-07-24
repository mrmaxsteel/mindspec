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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

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
	// AC-4 (R2) — reviewer mutation-isolation.
	{ac: "AC-4", desc: "ms-panel-run mutation-isolation worktree command", surface: "ms-panel-run", want: "git worktree add --detach"},
	{ac: "AC-4", desc: "ms-spec-final-review mutation-isolation worktree command", surface: "ms-spec-final-review", want: "git worktree add --detach"},
	{ac: "AC-4", desc: "ms-panel-tally contested-finding re-verify fragment", surface: "ms-panel-tally", want: "re-verify it yourself in a fresh detached checkout"},

	// AC-5 (R3) — verify-never-confirm.
	{ac: "AC-5", desc: "ms-bead-cycle no longer blindly trusts the empirical check", surface: "ms-bead-cycle", want: "trust the empirical check", negate: true},
	{ac: "AC-5", desc: "ms-bead-cycle verify-never-confirm fragment", surface: "ms-bead-cycle", want: "verify, never confirmed"},
	{ac: "AC-5", desc: "ms-panel-tally verify-never-confirm fragment", surface: "ms-panel-tally", want: "verify, never confirmed"},
	{ac: "AC-5", desc: "ms-panel-run documents the security-classifier substitution fallback", surface: "ms-panel-run", want: "correctness-framed persona"},

	// AC-8 (R6) — findings-never-out-voted, reviewer-facing.
	{ac: "AC-8", desc: "ms-panel-run findings-never-out-voted", surface: "ms-panel-run", want: "findings-never-out-voted"},
	{ac: "AC-8", desc: "ms-spec-final-review findings-never-out-voted", surface: "ms-spec-final-review", want: "findings-never-out-voted"},

	// AC-9 (R7) — absolute scratch, clean-worktree check, home-path sweep.
	{ac: "AC-9", desc: "ms-panel-run absolute-scratch: never a relative path", surface: "ms-panel-run", want: "never a relative path"},
	{ac: "AC-9", desc: "ms-panel-run absolute-scratch: harness cwd-resets", surface: "ms-panel-run", want: "harness cwd-resets"},
	{ac: "AC-9", desc: "ms-bead-fix absolute-scratch: never a relative path", surface: "ms-bead-fix", want: "never a relative path"},
	{ac: "AC-9", desc: "ms-bead-fix absolute-scratch: harness cwd-resets", surface: "ms-bead-fix", want: "harness cwd-resets"},
	{ac: "AC-9", desc: "ms-spec-final-review absolute-scratch: never a relative path", surface: "ms-spec-final-review", want: "never a relative path"},
	{ac: "AC-9", desc: "ms-spec-final-review absolute-scratch: harness cwd-resets", surface: "ms-spec-final-review", want: "harness cwd-resets"},
	{ac: "AC-9", desc: "ms-bead-cycle clean-worktree check before mindspec complete", surface: "ms-bead-cycle", want: "git status --porcelain"},
	{ac: "AC-9", desc: "ms-panel-run no longer names the operator's literal home path", surface: "ms-panel-run", want: "/Users/Max", negate: true},

	// AC-13 (partial) — every touched skill cites ADR-0044 (the ADR-file
	// existence + Accepted-marker half is TestReviewDiscipline_ADR0044ExistsAccepted
	// below; Bead 3 appends the two lifecycle-literal citation rows).
	{ac: "AC-13", desc: "ms-panel-run cites ADR-0044", surface: "ms-panel-run", want: "ADR-0044"},
	{ac: "AC-13", desc: "ms-panel-tally cites ADR-0044", surface: "ms-panel-tally", want: "ADR-0044"},
	{ac: "AC-13", desc: "ms-bead-cycle cites ADR-0044", surface: "ms-bead-cycle", want: "ADR-0044"},
	{ac: "AC-13", desc: "ms-bead-fix cites ADR-0044", surface: "ms-bead-fix", want: "ADR-0044"},
	{ac: "AC-13", desc: "ms-spec-final-review cites ADR-0044", surface: "ms-spec-final-review", want: "ADR-0044"},
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

// ---------------------------------------------------------------------------
// AC-10 — portability scan-plus-allowlist (Non-Goal filter).
// ---------------------------------------------------------------------------

// acTenModelRE is pattern class (i): model-family values. Categorical, not a
// single-char-suffix denylist — covers gpt-4o, gpt-5.6-sol, o4-mini,
// claude-4.5, and bare family names.
var acTenModelRE = regexp.MustCompile(`(?i)\b(opus|sonnet|haiku|fable|claude-[0-9][.\w-]*|gpt-[0-9][.\w-]*|o[0-9]+(-[a-z]+)?)\b`)

// acTenLoreMarkers is pattern class (ii): mindspec-self-development lore
// that must never ship in a consumer skill.
var acTenLoreMarkers = []string{
	"go test -short",
	"argv-ratchet",
	"internal/harness",
	"internal/instruct",
}

// acTenHomePathRE is pattern class (iii): absolute operator-home paths
// (POSIX only; Windows C:\Users\ is explicitly out of scope per spec.md).
var acTenHomePathRE = regexp.MustCompile(`/Users/|/home/|/root/`)

// acTenSpecIDRE is pattern class (iv): this project's own incident IDs.
var acTenSpecIDRE = regexp.MustCompile(`spec-[0-9]+`)

// acTenHit is one occurrence of a scanned pattern in one surface.
type acTenHit struct {
	surface string
	matched string
}

// acTenScanContent runs all four pattern classes over one surface's bytes,
// emitting one acTenHit per occurrence — a matched text repeated N times in
// one surface yields N hits, which is what makes occurrence-accounted
// allowlisting possible (an allowlist entry authorizes a COUNT of a given
// matched text within a given surface, never "any hit here").
func acTenScanContent(surface, content string) []acTenHit {
	var hits []acTenHit
	for _, m := range acTenModelRE.FindAllString(content, -1) {
		hits = append(hits, acTenHit{surface, m})
	}
	for _, marker := range acTenLoreMarkers {
		for i := 0; i < strings.Count(content, marker); i++ {
			hits = append(hits, acTenHit{surface, marker})
		}
	}
	for _, m := range acTenHomePathRE.FindAllString(content, -1) {
		hits = append(hits, acTenHit{surface, m})
	}
	for _, m := range acTenSpecIDRE.FindAllString(content, -1) {
		hits = append(hits, acTenHit{surface, m})
	}
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

// acTenAllowEntry is one occurrence-accounted allowlist entry: (surface,
// matched text) authorizes exactly `count` occurrences of that EXACT
// matched text within that surface — never "any hit in this file". locator
// is a human-readable, line-number-independent pointer for reviewers; it
// plays no role in the matching logic itself (matching is by
// surface+matched-text+count only, so an entry survives line-number drift,
// per plan.md's "robust to line-number drift" requirement).
type acTenAllowEntry struct {
	surface string
	locator string
	matched string
	count   int
	reason  string
}

// reviewDisciplineAC10Allowlist is the FULL retained allowlist, verified at
// base commit 7ec96295 (plan.md's AC-10 inventory table): 8 base
// spec-[0-9]+ occurrences at that commit, one of which (ms-panel-run's
// retry-note "lola spec-050" citation) this bead GENERALIZES away in the
// R7 sweep, leaving 7 retained occurrences across five surfaces. Pattern
// classes (i) model values, (ii) lore markers, and (iii) home paths all
// scan ZERO hits across the real embedded surfaces (verified) — their
// categorical enforcement burden is carried entirely by the negative
// mutation tests below, not by any allowlist entry.
var reviewDisciplineAC10Allowlist = []acTenAllowEntry{
	{
		surface: "ms-panel-run",
		locator: "Inputs section, panel-slug worked example",
		matched: "spec-050",
		count:   2,
		reason:  "intentionally retained neutral example slugs — the round-1 (`spec-050-bead2`) and round-2 (`spec-050-bead2-r2`) forms on the one Inputs line, AC-10's own worked example",
	},
	{
		surface: "ms-bead-cycle",
		locator: "step-0 bd-vs-plan disagreement case history",
		matched: "spec-050",
		count:   1,
		reason:  "deliberately kept cross-project case history (R7); not a mindspec incident ID",
	},
	{
		surface: "ms-panel-tally",
		locator: "lola-f4a8 $417 postmortem line (Artifact gates section)",
		matched: "spec-050",
		count:   2,
		reason:  "cross-project provenance powering the artifact-gate HARD-block rationale; both occurrences on the one postmortem line",
	},
	{
		surface: "ms-spec-final-review",
		locator: "escape-hatch fix-commit provenance line",
		matched: "spec-050",
		count:   1,
		reason:  "cross-project provenance for the escape-hatch legitimacy rule",
	},
	{
		surface: "ms-bead-impl",
		locator: "case history (Inputs area)",
		matched: "spec-050",
		count:   1,
		reason:  "pre-existing case history in a skill this spec does not edit, but the AC-10 scan runs over ALL embedded skills, so it still needs its own entry",
	},
}

// acTenValidate checks hits against the allowlist, consuming entries
// ONE-TO-ONE (matched text AND count must both agree). It is a pure
// function — it returns problem strings instead of calling t.Errorf — so
// the SAME logic can be exercised both for the real positive guard and for
// the negative mutation tests below without one masking the other's
// failures. FAILS (returns a non-empty slice) on (a) any scan hit not
// matched by an entry, and (b) any allowlist entry left unconsumed.
func acTenValidate(hits []acTenHit, allowlist []acTenAllowEntry) []string {
	type key struct{ surface, matched string }
	remaining := make(map[key]int)
	for _, e := range allowlist {
		remaining[key{e.surface, e.matched}] += e.count
	}

	var problems []string
	for _, h := range hits {
		k := key{h.surface, h.matched}
		if remaining[k] > 0 {
			remaining[k]--
			continue
		}
		problems = append(problems, "AC-10 scan: unallowlisted hit in "+h.surface+": "+h.matched)
	}
	for k, n := range remaining {
		if n > 0 {
			problems = append(problems, "AC-10 allowlist entry unused (or under-consumed): surface="+k.surface+" matched="+k.matched)
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

// TestReviewDiscipline_AC10ScanPlusAllowlist is the [CI] guard for AC-10: it
// scans every embedded skill and lifecycle literal for the four pattern
// classes and fails on any hit the allowlist above does not account for
// (exact surface + exact matched text + count), and on any allowlist entry
// left unconsumed.
func TestReviewDiscipline_AC10ScanPlusAllowlist(t *testing.T) {
	hits := acTenScanSurfaces(ac10Surfaces())
	for _, problem := range acTenValidate(hits, reviewDisciplineAC10Allowlist) {
		t.Error(problem)
	}
}

// TestReviewDiscipline_AC10NegativeCategoricalHits proves the scan REDs on
// each of the three mandated categorical negatives (spec.md's spec-gate
// G2), injected into a synthetic fixture surface — never a real shipped
// file.
func TestReviewDiscipline_AC10NegativeCategoricalHits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		inject string
	}{
		{"gpt-4o", "the model gpt-4o handled this probe"},
		{"o4-mini", "routed to o4-mini for the empirical check"},
		{"root-agent-path", "wrote scratch to /root/agent/notes.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := map[string]string{"fixture-surface": tc.inject}
			hits := acTenScanSurfaces(fixture)
			if len(hits) == 0 {
				t.Fatalf("fixture assumption broken: %q produced no scan hits at all", tc.inject)
			}
			if problems := acTenValidate(hits, nil); len(problems) == 0 {
				t.Fatalf("expected the AC-10 scan to RED on %q, but it found no problems", tc.inject)
			}
		})
	}
}

// TestReviewDiscipline_AC10NegativeDuplicateToken proves the occurrence
// accounting bites: an extra, UNALLOWLISTED occurrence of an already-
// allowlisted token in the SAME surface (ms-panel-tally is allowlisted for
// exactly 2 "spec-050" occurrences) REDs the scan even though the token
// itself is on the allowlist — the count, not just the token identity, is
// enforced. Mutation applied to a fixture copy of the real surfaces map,
// never the shipped file.
func TestReviewDiscipline_AC10NegativeDuplicateToken(t *testing.T) {
	surfaces := ac10Surfaces()
	original := surfaces["ms-panel-tally"]
	if !strings.Contains(original, "spec-050") {
		t.Fatal("fixture assumption broken: ms-panel-tally no longer contains spec-050 at all")
	}
	surfaces["ms-panel-tally"] = original + "\n\nA third spec-050 reference injected for the test.\n"

	hits := acTenScanSurfaces(surfaces)
	problems := acTenValidate(hits, reviewDisciplineAC10Allowlist)
	if len(problems) == 0 {
		t.Fatal("expected a 3rd spec-050 occurrence in ms-panel-tally (allowlisted for exactly 2) to RED the scan")
	}
	for _, p := range problems {
		if !strings.Contains(p, "ms-panel-tally") {
			t.Errorf("unexpected unrelated AC-10 problem alongside the duplicate-token overrun: %s", p)
		}
	}
}

// TestReviewDiscipline_AC10NegativeCountPreservingSubstitution proves a
// disallowed class cannot consume an allowed locator/count slot: ONE
// allowed "spec-050" occurrence at an allowlisted locator is replaced with
// "gpt-4o" — the locator's TOTAL token count is unchanged (still two
// tokens) — and the scan must still RED, because the allowlist key is
// (surface, EXACT matched text), not a raw hit count.
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
	problems := acTenValidate(hits, reviewDisciplineAC10Allowlist)
	if len(problems) == 0 {
		t.Fatal("expected the count-preserving spec-050->gpt-4o substitution to RED the AC-10 scan")
	}
}

// TestReviewDiscipline_AC10NegativeUnusedAllowlistEntry proves the OTHER
// residual the occurrence-accounted allowlist must catch: an allowlist
// entry that no scan hit ever consumes (e.g. left behind after a future
// cleanup) REDs too, so a stale entry cannot linger as a silent hole.
func TestReviewDiscipline_AC10NegativeUnusedAllowlistEntry(t *testing.T) {
	extended := append([]acTenAllowEntry{}, reviewDisciplineAC10Allowlist...)
	extended = append(extended, acTenAllowEntry{
		surface: "ms-bead-fix",
		locator: "synthetic — never actually present in the shipped skill",
		matched: "spec-999",
		count:   1,
		reason:  "test-only: demonstrates an unconsumed allowlist entry REDs",
	})

	hits := acTenScanSurfaces(ac10Surfaces())
	problems := acTenValidate(hits, extended)
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
