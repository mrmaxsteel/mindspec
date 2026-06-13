package main

import (
	"os"
	"path/filepath"
	"testing"
)

// adrDirFor returns the on-disk ADR directory for a checkout root.
func adrDirFor(root string) string {
	return filepath.Join(root, ".mindspec", "docs", "adr")
}

// writeADR writes a minimal slugged ADR file under root's ADR directory.
func writeADR(t *testing.T, root, name string) {
	t.Helper()
	dir := adrDirFor(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir adr dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write adr %s: %v", name, err)
	}
}

// setupWorktreePair builds a hand-crafted git-worktree linkage so that
// workspace.FindLocalRoot resolves to the worktree dir while
// workspace.FindRoot resolves back to the main checkout — mirroring the
// real layout a bead/spec worktree has. Returns (mainRoot, worktreeRoot).
func setupWorktreePair(t *testing.T) (string, string) {
	t.Helper()
	mainRepo := t.TempDir()
	// Establish the canonical .mindspec/docs layout so workspace.DocsDir
	// resolves under .mindspec (not the legacy ./docs fallback).
	if err := os.MkdirAll(filepath.Join(mainRepo, ".mindspec", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Main repo .git directory with a linked-worktree gitdir + commondir.
	wtGitDir := filepath.Join(mainRepo, ".git", "worktrees", "wt-adr")
	if err := os.MkdirAll(wtGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The worktree checkout: .mindspec marker + a .git FILE pointing at gitdir.
	wtDir := filepath.Join(mainRepo, ".worktrees", "wt-adr")
	if err := os.MkdirAll(filepath.Join(wtDir, ".mindspec", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: "+wtGitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return mainRepo, wtDir
}

// chdir changes the working directory and restores it after the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

// TestADRCreate_WritesIntoInvokingWorktree pins that `adr create` run from a
// bead/spec worktree authors the new ADR into THAT worktree's
// .mindspec/docs/adr/, and that the file does NOT appear in the main checkout.
// RED on revert to workspace.FindRoot (which resolves the worktree back to
// main, so the file would land in main instead). (mindspec-8lzq)
func TestADRCreate_WritesIntoInvokingWorktree(t *testing.T) {
	mainRoot, wtRoot := setupWorktreePair(t)
	chdir(t, wtRoot)

	if err := adrCreateCmd.RunE(adrCreateCmd, []string{"Worktree authored decision"}); err != nil {
		t.Fatalf("adr create: %v", err)
	}

	// The new ADR must exist in the WORKTREE's ADR dir.
	wtMatches, _ := filepath.Glob(filepath.Join(adrDirFor(wtRoot), "ADR-*.md"))
	if len(wtMatches) != 1 {
		t.Fatalf("expected exactly 1 ADR in worktree %q, got %v", adrDirFor(wtRoot), wtMatches)
	}

	// And it must NOT have leaked into the MAIN checkout's ADR dir.
	mainMatches, _ := filepath.Glob(filepath.Join(adrDirFor(mainRoot), "ADR-*.md"))
	if len(mainMatches) != 0 {
		t.Fatalf("ADR leaked into main checkout %q: %v", adrDirFor(mainRoot), mainMatches)
	}
}

// TestADRCreate_NextIDOverBranchMainUnion pins that the new ADR's ID is
// allocated over the BRANCH+MAIN union: main has ADR-0050 but the worktree
// only has ADR-0007, so the worktree create must produce ADR-0051
// (max(branch,main)+1), NOT ADR-0008 (a branch-only allocation that would
// collide with the main-only ADR-0050). RED if NextID is computed over only
// the worktree-local root. (mindspec-8lzq)
func TestADRCreate_NextIDOverBranchMainUnion(t *testing.T) {
	mainRoot, wtRoot := setupWorktreePair(t)
	writeADR(t, mainRoot, "ADR-0050-main-only-decision.md")
	writeADR(t, wtRoot, "ADR-0007-branch-decision.md")
	chdir(t, wtRoot)

	if err := adrCreateCmd.RunE(adrCreateCmd, []string{"Next union decision"}); err != nil {
		t.Fatalf("adr create: %v", err)
	}

	// The new file lands in the worktree; its ID must be 0051, not 0008.
	if !fileExists(filepath.Join(adrDirFor(wtRoot), "ADR-0051.md")) {
		all, _ := filepath.Glob(filepath.Join(adrDirFor(wtRoot), "ADR-*.md"))
		t.Fatalf("expected ADR-0051.md in worktree (union of main ADR-0050 + branch ADR-0007), got %v", all)
	}

	// Defensively assert no colliding ADR-0008 was allocated.
	if fileExists(filepath.Join(adrDirFor(wtRoot), "ADR-0008.md")) {
		t.Fatalf("NextID collided: allocated ADR-0008 over only the worktree-local root instead of the branch+main union")
	}
}

// TestADRCreate_MainCheckout pins that `adr create` from a plain (non-worktree)
// checkout still writes into that checkout — FindLocalRoot == FindRoot there,
// so the union numbering is a no-op and behavior is unchanged.
func TestADRCreate_MainCheckout(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeADR(t, root, "ADR-0003-existing.md")
	chdir(t, root)

	if err := adrCreateCmd.RunE(adrCreateCmd, []string{"Main checkout decision"}); err != nil {
		t.Fatalf("adr create: %v", err)
	}

	if !fileExists(filepath.Join(adrDirFor(root), "ADR-0004.md")) {
		all, _ := filepath.Glob(filepath.Join(adrDirFor(root), "ADR-*.md"))
		t.Fatalf("expected ADR-0004.md in main checkout, got %v", all)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
