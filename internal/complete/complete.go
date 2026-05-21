package complete

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
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
	closeBeadFn       = bead.Close
	worktreeListFn    = bead.WorktreeList
	runBDFn           = bead.RunBD
	listJSONFn        = bead.ListJSON
	resolveTargetFn   = resolve.ResolveTarget
	findLocalRootFn   = defaultFindLocalRoot
	fetchBeadByIDFn   = next.FetchBeadByID
	findEpicForBeadFn = phase.FindEpicForBead
	mergeMetadataFn   = bead.MergeMetadata
	gitUserEmailFn    = bead.GitUserEmail
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

	// 2. Find worktree matching bead (needed for commit/clean-tree paths)
	var wtPath string
	entries, err := worktreeListFn()
	if err == nil {
		expectedName := workspace.BeadWorktreeName(beadID)
		expectedBranch := workspace.BeadBranch(beadID)
		for _, e := range entries {
			if e.Name == expectedName || e.Branch == expectedBranch {
				wtPath = e.Path
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

	// 3. Check clean tree
	checkPath := wtPath
	if checkPath == "" {
		checkPath = root // No worktree — check main tree
	}
	if err := exec.IsTreeClean(checkPath); err != nil {
		if wtPath == "" {
			return nil, fmt.Errorf("%w\nhint: no active bead worktree is set. Run `mindspec next`, `cd` into the printed worktree path, then commit and rerun `mindspec complete`", err)
		}
		return nil, fmt.Errorf("%w\nhint: use `mindspec complete %s \"describe what you did\"` to auto-commit", err, beadID)
	}

	// 3.5. Spec 086 (F2) doc-sync enforcement gate. Computes the
	// merge-base of HEAD against the spec branch so the gate sees
	// exactly the bead's commits, then runs the doc-sync lane. The
	// `--allow-doc-skew "<reason>"` override allows the gate to pass
	// without doc updates; the reason is recorded on bead metadata
	// only AFTER the terminal mutation (`exec.CompleteBead`) succeeds
	// (see step 5.5 below). ADR divergence is a forward-compatible
	// stub (spec 087) — it is called for AST-anchoring +
	// future-proofing, but currently emits no failures.
	base, mbErr := exec.MergeBase(specBranch, "HEAD")
	if mbErr != nil {
		return nil, fmt.Errorf("computing merge-base for doc-sync: %w", mbErr)
	}
	docResult := validate.ValidateDocs(root, base, exec)
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
	adrResult, _ := validate.CheckADRDivergence(root, base, exec, specDir, beadID)
	if adrResult.HasFailures() {
		// ADR divergence is NOT covered by `--allow-doc-skew` per panel
		// CONSENSUS revision 6. Spec 087 Bead 2 fills the body — failures
		// now block bead completion until citations are updated or the
		// supersede flow (Bead 3) is invoked.
		return nil, fmt.Errorf("adr-divergence: %s", joinResultErrorMessages(adrResult))
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
	if completeErr != nil {
		fmt.Printf("Warning: bead cleanup: %v\n", completeErr)
	} else {
		result.WorktreeRemoved = true
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
		if err := mergeMetadataFn(beadID, meta); err != nil {
			fmt.Printf("Warning: could not record doc-skew override metadata on %s: %v\n", beadID, err)
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
			_ = bead.MergeMetadata(eid, map[string]interface{}{"mindspec_phase": nextMode})
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
