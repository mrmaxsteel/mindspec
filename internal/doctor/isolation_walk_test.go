package doctor

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func TestWalks_SkipIsolationRoot(t *testing.T) {
	root := t.TempDir()
	writeFileAt(t, root, ".mindspec/config.yaml", "panel: {isolation_root: project-docs/scratch}\n")
	writeFileAt(t, root, "project-docs/scratch/slot/source.go", "package scratch\n")
	writeFileAt(t, root, "project-docs/scratch/slot/review.md", "[bad](missing.md)\n")
	writeFileAt(t, root, "project-docs/live.md", "[bad](missing.md)\n")
	writeFileAt(t, root, ".mindspec/domains/x/OWNERSHIP.yaml", "paths:\n  - project-docs/scratch/**\n")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	files := enumerateWorkspaceFiles(root)
	if manifestResolvesAny(files, []string{"project-docs/scratch/**"}) {
		t.Fatalf("ownership sees scratch: %v", files)
	}
	if !manifestResolvesAny(files, []string{"project-docs/live.md"}) {
		t.Fatal("ownership positive control missing")
	}
	var report Report
	checkOwnershipManifests(&report, root)
	foundDead := false
	for _, check := range report.Checks {
		if check.Status == Warn && check.Name == manifestCheckName("x") {
			foundDead = true
		}
	}
	if !foundDead {
		t.Fatalf("scratch masked dead ownership manifest: %+v", report.Checks)
	}
	links, err := CheckMovedTreeLinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []DanglingLink{{File: "project-docs/live.md", Target: "missing.md", Resolved: "project-docs/missing.md"}}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("links leak or missing positive control: %+v", links)
	}
	// Removing the configured exclusion makes both scratch probes visible.
	writeFileAt(t, root, ".mindspec/config.yaml", "panel: {isolation_root: other-scratch}\n")
	config.ResetCache()
	if !manifestResolvesAny(enumerateWorkspaceFiles(root), []string{"project-docs/scratch/**"}) {
		t.Fatal("ownership negative control failed")
	}
	links, err = CheckMovedTreeLinks(root)
	if err != nil || len(links) != 2 {
		t.Fatalf("links negative control: %+v %v", links, err)
	}
	if filepath.IsAbs(links[0].File) {
		t.Fatal("links must use relative paths")
	}
}
