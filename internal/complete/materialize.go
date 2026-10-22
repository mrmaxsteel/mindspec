package complete

// Spec 128 Bead 6 (OQ-1(a); ADR-0041 §1 amendment): the prospective-
// snapshot protocol's orchestration. This file is `mindspec complete`'s
// side of the M(aterialize)/R(eset)/S(ync)/G(merge)/T(erminal-clear)
// staged flow: it takes ONE snapshot of the bead worktree's prospective
// content after the panel gate, runs the per-bead content gates (doc-sync,
// ADR-divergence) against that snapshot BEFORE any commit exists, and —
// only on a full gate pass — materializes the snapshot into the bead
// branch's real commits via a write-ahead, at-least-once-idempotent
// protocol. See plan.md's "OQ-1 — #186 mechanism" section for the full,
// verbatim-authoritative protocol text this file transmits.
//
// ADR-0030: every git read/mutation here routes through one of the eight
// Executor seam methods (internal/executor/executor.go) — this package
// imports no internal/gitutil symbol, not even its two Mindspec-Stage
// string constants (duplicated below instead).

import (
	"fmt"
	"io"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// stageMaterialization/stageArtifactSync mirror
// gitutil.StageMaterialization/StageArtifactSync's exact string values.
// ADR-0030 forbids this package importing internal/gitutil at all, so the
// comparison targets are duplicated here rather than imported; a
// same-repo test pins the two string sets equal (see
// materialize_stage_constants_test.go).
const (
	stageMaterialization = "materialization"
	stageArtifactSync    = "artifact-sync"
)

// nonArtifactPaths filters the shared tool-artifact set from an S-stage delta.
func nonArtifactPaths(paths []string, specDir ...string) []string {
	dir := ""
	if len(specDir) > 0 {
		dir = specDir[0]
	}
	var out []string
	for _, p := range paths {
		if !panel.IsToolArtifactPath(p, dir) {
			out = append(out, p)
		}
	}
	return out
}

// materializationSubject renders the M-stage commit's subject from the
// operator's own --commit-msg text (fix-round G6): "" when commitMsg is
// empty (the executor then falls back to its own fixed, stage-named
// default), else the exact `impl(<bead>): <msg>` shape the pre-bead-6
// porcelain CommitAll subject used — beadID is a FUNCTIONAL position
// here (a commit-message prefix, not a terminal-facing display), so it
// stays RAW per this file's own idrender convention (idrender.Bead is
// for display positions only); beadID already passed idvalidate.BeadID
// at Run's ingress (AC-6) before ever reaching this function.
func materializationSubject(beadID, commitMsg string) string {
	if commitMsg == "" {
		return ""
	}
	return fmt.Sprintf("impl(%s): %s", beadID, commitMsg)
}

// stageMarkerAttestationKey is the bd-metadata key pairing a bead with
// the exact marker OID THIS BINARY last published for it (final-review
// round 1, R6 F1/G1 — the marker-provenance attestation). The write-ahead
// marker ref alone proves nothing about WHO published it: a hand-authored
// commit ending in a clean `Mindspec-Stage` trailer plus an ordinary
// `git update-ref refs/mindspec/materialized/<bead> <tip>` reproduced
// every structural property the discriminator checked, and the S-resume
// route then skipped the panel AND content gates for ungated content
// (R6's executed repro). The attestation is the binary-side publication
// evidence — written through the bd metadata seam immediately BEFORE the
// ref publish, mirroring the spec-127 landed-merge evidence pattern
// (`mindspec_landed_merge_sha`) — and the discriminator honors a resume
// ONLY when the attestation names the observed marker OID exactly. The
// key persists after completion as an audit record (like the landed
// binding); each publication overwrites it.
//
// HONEST TRUST BOUNDARY (final-review round 2, R12): this NARROWS the
// forgery, it does not close it. bd metadata is writable with ordinary
// bd CLI access — which every implementer agent holds by design — so a
// forger of the ref+trailer can also write the pairing attestation with
// one more ordinary bd call. What it buys: git-only forgery (the
// round-1 executed repro) is dead, a successful forgery now leaves a
// durable attributable write in the tracker's audit trail, and the
// resume optimization's premise is stated truthfully — it TRUSTS the
// local operator environment's bd store, it is not an
// adversarial-integrity control against a bd-capable actor. The
// signature-verified escalation (attested AND marker-commit signature
// verified against an operator-trusted key, when signing is enabled) is
// follow-up bead mindspec-an8q. See ADR-0041's provenance amendment for
// the same statement at the decision level.
const stageMarkerAttestationKey = "mindspec_stage_marker_oid"

const stageMarkerReviewSHAKey = "mindspec_stage_marker_reviewed_sha"
const stageMarkerReviewPanelKey = "mindspec_stage_marker_review_panel"

type stageMarkerReview struct {
	sha, panel string
}

// readStageMarkerReview is used only after the discriminator authenticates a resume.
// Older attestations and publications with no Allow supply no review identity.
func readStageMarkerReview(beadID string) (*stageMarkerReview, error) {
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return nil, fmt.Errorf("reading the materialization review identity for %s: %s", idrender.Bead(beadID), next.RenderBDError(err))
	}
	sha, _ := meta[stageMarkerReviewSHAKey].(string)
	panel, _ := meta[stageMarkerReviewPanelKey].(string)
	if sha == "" || panel == "" {
		return nil, nil
	}
	return &stageMarkerReview{sha: sha, panel: panel}, nil
}

// readStageMarkerAttestation reads beadID's recorded attestation OID, ""
// when absent or non-string. A READ FAILURE propagates (fix-round G3's
// discipline): it is never mapped to "unattested" — that would clear
// genuine write-ahead evidence on a bd blip — and never to "attested".
func readStageMarkerAttestation(beadID string) (string, error) {
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return "", err
	}
	oid, _ := meta[stageMarkerAttestationKey].(string)
	return strings.TrimSpace(oid), nil
}

// recordStageMarkerAttestation writes the attestation for oid — called
// immediately BEFORE each marker-ref publish. The write-before-publish
// ordering creates TWO crash windows (final-review round 2, R7 N1 —
// enumerated honestly rather than generalized away):
//
//  1. Attestation written, ref publish never landed, NO marker present
//     (the M-stage's absent→C window): harmless — the discriminator
//     consults the attestation only when a marker ref exists, and the
//     next publication overwrites the orphaned value.
//  2. Attestation written for the NEXT stage while the ref still names
//     the PREVIOUS one (the C→D window: attestation=D recorded, the
//     ref-publish CAS from C never landed): on re-run the discriminator
//     reads oid=C, compares attested(D) != C, and classifies the
//     LEGITIMATE C marker FOREIGN — clearing it and falling back to a
//     normal, fully re-gated run (which, with a registered panel and a
//     tip already past reviewed_head_sha, can also mean a stale-SHA
//     panel refusal before convergence).
//
// So the honest guarantee is one-directional: a crash in the
// attestation-to-publish window can COST the resume (degrading to the
// fully-gated path, never silently), but no crash — and no forgery —
// ever ADMITS an unattested resume. On failure, refuses BEFORE anything
// is published (aerr is bd-derived — escaped per G3's render
// discipline).
func recordStageMarkerAttestation(beadID, oid string, review *stageMarkerReview) error {
	// Always overwrite both identity fields alongside the OID: a new publication
	// without an Allow must not inherit an identity from an older marker.
	updates := map[string]interface{}{
		stageMarkerAttestationKey: oid,
		stageMarkerReviewSHAKey:   "",
		stageMarkerReviewPanelKey: "",
	}
	if review != nil {
		updates[stageMarkerReviewSHAKey] = review.sha
		updates[stageMarkerReviewPanelKey] = review.panel
	}
	if aerr := completeMergeMetadataFn(beadID, updates); aerr != nil {
		return guard.NewFailure(
			fmt.Sprintf("bead %s's materialization failed while recording the write-ahead marker attestation: %v — refusing before any marker publish; nothing was published.", idrender.Bead(beadID), next.RenderBDError(aerr)),
			fmt.Sprintf("mindspec complete %s   (re-run once bd metadata writes are healthy)", idrender.Bead(beadID)),
		)
	}
	return nil
}

// isKnownStage reports whether stage is one of the two CLOSED,
// recognized Mindspec-Stage trailer values (round-1/round-5/round-6
// panel G2): the discriminator must never resume on anything else — a
// missing, empty, or unrecognized value is corrupt/foreign evidence,
// never a license to skip the panel or content gates.
func isKnownStage(stage string) bool {
	return stage == stageMaterialization || stage == stageArtifactSync
}

// resolveMaterializationDiscriminator is bead 6's re-entry discriminator
// (plan round-5 G3.2, made total at round-6 G1, closed over the known
// stage set at round-1 fix-round G2, provenance-authenticated at
// final-review round 1 G1): reads beadID's write-ahead marker, if any,
// and classifies this invocation as either a normal run (resumeStage ==
// "") or a resume of a specific completed stage. A resume value is
// ALWAYS one of the two stage constants above — never inferred from a
// tree diff, always read from the marker commit's own Mindspec-Stage
// trailer via exec.ReadStageMarker, and never anything outside
// {materialization, artifact-sync} (isKnownStage) — an unknown,
// ambiguous, or duplicate-encoded trailer is corrupt/foreign evidence,
// cleared and treated exactly like an absent one, never resumed on.
//
// PROVENANCE (final-review round 1, R6 F1; trust boundary stated
// honestly in round 2, R12): before EITHER success leg honors a resume,
// the marker OID must be corroborated by the binary's own bd-metadata
// attestation (stageMarkerAttestationKey — see its doc comment for the
// executed forgery this NARROWS, and for the trust boundary: git-only
// forgery is dead, but a bd-capable actor can still pair the
// attestation; the resume trusts the local operator environment's bd
// store — escalation is bead mindspec-an8q). The trailer and the ref
// are both authorable with ordinary repo git access; the attestation is
// the bd-store corroboration of publication. A trailer-bearing marker
// WITHOUT a matching attestation is FOREIGN: cleared, then a normal run
// — the panel gate and every content gate re-evaluate in full; nothing
// resumes on evidence the local bd store does not corroborate.
//
// Every git read failure here PROPAGATES (fix-round G3): a transient
// probe failure is never silently downgraded to a default classification
// (clear-and-proceed, or CREATE semantics) — the five-bead hazard class
// this codebase's own hazard fence exists to close.
//
// beadBranch is the ref the discriminator resolves the CURRENT tip
// against (workspace.BeadBranch(beadID) or the worktree-listed
// equivalent — the same value Run calls beadHead).
func resolveMaterializationDiscriminator(exec executor.Executor, root, beadID, beadBranch string) (resumeStage string, err error) {
	oid, stage, present, err := exec.ReadStageMarker(beadID)
	if err != nil {
		return "", fmt.Errorf("reading materialization marker for %s: %w", idrender.Bead(beadID), err)
	}
	if !present {
		return "", nil
	}

	tip, tipErr := exec.RevParseRef(root, beadBranch)
	if tipErr != nil {
		// PROPAGATE (fix-round G3): a transient read failure here must
		// never be downgraded to "treat as foreign and delete the
		// marker" — that would destroy write-ahead recovery evidence on
		// exactly the kind of blip a retry would otherwise absorb.
		return "", fmt.Errorf("resolving %s's current tip while reading its materialization marker: %w", idrender.Bead(beadID), tipErr)
	}

	if stage == "" || !isKnownStage(stage) {
		// A foreign or corrupt marker object (present, but carrying no
		// Mindspec-Stage trailer, an unrecognized value, or an ambiguous
		// duplicate-trailer encoding — gitutil.ReadStageTrailer reports
		// duplicates as absent): identical treatment to "no marker" —
		// clear it (CAS against the exact OID just observed) and proceed
		// as a normal run. The panel and content gates then re-evaluate
		// fully; nothing resumes on unverified evidence.
		if clrErr := exec.ClearStageMarker(beadID, oid); clrErr != nil {
			return "", fmt.Errorf("clearing foreign materialization marker for %s: %w", idrender.Bead(beadID), clrErr)
		}
		return "", nil
	}

	// PROVENANCE gate (final-review round 1 G1): both success legs below
	// require the attestation to name the observed marker OID exactly
	// before anything resumes or advances a branch ref toward the marker.
	// Evaluated ONLY on a genuine resume candidate — the structural
	// foreign/corrupt/error legs need no bd read. attestedOrClear returns
	// foreign=true after clearing an unattested marker: marker-shaped but
	// not marker-published (R6 F1's forged-ref route, or a
	// pre-attestation-era leftover) — the caller then proceeds as a
	// normal run and the panel + content gates re-evaluate in full.
	attestedOrClear := func() (foreign bool, err error) {
		attested, aerr := readStageMarkerAttestation(beadID)
		if aerr != nil {
			return false, fmt.Errorf("reading the stage-marker attestation for %s: %w", idrender.Bead(beadID), aerr)
		}
		if attested == oid {
			return false, nil
		}
		if clrErr := exec.ClearStageMarker(beadID, oid); clrErr != nil {
			return false, fmt.Errorf("clearing unattested materialization marker for %s: %w", idrender.Bead(beadID), clrErr)
		}
		return true, nil
	}

	if tip == oid {
		// The published stage COMPLETED — resume only with provenance.
		foreign, verr := attestedOrClear()
		if verr != nil {
			return "", verr
		}
		if foreign {
			return "", nil
		}
		return stage, nil
	}

	// tip == marker^ (the marker's own first parent): the write-ahead
	// advance is UNFINISHED — finish it (expected-old = tip) and resume
	// at the now-complete stage.
	parent, perr := exec.RevParseRef(root, oid+"^")
	if perr != nil {
		// PROPAGATE (fix-round G3): resolving the marker's own first
		// parent should always succeed (oid is a real commit this
		// invocation or a predecessor built) — a failure here is treated
		// as a genuine read error, never silently reclassified as
		// FOREIGN (which would delete the marker on a transient blip).
		return "", fmt.Errorf("resolving %s's write-ahead marker parent %s^: %w", idrender.Bead(beadID), oid, perr)
	}
	if tip == parent {
		// PROVENANCE gate BEFORE the branch advance (final-review round 1
		// G1): a forged marker must never pull the bead branch's real ref
		// forward onto it.
		foreign, verr := attestedOrClear()
		if verr != nil {
			return "", verr
		}
		if foreign {
			return "", nil
		}
		if advErr := exec.AdvanceBranchExpectedOld(beadBranch, oid, tip); advErr != nil {
			return "", guard.NewFailure(
				fmt.Sprintf("bead %s's write-ahead materialization marker names %s as the intended tip, but finishing the branch advance from %s failed: %v", idrender.Bead(beadID), oid, tip, advErr),
				fmt.Sprintf("mindspec complete %s", idrender.Bead(beadID)),
			)
		}
		return stage, nil
	}

	// FOREIGN: tip is neither the marker nor its parent — a genuinely
	// foreign commit landed on the bead branch after materialization.
	// Delete the marker (CAS against the exact OID just observed); the
	// panel gate's stale-SHA check below evaluates exactly as it would
	// with no marker at all — the cannot-launder boundary is retained.
	if clrErr := exec.ClearStageMarker(beadID, oid); clrErr != nil {
		return "", fmt.Errorf("clearing stale materialization marker for %s: %w", idrender.Bead(beadID), clrErr)
	}
	return "", nil
}

// materializationInput bundles runMaterialization's dependencies (the
// function has a lot of them — a struct keeps the call site and the
// signature both readable).
type materializationInput struct {
	exec                                                     executor.Executor
	root, beadID, beadHead, wtPath, specDir, commitMsg, base string
	opts                                                     CompleteOpts
	resumeStage                                              string
	landedReview                                             *stageMarkerReview
	warnWriter                                               io.Writer
}

// runMaterialization is bead 6's normal-path (non-reconcile) staged flow.
// On resumeStage == "" it takes ONE snapshot, runs the per-bead content
// gates against it, and — on a full pass — runs M (build C) -> R (reset)
// -> S (build D) -> the S-exit contract. On a non-empty resumeStage it
// skips the re-snapshot and the content gates, resuming directly at R+S
// (resumeStage == stageMaterialization) or the S-exit contract alone
// (resumeStage == stageArtifactSync). Run's own resumeStage guard means
// the panel gate is likewise never re-run on a resume.
//
// The ADR-divergence gate's override/supersede handling is delegated to
// handleADRGateOutcome (complete.go) — shared with the reconcile path's
// own committed-range gate call, so the --supersede-adr placeholder
// pre-create and the pass/fail decision are byte-identical in both modes.
func runMaterialization(in materializationInput) error {
	exec := in.exec
	beadID, beadHead, wtPath := in.beadID, in.beadHead, in.wtPath

	// The two porcelain paths this replaces (the --commit-msg CommitAll
	// and the pathspec-scoped artifact-sync commit) both refused rather
	// than ever falling back to committing on the root/main checkout
	// (ADR-0041 §1) when no bead worktree resolved. The new machinery
	// needs a real worktree for EVERY one of its git operations, so the
	// same refusal applies whenever there is anything at all to
	// materialize. On a RESUME (resumeStage != ""), this early
	// classification is SKIPPED entirely: R (the index-only mixed reset)
	// has not run yet at this point, so the real index still reflects the
	// PRE-materialization tip — the M-stage's own work (already safely
	// committed into C) would misclassify as "still dirty" against that
	// stale index, a false positive this check must never race against.
	// The resume path's own S-exit contract (after R actually runs) is
	// the authoritative dirt check for a resume.
	if wtPath == "" {
		if in.resumeStage == "" {
			artifactDirt, userDirt, dirtErr := checkDirtyTreeFn(in.root, in.root)
			if dirtErr != nil {
				return fmt.Errorf("checking working tree: %w", dirtErr)
			}
			if len(userDirt) > 0 {
				return userDirtRefusal(beadID, in.root, userDirt, wtPath)
			}
			if in.commitMsg != "" || len(artifactDirt) > 0 {
				return guard.NewFailure(
					fmt.Sprintf("bead %s's artifact materialization would commit on the main checkout (%s) — no bead worktree matched; refusing.", idrender.Bead(beadID), in.root),
					fmt.Sprintf("mindspec next %s   (recreates the missing bead worktree; then re-run `mindspec complete %s`)", idrender.Bead(beadID), idrender.Bead(beadID)),
				)
			}
			// Nothing needs materializing and no user dirt exists — proceed
			// with no snapshot at all (mirrors the pre-bead-6 no-op).
			return nil
		}
		// A resume with no worktree at all is an unexpected, unsafe state
		// (a marker implies a prior worktree existed) — refuse naming it,
		// rather than guessing.
		return guard.NewFailure(
			fmt.Sprintf("bead %s has a write-ahead materialization marker but no bead worktree matched — cannot safely resume.", idrender.Bead(beadID)),
			fmt.Sprintf("mindspec next %s   (recreates the missing bead worktree; then re-run `mindspec complete %s`)", idrender.Bead(beadID), idrender.Bead(beadID)),
		)
	}
	if in.resumeStage == "" {
		_, userDirt, dirtErr := checkDirtyTreeFn(wtPath, wtPath)
		if dirtErr != nil {
			return fmt.Errorf("checking working tree: %w", dirtErr)
		}
		if len(userDirt) > 0 && in.commitMsg == "" {
			return userDirtRefusal(beadID, in.root, userDirt, wtPath)
		}
	}

	// A staged materialization resume can also resume a preserved merge.
	// Recheck ownership at the resolved spec index before any stage resumes.
	if in.opts.ResolveMerge && in.resumeStage != "" {
		if err := runCommittedRangeGate(in.root, beadID, beadHead, in.specDir, in.base, exec, in.opts, in.warnWriter); err != nil {
			return err
		}
	}

	currentTip := ""
	if in.resumeStage == "" {
		// --- Normal run: snapshot once, gate, then materialize. -------
		tree, err := exec.SnapshotWorktreeTree(wtPath, panel.ToolArtifactPaths(workspace.SpecsDir(wtPath)))
		if err != nil {
			return fmt.Errorf("snapshotting bead worktree %s: %w", wtPath, err)
		}

		ownerRef, err := ownershipLandingTree(in.root, in.specDir, tree, beadHead, exec)
		if err != nil {
			return &landingOwnershipError{beadID: beadID, cause: err}
		}
		docResult := validate.ValidateDocsSnapshot(in.root, in.base, tree, ownerRef, wtPath, exec)
		printResultWarnings(in.warnWriter, docResult)
		if docResult.HasFailures() && in.opts.AllowDocSkew == "" {
			return fmt.Errorf("doc-sync: %s\nhint: re-run with --allow-doc-skew \"<reason>\" to override (records the reason in bead metadata)", joinResultErrorMessages(docResult))
		}

		adrResult, adrFindings := validate.CheckADRDivergenceSnapshot(in.root, in.base, exec, in.specDir, beadID, tree, ownerRef, wtPath)
		printResultWarnings(in.warnWriter, adrResult)
		beadScopeWarnAdvisory(exec, in.root, in.specDir, beadID, beadHead, in.base, beadHead, in.warnWriter)

		if err := handleADRGateOutcome(in.root, beadID, in.opts, adrResult, adrFindings); err != nil {
			return err
		}

		tip, tipErr := exec.RevParseRef(in.root, beadHead)
		if tipErr != nil {
			return fmt.Errorf("resolving %s's current tip: %w", idrender.Bead(beadID), tipErr)
		}
		currentTip = tip

		changedFromTip, cErr := exec.ChangedFilesTree(wtPath, tip, tree)
		if cErr != nil {
			return fmt.Errorf("measuring the snapshot's delta from %s's tip: %w", idrender.Bead(beadID), cErr)
		}
		if len(changedFromTip) > 0 {
			c, buildErr := exec.MaterializeStageCommit(wtPath, tree, tip, stageMaterialization, materializationSubject(beadID, in.commitMsg))
			if buildErr != nil {
				return fmt.Errorf("building the materialization commit for %s: %w", idrender.Bead(beadID), buildErr)
			}
			if aerr := recordStageMarkerAttestation(beadID, c, in.landedReview); aerr != nil {
				return aerr
			}
			if pubErr := exec.PublishStageMarker(beadID, c, ""); pubErr != nil {
				return materializationRaceRefusal(beadID, "publishing", pubErr)
			}
			if advErr := exec.AdvanceBranchExpectedOld(beadHead, c, tip); advErr != nil {
				return materializationRaceRefusal(beadID, "advancing the branch tip", advErr)
			}
			if rstErr := exec.ResetIndexToHead(wtPath); rstErr != nil {
				return materializationResetRefusal(beadID, rstErr)
			}
			currentTip = c
		}
	} else if in.resumeStage == stageMaterialization {
		// Resume at R then S: the marker/tip already show C complete.
		if err := exec.ResetIndexToHead(wtPath); err != nil {
			return materializationResetRefusal(beadID, err)
		}
		tip, tipErr := exec.RevParseRef(in.root, beadHead)
		if tipErr != nil {
			return fmt.Errorf("resolving %s's current tip: %w", idrender.Bead(beadID), tipErr)
		}
		currentTip = tip
	} else {
		// resumeStage == stageArtifactSync: the marker/tip already show D
		// complete — resume directly at the S-exit contract, below.
		if err := exec.ResetIndexToHead(wtPath); err != nil {
			return materializationResetRefusal(beadID, err)
		}
		return sExitContract(in, wtPath, beadID)
	}

	// --- Pre-S dirt classification (plan round-5 G2, moved BEFORE the
	// S-stage's own snapshot): classify remaining worktree dirt with the
	// SAME artifact/user classifier the pre-materialization safety check
	// above uses. GENUINE non-artifact dirt at this point is an INTERVAL
	// EDIT — one that arrived during the run itself (a race) or during a
	// resumed run's kill window — and REFUSES with a named-paths recovery
	// HERE, before the S-stage's own `add -A` snapshot could otherwise
	// silently sweep it into D alongside the artifact refresh. Only a
	// clean-except-artifact worktree proceeds to the snapshot below.
	_, preUserDirt, dErr := checkDirtyTreeFn(wtPath, wtPath)
	if dErr != nil {
		return fmt.Errorf("checking working tree (S-exit contract): %w", dErr)
	}
	if len(preUserDirt) > 0 {
		return sExitGenuineDirtRefusal(beadID, preUserDirt)
	}

	// --- S: build D (or elide) from the artifact's CURRENT projection. -
	// The dirty-tree classification above already ran `bd export` (via
	// checkDirtyTreeFn -> next.CheckDirtyTreeDetail's own normalization
	// leg) whenever the artifact appeared dirty, so the worktree's
	// .beads/issues.jsonl is already the current projection by the time
	// this snapshot runs — no second export is needed here. Nothing else
	// in the worktree is dirty (preUserDirt was empty, just checked), so
	// this add -A snapshot captures the artifact's refresh alone.
	tree2, err := exec.SnapshotWorktreeTree(wtPath, nil)
	if err != nil {
		return fmt.Errorf("snapshotting bead worktree %s (artifact-sync leg): %w", wtPath, err)
	}
	changedFromCurrent, cErr := exec.ChangedFilesTree(wtPath, currentTip, tree2)
	if cErr != nil {
		return fmt.Errorf("measuring the artifact-sync delta for %s: %w", idrender.Bead(beadID), cErr)
	}
	if len(changedFromCurrent) > 0 {
		// Fix-round G1: classify the EXACT tree about to be committed.
		// preUserDirt was empty at the pre-S check above, so BY
		// CONSTRUCTION the only legitimate delta between currentTip and
		// tree2 is the named artifact's own refresh — any OTHER path
		// here means a mutation raced the window between that check and
		// this snapshot's own `add -A` (an external writer: an editor
		// flush, a concurrent agent — no privileged seam required).
		// Refuse naming it BEFORE MaterializeStageCommit/publication:
		// ChangedFilesTree lists both rename endpoints with exact filename
		// bytes. This check inspects the tree that will form the commit.
		if nonArtifact := nonArtifactPaths(changedFromCurrent, workspace.SpecsDir(wtPath)); len(nonArtifact) > 0 {
			return sExitGenuineDirtRefusal(beadID, nonArtifact)
		}
		oid, _, present, merr := exec.ReadStageMarker(beadID)
		if merr != nil {
			// PROPAGATE (fix-round G3): a marker-read failure here must
			// never silently degrade to CREATE semantics (expectedOld ==
			// "") — if a marker genuinely exists (from the M-stage's own
			// publish), a spurious CREATE attempt would race against it
			// instead of correctly CAS-advancing from it.
			return fmt.Errorf("reading materialization marker for %s before the artifact-sync publish: %w", idrender.Bead(beadID), merr)
		}
		expectedOld := ""
		if present {
			expectedOld = oid
		}
		d, buildErr := exec.MaterializeStageCommit(wtPath, tree2, currentTip, stageArtifactSync, "")
		if buildErr != nil {
			return fmt.Errorf("building the artifact-sync commit for %s: %w", idrender.Bead(beadID), buildErr)
		}
		if aerr := recordStageMarkerAttestation(beadID, d, in.landedReview); aerr != nil {
			return aerr
		}
		if pubErr := exec.PublishStageMarker(beadID, d, expectedOld); pubErr != nil {
			return materializationRaceRefusal(beadID, "publishing", pubErr)
		}
		if advErr := exec.AdvanceBranchExpectedOld(beadHead, d, currentTip); advErr != nil {
			return materializationRaceRefusal(beadID, "advancing the branch tip", advErr)
		}
		if rstErr := exec.ResetIndexToHead(wtPath); rstErr != nil {
			return materializationResetRefusal(beadID, rstErr)
		}
	}

	return sExitContract(in, wtPath, beadID)
}

// sExitContract is the S-stage exit contract (plan round-5 G2): a final
// re-classification of worktree dirt with the SAME artifact/user
// classifier the pre-S check above uses (plumbing never mutates the
// worktree between that check and here, so this is a defensive re-
// confirmation, not expected to find anything NEW). Residual artifact
// dirt (e.g. a hook re-exporting mid-run) is a non-blocking WARN,
// mirroring the pre-bead-6 residual-dirt warning; genuine non-artifact
// dirt still REFUSES here too, in case anything changed since the pre-S
// check (belt-and-suspenders, not the primary enforcement point — see
// runMaterialization's own pre-S check for that).
//
// Final-review round 1 (R7 F6/G6): the fresh classification below is the
// SOLE authority for the survived-materialization claim. The removed
// `knownResidualArtifactDirt` fallback fired only when this fresh read
// found artifactDirt EMPTY — i.e. exactly when the earlier-known dirt
// did NOT survive (the S-stage committed it into D, or the classifier's
// own `bd export` normalized it away) — so its message asserted the
// opposite of what the gating measurement had just found. The WARN's
// subject is the house kebab-case grammar (R5/R11: `bead-scope`,
// `missing-source-globs`, ... — never a dotted/snake_case one-off), and
// its paths are termsafe-escaped like every other dirt render.
func sExitContract(in materializationInput, wtPath, beadID string) error {
	artifactDirt, userDirt, err := checkDirtyTreeFn(wtPath, wtPath)
	if err != nil {
		return fmt.Errorf("checking working tree (S-exit contract): %w", err)
	}
	if len(userDirt) > 0 {
		return sExitGenuineDirtRefusal(beadID, userDirt)
	}
	if len(artifactDirt) > 0 {
		fmt.Fprintf(in.warnWriter, "WARN residual-artifact-dirt: %v (survived materialization; non-blocking)\n", termsafeEscapeEach(artifactDirt))
	}
	return nil
}

// sExitGenuineDirtRefusal renders the S-exit contract's named-paths
// refusal for a GENUINE non-artifact interval edit surviving
// materialization: preserved on disk, no rollback, no re-panel, before
// any merge path that could force-remove the bead worktree. The paths
// are attacker-choosable filenames — termsafe-escaped per entry
// (final-review round 1 G3's render discipline, matching
// userDirtRefusal above).
func sExitGenuineDirtRefusal(beadID string, userDirt []string) error {
	return guard.NewFailure(
		fmt.Sprintf("bead %s has genuine uncommitted changes surviving materialization: %v — refusing before any merge (the edit is preserved on disk; no rollback, no re-panel).", idrender.Bead(beadID), termsafeEscapeEach(userDirt)),
		fmt.Sprintf("mindspec complete %s   (commit or revert the named path(s), then re-run)", idrender.Bead(beadID)),
	)
}

// dirtCheckPath returns wtPath when set, else root — the pre-existing
// "no worktree matched, check main" fallback the dirty-tree classifier
// has always used.
func dirtCheckPath(wtPath, root string) string {
	if wtPath == "" {
		return root
	}
	return wtPath
}

// userDirtRefusal renders the pre-existing "workspace has uncommitted
// user changes" refusal, byte-compatible with the pre-bead-6 message
// shape (complete_test.go pins substrings of this text).
func userDirtRefusal(beadID, root string, userDirt []string, wtPath string) error {
	msg := fmt.Sprintf("workspace has uncommitted user changes:\n  %s\n(.beads/issues.jsonl is auto-handled and never blocks)",
		strings.Join(termsafeEscapeEach(userDirt), "\n  "))
	checkPath := dirtCheckPath(wtPath, root)
	if wtPath == "" {
		msg += "\nno active bead worktree is set — claim work with `mindspec next`, commit in the printed worktree, then rerun `mindspec complete`"
	}
	cwd, cwdErr := completeGetwdFn()
	if cwdErr != nil || cwd == "" {
		cwd = checkPath
	}
	msg += "\n" + workspace.ContextLine(cwd, checkPath)
	if wtPath == "" {
		return guard.NewFailure(msg, "mindspec next")
	}
	return guard.NewFailure(msg, fmt.Sprintf("mindspec complete %s \"describe what you did\"", idrender.Bead(beadID)))
}

// materializationRaceRefusal renders the CAS-failure refusal (plan
// step-3 failure legs): a failed marker publication or branch advance
// leaves the already-published marker (if any) in place as write-ahead
// evidence for the next run's discriminator — a plain re-run re-measures
// and converges.
func materializationRaceRefusal(beadID, verb string, cause error) error {
	return guard.NewFailure(
		fmt.Sprintf("bead %s's materialization failed while %s (a concurrent invocation, or stale state): %v — refusing; no evidence was overwritten.", idrender.Bead(beadID), verb, cause),
		fmt.Sprintf("mindspec complete %s   (re-run once the concurrent invocation, if any, has finished)", idrender.Bead(beadID)),
	)
}

// materializationResetRefusal renders the index-reset failure refusal:
// forward-only (never rolled back), converging via the write-ahead
// resume machinery on the next invocation.
func materializationResetRefusal(beadID string, cause error) error {
	return guard.NewFailure(
		fmt.Sprintf("bead %s's post-materialization index refresh failed: %v — the branch tip already advanced; refusing rather than leaving a stale index.", idrender.Bead(beadID), cause),
		fmt.Sprintf("mindspec complete %s   (re-run — the write-ahead marker converges the retry)", idrender.Bead(beadID)),
	)
}
