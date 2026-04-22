package next

import (
	"fmt"
	"reflect"
	"testing"
)

// --- classifyDirty ---

func TestClassifyDirty_OnlyArtifact(t *testing.T) {
	artifact, user := classifyDirty([]string{".beads/issues.jsonl"})
	if !reflect.DeepEqual(artifact, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v, want [.beads/issues.jsonl]", artifact)
	}
	if len(user) != 0 {
		t.Errorf("userDirt = %v, want empty", user)
	}
}

func TestClassifyDirty_OnlyUser(t *testing.T) {
	artifact, user := classifyDirty([]string{"foo.txt", "internal/next/guard.go"})
	if len(artifact) != 0 {
		t.Errorf("artifactDirt = %v, want empty", artifact)
	}
	if !reflect.DeepEqual(user, []string{"foo.txt", "internal/next/guard.go"}) {
		t.Errorf("userDirt = %v", user)
	}
}

func TestClassifyDirty_Mixed(t *testing.T) {
	artifact, user := classifyDirty([]string{".beads/issues.jsonl", "foo.txt"})
	if !reflect.DeepEqual(artifact, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v", artifact)
	}
	if !reflect.DeepEqual(user, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v", user)
	}
}

func TestClassifyDirty_SkipsEmpty(t *testing.T) {
	artifact, user := classifyDirty([]string{"", "  ", ".beads/issues.jsonl"})
	if !reflect.DeepEqual(artifact, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v", artifact)
	}
	if len(user) != 0 {
		t.Errorf("userDirt = %v, want empty", user)
	}
}

func TestClassifyDirty_NearMissIsUserDirt(t *testing.T) {
	// Whole-path equality: a similarly-named file is NOT treated as an
	// artifact. Prevents a casual rename (e.g. `beads/issues.jsonl` without
	// the dot prefix, or a `.beads/issues.jsonl.bak`) from silently
	// bypassing the guard.
	artifact, user := classifyDirty([]string{
		"beads/issues.jsonl",
		".beads/issues.jsonl.bak",
		"other/.beads/issues.jsonl",
	})
	if len(artifact) != 0 {
		t.Errorf("artifactDirt = %v, want empty (only exact-path matches count)", artifact)
	}
	if len(user) != 3 {
		t.Errorf("userDirt count = %d, want 3", len(user))
	}
}

// --- parsePorcelain ---

func TestParsePorcelain_SingleFile(t *testing.T) {
	got := parsePorcelain(" M .beads/issues.jsonl\n")
	if !reflect.DeepEqual(got, []string{".beads/issues.jsonl"}) {
		t.Errorf("got %v", got)
	}
}

func TestParsePorcelain_MultipleFiles(t *testing.T) {
	input := " M .beads/issues.jsonl\n?? newfile.txt\n M internal/next/guard.go\n"
	got := parsePorcelain(input)
	want := []string{".beads/issues.jsonl", "newfile.txt", "internal/next/guard.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParsePorcelain_RenameTakesNewPath(t *testing.T) {
	got := parsePorcelain("R  old/name.go -> new/name.go\n")
	if !reflect.DeepEqual(got, []string{"new/name.go"}) {
		t.Errorf("got %v, want [new/name.go]", got)
	}
}

func TestParsePorcelain_EmptyAndShortLinesIgnored(t *testing.T) {
	got := parsePorcelain("\n \n M foo.txt\n")
	if !reflect.DeepEqual(got, []string{"foo.txt"}) {
		t.Errorf("got %v", got)
	}
}

func TestParsePorcelain_StagedAndUnstaged(t *testing.T) {
	// XY format: X=index status, Y=worktree status. Both should yield the path.
	got := parsePorcelain("MM .beads/issues.jsonl\nAM new.txt\n")
	want := []string{".beads/issues.jsonl", "new.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- CheckDirtyTree ---

// fakeGuard captures the calls made to the guard's injected helpers so tests
// can assert both the decision and the side effects (bd export invocation,
// re-check count).
type fakeGuard struct {
	porcelainResponses []string // popped front-first per call
	porcelainErr       error
	porcelainCalls     int
	exportCalled       bool
	exportErr          error
	exportRoot         string
}

func (f *fakeGuard) install(t *testing.T) {
	t.Helper()
	origStatus := statusPorcelainFn
	origExport := exportBeadsFn
	t.Cleanup(func() {
		statusPorcelainFn = origStatus
		exportBeadsFn = origExport
	})
	statusPorcelainFn = func(cwd string) (string, error) {
		if f.porcelainErr != nil {
			return "", f.porcelainErr
		}
		if f.porcelainCalls >= len(f.porcelainResponses) {
			f.porcelainCalls++
			return "", nil
		}
		resp := f.porcelainResponses[f.porcelainCalls]
		f.porcelainCalls++
		return resp, nil
	}
	exportBeadsFn = func(workdir string) error {
		f.exportCalled = true
		f.exportRoot = workdir
		return f.exportErr
	}
}

func TestCheckDirtyTree_Clean(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{""}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty", userDirt)
	}
	if g.exportCalled {
		t.Error("export should not be called when tree is clean")
	}
	if g.porcelainCalls != 1 {
		t.Errorf("porcelain calls = %d, want 1 (no re-check on clean)", g.porcelainCalls)
	}
}

func TestCheckDirtyTree_OnlyArtifact_Proceeds(t *testing.T) {
	// Artifact dirty → run bd export → re-check returns clean → proceed.
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n",
		"",
	}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty (proceed)", userDirt)
	}
	if !g.exportCalled {
		t.Error("export should run when artifact is dirty")
	}
	if g.exportRoot != "/repo" {
		t.Errorf("export called with root %q, want /repo", g.exportRoot)
	}
	if g.porcelainCalls != 2 {
		t.Errorf("porcelain calls = %d, want 2 (pre- and post-export)", g.porcelainCalls)
	}
}

func TestCheckDirtyTree_ArtifactPlusUserDirt_Aborts(t *testing.T) {
	// Mixed dirt — the export may clean up the artifact but foo.txt remains.
	// Guard must surface foo.txt so the caller can abort.
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n M foo.txt\n",
		" M foo.txt\n",
	}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(userDirt, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v, want [foo.txt]", userDirt)
	}
	if !g.exportCalled {
		t.Error("export should still run when artifact is among dirty paths")
	}
}

func TestCheckDirtyTree_OnlyUserDirt_SkipsExport(t *testing.T) {
	// No artifact in dirty set → no reason to run bd export (avoids an extra
	// subprocess when user dirt is going to block anyway).
	g := &fakeGuard{porcelainResponses: []string{" M foo.txt\n"}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(userDirt, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v", userDirt)
	}
	if g.exportCalled {
		t.Error("export should NOT run when only user dirt is present")
	}
	if g.porcelainCalls != 1 {
		t.Errorf("porcelain calls = %d, want 1 (no re-check needed)", g.porcelainCalls)
	}
}

func TestCheckDirtyTree_ArtifactDirtSurvivesExport(t *testing.T) {
	// Legitimate Dolt changes: bd export doesn't clean the diff (the JSONL
	// really has changed). The guard still proceeds — the artifact is the
	// bead's own first-commit payload (ADR-0025).
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n",
		" M .beads/issues.jsonl\n",
	}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty (still proceed on legitimate artifact diff)", userDirt)
	}
}

func TestCheckDirtyTree_ExportFailurePropagates(t *testing.T) {
	g := &fakeGuard{
		porcelainResponses: []string{" M .beads/issues.jsonl\n"},
		exportErr:          fmt.Errorf("bd export boom"),
	}
	g.install(t)

	_, err := CheckDirtyTree("/repo", "/repo")
	if err == nil {
		t.Fatal("expected error when bd export fails")
	}
}

func TestCheckDirtyTree_PorcelainFailurePropagates(t *testing.T) {
	g := &fakeGuard{porcelainErr: fmt.Errorf("git status boom")}
	g.install(t)

	_, err := CheckDirtyTree("/repo", "/repo")
	if err == nil {
		t.Fatal("expected error when git status fails")
	}
}
