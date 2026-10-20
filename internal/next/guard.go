package next

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// Package-level function variables for testability. Tests swap these to
// simulate porcelain output and to verify bd export is invoked when and only
// when an artifact path is dirty.
//
// statusPorcelainFn (newline `--porcelain`) backs CheckDirtyTree/
// CheckDirtyTreeDetail only — unchanged. statusPorcelainZFn (`-z`,
// round-1 panel G3/G6) backs CheckDirtyTreeProportional only. Both share
// exportBeadsFn for the artifact-normalization retry.
var (
	statusPorcelainFn  = defaultStatusPorcelain
	statusPorcelainZFn = defaultStatusPorcelainZ
	exportBeadsFn      = bead.Export
)

// entangleableClaimPaths is a TEST-INJECTABLE residual on top of
// ComputeEntangleablePaths' two DERIVED classes below (R5.1, spec 128
// #179). It stays empty in production; AC-8(iii)'s seam-injected refusal
// leg uses SetEntangleableClaimPathsForTest to add a path here so the
// block machinery stays exercised independent of the real, dynamic
// classes. A future claim step that writes some THIRD tracked path not
// covered by either derived class would need this residual appended for
// real, permanently — that append is what re-arms the block for it.
var entangleableClaimPaths []string

// EntangleableClaimPaths returns a copy of the current test-injectable
// residual (see entangleableClaimPaths above). It does NOT include the
// two dynamic classes ComputeEntangleablePaths derives per call — use
// ComputeEntangleablePaths for the full, honest enumeration a given
// repoRoot/specID pair produces (AC-8(ii)'s anti-drift pin).
func EntangleableClaimPaths() []string {
	out := make([]string, len(entangleableClaimPaths))
	copy(out, entangleableClaimPaths)
	return out
}

// SetEntangleableClaimPathsForTest swaps entangleableClaimPaths for a
// test's duration and returns a restore func. AC-8(iii)'s seam-injected
// refusal leg uses this to keep the block machinery under test
// independent of the two real, conditional dynamic classes
// ComputeEntangleablePaths derives (round-1 F1/G1): inject a path here,
// dirty it, and CheckDirtyTreeProportional must still refuse via the
// preserved DirtyTreeFailure contract.
func SetEntangleableClaimPathsForTest(paths []string) func() {
	orig := entangleableClaimPaths
	entangleableClaimPaths = paths
	return func() { entangleableClaimPaths = orig }
}

// ComputeEntangleablePaths returns the FULL R5.1 entanglement
// enumeration for one `mindspec next` worktree-creating claim, honest
// about the two conditional write classes the round-1 panel found
// missing from the original "exactly (a)(b)(c), all gitignored/exempt"
// claim (spec 128 Bead 7 round-1 panel, G1/F1/F1-R6):
//
//  1. `.gitignore` — DispatchBead's EnsureGitignoreEntry
//     (internal/executor/mindspec_executor.go) writes at
//     resolveAnchorRoot(specID): the target spec's WORKTREE if one
//     already exists on disk (the typical case — a bead only exists
//     under an already plan-approved, and therefore worktree-having,
//     spec), else repoRoot. claimAnchorRoot below replicates that exact
//     preference. The write is a no-op when cfg.WorktreeRoot's entry is
//     ALREADY present there (gitutil.HasGitignoreEntry is the SAME
//     presence check EnsureGitignoreEntry itself uses, so the two can
//     never diverge) — genuine only when absent.
//  2. The TARGET spec's two recording paths — `recording.EmitBeadMarker`
//     and `recording.AddBeadToPhase` (cmd/mindspec/next.go's claim-path
//     step 7.5) — append events.ndjson and read-modify-write
//     manifest.json under WHATEVER absolute directory
//     recording.EventsPath/ManifestPath resolve to. That resolution
//     goes through workspace.SpecDir, which ALSO prefers the spec
//     worktree's copy when one exists — but round-2 panel G2: SpecDir's
//     own worktree-existence probe is not cfg-aware (it hardcodes
//     `.worktrees` via DefaultWorktreesDir, never honoring a custom
//     cfg.WorktreeRoot), so under a custom worktree_root it can resolve
//     a DIFFERENT tree than claimAnchorRoot's cfg-aware preference does
//     for .gitignore. Rather than replicate (and now duplicate-drift)
//     that resolution, this function trusts recording's OWN answer
//     unconditionally and gates purely on CONTAINMENT (below) — correct
//     by construction no matter which tree SpecDir actually picked.
//     recording.enabled must be true; specID == "" omits this class
//     entirely — a conservative "unknown, so not claimed as entangled"
//     choice.
//
// BOTH classes are gated on CONTAINMENT (round-2 panel G1, a hard
// block): a class is included iff its RESOLVED absolute write target
// falls inside the CANONICAL top-level of cwd's OWN git working tree —
// never by comparing an anchor string to cwd. The prior
// samePath(anchor, cwd) lexical-equality gate was wrong two ways a real
// e2e fixture now pins: invoking `next` from a SUBDIRECTORY of the
// anchor tree disarmed every class (porcelain is toplevel-relative
// regardless of cwd — the exact reason workspace.FindRoot exists — so
// the guard was observing real dirt while every class silently went
// dark), and a symlink alias of the tree evaded it outright (a
// different lexical string resolving to the identical tree). toplevelOf
// resolves cwd's `git rev-parse --show-toplevel` through
// filepath.EvalSymlinks so a subdirectory invocation and a symlinked
// alias both resolve to the SAME canonical identity as the tree itself;
// containedRel resolves the write target the same way and reports its
// path relative to THAT toplevel (never relative to cwd — the old
// filepath.Rel(cwd, ...) base was ALSO wrong, since porcelain paths are
// toplevel-relative regardless of cwd, and would never have matched a
// subdirectory cwd's relative form anyway). A target outside the
// toplevel entirely is excluded, never forced into a leaking
// "../"-prefixed relative path.
//
// The `.mindspec/session.json` (gitignored) and bead-worktree-directory
// (gitignored, `.worktrees/`) classes from the original enumeration
// remain correctly excluded — they can never surface as tracked
// porcelain dirt regardless of config, so they need no runtime check
// here. The test-injectable residual (entangleableClaimPaths) is unioned
// in last, unconditionally (a test asserting a specific write class
// should not also have to fake toplevel/containment agreement).
func ComputeEntangleablePaths(repoRoot, cwd, specID string) []string {
	var paths []string
	cfg, cfgErr := config.Load(repoRoot)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}

	toplevel := toplevelOf(cwd)

	anchor := claimAnchorRoot(repoRoot, cfg, specID)
	if rel, ok := containedRel(toplevel, filepath.Join(anchor, ".gitignore")); ok && !gitutil.HasGitignoreEntry(anchor, cfg.WorktreeRoot) {
		paths = append(paths, rel)
	}

	if specID != "" && recording.IsEnabled(repoRoot) {
		if p, err := recording.EventsPath(repoRoot, specID); err == nil {
			if rel, ok := containedRel(toplevel, p); ok {
				paths = append(paths, rel)
			}
		}
		if p, err := recording.ManifestPath(repoRoot, specID); err == nil {
			if rel, ok := containedRel(toplevel, p); ok {
				paths = append(paths, rel)
			}
		}
	}

	paths = append(paths, entangleableClaimPaths...)
	return paths
}

// claimAnchorRoot replicates MindspecExecutor.resolveAnchorRoot's
// preference (internal/executor/mindspec_executor.go) without importing
// the executor package (internal/next stays on the enforcement side of
// the executor/enforcement boundary, ADR-0030): the target spec's
// worktree if one already exists on disk, else repoRoot. specID == ""
// or an invalid/nonexistent worktree path degrades to repoRoot, the same
// ambient-helper fallback resolveAnchorRoot itself uses.
func claimAnchorRoot(repoRoot string, cfg *config.Config, specID string) string {
	if specID == "" {
		return repoRoot
	}
	specWt, err := workspace.SpecWorktreePath(repoRoot, cfg, specID)
	if err != nil {
		return repoRoot
	}
	if fi, statErr := os.Stat(specWt); statErr == nil && fi.IsDir() {
		return specWt
	}
	return repoRoot
}

// toplevelOf resolves cwd's git working tree's CANONICAL top-level
// directory (round-2 panel G1): gitutil.Toplevel (`git rev-parse
// --show-toplevel`) — which resolves correctly from any subdirectory of
// the tree — followed by filepath.EvalSymlinks, so a symlink alias of
// the tree resolves to the SAME identity as the tree itself. A
// resolution failure (cwd not inside a git repo, or a transient git
// error) degrades to cwd unresolved, the pre-fix behavior, rather than
// erroring the whole guard.
func toplevelOf(cwd string) string {
	top, err := gitutil.Toplevel(cwd)
	if err != nil {
		// No git toplevel resolvable (e.g. a bare fixture directory in a
		// unit test) — fall back to cwd itself, but STILL run it through
		// the same EvalSymlinks pass below. Skipping that here would
		// compare an unresolved cwd against a resolved target
		// (evalSymlinksBestEffort always resolves what it can), turning
		// every path into a spurious "escapes the toplevel" mismatch
		// purely from symlink-vs-canonical lexical form (e.g. macOS
		// t.TempDir()'s /var/folders/... vs realpath's
		// /private/var/folders/...), not a real containment failure.
		top = cwd
	}
	if resolved, evalErr := filepath.EvalSymlinks(top); evalErr == nil {
		return resolved
	}
	return top
}

// containedRel resolves target (an absolute path that need not exist
// yet — recording.EventsPath's events.ndjson is a case in point: a
// FRESH file the coming claim is about to create) via
// evalSymlinksBestEffort and reports whether the result is CONTAINED
// within toplevel (equal to it or a descendant) — the round-2 panel G1
// replacement for the old samePath(anchor, cwd) lexical-equality gate.
// On success it returns target's path relative to toplevel in slash
// form, matching porcelain's own toplevel-relative convention; a target
// outside toplevel entirely reports ok=false rather than a leaking
// "../"-prefixed relative path.
func containedRel(toplevel, target string) (rel string, ok bool) {
	resolvedTarget := resolveContainingDirs(target)
	r, err := filepath.Rel(toplevel, resolvedTarget)
	if err != nil {
		return "", false
	}
	if r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(r), true
}

// resolveContainingDirs resolves symlinks in p's DIRECTORY chain only —
// walking up to the nearest existing ancestor (a fresh recording.
// EventsPath target's own `recording/` directory may not exist yet
// either — StartRecording's os.MkdirAll only runs once recording.enabled
// is toggled on, which the caller's fixture may not have exercised) —
// and rejoins p's own basename UNRESOLVED, even when that basename is
// itself a symlink. This is deliberate, not an oversight: a bug hunted
// down by this bead's OWN "symlinked .gitignore" e2e fixture. `git
// status` reports a symlinked .gitignore under its OWN tracked name,
// ".gitignore" — never the name of whatever it points to — so resolving
// the leaf itself here would compute a containedRel result
// (e.g. "gitignore-target.txt") that can never match that porcelain
// name, silently un-entangling a genuinely dirty, symlinked .gitignore.
// Resolving only the ANCESTOR directories still correctly normalizes a
// symlink ALIAS of the observed tree (this function's actual job) while
// leaving the tracked leaf name intact for porcelain matching. Reaching
// the filesystem root without finding an existing ancestor degrades to
// the raw, unresolved path — the ambient-helper degrade convention this
// package already uses elsewhere (e.g. claimAnchorRoot's fallback to
// repoRoot).
func resolveContainingDirs(p string) string {
	dir := filepath.Dir(p)
	suffix := []string{filepath.Base(p)}
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return p
		}
		suffix = append(suffix, filepath.Base(dir))
		dir = parent
	}
}

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

// defaultStatusPorcelainZ is statusPorcelainZFn's default implementation
// — the NUL-terminated `-z` form CheckDirtyTreeProportional parses
// (round-1 panel G3/G6). CheckDirtyTree/CheckDirtyTreeDetail keep using
// the newline form above unchanged.
func defaultStatusPorcelainZ(cwd string) (string, error) {
	out, err := gitutil.StatusZWithStderr(cwd)
	if err != nil {
		return "", fmt.Errorf("checking working tree: %w", err)
	}
	return out, nil
}

// classifyDirty partitions a list of repo-relative dirty paths into co-managed
// artifact dirt (safe to ignore per ADR-0025) and user dirt (must block).
// Empty inputs are skipped.
func classifyDirty(paths []string, specsDir ...string) (artifactDirt, userDirt []string) {
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if isArtifactPath(p, specsDir...) {
			artifactDirt = append(artifactDirt, p)
		} else {
			userDirt = append(userDirt, p)
		}
	}
	return
}

// escapeLines applies termsafe.Escape to each line of a (possibly
// multi-line) block of agent-influenced text — a wrapped bd/git error
// message — while preserving the real newlines that separate genuine
// lines (R4: per-line escaping for line-oriented bodies, never
// per-message, so a hostile line cannot forge additional lines while
// legitimate multi-line structure survives).
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

func isArtifactPath(p string, specsDir ...string) bool {
	dir := ""
	if len(specsDir) > 0 {
		dir = specsDir[0]
	}
	return panel.IsToolArtifactPath(p, dir)
}

// parsePorcelain extracts repo-relative paths from `git status --porcelain`
// (v1) output.
//
// Each non-empty line is "XY <path>" where XY is a 2-char status field.
// Rename/copy entries are "XY orig -> new"; the new path is what matters for
// classification. Git C-style quoted paths are decoded before classification,
// including non-ASCII bytes escaped by core.quotePath.
func parsePorcelain(out string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		path, _ := panel.PorcelainPath(line)
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

// dirtyEntry is one parsed `git status --porcelain -z` record. Path is
// the current path (the ONLY path for anything but a rename/copy);
// OrigPath is non-empty only for rename/copy records (status containing
// 'R' or 'C'), carrying the path the entry moved/copied FROM. The `-z`
// form gives each field a literal NUL terminator rather than the
// newline form's " -> "-joined single string, so a rename's two paths
// are never ambiguous to split (round-1 panel G3/G6) and no path needs
// quote-escaping for embedded whitespace.
type dirtyEntry struct {
	Status   string
	Path     string
	OrigPath string
}

// dirCollapsed reports whether Path is a collapsed untracked-directory
// entry (git's default untracked-files=normal mode reports an entirely
// untracked directory as ONE such entry, "?? dir/", rather than
// enumerating its contents).
func (e dirtyEntry) dirCollapsed() bool {
	return strings.HasSuffix(e.Path, "/")
}

// parsePorcelainZ parses `git status --porcelain -z` output (NUL-
// terminated records; a rename/copy record is "XY PATH\0ORIG_PATH\0" —
// the NEW path first, then the original — verified against real git
// output, spec 128 Bead 7 round-1 fix).
func parsePorcelainZ(out string) []dirtyEntry {
	if out == "" {
		return nil
	}
	fields := strings.Split(out, "\x00")
	var entries []dirtyEntry
	for i := 0; i < len(fields); {
		f := fields[i]
		i++
		if f == "" {
			continue
		}
		if len(f) < 4 {
			continue
		}
		status := f[:2]
		path := f[3:]
		e := dirtyEntry{Status: status, Path: path}
		if strings.ContainsAny(status, "RC") && i < len(fields) {
			e.OrigPath = fields[i]
			i++
		}
		entries = append(entries, e)
	}
	return entries
}

// classifyDirtyZ partitions parsed -z entries into co-managed artifact
// dirt and user dirt, matching classifyDirty's exact-new-path-only rule
// (a rename's OrigPath is not consulted for artifact classification —
// unchanged scope from the pre-round-1 behavior; only entanglement
// matching below considers both sides of a rename).
func classifyDirtyZ(entries []dirtyEntry, specsDir ...string) (artifactEntries, userEntries []dirtyEntry) {
	for _, e := range entries {
		if isArtifactPath(e.Path, specsDir...) {
			artifactEntries = append(artifactEntries, e)
		} else {
			userEntries = append(userEntries, e)
		}
	}
	return
}

// entryIntersectsEntangleable reports whether e collides with any path
// in entangleable — checking BOTH sides of a rename/copy (round-1 panel
// G3/G6: "a rename/copy must block when either source or destination
// intersects an entangleable path") and, for a collapsed untracked
// directory, whether the directory contains (is an ancestor of) an
// entangleable path.
func entryIntersectsEntangleable(e dirtyEntry, entangleable []string) bool {
	if pathIntersectsEntangleable(e.Path, e.dirCollapsed(), entangleable) {
		return true
	}
	if e.OrigPath != "" && pathIntersectsEntangleable(e.OrigPath, false, entangleable) {
		return true
	}
	return false
}

func pathIntersectsEntangleable(p string, dirCollapsed bool, entangleable []string) bool {
	for _, want := range entangleable {
		if p == want {
			return true
		}
		if dirCollapsed && strings.HasPrefix(want, p) {
			return true
		}
	}
	return false
}

// renderDirtyEntry formats e for a human-facing message: a rename/copy
// renders as "old -> new" (git's own convention, and unambiguous here
// since DirtyTreeFailure places one entry per line and IgnoredDirtNotice
// quotes every entry — see below); anything else renders as its bare
// path.
func renderDirtyEntry(e dirtyEntry) string {
	if e.OrigPath != "" {
		return e.OrigPath + " -> " + e.Path
	}
	return e.Path
}

// CheckDirtyTreeProportional narrows the dirty-tree guard to R5.1's
// derived entanglement rule (spec 128, #179 — the ADR-0025 clarifying
// amendment, scoped to the worktree-creating `mindspec next` claim path
// ONLY; every other dirty-tree guard, including `mindspec spec
// approve`'s and `mindspec release`'s, is unaffected and keeps calling
// CheckDirtyTree/CheckDirtyTreeDetail directly, unchanged).
//
// specID is the caller's best-known RESOLVED target spec (possibly ""
// when next.go could not determine one before the guard runs) — it
// feeds ComputeEntangleablePaths' per-spec recording-path class
// (round-1 panel G1/F1).
//
// The artifact/user-dirt classification itself is unchanged (ADR-0025);
// this reclassifies what happens to the USER-dirt half: dirt on a path
// ComputeEntangleablePaths names is blockingDirt; every other user dirt
// is ignoredDirt, so the caller can print an informational notice naming
// it (Max's standing directive, 2026-09-02: visibility without the
// block) while PROCEEDING with the claim.
func CheckDirtyTreeProportional(repoRoot, cwd, specID string) (blockingDirt, ignoredDirt []string, err error) {
	entangleable := ComputeEntangleablePaths(repoRoot, cwd, specID)

	out, err := statusPorcelainZFn(cwd)
	if err != nil {
		return nil, nil, err
	}
	artifactEntries, userEntries := classifyDirtyZ(parsePorcelainZ(out), workspace.SpecsDir(cwd))

	if len(artifactEntries) > 0 {
		if err := exportBeadsFn(repoRoot); err != nil {
			return nil, nil, fmt.Errorf("normalizing beads export: %w", err)
		}
		out2, err := statusPorcelainZFn(cwd)
		if err != nil {
			return nil, nil, err
		}
		_, userEntries = classifyDirtyZ(parsePorcelainZ(out2), workspace.SpecsDir(cwd))
	}

	for _, e := range userEntries {
		if entryIntersectsEntangleable(e, entangleable) {
			blockingDirt = append(blockingDirt, renderDirtyEntry(e))
		} else {
			ignoredDirt = append(ignoredDirt, renderDirtyEntry(e))
		}
	}
	return blockingDirt, ignoredDirt, nil
}

// IgnoredDirtNotice formats the one-line informational stderr notice
// naming unrelated dirty user paths a worktree-creating claim IGNORED
// (Max's standing directive, 2026-09-02): the claim proceeds despite
// this dirt, but the removal of the old block must not also remove
// visibility into what it ignored. Callers print this only when
// ignoredDirt is non-empty — never on a clean tree.
//
// Each entry is agent-writable porcelain text (same provenance as
// DirtyTreeFailure's dirty-path lines), so it is rendered via
// strconv.Quote UNCONDITIONALLY (round-1 panel G3/G5/G6's "unambiguous
// notice boundaries" ask) rather than termsafe.Escape's printable-ASCII-
// passthrough rule: this notice joins multiple entries on ONE line with
// ", ", so a hostile path containing a literal ", " could otherwise
// masquerade as a boundary between two DIFFERENT paths — Quote's
// internal backslash-escaping of `"` keeps every entry self-delimiting
// regardless of what it contains, closing both the control-byte/line-
// forging risk termsafe.Escape defends against AND this list's
// additional multi-entry ambiguity risk.
func IgnoredDirtNotice(ignoredDirt []string) string {
	quoted := make([]string, len(ignoredDirt))
	for i, p := range ignoredDirt {
		quoted[i] = strconv.Quote(p)
	}
	return fmt.Sprintf("note: claiming despite unrelated dirty path(s) this claim does not touch (not blocked — ADR-0025 R5.1): %s", strings.Join(quoted, ", "))
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
		// R4: each porcelain entry is an agent-writable filename — escape
		// per-line so a hostile filename cannot forge extra lines or
		// control bytes into the terminal-facing message.
		fmt.Fprintf(&b, "  %s\n", termsafe.Escape(p))
	}
	b.WriteString("these may be the user's work in progress — do NOT stash, discard, or commit them on the user's behalf\n")
	b.WriteString("(.beads/issues.jsonl is auto-handled per ADR-0025 and never blocks)\n")
	b.WriteString(workspace.ContextLine(cwd, cwd))
	if activeWorktree != "" && !pathWithin(cwd, activeWorktree) {
		return guard.NewFailure(b.String(),
			fmt.Sprintf("%s && mindspec next", containment.EmitCd(activeWorktree)))
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
	// R4: beadID/specID are ID-typed positions (idrender.Bead/idrender.Spec);
	// claimErr can carry agent-influenced bd porcelain/error text, so it is
	// escaped per-line. workspace.BeadBranch/specBranchOrPlaceholder emit
	// spine-validated `bead/<id>`/`spec/<id>` branch operands, which stay
	// RAW; specWorktreeOrPlaceholder/nestedBeadWorktreeRel are the `git -C`
	// path operands of a copy-paste recipe (same convention as the cd
	// emitter), also RAW.
	var b strings.Builder
	fmt.Fprintf(&b, "claiming bead %s: %s\n", idrender.Bead(beadID), escapeLines(fmt.Sprintf("%v", claimErr)))
	b.WriteString("if this is a bd event-recording failure (e.g. Dolt Error 1105 on large\n")
	b.WriteString("descriptions), claim manually:\n")
	fmt.Fprintf(&b, "  bd update %s --claim --status in_progress\n", idrender.Bead(beadID))
	fmt.Fprintf(&b, "  git -C %s worktree add %s -b %s %s",
		specWorktreeOrPlaceholder(root, cfg, specID),
		nestedBeadWorktreeRel(cfg, beadID),
		beadBranchOrPlaceholder(beadID),
		specBranchOrPlaceholder(specID))
	if specID == "" {
		return guard.NewFailure(b.String(),
			"mindspec next   (re-run to auto-recover the worktree)")
	}
	return guard.NewFailure(b.String(),
		fmt.Sprintf("mindspec next --spec %s   (re-run to auto-recover the worktree)", idrender.Spec(specID)))
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
	// R4: beadID/specID are ID-typed positions (idrender.Bead/idrender.Spec);
	// wtErr can carry agent-influenced text, so it is escaped per-line —
	// same convention as ClaimFailure above.
	var b strings.Builder
	fmt.Fprintf(&b, "worktree setup failed: %s\n", escapeLines(fmt.Sprintf("%v", wtErr)))
	fmt.Fprintf(&b, "bead %s is claimed but has no worktree — create it manually:\n", idrender.Bead(beadID))
	fmt.Fprintf(&b, "  git -C %s worktree add %s -b %s %s",
		specWorktreeOrPlaceholder(root, cfg, specID),
		nestedBeadWorktreeRel(cfg, beadID),
		beadBranchOrPlaceholder(beadID),
		specBranchOrPlaceholder(specID))
	if specID == "" {
		return guard.NewFailure(b.String(),
			"mindspec next   (re-run detects the in-progress bead and auto-recovers the worktree)")
	}
	return guard.NewFailure(b.String(),
		fmt.Sprintf("mindspec next --spec %s   (re-run detects the in-progress bead and auto-recovers the worktree)", idrender.Spec(specID)))
}

// specWorktreeOrPlaceholder interpolates the spec worktree path for the
// Req 3/4 recipes, or a readable placeholder when the spec is unknown OR
// fails idvalidate (ADR-0042 composition waist — a malformed specID must
// never reach a composed path in a rendered recipe).
func specWorktreeOrPlaceholder(root string, cfg *config.Config, specID string) string {
	if specID == "" {
		return "<spec-worktree>"
	}
	if p, err := workspace.SpecWorktreePath(root, cfg, specID); err == nil {
		return p
	}
	return "<spec-worktree>"
}

// specBranchOrPlaceholder interpolates the spec branch name, or a
// readable placeholder when the spec is unknown OR fails idvalidate.
func specBranchOrPlaceholder(specID string) string {
	if specID == "" {
		return "<spec-branch>"
	}
	if b, err := workspace.SpecBranch(specID); err == nil {
		return b
	}
	return "<spec-branch>"
}

// beadBranchOrPlaceholder interpolates the bead branch name, or a
// readable placeholder when beadID fails idvalidate.
func beadBranchOrPlaceholder(beadID string) string {
	if b, err := workspace.BeadBranch(beadID); err == nil {
		return b
	}
	return "<bead-branch>"
}

// nestedBeadWorktreeRel renders the bead worktree path relative to the
// spec worktree (the `git -C <spec-worktree>` target of the recipes),
// honoring cfg.WorktreeRoot. Falls back to a placeholder basename when
// beadID fails idvalidate.
func nestedBeadWorktreeRel(cfg *config.Config, beadID string) string {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	name, err := workspace.BeadWorktreeName(beadID)
	if err != nil {
		name = "<bead-worktree>"
	}
	return filepath.Join(cfg.WorktreeRoot, name)
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
// (panel.ToolArtifactPaths).
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
	artifactDirt, userDirt = classifyDirty(parsePorcelain(out), workspace.SpecsDir(cwd))

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
	artifactDirt, userDirt = classifyDirty(parsePorcelain(out2), workspace.SpecsDir(cwd))
	return artifactDirt, userDirt, nil
}
