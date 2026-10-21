package complete

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// landingFixture keeps the claim on the spec branch and the source on the bead.
func landingFixture(t *testing.T, claimAtBase bool) (root, specDir, base string, ex *executor.MindspecExecutor) {
	t.Helper()
	saveAndRestore(t)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	root = setupTempRoot(t)
	writeADRDivergenceFixture(t, root, "132-landing")
	specDir = filepath.Join(root, ".mindspec/docs/specs/132-landing")
	writeFile(t, root, ".mindspec/docs/adr/ADR-9001.md", "# ADR-9001: Ownership\n\n- **Status**: Accepted\n- **Domain(s)**: core\n\n## Decision\nOwn core.\n")
	manifest := ".mindspec/docs/domains/core/OWNERSHIP.yaml"
	if claimAtBase {
		writeFile(t, root, manifest, "paths:\n  - internal/core/**\n")
	} else {
		writeFile(t, root, manifest, "paths:\n  - internal/old/**\n")
	}
	writeFile(t, root, "internal/core/core.go", "package core\n\nconst Value = 0\n")
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "config", "user.email", "test@example.com")
	gitRun(t, root, "config", "user.name", "Test")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "base")
	base = gateRevParse(t, root, "HEAD")
	gitRun(t, root, "branch", "bead/landing-1")
	gitRun(t, root, "checkout", "-qb", "spec/132-landing")
	if claimAtBase {
		writeFile(t, root, manifest, "paths:\n  - internal/old/**\n")
	} else {
		writeFile(t, root, manifest, "paths:\n  - internal/core/**\n")
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "spec ownership")
	gitRun(t, root, "checkout", "-q", "bead/landing-1")
	writeFile(t, root, "internal/core/core.go", "package core\n\nconst Value = 1\n")
	writeFile(t, root, ".mindspec/docs/domains/core/architecture.md", "Core owns this source.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "bead source")
	ex = executor.NewMindspecExecutor(root)
	return
}

func TestCommittedRangeGate_SpecTipClaimOwns(t *testing.T) {
	root, specDir, base, ex := landingFixture(t, false)
	if err := runCommittedRangeGate(root, "landing-1", "bead/landing-1", specDir, base, ex, CompleteOpts{}, io.Discard); err != nil {
		t.Fatalf("landing claim must own source: %v", err)
	}
}

func TestCommittedRangeGate_RevokedAtSpecTipRefuses(t *testing.T) {
	root, specDir, base, ex := landingFixture(t, true)
	err := runCommittedRangeGate(root, "landing-1", "bead/landing-1", specDir, base, ex, CompleteOpts{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "adr-divergence-unowned") || !strings.Contains(err.Error(), "internal/core/core.go") {
		t.Fatalf("revoked landing claim must refuse: %v", err)
	}
}

func TestMaterialize_SpecTipClaimOwns(t *testing.T) {
	root, specDir, base, ex := landingFixture(t, false)
	checkDirtyTreeFn = func(string, string) ([]string, []string, error) { return nil, nil, nil }
	completeMergeMetadataFn = func(string, map[string]interface{}) error { return nil }
	before := gateRevParse(t, root, "HEAD")
	writeFile(t, root, "internal/core/core.go", "package core\n\nconst Value = 3\n")
	err := runMaterialization(materializationInput{
		exec: ex, root: root, beadID: "landing-1", beadHead: "bead/landing-1",
		wtPath: root, specDir: specDir, base: base, commitMsg: "snapshot source", warnWriter: io.Discard,
	})
	if err != nil {
		t.Fatalf("snapshot must use spec-tip claim: %v", err)
	}
	after := gateRevParse(t, root, "HEAD")
	if after == before {
		t.Fatal("snapshot was not materialized")
	}
	data, err := ex.FileAtRef(after, "internal/core/core.go")
	if err != nil || !strings.Contains(string(data), "Value = 3") {
		t.Fatalf("materialized content: %q, %v", data, err)
	}
}

func TestMaterialize_LandingClaimRequiresDomainDocs(t *testing.T) {
	root, specDir, base, ex := landingFixture(t, false)
	// Keep generic docs in the diff, but no core domain docs.
	if err := os.Rename(filepath.Join(root, ".mindspec/docs/domains/core/architecture.md"), filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "generic docs only")
	checkDirtyTreeFn = func(string, string) ([]string, []string, error) { return nil, nil, nil }
	completeMergeMetadataFn = func(string, map[string]interface{}) error { return nil }
	err := runMaterialization(materializationInput{
		exec: ex, root: root, beadID: "landing-1", beadHead: "bead/landing-1",
		wtPath: root, specDir: specDir, base: base, warnWriter: io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "internal-docs") {
		t.Fatalf("landing-owned source without domain docs must refuse with internal-docs: %v", err)
	}
}

func TestCommittedRangeGate_SourceConflictPassesThenResolves(t *testing.T) {
	root, specDir, base, ex := landingFixture(t, false)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	specWorktree, err := workspace.SpecWorktreePath(root, cfg, "132-landing")
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "worktree", "add", specWorktree, "spec/132-landing")
	writeFile(t, specWorktree, "internal/core/core.go", "package core\n\nconst Value = 2\n")
	gitRun(t, specWorktree, "add", "-A")
	gitRun(t, specWorktree, "commit", "-qm", "spec source")
	if err := runCommittedRangeGate(root, "landing-1", "bead/landing-1", specDir, base, ex, CompleteOpts{}, io.Discard); err != nil {
		t.Fatalf("source-only preview conflict must pass ownership: %v", err)
	}
	if err := gitutil.MergeInto(specWorktree, "bead/landing-1"); err == nil {
		t.Fatal("expected a source conflict")
	}
	if !gitutil.MergeInProgress(specWorktree) {
		t.Fatal("merge conflict was not preserved")
	}
	_, _, preservedErr := ex.LandingTree(specWorktree, base, "spec/132-landing", "bead/landing-1")
	if preservedErr == nil {
		t.Fatal("unmerged index must produce a preserved-merge refusal")
	}
	err = runCommittedRangeGate(root, "landing-1", "bead/landing-1", specDir, base, ex, CompleteOpts{ResolveMerge: true}, io.Discard)
	if err == nil {
		t.Fatal("unmerged index must still refuse")
	}
	refusal := err.Error()
	for _, want := range []string{
		"a merge is already in progress",
		"\nconflicted files:\n  internal/core/core.go",
		"\nrecovery: mindspec complete " + idrender.Bead("landing-1") + " --resolve-merge",
	} {
		if !strings.Contains(refusal, want) {
			t.Errorf("unmerged-index refusal lacks %q: %s", want, refusal)
		}
	}
	if strings.Contains(refusal, "<bead>") {
		t.Errorf("unmerged-index refusal has a placeholder: %s", refusal)
	}
	if want := preservedErr.Error() + "\nrecovery: mindspec complete " + idrender.Bead("landing-1") + " --resolve-merge"; refusal != want {
		t.Errorf("preserved refusal changed: got %q, want %q", refusal, want)
	}
	_, recovery, found := strings.Cut(refusal, "\nrecovery: ")
	if !found {
		t.Fatal("unmerged-index refusal has no recovery line")
	}
	if matches := guard.FindFloorMatches(recovery); len(matches) != 0 {
		t.Errorf("preserved-merge recovery matches the destructive floor: %+v", matches)
	}
	writeFile(t, specWorktree, "internal/core/core.go", "package core\n\nconst Value = 4\n")
	gitRun(t, specWorktree, "add", "internal/core/core.go")
	// The index is authoritative even when it revokes the committed claim.
	manifest := ".mindspec/docs/domains/core/OWNERSHIP.yaml"
	writeFile(t, specWorktree, manifest, "paths:\n  - internal/old/**\n")
	gitRun(t, specWorktree, "add", manifest)
	err = runCommittedRangeGate(root, "landing-1", "bead/landing-1", specDir, base, ex, CompleteOpts{ResolveMerge: true}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "adr-divergence-unowned") {
		t.Fatalf("resolved index must revoke claim: %v", err)
	}
	checkDirtyTreeFn = func(string, string) ([]string, []string, error) { return nil, nil, nil }
	resumeErr := runMaterialization(materializationInput{
		exec: ex, root: root, beadID: "landing-1", beadHead: "bead/landing-1",
		wtPath: root, specDir: specDir, base: base, resumeStage: stageArtifactSync,
		opts: CompleteOpts{ResolveMerge: true}, warnWriter: io.Discard,
	})
	if resumeErr == nil || !strings.Contains(resumeErr.Error(), "adr-divergence-unowned") {
		t.Fatalf("materialization resume skipped index ownership: %v", resumeErr)
	}

	writeFile(t, specWorktree, manifest, "paths:\n  - internal/core/**\n")
	gitRun(t, specWorktree, "add", manifest)
	if err := runCommittedRangeGate(root, "landing-1", "bead/landing-1", specDir, base, ex, CompleteOpts{ResolveMerge: true}, io.Discard); err != nil {
		t.Fatalf("resolved index must own source: %v", err)
	}
	gitRun(t, specWorktree, "commit", "--no-edit")
	if gitutil.MergeInProgress(specWorktree) {
		t.Fatal("merge did not finish")
	}
	if got := gateRevParse(t, specWorktree, "HEAD^2"); got != gateRevParse(t, root, "bead/landing-1") {
		t.Fatal("wrong second parent")
	}
}

func TestLandingTree_OwnershipConflictReturnsBeforeReaders(t *testing.T) {
	root := setupTempRoot(t)
	mock := &executor.MockExecutor{LandingTreeResult: "conflict-tree", LandingTreeConflicted: []string{".mindspec/domains/core/OWNERSHIP.yaml"}}
	err := runCommittedRangeGate(root, "landing-1", "bead/landing-1", filepath.Join(root, ".mindspec/specs/132-landing"), "base", mock, CompleteOpts{}, io.Discard)
	var conflict *OwnershipLandingConflict
	if !errors.As(err, &conflict) || conflict.Tree != "conflict-tree" || len(conflict.Conflicted) != 1 {
		t.Fatalf("conflict outcome: %v", err)
	}
	if len(mock.CallsTo("FileAtRefOrAbsent")) != 0 {
		t.Fatal("read conflicted ownership")
	}
}

func materializeCitations(src string) []string {
	f, err := parser.ParseFile(token.NewFileSet(), "materialize.go", src, 0)
	if err != nil {
		return []string{err.Error()}
	}
	forbidden := regexp.MustCompile(`ADR-[0-9]|q9ea|[Ss]pec [0-9]{3}|mindspec-an8q`)
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err == nil && forbidden.MatchString(value) {
			hits = append(hits, value)
		}
		return true
	})
	return hits
}

func TestMaterializeText_NoHistoricalCitations(t *testing.T) {
	// The scan must detect a planted citation before a clean scan counts.
	if len(materializeCitations(`package complete; const refusal = "ADR-0041 forbids it"`)) != 1 {
		t.Fatal("citation detector did not fire")
	}
	// Read via the test source directory, independent of cwd changes in other tests.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "materialize.go"))
	if err != nil {
		t.Fatal(err)
	}
	if hits := materializeCitations(string(src)); len(hits) > 0 {
		t.Fatalf("historical citations in surface text: %v", hits)
	}
}

func TestLandingTree_OperationalErrorRefuses(t *testing.T) {
	root := setupTempRoot(t)
	mock := &executor.MockExecutor{LandingTreeErr: errors.New("cannot preview")}
	err := runCommittedRangeGate(root, "landing-1", "bead/landing-1", filepath.Join(root, ".mindspec/specs/132-landing"), "base", mock, CompleteOpts{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot preview") {
		t.Fatalf("expected landing error: %v", err)
	}
	if want := "\nrecovery: mindspec complete " + idrender.Bead("landing-1"); !strings.HasSuffix(err.Error(), want) {
		t.Errorf("recovery must name the real bead: %v", err)
	}
	if len(mock.CallsTo("ChangedFiles")) != 0 {
		t.Fatal("gate ran after preview failure")
	}
}

func TestLandingOwnershipError_RendersBeadID(t *testing.T) {
	if len(guard.FindFloorMatches("git reset --hard HEAD")) == 0 {
		t.Fatal("destructive-floor detector did not fire")
	}
	for _, beadID := range []string{"mindspec-ab12.1", "bad;id\ngit reset --hard HEAD"} {
		err := &landingOwnershipError{beadID: beadID, cause: errors.New("cannot preview\nretry")}
		if want := "\nrecovery: mindspec complete " + idrender.Bead(beadID); !strings.HasSuffix(err.Error(), want) {
			t.Errorf("recovery must render the bead ID: %q", err.Error())
		}
		if strings.Contains(err.Error(), "<bead>") || strings.Contains(err.Error(), "cannot preview\nretry") {
			t.Errorf("unsafe landing error: %q", err.Error())
		}
		_, recovery, found := strings.Cut(err.Error(), "\nrecovery: ")
		if !found {
			t.Fatal("landing error has no recovery line")
		}
		if matches := guard.FindFloorMatches(recovery); len(matches) != 0 {
			t.Errorf("recovery matches the destructive floor: %+v", matches)
		}
	}
}
