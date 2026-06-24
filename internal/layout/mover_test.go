package layout

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// runGit runs git in repo with a deterministic identity and returns trimmed
// combined output, failing the test on error.
func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return strings.TrimSpace(string(out))
}

func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newCanonicalRepo builds a temp git repo with a representative captured copy
// of a canonical .mindspec/docs tree (specs/adr/domains/core/context-map) plus
// repo-root README/AGENTS that reference INTO the moved trees. In-tree links
// are SYMMETRIC (preserved); the root-doc absolute refs are the breaking set.
func newCanonicalRepo(t *testing.T) (string, *executor.MindspecExecutor) {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@test.com")
	runGit(t, root, "config", "user.name", "test")

	files := map[string]string{
		".mindspec/docs/specs/000-x/spec.md":         "# Spec 000-x\n[adr](../../adr/ADR-0001.md)\n[core](../../core/USAGE.md)\n",
		".mindspec/docs/adr/ADR-0001.md":             "# ADR-0001\n[core usage](../core/USAGE.md)\n",
		".mindspec/docs/domains/foo/overview.md":     "# foo\n[arch](architecture.md)\n",
		".mindspec/docs/domains/foo/architecture.md": "# foo arch\n",
		".mindspec/docs/core/USAGE.md":               "# Usage\n[cm](../context-map.md)\n",
		".mindspec/docs/core/DOCS-LAYOUT.md":         "# Layout\n[spec](../specs/000-x/spec.md)\n",
		".mindspec/docs/context-map.md":              "# Context Map\n[foo](domains/foo/overview.md)\n",
		"README.md": "# Project\n" +
			"- spec: [s](.mindspec/docs/specs/000-x/spec.md)\n" +
			"- adr: [a](.mindspec/docs/adr/ADR-0001.md)\n" +
			"- core: [c](.mindspec/docs/core/USAGE.md)\n" +
			"- cm: [m](.mindspec/docs/context-map.md)\n" +
			"- dom: [d](.mindspec/docs/domains/foo/overview.md)\n",
		"AGENTS.md": "# Agents\nSee [usage](.mindspec/docs/core/USAGE.md).\n",
	}
	for rel, content := range files {
		writeRepoFile(t, root, rel, content)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "initial fixture")
	return root, executor.NewMindspecExecutor(root)
}

func mustExist(t *testing.T, root, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Errorf("expected %s to exist: %v", rel, err)
	}
}

func mustNotExist(t *testing.T, root, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
		t.Errorf("expected %s to NOT exist", rel)
	}
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestMover_GoldenFlatten is the AC7/AC9/AC10 golden-file test: deterministic
// flat post-tree, 100%-similarity rename commits, root-doc link-rewrite,
// idempotent re-run, and a clean link-check.
func TestMover_GoldenFlatten(t *testing.T) {
	root, exec := newCanonicalRepo(t)

	if err := NewMover(exec, root, "run-golden").Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Deterministic flat post-tree.
	for _, rel := range []string{
		".mindspec/specs/000-x/spec.md",
		".mindspec/adr/ADR-0001.md",
		".mindspec/domains/foo/overview.md",
		".mindspec/domains/foo/architecture.md",
		".mindspec/core/USAGE.md",
		".mindspec/core/DOCS-LAYOUT.md",
		".mindspec/context-map.md",
	} {
		mustExist(t, root, rel)
	}
	mustNotExist(t, root, ".mindspec/docs")
	mustNotExist(t, root, ".mindspec/docs/specs/000-x/spec.md")

	// Root docs rewritten: no pre-flatten path remains; the flat path is there.
	readme := readRepoFile(t, root, "README.md")
	if strings.Contains(readme, ".mindspec/docs/") {
		t.Errorf("README still references .mindspec/docs/:\n%s", readme)
	}
	if !strings.Contains(readme, ".mindspec/specs/000-x/spec.md") {
		t.Errorf("README missing rewritten flat spec path:\n%s", readme)
	}
	if !strings.Contains(readRepoFile(t, root, "AGENTS.md"), ".mindspec/core/USAGE.md") {
		t.Error("AGENTS.md not rewritten to flat core path")
	}

	// The specs move's commit is a PURE 100%-similarity rename (git log
	// --follow survives across it).
	specRenameSHA := commitWithSubject(t, root, "migrate(layout): move .mindspec/docs/specs -> .mindspec/specs")
	if specRenameSHA == "" {
		t.Fatal("no specs rename commit found")
	}
	nameStatus := runGit(t, root, "show", "--name-status", "--format=", specRenameSHA)
	for _, line := range strings.Split(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "R100") {
			t.Errorf("specs move commit is not a pure 100%% rename: %q", line)
		}
	}
	follow := runGit(t, root, "log", "--follow", "--format=%s", "--", ".mindspec/specs/000-x/spec.md")
	if !strings.Contains(follow, "initial fixture") {
		t.Errorf("git log --follow did not survive the rename (no initial commit):\n%s", follow)
	}

	// link-check is clean (Run returned nil above). Double-check directly.
	dangling, err := doctor.CheckMovedTreeLinks(root)
	if err != nil {
		t.Fatalf("link-check: %v", err)
	}
	if len(dangling) != 0 {
		t.Errorf("expected zero dangling links, got %v", dangling)
	}

	// Idempotent re-run: a fresh run on the already-flat tree adds NO commits.
	headBefore := runGit(t, root, "rev-parse", "HEAD")
	if err := NewMover(exec, root, "run-golden-2").Run(); err != nil {
		t.Fatalf("idempotent Run: %v", err)
	}
	if headAfter := runGit(t, root, "rev-parse", "HEAD"); headAfter != headBefore {
		t.Errorf("idempotent re-run created commits: %s -> %s", headBefore, headAfter)
	}
}

// commitWithSubject returns the SHA of the commit whose subject equals subj.
func commitWithSubject(t *testing.T, root, subj string) string {
	t.Helper()
	out := runGit(t, root, "log", "--format=%H %s")
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 && parts[1] == subj {
			return parts[0]
		}
	}
	return ""
}

// newCustomRewriteRepo builds a minimal repo where a move group genuinely has
// an IN-TREE breaking link, so the per-move TWO-COMMIT protocol (pure rename,
// then link-rewrite) is exercised end to end.
func newCustomRewriteRepo(t *testing.T) (string, *executor.MindspecExecutor) {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@test.com")
	runGit(t, root, "config", "user.name", "test")
	writeRepoFile(t, root, "things/a.md", "# A\n[b](old-target.md)\n")
	writeRepoFile(t, root, "things/b.md", "# B\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "initial fixture")
	return root, executor.NewMindspecExecutor(root)
}

func customMover(exec *executor.MindspecExecutor, root, runID string) *Mover {
	m := NewMover(exec, root, runID)
	m.plan = []MoveGroup{{Src: "things", Dst: "moved"}}
	m.rules = []RewriteRule{{Old: "old-target.md", New: "b.md"}}
	m.rootDocs = nil
	m.linkCheck = func(string) ([]doctor.DanglingLink, error) { return nil, nil }
	return m
}

// TestMover_TwoCommitPerMove proves the per-move protocol: a pure 100%-rename
// commit FIRST, then a separate link-rewrite commit — and that git log --follow
// survives the split (AC7).
func TestMover_TwoCommitPerMove(t *testing.T) {
	root, exec := newCustomRewriteRepo(t)
	if err := customMover(exec, root, "run-2c").Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mustExist(t, root, "moved/a.md")
	mustExist(t, root, "moved/b.md")
	mustNotExist(t, root, "things")
	if got := readRepoFile(t, root, "moved/a.md"); !strings.Contains(got, "[b](b.md)") || strings.Contains(got, "old-target.md") {
		t.Errorf("a.md link not rewritten: %q", got)
	}

	// Move commit is a pure rename; rewrite commit changes content.
	renameSHA := commitWithSubject(t, root, "migrate(layout): move things -> moved")
	rewriteSHA := commitWithSubject(t, root, "migrate(layout): rewrite links in moved")
	if renameSHA == "" || rewriteSHA == "" {
		t.Fatalf("expected both a rename and a rewrite commit; got rename=%q rewrite=%q", renameSHA, rewriteSHA)
	}
	renameStatus := runGit(t, root, "show", "--name-status", "--format=", renameSHA)
	for _, line := range strings.Split(renameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "R100") {
			t.Errorf("move commit not a pure 100%% rename: %q", line)
		}
	}
	rewriteStatus := runGit(t, root, "show", "--name-status", "--format=", rewriteSHA)
	if !strings.Contains(rewriteStatus, "moved/a.md") || strings.Contains(rewriteStatus, "R100") {
		t.Errorf("rewrite commit should be a content change to moved/a.md, got: %q", rewriteStatus)
	}
	follow := runGit(t, root, "log", "--follow", "--format=%s", "--", "moved/a.md")
	if !strings.Contains(follow, "initial fixture") {
		t.Errorf("git log --follow did not survive the rename:\n%s", follow)
	}
}

// TestMover_CrashResumeEveryBoundary injects a simulated crash at each run-state
// boundary, then re-runs and asserts the resumed tree is correct (AC8).
func TestMover_CrashResumeEveryBoundary(t *testing.T) {
	boundaries := []struct {
		stage stage
		group int
	}{
		{stageBeforeMv, 0},
		{stageAfterMv, 0},
		{stageAfterMoveCommit, 0},
		{stageAfterRewrite, 0},
		{stageAfterRewriteCommit, 0},
		{stageRootRewrite, -1},
		{stageFinalize, -1},
	}
	for _, b := range boundaries {
		t.Run(string(b.stage), func(t *testing.T) {
			root, exec := newCustomRewriteRepo(t)

			crash := customMover(exec, root, "run-crash")
			crash.crashAt = b.stage
			crash.crashAtGroup = b.group
			if err := crash.Run(); err == nil {
				t.Fatalf("expected simulated crash at %s, got nil", b.stage)
			}

			// Resume with a fresh mover (same run-id, no injection).
			if err := customMover(exec, root, "run-crash").Run(); err != nil {
				t.Fatalf("resume after crash at %s: %v", b.stage, err)
			}

			mustExist(t, root, "moved/a.md")
			mustExist(t, root, "moved/b.md")
			mustNotExist(t, root, "things")
			if got := readRepoFile(t, root, "moved/a.md"); !strings.Contains(got, "[b](b.md)") {
				t.Errorf("after resume from %s, a.md not rewritten: %q", b.stage, got)
			}
			// Working tree is clean of doc changes after resume.
			if status := runGit(t, root, "status", "--porcelain", "--", "moved"); status != "" {
				t.Errorf("after resume from %s, moved/ dirty:\n%s", b.stage, status)
			}
		})
	}
}

// TestMover_RollbackOnFailure asserts an injected mid-run operational failure
// hard-resets to the pre-run ref with nothing published (AC7).
func TestMover_RollbackOnFailure(t *testing.T) {
	root, exec := newCanonicalRepo(t)
	preRun := runGit(t, root, "rev-parse", "HEAD")

	m := NewMover(exec, root, "run-rollback")
	// Fail AFTER the second group's move commit so at least one move landed
	// before the failure — proving rollback undoes committed moves.
	m.failAt = stageAfterMoveCommit
	m.failAtGroup = 1
	if err := m.Run(); err == nil {
		t.Fatal("expected injected failure, got nil")
	}

	if head := runGit(t, root, "rev-parse", "HEAD"); head != preRun {
		t.Errorf("rollback did not restore pre-run ref: %s != %s", head, preRun)
	}
	// The canonical tree is intact; no flat tree leaked.
	mustExist(t, root, ".mindspec/docs/specs/000-x/spec.md")
	mustNotExist(t, root, ".mindspec/specs/000-x/spec.md")
	if status := runGit(t, root, "status", "--porcelain", "--", ".mindspec/docs", ".mindspec/specs", ".mindspec/adr"); status != "" {
		t.Errorf("rollback left a dirty tree:\n%s", status)
	}
}

// TestMover_LinkCheckFailsDanglingRewritten asserts the migration FAILS when a
// rewrite rule points at a nonexistent target (AC10).
func TestMover_LinkCheckFailsDanglingRewritten(t *testing.T) {
	root, exec := newCanonicalRepo(t)
	// README references a core doc that does NOT exist; the rule rewrites it to
	// .mindspec/core/MISSING.md, which 404s after the move.
	writeRepoFile(t, root, "README.md", "# Project\n[gone](.mindspec/docs/core/MISSING.md)\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "inject dangling rewritten link")
	preRun := runGit(t, root, "rev-parse", "HEAD")

	err := NewMover(exec, root, "run-dangling").Run()
	if err == nil {
		t.Fatal("expected link-check failure, got nil")
	}
	if _, ok := errAsLinkCheck(err); !ok {
		t.Fatalf("expected *LinkCheckError, got %T: %v", err, err)
	}
	// Pre-publish failure rolled back to the pre-run ref.
	if head := runGit(t, root, "rev-parse", "HEAD"); head != preRun {
		t.Errorf("link-check failure did not roll back: %s != %s", head, preRun)
	}
}

// TestMover_LinkCheckFailsUnrewrittenBreaking asserts the migration FAILS on a
// breaking link the finite rewriter did NOT cover (a bare `docs/...` form),
// proving the gate scans EVERY link, not only the rewritten set (AC10).
func TestMover_LinkCheckFailsUnrewrittenBreaking(t *testing.T) {
	root, exec := newCanonicalRepo(t)
	// A spec file links to a bare `docs/adr/...` form (NOT the .mindspec/docs/
	// form the finite rule set normalizes), so it survives unrewritten and
	// 404s after the move.
	writeRepoFile(t, root, ".mindspec/docs/specs/000-x/spec.md",
		"# Spec 000-x\n[adr](../../adr/ADR-0001.md)\n[stale](docs/adr/ADR-0001.md)\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "inject un-rewritten breaking link")

	err := NewMover(exec, root, "run-unrewritten").Run()
	if err == nil {
		t.Fatal("expected link-check failure on un-rewritten breaking link, got nil")
	}
	if _, ok := errAsLinkCheck(err); !ok {
		t.Fatalf("expected *LinkCheckError, got %T: %v", err, err)
	}
}

func errAsLinkCheck(err error) (*LinkCheckError, bool) {
	var lce *LinkCheckError
	if errors.As(err, &lce) {
		return lce, true
	}
	return nil, false
}

// TestMover_AbortPrePublish asserts an explicit pre-publish --abort hard-resets
// to the pre-run ref (AC8).
func TestMover_AbortPrePublish(t *testing.T) {
	root, exec := newCustomRewriteRepo(t)
	preRun := runGit(t, root, "rev-parse", "HEAD")

	// Crash mid-run to leave a partial, unpublished run-state.
	crash := customMover(exec, root, "run-abort")
	crash.crashAt = stageAfterMoveCommit
	crash.crashAtGroup = 0
	if err := crash.Run(); err == nil {
		t.Fatal("expected crash, got nil")
	}
	if head := runGit(t, root, "rev-parse", "HEAD"); head == preRun {
		t.Fatal("precondition: expected a partial commit before abort")
	}

	if err := NewMover(exec, root, "run-abort").Abort(); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if head := runGit(t, root, "rev-parse", "HEAD"); head != preRun {
		t.Errorf("abort did not restore pre-run ref: %s != %s", head, preRun)
	}
	mustExist(t, root, "things/a.md")
	mustNotExist(t, root, "moved")
}

// TestMover_AbortRefusesAfterPublish asserts auto-rollback is REFUSED once the
// run is published (ADR-0023 forward-only).
func TestMover_AbortRefusesAfterPublish(t *testing.T) {
	root, exec := newCustomRewriteRepo(t)

	crash := customMover(exec, root, "run-pub")
	crash.crashAt = stageAfterMoveCommit
	crash.crashAtGroup = 0
	_ = crash.Run()

	// Mark the run published on disk.
	s, _, err := loadState(root, "run-pub")
	if err != nil {
		t.Fatal(err)
	}
	s.Published = true
	if err := writeJSON(statePath(root, "run-pub"), s); err != nil {
		t.Fatal(err)
	}

	err = NewMover(exec, root, "run-pub").Abort()
	if err == nil {
		t.Fatal("expected Abort to refuse a published run, got nil")
	}
	if !strings.Contains(err.Error(), "forward-only") {
		t.Errorf("expected forward-only refusal, got: %v", err)
	}
}

// TestMover_LineageManifest asserts a completed run writes a lineage manifest
// under .mindspec/lineage/ recording each move's source→dest, and a run-state
// record with the terminal "applied" stage (AC9; doctor-schema parse is
// covered in internal/doctor).
func TestMover_LineageManifest(t *testing.T) {
	root, exec := newCanonicalRepo(t)
	if err := NewMover(exec, root, "run-lineage").Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var manifest LineageManifest
	data, err := os.ReadFile(lineageManifestPath(root))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.RunID != "run-lineage" {
		t.Errorf("run_id = %q, want run-lineage", manifest.RunID)
	}
	// newCanonicalRepo carries the 5 symmetric groups but NO dogfood dirs, so
	// the 3 dogfood plan entries land no move and are correctly NOT recorded
	// (writeLineage records only groups whose move landed). The full
	// symmetric+dogfood+review recording is asserted in
	// TestMover_DogfoodAndReviewMoves.
	if len(manifest.Entries) != 5 {
		t.Errorf("entries = %d, want 5 (the symmetric groups present in the fixture): %+v", len(manifest.Entries), manifest.Entries)
	}
	foundSpecs := false
	for _, e := range manifest.Entries {
		if e.Source == ".mindspec/docs/specs" && e.Canonical == ".mindspec/specs" {
			foundSpecs = true
		}
		// A recorded group's destination must actually exist on disk.
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(e.Canonical))); err != nil {
			t.Errorf("lineage recorded a group whose dest is missing: %+v", e)
		}
	}
	if !foundSpecs {
		t.Errorf("manifest missing specs source→dest entry: %+v", manifest.Entries)
	}

	st, found, err := loadState(root, "run-lineage")
	if err != nil || !found {
		t.Fatalf("loadState: found=%v err=%v", found, err)
	}
	if st.Stage != string(stageApplied) {
		t.Errorf("terminal stage = %q, want applied", st.Stage)
	}
}

// newFullMoveRepo builds a canonical repo exercising the FULL spec-106 move
// set: the symmetric lifecycle flatten, the asymmetric dogfood eviction to
// project-docs/ (with both a symmetric sibling link and an absolute cross-tree
// link whose depth changes), and root review/<slug> dirs routed two ways —
// by panel.json `spec` (a non-numeric slug only panel.json can place) and by
// the slug's numeric prefix — plus one un-routable review dir.
func newFullMoveRepo(t *testing.T) (string, *executor.MindspecExecutor) {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@test.com")
	runGit(t, root, "config", "user.name", "test")

	files := map[string]string{
		// Lifecycle (symmetric flatten).
		".mindspec/docs/specs/106-layout-flatten/spec.md": "# Spec 106\n[adr](../../adr/ADR-0001.md)\n",
		".mindspec/docs/specs/099-prior/spec.md":          "# Spec 099\n",
		".mindspec/docs/adr/ADR-0001.md":                  "# ADR-0001\n",
		".mindspec/docs/core/USAGE.md":                    "# Usage\n",
		".mindspec/docs/context-map.md":                   "# Context Map\n",
		// Dogfood (asymmetric eviction to project-docs/). guide.md carries a
		// SYMMETRIC sibling link (preserved across the depth change — both
		// user/ and installation/ shed .mindspec/docs/ and gain project-docs/)
		// and a repo-root-relative absolute link into the flattened adr tree
		// (rewritten by the symmetric rule).
		".mindspec/docs/user/guide.md":         "# Guide\n[install](../installation/setup.md)\n[adr](/.mindspec/docs/adr/ADR-0001.md)\n",
		".mindspec/docs/installation/setup.md": "# Setup\n",
		".mindspec/docs/research/notes.md":     "# Notes\n",
		// Reviews: zzz-custom has NO numeric prefix, so ONLY its panel.json can
		// route it (isolates panel.json routing). 099-final-panel has no
		// panel.json, so ONLY the slug prefix can route it. prep is un-routable.
		"review/zzz-custom/panel.json":    `{"spec":"106-layout-flatten","target":"main"}`,
		"review/zzz-custom/summary.md":    "# Summary\n[v](verdict.md)\n",
		"review/zzz-custom/verdict.md":    "# Verdict\n",
		"review/099-final-panel/notes.md": "# Prior panel notes\n",
		"review/prep/log.md":              "# scratch log\n",
		// Root doc referencing INTO the evicted dogfood tree.
		"README.md": "# Project\n- guide: [g](.mindspec/docs/user/guide.md)\n",
	}
	for rel, content := range files {
		writeRepoFile(t, root, rel, content)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "initial fixture")
	return root, executor.NewMindspecExecutor(root)
}

// TestMover_DogfoodAndReviewMoves is the R6 blocker test: the dogfood eviction
// and review co-location moves apply, their depth-change links resolve (no
// 404), review routing reads panel.json `spec` AND falls back to the slug
// prefix, an un-routable review dir is skipped + recorded, and the lineage
// records ALL move groups (symmetric + dogfood + review).
func TestMover_DogfoodAndReviewMoves(t *testing.T) {
	root, exec := newFullMoveRepo(t)
	if err := NewMover(exec, root, "run-full").Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Symmetric flatten landed.
	mustExist(t, root, ".mindspec/specs/106-layout-flatten/spec.md")
	mustExist(t, root, ".mindspec/adr/ADR-0001.md")
	mustNotExist(t, root, ".mindspec/docs")

	// Dogfood evicted to project-docs/ (NOT a root docs/ alias).
	mustExist(t, root, "project-docs/user/guide.md")
	mustExist(t, root, "project-docs/installation/setup.md")
	mustExist(t, root, "project-docs/research/notes.md")
	mustNotExist(t, root, ".mindspec/docs/user/guide.md")
	mustNotExist(t, root, "docs/user/guide.md")
	// The absolute cross-tree link was rewritten to the flat adr path; the
	// symmetric sibling link is preserved.
	guide := readRepoFile(t, root, "project-docs/user/guide.md")
	if strings.Contains(guide, ".mindspec/docs/adr/") {
		t.Errorf("guide.md absolute adr link not rewritten:\n%s", guide)
	}
	if !strings.Contains(guide, ".mindspec/adr/ADR-0001.md") {
		t.Errorf("guide.md missing rewritten flat adr link:\n%s", guide)
	}
	if !strings.Contains(guide, "[install](../installation/setup.md)") {
		t.Errorf("guide.md symmetric sibling link should be preserved:\n%s", guide)
	}
	// Root doc reference into the evicted tree rewritten.
	if readme := readRepoFile(t, root, "README.md"); !strings.Contains(readme, "project-docs/user/guide.md") {
		t.Errorf("README not rewritten to evicted dogfood path:\n%s", readme)
	}

	// Review routing: zzz-custom via panel.json `spec` (non-numeric slug);
	// 099-final-panel via slug prefix → spec 099-prior.
	mustExist(t, root, ".mindspec/specs/106-layout-flatten/reviews/zzz-custom/summary.md")
	mustExist(t, root, ".mindspec/specs/106-layout-flatten/reviews/zzz-custom/verdict.md")
	mustExist(t, root, ".mindspec/specs/099-prior/reviews/099-final-panel/notes.md")
	mustNotExist(t, root, "review/zzz-custom")
	mustNotExist(t, root, "review/099-final-panel")
	// The un-routable dir is left in place (so root review/ is NOT removed).
	mustExist(t, root, "review/prep/log.md")

	// Depth-change links resolve: the gating link-check is clean across the
	// flat lifecycle tree, the co-located reviews, AND project-docs/.
	dangling, err := doctor.CheckMovedTreeLinks(root)
	if err != nil {
		t.Fatalf("link-check: %v", err)
	}
	if len(dangling) != 0 {
		t.Errorf("expected zero dangling links after full move, got %+v", dangling)
	}

	// Lineage records ALL move groups (symmetric + dogfood + review) and the
	// skipped un-routable review dir.
	var manifest LineageManifest
	data, err := os.ReadFile(lineageManifestPath(root))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	want := map[string]string{
		".mindspec/docs/specs":    ".mindspec/specs",
		".mindspec/docs/adr":      ".mindspec/adr",
		".mindspec/docs/user":     "project-docs/user",
		".mindspec/docs/research": "project-docs/research",
		"review/zzz-custom":       ".mindspec/specs/106-layout-flatten/reviews/zzz-custom",
		"review/099-final-panel":  ".mindspec/specs/099-prior/reviews/099-final-panel",
	}
	got := map[string]string{}
	for _, e := range manifest.Entries {
		got[e.Source] = e.Canonical
	}
	for src, dst := range want {
		if got[src] != dst {
			t.Errorf("lineage missing/incorrect group %s → %s (got %q)", src, dst, got[src])
		}
	}
	if !containsStr(manifest.Skipped, "review/prep") {
		t.Errorf("lineage Skipped should record the un-routable review/prep dir: %+v", manifest.Skipped)
	}
}

// TestMover_ReviewTreeRemovedWhenFullyMigrated asserts the root review/ tree is
// removed entirely once every review/<slug> dir has been co-located (no
// un-routable residue), resolving the homeless-review friction.
func TestMover_ReviewTreeRemovedWhenFullyMigrated(t *testing.T) {
	root, exec := newFullMoveRepo(t)
	// Drop the un-routable dir so every review dir routes.
	if err := os.RemoveAll(filepath.Join(root, "review", "prep")); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "drop un-routable review dir")

	if err := NewMover(exec, root, "run-review-clean").Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mustNotExist(t, root, "review")
	mustExist(t, root, ".mindspec/specs/106-layout-flatten/reviews/zzz-custom/summary.md")
}

// TestMover_RollbackAfterDogfoodReview asserts an injected failure AFTER a
// dogfood/review move has landed hard-resets to the pre-run ref, restoring the
// pre-run tree cleanly (no project-docs/ or co-located reviews leak, and the
// scoped clean leaves no residue) (R6 item 5 + R5 rollback safety).
func TestMover_RollbackAfterDogfoodReview(t *testing.T) {
	root, exec := newFullMoveRepo(t)
	preRun := runGit(t, root, "rev-parse", "HEAD")

	m := NewMover(exec, root, "run-full-rb")
	// The dogfood `user` group is index 5 in DefaultFlattenPlan; failing after
	// its move commit proves rollback undoes a landed dogfood eviction.
	m.failAt = stageAfterMoveCommit
	m.failAtGroup = 5
	if err := m.Run(); err == nil {
		t.Fatal("expected injected failure, got nil")
	}

	if head := runGit(t, root, "rev-parse", "HEAD"); head != preRun {
		t.Errorf("rollback did not restore pre-run ref: %s != %s", head, preRun)
	}
	// The canonical tree is intact; no evicted/co-located trees leaked.
	mustExist(t, root, ".mindspec/docs/user/guide.md")
	mustExist(t, root, "review/zzz-custom/summary.md")
	mustNotExist(t, root, "project-docs")
	mustNotExist(t, root, ".mindspec/specs")
	// The scoped clean left no untracked residue anywhere.
	if status := runGit(t, root, "status", "--porcelain"); status != "" {
		t.Errorf("rollback left a dirty tree:\n%s", status)
	}
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
