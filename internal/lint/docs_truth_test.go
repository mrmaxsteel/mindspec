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
//	    universe (docs_truth_skills_test.go) or carries the marker.
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
// deliberately narrow, audited answer: exactly these five sites, each
// justified, is-matched, and re-verified every run — the same
// ratchet idiom ratchet_argv_table_test.go already uses in this
// package for its own audited-but-unenforceable call sites. Anything
// NOT on this list still fails. This is not a loosening of R1/R2/R3;
// it is the visible, auditable form of the exemption the bead brief
// says already exists by (undocumented, unenforced) convention.
//
// A second, unrelated finding is on the list for a different reason:
// `/ms-explore` (project-docs/user/CLAUDE.md, guides/claude-code.md)
// has no install code path in internal/setup or plugins/mindspec at
// all — see docs_truth_skills_test.go's doc comment. It is a genuine
// gap, not a convention; it is listed here only because fixing it is a
// product decision (ship ms-explore for real, or retract the doc
// claim) outside this bead's scope, and the alternative — silently
// dropping it — would hide a real finding. See the bead report.
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
	return fmt.Sprintf("%s:%d: %s — %s", f.File, f.Line, f.Text, f.Why)
}

// checkR1 walks every extracted `mindspec ...` invocation in each doc
// and reports one finding per resolution failure that is not covered
// by ANY marker (marker registration itself is R3's job).
func checkR1(docs []*docFile, cmdRoot *cmdNode) []truthFinding {
	var out []truthFinding
	for _, df := range docs {
		for _, occ := range extractInvocations(df) {
			for _, words := range expandAlternatives(tokenizeInvocation(occ.Text)) {
				res := cmdRoot.resolve(words)
				if res.Resolved {
					continue
				}
				if covered, _ := df.coveredBy(occ.Line, ""); covered {
					continue
				}
				out = append(out, truthFinding{File: df.Path, Line: occ.Line, Text: quoteWords(words), Why: res.Reason})
			}
		}
	}
	return out
}

// checkR2 is R1's analogue for `/ms-*` skill/workflow references.
func checkR2(docs []*docFile, skills skillUniverse) []truthFinding {
	var out []truthFinding
	for _, df := range docs {
		for _, sref := range extractSkillRefs(df) {
			if _, ok := skills[sref.Name]; ok {
				continue
			}
			if covered, _ := df.coveredBy(sref.Line, ""); covered {
				continue
			}
			out = append(out, truthFinding{File: df.Path, Line: sref.Line, Text: "/" + sref.Name, Why: "no such skill or workflow"})
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
type docsTruthException struct {
	File   string
	Line   int
	Reason string
}

var knownDocsTruthExceptions = []docsTruthException{
	{File: "project-docs/user/README.md", Line: 112, Reason: "truthful retrospective prose: \"(The old `mindspec agentmind|viz|bench` verbs moved to ... hidden deprecation stubs ...)\" — inert-referent-described-as-live class, extended to removed (not just absent) referents; see package doc comment"},
	{File: "project-docs/user/guides/agentmind.md", Line: 7, Reason: "truthful retrospective prose: \"If you remember the old verbs: ... were removed by spec 084 ... hidden deprecation stubs\" — same class as above"},
	{File: "project-docs/user/guides/agentmind.md", Line: 52, Reason: "truthful retrospective prose: \"(formerly `mindspec bench`) moved to the agentmind repo ... deprecation stubs\" — same class as above"},
	{File: "project-docs/user/CLAUDE.md", Line: 20, Reason: "genuine gap, not convention: /ms-explore has no install code path in internal/setup or plugins/mindspec (see docs_truth_skills_test.go) — flagged here as a tracked finding pending a product decision, not fixed by this bead"},
	{File: "project-docs/user/guides/claude-code.md", Line: 94, Reason: "same /ms-explore gap as above"},
}

func matchesException(f truthFinding) bool {
	for i := range knownDocsTruthExceptions {
		e := &knownDocsTruthExceptions[i]
		if e.File == f.File && e.Line == f.Line {
			return true
		}
	}
	return false
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
	findings = append(findings, checkR1(docs, cmdRoot)...)
	findings = append(findings, checkR2(docs, skills)...)
	findings = append(findings, checkR3(docs, reg)...)

	matched := make([]bool, len(knownDocsTruthExceptions))
	var unexpected []truthFinding
	for _, f := range findings {
		hit := false
		for i := range knownDocsTruthExceptions {
			e := &knownDocsTruthExceptions[i]
			if e.File == f.File && e.Line == f.Line {
				matched[i] = true
				hit = true
			}
		}
		if !hit {
			unexpected = append(unexpected, f)
		}
	}

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

// --- R4: negative fixtures, one per required class -------------------

// TestR4AbsentVerbFlag: an absent flag on a real verb, unmarked, must
// fire R1. (`mindspec onboard` does not exist as a verb at all in the
// real tree — Bead 1 retracted it — so this fixture exercises the
// "verb resolves, flag does not" arm specifically by using a real verb
// with a real flag name deliberately misspelled/absent: `doctor` has
// `--fix` but not `--infer`.)
func TestR4AbsentVerbFlag(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run `mindspec doctor --infer` to auto-detect the mode.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t))
	requireFindingContains(t, findings, "doctor --infer", "flag --infer not registered")
}

// TestR4AbsentSubcommand: an absent subcommand, unmarked, must fire
// R1. Mirrors the real `mindspec loop status` site (README.md:145) but
// WITHOUT the marker that makes the real site pass — proving the
// marker, not some blanket leniency, is what saves the real one.
func TestR4AbsentSubcommand(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Poll progress with `mindspec loop status`.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t))
	requireFindingContains(t, findings, "loop status", `no subcommand "loop"`)
}

// TestR4AbsentSubcommand_MarkedVersionPasses proves the marker escape
// valve actually works (not just that its absence fails): the same
// invocation, now carrying a valid registered-id marker, produces zero
// R1 findings.
func TestR4AbsentSubcommand_MarkedVersionPasses(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Poll progress with `mindspec loop status` *(planned — claim `loop-status`, roadmap Core 4)*.\n")
	findings := checkR1([]*docFile{df}, sharedCmdRoot(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero R1 findings on a marked absent-subcommand claim, got: %v", findings)
	}
}

// TestR4AbsentSkill: an absent skill, unmarked, must fire R2.
func TestR4AbsentSkill(t *testing.T) {
	df := fixtureDoc(t, "fixture.md", "Run the fix lane with /ms-fix-cycle.\n")
	findings := checkR2([]*docFile{df}, sharedSkills(t))
	requireFindingContains(t, findings, "/ms-fix-cycle", "no such skill or workflow")
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

// TestR5BenchStubDoesNotResolve is spec-init's negative twin: `bench`
// is also Hidden with DisableFlagParsing, but its Run field (not RunE)
// makes it a one-shot deprecation stub — resolving it must fail, even
// though a naive "is it in the cobra tree" check would pass it (the
// exact naive-check failure mode the brief names for `bench report`).
func TestR5BenchStubDoesNotResolve(t *testing.T) {
	res := sharedCmdRoot(t).resolve([]string{"mindspec", "bench", "report", "--spec", "x"})
	if res.Resolved {
		t.Fatal("expected mindspec bench report --spec x to NOT resolve (deprecation stub)")
	}
	if !containsSub(res.Reason, "deprecation stub") {
		t.Fatalf("expected reason to cite the deprecation-stub signal, got: %s", res.Reason)
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
