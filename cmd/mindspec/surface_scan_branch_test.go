//go:build surfacescan

package main

import (
	"os"
	"strings"
	"testing"
)

const surfaceMainRef = "origin/main"

func TestS134SurfaceBranch_NoInternalRefs(t *testing.T) {
	repo := repoRootFromTestDir(t)
	base := os.Getenv("MINDSPEC_SURFACE_BASE")
	if base == "" {
		base = surfaceDefaultBase(t, repo)
	}
	scanner := surfaceScanner{repo: repo, floor: surfaceSpecBase}
	if err := scanner.scan(base, "HEAD", surfaceMainRef); err != nil {
		t.Fatal(err)
	}
}

// surfaceDefaultBase is the merge base of HEAD and the main ref, so lines
// that a merge of main brings in are not scanned as branch lines. When the
// merge base is older than the pinned history floor, the floor applies.
func surfaceDefaultBase(t *testing.T, repo string) string {
	t.Helper()
	out, err := surfaceGit(repo, "merge-base", "HEAD", surfaceMainRef)
	if err != nil {
		t.Fatalf("cannot find the merge base of HEAD and %s; run git fetch origin main or a full-history fetch: %v", surfaceMainRef, err)
	}
	base := strings.TrimSpace(out)
	if _, err := surfaceGit(repo, "merge-base", "--is-ancestor", surfaceSpecBase, base); err != nil {
		return surfaceSpecBase
	}
	return base
}
