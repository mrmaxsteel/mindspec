package complete

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// runCheck forks before migration, stage-marker reconciliation and panel reads.
// Snapshot and landing-tree operations use temporary indexes and write no refs.
func runCheck(root, beadID, specID string, exec executor.Executor, opts CompleteOpts) (*Result, error) {
	refuse := func(state string) (*Result, error) {
		return nil, guard.NewFailure("cannot check bead "+idrender.Bead(beadID)+": "+state, "run complete --check on an open bead with a branch and no pending merge-only run")
	}
	for _, flag := range []struct {
		name string
		set  bool
	}{
		{"--merge-only", opts.MergeOnly}, {"--resolve-merge", opts.ResolveMerge}, {"--override-adr", opts.OverrideADR != ""}, {"--supersede-adr", opts.SupersedeADR != ""}, {"--allow-net-deletion", opts.AllowNetDeletion != ""},
	} {
		if flag.set {
			return refuse("--check cannot be used with " + flag.name)
		}
	}
	info, err := fetchBeadByIDFn(beadID)
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading bead status", err)
	}
	status := strings.TrimSpace(info.Status)
	if status == "" || strings.EqualFold(status, "closed") {
		return refuse("status is " + termsafe.Escape(status))
	}
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, err
	}
	tip, err := exec.RevParseRef(root, beadBranch)
	if err != nil {
		return refuse("branch is absent or unreadable: " + termsafe.Escape(err.Error()))
	}
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading merge-only metadata", err)
	}
	if meta[mergeOnlyPendingKey] == "pending" {
		return refuse("a merge-only run is pending")
	}
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return nil, err
	}
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return nil, err
	}
	rows, err := namingMerges(exec, specBranch, beadID)
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading naming merges", err)
	}
	if err := checkMergeMarker(beadID, meta, rows); err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return refuse("multiple naming merges exist")
	}
	entries, err := worktreeListFn()
	if err != nil {
		return nil, completionFailure(beadID, opts, "reading bead worktrees", err)
	}
	expected, err := workspace.BeadWorktreeName(beadID)
	if err != nil {
		return nil, err
	}
	wtPath, beadHead := resolveBeadWorktree(entries, expected, beadBranch)
	if len(rows) == 1 {
		row := rows[0]
		if len(row.Parents) != 2 || tip != row.Parents[1] {
			return refuse("the branch tip does not match its naming merge")
		}
		err = runRangeGateWithOwner(root, beadID, row.SHA, specDir, row.Parents[0], row.SHA, exec, opts, warnWriter)
	} else {
		base, baseErr := exec.MergeBase(specBranch, beadHead)
		if baseErr != nil {
			return nil, completionFailure(beadID, opts, "reading merge base", baseErr)
		}
		opts.workDestructionErr = workDestructionReport(completeWorkDestructionPreflightFn(root, beadHead, specBranch, "", fmt.Sprintf("mindspec complete %s", idrender.Bead(beadID))))
		if wtPath == "" {
			err = runCommittedRangeGate(root, beadID, beadHead, specDir, base, exec, opts, warnWriter)
		} else {
			tree, snapshotErr := exec.SnapshotWorktreeTree(wtPath, panel.ToolArtifactPaths(workspace.SpecsDir(wtPath)))
			if snapshotErr != nil {
				return nil, errors.Join(opts.workDestructionErr, fmt.Errorf("snapshotting bead worktree: %w", snapshotErr))
			}
			err = runSnapshotContentGates(materializationInput{exec: exec, root: root, beadID: beadID, beadHead: beadHead, wtPath: wtPath, specDir: specDir, base: base, opts: opts, warnWriter: warnWriter}, tree)
		}
	}
	if err != nil {
		return nil, err
	}
	return &Result{BeadID: beadID, Check: true}, nil
}

func runSnapshotContentGates(in materializationInput, tree string) error {
	ownerRef, err := ownershipLandingTree(in.root, in.specDir, tree, in.beadHead, in.exec)
	if err != nil {
		return errors.Join(in.opts.workDestructionErr, &landingOwnershipError{beadID: in.beadID, cause: err})
	}
	docs := validate.ValidateDocsSnapshot(in.root, in.base, tree, ownerRef, in.wtPath, in.exec)
	adrs, findings := validate.CheckADRDivergenceSnapshot(in.root, in.base, in.exec, in.specDir, in.beadID, tree, ownerRef, in.wtPath)
	if !in.opts.Check {
		beadScopeWarnAdvisory(in.exec, in.root, in.specDir, in.beadID, in.beadHead, in.base, in.beadHead, in.warnWriter)
	}
	return reportContentGates(in.root, in.beadID, in.opts, docs, adrs, findings, in.warnWriter)
}

// reportContentGates reports every failure before the caller can mutate state.
func reportContentGates(root, beadID string, opts CompleteOpts, docs, adrs *validate.Result, findings []validate.DivergenceFinding, out io.Writer) error {
	printResultWarnings(out, docs)
	printResultWarnings(out, adrs)
	var docErr error
	if docs.HasFailures() && opts.AllowDocSkew == "" {
		docErr = guard.NewFailure("doc-sync: "+joinResultErrorMessages(docs), "update the affected domain docs, then re-run mindspec complete "+idrender.Bead(beadID)+"; or re-run with --allow-doc-skew \"<reason>\" to record a deferral")
	}
	adrErr := handleADRGateOutcome(root, beadID, opts, adrs, findings)
	return errors.Join(docErr, adrErr, opts.workDestructionErr)
}

func workDestructionReport(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("work-destruction preflight: %w", err)
}
