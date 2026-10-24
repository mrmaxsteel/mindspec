package complete

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

var completeDeleteMetadataFn = bead.DeleteMetadataKeys

const mergeOnlyPendingKey = "mindspec_merge_only"
const mergeOnlySHAKey = "mindspec_merge_only_sha"

// namingMerges uses the same conservative bead token as landed discovery.
// Metadata never supplies a merge identity.
func namingMerges(exec executor.Executor, specBranch, beadID string) ([]executor.MergeIdentity, error) {
	rows, err := exec.FirstParentMerges(specBranch)
	if err != nil {
		return nil, err
	}
	branch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, err
	}
	var found []executor.MergeIdentity
	for _, row := range rows {
		if len(row.Parents) != 2 {
			continue
		}
		start := strings.Index(row.Subject, workspace.BeadBranchPrefix)
		if start < 0 {
			continue
		}
		token := strings.FieldsFunc(row.Subject[start:], func(r rune) bool {
			return r < 0x20 || strings.ContainsRune(" '\x22\x60])>,;", r)
		})
		if len(token) > 0 && token[0] == branch {
			found = append(found, row)
		}
	}
	return found, nil
}

func checkMergeMarker(beadID string, meta map[string]interface{}, rows []executor.MergeIdentity) error {
	value := meta[mergeOnlySHAKey]
	marker, valid := value.(string)
	if value == nil || (valid && marker == "") {
		return nil
	}
	if !valid {
		return guard.NewFailure("merge-only marker is not a commit SHA for "+idrender.Bead(beadID), "bd update "+idrender.Bead(beadID)+" --unset-metadata "+mergeOnlySHAKey)
	}
	for _, row := range rows {
		if row.SHA == marker {
			return nil
		}
	}
	return guard.NewFailure("merge-only marker "+termsafe.Escape(marker)+" is not a naming merge for "+idrender.Bead(beadID),
		"bd update "+idrender.Bead(beadID)+" --unset-metadata "+mergeOnlySHAKey)
}

func completionCommand(beadID string, opts CompleteOpts) string {
	cmd := "mindspec complete " + idrender.Bead(beadID)
	if opts.MergeOnly {
		cmd += " --merge-only"
	}
	if opts.ResolveMerge {
		cmd += " --resolve-merge"
	}
	if opts.AllowDocSkew != "" {
		cmd += " --allow-doc-skew " + ledgerShellArg(opts.AllowDocSkew)
	}
	if opts.OverrideADR != "" {
		cmd += " --override-adr " + ledgerShellArg(opts.OverrideADR)
	}
	if opts.SupersedeADR != "" {
		cmd += " --supersede-adr " + ledgerShellArg(opts.SupersedeADR)
	}
	return cmd
}

func completionFailure(beadID string, opts CompleteOpts, stage string, err error) error {
	return guard.NewFailure(stage+": "+termsafe.Escape(err.Error()), completionCommand(beadID, opts))
}

func ownershipCompletionFailure(beadID string, opts CompleteOpts, err error) error {
	var conflict *OwnershipLandingConflict
	if errors.As(err, &conflict) {
		return guard.NewFailure("ownership manifests have merge conflicts. The bead stays open. Merge first, then resolve and check ownership.",
			"mindspec complete "+idrender.Bead(beadID)+" --merge-only")
	}
	if opts.MergeOnly && executor.IsPreservedMergeError(err) {
		branch, branchErr := workspace.BeadBranch(beadID)
		if branchErr != nil {
			return branchErr
		}
		opts.ResolveMerge = true
		return executor.PreservedMergeReentryRefusal(err, branch, completionCommand(beadID, opts))
	}
	return err
}

func mergeOnlyResult(root, beadID, specID string, row executor.MergeIdentity) *Result {
	cfg, err := config.Load(root)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	wt, _ := workspace.SpecWorktreePath(root, cfg, specID)
	return &Result{BeadID: beadID, NextSpec: specID, SpecWorktree: wt, MergeOnly: true, MergeSHA: row.SHA, MergeParents: row.Parents, mergePrinted: true}
}

func recordMergeOnly(root, beadID, specID string, row executor.MergeIdentity, opts CompleteOpts) (*Result, error) {
	fmt.Printf("merge: %s\nparents: %s\n", termsafe.Escape(row.SHA), strings.Join(termsafeEscapeEach(row.Parents), " "))
	// Record the SHA before removing pending. A failed write leaves a durable
	// mode hint, and the next run finds this same merge from git.
	if err := completeMergeMetadataFn(beadID, map[string]interface{}{mergeOnlySHAKey: row.SHA}); err != nil {
		return nil, completionFailure(beadID, opts, "recording merge-only commit "+row.SHA, err)
	}
	if err := deleteMergeOnlyKeys(beadID, mergeOnlyPendingKey); err != nil {
		return nil, completionFailure(beadID, opts, "clearing pending merge-only mode", err)
	}
	return mergeOnlyResult(root, beadID, specID, row), nil
}

func runMergeOnly(root, beadID, specID, specBranch string, exec executor.Executor, opts CompleteOpts) (*Result, error) {
	if err := completeMergeMetadataFn(beadID, map[string]interface{}{mergeOnlyPendingKey: "pending"}); err != nil {
		return nil, completionFailure(beadID, opts, "recording pending merge-only mode", err)
	}
	reentry := completionCommand(beadID, opts)
	if !opts.ResolveMerge {
		reentry += " --resolve-merge"
	}
	if err := exec.MergeBead(beadID, specBranch, opts.AllowNetDeletion, opts.ResolveMerge, executor.MergeBeadOptions{}); err != nil {
		// The executor keeps its conflict steps and their line boundaries.
		if guard.HasFinalRecoveryLine(err.Error()) {
			return nil, err
		}
		return nil, guard.NewFailure("merge-only did not finish: "+termsafe.Escape(err.Error()), reentry)
	}
	rows, err := namingMerges(exec, specBranch, beadID)
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading merged commit", err)
	}
	if len(rows) != 1 {
		return nil, guard.NewFailure(fmt.Sprintf("expected one naming merge; found %d", len(rows)), completionCommand(beadID, opts))
	}
	return recordMergeOnly(root, beadID, specID, rows[0], opts)
}

func deleteMergeOnlyKeys(beadID string, keys ...string) error {
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return err
	}
	var present []string
	for _, key := range keys {
		if _, ok := meta[key]; ok {
			present = append(present, key)
		}
	}
	if len(present) == 0 {
		return nil
	}
	return completeDeleteMetadataFn(beadID, present...)
}

// deferOwnershipConflict applies only to the caller's open/closed path choice.
// A resolved index and every non-ownership error still pass through the gates.
func deferOwnershipConflict(beadID string, opts CompleteOpts, cause error) (bool, error) {
	var conflict *OwnershipLandingConflict
	if !errors.As(cause, &conflict) {
		return false, nil
	}
	if opts.MergeOnly {
		return true, nil
	}
	info, err := fetchBeadByIDFn(beadID)
	if err != nil {
		return false, completionFailure(beadID, opts, "reading bead status", err)
	}
	return strings.EqualFold(strings.TrimSpace(info.Status), "closed"), nil
}

// materializeConflictedOwnership finishes the same write-ahead mechanical
// protocol from the already captured bead snapshot. The ownership gate is
// deferred to the preserved merge's resolved index. The artifact stage
// still takes its own snapshot.
func materializeConflictedOwnership(in materializationInput, conflict *OwnershipLandingConflict) error {
	exec := in.exec
	beadID, beadHead, wtPath := in.beadID, in.beadHead, in.wtPath
	tree := conflict.BeadTree
	currentTip := ""
	if in.opts.MergeOnly {
		if err := completeMergeMetadataFn(beadID, map[string]interface{}{mergeOnlyPendingKey: "pending"}); err != nil {
			return completionFailure(beadID, in.opts, "recording pending merge-only mode", err)
		}
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
	_, preUserDirt, dErr := checkMaterializationDirt(exec, wtPath)
	if dErr != nil {
		return fmt.Errorf("checking working tree (S-exit contract): %w", dErr)
	}
	if len(preUserDirt) > 0 {
		return sExitGenuineDirtRefusal(beadID, preUserDirt)
	}

	ledgers, err := exec.PanelLedgerPaths(wtPath)
	if err != nil {
		return fmt.Errorf("resolving local panel ledgers: %w", err)
	}
	tree2, err := exec.SnapshotWorktreeTree(wtPath, ledgers)
	if err != nil {
		return fmt.Errorf("snapshotting bead worktree %s (artifact-sync leg): %w", wtPath, err)
	}
	changedFromCurrent, cErr := exec.ChangedFilesTree(wtPath, currentTip, tree2)
	if cErr != nil {
		return fmt.Errorf("measuring the artifact-sync delta for %s: %w", idrender.Bead(beadID), cErr)
	}
	if len(changedFromCurrent) > 0 {
		if nonArtifact := nonArtifactPaths(changedFromCurrent, workspace.SpecsDir(wtPath)); len(nonArtifact) > 0 {
			return sExitGenuineDirtRefusal(beadID, nonArtifact)
		}
		oid, _, present, merr := exec.ReadStageMarker(beadID)
		if merr != nil {
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
