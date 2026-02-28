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

func TestEffectiveSpecRoot_WorktreeExists(t *testing.T) {
	mainRepo := t.TempDir()

	// Create worktree directory with .mindspec marker
	wtDir := filepath.Join(mainRepo, ".worktrees", "worktree-spec-044-launch-website")
	if err := os.MkdirAll(filepath.Join(wtDir, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := EffectiveSpecRoot(mainRepo, "044-launch-website")
	if got != wtDir {
		t.Errorf("EffectiveSpecRoot with worktree: got %q, want %q", got, wtDir)
	}
}

func TestEffectiveSpecRoot_NoWorktree(t *testing.T) {
	mainRepo := t.TempDir()

	// No worktree exists — should fall back to mainRoot
	got := EffectiveSpecRoot(mainRepo, "044-launch-website")
	if got != mainRepo {
		t.Errorf("EffectiveSpecRoot without worktree: got %q, want %q", got, mainRepo)
	}
}

func TestEffectiveSpecRoot_WorktreeDirExistsButNoMindspec(t *testing.T) {
	mainRepo := t.TempDir()

	// Worktree directory exists but without .mindspec marker
	wtDir := filepath.Join(mainRepo, ".worktrees", "worktree-spec-044-launch-website")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := EffectiveSpecRoot(mainRepo, "044-launch-website")
	if got != mainRepo {
		t.Errorf("EffectiveSpecRoot with dir but no .mindspec: got %q, want %q", got, mainRepo)
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
