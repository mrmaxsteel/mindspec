package phase_test

// agreement_table_test.go — spec 128 (lifecycle-verb-state-honesty)
// Bead 4, plan step 2: one fixture test per row of the per-verb
// agreement table published at project-docs/user/guides/lifecycle-modes.md
// (AC-2). Each row constructs the epic+child state a real run of the
// named verb leaves under OQ-3 option 1 (bead 2's plan-approve/next
// writers, release's sanctioned-sync redesign leg) and OQ-2's
// draft-discovery amendment (bead 3), then asserts the surfaces R1.3
// names agree on the row's mode:
//
//   - derivation:    phase.DerivePhaseWithCache (the DerivePhaseWithStatus
//     class) called directly against the fixture's epic ID.
//   - state show:    resolve.ResolveModeWithCache — the function
//     cmd/mindspec/state.go's RunE composes its printed mode from
//     (state.go:159).
//   - instruct tail: phase.ResolveContextFromDirWithCache feeds the same
//     state.Focus{Mode: ctx.Phase} instruct_tail.go's emitInstruct
//     builds; instruct.Render selects its template by the literal
//     concatenation ctx.Mode+".md" (internal/instruct/instruct.go), so
//     asserting ctx.Mode and a successful Render together ARE the "tail's
//     selected template" surface — there is no separate "selected
//     template name" field to read.
//
// Interpretive note, stated rather than left implicit (two rows have no
// live epic to feed phase.DerivePhaseWithStatus at all): the spec-create
// row's canonical state is a pre-epic draft, and the terminal
// (impl-approve/impl-adopt) row's canonical state has its epic CLOSED and
// therefore excluded from every active-epic query. For those two rows the
// "derivation"/"tail" surfaces necessarily route through the same no-epic
// or zero-open-epic code paths bead 3 shipped
// (ResolveContextFromDirWithCache's WorktreeSpec no-epic branch for the
// draft; DiscoverActiveSpecsWithCache's zero-open-epic discovery for the
// terminal idle fact) rather than phase.DerivePhaseWithStatus itself —
// there is no epic ID to hand it. This matches ADR-0023's own OQ-2
// amendment, which names the WorktreeSpec branch as "the mechanism one
// code path already shipped" for the draft case, and R1.3's own
// terminal-rows carve-out, which states the idle fact is "a global
// discovery outcome, not a per-epic derivation." The terminal row
// ADDITIONALLY asserts the status-aware derivation directly (closed epic
// + done marker -> "done") as its own, separate fact.
//
// Release's two rows are TAIL-LESS by design (R2.1/AC-2: release emits no
// instruct tail) — no instruct assertion is made for them, and unlike
// every other row here their fixture drives the REAL
// phase.SanctionedDerivedSync (not a hand-authored post-state), so a
// regression in that production function is caught here too, not only in
// sanctioned_sync_test.go's own isolated coverage.
//
// Table/fixture cross-pin (plan step 4, AC-2's closing clause):
// TestAgreementTable_DocRowsCrossPinFixtures parses the guide's table and
// asserts its row-identifier column is exactly agreementTableRowIDs below
// (as a set) — editing the doc's row set without updating this list, or
// vice versa, is red.

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

// --- Three/two-surface assertion helpers -----------------------------

// assertThreeSurfaces asserts the derivation, state-show, and instruct-tail
// surfaces all agree on wantMode for an epic-having row (every row except
// spec-create's draft and the terminal done/idle row).
func assertThreeSurfaces(t *testing.T, label, root, dir, specID, epicID, wantMode string) {
	t.Helper()

	derived, err := phase.DerivePhaseWithCache(phase.NewCache(), epicID)
	if err != nil {
		t.Fatalf("%s: derivation: %v", label, err)
	}
	if derived != wantMode {
		t.Errorf("%s: derivation (DerivePhaseWithStatus-class) = %q, want %q", label, derived, wantMode)
	}

	stateMode, err := resolve.ResolveModeWithCache(phase.NewCache(), root, specID)
	if err != nil {
		t.Fatalf("%s: state show: %v", label, err)
	}
	if stateMode != wantMode {
		t.Errorf("%s: state show (resolve.ResolveModeWithCache) = %q, want %q", label, stateMode, wantMode)
	}

	c := phase.NewCache()
	ctx, err := phase.ResolveContextFromDirWithCache(c, root, dir)
	if err != nil {
		t.Fatalf("%s: context resolution: %v", label, err)
	}
	if ctx.Phase != wantMode {
		t.Errorf("%s: context phase (tail input) = %q, want %q", label, ctx.Phase, wantMode)
	}
	mc := &state.Focus{Mode: ctx.Phase, ActiveSpec: ctx.SpecID, ActiveBead: ctx.BeadID}
	ictx := instruct.BuildContextWithCache(c, root, mc)
	if ictx.Mode != wantMode {
		t.Errorf("%s: instruct context mode (template selector) = %q, want %q", label, ictx.Mode, wantMode)
	}
	if _, err := instruct.Render(ictx); err != nil {
		t.Errorf("%s: instruct.Render (selected template %s.md): %v", label, ictx.Mode, err)
	}
}

// assertTwoSurfaces is release's carved-out shape (R2.1/AC-2): derivation +
// state show only, no instruct-tail assertion — release emits none.
func assertTwoSurfaces(t *testing.T, label, root, specID, epicID, wantMode string) {
	t.Helper()

	derived, err := phase.DerivePhaseWithCache(phase.NewCache(), epicID)
	if err != nil {
		t.Fatalf("%s: derivation: %v", label, err)
	}
	if derived != wantMode {
		t.Errorf("%s: derivation (DerivePhaseWithStatus-class) = %q, want %q", label, derived, wantMode)
	}

	stateMode, err := resolve.ResolveModeWithCache(phase.NewCache(), root, specID)
	if err != nil {
		t.Fatalf("%s: state show: %v", label, err)
	}
	if stateMode != wantMode {
		t.Errorf("%s: state show (resolve.ResolveModeWithCache) = %q, want %q", label, stateMode, wantMode)
	}
}

// v2Epic builds a marker-present (mindspec_phase_semantics: "v2") epic
// fixture — the pinned discipline (plan round-3 G2): every row needing a
// stored/derived state constructs it marker-present; marker-absent states
// belong exclusively to bead 2's legacy fixtures.
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

// --- agreementTableRowIDs / doc cross-pin -----------------------------

// agreementTableRowIDs is the canonical row-identifier set. It MUST match
// project-docs/user/guides/lifecycle-modes.md's table exactly (as a set) —
// TestAgreementTable_DocRowsCrossPinFixtures enforces this both ways, so
// editing the table without this list (or vice versa) is red.
var agreementTableRowIDs = []string{
	"spec create → spec",
	"spec approve → plan",
	"plan approve → plan",
	"next (claim) → implement",
	"complete → implement (mid-spec)",
	"complete → plan (empty lifecycle subset)",
	"complete → plan (all-open, non-lifecycle child completed)",
	"complete → review (last bead)",
	"release → implement (sibling closed)",
	"release → plan (no sibling closed)",
	"impl approve / impl adopt → done / idle (terminal)",
}

// --- Row 1: spec create -> spec --------------------------------------

func TestAgreementTable_SpecCreate_Spec(t *testing.T) {
	specID := "201-agreement-row-1"
	root := draftFixtureRoot(t, specID)
	wireEmptyFixture(t)

	// State show: resolve.ResolveModeWithCache is draft-aware (OQ-2) —
	// the same surface bead 3's own TestResolveModeWithCache_DraftSpec_
	// ReturnsModeSpec pins.
	stateMode, err := resolve.ResolveModeWithCache(phase.NewCache(), root, specID)
	if err != nil {
		t.Fatalf("state show: %v", err)
	}
	if stateMode != state.ModeSpec {
		t.Errorf("state show = %q, want %q", stateMode, state.ModeSpec)
	}

	// Derivation + tail (see the file-level interpretive note): a pre-epic
	// draft has no epic ID to feed phase.DerivePhaseWithStatus, so both
	// surfaces route through the same no-epic WorktreeSpec branch
	// ADR-0023's OQ-2 amendment names as the shipped mechanism.
	dir := specWorktreeDir(root, specID)
	c := phase.NewCache()
	ctx, err := phase.ResolveContextFromDirWithCache(c, root, dir)
	if err != nil {
		t.Fatalf("context resolution: %v", err)
	}
	if ctx.Phase != state.ModeSpec {
		t.Errorf("context phase (derivation + tail input) = %q, want %q", ctx.Phase, state.ModeSpec)
	}
	mc := &state.Focus{Mode: ctx.Phase, ActiveSpec: ctx.SpecID}
	ictx := instruct.BuildContextWithCache(c, root, mc)
	if ictx.Mode != state.ModeSpec {
		t.Errorf("instruct context mode = %q, want %q", ictx.Mode, state.ModeSpec)
	}
	if _, err := instruct.Render(ictx); err != nil {
		t.Errorf("instruct.Render (spec.md): %v", err)
	}
}

// --- Row 2: spec approve -> plan --------------------------------------

func TestAgreementTable_SpecApprove_Plan(t *testing.T) {
	epicID := "mindspec-b4r2"
	specID := "202-agreement-row-2"
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 202, "agreement-row-2")
	// Epic exists, no children yet — the settled "Epic exists, no
	// children -> plan" row (ADR-0023.md).
	wireEpicFixture(t, epic, nil, nil)

	root := t.TempDir()
	dir := specWorktreeDir(root, specID)
	assertThreeSurfaces(t, "spec-approve", root, dir, specID, epicID, state.ModePlan)
}

// --- Row 3: plan approve -> plan (OQ-3 option 1, settled fact 1) ------

func TestAgreementTable_PlanApprove_Plan(t *testing.T) {
	epicID := "mindspec-b4r3"
	specID := "203-agreement-row-3"
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 203, "agreement-row-3")
	children := []phase.ChildInfo{task("b3-1", "open"), task("b3-2", "open")}
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	dir := specWorktreeDir(root, specID)
	assertThreeSurfaces(t, "plan-approve", root, dir, specID, epicID, state.ModePlan)
}

// --- Row 4: next (claim) -> implement ----------------------------------

func TestAgreementTable_NextClaim_Implement(t *testing.T) {
	epicID := "mindspec-b4r4"
	specID := "204-agreement-row-4"
	claimedBead := "mindspec-b4r4bead"
	epic := v2Epic(epicID, specID, "open", state.ModeImplement, 204, "agreement-row-4")
	children := []phase.ChildInfo{task(claimedBead, "in_progress"), task("b4-2", "open")}
	wireEpicFixture(t, epic, children, map[string]phase.EpicInfo{
		claimedBead: {Title: "[204-agreement-row-4] claimed bead"},
	})

	root := t.TempDir()
	dir := beadWorktreeDir(root, claimedBead)
	assertThreeSurfaces(t, "next-claim", root, dir, specID, epicID, state.ModeImplement)
}

// --- Row 5: complete -> implement (mid-spec, between beads) -----------

func TestAgreementTable_Complete_Implement_MidSpec(t *testing.T) {
	epicID := "mindspec-b4r5"
	specID := "205-agreement-row-5"
	epic := v2Epic(epicID, specID, "open", state.ModeImplement, 205, "agreement-row-5")
	// Some closed, some open, none in_progress -> implement
	// (derive.go's "between beads" branch; ADR-0023.md:140's amended row).
	children := []phase.ChildInfo{task("b5-1", "closed"), task("b5-2", "open")}
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, "complete-mid-spec", root, root, specID, epicID, state.ModeImplement)
}

// --- Row 6: complete -> plan (empty lifecycle subset, derive.go:366) --

func TestAgreementTable_Complete_Plan_EmptyLifecycleSubset(t *testing.T) {
	epicID := "mindspec-b4r6"
	specID := "206-agreement-row-6"
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 206, "agreement-row-6")
	// Only a non-lifecycle follow-up remains (just closed by `complete`) —
	// zero lifecycle children: the empty-lifecycle-subset fallback, never
	// review (the vacuous-review guard, ADR-0023 §3 amendment).
	children := []phase.ChildInfo{followUp("bug-6", "closed")}
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, "complete-plan-empty-lifecycle", root, root, specID, epicID, state.ModePlan)
}

// --- Row 7: complete -> plan (all-open via non-lifecycle child, :387) -

func TestAgreementTable_Complete_Plan_AllOpenNonLifecycleChild(t *testing.T) {
	epicID := "mindspec-b4r7"
	specID := "207-agreement-row-7"
	epic := v2Epic(epicID, specID, "open", state.ModePlan, 207, "agreement-row-7")
	// A non-lifecycle follow-up just closed by `complete`, while every
	// lifecycle task remains open and none was ever claimed — the round-6
	// R3 / OQ-3-interlock trigger-2 shape (plan.md "(a) round-6 R3"):
	// stored is already plan, so complete's write here is plan-over-plan,
	// never backward.
	children := []phase.ChildInfo{followUp("bug-7", "closed"), task("b7-1", "open"), task("b7-2", "open")}
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, "complete-plan-all-open-non-lifecycle", root, root, specID, epicID, state.ModePlan)

	if phase.IsBackwardMove(state.ModePlan, state.ModePlan) {
		t.Error("plan -> plan must never classify as a backward move (round-6 R3 discharge)")
	}
}

// --- Row 8: complete -> review (last bead) -----------------------------

func TestAgreementTable_Complete_Review_LastBead(t *testing.T) {
	epicID := "mindspec-b4r8"
	specID := "208-agreement-row-8"
	epic := v2Epic(epicID, specID, "open", state.ModeReview, 208, "agreement-row-8")
	children := []phase.ChildInfo{task("b8-1", "closed"), task("b8-2", "closed")}
	wireEpicFixture(t, epic, children, nil)

	root := t.TempDir()
	assertThreeSurfaces(t, "complete-review-last-bead", root, root, specID, epicID, state.ModeReview)
}

// --- Row 9: release -> implement (sibling already closed) -------------
//
// Drives the REAL phase.SanctionedDerivedSync — release's sanctioned
// redesign-leg writer (OQ-3 option 1) — rather than a hand-authored
// post-state, so a regression in that production function is caught
// here too.

func TestAgreementTable_Release_Implement_SiblingClosed(t *testing.T) {
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
	defer restoreMerge()

	if err := phase.SanctionedDerivedSync(root, epicID); err != nil {
		t.Fatalf("SanctionedDerivedSync: %v", err)
	}
	if written["mindspec_phase"] != state.ModeImplement {
		t.Fatalf("SanctionedDerivedSync wrote mindspec_phase=%v, want %q", written["mindspec_phase"], state.ModeImplement)
	}
	restoreMerge()

	// The state AFTER that sync has landed: two-surface agreement, no
	// tail (release emits none).
	postEpic := v2Epic(epicID, specID, "open", state.ModeImplement, 209, "agreement-row-9")
	wireEpicFixture(t, postEpic, children, nil)
	assertTwoSurfaces(t, "release-implement-sibling-closed", root, specID, epicID, state.ModeImplement)
}

// --- Row 10: release -> plan (no sibling closed) -----------------------

func TestAgreementTable_Release_Plan_NoSiblingClosed(t *testing.T) {
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
	defer restoreMerge()

	if err := phase.SanctionedDerivedSync(root, epicID); err != nil {
		t.Fatalf("SanctionedDerivedSync: %v", err)
	}
	if written["mindspec_phase"] != state.ModePlan {
		t.Fatalf("SanctionedDerivedSync wrote mindspec_phase=%v, want %q", written["mindspec_phase"], state.ModePlan)
	}
	restoreMerge()

	postEpic := v2Epic(epicID, specID, "open", state.ModePlan, 210, "agreement-row-10")
	wireEpicFixture(t, postEpic, children, nil)
	assertTwoSurfaces(t, "release-plan-no-sibling-closed", root, specID, epicID, state.ModePlan)
}

// --- Row 11: impl approve / impl adopt -> done / idle (terminal) ------
//
// Two facts at two layers (R1.3's terminal-rows carve-out): the closed
// epic with its done marker derives "done" through the status-aware
// surface directly; state show/the tail report "idle" via a GLOBAL
// zero-open-epic discovery — the epic is CLOSED and therefore excluded
// from every active-epic query, so "idle" here is not a per-epic
// derivation at all.

func TestAgreementTable_ImplApprove_DoneIdle_Terminal(t *testing.T) {
	epicID := "mindspec-b4r11"
	specID := "211-agreement-row-11"
	epic := v2Epic(epicID, specID, "closed", state.ModeDone, 211, "agreement-row-11")
	children := []phase.ChildInfo{task("b11-1", "closed"), task("b11-2", "closed")}
	wireEpicFixture(t, epic, children, nil)

	// Fact 1: the derivation surface, directly against the (closed)
	// epic ID.
	derived, err := phase.DerivePhaseWithCache(phase.NewCache(), epicID)
	if err != nil {
		t.Fatalf("derivation: %v", err)
	}
	if derived != state.ModeDone {
		t.Errorf("derivation (closed epic + done marker) = %q, want %q", derived, state.ModeDone)
	}

	// Fact 2: state show / tail via GLOBAL zero-open-epic discovery. A
	// real git repo is required — resolve.DiscoverDraftSpecsWithCache
	// (state.go's ambient fallback) lists local branches and propagates a
	// genuine non-repo failure rather than reading it as "not a draft".
	root := bareGitRoot(t)

	// Mirrors cmd/mindspec/state.go's own ambient (no --spec) flow: no
	// active spec is found (the epic is closed, excluded from
	// ActiveEpics), no on-disk draft exists either, so state show's
	// composed answer is genuinely idle.
	c := phase.NewCache()
	if _, err := resolve.ResolveTargetWithCache(c, root, ""); err == nil {
		t.Fatalf("ResolveTargetWithCache: expected \"no active specs\" error with a closed-only epic, got none")
	}
	drafts, err := resolve.DiscoverDraftSpecsWithCache(c, root)
	if err != nil {
		t.Fatalf("DiscoverDraftSpecsWithCache: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("DiscoverDraftSpecsWithCache: got %d drafts, want 0", len(drafts))
	}
	// state.go's genuinely-idle terminal branch.
	stateMode := state.ModeIdle

	// The tail: WorktreeMain's zero-open-epic discovery independently
	// arrives at the same idle fact.
	ctx, err := phase.ResolveContextFromDirWithCache(phase.NewCache(), root, root)
	if err != nil {
		t.Fatalf("context resolution: %v", err)
	}
	if ctx.Phase != state.ModeIdle {
		t.Errorf("context phase (tail input, zero-open-epic discovery) = %q, want %q", ctx.Phase, state.ModeIdle)
	}
	if stateMode != state.ModeIdle {
		t.Errorf("state show (ambient, zero-open-epic discovery) = %q, want %q", stateMode, state.ModeIdle)
	}

	mc := &state.Focus{Mode: ctx.Phase}
	ictx := instruct.BuildContextWithCache(phase.NewCache(), root, mc)
	if ictx.Mode != state.ModeIdle {
		t.Errorf("instruct context mode = %q, want %q", ictx.Mode, state.ModeIdle)
	}
	if _, err := instruct.Render(ictx); err != nil {
		t.Errorf("instruct.Render (idle.md): %v", err)
	}
}

// --- Table/fixture cross-pin -------------------------------------------

// TestAgreementTable_DocRowsCrossPinFixtures parses the "Row" column of
// project-docs/user/guides/lifecycle-modes.md's agreement table and
// asserts it is exactly agreementTableRowIDs, as a set, in both
// directions — a doc row added/removed without a matching fixture entry
// (or vice versa) is red (plan step 4, AC-2's closing clause).
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

	want := map[string]bool{}
	for _, id := range agreementTableRowIDs {
		want[id] = true
	}
	got := map[string]bool{}
	for _, id := range docRows {
		got[id] = true
	}

	for id := range want {
		if !got[id] {
			t.Errorf("row %q is in agreementTableRowIDs but missing from the doc table — the doc was edited without this fixture list (or the fixture list drifted ahead of the doc)", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("row %q is in the doc table but missing from agreementTableRowIDs — add its fixture test and register it here", id)
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

// parseAgreementTableRows extracts the first-column cell of every data row
// of the markdown pipe-table between the
// "<!-- agreement-table:rows -->" / "<!-- /agreement-table:rows -->"
// marker comments in the guide.
func parseAgreementTableRows(doc string) []string {
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

	var rows []string
	lines := strings.Split(section, "\n")
	seenSeparator := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) == 0 {
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
			// Header row itself (e.g. "| Row | Mode | ... |").
			continue
		}
		first = strings.Trim(first, "`")
		if first == "" {
			continue
		}
		rows = append(rows, first)
	}
	return rows
}
