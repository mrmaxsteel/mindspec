package next

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// artifactPaths is the list of paths mindspec treats as co-managed build
// artifacts (ADR-0025). A dirty diff on these paths does not block workflow
// guards; user-authored dirt still does.
//
// Today the only artifact is .beads/issues.jsonl. Adding a future artifact
// (e.g. .beads/events.jsonl) is a one-line append.
var artifactPaths = []string{
	".beads/issues.jsonl",
}

// Package-level function variables for testability. Tests swap these to
// simulate porcelain output and to verify bd export is invoked when and only
// when an artifact path is dirty.
var (
	statusPorcelainFn = defaultStatusPorcelain
	exportBeadsFn     = bead.Export
)

func defaultStatusPorcelain(cwd string) (string, error) {
	// Use StatusWithStderr (CombinedOutput-based) so stderr is preserved on
	// failure — a missing `-C` target or corrupt index otherwise exits with
	// an opaque *ExitError whose message the caller cannot surface.
	out, err := gitutil.StatusWithStderr(cwd)
	if err != nil {
		return "", fmt.Errorf("checking working tree: %w", err)
	}
	return out, nil
}

// classifyDirty partitions a list of repo-relative dirty paths into co-managed
// artifact dirt (safe to ignore per ADR-0025) and user dirt (must block).
// Empty inputs are skipped.
func classifyDirty(paths []string) (artifactDirt, userDirt []string) {
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if isArtifactPath(p) {
			artifactDirt = append(artifactDirt, p)
		} else {
			userDirt = append(userDirt, p)
		}
	}
	return
}

func isArtifactPath(p string) bool {
	for _, a := range artifactPaths {
		if p == a {
			return true
		}
	}
	return false
}

// parsePorcelain extracts repo-relative paths from `git status --porcelain`
// (v1) output.
//
// Each non-empty line is "XY <path>" where XY is a 2-char status field.
// Rename/copy entries are "XY orig -> new"; the new path is what matters for
// classification. Quoted paths (for names with unusual characters) are not
// re-parsed — the mindspec-recognized artifact list contains only plain
// ASCII paths, so a quoted match would legitimately be user dirt.
func parsePorcelain(out string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = strings.TrimSpace(path[idx+4:])
		}
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// CheckDirtyTree decides whether a `mindspec next` claim may proceed from cwd,
// per ADR-0025.
//
// Flow:
//  1. Snapshot `git status --porcelain` at cwd.
//  2. If any artifact path is dirty, run `bd export` from repoRoot to
//     normalize the JSONL against stale throttled exports, then re-snapshot.
//  3. Return the user-authored dirt that remains. Callers treat a non-empty
//     slice as a blocking condition.
//
// repoRoot is the main repo root (where .beads/ lives). cwd is the working
// directory whose git status is authoritative — typically the same as
// repoRoot but callers are free to pass a worktree path.
func CheckDirtyTree(repoRoot, cwd string) (userDirt []string, err error) {
	_, userDirt, err = CheckDirtyTreeDetail(repoRoot, cwd)
	return userDirt, err
}

// DirtyTreeFailure formats the blocking user-dirt guard failure for
// `mindspec next` (spec 092 Reqs 8/12, mindspec-tjat).
//
// cwd is the directory the dirty-tree check evaluated (CheckDirtyTree
// snapshots `git status` at cwd, so the directory the agent is in IS
// the checked path). activeWorktree is the active spec/bead worktree
// (guard.ActiveWorktreePath) or "" when none is active.
//
// Message contract:
//   - the body names the dirty paths and warns the agent off touching
//     them — the dirt may be the HUMAN's work in progress, and the
//     pre-fix "discard them: git restore ." advice is exactly what the
//     wrong_directory_guard_recovery scenario forbids;
//   - the worktree-context line (workspace.ContextLine, Req 8) is the
//     last body line, preceding the final recovery line (Req 12
//     ordering);
//   - when an active worktree exists and cwd is outside it, the single
//     recovery command steers the agent there — the re-run's status
//     check then evaluates that worktree, not the dirty one here;
//   - otherwise (the dirt is in the agent's own claim location) the
//     recovery is a conditional commit — never stash, never restore,
//     never checkout (HC-5: no destructive semantics over state the
//     agent did not name).
func DirtyTreeFailure(cwd string, userDirt []string, activeWorktree string) error {
	var b strings.Builder
	b.WriteString("cannot claim work: the working tree has uncommitted user changes:\n")
	for _, p := range userDirt {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	b.WriteString("these may be the user's work in progress — do NOT stash, discard, or commit them on the user's behalf\n")
	b.WriteString("(.beads/issues.jsonl is auto-handled per ADR-0025 and never blocks)\n")
	b.WriteString(workspace.ContextLine(cwd, cwd))
	if activeWorktree != "" && !pathWithin(cwd, activeWorktree) {
		return guard.NewFailure(b.String(),
			fmt.Sprintf("cd %s && mindspec next", activeWorktree))
	}
	return guard.NewFailure(b.String(),
		"if these changes are yours, commit them (git add -A && git commit), then re-run: mindspec next")
}

// pathWithin reports whether dir is root or a descendant of root,
// comparing absolute paths.
func pathWithin(dir, root string) bool {
	d, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	return d == r || strings.HasPrefix(d, r+string(filepath.Separator))
}

// CheckDirtyTreeDetail is CheckDirtyTree with the residual artifact dirt
// exposed alongside the user dirt. The classification flow is identical;
// the extra return value is the artifact dirt that SURVIVES the `bd export`
// normalization (snapshot 2) — i.e. artifact content that genuinely differs
// from the last commit, not just a stale throttled export.
//
// Spec 092 Req 7 (mindspec-i4ad): `mindspec complete` consumes the residual
// artifact dirt to fold it into a follow-up `chore: sync beads artifact`
// commit instead of ignoring it, so the bead→spec merge operates on a
// genuinely clean tree. Per ADR-0025 the artifact list is explicit and small
// (today: .beads/issues.jsonl only).
//
// Note: bead.Export writes <repoRoot>/.beads/issues.jsonl (the path resolves
// relative to the export workdir), so callers checking a worktree should
// pass that worktree as repoRoot too — otherwise the normalization targets a
// different checkout than the one being status-checked.
func CheckDirtyTreeDetail(repoRoot, cwd string) (artifactDirt, userDirt []string, err error) {
	out, err := statusPorcelainFn(cwd)
	if err != nil {
		return nil, nil, err
	}
	artifactDirt, userDirt = classifyDirty(parsePorcelain(out))

	if len(artifactDirt) == 0 {
		return nil, userDirt, nil
	}

	if err := exportBeadsFn(repoRoot); err != nil {
		return nil, nil, fmt.Errorf("normalizing beads export: %w", err)
	}

	out2, err := statusPorcelainFn(cwd)
	if err != nil {
		return nil, nil, err
	}
	artifactDirt, userDirt = classifyDirty(parsePorcelain(out2))
	return artifactDirt, userDirt, nil
}
