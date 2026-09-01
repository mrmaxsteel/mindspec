// Package executor defines the Executor interface — the boundary between
// enforcement logic (what must happen) and execution logic (how it happens).
//
// Enforcement packages (validate, approve, bead/gate, state) call Executor
// methods; they never perform git or workspace operations directly.
// This package must NOT import any enforcement package.
//
// Executor is the **git/process I/O boundary** for the enforcement packages
// (internal/{validate,approve,complete,state,phase}). `bd` access is OUT OF
// SCOPE for this boundary and lives behind internal/bead.
package executor

// Executor abstracts workspace lifecycle operations. Implementations include
// MindspecExecutor (local git+worktrees) and MockExecutor (testing).
//
// Terminology: "workspace" means an isolated working copy (git worktree in
// MindspecExecutor, no-op directory in MockExecutor). "Epic" is a group of beads
// (implementation tasks) belonging to a single spec lifecycle.
type Executor interface {
	// InitSpecWorkspace creates a workspace for spec authoring.
	// Creates branch, worktree, and ensures gitignore entries.
	InitSpecWorkspace(specID string) (WorkspaceInfo, error)

	// HandoffEpic notifies the execution layer that beads are ready for
	// dispatch. For MindspecExecutor this is a no-op (beads are already created
	// by the enforcement layer). Other executors may use this to schedule
	// work distribution.
	HandoffEpic(epicID, specID string, beadIDs []string) error

	// DispatchBead creates a workspace for a bead. The specID is provided
	// so the executor can branch from the spec branch; the branching
	// strategy is the executor's concern.
	DispatchBead(beadID, specID string) (WorkspaceInfo, error)

	// CompleteBead commits outstanding changes, merges the bead branch back
	// into the spec branch, removes the bead workspace, and deletes the
	// bead branch. If msg is non-empty, it is used as the commit message.
	//
	// overrideReason (spec 127 R4(b)) is the operator-supplied
	// `--allow-net-deletion "<reason>"` text, or "" when no override was
	// requested. It is passed straight through to this producer's R4
	// preflight (immediately before the bead→spec MergeInto): a non-empty
	// value lets the merge proceed even when the shared work-destruction
	// predicate found DestructionSuperseded/DestructionStaleDeletion/
	// DestructionEvidenceError. The CALLER (internal/complete) is
	// responsible for recording the override on durable metadata and the
	// friction journal AFTER this call returns nil — this method only
	// decides proceed-vs-refuse.
	//
	// resolveMerge (spec 127 R5(d)) is the operator-supplied
	// `--resolve-merge` flag, consulted at exactly ONE decision point:
	// when a merge is ALREADY in progress in the target worktree with its
	// index RESOLVED and staged (no conflicted files remain),
	// resolveMerge==true completes it via `git commit --no-edit` and
	// proceeds to the same cleanup a fresh merge success would reach;
	// resolveMerge==false still refuses in that same state, naming
	// --resolve-merge as the way to finish it (a plain re-run never
	// silently completes someone else's staged resolution). Every OTHER
	// merge-state disposition is IDENTICAL regardless of resolveMerge: no
	// merge in progress → attempt a fresh merge, and on conflict stop in
	// place (never abort, never checkout) rather than replaying today's
	// abort-and-refuse behavior — see resumeAwareMerge's doc comment
	// (internal/executor/merge_resumption.go) for the reasoning;
	// unmerged index entries present → refuse naming the resolution
	// steps (never a new merge, never a checkout, never an abort).
	CompleteBead(beadID, specBranch, msg, overrideReason string, resolveMerge bool) error

	// FinalizeEpic merges the spec branch to main (or pushes for PR),
	// cleans up all workspaces and branches for the spec lifecycle.
	//
	// lifecycleAllowSet (Spec 119 Bead 3, R6/P6) scopes BOTH bead-branch
	// enumerations below — the auto-merge leg and the worktree/branch
	// cleanup leg — to the finalizing spec's plan-declared,
	// lifecycle-classified bead IDs. It is computed by the ENFORCEMENT
	// layer (internal/approve, via the new internal/phase classifier)
	// and passed in as a fact, because this package may not import
	// internal/phase (the executor/enforcement boundary declared in this
	// package's doc comment above). A candidate bead/<id> branch is
	// admitted iff its id is a member of the set; a nil set is the
	// "not computed" sentinel and implementations MUST abort fail-closed
	// rather than silently skip or admit any bead/<id> candidate (AC-14)
	// — a caller with a genuinely empty scope must pass a non-nil,
	// zero-length slice.
	//
	// overrideReason (spec 127 R4(b)) is the same audited
	// `--allow-net-deletion "<reason>"` text CompleteBead documents above,
	// consulted at BOTH of this method's own R4 preflight sites (the
	// per-bead auto-merge loop and the direct spec→main merge).
	//
	// resolveMerge (spec 127 R5(d)) is the same `--resolve-merge` flag
	// CompleteBead documents above, consulted independently at BOTH of
	// this method's own merge sites (the per-bead auto-merge loop's spec
	// worktree, and the direct spec→main leg's main checkout) — only
	// whichever site actually has a preserved merge acts on it.
	FinalizeEpic(epicID, specID, specBranch string, lifecycleAllowSet []string, overrideReason string, resolveMerge bool) (FinalizeResult, error)

	// Cleanup removes stale workspaces and branches for a spec.
	// If force is true, skips lifecycle state checks.
	Cleanup(specID string, force bool) error

	// --- Query methods ---

	// IsTreeClean returns nil if the workspace at path has no uncommitted
	// changes, or an error describing the dirty files.
	IsTreeClean(path string) error

	// DiffStat returns a short diffstat summary between two refs.
	DiffStat(base, head string) (string, error)

	// CommitCount returns the number of commits between base and head.
	CommitCount(base, head string) (int, error)

	// CommitAll stages all changes in the workspace at path and commits
	// with the given message. No-op if the tree is clean.
	CommitAll(path, msg string) error

	// ChangedFiles returns the list of paths changed between two git refs.
	// Passing base == "" means working tree vs head, matching
	// gitutil.DiffNameOnlyRef("", ref) semantics. With both refs set it is
	// equivalent to `git diff --name-only <base>..<head>`.
	ChangedFiles(base, head string) ([]string, error)

	// FileAtRef returns the content of path at git ref (wraps
	// `git show <ref>:<path>`). Empty ref is undefined.
	FileAtRef(ref, path string) ([]byte, error)

	// FileAtRefOrAbsent returns the bytes of path at ref, DISTINGUISHING
	// a path absent from ref's (valid) tree from an operational git
	// failure. present is false with a nil error ONLY when ref is a
	// valid tree-ish that does not contain path; an invalid ref / git
	// failure returns a non-nil error. This is the seam the ref-anchored
	// OWNERSHIP loader (spec 095 / mindspec-vvs9) uses to keep
	// absent-→claims-nothing (ADR-0036) distinct from an operational
	// error, which must hard-fail rather than silently un-gate doc-drift.
	FileAtRefOrAbsent(ref, path string) (data []byte, present bool, err error)

	// TreeDirsAtRef returns the names of sub-directory (tree) entries
	// directly under dirPath in ref's tree (wraps `git ls-tree`). An
	// absent dirPath at a VALID ref yields an empty slice with a nil
	// error (mirroring listDomainDirs on a missing directory); an
	// invalid ref / git failure returns a non-nil error. The ref-aware
	// domain enumeration (spec 095) consumes this so a branch-only
	// domain directory is discovered from the diffed ref.
	TreeDirsAtRef(ref, dirPath string) ([]string, error)

	// MergeBase returns the merge-base SHA of refs a and b (wraps
	// `git merge-base <a> <b>`).
	MergeBase(a, b string) (string, error)

	// RevParseRef resolves ref to its commit SHA in workdir (wraps
	// `git rev-parse --verify <ref>^{commit}`). A genuinely-absent ref
	// returns an error satisfying IsRefNotFound; any other failure (not-a-
	// repo / lock contention) returns a non-ref-not-found error. This is the
	// git-I/O seam the in-binary panel gate (spec 099) uses for the bead/<id>
	// staleness rev-parse, keeping internal/complete off gitutil (ADR-0030).
	RevParseRef(workdir, ref string) (string, error)

	// Status returns `git status --porcelain` for workdir (the worktree
	// dirty-check seam for the panel gate). Empty output means a clean tree.
	Status(workdir string) (string, error)

	// IsRefNotFound reports whether err is the genuine "ref absent" case
	// (branch already deleted) from RevParseRef vs a transient/structural git
	// failure — the panel gate's missing-ref pass-through distinction. Routing
	// it through the executor keeps the gitutil.ErrRefNotFound sentinel out of
	// the enforcement packages (ADR-0030).
	IsRefNotFound(err error) bool

	// --- layout-mover git primitives (spec 106 Bead 3) ---
	//
	// These surface the net-new gitutil mover primitives on the executor
	// boundary so internal/layout drives the `migrate layout` transactional
	// mover THROUGH the executor (ADR-0030) instead of shelling out. They are
	// thin pass-throughs to gitutil in MindspecExecutor.

	// GitMv runs a history-preserving `git mv -- <src> <dst>` in workdir (the
	// pure 100%-similarity rename step of each move group).
	GitMv(workdir, src, dst string) error

	// ResetHard runs `git reset --hard <ref>` in workdir — the mover's
	// pre-publish rollback to the pre-run ref.
	ResetHard(workdir, ref string) error

	// CleanForce runs `git clean -fd` in workdir — removes the untracked
	// run-state/lineage residue after a rolled-back run (paired with ResetHard).
	CleanForce(workdir string) error

	// CleanForcePaths runs `git clean -fd -- <paths...>` in workdir — the
	// SCOPED clean the mover's rollback uses so it removes untracked residue
	// only under its own touched roots and cannot delete user-untracked files
	// outside the move set.
	CleanForcePaths(workdir string, paths []string) error

	// CommitPaths stages the given repo-relative paths and commits them with
	// msg in workdir (`--no-verify`). Empty paths commits whatever is already
	// staged (the pure-rename commit). A no-op when nothing is staged.
	CommitPaths(workdir, msg string, paths []string) error

	// LocalBranchRefs returns the short names of every local branch in workdir
	// — the local-refs source of the migrate-layout pre-flatten discovery scan.
	LocalBranchRefs(workdir string) ([]string, error)

	// RemoteTrackingRefs returns the short names of every remote-tracking ref
	// in workdir (e.g. "origin/main", "fork/feature") — the remote-tracking
	// source of the migrate-layout discovery scan.
	RemoteTrackingRefs(workdir string) ([]string, error)
}

// WorkspaceInfo describes a created workspace.
type WorkspaceInfo struct {
	Path   string // Absolute path to the workspace directory
	Branch string // Branch name checked out in the workspace
}

// FinalizeResult describes the outcome of epic finalization.
type FinalizeResult struct {
	MergeStrategy string // "pr", "direct", or "auto"
	CommitCount   int    // Number of commits merged
	DiffStat      string // Short diffstat summary
	PRURL         string // Pull request URL (empty for direct merge)

	// FinalizeBranch is bug wu7t's protected-main finalize carrier: the
	// name of a chore/finalize-<specID> branch (empty when unused) that
	// carries the epic-close JSONL export commit when the spec branch was
	// ALREADY merged into origin/main before `impl approve` ran (the
	// common already-merged-implementation-PR case — a spec branch is a
	// one-shot PR carrier, spec 101, so a second PR off it never gets
	// reviewed). When non-empty, main's committed .beads/issues.jsonl is
	// STALE until a PR from this branch merges — the bd post-merge hook
	// will keep reverting the epic-close/bead-done state in Dolt on every
	// merge/FF until then. Always empty on the no-remote "direct" path.
	FinalizeBranch string
}
