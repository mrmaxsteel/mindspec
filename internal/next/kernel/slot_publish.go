package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

const slotPublishRecovery = "restore access to the repository, then run mindspec next-action"

// slotPublishAttempts bounds the retries when another writer moves the spec
// branch between the read of its tip and the ref update.
const slotPublishAttempts = 5

// slotPending is a slot verdict that passed no check yet. It has a path and a
// blob, and no holding commit. publish writes the commit after every check.
type slotPending struct {
	key, path, blob string
}

// receipt returns the receipt of the pending verdict. The holding commit is
// empty until publish runs.
func (p *slotPending) receipt() Receipt {
	return Receipt{Kind: "passed", Path: p.path, Blob: p.blob}
}

// slotEvidence selects the evidence of a slot submit. The verdict file that the
// slot prompt names is in the slot's attempt directory, which no commit can
// hold. For that file it returns a pending verdict at the registered verdict
// path, and Submit publishes it only after every check passes. Any other file
// keeps the committed-file rules.
func slotEvidence(s State, r Record, t Tuple, o SubmitOptions) (*slotPending, error) {
	checked, err := checkSuppliedPath(s.Root, o.Artifact, artifactPathRecovery)
	if err != nil {
		return nil, pathFailure(err, "artifact must be a regular committed file: ", artifactPathRecovery)
	}
	if !checked.info.Mode().IsRegular() || !inSlotAttempt(s, t, checked.resolved) {
		if _, err := resolveArtifact(s, o); err != nil {
			var failure *termsafe.Failure
			if errors.As(err, &failure) {
				return nil, termsafe.NewWrappedFailure(err, failure.Message(), slotRecovery(s, t))
			}
			return nil, err
		}
		return nil, nil
	}
	return pendingSlotVerdict(s, r, t, checked.resolved)
}

// inSlotAttempt reports whether file is directly in an attempt directory of
// the slot directory of this step.
func inSlotAttempt(s State, t Tuple, file string) bool {
	reg, has, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
	if err != nil || !has {
		return false
	}
	iso, err := config.IsolationRootFor(s.Root)
	if err != nil {
		return false
	}
	layout, err := panel.NewSlotLayout(s.Root, iso, s.SpecID, reg.Slug, t.Round, fmt.Sprintf("R%d", t.Slot))
	if err != nil {
		return false
	}
	dir, err := filepath.EvalSymlinks(layout.Dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 2 || parts[1] == "" || parts[1] == "." || parts[1] == ".." {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "attempt-"))
	return err == nil && n > 0 && parts[0] == "attempt-"+strconv.Itoa(n)
}

// SlotVerdictPath is the registered verdict path of a slot in the active gate
// round: R<n>-round-<k>.json in the registered panel directory.
func SlotVerdictPath(s State, step string) (string, error) {
	t, has, err := s.Records[step].Tuple()
	if err != nil || !has || t.Kind != "review-slot" {
		return "", actorFailure("step is not a review slot")
	}
	return slotVerdictPath(s, t)
}

func slotVerdictPath(s State, t Tuple) (string, error) {
	reg, has, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
	if err != nil {
		return "", err
	}
	if !has {
		return "", actorFailure("slot gate has no registered panel")
	}
	dirs := registrationPaths(s, reg)
	if len(dirs) == 0 {
		return "", actorFailure("slot gate has no registered panel directory")
	}
	dir := dirs[0]
	for _, candidate := range dirs {
		if strings.HasSuffix(candidate, "/"+reg.Slug+"/") {
			dir = candidate
			break
		}
	}
	return fmt.Sprintf("%sR%d-round-%d.json", dir, t.Slot, t.Round), nil
}

// pendingSlotVerdict reads the verdict file and stores its bytes as a blob
// object. It writes no commit and no ref.
func pendingSlotVerdict(s State, r Record, t Tuple, file string) (*slotPending, error) {
	path, err := slotVerdictPath(s, t)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, pathFailure(err, "cannot read verdict file: ", artifactPathRecovery)
	}
	data, err := io.ReadAll(io.LimitReader(f, panel.MaxVerdictBytes+1))
	f.Close()
	if err != nil {
		return nil, pathFailure(err, "cannot read verdict file: ", artifactPathRecovery)
	}
	if len(data) > panel.MaxVerdictBytes {
		return nil, actorFailure("verdict file is too large")
	}
	blob, err := publishGit(s.Root, nil, data, "hash-object", "-w", "--stdin")
	if err != nil {
		return nil, err
	}
	return &slotPending{key: r.Key(), path: path, blob: blob}, nil
}

// ReceiptBytes returns the bytes that a receipt names. A slot receipt that is
// not published yet has no holding commit, and its blob gives the bytes.
func ReceiptBytes(root string, receipt Receipt) ([]byte, error) {
	if receipt.Commit != "" {
		return actorGitBytes(root, "show", receipt.Commit+":"+receipt.Path)
	}
	if !objectID(receipt.Blob) {
		return nil, actorFailure("receipt names no blob")
	}
	return actorGitBytes(root, "cat-file", "blob", receipt.Blob)
}

// publish commits the verdict at its registered path onto the spec branch and
// returns the holding commit. The spec branch push and the merge to main then
// carry the commit to every clone. The caller holds the phase lock. When the
// spec branch already holds the blob at the path, publish returns the commit
// that wrote it, so a retry with the same bytes writes no commit. On every
// path, publish brings each worktree that has the spec branch checked out to
// the holding commit, so a retry repairs a sync that an earlier run did not
// finish.
func (p *slotPending) publish(s State) (string, error) {
	branch, err := workspace.SpecBranch(s.SpecID)
	if err != nil {
		return "", err
	}
	ref := "refs/heads/" + branch
	for attempt := 0; attempt < slotPublishAttempts; attempt++ {
		tip, err := publishGit(s.Root, nil, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		if err != nil || !objectID(tip) {
			return "", termsafe.NewFailure("spec branch "+termsafe.Escape(branch)+" does not exist", "restore the spec branch, then run mindspec next-action")
		}
		old, _ := publishGit(s.Root, nil, nil, "rev-parse", "--verify", "--quiet", tip+":"+p.path)
		if old == p.blob {
			commit, err := publishGit(s.Root, nil, nil, "log", "-1", "--format=%H", tip, "--", p.path)
			if err != nil {
				return "", err
			}
			if held, e := publishGit(s.Root, nil, nil, "rev-parse", "--verify", "--quiet", commit+":"+p.path); e != nil || held != p.blob {
				return "", actorFailure("spec branch holds the verdict without a holding commit")
			}
			// A holding commit has one parent and changes only the verdict
			// path. Sync from that parent, which the caller no longer knows.
			if parent, ok := holdingParent(s.Root, p.path, commit); ok {
				if err = syncCheckouts(s.Root, ref, p.path, parent, commit); err != nil {
					return "", err
				}
			}
			return commit, nil
		}
		commit, err := p.commitOn(s.Root, tip)
		if err != nil {
			return "", err
		}
		if _, err = publishGit(s.Root, nil, nil, "update-ref", ref, commit, tip); err != nil {
			if now, e := publishGit(s.Root, nil, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); e == nil && now != tip {
				// Another writer moved the branch. Build again on its new tip.
				continue
			}
			return "", err
		}
		if err = syncCheckouts(s.Root, ref, p.path, tip, commit); err != nil {
			return "", err
		}
		return commit, nil
	}
	return "", termsafe.NewFailure("spec branch "+termsafe.Escape(branch)+" moved during the verdict commit", slotRetryRecovery)
}

// slotRetryRecovery is the recovery when a retry of the same submit finishes
// the publish.
const slotRetryRecovery = "run the same submit command again"

// syncHeldVerdict brings each spec worktree to the holding commit of a slot
// receipt. Every submit route that ends with the verdict held on the spec
// branch runs it: a repair, a replay of a closed receipt and a retry of the
// same bytes. A run that an index lock or a crash stopped after the spec
// branch moved is thus finished by the command that the binary prints. It
// does nothing when the spec branch tip does not hold the receipt blob at the
// receipt path, or when the receipt commit is not a holding commit.
func syncHeldVerdict(s State, receipt Receipt) error {
	if receipt.Commit == "" || receipt.Path == "" || !objectID(receipt.Blob) {
		return nil
	}
	branch, err := workspace.SpecBranch(s.SpecID)
	if err != nil {
		return nil
	}
	ref := "refs/heads/" + branch
	if held, _ := publishGit(s.Root, nil, nil, "rev-parse", "--verify", "--quiet", ref+":"+receipt.Path); held != receipt.Blob {
		return nil
	}
	parent, ok := holdingParent(s.Root, receipt.Path, receipt.Commit)
	if !ok {
		return nil
	}
	return syncCheckouts(s.Root, ref, receipt.Path, parent, receipt.Commit)
}

// holdingParent returns the parent of commit when commit has one parent and
// changes only path.
func holdingParent(root, path, commit string) (string, bool) {
	parents, err := publishGit(root, nil, nil, "rev-list", "--parents", "-n", "1", commit)
	if err != nil {
		return "", false
	}
	fields := strings.Fields(parents)
	if len(fields) != 2 || fields[0] != commit || !objectID(fields[1]) {
		return "", false
	}
	changed, err := publishGit(root, nil, nil, "diff-tree", "--no-commit-id", "--name-only", "-r", fields[1], commit)
	if err != nil || changed != path {
		return "", false
	}
	return fields[1], true
}

// commitOn writes a commit with parent tip whose tree is the tree of tip plus
// the verdict blob at its path. The commit uses the git identity of the
// operator and is signed when commit.gpgsign asks for it, as git commit does.
func (p *slotPending) commitOn(root, tip string) (string, error) {
	tmp, err := os.MkdirTemp("", "mindspec-verdict-")
	if err != nil {
		return "", pathFailure(err, "cannot publish slot verdict: ", slotPublishRecovery)
	}
	defer os.RemoveAll(tmp)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(tmp, "index")}
	if _, err = publishGit(root, env, nil, "read-tree", tip); err != nil {
		return "", err
	}
	if _, err = publishGit(root, env, nil, "update-index", "--add", "--cacheinfo", "100644,"+p.blob+","+p.path); err != nil {
		return "", err
	}
	tree, err := publishGit(root, env, nil, "write-tree")
	if err != nil {
		return "", err
	}
	signing, err := gitutil.CommitSigningEnabled(root)
	if err != nil {
		return "", termsafe.NewWrappedFailure(err, "cannot publish slot verdict: "+err.Error(), slotCommitRecovery)
	}
	argv := []string{"commit-tree"}
	if signing {
		argv = append(argv, "-S")
	}
	argv = append(argv, tree, "-p", tip, "-m", "Record slot verdict "+p.key)
	commit, err := publishGit(root, commitIdentity(root), nil, argv...)
	if err != nil {
		var failure *termsafe.Failure
		if errors.As(err, &failure) {
			return "", termsafe.NewWrappedFailure(err, failure.Message(), slotCommitRecovery)
		}
		return "", err
	}
	return commit, nil
}

// slotCommitRecovery is the recovery when git cannot write the verdict
// commit. The spec branch did not move.
const slotCommitRecovery = "fix the git identity or commit signing setup that the message names, then run the same submit command again"

// commitIdentity returns the identity environment for the verdict commit.
// The operator's git identity applies when git can resolve one, as for git
// commit. Only a role that git cannot resolve (no user.email and no
// detectable address, as on a bare runner) gets the fixed mindspec identity,
// so the publish does not fail where a commit needs no human author.
func commitIdentity(root string) []string {
	var env []string
	for _, role := range []string{"AUTHOR", "COMMITTER"} {
		if _, err := publishGit(root, nil, nil, "var", "GIT_"+role+"_IDENT"); err != nil {
			env = append(env, "GIT_"+role+"_NAME=mindspec", "GIT_"+role+"_EMAIL=mindspec@localhost")
		}
	}
	return env
}

// syncCheckouts brings each worktree that has the spec branch checked out
// from base to holding. holding differs from base only at the verdict path.
// A listed worktree whose directory is gone (git marks it prunable) holds no
// work and no index that a user can see, so the sync skips it; git worktree
// prune removes its entry.
func syncCheckouts(root, ref, path, base, holding string) error {
	list, err := publishGit(root, nil, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	for _, tree := range specTrees(list, ref) {
		if err := syncCheckout(root, tree, path, base, holding); err != nil {
			return err
		}
	}
	return nil
}

// specTrees returns the directory of each listed worktree that has ref checked
// out and that exists. A worktree that git marks prunable, or whose directory
// does not exist, is left out.
func specTrees(list, ref string) []string {
	var trees []string
	for _, block := range strings.Split(list, "\n\n") {
		dir, onRef, prunable := "", false, false
		for _, line := range strings.Split(block, "\n") {
			if value, ok := strings.CutPrefix(line, "worktree "); ok {
				dir = value
			}
			onRef = onRef || line == "branch "+ref
			prunable = prunable || line == "prunable" || strings.HasPrefix(line, "prunable ")
		}
		if dir == "" || !onRef || prunable {
			continue
		}
		if info, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
			continue
		}
		trees = append(trees, dir)
	}
	return trees
}

// syncCheckout brings one worktree from base to holding. When a file is at
// the path, git read-tree -m -u does the update in one git process under the
// index lock, like git checkout: it keeps every local change and refuses when
// a local change at the path would be overwritten. When it refuses, or when no
// file is at the path, the index alone moves (read-tree -m -i). That command
// refuses a staged change at the path, an unmerged index, or a lock that
// another git command holds; the refusal stops the sync with a recovery, and a
// retry of the same submit finishes it. A deleted tracked file stays deleted:
// the path is tracked when the base commit or the worktree index has it, so a
// file that an earlier run wrote and the user then deleted stays deleted on a
// retry. Only for a path that neither tracks does checkout-index without
// --force write the file, and it never replaces a file that appears before it
// writes.
func syncCheckout(root, tree, path, base, holding string) error {
	file := filepath.Join(tree, filepath.FromSlash(path))
	_, err := os.Lstat(file)
	absent := errors.Is(err, fs.ErrNotExist)
	if err != nil && !absent {
		return syncFailure(err, tree, path)
	}
	indexed := false
	if absent {
		staged, err := publishGit(tree, nil, nil, "ls-files", "--stage", "--", path)
		if err != nil {
			return syncFailure(err, tree, path)
		}
		indexed = staged != ""
	}
	if !absent {
		if _, err := publishGit(tree, nil, nil, "read-tree", "-m", "-u", base, holding); err == nil {
			return nil
		}
	}
	if _, err := publishGit(tree, nil, nil, "read-tree", "-m", "-i", base, holding); err != nil {
		return syncFailure(err, tree, path)
	}
	if !absent {
		// A local change at the path stays as the user left it.
		return nil
	}
	if indexed {
		// The index tracked the path before this sync: an earlier run wrote
		// the file and the user deleted it. Keep the deletion.
		return nil
	}
	if baseBlob, _ := publishGit(root, nil, nil, "rev-parse", "--verify", "--quiet", base+":"+path); baseBlob != "" {
		// The user deleted the tracked file. Keep the deletion.
		return nil
	}
	if _, err := publishGit(tree, nil, nil, "checkout-index", "--", path); err != nil {
		if _, e := os.Lstat(file); errors.Is(e, fs.ErrNotExist) {
			return syncFailure(err, tree, path)
		}
	}
	return nil
}

// syncFailure reports a worktree that is not yet at the holding commit. The
// spec branch holds the verdict; the same submit finishes the sync.
func syncFailure(err error, tree, path string) error {
	cause := err.Error()
	var failure *termsafe.Failure
	if errors.As(err, &failure) {
		cause = failure.Message()
	}
	return termsafe.NewWrappedFailure(err, "spec branch holds the verdict, but the worktree "+termsafe.Escape(tree)+" is not in step with it: "+cause,
		"in "+termsafe.Escape(tree)+", let the git command that holds the index lock finish, resolve any unmerged path and unstage any change at "+termsafe.Escape(path)+" (git status names both), then run the same submit command again")
}

func publishGit(root string, env []string, stdin []byte, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(bytes.TrimSpace(exit.Stderr)) > 0 {
			err = fmt.Errorf("git %s: %w: %s", args[0], err, stderrLine(exit.Stderr))
		} else {
			err = fmt.Errorf("git %s: %w", args[0], err)
		}
		return "", pathFailure(err, "cannot publish slot verdict: ", slotPublishRecovery)
	}
	return strings.TrimSpace(string(out)), nil
}

// stderrLine joins the non-empty lines of git stderr with "; ", so a refusal
// shows the git cause on one readable line.
func stderrLine(stderr []byte) string {
	var parts []string
	for _, line := range strings.FieldsFunc(string(stderr), func(r rune) bool { return r == '\n' || r == '\r' }) {
		if line = strings.TrimSpace(line); line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "; ")
}

// SlotPromptRecovery names the command that prints the verdict path and the
// submit command of a slot step.
func SlotPromptRecovery(s State, step string) string {
	t, has, err := s.Records[step].Tuple()
	if err != nil || !has || t.Kind != "review-slot" {
		return "mindspec next-action"
	}
	return slotRecovery(s, t)
}

// slotRecovery names the command that prints the verdict path and submit
// command of a slot.
func slotRecovery(s State, t Tuple) string {
	slug := "<slug>"
	if reg, has, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration); err == nil && has && reg.Slug != "" {
		slug = briefQuote(reg.Slug)
	}
	return fmt.Sprintf("mindspec panel prompt %s --slot R%d --spec %s, then write the verdict to the path it names and run the submit command it prints", slug, t.Slot, briefQuote(idrender.Spec(s.SpecID)))
}
