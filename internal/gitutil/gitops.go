package gitutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// ErrRefNotFound is returned by RevParseRef when the named ref genuinely does
// not exist (git `rev-parse --verify --quiet` exits 1 with empty output). It
// is distinguished from a transient/structural git failure (exit 128, git
// missing, lock contention) so callers can treat the "ref absent" case as the
// expected branch-already-deleted condition without also fail-clearing on a
// transient error (Spec 093 Req 11 missing-ref pass-through).
var ErrRefNotFound = errors.New("ref not found")

// Package-level function variables for testability.
var execCommand = exec.Command

// noPrompt sets GIT_TERMINAL_PROMPT=0 on a built *exec.Cmd so git fast-fails
// (non-zero exit) on a slow/auth-prompting origin instead of HANGING or
// prompting on stdin — e.g. during `mindspec spec create` (Spec 103 R1,
// ADR-0030 git-exec boundary). The executor reads that non-zero exit as the
// signal to fall back to a local base + WARN. The env is APPENDED to
// os.Environ() (NOT a fresh slice) so PATH/HOME/git-config discovery survive;
// a fresh []string{...} would clobber the inherited environment and break git.
// Applied only to the network/credential ops (fetch, push, remote show,
// symbolic-ref); pure-local ops never prompt. Returns cmd for chaining.
func noPrompt(cmd *exec.Cmd) *exec.Cmd {
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

// RejectOptionLike is the package-boundary argument-safety guard
// (SEC-5 / spec 097 R1, finding obxo). git parses a positional argv slot
// that begins with `-` as an OPTION rather than a ref/branch/refspec/range
// operand — a ref literally named `-x` or `--upload-pack=…` would be
// reinterpreted as `git checkout --upload-pack=…`. `internal/gitutil` is
// the Git-process I/O boundary (ADR-0030), so it rejects any such hostile
// operand at its own edge before shelling out, returning an ADR-0035-shaped
// error (message body + final `recovery:` line). All current callers pass
// controlled refs (`main`, `spec/<id>`, `bead/<id>`), so this is
// defense-in-depth: the only behavior change is that a `-`-prefixed operand
// now errors instead of being handed to git.
//
// It is EXPORTED so sibling boundary packages that run their OWN direct
// git exec (notably internal/executor's ref-bearing MergeBase / FileAtRef /
// pathExistsAtRef / TreeDirsAtRef / ChangedFiles, which do not route through
// this package) can apply the identical guard before reaching git argv —
// closing the option-injection path a panel reviewer traced through
// internal/complete overwriting beadHead with a `bd worktree list` name
// (spec 097 R1, executor gap). gitutil is the canonical home: it already
// depends only on internal/guard (a leaf), so executor→gitutil stays the
// normal, cycle-free import direction.
//
// It is applied PER-OPERAND at each ref-bearing entry point rather than
// inside the shared gitArgs builder, because gitArgs cannot distinguish a
// ref from a legitimate option-flag (e.g. `--no-ff`, `--stat`) or a
// pathspec. The empty string is allowed (`workdir==""` and empty
// pathspec lists are valid and never reach this guard as operands).
func RejectOptionLike(operand string) error {
	if strings.HasPrefix(operand, "-") {
		return guard.NewFailure(
			fmt.Sprintf("blocked: git operand %q looks like an option (begins with %q); refusing to pass a hostile ref/branch/refspec to git (SEC-5)", operand, "-"),
			"pass a ref that does not begin with '-' (e.g. main, spec/<id>, bead/<id>)",
		)
	}
	return nil
}

// rejectOptionLike is the unexported alias retained for the existing
// in-package call sites; it delegates to the exported RejectOptionLike.
func rejectOptionLike(operand string) error { return RejectOptionLike(operand) }

// CurrentBranch returns the name of the current git branch.
func CurrentBranch() (string, error) {
	cmd := execCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// BranchExists returns true if the named branch exists locally.
//
// The `refs/heads/` prefix already prevents a leading `-` in name from
// reaching git as an option, but the SEC-5 guard is applied for
// consistency: a `-`-prefixed name is treated as a non-existent branch.
func BranchExists(name string) bool {
	if rejectOptionLike(name) != nil {
		return false
	}
	cmd := execCommand("git", "rev-parse", "--verify", "refs/heads/"+name)
	return cmd.Run() == nil
}

// CreateBranch creates a new branch from the given base.
func CreateBranch(name, from string) error {
	if err := rejectOptionLike(name); err != nil {
		return err
	}
	if err := rejectOptionLike(from); err != nil {
		return err
	}
	cmd := execCommand("git", "branch", "--", name, from)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating branch %s from %s: %s", name, from, strings.TrimSpace(string(out)))
	}
	return nil
}

// MergeBranch merges source into target using --no-ff (from the given workdir).
// If workdir is empty, uses the current directory.
func MergeBranch(workdir, source, target string) error {
	if err := rejectOptionLike(source); err != nil {
		return err
	}
	if err := rejectOptionLike(target); err != nil {
		return err
	}

	// Checkout target. The `--` is TRAILING (`checkout <ref> --`): a leading
	// `--` would force git to treat `target` as a pathspec rather than a
	// branch to switch to. Trailing `--` disambiguates ref-vs-path while
	// still selecting the branch.
	checkoutCmd := execCommand("git", "-C", workdir, "checkout", target, "--")
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s: %s", target, strings.TrimSpace(string(out)))
	}

	// Merge source. `-m <msg>` precedes the `--` separator so the message
	// is not reparsed as a commit operand (everything after `--` is a
	// commit to merge).
	mergeCmd := execCommand("git", "-C", workdir, "merge", "--no-ff", "-m",
		fmt.Sprintf("Merge %s into %s", source, target), "--", source)
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge %s into %s: %s", source, target, strings.TrimSpace(string(out)))
	}

	return nil
}

// MergeInto merges sourceBranch into the current branch of targetWorkdir.
// Unlike MergeBranch, this does not checkout — it assumes targetWorkdir already
// has the desired branch checked out (e.g. a spec worktree).
func MergeInto(targetWorkdir, sourceBranch string) error {
	if err := rejectOptionLike(sourceBranch); err != nil {
		return err
	}
	mergeCmd := execCommand("git", "-C", targetWorkdir, "merge", "--no-ff", "-m",
		fmt.Sprintf("Merge %s", sourceBranch), "--", sourceBranch)
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge %s in %s: %s", sourceBranch, targetWorkdir, strings.TrimSpace(string(out)))
	}
	return nil
}

// ConflictedFiles returns the paths with unmerged index entries in
// workdir (the conflicted files of an in-progress merge). Best-effort:
// returns nil when git fails or there are no unmerged entries.
func ConflictedFiles(workdir string) []string {
	cmd := execCommand("git", "-C", workdir, "diff", "--name-only", "--diff-filter=U")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// MergeInProgress reports whether workdir has an in-progress merge
// (MERGE_HEAD present).
func MergeInProgress(workdir string) bool {
	cmd := execCommand("git", "-C", workdir, "rev-parse", "-q", "--verify", "MERGE_HEAD")
	return cmd.Run() == nil
}

// AbortMerge aborts an in-progress merge in workdir, restoring the
// pre-merge working tree (`git merge --abort`).
func AbortMerge(workdir string) error {
	cmd := execCommand("git", "-C", workdir, "merge", "--abort")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("aborting merge in %s: %s", workdir, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteBranch deletes a local branch.
func DeleteBranch(name string) error {
	if err := rejectOptionLike(name); err != nil {
		return err
	}
	cmd := execCommand("git", "branch", "-D", "--", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deleting branch %s: %s", name, strings.TrimSpace(string(out)))
	}
	return nil
}

// HasRemote returns true if at least one git remote is configured.
func HasRemote() bool {
	cmd := execCommand("git", "remote")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// FetchRemote runs `git fetch <remote>` from the current working directory so
// the remote-tracking refs (origin/*) are current before a branch is created
// from them (Spec 101 R4). A non-zero exit (offline, auth failure, missing
// remote) surfaces as an error; callers treat that as the signal to fall back
// to a local base rather than hard-failing.
func FetchRemote(remote string) error {
	if err := rejectOptionLike(remote); err != nil {
		return err
	}
	cmd := noPrompt(execCommand("git", "fetch", remote))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetching %s: %s", remote, strings.TrimSpace(string(out)))
	}
	return nil
}

// FetchRemoteBranch runs `git fetch <remote> <branch>` from the current
// working directory — a NARROWER fetch than FetchRemote's `git fetch
// <remote>` (which pulls every branch). Bug wu7t's protected-main finalize
// check (MindspecExecutor.FinalizeEpic) only needs origin/main's tip current
// before comparing ancestry, so this avoids paying for a full multi-branch
// fetch on every `impl approve`. Same GIT_TERMINAL_PROMPT=0 fast-fail
// (noPrompt) and SEC-5 RejectOptionLike argv hygiene as FetchRemote; a
// non-zero exit (offline, auth failure, missing remote/branch) surfaces as
// an error so the caller can fall back rather than hard-failing.
func FetchRemoteBranch(remote, branch string) error {
	if err := rejectOptionLike(remote); err != nil {
		return err
	}
	if err := rejectOptionLike(branch); err != nil {
		return err
	}
	cmd := noPrompt(execCommand("git", "fetch", remote, branch))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetching %s %s: %s", remote, branch, strings.TrimSpace(string(out)))
	}
	return nil
}

// DetectDefaultBranch returns the default branch name of remote (e.g. "main",
// "develop") WITHOUT hardcoding (Spec 101 R4). It tries the cheap cached
// `git symbolic-ref refs/remotes/<remote>/HEAD` first; if that output is empty
// or not a valid `refs/remotes/<remote>/<name>` (the cached ref is not always
// populated, so an unparseable result is a MISS, not a default) it falls
// THROUGH to `git remote show <remote>` and parses its "HEAD branch:" line.
// An error is returned only when BOTH sources fail to yield a branch name —
// the executor funnels that into its local-HEAD + WARN fallback.
func DetectDefaultBranch(remote string) (string, error) {
	if err := rejectOptionLike(remote); err != nil {
		return "", err
	}

	// 1) Cached symbolic-ref (cheap, offline-friendly).
	symRefPrefix := "refs/remotes/" + remote + "/"
	out, err := noPrompt(execCommand("git", "symbolic-ref", symRefPrefix+"HEAD")).Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// Only accept a well-formed refs/remotes/<remote>/<name>; anything
		// else (empty, refs/heads/*, garbage) is treated as a miss.
		if name := strings.TrimPrefix(ref, symRefPrefix); name != "" && name != ref && !strings.Contains(name, "/") {
			return name, nil
		}
	}

	// 2) Fall through to `git remote show <remote>` ("HEAD branch: <name>").
	out, err = noPrompt(execCommand("git", "remote", "show", remote)).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("detecting default branch of %s: %s", remote, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if name := strings.TrimPrefix(line, "HEAD branch:"); name != line {
			name = strings.TrimSpace(name)
			if name != "" && name != "(unknown)" {
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("detecting default branch of %s: no HEAD branch in `git remote show`", remote)
}

// PushBranch pushes a branch to origin.
func PushBranch(branch string) error {
	if err := rejectOptionLike(branch); err != nil {
		return err
	}
	cmd := noPrompt(execCommand("git", "push", "-u", "origin", branch))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pushing %s: %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoteHeadSHA returns the SHA of refs/heads/<branch> on remote via
// `git ls-remote --heads <remote> <branch>`, or "" (nil error) when the
// branch does not exist on the remote. It queries the REMOTE directly —
// never the local remote-tracking ref, which can be stale or absent after a
// crashed run — so bug wu7t's retry-idempotent chore-branch push can pin its
// force-with-lease expectation to the remote's true current tip. Same
// GIT_TERMINAL_PROMPT=0 fast-fail and SEC-5 argv hygiene as the other
// network ops; a non-zero exit (offline, auth failure) is an error, NOT an
// absent branch.
//
// Spec 119 Bead 3 (AC-16): `git ls-remote --heads <remote> <pattern>`
// matches any ref whose name is a slash-delimited SUFFIX of pattern — e.g.
// pattern "chore/finalize-105" also matches "refs/heads/aaa/chore/finalize-105"
// (a decoy branch nested under an unrelated prefix) — so the command can
// return MULTIPLE lines, or a decoy-only line, for a single-branch query.
// Every returned line is scanned and ONLY the one whose refname column is
// EXACTLY "refs/heads/<branch>" is accepted; a decoy-only result yields ""
// (absent), never the decoy's SHA.
func RemoteHeadSHA(remote, branch string) (string, error) {
	if err := rejectOptionLike(remote); err != nil {
		return "", err
	}
	if err := rejectOptionLike(branch); err != nil {
		return "", err
	}
	cmd := noPrompt(execCommand("git", "ls-remote", "--heads", remote, branch))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ls-remote %s %s: %w", remote, branch, err)
	}
	// Output: one "<sha>\trefs/heads/<name>\n" line per matching ref, or
	// empty when nothing matches at all.
	wantRef := "refs/heads/" + branch
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == wantRef {
			return fields[0], nil
		}
	}
	return "", nil
}

// PushBranchForceWithLease pushes branch to origin with
// `--force-with-lease=refs/heads/<branch>:<expectedSHA>` — a compare-and-swap
// push that succeeds only while the remote tip is still exactly expectedSHA.
// Bug wu7t's chore/finalize-<specID> branch is MACHINE-OWNED (regenerated
// fresh from origin/main + live Dolt on every run), so a retried
// `impl approve` must be able to overwrite the remote tip left by a prior
// run's push (a plain push is rejected non-fast-forward, hard-failing the
// retry) — but the lease is pinned to the SHA the caller just observed via
// RemoteHeadSHA.
//
// Spec 119 Bead 3 (AC-16): this protects only the READ-TO-PUSH window
// between that RemoteHeadSHA observation and this push — it is NOT a
// guarantee that the tip is "never silently clobbered". If some OTHER
// writer moves the remote tip AFTER the caller's RemoteHeadSHA read but
// BEFORE this push lands, the lease correctly fails loudly (non-fast-
// forward-equivalent rejection) rather than clobbering; but if that other
// write happens to land on the SAME expectedSHA the caller observed (e.g.
// a concurrent retry racing this one), the lease's compare succeeds and
// this push overwrites whatever the other writer produced — the lease
// compares a SHA, not a serialized "nobody else touched this ref" claim.
func PushBranchForceWithLease(branch, expectedSHA string) error {
	if err := rejectOptionLike(branch); err != nil {
		return err
	}
	if err := rejectOptionLike(expectedSHA); err != nil {
		return err
	}
	lease := "--force-with-lease=refs/heads/" + branch + ":" + expectedSHA
	cmd := noPrompt(execCommand("git", "push", lease, "-u", "origin", branch))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pushing %s (lease %s): %s", branch, expectedSHA, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureGitignoreEntry adds an entry to .gitignore if not already present.
func EnsureGitignoreEntry(root, entry string) error {
	gitignorePath := root + "/.gitignore"

	// Read existing content
	data, err := readFile(gitignorePath)
	if err != nil {
		data = nil // File doesn't exist yet
	}

	// Check if already present
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry || trimmed == entry+"/" {
			return nil // Already present
		}
	}

	// Append
	content := string(data)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "# mindspec worktrees\n" + entry + "/\n"

	return writeFile(gitignorePath, []byte(content), 0o644)
}

// FileAtRef returns the byte contents of path at git ref in the repo rooted
// at workdir, via `git show <ref>:<path>`. Mirrors
// executor.MindspecExecutor.FileAtRef, but lives in gitutil so
// internal/lifecycle (outside the executor/enforcement boundary, ADR-0030)
// can read a committed blob — e.g. main's committed .beads/issues.jsonl —
// without going through the executor.
func FileAtRef(workdir, ref, path string) ([]byte, error) {
	if err := rejectOptionLike(ref); err != nil {
		return nil, err
	}
	cmd := execCommand("git", "-C", workdir, "show", ref+":"+path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", ref, path, err)
	}
	return out, nil
}

// DiffStat returns a short diffstat summary between two refs.
// workdir specifies the git repository path.
func DiffStat(workdir, base, head string) (string, error) {
	if err := rejectOptionLike(base); err != nil {
		return "", err
	}
	if err := rejectOptionLike(head); err != nil {
		return "", err
	}
	// NOTE: range operands MUST NOT get a `--` separator — `--` would
	// reinterpret `base..head` as a pathspec. The leading-`-` guard alone
	// protects them (SEC-5).
	cmd := execCommand("git", "-C", workdir, "diff", "--stat", base+".."+head)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("diffstat %s..%s: %w", base, head, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CommitCount returns the number of commits between base and head.
func CommitCount(workdir, base, head string) (int, error) {
	if err := rejectOptionLike(base); err != nil {
		return 0, err
	}
	if err := rejectOptionLike(head); err != nil {
		return 0, err
	}
	// Range operand: no `--` separator (see DiffStat).
	cmd := execCommand("git", "-C", workdir, "rev-list", "--count", base+".."+head)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("commit count %s..%s: %w", base, head, err)
	}
	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count); err != nil {
		return 0, fmt.Errorf("parsing commit count: %w", err)
	}
	return count, nil
}

// IsAncestor returns true if ancestor is an ancestor of descendant.
// Uses git merge-base --is-ancestor.
func IsAncestor(workdir, ancestor, descendant string) (bool, error) {
	if err := rejectOptionLike(ancestor); err != nil {
		return false, err
	}
	if err := rejectOptionLike(descendant); err != nil {
		return false, err
	}
	cmd := execCommand("git", "-C", workdir, "merge-base", "--is-ancestor", ancestor, descendant)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	// Exit code 1 means not an ancestor; other errors are real failures.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("checking ancestry %s..%s: %w", ancestor, descendant, err)
}

// MergeCommit describes one first-parent merge commit as returned by
// FirstParentMerges: its own SHA, its full parent-SHA list (in the order
// git records them — Parents[0] is the first-parent chain member,
// Parents[1] is the merged-in side for a plain two-parent merge), and its
// commit subject line.
type MergeCommit struct {
	SHA     string
	Parents []string
	Subject string
}

// FirstParentMerges lists ref's first-parent merge commits in workdir,
// newest-first, each with its full parent list and subject line. It is the
// git-I/O primitive behind the landed-merge-commit-identity predicate
// (internal/lifecycle.FindLandedMerge, Spec 119 R4): every in-binary
// bead->spec merge is gitutil.MergeInto's deterministic
// `git merge --no-ff -m "Merge bead/<id>"`, which lands as a first-parent
// merge commit on the spec branch whose subject is exactly that string and
// whose second parent is the merged bead branch's tip at merge time.
//
// Uses `git log` (not `git rev-list`) with a bare --format: unlike
// `--pretty`, this emits exactly one line per commit with no leading
// "commit <sha>" header line, which rev-list's --format does NOT suppress.
// Fields are separated with \x1f (unit separator) so a subject containing
// spaces or (in principle) other punctuation never misparses.
func FirstParentMerges(workdir, ref string) ([]MergeCommit, error) {
	if err := rejectOptionLike(ref); err != nil {
		return nil, err
	}
	cmd := execCommand("git", gitArgs(workdir, "log", "--first-parent", "--merges", "--format=%H%x1f%P%x1f%s", ref)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("log --first-parent --merges %s: %w", ref, err)
	}
	var merges []MergeCommit
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\x1f", 3)
		if len(fields) != 3 {
			continue
		}
		merges = append(merges, MergeCommit{
			SHA:     fields[0],
			Parents: strings.Fields(fields[1]),
			Subject: fields[2],
		})
	}
	return merges, nil
}

// ExactSecondParentMerges returns branch's first-parent merge commits in
// workdir (per FirstParentMerges, so already newest-first), filtered to
// those that are a plain two-parent merge whose SECOND parent EQUALS tip
// exactly. This is the root-of-trust EXACT-MATCH identity primitive spec
// 125 shares between the merge-time binding write
// (internal/executor.locateLandedMergeByIdentity, R1/R2) and the
// landed-merge read (internal/lifecycle.FindLandedMerge, R5): "landed-ness"
// is git TOPOLOGY, never a merge's subject text. A candidate with anything
// other than exactly two parents (an octopus merge, or a non-merge — which
// FirstParentMerges never returns anyway) is EXCLUDED, never guessed at; a
// candidate whose second parent merely descends from or is an ancestor of
// tip, rather than EQUALING it, is likewise excluded — ancestor-consistent
// is not exact-match (an ancestor-consistent scan is exactly the
// misattribution vector a later, unrelated bead's merge can defeat).
func ExactSecondParentMerges(workdir, branch, tip string) ([]MergeCommit, error) {
	if err := rejectOptionLike(tip); err != nil {
		return nil, err
	}
	merges, err := FirstParentMerges(workdir, branch)
	if err != nil {
		return nil, err
	}
	var out []MergeCommit
	for _, m := range merges {
		if len(m.Parents) != 2 {
			continue
		}
		if m.Parents[1] == tip {
			out = append(out, m)
		}
	}
	return out, nil
}

// CommitAll stages all changes in workdir and commits with the given message.
// Used for auto-commits at lifecycle boundaries (spec-init, approvals) to ensure
// artifacts are on the branch before downstream worktrees branch from it.
// Returns nil if there are no changes to commit.
func CommitAll(workdir, message string) error {
	statusCmd := execCommand("git", "-C", workdir, "status", "--porcelain")
	statusOut, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("checking git status: %w", err)
	}
	if strings.TrimSpace(string(statusOut)) == "" {
		return nil // nothing to commit
	}

	addCmd := execCommand("git", "-C", workdir, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		// R4: git's CombinedOutput can echo back agent-writable repo content
		// (e.g. a hostile pathspec/filename) — escape per-line before it
		// reaches the error/terminal path.
		return fmt.Errorf("staging changes: %s", escapeLines(strings.TrimSpace(string(out))))
	}

	commitCmd := execCommand("git", "-C", workdir, "commit", "-m", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("committing: %s", escapeLines(strings.TrimSpace(string(out))))
	}

	return nil
}

// escapeLines applies termsafe.Escape to each line of a (possibly
// multi-line) block of agent-influenced text — git porcelain/error output —
// while preserving the real newlines that separate genuine lines (R4:
// per-line escaping for line-oriented bodies, never per-message, so a
// hostile line cannot forge additional lines while legitimate multi-line
// structure survives).
func escapeLines(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = termsafe.Escape(l)
	}
	return strings.Join(lines, "\n")
}

// File I/O wrappers for testability.
var (
	readFile  = os.ReadFile
	writeFile = os.WriteFile
)

// gitArgs builds an argv for `git`, optionally prefixed with `-C workdir`.
// When workdir is empty, no `-C` is added and git inherits the caller's cwd.
func gitArgs(workdir string, args ...string) []string {
	if workdir == "" {
		out := make([]string, len(args))
		copy(out, args)
		return out
	}
	out := make([]string, 0, len(args)+2)
	out = append(out, "-C", workdir)
	out = append(out, args...)
	return out
}

// --- read helpers ----------------------------------------------------------

// RevParseHEAD returns the HEAD commit SHA of workdir, trimmed.
func RevParseHEAD(workdir string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, "rev-parse", "HEAD")...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RevParseRef resolves an arbitrary ref (e.g. "bead/<id>") to its commit
// SHA in workdir, trimmed. Unlike RevParseHEAD it targets a named ref, so a
// missing ref returns an error (the panel gate reads this as the
// rerun-after-merge case where the bead branch was already deleted — Spec
// 093 Req 11 missing-ref pass-through). `^{commit}` peels annotated tags to
// their commit so the result is always comparable to a reviewed_head_sha.
func RevParseRef(workdir, ref string) (string, error) {
	if err := rejectOptionLike(ref); err != nil {
		return "", err
	}
	cmd := execCommand("git", gitArgs(workdir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")...)
	out, err := cmd.Output()
	if err != nil {
		// `--verify --quiet` exits 1 with empty output when the ref is simply
		// absent — the expected branch-already-deleted case. Any other exit
		// code (128 not-a-repo / git missing / lock contention) is a transient
		// or structural failure, which the caller must NOT treat as a confirmed
		// missing ref.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", fmt.Errorf("rev-parse %s: %w", ref, ErrRefNotFound)
		}
		return "", fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		// Empty output with a zero exit also means the ref did not resolve.
		return "", fmt.Errorf("rev-parse %s: %w", ref, ErrRefNotFound)
	}
	return sha, nil
}

// LogOneline returns `git log -1 --oneline <ref>` for workdir
// (workdir=="" → cwd), trimmed: the one-line "<short-sha> <subject>"
// summary of the tip commit of ref. The error is non-nil when ref does
// not resolve (e.g. a deleted branch); callers that only want a display
// string treat an error as "no detail available" (Spec 093 Req 14
// in-progress-beads last-commit line).
func LogOneline(workdir, ref string) (string, error) {
	if err := rejectOptionLike(ref); err != nil {
		return "", err
	}
	// Trailing `--` ensures a non-`-` but ref/path-ambiguous ref is parsed
	// as a revision, not a pathspec.
	cmd := execCommand("git", gitArgs(workdir, "log", "-1", "--oneline", ref, "--")...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("log -1 --oneline %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RevParseShowToplevel returns `git rev-parse --show-toplevel` from the
// current working directory. No `-C` is set.
func RevParseShowToplevel() (string, error) {
	cmd := execCommand("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsInsideWorkTree reports whether workdir is inside a git work tree.
// Returns false on any error (missing git, not a repo, bare repo).
func IsInsideWorkTree(workdir string) bool {
	cmd := execCommand("git", gitArgs(workdir, "rev-parse", "--is-inside-work-tree")...)
	return cmd.Run() == nil
}

// Status runs `git status --porcelain` in workdir (workdir=="" → cwd) and
// returns stdout. Use this when stderr is not interesting.
func Status(workdir string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, "status", "--porcelain")...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("status --porcelain: %w", err)
	}
	return string(out), nil
}

// StatusWithStderr runs `git status --porcelain` and uses CombinedOutput so
// stderr is preserved in the error on failure (e.g. missing `-C` target).
// On success the returned string is the combined output (which is stdout
// only when the command succeeds).
func StatusWithStderr(workdir string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, "status", "--porcelain")...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("status --porcelain: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// LsFiles runs `git ls-files <args...>` in workdir (workdir=="" → cwd) and
// returns stdout. Caller is responsible for adding `--` separators where
// untrusted paths are passed (SEC-5).
func LsFiles(workdir string, args ...string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, append([]string{"ls-files"}, args...)...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ls-files: %w", err)
	}
	return string(out), nil
}

// LsFilesErrorUnmatch runs `git ls-files --error-unmatch -- <file>`. Returns
// nil if the file is tracked; non-nil error otherwise (exit code 1 for
// untracked, other errors surface as-is).
func LsFilesErrorUnmatch(workdir, file string) error {
	cmd := execCommand("git", gitArgs(workdir, "ls-files", "--error-unmatch", "--", file)...)
	return cmd.Run()
}

// LsFilesFullName runs `git ls-files --full-name -- <file>` in workdir and
// returns stdout.
func LsFilesFullName(workdir, file string) (string, error) {
	cmd := execCommand("git", gitArgs(workdir, "ls-files", "--full-name", "--", file)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ls-files --full-name: %w", err)
	}
	return string(out), nil
}

// CheckIgnore runs `git check-ignore --quiet -- <file>`. Returns nil if the
// file is gitignored. Always uses `--` to separate refs from paths (SEC-5).
func CheckIgnore(workdir, file string) error {
	cmd := execCommand("git", gitArgs(workdir, "check-ignore", "--quiet", "--", file)...)
	return cmd.Run()
}

// DiffNameOnly returns the list of paths from `git diff --name-only base..head`
// (newline-trimmed, empty entries dropped). The base and head are joined as
// `base..head` matching DiffStat / CommitCount conventions.
func DiffNameOnly(workdir, base, head string) ([]string, error) {
	if err := rejectOptionLike(base); err != nil {
		return nil, err
	}
	if err := rejectOptionLike(head); err != nil {
		return nil, err
	}
	// Range operand: no `--` separator (see DiffStat).
	cmd := execCommand("git", gitArgs(workdir, "diff", "--name-only", base+".."+head)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diff --name-only %s..%s: %w", base, head, err)
	}
	return splitLines(string(out)), nil
}

// DiffNameOnlyRef runs `git diff --name-only <ref>` in workdir (single-ref
// form, comparing against the working tree). workdir=="" → cwd.
func DiffNameOnlyRef(workdir, ref string) ([]string, error) {
	if err := rejectOptionLike(ref); err != nil {
		return nil, err
	}
	// Trailing `--` separates the single ref from any pathspec
	// interpretation (single-ref form, not a range).
	cmd := execCommand("git", gitArgs(workdir, "diff", "--name-only", ref, "--")...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diff --name-only %s: %w", ref, err)
	}
	return splitLines(string(out)), nil
}

// DiffPathspec runs `git diff <base> <head> -- <pathspecs...>` and returns
// the raw diff text. The `--` separator is always inserted between refs
// and pathspecs.
func DiffPathspec(workdir, base, head string, pathspecs []string) (string, error) {
	if err := rejectOptionLike(base); err != nil {
		return "", err
	}
	if err := rejectOptionLike(head); err != nil {
		return "", err
	}
	args := []string{"diff", base, head, "--"}
	args = append(args, pathspecs...)
	cmd := execCommand("git", gitArgs(workdir, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("diff %s %s -- pathspec: %w", base, head, err)
	}
	return string(out), nil
}

// DiffQuiet runs `git diff --quiet` in workdir and returns the exit error.
// Nil means the tree is clean; non-nil means dirty.
func DiffQuiet(workdir string) error {
	cmd := execCommand("git", gitArgs(workdir, "diff", "--quiet")...)
	return cmd.Run()
}

// DiffCachedQuiet runs `git diff --cached --quiet` in workdir.
func DiffCachedQuiet(workdir string) error {
	cmd := execCommand("git", gitArgs(workdir, "diff", "--cached", "--quiet")...)
	return cmd.Run()
}

// --- mutating helpers ------------------------------------------------------

// Add runs `git add <args...>` in workdir.
//
// Note: this helper does NOT insert a `--` separator. Callers that pass
// untrusted path arguments must include `--` themselves. SEC-5 ref/path
// hardening will keep this contract.
func Add(workdir string, args ...string) error {
	cmd := execCommand("git", gitArgs(workdir, append([]string{"add"}, args...)...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CommitNoVerify runs `git commit -m <message> --no-verify` in workdir.
// Bypasses pre-commit / commit-msg hooks — used for synthetic commits
// (bench artifacts) where hooks would block deliberately.
func CommitNoVerify(workdir, message string) error {
	cmd := execCommand("git", gitArgs(workdir, "commit", "-m", message, "--no-verify")...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("commit --no-verify: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RmCached runs `git rm --cached -- <file>` in workdir.
func RmCached(workdir, file string) error {
	cmd := execCommand("git", gitArgs(workdir, "rm", "--cached", "--", file)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rm --cached: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// --- worktree helpers ------------------------------------------------------

// WorktreeAddDetach runs `git worktree add --detach <wtPath> <commit>` in workdir.
//
// R5 check-at-use (ADR-0042 §4, AC-11): wtPath is re-validated for
// symlink-aware containment under workdir immediately before the git
// invocation — the wrapper-level check, which covers every present AND
// future caller by construction (WorktreeAddDetach has zero non-test
// callers today, per the spec's grep-complete inventory).
func WorktreeAddDetach(workdir, wtPath, commit string) error {
	if err := rejectOptionLike(commit); err != nil {
		return err
	}
	if err := containment.CheckContainment(workdir, wtPath); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("refusing worktree add --detach: %v", err),
			containment.RejectionLever,
		)
	}
	cmd := execCommand("git", gitArgs(workdir, "worktree", "add", "--detach", wtPath, commit)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("worktree add --detach: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// WorktreeAdd runs `git worktree add <wtPath> <branch>` in workdir.
//
// R5 check-at-use (ADR-0042 §4, AC-11): wtPath is re-validated for
// symlink-aware containment under workdir immediately before the git
// invocation — the wrapper-level check covers every caller by
// construction, in addition to the named executor-level checks that
// already precede most calls (defense-in-depth, not a replacement).
func WorktreeAdd(workdir, wtPath, branch string) error {
	if err := rejectOptionLike(branch); err != nil {
		return err
	}
	if err := containment.CheckContainment(workdir, wtPath); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("refusing worktree add: %v", err),
			containment.RejectionLever,
		)
	}
	cmd := execCommand("git", gitArgs(workdir, "worktree", "add", wtPath, branch)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("worktree add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// WorktreeRemoveForce runs `git worktree remove --force <wtPath>` in workdir.
//
// R7 check-at-use (ADR-0042 check-at-use discipline, spec 121,
// mindspec-17bd): wtPath is re-validated for symlink-aware containment
// under workdir immediately before the git invocation — the same
// wrapper-level discipline WorktreeAdd/WorktreeAddDetach already carry
// (:895/:920), which spec 120's containment sweep scoped this one
// destructive wrapper out of. wtPath is ALSO the sole operand here (unlike
// the add wrappers, which gate a separate commit/branch operand and
// containment-check wtPath independently), so the option-like guard
// applies to it directly: an option-like wtPath is refused before it can
// reach git's argv at all. In-tree, plainly-named removals are
// byte-identical in behavior. The caller-level gate spec 120's final
// review added before the finalize self-heal's os.RemoveAll fallback
// (mindspec_executor.go, the code preceding its WorktreeRemoveForce call)
// remains — defense-in-depth, not a replacement — so this closes every
// OTHER present and future caller at the wrapper itself.
func WorktreeRemoveForce(workdir, wtPath string) error {
	if err := rejectOptionLike(wtPath); err != nil {
		return err
	}
	if err := containment.CheckContainment(workdir, wtPath); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("refusing worktree remove --force: %v", err),
			containment.RejectionLever,
		)
	}
	cmd := execCommand("git", gitArgs(workdir, "worktree", "remove", "--force", wtPath)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("worktree remove --force: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// WorktreePrune runs `git worktree prune` in workdir.
func WorktreePrune(workdir string) error {
	cmd := execCommand("git", gitArgs(workdir, "worktree", "prune")...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("worktree prune: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// --- checkout helpers ------------------------------------------------------

// CheckoutNewBranch runs `git checkout -b <branch>` in workdir.
func CheckoutNewBranch(workdir, branch string) error {
	if err := rejectOptionLike(branch); err != nil {
		return err
	}
	// Trailing `--` separates the new-branch operand from any pathspec
	// interpretation on the single-ref `checkout -b` form.
	cmd := execCommand("git", gitArgs(workdir, "checkout", "-b", branch, "--")...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("checkout -b %s: %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}

// --- layout-mover primitives (spec 106 Bead 3) ------------------------------
//
// These are NET-NEW git primitives the `migrate layout` mover stands on
// (R4 blocker 2). They live here, the ADR-0030 git-process I/O boundary, and
// are surfaced on the Executor interface so internal/layout reaches git THROUGH
// the executor seam rather than shelling out itself. Each ref/path operand is
// passed the SEC-5 RejectOptionLike guard before reaching git argv, and path
// operands additionally ride behind a trailing `--` so a path that begins with
// `-` cannot be reparsed as an option.

// GitMv runs `git mv -- <src> <dst>` in workdir — a history-preserving rename
// (the move's pure 100%-similarity step, so `git log --follow` and 3-way
// rename detection stay reliable). src and dst are repo-relative paths.
func GitMv(workdir, src, dst string) error {
	if err := rejectOptionLike(src); err != nil {
		return err
	}
	if err := rejectOptionLike(dst); err != nil {
		return err
	}
	cmd := execCommand("git", gitArgs(workdir, "mv", "--", src, dst)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git mv %s %s: %s", src, dst, strings.TrimSpace(string(out)))
	}
	return nil
}

// ResetHard runs `git reset --hard <ref>` in workdir — the mover's
// pre-publish rollback to the pre-run ref. ref is guarded (SEC-5); no `--`
// is appended because `git reset --hard <ref>` takes a commit operand, not a
// pathspec.
func ResetHard(workdir, ref string) error {
	if err := rejectOptionLike(ref); err != nil {
		return err
	}
	cmd := execCommand("git", gitArgs(workdir, "reset", "--hard", ref)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset --hard %s: %s", ref, strings.TrimSpace(string(out)))
	}
	return nil
}

// CleanForce runs `git clean -fd` in workdir — removes untracked files and
// directories left behind by a rolled-back mover run (paired with ResetHard).
// It does NOT pass `-x`, so gitignored runtime files are preserved.
func CleanForce(workdir string) error {
	cmd := execCommand("git", gitArgs(workdir, "clean", "-fd")...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clean -fd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CleanForcePaths runs `git clean -fd -- <paths...>` in workdir — the SCOPED
// counterpart of CleanForce. It removes untracked residue only UNDER the given
// repo-relative pathspecs, so a rolled-back mover run cannot delete
// user-untracked files OUTSIDE the move set (the mover scopes its rollback to
// its own touched roots — `.mindspec`, `project-docs`, `review`, etc.). Each
// path operand rides behind the `--` separator and the SEC-5 leading-`-` guard.
// A pathspec that matches nothing on disk is a no-op (git clean does not error
// on an absent pathspec). With no paths it is equivalent to CleanForce.
func CleanForcePaths(workdir string, paths []string) error {
	if len(paths) == 0 {
		return CleanForce(workdir)
	}
	for _, p := range paths {
		if err := rejectOptionLike(p); err != nil {
			return err
		}
	}
	args := append([]string{"clean", "-fd", "--"}, paths...)
	cmd := execCommand("git", gitArgs(workdir, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clean -fd -- %s: %s", strings.Join(paths, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

// CommitPaths stages the given repo-relative paths (`git add -- <paths...>`)
// and commits them with msg (`git commit -m <msg> --no-verify`) in workdir.
// When paths is empty it commits whatever is already staged (used for the
// pure-rename commit, where `git mv` already staged the rename). `--no-verify`
// bypasses the pre-commit hooks the deterministic migration must not trip.
// Returns nil silently when there is nothing staged to commit.
func CommitPaths(workdir, msg string, paths []string) error {
	if len(paths) > 0 {
		for _, p := range paths {
			if err := rejectOptionLike(p); err != nil {
				return err
			}
		}
		addArgs := append([]string{"add", "--"}, paths...)
		addCmd := execCommand("git", gitArgs(workdir, addArgs...)...)
		if out, err := addCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git add: %s", strings.TrimSpace(string(out)))
		}
	}
	// Nothing staged → nothing to commit (idempotent resume reaches here).
	if DiffCachedQuiet(workdir) == nil {
		return nil
	}
	cmd := execCommand("git", gitArgs(workdir, "commit", "-m", msg, "--no-verify")...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// LocalBranchRefs returns the short names of every local branch
// (`git for-each-ref --format=%(refname:short) refs/heads`) in workdir — the
// (1) local-refs source of the migrate-layout pre-flatten discovery scan
// (Req 11).
func LocalBranchRefs(workdir string) ([]string, error) {
	return forEachRefShort(workdir, "refs/heads")
}

// RemoteTrackingRefs returns the short names of every remote-tracking ref
// (`git for-each-ref --format=%(refname:short) refs/remotes`) in workdir —
// e.g. "origin/main", "fork/feature" — the (2) remote-tracking source of the
// migrate-layout discovery scan (Req 11). A non-origin remote prefix marks an
// external fork, which the precondition tolerates rather than blocks.
func RemoteTrackingRefs(workdir string) ([]string, error) {
	return forEachRefShort(workdir, "refs/remotes")
}

// LockedWorktreeBranches returns the short branch names checked out in LOCKED
// linked worktrees (parsed from `git worktree list --porcelain`). The
// migrate-layout precondition TOLERATES these (a locked agent worktree cannot
// be drained, only fingerprint-guarded at merge — Req 11), so they are excluded
// from the block-candidate set. Best-effort: returns nil on any git failure.
func LockedWorktreeBranches(workdir string) ([]string, error) {
	cmd := execCommand("git", gitArgs(workdir, "worktree", "list", "--porcelain")...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("worktree list --porcelain: %w", err)
	}
	var branches []string
	var curBranch string
	var curLocked bool
	flush := func() {
		if curLocked && curBranch != "" {
			branches = append(branches, curBranch)
		}
		curBranch = ""
		curLocked = false
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			curBranch = strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/")
		case line == "locked" || strings.HasPrefix(line, "locked "):
			curLocked = true
		}
	}
	flush()
	return branches, nil
}

// forEachRefShort runs `git for-each-ref --format=%(refname:short) <pattern>`
// and returns the trimmed, non-empty lines.
func forEachRefShort(workdir, pattern string) ([]string, error) {
	cmd := execCommand("git", gitArgs(workdir, "for-each-ref", "--format=%(refname:short)", pattern)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("for-each-ref %s: %w", pattern, err)
	}
	return splitLines(string(out)), nil
}

// splitLines splits s on '\n', trims each entry, and drops empty entries.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
