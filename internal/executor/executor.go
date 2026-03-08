// Package executor defines the Executor interface — the boundary between
// enforcement logic (what must happen) and execution logic (how it happens).
//
// Enforcement packages (validate, approve, bead/gate, state) call Executor
// methods; they never perform git or workspace operations directly.
// This package must NOT import any enforcement package.
package executor

// Executor abstracts workspace lifecycle operations. Implementations include
// GitExecutor (local git+worktrees) and MockExecutor (testing).
//
// Terminology: "workspace" means an isolated working copy (git worktree in
// GitExecutor, no-op directory in MockExecutor). "Epic" is a group of beads
// (implementation tasks) belonging to a single spec lifecycle.
type Executor interface {
	// InitSpecWorkspace creates a workspace for spec authoring.
	// Creates branch, worktree, and ensures gitignore entries.
	InitSpecWorkspace(specID string) (WorkspaceInfo, error)

	// HandoffEpic notifies the execution layer that beads are ready for
	// dispatch. For GitExecutor this is a no-op (beads are already created
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
	CompleteBead(beadID, specBranch, msg string) error

	// FinalizeEpic merges the spec branch to main (or pushes for PR),
	// cleans up all workspaces and branches for the spec lifecycle.
	FinalizeEpic(epicID, specID, specBranch string) (FinalizeResult, error)

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
}
