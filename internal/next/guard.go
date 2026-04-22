package next

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
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
	cmd := exec.Command("git", "-C", cwd, "status", "--porcelain")
	// CombinedOutput (not Output) so stderr is preserved on failure — a
	// missing `-C` target or corrupt index otherwise exits with an opaque
	// *ExitError whose message the caller cannot surface.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("checking working tree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
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
	out, err := statusPorcelainFn(cwd)
	if err != nil {
		return nil, err
	}
	artifactDirt, userDirt := classifyDirty(parsePorcelain(out))

	if len(artifactDirt) == 0 {
		return userDirt, nil
	}

	if err := exportBeadsFn(repoRoot); err != nil {
		return nil, fmt.Errorf("normalizing beads export: %w", err)
	}

	out2, err := statusPorcelainFn(cwd)
	if err != nil {
		return nil, err
	}
	_, userDirt = classifyDirty(parsePorcelain(out2))
	return userDirt, nil
}
