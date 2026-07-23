package readiness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		SignalPlanSection:  "placeholder",
		SignalTokens:       "AC-99",
		SignalDependencies: "not landed-merged",
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
		{"enumerator basic: AC-9(i) resolves via bare AC-9", "This bead claims AC-9(i).", "- [ ] AC-9 — some criterion.", true},
		{"enumerator collision: AC-9(i) still resolves via base, even though AC-9i also exists", "This bead claims AC-9(i).", "- [ ] AC-9i — some criterion.", true},
		{"direct sub-letter (no parens) AC-9i dangles against bare AC-9 only", "This bead claims AC-9i directly.", "- [ ] AC-9 — some criterion.", false},
		{"sub-letter parenthetical R5(b) resolves exact", "This bead claims R5(b).", "- [ ] R5b — some criterion.", true},
		{"sub-letter parenthetical R5(b) fails against bare spec token (lenient-impl trap)", "This bead claims R5(b).", "- [ ] R5 — some criterion.", false},
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
