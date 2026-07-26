package approve

// Spec 127 R1 — AdoptSpec: an audited, evidence-gated, MERGE-FREE path
// to a review-state spec's terminal state, for the case where the
// spec's content reached main by another route (GH #218's cluster). See
// adopt_lattice.go for the evidence aggregation this entrypoint consumes.
//
// AdoptSpec performs NO spec->main merge: the only main-side mutation is
// the finalize-export commit (exec.CommitAll, which refreshes
// .beads/issues.jsonl and commits — never a merge). Every refusal leg
// below fires before that mutation.

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// AdoptOpts holds options for AdoptSpec.
type AdoptOpts struct {
	// Reason is the required, operator-supplied justification (AC-2(i)):
	// AdoptSpec refuses before any mutation when this is empty.
	Reason string
	// AttestUnverified is R1(c)'s attestation escape flag
	// (--attest-unverified): honored in exactly the three pinned
	// trigger states (no-source / negative / error), never over an
	// all-positive-with-coverage aggregate.
	AttestUnverified bool
}

// AdoptResult holds the result of a successful AdoptSpec call — either a
// VERIFIED terminal transition or an attested one (Verified == false).
type AdoptResult struct {
	SpecID   string
	EpicID   string
	Verified bool
	// Trigger names which of R1(c)'s three pinned aggregate states the
	// attestation escape was exercised in. Empty when Verified is true.
	Trigger adoptAttestTrigger
}

// --- in-package seams (default-pinned to the real functions).
var (
	adoptFindEpicFn        = phase.FindEpicBySpecID
	adoptScanOrphanFn      = adoptScanOrphanPresent
	adoptEvaluateLatticeFn = evaluateAdoptLattice
	adoptRunBDCombinedFn   = bead.RunBDCombined
	adoptMergeMetadataFn   = bead.MergeMetadata
	adoptGitUserEmailFn    = bead.GitUserEmail
)

// AdoptSpec is R1's entrypoint. It is NEVER IMPLICIT (R1(e)): the only
// production caller is the `mindspec impl adopt <spec-id>` command
// handler (cmd/mindspec/impl.go) — pinned by
// cmd/mindspec/impl_adopt_test.go's call-site enumeration.
func AdoptSpec(root, specID string, exec executor.Executor, opts AdoptOpts) (*AdoptResult, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}

	// AC-2(i): --reason is required, refused before any I/O.
	reason := strings.TrimSpace(opts.Reason)
	if reason == "" {
		return nil, guard.NewFailure(
			fmt.Sprintf("mindspec impl adopt %s requires --reason \"<text>\": adopt is an audited, merge-free terminal transition and must not run without a recorded justification", idrender.Spec(specID)),
			fmt.Sprintf(`mindspec impl adopt %s --reason "<why this spec's content already reached main outside the lifecycle>"`, idrender.Spec(specID)),
		)
	}

	epicID, err := adoptFindEpicFn(specID)
	if err != nil {
		return nil, fmt.Errorf("no epic found for spec %s: %w", idrender.Spec(specID), err)
	}
	if epicID == "" {
		return nil, guard.NewFailure(
			fmt.Sprintf("spec %s has no lifecycle epic to adopt", idrender.Spec(specID)),
			"mindspec spec list   (confirm the spec id and its lifecycle state)",
		)
	}

	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return nil, err
	}

	// R1(d): branch-present refusals. This is the LOCAL spec branch —
	// adopt exists for exactly the case where it is gone; when it is
	// still here, the disposition depends on whether it is current (the
	// normal `impl approve` path still applies) or stale (the #218
	// step-2 recreated-branch shape).
	if adoptBranchExistsFn(root, specBranch) {
		st, outcome, evalErr := adoptEvaluateAgainstMainFn(root, specBranch)
		if evalErr != nil {
			return nil, adoptEvidenceErrorRefusal(specID, fmt.Sprintf("could not evaluate spec branch %s against main: %v", specBranch, evalErr))
		}
		_ = st
		switch outcome {
		case guard.DestructionSuperseded, guard.DestructionStaleDeletion:
			return nil, adoptStaleBranchPresentRefusal(specID, specBranch, outcome)
		default:
			return nil, adoptCurrentBranchPresentRefusal(specID, specBranch)
		}
	}

	// R1(g), interim (bead 3 scope — the R2-derived hint arrives with
	// bead 4): the composite incident state. A CLOSED bead whose
	// surviving bead branch is not landed in main refuses BEFORE
	// mutation, with an inspection-first message carrying NO destructive
	// command (declared in the plan's Decomposition so this interim
	// state is priced, not discovered).
	orphanBeadID, orphanBranch, orphanErr := adoptScanOrphanFn(root, epicID)
	if orphanErr != nil {
		return nil, adoptEvidenceErrorRefusal(specID, fmt.Sprintf("could not scan epic %s's closed beads for a stale surviving branch: %v", idrender.Bead(epicID), orphanErr))
	}
	if orphanBeadID != "" {
		return nil, adoptOrphanPresentRefusal(specID, orphanBeadID, orphanBranch)
	}

	// R1(b): the aggregation lattice.
	result, latticeErr := adoptEvaluateLatticeFn(root, specID, epicID, specBranch)
	if latticeErr != nil {
		return nil, fmt.Errorf("evaluating adopt evidence for spec %s: %w", idrender.Spec(specID), latticeErr)
	}

	if result.Class != adoptClassNone {
		// A refusal state. By construction every non-None class maps
		// onto one of R1(c)'s three pinned trigger states, so attestation
		// is available whenever we reach this branch at all (never over
		// the all-positive-with-coverage aggregate, which never reaches
		// here).
		if opts.AttestUnverified {
			if err := adoptFinalize(root, exec, specID, epicID, reason, false, result.Trigger); err != nil {
				return nil, err
			}
			return &AdoptResult{SpecID: specID, EpicID: epicID, Verified: false, Trigger: result.Trigger}, nil
		}
		return nil, adoptRefusalFailure(specID, result)
	}

	if err := adoptFinalize(root, exec, specID, epicID, reason, true, ""); err != nil {
		return nil, err
	}
	return &AdoptResult{SpecID: specID, EpicID: epicID, Verified: true}, nil
}

// adoptScanOrphanPresent is R1(g)'s interim closed-bead-orphan check: any
// CLOSED bead under epicID whose bead/<id> branch survives and evaluates
// negative against main (via the shared predicate). Best-effort per-bead
// on an evaluation error (skip and keep scanning) is SAFE here only
// because evaluateAdoptLattice below re-evaluates every surviving branch
// (closed or not) and fails closed on the identical error — this function
// is a strictly-earlier, narrower gate layered in front of that one, not
// a replacement for its fail-closed discipline.
func adoptScanOrphanPresent(root, epicID string) (beadID, beadBranch string, err error) {
	beads, listErr := adoptListEpicBeadsFn(root, epicID)
	if listErr != nil {
		return "", "", listErr
	}
	for _, b := range beads {
		if b.Status != "closed" {
			continue
		}
		branch, branchErr := workspace.BeadBranch(b.ID)
		if branchErr != nil {
			continue
		}
		if !adoptBranchExistsFn(root, branch) {
			continue
		}
		st, _, evalErr := adoptEvaluateAgainstMainFn(root, branch)
		if evalErr != nil {
			// evaluateAdoptLattice re-evaluates this exact branch and
			// will fail closed on the same error; this leg simply does
			// not double-report it under interim wording.
			continue
		}
		if st == evidenceNegative {
			return b.ID, branch, nil
		}
	}
	return "", "", nil
}

// adoptFinalize is R1(a)'s terminal transition: close the epic, write
// the same done-state phase metadata a normal `impl approve` writes
// (R1(f)'s no-third-terminal-shape parity), record the adopt audit
// marker, and write the finalize export — via exec.CommitAll, which
// refreshes .beads/issues.jsonl and commits (never a merge; the ONLY
// main-side delta this surface ever produces).
func adoptFinalize(root string, exec executor.Executor, specID, epicID, reason string, verified bool, trigger adoptAttestTrigger) error {
	// Gate-all-ids (ADR-0042 §1): epicID feeds a `bd close` argv build
	// directly — validated before any bd spawn (defense-in-depth atop
	// phase.FindEpicBySpecID's own return-gating, the impl.go
	// ApproveImpl precedent at its equivalent MUTATION (1/3) step).
	if err := idvalidate.BeadID(epicID); err != nil {
		return fmt.Errorf("resolved epic id %s is invalid: %w", idrender.Bead(epicID), err)
	}
	if _, err := adoptRunBDCombinedFn("close", epicID); err != nil && !isAlreadyClosedErr(err) {
		return fmt.Errorf("closing epic %s: %w", idrender.Bead(epicID), err)
	}

	meta := map[string]interface{}{
		"mindspec_phase":        "done",
		"mindspec_done":         true,
		"mindspec_adopt_reason": reason,
		"mindspec_adopt_actor":  adoptActor(),
		"mindspec_adopt_at":     time.Now().UTC().Format(time.RFC3339),
		"mindspec_adopt_op":     "mindspec impl adopt",
	}
	if verified {
		meta["mindspec_adopt_evidence"] = "verified"
	} else {
		meta["mindspec_adopt_evidence"] = "attested"
		meta["mindspec_adopt_attest_trigger"] = string(trigger)
	}
	if err := adoptMergeMetadataFn(epicID, meta); err != nil {
		return fmt.Errorf("recording adopt audit marker on epic %s: %w", idrender.Bead(epicID), err)
	}

	commitMsg := fmt.Sprintf("chore(beads): adopt epic %s for spec %s (mindspec impl adopt)", idrender.Bead(epicID), idrender.Spec(specID))
	if err := exec.CommitAll(root, commitMsg); err != nil {
		return fmt.Errorf("writing finalize export for spec %s: %w", idrender.Spec(specID), err)
	}
	return nil
}

// adoptActor mirrors reattestActor's audit-identity convention (spec
// 125's cmd/mindspec/reattest.go): user@host via argv0. Best-effort
// degrade to "unknown" rather than blocking an audited transition on an
// unreadable passwd entry.
func adoptActor() string {
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

// --- refusal renderers -------------------------------------------------

// adoptCurrentBranchPresentRefusal is AC-2(iv): the local spec branch
// exists and is current — the normal path applies, never adopt.
func adoptCurrentBranchPresentRefusal(specID, specBranch string) error {
	return guard.NewFailure(
		fmt.Sprintf("spec %s's branch %s still exists and is current — the normal path applies, not adopt", idrender.Spec(specID), termsafe.Escape(specBranch)),
		fmt.Sprintf("mindspec impl approve %s", idrender.Spec(specID)),
	)
}

// adoptStaleBranchPresentRefusal is AC-2(v): the local spec branch
// exists but is stale (superseded/stale-deletion per the shared
// predicate) — the #218 step-2 shape. The deletion hint is
// CONSTRUCTOR-DERIVED FROM BIRTH (never seeded onto the guard allowlist):
// guard.NewDestructiveCommand is called with the predicate's own
// DestructionOutcome, so this site can never emit a raw, unconstructed
// `git branch -D` string.
func adoptStaleBranchPresentRefusal(specID, specBranch string, outcome guard.DestructionOutcome) error {
	// Error-handled bind, `.String()` reached only past the check — the
	// internal/lint convention scan's constructor-provenance trace
	// requires exactly this shape (a checked error that RETURNS on
	// failure, never a reassignment that falls through to the same
	// String() call): `git branch -D` is on the reviewed floor
	// (FamilyGitBranchDeleteForce) so this branch is unreachable in
	// practice, but if the floor ever stopped matching this exact family
	// underneath this call, refusing to render ANY deletion hint here is
	// the fail-closed choice — never an unconstructed one.
	deleteCmd, ctorErr := guard.NewDestructiveCommand(fmt.Sprintf("git branch -D %s", specBranch), outcome)
	if ctorErr != nil {
		return adoptEvidenceErrorRefusal(specID, fmt.Sprintf("could not construct the stale-branch deletion recovery for %s: %v", termsafe.Escape(specBranch), ctorErr))
	}
	return guard.NewFailure(
		fmt.Sprintf("spec %s's branch %s exists but is stale (%s per the shared work-destruction predicate) — the normal path would merge stale content into main", idrender.Spec(specID), termsafe.Escape(specBranch), outcome),
		fmt.Sprintf("git diff main %s   (inspect before deleting)", specBranch),
		deleteCmd.String(),
		fmt.Sprintf(`mindspec impl adopt %s --reason "<why>"   (re-run once the stale branch is deleted)`, idrender.Spec(specID)),
	)
}

// adoptOrphanPresentRefusal is R1(g)'s INTERIM composite-incident
// refusal (AC-2(viii)'s bead-3 slice): inspection-first, no destructive
// command. Bead 4 upgrades this to the full R2-derived hint (evidence-
// proven stale-branch deletion with the preserve-first clause).
func adoptOrphanPresentRefusal(specID, beadID, beadBranch string) error {
	return guard.NewFailure(
		fmt.Sprintf(
			"spec %s's epic has a closed bead %s whose branch %s still exists and is not landed in main — this is the composite incident state (a stale bead branch alongside externally-landed spec content); adopting now, before that branch's state is resolved, is refused",
			idrender.Spec(specID), idrender.Bead(beadID), termsafe.Escape(beadBranch),
		),
		fmt.Sprintf("git diff main %s   (inspect %s's unlanded work before doing anything else)", beadBranch, idrender.Bead(beadID)),
		fmt.Sprintf(`mindspec impl adopt %s --reason "<why>"   (re-run once %s's branch state is resolved)`, idrender.Spec(specID), idrender.Bead(beadID)),
	)
}

// adoptEvidenceErrorRefusal is AC-2(vi): a fail-closed, retryable
// refusal, wording distinct from the negative refusal, naming retry
// FIRST and stating the attestation escape remains available.
func adoptEvidenceErrorRefusal(specID, detail string) error {
	return guard.NewFailure(
		fmt.Sprintf("%s: could not verify spec %s's landed evidence — %s", adoptMarkerEvidenceError, idrender.Spec(specID), detail),
		fmt.Sprintf(`mindspec impl adopt %s --reason "<why>"   (retry once the underlying failure is resolved)`, idrender.Spec(specID)),
		fmt.Sprintf(`mindspec impl adopt %s --reason "<why>" --attest-unverified   (attestation remains available)`, idrender.Spec(specID)),
	)
}

// adoptRefusalFailure renders the lattice's four non-attested refusal
// classes. Each carries its own class marker (F-r5-3) and, for the three
// non-error classes, names the attestation escape as the forward exit.
func adoptRefusalFailure(specID string, r adoptLatticeResult) error {
	body := fmt.Sprintf("%s: %s", r.Marker, r.Detail)
	attestLine := fmt.Sprintf(`mindspec impl adopt %s --reason "<why>" --attest-unverified   (attestation escape — records landing was NOT verified, trigger=%s)`, idrender.Spec(specID), r.Trigger)

	switch r.Class {
	case adoptClassNegative:
		return guard.NewFailure(body,
			"git log --first-parent --merges main   (inspect main for a landed merge of this spec's work)",
			attestLine,
		)
	case adoptClassSourcesConflict:
		return guard.NewFailure(body,
			"git log --first-parent --merges main   (the corroborated spec ref and a surviving bead branch disagree — inspect before proceeding)",
			attestLine,
		)
	case adoptClassCoverageUnavailable:
		return guard.NewFailure(body,
			fmt.Sprintf("bd list --parent <epic-id> --status=%s   (inspect the epic's beads for the uncovered one)", "open,in_progress,blocked,closed"),
			attestLine,
		)
	case adoptClassEvidenceError:
		return adoptEvidenceErrorRefusal(specID, r.Detail)
	default:
		return fmt.Errorf("adopt: unhandled refusal class %d for spec %s", r.Class, idrender.Spec(specID))
	}
}
