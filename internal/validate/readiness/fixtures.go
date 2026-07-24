package readiness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// fixtures.go hosts the EXPORTED NEGATIVE/POSITIVE readiness fixture
// builders (spec 124 R6) as a NON-test file: Go cannot import a _test.go
// file across packages, and all three consumers — this package's own
// engine tests, cmd/mindspec's verb tests, and (via Bead 2) the `next`
// gate tests — need the identical planted defects / benign features so
// every consumer exercises the SAME fixture.

// FakeDependency is one "blocks" dependency edge a FakeBeadRecord carries.
type FakeDependency struct {
	ID     string
	Status string
}

// FakeBeadRecord is an in-memory stand-in for a bead's bd record.
type FakeBeadRecord struct {
	Description  string
	Dependencies []FakeDependency
	// Metadata mirrors the metadata field a real `bd show` also returns.
	// EvaluateReadiness's mechanical signals NEVER read this field back —
	// spec 124 AC-12's layer-boundary invariance holds by construction
	// (no MF signal's code path inspects it), not via a runtime check.
	// SeedReadinessAttempt populates it for tests that demonstrate the
	// invariance explicitly.
	Metadata map[string]interface{}
}

// FakeLineage is an in-memory stand-in for a bead's bd/phase lineage
// (bead -> epic -> owning spec).
type FakeLineage struct {
	EpicID string
	SpecID string
}

// FakeBDStore is an in-memory, seam-injectable stand-in for EVERY bd read
// EvaluateReadiness performs (spec 124 plan-gate F3-1 bd-less
// hermeticity): Install swaps findEpicForBeadFn/fetchBeadRecordFn to read
// from Records/Lineage instead of a real bd process, so engine tests
// consult no real bd and never t.Skip.
type FakeBDStore struct {
	Records map[string]FakeBeadRecord
	Lineage map[string]FakeLineage

	// LandedDeps, when non-empty, ALSO fakes MF-3's landed-merge decision
	// (FX-1): Install swaps findLandedMergeFn so a dep listed here (value
	// true) reports a positive landed merge and any OTHER dep reports
	// ErrLandedMergeNotFound — consulting neither git NOR the transitive
	// bd read (lifecycle.FindLandedMerge -> bead.GetMetadata) the real
	// function performs. When LandedDeps AND NoEvidenceDeps are both
	// empty/nil, Install leaves findLandedMergeFn at its real default so
	// the AC-3 real-repo fixtures exercise the genuine git+bd landed-merge
	// predicate end-to-end.
	LandedDeps map[string]bool

	// NoEvidenceDeps, when non-empty, makes the faked findLandedMergeFn
	// return a *lifecycle.LandedMergeNoEvidence for a dep listed here
	// (checked BEFORE LandedDeps) — the spec-125 corroboration-unavailable
	// state: a candidate merge exists but no admissible datum confirms it.
	// This is the final-review r2 F2-1 seam: it lets a consumer test drive
	// evaluateMF3's errors.As arm (Detail: "no admissible datum
	// corroborates it", Recovery: `mindspec reattest`) without a real git
	// history, pinning the arm discrimination against the plain
	// ErrLandedMergeNotFound arm's `mindspec complete` recovery.
	NoEvidenceDeps map[string]bool
}

// NewFakeBDStore returns an empty, ready-to-populate FakeBDStore.
func NewFakeBDStore() *FakeBDStore {
	return &FakeBDStore{
		Records: map[string]FakeBeadRecord{},
		Lineage: map[string]FakeLineage{},
	}
}

// SeedReadinessAttempt mimics bead.MergeMetadata's read-merge-write
// semantics against this in-memory store: it merges key/value into
// beadID's Metadata map (creating the record if absent). Spec 124 AC-12
// uses this to seed a readiness-attempt record without a real bd process;
// EvaluateReadiness never reads this field back (see FakeBeadRecord.Metadata).
func (s *FakeBDStore) SeedReadinessAttempt(beadID, key string, value interface{}) {
	rec := s.Records[beadID]
	if rec.Metadata == nil {
		rec.Metadata = map[string]interface{}{}
	}
	rec.Metadata[key] = value
	s.Records[beadID] = rec
}

// Install swaps the package-level bd/lineage seam vars to read from s,
// returning a restore func the caller must invoke (t.Cleanup/defer) to
// avoid leaking fixture state into a later test.
func (s *FakeBDStore) Install() (restore func()) {
	origFindEpic := findEpicForBeadFn
	origFetch := fetchBeadRecordFn
	findEpicForBeadFn = func(beadID string) (string, string, error) {
		l, ok := s.Lineage[beadID]
		if !ok {
			return "", "", fmt.Errorf("fake bd store: no lineage recorded for %s", beadID)
		}
		return l.EpicID, l.SpecID, nil
	}
	fetchBeadRecordFn = func(beadID string) (*beadRecord, error) {
		r, ok := s.Records[beadID]
		if !ok {
			return nil, fmt.Errorf("fake bd store: no record recorded for %s", beadID)
		}
		rec := &beadRecord{Description: r.Description}
		for _, d := range r.Dependencies {
			rec.Dependencies = append(rec.Dependencies, dependencyEdge{ID: d.ID, Status: d.Status})
		}
		return rec, nil
	}
	origFindLanded := findLandedMergeFn
	if len(s.LandedDeps) > 0 || len(s.NoEvidenceDeps) > 0 {
		findLandedMergeFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			if s.NoEvidenceDeps[beadID] {
				return nil, &lifecycle.LandedMergeNoEvidence{
					BeadID:       beadID,
					SpecBranch:   specBranch,
					MergeSHA:     "fakedcandidatemerge",
					SecondParent: "fakedsecondparent",
				}
			}
			if s.LandedDeps[beadID] {
				return &lifecycle.LandedMerge{SHA: "faked", FirstParent: "faked", SecondParent: "faked"}, nil
			}
			return nil, fmt.Errorf("%s: %w", beadID, lifecycle.ErrLandedMergeNotFound)
		}
	}
	return func() {
		findEpicForBeadFn = origFindEpic
		fetchBeadRecordFn = origFetch
		findLandedMergeFn = origFindLanded
	}
}

// Fixture bundles what a readiness fixture builder hands back: the spec/
// bead IDs under test and the FakeBDStore carrying the bd-side state
// (installed by the caller via Store.Install()).
type Fixture struct {
	SpecID    string
	BeadID    string
	DepBeadID string
	Store     *FakeBDStore
}

// --- git plumbing shared by both fixtures' MF-3 dependency state ---

func gitRun(root string, args ...string) error {
	_, err := gitOutput(root, args...)
	return err
}

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=readiness-fixture", "GIT_AUTHOR_EMAIL=readiness-fixture@test.invalid",
		"GIT_COMMITTER_NAME=readiness-fixture", "GIT_COMMITTER_EMAIL=readiness-fixture@test.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %s: %w", args, strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// registerLandedPanel writes a fs-only, hermetic (zero-bd) panel.json
// registration under root/review/<slug>/ naming depID as the panel's bead
// and headSHA as its reviewed_head_sha — one of the three admissible
// corroboration data internal/lifecycle.FindLandedMerge consults
// (internal/panel is fs-only by design, ADR-0037). Real completed work
// merged via `mindspec complete` is corroborated in production by the
// merge-time landed-binding bd metadata OR a registered panel OR a
// surviving branch tip; fixtures use the panel leg specifically because
// it requires no bd process at all, keeping MF-3's git-real fixtures
// (see landed_test.go's own hermetic TestMain) genuinely bd-less.
func registerLandedPanel(root, depID, target, headSHA string) error {
	dir := filepath.Join(root, "review", "fixture-"+strings.NewReplacer(".", "-", "/", "-").Replace(depID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	payload := fmt.Sprintf(`{"bead_id": %q, "spec": "fixture", "target": %q, "round": 1, "expected_reviewers": 1, "reviewed_head_sha": %q}`,
		depID, target, headSHA)
	return os.WriteFile(filepath.Join(dir, "panel.json"), []byte(payload), 0o644)
}

// initGitRepo initializes a throwaway git repo at root with specID's spec
// branch checked out (the internal/lifecycle/landed_test.go fixture shape
// FindLandedMerge operates over). Returns the spec branch name.
func initGitRepo(root, specID string) (string, error) {
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return "", err
	}
	if err := gitRun(root, "init", "-b", "main"); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# readiness fixture\n"), 0o644); err != nil {
		return "", err
	}
	if err := gitRun(root, "add", "-A"); err != nil {
		return "", err
	}
	if err := gitRun(root, "commit", "-m", "initial"); err != nil {
		return "", err
	}
	if err := gitRun(root, "checkout", "-b", specBranch); err != nil {
		return "", err
	}
	return specBranch, nil
}

// mergeDependency creates bead/<depID> off the currently-checked-out
// specBranch, commits one change, merges it back (--no-ff, the
// deterministic gitutil.MergeInto subject "Merge bead/<id>"), and — when
// deleteBranch is true — deletes the bead branch, mirroring the state
// `mindspec complete` leaves behind (mindspec_executor.go:468): the normal,
// branch-deleted landed-merge shape MF-3 must tolerate.
func mergeDependency(root, specBranch, depID string, deleteBranch bool) error {
	beadBranch, err := workspace.BeadBranch(depID)
	if err != nil {
		return err
	}
	if err := gitRun(root, "checkout", "-b", beadBranch); err != nil {
		return err
	}
	fname := filepath.Join(root, strings.ReplaceAll(depID, "/", "_")+".txt")
	if err := os.WriteFile(fname, []byte(depID+"\n"), 0o644); err != nil {
		return err
	}
	if err := gitRun(root, "add", "-A"); err != nil {
		return err
	}
	if err := gitRun(root, "commit", "-m", "work "+depID); err != nil {
		return err
	}
	tipSHA, err := gitOutput(root, "rev-parse", beadBranch)
	if err != nil {
		return err
	}
	if err := gitRun(root, "checkout", specBranch); err != nil {
		return err
	}
	if err := gitRun(root, "merge", "--no-ff", "-m", "Merge "+beadBranch, beadBranch); err != nil {
		return err
	}
	// Register a fs-only, hermetic panel corroboration BEFORE any branch
	// deletion — internal/lifecycle.FindLandedMerge never treats a bare
	// "Merge bead/<id>" subject match as sufficient on its own (spec 121);
	// it needs at least one admissible datum. A surviving branch tip
	// already corroborates the deleteBranch=false case, but the
	// deleteBranch=true case (the normal `mindspec complete` shape) has
	// no OTHER corroboration available without a real bd process, so
	// every fixture merge registers this panel leg unconditionally
	// (harmless — and idempotent evidence — when the branch also survives).
	if err := registerLandedPanel(root, depID, specBranch, tipSHA); err != nil {
		return err
	}
	if deleteBranch {
		if err := gitRun(root, "branch", "-D", beadBranch); err != nil {
			return err
		}
	}
	return nil
}

// createUnmergedDependencyBranch creates bead/<depID> off the
// currently-checked-out specBranch with one commit, WITHOUT merging it —
// the 2u0u split fixture state (closed in bd, branch present, never
// landed-merged) — then returns to specBranch.
func createUnmergedDependencyBranch(root, specBranch, depID string) error {
	beadBranch, err := workspace.BeadBranch(depID)
	if err != nil {
		return err
	}
	if err := gitRun(root, "checkout", "-b", beadBranch); err != nil {
		return err
	}
	fname := filepath.Join(root, strings.ReplaceAll(depID, "/", "_")+".txt")
	if err := os.WriteFile(fname, []byte(depID+"\n"), 0o644); err != nil {
		return err
	}
	if err := gitRun(root, "add", "-A"); err != nil {
		return err
	}
	if err := gitRun(root, "commit", "-m", "work "+depID); err != nil {
		return err
	}
	return gitRun(root, "checkout", specBranch)
}

// writeWorkspace writes spec.md/plan.md for specID under root at the
// on-disk path workspace.SpecDir resolves for a plain, non-worktree,
// non-.mindspec root: docs/specs/<id>/ (the internal/validate
// plan_test.go/divergence_test.go temp-workspace convention).
func writeWorkspace(root, specID, specMD, planMD string) error {
	dir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specMD), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "plan.md"), []byte(planMD), 0o644)
}

// --- NEGATIVE fixture (spec 124 R6(a) / AC-1) ---

const negativeSpecID = "999-fixture-negative"
const negativeEpicID = "mindspec-negb"
const negativeBeadID = "mindspec-negb.1"
const negativeDepBeadID = "mindspec-negb.0"

const negativeSpecMD = `# Spec 999-fixture-negative

## Requirements

1. **R1.** A placeholder requirement for the negative fixture.

## Acceptance Criteria

- [ ] AC-19 — the only acceptance criterion this fixture's spec defines.
`

const negativePlanMD = `---
status: Approved
spec_id: 999-fixture-negative
version: "1"
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - path/to/file.go
---
# Plan: 999-fixture-negative

## ADR Fitness

No ADRs are relevant to this fixture.

## Testing Strategy

Unit tests.

## Bead 1: Negative fixture

**Steps**
1. Step one

**Acceptance Criteria**
- <Specific, measurable criterion for this bead>

This bead claims AC-99 and AC-1 (both dangling — spec.md defines only AC-19).

Note: TBD - finalize the retry count.

**Blocking Questions**
- [ ] Should we support X?

**Depends on**
mindspec-negb.0 (human-readable only; bd dependency edges are wired via bd, not this prose)
`

// BuildNegativeFixture writes the negative fixture's spec.md/plan.md under
// root, initializes a throwaway git repo with the dependency bead's branch
// present but NOT landed-merged (the 2u0u closed-but-unmerged split — the
// MF-3 planted defect), and returns a Fixture whose Store is populated but
// NOT yet installed (the caller calls Store.Install()).
func BuildNegativeFixture(root string) (*Fixture, error) {
	if err := writeWorkspace(root, negativeSpecID, negativeSpecMD, negativePlanMD); err != nil {
		return nil, err
	}
	specBranch, err := initGitRepo(root, negativeSpecID)
	if err != nil {
		return nil, err
	}
	if err := createUnmergedDependencyBranch(root, specBranch, negativeDepBeadID); err != nil {
		return nil, err
	}

	store := NewFakeBDStore()
	store.Lineage[negativeBeadID] = FakeLineage{EpicID: negativeEpicID, SpecID: negativeSpecID}
	store.Records[negativeBeadID] = FakeBeadRecord{
		Description: "Negative fixture bead for spec 124 AC-1 pinning.",
		Dependencies: []FakeDependency{
			{ID: negativeDepBeadID, Status: "closed"},
		},
	}
	return &Fixture{SpecID: negativeSpecID, BeadID: negativeBeadID, DepBeadID: negativeDepBeadID, Store: store}, nil
}

// --- POSITIVE fixture (spec 124 R6(b) / AC-2) ---

const positiveSpecID = "998-fixture-positive"
const positiveEpicID = "mindspec-posb"
const positiveBeadID = "mindspec-posb.1"
const positiveDepBeadID = "mindspec-posb.0"

const positiveSpecMD = `# Spec 998-fixture-positive

## Requirements

1. **R5a.** A sub-lettered-only requirement.

## Acceptance Criteria

- [ ] AC-7 — a real, resolvable acceptance criterion.
- [ ] AC-9 — another real, resolvable acceptance criterion.
`

const positivePlanMD = `---
status: Approved
spec_id: 998-fixture-positive
version: "1"
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/fixture/positive.go
---
# Plan: 998-fixture-positive

## ADR Fitness

No ADRs are relevant to this fixture.

## Testing Strategy

Unit tests.

## Bead 1: Positive fixture

**Steps**
1. Step one
2. Step two

**Verification**
- [ ] ` + "`make test`" + ` passes

**Acceptance Criteria**
- [ ] AC-7 — some real criterion, satisfied by this bead.
- [ ] AC-9 — another real criterion, satisfied by this bead.

This bead claims R5 in full and AC-9(i).

It follows the spec 123 AC-17 pattern for its harvest-exclusion fixture
(a citation, not a claim — the foreign-citation SPAN exclusion covers
exactly the citation's own adjacent tokens, never the rest of the line
or the claim above). See the ` + "`TBD`" + `/` + "`OPEN QUESTION`" + `
convention quoted here as fixture data, inside backticks.

**Depends on**
mindspec-posb.0 (human-readable only; bd dependency edges are wired via bd, not this prose)
`

// BuildPositiveFixture writes the positive fixture's spec.md/plan.md under
// root, initializes a throwaway git repo with the dependency bead landed-
// merged AND branch-deleted (the normal post-`mindspec complete` shape),
// and returns a Fixture whose Store is populated but not yet installed.
func BuildPositiveFixture(root string) (*Fixture, error) {
	if err := writeWorkspace(root, positiveSpecID, positiveSpecMD, positivePlanMD); err != nil {
		return nil, err
	}
	specBranch, err := initGitRepo(root, positiveSpecID)
	if err != nil {
		return nil, err
	}
	if err := mergeDependency(root, specBranch, positiveDepBeadID, true); err != nil {
		return nil, err
	}

	store := NewFakeBDStore()
	store.Lineage[positiveBeadID] = FakeLineage{EpicID: positiveEpicID, SpecID: positiveSpecID}
	store.Records[positiveBeadID] = FakeBeadRecord{
		Description: "Positive fixture bead for spec 124 AC-2 pinning — modeled on spec 123 Bead 1.",
		Dependencies: []FakeDependency{
			{ID: positiveDepBeadID, Status: "closed"},
		},
	}
	return &Fixture{SpecID: positiveSpecID, BeadID: positiveBeadID, DepBeadID: positiveDepBeadID, Store: store}, nil
}
