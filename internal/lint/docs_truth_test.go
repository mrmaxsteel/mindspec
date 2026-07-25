// docs_truth_test.go is the docs-truth lint proper: mindspec-ks4u
// (spec w0-docs-truth Bead 2). It makes Bead 1's "docs claims registry"
// convention (project-docs/claims-registry.yaml) mechanically
// un-repeatable by checking, over README.md and project-docs/** (see
// docsScopeRoots/docsScopeExcludeDirs in docs_truth_docs_test.go):
//
//	R1  every `mindspec <verb> [subverb] [--flag]` invocation resolves
//	    against the real cobra tree (docs_truth_cmdtree_test.go) or
//	    carries the marker grammar.
//	R2  every `/ms-*` reference resolves against the real skill/workflow
//	    universe (docs_truth_skills_test.go) or carries the marker; a
//	    slash-prefixed retired pre-"ms-"-rename spelling (e.g.
//	    `/spec-approve`) is caught the same way (O2-7).
//	R3  every marker's id is registered, and every registered claim has
//	    at least one marked site whose literal tokens are all, in turn,
//	    covered by a marker bearing that id (both directions, per the
//	    registry's own contract).
//	R5  a command that structurally resolves to a one-shot deprecation
//	    stub (Run, not RunE — see docs_truth_cmdtree_test.go) does NOT
//	    count as "resolves" for R1.
//
// # Finding: an unlintable residue beyond the one Bead 1 already named
//
// Bead 1's brief already names one mechanically-unenforceable class:
// an inert referent described as live. Running R1 for real surfaces a
// second instance of exactly that class, extended from "absent" to
// "removed" referents: project-docs/user/guides/agentmind.md (two
// sites) and project-docs/user/README.md:112 truthfully describe the
// six retired agentmind/viz/bench verbs in retrospective prose ("If you
// remember the old verbs...", "were removed by spec 084",
// "deprecation stubs") — accurate statements that a mechanical checker
// cannot distinguish from a wrongly-live-presented claim without prose
// understanding. knownDocsTruthExceptions below is this lint's
// deliberately narrow, audited answer: exactly these sites, each
// justified, keyed on File+Text+Count (never Line — see the type's doc
// comment for why a line-keyed table is itself a hiding place, and
// Count for why File+Text alone is a hiding place TOO — Bead 3's
// finding F2-2/O1-2/O2-2/O3-2), is-matched, and re-verified every run —
// the same ratchet idiom ratchet_argv_table_test.go already uses in
// this package for its own audited-but-unenforceable call sites (a
// semantic identity, never a position). Anything NOT on this list
// still fails, and any occurrence beyond an entry's audited Count
// still fails too. This is not a loosening of R1/R2/R3; it is the
// visible, auditable form of the exemption the bead brief says already
// exists by (undocumented, unenforced) convention.
//
// A second, unrelated finding is on the list for a different reason:
// `/ms-explore` (project-docs/user/CLAUDE.md, guides/claude-code.md)
// has no install code path in internal/setup or plugins/mindspec at
// all — see docs_truth_skills_test.go's doc comment. It is a genuine
// gap, not a convention; it is listed here only because fixing it is a
// product decision (ship ms-explore for real, or retract the doc
// claim) outside this bead's scope, and the alternative — silently
// dropping it — would hide a real finding. See the bead report.
//
// # What is, and is not, mechanically checked (O2-9)
//
// An inert config key (declared, parsed, validated, printed —
// cmd/mindspec/config.go's inertAnnotation, currently models:/loop:/
// runner:) described in prose as though it behaves is NOT checked here
// — a doc paragraph asserting behavior for such a key produces zero
// findings from R1/R2/R3, because the untruth is in the VERB ("selects
// which model runs"), not in a resolvable command/skill token. That
// half genuinely needs prose understanding, same as the
// removed-verb-retrospective-prose class above. But the inert KEY SET
// itself is not similarly unknowable: it is exactly the const at
// cmd/mindspec/config.go:22, consumed at its three inertAnnotation call
// sites (:209/:211 models:, :249 loop:, :275 runner:) — a mechanical
// half-measure (require the literal "declared, not yet enforced" in
// any doc section mentioning one of those keys) was available and is
// deferred, not built, in this bead. Tracked for a follow-up rather
// than silently treated as part of the "not mechanically lintable"
// claim this comment used to overstate.
package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// --- pure check functions (shared by the real-corpus test and every fixture test) ---

type truthFinding struct {
	File string
	Line int
	Text string // the specific invocation/token/marker text
	Why  string
}

func (f truthFinding) String() string {
	return fmt.Sprintf("%s:%d: %q — %s", f.File, f.Line, f.Text, f.Why)
}

// checkR1 walks every extracted `mindspec ...` invocation in each doc
// and reports one finding per resolution failure that is not covered
// by a marker whose registered claim's tokens include the specific
// unresolved text (markerTokenCovers — F2-1/O2-1: a marker exempts
// only the claim it was registered for, never every unresolved
// invocation in its heading-scoped range; marker id registration
// itself is R3's job).
func checkR1(docs []*docFile, cmdRoot *cmdNode, reg *claimsRegistryDoc) []truthFinding {
	var out []truthFinding
	for _, df := range docs {
		for _, occ := range extractInvocations(df) {
			for _, words := range expandAlternatives(tokenizeInvocation(occ.Text)) {
				res := cmdRoot.resolve(words)
				if res.Resolved {
					continue
				}
				text := joinWords(words)
				if markerTokenCovers(df, occ.Line, text, reg) {
					continue
				}
				out = append(out, truthFinding{File: df.Path, Line: occ.Line, Text: text, Why: res.Reason})
			}
		}
	}
	return out
}

// checkR2 is R1's analogue for `/ms-*` skill/workflow references. It
// also enforces the O2-7 residual: a slash-prefixed occurrence of a
// name that used to be a valid skill/workflow before the "ms-" prefix
// rename (e.g. `/spec-approve`, retired when `ms-spec-approve` was
// adopted) is reported too, even though it never collides with
// skillRefRe's `ms-` anchor — see extractRetiredSlashRefs and
// retiredPreRenameNames.
func checkR2(docs []*docFile, skills skillUniverse, reg *claimsRegistryDoc) []truthFinding {
	var out []truthFinding
	retired := retiredPreRenameNames(skills)
	for _, df := range docs {
		for _, sref := range extractSkillRefs(df) {
			if _, ok := skills[sref.Name]; ok {
				continue
			}
			text := "/" + sref.Name
			if markerTokenCovers(df, sref.Line, text, reg) {
				continue
			}
			out = append(out, truthFinding{File: df.Path, Line: sref.Line, Text: text, Why: "no such skill or workflow"})
		}
		for _, sref := range extractRetiredSlashRefs(df, retired) {
			text := "/" + sref.Name
			if markerTokenCovers(df, sref.Line, text, reg) {
				continue
			}
			out = append(out, truthFinding{File: df.Path, Line: sref.Line, Text: text, Why: fmt.Sprintf("retired pre-rename spelling — now /ms-%s", sref.Name)})
		}
	}
	return out
}

// checkR3 enforces the registry's both-direction contract: every
// marker id must be registered (direction 1), and every registered
// claim must have at least one marked site whose tokens are, in turn,
// all covered by a marker bearing that same id (direction 2 + the
// token-coverage clause).
func checkR3(docs []*docFile, reg *claimsRegistryDoc) []truthFinding {
	var out []truthFinding

	for _, df := range docs {
		for _, m := range df.Markers {
			if _, ok := reg.Claims[m.ID]; !ok {
				out = append(out, truthFinding{File: df.Path, Line: m.Line, Text: "claim `" + m.ID + "`", Why: "marker id is not registered in project-docs/claims-registry.yaml"})
			}
		}
	}

	ids := make([]string, 0, len(reg.Claims))
	for id := range reg.Claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		claim := reg.Claims[id]
		anySite := false
		for _, df := range docs {
			for _, m := range df.Markers {
				if m.ID == id {
					anySite = true
				}
			}
			for _, tok := range claim.Tokens {
				for _, line := range df.findAllLiteral(tok) {
					if covered, _ := df.coveredBy(line, id); !covered {
						out = append(out, truthFinding{File: df.Path, Line: line, Text: tok, Why: "token occurrence for claim `" + id + "` is not covered by a claim-`" + id + "` marker"})
					}
				}
			}
		}
		if !anySite {
			out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Line: 0, Text: "claim `" + id + "`", Why: "registered claim has no marked site anywhere in the scanned docs — stale entry, should have been deleted when the work landed"})
		}
	}
	return out
}

// --- the real-corpus test -------------------------------------------

// docsTruthException is one audited, pre-existing exception to R1/R2 —
// see the package doc comment above for what these two classes are and
// why they are not simply fixed or suppressed silently.
//
// Anchored on File+Text, NOT File+Line. Line numbers drift under
// perfectly ordinary edits (inserting a paragraph above shifts every
// line below it) — a line-keyed entry would then silently exempt
// whatever unrelated claim happens to land on the old line number,
// which is exactly the "anonymous label becomes the next hiding
// place" failure R3 already guards against for markers. Text is the
// exact, normalized invocation/skill-reference string the check
// produced (docs_truth_test.go's joinWords / the "/name" form) — the
// same content-identity idiom ratchet_argv_table_test.go uses (a
// semantic key, never a line number) rather than the position-based
// shape this table used before this fix. Line is kept only as
// diagnostic context in comments/output, never compared.
//
// Count is the exact number of findings this entry is audited to
// cover (F2-2/O1-2/O2-2/O3-2 — four independent panel slots, four
// working exploits: File+Text alone exempts UNBOUNDED future
// occurrences of that string in that file, so a plausible later edit
// re-documenting the same removed verb, or reusing the bare
// `mindspec bench`/`mindspec viz` spelling, in the same file, passed
// silently). applyExceptions fails when the observed occurrence count
// for an entry's File+Text differs from Count in EITHER direction: an
// unmatched entry (0 observed) is the pre-existing stale-entry failure,
// and — the new half — MORE than Count occurrences means a new,
// unaudited claim landed under cover of an old audit and must be
// reported like any other unexempted finding. Every current entry
// audits exactly one site, so every Count below is 1.
type docsTruthException struct {
	File   string
	Text   string
	Count  int // exact audited occurrence count for File+Text — see doc comment above
	Line   int // diagnostic only — NOT part of the matching key
	Reason string
}

var knownDocsTruthExceptions = []docsTruthException{
	// project-docs/user/README.md:112 — "(The old `mindspec
	// agentmind|viz|bench` verbs moved to ... hidden deprecation
	// stubs ...)": truthful retrospective prose, the
	// inert-referent-described-as-live class extended to removed (not
	// just absent) referents. See package doc comment.
	{File: "project-docs/user/README.md", Line: 112, Count: 1, Text: "mindspec agentmind", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/README.md", Line: 112, Count: 1, Text: "mindspec viz", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/README.md", Line: 112, Count: 1, Text: "mindspec bench", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},

	// project-docs/user/guides/agentmind.md:7 — "If you remember the
	// old verbs: ... were removed by spec 084 ... hidden deprecation
	// stubs": same class as above.
	{File: "project-docs/user/guides/agentmind.md", Line: 7, Count: 1, Text: "mindspec agentmind serve", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 7, Count: 1, Text: "mindspec agentmind replay", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 7, Count: 1, Text: "mindspec agentmind setup", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 7, Count: 1, Text: "mindspec viz", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 7, Count: 1, Text: "mindspec bench …", Reason: "truthful retrospective prose about a removed verb (literal trailing ellipsis from the source text) — see package doc comment"},

	// project-docs/user/guides/agentmind.md:52 — "(formerly `mindspec
	// bench`) moved to the agentmind repo ... deprecation stubs": same
	// class as above.
	{File: "project-docs/user/guides/agentmind.md", Line: 52, Count: 1, Text: "mindspec bench", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 52, Count: 1, Text: "mindspec bench setup", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 52, Count: 1, Text: "mindspec bench collect", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 52, Count: 1, Text: "mindspec bench report", Reason: "truthful retrospective prose about a removed verb — see package doc comment"},

	// /ms-explore: a genuine gap, not a convention — see
	// docs_truth_skills_test.go's doc comment. Listed here (rather than
	// fixed) because the fix is a product decision (ship it for real,
	// or retract the doc claim) outside this bead's scope. O1-2
	// sharpened this: the gap is a missing `mindspec explore` VERB, not
	// only a missing install path (the skill's own body invokes
	// `mindspec explore`, so there is no verb to wire an install path
	// to) — the string exempted here is a KNOWN UNTRUTH pending a
	// product decision, not audited-acceptable prose, which is exactly
	// why the Count ratchet below matters most for these two rows: it
	// stops the untruth from multiplying if `/ms-explore` gets
	// mentioned again elsewhere in either file.
	{File: "project-docs/user/CLAUDE.md", Line: 20, Count: 1, Text: "/ms-explore", Reason: "genuine /ms-explore install-path gap (no mindspec explore verb exists to install a path to), tracked pending a product decision — not fixed by this bead"},
	{File: "project-docs/user/guides/claude-code.md", Line: 94, Count: 1, Text: "/ms-explore", Reason: "genuine /ms-explore install-path gap (no mindspec explore verb exists to install a path to), tracked pending a product decision — not fixed by this bead"},
}

// applyExceptions partitions findings into those covered by an
// occurrence-count-respecting File+Text-matching entry in exceptions
// and those that are not (unexpected — these must fail the caller's
// test), and reports which exception entries were matched at least
// once (an unmatched entry is stale and must also fail the caller's
// test, per R3's own both-directions discipline). Matching is
// deliberately on File+Text, never Line — see docsTruthException's
// doc comment for why — but, per that same doc comment, is bounded by
// Count: findings are grouped by (File, Text), and only the first
// e.Count of each group are treated as covered; any beyond that are
// new, unaudited occurrences and are returned as unexpected exactly
// like any other unexempted finding.
func applyExceptions(findings []truthFinding, exceptions []docsTruthException) (unexpected []truthFinding, matched []bool) {
	matched = make([]bool, len(exceptions))

	type key struct{ file, text string }
	idxByKey := make(map[key]int, len(exceptions))
	for i, e := range exceptions {
		idxByKey[key{e.File, e.Text}] = i
	}

	byKey := make(map[key][]truthFinding)
	var order []key
	for _, f := range findings {
		k := key{f.File, f.Text}
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], f)
	}

	for _, k := range order {
		group := byKey[k]
		i, known := idxByKey[k]
		if !known {
			unexpected = append(unexpected, group...)
			continue
		}
		matched[i] = true
		if extra := len(group) - exceptions[i].Count; extra > 0 {
			unexpected = append(unexpected, group[len(group)-extra:]...)
		}
	}
	return unexpected, matched
}

// TestDocsTruthReal runs R1+R2+R3 against the real, live docs surface.
// Any finding not on knownDocsTruthExceptions fails the test; any
// exception entry that goes unmatched (the doc changed and no longer
// reproduces it) also fails, forcing the table to stay honest — the
// same both-directions discipline R3 itself enforces on the claims
// registry.
func TestDocsTruthReal(t *testing.T) {
	repoRoot := realRepoRoot(t)

	cmdRoot, err := buildCmdTree(realCmdDir(t))
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	skills, err := buildSkillUniverse(repoRoot)
	if err != nil {
		t.Fatalf("buildSkillUniverse: %v", err)
	}
	reg, err := loadClaimsRegistry(repoRoot)
	if err != nil {
		t.Fatalf("loadClaimsRegistry: %v", err)
	}

	relPaths, err := listScopedDocs(repoRoot)
	if err != nil {
		t.Fatalf("listScopedDocs: %v", err)
	}
	var docs []*docFile
	for _, rel := range relPaths {
		if filepath.Ext(rel) != ".md" {
			continue
		}
		df, err := parseDocFile(repoRoot, rel)
		if err != nil {
			t.Fatalf("parseDocFile %s: %v", rel, err)
		}
		docs = append(docs, df)
	}

	var findings []truthFinding
	findings = append(findings, checkR1(docs, cmdRoot, reg)...)
	findings = append(findings, checkR2(docs, skills, reg)...)
	findings = append(findings, checkR3(docs, reg)...)

	unexpected, matched := applyExceptions(findings, knownDocsTruthExceptions)

	if len(unexpected) > 0 {
		var b []string
		for _, f := range unexpected {
			b = append(b, f.String())
		}
		t.Errorf("docs-truth violations with no audited exception (%d):\n  %s", len(unexpected), joinLines(b))
	}
	for i, ok := range matched {
		if !ok {
			t.Errorf("stale exception entry (no longer reproduces — fix the doc drift, or remove this entry): %s:%d %q",
				knownDocsTruthExceptions[i].File, knownDocsTruthExceptions[i].Line, knownDocsTruthExceptions[i].Reason)
		}
	}
}

// TestExceptionsAreContentAnchoredNotLineAnchored is the regression
// test for the defect a File+Line-keyed exception table had: an
// unrelated edit inserting a paragraph above an exempted claim shifts
// it to a new line, AND leaves whatever unrelated (possibly untrue)
// claim now occupies the OLD line silently exempted too. Content
// (File+Text) anchoring must fix both halves of that failure:
//
//  1. the real exempted claim keeps matching even after it moves to a
//     different line (the "doc got edited, nothing actually changed
//     about this claim" case — must NOT become a stale-entry failure);
//  2. a DIFFERENT, untrue claim that lands at the exact file+line an
//     exemption used to occupy must still be reported — the exact
//     "one unrelated edit converts an audited exception into a blind
//     spot" scenario the fix closes.
func TestExceptionsAreContentAnchoredNotLineAnchored(t *testing.T) {
	exceptions := []docsTruthException{
		{File: "guide.md", Line: 7, Count: 1, Text: "mindspec viz", Reason: "test fixture: truthful historical mention"},
	}

	// Simulates the doc after an unrelated paragraph was inserted
	// above: the real exempted claim is now three lines lower...
	movedRealClaim := truthFinding{File: "guide.md", Line: 10, Text: "mindspec viz", Why: "resolves to a one-shot deprecation stub"}
	// ...and a completely different, untrue claim an author
	// introduced in that same edit now sits at line 7, the exact
	// position the exception used to anchor on.
	differentUntrueClaim := truthFinding{File: "guide.md", Line: 7, Text: "mindspec onboard --infer", Why: `no subcommand "onboard" under "mindspec"`}

	unexpected, matched := applyExceptions([]truthFinding{movedRealClaim, differentUntrueClaim}, exceptions)

	if !matched[0] {
		t.Error("expected the real claim's exception to still match after it moved to a new line — content anchoring must not care about position")
	}
	foundMoved := false
	foundDifferent := false
	for _, f := range unexpected {
		if f.Text == "mindspec viz" {
			foundMoved = true
		}
		if f.Text == "mindspec onboard --infer" {
			foundDifferent = true
		}
	}
	if foundMoved {
		t.Error("the real exempted claim must not appear as unexpected just because it moved lines")
	}
	if !foundDifferent {
		t.Fatal("the different, untrue claim that landed at the exempted line/position must still be reported — a line-keyed table would have silently swallowed this")
	}
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n  "
		}
		out += l
	}
	return out
}

// --- fixture helpers --------------------------------------------------

// fixtureDoc writes content to <tmpdir>/<name> and parses it as a
// docFile through the exact same code path the real scan uses.
func fixtureDoc(t *testing.T, name, content string) *docFile {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	df, err := parseDocFile(dir, name)
	if err != nil {
		t.Fatalf("parseDocFile fixture: %v", err)
	}
	return df
}

func sharedCmdRoot(t *testing.T) *cmdNode {
	t.Helper()
	root, err := buildCmdTree(realCmdDir(t))
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	return root
}

func sharedSkills(t *testing.T) skillUniverse {
	t.Helper()
	skills, err := buildSkillUniverse(realRepoRoot(t))
	if err != nil {
		t.Fatalf("buildSkillUniverse: %v", err)
	}
	return skills
}

// sharedRegistry loads the real project-docs/claims-registry.yaml —
// used by fixtures that need a real claim (e.g. loop-status) to prove
// the marker+token exemption actually fires, as opposed to fixtures
// that need to prove it does NOT fire for an unregistered/mismatched
// claim, which pass an empty or synthetic registry instead.
func sharedRegistry(t *testing.T) *claimsRegistryDoc {
	t.Helper()
	reg, err := loadClaimsRegistry(realRepoRoot(t))
	if err != nil {
		t.Fatalf("loadClaimsRegistry: %v", err)
	}
	return reg
}

// --- R4: negative fixtures, one per required class -------------------

// TestR4AbsentVerbFlag: an absent flag on a real verb, unmarked, must
// fire R1. (`mindspec onboard` does not exist as a verb at all in the
// real tree — Bead 1 retracted it — so this fixture exercises the
// "verb resolves, flag does not" arm specifically by using a real verb
// with a real flag name deliberately misspelled/absent: `doctor` has
// `--fix` but not `--infer`.)
func TestR4AbsentVerbFlag(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run `mindspec doctor --infer` to auto-detect the mode.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	requireFindingContains(t, findings, "doctor --infer", "flag --infer not registered")
}

// TestR4AbsentSubcommand: an absent subcommand, unmarked, must fire
// R1. Mirrors the real `mindspec loop status` site (README.md:145) but
// WITHOUT the marker that makes the real site pass — proving the
// marker, not some blanket leniency, is what saves the real one.
func TestR4AbsentSubcommand(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Poll progress with `mindspec loop status`.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	requireFindingContains(t, findings, "loop status", `no subcommand "loop"`)
}

// TestR4AbsentSubcommand_MarkedVersionPasses proves the marker escape
// valve actually works (not just that its absence fails): the same
// invocation, now carrying a valid registered-id marker WHOSE
// REGISTERED TOKENS INCLUDE THE EXACT TEXT (the real registry's
// loop-status claim tokens ["mindspec loop status"]), produces zero R1
// findings.
func TestR4AbsentSubcommand_MarkedVersionPasses(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Poll progress with `mindspec loop status` *(planned — claim `loop-status`, roadmap Core 4)*.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero R1 findings on a marked absent-subcommand claim whose text matches the claim's registered tokens, got: %v", findings)
	}
}

// TestR4MarkedButUnrelatedClaimStillFails is the regression test for
// the marker-amnesty defect (F2-1/O1/O2-1): a marker's heading-scoped
// coverage range must exempt ONLY the specific claim it names via its
// registered tokens, never every unresolved claim that happens to fall
// inside that range. Same section, same valid loop-status marker as
// the passing fixture above — but a SECOND, unrelated absent-verb
// claim in the same section, whose text is not one of loop-status's
// registered tokens, must still fire R1. (This reproduces, in
// miniature, the orchestrator's real-corpus repro: injecting
// `mindspec onboard --infer` under autonomy.md's marker-covered
// section passed before this fix.)
func TestR4MarkedButUnrelatedClaimStillFails(t *testing.T) {
	content := "## Status\n\n" +
		"Poll progress with `mindspec loop status` *(planned — claim `loop-status`, roadmap Core 4)*.\n" +
		"Also try `mindspec bogusverb --nope`, which is unrelated to that claim.\n"
	df := fixtureDoc(t, "fixture.md", content)
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	requireFindingContains(t, findings, "bogusverb --nope", `no subcommand "bogusverb"`)
	for _, f := range findings {
		if f.Text == "mindspec loop status" {
			t.Errorf("the marked, token-matched claim must still pass; got an unexpected finding for it: %s", f)
		}
	}
}

// TestR4AbsentSkill: an absent skill, unmarked, must fire R2.
func TestR4AbsentSkill(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run the fix lane with /ms-fix-cycle.\n")
	findings := checkR2([]*docFile{df}, sharedSkills(t), sharedRegistry(t))
	requireFindingContains(t, findings, "/ms-fix-cycle", "no such skill or workflow")
}

// TestR4MarkedSkillPassesTokenMatch: the R2 analogue of
// TestR4AbsentSubcommand_MarkedVersionPasses — the real fix-cycle
// claim's marker (token "/ms-fix-cycle") exempts the matching skill
// ref.
func TestR4MarkedSkillPassesTokenMatch(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run the fix lane with /ms-fix-cycle *(planned — claim `fix-cycle`, roadmap Core 6)*.\n")
	findings := checkR2([]*docFile{df}, sharedSkills(t), sharedRegistry(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero R2 findings on a marked absent-skill claim whose text matches the claim's registered tokens, got: %v", findings)
	}
}

// TestR4MarkedButUnrelatedSkillStillFails is R2's analogue of
// TestR4MarkedButUnrelatedClaimStillFails: a second, unrelated absent
// skill ref in the same fix-cycle-marked section, not itself one of
// that claim's registered tokens, must still fire R2.
func TestR4MarkedButUnrelatedSkillStillFails(t *testing.T) {
	content := "## Fix lane\n\n" +
		"Run the fix lane with /ms-fix-cycle *(planned — claim `fix-cycle`, roadmap Core 6)*.\n" +
		"Also see /ms-totally-bogus-skill for something unrelated.\n"
	df := fixtureDoc(t, "fixture.md", content)
	findings := checkR2([]*docFile{df}, sharedSkills(t), sharedRegistry(t))
	requireFindingContains(t, findings, "/ms-totally-bogus-skill", "no such skill or workflow")
	for _, f := range findings {
		if f.Text == "/ms-fix-cycle" {
			t.Errorf("the marked, token-matched skill ref must still pass; got an unexpected finding for it: %s", f)
		}
	}
}

// --- O2-7: retired pre-"ms-"-rename slash spelling ---------------------

// TestO2_7RetiredSlashSpellingFails is the regression fixture for the
// O2-7 residual: skillRefRe's `ms-` anchor let the bare pre-rename
// spelling of a real skill escape R2 entirely (the exact shape of the
// F1 finding: `/spec-approve` in the codex/copilot comparison tables,
// retired when spec 105 adopted the ms- prefix). `/spec-approve`
// itself no longer occurs in the real corpus (W0 Bead 4 replaced it
// with `/ms-spec-approve`), so this fixture reconstructs the shape
// directly rather than relying on a real site that may not exist by
// the time this runs.
func TestO2_7RetiredSlashSpellingFails(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "See /spec-approve for the old gate name.\n")
	findings := checkR2([]*docFile{df}, sharedSkills(t), sharedRegistry(t))
	requireFindingContains(t, findings, "/spec-approve", "retired pre-rename spelling")
}

// TestO2_7CurrentMsSpellingPasses is the positive twin: the CURRENT
// `/ms-spec-approve` spelling of the same skill must not be flagged as
// retired (it resolves directly against skillUniverse in checkR2's
// first loop, before extractRetiredSlashRefs ever runs).
func TestO2_7CurrentMsSpellingPasses(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "See /ms-spec-approve for the gate.\n")
	findings := checkR2([]*docFile{df}, sharedSkills(t), sharedRegistry(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero R2 findings for the current /ms-spec-approve spelling, got: %v", findings)
	}
}

// TestO2_7OrdinaryProseSlashesDoNotFalsePositive proves
// extractRetiredSlashRefs's narrow, exact-alternation design does not
// turn ordinary prose slashes into findings — the risk a generic "any
// slash-prefixed hyphenated word" pattern would have carried.
func TestO2_7OrdinaryProseSlashesDoNotFalsePositive(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Toggle the feature on/off, and/or read the release notes at v1/2026-07.\n")
	findings := checkR2([]*docFile{df}, sharedSkills(t), sharedRegistry(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero R2 findings on ordinary prose slashes, got: %v", findings)
	}
}

// TestApplyExceptions_DuplicateOccurrenceStillFails is the regression
// test for the exception-multiplicity defect (F2-2/O1-2/O2-2/O3-2 —
// four independent panel slots, each with a working exploit): a single
// audited File+Text entry must NOT exempt more occurrences of that
// exact string than it was audited to cover. A second, genuinely new
// occurrence of the same File+Text — the shape of the real exploit
// (e.g. a future edit re-documenting `mindspec bench` as live in
// project-docs/user/README.md, where the bare string was already
// exempted for one truthful retrospective mention) — must be reported.
func TestApplyExceptions_DuplicateOccurrenceStillFails(t *testing.T) {
	exceptions := []docsTruthException{
		{File: "guide.md", Line: 52, Count: 1, Text: "mindspec bench", Reason: "test fixture: one audited truthful retrospective mention"},
	}
	auditedOccurrence := truthFinding{File: "guide.md", Line: 52, Text: "mindspec bench", Why: "resolves to a one-shot deprecation stub"}
	newUnauditedOccurrence := truthFinding{File: "guide.md", Line: 63, Text: "mindspec bench", Why: "resolves to a one-shot deprecation stub"}

	unexpected, matched := applyExceptions([]truthFinding{auditedOccurrence, newUnauditedOccurrence}, exceptions)

	if !matched[0] {
		t.Error("expected the entry to match at least once")
	}
	if len(unexpected) != 1 {
		t.Fatalf("expected exactly 1 unexpected finding (the occurrence beyond the audited Count), got %d: %v", len(unexpected), unexpected)
	}
	if unexpected[0].Line != 63 {
		t.Errorf("expected the SECOND (new, unaudited) occurrence to be the one reported, got line %d", unexpected[0].Line)
	}
}

// TestR4UnregisteredLabel: a marker whose id is NOT in the registry
// must fire R3 direction 1 — an anonymous/unregistered "coming soon" is
// exactly the failure mode the registry exists to prevent (per its own
// header comment).
func TestR4UnregisteredLabel(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Something new is coming *(planned — claim `totally-unregistered-id`, some day)*.\n")
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{}}
	findings := checkR3([]*docFile{df}, reg)
	requireFindingContains(t, findings, "claim `totally-unregistered-id`", "not registered")
}

// TestR3RegistryDirection2_StaleEntryFails proves the OTHER direction:
// a registered claim with zero marked sites anywhere in the scanned
// docs must fail — an entry whose doc sites all disappeared is stale
// and should have been deleted when the work landed (registry header
// comment, point 4).
func TestR3RegistryDirection2_StaleEntryFails(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Nothing here mentions the claim at all.\n")
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"orphaned-claim": {Tokens: []string{"mindspec ghost verb"}, Owner: "nobody"},
	}}
	findings := checkR3([]*docFile{df}, reg)
	requireFindingContains(t, findings, "claim `orphaned-claim`", "no marked site")
}

// TestR3TokenCoverage_UncoveredOccurrenceFails proves the token-level
// clause: a registered token appearing OUTSIDE any matching marker's
// heading-scoped coverage fails, even though a marker with that id
// exists somewhere else in the same file.
func TestR3TokenCoverage_UncoveredOccurrenceFails(t *testing.T) {
	content := "# Section A\n\n" +
		"`mindspec loop status` *(planned — claim `loop-status`, roadmap Core 4)* is covered here.\n\n" +
		"# Section B\n\n" +
		"But `mindspec loop status` is mentioned again here, past this heading of equal level, so it is out of the marker's coverage.\n"
	df := fixtureDoc(t, "fixture.md", content)
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"loop-status": {Tokens: []string{"mindspec loop status"}, Owner: "roadmap"},
	}}
	findings := checkR3([]*docFile{df}, reg)
	requireFindingContains(t, findings, "mindspec loop status", "not covered by a claim-`loop-status` marker")
}

func requireFindingContains(t *testing.T, findings []truthFinding, textSub, whySub string) {
	t.Helper()
	for _, f := range findings {
		if containsSub(f.Text, textSub) && containsSub(f.Why, whySub) {
			return
		}
	}
	var got []string
	for _, f := range findings {
		got = append(got, f.String())
	}
	t.Fatalf("expected a finding with text~=%q why~=%q, got %d findings:\n  %s", textSub, whySub, len(findings), joinLines(got))
}

func containsSub(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// --- expandAlternatives: both alternation forms (O1-1/O2-4) -----------

// TestExpandAlternatives_SpaceSeparatedFormChecksEveryAlternative is
// the regression test for O1-1/O2-4's primary defect: the
// space-separated bare-pipe form (README.md:77's own
// `mindspec panel create | verify | tally`) previously folded the
// FIRST alternative into the prefix and never generated it as its own
// candidate, so a retired/renamed/bogus later alternative went
// unchecked. `panel create`/`panel verify`/`panel tally` are all real
// (cmd/mindspec/panel.go); substituting a bogus final alternative must
// produce a finding for it, and neither real alternative may
// spuriously appear as a finding.
func TestExpandAlternatives_SpaceSeparatedFormChecksEveryAlternative(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run `mindspec panel create | verify | bogusverb`.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	requireFindingContains(t, findings, "panel bogusverb", `no subcommand "bogusverb"`)
	for _, f := range findings {
		if containsSub(f.Text, "panel create") || containsSub(f.Text, "panel verify") {
			t.Errorf("real alternatives must not appear as findings: %s", f)
		}
	}
}

// TestExpandAlternatives_GluedFormPreservesTail is the regression test
// for O1-1/O2-4's second defect: the glued single-token form
// (`create|verify`) previously folded every trailing word — including
// flags and flag VALUES — into the alternative list instead of
// carrying them along on each alternative, generating garbage
// candidates (`spec --title`, `spec x`) and never checking the true
// claim. `mindspec spec create` takes a real `--title` flag
// (cmd/mindspec/spec.go:179); `spec verify` does not exist. The fixed
// expansion must produce EXACTLY one finding — `spec verify --title
// x` — and no noise about `--title` or `x` as bogus subcommands.
func TestExpandAlternatives_GluedFormPreservesTail(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run `mindspec spec create|verify --title x`.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding (spec verify --title x), got %d: %v", len(findings), findings)
	}
	requireFindingContains(t, findings, "spec verify --title x", `no subcommand "verify"`)
}

// --- resolveHelperCall: one-level helper indirection (O2-3) -----------

// TestResolveHelperCall_ChildrenAndFlagsVisible is the regression test
// for O2-3: a command built by one-level helper indirection
// (`reportCmd = newReportCmd()`, report.go:81-103) previously modeled
// as a childless, flagless leaf — so ANY invented subcommand resolved
// true via the leaf-positional-args arm, and the real `--resolve`
// flag on `report list` false-positived as unregistered. All three
// must now resolve correctly against the real tree.
func TestResolveHelperCall_ChildrenAndFlagsVisible(t *testing.T) {
	root := sharedCmdRoot(t)

	if res := root.resolve([]string{"mindspec", "report", "list"}); !res.Resolved {
		t.Errorf("expected mindspec report list to resolve, got: %s", res.Reason)
	}
	if res := root.resolve([]string{"mindspec", "report", "list", "--resolve", "abc123"}); !res.Resolved {
		t.Errorf("expected mindspec report list --resolve abc123 to resolve, got: %s", res.Reason)
	}
	if res := root.resolve([]string{"mindspec", "report", "bogus-subcommand"}); res.Resolved {
		t.Error("expected mindspec report bogus-subcommand to NOT resolve — it must not silently pass as a positional arg on a leaf")
	}
}

// --- auto-registered cobra flags: --help/-h/--version (O2-6) ---------

// TestAutoFlags_HelpAndVersionResolve is the regression test for
// O2-6: cobra auto-registers --help/-h on every command and --version
// on root (root.go:57 sets Version), but Pass 3 only records explicit
// Flags()/PersistentFlags() calls, so resolve() previously reported
// "flag --help not registered" on the most ordinary true invocation a
// doc can contain.
func TestAutoFlags_HelpAndVersionResolve(t *testing.T) {
	root := sharedCmdRoot(t)
	cases := [][]string{
		{"mindspec", "--help"},
		{"mindspec", "doctor", "--help"},
		{"mindspec", "--version"},
	}
	for _, words := range cases {
		if res := root.resolve(words); !res.Resolved {
			t.Errorf("expected %q to resolve, got: %s", joinWords(words), res.Reason)
		}
	}
}

// --- Args: cobra.NoArgs arity enforcement (O2-8, "at minimum") -------

// TestR1NoArgsRejectsPositional is the regression test for O2-8's
// minimum required fix: resolve() previously ignored Args entirely, so
// an invented positional argument on a cobra.NoArgs command (`version`,
// version.go:27) resolved true even though the real binary rejects it.
func TestR1NoArgsRejectsPositional(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Check the version with `mindspec version latest`.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	requireFindingContains(t, findings, "version latest", "does not accept positional arguments")
}

// TestR1ExactArgsRejectsExtraPositional is O2-8's ExactArgs/
// MaximumNArgs closure: adr.go:22 sets Args: cobra.ExactArgs(1) on
// `adr create <title>`; a doc inventing a SECOND concrete positional
// beyond the one placeholder must fail, exactly as the real binary
// would reject it.
func TestR1ExactArgsRejectsExtraPositional(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run `mindspec adr create foo bar` to file one.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	requireFindingContains(t, findings, "adr create foo bar", "accepts at most 1 positional argument")
}

// TestR1ExactArgsQuotedMultiWordTitlePasses is the regression fixture
// for the false positive TestR1ExactArgsRejectsExtraPositional's own
// fix nearly introduced: project-docs/user/README.md:84 documents
// `mindspec adr create "Use WebSockets for real-time updates" --domain
// viz` — a TRUE claim (one quoted <title> placeholder, one real flag)
// that whitespace tokenizing splits into five separate words. Without
// mergeQuotedPositionals collapsing the quoted phrase back into one
// placeholder token, each inner word after the first miscounts as an
// extra concrete positional and this true claim goes red.
func TestR1ExactArgsQuotedMultiWordTitlePasses(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run `mindspec adr create \"Use WebSockets for real-time updates\" --domain viz` to file one.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t), sharedRegistry(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero R1 findings for a quoted multi-word ExactArgs(1) title, got: %v", findings)
	}
}

// --- R5: stub vs. live-hidden-alias -----------------------------------

// TestR5SpecInitAliasResolvesLive is, per the bead brief, the most
// important test in this file: `spec-init` is Hidden AND (transitively,
// via specCreateCmd) shares flag-parsing behavior, yet it is a fully
// live alias (spec_init.go wires RunE = specCreateCmd.RunE in init()).
// A lint that used Hidden, or Hidden&&DisableFlagParsing, as its stub
// signal would wrongly flag this. Ours must not.
func TestR5SpecInitAliasResolvesLive(t *testing.T) {
	res := sharedCmdRoot(t).resolve([]string{"mindspec", "spec-init"})
	if !res.Resolved {
		t.Fatalf("expected mindspec spec-init to resolve live, got unresolved: %s", res.Reason)
	}
}

// writeSyntheticCmdDir writes files (relative name -> Go source) into
// a fresh temp dir shaped like cmd/mindspec, for R5/O1-3/O1-5 fixtures
// that must NOT depend on any real, shipped verb name — O1-4's
// finding (see TestR5InlineLiteralStubDoesNotResolve below): a fixture
// keyed on a real verb name goes vacuously green the moment that verb
// is deleted from cmd/mindspec, silently losing its coverage. Every
// file must be valid, self-contained Go — buildCmdTree parses real
// source, not a mock.
func writeSyntheticCmdDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write synthetic %s: %v", name, err)
		}
	}
	return dir
}

// TestR5InlineLiteralStubDoesNotResolve is O1-4's fix: R5's negative
// fixture used to be TestR5BenchStubDoesNotResolve, keyed on the real
// `bench` verb — which goes vacuously green the moment
// deprecated_commands.go is deleted (its own header says a follow-up
// does exactly that, one release after spec 084), silently losing R5
// negative coverage entirely. This constructs its OWN inline-literal
// stub shape (Run set directly on the composite literal, the
// discriminator's simplest form) rather than naming a shipped verb.
func TestR5InlineLiteralStubDoesNotResolve(t *testing.T) {
	dir := writeSyntheticCmdDir(t, map[string]string{
		"main.go": `package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{Use: "mindspec"}

var stubCmd = &cobra.Command{
	Use:    "syntheticstub",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {},
}

func init() {
	rootCmd.AddCommand(stubCmd)
}
`,
	})
	root, err := buildCmdTree(dir)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	res := root.resolve([]string{"mindspec", "syntheticstub"})
	if res.Resolved {
		t.Fatal("expected mindspec syntheticstub to NOT resolve (inline-literal deprecation stub)")
	}
	if !containsSub(res.Reason, "deprecation stub") {
		t.Fatalf("expected reason to cite the deprecation-stub signal, got: %s", res.Reason)
	}
}

// --- O1-3: out-of-line Run/RunE assignment (Pass 4) --------------------

// TestR5OutOfLineRunIsStub is O1-3's core regression test — R5's
// FOURTH wrong version, orchestrator-reproduced at 05cc8f7e: the
// pre-fix discriminator read Run/RunE off the `&cobra.Command{...}`
// LITERAL only. This fixture's stub command's literal sets NEITHER
// field — its Run is assigned out-of-line inside init(), the exact
// idiom cmd/mindspec/spec_init.go:18 already uses for a genuinely live
// command (RunE, not Run). Before Pass 4 existed, this classified
// live: a doc claiming this verb works would have passed.
func TestR5OutOfLineRunIsStub(t *testing.T) {
	dir := writeSyntheticCmdDir(t, map[string]string{
		"main.go": `package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{Use: "mindspec"}

var probeCmd = &cobra.Command{
	Use:    "probeverb",
	Hidden: true,
}

func init() {
	probeCmd.Run = func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "probeverb moved: see ADR-0000")
		os.Exit(2)
	}
	rootCmd.AddCommand(probeCmd)
}
`,
	})
	root, err := buildCmdTree(dir)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	res := root.resolve([]string{"mindspec", "probeverb"})
	if res.Resolved {
		t.Fatal("expected mindspec probeverb to NOT resolve (out-of-line Run makes it a deprecation stub)")
	}
	if !containsSub(res.Reason, "deprecation stub") {
		t.Fatalf("expected reason to cite the deprecation-stub signal, got: %s", res.Reason)
	}
}

// TestR5OutOfLineRunEIsLive is the live counterpart, decoupled from
// real spec_init.go (which TestR5SpecInitAliasResolvesLive above
// already covers and must keep covering — this is additional, not a
// replacement): a command whose literal sets neither Run nor RunE, but
// whose RunE IS assigned out-of-line in init(), must resolve live.
// Without this, Pass 4 could have been implemented to treat ANY
// out-of-line Run/RunE assignment as suspect rather than resolving the
// real field value — this proves the out-of-line RunE case is
// correctly classified live, not just the out-of-line Run case stub.
func TestR5OutOfLineRunEIsLive(t *testing.T) {
	dir := writeSyntheticCmdDir(t, map[string]string{
		"main.go": `package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{Use: "mindspec"}

var aliasCmd = &cobra.Command{
	Use:    "aliasverb",
	Hidden: true,
}

var realCmd = &cobra.Command{
	Use: "realverb",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	aliasCmd.RunE = realCmd.RunE
	rootCmd.AddCommand(aliasCmd)
	rootCmd.AddCommand(realCmd)
}
`,
	})
	root, err := buildCmdTree(dir)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	res := root.resolve([]string{"mindspec", "aliasverb"})
	if !res.Resolved {
		t.Fatalf("expected mindspec aliasverb to resolve live (out-of-line RunE), got unresolved: %s", res.Reason)
	}
}

// --- O1-5: variadic AddCommand(a, b, c) ---------------------------------

// TestO1_5VariadicAddCommandWalksAllChildren is the regression fixture
// for the multi-argument `AddCommand(a, b, c)` form — cobra's
// AddCommand is variadic, but the pre-fix extraction only ever took a
// single-argument call's lone arg, so every child registered via the
// multi-arg form was invisible to the tree entirely. This does NOT
// fail loudly as "no subcommand": resolve()'s leaf arm treats an
// unresolvable trailing word as a POSITIONAL ARGUMENT of the deepest
// node actually reached (here, `parent`, which the tree shows as
// childless) and still reports Resolved=true — the exact
// "positional-args arm swallows it" failure mode resolveHelperCall's
// own doc comment already names for the one-level-helper-indirection
// case. So this fixture asserts not just Resolved, but that the
// resolved NODE is actually the child itself.
func TestO1_5VariadicAddCommandWalksAllChildren(t *testing.T) {
	dir := writeSyntheticCmdDir(t, map[string]string{
		"main.go": `package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{Use: "mindspec"}

var parentCmd = &cobra.Command{Use: "parent"}

var childACmd = &cobra.Command{
	Use: "childa",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
var childBCmd = &cobra.Command{
	Use: "childb",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
var childCCmd = &cobra.Command{
	Use: "childc",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	parentCmd.AddCommand(childACmd, childBCmd, childCCmd)
	rootCmd.AddCommand(parentCmd)
}
`,
	})
	root, err := buildCmdTree(dir)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	for _, child := range []string{"childa", "childb", "childc"} {
		res := root.resolve([]string{"mindspec", "parent", child})
		if !res.Resolved {
			t.Errorf("expected mindspec parent %s to resolve (variadic AddCommand), got unresolved: %s", child, res.Reason)
			continue
		}
		// Checking Resolved alone is not enough: without the fix,
		// parentCmd has ZERO children in the tree (the variadic
		// AddCommand call is invisible), so resolve()'s leaf arm
		// treats "childa" as a POSITIONAL ARGUMENT of `parent` itself
		// and still reports Resolved=true — just against the wrong
		// node. Asserting the resolved node's own name is what
		// actually catches the regression.
		if res.Node == nil || res.Node.Name != child {
			gotName := "<nil>"
			if res.Node != nil {
				gotName = res.Node.Name
			}
			t.Errorf("expected mindspec parent %s to resolve TO the %s node itself, got node %q (likely swallowed as a positional arg of `parent`)", child, child, gotName)
		}
	}
}

// TestO1_5VariadicAddCommandInHelperBodyWalksAllChildren is
// TestO1_5VariadicAddCommandWalksAllChildren's sibling for the OTHER
// code path the variadic form must be fixed in: a parent built via
// one-level helper indirection (`parentCmd = newParentCmd()`,
// report.go:81's real idiom) registers its children INSIDE the
// helper's own body (`c.AddCommand(...)`), extracted by
// addCommandArgsForLocal — a separate function from Pass 2's
// package-level walk, with its own single-arg-only bug to fix.
func TestO1_5VariadicAddCommandInHelperBodyWalksAllChildren(t *testing.T) {
	dir := writeSyntheticCmdDir(t, map[string]string{
		"main.go": `package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{Use: "mindspec"}

var childACmd = &cobra.Command{
	Use: "childa",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
var childBCmd = &cobra.Command{
	Use: "childb",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func newParentCmd() *cobra.Command {
	c := &cobra.Command{Use: "parent"}
	c.AddCommand(childACmd, childBCmd)
	return c
}

var parentCmd = newParentCmd()

func init() {
	rootCmd.AddCommand(parentCmd)
}
`,
	})
	root, err := buildCmdTree(dir)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	for _, child := range []string{"childa", "childb"} {
		res := root.resolve([]string{"mindspec", "parent", child})
		if !res.Resolved {
			t.Errorf("expected mindspec parent %s to resolve (variadic AddCommand in a helper body), got unresolved: %s", child, res.Reason)
			continue
		}
		if res.Node == nil || res.Node.Name != child {
			gotName := "<nil>"
			if res.Node != nil {
				gotName = res.Node.Name
			}
			t.Errorf("expected mindspec parent %s to resolve TO the %s node itself, got node %q", child, child, gotName)
		}
	}
}

// --- degradation: deprecated_commands.go eventually goes away --------

// TestCmdTreeDegradesWithoutDeprecatedFile proves the fragility
// handling mandated by the bead brief: deprecated_commands.go's own
// header says a follow-up deletes the whole file one release after
// spec 084. Copy cmd/mindspec to a temp dir with that one file
// physically removed and confirm buildCmdTree still succeeds (no
// crash, no error) and that the six retired verbs are simply absent —
// R1 then catches any doc mention of them as a plain unresolved verb,
// with no special-casing anywhere in this lint.
func TestCmdTreeDegradesWithoutDeprecatedFile(t *testing.T) {
	srcDir := realCmdDir(t)
	dstDir := t.TempDir()
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read %s: %v", srcDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "deprecated_commands.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, e.Name()), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}

	root, err := buildCmdTree(dstDir)
	if err != nil {
		t.Fatalf("buildCmdTree without deprecated_commands.go: %v", err)
	}

	for _, verb := range []string{"agentmind", "viz", "bench"} {
		res := root.resolve([]string{"mindspec", verb})
		if res.Resolved {
			t.Errorf("expected %q to be absent once deprecated_commands.go is gone, but it resolved", verb)
		}
		if containsSub(res.Reason, "stub") {
			t.Errorf("expected a plain %q not-found reason once the file is gone, got a stub-classification reason: %s", verb, res.Reason)
		}
	}
}
