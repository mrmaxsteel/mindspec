package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// TestBlockingRefs_OnlyUnmergedPreFlatten pins the Req 11 block predicate:
// block ⟺ unmerged AND pre-flatten. Merged refs and post-flatten refs never
// block.
func TestBlockingRefs_OnlyUnmergedPreFlatten(t *testing.T) {
	cands := []RefCandidate{
		{Name: "spec/pre", Merged: false, PreFlatten: true},   // BLOCKS
		{Name: "spec/merged", Merged: true, PreFlatten: true}, // merged → no
		{Name: "spec/flat", Merged: false, PreFlatten: false}, // post-flatten → no
		{Name: "spec/done", Merged: true, PreFlatten: false},  // no
		{Name: "spec/pre2", Merged: false, PreFlatten: true},  // BLOCKS
	}
	blocking := blockingRefs(cands)
	if len(blocking) != 2 {
		t.Fatalf("expected 2 blocking refs, got %d: %+v", len(blocking), blocking)
	}
	names := map[string]bool{}
	for _, b := range blocking {
		names[b.Name] = true
	}
	if !names["spec/pre"] || !names["spec/pre2"] {
		t.Errorf("wrong blocking set: %+v", blocking)
	}
}

// TestClassifyRefs_TolerateLockedAndForks asserts the precondition does NOT
// count locked agent worktrees or external-fork refs as block-candidates
// (AC16 tolerate half) while a normal unmerged local branch IS a candidate.
func TestClassifyRefs_TolerateLockedAndForks(t *testing.T) {
	locals := []string{"main", "spec/106", "bead/locked-wt"}
	remotes := []string{"origin/main", "origin/spec/106", "fork/feature", "someuser/experiment"}
	locked := map[string]bool{"bead/locked-wt": true}

	candidates, tolerated := classifyRefs(locals, remotes, "main", locked)

	hasCand := func(n string) bool {
		for _, c := range candidates {
			if c == n {
				return true
			}
		}
		return false
	}
	hasTol := func(n string) bool {
		for _, c := range tolerated {
			if c == n {
				return true
			}
		}
		return false
	}

	// The target (main / origin/main) is excluded entirely.
	if hasCand("main") || hasCand("origin/main") {
		t.Error("target ref must not be a block-candidate")
	}
	// A normal unmerged local branch is a candidate.
	if !hasCand("spec/106") {
		t.Error("spec/106 should be a block-candidate")
	}
	// Locked agent worktree → tolerated, not a candidate.
	if hasCand("bead/locked-wt") || !hasTol("bead/locked-wt") {
		t.Error("locked worktree branch must be tolerated, not a candidate")
	}
	// External forks (non-origin remotes) → tolerated.
	if hasCand("fork/feature") || !hasTol("fork/feature") {
		t.Error("fork/feature must be tolerated, not a candidate")
	}
	if hasCand("someuser/experiment") || !hasTol("someuser/experiment") {
		t.Error("someuser/experiment fork must be tolerated, not a candidate")
	}
}

// fakeGit is a minimal GitOps fake for the precondition discovery scan.
type fakeGit struct {
	locals    []string
	remotes   []string
	remoteErr error
	status    string
	// mergeBase[ref] and refSha[ref] drive merged detection; sig[mergeBase]
	// drives the pre-flatten fingerprint.
	mergeBase map[string]string
	refSha    map[string]string
	sig       map[string][]string // mergeBase sha -> .mindspec child dirs
}

func (f *fakeGit) RevParseRef(_, ref string) (string, error)     { return f.refSha[ref], nil }
func (f *fakeGit) Status(string) (string, error)                 { return f.status, nil }
func (f *fakeGit) GitMv(string, string, string) error            { return nil }
func (f *fakeGit) ResetHard(string, string) error                { return nil }
func (f *fakeGit) CleanForce(string) error                       { return nil }
func (f *fakeGit) CommitPaths(string, string, []string) error    { return nil }
func (f *fakeGit) LocalBranchRefs(string) ([]string, error)      { return f.locals, nil }
func (f *fakeGit) RemoteTrackingRefs(string) ([]string, error)   { return f.remotes, f.remoteErr }
func (f *fakeGit) MergeBase(_, b string) (string, error)         { return f.mergeBase[b], nil }
func (f *fakeGit) TreeDirsAtRef(ref, _ string) ([]string, error) { return f.sig[ref], nil }

// TestCheckPrecondition_BlocksUnmergedPreFlattenBranch asserts the discovery
// scan BLOCKS on an unmerged pre-flatten local branch (AC16 block half).
func TestCheckPrecondition_BlocksUnmergedPreFlattenBranch(t *testing.T) {
	f := &fakeGit{
		locals:  []string{"main", "spec/old"},
		remotes: []string{"origin/main"},
		mergeBase: map[string]string{
			"spec/old":    "base-canonical",
			"origin/main": "base-flat",
		},
		refSha: map[string]string{
			"spec/old":    "tip-old", // != merge-base → unmerged
			"origin/main": "base-flat",
		},
		sig: map[string][]string{
			"base-canonical": {"docs"},                            // pre-flatten
			"base-flat":      {"specs", "adr", "domains", "core"}, // flat
		},
	}
	res, err := CheckPrecondition(f, "/repo", PreconditionOptions{Target: "main"})
	if err != nil {
		t.Fatalf("CheckPrecondition: %v", err)
	}
	if len(res.Blocking) != 1 || res.Blocking[0].Name != "spec/old" {
		t.Fatalf("expected spec/old to block, got %+v", res.Blocking)
	}
}

// TestCheckPrecondition_TolerateForkAndPostFlatten asserts a post-flatten
// unmerged branch and an external fork do NOT block (AC16 tolerate half).
func TestCheckPrecondition_TolerateForkAndPostFlatten(t *testing.T) {
	f := &fakeGit{
		locals:  []string{"main", "spec/new"},
		remotes: []string{"origin/main", "fork/experiment"},
		mergeBase: map[string]string{
			"spec/new":    "base-flat",
			"origin/main": "base-flat",
		},
		refSha: map[string]string{
			"spec/new":    "tip-new", // unmerged...
			"origin/main": "base-flat",
		},
		sig: map[string][]string{
			"base-flat": {"specs", "adr", "domains", "core"}, // ...but post-flatten → no block
		},
	}
	res, err := CheckPrecondition(f, "/repo", PreconditionOptions{Target: "main"})
	if err != nil {
		t.Fatalf("CheckPrecondition: %v", err)
	}
	if len(res.Blocking) != 0 {
		t.Errorf("expected no blocking refs, got %+v", res.Blocking)
	}
	tolerated := strings.Join(res.Tolerated, ",")
	if !strings.Contains(tolerated, "fork/experiment") {
		t.Errorf("fork/experiment should be tolerated, got %q", tolerated)
	}
}

// TestCheckPrecondition_OfflineWarns asserts that with no remote-tracking refs
// the scan degrades and WARNS (does not silently pass) (AC16 offline half).
func TestCheckPrecondition_OfflineWarns(t *testing.T) {
	f := &fakeGit{
		locals:  []string{"main"},
		remotes: nil,
	}
	res, err := CheckPrecondition(f, "/repo", PreconditionOptions{Target: "main", Offline: true})
	if err != nil {
		t.Fatalf("CheckPrecondition: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected an offline warning, got none")
	}
}

// TestCheckPrecondition_DirtyTreeBlocks asserts a dirty idle working tree is
// refused before any mutation (Req 11).
func TestCheckPrecondition_DirtyTreeBlocks(t *testing.T) {
	f := &fakeGit{
		locals:  []string{"main"},
		remotes: []string{"origin/main"},
		status:  " M some/file.go\n",
	}
	_, err := CheckPrecondition(f, "/repo", PreconditionOptions{Target: "main", RequireCleanTree: true})
	if err == nil {
		t.Fatal("expected a dirty-tree refusal, got nil")
	}
	if !strings.Contains(err.Error(), "dirty working tree") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCheckPrecondition_DirtyTreeIgnoresOperationalResidue asserts the
// clean-tree check ignores the mover's own run-state/lineage residue.
func TestCheckPrecondition_DirtyTreeIgnoresOperationalResidue(t *testing.T) {
	f := &fakeGit{
		locals:  []string{"main"},
		remotes: []string{"origin/main"},
		status:  "?? .mindspec/migrations/run-1/state.json\n?? .mindspec/lineage/manifest.json\n",
	}
	_, err := CheckPrecondition(f, "/repo", PreconditionOptions{Target: "main", RequireCleanTree: true})
	if err != nil {
		t.Errorf("operational residue should not count as dirty: %v", err)
	}
}

// TestLayoutPackageIsOwned asserts internal/layout/** is claimed in the
// workflow OWNERSHIP.yaml so the net-new mover package does not trip
// adr-divergence-unowned at complete (AC21 ownership half). The test resolves
// the repo-root manifest relative to this package directory.
func TestLayoutPackageIsOwned(t *testing.T) {
	manifest := filepath.Join("..", "..", ".mindspec", "docs", "domains", "workflow", "OWNERSHIP.yaml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read OWNERSHIP.yaml: %v", err)
	}
	if !strings.Contains(string(data), "internal/layout/**") {
		t.Errorf("workflow OWNERSHIP.yaml does not claim internal/layout/**:\n%s", data)
	}
}

// ensure the executor satisfies the GitOps surface the mover/precondition use.
var _ GitOps = (*executor.MindspecExecutor)(nil)
var _ GitOps = (*executor.MockExecutor)(nil)
