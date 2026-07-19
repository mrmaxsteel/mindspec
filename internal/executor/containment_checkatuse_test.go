package executor

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// TestContainmentCheckAtUse is AC-11: the containment predicate re-runs
// immediately before each USE of a composed worktree path, and a
// composition-time-clean path that has its ancestor swapped for an
// outside-pointing symlink (modelling the TOCTOU race) is refused at the
// use site — the git/mkdir operation is never attempted.
func TestContainmentCheckAtUse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires POSIX symlink semantics")
	}

	t.Run("composition-time clean path passes", func(t *testing.T) {
		g, fake, _ := newRepoExecutor(t)
		if _, err := g.InitSpecWorkspace("077-clean"); err != nil {
			t.Fatalf("unexpected error on a clean repo: %v", err)
		}
		if len(fake.createCalls) != 1 {
			t.Fatalf("WorktreeOps.Create calls = %d, want 1 on the clean path", len(fake.createCalls))
		}
	})

	// WorktreeOps.Create-path subtest (G3's site): mindspec_executor.go's
	// InitSpecWorkspace is the PRIMARY spec-worktree create path. Swap
	// its .worktrees ancestor for an outside symlink AFTER the repo is
	// set up but BEFORE InitSpecWorkspace runs — a hostile-ancestor
	// composed path must never reach WorktreeOps.Create.
	t.Run("WorktreeOps.Create pins G3's site: hostile ancestor never reaches Create", func(t *testing.T) {
		g, fake, dir := newRepoExecutor(t)
		outside := t.TempDir()
		wtRoot := filepath.Join(dir, ".worktrees")
		if err := os.Symlink(outside, wtRoot); err != nil {
			t.Fatalf("os.Symlink: %v", err)
		}

		_, err := g.InitSpecWorkspace("077-swapped")
		if err == nil {
			t.Fatal("expected InitSpecWorkspace to REFUSE when .worktrees is a symlink escaping the root")
		}
		if len(fake.createCalls) != 0 {
			t.Errorf("WorktreeOps.Create must NEVER be called on a hostile-ancestor composed path; got %d calls", len(fake.createCalls))
		}
	})

	t.Run("DispatchBead refuses when the bead worktree-root ancestor is swapped", func(t *testing.T) {
		g, fake, dir := newRepoExecutor(t)
		outside := t.TempDir()
		wtRoot := filepath.Join(dir, ".worktrees")
		if err := os.Symlink(outside, wtRoot); err != nil {
			t.Fatalf("os.Symlink: %v", err)
		}

		_, err := g.DispatchBead("mindspec-abc.1", "")
		if err == nil {
			t.Fatal("expected DispatchBead to REFUSE when .worktrees is a symlink escaping the root")
		}
		if len(fake.createCalls) != 0 {
			t.Errorf("WorktreeOps.Create must NEVER be called on a hostile-ancestor composed path; got %d calls", len(fake.createCalls))
		}
	})

	t.Run("direct CheckContainment refuses a post-check symlink swap", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		composed := filepath.Join(root, ".worktrees", "worktree-spec-x")

		// Passes before the swap (no .worktrees entry yet — nearest
		// existing ancestor is root itself).
		if err := containment.CheckContainment(root, composed); err != nil {
			t.Fatalf("unexpected rejection before the swap: %v", err)
		}

		// Model the race: something replaces .worktrees with an
		// outside-pointing symlink between the composition-time check
		// and the actual use.
		if err := os.Symlink(outside, filepath.Join(root, ".worktrees")); err != nil {
			t.Fatalf("os.Symlink: %v", err)
		}

		if err := containment.CheckContainment(root, composed); err == nil {
			t.Fatal("expected the check-at-use re-check to REFUSE after the ancestor swap")
		}
	})

	// The grep-complete set-equality companion (round-4 G3): a
	// regex-based source scan pinning that every non-test
	// WorktreeOps.Create / gitutil.WorktreeAdd / gitutil.WorktreeAddDetach
	// / composed-path os.Chdir / os.MkdirAll call site named in the
	// spec's inventory carries a containment.CheckContainment (direct or
	// via the checkWorktreeContainment helper) call in its immediate
	// vicinity. This is a heuristic line-window scan, not full go/ast
	// call-graph analysis (that lint-grade tool is a distinct, larger
	// undertaking) — but it goes RED if a pinned gate is deleted, or if
	// a new sink line is added to a pinned file without a nearby gate,
	// which is the regression this companion assertion exists to catch.
	t.Run("grep-complete: every pinned inventory site carries a nearby containment check", func(t *testing.T) {
		repoRoot := findRepoRootForTest(t)

		type site struct {
			relPath      string // relative to repoRoot
			sinkPattern  string // regex identifying the sink call
			gateLookback int    // lines to look back (and at the same line) for a gate call
		}
		gatePattern := regexp.MustCompile(`checkWorktreeContainment\(|containment\.CheckContainment\(`)

		sites := []site{
			{"internal/executor/mindspec_executor.go", `os\.MkdirAll\(wtRootPath`, 8},
			{"internal/executor/mindspec_executor.go", `g\.WorktreeOps\.Create\(relWtPath, specBranch\)`, 8},
			{"internal/executor/mindspec_executor.go", `os\.MkdirAll\(anchorWtRootPath`, 8},
			{"internal/executor/mindspec_executor.go", `return g\.WorktreeOps\.Create\(relWtPath, branchName\)`, 8},
			{"internal/executor/mindspec_executor.go", `os\.MkdirAll\(filepath\.Dir\(wtPath\)`, 8},
			{"internal/executor/mindspec_executor.go", `gitutil\.WorktreeAdd\(g\.Root, wtPath, choreBranch\)`, 8},
			{"internal/gitutil/gitops.go", `execCommand\("git", gitArgs\(workdir, "worktree", "add", "--detach"`, 8},
			{"internal/gitutil/gitops.go", `execCommand\("git", gitArgs\(workdir, "worktree", "add", wtPath, branch\)`, 8},
			{"cmd/mindspec/impl.go", `os\.Chdir\(specWtPath\)`, 8},
			{"cmd/mindspec/complete.go", `os\.Chdir\(specWtPath\)`, 8},
		}

		for _, s := range sites {
			content, err := os.ReadFile(filepath.Join(repoRoot, s.relPath))
			if err != nil {
				t.Fatalf("reading %s: %v", s.relPath, err)
			}
			lines := strings.Split(string(content), "\n")
			sinkRe := regexp.MustCompile(s.sinkPattern)

			found := false
			for i, line := range lines {
				if !sinkRe.MatchString(line) {
					continue
				}
				found = true
				start := i - s.gateLookback
				if start < 0 {
					start = 0
				}
				window := strings.Join(lines[start:i+1], "\n")
				if !gatePattern.MatchString(window) {
					t.Errorf("%s:%d matches sink pattern %q but has no containment check within %d lines above it",
						s.relPath, i+1, s.sinkPattern, s.gateLookback)
				}
			}
			if !found {
				t.Errorf("%s: sink pattern %q not found — the pinned inventory site may have moved or been removed", s.relPath, s.sinkPattern)
			}
		}
	})

	// The DISCOVERY direction (F/codex panel finding on the R5 fix-up
	// round): the grep-complete subtest above only checks ONE direction
	// — that every PINNED site still carries a nearby gate. It says
	// nothing about the REVERSE: a brand-new create/chdir/mkdir site
	// appearing elsewhere in the tree, on a composed worktree path,
	// that nobody ever added to the inventory (and so never gated).
	// This subtest closes that gap. It does NOT consult the pinned
	// list while scanning — it independently DISCOVERS every
	// composed-worktree-path USE site in cmd/+internal by walking the
	// real source, then asserts the discovered set is IDENTICAL to the
	// pinned inventory (set-equality): an un-pinned discovery is RED,
	// and a pinned site the scan no longer finds (moved/renamed/
	// deleted) is also RED. Together with the subtest above, AC-11 is
	// now genuinely two-way.
	//
	// Second-round fix-up (codex finding, verified test-completeness-only:
	// the functional gate was already correct): the scan below ALSO
	// recognizes withWorkingDir(<arg>) — mindspec_executor.go's chdir
	// wrapper whose own body performs the real os.Chdir — as a
	// composed-path chdir site. The wt-name heuristic alone can't see
	// these calls: neither "withWorkingDir" nor its "anchorRoot"/"g.Root"
	// argument identifiers contain "wt". A dedicated classifier inspects
	// the call's first argument directly instead: "g.Root" is the
	// trusted repo root (excluded, same reasoning as a plain
	// os.Chdir(g.Root) — no containment gate is needed to chdir to the
	// repo's own root) and anything else is a composed path that must be
	// in the pinned inventory like any other sink.
	t.Run("discovery: independently-found composed-worktree-path sites equal the pinned inventory (set-equality)", func(t *testing.T) {
		repoRoot := findRepoRootForTest(t)

		// Sink categories a composed worktree path is ever handed to,
		// per AC-11's own wording: the low-level git-worktree-ADD spawn
		// inside gitutil itself, the gitutil.WorktreeAdd/
		// WorktreeAddDetach call sites elsewhere, the WorktreeOps.Create
		// interface sink (mindspec_executor.go's higher-layer wrapper
		// around the same operation), and an os.Chdir/os.MkdirAll on a
		// composed path.
		//
		// Scope note: the execCommand pattern below is deliberately
		// anchored to `"worktree", "add"` (not a bare `gitArgs(` match).
		// A broader match also catches gitops.go's WorktreeRemoveForce
		// (`git worktree remove --force`), which this scan DID surface
		// during authoring — and which turned out to have NO containment
		// check at all. That is a real, pre-existing gap, but removal is
		// outside AC-11's pinned inventory (creation/chdir/mkdir sites
		// only) and outside this fix-up's scope; narrowing here avoids
		// silently expanding what this bead changes. Filed as a
		// follow-up rather than fixed inline.
		sinkPatterns := []*regexp.Regexp{
			regexp.MustCompile(`gitutil\.WorktreeAdd\(`),
			regexp.MustCompile(`gitutil\.WorktreeAddDetach\(`),
			regexp.MustCompile(`execCommand\("git",\s*gitArgs\([^)]*"worktree",\s*"add"`),
			regexp.MustCompile(`\.WorktreeOps\.Create\(`),
			regexp.MustCompile(`os\.Chdir\(`),
			regexp.MustCompile(`os\.MkdirAll\(`),
		}
		// A composed worktree path is, by this codebase's uniform
		// naming convention, spelled with "wt" (case-insensitive)
		// somewhere in the argument expression on the sink line —
		// wtPath, relWtPath, wtRootPath, anchorWtRootPath, specWtPath.
		// This is what actually discriminates a worktree-path sink from
		// the many OTHER os.Chdir/os.MkdirAll calls in the tree (ADR
		// dirs, recording dirs, bootstrap dirs, journal/lock dirs,
		// plain root/g.Root/wd/dir chdirs, ...) — verified empirically:
		// every non-worktree sink call in cmd/+internal fails this
		// discriminator, and every one of the ten pinned sites passes
		// it, which is exactly why it is useful as a filter rather than
		// a no-op.
		wtHeuristic := regexp.MustCompile(`(?i)wt`)

		// withWorkingDir(<arg>, func() error { ... }) call sites: a separate
		// classifier (not the wt-name heuristic above, which cannot
		// discriminate these -- see the comment above this subtest).
		// Requires a trailing comma so it only matches an actual call (e.g.
		// "withWorkingDir(anchorRoot,") and not the wrapper's own
		// "func withWorkingDir(dir string, ...)" declaration, whose captured
		// token is followed by a type name, not a comma.
		withWorkingDirCallRe := regexp.MustCompile(`withWorkingDir\(\s*([A-Za-z0-9_.]+)\s*,`)
		const trustedRootArg = "g.Root"

		type foundSite struct {
			relPath string
			line    string // trimmed source text, not a line NUMBER —
			// robust to incidental line-number churn (e.g. a comment
			// added elsewhere in the file) while still requiring an
			// exact match on the sink invocation itself.
		}
		var discovered []foundSite
		trustedExcludedWithWorkingDir := 0 // count of withWorkingDir(g.Root, ...) sites seen

		walkErr := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == "containment" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if !strings.HasPrefix(rel, "cmd/") && !strings.HasPrefix(rel, "internal/") {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, line := range strings.Split(string(content), "\n") {
				// withWorkingDir(...) is classified separately (see comment
				// above): its own body performs the real os.Chdir, so a
				// call site is itself a composed-or-trusted chdir sink that
				// the wt-name heuristic below cannot see. None of the other
				// sinkPatterns can match "withWorkingDir(" text, so it is
				// safe to classify and `continue` without falling through.
				if m := withWorkingDirCallRe.FindStringSubmatch(line); m != nil {
					if m[1] == trustedRootArg {
						trustedExcludedWithWorkingDir++
					} else {
						discovered = append(discovered, foundSite{relPath: rel, line: strings.TrimSpace(line)})
					}
					continue
				}

				sinkHit := false
				for _, sink := range sinkPatterns {
					if sink.MatchString(line) {
						sinkHit = true
						break
					}
				}
				if !sinkHit || !wtHeuristic.MatchString(line) {
					continue
				}
				discovered = append(discovered, foundSite{relPath: rel, line: strings.TrimSpace(line)})
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walking repo tree for discovery scan: %v", walkErr)
		}

		// The pinned AC-11 inventory (the same sites the grep-complete
		// subtest above pins, plus the withWorkingDir(anchorRoot, ...)
		// composed-path call at mindspec_executor.go:278 -- gated by the
		// checkWorktreeContainment(g.Root, anchorRoot) check a few lines
		// above it at :244, and independently re-checked again at :275
		// immediately before this call), keyed by exact trimmed source
		// text so a pure line-number shift never trips this test.
		pinned := map[string]map[string]bool{
			"cmd/mindspec/complete.go": {
				"os.Chdir(specWtPath)": true,
			},
			"cmd/mindspec/impl.go": {
				"_ = os.Chdir(specWtPath)": true,
			},
			"internal/gitutil/gitops.go": {
				`cmd := execCommand("git", gitArgs(workdir, "worktree", "add", "--detach", wtPath, commit)...)`: true,
				`cmd := execCommand("git", gitArgs(workdir, "worktree", "add", wtPath, branch)...)`:             true,
			},
			"internal/executor/mindspec_executor.go": {
				"if err := os.MkdirAll(wtRootPath, 0o755); err != nil {":                   true,
				"if err := g.WorktreeOps.Create(relWtPath, specBranch); err != nil {":      true,
				"if err := os.MkdirAll(anchorWtRootPath, 0o755); err != nil {":             true,
				"return g.WorktreeOps.Create(relWtPath, branchName)":                       true,
				"if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {":         true,
				"if err := gitutil.WorktreeAdd(g.Root, wtPath, choreBranch); err != nil {": true,
				"if err := withWorkingDir(anchorRoot, func() error {":                      true,
			},
		}
		wantTotal := 0
		for _, lines := range pinned {
			wantTotal += len(lines)
		}

		// Sanity check on the classifier itself (not just its output): the
		// five withWorkingDir(g.Root, ...) trusted-root-excluded call sites
		// (CompleteBead, ForceMergeSpecIntoMain's fetch, and the three
		// FinalizeEpic-family cleanups) are a known, fixed count. If this
		// drifts it means either a new withWorkingDir(g.Root, ...) site
		// appeared (fine, but the count below documents it so it isn't a
		// silent change) or, more importantly, that a NEW composed-path
		// withWorkingDir call was miscounted as trusted-root by a typo'd
		// argument -- which would let it silently skip the pinned-inventory
		// check above.
		const wantTrustedExcludedWithWorkingDir = 5
		if trustedExcludedWithWorkingDir != wantTrustedExcludedWithWorkingDir {
			t.Errorf("found %d withWorkingDir(g.Root, ...) trusted-root-excluded call sites, want exactly %d -- verify no composed-path withWorkingDir call was misclassified as trusted, and no site was added/removed without review",
				trustedExcludedWithWorkingDir, wantTrustedExcludedWithWorkingDir)
		}

		seen := map[string]map[string]bool{}
		for _, d := range discovered {
			if seen[d.relPath] == nil {
				seen[d.relPath] = map[string]bool{}
			}
			seen[d.relPath][d.line] = true

			fileLines, known := pinned[d.relPath]
			if !known || !fileLines[d.line] {
				t.Errorf("discovered composed-worktree-path use site is NOT in the pinned AC-11 inventory: %s: %q — a new create/chdir/mkdir site was added without updating the inventory (and its nearby containment.CheckContainment gate)", d.relPath, d.line)
			}
		}
		for relPath, lines := range pinned {
			for line := range lines {
				if !seen[relPath][line] {
					t.Errorf("pinned AC-11 inventory site %s: %q was not (re)discovered by the independent scan — it may have moved, been renamed, or removed; update the pinned inventory (and check the gate still applies) if this is intentional", relPath, line)
				}
			}
		}
		if len(discovered) != wantTotal {
			t.Errorf("discovered %d composed-worktree-path use sites, want exactly %d (the pinned inventory size) — set is not equal", len(discovered), wantTotal)
		}
	})
}

// findRepoRootForTest locates the repository root from this test file's own
// source path (internal/executor/ -> two levels up), so the grep-complete
// scan reads real, current source rather than a hardcoded absolute path.
func findRepoRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: could not resolve this test file's path")
	}
	// thisFile = <repoRoot>/internal/executor/containment_checkatuse_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}
