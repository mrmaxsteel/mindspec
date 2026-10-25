package approve

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/complete"
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
func implFinalReviewGate(root, specID, epicID, storedPhase, specDir string, exec executor.Executor, placeholder implPlaceholderWrite) (Outcome, bool, error) {
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
	if err != nil || !cfg.Enforcement.PanelGate || os.Getenv(panel.SkipPanelEnv) == "1" {
		return o, false, err
	}
	branch, err := workspace.SpecBranch(specID)
	if err != nil {
		return o, false, err
	}
	head, err := exec.RevParseRef(wt, "refs/heads/"+branch)
	if err != nil {
		return o, false, err
	}
	var reviewed []string
	for _, reg := range lifecycleRegistrations(panel.Scan(complete.PanelGateRoots(root, wt, specID)...), specID, "final_review") {
		if !reg.Panel.Abandoned {
			reviewed = append(reviewed, reg.Panel.ReviewedHeadSHA)
		}
	}
	// Without a live review, only pending files need this check.
	if len(reviewed) == 0 {
		reviewed = append(reviewed, head)
	}
	err = implPlaceholderPreflight(exec, wt, specDir, head, reviewed, placeholder, true)
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
	return implPlaceholderPreflight(exec, root, specDir, "refs/heads/"+branch, []string{tip}, implPlaceholderWrite{}, false)
}

// implBeadWorktreePreflight checks the same plan-scoped worktree population
// that finalization consumes. Unlike the review subject it excludes no dirt.
func implBeadWorktreePreflight(specID string, allowIDs []string, exec executor.Executor) error {
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

// implPlaceholderWrite has the same domain seed as the placeholder writer.
type implPlaceholderWrite struct {
	ID      string
	Domains []string
}

func implADRPath(name string) bool {
	for _, root := range []string{".mindspec/adr/", ".mindspec/docs/adr/", "docs/adr/"} {
		if strings.HasPrefix(name, root) {
			return true
		}
	}
	return false
}

// implPlaceholderTree uses the digest's exact placeholder matcher.
func implPlaceholderTree(exec executor.Executor, worktree, specDir, ref string) (map[string][]byte, error) {
	listing, err := exec.LsTree(worktree, ref)
	if err != nil {
		return nil, err
	}
	result := map[string][]byte{}
	for _, line := range strings.Split(listing, "\n") {
		if line == "" {
			continue
		}
		meta, name, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("malformed subject tree listing")
		}
		if strings.HasPrefix(name, "\"") {
			name, err = strconv.Unquote(name)
			if err != nil {
				return nil, err
			}
		}
		if !implADRPath(name) || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			continue
		}
		data, err := exec.FileAtRef(ref, name)
		if err != nil {
			return nil, err
		}
		_, omit, err := panel.DocumentSubjectContent(name, data, specDir, "final_review")
		if err != nil {
			return nil, err
		}
		if omit {
			result[name] = data
		}
	}
	return result, nil
}

func implPlaceholderDirt(exec executor.Executor, worktree, specDir string) (map[string][]byte, error) {
	status, err := exec.Status(worktree)
	if err != nil {
		return nil, err
	}
	result := map[string][]byte{}
	read := func(name string) error {
		if !implADRPath(name) {
			return nil
		}
		full := filepath.Join(worktree, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		if os.IsNotExist(err) {
			result[name] = nil
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := safefile.Open(full)
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
		_, omit, err := panel.DocumentSubjectContent(name, data, specDir, "final_review")
		if err != nil {
			return err
		}
		if omit {
			result[name] = data
		}
		return nil
	}
	for _, line := range strings.Split(status, "\n") {
		name, err := panel.PorcelainPath(line)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		if strings.HasSuffix(name, "/") {
			err = filepath.WalkDir(filepath.Join(worktree, filepath.FromSlash(name)), func(full string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(worktree, full)
				if err != nil {
					return err
				}
				return read(filepath.ToSlash(rel))
			})
		} else {
			err = read(name)
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func implPlaceholderPreflight(exec executor.Executor, worktree, specDir, head string, reviewed []string, write implPlaceholderWrite, withDirt bool) error {
	refuse := func(detail string) error {
		return guard.NewFailure("gate final_review: "+detail,
			"commit the ADR and run a new final-review round, or remove the file")
	}
	current, err := implPlaceholderTree(exec, worktree, specDir, head)
	if err != nil {
		return refuse(termsafe.Escape(err.Error()))
	}
	changed := map[string][][]byte{}
	for _, ref := range reviewed {
		before, err := implPlaceholderTree(exec, worktree, specDir, ref)
		if err != nil {
			return refuse(termsafe.Escape(err.Error()))
		}
		for name, data := range current {
			if !bytes.Equal(before[name], data) {
				changed[name] = append(changed[name], data)
			}
		}
		for name := range before {
			if _, present := current[name]; !present {
				changed[name] = append(changed[name], nil)
			}
		}
	}
	pending := map[string][]byte{}
	if withDirt {
		pending, err = implPlaceholderDirt(exec, worktree, specDir)
		if err != nil {
			return refuse(termsafe.Escape(err.Error()))
		}
		for name, data := range pending {
			if !bytes.Equal(current[name], data) {
				changed[name] = append(changed[name], data)
			}
		}
	}
	allowed := ""
	if write.ID != "" {
		full, err := workspace.ADRFilePath(worktree, write.ID)
		if err != nil {
			return refuse(termsafe.Escape(err.Error()))
		}
		rel, err := filepath.Rel(worktree, full)
		if err != nil {
			return refuse(termsafe.Escape(err.Error()))
		}
		allowed = filepath.ToSlash(rel)
	}
	domains := "<comma-separated list>"
	if len(write.Domains) > 0 {
		domains = strings.Join(write.Domains, ", ")
	}
	// The writer keeps an exact existing placeholder on replay. Retain its
	// creation date so a retry on a later day compares the same text.
	creationDate := time.Now().Format("2006-01-02")
	retained := current[allowed]
	if data, present := pending[allowed]; present {
		retained = data
	}
	if bytes.Contains(retained, []byte("\n- **Domain(s)**: "+domains+"\n")) {
		for _, line := range strings.Split(string(retained), "\n") {
			if value, ok := strings.CutPrefix(line, "- **Date**: "); ok {
				creationDate = value
				break
			}
		}
	}
	var paths []string
	for name, versions := range changed {
		for _, data := range versions {
			// The digest matcher already checks every other template byte.
			// Use the creation date and the seed known before the gate.
			text := string(data)
			matches := strings.Contains(text, "\n- **Domain(s)**: "+domains+"\n") &&
				strings.Contains(text, "\n- **Date**: "+creationDate+"\n")
			if name != allowed || !matches {
				paths = append(paths, termsafe.Escape(name))
				break
			}
		}
	}
	if len(paths) > 0 {
		sort.Strings(paths)
		return refuse("placeholder changes are outside this approval's write set:\n" + strings.Join(paths, "\n"))
	}
	return nil
}
