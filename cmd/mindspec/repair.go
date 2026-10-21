package main

// Spec 092 (agent-contract-hardening) Bead 3, Req 19:
// `mindspec repair phase <spec-id>` is the ONLY phase-metadata fix
// mindspec ever tells a user or agent to run. Raw
// `bd update <id> --metadata '{...}'` REPLACES the entire metadata map
// (silently wiping mindspec_migrated_at, doc-skew audit keys, and
// ADR-override keys when pasted), so it is banned from all emitted
// output (HC-5); every recovery line that needs a phase fix emits this
// subcommand instead.

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

// repairMergeMetadataFn is the test seam for the Req 19 merge-write.
// It MUST stay bead.MergeMetadata (read-merge-write) in production —
// merge semantics are the whole point of the subcommand.
var repairMergeMetadataFn = bead.MergeMetadata

// repairGetMetadataFn is the test seam `repair spec-title` uses to read
// the epic's existing spec_num metadata before composing the checked
// SpecIDFromMetadata derivation. Defaults to bead.GetMetadata.
var repairGetMetadataFn = bead.GetMetadata

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair mindspec lifecycle state",
}

var repairPhaseCmd = &cobra.Command{
	Use:   "phase <spec-id>",
	Short: "Re-derive the spec's lifecycle phase and merge-write it to the epic",
	Long: `Re-derives the lifecycle phase for the given spec from its child bead
statuses (bead statuses are the ground truth; the
epic's mindspec_phase metadata is a cache of that derivation) and writes the derived value back to the epic's
mindspec_phase metadata.

The write is a MERGE (read-merge-write, the same path lifecycle
commands use): unrelated metadata keys such as mindspec_migrated_at,
doc-skew audit keys, and ADR-override keys are preserved. Never repair
the phase with a raw bd metadata update — that path REPLACES the
entire metadata map and silently wipes every unrelated key.

Use this when a lifecycle gate or warning reports that the stored
phase disagrees with the child-derived phase and the bead states are
already correct.`,
	Args: cobra.ExactArgs(1),
	RunE: repairPhaseRunE,
}

var repairSpecTitleCmd = &cobra.Command{
	Use:   "spec-title <epic-id> <title>",
	Short: "Repair a spec epic's spec_title metadata (the malformed-metadata lever)",
	Long: `Merge-writes a corrected spec_title onto the given epic's metadata
— the convergent recovery lever every malformed-lineage
refusal names (see "mindspec repair spec-title <epic-id> <title>" in
those refusal messages).

The write is a MERGE (read-merge-write, the same discipline as
'mindspec repair phase'): unrelated metadata keys are preserved, and a
raw 'bd update --metadata' replace is never emitted.

Both arguments are validated before any bd MUTATION: <epic-id> is gated
as a well-formed bead ID before any bd invocation, and the replacement
<title> must slugify (via the same derivation 'mindspec next'/'mindspec
complete' use) to a well-formed spec ID — that slug convergence check
runs after the epic's spec_num is read but before the metadata merge, so
a title whose slug still fails validation is refused with no write
attempted, and the lever is guaranteed to converge once applied.`,
	Args: cobra.ExactArgs(2),
	RunE: repairSpecTitleRunE,
}

func init() {
	repairCmd.AddCommand(repairPhaseCmd)
	repairCmd.AddCommand(repairSpecTitleCmd)
	// Spec 128 R3.3: the backward-move guard's force flag. Empty means
	// "not forcing"; an explicit empty reason (--force-backward="") is
	// rejected below, same discipline as --allow-doc-skew/--override-adr.
	repairPhaseCmd.Flags().String("force-backward", "", "Record a reason and force a backward mindspec_phase move (refused by default)")
}

// repairSpecTitleRunE implements `mindspec repair spec-title <epic-id>
// <title>` (spec 120 R3, AC-8). ADR-0035 discipline: every refusal below
// carries a single convergent lever, the hostile value escaped/quoted
// only, and a final recovery line.
func repairSpecTitleRunE(cmd *cobra.Command, args []string) error {
	epicID := args[0]
	newTitle := args[1]

	// Own-arg gate (round-3 O3): validate epicID BEFORE any bd argv
	// embed — a malformed epic ID never reaches a bd spawn.
	if err := idvalidate.BeadID(epicID); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("%s is not a valid epic ID: %v", termsafe.Escape(epicID), err),
			"bd ready   (pick a listed epic ID and re-run)",
		)
	}

	// Read the epic's existing spec_num so the checked SpecIDFromMetadata
	// derivation (D1) can be evaluated against the SAME number the
	// corrected title would pair with — this verb repairs spec_title
	// only, never spec_num.
	meta, err := repairGetMetadataFn(epicID)
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: reading epic %s metadata failed: %v", epicID, err),
			fmt.Sprintf("bd show %s", epicID),
		)
	}
	var specNum int
	switch v := meta["spec_num"].(type) {
	case float64:
		specNum = int(v)
	case int:
		specNum = v
	}
	if specNum <= 0 {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: epic %s has no spec_num metadata to pair with the new title", epicID),
			fmt.Sprintf("bd show %s", epicID),
		)
	}

	// Refuse a replacement title whose slug still fails the corrected
	// idvalidate.SpecID — the whole point of the lever is to converge,
	// so a still-hostile title is rejected before any write.
	newSpecID, err := phase.SpecIDFromMetadata(specNum, newTitle)
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: replacement title %s slugifies to an invalid spec ID: %v", termsafe.Escape(newTitle), err),
			fmt.Sprintf("choose a title whose slug is alphanumeric/hyphen only, then re-run: mindspec repair spec-title %s \"<corrected-title>\"", epicID),
		)
	}

	// HC-5-safe merge-write: preserves every unrelated metadata key
	// (mindspec_phase, mindspec_migrated_at, doc-skew/ADR-override audit
	// keys, spec_num) — never a raw `bd update --metadata` replace.
	if err := repairMergeMetadataFn(epicID, map[string]interface{}{
		"spec_title": newTitle,
	}); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair spec-title: writing spec_title to epic %s failed: %v", epicID, err),
			fmt.Sprintf("mindspec repair spec-title %s %q", epicID, newTitle),
		)
	}

	fmt.Fprintf(os.Stderr, "event=lifecycle.spec_title_repaired epic=%s spec_id=%s\n", epicID, newSpecID)
	fmt.Printf("spec_title for epic %s repaired to %q (derives spec id %s; unrelated metadata keys preserved).\n",
		epicID, newTitle, newSpecID)
	return nil
}

// repairFindRootFn is repairPhaseRunE's test seam for root resolution.
// Production defaults to findRoot() (cwd-based `.mindspec`/`.git`
// discovery). Round-1 panel G2/G4 (hard-block driver): from a git-linked
// bead worktree, findRoot()'s ambient-cwd resolution can walk up to the
// SHARED MAIN checkout (workspace.FindRoot resolves a linked worktree
// back to its main repo root) — a real, observed failure mode: a test
// run from this bead's own worktree wrote an untracked
// .mindspec/phase.lock into the enclosing main checkout, and in a
// sandboxed CI environment that cannot even open that path the whole
// suite goes red. Every repair-phase test fixture-scopes this seam to a
// throwaway t.TempDir() so the R3.3 phase lock never resolves to, reads
// from, or writes into the real enclosing checkout.
var repairFindRootFn = findRoot

// errRepairDerivationFailed and errRepairNoDerivedPhase are repair
// phase's two derivation-failure sentinels (errors.Is-matched below to
// pick the right recovery command) — both now raised INSIDE
// WithPhaseWriteLock's critical section (round-1 panel G1/G3/R6: the
// derivation moves inside the lock, mirroring SanctionedDerivedSync, so
// the guard compares fresh-stored against fresh-derived).
var (
	errRepairDerivationFailed = errors.New("repair phase: derivation failed")
	errRepairNoDerivedPhase   = errors.New("repair phase: no derivable phase")
)

// repairPhaseRunE implements `mindspec repair phase <spec-id>`.
//
// Spec 128 R3.3: the backward-move guard. The phase DERIVATION (a fresh
// per-call Cache reading children fresh — the SanctionedDerivedSync
// house pattern, sanctioned_sync.go) and the guard comparison both run
// INSIDE WithPhaseWriteLock's critical section, against the FRESH stored
// value the lock reads — never a pre-lock snapshot — so the guard
// compares fresh-vs-fresh and the written value is self-consistent at
// write time (spec.md:113's pinned read-stored -> derive -> guard-compare
// -> MergeMetadata mechanism). A backward move is refused unless
// --force-backward "<reason>" is passed, in which case it proceeds and
// the reason is both recorded on the epic metadata and named in the
// printed success line (AC-5(iii)).
func repairPhaseRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]

	root, err := repairFindRootFn()
	if err != nil {
		return guard.NewFailure(
			fmt.Sprintf("repair phase: %v", err),
			fmt.Sprintf("cd into a mindspec-managed repository and re-run `mindspec repair phase %s`", idrender.Spec(specID)),
		)
	}

	forceBackwardReason, _ := cmd.Flags().GetString("force-backward")
	if cmd.Flags().Changed("force-backward") && strings.TrimSpace(forceBackwardReason) == "" {
		return fmt.Errorf("--force-backward requires a non-empty reason")
	}

	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		msg := fmt.Sprintf("repair phase: no lifecycle epic found for spec %s", idrender.Spec(specID))
		if err != nil {
			// Final-review round 1 (R5/G3): err chains bd stderr — escape
			// at this render sink.
			msg += fmt.Sprintf(": %v", next.RenderBDError(err))
		}
		return guard.NewFailure(msg, "mindspec spec list")
	}

	// Spec 128 Bead 2 (plan step 7): the v2 phase-semantics self-heal
	// runs BEFORE the derive-and-guard section below, so a legacy epic
	// (marker absent, stored `implement`, all-open) reconciles to `plan`
	// + the durable marker first, then the guard/derivation runs against
	// the now-consistent state — the recovery command the LEGACY NOTICE
	// (derive.go) names for this exact state is `mindspec repair phase
	// <spec-id>`, and this call is what makes that command actually
	// converge rather than repeatedly reporting the same legacy warning.
	// Runs OUTSIDE the WithPhaseWriteLock call below (the lock is
	// non-reentrant — calling a locked helper from inside another fn
	// self-blocks until ErrPhaseLockTimeout).
	if _, err := phase.EnsurePhaseSemanticsV2(root, epicID); err != nil {
		// Final-review round 1 (R5/G3): err can chain bd stderr (children
		// read / merge write) — escape at this render sink.
		return guard.NewFailure(
			fmt.Sprintf("repair phase: v2 phase-semantics reconciliation for spec %s (epic %s) failed: %v", idrender.Spec(specID), idrender.Bead(epicID), next.RenderBDError(err)),
			fmt.Sprintf("mindspec repair phase %s", idrender.Spec(specID)),
		)
	}

	var (
		freshStored     string
		freshDerived    string
		refusedBackward bool
		wroteWithForce  bool
	)
	lockErr := phase.WithPhaseWriteLock(root, epicID, repairMergeMetadataFn, func(stored string, _ bool) (map[string]interface{}, error) {
		freshStored = stored

		// Fresh, in-critical-section derivation (AC-5(v)'s stale-target
		// concurrency leg is driven through the SetListJSONForTest
		// children fixture, which this read observes exactly as
		// SanctionedDerivedSync's own children read would).
		//
		// Round-2 panel G2: MUST be the status-aware surface, not bare
		// phase.DerivePhaseFromChildren — bare children-derivation returns
		// "review" for an all-closed set regardless of a done marker
		// (DerivePhaseFromChildren's all-closed arm, internal/phase/
		// derive.go — function anchor, final-review round 2 R7 N2), so at
		// the prior revision `repair phase` on an
		// already-terminal (closed epic, done marker) spec refused a
		// phantom "backward move" to review and, if --force-backward'd,
		// would have REGRESSED a healthy done epic. This restores the
		// pre-spec-128 behavior (repair.go used phase.DerivePhaseDetail,
		// the non-cache twin of DerivePhaseDetailWithCache, for exactly
		// this reason) while keeping the fresh-per-call-Cache-inside-fn
		// discipline this bead adds. Disclosed scope note: like
		// readStoredPhaseWithCache (round-1's identical judgment call),
		// DerivePhaseDetailWithCache is a pre-existing, widely-shared
		// derivation helper whose OWN children-read swallows a
		// GetChildren failure to "" internally — changing that established
		// contract is out of this bead's scope, exactly as round-1 held
		// for the stored-phase read; the derr check below is a defensive
		// (currently unreachable) guard against the day that contract
		// changes, kept honest rather than silently assumed impossible.
		c := phase.NewCache()
		detail, derr := phase.DerivePhaseDetailWithCache(c, epicID)
		if derr != nil {
			return nil, fmt.Errorf("%w for spec %s (epic %s): %v", errRepairDerivationFailed, idrender.Spec(specID), idrender.Bead(epicID), derr)
		}
		derived := detail.Derived
		if derived == "" {
			return nil, fmt.Errorf("%w for spec %s from epic %s children", errRepairNoDerivedPhase, idrender.Spec(specID), idrender.Bead(epicID))
		}
		freshDerived = derived

		if phase.IsBackwardMove(stored, derived) {
			if forceBackwardReason == "" {
				refusedBackward = true
				return nil, nil
			}
			wroteWithForce = true
			return map[string]interface{}{
				"mindspec_phase":              derived,
				"mindspec_phase_force_reason": forceBackwardReason,
			}, nil
		}
		// Req 19: merge-write via bead.MergeMetadata — preserves every
		// unrelated metadata key on the epic.
		return map[string]interface{}{
			"mindspec_phase": derived,
		}, nil
	})
	if lockErr != nil {
		recovery := fmt.Sprintf("mindspec repair phase %s", specID)
		if errors.Is(lockErr, errRepairDerivationFailed) || errors.Is(lockErr, errRepairNoDerivedPhase) {
			recovery = fmt.Sprintf("bd show %s", epicID)
		}
		// Final-review round 1 (R5/G3): lockErr can chain bd stderr (the
		// in-lock derivation and merge write) — render the escaped form.
		return guard.NewFailure(next.RenderBDError(lockErr).Error(), recovery)
	}
	if refusedBackward {
		return guard.NewFailure(
			fmt.Sprintf("repair phase: refusing backward move for spec %s (epic %s): stored phase %q is ahead of the re-derived phase %q in lifecycle order (spec < plan < implement < review < done) — the derivation may be stale, or the epic genuinely regressed",
				idrender.Spec(specID), idrender.Bead(epicID), freshStored, freshDerived),
			fmt.Sprintf("bd show %s   (inspect the epic before forcing a backward move)", idrender.Bead(epicID)),
			fmt.Sprintf("mindspec repair phase %s --force-backward \"<reason>\"   (only if the backward move is intentional)", idrender.Spec(specID)),
		)
	}

	fmt.Fprintf(os.Stderr, "event=lifecycle.phase_repaired spec=%s epic=%s stored=%s derived=%s\n",
		specID, epicID, freshStored, freshDerived)
	switch {
	case freshStored == freshDerived:
		fmt.Printf("Phase for %s already consistent: %s (merge-write refreshed; unrelated metadata keys preserved).\n",
			specID, freshDerived)
	case wroteWithForce:
		fmt.Printf("Phase for %s force-repaired: %q -> %q (reason: %q; epic %s; unrelated metadata keys preserved).\n",
			idrender.Spec(specID), freshStored, freshDerived, forceBackwardReason, idrender.Bead(epicID))
	default:
		fmt.Printf("Phase for %s repaired: %q -> %q (epic %s; unrelated metadata keys preserved).\n",
			specID, freshStored, freshDerived, epicID)
	}
	return nil
}
