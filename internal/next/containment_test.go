package next

// containment_test.go — spec 128 (lifecycle-verb-state-honesty) Bead 7,
// ROUND-2 PANEL FIX (G1, hard block): the samePath(anchor, cwd) lexical-
// equality gate is replaced by canonical-toplevel CONTAINMENT
// (toplevelOf/containedRel in guard.go). These tests use a REAL git repo
// (not fakeGuardZ's stubbed porcelain) because toplevelOf calls the real
// `git rev-parse --show-toplevel` — the exact mechanism under test.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func runRealGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initRealGitRepo creates a real git repo at t.TempDir() with a pinned
// test identity and a baseline commit whose .gitignore LACKS the
// .worktrees entry — the entry-absent shape every test below dirties or
// observes.
func initRealGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runRealGit(t, root, "init", "-q", "-b", "main")
	runRealGit(t, root, "config", "user.email", "test@example.invalid")
	runRealGit(t, root, "config", "user.name", "mindspec-test")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "subdir", "keep.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runRealGit(t, root, "add", "-A")
	runRealGit(t, root, "commit", "-q", "-m", "baseline")
	return root
}

// --- toplevelOf ---

func TestToplevelOf_ResolvesFromRootItself(t *testing.T) {
	root := initRealGitRepo(t)
	got := toplevelOf(root)
	want := toplevelOf(root) // idempotent — real assertion is against the subdir case below
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got == "" {
		t.Fatal("toplevelOf returned empty")
	}
}

// Round-2 panel G1: invoking from a SUBDIRECTORY of the tree must
// resolve to the SAME canonical toplevel as invoking from the tree's
// own root — git status/rev-parse --show-toplevel both resolve
// correctly regardless of which subdirectory cwd is, the exact reason
// workspace.FindRoot exists.
func TestToplevelOf_SubdirectoryResolvesSameAsRoot(t *testing.T) {
	root := initRealGitRepo(t)
	fromRoot := toplevelOf(root)
	fromSubdir := toplevelOf(filepath.Join(root, "subdir"))
	if fromRoot != fromSubdir {
		t.Errorf("toplevelOf(root) = %q, toplevelOf(subdir) = %q — must be identical", fromRoot, fromSubdir)
	}
}

// Round-2 panel G1: a symlink ALIAS of the tree must resolve to the
// SAME canonical identity as the tree itself.
func TestToplevelOf_SymlinkAliasResolvesSameAsRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on windows")
	}
	root := initRealGitRepo(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatalf("creating symlink alias: %v", err)
	}
	fromRoot := toplevelOf(root)
	fromAlias := toplevelOf(alias)
	if fromRoot != fromAlias {
		t.Errorf("toplevelOf(root) = %q, toplevelOf(alias) = %q — a symlink alias must resolve to the SAME tree", fromRoot, fromAlias)
	}
}

func TestToplevelOf_NonGitDirDegradesToCwd(t *testing.T) {
	nonGit := t.TempDir()
	got := toplevelOf(nonGit)
	resolved, err := filepath.EvalSymlinks(nonGit)
	if err != nil {
		resolved = nonGit
	}
	if got != resolved {
		t.Errorf("got %q, want the EvalSymlinks-resolved cwd %q", got, resolved)
	}
}

// --- containedRel ---

// containedRel's toplevel argument is always a toplevelOf(...) result in
// production (already symlink-resolved) — these tests pass it through
// toplevelOf too, rather than a raw t.TempDir() string, so the toplevel
// and target sides agree on canonical form the same way production code
// guarantees (e.g. macOS's /var/folders/... vs realpath's
// /private/var/folders/...).

func TestContainedRel_DirectChild(t *testing.T) {
	root := toplevelOf(initRealGitRepo(t))
	rel, ok := containedRel(root, filepath.Join(root, ".gitignore"))
	if !ok {
		t.Fatal("expected containment")
	}
	if rel != ".gitignore" {
		t.Errorf("rel = %q, want .gitignore", rel)
	}
}

func TestContainedRel_NestedNonexistentPath(t *testing.T) {
	root := toplevelOf(initRealGitRepo(t))
	target := filepath.Join(root, "does", "not", "exist", "yet.txt")
	rel, ok := containedRel(root, target)
	if !ok {
		t.Fatal("expected containment even though the path doesn't exist yet")
	}
	want := filepath.ToSlash(filepath.Join("does", "not", "exist", "yet.txt"))
	if rel != want {
		t.Errorf("rel = %q, want %q", rel, want)
	}
}

func TestContainedRel_OutsideToplevelExcluded(t *testing.T) {
	root := toplevelOf(initRealGitRepo(t))
	outside := filepath.Join(filepath.Dir(root), "sibling", "file.txt")
	_, ok := containedRel(root, outside)
	if ok {
		t.Error("expected a path outside the toplevel to be excluded (ok=false), not forced into a leaking ../ relative path")
	}
}

// Round-2 fix-round regression (caught by this bead's own real-claim
// symlinked-.gitignore e2e fixture): the target's LEAF must NOT be
// symlink-resolved, even when the leaf itself is a symlink. `git
// status` reports a symlinked file under its OWN tracked name — never
// the name of whatever it points to — so resolving the leaf here would
// compute a relative path that can never match that porcelain name,
// silently un-entangling genuinely dirty, symlinked-leaf dirt.
func TestContainedRel_SymlinkLeafKeepsOwnTrackedName(t *testing.T) {
	root := toplevelOf(initRealGitRepo(t))
	if err := os.WriteFile(filepath.Join(root, "real-target.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkLeaf := filepath.Join(root, ".gitignore-symlink")
	if err := os.Symlink("real-target.txt", symlinkLeaf); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks require elevated privileges on windows")
		}
		t.Fatalf("creating symlink leaf: %v", err)
	}

	rel, ok := containedRel(root, symlinkLeaf)
	if !ok {
		t.Fatal("expected containment")
	}
	if rel != ".gitignore-symlink" {
		t.Errorf("rel = %q, want .gitignore-symlink (the symlink's OWN tracked name, never its target's) — a leaf-resolving regression would report real-target.txt instead", rel)
	}
}

// --- ComputeEntangleablePaths: round-2 regressions ---

// Round-2 panel G1 (hard block, real-claim regression #1): invoking
// from a SUBDIRECTORY of the anchor tree must NOT disarm the .gitignore
// class — the pre-fix samePath(anchor, cwd) gate went false here even
// though the dirt is fully observable (porcelain is toplevel-relative
// regardless of cwd).
func TestComputeEntangleablePaths_SubdirectoryInvocationStillDetectsGitignore(t *testing.T) {
	root := initRealGitRepo(t)
	subdir := filepath.Join(root, "subdir")

	got := ComputeEntangleablePaths(root, subdir, "")
	if !reflect.DeepEqual(got, []string{".gitignore"}) {
		t.Errorf("got %v, want [.gitignore] — a subdirectory invocation must not disarm the class", got)
	}
}

// Round-2 panel G1 (hard block, real-claim regression #2): invoking
// through a SYMLINK ALIAS of the tree must likewise not disarm the
// class.
func TestComputeEntangleablePaths_SymlinkAliasInvocationStillDetectsGitignore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on windows")
	}
	root := initRealGitRepo(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatalf("creating symlink alias: %v", err)
	}

	got := ComputeEntangleablePaths(root, alias, "")
	if !reflect.DeepEqual(got, []string{".gitignore"}) {
		t.Errorf("got %v, want [.gitignore] — a symlink alias invocation must not disarm the class", got)
	}
}

// --- CheckDirtyTreeProportional: round-2 regressions over the FULL guard ---

// The full-guard counterpart of the subdirectory regression above: with
// a real .gitignore genuinely dirty (entry absent) and the claim
// invoked from a subdirectory, the guard must still REFUSE.
func TestCheckDirtyTreeProportional_SubdirectoryInvocationBlocksOnDirtyGitignore(t *testing.T) {
	root := initRealGitRepo(t)
	subdir := filepath.Join(root, "subdir")
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", ".gitignore")}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, subdir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(blocking, []string{".gitignore"}) {
		t.Errorf("blockingDirt = %v, want [.gitignore] — a subdirectory invocation must not disarm the block", blocking)
	}
	if len(ignored) != 0 {
		t.Errorf("ignoredDirt = %v, want empty", ignored)
	}
}

// The full-guard counterpart of the symlink-alias regression: same
// dirty .gitignore, claim invoked through a symlink alias of the tree.
func TestCheckDirtyTreeProportional_SymlinkAliasInvocationBlocksOnDirtyGitignore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on windows")
	}
	root := initRealGitRepo(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatalf("creating symlink alias: %v", err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", ".gitignore")}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, alias, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(blocking, []string{".gitignore"}) {
		t.Errorf("blockingDirt = %v, want [.gitignore] — a symlink alias invocation must not disarm the block", blocking)
	}
	if len(ignored) != 0 {
		t.Errorf("ignoredDirt = %v, want empty", ignored)
	}
}
