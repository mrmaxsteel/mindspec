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
//     ONLY when the bead's epic linkage is cleanly, determinately
//     ABSENT (no lineage recorded — fallback-only, plan-gate F2-2). A
//     lineage LOOKUP ERROR is INDETERMINATE ownership and fails CLOSED
//     (spec 125 final-review FIX-1): the flag must never scope a scan
//     whose ownership could not be established — fix the lookup and
//     re-run. The branch actually scanned is recorded in the audit
//     (mindspec_landed_reattest_scanned_branch) either way.

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
	Short: "Derive a landed binding from an independent git scan",
	Long: `Recover a missing or stale landed binding from an independent git scan.
The scan checks two-parent first-parent merges whose subjects name the bead.
With no candidate, it checks reachable source-named merges. That fallback
requires one candidate, no contradictory surviving tip, and content that
was not reverted. Read errors refuse. Review registrations are advisory.
An accepted merge writes its derived SHAs and an audit record in metadata.

When neither scan finds a candidate, the tool checks the bead's full work
range at the spec tip. Present content with no naming merge writes no
binding. A re-merge cannot help. A person must verify the work before using
the printed readiness override for each open dependent. With no dependent,
the recovery is mindspec doctor. Reverted content, an unknown base, and
another bead's exact tip receive no override recommendation.

An attested restore requires a person to verify the merge and its diff
first. The tool accepts no operator-supplied merge or second-parent SHA.
There is no bypass flag or batch mode.

--spec-branch selects the scan branch only when the bead has no epic
linkage. A known lineage decides the branch. A failed lineage lookup
refuses even with the flag. The audit records the branch scanned.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := strings.TrimSpace(args[0])
		// R3 explicit-ingress early gate (ADR-0042): a malformed beadID
		// refuses HERE, before any composition or bd/git spawn.
		if err := idvalidate.BeadID(beadID); err != nil {
			return guard.NewFailure(
				fmt.Sprintf("%s is not a valid bead ID: %v", termsafe.Escape(beadID), err),
				"bd list --status closed",
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
	// linkage is cleanly, determinately ABSENT (no lineage recorded); an
	// error is a real lookup failure — INDETERMINATE ownership, on which
	// runReattest fails closed even when --spec-branch was supplied
	// (FIX-1).
	deriveSpecBranch func(beadID string) (string, error)
	// reattest is the derivation engine (lifecycle.ReattestLandedMerge).
	reattest func(root, specBranch, beadID, actor string, report ...func(string)) (*lifecycle.ReattestResult, error)
	// actor is the audit acting-identity value (user@host via argv0).
	actor  string
	stdout io.Writer
	stderr io.Writer
}

// deriveSpecBranchFromLineage is the production linkage resolver: bead →
// parent epic → spec ID → spec/<id> via the workspace composition waist.
// A genuinely-lineage-less bead yields ("", nil); real lookup failures
// propagate, and runReattest FAILS CLOSED on them — --spec-branch never
// substitutes for an indeterminate lookup (FIX-1).
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
// --spec-branch consulted ONLY when the lineage is cleanly ABSENT — a
// lineage LOOKUP ERROR fails closed even with the flag, FIX-1), invokes
// the derivation engine, and renders every fail-closed refusal as an
// ADR-0035 guard failure with a named forward exit.
func runReattest(deps reattestDeps, root, beadID, specBranchFlag string) error {
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return fmt.Errorf("invalid bead id: %w", err)
	}
	safeBeadID := idrender.Bead(beadID)

	derived, deriveErr := deps.deriveSpecBranch(beadID)
	var scanned string
	switch {
	case deriveErr != nil:
		// FIX-1 (spec 125 final-review, MAJOR): a lineage LOOKUP ERROR is
		// INDETERMINATE ownership — not the determinate no-lineage state
		// the flag fallback exists for. Refuse even when --spec-branch was
		// supplied: an operator flag must never scope a scan whose
		// ownership could not be established (the flag would silently
		// substitute for a lookup that might have derived a DIFFERENT
		// branch). The forward exit is fixing the lookup, not the flag.
		return guard.NewFailure(
			fmt.Sprintf("cannot resolve the spec branch for bead %s: the epic-linkage lookup FAILED (%s). This is an indeterminate-ownership state, so --spec-branch is refused too — the flag scopes only a bead whose lineage is cleanly absent, never one whose lookup errored.", safeBeadID, termsafe.Escape(deriveErr.Error())),
			fmt.Sprintf("bd show %s --json", safeBeadID),
		)
	case derived != "":
		// Epic linkage derivable — it WINS. --spec-branch is consulted
		// ONLY when the linkage is cleanly absent (plan-gate F2-2), so a
		// supplied flag is ignored, loudly.
		scanned = derived
		if strings.TrimSpace(specBranchFlag) != "" {
			fmt.Fprintf(deps.stderr, "note: --spec-branch %s ignored — the bead's epic linkage derives %s, and the linkage wins whenever derivable (--spec-branch is fallback-only scoping)\n",
				termsafe.Escape(specBranchFlag), derived)
		}
	case strings.TrimSpace(specBranchFlag) != "":
		// Lineage cleanly, determinately ABSENT (no epic link exists, no
		// error) and the operator scoped the scan explicitly — the ONLY
		// state in which the flag is honored.
		normalized, normErr := normalizeSpecBranchFlag(specBranchFlag)
		if normErr != nil {
			return guard.NewFailure(
				fmt.Sprintf("--spec-branch %s is not a valid spec branch (want spec/<spec-id>): %v", termsafe.Escape(specBranchFlag), normErr),
				fmt.Sprintf("mindspec reattest %s --spec-branch spec/<spec-id>", safeBeadID),
			)
		}
		scanned = normalized
	default:
		return guard.NewFailure(
			fmt.Sprintf("cannot derive the spec branch for bead %s from its epic linkage (no lineage recorded)", safeBeadID),
			fmt.Sprintf("mindspec reattest %s --spec-branch spec/<spec-id>", safeBeadID),
		)
	}

	res, err := deps.reattest(root, scanned, beadID, deps.actor, func(line string) { fmt.Fprintln(deps.stdout, line) })
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
	inspect := fmt.Sprintf("git log --first-parent --merges %s", scanned)
	if r.State == lifecycle.ReattestStateAncestor || r.NoRestore || r.State == lifecycle.ReattestStateIndeterminate {
		recovery := r.Recovery
		if len(recovery) == 0 {
			if r.State == lifecycle.ReattestStateNoOwnedMerge {
				// R6.4 and R6.7 keep the no-owned exit, without an override
				// or a branch restore that cannot fix this diagnosed state.
				recovery = []string{inspect}
			} else {
				recovery = []string{"mindspec doctor"}
			}
		}
		return guard.NewFailure("cannot re-attest bead "+safeBeadID+": "+r.Detail, recovery...)
	}
	if r.Fallback || r.State == lifecycle.ReattestStateMultipleFallbackCandidates {
		return guard.NewFailure(
			fmt.Sprintf("cannot re-attest bead %s: %s\nThe single-candidate, branch-tip, not-reverted and read checks must pass; nothing was written. For the dependent bead, override deliberately with fresh justification.", idrender.Bead(safeBeadID), r.Detail),
			fmt.Sprintf("git log --merges --topo-order %s", scanned),
			"mindspec next <dependent> --allow-not-ready=MF-3 --evidence <path-to-justification>",
		)
	}
	switch r.State {
	case lifecycle.ReattestStateNoOwnedMerge:
		// ADR-0041 §2(ii): verify the merge before an attested restore.
		return guard.NewFailure(
			fmt.Sprintf("cannot re-attest bead %s: %s\nUse an attested restore. A person must verify the merge and its diff first. Then restore the bead branch at that merge's second parent.", safeBeadID, r.Detail),
			inspect,
			fmt.Sprintf("git branch %s <verified-second-parent-sha>", beadBranch),
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
