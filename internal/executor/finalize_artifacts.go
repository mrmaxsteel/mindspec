package executor

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// FinalizeArtifactPolicy limits finalization to the accepted subject and artifacts.
// The workflow supplies it only for an enrolled spec.
type FinalizeArtifactPolicy struct {
	SpecDir         string
	AcceptedSubject string
}

// CheckFinalizeArtifacts refuses pending user content without writing anything.
func CheckFinalizeArtifacts(exec Executor, worktree, specDir string) error {
	status, err := exec.Status(worktree)
	if err != nil {
		return err
	}
	paths, err := panel.FinalizeSubjectDirt(worktree, specDir, status)
	if err != nil {
		return err
	}
	return finalizeArtifactRefusal(worktree, specDir, paths)
}

// finalizeArtifactRefusal gives the same recovery for pending and committed drift.
func finalizeArtifactRefusal(worktree, specDir string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	safe := make([]string, len(paths))
	for i, name := range paths {
		safe[i] = termsafe.Escape(name)
	}
	return guard.NewFailure(fmt.Sprintf("approval would include content past review in %s:\n%s", termsafe.Escape(worktree), strings.Join(safe, "\n")),
		"commit the listed changes through the normal workflow and run a new review round, or move them out of the worktree before retrying approval",
		fmt.Sprintf("after a commit, run mindspec next-action %s --apply; after moving the changes, run mindspec impl approve %s", idrender.Spec(filepath.Base(specDir)), idrender.Spec(filepath.Base(specDir))))
}

func (g *MindspecExecutor) checkFinalizeTree(policy FinalizeArtifactPolicy, branch string) error {
	paths, err := g.ChangedFiles(policy.AcceptedSubject, branch, ChangedFilesOptions{NoRenames: true, ExactPaths: true})
	if err != nil {
		return err
	}
	var dirt []string
	for _, name := range paths {
		if !panel.FinalizeArtifactPath(name, policy.SpecDir) {
			dirt = append(dirt, name)
		}
	}
	return finalizeArtifactRefusal(g.Root, policy.SpecDir, dirt)
}
