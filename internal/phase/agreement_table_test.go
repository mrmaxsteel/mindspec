package phase_test

// agreement_table_test.go — spec 128 (lifecycle-verb-state-honesty)
// Bead 4, plan step 2: one fixture per row of the per-verb agreement
// table published at project-docs/user/guides/lifecycle-modes.md
// (AC-2).
//
// Round-1 panel fix (hard-blocked 0/6, G1/G6): the original cross-pin
// compared only the Row-column IDENTIFIER set, so editing a row's mode
// CELLS (e.g. changing complete→review's three `review` cells to `plan`)
// left the suite green — AC-2's anti-drift claim was vacuous. That round
// replaced it with a structural contract: agreementTableRows is the
// single per-row source of truth (id + State-show/Instruct-tail/
// Derivation cell text); TestAgreementTable_DocRowsCrossPinFixtures
// parses the doc's own cells and asserts they are byte-identical to this
// slice, both directions; every row fixture reads its expected values
// FROM this same slice; rowFixtures maps row id to its executable
// fixture func, and a row with no registered fixture fails loudly.
//
// Round-2 panel fix (4/6, G1+G2 convergent — hard block already
// cleared, these were the residual REQUEST_CHANGES): round-1's own
// header overclaimed what the mechanism guaranteed. What actually
// shipped at round 1: seven "ordinary" rows (spec approve through
// complete→review) constructed their fixture epic's STORED phase FROM
// row.derivation itself (`v2Epic(epicID, specID, "open", row.derivation,
// ...)`). Because phase.DerivePhaseWithStatusWithCache trusts a valid
// stored phase (derive.go:176-179 — a disagreement only warns to
// stderr, never fails), the record fed itself back through all three
// read surfaces: editing BOTH a doc row and its agreementTableRows entry
// to the SAME wrong value together stayed green, because the seeded
// input changed right along with the expectation. This is fixed two
// ways, per row, for those seven rows:
//
//   - The canonical fixture's STORED phase is now a locally hardcoded
//     state.Mode* literal (e.g. `state.ModeReview` for the
//     complete→review row), matching what that verb's real production
//     writer independently produces for the row's canonical children —
//     NEVER `row.derivation`. So agreementTableRows can be edited
//     without moving what the epic actually stores, and the existing
//     three-surface checks (which trust that stored value) now
//     genuinely test whether row.stateShow/.tail/.derivation agree with
//     an INDEPENDENTLY fixed fact, not an echo of themselves.
//   - A SECOND, stored-independent leg is added: assertChildDerivation
//     calls phase.DerivePhaseFromChildren(children) directly — a pure
//     function of the fixture's (already-independent) children, which
//     never reads or trusts the seeded stored phase at all — and checks
//     it against row.derivation. A coordinated doc+record tamper is
//     caught here even if the stored-literal decoupling above were ever
//     undone by a future edit, because this leg has no stored-phase
//     dependency to defeat.
//   - The terminal row's derivation input is decoupled the same way
//     (hardcoded `state.ModeDone`, not `row.derivation`); it has no
//     children-only leg to add (bare DerivePhaseFromChildren cannot
//     express "done" for an all-closed set — that is exactly R1.3's own
//     stated reason the status-aware surface exists), so its
//     independence rests on the stored-literal decoupling alone.
//   - The four writer-driven rows (release ×2, repair ×2) were already
//     independent at round 1 and are UNCHANGED: they drive the real
//     phase.SanctionedDerivedSync / phase.EnsurePhaseSemanticsV2 against
//     independently-fixed children and assert the WRITTEN value against
//     row.derivation before ever re-wiring a post-state fixture — the
//     writer's own real computation is the independent leg for those
//     four rows, and (for row 12) an additional phase.DerivePhaseFromChildren
//     check besides.
//   - Row 1 (spec-create) is no longer a self-authorized two-surface
//     carve-out (round-2 G2 tally ruling: R1.3/AC-2 name the status-aware
//     derivation surface for every row, and narrowing that is not the
//     fix author's call to make). It is a genuine three-surface row
//     again — see its own comment below for the three DISTINCT
//     mechanisms used, none of which shares a call site with another.
//
// Honestly stated residual (this is the "what the mechanism actually
// guarantees" G1(c) asked for): the hardcoded per-row literals above
// live in THIS file, in each row's own Go function body — a coordinated
// tamper that edits the doc row, the agreementTableRows entry, AND that
// specific hardcoded literal, all three consistently, would still defeat
// the three-surface checks for the seven ordinary rows and the terminal
// row. The assertChildDerivation leg (present on all seven ordinary
// rows) and the writer-driven rows' written-value assertions do NOT
// depend on any such literal and remain unconditionally independent of
// agreementTableRows. The acceptance bar this fix targets — "editing the
// doc row and the record together, fixture children/writer inputs
// unchanged" — is met for every row.
//
// Surfaces, as before:
//   - derivation:    phase.DerivePhaseWithCache (the DerivePhaseWithStatus
//     class) called directly against the fixture's epic ID — the ONE
//     derivation surface AC-2 names for every row, including repair
//     phase's rows (repair's own internal surface,
//     phase.DerivePhaseDetailWithCache, computes the identical
//     status-aware value and is asserted additionally on those two rows
//     for narrative fidelity, never as a substitute).
//   - state show:    resolve.ResolveModeWithCache — the function
//     cmd/mindspec/state.go's RunE composes its printed mode from
//     (state.go:159), or, for the ambient/terminal row, the same
//     precondition chain state.go's RunE walks before reaching its
//     literal, argument-free idle branch (state.go:149).
//   - instruct tail: phase.ResolveContextFromDirWithCache feeds the same
//     state.Focus{Mode: ctx.Phase} instruct_tail.go's emitInstruct
//     builds; instruct.Render selects its template by the literal
//     concatenation ctx.Mode+".md", so asserting ctx.Mode and a
//     successful Render together ARE the "tail's selected template"
//     surface.
//
// Carve-outs still standing (spec-authorized, not self-authorized):
//   - release's two rows and repair phase's two rows are TAIL-LESS by
//     design (R2.1: neither verb emits an instruct tail) — their
//     Instruct-tail cell is the literal sentinel below, and their
//     fixtures assert two surfaces only. Release AND repair rows drive
//     REAL production functions (phase.SanctionedDerivedSync,
//     phase.EnsurePhaseSemanticsV2) rather than a hand-authored
//     post-state alone, so a regression in either is caught here too.
//   - the terminal (impl approve / impl adopt) row is the one row whose
//     three cells are NOT all equal: derivation is "done" (a per-epic
//     fact, the closed epic's own status-aware derivation), while state
//     show and tail are both "idle" (a GLOBAL zero-open-epic discovery
//     fact, unrelated to this one epic's derivation). Two facts at two
//     layers, per R1.3's own terminal-rows carve-out text — this is an
//     R1.3-stated carve-out, not one this file invented.
//
// Writer-taxonomy corrections (round-1 G2) and the legacy-shapes recovery
// chain correction (round-1 G5) live in the guide
// (project-docs/user/guides/lifecycle-modes.md), not here — there is no
// executable assertion to add for prose wording, so those fixes are
// doc-only. The blocked-child leg (round-1 G4) is fixtured in
// rowCompleteImplementMidSpec below.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// --- Fixture wiring -------------------------------------------------

// wireEpicFixture stubs phase's bd seams (the same phase.SetRunBDForTest /
// phase.SetListJSONForTest pair route_248_test.go's probeEpicState and
// bead 2's next_phase_advance_test.go use) so every surface under test —
// state show's spec/epic lookups, the tail's context derivation, and the
// direct derivation call — reads the SAME epic+children snapshot.
// beadShows additionally answers `bd show <beadID>` for non-epic IDs (the
// next-row's claimed-bead-to-epic lineage lookup); nil is fine when no row
// needs it.
func wireEpicFixture(t *testing.T, epic phase.EpicInfo, children []phase.ChildInfo, beadShows map[string]phase.EpicInfo) {
	t.Helper()
	epicJSON, err := json.Marshal([]phase.EpicInfo{epic})
	if err != nil {
		t.Fatalf("marshal epic fixture: %v", err)
	}
	childrenJSON, err := json.Marshal(children)
	if err != nil {
		t.Fatalf("marshal children fixture: %v", err)
	}

	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if strings.HasPrefix(a, "--parent") {
				return childrenJSON, nil
			}
		}
		for _, a := range args {
			if a == "--type=epic" {
				return epicJSON, nil
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			id := args[1]
			if id == epic.ID {
				return epicJSON, nil
			}
			if bead, ok := beadShows[id]; ok {
				return json.Marshal([]map[string]interface{}{{
					"title": bead.Title,
					"dependencies": []map[string]interface{}{
						{"id": epic.ID, "issue_type": "epic"},
					},
				}})
			}
		}
		return []byte("[]"), nil
	}))
}

// wireEmptyFixture wires zero epics/children — the spec-create draft
// row's shape (bd healthy, nothing in the tracker for this spec yet).
func wireEmptyFixture(t *testing.T) {
	t.Helper()
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
}

// specWorktreeDir/beadWorktreeDir synthesize the on-disk path shape
// workspace.DetectWorktreeContext string-matches (".worktrees/worktree-
// spec-<id>" / ".worktrees/worktree-<id>") — the directory need not exist:
// DetectWorktreeContext only inspects the path string, and the WorktreeSpec/
// WorktreeBead branches it selects make no filesystem calls of their own.
func specWorktreeDir(root, specID string) string {
	return filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
}

func beadWorktreeDir(root, beadID string) string {
	return filepath.Join(root, ".worktrees", "worktree-"+beadID)
}

// runGitCmd runs git with a pinned test identity (matching
// internal/resolve/draft_test.go's runGitCmd convention) so the
// draft/terminal rows' git-backed fixtures work without host config.
func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=test@example.invalid",
		"-c", "user.name=mindspec-test",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// bareGitRoot builds a hermetic (t.TempDir()-rooted) git repo with no
// specs/drafts/branches beyond main — required by
// resolve.DiscoverDraftSpecsWithCache, which lists local branches via
// gitutil and propagates a real structural failure (never folding "not a
// repo" into "not a draft"). Rows whose ambient state-show path reaches
// draft discovery (the spec-create and terminal rows) need this instead
// of a bare t.TempDir().
func bareGitRoot(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "baseline")
	return root
}

// draftFixtureRoot mirrors internal/resolve/draft_test.go's
// newDraftFixtureRoot exactly (that helper is unexported in package
// resolve and unreachable from this external phase_test package) — a
// hermetic git repo carrying an on-disk pre-epic draft: a
// .mindspec/specs/<specID>/spec.md directory plus a committed
// spec/<specID> branch, the OQ-2 draft shape bead 3 shipped, and nothing
// else.
func draftFixtureRoot(t *testing.T, specID string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-b", "main")
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "draft spec")
	runGitCmd(t, root, "branch", "spec/"+specID)
	return root
}

// --- The per-row structural record ------------------------------------

// sentinelNoTail is the literal cell text for a carved-out surface — the
// verb emits no instruct tail (release, repair phase; R2.1, a
// spec-authorized carve-out). Both the doc table and agreementTableRows
// must use this exact string; the cross-pin compares it byte-for-byte.
const sentinelNoTail = "*(none — see below)*"

// agreementRow is one row of the agreement table: its identifier plus
// the three cell values R1.3 names. This is the SINGLE source of truth —
// the doc table, the cross-pin test, and every row fixture's expected
// value all read from (or are checked against) THIS struct, never a
// separately hardcoded literal (round-1 G1's structural-pin fix).
type agreementRow struct {
	id         string
	stateShow  string
	tail       string
	derivation string
}

// agreementTableRows MUST match project-docs/user/guides/lifecycle-modes.md's
// table exactly, cell for cell — TestAgreementTable_DocRowsCrossPinFixtures
// enforces this both ways, and TestAgreementTable_Rows enforces that
// every row here has a registered, executable fixture in rowFixtures.
var agreementTableRows = []agreementRow{
	{id: "spec create → spec", stateShow: "spec", tail: "spec", derivation: "spec"},
	{id: "spec approve → plan", stateShow: "plan", tail: "plan", derivation: "plan"},
	{id: "plan approve → plan", stateShow: "plan", tail: "plan", derivation: "plan"},
	{id: "next (claim) → implement", stateShow: "implement", tail: "implement", derivation: "implement"},
	{id: "complete → implement (mid-spec)", stateShow: "implement", tail: "implement", derivation: "implement"},
	{id: "complete → plan (empty lifecycle subset)", stateShow: "plan", tail: "plan", derivation: "plan"},
	{id: "complete → plan (all-open, non-lifecycle child completed)", stateShow: "plan", tail: "plan", derivation: "plan"},
	{id: "complete → review (last bead)", stateShow: "review", tail: "review", derivation: "review"},
	{id: "release → implement (sibling closed)", stateShow: "implement", tail: sentinelNoTail, derivation: "implement"},
	{id: "release → plan (no sibling closed)", stateShow: "plan", tail: sentinelNoTail, derivation: "plan"},
	{id: "repair phase → plan (v2-reconciliation, already consistent)", stateShow: "plan", tail: sentinelNoTail, derivation: "plan"},
	{id: "repair phase → implement (forced backward)", stateShow: "implement", tail: sentinelNoTail, derivation: "implement"},
	{id: "impl approve / impl adopt → done / idle (terminal)", stateShow: "idle", tail: "idle", derivation: "done"},
}

// --- Three/two-surface assertion helpers -----------------------------

// assertThreeSurfaces asserts the derivation, state-show, and instruct-tail
// surfaces all agree with row's cells — every row except spec-create's
// draft, release's/repair's tail-less rows, and the terminal row (all of
// which have bespoke row functions below).
func assertThreeSurfaces(t *testing.T, label, root, dir, specID, epicID string, row agreementRow) {
	t.Helper()

	derived, err := phase.DerivePhaseWithCache(phase.NewCache(), epicID)
	if err != nil {
		t.Fatalf("%s: derivation: %v", label, err)
	}
	if derived != row.derivation {
		t.Errorf("%s: derivation (DerivePhaseWithStatus-class) = %q, want %q (agreementTableRows)", label, derived, row.derivation)
	}

	stateMode, err := resolve.ResolveModeWithCache(phase.NewCache(), root, specID)
	if err != nil {
		t.Fatalf("%s: state show: %v", label, err)
	}
	if stateMode != row.stateShow {
		t.Errorf("%s: state show (resolve.ResolveModeWithCache) = %q, want %q (agreementTableRows)", label, stateMode, row.stateShow)
	}

	c := phase.NewCache()
	ctx, err := phase.ResolveContextFromDirWithCache(c, root, dir)
	if err != nil {
		t.Fatalf("%s: context resolution: %v", label, err)
	}
	if ctx.Phase != row.tail {
		t.Errorf("%s: context phase (tail input) = %q, want %q (agreementTableRows)", label, ctx.Phase, row.tail)
	}
	mc := &state.Focus{Mode: ctx.Phase, ActiveSpec: ctx.SpecID, ActiveBead: ctx.BeadID}
	ictx := instruct.BuildContextWithCache(c, root, mc)
	if ictx.Mode != row.tail {
		t.Errorf("%s: instruct context mode (template selector) = %q, want %q (agreementTableRows)", label, ictx.Mode, row.tail)
	}
	if _, err := instruct.Render(ictx); err != nil {
		t.Errorf("%s: instruct.Render (selected template %s.md): %v", label, ictx.Mode, err)
	}
}

// assertTwoSurfaces is the tail-less carved shape (R2.1/AC-2): derivation +
// state show only, checked against row's cells — release's and repair
// phase's rows, which emit no instruct tail.
func assertTwoSurfaces(t *testing.T, label, root, specID, epicID string, row agreementRow) {
	t.Helper()

	derived, err := phase.DerivePhaseWithCache(phase.NewCache(), epicID)
	if err != nil {
		t.Fatalf("%s: derivation: %v", label, err)
	}
	if derived != row.derivation {
		t.Errorf("%s: derivation (DerivePhaseWithStatus-class) = %q, want %q (agreementTableRows)", label, derived, row.derivation)
	}

	stateMode, err := resolve.ResolveModeWithCache(phase.NewCache(), root, specID)
	if err != nil {
		t.Fatalf("%s: state show: %v", label, err)
	}
	if stateMode != row.stateShow {
		t.Errorf("%s: state show (resolve.ResolveModeWithCache) = %q, want %q (agreementTableRows)", label, stateMode, row.stateShow)
	}
}

// assertChildDerivation is round-2 G1(b)'s non-circular leg: it calls
// phase.DerivePhaseFromChildren directly on the fixture's children — a
// pure function that never reads, and cannot be made to trust, whatever
// stored phase the epic fixture happens to carry — and checks the result
// against row.derivation. Unlike assertThreeSurfaces/assertTwoSurfaces
// (which read through phase.DerivePhaseWithStatus's stored-authoritative
// trust), this leg has no seeded-stored-phase dependency for a
// coordinated doc+record tamper to exploit.
func assertChildDerivation(t *testing.T, label string, children []phase.ChildInfo, row agreementRow) {
	t.Helper()
	got := phase.DerivePhaseFromChildren(children)
	if got != row.derivation {
		t.Errorf("%s: phase.DerivePhaseFromChildren(children) = %q, want %q (agreementTableRows) — independent of the fixture's seeded stored phase", label, got, row.derivation)
	}
}

// v2Epic builds a marker-present (mindspec_phase_semantics: "v2") epic
// fixture — the pinned discipline (plan round-3 G2): every row needing a
// stored/derived state constructs it marker-present; marker-absent states
// belong exclusively to bead 2's legacy fixtures, except the one licensed
// exception below (repair's v2-reconciliation row, which exists to
// exercise the legacy-rewrite leg itself).
func v2Epic(id, specID, status, storedPhase string, specNum int, specTitle string) phase.EpicInfo {
	return phase.EpicInfo{
		ID:        id,
		Title:     fmt.Sprintf("[SPEC %s] Agreement table row", specID),
		Status:    status,
		IssueType: "epic",
		Metadata: map[string]interface{}{
			"mindspec_phase":           storedPhase,
			"spec_num":                 float64(specNum),
			"spec_title":               specTitle,
			"mindspec_phase_semantics": "v2",
		},
	}
}

func task(id, status string) phase.ChildInfo {
	return phase.ChildInfo{ID: id, Status: status, IssueType: "task"}
}

func followUp(id, status string) phase.ChildInfo {
	return phase.ChildInfo{ID: id, Status: status, IssueType: "bug"}
}

// --- Row 1: spec create -> spec (three DISTINCT mechanisms) -----------
//
// Round-2 G2 (tally ruling): no self-authorized carve-out. R1.3/AC-2 name
// the status-aware derivation surface for every row; narrowing that to
// two surfaces was not this fix's call to make. This row is a genuine
// three-surface row again, using three call chains that share NO
// function call site with one another:
//
//   - state show:  resolve.ResolveModeWithCache — a PER-SPEC lookup
//     (FindEpicBySpecID fails, then draftSpecStatusWithCache verifies
//     via a direct bd-health probe + git branch check + on-disk tree
//     check for THIS ONE specID).
//   - tail:        phase.ResolveContextFromDirWithCache's WorktreeSpec
//     branch — an unconditional PATH-CONVENTION shortcut (no epic found
//     -> ModeSpec) with no git/bd verification of its own. DISCLOSED
//     (round-1 G3.1, unchanged): this is the only surface this row
//     proxies rather than the verb's own real invocation, because the
//     verb's own emitInstruct (main-checkout WorktreeMain) is not yet
//     draft-aware at this commit and still renders idle until bead 5's
//     AC-3.
//   - derivation:  resolve.DiscoverDraftSpecsWithCache — bead 3's own
//     ENUMERATE-ALL discovery mechanism (internal/resolve/resolve.go),
//     invoked with a fresh, independent Cache that shares no variable
//     with the tail's ctx above. This is the closest "status-aware"
//     analog for a no-epic state that phase.DerivePhaseWithStatus itself
//     cannot express (there is no epic to feed it): it health-probes bd,
//     confirms the spec/<id> branch exists via git, and confirms the
//     spec directory's committed tree content before ever concluding
//     "draft". A confirmed draft always derives `spec` — DraftSpec
//     carries no separate mode field because there is no OTHER mode a
//     confirmed draft can be in.

func rowSpecCreate(t *testing.T, row agreementRow) {
	specID := "201-agreement-row-1"
	root := draftFixtureRoot(t, specID)
	wireEmptyFixture(t)

	// Surface 1 — state show.
	stateMode, err := resolve.ResolveModeWithCache(phase.NewCache(), root, specID)
	if err != nil {
		t.Fatalf("state show: %v", err)
	}
	if stateMode != row.stateShow {
		t.Errorf("state show = %q, want %q (agreementTableRows)", stateMode, row.stateShow)
	}

	// Surface 2 — tail (WorktreeSpec context-path proxy; see the comment
	// above and the guide for the disclosed scope).
	dir := specWorktreeDir(root, specID)
	tailCache := phase.NewCache()
	ctx, err := phase.ResolveContextFromDirWithCache(tailCache, root, dir)
	if err != nil {
		t.Fatalf("context resolution (tail): %v", err)
	}
	if ctx.Phase != row.tail {
		t.Errorf("context phase (tail input) = %q, want %q (agreementTableRows)", ctx.Phase, row.tail)
	}
	mc := &state.Focus{Mode: ctx.Phase, ActiveSpec: ctx.SpecID}
	ictx := instruct.BuildContextWithCache(tailCache, root, mc)
	if ictx.Mode != row.tail {
		t.Errorf("instruct context mode = %q, want %q (agreementTableRows)", ictx.Mode, row.tail)
	}
	if _, err := instruct.Render(ictx); err != nil {
		t.Errorf("instruct.Render (%s.md): %v", ictx.Mode, err)
	}

	// Surface 3 — derivation: a SEPARATE, independently-invoked call into
	// bead 3's status-aware draft-discovery mechanism, never sharing the
	// tail's Cache or ctx variable above.
	derivCache := phase.NewCache()
	drafts, err := resolve.DiscoverDraftSpecsWithCache(derivCache, root)
	if err != nil {
		t.Fatalf("DiscoverDraftSpecsWithCache (derivation): %v", err)
	}
	foundAsDraft := false
	for _, d := range drafts {
		if d.SpecID == specID {
			foundAsDraft = true
			break
		}
	}
	if !foundAsDraft {
		t.Fatalf("DiscoverDraftSpecsWithCache did not independently confirm %q as a draft (drafts=%+v)", specID, drafts)
	}
	if row.derivation != state.ModeSpec {
		t.Errorf("agreementTableRows[%q].derivation = %q, want %q (a status-aware-confirmed draft always derives spec)", row.id, row.derivation, state.ModeSpec)
	}
}

// --- Row 2: spec approve -> plan --------------------------------------
//
// Round-2 G1: stored is now the hardcoded state.ModePlan literal — what
// spec.go's own epic-creation write literally is, independent of
// row.derivation — never `row.derivation` itself. assertChildDerivation
// adds the stored-independent leg.

func rowSpecApprovePlan(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r2"
	specID := "202-agreement-row-2"
	// Epic exists, no children yet — the settled "Epic exists, no
	// children -> plan" row (ADR-0023.md).
	var children []phase.ChildInfo
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 202, "agreement-row-2")
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	dir := specWorktreeDir(root, specID)
	assertThreeSurfaces(t, row.id, root, dir, specID, epicID, row)
	assertChildDerivation(t, row.id, children, row)
}

// --- Row 3: plan approve -> plan (OQ-3 option 1, settled fact 1) ------
//
// Round-2 G1: stored is the hardcoded state.ModePlan literal — plan.go's
// OQ-3 option-1 write is itself a constant, not a computation — never
// `row.derivation`.

func rowPlanApprovePlan(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r3"
	specID := "203-agreement-row-3"
	children := []phase.ChildInfo{task("b3-1", "open"), task("b3-2", "open")}
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 203, "agreement-row-3")
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	dir := specWorktreeDir(root, specID)
	assertThreeSurfaces(t, row.id, root, dir, specID, epicID, row)
	assertChildDerivation(t, row.id, children, row)
}

// --- Row 4: next (claim) -> implement ----------------------------------
//
// Round-2 G1: stored is the hardcoded state.ModeImplement literal —
// advancePhaseOnClaim only ever writes this constant, conditioned on a
// fresh re-derivation of these SAME (independently-fixed) children
// already saying implement — never `row.derivation`.

func rowNextClaimImplement(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r4"
	specID := "204-agreement-row-4"
	claimedBead := "mindspec-b4r4bead"
	children := []phase.ChildInfo{task(claimedBead, "in_progress"), task("b4-2", "open")}
	epic := v2Epic(epicID, specID, "open", state.ModeImplement, 204, "agreement-row-4")
	wireEpicFixture(t, epic, children, map[string]phase.EpicInfo{
		claimedBead: {Title: "[204-agreement-row-4] claimed bead"},
	})

	root := t.TempDir()
	dir := beadWorktreeDir(root, claimedBead)
	assertThreeSurfaces(t, row.id, root, dir, specID, epicID, row)
	assertChildDerivation(t, row.id, children, row)
}

// --- Row 5: complete -> implement (mid-spec, between beads) -----------
//
// Round-1 G4: includes a `blocked` sibling (neither closed nor
// in_progress, derive.go's amended row) alongside the closed+open pair,
// so the guide's "a blocked sibling counts the same way" note and R1.3's
// "remaining beads blocked" trigger are fixture-backed, not just
// code-verified. Round-2 G1: stored is the hardcoded state.ModeImplement
// literal — completeSyncPhase's own write IS
// phase.DerivePhaseFromChildren(children) for this shape, so hardcoding
// the value that function currently produces for these
// independently-fixed children mirrors production exactly — never
// `row.derivation`.

func rowCompleteImplementMidSpec(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r5"
	specID := "205-agreement-row-5"
	children := []phase.ChildInfo{
		task("b5-1", "closed"),
		task("b5-2", "open"),
		task("b5-3", "blocked"),
	}
	epic := v2Epic(epicID, specID, "open", state.ModeImplement, 205, "agreement-row-5")
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, row.id, root, root, specID, epicID, row)
	assertChildDerivation(t, row.id, children, row)
}

// --- Row 6: complete -> plan (empty lifecycle subset, derive.go:366) --

func rowCompletePlanEmptyLifecycleSubset(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r6"
	specID := "206-agreement-row-6"
	// Only a non-lifecycle follow-up remains (just closed by `complete`) —
	// zero lifecycle children: the empty-lifecycle-subset fallback, never
	// review (the vacuous-review guard, ADR-0023 §3 amendment).
	children := []phase.ChildInfo{followUp("bug-6", "closed")}
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 206, "agreement-row-6")
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, row.id, root, root, specID, epicID, row)
	assertChildDerivation(t, row.id, children, row)
}

// --- Row 7: complete -> plan (all-open via non-lifecycle child, :387) -

func rowCompletePlanAllOpenNonLifecycleChild(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r7"
	specID := "207-agreement-row-7"
	// A non-lifecycle follow-up just closed by `complete`, while every
	// lifecycle task remains open and none was ever claimed — the round-6
	// R3 / OQ-3-interlock trigger-2 shape (plan.md "(a) round-6 R3"):
	// stored is already plan, so complete's write here is plan-over-plan,
	// never backward.
	children := []phase.ChildInfo{followUp("bug-7", "closed"), task("b7-1", "open"), task("b7-2", "open")}
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 207, "agreement-row-7")
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, row.id, root, root, specID, epicID, row)
	assertChildDerivation(t, row.id, children, row)

	if phase.IsBackwardMove(state.ModePlan, state.ModePlan) {
		t.Error("plan -> plan must never classify as a backward move (round-6 R3 discharge)")
	}
}

// --- Row 8: complete -> review (last bead) -----------------------------

func rowCompleteReviewLastBead(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r8"
	specID := "208-agreement-row-8"
	children := []phase.ChildInfo{task("b8-1", "closed"), task("b8-2", "closed")}
	epic := v2Epic(epicID, specID, "open", state.ModeReview, 208, "agreement-row-8")
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, row.id, root, root, specID, epicID, row)
	assertChildDerivation(t, row.id, children, row)
}

// --- Row 9: release -> implement (sibling already closed) -------------
//
// Drives the REAL phase.SanctionedDerivedSync — release's sanctioned
// redesign-leg writer (OQ-3 option 1) — rather than a hand-authored
// post-state, so a regression in that production function is caught
// here too.

func rowReleaseImplementSiblingClosed(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r9"
	specID := "209-agreement-row-9"
	// Stored starts deliberately mismatched (review) to prove the sync
	// unconditionally overwrites with fresh child-derived evidence.
	preEpic := v2Epic(epicID, specID, "open", state.ModeReview, 209, "agreement-row-9")
	children := []phase.ChildInfo{task("b9-1", "closed"), task("b9-2", "open")}
	wireEpicFixture(t, preEpic, children, nil)

	root := t.TempDir()
	var written map[string]interface{}
	restoreMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		if issueID != epicID {
			t.Fatalf("SanctionedDerivedSync wrote to %q, want %q", issueID, epicID)
		}
		written = updates
		return nil
	})

	if err := phase.SanctionedDerivedSync(root, epicID); err != nil {
		t.Fatalf("SanctionedDerivedSync: %v", err)
	}
	if written["mindspec_phase"] != row.derivation {
		t.Fatalf("SanctionedDerivedSync wrote mindspec_phase=%v, want %q (agreementTableRows)", written["mindspec_phase"], row.derivation)
	}
	restoreMerge()

	// The state AFTER that sync has landed: two-surface agreement, no
	// tail (release emits none).
	postEpic := v2Epic(epicID, specID, "open", row.derivation, 209, "agreement-row-9")
	wireEpicFixture(t, postEpic, children, nil)
	assertTwoSurfaces(t, row.id, root, specID, epicID, row)
}

// --- Row 10: release -> plan (no sibling closed) -----------------------

func rowReleasePlanNoSiblingClosed(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r10"
	specID := "210-agreement-row-10"
	preEpic := v2Epic(epicID, specID, "open", state.ModeImplement, 210, "agreement-row-10")
	children := []phase.ChildInfo{task("b10-1", "open")}
	wireEpicFixture(t, preEpic, children, nil)

	root := t.TempDir()
	var written map[string]interface{}
	restoreMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		written = updates
		return nil
	})

	if err := phase.SanctionedDerivedSync(root, epicID); err != nil {
		t.Fatalf("SanctionedDerivedSync: %v", err)
	}
	if written["mindspec_phase"] != row.derivation {
		t.Fatalf("SanctionedDerivedSync wrote mindspec_phase=%v, want %q (agreementTableRows)", written["mindspec_phase"], row.derivation)
	}
	restoreMerge()

	postEpic := v2Epic(epicID, specID, "open", row.derivation, 210, "agreement-row-10")
	wireEpicFixture(t, postEpic, children, nil)
	assertTwoSurfaces(t, row.id, root, specID, epicID, row)
}

// --- Row 11: repair phase -> plan (v2-reconciliation, already consistent)
//
// Round-1 G6: repair phase was entirely absent from the table. This row
// exercises the REAL phase.EnsurePhaseSemanticsV2 legacy-rewrite leg
// (repairPhaseRunE's own first step, cmd/mindspec/repair.go:240-246)
// against a legacy (marker-absent, stored `implement`, all-open) epic —
// the licensed marker-absent exception (hazards discipline): this row's
// whole purpose IS exercising that legacy shape. After the rewrite,
// stored ("plan") and children-derived ("plan") agree, matching
// repair.go's "already consistent" success branch (freshStored ==
// freshDerived). repairPhaseRunE itself lives in cmd/mindspec (package
// main) and is unreachable from this external phase_test package, so
// the post-repair READ surfaces (repair's own phase.DerivePhaseDetailWithCache,
// plus the two AC-2-mandated surfaces) are asserted against the fixture
// state the real reconciliation call just produced.

func rowRepairPhaseV2ReconciliationConsistent(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r11"
	specID := "211-agreement-row-11"
	legacy := phase.EpicInfo{
		ID:        epicID,
		Title:     fmt.Sprintf("[SPEC %s] Agreement table row", specID),
		Status:    "open",
		IssueType: "epic",
		Metadata: map[string]interface{}{
			"mindspec_phase": state.ModeImplement,
			"spec_num":       float64(211),
			"spec_title":     "agreement-row-11",
			// mindspec_phase_semantics deliberately ABSENT — this is the
			// licensed legacy-fixture exception (hazards discipline):
			// this row exists specifically to exercise
			// EnsurePhaseSemanticsV2's one-shot rewrite leg.
		},
	}
	children := []phase.ChildInfo{task("b11-1", "open"), task("b11-2", "open")}
	wireEpicFixture(t, legacy, children, nil)

	root := t.TempDir()
	var written map[string]interface{}
	restoreMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		if issueID != epicID {
			t.Fatalf("EnsurePhaseSemanticsV2 wrote to %q, want %q", issueID, epicID)
		}
		written = updates
		return nil
	})
	rewrote, err := phase.EnsurePhaseSemanticsV2(root, epicID)
	if err != nil {
		t.Fatalf("EnsurePhaseSemanticsV2: %v", err)
	}
	restoreMerge()
	if !rewrote {
		t.Fatalf("EnsurePhaseSemanticsV2: expected the legacy rewrite leg to fire (stored=implement, all lifecycle children open, marker absent)")
	}
	if written["mindspec_phase"] != row.derivation {
		t.Fatalf("EnsurePhaseSemanticsV2 wrote mindspec_phase=%v, want %q (agreementTableRows)", written["mindspec_phase"], row.derivation)
	}
	if written[phase.PhaseSemanticsV2MarkerKey] != phase.PhaseSemanticsV2Value {
		t.Fatalf("EnsurePhaseSemanticsV2 wrote %s=%v, want %q", phase.PhaseSemanticsV2MarkerKey, written[phase.PhaseSemanticsV2MarkerKey], phase.PhaseSemanticsV2Value)
	}

	// Post-reconciliation state: stored is now `plan` + the v2 marker is
	// present; children are unchanged (still all-open), so repair's own
	// derivation surface agrees with the freshly-written stored value —
	// repair.go's "already consistent" (freshStored == freshDerived)
	// success branch.
	postEpic := v2Epic(epicID, specID, "open", row.derivation, 211, "agreement-row-11")
	wireEpicFixture(t, postEpic, children, nil)

	detail, err := phase.DerivePhaseDetailWithCache(phase.NewCache(), epicID)
	if err != nil {
		t.Fatalf("DerivePhaseDetailWithCache: %v", err)
	}
	if detail.Stored != row.derivation || detail.Derived != row.derivation {
		t.Errorf("repair's own derivation surface (PhaseDetail): stored=%q derived=%q, want both %q (already-consistent no-op)", detail.Stored, detail.Derived, row.derivation)
	}

	assertTwoSurfaces(t, row.id, root, specID, epicID, row)
}

// --- Row 12: repair phase -> implement (forced backward) --------------
//
// A bead was reopened after review (an operator or out-of-band bd edit) —
// the genuine backward-disagreement shape repair phase's guard exists
// for. repairPhaseRunE is unreachable from this package (see row 11's
// comment), so this fixtures the POST-repair state a real
// `mindspec repair phase <spec-id> --force-backward "<reason>"` run
// leaves (repair.go's wroteWithForce branch: writes the derived value
// plus mindspec_phase_force_reason, unconditionally, once forced) — but
// FIRST proves, against the real comparator, that the premise is
// genuinely backward, so this row cannot silently degenerate into a
// forward-move fixture.

func rowRepairPhaseForcedBackward(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r12"
	specID := "212-agreement-row-12"
	preStored := state.ModeReview
	children := []phase.ChildInfo{task("b12-1", "closed"), task("b12-2", "open")}

	freshDerived := phase.DerivePhaseFromChildren(children)
	if freshDerived != row.derivation {
		t.Fatalf("test setup: DerivePhaseFromChildren(children) = %q, want %q (fixture no longer matches agreementTableRows)", freshDerived, row.derivation)
	}
	if !phase.IsBackwardMove(preStored, freshDerived) {
		t.Fatalf("test setup: IsBackwardMove(%q, %q) = false, want true — this row's entire premise is a genuine backward disagreement", preStored, freshDerived)
	}

	postEpic := phase.EpicInfo{
		ID:        epicID,
		Title:     fmt.Sprintf("[SPEC %s] Agreement table row", specID),
		Status:    "open",
		IssueType: "epic",
		Metadata: map[string]interface{}{
			"mindspec_phase":              row.derivation,
			"mindspec_phase_force_reason": "agreement-table fixture: forced backward repair",
			"spec_num":                    float64(212),
			"spec_title":                  "agreement-row-12",
			"mindspec_phase_semantics":    "v2",
		},
	}
	wireEpicFixture(t, postEpic, children, nil)

	root := t.TempDir()
	assertTwoSurfaces(t, row.id, root, specID, epicID, row)
}

// --- Row 13: impl approve / impl adopt -> done / idle (terminal) ------
//
// Two facts at two layers (R1.3's terminal-rows carve-out): the closed
// epic with its done marker derives "done" through the status-aware
// surface directly; state show/the tail report "idle" via a GLOBAL
// zero-open-epic discovery — the epic is CLOSED and therefore excluded
// from every active-epic query, so "idle" here is not a per-epic
// derivation at all.
//
// Round-1 R2 nit fix: the state-show assertion no longer compares a
// hardcoded literal to itself. It asserts every precondition
// cmd/mindspec/state.go's RunE checks before reaching its literal,
// argument-free idle branch (a non-ambiguous resolution failure, a
// healthy bd, and zero on-disk drafts), then checks row.stateShow
// against the one value that branch can produce — so editing the doc's
// terminal-row State-show cell to anything but "idle" now fails here.
//
// Round-2 G1 (consistency with the other rows, though this row was not
// among the panel's enumerated seven): stored is the hardcoded
// state.ModeDone literal — impl approve/adopt's own write is itself the
// constant ModeDone, not a computation — never `row.derivation`. There
// is no children-only leg to add here (bare DerivePhaseFromChildren
// cannot express "done" for an all-closed set — that is exactly why
// R1.3 requires the status-aware surface for this row in the first
// place), so this row's independence rests on the literal decoupling.

func rowImplApproveDoneIdleTerminal(t *testing.T, row agreementRow) {
	epicID := "mindspec-b4r13"
	specID := "213-agreement-row-13"
	epic := v2Epic(epicID, specID, "closed", state.ModeDone, 213, "agreement-row-13")
	children := []phase.ChildInfo{task("b13-1", "closed"), task("b13-2", "closed")}
	wireEpicFixture(t, epic, children, nil)

	// Fact 1: the derivation surface, directly against the (closed)
	// epic ID.
	derived, err := phase.DerivePhaseWithCache(phase.NewCache(), epicID)
	if err != nil {
		t.Fatalf("derivation: %v", err)
	}
	if derived != row.derivation {
		t.Errorf("derivation (closed epic + done marker) = %q, want %q (agreementTableRows)", derived, row.derivation)
	}

	// Fact 2: state show / tail via GLOBAL zero-open-epic discovery. A
	// real git repo is required — resolve.DiscoverDraftSpecsWithCache
	// (state.go's ambient fallback) lists local branches and propagates a
	// genuine non-repo failure rather than reading it as "not a draft".
	root := bareGitRoot(t)

	// Mirrors cmd/mindspec/state.go's own ambient (no --spec) flow, whose
	// genuinely-idle branch (state.go:149) is a literal `Mode:
	// state.ModeIdle` with no further computation to call — these are
	// the preconditions RunE checks before reaching it.
	c := phase.NewCache()
	_, targetErr := resolve.ResolveTargetWithCache(c, root, "")
	if targetErr == nil {
		t.Fatalf("ResolveTargetWithCache: expected a \"no active specs\" error with a closed-only epic, got none")
	}
	if _, ambiguous := targetErr.(*resolve.ErrAmbiguousTarget); ambiguous {
		t.Fatalf("ResolveTargetWithCache: unexpectedly ambiguous: %v", targetErr)
	}
	// state.go's round-1 G2 health probe: confirm bd itself is healthy
	// before trusting "nothing active" (a transient bd flake must never
	// be misread as idle).
	if _, err := c.AllEpics(); err != nil {
		t.Fatalf("AllEpics (bd health probe): %v", err)
	}
	drafts, err := resolve.DiscoverDraftSpecsWithCache(c, root)
	if err != nil {
		t.Fatalf("DiscoverDraftSpecsWithCache: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("DiscoverDraftSpecsWithCache: got %d drafts, want 0", len(drafts))
	}
	// Every precondition state.go's RunE checks before its literal idle
	// branch now holds; that branch can only ever produce ModeIdle, so
	// this is the one place row.stateShow is checked against a fact
	// rather than a call.
	if row.stateShow != state.ModeIdle {
		t.Errorf("agreementTableRows terminal row stateShow = %q, want %q (state.go's genuinely-idle branch is the only reachable outcome once these preconditions hold)", row.stateShow, state.ModeIdle)
	}

	// The tail: WorktreeMain's zero-open-epic discovery independently
	// arrives at the same idle fact via a DIFFERENT call path
	// (phase.ResolveContextFromDirWithCache, not resolve.ResolveTargetWithCache).
	ctx, err := phase.ResolveContextFromDirWithCache(phase.NewCache(), root, root)
	if err != nil {
		t.Fatalf("context resolution: %v", err)
	}
	if ctx.Phase != row.tail {
		t.Errorf("context phase (tail input, zero-open-epic discovery) = %q, want %q (agreementTableRows)", ctx.Phase, row.tail)
	}

	mc := &state.Focus{Mode: ctx.Phase}
	ictx := instruct.BuildContextWithCache(phase.NewCache(), root, mc)
	if ictx.Mode != row.tail {
		t.Errorf("instruct context mode = %q, want %q (agreementTableRows)", ictx.Mode, row.tail)
	}
	if _, err := instruct.Render(ictx); err != nil {
		t.Errorf("instruct.Render (idle.md): %v", err)
	}
}

// --- The fixture registry + row runner ---------------------------------

// rowFixtures maps every agreementTableRows identifier to its executable
// fixture. TestAgreementTable_Rows fails loudly for any row with no
// entry here — adding a row identifier (even with a correct doc cell set)
// can never satisfy the gate without an executable assertion (round-1
// G1/G6).
var rowFixtures = map[string]func(t *testing.T, row agreementRow){
	"spec create → spec":                                          rowSpecCreate,
	"spec approve → plan":                                         rowSpecApprovePlan,
	"plan approve → plan":                                         rowPlanApprovePlan,
	"next (claim) → implement":                                    rowNextClaimImplement,
	"complete → implement (mid-spec)":                             rowCompleteImplementMidSpec,
	"complete → plan (empty lifecycle subset)":                    rowCompletePlanEmptyLifecycleSubset,
	"complete → plan (all-open, non-lifecycle child completed)":   rowCompletePlanAllOpenNonLifecycleChild,
	"complete → review (last bead)":                               rowCompleteReviewLastBead,
	"release → implement (sibling closed)":                        rowReleaseImplementSiblingClosed,
	"release → plan (no sibling closed)":                          rowReleasePlanNoSiblingClosed,
	"repair phase → plan (v2-reconciliation, already consistent)": rowRepairPhaseV2ReconciliationConsistent,
	"repair phase → implement (forced backward)":                  rowRepairPhaseForcedBackward,
	"impl approve / impl adopt → done / idle (terminal)":          rowImplApproveDoneIdleTerminal,
}

func TestAgreementTable_Rows(t *testing.T) {
	for _, row := range agreementTableRows {
		row := row
		t.Run(row.id, func(t *testing.T) {
			fn, ok := rowFixtures[row.id]
			if !ok {
				t.Fatalf("agreementTableRows contains %q with no registered fixture in rowFixtures — a row identifier alone (even with correct cells) must not satisfy the gate (round-1 panel G1/G6)", row.id)
			}
			fn(t, row)
		})
	}
}

// --- Table/fixture cross-pin -------------------------------------------

// TestAgreementTable_DocRowsCrossPinFixtures parses
// project-docs/user/guides/lifecycle-modes.md's agreement table cell by
// cell (Row, State show, Instruct tail, Derivation) and asserts the
// parsed rows are byte-identical, as a set, to agreementTableRows — a
// row added/removed/renamed, OR a single cell edited on either side
// (round-1 G1's structural fix), is red. It also asserts every row has a
// registered rowFixtures entry, closing the "row identifier without an
// executable assertion" gap (round-1 G6/R6).
func TestAgreementTable_DocRowsCrossPinFixtures(t *testing.T) {
	root := repoRootForTest(t)
	docPath := filepath.Join(root, "project-docs", "user", "guides", "lifecycle-modes.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("reading %s: %v", docPath, err)
	}

	docRows := parseAgreementTableRows(string(data))
	if len(docRows) == 0 {
		t.Fatalf("parsed zero rows from %s's agreement table — table markers missing or table empty", docPath)
	}

	want := map[string]agreementRow{}
	for _, r := range agreementTableRows {
		want[r.id] = r
	}
	got := map[string]agreementRow{}
	for _, r := range docRows {
		got[r.id] = r
	}

	for id, wantRow := range want {
		gotRow, ok := got[id]
		if !ok {
			t.Errorf("row %q is in agreementTableRows but missing from the doc table — the doc was edited without this fixture list (or the fixture list drifted ahead of the doc)", id)
			continue
		}
		if gotRow != wantRow {
			t.Errorf("row %q cells differ between the doc table and agreementTableRows:\n  doc:      %+v\n  fixture:  %+v", id, gotRow, wantRow)
		}
	}
	for id, gotRow := range got {
		if _, ok := want[id]; !ok {
			t.Errorf("row %q (%+v) is in the doc table but missing from agreementTableRows — add its fixture and register it in rowFixtures", id, gotRow)
		}
	}

	// Round-1 G6/R6: a row identifier (even with matching cells) must
	// still have an executable fixture — this closes that loop
	// redundantly with TestAgreementTable_Rows's own map-lookup failure.
	for id := range want {
		if _, ok := rowFixtures[id]; !ok {
			t.Errorf("row %q has no registered fixture in rowFixtures — adding a row identifier without an executable assertion must fail", id)
		}
	}
}

// repoRootForTest walks up from the test's working directory (the package
// directory under `go test`) to the nearest go.mod, independent of
// invocation cwd.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root (go.mod) walking up from %s", dir)
		}
		dir = parent
	}
}

// parseAgreementTableRows extracts the Row / State show / Instruct tail /
// Derivation cells of every data row of the markdown pipe-table between
// the "<!-- agreement-table:rows -->" / "<!-- /agreement-table:rows -->"
// marker comments in the guide. The fifth (Notes) column, if present, is
// intentionally not parsed — it carries prose, not a contracted value.
func parseAgreementTableRows(doc string) []agreementRow {
	const startMarker = "<!-- agreement-table:rows -->"
	const endMarker = "<!-- /agreement-table:rows -->"
	start := strings.Index(doc, startMarker)
	if start < 0 {
		return nil
	}
	end := strings.Index(doc[start:], endMarker)
	if end < 0 {
		return nil
	}
	section := doc[start+len(startMarker) : start+end]

	var rows []agreementRow
	seenSeparator := false
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 4 {
			continue
		}
		first := strings.TrimSpace(cells[0])
		// The header separator row ("---" / ":---" cells) never contains
		// a real row identifier; skip exactly one such row (the
		// separator immediately following the header) and treat every
		// following row as data.
		if strings.Trim(first, "-: ") == "" {
			seenSeparator = true
			continue
		}
		if !seenSeparator {
			// Header row itself (e.g. "| Row | State show | ... |").
			continue
		}
		id := trimCell(cells[0])
		if id == "" {
			continue
		}
		rows = append(rows, agreementRow{
			id:         id,
			stateShow:  trimCell(cells[1]),
			tail:       trimCell(cells[2]),
			derivation: trimCell(cells[3]),
		})
	}
	return rows
}

// trimCell trims whitespace and, if present, a wrapping backtick pair
// from one markdown table cell's raw text.
func trimCell(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	return strings.TrimSpace(s)
}
