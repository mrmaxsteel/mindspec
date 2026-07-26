package approve

// Spec 127 R1 — AdoptSpec: an audited, evidence-gated, MERGE-FREE path
// to a review-state spec's terminal state, for the case where the
// spec's content reached main by another route (GH #218's cluster). See
// adopt_lattice.go for the evidence aggregation this entrypoint consumes.
//
// AdoptSpec performs NO spec->main merge: the only main-side mutation is
// the finalize-export commit (exec.CommitPaths, staging and committing
// ONLY the refreshed .beads/issues.jsonl artifact — never `git add -A`,
// and never a merge; fix round G1-B3-01). Every refusal leg below fires
// before that mutation.
//
// Fix round additions: a review-state precondition equivalent in
// strength to ApproveImpl's own review/done phase gate, plus an explicit
// open-lifecycle-bead refusal that never trusts a stale stored-phase
// cache (G1-B3-02); and a staged, idempotent finalize protocol that
// preserves the first run's audit payload and resumes only the missing
// export/commit stage on a re-run (G1-B3-03).

import (
	"encoding/json"
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
	"github.com/mrmaxsteel/mindspec/internal/state"
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
	adoptFindEpicFn          = phase.FindEpicBySpecID
	adoptDerivePhaseDetailFn = phase.DerivePhaseDetail
	adoptLifecycleChildIDsFn = phase.LifecycleChildIDsForEpic
	adoptReadBeadStatusFn    = adoptReadBeadStatus
	adoptScanOrphanFn        = adoptScanOrphanPresent
	adoptEvaluateLatticeFn   = evaluateAdoptLattice
	adoptRunBDCombinedFn     = bead.RunBDCombined
	adoptGetMetadataFn       = bead.GetMetadata
	adoptMergeMetadataFn     = bead.MergeMetadata
	adoptGitUserEmailFn      = bead.GitUserEmail
	adoptExportBeadsFn       = bead.Export
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

	// R1 precondition (G1-B3-02): adopt is only for a REVIEW-STATE spec —
	// its own lifecycle work already complete, only the final merge-to-
	// main missing (the #218 shape). Equivalent in strength to
	// ApproveImpl's own review/done gate (impl.go): the SAME stored-cache-
	// trusted-else-derived mechanism (internal/phase), so adopt's gate can
	// never be weaker than the normal path's. A HARD refusal — no
	// attestation escape — because this gates the INPUT (is this even a
	// review-state spec), never one of R1(c)'s three evidence trigger
	// states.
	phaseDetail, phaseErr := adoptDerivePhaseDetailFn(epicID)
	if phaseErr != nil {
		return nil, fmt.Errorf("deriving phase for spec %s: %w", idrender.Spec(specID), phaseErr)
	}
	if !adoptPhaseGateOK(phaseDetail.Stored) && !adoptPhaseGateOK(phaseDetail.Derived) {
		return nil, guard.NewFailure(
			fmt.Sprintf("mindspec impl adopt %s requires a review-state spec: stored phase %q and child-derived phase %q both fail the review/done gate — adopt is only for a spec whose lifecycle work is already complete but whose content reached main by another route", idrender.Spec(specID), phaseDetail.Stored, phaseDetail.Derived),
			"mindspec complete <bead-id>   (close remaining lifecycle beads, then re-run)",
			fmt.Sprintf(`mindspec impl adopt %s --reason "<why>"   (retry once the spec is in review mode)`, idrender.Spec(specID)),
		)
	}

	// Explicit open-lifecycle-bead refusal (G1-B3-02): the phase gate
	// above can pass on a STORED cache alone (trusted, matching
	// ApproveImpl's own stale-cache-tolerant behavior) — this check never
	// trusts the cache. It refuses over any lifecycle child that is not
	// closed RIGHT NOW, naming it, before any mutation — closing the
	// adversary's gap where an open lifecycle bead with landed branch
	// evidence would otherwise be silently adopted.
	lifecycleChildren, lcErr := adoptLifecycleChildIDsFn(epicID)
	if lcErr != nil {
		return nil, adoptEvidenceErrorRefusal(specID, fmt.Sprintf("could not classify epic %s's lifecycle children: %v", idrender.Bead(epicID), lcErr))
	}
	for _, lcID := range lifecycleChildren {
		status, statusErr := adoptReadBeadStatusFn(lcID)
		if statusErr != nil {
			return nil, adoptEvidenceErrorRefusal(specID, fmt.Sprintf("could not read lifecycle bead %s's status: %v", idrender.Bead(lcID), statusErr))
		}
		if status != "closed" {
			return nil, guard.NewFailure(
				fmt.Sprintf("spec %s's epic has an open lifecycle bead %s (status %q) — adopt refuses until the epic's own lifecycle work is confirmed complete", idrender.Spec(specID), idrender.Bead(lcID), status),
				fmt.Sprintf("mindspec complete %s", idrender.Bead(lcID)),
			)
		}
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
	specBranchExists, specBranchExistsErr := adoptBranchExistsFn(root, specBranch)
	if specBranchExistsErr != nil {
		return nil, adoptEvidenceErrorRefusal(specID, fmt.Sprintf("could not check whether spec branch %s exists: %v", specBranch, specBranchExistsErr))
	}
	if specBranchExists {
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
	orphanBeadID, orphanBranch, orphanOutcome, orphanErr := adoptScanOrphanFn(root, epicID)
	if orphanErr != nil {
		return nil, adoptEvidenceErrorRefusal(specID, fmt.Sprintf("could not scan epic %s's closed beads for a stale surviving branch: %v", idrender.Bead(epicID), orphanErr))
	}
	if orphanBeadID != "" {
		return nil, adoptOrphanPresentRefusal(specID, orphanBeadID, orphanBranch, orphanOutcome)
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
		return nil, adoptRefusalFailure(root, specID, result)
	}

	if err := adoptFinalize(root, exec, specID, epicID, reason, true, ""); err != nil {
		return nil, err
	}
	return &AdoptResult{SpecID: specID, EpicID: epicID, Verified: true}, nil
}

// adoptPhaseGateOK is the review-state precondition's gate predicate
// (G1-B3-02) — identical to ApproveImpl's own implGateOK (impl.go): a
// phase satisfies adopt's precondition iff it is review or done.
func adoptPhaseGateOK(p string) bool { return p == state.ModeReview || p == state.ModeDone }

// adoptReadBeadStatus reads bead id's current status via `bd show <id>
// --json` — the same call shape as impl.go's readBeadStatus, kept
// independent so adopt's own bd seam family (adoptListBDFn) is the
// single point test fixtures repoint for this package, without also
// reaching into impl.go's implRunBDFn seam.
func adoptReadBeadStatus(id string) (string, error) {
	if err := idvalidate.BeadID(id); err != nil {
		return "", fmt.Errorf("invalid bead id %s: %w", idrender.Bead(id), err)
	}
	out, err := adoptListBDFn("show", id, "--json")
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

// adoptScanOrphanPresent is R1(g)'s interim closed-bead-orphan check: any
// CLOSED bead under epicID whose bead/<id> branch survives and evaluates
// negative against main (via the shared predicate). Best-effort per-bead
// on an evaluation error (skip and keep scanning) is SAFE here only
// because evaluateAdoptLattice below re-evaluates every surviving branch
// (closed or not) and fails closed on the identical error — this function
// is a strictly-earlier, narrower gate layered in front of that one, not
// a replacement for its fail-closed discipline. The same reasoning
// covers a BranchExistsIn ERROR (as opposed to a genuine absence): the
// lattice probes the identical branch with the identical helper and
// fails closed on the same error.
//
// Returns the guard.DestructionOutcome the shared predicate produced
// when an orphan IS found (F1-1): bead 3 discards no evidence its own
// call already computed, so bead 4's upgrade to the full R2-derived hint
// is a pure call-site swap at adoptOrphanPresentRefusal, with no
// signature churn on this function. The returned outcome is meaningless
// when beadID == "" (no orphan found).
func adoptScanOrphanPresent(root, epicID string) (beadID, beadBranch string, outcome guard.DestructionOutcome, err error) {
	beads, listErr := adoptListEpicBeadsFn(root, epicID)
	if listErr != nil {
		return "", "", guard.DestructionAncestor, listErr
	}
	for _, b := range beads {
		if b.Status != "closed" {
			continue
		}
		branch, branchErr := workspace.BeadBranch(b.ID)
		if branchErr != nil {
			continue
		}
		exists, existsErr := adoptBranchExistsFn(root, branch)
		if existsErr != nil || !exists {
			continue
		}
		st, o, evalErr := adoptEvaluateAgainstMainFn(root, branch)
		if evalErr != nil {
			// evaluateAdoptLattice re-evaluates this exact branch and
			// will fail closed on the same error; this leg simply does
			// not double-report it under interim wording.
			continue
		}
		if st == evidenceNegative {
			return b.ID, branch, o, nil
		}
	}
	return "", "", guard.DestructionAncestor, nil
}

// adoptExportCommittedMetaKey marks that adoptFinalize's SECOND stage
// (export refresh + finalize commit) has durably completed (G1-B3-03).
// adoptFinalize is a two-stage protocol — stage 1 (close + audit
// metadata) then stage 2 (export + commit, marked by this key) — so a
// crash/failure between the two stages is RESUMABLE: a re-run sees stage
// 1's audit payload already present and never re-runs (and therefore
// never overwrites) it, resuming only the missing stage 2.
const adoptExportCommittedMetaKey = "mindspec_adopt_export_committed"

// adoptFinalize is R1(a)'s terminal transition: close the epic, write
// the same done-state phase metadata a normal `impl approve` writes
// (R1(f)'s no-third-terminal-shape parity), record the adopt audit
// marker, and write the finalize export — via exec.CommitPaths, staging
// and committing ONLY the refreshed .beads/issues.jsonl artifact (never
// `git add -A`; G1-B3-01 — this is the ONLY main-side delta this surface
// ever produces, confined to the finalize-export artifact, so an
// operator's unrelated dirty/untracked work in root is never swept into
// the terminal adoption commit).
//
// G1-B3-03: idempotent and crash-recoverable. Before any mutation, reads
// the epic's CURRENT metadata to classify which of four states it is in:
//   - already done via the NORMAL path (no adopt audit marker): refuse —
//     adopt is not a resume mechanism for that terminal shape.
//   - already fully adopted (audit marker present AND stage 2 marked
//     complete): refuse — a one-shot terminal transition never re-runs
//     (and therefore never overwrites) its own recorded reason/actor/
//     timestamp/evidence class.
//   - interrupted mid-adopt (audit marker present, stage 2 NOT marked
//     complete): resume — run ONLY stage 2, preserving stage 1's
//     original audit payload verbatim.
//   - fresh: run both stages in order.
//
// Confirm round (G1-B3-03 residual): stage 1 writes the audit-metadata
// merge BEFORE calling `bd close`, not after. A crash before that write
// leaves nothing durable at all (genuinely fresh — safe to retry from
// scratch, nothing to lose). A crash any time AFTER that write — whether
// or not the close call itself has run yet — lands in the "interrupted"
// branch below on the next attempt, because the audit marker is now the
// FIRST durable fact stage 1 produces, never the close call. That branch
// therefore always (idempotently) re-issues `bd close` before resuming
// stage 2, tolerating an already-closed epic exactly like the fresh path
// does. This closes the window the prior ordering left open: `bd close`
// succeeding with the audit-metadata write then failing used to leave
// the epic durably closed with NO record of who closed it or why, and a
// retry would reclassify as fresh and record a SECOND, possibly
// different, reason/trigger — permanently losing the first invocation's
// audit trail. With the write ordered first, that state can no longer
// occur: the audit payload is always durable before or at the same time
// as the close, never after it.
func adoptFinalize(root string, exec executor.Executor, specID, epicID, reason string, verified bool, trigger adoptAttestTrigger) error {
	// Gate-all-ids (ADR-0042 §1): epicID feeds a `bd close`/`bd show` argv
	// build directly — validated before any bd spawn (defense-in-depth
	// atop phase.FindEpicBySpecID's own return-gating, the impl.go
	// ApproveImpl precedent at its equivalent MUTATION (1/3) step).
	if err := idvalidate.BeadID(epicID); err != nil {
		return fmt.Errorf("resolved epic id %s is invalid: %w", idrender.Bead(epicID), err)
	}

	existing, metaErr := adoptGetMetadataFn(epicID)
	if metaErr != nil {
		return fmt.Errorf("reading epic %s's current metadata before finalizing: %w", idrender.Bead(epicID), metaErr)
	}
	_, alreadyAdopted := existing["mindspec_adopt_reason"]
	alreadyDone, _ := existing["mindspec_done"].(bool)
	stage2Done, _ := existing[adoptExportCommittedMetaKey].(bool)

	switch {
	case alreadyDone && !alreadyAdopted:
		return guard.NewFailure(
			fmt.Sprintf("spec %s's epic %s already reached the done state via the normal path (no adopt audit marker present) — nothing for adopt to do", idrender.Spec(specID), idrender.Bead(epicID)),
			fmt.Sprintf("bd show %s --json   (inspect the epic's existing done-state metadata)", idrender.Bead(epicID)),
		)
	case alreadyAdopted && stage2Done:
		return guard.NewFailure(
			fmt.Sprintf("spec %s's epic %s was already adopted (reason %q) — adopt is a one-shot terminal transition and refuses to run again", idrender.Spec(specID), idrender.Bead(epicID), existing["mindspec_adopt_reason"]),
			fmt.Sprintf("bd show %s --json   (inspect the recorded adopt audit marker)", idrender.Bead(epicID)),
		)
	case alreadyAdopted:
		// Interrupted: stage 1's audit payload already landed on a PRIOR
		// run and is preserved verbatim — never re-derived from THIS
		// invocation's reason/verified/trigger. Because the audit write
		// now durably precedes `bd close` (see the doc comment above),
		// reaching this branch does not guarantee close itself already
		// ran, so it is always (idempotently) re-issued here;
		// isAlreadyClosedErr tolerates the case where it already
		// succeeded on the prior run.
		if _, err := adoptRunBDCombinedFn("close", epicID); err != nil && !isAlreadyClosedErr(err) {
			return fmt.Errorf("closing epic %s: %w", idrender.Bead(epicID), err)
		}
		return adoptFinalizeExportStage(root, exec, specID, epicID)
	}

	// Fresh run: stage 1. The audit-metadata merge is written FIRST,
	// before `bd close` — see the doc comment above for why the ordering
	// itself is the fix. A crash before this write leaves nothing durable
	// (fresh, safe to retry). A crash after it is resumed by the
	// "interrupted" branch above, which re-issues close idempotently.
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

	if _, err := adoptRunBDCombinedFn("close", epicID); err != nil && !isAlreadyClosedErr(err) {
		return fmt.Errorf("closing epic %s: %w", idrender.Bead(epicID), err)
	}

	return adoptFinalizeExportStage(root, exec, specID, epicID)
}

// adoptFinalizeExportStage is adoptFinalize's SECOND stage (G1-B3-03):
// refresh the tracked beads export and commit ONLY that artifact
// (G1-B3-01), then record that this stage has durably completed.
// exec.CommitPaths is itself a no-op when nothing is staged, so a resumed
// run whose export+commit already landed on a PRIOR partial run (one that
// failed only at the metadata-mark-complete write below) commits nothing
// new and simply marks completion.
func adoptFinalizeExportStage(root string, exec executor.Executor, specID, epicID string) error {
	if err := adoptExportBeadsFn(root); err != nil {
		return fmt.Errorf("refreshing .beads/issues.jsonl for spec %s: %w", idrender.Spec(specID), err)
	}
	commitMsg := fmt.Sprintf("chore(beads): adopt epic %s for spec %s (mindspec impl adopt)", idrender.Bead(epicID), idrender.Spec(specID))
	if err := exec.CommitPaths(root, commitMsg, []string{".beads/issues.jsonl"}); err != nil {
		return fmt.Errorf("writing finalize export for spec %s: %w", idrender.Spec(specID), err)
	}
	if err := adoptMergeMetadataFn(epicID, map[string]interface{}{adoptExportCommittedMetaKey: true}); err != nil {
		return fmt.Errorf("recording adopt export-committed marker on epic %s: %w", idrender.Bead(epicID), err)
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
//
// outcome is UNUSED for now (F1-1): adoptScanOrphanPresent already
// computes and returns it (the shared predicate's own
// guard.DestructionOutcome for the orphaned branch), so bead 4's
// upgrade — swapping this interim message for the full R2-derived hint —
// is a pure call-site change with no signature churn on
// adoptScanOrphanPresent, whose caller (AdoptSpec) already has the value
// to hand.
func adoptOrphanPresentRefusal(specID, beadID, beadBranch string, outcome guard.DestructionOutcome) error { //nolint:unparam // F1-1: threaded now so bead 4's upgrade to the full R2-derived hint is a pure call-site swap; consumed once that lands
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
func adoptRefusalFailure(root, specID string, r adoptLatticeResult) error {
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
		// O1-3: the hint's --status= list is COMPUTED at the same
		// AllStatuses breadth the lattice itself just enumerated at
		// (bead.AllStatuses), never the 4 built-ins hardcoded — a project
		// with a custom status would otherwise have an operator's
		// copy-pasted recovery command miss exactly the bead sitting in
		// that custom status.
		return guard.NewFailure(body,
			fmt.Sprintf("bd list --parent <epic-id> --status=%s   (inspect the epic's beads for the uncovered one)", strings.Join(bead.AllStatuses(root), ",")),
			attestLine,
		)
	case adoptClassEvidenceError:
		return adoptEvidenceErrorRefusal(specID, r.Detail)
	default:
		return fmt.Errorf("adopt: unhandled refusal class %d for spec %s", r.Class, idrender.Spec(specID))
	}
}
