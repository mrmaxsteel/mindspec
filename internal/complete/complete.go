package complete

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
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
	closeBeadFn             = bead.Close
	worktreeListFn          = bead.WorktreeList
	runBDFn                 = bead.RunBD
	listJSONFn              = bead.ListJSON
	resolveTargetFn         = resolve.ResolveTarget
	findLocalRootFn         = defaultFindLocalRoot
	fetchBeadByIDFn         = next.FetchBeadByID
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

	// 1.25. Spec 089 / ADR-0034: one-shot legacy-to-metadata migration on
	// first lifecycle command. Must precede the phase-dependent guard
	// below (and the eventual phase.DerivePhaseFromChildren call in
	// advanceState) so legacy epics get their mindspec_phase metadata
	// before any phase read. No-op when already migrated or no epic.
	if _, err := phase.EnsureMigrated(specID); err != nil {
		return nil, err
	}

	// 1.5. Impl-only guard: verify the epic phase is implement or review.
	epicID, epicErr := phase.FindEpicBySpecID(specID)
	if epicErr == nil && epicID != "" {
		epicPhase, phaseErr := phase.DerivePhase(epicID)
		if phaseErr == nil && epicPhase != state.ModeImplement && epicPhase != state.ModeReview {
			return nil, fmt.Errorf("bead %s belongs to spec %s which is in '%s' phase.\nmindspec complete is for implementation beads only.", beadID, specID, epicPhase)
		}
	}

	// Derive spec branch from conventions
	specBranch := workspace.SpecBranch(specID)

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
	docResult := validate.ValidateDocsRange(root, base, beadHead, exec)
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
	adrResult, adrFindings := validate.CheckADRDivergence(root, base, exec, specDir, beadID, beadHead)

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
		return nil, fmt.Errorf("adr-divergence: %s\nhint: re-run with --override-adr \"<reason>\" or --supersede-adr ADR-NNNN to bypass",
			joinResultErrorMessages(adrResult))
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

	// 6. Advance state
	nextMode, nextBead := advanceState(root, specID)
	result.NextMode = nextMode
	result.NextBead = nextBead
	result.NextSpec = specID

	// 6.5. Sync stored phase (Spec 080): keep epic mindspec_phase in sync
	// so that DerivePhase (metadata-first) returns the correct phase for
	// downstream commands like `mindspec impl approve`.
	if nextMode != "" {
		if eid, findErr := phase.FindEpicBySpecID(specID); findErr == nil && eid != "" {
			_ = completeMergeMetadataFn(eid, map[string]interface{}{"mindspec_phase": nextMode})
		}
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
func advanceState(root, specID string) (mode, nextBead string) {
	if specID == "" {
		return state.ModeIdle, ""
	}

	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return state.ModeIdle, ""
	}

	children := queryAllChildren(root, epicID)
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

// queryAllChildren pulls child beads under an epic across every status bd
// recognizes — built-in (open, in_progress, blocked, closed) plus every
// custom status declared in <root>/.beads/config.yaml. Reading the custom
// set at runtime means new statuses added later (or different per project)
// are picked up without touching this code. Mirrors phase.queryChildren
// (package-private there).
func queryAllChildren(root, epicID string) []phase.ChildInfo {
	statuses := bead.AllStatuses(root)
	var all []phase.ChildInfo
	seen := map[string]bool{}
	for _, status := range statuses {
		out, err := listJSONFn("--parent", epicID, "--status="+status)
		if err != nil {
			continue
		}
		var batch []phase.ChildInfo
		if json.Unmarshal(out, &batch) != nil {
			continue
		}
		for _, c := range batch {
			if !seen[c.ID] {
				seen[c.ID] = true
				all = append(all, c)
			}
		}
	}
	return all
}
