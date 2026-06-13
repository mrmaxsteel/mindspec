package complete

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// readStubMergeExecutor wraps a REAL MindspecExecutor (so the gate's
// git reads — MergeBase, ChangedFiles, FileAtRefOrAbsent, TreeDirsAtRef
// — run against a real temp repo) but makes the terminal bead→spec
// merge a no-op, so the e2e test exercises the ref-anchored gates
// without standing up worktrees.
type readStubMergeExecutor struct {
	executor.Executor
	completeCalled bool
}

func (e *readStubMergeExecutor) CompleteBead(beadID, specBranch, msg string) error {
	e.completeCalled = true
	return nil
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// TestCompleteRun_RefAnchoredOwnershipClaimPassesFromMainRoot is the
// load-bearing mindspec-vvs9 proof (spec 095 AC #1). A synthetic bead
// branch commits an OWNERSHIP claim for a file it also changes; running
// `complete.Run` FROM THE MAIN ROOT — where the claim is absent on disk
// — the per-bead doc-sync + ADR-divergence gates PASS with NO
// `--override-adr` / `--allow-doc-skew`, and NO override / doc-skew
// metadata is recorded on the bead.
//
// RED-on-revert: reverting the gate's OWNERSHIP read to the ambient
// working tree (os.ReadFile(root) / listDomainDirs(root)) makes the
// claim invisible at main → internal/widget/widget.go surfaces as
// `adr-divergence-unowned`, the gate blocks, and complete.Run errors.
func TestCompleteRun_RefAnchoredOwnershipClaimPassesFromMainRoot(t *testing.T) {
	saveAndRestore(t)

	const specID = "095-refanchor"
	const beadID = "mindspec-095rf.1"
	specBranch := "spec/" + specID
	beadBranch := "bead/" + beadID

	root := t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")

	// Base commit on main: spec.md (impacted domain widget), plan.md
	// (cites Accepted ADR-0195 covering widget), and the ADR — all read
	// from disk by the gates. NO widget OWNERSHIP / domain dir here:
	// the claim lives only on the bead branch.
	specDir := ".mindspec/docs/specs/" + specID
	writeFile(t, root, specDir+"/spec.md",
		"# Spec "+specID+"\n\n## Impacted Domains\n\n- widget\n")
	writeFile(t, root, specDir+"/plan.md",
		"---\nspec_id: "+specID+"\nstatus: Approved\nbead_ids:\n  - "+beadID+
			"\nadr_citations:\n  - id: ADR-0195\n---\n\n# Plan\n")
	writeFile(t, root, ".mindspec/docs/adr/ADR-0195.md",
		"# ADR-0195: Widget\n\n"+
			"- **Date**: 2026-01-01\n"+
			"- **Status**: Accepted\n"+
			"- **Domain(s)**: widget\n"+
			"- **Supersedes**: n/a\n"+
			"- **Superseded-by**: n/a\n\n"+
			"## Decision\nTest fixture.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")

	// Spec branch at the fork point.
	gitRun(t, root, "branch", specBranch)

	// Bead branch: commit the OWNERSHIP claim AND the source file it
	// covers, then return to main so the claim is ABSENT on disk.
	gitRun(t, root, "checkout", "-q", "-b", beadBranch, specBranch)
	writeFile(t, root, ".mindspec/docs/domains/widget/OWNERSHIP.yaml",
		"paths:\n  - internal/widget/**\n")
	writeFile(t, root, "internal/widget/widget.go",
		"package widget\n\nfunc New() {}\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "impl: widget + ownership claim")
	gitRun(t, root, "checkout", "-q", "main")

	// The claim must NOT be on disk at the main root (proves the gate
	// cannot be reading the working tree).
	if _, err := os.Stat(filepath.Join(root, ".mindspec/docs/domains/widget/OWNERSHIP.yaml")); !os.IsNotExist(err) {
		t.Fatalf("precondition: widget OWNERSHIP must be absent on disk at main root, stat err=%v", err)
	}

	// Lifecycle seams: implement-mode epic, no worktree (canonical
	// beadHead), capture every metadata write.
	stubPhaseEpic(t, specID, "epic-095rf")
	resolveTargetFn = func(r, flag string) (string, error) { return specID, nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(ids ...string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }
	findLocalRootFn = func() (string, error) { return root, nil }

	var metaWrites []map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaWrites = append(metaWrites, updates)
		return nil
	}

	exec := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}

	_, err := Run(root, beadID, specID, "", exec, CompleteOpts{})
	if err != nil {
		t.Fatalf("ref-anchored OWNERSHIP claim must satisfy its own gate from the MAIN ROOT with NO override; got: %v", err)
	}
	if !exec.completeCalled {
		t.Error("expected the terminal CompleteBead to run (gates passed)")
	}

	// AC: ZERO override / doc-skew metadata recorded.
	for _, w := range metaWrites {
		for k := range w {
			if strings.HasPrefix(k, "mindspec_adr_override_") ||
				strings.HasPrefix(k, "mindspec_adr_supersede_") ||
				strings.HasPrefix(k, "mindspec_doc_skew_") {
				t.Errorf("no override/doc-skew metadata may be recorded; found key %q", k)
			}
		}
	}
}
