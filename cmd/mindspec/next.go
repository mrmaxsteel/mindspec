package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
	"github.com/spf13/cobra"
)

// nextPhaseMergeMetadataFn is the test seam for advancePhaseOnClaim's
// Step 5.2 mindspec_phase write (spec 128 Bead 2), matching the house
// per-package seam convention (planMergeMetadataFn, completeMergeMetadataFn,
// repairMergeMetadataFn) so a test can observe/stub the write without
// hitting a real bd invocation.
var nextPhaseMergeMetadataFn = bead.MergeMetadata
var nextGateReadinessFn = next.GateReadiness
var nextEnsureWorktreeFn = next.EnsureWorktree
var nextResolveActiveBeadFn = next.ResolveActiveBead

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Discover, claim, and start the next piece of work",
	Long: `Queries Beads for ready work, claims it, updates MindSpec state,
and emits mode-appropriate guidance — going from "what should I do?"
to "here's your bead, here's the mode, here are your rules" in one step.

Use --spec to filter ready work to a specific spec. If multiple active specs
exist and no --spec is given, the command fails with a list of candidates.

An optional positional bead ID is honored on BOTH paths:
  - claim path (default): claim that bead — or error clearly if it is not
    found / not in the ready set — instead of defaulting to the first ready item.
  - --emit-only: build the primer for that bead without claiming.
A positional bead ID and --pick are mutually exclusive selectors; supplying
both is an error.

Use --emit-only to build and emit a bead primer without claiming the bead,
creating a worktree, or updating state. Ideal for multi-agent mode where a
team lead spawns fresh agents per bead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pick, _ := cmd.Flags().GetInt("pick")
		specFlag, _ := cmd.Flags().GetString("spec")
		force, _ := cmd.Flags().GetBool("force")
		emitOnly, _ := cmd.Flags().GetBool("emit-only")
		allowNotReady, _ := cmd.Flags().GetString("allow-not-ready")
		evidencePath, _ := cmd.Flags().GetString("evidence")
		flagGiven := cmd.Flags().Changed("allow-not-ready")
		namedSignals, parseErr := next.ParseReadinessSignals(allowNotReady, flagGiven)
		if parseErr != nil {
			if flagGiven && (allowNotReady == next.BareOverride || allowNotReady == "") {
				return bareReadinessRefusal(args)
			}
			return parseErr
		}

		// Spec 128 Bead 8 round-1 panel G2: --pick < 0 is pure arg
		// validation — checked IMMEDIATELY after flag parsing, before
		// EVERYTHING else on either path (positional-vs-pick mutual
		// exclusion, the --emit-only dispatch below, scope resolution,
		// the dirty-tree guard, the ready query). This is the single
		// shared entry point for both the claim path and --emit-only
		// (runEmitOnly is only ever reached via the dispatch below), so
		// one check here covers both: an invalid pick refuses with ZERO
		// side effects — no bd call, no dirty-tree check, nothing
		// claimed/fetched — rather than surfacing deep inside SelectWork
		// after real work already ran. G4/R5's explicit decision, stated
		// here rather than left implicit: --pick=0 is NOT invalid — it is
		// the documented "unset" sentinel (OQ-5's default-to-row-1 path,
		// SelectWork's own pick==0 branch); only a NEGATIVE value is
		// rejected.
		if pick < 0 {
			return fmt.Errorf("pick %d is invalid: must be >= 0 (0 selects the default)", pick)
		}

		// Spec 101 R1 (mindspec-mfe0): a positional bead ID and --pick are
		// mutually exclusive selectors — the positional names a specific bead,
		// --pick is an index into the ready set, and they may disagree. Reject
		// the conflict up front (pure arg validation), before any gate or claim.
		if len(args) > 0 && pick > 0 {
			return fmt.Errorf("supply exactly one selector: a positional bead ID OR --pick, not both")
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		exec := newExecutor(root)

		// Emit-only mode: build and print primer without claiming or state changes
		if emitOnly {
			return runEmitOnly(root, specFlag, pick, args)
		}

		// Step 0a: --spec prefix resolution (e.g., "079" → "079-location-agnostic-commands")
		if specFlag != "" {
			resolved, resolveErr := resolve.ResolveSpecPrefixProven(root, specFlag)
			if resolveErr != nil {
				return resolveErr
			}
			specFlag = resolved
		}

		// Step 0b: Bead worktree informational note
		kind, _, _ := workspace.DetectWorktreeContext(cwd)
		if kind == workspace.WorktreeBead {
			fmt.Fprintf(os.Stderr, "Note: you're in a bead worktree. Run `mindspec complete <bead-id>` when done.\n")
		}

		// Step 0c: Session freshness gate
		if sess, readErr := state.ReadSession(root); readErr == nil && sess.SessionStartedAt != "" {
			stale := false
			reason := ""
			if sess.SessionSource == "resume" || sess.SessionSource == "compact" {
				stale = true
				reason = "session was " + sess.SessionSource + " (not fresh)"
			} else if sess.BeadClaimedAt != "" && sess.BeadClaimedAt >= sess.SessionStartedAt {
				stale = true
				reason = "a bead was already claimed in this session"
			}
			if stale {
				if !force {
					return fmt.Errorf("session freshness gate: %s.\nYou MUST run /clear to reset your context, then retry.\nDo NOT attempt workarounds — /clear is required.", reason)
				}
				fmt.Fprintf(os.Stderr, "warning: bypassing session freshness gate (%s)\n", reason)
			}
		}

		// Step 0d: Resolve target spec if ambiguous. MOVED ahead of the
		// dirty-tree guard below (spec 128 Bead 7 round-1 panel fix): the
		// guard's entanglement enumeration is per-spec-aware (it must know
		// the target spec's recording paths, if any), so the target spec
		// needs to be known BEFORE the guard runs whenever it is
		// resolvable at all. This does not change behavior for --spec
		// callers (specFlag is already set from Step 0a); for the no-flag
		// path it means an ambiguous-target exit now happens before the
		// dirty-tree check runs rather than after — a genuinely dirty tree
		// AND an ambiguous target is vanishingly rare, and knowing which
		// spec you're even targeting is the more fundamental question.
		if specFlag == "" {
			targetSpec, err := resolve.ResolveTarget(root, "")
			if err != nil {
				if ambErr, ok := err.(*resolve.ErrAmbiguousTarget); ok {
					fmt.Fprintf(os.Stderr, "Multiple active specs have ready beads:\n\n")
					for i, s := range ambErr.Active {
						fmt.Fprintf(os.Stderr, "  %d. %s  (phase: %s)\n", i+1, idrender.Spec(s.SpecID), s.Mode)
					}
					fmt.Fprintf(os.Stderr, "\nAsk the user which spec to work on, then re-run:\n")
					fmt.Fprintln(os.Stderr, nextSpecMenuCommand)
					return fmt.Errorf("select an active spec")
				}
				// Non-ambiguous errors are OK — next will query all ready work
			} else {
				specFlag = targetSpec
			}
		}

		// Guard a known scope before any recovery worktree can be created.
		// With no resolved scope, defer until the readiness gate supplies T:
		// its recording paths must never be reported as unrelated dirt.
		checkClaimDirt := func(specID string) error {
			blockingDirt, ignoredDirt, err := next.CheckDirtyTreeProportional(root, cwd, specID)
			if err != nil {
				return fmt.Errorf("checking working tree: %w", err)
			}
			if len(blockingDirt) > 0 {
				return next.DirtyTreeFailure(cwd, blockingDirt, guard.ActiveWorktreePath(root))
			}
			if len(ignoredDirt) > 0 {
				fmt.Fprintln(os.Stderr, next.IgnoredDirtNotice(ignoredDirt))
			}
			return nil
		}
		guardedSpec := specFlag
		if guardedSpec != "" {
			if err := checkClaimDirt(guardedSpec); err != nil {
				return err
			}
		}

		// Step 1.7: Unmerged-bead guard — block if a predecessor bead was closed
		// without `mindspec complete` (bead branch still exists).
		if specFlag != "" {
			if err := checkUnmergedBeads(specFlag); err != nil {
				return err
			}
		}

		// One global ready query, scoped by plan-bead form, lineage and mapping.
		// Loud emptiness still carries a valid set for the derived-state report.

		rs, err := next.BuildReadySet(root, specFlag)
		var emptyErr *next.EmptyAfterFilterError
		if err != nil && !errors.As(err, &emptyErr) {
			// A failed query cannot proceed to a claim. Preserve dirt
			// visibility on this early exit even when no target was learned.
			if guardedSpec == "" {
				if dirtErr := checkClaimDirt(""); dirtErr != nil {
					return dirtErr
				}
			}
			return err
		}
		items := rs.Filtered
		named := ""
		if len(args) > 0 {
			named = args[0]
		}
		if notice := rs.ExclusionNotice(); notice != "" {
			fmt.Fprintln(os.Stderr, notice)
		}

		if len(items) > 1 {
			fmt.Printf("Ready work (%d items):\n", len(items))
			fmt.Print(next.FormatWorkList(items))
			fmt.Println()
			if pick == 0 && named == "" {
				fmt.Println("Defaulting to item 1 (bd ready priority order). Use --pick=N or a bead ID to select a specific item.")
			}
		}
		// A selector is a request, even when the displayed set is empty.
		var selected next.BeadInfo
		if named != "" {
			selected, err = next.SelectNamedFromReadySet(rs, named)
		} else if len(items) == 0 && pick > 0 && specFlag != "" {
			return fmt.Errorf("--pick %d is out of range: 0 items\nrecovery: mindspec next --spec %s", pick, idrender.Spec(specFlag))
		} else if len(items) == 0 {
			if specFlag != "" {
				return handleEmptyScoped(root, rs, exec, func(activeBead string) {
					resolved := next.ResolveMode(root, next.BeadInfo{ID: activeBead, Title: "[" + specFlag + "] recovered"})
					resolved.SpecID = specFlag
					fmt.Print(formatStateLine(resolved.Mode, resolved.SpecID, activeBead))
				})
			}
			fmt.Println("No ready work found.")
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  - Create a new spec: mindspec spec create <slug>")
			fmt.Println("  - Check blocked items: bd blocked")
			fmt.Println("  - List all open work: bd list --status=open")
			return nil
		} else {
			selected, err = next.SelectWork(items, pick)
		}
		if err != nil {
			return fmt.Errorf("selecting work: %w", err)
		}

		// Step 4.5: Gate-before-mutate readiness floor (spec 124 R3;
		// ADR-0041's "preflight-leg-only addition" fourth-verb clause —
		// mindspec next adopts ONLY the preflight leg of the gate-before-
		// mutate contract for bead readiness). This MUST run after
		// selection and BEFORE any claim/branch/worktree mutation below,
		// so a NOT-READY refusal leaves bd state, branches, and worktrees
		// byte-identical to their pre-call state (AC-4).
		gateResult, gateErr := nextGateReadinessFn(root, selected.ID, next.GateOptions{ScopeSpec: specFlag, NamedSignals: namedSignals, FlagGiven: flagGiven, EvidencePath: evidencePath})
		if gateErr != nil {
			return gateErr
		}

		// Lineage is the dispatch and recording spec, including unscoped claims.
		specFlag = gateResult.SpecID
		if specFlag != guardedSpec {
			if err := checkClaimDirt(specFlag); err != nil {
				return err
			}
		}
		for _, note := range gateResult.Notes {
			fmt.Fprintln(os.Stderr, note)
		}

		// Step 4.6: On the --allow-not-ready proceed path, write the
		// durable override marker (spec 124 R3 / AC-4) BEFORE any
		// claim/branch/worktree mutation, FAIL-CLOSED. AC-4 requires the
		// marker as a GUARANTEE, not best-effort: `--allow-not-ready`
		// success ⟹ (marker durably written AND bead claimed). Writing
		// advisory metadata to the not-yet-claimed bead is safe (the bead
		// exists in bd regardless of claim status), and it keeps AC-4's
		// refusal-zero-mutation intact — a plain refusal (no
		// --allow-not-ready) never reaches here (gateErr returned above),
		// and a marker-write failure refuses with nothing claimed, no
		// worktree created. If the marker lands but ClaimBead then FAILS,
		// the claim-failure branch below ROLLS BACK the marker (final-
		// review r1 G3-OVERRIDE-ORPHAN) — a lost claim must not leave an
		// authorizing override on a bead this run never claimed.
		if len(gateResult.FailingSignals) > 0 {
			fmt.Fprintf(os.Stderr, "warning: claiming %s despite failing readiness signal(s) (--allow-not-ready): %s\n",
				idrender.Bead(selected.ID), strings.Join(gateResult.FailingSignals, ", "))
			if err := next.RecordReadinessOverride(selected.ID, gateResult); err != nil {
				return readinessOverrideFailure(selected.ID, err)
			}
		}

		// Step 5: Claim
		fmt.Print(formatClaimLine(selected.ID, selected.Title))
		if err := next.ClaimBead(selected.ID); err != nil {
			// G3-OVERRIDE-ORPHAN (spec 124 final-review r1): the Step 4.6
			// override marker landed BEFORE this claim; a failed claim
			// must not leave that authorizing marker orphaned on an
			// unclaimed bead. Roll it back (best-effort — the claim
			// failure below stays the primary error) and warn LOUDLY if
			// the rollback itself also fails, naming the orphaned key.
			if len(gateResult.FailingSignals) > 0 {
				if rbErr := next.RollbackReadinessOverride(selected.ID); rbErr != nil {
					// Final-review round 1 (R5/G3): rbErr is bd-derived
					// (metadata delete) — escape at this render sink.
					fmt.Fprintf(os.Stderr,
						"WARNING: claim failed AND rolling back the --allow-not-ready override marker on %s also failed: %v — an orphaned mindspec_readiness_override marker may remain; remove that metadata key before re-claiming or dispatching this bead\n",
						idrender.Bead(selected.ID), next.RenderBDError(rbErr))
				} else {
					fmt.Fprintf(os.Stderr,
						"note: rolled back the --allow-not-ready override marker on %s (claim failed — no override left behind)\n",
						idrender.Bead(selected.ID))
				}
			}
			// Spec 093 Req 3: the recovery recipe lives at this caller —
			// it has the root/specFlag context the interpolation needs.
			// The wiring (which error to return) is pinned by
			// claimFailureError in next_recovery_test.go.
			return claimFailureError(root, specFlag, selected, err)
		}

		// Step 5.1: Record bead claim time for session freshness gate
		if sess, readErr := state.ReadSession(root); readErr == nil {
			sess.BeadClaimedAt = time.Now().UTC().Format(time.RFC3339)
			_ = state.WriteSessionFile(root, sess)
		}

		// Step 5.2: Spec 128 Bead 2 (OQ-3 option 1) — advance stored
		// phase after a successful lifecycle claim. Extracted to its own
		// named function (matching claimFailureError/
		// warnWorktreeSetupFailure's established pattern) so it is
		// directly unit-testable without exercising the full RunE flow.
		advancePhaseOnClaim(root, selected)

		// Step 5.5: Create or reuse worktree
		wtPath, wtErr := nextEnsureWorktreeFn(root, selected.ID, specFlag, exec)
		if wtErr != nil {
			// Spec 093 Req 4: warn-and-continue is preserved (zero
			// behavior change), but the message now carries the concrete
			// `git worktree add` recipe and the auto-recovery re-run —
			// the agent is claimed-but-homeless without it. The
			// warn-AND-continue wiring is pinned by warnWorktreeSetupFailure
			// in next_recovery_test.go.
			warnWorktreeSetupFailure(os.Stderr, root, specFlag, selected, wtErr)
		} else if wtPath != "" {
			fmt.Printf("Worktree: %s\n", wtPath)
			fmt.Printf("  %s\n", containment.EmitCd(wtPath))
		}

		// Step 6: Resolve mode and spec ID
		resolved := next.ResolveMode(root, selected)
		if specFlag != "" {
			// Explicit --spec flag always wins over title parsing
			resolved.SpecID = specFlag
		}

		// Note: parent status propagation handled natively by beads epics

		// ADR-0023: no focus write — state is derived from beads.
		fmt.Print(formatStateLine(resolved.Mode, resolved.SpecID, selected.ID))
		fmt.Println()

		// Step 7.5: Emit recording bead marker (best-effort)
		if resolved.SpecID != "" {
			if err := recording.EmitBeadMarker(root, resolved.SpecID, "start", selected.ID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not emit recording marker: %v\n", err)
			}
			if err := recording.AddBeadToPhase(root, resolved.SpecID, selected.ID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not update recording manifest: %v\n", err)
			}
		}

		// Step 8: Emit bead context from bd show (Spec 074)
		if selected.ID != "" {
			rendered, renderErr := contextpack.RenderBeadContext(selected.ID)
			if renderErr == nil {
				fmt.Println()
				fmt.Print(rendered)
				// Completion reminder — ensures agents see "how to finish" right
				// before they start coding. Without this, the implement template's
				// completion section may scroll out of small-context models.
				fmt.Print(completionGuidance(selected.ID))
			} else {
				fmt.Fprintf(os.Stderr, "warning: could not render bead context: %v (falling back to generic guidance)\n", next.RenderBDError(renderErr))
				if err := emitInstruct(root); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", next.RenderBDError(err))
				}
			}
		} else {
			if err := emitInstruct(root); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", next.RenderBDError(err))
			}
		}

		return nil
	},
}

// bareReadinessRefusal decides the syntax refusal before resolving any bead.
// A real positional ID then gets a read-only preflight solely for recovery;
// a space-separated signal token never enters selection or readiness.
func bareReadinessRefusal(args []string) error {
	beadID := ""
	if len(args) > 0 {
		beadID = args[0]
	}
	root := ""
	if beadID != "" && !strings.HasPrefix(beadID, "MF-") {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err = workspace.FindRoot(cwd)
		if err != nil {
			return err
		}
	}
	_, err := next.GateReadiness(root, beadID, next.GateOptions{BareFlag: true})
	return err
}

// completionGuidance renders the "When done" tail appended to the bead
// context. Spec 092 Req 5 (mindspec-qxsy / mindspec-tjat): the guidance
// is location-agnostic — it must NOT instruct agents to `cd` into the
// bead worktree before running `mindspec complete`. The command resolves
// the bead worktree itself (internal/complete/complete.go) and may be
// run from the repo root; it removes the bead worktree when it succeeds,
// so a shell that followed a cd-then-complete instruction is stranded in
// a deleted directory.
func completionGuidance(beadID string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	// R4: beadID is an ID-typed position — idrender.Bead.
	fmt.Fprintf(&sb, "**When done**: do the work and commit it in the worktree. Then run `mindspec complete %s \"describe what you did\"` to close this bead — run it from the repo root (it resolves the bead worktree itself) and note it will remove the bead worktree when it succeeds.\n", idrender.Bead(beadID))
	sb.WriteString("Do NOT use `bd close` or raw git — `mindspec complete` handles merge, cleanup, and state transitions.\n")
	// Spec 092 Req 14 (mindspec-pi24): anti-merge-main warning in the
	// bead-context channel too — the implement template's completion
	// section may scroll out of small-context models.
	sb.WriteString("Do NOT merge `main` into the bead branch mid-implementation — bead work flows bead → spec → main, and pulling `main` in creates merge conflicts at `mindspec impl approve`.\n")
	return sb.String()
}

// runEmitOnly handles the --emit-only path: build and print primer without
// claiming. specFlag is prefix-resolved and existence-proven identically
// to the claim path (spec 128 R6.7) — refusing before any ready query
// when an explicit --spec names no actual spec, rather than falling back
// to the global, unscoped `bd ready` (the removed next.go:417-424
// fallback).
func runEmitOnly(root, specFlag string, pick int, args []string) error {
	if specFlag != "" {
		resolved, resolveErr := resolve.ResolveSpecPrefixProven(root, specFlag)
		if resolveErr != nil {
			return resolveErr
		}
		specFlag = resolved
	}

	if len(args) > 0 {
		// Explicit bead ID provided as positional argument — R6.1's
		// pinned preview semantics: fetch by ID directly, bypassing
		// ready-set membership and the spec filter entirely (a preview
		// needs no claimable bead), and SAY SO in the output rather than
		// leaving the asymmetry with the claim path unstated.
		info, err := next.FetchBeadByID(args[0])
		if err != nil {
			// round-1 panel G3 audit: args[0] is an ID-typed position on
			// this path (FetchBeadByID does no short-form resolution —
			// unlike the claim path's positional, it must already be the
			// full bead ID or FetchBeadByID's own idvalidate.BeadID gate
			// refuses first), so idrender.Bead — not termsafe.Escape — is
			// the matching convention: a clean full ID renders
			// byte-identical, a malformed/hostile one is force-quoted.
			return fmt.Errorf("fetching bead %s: %w", idrender.Bead(args[0]), err)
		}
		fmt.Println("emit-only: bead fetched by ID; readiness not checked.")
		return emitOnlyPrimer(info)
	}

	// Query ready work and select, via the SAME shared construction the
	// claim path uses (R6.1/AC-9 parity).
	rs, err := next.BuildReadySet(root, specFlag)
	var emptyErr *next.EmptyAfterFilterError
	if err != nil && !errors.As(err, &emptyErr) {
		return err
	}
	if notice := rs.ExclusionNotice(); notice != "" {
		fmt.Fprintln(os.Stderr, notice)
	}
	items := rs.Filtered
	if len(items) == 0 {
		if specFlag != "" && pick > 0 {
			return fmt.Errorf("--pick %d is out of range: 0 items\nrecovery: mindspec next --spec %s", pick, idrender.Spec(specFlag))
		}
		if specFlag != "" {
			return handleEmptyScoped(root, rs, nil, nil)
		}
		return fmt.Errorf("no ready work found for --emit-only")
	}
	if len(items) > 1 {
		fmt.Printf("Ready work (%d items):\n", len(items))
		fmt.Print(next.FormatWorkList(items))
		fmt.Println()
		if pick == 0 {
			fmt.Println("Defaulting to item 1 (bd ready priority order). Use --pick=N or a bead ID to select a specific item.")
		}
	}

	selected, err := next.SelectWork(items, pick)
	if err != nil {
		return fmt.Errorf("selecting work: %w", err)
	}

	return emitOnlyPrimer(selected)
}

// emitOnlyPrimer renders and prints the bead context primer for the
// --emit-only path. Extracted so both the positional-bypass branch and
// the ready-set-selection branch share one render call.
func emitOnlyPrimer(selected next.BeadInfo) error {
	rendered, err := contextpack.RenderBeadContext(selected.ID)
	if err != nil {
		return fmt.Errorf("rendering bead context: %w", err)
	}
	fmt.Print(rendered)
	return nil
}

// formatClaimLine renders the "Claiming [<id>] <title> ..." line printed
// just before a bead is claimed. id is an ID-typed position (routed
// through idrender.Bead so a malformed value is forced-quoted rather than
// rendered raw); title is agent-writable free text from the bd Title
// field (routed through termsafe.Escape per R4).
func formatClaimLine(id, title string) string {
	return fmt.Sprintf("Claiming [%s] %s ...\n", idrender.Bead(id), termsafe.Escape(title))
}

// formatStateLine renders the "State updated: mode=..., spec=..., bead=..."
// line printed after mode resolution. specID and beadID are ID-typed
// positions routed through idrender (AC-24): a malformed value renders
// forced-quoted; a clean value (including the dotted-child shape) renders
// byte-identically. The empty-sentinel discipline (round-6 F1) matters
// here specifically: specID == "" is the legitimate spec-mode value
// (today's "spec=" with nothing after the "=") — idrender.Spec treats the
// empty string as identity, so this NEVER renders as `spec=""`.
func formatStateLine(mode, specID, beadID string) string {
	return fmt.Sprintf("State updated: mode=%s, spec=%s, bead=%s\n", mode, idrender.Spec(specID), idrender.Bead(beadID))
}

// advancePhaseOnClaim implements Step 5.2 (spec 128 Bead 2, OQ-3
// option 1): after a successful ClaimBead, self-heal any legacy v2
// phase-semantics signature, then advance the epic's stored
// mindspec_phase to `implement` under the R3.3 lock, but ONLY for a
// LIFECYCLE (task-typed or untyped — the ADR-0023 discriminator) claim;
// a non-lifecycle plan-bead claim (e.g. a `bug` bead) advances nothing.
// A bug with no epic lineage is refused earlier by EvaluateReadiness.
// Forward-only: no-op when already `implement` — this is a NEW writer,
// so it registers in the bead-1 writer-inventory test (writer_inventory_test.go)
// by construction; the composite literal below must stay lexically
// nested inside the phase.WithPhaseWriteLock call for that AST-walk
// test to recognize it as guarded.
//
// Round-1 panel G1 (hard block — the core defect): the write is issued
// AFTER ClaimBead already ran (Step 5), so it can be arbitrarily
// "delayed" relative to the claim (worktree setup, agent scheduling).
// By the time this fn finally runs INSIDE the lock, a concurrent
// `release` or `complete` invocation may have ALREADY mutated the child
// set — and, via its own guarded write, stored — out from under the
// stale "we just claimed it, so implement is still correct" assumption
// the pre-fix code made. The fn therefore re-derives the CURRENT child
// state on a FRESH per-call Cache (never one built before the lock was
// acquired — the same named hazard bead 1 was hard-blocked on twice)
// and writes `implement` ONLY while that fresh evidence still agrees:
// if release already returned the sole claimed bead to open (derived
// plan) or complete already closed it (derived review, or implement
// again if other lifecycle beads remain open), this write no-ops rather
// than regressing that fresher state back to implement.
//
// Failure-semantics contract (round-1 panel G1.4): best-effort — a
// failure here never fails `next` itself (the bead is already claimed
// and the worktree not yet set up), so a genuine race remains POSSIBLE
// (next's own claim advance losing a lock timeout or merge-write leaves
// stored behind derived momentarily). What the contract GUARANTEES is
// that the divergence is always FORWARD (derived implement, stored
// whatever it was — plan in the common case) and NAMES A RECOVERY
// COMMAND THAT ACTUALLY CONVERGES: `mindspec repair phase <spec-id>`,
// never the dead-end `mindspec next <bead-id>` (the bead is already
// in_progress, so a re-run can never re-select it — round-1 panel
// G5/R3-G2). The forward direction means the unforced operator guard
// permits that command unconditionally.
//
// A genuine epic-lineage lookup failure (as opposed to the bead simply
// having no epic lineage at all, phase.ErrNoEpicLineage) is still
// surfaced as a warning rather than silently discarded.
func advancePhaseOnClaim(root string, selected next.BeadInfo) {
	epicID, specID, epicErr := phase.FindEpicForBead(selected.ID)
	if epicErr != nil {
		if !errors.Is(epicErr, phase.ErrNoEpicLineage) {
			// Final-review round 1 (R2/G3): epicErr chains
			// FindEpicForBead -> wrapBDError's raw stderr excerpt —
			// escape at this render sink like the function's other two
			// failure legs (describePhaseSyncFailure / RenderBDError).
			fmt.Fprintf(os.Stderr, "warning: could not resolve epic for bead %s: %v\n", idrender.Bead(selected.ID), next.RenderBDError(epicErr))
		}
		return
	}
	if epicID == "" {
		return
	}
	recovery := nextPhaseAdvanceRecoveryCommand(specID)
	if _, err := phase.EnsurePhaseSemanticsV2(root, epicID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s\n", describePhaseSyncFailure(err, recovery))
	}
	if !phase.IsLifecycleIssueType(selected.IssueType) {
		return
	}
	if lockErr := phase.WithPhaseWriteLock(root, epicID, nextPhaseMergeMetadataFn, func(stored string, _ bool) (map[string]interface{}, error) {
		if stored == state.ModeImplement {
			return nil, nil
		}
		// Fresh per-call Cache — never a Cache built before this call
		// reached the lock.
		freshCache := phase.NewCache()
		children, cerr := freshCache.GetChildren(epicID)
		if cerr != nil {
			return nil, fmt.Errorf("advance-phase-on-claim: reading children of epic %s: %w", idrender.Bead(epicID), cerr)
		}
		if phase.DerivePhaseFromChildren(children) != state.ModeImplement {
			// The fresh evidence no longer supports advancing to
			// implement — a concurrent release/complete already moved
			// stored to something else. NEVER overwrite fresher state.
			return nil, nil
		}
		return map[string]interface{}{"mindspec_phase": state.ModeImplement}, nil
	}); lockErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %s\n", describePhaseSyncFailure(lockErr, recovery))
	}
}

// describePhaseSyncFailure renders advancePhaseOnClaim's best-effort
// failures with the ALWAYS-convergent recovery command (round-1 panel
// G1.4/G5): unlike phase.DescribeLockFailure's general contract (a
// non-timeout failure gets no manufactured recovery line, because
// "that belongs to whatever wrote the failing value" — its own doc
// comment), this specific write's convergent fix is the SAME regardless
// of failure shape (a lock timeout or a genuine merge-write error), so
// the recovery line is appended unconditionally rather than only on the
// timeout leg.
//
// The non-timeout leg's err can be bd/lock-derived (a merge-write failure
// wrapping bd's own possibly-hostile stderr, or a children-read failure
// via freshCache.GetChildren) — spec 128 Bead 8's folded-in ta30.9
// render-leg follow-up: next.RenderBDError applies the SAME per-line
// termsafe.Escape discipline bdRenderedError uses (ADR-0042) rather than
// forking a second escaping path. The timeout leg is untouched —
// ErrPhaseLockTimeout wraps only an internal lock-file path, never
// agent-writable content.
func describePhaseSyncFailure(err error, recovery string) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, phase.ErrPhaseLockTimeout) {
		return phase.DescribeLockFailure(err, recovery)
	}
	return fmt.Sprintf("could not sync phase metadata: %v\nrecovery: %s", next.RenderBDError(err), recovery)
}

// nextPhaseAdvanceRecoveryCommand renders advancePhaseOnClaim's
// convergent recovery (round-1 panel G1.4/G5): `mindspec repair phase
// <spec-id>` when specID is known, else the documented "<spec-id>"
// placeholder (specIDForEpicWithCache's own convention) for the rare
// case a bead resolves to an epic without resolvable spec metadata.
func nextPhaseAdvanceRecoveryCommand(specID string) string {
	if specID == "" {
		return "mindspec repair phase <spec-id>"
	}
	return fmt.Sprintf("mindspec repair phase %s", idrender.Spec(specID))
}

// readinessOverrideFailure builds the guard failure for a
// RecordReadinessOverride write failure (Step 4.6). Extracted from the
// RunE closure for direct unit-testability (spec 128 Bead 8's folded-in
// ta30.9 render-leg follow-up): err is bd/lock-derived and MAY carry
// hostile content (bd's own possibly-adversarial stderr, folded in by
// bdcli.go's wrapBDError). RecordReadinessOverride already returns it
// wrapped in bdRenderedError (internal/next/ready_gate.go:130, bead
// mindspec-ta30.9 round-2 panel) — but this call site wraps it AGAIN via
// next.RenderBDError before formatting, so the escaping guarantee lives
// HERE, explicitly, rather than resting on an implicit fmt %v/error-
// interface interaction with a detail of a different file that a future
// refactor could silently break. Re-wrapping an already-escaped error is
// a safe no-op (termsafe.Escape's output is plain printable ASCII, which
// re-escapes byte-identical), so this is defense in depth, not a second
// divergent escaping path.
func readinessOverrideFailure(beadID string, err error) error {
	return guard.NewFailure(
		fmt.Sprintf("recording the --allow-not-ready override marker for %s failed: %v (nothing was claimed — the readiness gate refuses rather than claim without a durable override marker, spec 124 AC-4)", idrender.Bead(beadID), next.RenderBDError(err)),
		"re-run this exact `mindspec next` invocation with --allow-not-ready=<ids> --evidence <path-to-justification> once bd metadata writes are healthy",
	)
}

// claimFailureError builds the spec 093 Req 3 claim-failure recovery
// error for the `mindspec next` ClaimBead caller. Extracted from the
// RunE closure so the wiring — the FULL recipe is returned, never the
// pre-093 bare `fmt.Errorf("claiming bead: %w")` — is unit-testable
// (next_recovery_test.go); reverting the call site to the bare wrap
// fails TestClaimFailureError_FullRecipe. (Final-review round 1 G7:
// re-attached — bead 8 inserted readinessOverrideFailure between this
// comment and its function with no separating blank line, so godoc
// attributed it to the wrong function.)
func claimFailureError(root, specFlag string, selected next.BeadInfo, claimErr error) error {
	return next.ClaimFailure(root, recoveryConfig(root),
		selected.ID, recoverySpecSlug(root, specFlag, selected), claimErr)
}

// warnWorktreeSetupFailure writes the spec 093 Req 4 worktree-setup
// recovery recipe to w and returns — it never fails the command
// (warn-AND-continue, the claimed-but-homeless agent still proceeds to
// the state update + auto-recovery hint). Extracted from the RunE
// closure so BOTH halves are pinned: the FULL recipe text
// (TestWarnWorktreeSetupFailure_FullRecipe) and the continue semantics
// — the function returns nothing, so flipping the call site to a fatal
// `return ...` is a visible signature change that fails
// TestWarnWorktreeSetupFailure_DoesNotFatal.
func warnWorktreeSetupFailure(w io.Writer, root, specFlag string, selected next.BeadInfo, wtErr error) {
	fmt.Fprintf(w, "Warning: %v\n", next.WorktreeSetupFailure(root, recoveryConfig(root),
		selected.ID, recoverySpecSlug(root, specFlag, selected), wtErr))
}

// recoveryConfig loads config for failure-path message interpolation
// (spec 093 Reqs 3-4). Falls back to defaults so a config problem never
// masks the original failure. Runs only on failure paths.
func recoveryConfig(root string) *config.Config {
	cfg, err := config.Load(root)
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

// recoverySpecSlug resolves the spec slug for the Req 3/4 recovery
// recipes: the explicit --spec flag wins; otherwise the slug is parsed
// from the selected bead's title (next.ResolveMode). May return "" —
// the recipe constructors then fall back to placeholders. Runs only on
// failure paths.
func recoverySpecSlug(root, specFlag string, selected next.BeadInfo) string {
	if specFlag != "" {
		return specFlag
	}
	return next.ResolveMode(root, selected).SpecID
}

// checkUnmergedBeads checks for closed sibling beads that still have a bead/<id>
// branch which is NOT merged into the spec branch, indicating they were closed
// via `bd close` without `mindspec complete`. Returns an error to block
// `mindspec next` until cleanup is performed.
//
// It delegates to the shared lifecycle.FindOrphanedClosedBeads predicate so the
// IsAncestor confirmation (skipping a benign merged-but-branch-undeleted bead)
// stays in lockstep with `mindspec complete` and `mindspec doctor`.
func checkUnmergedBeads(specID string) error {
	orphans := lifecycle.FindOrphanedClosedBeads(specID, ".", "")
	if len(orphans) == 0 {
		return nil
	}
	return unmergedBeadError(orphans[0])
}

// unmergedBeadError formats the checkUnmergedBeads refusal for a single
// orphan. Extracted so a hostile o.BeadID (unvalidated bd-list data — see
// lifecycle.Orphan) can be pinned directly: idrender.Bead forces a
// malformed-but-printable ID through strconv.Quote so it can never be
// mistaken for — or forge output around — a genuine bead ID (R4 display-
// forgery sweep, spec 120).
func unmergedBeadError(o lifecycle.Orphan) error {
	safeBeadID := idrender.Bead(o.BeadID)
	return fmt.Errorf("bead %s was closed without `mindspec complete` — merge topology is broken.\nRun `mindspec complete %s` to recover, then retry `mindspec next`.", safeBeadID, safeBeadID)
}

func init() { registerNextFlags(nextCmd) }

// registerNextFlags is shared with command fixtures so they parse the exact
// production flag types and NoOptDefVal behavior.
func registerNextFlags(cmd *cobra.Command) {
	cmd.Flags().Int("pick", 0, "Pick a specific item by number (1-based) when multiple are ready; 0 (default) means unset — defaults to item 1; negative values are rejected")
	cmd.Flags().String("spec", "", "Target spec ID to filter ready work (auto-detected if exactly one active spec)")
	cmd.Flags().Bool("force", false, "Bypass the context clear gate (use when you know your context is clean)")
	cmd.Flags().Bool("emit-only", false, "Emit bead primer without claiming, creating worktree, or updating state (for multi-agent mode)")
	cmd.Flags().String("allow-not-ready", "", "Override named readiness signals: --allow-not-ready=MF-1,MF-2,MF-3,MF-4; requires --evidence when recording failures")
	cmd.Flags().Lookup("allow-not-ready").NoOptDefVal = next.BareOverride
	cmd.Flags().String("evidence", "", "Readable justification file (1 byte to 64 KiB); stores only path, sha256 and size")
}

// nextSpecMenuCommand names the literal form accepted under the listed spec IDs.
const nextSpecMenuCommand = "  mindspec next --spec=<spec-id>"

// handleEmptyScoped reports the child-derived state even for loud emptiness.
// A nil executor is the emit-only path and cannot recover a worktree.
func handleEmptyScoped(root string, rs next.ReadySet, exec executor.Executor, reportRecovered func(string)) error {
	children, err := rs.Cache.GetChildren(rs.EpicID)
	if err != nil {
		return fmt.Errorf("child-derived phase: indeterminate (could not read children of %s: %v)\nrecovery: mindspec state show", idrender.Bead(rs.EpicID), next.RenderBDError(err))
	}
	// PhaseDetail's legacy API represents a failed read with an empty Derived;
	// explicitly preflight both reads so an infrastructure failure cannot look empty.
	if _, err := rs.Cache.FindEpic(rs.EpicID); err != nil {
		return fmt.Errorf("stored phase: indeterminate: %w\nrecovery: mindspec state show", next.RenderBDError(err))
	}
	detail, err := phase.DerivePhaseDetailWithCache(rs.Cache, rs.EpicID)
	if err != nil {
		return err
	}
	closed, progress, open := 0, 0, 0
	for _, child := range children {
		if !phase.IsLifecycleIssueType(child.IssueType) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(child.Status)) {
		case "closed":
			closed++
		case "in_progress":
			progress++
		default:
			open++
		}
	}
	fmt.Printf("lifecycle children: %d closed / %d in_progress / %d open\n", closed, progress, open)
	fmt.Printf("stored phase: %s; child-derived phase: %s\n", detail.Stored, detail.Derived)
	fmt.Println("an empty ready set is not evidence the spec is complete; the derived phase above is")
	switch {
	case progress > 0:
		if exec != nil {
			activeBead, err := nextResolveActiveBeadFn(root, rs.SpecFlag)
			if err == nil && activeBead != "" {
				fmt.Printf("No ready work, but bead %s is in-progress with missing worktree. Recovering...\n", idrender.Bead(activeBead))
				wtPath, err := nextEnsureWorktreeFn(root, activeBead, rs.SpecFlag, exec)
				if err != nil {
					return fmt.Errorf("recovering worktree for in-progress bead %s: %w", idrender.Bead(activeBead), err)
				}
				if wtPath != "" {
					fmt.Printf("Worktree recovered: %s\n  %s\n", wtPath, containment.EmitCd(wtPath))
				}
				if reportRecovered != nil {
					reportRecovered(activeBead)
				}
			} else {
				fmt.Printf("recovery: bd list --parent %s --status=in_progress\n", idrender.Bead(rs.EpicID))
			}
		} else {
			fmt.Printf("recovery: mindspec next --spec %s\n", idrender.Spec(rs.SpecFlag))
		}
	case open > 0:
		fmt.Printf("%d open lifecycle bead(s), none selectable (any that are bd-ready but unmapped or unlinked are named above)\n", open)
		fmt.Printf("recovery: bd blocked --parent %s\n", idrender.Bead(rs.EpicID))
	case closed > 0:
		fmt.Printf("recovery: mindspec impl approve %s\n", idrender.Spec(rs.SpecFlag))
	default:
		fmt.Printf("recovery: mindspec plan approve %s\n", idrender.Spec(rs.SpecFlag))
	}
	if detail.Stored != "" && detail.Derived != "" && detail.Stored != detail.Derived && !phase.IsBackwardMove(detail.Stored, detail.Derived) {
		fmt.Printf("recovery: mindspec repair phase %s\n", idrender.Spec(rs.SpecFlag))
	}
	if rs.HasBlockingNearMisses() {
		return fmt.Errorf("no ready plan bead for spec %s; blocking near-misses are named above", idrender.Spec(rs.SpecFlag))
	}
	return nil
}
