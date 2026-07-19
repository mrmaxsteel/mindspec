package approve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

var (
	implRunBDCombinedFn = bead.RunBDCombined
	implRunBDFn         = bead.RunBD
	implMergeMetadataFn = bead.MergeMetadata
	implGitUserEmailFn  = bead.GitUserEmail
	// implPhaseMetadataFn is the Spec 092 Bead 3 seam for the two
	// mindspec_phase writes inside ApproveImpl: the deferred stale-
	// phase reconcile (Req 1) and the MUTATION (2/3) done write. Both
	// are merge-writes (bead.MergeMetadata) so unrelated metadata keys
	// — mindspec_migrated_at, doc-skew audit keys, ADR-override keys —
	// are preserved (Req 19).
	implPhaseMetadataFn = bead.MergeMetadata
	// implCreateWithIDFn is the Spec 087 Bead 3 seam for the
	// placeholder-ADR creation step in the supersede flow on the
	// backstop (`approve impl`) path.
	implCreateWithIDFn = adr.CreateWithID
	// implGetwdFn feeds the Req 8 worktree-context line on the phase
	// and plan-bead gate failures (spec 092, mindspec-tjat). Tests swap
	// it to pin the worktree kind regardless of where `go test` runs.
	implGetwdFn = os.Getwd

	// --- Spec 115 Bead 2: the pre-terminal orphan/obligation refusal
	// gate's seams. All default to the real functions so every existing
	// test — which never touches these vars — exercises production
	// behavior unchanged; the new gate's own tests override them.
	//
	// implScanOrphansFn is R1's error-preserving orphan scan (fail-
	// closed on the three cleanly-signaled infra legs: epic-lookup,
	// bd-list, ancestry). The branch-existence trigger inside it stays
	// the unchanged bool gitutil.BranchExists (round-6 C+B, via
	// lifecycle's own internal seam) — absent-or-probe-failure reads as
	// "no trigger", never a false refusal.
	implScanOrphansFn = lifecycle.ScanOrphanedClosedBeads
	// implClosedEpicBeadIDsFn and implWorktreeListFn back the round-7
	// Option B worktree-enumeration merge-prevention leg (AC13): the
	// SAME epic-scoped closed-bead set the orphan scan uses, and the
	// SAME `bd worktree list` enumeration FinalizeEpic itself merges
	// from, so a transient branch-existence-probe miss can never hide a
	// merge candidate the merge loop will see.
	implClosedEpicBeadIDsFn = lifecycle.ClosedEpicBeadIDs
	implWorktreeListFn      = bead.WorktreeList
	// implIsAncestorFn drives the worktree-enum leg's ancestry check —
	// routed through lifecycle.IsAncestor, a thin wrapper over
	// gitutil.IsAncestor, so this ADR-0030 enforcement package never
	// imports the git-plumbing package directly (internal/lint
	// boundary; internal/lifecycle already consumes it the same way).
	implIsAncestorFn = lifecycle.IsAncestor
	// implBranchExistsFn feeds ONLY the R3 obligation backstop's
	// branch-state-truthful recovery line (round-2 G3) — never the
	// orphan-detection trigger, which stays inside implScanOrphansFn.
	// Routed through lifecycle.BranchExists (thin wrapper over
	// gitutil.BranchExists) for the same ADR-0030 boundary reason as
	// implIsAncestorFn above.
	implBranchExistsFn = lifecycle.BranchExists
	// implGetMetadataFn and implCheckObligationsFn back R3's durable-
	// obligation backstop: the SAME check-only coverage predicate
	// (Spec 114 R2 discipline) `mindspec complete` itself settles,
	// exported by Bead 1 so this gate never re-implements it.
	implGetMetadataFn      = bead.GetMetadata
	implCheckObligationsFn = complete.CheckPendingObligations
)

// implContextLine renders the Req 8 worktree-context line for impl
// approve's gate failures (spec 092, mindspec-tjat): the directory the
// command ran from, plus root — the repo whose bd-derived state (epic
// phase, plan bead statuses) the gate evaluated. Getwd failure falls
// back to root: a degraded but truthful line beats no line.
func implContextLine(root string) string {
	cwd, err := implGetwdFn()
	if err != nil || cwd == "" {
		cwd = root
	}
	return workspace.ContextLine(cwd, root)
}

// ImplOpts holds options for implementation approval.
//
// Spec 086 Bead 3: `AllowDocSkew` activates the doc-sync override
// gate. Empty string means "no override". A non-empty string is
// recorded as `mindspec_impl_skew_reason` (alongside `_at` and
// `_by`) on the spec EPIC's metadata AFTER `exec.FinalizeEpic`
// returns nil. If FinalizeEpic fails, no override metadata is
// written — the failure itself is the audit trail.
type ImplOpts struct {
	AllowDocSkew string

	// OverrideADR is the human-readable reason for bypassing the
	// ADR-divergence backstop gate. Empty string means "no override".
	// A non-empty string causes the gate to be SKIPPED. After
	// `exec.FinalizeEpic` returns nil the reason is recorded on the
	// spec EPIC's metadata under the `mindspec_adr_override_*`
	// namespace.
	// Spec 087 Bead 3.
	OverrideADR string

	// SupersedeADR is the user-supplied ADR ID (e.g. "ADR-0099") for
	// the supersede backstop flow. Empty string means "no supersede".
	// When set, a placeholder ADR is pre-created on disk (Status:
	// Proposed, Domain(s) seeded from the first uncovered
	// DivergenceFinding) BEFORE the gate-skip decision; the gate is
	// then SKIPPED, and the four `mindspec_adr_supersede_*` keys are
	// written to the EPIC's metadata AFTER FinalizeEpic returns nil.
	// Mutually exclusive with OverrideADR at the CLI layer.
	// Spec 087 Bead 3.
	SupersedeADR string
}

// ImplResult holds the result of implementation approval.
type ImplResult struct {
	SpecID      string
	Warnings    []string
	SpecBranch  string
	CommitCount int
	DiffStat    string
	Pushed      bool // true if branch was pushed to remote

	// FinalizeBranch is bug wu7t's protected-main finalize carrier: set to
	// the chore/finalize-<specID> branch name when the spec branch was
	// already merged into main before this ran (see
	// executor.FinalizeResult.FinalizeBranch); empty on the normal,
	// not-yet-merged path.
	FinalizeBranch string
}

// ApproveImpl transitions from review mode to idle, completing the spec lifecycle.
// Enforcement logic (phase validation, epic closure, bead status checks) stays here.
// Git operations (merge, push, cleanup) are delegated to the Executor.
//
// Spec 086 Bead 3 ordering contract — all gates run BEFORE every
// mutating/terminal operation; the override metadata write runs AFTER
// `exec.FinalizeEpic` returns nil:
//
//  1. readBeadStatus loop (bead-status verification, non-mutating)
//  2. validate.ValidateDocs (doc-sync gate; honors AllowDocSkew override)
//  3. validate.CheckADRDivergence (ADR-divergence gate; NOT covered by override)
//  4. Spec 115 Bead 2: runOrphanObligationGate — the pre-terminal
//     refusal gate (R1 orphan scan + the worktree-enumeration merge-
//     prevention leg + R3 durable-obligation backstop), fail-closed on
//     every cleanly-signaled infra error on all three legs; hatches
//     bypass NOTHING here (this is a 4gsz-class lifecycle-bypass guard,
//     not the panel-gate decision). A refusal here performs no epic
//     close, no phase write, no merge, no push.
//  5. implPhaseMetadataFn(epicID, mindspec_phase=<derived>) — Spec 092
//     Req 1 deferred stale-phase reconcile; runs ONLY when the stored
//     phase failed the review/done gate but the child-derived phase
//     passed it, after the LAST pre-terminal gate (step 4) and before
//     the first mutation (step 6); never after step 7's done write
//  6. implRunBDCombinedFn("close", epicID) (EPIC CLOSE — first mutation)
//  7. implPhaseMetadataFn(epicID, mindspec_phase=done) (PHASE METADATA)
//  8. exec.CommitCount (pre-flight)
//  9. exec.FinalizeEpic (TERMINAL MUTATION)
//  10. implMergeMetadataFn(epicID, mindspec_impl_skew_*) — only if
//     AllowDocSkew set AND FinalizeEpic returned nil
//
// ADR-0041 (gate-before-mutate): steps 1-4 above are this verb's PREFLIGHT
// phase — the phase gate, the plan-bead gate, the doc-sync gate, the
// ADR-divergence backstop, and the Spec 115 orphan/obligation gate all
// resolve their facts and evaluate every derivable refusal before the first
// mutation (step 5's deferred phase reconcile is itself idempotent and
// still precedes step 6, the first hard mutation). The idempotent ADR-0034
// migration (phase.EnsureMigrated, immediately below) is the ADR's named
// exemption. Steps 6-9 are the COMMIT phase; step 9's exec.FinalizeEpic is
// the terminal mutation chain, whose own internal stages are individually
// classified KILL-TESTED / DOCUMENTED-FORWARD-SAFE in
// internal/executor/finalize_fault_test.go (Spec 119 Bead 6, AC-26 i4).
// This verb's own RECONCILE contract is bounded re-invocation converging to
// completion or a clean named refusal — see internal/approve/impl_fault_test.go.
func ApproveImpl(root, specID string, exec executor.Executor, opts ...ImplOpts) (*ImplResult, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	// Spec 089 / ADR-0034: one-shot legacy-to-metadata migration on first
	// lifecycle command. No-op if the epic already has mindspec_phase, or
	// when no epic exists yet. Migration errors fail the command
	// (spec 089 Requirement 9).
	if _, err := phase.EnsureMigrated(specID); err != nil {
		return nil, err
	}
	var o ImplOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	result := &ImplResult{SpecID: specID}

	// Find the epic for this spec directly (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		return nil, fmt.Errorf("no epic found for spec %s: %w", specID, err)
	}

	// Verify state is review mode: all children closed, pending final merge.
	//
	// Spec 092 Req 1 (mindspec-3smk): the stored mindspec_phase is a
	// trusted CACHE of the child-derived truth (ADR-0023 §3/§5,
	// ADR-0034 amendment). When the stored phase fails the review/done
	// gate but the child-derived phase satisfies it, gate evaluation
	// CONTINUES on the derived phase READ-ONLY — nothing is written
	// here. The forward reconcile write is deferred until after the
	// LAST pre-terminal gate (the ADR-divergence gate below) passes;
	// see the reconcile block immediately before MUTATION (1/3).
	phaseDetail, err := phase.DerivePhaseDetail(epicID)
	if err != nil {
		return nil, fmt.Errorf("deriving phase for spec %s: %w", specID, err)
	}
	implGateOK := func(p string) bool { return p == state.ModeReview || p == state.ModeDone }
	needsPhaseReconcile := false
	switch {
	case implGateOK(phaseDetail.Stored):
		// Stored phase satisfies the gate — no reconcile needed.
	case implGateOK(phaseDetail.Derived):
		// Stale cache, healthy ground truth: continue on the derived
		// phase; reconcile forward after the last pre-terminal gate.
		needsPhaseReconcile = true
	default:
		// Spec 092 Req 2: neither stored nor derived satisfies the
		// gate — name both phases and end with the spec-mandated
		// recovery line. Raw `bd update` metadata commands are never
		// emitted (Req 19: replace semantics over the whole map).
		// Req 8: the worktree-context line precedes the recovery line.
		return nil, guard.NewFailure(
			fmt.Sprintf("expected review mode for spec %s: stored phase %q and child-derived phase %q both fail the review/done gate\n%s", specID, phaseDetail.Stored, phaseDetail.Derived, implContextLine(root)),
			fmt.Sprintf("close remaining beads with 'mindspec complete <bead-id>', or if bead states are already correct run: mindspec repair phase %s", specID),
		)
	}

	// Spec 095 (mindspec-ry73): the phase gate now passes via `review`
	// even when a non-lifecycle follow-up child (e.g. a bug filed after the
	// last `complete`) is still open — the lifecycle-only derivation ignores
	// it. Emit an ADVISORY guard hint (ADR-0035 recovery-line convention)
	// naming any such open follow-up child so the operator can re-file or
	// detach it if they disagree. The hint NEVER blocks — the gate has
	// already passed. The recovery line deliberately does NOT bare-recommend
	// `bd update <id> --parent ""`: that detach is buggy (mindspec-bk5t — it
	// is not reflected in `bd list --parent`), so re-filing as standalone
	// backlog (or leaving the child attached) is recommended instead.
	if epicID != "" {
		if open := phase.OpenNonLifecycleChildrenForEpic(epicID); len(open) > 0 {
			fmt.Fprint(os.Stderr, formatOpenChildHint(specID, open))
		}
	}

	// Derive spec branch from convention. specID already validated at the
	// top of ApproveImpl (validate.SpecID == idvalidate.SpecID), so this
	// waist call cannot fail.
	specBranch, _ := workspace.SpecBranch(specID)
	result.SpecBranch = specBranch

	// Enforcement gate (1/3): verify all plan beads are closed.
	specDir, sdErr := workspace.SpecDir(root, specID)
	if sdErr != nil {
		return nil, sdErr
	}
	planPath := filepath.Join(specDir, "plan.md")
	beadIDs, planErr := readPlanBeadIDs(planPath)
	if planErr != nil && errors.Is(planErr, ErrPlanBeadIDsMalformed) {
		// AC-25: a malformed bead_ids entry REFUSES before ANY bd
		// invocation — unlike the benign "no plan.md" / "no frontmatter"
		// cases below, which silently skip this gate.
		return nil, planErr
	}
	if planErr == nil {
		for _, bid := range beadIDs {
			// bid is a bead_ids entry from the agent-authored plan.md
			// frontmatter (readPlanBeadIDs, R4 cluster 2) — NEVER
			// idvalidate'd. The functional bd lookup (readBeadStatus)
			// takes the raw bid; every DISPLAY position below renders
			// safeBid instead, matching internal/validate/beads.go's
			// checkBeadIDs treatment of the same untrusted source.
			safeBid := idrender.Bead(bid)
			status, err := readBeadStatus(bid)
			if err != nil {
				return nil, fmt.Errorf("checking bead %s status: %w", safeBid, err)
			}
			if status != "closed" {
				// Spec 092 Reqs 8/12 (mindspec-tjat): context line plus
				// a final copy-pastable recovery line.
				return nil, guard.NewFailure(
					fmt.Sprintf("bead %s is still %q — close all beads before approving implementation\n%s", safeBid, status, implContextLine(root)),
					fmt.Sprintf("mindspec complete %s", safeBid),
				)
			}
		}
	}

	// Enforcement gate (2/3): Spec 086 (F2) doc-sync. Compute the
	// merge-base against `main` so the gate sees the whole spec
	// branch's diff. The `--allow-doc-skew "<reason>"` override
	// suppresses the failure; the reason is recorded on the spec
	// epic's metadata only AFTER FinalizeEpic returns nil.
	base, mbErr := exec.MergeBase("main", specBranch)
	if mbErr != nil {
		return nil, fmt.Errorf("computing merge-base for doc-sync: %w", mbErr)
	}
	// Spec 095: the whole-branch doc-sync gate diffs the explicit
	// base..specBranch RANGE (NOT working-tree-vs-base) and reads
	// OWNERSHIP attribution from the spec-branch tip — both the diff
	// head and the ownership ref are the spec-branch tip — so an
	// OWNERSHIP claim committed anywhere on the spec branch satisfies
	// the backstop with no override (mindspec-vvs9). The prior
	// ValidateDocs(root, base, exec) read the working tree on both
	// counts.
	docResult := validate.ValidateDocsRange(root, base, specBranch, specBranch, exec)
	// Spec 091 Req 22(a): surface warning-severity issues BEFORE the
	// failure decision so they print on every run — including when
	// HasFailures() is false and the flow proceeds normally, and on
	// the override/error paths.
	printResultWarnings(warnWriter, docResult)
	if docResult.HasFailures() {
		if o.AllowDocSkew == "" {
			return nil, fmt.Errorf("doc-sync: %s\nhint: re-run with --allow-doc-skew \"<reason>\" to override (records the reason on the spec epic's metadata)", joinResultErrorMessages(docResult))
		}
		// Override path: fall through. Metadata write happens AFTER
		// FinalizeEpic per panel CONSENSUS revision 4.
	}

	// Enforcement gate (3/3): Spec 087 Bead 2 fills the body; this
	// backstop runs across the full spec branch. The
	// `--override-adr` and `--supersede-adr` flags (Spec 087 Bead 3)
	// bypass the gate; `--allow-doc-skew` does NOT (panel CONSENSUS
	// rev 6). The findings slice seeds the supersede placeholder's
	// Domains field structurally (revision 2 — no string parsing).
	// headRef "" + beadID "" → the lane derives the spec branch tip
	// itself; the measured refs stay main-merge-base..spec-branch-tip.
	// Ownership ref = spec-branch tip (specBranch), independent of the
	// derived diff head (spec 095 / mindspec-vvs9).
	adrResult, adrFindings := validate.CheckADRDivergence(root, base, exec, specDir, "", "", specBranch)
	// Same severity-generic pipe for the ADR-divergence backstop: any
	// SevWarning the gate emits (e.g. adr-divergence-proposed) renders
	// without further wiring. No-op while the gate emits none.
	printResultWarnings(warnWriter, adrResult)

	// Spec 119 Bead 3 (P6/P2/R1): resolve the FinalizeEpic lifecycle
	// allow-set HERE — with the other preflight FACTS, immediately after
	// the last read-only gate's fact computation (ADR-divergence above)
	// and BEFORE the supersede-ADR placeholder file write below (which
	// today runs, and mutates disk, ahead of a derivable refusal — R1).
	// The allow-set is the intersection planDeclared(specID) ∩
	// lifecycleChildren(epicID): planDeclared is the beadIDs already read
	// above (readPlanBeadIDs); lifecycleChildren is the NEW
	// phase.LifecycleChildIDsForEpic classifier (P3). A classification
	// failure refuses PRE-mutation with a named error — strictly
	// stronger than the executor-side fail-closed abort, since nothing
	// has mutated yet. When the plan itself is unreadable (planErr != nil)
	// the set is left nil here; Leg 3 of runOrphanObligationGate below
	// (unchanged) is the single place that names that specific refusal
	// (R8/AC-17), so this resolution step does not duplicate it.
	lifecycleChildren, lcErr := phase.LifecycleChildIDsForEpic(epicID)
	if lcErr != nil {
		return nil, guard.NewFailure(
			fmt.Sprintf("could not classify spec %s's epic %s children to scope finalize: %v", specID, epicID, lcErr),
			fmt.Sprintf("mindspec impl approve %s", specID),
		)
	}
	var lifecycleAllowSet []string
	if planErr == nil {
		lifecycleAllowSet = intersectIDs(beadIDs, lifecycleChildren)
	}

	// Spec 115 Bead 2: the pre-terminal orphan/obligation refusal gate.
	// Spec 119 Bead 3 (R1): now runs immediately after the allow-set
	// resolution above and BEFORE the supersede-ADR placeholder write
	// and the ADR-divergence refusal decision below — a derivable
	// refusal here must precede that disk mutation. Still AFTER every
	// read-only gate's fact computation (the ADR-divergence gate above
	// is the last of them) and BEFORE the Spec 092 deferred phase-
	// reconcile write, MUTATION (1/3) epic close, the mindspec_phase=done
	// write, the CommitCount preflight, and exec.FinalizeEpic below — so
	// a refusal here performs NO epic close, NO phase write, NO merge,
	// NO push, NO placeholder-ADR write. Hatches (MINDSPEC_SKIP_PANEL,
	// enforcement.panel_gate: false) bypass NOTHING here.
	if err := runOrphanObligationGate(root, specID, specBranch, beadIDs, planErr); err != nil {
		return nil, err
	}

	// Pre-create the placeholder ADR FIRST when --supersede-adr is
	// requested so the new file exists on disk even if a downstream
	// step fails. Spec 087's pre-create-before-the-gate-skip-decision
	// rule is preserved: the placeholder still exists before the
	// ADR-divergence refusal decision immediately below, and when
	// --supersede-adr is set that decision is skipped, so no refusal
	// follows the write.
	var supersedeNewID string
	if o.SupersedeADR != "" {
		var seedDomains []string
		for _, f := range adrFindings {
			if f.Kind == "uncovered" && f.Domain != "" {
				seedDomains = []string{f.Domain}
				break
			}
		}
		title := "Placeholder for " + o.SupersedeADR
		if _, err := implCreateWithIDFn(root, o.SupersedeADR, title, adr.CreateOpts{Domains: seedDomains}); err != nil {
			return nil, fmt.Errorf("--supersede-adr: %w", err)
		}
		supersedeNewID = o.SupersedeADR
	}

	if o.OverrideADR == "" && o.SupersedeADR == "" && adrResult.HasFailures() {
		return nil, fmt.Errorf("adr-divergence: %s\nhint: re-run with --override-adr \"<reason>\" or --supersede-adr ADR-NNNN to bypass",
			joinResultErrorMessages(adrResult))
	}

	// Spec 092 Req 1: deferred forward reconcile of the stale phase
	// cache. Placement is panel-pinned: AFTER the last pre-terminal
	// gate (the ADR-divergence gate above, the last of phase gate /
	// plan-bead gate / doc-sync gate / ADR-divergence gate) and BEFORE
	// MUTATION (1/3) below — and NEVER after the mindspec_phase=done
	// write, which must run after (and supersede) the reconcile. The
	// CommitCount preflight further down is NOT a pre-terminal gate
	// for this purpose (its post-mutation placement is pinned by Spec
	// 086 panel CONSENSUS revision 9). If this write fails the command
	// exits non-zero having performed no terminal mutation (HC-4);
	// re-derivation is deterministic, so a re-run repeats the
	// reconcile idempotently.
	if needsPhaseReconcile && epicID != "" {
		if err := implPhaseMetadataFn(epicID, map[string]interface{}{
			"mindspec_phase": phaseDetail.Derived,
		}); err != nil {
			return nil, guard.NewFailure(
				fmt.Sprintf("reconciling stale phase for spec %s (stored %q, child-derived %q): %v", specID, phaseDetail.Stored, phaseDetail.Derived, err),
				fmt.Sprintf("mindspec repair phase %s", specID),
			)
		}
		// HC-3: silent-on-success self-heal — one structured stderr
		// line in the event=<ns>.<name> key=value convention.
		fmt.Fprintf(os.Stderr, "event=lifecycle.phase_reconciled spec=%s epic=%s stored=%s derived=%s\n",
			specID, epicID, phaseDetail.Stored, phaseDetail.Derived)
	}

	// MUTATION (1/3): close epic and mark as explicitly done.
	if epicID != "" {
		// Gate-all-ids (ADR-0042 §1, round 9): epicID feeds a `bd close`
		// argv build directly — validate BEFORE any bd spawn. epicID is
		// already RETURN-gated at phase.FindEpicBySpecIDWithCache, so this
		// is defense in depth (a well-formed id passes for free).
		if err := idvalidate.BeadID(epicID); err != nil {
			return nil, guard.NewFailure(
				fmt.Sprintf("resolved epic id %s is invalid: %v", termsafe.Escape(epicID), err),
				fmt.Sprintf("mindspec repair phase %s", specID),
			)
		}
		if _, err := implRunBDCombinedFn("close", epicID); err != nil {
			if !isAlreadyClosedErr(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not close lifecycle epic %s: %v", epicID, err))
			}
		}
		// MUTATION (2/3): Spec 080 phase metadata write. Runs after
		// (and supersedes) the Req 1 reconcile above — the end state
		// of a fully successful ApproveImpl is mindspec_phase=done.
		if err := implPhaseMetadataFn(epicID, map[string]interface{}{
			"mindspec_phase": "done",
			"mindspec_done":  true,
		}); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not set done marker on epic %s: %v", epicID, err))
		}
	}

	// Pre-flight: check spec branch has commits (via executor).
	// Pinned between phase-metadata write and FinalizeEpic per panel
	// CONSENSUS revision 9 so a future regression that re-shuffles
	// this line is caught by TestApproveImplCallOrder.
	//
	// Spec 119 Bead 3 (AC-17/R7): the refusal DISJUNCTION that used to
	// live here (`count == 0 && (planErr != nil || len(beadIDs) == 0)`)
	// is REMOVED — it was unreachable in normal flow, since Leg 3 of
	// runOrphanObligationGate above already refuses whenever
	// planErr != nil (and readPlanBeadIDs errors on an empty bead_ids
	// list, so len(beadIDs)==0 implies planErr!=nil too), before this
	// preflight runs. The CALL itself is PRESERVED at this exact
	// position — a documented retention, not dead code — solely so
	// TestApproveImplCallOrder continues to pin its place between the
	// phase-metadata write and FinalizeEpic (CONSENSUS revision 9); its
	// result is intentionally unused. A valid-plan, zero-commit spec is
	// the legitimate cleanup path (see
	// TestApproveImpl_NoCommitsButClosedBeads_AllowsCleanup) and was
	// never blocked by the removed disjunction either.
	_, _ = exec.CommitCount("main", specBranch)

	// MUTATION (3/3, terminal): delegate to executor for merge/push,
	// scoped to lifecycleAllowSet (Spec 119 Bead 3, R6/P6) resolved
	// above.
	fr, err := exec.FinalizeEpic(epicID, specID, specBranch, lifecycleAllowSet)
	if err != nil {
		return nil, fmt.Errorf("finalizing epic: %w", err)
	}

	// Spec 086 Bead 3: record doc-sync skew override ONLY after the
	// terminal mutation returned nil. On failure we'd have already
	// returned above — the absence of metadata is then the audit
	// trail. Best-effort: a metadata-write failure becomes a warning.
	if o.AllowDocSkew != "" && epicID != "" {
		meta := buildImplSkewMetadata(o.AllowDocSkew)
		if err := implMergeMetadataFn(epicID, meta); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not record impl-skew override metadata on %s: %v", epicID, err))
		}
	}

	// Spec 087 Bead 3: ADR-divergence override / supersede metadata
	// writes on the EPIC, mirroring the doc-skew discipline above.
	// Distinct namespace per spec.md Requirement 13.
	if o.OverrideADR != "" && epicID != "" {
		meta := map[string]interface{}{
			"mindspec_adr_override_reason": o.OverrideADR,
			"mindspec_adr_override_at":     time.Now().UTC().Format(time.RFC3339),
			"mindspec_adr_override_by":     implGitUserEmailFn(),
		}
		if err := implMergeMetadataFn(epicID, meta); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not record adr-override metadata on %s: %v", epicID, err))
		}
	}
	if o.SupersedeADR != "" && epicID != "" {
		reason := o.OverrideADR
		if reason == "" {
			reason = "superseded by " + supersedeNewID
		}
		meta := map[string]interface{}{
			"mindspec_adr_supersede_id":     supersedeNewID,
			"mindspec_adr_supersede_reason": reason,
			"mindspec_adr_supersede_at":     time.Now().UTC().Format(time.RFC3339),
			"mindspec_adr_supersede_by":     implGitUserEmailFn(),
		}
		if err := implMergeMetadataFn(epicID, meta); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not record adr-supersede metadata on %s: %v", epicID, err))
		}
	}

	result.CommitCount = fr.CommitCount
	result.DiffStat = fr.DiffStat
	result.Pushed = (fr.MergeStrategy == "pr")
	result.FinalizeBranch = fr.FinalizeBranch

	// Stop recording (best-effort — before transitioning to idle)
	if err := recording.StopRecording(root, specID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not stop recording: %v", err))
	}

	return result, nil
}

func readBeadStatus(id string) (string, error) {
	// Gate-all-ids (ADR-0042 §1, round 6/9): id feeds a `bd show` argv
	// build via the generic RunBD seam — validated here as defense in
	// depth, on top of the readPlanBeadIDs read-gate its sole production
	// caller already applies (AC-25/AC-26). The functional bd invocation
	// below takes the raw id; the error-message DISPLAY positions render
	// the idrender'd copy (R4 cluster 2).
	if err := idvalidate.BeadID(id); err != nil {
		return "", fmt.Errorf("invalid bead id %s: %w", idrender.Bead(id), err)
	}
	out, err := implRunBDFn("show", id, "--json")
	if err != nil {
		return "", err
	}

	var payload []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parsing bd show output for %s: %w", idrender.Bead(id), err)
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("no bead returned for %s", idrender.Bead(id))
	}
	return strings.ToLower(strings.TrimSpace(payload[0].Status)), nil
}

// readPlanBeadIDs reads bead_ids from the plan.md YAML frontmatter. The block
// is located via the canonical internal/frontmatter.Parse (ARCH-6) rather than
// a hand-rolled `\n---` substring scan, so only a whole-line `---` fence closes
// the block and a space-padded fence reads as no-frontmatter.
//
// Class-2 executable-operand consumer gate (ADR-0042 §1, spec 120 R2 AC-25,
// round 6 G3): plan.md's bead_ids frontmatter is agent-writable, and every
// entry here previously reached bd argv (readBeadStatus's `bd show <id>`
// and, via the plan-declared intersection, implCheckObligationsFn) with NO
// idvalidate.BeadID gate — a `--help`/`-`-prefixed entry is bd option
// injection. Every entry is validated HERE, at the read, before ANY caller
// ever sees the list: a malformed entry REFUSES convergently (the plan-
// frontmatter lever) rather than letting it flow to a bd spawn. Clean
// dotted-child bead_ids (e.g. "mindspec-9cyu.1") pass byte-identically.
func readPlanBeadIDs(planPath string) ([]string, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, err
	}

	block, _, ok := frontmatter.Parse(data)
	if !ok {
		return nil, fmt.Errorf("no frontmatter found")
	}

	var fm struct {
		BeadIDs []string `yaml:"bead_ids"`
	}
	if err := yaml.Unmarshal(block, &fm); err != nil {
		return nil, fmt.Errorf("parsing plan frontmatter: %w", err)
	}
	if len(fm.BeadIDs) == 0 {
		return nil, fmt.Errorf("no bead_ids in plan frontmatter")
	}
	for _, bid := range fm.BeadIDs {
		if err := idvalidate.BeadID(bid); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrPlanBeadIDsMalformed, guard.FormatFailure(
				fmt.Sprintf("plan frontmatter bead_ids entry %s is not a valid bead ID: %v", idrender.Bead(bid), err),
				"fix plan.md's bead_ids frontmatter, then re-run: mindspec plan approve",
			))
		}
	}
	return fm.BeadIDs, nil
}

// ErrPlanBeadIDsMalformed is the sentinel readPlanBeadIDs wraps when a
// plan.md bead_ids entry fails idvalidate.BeadID (spec 120 AC-25). Callers
// that otherwise treat a readPlanBeadIDs error as "no plan-bead gate to
// run" (a genuinely-absent or unparseable plan.md, the pre-existing
// benign case) use errors.Is(err, ErrPlanBeadIDsMalformed) to instead
// REFUSE convergently — a hostile bead_ids entry must never silently skip
// the gate the same way a missing plan.md does.
var ErrPlanBeadIDsMalformed = errors.New("plan frontmatter bead_ids contains a malformed bead id")

func isAlreadyClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already closed")
}

// intersectIDs returns the elements of planBeadIDs that are also present in
// lifecycleChildren — the FinalizeEpic lifecycle allow-set (Spec 119 Bead 3,
// R6/P6): plan-declared ∩ lifecycle-classified. Always returns a non-nil
// slice (possibly empty) so the executor's "nil means not computed" sentinel
// (AC-14) is never mistaken for "no lifecycle beads" — Go's zero value for a
// nil map read is false, so an empty planBeadIDs or lifecycleChildren input
// correctly yields an empty (non-nil) result via append's own semantics.
func intersectIDs(planBeadIDs, lifecycleChildren []string) []string {
	lifecycle := make(map[string]bool, len(lifecycleChildren))
	for _, id := range lifecycleChildren {
		lifecycle[id] = true
	}
	out := make([]string, 0, len(planBeadIDs))
	for _, id := range planBeadIDs {
		if lifecycle[id] {
			out = append(out, id)
		}
	}
	return out
}

// --- Spec 115 Bead 2: the pre-terminal orphan/obligation refusal gate ---
//
// runOrphanObligationGate closes the last un-gated merge path in the
// binary (spec 115's Goal): a bead closed via a raw `bd close` — or one
// whose durable refutation obligation was never settled — must not ride
// `impl approve`'s auto-merge un-gated. Three legs, all fail-CLOSED on
// their own cleanly-signaled infra errors:
//
//  1. R1 — implScanOrphansFn (lifecycle.ScanOrphanedClosedBeads): any
//     closed epic bead whose bead/<id> branch exists and is NOT an
//     ancestor of the spec branch was closed without `mindspec
//     complete`. The branch-existence trigger inside it stays the
//     unchanged bool gitutil.BranchExists (round-6 C+B, via lifecycle's
//     own internal seam): absent, or a probe-infra failure, both read
//     as "no trigger" — a genuinely deleted (merged-and-cleaned) branch
//     never false-refuses.
//  2. The round-7 Option B worktree-enumeration merge-prevention leg
//     (AC13): keyed off the SAME `bd worktree list` enumeration
//     FinalizeEpic itself merges from, so a transient branch-existence-
//     probe miss (the round-6 G2 race) can never hide a merge candidate
//     the merge loop will see.
//  3. R3 — the durable-obligation backstop: every plan bead's recorded
//     refutation_pending obligations must be (slot, round)-exactly
//     covered by a durable panel_refuted record, via the SAME check-
//     only predicate `mindspec complete` uses to settle it.
//
// Hatches (MINDSPEC_SKIP_PANEL, enforcement.panel_gate: false) are never
// consulted here — this is a 4gsz-class lifecycle-bypass guard, not the
// panel-gate decision; they keep their exact 114 semantics inside the
// `mindspec complete <bead>` recovery run this gate always recommends.
func runOrphanObligationGate(root, specID, specBranch string, planBeadIDs []string, planErr error) error {
	// Leg 1 (R1): the error-preserving orphan scan. Any of the three
	// cleanly-signaled infra errors (epic-lookup, bd-list, ancestry)
	// refuses fail-closed — an unreadable store cannot prove the epic
	// is settled.
	orphans, err := implScanOrphansFn(specID, root, "")
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("could not verify every closed bead under spec %s's epic is merged (orphan scan failed): %v", specID, err),
			fmt.Sprintf("mindspec impl approve %s", specID),
		)
	}
	if len(orphans) > 0 {
		return implOrphanRefusal(root, specID, orphans[0])
	}

	// Leg 2 (R1 round-7 Option B): the worktree-enumeration merge-
	// prevention leg. Fail-closed on its own infra (WorktreeList,
	// ClosedEpicBeadIDs, ancestry) — deliberate asymmetry vs
	// FinalizeEpic itself, whose equivalent failures merge-SKIP (the
	// safe direction there; refusing is the safe direction here).
	if err := runWorktreeEnumerationLeg(root, specID, specBranch); err != nil {
		return err
	}

	// Leg 3 (R3): the durable-obligation backstop. Fail-CLOSED on an
	// unreadable plan-bead enumeration too — unlike gate (1/3) above in
	// ApproveImpl, which silently skips on the same planErr (unchanged,
	// on purpose: that gate's contract predates this one). A corrupt or
	// missing plan.md must not make this leg's unique coverage (raw-
	// merged or branch-deleted beads carrying an unsettled obligation)
	// silently vanish.
	if planErr != nil {
		return guard.NewFailure(
			fmt.Sprintf("spec %s's plan bead list could not be read to verify every durable refutation obligation is settled: %v", specID, planErr),
			fmt.Sprintf("mindspec impl approve %s", specID),
		)
	}
	for _, bid := range planBeadIDs {
		if obErr := implCheckObligationsFn(bid, implGetMetadataFn); obErr != nil {
			return implObligationRefusal(bid, obErr)
		}
	}
	return nil
}

// runWorktreeEnumerationLeg is the round-7 Option B worktree-
// enumeration merge-prevention leg (AC13): it enumerates the SAME `bd
// worktree list` source FinalizeEpic itself merges from
// (defaultWorktreeOps.List(), mindspec_executor.go), filters to real
// bead/<id> branch lines (the same filter FinalizeEpic applies), scopes
// to the finalizing spec's own closed-epic-bead set (so a different
// spec's worktree neither triggers nor suppresses this leg — blp6
// unchanged), and refuses if any such branch is NOT an ancestor of the
// spec branch — regardless of what the branch-existence probe inside
// implScanOrphansFn reported. Because the gate and the merge loop key
// off the identical enumeration, a transient branch-existence-probe
// miss can no longer hide a merge candidate the loop will see.
func runWorktreeEnumerationLeg(root, specID, specBranch string) error {
	entries, err := implWorktreeListFn()
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("could not enumerate worktrees to verify spec %s's closed epic beads are all merged: %v", specID, err),
			fmt.Sprintf("mindspec impl approve %s", specID),
		)
	}
	closedIDs, err := implClosedEpicBeadIDsFn(specID)
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("could not enumerate spec %s's closed epic beads to verify they are all merged: %v", specID, err),
			fmt.Sprintf("mindspec impl approve %s", specID),
		)
	}
	closed := make(map[string]bool, len(closedIDs))
	for _, id := range closedIDs {
		closed[id] = true
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Branch, workspace.BeadBranchPrefix) {
			continue
		}
		// Reverse-derivation gate (ADR-0042 §1 reverse, AC-23): beadID is
		// parsed back OUT of an agent-creatable worktree/branch entry. A
		// malformed candidate is skipped — never matched against the
		// closed-epic-bead set, never embedded in an ID role.
		beadID := strings.TrimPrefix(e.Branch, workspace.BeadBranchPrefix)
		if beadID == "" || idvalidate.BeadID(beadID) != nil || !closed[beadID] {
			continue
		}
		isAnc, ancErr := implIsAncestorFn(root, e.Branch, specBranch)
		if ancErr != nil {
			// R4 (spec 120): e.Branch is a free-text field from
			// `bd worktree list --json` (agent-writable, never idvalidate'd),
			// and ancErr echoes it back (gitutil IsAncestor). Escape the
			// branch display and the whole error string (which re-embeds the
			// branch + raw git stderr) — byte-identical for a genuine ref,
			// control-bytes neutralized for a hostile one. Mirrors the
			// implOrphanRefusal sibling below.
			return guard.NewFailure(
				fmt.Sprintf("could not verify worktree branch %s is merged into %s: %s", termsafe.Escape(e.Branch), termsafe.Escape(specBranch), termsafe.Escape(ancErr.Error())),
				fmt.Sprintf("mindspec impl approve %s", idrender.Spec(specID)),
			)
		}
		if isAnc {
			continue
		}
		return implOrphanRefusal(root, specID, lifecycle.Orphan{
			BeadID:     beadID,
			BeadBranch: e.Branch,
			SpecBranch: specBranch,
		})
	}
	return nil
}

// implOrphanRefusal renders the (a)/(b)/(c)-shaped refusal shared by
// Leg 1 and Leg 2: (a) names the bead ID, its unmerged bead/<id>
// branch, and the spec branch; (b) states it was closed without
// `mindspec complete`; (c) ends with o.RecoveryCommand() as the FINAL
// line (ADR-0035; internal/guard/recovery_convention_test.go enforces
// the final-line shape). The advisory slot line (R2) is best-effort
// decoration ONLY — never load-bearing, never printed if unreadable.
func implOrphanRefusal(root, specID string, o lifecycle.Orphan) error {
	msg := fmt.Sprintf(
		"bead %s (branch %s) was closed without running mindspec complete and is not merged into %s",
		idrender.Bead(o.BeadID), termsafe.Escape(o.BeadBranch), termsafe.Escape(o.SpecBranch),
	)
	if slot := implAdvisorySlotLine(root, specID, o.BeadID); slot != "" {
		msg += "\n" + slot
	}
	return guard.NewFailure(msg, o.RecoveryCommand())
}

// formatOpenChildHint renders the Spec 095 (mindspec-ry73) advisory hint
// naming open non-lifecycle follow-up children of specID's epic (see the
// call site above for the full rationale). R4: c.ID is an ID-typed
// position (idrender.Bead); c.Title is agent-writable free text
// (termsafe.Escape); specID is likewise idrender'd.
func formatOpenChildHint(specID string, open []phase.ChildInfo) string {
	names := make([]string, 0, len(open))
	for _, c := range open {
		if c.Title != "" {
			names = append(names, fmt.Sprintf("%s (%s)", idrender.Bead(c.ID), termsafe.Escape(c.Title)))
		} else {
			names = append(names, idrender.Bead(c.ID))
		}
	}
	return fmt.Sprintf(
		"hint: spec %s reached review with open non-lifecycle follow-up child(ren) not blocking the lifecycle: %s\nrecovery: leave attached, or re-file as standalone backlog with 'bd create' then close the epic child — do NOT use 'bd update <id> --parent \"\"' (the detach is not reflected in 'bd list --parent', mindspec-bk5t)\n",
		idrender.Spec(specID), strings.Join(names, ", "))
}

// implAdvisorySlotLine is Spec 115 R2: best-effort naming of the
// unresolved reviewer slot(s) from beadID's registered panel, decorating
// an orphan refusal. Strictly advisory and read-only: an unreadable,
// missing, or removed panel simply omits the line — never a pass, never
// a crash, no gate decision computed here, no metadata written. The
// returned line never contains MINDSPEC_SKIP_PANEL (HC-7) or a paste-
// able refutation incantation.
func implAdvisorySlotLine(root, specID, beadID string) string {
	roots := complete.PanelGateRoots(root, "", specID)
	regs := panel.ForBead(panel.Scan(roots...), beadID)
	if len(regs) == 0 {
		return ""
	}
	res, err := panel.Tally(regs[0].Dir)
	if err != nil {
		return ""
	}
	unresolved := res.UnresolvedVerdicts()
	if len(unresolved) == 0 {
		return ""
	}
	return formatAdvisorySlotLine(beadID, unresolved)
}

// formatAdvisorySlotLine renders implAdvisorySlotLine's message body from
// an already-resolved unresolved-verdict slice. R4: beadID is an
// ID-typed position (idrender.Bead); each Slot is derived from a
// reviewer verdict filename and is escaped per-entry before joining.
func formatAdvisorySlotLine(beadID string, unresolved []panel.Verdict) string {
	slots := make([]string, 0, len(unresolved))
	for _, v := range unresolved {
		slots = append(slots, termsafe.Escape(v.Slot))
	}
	return fmt.Sprintf("panel advisory: bead %s carries unresolved reviewer slot(s) %s on its registered panel", idrender.Bead(beadID), strings.Join(slots, ", "))
}

// implObligationRefusal wraps an uncovered-obligation error (Leg 3, R3)
// with a branch-state-truthful recovery (round-2 G3): when beadID's
// bead/<id> branch still exists, a bare `mindspec complete <bead>` is
// runnable and settles the obligation at step 3.75; when the branch is
// genuinely absent, a bare complete would die at the step-3.5 merge-base
// BEFORE reaching that reconciliation, so the recourse names the
// restoration prerequisite first — the message must never present a
// command known to fail.
func implObligationRefusal(beadID string, cause error) error {
	// beadID flows in from planBeadIDs (readPlanBeadIDs, R4 cluster 2).
	// workspace.BeadBranch(beadID) is a FUNCTIONAL branch-name
	// construction (fed to implBranchExistsFn's git lookup), so it takes
	// the raw beadID; every DISPLAY position below (the two
	// recovery-command lines and the branch-name mention) renders the
	// idrender'd copy instead.
	branch, err := workspace.BeadBranch(beadID)
	branchValid := err == nil
	if !branchValid {
		// beadID here is a plan-declared, already-gated id (readPlanBeadIDs
		// validates every entry, AC-25) reaching this rendering helper —
		// this should be unreachable, but degrade to a placeholder rather
		// than composing a hostile branch name.
		branch = "<bead-branch>"
	}
	safeBeadID := idrender.Bead(beadID)
	safeBranch := workspace.BeadBranchPrefix + safeBeadID
	if branchValid && implBranchExistsFn(branch) {
		return guard.NewFailure(cause.Error(), fmt.Sprintf("mindspec complete %s", safeBeadID))
	}
	return guard.NewFailure(
		cause.Error(),
		fmt.Sprintf("restore the %s branch ref (it no longer exists) so 'mindspec complete' can reach its reconciliation step, or settle the obligation out-of-band", safeBranch),
		fmt.Sprintf("mindspec complete %s", safeBeadID),
	)
}

// buildImplSkewMetadata returns the override metadata for the spec
// epic, mirroring `complete.buildSkewMetadata` but with the
// impl-prefixed keys baked in. Spec 086 Bead 3 panel CONSENSUS
// revision 4: write-order rule means this is only called AFTER
// `exec.FinalizeEpic` returns nil.
func buildImplSkewMetadata(reason string) map[string]interface{} {
	return map[string]interface{}{
		"mindspec_impl_skew_reason": reason,
		"mindspec_impl_skew_at":     time.Now().UTC().Format(time.RFC3339),
		"mindspec_impl_skew_by":     implGitUserEmailFn(),
	}
}

// warnWriter is the destination for WARN lines rendered from
// validation results (Spec 091 Bead 5, Req 22). Production writes to
// stderr; package-level seam so tests can capture the output.
var warnWriter io.Writer = os.Stderr

// printResultWarnings renders every warning-severity issue carried by
// a *validate.Result as `WARN <name>: <message>` — one line per
// issue. Severity-generic: it prints ANY SevWarning regardless of
// which validator lane produced it (cmd-docs, missing-source-globs,
// adr-divergence-proposed, ...). Stateless by construction (HC-2):
// no marker files, no seen-tracking, no dedup — the same warning
// prints on every invocation for as long as the Result carries it.
// Warnings never affect the pass/fail decision.
func printResultWarnings(w io.Writer, r *validate.Result) {
	for _, i := range r.Issues {
		if i.Severity != validate.SevWarning {
			continue
		}
		fmt.Fprintf(w, "WARN %s: %s\n", i.Name, i.Message)
	}
}

// joinResultErrorMessages flattens SevError-severity issues from a
// *validate.Result into a single line suitable for fmt.Errorf wrapping.
func joinResultErrorMessages(r *validate.Result) string {
	msgs := make([]string, 0, len(r.Issues))
	for _, i := range r.Issues {
		if i.Severity != validate.SevError {
			continue
		}
		msgs = append(msgs, fmt.Sprintf("[%s] %s", i.Name, i.Message))
	}
	return strings.Join(msgs, "; ")
}
