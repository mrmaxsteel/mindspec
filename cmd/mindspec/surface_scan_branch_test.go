//go:build surfacescan

package main

import (
	"os"
	"testing"
)

func TestS134SurfaceBranch_NoInternalRefs(t *testing.T) {
	base := os.Getenv("MINDSPEC_SURFACE_BASE")
	if base == "" {
		base = surfaceSpecBase
	}
	scanner := surfaceScanner{repo: repoRootFromTestDir(t), floor: surfaceSpecBase}
	if err := scanner.scan(base, "HEAD", "origin/main"); err != nil {
		t.Fatal(err)
	}
}
