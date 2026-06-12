package next

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
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

// ClaimFailure formats the claim-failure recovery recipe (spec 093
// Req 3, replacing the bare "claiming bead: <err>" wrap and the
// pick+claim skill prose that was folded into ms-bead-cycle and
// ms-bead-impl per spec 093).
//
// The body carries the manual-claim recipe kept VERBATIM from the
// battle-tested skill text: `bd update <id> --claim --status
// in_progress` (--claim carries the atomic claim/assignee semantics
// that prevent two agents on one bead) plus the interpolated
// `git worktree add` line (workspace.BeadWorktreeName/BeadBranch/
// SpecWorktreePath). The final `recovery:` line is the re-run —
// `mindspec next --spec <slug>` — whose in-progress recovery path
// recreates the worktree automatically.
//
// A claim-less Dolt-1105 auto-fallback is explicitly DEFERRED
// (spec 093 Req 3, jkhd.5 validator batch); this failure message IS
// the 1105 handling. specID may be "" when no --spec flag was given
// and the bead title carried no spec slug; the recipe then falls back
// to placeholders and a plain `mindspec next` re-run.
func ClaimFailure(root string, cfg *config.Config, beadID, specID string, claimErr error) error {
	var b strings.Builder
	fmt.Fprintf(&b, "claiming bead %s: %v\n", beadID, claimErr)
	b.WriteString("if this is a bd event-recording failure (e.g. Dolt Error 1105 on large\n")
	b.WriteString("descriptions), claim manually:\n")
	fmt.Fprintf(&b, "  bd update %s --claim --status in_progress\n", beadID)
	fmt.Fprintf(&b, "  git -C %s worktree add %s -b %s %s",
		specWorktreeOrPlaceholder(root, cfg, specID),
		nestedBeadWorktreeRel(cfg, beadID),
		workspace.BeadBranch(beadID),
		specBranchOrPlaceholder(specID))
	if specID == "" {
		return guard.NewFailure(b.String(),
			"mindspec next   (re-run to auto-recover the worktree)")
	}
	return guard.NewFailure(b.String(),
		fmt.Sprintf("mindspec next --spec %s   (re-run to auto-recover the worktree)", specID))
}

// WorktreeSetupFailure formats the worktree-setup-failure recipe
// (spec 093 Req 4, replacing the bare "Warning: worktree setup failed"
// line that left the agent claimed-but-homeless).
//
// The body carries the concrete `git worktree add` recipe interpolated
// via workspace.BeadWorktreeName/BeadBranch/SpecWorktreePath, and the
// final `recovery:` line references the existing in-progress
// auto-recovery path: re-running `mindspec next --spec <slug>` detects
// the claimed bead with a missing worktree and recreates it.
//
// Behavior is unchanged at the call site (warn and continue): callers
// print the formatted failure to stderr — only the message routes
// through guard.NewFailure (HC-5).
func WorktreeSetupFailure(root string, cfg *config.Config, beadID, specID string, wtErr error) error {
	var b strings.Builder
	fmt.Fprintf(&b, "worktree setup failed: %v\n", wtErr)
	fmt.Fprintf(&b, "bead %s is claimed but has no worktree — create it manually:\n", beadID)
	fmt.Fprintf(&b, "  git -C %s worktree add %s -b %s %s",
		specWorktreeOrPlaceholder(root, cfg, specID),
		nestedBeadWorktreeRel(cfg, beadID),
		workspace.BeadBranch(beadID),
		specBranchOrPlaceholder(specID))
	if specID == "" {
		return guard.NewFailure(b.String(),
			"mindspec next   (re-run detects the in-progress bead and auto-recovers the worktree)")
	}
	return guard.NewFailure(b.String(),
		fmt.Sprintf("mindspec next --spec %s   (re-run detects the in-progress bead and auto-recovers the worktree)", specID))
}

// specWorktreeOrPlaceholder interpolates the spec worktree path for the
// Req 3/4 recipes, or a readable placeholder when the spec is unknown.
func specWorktreeOrPlaceholder(root string, cfg *config.Config, specID string) string {
	if specID == "" {
		return "<spec-worktree>"
	}
	return workspace.SpecWorktreePath(root, cfg, specID)
}

// specBranchOrPlaceholder interpolates the spec branch name, or a
// readable placeholder when the spec is unknown.
func specBranchOrPlaceholder(specID string) string {
	if specID == "" {
		return "<spec-branch>"
	}
	return workspace.SpecBranch(specID)
}

// nestedBeadWorktreeRel renders the bead worktree path relative to the
// spec worktree (the `git -C <spec-worktree>` target of the recipes),
// honoring cfg.WorktreeRoot.
func nestedBeadWorktreeRel(cfg *config.Config, beadID string) string {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return filepath.Join(cfg.WorktreeRoot, workspace.BeadWorktreeName(beadID))
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
