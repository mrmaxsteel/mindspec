package workspace

import (
	"os"
	"path/filepath"
	"testing"
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

	got := SpecDir(root, "001-test")
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

	got := DomainDir(root, "core")
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

	got := RecordingDir(root, "001-test")
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

	got := SpecDir(root, specID)
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

	got := SpecDir(root, specID)
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

	got := SpecDir(root, specID)
	if got != legacy {
		t.Errorf("SpecDir legacy fallback: got %q, want %q", got, legacy)
	}
}

func TestSpecDir_WorktreeAware_DefaultsToCanonical(t *testing.T) {
	root := t.TempDir()

	// Nothing exists on disk — should default to canonical path
	specID := "044-launch-website"
	want := filepath.Join(root, ".mindspec", "docs", "specs", specID)

	got := SpecDir(root, specID)
	if got != want {
		t.Errorf("SpecDir default canonical: got %q, want %q", got, want)
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

	got := RecordingDir(root, specID)
	want := filepath.Join(wtSpec, "recording")
	if got != want {
		t.Errorf("RecordingDir worktree: got %q, want %q", got, want)
	}
}

func TestLifecyclePath_WorktreeAware(t *testing.T) {
	root := t.TempDir()

	// Create spec dir only in worktree
	specID := "044-launch-website"
	wtSpec := filepath.Join(root, ".worktrees", "worktree-spec-"+specID, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(wtSpec, 0o755); err != nil {
		t.Fatal(err)
	}

	got := LifecyclePath(root, specID)
	want := filepath.Join(wtSpec, "lifecycle.yaml")
	if got != want {
		t.Errorf("LifecyclePath worktree: got %q, want %q", got, want)
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

func TestCanonicalAndLegacyDocsDir(t *testing.T) {
	root := "/project"
	if got := CanonicalDocsDir(root); got != filepath.Join(root, ".mindspec", "docs") {
		t.Errorf("CanonicalDocsDir: got %q", got)
	}
	if got := LegacyDocsDir(root); got != filepath.Join(root, "docs") {
		t.Errorf("LegacyDocsDir: got %q", got)
	}
}
