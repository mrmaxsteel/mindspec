package approve

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/safefile"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// implFinalReviewGate runs on every pre-close pass while the spec branch exists.
// A published spec without its worktree can only replay the same subject.
func implFinalReviewGate(root, specID, epicID, storedPhase, specDir string, exec executor.Executor) (Outcome, bool, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return Outcome{}, false, err
	}
	wt, err := workspace.SpecWorktreePath(root, cfg, specID)
	if err != nil {
		return Outcome{}, false, err
	}
	_, statErr := os.Stat(wt)
	if statErr != nil && !os.IsNotExist(statErr) {
		return Outcome{}, false, statErr
	}
	if os.IsNotExist(statErr) && storedPhase == "done" {
		status, err := readBeadStatus(epicID)
		if err != nil {
			return Outcome{}, false, err
		}
		if status == "closed" {
			err := implPublishedSubject(root, specID, epicID, specDir, exec)
			return Outcome{Gate: "final_review"}, true, err
		}
	}
	if os.IsNotExist(statErr) {
		return Outcome{Gate: "final_review"}, false, guard.NewFailure("gate final_review: spec worktree is absent",
			fmt.Sprintf("re-create the spec worktree at spec/%s, then run a final-review panel", idrender.Spec(specID)))
	}
	o, err := runLifecycleGate(root, specID, "final_review", wt, warnWriter)
	return o, false, err
}

func implPublishedSubject(root, specID, epicID, specDir string, exec executor.Executor) error {
	refuse := func(detail string) error {
		return guard.NewFailure("gate final_review: "+termsafe.Escape(detail), fmt.Sprintf("re-create the spec worktree at spec/%s, then run a final-review panel", idrender.Spec(specID)))
	}
	meta, err := implGetMetadataFn(epicID)
	if err != nil {
		return refuse(err.Error())
	}
	tip, _ := meta["mindspec_finalize_tip"].(string)
	if len(tip) != 40 || strings.Trim(tip, "0123456789abcdef") != "" {
		return refuse("final publication tip is absent or invalid")
	}
	reader := panel.DocumentSubjectTreeReader("final_review", specDir, exec.LsTree, func(workdir, ref, name string) ([]byte, error) {
		return lifecycleExecutorFn(workdir).FileAtRef(ref, name)
	})
	var digests [2]string
	branch, err := workspace.SpecBranch(specID)
	if err != nil {
		return refuse(err.Error())
	}
	for i, ref := range []string{"refs/heads/" + branch, tip} {
		digests[i], err = reader(root, ref, panel.Subject("final_review", specDir))
		if err != nil {
			return refuse(err.Error())
		}
	}
	if digests[0] != digests[1] {
		return refuse("stale subject: content changed after publication")
	}
	return nil
}

// implBeadWorktreePreflight checks the same plan-scoped worktree population
// that finalization consumes. Unlike the review subject it excludes no dirt.
func implBeadWorktreePreflight(root, specID string, allowIDs []string, exec executor.Executor) error {
	entries, err := implWorktreeListFn()
	retry := fmt.Sprintf("mindspec impl approve %s", idrender.Spec(specID))
	if err != nil {
		return guard.NewFailure("cannot enumerate bead worktrees: "+termsafe.Escape(err.Error()), retry)
	}
	allow := map[string]bool{}
	for _, id := range allowIDs {
		allow[id] = true
	}
	for _, entry := range entries {
		id, ok := strings.CutPrefix(entry.Branch, workspace.BeadBranchPrefix)
		if !ok || idvalidate.BeadID(id) != nil || !allow[id] || entry.Path == "" {
			continue
		}
		if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return guard.NewFailure("cannot inspect bead worktree: "+termsafe.Escape(err.Error()), retry)
		}
		status, err := exec.Status(entry.Path)
		if err != nil {
			return guard.NewFailure("cannot read bead worktree status: "+termsafe.Escape(err.Error()), retry)
		}
		var paths []string
		exportOnly := true
		for _, line := range strings.Split(status, "\n") {
			name, err := panel.PorcelainPath(line)
			if err != nil {
				return guard.NewFailure("cannot parse bead worktree status: "+termsafe.Escape(err.Error()), retry)
			}
			if name == "" {
				continue
			}
			paths = append(paths, termsafe.Escape(name))
			if !implCleanupArtifact(name) {
				exportOnly = false
			}
		}
		if len(paths) == 0 {
			continue
		}
		complete := fmt.Sprintf("mindspec complete %s", idrender.Bead(id))
		recovery := []string{complete}
		if !exportOnly {
			recovery = []string{fmt.Sprintf("commit or discard the listed paths in %s", termsafe.Escape(entry.Path)), complete,
				fmt.Sprintf("after a commit, follow the follow-up bead steps that mindspec complete prints, then run a new final-review round with mindspec panel create <slug> --spec %s --gate final_review --target spec/%s", idrender.Spec(specID), idrender.Spec(specID))}
		}
		return guard.NewFailure(fmt.Sprintf("bead %s worktree has uncommitted changes:\n%s", idrender.Bead(id), strings.Join(paths, "\n")), recovery...)
	}
	return nil
}

func implCleanupArtifact(name string) bool {
	if name == ".beads/issues.jsonl" {
		return true
	}
	for _, prefix := range []string{".mindspec/specs/", ".mindspec/docs/specs/", "docs/specs/"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			parts := strings.Split(rest, "/")
			return len(parts) == 4 && parts[1] == "reviews" && parts[3] == "dispositions.jsonl"
		}
	}
	return false
}

// An exact generated placeholder is a replay of our own write. Any other
// existing file retains the ADR creator's collision refusal.
func implEnsurePlaceholder(root, id, title string, opts adr.CreateOpts) error {
	name, err := workspace.ADRFilePath(root, id)
	if err != nil {
		return err
	}
	info, err := os.Lstat(name)
	if err == nil && info.Mode().IsRegular() {
		file, err := safefile.Open(name)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		_, omit, err := panel.DocumentSubjectContent(filepath.ToSlash(rel), data, "", "final_review")
		if err != nil {
			return err
		}
		if omit {
			return nil
		}
	}
	_, err = implCreateWithIDFn(root, id, title, opts)
	return err
}
