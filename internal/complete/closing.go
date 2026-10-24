package complete

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/mergefollowup"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// closingStepHook is a fault seam. It runs before each durable closing step.
var closingStepHook func(step int) error

func runMergedCompletion(root, beadID, specID, epicID, beadHead, wtPath string, row executor.MergeIdentity, exec executor.Executor, opts CompleteOpts, pending bool) (*Result, error) {
	specBranch, _ := workspace.SpecBranch(specID)
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return nil, completionFailure(beadID, opts, "resolving spec directory", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading configuration", err)
	}
	specWT, err := workspace.SpecWorktreePath(root, cfg, specID)
	if err != nil {
		return nil, completionFailure(beadID, opts, "resolving spec worktree", err)
	}
	info, err := fetchBeadByIDFn(beadID)
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading bead status", err)
	}
	closed := strings.EqualFold(strings.TrimSpace(info.Status), "closed")
	mergeOnly := opts.MergeOnly || pending
	opts.MergeOnly = mergeOnly
	tip, err := exec.RevParseRef(root, beadHead)
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading bead tip", err)
	}
	state, err := exec.CompletionState(specWT, "", "")
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading spec worktree state", err)
	}
	if state.MergeHead != "" {
		commands := []string{"git -C " + ledgerShellArg(specWT) + " status"}
		if closed && state.MergeHead == tip {
			commands = []string{"git -C " + ledgerShellArg(specWT) + " merge --abort"}
			commands = append(commands, mergefollowup.Commands(beadID, epicID)...)
		} else {
			commands = append(commands, completionCommand(beadID, opts))
		}
		return nil, guard.NewFailure("spec worktree is busy: a merge is in progress at "+termsafe.Escape(state.MergeHead)+". Resolve or complete that merge directly before you retry.", commands...)
	}
	ledgers, err := exec.PanelLedgerPaths(specWT)
	if err != nil {
		return nil, completionFailure(beadID, opts, "resolving local panel ledgers", err)
	}
	// Staged ledgers remain busy: only the tracker may be staged here.
	// This deliberately keeps the closing gate stricter than local-ledger status checks.
	for _, path := range state.StagedPaths {
		if slices.Contains(ledgers, path) || path != panel.TrackerArtifactPath {
			return nil, guard.NewFailure("spec worktree is busy: staged user change: "+termsafe.Escape(path),
				"git -C "+ledgerShellArg(specWT)+" restore --staged -- "+ledgerShellArg(path), completionCommand(beadID, opts))
		}
	}
	if tip != row.Parents[1] {
		return nil, movedTipFailure(root, beadID, epicID, beadHead, wtPath, tip, row, exec)
	}
	if err := checkMergedBeadUserChanges(root, beadID, wtPath, opts); err != nil {
		return nil, err
	}
	roots := panelGateRoots(root, wtPath, specID)
	if closed && !mergeOnly && cfg.Enforcement.PanelGate && !panelSkipEnvFn() {
		layout, err := workspace.DetectLayout(root)
		if err != nil {
			return nil, completionFailure(beadID, opts, "reading workspace layout", err)
		}
		_, statErr := os.Stat(wtPath)
		absent := wtPath == "" || os.IsNotExist(statErr)
		if layout != workspace.LayoutFlat && absent && len(panel.ForBead(panelScanFn(roots...), beadID)) == 0 {
			return nil, guard.NewFailure("the bead worktree is absent and no panel registration survives",
				"run a new panel round for the bead at its tip", completionCommand(beadID, opts))
		}
	}
	out := panelAdvisoryOut
	if out == nil {
		out = os.Stderr
	}
	decisions, err := panelGate(beadID, roots, wtPath, cfg.Enforcement.PanelGate, out)
	if err != nil {
		return nil, err
	}
	if err := checkAndRecordPanelDeferrals(beadID, decisions, cfg.Enforcement.PanelGate); err != nil {
		return nil, err
	}
	if err := checkPanelCore3(decisions, cfg.Enforcement.PanelGate, out); err != nil {
		return nil, err
	}
	if err := runRangeGateWithOwner(root, beadID, row.SHA, specDir, row.Parents[0], row.SHA, exec, opts, warnWriter); err != nil {
		return nil, mergedRangeFailure(root, beadID, specBranch, specWT, row, exec, opts, err)
	}
	landed := &lifecycle.LandedMerge{SHA: row.SHA, FirstParent: row.Parents[0], SecondParent: row.Parents[1]}
	// A closed resume checks identity and gates again, but skips presence.
	if !closed || mergeOnly {
		presence, err := lifecycle.CheckMergePresence(root, specBranch, specDir, landed)
		if err != nil {
			return nil, completionFailure(beadID, opts, "checking merged changes", err)
		}
		if len(presence.RevertedPaths) > 0 {
			text, err := lifecycle.RestoreMergeRecovery(specWT, beadID, epicID, landed, presence, lifecycle.MergeRestoreOutcome(specWT, specDir))
			if err != nil {
				return nil, completionFailure(beadID, opts, "building restore recovery", err)
			}
			return nil, fmt.Errorf("%s", text)
		}
	}
	if mergeOnly {
		if pending && !opts.ResolveMerge {
			fmt.Printf("A merge-only run is pending. The bead stays open.\nrecovery: mindspec complete %s\n", idrender.Bead(beadID))
		}
		return recordMergeOnly(root, beadID, specID, row, opts)
	}
	if cfg.Enforcement.PanelGate && !panelSkipEnvFn() {
		if err := replaceGatedDeferrals(beadID, decisions, nil, true); err != nil {
			return nil, PanelDeferredAuditFailure(beadID, err)
		}
	}
	if err := reconcilePendingDeferrals(beadID); err != nil {
		return nil, err
	}
	if err := reconcilePendingRefutations(beadID); err != nil {
		return nil, err
	}
	return closeMergedBead(root, beadID, specID, epicID, specBranch, specWT, wtPath, row, decisions, closed, exec, opts)
}

func closeMergedBead(root, beadID, specID, epicID, specBranch, specWT, wtPath string, row executor.MergeIdentity, decisions []PanelGateResult, closed bool, exec executor.Executor, opts CompleteOpts) (*Result, error) {
	for step := 1; step <= 6; step++ {
		if closingStepHook != nil {
			if err := closingStepHook(step); err != nil {
				return nil, completionFailure(beadID, opts, fmt.Sprintf("closing step %d failed", step), err)
			}
		}
		var err error
		switch step {
		case 1:
			if !closed {
				err = closeBeadFn(beadID)
				if err == nil {
					err = doltCommitFn()
				}
				if err == nil {
					err = verifyCommittedFn(beadID)
				}
			}
		case 2:
			updates := map[string]interface{}{"mindspec_landed_merge_sha": row.SHA, "mindspec_landed_second_parent": row.Parents[1], "mindspec_landed_at": time.Now().UTC().Format(time.RFC3339)}
			for _, d := range decisions {
				if d.Decision.Action == panel.Allow {
					p := d.Registration.Panel
					updates["mindspec_landed_reviewed_sha"] = p.ReviewedHeadSHA
					updates["mindspec_landed_review_panel"] = fmt.Sprintf("%s@r%d", d.Registration.Slug(), p.Round)
					break
				}
			}
			err = mergeMissingMetadata(beadID, updates)
		case 3:
			for _, override := range []struct{ reason, key string }{
				{opts.AllowDocSkew, "mindspec_doc_skew"}, {opts.OverrideADR, "mindspec_adr_override"}, {opts.AllowNetDeletion, "mindspec_net_deletion_override"},
			} {
				if override.reason != "" {
					err = mergeMissingMetadata(beadID, buildSkewMetadata(override.reason, override.key+"_reason", override.key+"_at", override.key+"_by"))
					if err != nil {
						break
					}
				}
			}
			if err == nil && opts.SupersedeADR != "" {
				err = mergeMissingMetadata(beadID, map[string]interface{}{"mindspec_adr_supersede_id": opts.SupersedeADR, "mindspec_adr_supersede_reason": "superseded by " + opts.SupersedeADR, "mindspec_adr_supersede_at": time.Now().UTC().Format(time.RFC3339), "mindspec_adr_supersede_by": gitUserEmailFn()})
			}
			if err == nil && len(decisions) > 0 {
				writePanelAuditMetadata(beadID, &decisions[0].Registration, os.Stderr)
			}
		case 4:
			var done bool
			done, err = closePublicationDone(beadID, specBranch, exec)
			if err == nil && !done {
				paths := []string{panel.TrackerArtifactPath}
				err = exec.PublishBeadClose(specWT, paths)
			}
		case 5:
			var oid string
			var present bool
			oid, _, present, err = exec.ReadStageMarker(beadID)
			if err == nil && present {
				err = exec.ClearStageMarker(beadID, oid)
			}
			if err == nil {
				err = deleteMergeOnlyKeys(beadID, mergeOnlyPendingKey, mergeOnlySHAKey)
			}
		case 6:
			// Close publication can run hooks that create new user work.
			if err := checkMergedBeadUserChanges(root, beadID, wtPath, opts); err != nil {
				return nil, err
			}
			// The caller may run from the worktree that is about to disappear.
			err = os.Chdir(root)
			if err == nil {
				err = exec.CleanupBead(beadID, specBranch)
			}
		}
		if err != nil {
			return nil, completionFailure(beadID, opts, fmt.Sprintf("closing step %d failed", step), err)
		}
	}
	result := &Result{BeadID: beadID, BeadClosed: true, WorktreeRemoved: true, SpecWorktree: specWT, NextSpec: specID}
	mode, nextBead, err := advanceState(epicID)
	if err != nil {
		result.NextModeIndeterminate = true
		result.IndeterminateReason = err.Error()
		return result, nil
	}
	result.NextMode, result.NextBead = mode, nextBead
	if epicID != "" && mode != "" {
		_ = completeSyncPhase(root, epicID, mode, beadID)
	}
	return result, nil
}

// checkMergedBeadUserChanges keeps the same artifact exemption before
// closing and after publication, immediately before cleanup.
func checkMergedBeadUserChanges(root, beadID, wtPath string, opts CompleteOpts) error {
	if wtPath == "" {
		return nil
	}
	if _, statErr := os.Stat(wtPath); statErr == nil {
		_, dirt, err := checkDirtyTreeFn(wtPath, wtPath)
		if err != nil {
			return completionFailure(beadID, opts, "reading bead user changes", err)
		}
		if len(dirt) > 0 {
			return userDirtRefusal(beadID, root, dirt, wtPath)
		}
	} else if !os.IsNotExist(statErr) {
		return completionFailure(beadID, opts, "reading bead worktree", statErr)
	}
	return nil
}

// mergeMissingMetadata preserves timestamps on idempotent retries.
func mergeMissingMetadata(beadID string, updates map[string]interface{}) error {
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return err
	}
	matched := true
	for key, value := range updates {
		if strings.HasSuffix(key, "_at") || strings.HasSuffix(key, "_by") {
			if meta[key] == nil {
				matched = false
			}
			continue
		}
		if meta[key] != value {
			matched = false
		}
	}
	if matched {
		return nil
	}
	return completeMergeMetadataFn(beadID, updates)
}

func movedTipFailure(root, beadID, epicID, beadHead, wtPath, tip string, row executor.MergeIdentity, exec executor.Executor) error {
	state, err := exec.CompletionState(root, row.Parents[1], beadHead)
	if err != nil {
		return completionFailure(beadID, CompleteOpts{}, "reading new bead commits", err)
	}
	commits := make([]mergefollowup.Commit, 0, len(state.NewCommits))
	for _, commit := range state.NewCommits {
		commits = append(commits, mergefollowup.Commit{SHA: commit.SHA, Subject: commit.Subject})
	}
	return mergefollowup.MovedTipFailure(beadID, epicID, wtPath, row.Parents[1], tip, commits)
}

func mergedRangeFailure(root, beadID, specBranch, specWT string, row executor.MergeIdentity, exec executor.Executor, opts CompleteOpts, cause error) error {
	// Preserve gate recovery lines, then add exits that change the measured merge.
	text := fmt.Sprintf("gates refused merge %s (first parent %s, bead parent %s):\n%s", termsafe.Escape(row.SHA), termsafe.Escape(row.Parents[0]), termsafe.Escape(row.Parents[1]), termsafe.Escape(cause.Error()))
	raw := cause.Error()
	ownership := strings.Contains(raw, "adr-divergence-unowned") || strings.Contains(raw, "ownership of ")
	docsync := strings.Contains(raw, "doc-sync:")
	commands := []string{}
	tip, err := exec.RevParseRef(root, specBranch)
	meta, metaErr := completeGetMetadataFn(beadID)
	if err == nil && metaErr == nil && tip == row.SHA && meta["mindspec_landed_merge_sha"] == nil && (ownership || docsync) {
		text += "\nStage only the named ownership manifests or doc paths in the spec worktree. Any other staged path is outside this repair."
		commands = append(commands, "git -C "+ledgerShellArg(specWT)+" commit --amend --no-edit", "bd update "+idrender.Bead(beadID)+" --unset-metadata "+mergeOnlySHAKey)
	}
	if ownership {
		text += "\nIf a domain owns the path, add that domain to the spec's Impacted Domains and commit the spec."
	}
	text += "\nA later commit does not change the merge that these gates measure."
	commands = append(commands, completionCommand(beadID, opts))
	if ownership || !docsync {
		commands = append(commands, "mindspec complete "+idrender.Bead(beadID)+" --override-adr \"<reason>\"")
	}
	if docsync {
		commands = append(commands, "mindspec complete "+idrender.Bead(beadID)+" --allow-doc-skew \"<reason>\"")
	}
	// Gate failures are framework-formatted; inner errors are escaped at their seams.
	return guard.NewFailure(text, commands...)
}

// closePublicationDone uses the committed tracker close.
// Later metadata-marker cleanup does not invalidate a published close.
func closePublicationDone(beadID, specBranch string, exec executor.Executor) (bool, error) {
	data, present, err := exec.FileAtRefOrAbsent(specBranch, panel.TrackerArtifactPath)
	if err != nil || !present {
		return false, err
	}
	closed := false
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			return false, nil
		}
		if row.ID == beadID {
			closed = strings.EqualFold(strings.TrimSpace(row.Status), "closed")
		}
	}
	if !closed {
		return false, nil
	}
	return true, nil
}

// ledgerShellArg quotes an operand for a POSIX shell, escaping display controls.
func ledgerShellArg(s string) string {
	return "'" + strings.ReplaceAll(termsafe.Escape(s), "'", "'\"'\"'") + "'"
}
