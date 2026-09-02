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

// materializedArtifactPaths is the named-artifact exclusion set the
// snapshot's M-stage leaves unchanged-at-tip (ADR-0025). Mirrors
// internal/next's own artifactPaths list (unexported there); duplicated
// here rather than imported because internal/next is not a shared leaf
// this package should widen its surface through for one constant.
var materializedArtifactPaths = []string{".beads/issues.jsonl"}

// resolveMaterializationDiscriminator is bead 6's re-entry discriminator
// (plan round-5 G3.2, made total at round-6 G1): reads beadID's
// write-ahead marker, if any, and classifies this invocation as either a
// normal run (resumeStage == "") or a resume of a specific completed
// stage. A resume value is ALWAYS one of the two stage constants above —
// never inferred from a tree diff, always read from the marker commit's
// own Mindspec-Stage trailer via exec.ReadStageMarker.
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
		// The branch itself is unreadable here — reconcile detection
		// above already handles the "branch genuinely gone" case before
		// this discriminator runs, so this is an unexpected condition.
		// Treat conservatively as foreign: clear the marker and let the
		// normal-run path re-evaluate from current, re-read facts.
		if clrErr := exec.ClearStageMarker(beadID); clrErr != nil {
			return "", fmt.Errorf("clearing materialization marker for %s (tip unreadable: %v): %w", idrender.Bead(beadID), tipErr, clrErr)
		}
		return "", nil
	}

	if stage == "" {
		// A foreign marker object (present, but carrying no
		// Mindspec-Stage trailer): identical treatment to "no marker" —
		// clear it and proceed as a normal run.
		if clrErr := exec.ClearStageMarker(beadID); clrErr != nil {
			return "", fmt.Errorf("clearing foreign materialization marker for %s: %w", idrender.Bead(beadID), clrErr)
		}
		return "", nil
	}

	if tip == oid {
		// The published stage COMPLETED.
		return stage, nil
	}

	// tip == marker^ (the marker's own first parent): the write-ahead
	// advance is UNFINISHED — finish it (expected-old = tip) and resume
	// at the now-complete stage.
	if parent, perr := exec.RevParseRef(root, oid+"^"); perr == nil && tip == parent {
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
	// Delete the marker; the panel gate's stale-SHA check below evaluates
	// exactly as it would with no marker at all — the cannot-launder
	// boundary is retained.
	if clrErr := exec.ClearStageMarker(beadID); clrErr != nil {
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

	currentTip := ""
	if in.resumeStage == "" {
		// --- Normal run: snapshot once, gate, then materialize. -------
		tree, err := exec.SnapshotWorktreeTree(wtPath, materializedArtifactPaths)
		if err != nil {
			return fmt.Errorf("snapshotting bead worktree %s: %w", wtPath, err)
		}

		docResult := validate.ValidateDocsSnapshot(in.root, in.base, tree, wtPath, exec)
		printResultWarnings(in.warnWriter, docResult)
		if docResult.HasFailures() && in.opts.AllowDocSkew == "" {
			return fmt.Errorf("doc-sync: %s\nhint: re-run with --allow-doc-skew \"<reason>\" to override (records the reason in bead metadata)", joinResultErrorMessages(docResult))
		}

		adrResult, adrFindings := validate.CheckADRDivergenceSnapshot(in.root, in.base, exec, in.specDir, beadID, tree, wtPath)
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
			c, buildErr := exec.MaterializeStageCommit(wtPath, tree, tip, stageMaterialization)
			if buildErr != nil {
				return fmt.Errorf("building the materialization commit for %s: %w", idrender.Bead(beadID), buildErr)
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
		return sExitContract(in, wtPath, beadID, nil)
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
	preArtifactDirt, preUserDirt, dErr := checkDirtyTreeFn(wtPath, wtPath)
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
		expectedOld := ""
		if oid, _, present, merr := exec.ReadStageMarker(beadID); merr == nil && present && oid != "" {
			expectedOld = oid
		}
		d, buildErr := exec.MaterializeStageCommit(wtPath, tree2, currentTip, stageArtifactSync)
		if buildErr != nil {
			return fmt.Errorf("building the artifact-sync commit for %s: %w", idrender.Bead(beadID), buildErr)
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

	return sExitContract(in, wtPath, beadID, preArtifactDirt)
}

// sExitContract is the S-stage exit contract (plan round-5 G2): a final
// re-classification of worktree dirt with the SAME artifact/user
// classifier the pre-S check above uses (plumbing never mutates the
// worktree between that check and here, so this is a defensive re-
// confirmation, not expected to find anything NEW) plus the WARN for
// artifact dirt already known to have survived materialization
// (knownResidualArtifactDirt, passed by the caller when it already ran
// the classification — nil on the pure-resume route, which re-derives
// its own). Residual artifact dirt (e.g. a hook re-exporting mid-run) is
// a non-blocking WARN, mirroring the pre-bead-6 residual-dirt warning;
// genuine non-artifact dirt still REFUSES here too, in case anything
// changed since the pre-S check (belt-and-suspenders, not the primary
// enforcement point — see runMaterialization's own pre-S check for that).
func sExitContract(in materializationInput, wtPath, beadID string, knownResidualArtifactDirt []string) error {
	artifactDirt, userDirt, err := checkDirtyTreeFn(wtPath, wtPath)
	if err != nil {
		return fmt.Errorf("checking working tree (S-exit contract): %w", err)
	}
	if len(userDirt) > 0 {
		return sExitGenuineDirtRefusal(beadID, userDirt)
	}
	if len(artifactDirt) > 0 {
		fmt.Fprintf(in.warnWriter, "WARN complete.residual_artifact_dirt: %v (survived materialization; non-blocking per ADR-0025)\n", artifactDirt)
	} else if len(knownResidualArtifactDirt) > 0 {
		fmt.Fprintf(in.warnWriter, "WARN complete.residual_artifact_dirt: %v (survived materialization; non-blocking per ADR-0025)\n", knownResidualArtifactDirt)
	}
	return nil
}

// sExitGenuineDirtRefusal renders the S-exit contract's named-paths
// refusal for a GENUINE non-artifact interval edit surviving
// materialization: preserved on disk, no rollback, no re-panel, before
// any merge path that could force-remove the bead worktree.
func sExitGenuineDirtRefusal(beadID string, userDirt []string) error {
	return guard.NewFailure(
		fmt.Sprintf("bead %s has genuine uncommitted changes surviving materialization: %v — refusing before any merge (the edit is preserved on disk; no rollback, no re-panel).", idrender.Bead(beadID), userDirt),
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
	msg := fmt.Sprintf("workspace has uncommitted user changes:\n  %s\n(.beads/issues.jsonl is auto-handled per ADR-0025 and never blocks)",
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
