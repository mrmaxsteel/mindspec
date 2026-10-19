package readiness

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// gitPorcelain returns `git status --porcelain` output for root — the
// no-mutation audit primitive (spec 124 AC-2).
func gitPorcelain(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain: %v: %s", err, out)
	}
	return string(out)
}

func gitBranches(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "branch", "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list: %v: %s", err, out)
	}
	return string(out)
}

func TestPlanMapsBead_AndPositional(t *testing.T) {
	var plan strings.Builder
	plan.WriteString("---\nwork_chunks:\n")
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 10} {
		fmt.Fprintf(&plan, "  - id: %d\n    key_file_paths: [real.go]\n", n)
	}
	plan.WriteString("---\n")
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 12} {
		fmt.Fprintf(&plan, "## Bead %d: heading\n**Acceptance Criteria**\n- concrete outcome\n", n)
	}
	base := plan.String()
	for _, tc := range []struct {
		n      int
		mapped bool
		why    string
	}{
		{1, true, ""}, {8, true, ""}, {9, false, "plan.md declares 8 ## Bead sections"},
		{10, false, "plan.md declares 8 ## Bead sections"}, {12, false, "plan.md declares 8 ## Bead sections"},
		{0, false, "plan.md declares 8 ## Bead sections"}, {-1, false, "plan.md declares 8 ## Bead sections"},
	} {
		mapped, why := PlanMapsBead(base, tc.n)
		if mapped != tc.mapped || why != tc.why {
			t.Errorf("N=%d: (%v,%q), want (%v,%q)", tc.n, mapped, why, tc.mapped, tc.why)
		}
	}
	section, found := extractBeadSectionRaw(base, 8)
	if !found || !strings.HasPrefix(section, "## Bead 12:") {
		t.Fatalf("eighth section=%q found=%v", section, found)
	}
	if s := evaluateMF1(base, 8); !s.Pass {
		t.Fatalf("MF-1 must share positional mapping: %+v", s)
	}
	fenced := base + "```markdown\n## Bead 99: fenced heading\n```\n"
	if _, found := extractBeadSectionRaw(fenced, 9); !found {
		t.Fatal("extractor did not count fenced heading")
	}
	if mapped, why := PlanMapsBead(fenced, 9); mapped || why != "no work_chunks id 9" {
		t.Fatalf("fenced N=9: (%v,%q)", mapped, why)
	}
	withChunk9 := strings.Replace(fenced, "work_chunks:\n", "work_chunks:\n  - id: 9\n", 1)
	if mapped, why := PlanMapsBead(withChunk9, 9); !mapped || why != "" {
		t.Fatalf("fenced mapped N=9: (%v,%q)", mapped, why)
	}
	if mapped, why := PlanMapsBead(fenced, 10); mapped || why != "plan.md declares 9 ## Bead sections" {
		t.Fatalf("chunk-only N=10: (%v,%q)", mapped, why)
	}
	sectionOnly := strings.Replace(base, "  - id: 8\n    key_file_paths: [real.go]\n", "", 1)
	if mapped, why := PlanMapsBead(sectionOnly, 8); mapped || why != "no work_chunks id 8" {
		t.Fatalf("section-only N=8: (%v,%q)", mapped, why)
	}
	if s := evaluateMF1(sectionOnly, 8); !reflect.DeepEqual(s.Failures, []string{"chunk-missing:8"}) {
		t.Fatalf("half-mapping MF-1=%+v", s)
	}
	if mapped, why := PlanMapsBead("---\nwork_chunks: [\n---\n## Bead 1: malformed\n", 1); mapped || why != "invalid plan.md frontmatter" {
		t.Fatalf("malformed mapping=(%v,%q)", mapped, why)
	}
}

func TestReadiness_MF1AggregatesAllFailures(t *testing.T) {
	plan := "---\nwork_chunks:\n  - id: 1\n    key_file_paths: [path/to/file.go, ' ', path/to/file.go]\n---\n## Bead 1: incomplete\n**Acceptance Criteria**\n- <Specific, measurable criterion for this bead>\n"
	s := evaluateMF1(plan, 1)
	want := []string{"ac-entry-missing:1", "key-file-paths-blank:1", "key-file-paths-placeholder:1"}
	if s.Pass || !reflect.DeepEqual(s.Failures, want) {
		t.Fatalf("MF-1 failures=%v Pass=%v, want %v", s.Failures, s.Pass, want)
	}
	for _, fragment := range []string{"Acceptance Criteria block has no entry", "scaffold placeholder", "no non-blank concrete path"} {
		if !strings.Contains(s.Detail, fragment) {
			t.Errorf("detail %q missing %q", s.Detail, fragment)
		}
	}
	if strings.Contains(s.Recovery, "\n") || !strings.Contains(s.Recovery, "replace the placeholder with a concrete, measurable criterion") {
		t.Fatalf("one first recovery expected: %q", s.Recovery)
	}
	for _, tc := range []struct {
		plan    string
		records []string
	}{
		{"---\nwork_chunks: []\n---\n", []string{"chunk-missing:1", "section-missing:1"}},
		{"---\nwork_chunks:\n  - id: 1\n    key_file_paths: []\n---\n## Bead 1: empty\n", []string{"ac-entry-missing:1", "key-file-paths-empty:1"}},
	} {
		got := evaluateMF1(tc.plan, 1)
		if got.Pass || !reflect.DeepEqual(got.Failures, tc.records) {
			t.Errorf("aggregate=%+v, want %v", got, tc.records)
		}
	}
	for _, tc := range []struct {
		name, paths string
		pass        bool
	}{
		{"concrete-with-blank", "[real.go, '']", true},
		{"all-blank", "['', ' ']", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := "---\nwork_chunks:\n  - id: 1\n    key_file_paths: " + tc.paths + "\n---\n## Bead 1: paths\n**Acceptance Criteria**\n- A concrete outcome is verified.\n"
			got := evaluateMF1(plan, 1)
			if got.Pass != tc.pass {
				t.Fatalf("paths %s: Pass=%v, want %v; failures=%v detail=%q", tc.paths, got.Pass, tc.pass, got.Failures, got.Detail)
			}
			if tc.pass {
				if len(got.Failures) != 0 || got.Detail != "" || got.Recovery != "" {
					t.Fatalf("concrete path must pass without failures: %+v", got)
				}
			} else if !reflect.DeepEqual(got.Failures, []string{"key-file-paths-blank:1"}) || got.Detail != "work_chunks[1].key_file_paths has no non-blank concrete path element" {
				t.Fatalf("all-blank paths must retain FX-3 record and detail: %+v", got)
			}
		})
	}
}

func TestReadiness_MF4AggregatesAllOccurrences(t *testing.T) {
	description := "TBD\nTBD\nOPEN QUESTION\n"
	plan := "## Bead 1: blockers\n### First\nTBD\nTBD\n**Blocking Questions**\n- [ ] decide\n- [ ] decide\n### Second\nTBD\n```\nTBD\n**Blocking Questions**\n- [ ] ignored\n```\n`TBD`\n"
	s := evaluateMF4(description, plan)
	var want []string
	add := func(kind, source, section, line string, k int) {
		want = append(want, fmt.Sprintf("%s:%s:%s:%d:%x", kind, source, section, k, sha256.Sum256([]byte(line))))
	}
	for _, k := range []int{1, 2} {
		add("token", "description", "-", "TBD", k)
		add("token", "plan", "## Bead 1: blockers / ### First", "TBD", k)
		add("checklist", "plan", "## Bead 1: blockers / ### First", "- [ ] decide", k)
	}
	add("token", "description", "-", "OPEN QUESTION", 1)
	add("token", "plan", "## Bead 1: blockers / ### Second", "TBD", 1)
	want = sortedUnique(want)
	if s.Pass || !reflect.DeepEqual(s.Failures, want) {
		t.Fatalf("MF-4 failures=%v, want %v", s.Failures, want)
	}
	if strings.Count(s.Detail, "unchecked item under a blocking-region header:") != 2 || !strings.Contains(s.Detail, "OPEN QUESTION") {
		t.Fatalf("detail omitted occurrences: %q", s.Detail)
	}
	if s.Recovery == "" || strings.Contains(s.Recovery, "\n") {
		t.Fatalf("expected one recovery: %q", s.Recovery)
	}
	if got := evaluateMF4("TBD TBD", ""); len(got.Failures) != 2 {
		t.Fatalf("same-line multiplicity lost: %v", got.Failures)
	}
	t.Run("description-checklist-passes", func(t *testing.T) {
		if got := evaluateMF4("**Open Questions**\n- [ ] decide\n", ""); !got.Pass || len(got.Failures) != 0 {
			t.Fatalf("description-only checklist must pass MF-4: %+v", got)
		}
	})
	t.Run("region-termination", func(t *testing.T) {
		for _, heading := range []string{"#### Sub", "# Top", "##### Sub", "###### Sub"} {
			t.Run(heading, func(t *testing.T) {
				got := evaluateMF4("", "## Bead 1: blockers\n**Blocking Questions**\n"+heading+"\n- [ ] decide\n")
				if got.Pass || len(got.Failures) != 1 || !strings.HasPrefix(got.Failures[0], "checklist:plan:") {
					t.Fatalf("heading %q must keep blocking region open: %+v", heading, got)
				}
			})
		}
		for _, heading := range []string{"## End", "### End", "  ## End", "\t### End", "**Verification**"} {
			t.Run(heading, func(t *testing.T) {
				got := evaluateMF4("", "**Blocking Questions**\n"+heading+"\n- [ ] routine\n")
				if !got.Pass || len(got.Failures) != 0 {
					t.Fatalf("trimmed heading %q must end blocking region: %+v", heading, got)
				}
			})
		}
	})
	t.Run("line-normalisation", func(t *testing.T) {
		for _, line := range []string{"  TBD  decide  ", "\tTBD\tdecide\t"} {
			got := evaluateMF4(line, "")
			want := []string{fmt.Sprintf("token:description:-:1:%x", sha256.Sum256([]byte("TBD decide")))}
			if got.Pass || !reflect.DeepEqual(got.Failures, want) {
				t.Errorf("normalised occurrence for %q=%v, want %v", line, got.Failures, want)
			}
		}
	})
	t.Run("description-heading", func(t *testing.T) {
		got := evaluateMF4("## heading\nTBD\n", "")
		want := []string{fmt.Sprintf("token:description:-:1:%x", sha256.Sum256([]byte("TBD")))}
		if got.Pass || !reflect.DeepEqual(got.Failures, want) {
			t.Fatalf("description heading must retain section '-': %v, want %v", got.Failures, want)
		}
	})
	if got := evaluateMF4("TBD", "## Bead 1: blockers\nTBD\n"); len(got.Failures) != 2 {
		t.Fatalf("source identity lost: %v", got.Failures)
	}
}

func TestEvaluateReadiness_NonPlanFormErrorCarriesLineage(t *testing.T) {
	store := NewFakeBDStore()
	cases := []struct{ id, message string }{
		{"mindspec-plain", "bead mindspec-plain has no dotted epic-child index (expected <epic>.<N>)"},
		{"mindspec-plain.0", "bead mindspec-plain.0 has an invalid epic-child index"},
		{"mindspec-plain.99999999999999999999999999", "bead mindspec-plain.99999999999999999999999999 has an invalid epic-child index"},
	}
	for _, tc := range cases {
		store.Lineage[tc.id] = FakeLineage{EpicID: "mindspec-owner", SpecID: "129-owner"}
	}
	t.Cleanup(store.Install())
	for _, tc := range cases {
		id := tc.id
		report, err := EvaluateReadiness(t.TempDir(), id) // No plan or bd record exists.
		var typed *NonPlanFormError
		if report != nil || !errors.As(err, &typed) {
			t.Fatalf("%s: report=%v err=%T %v, want NonPlanFormError before file reads", id, report, err, err)
		}
		if typed.BeadID != id || typed.EpicID != "mindspec-owner" || typed.SpecID != "129-owner" {
			t.Fatalf("lineage lost: %+v", typed)
		}
		if typed.Error() != tc.message {
			t.Fatalf("diagnostic changed: %q, want literal %q", typed.Error(), tc.message)
		}
	}
}

// --- AC-1: negative fixture refused with evidence ---

func TestReadiness_NegativeFixtureAllFail(t *testing.T) {
	root := t.TempDir()
	fx, err := BuildNegativeFixture(root)
	if err != nil {
		t.Fatalf("BuildNegativeFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	report, err := EvaluateReadiness(root, fx.BeadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness: %v", err)
	}
	if report.AllPass() {
		t.Fatalf("negative fixture unexpectedly passed: %s", Render(report))
	}
	if len(report.Signals) != 4 {
		t.Fatalf("expected 4 signals, got %d", len(report.Signals))
	}

	want := map[string]string{
		SignalPlanSection: "placeholder",
		SignalTokens:      "AC-99",
		// The NotFound-arm detail states the two possibilities
		// conservatively (final-review r2 G2-R2-MF3-ANONYMOUS): no landed
		// merge NAMES the dep's bead branch — never merged, or anonymous
		// merge subject.
		SignalDependencies: "no landed merge",
		SignalBlocking:     "blocking",
	}
	for _, s := range report.Signals {
		if s.Pass {
			t.Errorf("signal %s unexpectedly PASSed", s.ID)
			continue
		}
		if s.Recovery == "" {
			t.Errorf("signal %s FAILed with no recovery lever", s.ID)
		}
		frag := want[s.ID]
		if frag != "" && !strings.Contains(strings.ToLower(s.Detail), strings.ToLower(frag)) {
			t.Errorf("signal %s detail %q does not name its planted defect (want substring %q)", s.ID, s.Detail, frag)
		}
	}
	// Both MF-2 classes are planted: AC-99 (wholly dangling) AND the
	// prefix-dangling AC-1 (spec.md carries only AC-19).
	for _, s := range report.Signals {
		if s.ID == SignalTokens {
			if !strings.Contains(s.Detail, "AC-99") {
				t.Errorf("MF-2 detail missing AC-99: %q", s.Detail)
			}
			if !strings.Contains(s.Detail, "AC-1") || strings.Contains(s.Detail, "AC-19") {
				// AC-1 must be named; AC-19 (the spec's real token) must
				// NOT appear as if it were itself dangling.
				if !strings.Contains(s.Detail, "AC-1,") && !strings.Contains(s.Detail, "AC-1 ") && !strings.HasSuffix(s.Detail, "AC-1") {
					t.Errorf("MF-2 detail missing the prefix-dangling AC-1 claim: %q", s.Detail)
				}
			}
		}
	}
}

// --- AC-2: positive fixture passes, read-only, idempotent ---

func TestReadiness_PositiveFixtureAllPass(t *testing.T) {
	root := t.TempDir()
	fx, err := BuildPositiveFixture(root)
	if err != nil {
		t.Fatalf("BuildPositiveFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	statusBefore := gitPorcelain(t, root)
	branchesBefore := gitBranches(t, root)

	report1, err := EvaluateReadiness(root, fx.BeadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness (1st): %v", err)
	}
	if !report1.AllPass() {
		t.Fatalf("positive fixture unexpectedly FAILed: %s", Render(report1))
	}

	report2, err := EvaluateReadiness(root, fx.BeadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness (2nd): %v", err)
	}
	if Render(report1) != Render(report2) {
		t.Errorf("EvaluateReadiness is not idempotent:\n1st: %s\n2nd: %s", Render(report1), Render(report2))
	}

	statusAfter := gitPorcelain(t, root)
	branchesAfter := gitBranches(t, root)
	if statusBefore != statusAfter {
		t.Errorf("git status --porcelain changed: before=%q after=%q", statusBefore, statusAfter)
	}
	if branchesBefore != branchesAfter {
		t.Errorf("git branch --list changed: before=%q after=%q", branchesBefore, branchesAfter)
	}
}

// TestReadiness_PositiveFixtureShapeTrapdoor pins the AC-2 sanitized-
// fixture trapdoor: stripping a benign feature that is actually
// load-bearing (not decorative) flips the fixture's own bead from PASS to
// FAIL, proving the feature was doing real work.
func TestReadiness_PositiveFixtureShapeTrapdoor(t *testing.T) {
	t.Run("strip sub-lettered spec token (benign feature iii)", func(t *testing.T) {
		root := t.TempDir()
		sanitizedSpecMD := strings.ReplaceAll(positiveSpecMD, "R5a", "unrelated-noise")
		runSanitizedCase(t, root, "sani-iii", sanitizedSpecMD, positivePlanMD, false)
	})
	t.Run("strip foreign-citation qualifier (benign feature iv)", func(t *testing.T) {
		root := t.TempDir()
		sanitizedPlanMD := strings.ReplaceAll(positivePlanMD, "the spec 123 AC-17\npattern", "the AC-17 pattern")
		sanitizedPlanMD = strings.ReplaceAll(sanitizedPlanMD, "the spec 123 AC-17 pattern", "the AC-17 pattern")
		if sanitizedPlanMD == positivePlanMD {
			t.Fatal("sanitization did not change plan content — fixture text drifted from this test's expectation")
		}
		runSanitizedCase(t, root, "sani-iv", positiveSpecMD, sanitizedPlanMD, false)
	})
}

func runSanitizedCase(t *testing.T, root, tag, specMD, planMD string, wantAllPass bool) {
	t.Helper()
	specID := "998-fixture-" + tag
	beadID := "mindspec-" + tag + ".1"
	epicID := "mindspec-" + tag
	specMD = strings.ReplaceAll(specMD, positiveSpecID, specID)
	planMD = strings.ReplaceAll(planMD, positiveSpecID, specID)

	if err := writeWorkspace(root, specID, specMD, planMD); err != nil {
		t.Fatalf("writeWorkspace: %v", err)
	}
	store := NewFakeBDStore()
	store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = FakeBeadRecord{Description: ""}
	restore := store.Install()
	t.Cleanup(restore)

	report, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness: %v", err)
	}
	if report.AllPass() != wantAllPass {
		t.Errorf("sanitized fixture AllPass()=%v, want %v: %s", report.AllPass(), wantAllPass, Render(report))
	}
}

// --- AC-3: MF-3 landed-merge predicate, all four variants ---

func TestReadiness_MF3Variants(t *testing.T) {
	const specID = "996-fixture-mf3"
	const epicID = "mindspec-mf3b"

	baseSpecMD := "# Spec 996-fixture-mf3\n\n## Acceptance Criteria\n\n- [ ] AC-1 — a criterion.\n"
	basePlanMD := mf3PlanMD(specID)

	cases := []struct {
		name     string
		tag      string
		setup    func(t *testing.T, root, specBranch, depID string)
		depStat  string
		wantPass bool
	}{
		{
			name:     "i open dependency",
			tag:      "open",
			setup:    func(t *testing.T, root, specBranch, depID string) {},
			depStat:  "open",
			wantPass: false,
		},
		{
			name: "ii closed, branch present, not landed-merged (2u0u split)",
			tag:  "unmerged",
			setup: func(t *testing.T, root, specBranch, depID string) {
				if err := createUnmergedDependencyBranch(root, specBranch, depID); err != nil {
					t.Fatalf("createUnmergedDependencyBranch: %v", err)
				}
			},
			depStat:  "closed",
			wantPass: false,
		},
		{
			name: "iii closed, landed-merged, branch DELETED",
			tag:  "deleted",
			setup: func(t *testing.T, root, specBranch, depID string) {
				if err := mergeDependency(root, specBranch, depID, true); err != nil {
					t.Fatalf("mergeDependency: %v", err)
				}
			},
			depStat:  "closed",
			wantPass: true,
		},
		{
			name: "iv closed, landed-merged, branch present",
			tag:  "present",
			setup: func(t *testing.T, root, specBranch, depID string) {
				if err := mergeDependency(root, specBranch, depID, false); err != nil {
					t.Fatalf("mergeDependency: %v", err)
				}
			},
			depStat:  "closed",
			wantPass: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			beadID := "mindspec-mf3b." + "1"
			depID := "mindspec-mf3dep-" + tc.tag + ".1"
			// Each subtest needs its own bead ID to avoid FakeBDStore
			// collisions across subtests sharing the seam; the plan
			// section/spec.md content is identical across variants
			// (only the dependency's git/bd state differs), so reuse one
			// specID per subtest by suffixing it.
			specIDCase := specID + "-" + tc.tag
			specMD := strings.ReplaceAll(baseSpecMD, specID, specIDCase)
			planMD := strings.ReplaceAll(basePlanMD, specID, specIDCase)
			if err := writeWorkspace(root, specIDCase, specMD, planMD); err != nil {
				t.Fatalf("writeWorkspace: %v", err)
			}
			specBranch, err := initGitRepo(root, specIDCase)
			if err != nil {
				t.Fatalf("initGitRepo: %v", err)
			}
			tc.setup(t, root, specBranch, depID)

			store := NewFakeBDStore()
			store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specIDCase}
			store.Records[beadID] = FakeBeadRecord{
				Description:  "MF-3 variant fixture.",
				Dependencies: []FakeDependency{{ID: depID, Status: tc.depStat}},
			}
			restore := store.Install()
			t.Cleanup(restore)

			report, err := EvaluateReadiness(root, beadID)
			if err != nil {
				t.Fatalf("EvaluateReadiness: %v", err)
			}
			var mf3 Signal
			for _, s := range report.Signals {
				if s.ID == SignalDependencies {
					mf3 = s
				}
			}
			if mf3.Pass != tc.wantPass {
				t.Errorf("MF-3 Pass=%v, want %v: detail=%q", mf3.Pass, tc.wantPass, mf3.Detail)
			}
		})
	}
}

func mf3PlanMD(specID string) string {
	return fmt.Sprintf(`---
status: Approved
spec_id: %s
version: "1"
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/fixture/mf3.go
---
# Plan: %s

## ADR Fitness

No ADRs relevant.

## Testing Strategy

Unit tests.

## Bead 1: MF-3 variant

**Steps**
1. Step one

**Acceptance Criteria**
- [ ] Provide a concrete, testable outcome for this fixture case.
`, specID, specID)
}

// --- AC-14: MF-2 exact resolution, all classes + the enumerator arm ---

func TestReadiness_MF2AllClasses(t *testing.T) {
	cases := []struct {
		name      string
		claimText string
		specBody  string
		wantPass  bool
	}{
		{"i letterless claim resolves via sub-lettered spec token", "This bead claims R5 in full.", "- [ ] R5a — some criterion.", true},
		{"ii sub-lettered claim fails against bare spec token", "This bead claims R5a specifically.", "- [ ] R5 — some criterion.", false},
		{"ii sub-lettered claim fails against sibling spec token", "This bead claims R5a specifically.", "- [ ] R5b — some criterion.", false},
		{"ii sub-lettered claim passes exact match", "This bead claims R5a specifically.", "- [ ] R5a — some criterion.", true},
		{"iii numeric-prefix never resolves", "This bead claims AC-1.", "- [ ] AC-19 — some criterion.", false},
		{"iii wholly dangling token fails", "This bead claims AC-99.", "- [ ] AC-1 — some criterion.", false},
		{"iv foreign citation excluded from harvest", "This bead follows the spec 123 AC-17 pattern.", "- [ ] AC-1 — some criterion.", true},
		{"iv control: bare AC-17 without citation dangles", "This bead follows the AC-17 pattern.", "- [ ] AC-1 — some criterion.", false},
		// Final-review r1 G1-MF2-MIXED-CITATION-LINE: the exclusion is
		// SPAN-scoped, never whole-line — a dangling OWNING-spec token
		// sharing a line with a benign foreign citation must still be
		// harvested and FAIL. Reverting to the whole-line exclusion
		// turns the first case green-to-red here (RED-on-revert).
		{"iv mixed line: dangling owning token beside a foreign citation FAILs", "Preserve the spec 123 AC-17 pattern while satisfying AC-999.", "- [ ] AC-1 — some criterion.", false},
		{"iv mixed line: resolvable owning token beside a foreign citation passes", "Preserve the spec 123 AC-17 pattern while satisfying AC-1.", "- [ ] AC-1 — some criterion.", true},
		{"iv citation token chain excluded (slash-separated)", "This bead follows the spec 123 AC-17/AC-18 conventions.", "- [ ] AC-1 — some criterion.", true},
		{"enumerator basic: AC-9(i) resolves via bare AC-9", "This bead claims AC-9(i).", "- [ ] AC-9 — some criterion.", true},
		{"enumerator collision: AC-9(i) still resolves via base, even though AC-9i also exists", "This bead claims AC-9(i).", "- [ ] AC-9i — some criterion.", true},
		{"direct sub-letter (no parens) AC-9i dangles against bare AC-9 only", "This bead claims AC-9i directly.", "- [ ] AC-9 — some criterion.", false},
		{"sub-letter parenthetical R5(b) resolves exact", "This bead claims R5(b).", "- [ ] R5b — some criterion.", true},
		{"sub-letter parenthetical R5(b) fails against bare spec token (lenient-impl trap)", "This bead claims R5(b).", "- [ ] R5 — some criterion.", false},
		{"R5.2 leg 1 parenthetical matches parenthetical", "R9(a)", "R9(a)", true},
		{"R5.2 leg 2 parenthetical matches appended", "R3(c)", "R3c", true},
		{"R5.2 leg 3 hyphenless claim resolves", "AC9", "AC-9", true},
		{"R5.2 leg 4 hyphenless spec resolves", "AC-9", "AC9", true},
		{"R5.2 leg 5 sibling stays dangling", "R9(b)", "R9(a)", false},
		{"R5.2 leg 6 hyphenless claim dangles", "AC9", "AC-8", false},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tag := fmt.Sprintf("mf2case%d", i)
			root := t.TempDir()
			specID := "995-fixture-" + tag
			beadID := "mindspec-" + tag + ".1"
			epicID := "mindspec-" + tag

			specMD := fmt.Sprintf("# Spec %s\n\n## Acceptance Criteria\n\n%s\n", specID, tc.specBody)
			planMD := fmt.Sprintf(`---
status: Approved
spec_id: %s
version: "1"
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/fixture/%s.go
---
# Plan: %s

## ADR Fitness

No ADRs relevant.

## Testing Strategy

Unit tests.

## Bead 1: MF-2 case

**Steps**
1. Step one

**Acceptance Criteria**
- [ ] Provide a concrete, testable outcome for this fixture case.

%s
`, specID, tag, specID, tc.claimText)

			if err := writeWorkspace(root, specID, specMD, planMD); err != nil {
				t.Fatalf("writeWorkspace: %v", err)
			}
			store := NewFakeBDStore()
			store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
			store.Records[beadID] = FakeBeadRecord{Description: ""}
			restore := store.Install()
			t.Cleanup(restore)

			report, err := EvaluateReadiness(root, beadID)
			if err != nil {
				t.Fatalf("EvaluateReadiness: %v", err)
			}
			var mf2 Signal
			for _, s := range report.Signals {
				if s.ID == SignalTokens {
					mf2 = s
				}
			}
			if mf2.Pass != tc.wantPass {
				t.Errorf("MF-2 Pass=%v, want %v: detail=%q", mf2.Pass, tc.wantPass, mf2.Detail)
			}
			if strings.HasPrefix(tc.name, "R5.2") && !tc.wantPass {
				wantRecord := "dangling:R9b"
				if strings.Contains(tc.name, "leg 6") {
					wantRecord = "dangling:AC-9"
				}
				if !reflect.DeepEqual(mf2.Failures, []string{wantRecord}) || !strings.Contains(mf2.Detail, tc.claimText) {
					t.Errorf("dangling claim %q: failures=%v detail=%q", tc.claimText, mf2.Failures, mf2.Detail)
				}
			}
			if report.EpicID != epicID || report.SpecID != specID {
				t.Errorf("report lineage=(%q,%q), want (%q,%q)", report.EpicID, report.SpecID, epicID, specID)
			}
		})
	}
}

// --- AC-12: layer boundary, both directions ---

func TestReadiness_LayerBoundaryBothDirections(t *testing.T) {
	t.Run("i fail-stays-fail: attempt record addressing an MF-2 failure does not flip the verdict", func(t *testing.T) {
		root := t.TempDir()
		fx, err := BuildNegativeFixture(root)
		if err != nil {
			t.Fatalf("BuildNegativeFixture: %v", err)
		}
		restore := fx.Store.Install()
		t.Cleanup(restore)

		before, err := EvaluateReadiness(root, fx.BeadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness (before): %v", err)
		}
		if before.AllPass() {
			t.Fatalf("negative fixture unexpectedly passed before seeding")
		}

		fx.Store.SeedReadinessAttempt(fx.BeadID, bead.MetaKeyReadinessAttempt, map[string]interface{}{
			"report":         []map[string]interface{}{{"ordinal": 1, "signal": "MF-2", "reason": "AC-99 dangling"}},
			"clarifications": []map[string]interface{}{{"ordinal": 1, "answer": "AC-99 was addressed", "span": "spec.md line 5"}},
		})

		after, err := EvaluateReadiness(root, fx.BeadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness (after seeding): %v", err)
		}
		if Render(before) != Render(after) {
			t.Errorf("mechanical verdict changed after seeding an attempt record: before=%s after=%s", Render(before), Render(after))
		}
		if after.AllPass() {
			t.Fatalf("a fixture with a genuine mechanical FAIL passed after a clarification record was seeded (layer-boundary breach)")
		}
	})

	t.Run("ii pass-stays-pass: hostile-token attempt record on the positive bead does not flip the verdict", func(t *testing.T) {
		root := t.TempDir()
		fx, err := BuildPositiveFixture(root)
		if err != nil {
			t.Fatalf("BuildPositiveFixture: %v", err)
		}
		restore := fx.Store.Install()
		t.Cleanup(restore)

		before, err := EvaluateReadiness(root, fx.BeadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness (before): %v", err)
		}
		if !before.AllPass() {
			t.Fatalf("positive fixture unexpectedly failed before seeding: %s", Render(before))
		}

		fx.Store.SeedReadinessAttempt(fx.BeadID, bead.MetaKeyReadinessAttempt, map[string]interface{}{
			"report": []map[string]interface{}{{"ordinal": 1, "signal": "SR-1", "reason": "AC-7 unclear"}},
			"clarifications": []map[string]interface{}{{
				"ordinal": 1,
				"answer":  "See AC-7: TBD, and note the unchecked item below.\n- [ ] still open",
				"span":    "plan.md Bead 1",
			}},
		})

		after, err := EvaluateReadiness(root, fx.BeadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness (after seeding): %v", err)
		}
		if Render(before) != Render(after) {
			t.Errorf("mechanical verdict changed after seeding a hostile-token attempt record: before=%s after=%s", Render(before), Render(after))
		}
		if !after.AllPass() {
			t.Fatalf("positive fixture FAILed after a hostile-token clarification record was seeded (layer-boundary breach): %s", Render(after))
		}
	})
}

// --- bd-less hermeticity: engine tests run green with `bd` OFF PATH ---

func TestReadiness_HermeticWithoutRealBD(t *testing.T) {
	// Every test in this file installs a FakeBDStore, so none of them
	// ever spawns a real `bd` process. This test makes that guarantee
	// explicit and mechanically checked (spec 124 plan-gate F3-1): strip
	// `bd` from PATH entirely and re-run the positive-fixture happy path,
	// proving EvaluateReadiness never falls through to fetchBeadRecordReal
	// or a real phase.FindEpicForBead call.
	// Fixture construction needs real `git` (for MF-3's real temp repos);
	// only `bd` needs to be unreachable. Symlink the real git binary into
	// an otherwise-empty PATH directory (the internal/harness
	// hideRealGH-style precedent) so `bd` resolves nowhere while `git`
	// still works.
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locating real git: %v", err)
	}
	binDir := t.TempDir()
	if err := os.Symlink(realGit, filepath.Join(binDir, "git")); err != nil {
		t.Fatalf("symlinking git: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	if _, err := exec.LookPath("bd"); err == nil {
		t.Fatal("test setup failed: `bd` is still resolvable on the stripped PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("test setup failed: `git` is not resolvable on the stripped PATH: %v", err)
	}

	root := t.TempDir()
	fx, err := BuildPositiveFixture(root)
	if err != nil {
		t.Fatalf("BuildPositiveFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	report, err := EvaluateReadiness(root, fx.BeadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness with bd off PATH: %v", err)
	}
	if !report.AllPass() {
		t.Fatalf("positive fixture failed with bd off PATH: %s", Render(report))
	}
}

// sanity: fixture content actually contains the four benign features this
// test file's trapdoor subtests rely on stripping — guards against the
// fixture text silently drifting out from under those subtests.
func TestPositiveFixtureContent_CarriesBenignFeatures(t *testing.T) {
	checks := []struct {
		name string
		text string
	}{
		{"scaffold Verification checklist", "**Verification**"},
		{"checkbox AC entry", "- [ ] AC-7"},
		{"sub-lettered spec token", "R5a"},
		{"foreign citation", "spec 123 AC-17"},
		{"code-quoted TBD", "`TBD`"},
		{"code-quoted OPEN QUESTION", "`OPEN QUESTION`"},
		{"enumerator-parenthetical claim", "AC-9(i)"},
	}
	for _, c := range checks {
		if !strings.Contains(positivePlanMD, c.text) && !strings.Contains(positiveSpecMD, c.text) {
			t.Errorf("positive fixture missing benign feature %q (%s)", c.text, c.name)
		}
	}
}

// writeSimpleWorkspace writes a minimal spec.md/plan.md pair for a
// single-bead spec under root, with the given work_chunks[0].key_file_paths
// YAML block, spec.md acceptance-criteria body, and bead-section trailer
// prose (the MF-2/MF-4 harvest surface). It returns the specID/beadID/epicID.
func writeSimpleWorkspace(t *testing.T, root, tag, keyFilePathsYAML, specACBody, beadTrailer string) (specID, beadID, epicID string) {
	t.Helper()
	specID = "990-fixture-" + tag
	beadID = "mindspec-" + tag + ".1"
	epicID = "mindspec-" + tag
	specMD := fmt.Sprintf("# Spec %s\n\n## Acceptance Criteria\n\n%s\n", specID, specACBody)
	planMD := fmt.Sprintf(`---
status: Approved
spec_id: %s
version: "1"
work_chunks:
  - id: 1
    depends_on: []
%s
---
# Plan: %s

## ADR Fitness

No ADRs relevant.

## Testing Strategy

Unit tests.

## Bead 1: Fixture

**Steps**
1. Step one

**Acceptance Criteria**
- [ ] Provide a concrete, testable outcome for this fixture case.

%s
`, specID, keyFilePathsYAML, specID, beadTrailer)
	if err := writeWorkspace(root, specID, specMD, planMD); err != nil {
		t.Fatalf("writeWorkspace: %v", err)
	}
	return specID, beadID, epicID
}

func signalByID(r *Report, id string) Signal {
	for _, s := range r.Signals {
		if s.ID == id {
			return s
		}
	}
	return Signal{}
}

// TestReadiness_MF3_MetadataCorroboratedLandedPath_BdLess pins FX-1: MF-3's
// landed-merge decision is fully fakeable behind the findLandedMergeFn seam,
// so the metadata-corroborated landed path (which the real
// lifecycle.FindLandedMerge reaches via a TRANSITIVE bd read,
// landedBindingForBead -> bead.GetMetadata) is exercised with NEITHER a real
// bd NOR a real git process on PATH. Without the seam, this dependency's
// landed state could only be established by a real git repo + the transitive
// bd metadata read — the very unseamed bd read the plan's F3-1 forbids.
func TestReadiness_MF3_MetadataCorroboratedLandedPath_BdLess(t *testing.T) {
	// Strip PATH to an empty dir: no bd, no git resolvable at all. If the
	// landed-merge decision were NOT behind the seam, EvaluateReadiness
	// would have to shell out to git/bd here and fail.
	binDir := t.TempDir()
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	if _, err := exec.LookPath("bd"); err == nil {
		t.Fatal("test setup failed: bd is still resolvable on the stripped PATH")
	}
	if _, err := exec.LookPath("git"); err == nil {
		t.Fatal("test setup failed: git is still resolvable on the stripped PATH")
	}

	root := t.TempDir()
	specID, beadID, epicID := writeSimpleWorkspace(t, root, "mf3seam",
		"    key_file_paths:\n      - internal/fixture/mf3seam.go",
		"- [ ] AC-1 — a criterion.",
		"This bead claims AC-1.")
	depID := "mindspec-mf3seam.0"

	// Landed arm: the dep is corroborated as landed purely via the seam.
	store := NewFakeBDStore()
	store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = FakeBeadRecord{
		Description:  "MF-3 seam fixture.",
		Dependencies: []FakeDependency{{ID: depID, Status: "closed"}},
	}
	store.LandedDeps = map[string]bool{depID: true}
	restore := store.Install()
	t.Cleanup(restore)

	report, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness (landed arm, bd+git off PATH): %v", err)
	}
	if mf3 := signalByID(report, SignalDependencies); !mf3.Pass {
		t.Errorf("MF-3 expected PASS via the faked landed-merge seam, got FAIL: %q", mf3.Detail)
	}

	// Not-landed arm (same seam, dep unlisted): MF-3 FAILs — proving the
	// seam genuinely drives the decision, not a silent always-pass.
	store.LandedDeps = map[string]bool{}
	report2, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness (not-landed arm): %v", err)
	}
	if mf3 := signalByID(report2, SignalDependencies); mf3.Pass {
		t.Errorf("MF-3 expected FAIL when the seam reports the dep not landed, got PASS")
	}
}

// TestReadiness_FX2_MultiBacktickCodeSpanExcluded pins FX-2: a TBD marker
// and an AC-<n> claim appearing ONLY inside a DOUBLE-backtick inline code
// span are excluded from the MF-2/MF-4 scans (per CommonMark, an opening
// run of N backticks is closed by the next run of exactly N), so the bead
// PASSes. The pre-fix single-backtick regex left the double-backtick
// payload visible, producing a false MF-2 (dangling AC-99) / MF-4 (TBD)
// refusal.
func TestReadiness_FX2_MultiBacktickCodeSpanExcluded(t *testing.T) {
	root := t.TempDir()
	trailer := "See the ``TBD`` convention and the ``AC-99`` token quoted here as\n" +
		"fixture data inside DOUBLE-backtick spans — neither is a genuine marker\n" +
		"nor a real claim."
	specID, beadID, epicID := writeSimpleWorkspace(t, root, "fx2",
		"    key_file_paths:\n      - internal/fixture/fx2.go",
		"- [ ] AC-1 — a criterion.",
		trailer)

	store := NewFakeBDStore()
	store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = FakeBeadRecord{Description: ""}
	restore := store.Install()
	t.Cleanup(restore)

	report, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness: %v", err)
	}
	if mf2 := signalByID(report, SignalTokens); !mf2.Pass {
		t.Errorf("MF-2 expected PASS (AC-99 is inside a double-backtick span), got FAIL: %q", mf2.Detail)
	}
	if mf4 := signalByID(report, SignalBlocking); !mf4.Pass {
		t.Errorf("MF-4 expected PASS (TBD is inside a double-backtick span), got FAIL: %q", mf4.Detail)
	}

	// Control: the SAME tokens OUTSIDE any span DO refuse — proving the
	// pass above is exclusion, not a scan that stopped seeing tokens.
	root2 := t.TempDir()
	specID2, beadID2, epicID2 := writeSimpleWorkspace(t, root2, "fx2ctl",
		"    key_file_paths:\n      - internal/fixture/fx2ctl.go",
		"- [ ] AC-1 — a criterion.",
		"This bead claims AC-99 and notes TBD, both bare (no code span).")
	store2 := NewFakeBDStore()
	store2.Lineage[beadID2] = FakeLineage{EpicID: epicID2, SpecID: specID2}
	store2.Records[beadID2] = FakeBeadRecord{Description: ""}
	restore2 := store2.Install()
	t.Cleanup(restore2)
	report2, err := EvaluateReadiness(root2, beadID2)
	if err != nil {
		t.Fatalf("EvaluateReadiness (control): %v", err)
	}
	if signalByID(report2, SignalTokens).Pass {
		t.Error("control: MF-2 expected FAIL on a bare AC-99, got PASS")
	}
	if signalByID(report2, SignalBlocking).Pass {
		t.Error("control: MF-4 expected FAIL on a bare TBD, got PASS")
	}
}

// TestReadiness_FX3_AllBlankKeyFilePathsFails pins FX-3: a non-empty
// key_file_paths slice whose elements are all blank/whitespace ("", "  ")
// is NOT a concrete files-in-scope declaration — MF-1 FAILs it, exactly as
// it would an empty slice. The pre-fix check only rejected len==0 or an
// exact scaffold-placeholder match, so an all-blank slice passed.
func TestReadiness_FX3_AllBlankKeyFilePathsFails(t *testing.T) {
	root := t.TempDir()
	specID, beadID, epicID := writeSimpleWorkspace(t, root, "fx3",
		"    key_file_paths:\n      - \"\"\n      - \"   \"",
		"- [ ] AC-1 — a criterion.",
		"This bead claims AC-1.")

	store := NewFakeBDStore()
	store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = FakeBeadRecord{Description: ""}
	restore := store.Install()
	t.Cleanup(restore)

	report, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatalf("EvaluateReadiness: %v", err)
	}
	mf1 := signalByID(report, SignalPlanSection)
	if mf1.Pass {
		t.Errorf("MF-1 expected FAIL for an all-blank key_file_paths slice, got PASS")
	}
	if mf1.Recovery == "" {
		t.Error("MF-1 FAIL must carry a recovery lever")
	}

	// Control: a single genuine concrete path PASSes MF-1.
	root2 := t.TempDir()
	specID2, beadID2, epicID2 := writeSimpleWorkspace(t, root2, "fx3ok",
		"    key_file_paths:\n      - internal/fixture/fx3ok.go",
		"- [ ] AC-1 — a criterion.",
		"This bead claims AC-1.")
	store2 := NewFakeBDStore()
	store2.Lineage[beadID2] = FakeLineage{EpicID: epicID2, SpecID: specID2}
	store2.Records[beadID2] = FakeBeadRecord{Description: ""}
	restore2 := store2.Install()
	t.Cleanup(restore2)
	report2, err := EvaluateReadiness(root2, beadID2)
	if err != nil {
		t.Fatalf("EvaluateReadiness (control): %v", err)
	}
	if !signalByID(report2, SignalPlanSection).Pass {
		t.Errorf("control: MF-1 expected PASS for a real concrete path, got FAIL: %q", signalByID(report2, SignalPlanSection).Detail)
	}
}

// TestReadiness_MF3ErrorArmDiscrimination (final-review r2 F2-1 /
// G2-R2-MF3): evaluateMF3's TWO FindLandedMerge refusal arms are
// operational ROUTING, not cosmetics — each must emit its own truthful
// state description and recovery verb:
//
//   - *lifecycle.LandedMergeNoEvidence (corroboration unavailable: a
//     candidate merge exists, no admissible datum confirms it) → the
//     dependency is already complete, so the recovery is `mindspec
//     reattest <dep>` (the spec-125 verb that git-corroborates the merge
//     and writes the missing landed binding) — NEVER `mindspec complete`.
//   - plain ErrLandedMergeNotFound (no candidate merge names the bead) →
//     the genuine closed-but-unmerged 2u0u split keeps `mindspec complete
//     <dep>` as its recovery, stated conservatively (an anonymous-subject
//     merge also lands here and is named as the other possibility).
//
// RED-on-revert: LandedMergeNoEvidence unwraps to ErrLandedMergeNotFound,
// so deleting the errors.As arm silently reroutes the NoEvidence state
// through the NotFound arm — still FAIL, but with the actively WRONG
// `mindspec complete` recovery for an already-merged dependency. The
// recovery-text assertions below turn that regression (and a collapse to
// false-PASS) red.
func TestReadiness_MF3ErrorArmDiscrimination(t *testing.T) {
	t.Run("i NoEvidence arm: corroboration-unavailable routes to reattest", func(t *testing.T) {
		root := t.TempDir()
		specID, beadID, epicID := writeSimpleWorkspace(t, root, "mf3noev",
			"    key_file_paths:\n      - internal/fixture/mf3noev.go",
			"- [ ] AC-1 — a criterion.",
			"This bead claims AC-1.")
		depID := "mindspec-mf3noev.0"

		store := NewFakeBDStore()
		store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
		store.Records[beadID] = FakeBeadRecord{
			Description:  "MF-3 NoEvidence-arm fixture.",
			Dependencies: []FakeDependency{{ID: depID, Status: "closed"}},
		}
		store.NoEvidenceDeps = map[string]bool{depID: true}
		restore := store.Install()
		t.Cleanup(restore)

		report, err := EvaluateReadiness(root, beadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness: %v", err)
		}
		mf3 := signalByID(report, SignalDependencies)
		if mf3.Pass {
			t.Fatal("MF-3 expected NOT-READY (FAIL) for a NoEvidence dependency, got PASS")
		}
		if !strings.Contains(mf3.Detail, "no admissible datum corroborates it") {
			t.Errorf("NoEvidence Detail must carry the corroboration-unavailable message, got %q", mf3.Detail)
		}
		if !strings.Contains(mf3.Recovery, "mindspec reattest "+depID) {
			t.Errorf("NoEvidence Recovery must route to `mindspec reattest %s`, got %q", depID, mf3.Recovery)
		}
		if strings.Contains(mf3.Recovery, "mindspec complete") {
			t.Errorf("NoEvidence Recovery must NOT prescribe `mindspec complete` (the dep is already complete), got %q", mf3.Recovery)
		}
	})

	t.Run("ii NotFound arm: no candidate merge keeps the complete recovery", func(t *testing.T) {
		root := t.TempDir()
		specID, beadID, epicID := writeSimpleWorkspace(t, root, "mf3nf",
			"    key_file_paths:\n      - internal/fixture/mf3nf.go",
			"- [ ] AC-1 — a criterion.",
			"This bead claims AC-1.")
		depID := "mindspec-mf3nf.0"

		store := NewFakeBDStore()
		store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
		store.Records[beadID] = FakeBeadRecord{
			Description:  "MF-3 NotFound-arm fixture.",
			Dependencies: []FakeDependency{{ID: depID, Status: "closed"}},
		}
		// The dep is listed FALSE so the seam is installed but reports the
		// plain ErrLandedMergeNotFound sentinel (no candidate merge).
		store.LandedDeps = map[string]bool{depID: false}
		restore := store.Install()
		t.Cleanup(restore)

		report, err := EvaluateReadiness(root, beadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness: %v", err)
		}
		mf3 := signalByID(report, SignalDependencies)
		if mf3.Pass {
			t.Fatal("MF-3 expected NOT-READY (FAIL) for an unmerged dependency, got PASS")
		}
		if strings.Contains(mf3.Detail, "no admissible datum corroborates it") {
			t.Errorf("NotFound Detail must NOT claim a candidate merge exists, got %q", mf3.Detail)
		}
		if !strings.Contains(mf3.Recovery, "mindspec complete "+depID) {
			t.Errorf("NotFound Recovery must keep `mindspec complete %s` for the genuine 2u0u split, got %q", depID, mf3.Recovery)
		}
		if !strings.Contains(mf3.Detail, "does not name the bead") {
			t.Errorf("NotFound Detail must state the anonymous-subject possibility conservatively (G2-R2-MF3-ANONYMOUS), got %q", mf3.Detail)
		}
	})
}

// TestReadiness_DescriptionLegDefectsRefuse (final-review r2 F2-3): the bd
// DESCRIPTION is a first-class scan surface for MF-2 and MF-4, not just
// the plan section — every other fixture plants its defect in the plan
// section, so a regression dropping rec.Description from either harvest
// would stay green without these two cases. Here the plan section is
// entirely benign and the ONLY defect lives in the bd description.
func TestReadiness_DescriptionLegDefectsRefuse(t *testing.T) {
	t.Run("MF-2: dangling AC token only in the bd description", func(t *testing.T) {
		root := t.TempDir()
		specID, beadID, epicID := writeSimpleWorkspace(t, root, "descmf2",
			"    key_file_paths:\n      - internal/fixture/descmf2.go",
			"- [ ] AC-1 — a criterion.",
			"This bead claims AC-1.")

		store := NewFakeBDStore()
		store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
		store.Records[beadID] = FakeBeadRecord{Description: "This bead also claims AC-99."}
		restore := store.Install()
		t.Cleanup(restore)

		report, err := EvaluateReadiness(root, beadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness: %v", err)
		}
		mf2 := signalByID(report, SignalTokens)
		if mf2.Pass {
			t.Error("MF-2 expected FAIL for a dangling AC-99 claimed ONLY in the bd description, got PASS")
		} else if !strings.Contains(mf2.Detail, "AC-99") {
			t.Errorf("MF-2 Detail must name the description-sourced dangling token, got %q", mf2.Detail)
		}
	})

	t.Run("MF-4: TBD marker only in the bd description", func(t *testing.T) {
		root := t.TempDir()
		specID, beadID, epicID := writeSimpleWorkspace(t, root, "descmf4",
			"    key_file_paths:\n      - internal/fixture/descmf4.go",
			"- [ ] AC-1 — a criterion.",
			"This bead claims AC-1.")

		store := NewFakeBDStore()
		store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
		store.Records[beadID] = FakeBeadRecord{Description: "TBD: decide the export format."}
		restore := store.Install()
		t.Cleanup(restore)

		report, err := EvaluateReadiness(root, beadID)
		if err != nil {
			t.Fatalf("EvaluateReadiness: %v", err)
		}
		mf4 := signalByID(report, SignalBlocking)
		if mf4.Pass {
			t.Error("MF-4 expected FAIL for a TBD marker present ONLY in the bd description, got PASS")
		} else if !strings.Contains(mf4.Detail, "TBD") {
			t.Errorf("MF-4 Detail must surface the description-sourced blocking marker, got %q", mf4.Detail)
		}
	})
}
