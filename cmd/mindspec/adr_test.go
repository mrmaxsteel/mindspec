package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
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

// writeADRWithBody writes an ADR file with the given full body under root's
// ADR directory (so show/list can render its Status/Domain(s)).
func writeADRWithBody(t *testing.T, root, name, body string) {
	t.Helper()
	dir := adrDirFor(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir adr dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
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
	// R5(a) (spec 123): create now emits a slugged filename, so match by
	// canonical-number prefix rather than the exact bare "ADR-0051.md".
	if matches, _ := filepath.Glob(filepath.Join(adrDirFor(wtRoot), "ADR-0051-*.md")); len(matches) != 1 {
		all, _ := filepath.Glob(filepath.Join(adrDirFor(wtRoot), "ADR-*.md"))
		t.Fatalf("expected exactly one ADR-0051-*.md in worktree (union of main ADR-0050 + branch ADR-0007), got %v", all)
	}

	// Defensively assert no colliding ADR-0008 was allocated.
	if matches, _ := filepath.Glob(filepath.Join(adrDirFor(wtRoot), "ADR-0008*.md")); len(matches) != 0 {
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

	// R5(a) (spec 123): create now emits a slugged filename, so match by
	// canonical-number prefix rather than the exact bare "ADR-0004.md".
	if matches, _ := filepath.Glob(filepath.Join(adrDirFor(root), "ADR-0004-*.md")); len(matches) != 1 {
		all, _ := filepath.Glob(filepath.Join(adrDirFor(root), "ADR-*.md"))
		t.Fatalf("expected exactly one ADR-0004-*.md in main checkout, got %v", all)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// adrWithDomains returns a minimal ADR body carrying a Status + Domain(s)
// line so show/list rendering can be asserted.
func adrWithDomains(id, title, domains string) string {
	return "# " + id + ": " + title + "\n\n" +
		"- **Date**: 2026-06-01\n" +
		"- **Status**: Accepted\n" +
		"- **Domain(s)**: " + domains + "\n" +
		"- **Supersedes**: n/a\n" +
		"- **Superseded-by**: n/a\n\n" +
		"## Decision\nX.\n"
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// fn printed. The adr show/list commands write via fmt.Print* to os.Stdout.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return buf.String(), runErr
}

// TestAdrShowWorktree_FindsWorktreeLocalADR pins R3 AC1: an ADR that exists
// ONLY in the worktree-local .mindspec/docs/adr/ is found and rendered with
// its Domain(s) by `adr show` run from inside the worktree. RED on the
// pre-fix FindRoot path (which resolves the worktree back to main, where the
// ADR is absent → store.Get errors "not found"). (mindspec-3cfr / R3)
func TestAdrShowWorktree_FindsWorktreeLocalADR(t *testing.T) {
	_, wtRoot := setupWorktreePair(t)
	writeADRWithBody(t, wtRoot, "ADR-0042-worktree-only.md",
		adrWithDomains("ADR-0042", "Worktree Only Decision", "workflow, validation"))
	chdir(t, wtRoot)

	out, err := captureStdout(t, func() error {
		return adrShowCmd.RunE(adrShowCmd, []string{"ADR-0042"})
	})
	if err != nil {
		t.Fatalf("adr show ADR-0042 (worktree-local): %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("workflow")) {
		t.Fatalf("adr show output missing Domain(s) 'workflow'; got:\n%s", out)
	}
}

// TestAdrListWorktree_ListsWorktreeLocalADR pins R3 AC1 for `adr list`: a
// worktree-local-only ADR appears in the list run from inside the worktree.
// RED on the pre-fix FindRoot path (resolves to main; ADR absent → "No ADRs
// found."). (mindspec-3cfr / R3)
func TestAdrListWorktree_ListsWorktreeLocalADR(t *testing.T) {
	_, wtRoot := setupWorktreePair(t)
	writeADRWithBody(t, wtRoot, "ADR-0042-worktree-only.md",
		adrWithDomains("ADR-0042", "Worktree Only Decision", "workflow"))
	chdir(t, wtRoot)

	out, err := captureStdout(t, func() error {
		return adrListCmd.RunE(adrListCmd, nil)
	})
	if err != nil {
		t.Fatalf("adr list (worktree-local): %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("ADR-0042")) {
		t.Fatalf("adr list output missing worktree-local ADR-0042; got:\n%s", out)
	}
}

// TestAdrShowWorktree_StillFindsMainOnlyADR pins that the overlay keeps
// main-only ADRs visible from the worktree (branch unioned over main): an ADR
// present only in the MAIN checkout is still found by `adr show` run from the
// worktree. (mindspec-3cfr / R3 — no regression for main-only ADRs)
func TestAdrShowWorktree_StillFindsMainOnlyADR(t *testing.T) {
	mainRoot, wtRoot := setupWorktreePair(t)
	writeADRWithBody(t, mainRoot, "ADR-0010-main-only.md",
		adrWithDomains("ADR-0010", "Main Only Decision", "core"))
	chdir(t, wtRoot)

	out, err := captureStdout(t, func() error {
		return adrShowCmd.RunE(adrShowCmd, []string{"ADR-0010"})
	})
	if err != nil {
		t.Fatalf("adr show ADR-0010 (main-only, from worktree): %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("core")) {
		t.Fatalf("adr show output missing main-only ADR Domain(s) 'core'; got:\n%s", out)
	}
}

// TestAdrShowMainCheckout_StillWorks pins that show from a plain (non-worktree)
// checkout is unchanged: FindLocalRoot == FindRoot there, so the overlay is a
// no-op over the same root and the ADR is found. (mindspec-3cfr / R3 — main
// behavior preserved)
func TestAdrShowMainCheckout_StillWorks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeADRWithBody(t, root, "ADR-0005-plain.md",
		adrWithDomains("ADR-0005", "Plain Checkout Decision", "core"))
	chdir(t, root)

	out, err := captureStdout(t, func() error {
		return adrShowCmd.RunE(adrShowCmd, []string{"ADR-0005"})
	})
	if err != nil {
		t.Fatalf("adr show ADR-0005 (plain checkout): %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("core")) {
		t.Fatalf("adr show output missing Domain(s) 'core'; got:\n%s", out)
	}
}

// resetAdrCreateSlugFlag zeroes the --slug flag state between invocations
// so a prior test's --slug value (and its Changed bit) never leaks into a
// later test that omits the flag.
func resetAdrCreateSlugFlag(t *testing.T) {
	t.Helper()
	f := adrCreateCmd.Flags().Lookup("slug")
	if f == nil {
		t.Fatal("adrCreateCmd has no --slug flag")
	}
	_ = f.Value.Set(f.DefValue)
	f.Changed = false
}

// TestAdrCreate_SluggedFilename pins AC-8 (spec 123 R5(a)): a fresh
// workspace's `adr create` with a multi-word title writes a SLUGGED
// filename derived from the title, not the bare "ADR-0001.md" the
// pre-123 CLI wrote. RED on revert.
func TestAdrCreate_SluggedFilename(t *testing.T) {
	resetAdrCreateSlugFlag(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)

	if err := adrCreateCmd.RunE(adrCreateCmd, []string{"Integrate at contracts, not tools"}); err != nil {
		t.Fatalf("adr create: %v", err)
	}

	const wantStem = "ADR-0001-integrate-at-contracts-not-tools"
	wantPath := filepath.Join(adrDirFor(root), wantStem+".md")
	if !fileExists(wantPath) {
		all, _ := filepath.Glob(filepath.Join(adrDirFor(root), "ADR-*.md"))
		t.Fatalf("expected %s, got %v", wantPath, all)
	}
	if err := idvalidate.ADRID(wantStem); err != nil {
		t.Fatalf("computed stem %q fails idvalidate.ADRID: %v", wantStem, err)
	}

	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# ADR-0001:") {
		t.Errorf("expected heading '# ADR-0001:', got:\n%s", data)
	}
}

// TestAdrCreate_SlugFlagOverride pins AC-8's `--slug` override: an
// explicit --slug value replaces title-derived slugging.
func TestAdrCreate_SlugFlagOverride(t *testing.T) {
	resetAdrCreateSlugFlag(t)
	t.Cleanup(func() { resetAdrCreateSlugFlag(t) })
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)

	if err := adrCreateCmd.Flags().Set("slug", "my-custom-slug"); err != nil {
		t.Fatal(err)
	}
	if err := adrCreateCmd.RunE(adrCreateCmd, []string{"A totally different title"}); err != nil {
		t.Fatalf("adr create --slug: %v", err)
	}

	want := filepath.Join(adrDirFor(root), "ADR-0001-my-custom-slug.md")
	if !fileExists(want) {
		all, _ := filepath.Glob(filepath.Join(adrDirFor(root), "ADR-*.md"))
		t.Fatalf("expected %s (--slug override), got %v", want, all)
	}
}

// TestAdrCreate_PunctuationOnlyTitleFallsBackToBare pins AC-8's bare
// fallback: a title with no alphanumeric characters derives an empty
// slug, so create writes the bare "ADR-NNNN.md" form instead of an
// invalid "ADR-NNNN-.md".
func TestAdrCreate_PunctuationOnlyTitleFallsBackToBare(t *testing.T) {
	resetAdrCreateSlugFlag(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)

	if err := adrCreateCmd.RunE(adrCreateCmd, []string{"!!! ??? ---"}); err != nil {
		t.Fatalf("adr create: %v", err)
	}

	want := filepath.Join(adrDirFor(root), "ADR-0001.md")
	if !fileExists(want) {
		all, _ := filepath.Glob(filepath.Join(adrDirFor(root), "ADR-*.md"))
		t.Fatalf("expected bare %s (punctuation-only title), got %v", want, all)
	}
}

// TestAdrCreate_SlugFlagInvalidShapeRefused pins that a malformed --slug
// value (not lowercase kebab-case) is refused with a recovery line,
// never silently corrected into something else.
func TestAdrCreate_SlugFlagInvalidShapeRefused(t *testing.T) {
	resetAdrCreateSlugFlag(t)
	t.Cleanup(func() { resetAdrCreateSlugFlag(t) })
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)

	if err := adrCreateCmd.Flags().Set("slug", "Not A Valid Slug!"); err != nil {
		t.Fatal(err)
	}
	err := adrCreateCmd.RunE(adrCreateCmd, []string{"Some Title"})
	if err == nil {
		t.Fatal("expected error for malformed --slug, got nil")
	}
	if !strings.Contains(err.Error(), "recovery:") {
		t.Errorf("expected a recovery line, got: %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(adrDirFor(root), "ADR-*.md")); len(matches) != 0 {
		t.Errorf("malformed --slug must write nothing, got: %v", matches)
	}
}
