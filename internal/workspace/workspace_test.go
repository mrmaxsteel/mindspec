package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestFindRoot_MindspecDir(t *testing.T) {
	tmp := t.TempDir()
	// Create .mindspec/ directory marker
	if err := os.Mkdir(filepath.Join(tmp, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_GitOnly(t *testing.T) {
	tmp := t.TempDir()
	// Create .git directory (no .mindspec/)
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_WalksUp(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_MindspecDirPriority(t *testing.T) {
	tmp := t.TempDir()
	// Both .mindspec/ and .git exist — .mindspec/ should be found first
	if err := os.Mkdir(filepath.Join(tmp, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_WorktreeResolvesToMainRepo(t *testing.T) {
	// Simulate a git worktree inside the main repo:
	// mainRepo/.mindspec/  mainRepo/.git/worktrees/wt-033/  (directory)
	// mainRepo/.worktrees/wt-033/.mindspec/
	// mainRepo/.worktrees/wt-033/.git  (file → gitdir: ../../.git/worktrees/wt-033)
	mainRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mainRepo, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Main repo .git directory with worktrees subdir
	wtGitDir := filepath.Join(mainRepo, ".git", "worktrees", "wt-033")
	if err := os.MkdirAll(wtGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// commondir file inside the worktree's gitdir
	if err := os.WriteFile(filepath.Join(wtGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Worktree directory with .mindspec/ and .git file
	wtDir := filepath.Join(mainRepo, ".worktrees", "wt-033")
	if err := os.MkdirAll(filepath.Join(wtDir, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitFileContent := "gitdir: " + wtGitDir + "\n"
	if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte(gitFileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// FindRoot from inside the worktree should resolve to mainRepo
	root, err := FindRoot(wtDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != mainRepo {
		t.Errorf("expected root %q (main repo), got %q (worktree)", mainRepo, root)
	}
}

func TestFindRoot_WorktreeNestedSubdir(t *testing.T) {
	// FindRoot from a subdirectory inside a worktree should still resolve to main repo
	mainRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mainRepo, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	wtGitDir := filepath.Join(mainRepo, ".git", "worktrees", "wt-x")
	if err := os.MkdirAll(wtGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtGitDir, "commondir"), []byte("../.."), 0o644); err != nil {
		t.Fatal(err)
	}

	wtDir := filepath.Join(mainRepo, ".worktrees", "wt-x")
	if err := os.MkdirAll(filepath.Join(wtDir, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: "+wtGitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(wtDir, "internal", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != mainRepo {
		t.Errorf("expected root %q (main repo), got %q", mainRepo, root)
	}
}

func TestFindRoot_NoMarker(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "isolated")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := FindRoot(nested)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrNoRoot {
		t.Errorf("expected ErrNoRoot, got %v", err)
	}
}

func TestDocsDir(t *testing.T) {
	got := DocsDir("/project")
	want := filepath.Join("/project", "docs")
	if got != want {
		t.Errorf("DocsDir: got %q, want %q", got, want)
	}
}

func TestDocsDir_CanonicalPreferred(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := DocsDir(root)
	if got != canonical {
		t.Errorf("DocsDir canonical: got %q, want %q", got, canonical)
	}
}

func TestSpecDir_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := SpecDir(root, "001-test")
	if err != nil {
		t.Fatalf("SpecDir unexpected error: %v", err)
	}
	want := filepath.Join(canonical, "specs", "001-test")
	if got != want {
		t.Errorf("SpecDir canonical: got %q, want %q", got, want)
	}
}

func TestContextMapPath_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ContextMapPath(root)
	want := filepath.Join(canonical, "context-map.md")
	if got != want {
		t.Errorf("ContextMapPath canonical: got %q, want %q", got, want)
	}
}

func TestADRDir_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ADRDir(root)
	want := filepath.Join(canonical, "adr")
	if got != want {
		t.Errorf("ADRDir canonical: got %q, want %q", got, want)
	}
}

func TestDomainDir_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DomainDir(root, "core")
	if err != nil {
		t.Fatalf("DomainDir unexpected error: %v", err)
	}
	want := filepath.Join(canonical, "domains", "core")
	if got != want {
		t.Errorf("DomainDir canonical: got %q, want %q", got, want)
	}
}

func TestRecordingDir_UsesSpecDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := RecordingDir(root, "001-test")
	if err != nil {
		t.Fatalf("RecordingDir unexpected error: %v", err)
	}
	want := filepath.Join(canonical, "specs", "001-test", "recording")
	if got != want {
		t.Errorf("RecordingDir canonical: got %q, want %q", got, want)
	}
}

func TestSpecDir_WorktreeAware_WorktreeFirst(t *testing.T) {
	root := t.TempDir()

	// Create spec dir in worktree, canonical, and legacy locations
	specID := "044-launch-website"
	wtSpec := filepath.Join(root, ".worktrees", "worktree-spec-"+specID, ".mindspec", "docs", "specs", specID)
	canonical := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	legacy := filepath.Join(root, "docs", "specs", specID)
	for _, p := range []string{wtSpec, canonical, legacy} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir unexpected error: %v", err)
	}
	if got != wtSpec {
		t.Errorf("SpecDir worktree-first: got %q, want %q", got, wtSpec)
	}
}

func TestSpecDir_WorktreeAware_CanonicalFallback(t *testing.T) {
	root := t.TempDir()

	// Only canonical and legacy exist (no worktree)
	specID := "044-launch-website"
	canonical := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	legacy := filepath.Join(root, "docs", "specs", specID)
	for _, p := range []string{canonical, legacy} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir unexpected error: %v", err)
	}
	if got != canonical {
		t.Errorf("SpecDir canonical fallback: got %q, want %q", got, canonical)
	}
}

func TestSpecDir_WorktreeAware_LegacyFallback(t *testing.T) {
	root := t.TempDir()

	// Only legacy exists
	specID := "044-launch-website"
	legacy := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir unexpected error: %v", err)
	}
	if got != legacy {
		t.Errorf("SpecDir legacy fallback: got %q, want %q", got, legacy)
	}
}

func TestSpecDir_WorktreeAware_DefaultsToCanonical(t *testing.T) {
	root := t.TempDir()

	// Nothing exists on disk (greenfield) — the write-default stays the
	// historical canonical path, byte-for-byte as before (Req 15). Born-flat
	// is realized only once bootstrap has created the flat lifecycle dirs (see
	// TestSpecDir_DefaultsToFlatOnFlatTree).
	specID := "044-launch-website"
	want := filepath.Join(root, ".mindspec", "docs", "specs", specID)

	got, err := SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("SpecDir greenfield default: got %q, want %q", got, want)
	}
}

func TestSpecDir_DefaultsToFlatOnFlatTree(t *testing.T) {
	root := t.TempDir()
	// A bootstrapped flat tree (flat lifecycle dirs present) is born flat: a
	// NEW spec's write target is the flat .mindspec/specs/<id> (Req 2/AC4).
	for _, d := range []string{
		filepath.Join(root, ".mindspec", "specs"),
		filepath.Join(root, ".mindspec", "domains"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	specID := "044-launch-website"
	want := filepath.Join(root, ".mindspec", "specs", specID)

	got, err := SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("SpecDir flat-tree default: got %q, want %q", got, want)
	}
}

func TestRecordingDir_WorktreeAware(t *testing.T) {
	root := t.TempDir()

	// Create spec dir only in worktree
	specID := "044-launch-website"
	wtSpec := filepath.Join(root, ".worktrees", "worktree-spec-"+specID, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(wtSpec, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := RecordingDir(root, specID)
	if err != nil {
		t.Fatalf("RecordingDir unexpected error: %v", err)
	}
	want := filepath.Join(wtSpec, "recording")
	if got != want {
		t.Errorf("RecordingDir worktree: got %q, want %q", got, want)
	}
}

func TestFindLocalRoot_ReturnsWorktreeDir(t *testing.T) {
	// FindLocalRoot should return the worktree directory itself, NOT the main repo.
	mainRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mainRepo, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	wtGitDir := filepath.Join(mainRepo, ".git", "worktrees", "wt-local")
	if err := os.MkdirAll(wtGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wtDir := filepath.Join(mainRepo, ".worktrees", "wt-local")
	if err := os.MkdirAll(filepath.Join(wtDir, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitFileContent := "gitdir: " + wtGitDir + "\n"
	if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte(gitFileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// FindLocalRoot from inside the worktree should return the worktree dir (NOT mainRepo).
	root, err := FindLocalRoot(wtDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != wtDir {
		t.Errorf("FindLocalRoot: expected worktree dir %q, got %q", wtDir, root)
	}

	// Contrast with FindRoot which resolves to mainRepo.
	mainRoot, err := FindRoot(wtDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mainRoot != mainRepo {
		t.Errorf("FindRoot: expected main repo %q, got %q", mainRepo, mainRoot)
	}
}

func TestFindLocalRoot_NonWorktree(t *testing.T) {
	// For a non-worktree directory, FindLocalRoot and FindRoot should return the same result.
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}

	localRoot, err := FindLocalRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	root, err := FindRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if localRoot != root {
		t.Errorf("FindLocalRoot and FindRoot should match for non-worktree: local=%q root=%q", localRoot, root)
	}
}

func TestFindLocalRoot_NestedSubdir(t *testing.T) {
	// FindLocalRoot from a subdirectory inside a worktree should return the worktree root.
	mainRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mainRepo, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	wtGitDir := filepath.Join(mainRepo, ".git", "worktrees", "wt-nested")
	if err := os.MkdirAll(wtGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtGitDir, "commondir"), []byte("../.."), 0o644); err != nil {
		t.Fatal(err)
	}

	wtDir := filepath.Join(mainRepo, ".worktrees", "wt-nested")
	if err := os.MkdirAll(filepath.Join(wtDir, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: "+wtGitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(wtDir, "internal", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindLocalRoot(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != wtDir {
		t.Errorf("FindLocalRoot nested: expected worktree %q, got %q", wtDir, root)
	}
}

func TestDetectWorktreeContext_Main(t *testing.T) {
	t.Parallel()
	kind, specID, beadID := DetectWorktreeContext("/Users/dev/project/internal/pkg")
	if kind != WorktreeMain {
		t.Errorf("expected main, got %s", kind)
	}
	if specID != "" || beadID != "" {
		t.Errorf("expected empty IDs, got spec=%q bead=%q", specID, beadID)
	}
}

func TestDetectWorktreeContext_Spec(t *testing.T) {
	t.Parallel()
	kind, specID, beadID := DetectWorktreeContext("/Users/dev/project/.worktrees/worktree-spec-058-zero-git/internal")
	if kind != WorktreeSpec {
		t.Errorf("expected spec, got %s", kind)
	}
	if specID != "058-zero-git" {
		t.Errorf("expected spec ID 058-zero-git, got %q", specID)
	}
	if beadID != "" {
		t.Errorf("expected empty bead ID, got %q", beadID)
	}
}

func TestDetectWorktreeContext_Bead(t *testing.T) {
	t.Parallel()
	kind, specID, beadID := DetectWorktreeContext("/Users/dev/project/.worktrees/worktree-mindspec-abc123/src")
	if kind != WorktreeBead {
		t.Errorf("expected bead, got %s", kind)
	}
	if specID != "" {
		t.Errorf("expected empty spec ID, got %q", specID)
	}
	if beadID != "mindspec-abc123" {
		t.Errorf("expected bead ID mindspec-abc123, got %q", beadID)
	}
}

// ContextLine (spec 092 Req 8): exact-format assertions for all three
// worktree kinds.
func TestContextLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		dir         string
		checkedPath string
		want        string
	}{
		{
			name:        "main",
			dir:         "/Users/dev/project",
			checkedPath: "/Users/dev/project",
			want:        "you are in the main worktree (/Users/dev/project); this check evaluated /Users/dev/project",
		},
		{
			name:        "spec",
			dir:         "/Users/dev/project/.worktrees/worktree-spec-058-zero-git",
			checkedPath: "/Users/dev/project",
			want:        "you are in the spec worktree (/Users/dev/project/.worktrees/worktree-spec-058-zero-git); this check evaluated /Users/dev/project",
		},
		{
			name:        "bead",
			dir:         "/Users/dev/project/.worktrees/worktree-mindspec-abc123",
			checkedPath: "/Users/dev/project/.worktrees/worktree-mindspec-abc123",
			want:        "you are in the bead worktree (/Users/dev/project/.worktrees/worktree-mindspec-abc123); this check evaluated /Users/dev/project/.worktrees/worktree-mindspec-abc123",
		},
		{
			name:        "nested bead worktree resolves to innermost kind",
			dir:         "/Users/dev/project/.worktrees/worktree-spec-092-x/.worktrees/worktree-mindspec-fwo5.1",
			checkedPath: "/Users/dev/project",
			want:        "you are in the bead worktree (/Users/dev/project/.worktrees/worktree-spec-092-x/.worktrees/worktree-mindspec-fwo5.1); this check evaluated /Users/dev/project",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ContextLine(tc.dir, tc.checkedPath); got != tc.want {
				t.Errorf("ContextLine mismatch:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestCanonicalAndLegacyDocsDir(t *testing.T) {
	root := "/project"
	if got := CanonicalDocsDir(root); got != filepath.Join(root, ".mindspec", "docs") {
		t.Errorf("CanonicalDocsDir: got %q", got)
	}
	if got := LegacyDocsDir(root); got != filepath.Join(root, "docs") {
		t.Errorf("LegacyDocsDir: got %q", got)
	}
}

func TestTreeRootForSpecDir(t *testing.T) {
	cases := []struct {
		name    string
		specDir string
		want    string
	}{
		{
			name:    "canonical layout in spec worktree",
			specDir: "/repo/.worktrees/worktree-spec-091-x/.mindspec/docs/specs/091-x",
			want:    "/repo/.worktrees/worktree-spec-091-x",
		},
		{
			name:    "canonical layout in primary checkout",
			specDir: "/repo/.mindspec/docs/specs/091-x",
			want:    "/repo",
		},
		{
			// Req 7 / mindspec-ew79: the flat shape must resolve the tree root
			// (the pre-spec Base(docs)!="docs" check returned "" here).
			name:    "flat layout in primary checkout",
			specDir: "/repo/.mindspec/specs/091-x",
			want:    "/repo",
		},
		{
			name:    "flat layout in spec worktree",
			specDir: "/repo/.worktrees/worktree-spec-091-x/.mindspec/specs/091-x",
			want:    "/repo/.worktrees/worktree-spec-091-x",
		},
		{
			name:    "legacy layout",
			specDir: "/repo/docs/specs/091-x",
			want:    "/repo",
		},
		{
			name:    "legacy layout in spec worktree",
			specDir: "/repo/.worktrees/worktree-spec-091-x/docs/specs/091-x",
			want:    "/repo/.worktrees/worktree-spec-091-x",
		},
		{
			name:    "trailing slash cleaned",
			specDir: "/repo/.mindspec/docs/specs/091-x/",
			want:    "/repo",
		},
		{
			name:    "unrecognized layout",
			specDir: "/repo/somewhere/091-x",
			want:    "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TreeRootForSpecDir(tc.specDir); got != tc.want {
				t.Errorf("TreeRootForSpecDir(%q) = %q, want %q", tc.specDir, got, tc.want)
			}
		})
	}
}

// --- Spec 106: per-artifact three-tier resolvers + DetectLayout ---

// mkTree creates a docs lifecycle tree under docsRoot (an absolute path) with
// the artifacts the resolver matrix exercises.
func mkTree(t *testing.T, docsRoot, specID string) {
	t.Helper()
	for _, d := range []string{
		filepath.Join(docsRoot, "specs", specID),
		filepath.Join(docsRoot, "adr"),
		filepath.Join(docsRoot, "domains", "core"),
		filepath.Join(docsRoot, "core"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "context-map.md"), []byte("# map\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolverMatrix asserts every accessor resolves byte-identically on
// canonical/legacy fixtures (AC1) and flat-first on a flat fixture (AC2).
func TestResolverMatrix(t *testing.T) {
	const specID = "044-launch-website"
	cases := []struct {
		name    string
		docsRel []string // path segments of the docs root, relative to root
	}{
		{name: "canonical", docsRel: []string{".mindspec", "docs"}},
		{name: "legacy", docsRel: []string{"docs"}},
		{name: "flat", docsRel: []string{".mindspec"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			docsRoot := filepath.Join(append([]string{root}, tc.docsRel...)...)
			mkTree(t, docsRoot, specID)

			specDir, err := SpecDir(root, specID)
			if err != nil {
				t.Fatalf("SpecDir: %v", err)
			}
			if want := filepath.Join(docsRoot, "specs", specID); specDir != want {
				t.Errorf("SpecDir = %q, want %q", specDir, want)
			}
			if got, want := ADRDir(root), filepath.Join(docsRoot, "adr"); got != want {
				t.Errorf("ADRDir = %q, want %q", got, want)
			}
			dom, err := DomainDir(root, "core")
			if err != nil {
				t.Fatalf("DomainDir: %v", err)
			}
			if want := filepath.Join(docsRoot, "domains", "core"); dom != want {
				t.Errorf("DomainDir = %q, want %q", dom, want)
			}
			if got, want := ContextMapPath(root), filepath.Join(docsRoot, "context-map.md"); got != want {
				t.Errorf("ContextMapPath = %q, want %q", got, want)
			}
			if got, want := CoreDir(root), filepath.Join(docsRoot, "core"); got != want {
				t.Errorf("CoreDir = %q, want %q", got, want)
			}
			rec, err := RecordingDir(root, specID)
			if err != nil {
				t.Fatalf("RecordingDir: %v", err)
			}
			if want := filepath.Join(docsRoot, "specs", specID, "recording"); rec != want {
				t.Errorf("RecordingDir = %q, want %q", rec, want)
			}
		})
	}
}

// TestDownstreamCompat_NoFlatFlip_ReadWrite is the panel R3/R6
// downstream-compatibility smoke test: a do-nothing upgrader (one that never
// runs `migrate layout`) is NOT broken by spec 106. It EXTENDS the read-only
// coverage in TestResolverMatrix (which already pins byte-identical READ
// resolution on canonical/legacy/flat fixtures) with the missing WRITE-path
// assertion: for both a canonical .mindspec/docs/... project and a legacy
// docs/... project, neither layout false-flips to flat, the resolvers READ AND
// WRITE byte-identically, and a write through the resolver lands in — and reads
// back unchanged from — the same legacy/canonical location, creating no flat
// lifecycle tree.
//
// What I found: TestResolverMatrix asserts read resolution only and never writes
// through a resolver nor re-checks DetectLayout afterward, so this adds the
// explicit write-path + no-flat-flip guarantee.
func TestDownstreamCompat_NoFlatFlip_ReadWrite(t *testing.T) {
	const specID = "044-launch-website"
	const body = "# downstream content\nbyte-identical roundtrip\n"

	cases := []struct {
		name       string
		docsRel    []string
		wantLayout Layout
	}{
		{name: "canonical", docsRel: []string{".mindspec", "docs"}, wantLayout: LayoutCanonical},
		{name: "legacy", docsRel: []string{"docs"}, wantLayout: LayoutLegacy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			docsRoot := filepath.Join(append([]string{root}, tc.docsRel...)...)
			mkTree(t, docsRoot, specID)

			// (1) No false-flip to flat: the whole-tree classification stays
			// canonical/legacy (and is not a hard error).
			if got, err := DetectLayout(root); err != nil || got != tc.wantLayout {
				t.Fatalf("DetectLayout = %q (err %v), want %q", got, err, tc.wantLayout)
			}

			// (2) READ path: every resolver points at the legacy/canonical tree
			// (byte-identical to the pre-spec resolution).
			specDir, err := SpecDir(root, specID)
			if err != nil {
				t.Fatalf("SpecDir: %v", err)
			}
			adrDir := ADRDir(root)
			cmPath := ContextMapPath(root)
			coreDir := CoreDir(root)
			domDir, err := DomainDir(root, "core")
			if err != nil {
				t.Fatalf("DomainDir: %v", err)
			}
			for _, c := range []struct{ got, want string }{
				{specDir, filepath.Join(docsRoot, "specs", specID)},
				{adrDir, filepath.Join(docsRoot, "adr")},
				{cmPath, filepath.Join(docsRoot, "context-map.md")},
				{coreDir, filepath.Join(docsRoot, "core")},
				{domDir, filepath.Join(docsRoot, "domains", "core")},
			} {
				if c.got != c.want {
					t.Errorf("read resolver = %q, want %q", c.got, c.want)
				}
			}

			// (3) WRITE path: writing a new artifact THROUGH the resolver lands in
			// the legacy/canonical tree. Write a new spec file under the resolved
			// spec dir and a new ADR via ADRFilePath.
			specFile := filepath.Join(specDir, "plan.md")
			if err := os.WriteFile(specFile, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			adrPath, err := ADRFilePath(root, "ADR-0007")
			if err != nil {
				t.Fatalf("ADRFilePath: %v", err)
			}
			if want := filepath.Join(docsRoot, "adr", "ADR-0007.md"); adrPath != want {
				t.Fatalf("ADRFilePath = %q, want %q (legacy/canonical location)", adrPath, want)
			}
			if err := os.WriteFile(adrPath, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			// Re-resolve AFTER the writes: paths are unchanged (no flat flip).
			if got, _ := SpecDir(root, specID); got != specDir {
				t.Errorf("post-write SpecDir = %q, want %q (unchanged)", got, specDir)
			}
			if got := ADRDir(root); got != adrDir {
				t.Errorf("post-write ADRDir = %q, want %q (unchanged)", got, adrDir)
			}

			// Read back byte-identical from the same location.
			for _, p := range []string{specFile, adrPath} {
				data, rerr := os.ReadFile(p)
				if rerr != nil {
					t.Fatalf("read back %s: %v", p, rerr)
				}
				if string(data) != body {
					t.Errorf("read back %s = %q, want byte-identical %q", p, string(data), body)
				}
			}

			// (4) Still NOT flat after writing, and NO flat lifecycle tree was
			// created directly under .mindspec/.
			if got, err := DetectLayout(root); err != nil || got != tc.wantLayout {
				t.Fatalf("after writes DetectLayout = %q (err %v), want %q", got, err, tc.wantLayout)
			}
			for _, flatChild := range []string{"specs", "adr", "domains", "core"} {
				if _, err := os.Stat(filepath.Join(root, ".mindspec", flatChild)); err == nil {
					t.Errorf("a flat .mindspec/%s tree was created — layout false-flipped to flat", flatChild)
				}
			}
		})
	}
}

// TestResolverFlatFirstWins pins the flat-FIRST read precedence: when a flat
// artifact coexists with a canonical one, the flat path wins (AC2).
func TestResolverFlatFirstWins(t *testing.T) {
	root := t.TempDir()
	// Canonical adr/ AND flat adr/ both present.
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := ADRDir(root), filepath.Join(root, ".mindspec", "adr"); got != want {
		t.Errorf("ADRDir flat-first: got %q, want %q", got, want)
	}
}

// TestDetectLayout_FiveStates covers all five whole-tree states incl. the
// mixed hard error, the recorded-recovery exception, and new-id-in-legacy
// staying legacy (AC3).
func TestDetectLayout_FiveStates(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0o755)
		assertLayout(t, root, LayoutCanonical, false)
	})
	t.Run("legacy", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755)
		assertLayout(t, root, LayoutLegacy, false)
	})
	t.Run("flat", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0o755)
		assertLayout(t, root, LayoutFlat, false)
	})
	t.Run("greenfield", func(t *testing.T) {
		root := t.TempDir()
		assertLayout(t, root, LayoutGreenfield, false)
	})
	t.Run("mixed flat+canonical is a hard error", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0o755)
		os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0o755)
		assertLayout(t, root, LayoutMixed, true)
	})
	t.Run("mixed flat+legacy is a hard error", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".mindspec", "adr"), 0o755)
		os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755)
		assertLayout(t, root, LayoutMixed, true)
	})
	t.Run("mixed is tolerated under an IN-PROGRESS migration recovery", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0o755)
		os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0o755)
		// A LIVE run: state.json records a non-terminal stage. The transient
		// mixed tree of a recovery in flight is tolerated (Req 2).
		writeMigrationState(t, root, "20260101T000000Z", "after-mv")
		assertLayout(t, root, LayoutMixed, false)
	})
	t.Run("mixed with a COMPLETED migration record is STILL a hard error", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0o755)
		os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0o755)
		// A finished run persists its record with the terminal "applied" stage
		// (Req 4 / AC9). A stale completed record must NOT mask a real
		// half-old/half-flat split — the exception is scoped to LIVE runs.
		writeMigrationState(t, root, "20260217T213341Z", "applied")
		assertLayout(t, root, LayoutMixed, true)
	})
	t.Run("mixed with a state-less migration dir is STILL a hard error", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0o755)
		os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0o755)
		// Mere dir existence no longer activates the exception: an empty run dir
		// (no readable state.json) is not a live recovery (BLOCKER regression).
		os.MkdirAll(filepath.Join(root, ".mindspec", "migrations", "20260101T000000Z"), 0o755)
		assertLayout(t, root, LayoutMixed, true)
	})
}

// writeMigrationState writes a layout-mover run-state record
// (.mindspec/migrations/<runID>/state.json) carrying the given stage, matching
// the Bead-3 schema (internal/layout/runstate.go State.Stage). Stage "applied"
// is the terminal/completed value; any other non-empty stage is an in-progress
// (live recovery) run.
func writeMigrationState(t *testing.T, root, runID, stage string) {
	t.Helper()
	runDir := filepath.Join(root, ".mindspec", "migrations", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"run_id":"` + runID + `","stage":"` + stage + `"}` + "\n")
	if err := os.WriteFile(filepath.Join(runDir, "state.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDetectLayout_NewIdInLegacyStaysLegacy: a legacy tree's whole-tree
// classification stays legacy and does NOT split when a not-yet-existing spec
// id is referenced (the classification is whole-tree, not per-id) (AC3). An
// existing legacy spec resolves byte-identically to its legacy path.
func TestDetectLayout_NewIdInLegacyStaysLegacy(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "specs", "001-existing"), 0o755)

	// Whole-tree classification is legacy, even though we are about to ask for
	// a brand-new id — it does not flip to mixed/greenfield/flat.
	assertLayout(t, root, LayoutLegacy, false)

	// An existing legacy spec resolves to its legacy path (byte-identical).
	got, err := SpecDir(root, "001-existing")
	if err != nil {
		t.Fatalf("SpecDir: %v", err)
	}
	if want := filepath.Join(root, "docs", "specs", "001-existing"); got != want {
		t.Errorf("existing legacy SpecDir = %q, want %q", got, want)
	}

	// Referencing a brand-new id leaves the classification legacy (not split).
	assertLayout(t, root, LayoutLegacy, false)
}

func assertLayout(t *testing.T, root string, want Layout, wantErr bool) {
	t.Helper()
	got, err := DetectLayout(root)
	if got != want {
		t.Errorf("DetectLayout = %q, want %q", got, want)
	}
	if wantErr && err == nil {
		t.Errorf("DetectLayout(%q): expected error, got nil", root)
	}
	if !wantErr && err != nil {
		t.Errorf("DetectLayout(%q): unexpected error: %v", root, err)
	}
}

// TestClassifyLayout unit-tests the pure layout-signature classifier — the
// single source of truth the Bead-4 merge guard reuses (minor 12).
func TestClassifyLayout(t *testing.T) {
	cases := []struct {
		name string
		in   LayoutMarkers
		want Layout
	}{
		{"flat only", LayoutMarkers{Flat: true}, LayoutFlat},
		{"canonical only", LayoutMarkers{Canonical: true}, LayoutCanonical},
		{"legacy only", LayoutMarkers{Legacy: true}, LayoutLegacy},
		{"empty is greenfield", LayoutMarkers{}, LayoutGreenfield},
		{"flat+canonical is mixed", LayoutMarkers{Flat: true, Canonical: true}, LayoutMixed},
		{"flat+legacy is mixed", LayoutMarkers{Flat: true, Legacy: true}, LayoutMixed},
		{"canonical+legacy prefers canonical (not mixed)", LayoutMarkers{Canonical: true, Legacy: true}, LayoutCanonical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyLayout(tc.in); got != tc.want {
				t.Errorf("ClassifyLayout(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestLayoutMarkersFromMindspecChildren pins the pure FLAT-only marker
// derivation from .mindspec's immediate children (executor.TreeDirsAtRef
// output). Bead 2 (spec 118 / AC-9) supersedes this helper's former
// "docs"-name→Canonical shortcut: a bare `.mindspec/docs` child name no
// longer sets Canonical here (or anywhere) — canonical/legacy derivation at
// a git ref now lives in internal/executor's layoutAtRef/tierMarkerAtRef,
// which descends the wrapper instead of pattern-matching its name.
func TestLayoutMarkersFromMindspecChildren(t *testing.T) {
	cases := []struct {
		name     string
		children []string
		want     LayoutMarkers
	}{
		{"bare docs child name sets no marker (Bead 2 supersedes the canonical shortcut)", []string{"docs"}, LayoutMarkers{}},
		{"flat lifecycle children", []string{"specs", "adr", "domains", "core"}, LayoutMarkers{Flat: true}},
		{"flat context-map file", []string{"context-map.md"}, LayoutMarkers{Flat: true}},
		{"bare docs child among unrelated mover state dirs sets no marker", []string{"docs", "migrations", "lineage", "config.yaml"}, LayoutMarkers{}},
		{"flat lifecycle child alongside bare docs child sets only Flat", []string{"specs", "docs"}, LayoutMarkers{Flat: true}},
		{"tolerates trailing slashes and full paths (bare docs child still sets no marker)", []string{".mindspec/specs/", "docs/"}, LayoutMarkers{Flat: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LayoutMarkersFromMindspecChildren(tc.children); got != tc.want {
				t.Errorf("LayoutMarkersFromMindspecChildren(%v) = %+v, want %+v", tc.children, got, tc.want)
			}
		})
	}
}

// TestSpecsDirAndDomainsDir pins the flat-aware ENUMERATION roots (Fold-in for
// Bead 2): SpecsDir/DomainsDir resolve byte-identically to the pre-spec
// DocsDir-join on canonical/legacy/greenfield trees, and flat on a flat tree.
func TestSpecsDirAndDomainsDir(t *testing.T) {
	const specID = "044-launch-website"
	cases := []struct {
		name    string
		docsRel []string // path segments of the docs root, relative to root; nil = greenfield (no tree)
	}{
		{name: "canonical", docsRel: []string{".mindspec", "docs"}},
		{name: "legacy", docsRel: []string{"docs"}},
		{name: "flat", docsRel: []string{".mindspec"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			docsRoot := filepath.Join(append([]string{root}, tc.docsRel...)...)
			mkTree(t, docsRoot, specID)

			if got, want := SpecsDir(root), filepath.Join(docsRoot, "specs"); got != want {
				t.Errorf("SpecsDir = %q, want %q", got, want)
			}
			if got, want := DomainsDir(root), filepath.Join(docsRoot, "domains"); got != want {
				t.Errorf("DomainsDir = %q, want %q", got, want)
			}
		})
	}

	t.Run("greenfield falls back to the DocsDir join (byte-identical)", func(t *testing.T) {
		root := t.TempDir()
		// No tree present: matches the pre-spec filepath.Join(DocsDir(root), …).
		if got, want := SpecsDir(root), filepath.Join(DocsDir(root), "specs"); got != want {
			t.Errorf("SpecsDir greenfield = %q, want %q", got, want)
		}
		if got, want := DomainsDir(root), filepath.Join(DocsDir(root), "domains"); got != want {
			t.Errorf("DomainsDir greenfield = %q, want %q", got, want)
		}
	})

	t.Run("flat-first wins at the enumeration root", func(t *testing.T) {
		root := t.TempDir()
		// Flat AND canonical specs/domains roots both present: flat wins.
		for _, d := range []string{
			filepath.Join(root, ".mindspec", "specs"),
			filepath.Join(root, ".mindspec", "domains"),
			filepath.Join(root, ".mindspec", "docs", "specs"),
			filepath.Join(root, ".mindspec", "docs", "domains"),
		} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if got, want := SpecsDir(root), filepath.Join(root, ".mindspec", "specs"); got != want {
			t.Errorf("SpecsDir flat-first = %q, want %q", got, want)
		}
		if got, want := DomainsDir(root), filepath.Join(root, ".mindspec", "domains"); got != want {
			t.Errorf("DomainsDir flat-first = %q, want %q", got, want)
		}
	})
}

// TestSpecDir_BothWorktreeShapes pins SpecDir resolution for BOTH the canonical
// and the flat worktree shapes (AC12 / Req 7).
func TestSpecDir_BothWorktreeShapes(t *testing.T) {
	const specID = "044-launch-website"
	t.Run("flat worktree shape", func(t *testing.T) {
		root := t.TempDir()
		specWtName, err := SpecWorktreeName(specID)
		if err != nil {
			t.Fatalf("SpecWorktreeName: %v", err)
		}
		wt := filepath.Join(root, ".worktrees", specWtName, ".mindspec", "specs", specID)
		if err := os.MkdirAll(wt, 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := SpecDir(root, specID)
		if err != nil {
			t.Fatalf("SpecDir: %v", err)
		}
		if got != wt {
			t.Errorf("SpecDir flat worktree: got %q, want %q", got, wt)
		}
	})
	t.Run("canonical worktree shape", func(t *testing.T) {
		root := t.TempDir()
		specWtName, err := SpecWorktreeName(specID)
		if err != nil {
			t.Fatalf("SpecWorktreeName: %v", err)
		}
		wt := filepath.Join(root, ".worktrees", specWtName, ".mindspec", "docs", "specs", specID)
		if err := os.MkdirAll(wt, 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := SpecDir(root, specID)
		if err != nil {
			t.Fatalf("SpecDir: %v", err)
		}
		if got != wt {
			t.Errorf("SpecDir canonical worktree: got %q, want %q", got, wt)
		}
	})
}

// TestTreeRootForSpecDir_Ew79FlatWorktree pins the mindspec-ew79 cross-worktree
// ADR-visibility fix for the FLAT worktree shape: TreeRootForSpecDir resolves
// the worktree root from a flat spec dir, so an ADR store rooted there sees the
// branch-only ADRs (AC12).
func TestTreeRootForSpecDir_Ew79FlatWorktree(t *testing.T) {
	const specID = "091-x"
	root := t.TempDir()
	specWtName, err := SpecWorktreeName(specID)
	if err != nil {
		t.Fatalf("SpecWorktreeName: %v", err)
	}
	wtRoot := filepath.Join(root, ".worktrees", specWtName)
	specDir := filepath.Join(wtRoot, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A flat ADR dir committed only on the spec branch (visible in the worktree).
	if err := os.MkdirAll(filepath.Join(wtRoot, ".mindspec", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}

	gotRoot := TreeRootForSpecDir(specDir)
	if gotRoot != wtRoot {
		t.Fatalf("TreeRootForSpecDir flat worktree: got %q, want %q", gotRoot, wtRoot)
	}
	// ADRDir rooted at the worktree resolves the branch-only flat ADR tree —
	// the visibility the ew79 fix preserves.
	if got, want := ADRDir(gotRoot), filepath.Join(wtRoot, ".mindspec", "adr"); got != want {
		t.Errorf("ADRDir(treeRoot) = %q, want %q", got, want)
	}
}

// --- Spec 118 Bead 1: scope filesystem layout markers to resolver-shaped
// artifacts (ADR-0039 Decision §2) ---
//
// TestLayoutMarkerResolveMatrix is the complete table-driven filesystem
// resolve matrix (plan Bead 1, Testing Strategy). Each row is RED on revert:
// reverting canonical/legacy detection to bare wrapper existence, or
// dropping the IsDir/regular-file type checks, makes the applicable row(s)
// fail. All four shared lifecycle names (specs, adr, domains, core) are
// exercised across the table, including empty lifecycle directories (every
// lifecycle dir fixture below is created via MkdirAll with no further
// content).
func TestLayoutMarkerResolveMatrix(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, root string)
		wantLayout Layout
		wantErr    bool
	}{
		{
			// AC-1: flat marker + an ordinary root docs/ with no direct
			// lifecycle child and no context-map.md file → flat (the legacy
			// wedge fix; a bare/unrelated docs/ is no longer a marker).
			name: "flat plus ordinary unrelated docs wrapper resolves flat",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "specs"))
				mustMkdirAll(t, filepath.Join(root, "docs"))
				mustWriteFile(t, filepath.Join(root, "docs", "README.md"), "unrelated\n")
			},
			wantLayout: LayoutFlat,
		},
		{
			// AC-2: flat marker + empty/leftover .mindspec/docs/ with no
			// direct lifecycle child and no context-map.md file → flat (the
			// canonical wedge fix).
			name: "flat plus empty leftover canonical wrapper resolves flat",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "adr"))
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs"))
			},
			wantLayout: LayoutFlat,
		},
		{
			// AC-3: no flat marker; root docs/ directly contains an EMPTY
			// lifecycle directory → legacy.
			name: "no flat marker plus empty direct legacy lifecycle directory resolves legacy",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, "docs", "domains"))
			},
			wantLayout: LayoutLegacy,
		},
		{
			// AC-4: no flat marker; .mindspec/docs/ directly contains an
			// EMPTY lifecycle directory → canonical.
			name: "no flat marker plus empty direct canonical lifecycle directory resolves canonical",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs", "core"))
			},
			wantLayout: LayoutCanonical,
		},
		{
			// AC-5: flat lifecycle directory + .mindspec/docs/<lifecycle>/
			// both exist → mixed, DetectLayout errors.
			name: "flat plus canonical lifecycle directory resolves mixed and errors",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "adr"))
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs", "domains"))
			},
			wantLayout: LayoutMixed,
			wantErr:    true,
		},
		{
			// AC-6: flat lifecycle directory + root docs/<lifecycle>/ both
			// exist → mixed, DetectLayout errors.
			name: "flat plus legacy lifecycle directory resolves mixed and errors",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "domains"))
				mustMkdirAll(t, filepath.Join(root, "docs", "core"))
			},
			wantLayout: LayoutMixed,
			wantErr:    true,
		},
		{
			// AC-13: no other marker; only .mindspec/docs/context-map.md is
			// a REGULAR FILE → canonical.
			name: "canonical context map file",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs"))
				mustWriteFile(t, filepath.Join(root, ".mindspec", "docs", "context-map.md"), "# map\n")
			},
			wantLayout: LayoutCanonical,
		},
		{
			// AC-14: no other marker; only root docs/context-map.md is a
			// REGULAR FILE → legacy.
			name: "legacy context map file",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, "docs"))
				mustWriteFile(t, filepath.Join(root, "docs", "context-map.md"), "# map\n")
			},
			wantLayout: LayoutLegacy,
		},
		{
			// AC-15: flat marker coexists with .mindspec/docs/context-map.md
			// → mixed, DetectLayout errors (an incomplete flatten is not
			// mistaken for the AC-1/AC-2 wedge).
			name: "flat plus canonical context map file resolves mixed and errors",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "specs"))
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs"))
				mustWriteFile(t, filepath.Join(root, ".mindspec", "docs", "context-map.md"), "# map\n")
			},
			wantLayout: LayoutMixed,
			wantErr:    true,
		},
		{
			// AC-21: flat marker coexists with root docs/context-map.md as a
			// REGULAR FILE → mixed, DetectLayout errors (mirrors AC-15 for
			// the legacy tier).
			name: "flat plus legacy context map file resolves mixed and errors",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "core"))
				mustMkdirAll(t, filepath.Join(root, "docs"))
				mustWriteFile(t, filepath.Join(root, "docs", "context-map.md"), "# map\n")
			},
			wantLayout: LayoutMixed,
			wantErr:    true,
		},
		{
			// AC-17 (canonical tier): a lifecycle name nested BELOW a
			// non-lifecycle immediate child (.mindspec/docs/sub/specs, not
			// .mindspec/docs/specs) is not an immediate child, so it sets no
			// canonical marker — the tree stays greenfield.
			name: "canonical nested lifecycle name sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs", "sub", "specs"))
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-17 (legacy tier): same nested-below-non-lifecycle-child
			// guard for root docs/sub/specs.
			name: "legacy nested lifecycle name sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, "docs", "sub", "specs"))
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-18: root docs exists as a REGULAR FILE (not a directory) →
			// no legacy marker, no panic.
			name: "legacy wrapper as regular file sets no marker",
			setup: func(t *testing.T, root string) {
				mustWriteFile(t, filepath.Join(root, "docs"), "not a directory\n")
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-18: .mindspec/docs exists as a REGULAR FILE (not a
			// directory) → no canonical marker, no panic.
			name: "canonical wrapper as regular file sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec"))
				mustWriteFile(t, filepath.Join(root, ".mindspec", "docs"), "not a directory\n")
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-19 (flat tier): .mindspec/context-map.md exists as a
			// DIRECTORY rather than a regular file → no flat marker, no
			// panic.
			name: "flat context map directory sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "context-map.md"))
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-19 (canonical tier): .mindspec/docs/context-map.md exists
			// as a DIRECTORY → no canonical marker, no panic.
			name: "canonical context map directory sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs", "context-map.md"))
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-19 (legacy tier): docs/context-map.md exists as a
			// DIRECTORY → no legacy marker, no panic.
			name: "legacy context map directory sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, "docs", "context-map.md"))
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-20 (flat tier): the lifecycle name "core" exists directly
			// under .mindspec as a REGULAR FILE (not IsDir) → no flat
			// marker, no panic.
			name: "flat lifecycle name regular file sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec"))
				mustWriteFile(t, filepath.Join(root, ".mindspec", "core"), "not a directory\n")
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-20 (canonical tier): the lifecycle name "adr" exists
			// directly under .mindspec/docs as a REGULAR FILE → no
			// canonical marker, no panic.
			name: "canonical lifecycle name regular file sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, ".mindspec", "docs"))
				mustWriteFile(t, filepath.Join(root, ".mindspec", "docs", "adr"), "not a directory\n")
			},
			wantLayout: LayoutGreenfield,
		},
		{
			// AC-20 (legacy tier): the lifecycle name "domains" exists
			// directly under docs as a REGULAR FILE → no legacy marker, no
			// panic.
			name: "legacy lifecycle name regular file sets no marker",
			setup: func(t *testing.T, root string) {
				mustMkdirAll(t, filepath.Join(root, "docs"))
				mustWriteFile(t, filepath.Join(root, "docs", "domains"), "not a directory\n")
			},
			wantLayout: LayoutGreenfield,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			assertLayout(t, root, tc.wantLayout, tc.wantErr)
		})
	}
}

// mustMkdirAll is a t.Helper MkdirAll for the layout-marker fixtures above.
func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

// mustWriteFile is a t.Helper WriteFile for the layout-marker fixtures above.
// It creates the parent directory first so a fixture can write directly into
// a not-yet-created wrapper.
func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSpecDir_UnwedgedFlatNewSpec proves AC-8: in the AC-1 un-wedged flat
// repository (a flat lifecycle tree coexisting with an ordinary,
// non-lifecycle root docs/ wrapper), a brand-new spec slug resolves under
// .mindspec/specs/, not the legacy docs/specs/ the pre-fix bare-wrapper
// marker would have wedged it into (split-brain prevention through
// classification recovery). RED on revert: reverting legacy detection to
// bare `docs/` existence reclassifies this tree mixed/legacy and breaks this
// assertion.
func TestSpecDir_UnwedgedFlatNewSpec(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".mindspec", "specs"))
	mustMkdirAll(t, filepath.Join(root, "docs"))
	mustWriteFile(t, filepath.Join(root, "docs", "README.md"), "unrelated\n")

	// Sanity: the whole-tree classification itself is unwedged (flat, not
	// mixed/legacy).
	assertLayout(t, root, LayoutFlat, false)

	const newSlug = "119-brand-new-spec"
	got, err := SpecDir(root, newSlug)
	if err != nil {
		t.Fatalf("SpecDir: %v", err)
	}
	want := filepath.Join(root, ".mindspec", "specs", newSlug)
	if got != want {
		t.Errorf("SpecDir(unwedged flat, new slug) = %q, want %q", got, want)
	}
}

// TestContextMapPath_ThreeTierFallback proves AC-14: with no flat or
// canonical marker present and only root docs/context-map.md existing as a
// regular file, the whole tree classifies legacy and ContextMapPath falls
// through the three-tier resolver to that legacy file. RED on revert:
// reverting legacy detection to bare `docs/` existence still passes today,
// but reverting the regular-file type check (accepting a context-map
// directory) or dropping the context-map fallback entirely breaks this.
func TestContextMapPath_ThreeTierFallback(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "docs"))
	mustWriteFile(t, filepath.Join(root, "docs", "context-map.md"), "# legacy map\n")

	assertLayout(t, root, LayoutLegacy, false)

	want := filepath.Join(root, "docs", "context-map.md")
	if got := ContextMapPath(root); got != want {
		t.Errorf("ContextMapPath legacy three-tier fallback = %q, want %q", got, want)
	}
}

// TestDetectWorktreeContextRejectsMalformedNames is spec 120 AC-5 (D2,
// ADR-0042 §1 reverse): hostile .worktrees/worktree-spec-<hostile> and
// .worktrees/worktree-<hostile> segments resolve unrecognized (WorktreeMain,
// empty IDs) — the reverse-derivation TrimPrefix must never hand an
// unvalidated segment to a caller as a real ID. Every currently-valid
// naming fixture — incl. a dotted-child bead worktree and a finalize
// worktree — resolves byte-identically to today (this was RED under the
// pre-Bead-1 grammar: both parse to IDs the old pattern rejected).
func TestDetectWorktreeContextRejectsMalformedNames(t *testing.T) {
	t.Parallel()

	hostileSuffixes := []string{
		"x;evil",
		"../../outside",
		"x evil",
		"x\x00\x1b[31m\nrecovery: forged",
	}

	t.Run("hostile spec worktree segment resolves unrecognized", func(t *testing.T) {
		for _, h := range hostileSuffixes {
			dir := "/Users/dev/project/.worktrees/worktree-spec-" + h + "/internal"
			kind, specID, beadID := DetectWorktreeContext(dir)
			if kind != WorktreeMain {
				t.Errorf("hostile suffix %q: expected WorktreeMain, got %s (specID=%q)", h, kind, specID)
			}
			if specID != "" || beadID != "" {
				t.Errorf("hostile suffix %q: expected empty IDs, got spec=%q bead=%q", h, specID, beadID)
			}
		}
	})

	t.Run("hostile bead worktree segment resolves unrecognized", func(t *testing.T) {
		for _, h := range hostileSuffixes {
			dir := "/Users/dev/project/.worktrees/worktree-" + h + "/src"
			kind, specID, beadID := DetectWorktreeContext(dir)
			if kind != WorktreeMain {
				t.Errorf("hostile suffix %q: expected WorktreeMain, got %s (beadID=%q)", h, kind, beadID)
			}
			if specID != "" || beadID != "" {
				t.Errorf("hostile suffix %q: expected empty IDs, got spec=%q bead=%q", h, specID, beadID)
			}
		}
	})

	t.Run("clean shapes resolve byte-identically", func(t *testing.T) {
		kind, specID, beadID := DetectWorktreeContext("/Users/dev/project/.worktrees/worktree-mindspec-9cyu.1/src")
		if kind != WorktreeBead || specID != "" || beadID != "mindspec-9cyu.1" {
			t.Errorf("dotted-child bead worktree: got kind=%s spec=%q bead=%q", kind, specID, beadID)
		}

		kind, specID, beadID = DetectWorktreeContext("/Users/dev/project/.worktrees/worktree-finalize-053-foo/src")
		if kind != WorktreeBead || specID != "" || beadID != "finalize-053-foo" {
			t.Errorf("finalize worktree: got kind=%s spec=%q bead=%q", kind, specID, beadID)
		}

		kind, specID, beadID = DetectWorktreeContext("/Users/dev/project/.worktrees/worktree-spec-008b-human-gates/src")
		if kind != WorktreeSpec || specID != "008b-human-gates" || beadID != "" {
			t.Errorf("letter-suffixed spec worktree: got kind=%s spec=%q bead=%q", kind, specID, beadID)
		}
	})
}

// --- ResolveADRFile (spec 123 R5(c)) ---

func writeFileMust(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveADRFile_Bare pins the plain bare-file case: a canonical ID
// resolves to its bare "<id>.md" file when no slugged sibling exists.
func TestResolveADRFile_Bare(t *testing.T) {
	root := t.TempDir()
	adrDir := ADRDir(root)
	writeFileMust(t, filepath.Join(adrDir, "ADR-0001.md"), "# ADR-0001: Bare\n")

	got, err := ResolveADRFile(root, "ADR-0001")
	if err != nil {
		t.Fatalf("ResolveADRFile: %v", err)
	}
	want := filepath.Join(adrDir, "ADR-0001.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveADRFile_SluggedOnly pins the slug-tolerant lookup: a
// canonical ID resolves to its slugged file when no bare sibling exists.
func TestResolveADRFile_SluggedOnly(t *testing.T) {
	root := t.TempDir()
	adrDir := ADRDir(root)
	writeFileMust(t, filepath.Join(adrDir, "ADR-0001-my-slug.md"), "# ADR-0001: Slugged\n")

	got, err := ResolveADRFile(root, "ADR-0001")
	if err != nil {
		t.Fatalf("ResolveADRFile: %v", err)
	}
	want := filepath.Join(adrDir, "ADR-0001-my-slug.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveADRFile_FullSluggedInputUniqueResolves pins that a
// caller-supplied FULL slugged stem still resolves when it is the ONLY
// file carrying its canonical number (the slug is ergonomics — canonical
// ADR-NNNN is the reference currency). This is the AC-9 guard leg at the
// resolver level: a full slugged citation keeps working absent a
// collision.
func TestResolveADRFile_FullSluggedInputUniqueResolves(t *testing.T) {
	root := t.TempDir()
	adrDir := ADRDir(root)
	writeFileMust(t, filepath.Join(adrDir, "ADR-0001-my-slug.md"), "# ADR-0001: Slugged\n")

	got, err := ResolveADRFile(root, "ADR-0001-my-slug")
	if err != nil {
		t.Fatalf("ResolveADRFile with full slugged input: %v", err)
	}
	want := filepath.Join(adrDir, "ADR-0001-my-slug.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveADRFile_FullSluggedInputCollisionErrors is the FX-2 pin
// (spec 123 R5(c), input-shape-independent collision detection): a FULL
// slugged input must NOT bypass collision detection. With both a bare
// ADR-0001.md and a slugged ADR-0001-my-slug.md present — a genuine
// number-collision — `ResolveADRFile("ADR-0001-my-slug")` must return
// the ambiguity error rather than silently resolving to the named file.
// RED on revert to an exact-join-for-full-slugged short-circuit.
func TestResolveADRFile_FullSluggedInputCollisionErrors(t *testing.T) {
	root := t.TempDir()
	adrDir := ADRDir(root)
	writeFileMust(t, filepath.Join(adrDir, "ADR-0001.md"), "# ADR-0001: Bare\n")
	writeFileMust(t, filepath.Join(adrDir, "ADR-0001-my-slug.md"), "# ADR-0001: Slugged\n")

	_, err := ResolveADRFile(root, "ADR-0001-my-slug")
	if err == nil {
		t.Fatal("expected collision error for full-slugged input against a sibling collision, got nil")
	}
	if !strings.Contains(err.Error(), "ADR-0001.md") || !strings.Contains(err.Error(), "ADR-0001-my-slug.md") {
		t.Errorf("error must name both colliding paths, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery:") {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
}

// TestResolveADRFile_CollisionEscapesHostileFilename is the FX-1 pin
// (spec 116 R4 / ADR-0042): the collision error renders on-disk
// filenames through termsafe.Escape, so a control byte (ESC 0x1b) in a
// filename can never reach CLI stderr raw and forge terminal output. The
// hostile file name shares a canonical number with a bare sibling to
// force the collision branch. RED on revert to a raw filepath.Base
// render.
func TestResolveADRFile_CollisionEscapesHostileFilename(t *testing.T) {
	root := t.TempDir()
	adrDir := ADRDir(root)
	// A slugged file whose stem carries a raw ESC byte. idvalidate.ADRID
	// would reject such a stem as an INPUT, but nothing stops it existing
	// ON DISK — an operator or agent can create the file directly — and
	// the glob enumerates it as a candidate.
	hostile := "ADR-0007-x\x1b[31mred.md"
	writeFileMust(t, filepath.Join(adrDir, "ADR-0007.md"), "# ADR-0007: Bare\n")
	writeFileMust(t, filepath.Join(adrDir, hostile), "# ADR-0007: Hostile\n")

	_, err := ResolveADRFile(root, "ADR-0007")
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if strings.ContainsRune(err.Error(), '\x1b') {
		t.Errorf("collision error leaked a raw ESC byte from an on-disk filename: %q", err.Error())
	}
	// termsafe.Escape wraps a control-byte string as a quoted Go literal,
	// so the escaped form of the hostile base name appears in the message.
	if !strings.Contains(err.Error(), termsafe.Escape("ADR-0007-x\x1b[31mred.md")) {
		t.Errorf("expected the termsafe-escaped hostile filename in the error, got: %q", err.Error())
	}
}

// TestResolveADRFile_Collision is the AC-10 collision pin: a bare AND a
// slugged file sharing the same canonical number make a bare-canonical
// lookup error, naming both paths with an ADR-0035 recovery line —
// never the silent short-circuit to the bare file.
func TestResolveADRFile_Collision(t *testing.T) {
	root := t.TempDir()
	adrDir := ADRDir(root)
	writeFileMust(t, filepath.Join(adrDir, "ADR-0002.md"), "# ADR-0002: Bare\n")
	writeFileMust(t, filepath.Join(adrDir, "ADR-0002-foo.md"), "# ADR-0002: Foo\n")

	_, err := ResolveADRFile(root, "ADR-0002")
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "ADR-0002.md") || !strings.Contains(err.Error(), "ADR-0002-foo.md") {
		t.Errorf("error must name both paths, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery:") {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
}

// TestResolveADRFile_MultipleSluggedCollision pins the collision check
// also fires when TWO OR MORE slugged files share a canonical number
// with no bare file present.
func TestResolveADRFile_MultipleSluggedCollision(t *testing.T) {
	root := t.TempDir()
	adrDir := ADRDir(root)
	writeFileMust(t, filepath.Join(adrDir, "ADR-0003-first.md"), "# ADR-0003: First\n")
	writeFileMust(t, filepath.Join(adrDir, "ADR-0003-second.md"), "# ADR-0003: Second\n")

	_, err := ResolveADRFile(root, "ADR-0003")
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "ADR-0003-first.md") || !strings.Contains(err.Error(), "ADR-0003-second.md") {
		t.Errorf("error must name both paths, got: %v", err)
	}
}

// TestResolveADRFile_NotFound pins the not-found case for both a
// canonical ID and a full slugged input whose canonical number carries
// no file on disk at all.
func TestResolveADRFile_NotFound(t *testing.T) {
	root := t.TempDir()
	// Ensure the ADR dir exists but is empty.
	if err := os.MkdirAll(ADRDir(root), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveADRFile(root, "ADR-9999"); err == nil {
		t.Error("expected not-found error for canonical id, got nil")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found', got: %v", err)
	}

	if _, err := ResolveADRFile(root, "ADR-9999-nope"); err == nil {
		t.Error("expected not-found error for full slugged id, got nil")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found', got: %v", err)
	}
}

// TestResolveADRFile_InvalidID pins SEC-1 defense-in-depth: an
// unvalidated id (glob metacharacter, path separator) is rejected before
// any filepath.Glob/Join runs.
func TestResolveADRFile_InvalidID(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"ADR-0001*", "../etc/passwd", "ADR-0001/../x"} {
		if _, err := ResolveADRFile(root, bad); err == nil {
			t.Errorf("ResolveADRFile(%q) expected error, got nil", bad)
		}
	}
}
