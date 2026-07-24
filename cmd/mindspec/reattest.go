package main

// mindspec reattest — the spec 125 R4 explicit, operator-invoked,
// git-corroborated landed-binding recovery (ADR-0041 §2(ii) as amended
// by spec 125: "Re-attested landed-bindings under §2(ii)").
//
// Deliberate surface constraints (AC-7/AC-8):
//   - one bead per invocation — NO --all/fleet flag, so the
//     mass-mutation vector across the merged-bead history stays a
//     scripted sequence of explicit per-bead invocations;
//   - NO bypass flag of any kind — corroboration cannot be disabled;
//   - NO way to supply a merge/second-parent SHA pair — an
//     operator-asserted pair corroborating itself is circular and
//     inadmissible; the binding is DERIVED from the independent git
//     scan (internal/lifecycle.ReattestLandedMerge) or not written;
//   - never invoked or written-through by `doctor` — writes happen
//     ONLY under this explicit verb;
//   - --spec-branch is SCOPING input only (WHERE to scan), consulted
//     ONLY when the bead's epic linkage is underivable (fallback-only,
//     plan-gate F2-2); the branch actually scanned is recorded in the
//     audit (mindspec_landed_reattest_scanned_branch) either way.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var reattestCmd = &cobra.Command{
	Use:   "reattest <bead-id>",
	Short: "Re-attest an already-merged bead's landed-binding from independent git topology (ADR-0041 §2(ii))",
	Long: `Explicit, operator-invoked, fail-closed recovery for an already-merged
bead whose merge-time landed-binding is missing or stale (the pre-125
fleet state): DERIVES the binding from an independent git scan of the
spec branch — a two-parent first-parent merge whose subject names this
bead (ownership) and whose topology proves it landed (landed-ness) —
and writes ONLY the scan-derived SHAs, together with an inspectable
audit record (actor, before/after values, timestamp, operation,
corroborating datum, scanned branch) in bd metadata.

Fail-closed: with no owned exact merge it REFUSES (no guess, no write)
to the audited ADR-0035 q9ea human attested-restore exit. There is no
bypass flag, and no way to assert a merge/second-parent pair — an
operator-asserted pair corroborating itself is circular and
inadmissible (ADR-0041 §2(ii), spec 125 amendment).

--spec-branch names WHERE to scan when the bead's epic linkage cannot
be derived; the linkage wins whenever derivable. It is scoping input
only — never a corroboration substitute — and the branch actually
scanned is recorded in the audit.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := strings.TrimSpace(args[0])
		// R3 explicit-ingress early gate (ADR-0042): a malformed beadID
		// refuses HERE, before any composition or bd/git spawn.
		if err := idvalidate.BeadID(beadID); err != nil {
			return guard.NewFailure(
				fmt.Sprintf("%s is not a valid bead ID: %v", termsafe.Escape(beadID), err),
				"bd list --status closed   (pick the bead ID to re-attest and re-run)",
			)
		}

		root, err := findRoot()
		if err != nil {
			return err
		}
		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		specBranchFlag, _ := cmd.Flags().GetString("spec-branch")

		deps := reattestDeps{
			deriveSpecBranch: deriveSpecBranchFromLineage,
			reattest:         lifecycle.ReattestLandedMerge,
			actor:            reattestActor(),
			stdout:           os.Stdout,
			stderr:           os.Stderr,
		}
		if err := runReattest(deps, root, beadID, specBranchFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return nil
	},
}

// reattestDeps collects the seams runReattest drives so the precedence
// (linkage-wins, flag-fallback-only) and refusal-rendering contracts are
// unit-testable without bd/git on PATH.
type reattestDeps struct {
	// deriveSpecBranch resolves the bead's owning spec branch from its
	// epic linkage (bead → parent epic → spec/<id>). ("", nil) means the
	// linkage is genuinely underivable (no lineage); an error is a real
	// lookup failure.
	deriveSpecBranch func(beadID string) (string, error)
	// reattest is the derivation engine (lifecycle.ReattestLandedMerge).
	reattest func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error)
	// actor is the audit acting-identity value (user@host via argv0).
	actor  string
	stdout io.Writer
	stderr io.Writer
}

// deriveSpecBranchFromLineage is the production linkage resolver: bead →
// parent epic → spec ID → spec/<id> via the workspace composition waist.
// A genuinely-lineage-less bead yields ("", nil); real lookup failures
// propagate (runReattest degrades to --spec-branch only when the caller
// supplied one, and says so).
func deriveSpecBranchFromLineage(beadID string) (string, error) {
	_, specID, err := phase.FindEpicForBead(beadID)
	if err != nil {
		if errors.Is(err, phase.ErrNoEpicLineage) {
			return "", nil
		}
		return "", err
	}
	if specID == "" {
		return "", nil
	}
	// specID from FindEpicForBead already passed idvalidate.SpecID; the
	// waist re-validates and fails closed regardless.
	return workspace.SpecBranch(specID)
}

// normalizeSpecBranchFlag validates the operator-supplied --spec-branch
// value and recomposes it through the workspace waist. Accepts either
// "spec/<spec-id>" or a bare "<spec-id>". Reverse-derivation gate
// (ADR-0042 §1 reverse, the spec-120 R6(e) ratchet): the ID parsed back
// OUT of the branch-shaped value is idvalidate-gated in this same
// declaration before any recomposition.
func normalizeSpecBranchFlag(v string) (string, error) {
	specID := strings.TrimPrefix(strings.TrimSpace(v), workspace.SpecBranchPrefix)
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	return workspace.SpecBranch(specID)
}

// reattestActor builds the audit acting-identity/authority value:
// user@host plus the invoking operation's argv0 (the plan's audit-key
// choice). Best-effort fields degrade to "unknown" rather than blocking
// an explicit recovery on an unreadable passwd entry.
func reattestActor() string {
	username := "unknown"
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		username = u.Username
	}
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s@%s via %s", username, host, filepath.Base(os.Args[0]))
}

// runReattest resolves the branch to scan (linkage FIRST and WINNING;
// --spec-branch fallback-only scoping), invokes the derivation engine,
// and renders every fail-closed refusal as an ADR-0035 guard failure
// with a named forward exit.
func runReattest(deps reattestDeps, root, beadID, specBranchFlag string) error {
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return fmt.Errorf("invalid bead id: %w", err)
	}
	safeBeadID := idrender.Bead(beadID)

	derived, deriveErr := deps.deriveSpecBranch(beadID)
	var scanned string
	switch {
	case deriveErr == nil && derived != "":
		// Epic linkage derivable — it WINS. --spec-branch is consulted
		// ONLY when the linkage is underivable (plan-gate F2-2), so a
		// supplied flag is ignored, loudly.
		scanned = derived
		if strings.TrimSpace(specBranchFlag) != "" {
			fmt.Fprintf(deps.stderr, "note: --spec-branch %s ignored — the bead's epic linkage derives %s, and the linkage wins whenever derivable (--spec-branch is fallback-only scoping)\n",
				termsafe.Escape(specBranchFlag), derived)
		}
	case strings.TrimSpace(specBranchFlag) != "":
		// Linkage underivable (or the lookup failed) and the operator
		// scoped the scan explicitly.
		normalized, normErr := normalizeSpecBranchFlag(specBranchFlag)
		if normErr != nil {
			return guard.NewFailure(
				fmt.Sprintf("--spec-branch %s is not a valid spec branch (want spec/<spec-id>): %v", termsafe.Escape(specBranchFlag), normErr),
				fmt.Sprintf("mindspec reattest %s --spec-branch spec/<spec-id>", safeBeadID),
			)
		}
		scanned = normalized
		if deriveErr != nil {
			fmt.Fprintf(deps.stderr, "warning: resolving %s's epic linkage failed (%s) — scanning the operator-scoped %s instead; the scanned branch is recorded in the audit\n",
				safeBeadID, termsafe.Escape(deriveErr.Error()), scanned)
		}
	case deriveErr != nil:
		return guard.NewFailure(
			fmt.Sprintf("cannot resolve the spec branch for bead %s: the epic-linkage lookup failed: %s", safeBeadID, termsafe.Escape(deriveErr.Error())),
			fmt.Sprintf("mindspec reattest %s --spec-branch spec/<spec-id>   (name the branch to scan explicitly)", safeBeadID),
		)
	default:
		return guard.NewFailure(
			fmt.Sprintf("cannot derive the spec branch for bead %s from its epic linkage (no lineage recorded)", safeBeadID),
			fmt.Sprintf("mindspec reattest %s --spec-branch spec/<spec-id>   (name the branch to scan explicitly)", safeBeadID),
		)
	}

	res, err := deps.reattest(root, scanned, beadID, deps.actor)
	if err != nil {
		var refusal *lifecycle.ReattestRefusal
		if errors.As(err, &refusal) {
			return reattestRefusalFailure(safeBeadID, beadBranch, scanned, refusal)
		}
		return err
	}

	if !res.Wrote {
		fmt.Fprintf(deps.stdout, "Landed-binding for bead %s already git-corroborates to merge %s (second parent %s) on %s — convergent no-op, nothing written.\n",
			safeBeadID, res.MergeSHA, res.SecondParent, res.SpecBranch)
		return nil
	}
	fmt.Fprintf(deps.stdout, "Re-attested landed-binding for bead %s on %s:\n  merge         %s\n  second parent %s\n  corroboration %s\n",
		safeBeadID, res.SpecBranch, res.MergeSHA, res.SecondParent, res.Corroboration)
	if res.PriorMergeSHA != "" || res.PriorSecondParent != "" {
		// G3-1: contradictory prior binding overwritten with the
		// git-corroborated exact identity; before-values are in the audit.
		fmt.Fprintf(deps.stdout, "  prior binding (recorded in the audit): merge %s, second parent %s\n",
			termsafe.Escape(res.PriorMergeSHA), termsafe.Escape(res.PriorSecondParent))
	}
	fmt.Fprintf(deps.stdout, "Audit recorded in bd metadata (mindspec_landed_reattest_*): inspect with `bd show %s --json`.\n", safeBeadID)
	return nil
}

// reattestRefusalFailure maps each fail-closed ReattestRefusal state to
// an ADR-0035 guard failure with a genuine forward exit (§2(i): never a
// bare re-invocation that cannot change the refused fact).
func reattestRefusalFailure(safeBeadID, beadBranch, scanned string, r *lifecycle.ReattestRefusal) error {
	inspect := fmt.Sprintf("git log --first-parent --merges %s   (inspect the candidate merges before any recovery)", scanned)
	switch r.State {
	case lifecycle.ReattestStateNoOwnedMerge:
		// Truly-bare: the audited ADR-0035 mindspec-q9ea human
		// attested-restore is the ONLY exit — named explicitly, and
		// deliberately non-mechanical (verify before running, never blind).
		return guard.NewFailure(
			fmt.Sprintf("cannot re-attest bead %s: %s.\n"+
				"this is the genuinely-no-mechanical-corroboration state — the blessed exit is the audited ADR-0035 mindspec-q9ea human attested-restore: locate the merge you believe carried this bead's work, VERIFY it by inspecting its diff, then restore the branch ref at its second parent (`git branch %s <verified-second-parent-sha>`) — never run it blindly.",
				safeBeadID, r.Detail, beadBranch),
			inspect,
			fmt.Sprintf("git branch %s <verified-second-parent-sha>   (ADR-0035 q9ea attested-restore — human verification REQUIRED first)", beadBranch),
		)
	case lifecycle.ReattestStateAmbiguous:
		return guard.NewFailure(
			fmt.Sprintf("cannot re-attest bead %s: %s.\nre-attestation is fail-closed on ambiguity — it never guesses among competing landings.", safeBeadID, r.Detail),
			inspect,
		)
	case lifecycle.ReattestStateTipContradiction, lifecycle.ReattestStatePanelContradiction:
		return guard.NewFailure(
			fmt.Sprintf("cannot re-attest bead %s: %s.\na corroborating datum CONTRADICTS the owned merge — the candidate is not attestable (decoy/stale state); nothing was written.", safeBeadID, r.Detail),
			inspect,
		)
	case lifecycle.ReattestStateReverted:
		return guard.NewFailure(
			fmt.Sprintf("cannot re-attest bead %s: %s.\nre-attesting reverted content would forge landed evidence; nothing was written.", safeBeadID, r.Detail),
			inspect,
		)
	default:
		return guard.NewFailure(
			fmt.Sprintf("cannot re-attest bead %s: %s", safeBeadID, r.Detail),
			inspect,
		)
	}
}

func init() {
	// The ONLY flag on this surface (AC-8's flag-set assertion): scoping
	// input, fallback-only, never a corroboration substitute. There is
	// deliberately no --all, no bypass, and no SHA-assertion flag.
	reattestCmd.Flags().String("spec-branch", "", "Spec branch to scan when the bead's epic linkage is underivable (scoping only; recorded in the audit)")
}
