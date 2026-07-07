package complete

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Package-level function variables for testability.
var (
	closeBeadFn     = bead.Close
	worktreeListFn  = bead.WorktreeList
	runBDFn         = bead.RunBD
	resolveTargetFn = resolve.ResolveTarget
	findLocalRootFn = defaultFindLocalRoot
	fetchBeadByIDFn = next.FetchBeadByID
	// fetchBeadAsOfFn is the committed-state read seam (bead mindspec-uopd):
	// `bd show <id> --as-of HEAD --json` (bd >= 1.0.4). See the
	// defaultVerifyCommitted doc comment for how this is used and how it
	// degrades on an older bd. Tests swap this to simulate an
	// unsupported-flag response or a definitive closed/open committed read.
	fetchBeadAsOfFn         = next.FetchBeadAsOf
	findEpicForBeadFn       = phase.FindEpicForBead
	completeMergeMetadataFn = bead.MergeMetadata
	gitUserEmailFn          = bead.GitUserEmail
	// checkDirtyTreeFn is the ADR-0025 artifact-aware tree classification
	// shared with `mindspec next` (spec 092 Req 6, DQ-2: direct reuse of
	// next's classifier — internal/complete already imports internal/next).
	// Tests swap this to simulate artifact/user dirt without a real repo.
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	// completeGetwdFn feeds the Req 8 worktree-context line on the
	// user-dirt guard failure (spec 092, mindspec-tjat). Tests swap it
	// to pin the worktree kind regardless of where `go test` runs.
	completeGetwdFn = os.Getwd
	// adrCreateWithIDFn is the package-level seam for the placeholder-
	// ADR creation step in the --supersede-adr flow. Tests swap this
	// to avoid writing real ADR files when only asserting flow
	// behavior, though the default is the real implementation since
	// TestSupersedeUnblocks asserts on-disk presence.
	adrCreateWithIDFn = adr.CreateWithID
	// Spec 098 Req 2 (mindspec-9n2h): after a successful `bd close`, force
	// durability with `bd dolt commit` and then re-verify the bead is
	// closed before accepting case (a) ("re-read affirms closed → proceed").
	// A nil return from closeBeadFn + a session re-read of "closed" does NOT
	// prove the close PERSISTED to committed Dolt state (the 2u0u
	// recurrence: a non-persisting close still reads back "closed" and slips
	// through to merge + worktree removal on exit 0).
	//
	// doltCommitFn forces the strongest available bd durability primitive
	// (idempotent: a clean working set is a no-op success — see
	// bead.DoltCommit). verifyCommittedFn performs the post-commit
	// verification re-read.
	//
	// HONESTY-CLAUSE (spec 098 Req 2, step 6; closed by bead mindspec-uopd):
	// the original verify-first probe found that `bd` runs in EMBEDDED
	// auto-commit mode — every write (including `bd close`) auto-commits,
	// `bd dolt commit` after a close is a clean-working-set no-op ("Nothing
	// to commit"), and at the time `bd` exposed NO committed-state read
	// distinct from `bd show` (`bd dolt status` reports the Dolt ENGINE
	// status, not a working-set diff), so verifyCommittedFn was scoped to
	// detection-via-the-same-read.
	//
	// bd >= 1.0.4 closes that gap: `bd show <id> --as-of HEAD --json`
	// ("Show issue as it existed at a specific commit hash or branch")
	// reads COMMITTED Dolt state, which — because bd auto-commits every
	// write — is a genuine committed-state re-read distinct from the
	// in-session `bd show`. defaultVerifyCommitted now verifies via
	// fetchBeadAsOfFn (the `--as-of HEAD` path) FIRST. Only when that
	// invocation fails with bd's `unknown flag: --as-of` signature (an
	// older, pre-1.0.4 bd that does not recognize the flag —
	// bead.IsUnsupportedFlagError) does it gracefully degrade to the
	// original same-read fallback (fetchBeadByIDFn, the `bd show --json`
	// path), logging a one-line warning that verification was downgraded.
	// A hard read failure or a not-closed status in EITHER path still
	// errors — complete never proceeds on an unverified close, on the
	// FORCED `bd dolt commit` (durability), and on a committed-read that
	// shows not-closed/errors it errors recoverably.
	doltCommitFn      = bead.DoltCommit
	verifyCommittedFn = defaultVerifyCommitted
	// findOrphanedClosedBeadsFn detects sibling lifecycle beads closed via a
	// bare `bd close` (without `mindspec complete`) — their bead/<id> branch
	// exists and is NOT merged into the spec branch (bead mindspec-4gsz). The
	// shared predicate is reused by `mindspec next` and `mindspec doctor`.
	// Tests swap this to drive the chicken-and-egg guard without a real repo.
	findOrphanedClosedBeadsFn = lifecycle.FindOrphanedClosedBeads
)

// defaultVerifyCommitted is the production committed-state verifier for the
// spec 098 Req 2 post-close durability gate. Per the HONESTY-CLAUSE above,
// it re-reads the bead AFTER the forced `bd dolt commit` via fetchBeadAsOfFn
// (the `bd show --as-of HEAD --json` committed-state path, bd >= 1.0.4,
// bead mindspec-uopd) and returns an error unless the bead reads back
// closed.
//
// Graceful degradation: when the --as-of invocation itself fails with bd's
// `unknown flag: --as-of` signature (bead.IsUnsupportedFlagError) — an
// older, pre-1.0.4 bd binary that does not recognize the flag — this falls
// back to the same-read path (fetchBeadByIDFn, `bd show --json`) and logs a
// one-line warning that the committed-state read is unavailable and
// verification is downgraded. Any OTHER --as-of failure (bead not found,
// Dolt lock contention, ...) is a genuine read error, not an
// unsupported-flag signal, and is never treated as a fallback trigger.
//
// A hard read failure or a not-closed status in EITHER path surfaces as an
// error so the caller keeps the worktree and never proceeds on an
// unverified close.
func defaultVerifyCommitted(beadID string) error {
	info, err := fetchBeadAsOfFn(beadID, "HEAD")
	if err != nil {
		if bead.IsUnsupportedFlagError(err, "as-of") {
			fmt.Fprintf(os.Stderr,
				"event=complete.committed_read_downgraded bead=%s reason=%q\n",
				beadID, "bd show --as-of unsupported by installed bd (bd < 1.0.4) — falling back to same-read verification (bd show)")
			return verifyCommittedSameRead(beadID)
		}
		return fmt.Errorf("committed-state re-read (--as-of HEAD) failed: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(info.Status), "closed") {
		return fmt.Errorf("committed-state re-read (--as-of HEAD) shows status %q (not closed)", strings.TrimSpace(info.Status))
	}
	return nil
}

// verifyCommittedSameRead is the pre-mindspec-uopd fallback verifier: a
// same-read re-check via fetchBeadByIDFn (`bd show --json`, no committed-
// state distinction). Used only when fetchBeadAsOfFn reports the --as-of
// flag is unsupported by the installed bd.
func verifyCommittedSameRead(beadID string) error {
	info, err := fetchBeadByIDFn(beadID)
	if err != nil {
		return fmt.Errorf("committed-state re-read failed: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(info.Status), "closed") {
		return fmt.Errorf("committed-state re-read shows status %q (not closed)", strings.TrimSpace(info.Status))
	}
	return nil
}

// Spec 096 final-review (mindspec-2u0u, persona-closeverify): the
// post-close status re-read is RETRIED a small bounded number of times
// before any decision. A genuine silent close-loss can correlate with a
// TRANSIENT Dolt read failure (lock contention) — without a retry, that
// correlated case would slip through the old "tolerate + proceed to
// merge" branch and complete an UNVERIFIED close. The retry lets a
// transient lock clear so the re-read converges to a definitive
// closed/open status; only a PERSISTENT read failure across all attempts
// triggers the recoverable soft-block. Both seams are package vars so
// tests can shrink the count and no-op the backoff (no real sleeps).
var (
	// postCloseReadAttempts bounds the post-close re-read. Default 3:
	// one immediate read plus two retries — enough for a transient Dolt
	// lock to clear without materially slowing a healthy complete.
	postCloseReadAttempts = 3
	// postCloseReadBackoff sleeps between failed re-read attempts. The
	// argument is the zero-based attempt index just completed. Injectable
	// so tests run instantly.
	postCloseReadBackoff = func(attempt int) {
		time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
	}
)

// CompleteOpts holds options for bead completion.
//
// Spec 086 Bead 3: `AllowDocSkew` activates the doc-sync override gate.
// Empty string means "no override". A non-empty string is interpreted
// as the human-readable reason; it is recorded as
// `mindspec_doc_skew_reason` (alongside `_by` and `_at`) on the bead's
// metadata AFTER the terminal mutation (`exec.CompleteBead`) returns
// nil — symmetric with ApproveImpl's post-FinalizeEpic write
// discipline. If CompleteBead fails, the metadata is not written —
// the failure itself is the audit trail.
type CompleteOpts struct {
	AllowDocSkew string

	// OverrideADR is the human-readable reason for bypassing the
	// ADR-divergence gate. Empty string means "no override". A
	// non-empty string causes the gate to be SKIPPED (treated as
	// passed) regardless of detected divergence. After the terminal
	// mutation (`exec.CompleteBead`) returns nil the reason is
	// recorded on the bead's metadata under the
	// `mindspec_adr_override_*` namespace (reason / by / at).
	// Spec 087 Bead 3.
	OverrideADR string

	// SupersedeADR is the user-supplied ADR ID (e.g. "ADR-0099") for
	// the supersede flow. Empty string means "no supersede". When set:
	//   1. A placeholder ADR is pre-created on disk at the supplied
	//      ID via `adr.CreateWithID` with `Status: Proposed` and
	//      Domain(s) seeded from the first uncovered
	//      DivergenceFinding's Domain. This happens BEFORE the
	//      gate-skip decision so the file exists even when downstream
	//      steps fail.
	//   2. The ADR-divergence gate is SKIPPED (same semantics as
	//      OverrideADR).
	//   3. After `exec.CompleteBead` returns nil the four
	//      `mindspec_adr_supersede_*` keys (id / reason / by / at)
	//      are written to bead metadata.
	// OverrideADR and SupersedeADR are mutually exclusive at the CLI
	// layer; the override metadata namespaces are distinct.
	// Spec 087 Bead 3.
	SupersedeADR string
}

// Result summarizes what mindspec complete did.
type Result struct {
	BeadID          string
	BeadClosed      bool
	WorktreeRemoved bool
	NextMode        string
	NextBead        string
	NextSpec        string
	SpecWorktree    string
}

func defaultFindLocalRoot() (string, error) {
	return workspace.FindLocalRoot(".")
}

// Run orchestrates bead completion: close bead, remove worktree, advance state.
// root is the main repo root (for spec dirs, lifecycle, merges).
// beadID is required — it must always be provided by the caller.
// exec is the Executor used for all git/workspace operations.
// specIDHint is optional and typically comes from --spec for disambiguation.
// opts carries lifecycle options including the doc-sync skew override.
func Run(root, beadID, specIDHint, commitMsg string, exec executor.Executor, opts CompleteOpts) (*Result, error) {
	// Determine local root for per-worktree context resolution.
	localRoot := root
	if lr, err := findLocalRootFn(); err == nil {
		localRoot = lr
	}

	// 1. Derive activeSpec from resolver.
	// Try localRoot first (per-worktree context) then fall back to root.
	specID, err := resolveTargetFn(localRoot, specIDHint)
	if err != nil && localRoot != root {
		specID, err = resolveTargetFn(root, specIDHint)
	}
	// If still ambiguous but we have a bead ID, resolve spec from the bead's parent epic.
	if err != nil && beadID != "" {
		if _, derivedSpec, beadErr := findEpicForBeadFn(beadID); beadErr == nil && derivedSpec != "" {
			specID = derivedSpec
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("resolving active spec: %w", err)
	}

	// Spec 107 wave 1 (mindspec-oexu.3): the spec→epic mapping is immutable for
	// the life of this call, so resolve it ONCE through a shared cache and
	// thread that cache through the migration, the impl-only guard, and the
	// post-close state advance. This collapses the four throwaway
	// `phase.NewCache()` + `bd list --type=epic` lookups (migrate, guard,
	// phase-sync, advanceState) down to at most one `bd list --type=epic`. The
	// post-close children read is deliberately NOT memoized here — advanceState
	// re-issues it via the uncached phase.FetchChildren so it observes the
	// child set `bd close` mutated mid-run.
	epicCache := phase.NewCache()

	// 1.25. Spec 089 / ADR-0034: one-shot legacy-to-metadata migration on
	// first lifecycle command. Must precede the phase-dependent guard
	// below (and the eventual phase.DerivePhaseFromChildren call in
	// advanceState) so legacy epics get their mindspec_phase metadata
	// before any phase read. No-op when already migrated or no epic.
	if _, err := phase.EnsureMigratedWithCache(epicCache, specID); err != nil {
		return nil, err
	}

	// 1.5. Impl-only guard: verify the epic phase is implement or review.
	epicID, epicErr := phase.FindEpicBySpecIDWithCache(epicCache, specID)
	if epicErr == nil && epicID != "" {
		epicPhase, phaseErr := phase.DerivePhaseWithCache(epicCache, epicID)
		if phaseErr == nil && epicPhase != state.ModeImplement && epicPhase != state.ModeReview {
			return nil, fmt.Errorf("bead %s belongs to spec %s which is in '%s' phase.\nmindspec complete is for implementation beads only.", beadID, specID, epicPhase)
		}
	}

	// Derive spec branch from conventions
	specBranch := workspace.SpecBranch(specID)

	// 1.6. bd_close lifecycle-bypass guard (bead mindspec-4gsz). Before ANY
	// mutation, block if some OTHER sibling bead under this epic was closed
	// without `mindspec complete` — its bead/<id> branch exists and is NOT an
	// ancestor of the spec branch, so its work is unmerged and ungated. The
	// shared predicate (also used by `mindspec next` and `mindspec doctor`)
	// applies the IsAncestor confirmation so a benign merged-but-undeleted
	// branch is not flagged. excludeBeadID = beadID avoids the chicken-and-egg
	// of blocking on the very bead being completed (it may itself be an
	// orphaned-yet-being-recovered branch — that is exactly what this run
	// converges).
	if orphans := findOrphanedClosedBeadsFn(specID, root, beadID); len(orphans) > 0 {
		o := orphans[0]
		return nil, fmt.Errorf("bead %s was closed without `mindspec complete` — its branch %s is unmerged into %s (closed-but-unmerged).\nRun `mindspec complete %s` to recover, then re-run `mindspec complete %s`.",
			o.BeadID, o.BeadBranch, o.SpecBranch, o.BeadID, beadID)
	}

	// 2. Find worktree matching bead (needed for commit/clean-tree paths).
	// The same resolution also pins beadHead — the ref the per-bead
	// gates (step 3.5) measure against: the matched worktree's actual
	// branch when one exists, else the canonical bead branch name
	// (mindspec-aqey / mindspec-perm anchoring).
	var wtPath string
	beadHead := workspace.BeadBranch(beadID)
	entries, err := worktreeListFn()
	if err == nil {
		expectedName := workspace.BeadWorktreeName(beadID)
		expectedBranch := workspace.BeadBranch(beadID)
		for _, e := range entries {
			if e.Name == expectedName || e.Branch == expectedBranch {
				wtPath = e.Path
				if e.Branch != "" {
					beadHead = e.Branch
				}
				break
			}
		}
	}

	// 2.25. AUTHORITATIVE panel gate (Spec 099 Bead 2, R1+R2+R5; ADR-0037).
	// This is the in-binary enforcement point — it runs over the DECLARED
	// beadID (no shell parsing; ADR-0036) and BEFORE step-2.5 exec.CommitAll,
	// bd close (step 4), and the bead→spec merge (step 5). The ordering is
	// load-bearing: CommitAll advances the bead/<id> tip PAST
	// reviewed_head_sha (false-firing §4 staleness) and clears user dirt
	// (false-clearing §5), so the gate measures the PRE-CommitAll beadHead
	// tip. It calls the SAME panel.PanelGateDecision over panel.GateFacts
	// from the SAME panel.ResolveGateFacts the PreToolUse hook uses — the
	// hook is now a defense-in-depth backstop, and the two cannot disagree.
	//
	// On a Block it returns a guard.NewFailure (fence in the body + a genuine
	// recovery line) exiting non-zero having mutated nothing (HC-4). §6
	// fail-open (no panel.json → completes) and the §7 hatches
	// (MINDSPEC_SKIP_PANEL never named in a block — HC-7; enforcement.panel_gate)
	// are honored. The matched registration is reused below for the
	// post-completion audit writes (Reqs 13b/13e).
	advisoryOut := panelAdvisoryOut
	if advisoryOut == nil {
		advisoryOut = os.Stderr
	}
	// CONFIG: cfg is loaded at step 5.5 (AFTER this gate); read an EARLIER
	// copy here for the enforcement.panel_gate toggle (default true) AND
	// (spec 109 R8) the PanelExpectedReviewers() default the reviewer-count
	// advisory below compares against. gateCfgErr is deliberately non-fatal
	// here (mirrors the pre-109 behavior of leaving panelGateEnabled at its
	// true default on a load error) — the advisory below is simply skipped
	// on that same error, never blocking the gate.
	gateCfg, gateCfgErr := config.Load(root)
	panelGateEnabled := true
	if gateCfgErr == nil {
		panelGateEnabled = gateCfg.Enforcement.PanelGate
	}
	// Spec 106 Bead 4 (AC13): the scan roots are LAYOUT-AWARE — on a
	// canonical/legacy tree the gate honors BOTH the repo-root review/ and the
	// co-located <spec-dir>/reviews/ panels (the transition union); on a flat
	// tree it honors the co-located reviews ONLY (root review/ ignored once
	// flat). panelGateRoots picks the set from workspace.DetectLayout.
	panelReg, panelGateErr := panelGate(beadID, panelGateRoots(root, wtPath, specID), wtPath, panelGateEnabled, advisoryOut)
	if panelGateErr != nil {
		return nil, panelGateErr
	}
	// Caller-side panel.ReviewerCountNote advisory (spec 109 R8): the
	// Allow/Block decision above is already final; this only surfaces a
	// legitimately smaller/larger substituted reviewer quorum, never
	// altering it.
	if gateCfgErr == nil {
		reviewerCountAdvisory(panelReg, gateCfg.PanelExpectedReviewers(), advisoryOut)
	}

	// 2.5. Auto-commit if commit message provided (via Executor)
	commitPath := wtPath
	if commitPath == "" {
		commitPath = root
	}
	if commitMsg != "" {
		msg := fmt.Sprintf("impl(%s): %s", beadID, commitMsg)
		if err := exec.CommitAll(commitPath, msg); err != nil {
			return nil, fmt.Errorf("auto-commit failed: %w", err)
		}
	}

	// 3. Artifact-aware clean-tree check (spec 092 Reqs 6/7, ADR-0025,
	// mindspec-i4ad). The classification is the exact one `mindspec next`
	// uses (next.CheckDirtyTreeDetail, DQ-2): artifact dirt
	// (.beads/issues.jsonl) is normalized via `bd export` and never
	// blocks; only user-authored dirt blocks. checkPath is passed as BOTH
	// repoRoot and cwd so the normalization targets the same checkout
	// whose status is being classified (bead.Export writes
	// <workdir>/.beads/issues.jsonl).
	checkPath := wtPath
	if checkPath == "" {
		checkPath = root // No worktree — check main tree
	}
	artifactDirt, userDirt, dirtErr := checkDirtyTreeFn(checkPath, checkPath)
	if dirtErr != nil {
		return nil, fmt.Errorf("checking working tree: %w", dirtErr)
	}
	if len(userDirt) > 0 {
		// User dirt blocks even when artifact dirt coexists — the
		// artifact handling below must never mask user-authored changes.
		msg := fmt.Sprintf("workspace has uncommitted user changes:\n  %s\n(.beads/issues.jsonl is auto-handled per ADR-0025 and never blocks)",
			strings.Join(userDirt, "\n  "))
		if wtPath == "" {
			msg += "\nno active bead worktree is set — claim work with `mindspec next`, commit in the printed worktree, then rerun `mindspec complete`"
		}
		// Spec 092 Req 8 (mindspec-tjat): worktree-context line naming
		// where the command ran vs. the checkout this guard evaluated.
		// Last body line — it precedes the final recovery line (Req 12
		// ordering). Getwd failure falls back to checkPath: a degraded
		// but truthful context line beats no line.
		cwd, cwdErr := completeGetwdFn()
		if cwdErr != nil || cwd == "" {
			cwd = checkPath
		}
		msg += "\n" + workspace.ContextLine(cwd, checkPath)
		if wtPath == "" {
			return nil, guard.NewFailure(msg, "mindspec next")
		}
		// Existing auto-commit hint, now a Req 12 recovery line.
		return nil, guard.NewFailure(msg,
			fmt.Sprintf("mindspec complete %s \"describe what you did\"", beadID))
	}
	if len(artifactDirt) > 0 {
		// Req 7 (DQ-4): artifact dirt that survives normalization (e.g.
		// a pre-commit hook re-exported the JSONL during the auto-commit
		// above) is COMMITTED, not ignored — as a follow-up commit, never
		// an amend — so the bead→spec merge below operates on a genuinely
		// clean tree and field workarounds (--no-verify, core.hooksPath)
		// are never necessary. The commit stays behind the executor
		// (ADR-0030); CommitAll re-exports the JSONL from Dolt before
		// staging (ADR-0025 §3), so the committed bytes match Dolt.
		if err := exec.CommitAll(checkPath, "chore: sync beads artifact"); err != nil {
			return nil, fmt.Errorf("committing beads artifact sync: %w", err)
		}
		// HC-3: self-heal is silent-on-success save one structured line.
		fmt.Fprintf(os.Stderr, "event=complete.artifact_synced paths=%s\n",
			strings.Join(artifactDirt, ","))
	}

	// 3.5. Spec 086 (F2) doc-sync enforcement gate. The measured range
	// is anchored to the BEAD's own work: base is the bead branch's
	// fork point from the spec branch (merge-base(specBranch,
	// beadHead)) and head is the bead branch tip (beadHead, resolved
	// at step 2). It is deliberately NOT relative to the ambient HEAD
	// of whatever checkout this process runs from — that measured
	// main-side drift from the repo root (false blocks,
	// mindspec-aqey) and an empty range from the spec worktree
	// (vacuous passes, mindspec-perm). The whole-branch backstop at
	// impl approve (internal/approve/impl.go) keeps its explicit
	// main..specBranch refs and is unaffected. The
	// `--allow-doc-skew "<reason>"` override allows the gate to pass
	// without doc updates; the reason is recorded on bead metadata
	// only AFTER the terminal mutation (`exec.CompleteBead`) succeeds
	// (see step 5.5 below).
	base, mbErr := exec.MergeBase(specBranch, beadHead)
	if mbErr != nil {
		return nil, fmt.Errorf("computing merge-base of %s and %s for the per-bead gates: %w", specBranch, beadHead, mbErr)
	}
	// OWNERSHIP attribution (manifests + domain enumeration) is read
	// from beadHead — the SAME ref the gate diffs — so an OWNERSHIP
	// claim committed on the bead branch satisfies its own gate with no
	// override (spec 095 / mindspec-vvs9).
	docResult := validate.ValidateDocsRange(root, base, beadHead, beadHead, exec)
	// Spec 091 Req 22(a): surface warning-severity issues BEFORE the
	// failure decision so they print on every run — including when
	// HasFailures() is false and the flow proceeds normally, and on
	// the override/error paths.
	printResultWarnings(warnWriter, docResult)
	if docResult.HasFailures() {
		if opts.AllowDocSkew == "" {
			return nil, fmt.Errorf("doc-sync: %s\nhint: re-run with --allow-doc-skew \"<reason>\" to override (records the reason in bead metadata)", joinResultErrorMessages(docResult))
		}
		// Override path: fall through. Metadata is written AFTER the
		// terminal mutation succeeds (panel CONSENSUS revision 4).
	}
	specDir, sdErr := workspace.SpecDir(root, specID)
	if sdErr != nil {
		return nil, fmt.Errorf("resolving spec dir for adr-divergence: %w", sdErr)
	}
	// The gate runs EXACTLY ONCE (revision 3 — no probe-call). The
	// findings slice is consumed by the supersede path below to seed
	// the placeholder ADR's Domains field. The failure-decision is
	// bypassed when either override or supersede flag is set. Same
	// bead-branch anchoring as doc-sync above: base..beadHead.
	// Ownership ref = beadHead, same as doc-sync above (spec 095).
	adrResult, adrFindings := validate.CheckADRDivergence(root, base, exec, specDir, beadID, beadHead, beadHead)
	// Same severity-generic pipe for the ADR-divergence gate: any
	// SevWarning the gate emits (e.g. adr-divergence-proposed) renders
	// without further wiring. No-op while the gate emits none.
	printResultWarnings(warnWriter, adrResult)

	// Pre-create the placeholder ADR FIRST when --supersede-adr is
	// requested, so the new file exists on disk even if a downstream
	// step fails (Bead 3 step 4 ordering rule).
	var supersedeNewID string
	if opts.SupersedeADR != "" {
		// Seed Domains from the structured findings slice (revision 2
		// — NO fmt.Sprintf parsing of Issue messages). When no
		// violation exists the seed list is empty and the operator
		// fills it in later when editing the placeholder.
		var seedDomains []string
		for _, f := range adrFindings {
			if f.Kind == "uncovered" && f.Domain != "" {
				seedDomains = []string{f.Domain}
				break
			}
		}
		title := "Placeholder for " + opts.SupersedeADR
		if _, err := adrCreateWithIDFn(root, opts.SupersedeADR, title, adr.CreateOpts{Domains: seedDomains}); err != nil {
			return nil, fmt.Errorf("--supersede-adr: %w", err)
		}
		supersedeNewID = opts.SupersedeADR
	}

	// Gate-failure decision: only fatal when no override/supersede
	// flag is set.
	if opts.OverrideADR == "" && opts.SupersedeADR == "" && adrResult.HasFailures() {
		return nil, adrDivergenceFailure(beadID, joinResultErrorMessages(adrResult))
	}

	// 4. Close bead (idempotent: tolerate already-closed beads)
	if err := closeBeadFn(beadID); err != nil {
		// Check if the bead is already closed — if so, warn and continue cleanup.
		info, fetchErr := fetchBeadByIDFn(beadID)
		if fetchErr == nil && strings.EqualFold(strings.TrimSpace(info.Status), "closed") {
			fmt.Printf("Warning: bead %s already closed — performing merge and cleanup.\n", beadID)
		} else {
			return nil, fmt.Errorf("closing bead: %w", err)
		}
	} else {
		// Spec 096 Req 2 (mindspec-2u0u): closeBeadFn returned nil, but a
		// nil return does NOT prove the close PERSISTED. A lost/raced
		// Dolt close can return success while `bd show` still reports
		// `in_progress` with `closed_at None` (the spec-092 Bead 7
		// symptom: prints `closed`, exits 0, yet the bead stays open —
		// violating the "exit codes never lie" invariant). Re-read the
		// persisted status and decide across THREE cases, mirroring the
		// already-closed branch above and reusing its exact predicate
		// (strings.EqualFold(strings.TrimSpace(status), "closed")).
		//
		// Final-review (persona-closeverify): the re-read is RETRIED a
		// bounded number of times (postCloseReadAttempts) before any
		// decision. A genuine close-loss can correlate with a TRANSIENT
		// Dolt read error (lock contention); the retry lets that lock
		// clear so the re-read converges to a DEFINITIVE closed/open
		// status instead of slipping through a "tolerate + proceed"
		// branch on an unverified close. Only a PERSISTENT read failure
		// across every attempt reaches case (c).
		var info next.BeadInfo
		var fetchErr error
		for attempt := 0; attempt < postCloseReadAttempts; attempt++ {
			info, fetchErr = fetchBeadByIDFn(beadID)
			if fetchErr == nil {
				break
			}
			if attempt < postCloseReadAttempts-1 {
				postCloseReadBackoff(attempt)
			}
		}
		switch {
		case fetchErr != nil:
			// (c) The re-read STILL errors after every bounded retry — the
			// close could NOT be VERIFIED. The OLD behavior tolerated this
			// (warn + proceed to merge + worktree removal), but a genuine
			// silent close-loss can surface as exactly this persistent read
			// error (closeBeadFn returns nil + the re-read errors), so
			// proceeding would complete an UNVERIFIED close — exit 0 with
			// the bead still in_progress, re-exposing the silent close-loss
			// class spec 096 exists to kill. Mirror case (b)'s PRE-MERGE
			// return instead: a RECOVERABLE soft-block (ADR-0035 recovery
			// line) that KEEPS the worktree and does NOT set BeadClosed.
			// Safe to re-run once bd/Dolt is reachable (the close step is
			// idempotent). A transient error that RESOLVES on retry never
			// reaches here — it converges to case (a)/(b) above — so a
			// legitimately-closed complete whose Dolt was briefly slow is
			// NOT false-blocked.
			msg := fmt.Sprintf(
				"bead %s close returned success but the post-close status re-read could not be VERIFIED after %d attempts (last error: %v) — the close was NOT confirmed to persist.\n"+
					"this state is recoverable: the worktree is kept and the close step is idempotent — re-run `mindspec complete %s` once bd/Dolt is reachable and it converges.",
				beadID, postCloseReadAttempts, fetchErr, beadID)
			return nil, guard.NewFailure(msg, fmt.Sprintf("mindspec complete %s", beadID))
		case strings.EqualFold(strings.TrimSpace(info.Status), "closed"):
			// (a) Re-read AFFIRMS closed — but a SESSION re-read of "closed"
			// does NOT prove the close PERSISTED to committed Dolt state
			// (spec 098 Req 2, mindspec-9n2h: the 2u0u recurrence — a
			// non-persisting close still reads back "closed" via the same
			// bd/Dolt path the close wrote through, and the old case (a)
			// proceeded to merge + worktree removal on exit 0). Before
			// accepting case (a), FORCE durability with `bd dolt commit`
			// (idempotent: a clean working set is a no-op success) and then
			// VERIFY via a committed-state re-read. On a commit FAILURE or a
			// committed read that shows not-closed/errors, return a
			// recoverable ADR-0035 soft-block that KEEPS the worktree, does
			// NOT set BeadClosed, and is safe to re-run (both the close and
			// `bd dolt commit` are idempotent) — mirroring case (b)'s
			// PRE-MERGE return. complete must NEVER print `closed` + exit 0
			// on an unverified/uncommitted close.
			if commitErr := doltCommitFn(); commitErr != nil {
				msg := fmt.Sprintf(
					"bead %s close returned success and re-read as closed, but the forced `bd dolt commit` to make the close DURABLE failed (%v) — the close was NOT confirmed to persist.\n"+
						"this state is recoverable: the worktree is kept and both the close and `bd dolt commit` are idempotent — re-run `mindspec complete %s` once bd/Dolt is reachable and it converges.",
					beadID, commitErr, beadID)
				return nil, guard.NewFailure(msg, fmt.Sprintf("mindspec complete %s", beadID))
			}
			if verifyErr := verifyCommittedFn(beadID); verifyErr != nil {
				msg := fmt.Sprintf(
					"bead %s close returned success but the post-commit committed-state verification did NOT confirm the close persisted (%v) — the close was NOT confirmed to persist.\n"+
						"this state is recoverable: the worktree is kept and the close step is idempotent — re-run `mindspec complete %s` and it converges once the close persists.",
					beadID, verifyErr, beadID)
				return nil, guard.NewFailure(msg, fmt.Sprintf("mindspec complete %s", beadID))
			}
			// Forced commit + committed-state verify both passed — proceed.
		default:
			// (b) Re-read AFFIRMS open/in_progress: the REAL silent
			// close-loss bug (mindspec-2u0u). closeBeadFn returned nil but
			// the close did NOT persist. Surface a HARD error + non-zero
			// exit so `complete` NEVER prints `closed` + exit 0 on an
			// unpersisted close (ADR-0035 recovery line).
			msg := fmt.Sprintf(
				"bead %s close returned success but a re-read shows it is still %q (not closed) — the close did NOT persist (silent close-loss).\n"+
					"this state is recoverable: re-run `mindspec complete %s` — the close step is idempotent and converges once the close persists.",
				beadID, strings.TrimSpace(info.Status), beadID)
			return nil, guard.NewFailure(msg, fmt.Sprintf("mindspec complete %s", beadID))
		}
	}

	// 4.5. Emit recording bead marker (best-effort)
	if specID != "" {
		_ = recording.EmitBeadMarker(root, specID, "complete", beadID)
	}

	cfg, cfgErr := config.Load(root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	result := &Result{
		BeadID:       beadID,
		BeadClosed:   true,
		SpecWorktree: workspace.SpecWorktreePath(root, cfg, specID),
	}

	// 5. Merge bead→spec, remove worktree, delete branch (via Executor).
	// Pass empty msg since we already handled commit+clean-tree above.
	completeErr := exec.CompleteBead(beadID, specBranch, "")
	if completeErr == nil {
		result.WorktreeRemoved = true
	}

	// Spec 092 Req 3c (mindspec-qxsy): CompleteBead may have removed the
	// very directory this process was invoked from — running `complete`
	// from inside the bead worktree is supported. Move to the repo root
	// NOW, before any bd subprocess below: advanceState swallows all bd
	// errors and would silently degrade to ModeIdle when those
	// subprocesses are spawned from a deleted cwd, producing a false
	// `Mode: idle` AND skipping the mindspec_phase sync at step 6.5 —
	// recreating the exact stale-phase condition the Req 1 reconcile
	// exists to heal. The chdir lives INSIDE Run (not at the cmd layer)
	// so the metadata writes and advanceState are protected for every
	// caller.
	if chdirErr := os.Chdir(root); chdirErr != nil {
		fmt.Printf("Warning: could not chdir to repo root %s: %v\n", root, chdirErr)
	}

	// Spec 092 Req 14(a) incident amendment (2026-06-11 merge-driver
	// incident): a CompleteBead failure used to be downgraded to
	// `Warning: bead cleanup: ...` and Run continued to exit 0 —
	// leaving a closed-but-unmerged bead the lifecycle could not see.
	// HC-4: the bead→spec merge is part of complete's terminal
	// mutation, so its failure must surface as a non-zero exit.
	//
	// ORDERING DECISION (incident amendment iii): close-before-merge is
	// KEPT. Closing first keeps Dolt — the single state authority
	// (ADR-0023) — ahead of the git projection, so the merge commit's
	// re-exported issues.jsonl (ADR-0025 §3) records the bead as
	// closed; merge-before-close would invert that and require
	// splitting CompleteBead's merge from its cleanup. The
	// closed-but-unmerged window this leaves is made EXPLICIT (named
	// in the error below, never hidden behind a warning) and
	// RECONVERGENT: the close step is idempotent (step 4 above
	// tolerates already-closed beads), so after the operator resolves
	// the merge, re-running `mindspec complete <bead-id>` converges —
	// the re-attempted merge sees the bead branch as an ancestor and
	// cleanup proceeds.
	if completeErr != nil {
		msg := fmt.Sprintf(
			"bead %s is CLOSED in Dolt but completion did NOT finish — its branch may not be merged into %s (closed-but-unmerged).\n"+
				"this state is recoverable: fix the cause below, then re-run `mindspec complete %s` — the close step is idempotent and completion converges.\n"+
				"cause: %v",
			beadID, specBranch, beadID, completeErr)
		if guard.HasFinalRecoveryLine(msg) {
			// The executor failure already carries Req 12 recovery
			// lines (e.g. the conflict-abort failures) — keep them
			// final instead of stacking a redundant generic one.
			return nil, errors.New(msg)
		}
		return nil, guard.NewFailure(msg, fmt.Sprintf("mindspec complete %s", beadID))
	}

	// 5.5. Spec 086 (F2): record doc-sync skew override AFTER the
	// terminal bead→spec merge (`exec.CompleteBead`) returns nil. This
	// mirrors ApproveImpl's post-FinalizeEpic discipline — the override
	// metadata write must be symmetric with the terminal mutation, not
	// just the prior `closeBeadFn` step. If CompleteBead failed we skip
	// the write; the failure itself is the audit trail (panel CONSENSUS
	// revision 4). Best-effort: a metadata write failure surfaces as a
	// warning print but does not fail the lifecycle.
	if opts.AllowDocSkew != "" && completeErr == nil {
		meta := buildSkewMetadata(opts.AllowDocSkew,
			"mindspec_doc_skew_reason",
			"mindspec_doc_skew_at",
			"mindspec_doc_skew_by",
		)
		if err := completeMergeMetadataFn(beadID, meta); err != nil {
			fmt.Printf("Warning: could not record doc-skew override metadata on %s: %v\n", beadID, err)
		}
	}

	// Spec 087 Bead 3: record ADR-divergence override metadata on
	// the bead AFTER the terminal mutation succeeds, mirroring the
	// doc-skew discipline above. The keys live under DISTINCT
	// namespaces (`mindspec_adr_override_*` vs
	// `mindspec_adr_supersede_*`) per spec.md Requirement 13.
	if opts.OverrideADR != "" && completeErr == nil {
		meta := buildSkewMetadata(opts.OverrideADR,
			"mindspec_adr_override_reason",
			"mindspec_adr_override_at",
			"mindspec_adr_override_by",
		)
		if err := completeMergeMetadataFn(beadID, meta); err != nil {
			fmt.Printf("Warning: could not record adr-override metadata on %s: %v\n", beadID, err)
		}
	}
	if opts.SupersedeADR != "" && completeErr == nil {
		// Auto-fill the reason when no separate --override-adr reason
		// was passed alongside (these flags are mutually exclusive at
		// the CLI, but defending in depth in case a future direct
		// caller passes both).
		reason := opts.OverrideADR
		if reason == "" {
			reason = "superseded by " + supersedeNewID
		}
		meta := map[string]interface{}{
			"mindspec_adr_supersede_id":     supersedeNewID,
			"mindspec_adr_supersede_reason": reason,
			"mindspec_adr_supersede_at":     time.Now().UTC().Format(time.RFC3339),
			"mindspec_adr_supersede_by":     gitUserEmailFn(),
		}
		if err := completeMergeMetadataFn(beadID, meta); err != nil {
			fmt.Printf("Warning: could not record adr-supersede metadata on %s: %v\n", beadID, err)
		}
	}

	// 5.6. Panel-gate audit writes (Spec 093 Reqs 13b/13e): record
	// panel_gate_skipped (env hatch used against a registered panel) and/or
	// panel_abandoned (matched panel.json marked abandoned) AFTER the
	// terminal mutation succeeds, mirroring the doc-skew discipline above.
	// Best-effort; reuses the panelAdvisory scan (no second fs walk).
	if completeErr == nil {
		writePanelAuditMetadata(beadID, panelReg, advisoryOut)
	}

	// 6. Advance state. advanceState re-reads the post-close child set fresh
	// (phase.FetchChildren) against the epic resolved once above.
	nextMode, nextBead := advanceState(epicID)
	result.NextMode = nextMode
	result.NextBead = nextBead
	result.NextSpec = specID

	// 6.5. Sync stored phase (Spec 080): keep epic mindspec_phase in sync
	// so that DerivePhase (metadata-first) returns the correct phase for
	// downstream commands like `mindspec impl approve`. Reuses the epicID
	// resolved once at the top (no extra `bd list --type=epic`).
	if nextMode != "" && epicID != "" {
		_ = completeMergeMetadataFn(epicID, map[string]interface{}{"mindspec_phase": nextMode})
	}

	// ADR-0023: no focus write — state is derived from beads.
	if nextMode == state.ModeIdle {
		result.NextSpec = ""
	}

	return result, nil
}

// FormatResult returns a human-readable summary of the completion.
func FormatResult(r *Result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Bead %s closed.\n", r.BeadID)
	if r.WorktreeRemoved {
		sb.WriteString("Worktree removed.\n")
	}
	switch r.NextMode {
	case state.ModeImplement:
		fmt.Fprintf(&sb, "Next bead ready: %s\n", r.NextBead)
		fmt.Fprintf(&sb, "Mode: implement (spec: %s)\n", r.NextSpec)
		// Spec 092 Req 4 (mindspec-qxsy): the implement branch carries
		// the same cd hint as plan/review — the removed bead worktree
		// may have been the shell's cwd.
		if r.WorktreeRemoved && r.SpecWorktree != "" {
			fmt.Fprintf(&sb, "Run: `cd %s`\n", r.SpecWorktree)
		}
		sb.WriteString("\nSTOP HERE. Do NOT run `mindspec next` or claim another bead.\nTell the user: run `/clear` (or start a fresh agent), then `mindspec next` to continue.\n")
	case state.ModePlan:
		fmt.Fprintf(&sb, "Remaining beads are blocked. Mode: plan (spec: %s)\n", r.NextSpec)
		if r.WorktreeRemoved && r.SpecWorktree != "" {
			fmt.Fprintf(&sb, "Run: `cd %s`\n", r.SpecWorktree)
		}
	case state.ModeReview:
		fmt.Fprintf(&sb, "All beads complete. Mode: review (spec: %s)\n", r.NextSpec)
		if r.WorktreeRemoved && r.SpecWorktree != "" {
			fmt.Fprintf(&sb, "Run: `cd %s`\n", r.SpecWorktree)
		}
		sb.WriteString("Run `mindspec instruct` for review guidance and next steps.\n")
	default:
		sb.WriteString("All beads complete. Mode: idle\n")
	}
	return sb.String()
}

// advanceState determines the next mode after completing a bead.
//
// Phase is derived authoritatively via phase.DerivePhaseFromChildren against
// the full child-status mix (open, in_progress, blocked, closed, and every
// custom status declared in the project's .beads/config.yaml). Earlier
// revisions only queried `--status=open`, which silently dropped in_progress
// beads held by a parallel agent and any custom status, causing premature
// flips to review mode.
//
// If phase derives to implement, a `bd ready` call resolves a specific next
// bead; otherwise nextBead stays empty and the caller prints the right
// guidance for plan / review / idle.
//
// epicID is the spec's epic, resolved once by Run and passed in ("" when no
// epic exists → idle). The child set is re-read FRESH via phase.FetchChildren
// (a single all-status `bd list --parent` call, uncached) so it reflects the
// `bd close` this run just performed rather than a memoized pre-close view.
func advanceState(epicID string) (mode, nextBead string) {
	if epicID == "" {
		return state.ModeIdle, ""
	}

	children, _ := phase.FetchChildren(epicID)
	derivedPhase := phase.DerivePhaseFromChildren(children)

	if derivedPhase == state.ModeImplement {
		if out, rerr := runBDFn("ready", "--parent", epicID, "--json"); rerr == nil {
			var ready []bead.BeadInfo
			if json.Unmarshal(out, &ready) == nil && len(ready) > 0 {
				return state.ModeImplement, ready[0].ID
			}
		}
		// Implement phase without a ready bead: we're between beads (next one
		// is blocked on a dep that just closed but hasn't propagated, or the
		// only remaining work is in_progress with a peer). Stay in implement
		// without a concrete next bead rather than flipping to review.
		return state.ModeImplement, ""
	}

	return derivedPhase, ""
}

// buildSkewMetadata returns a metadata map with the override reason,
// an RFC3339-UTC timestamp, and a best-effort actor identity, keyed by
// the caller-provided field names. Spec 086 Bead 3.
func buildSkewMetadata(reason, reasonKey, atKey, byKey string) map[string]interface{} {
	return map[string]interface{}{
		reasonKey: reason,
		atKey:     time.Now().UTC().Format(time.RFC3339),
		byKey:     gitUserEmailFn(),
	}
}

// adrDivergenceFailure formats the ADR-divergence gate failure with the
// repair-first triage ladder (spec 093 Req 2, replacing the bypass-first
// `--override-adr`/`--supersede-adr` hint and the merge-skill prose that
// was folded into ms-bead-cycle per spec 093). findings is
// joinResultErrorMessages(adrResult) — it carries
// the offending file names, so the ladder is actionable without any
// skill. Ladder order is deliberate: repair (OWNERSHIP.yaml), then
// revert, and only LAST the bypass flags. Per HC-5 the ladder lives in
// the body and the final `recovery:` lines carry the re-run and bypass
// commands (guard.NewFailure, ADR-0035 convention).
func adrDivergenceFailure(beadID, findings string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "adr-divergence: %s\n", findings)
	b.WriteString("triage before bypassing (repair-first):\n")
	b.WriteString("  1. the file belongs to this bead's domain → add it to the relevant\n")
	b.WriteString("     .mindspec/domains/<name>/OWNERSHIP.yaml and re-run\n")
	b.WriteString("  2. the file is an accidental stray edit picked up by auto-stage → revert it and re-run\n")
	b.WriteString("  3. only after 1-2 do not apply, bypass with --override-adr \"<reason>\"\n")
	b.WriteString("     (recorded on bead metadata) or --supersede-adr ADR-NNNN")
	return guard.NewFailure(b.String(),
		fmt.Sprintf("mindspec complete %s   (re-run after the OWNERSHIP.yaml fix or the revert)", beadID),
		fmt.Sprintf("mindspec complete %s --override-adr \"<reason>\"", beadID),
		fmt.Sprintf("mindspec complete %s --supersede-adr ADR-NNNN", beadID))
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

// joinResultErrorMessages flattens SevError-severity Issues from a
// *validate.Result into a single string suitable for fmt.Errorf
// wrapping. Spec 086 Bead 3.
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
