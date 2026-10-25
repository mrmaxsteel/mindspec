package complete

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/bootstrap"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

const closingSpec = "132-window"
const closingBead = "window-1"

type windowFixture struct {
	t                                 *testing.T
	root, specWT, beadWT, parent, tip string
	closed                            bool
	closeCalls                        int
	meta                              map[string]interface{}
	ex                                *windowExecutor
}

type windowExecutor struct {
	*executor.MindspecExecutor
	f          *windowFixture
	publishErr error
	cleanupErr error
}

func (e *windowExecutor) PublishBeadClose(wt string, paths []string) error {
	if e.publishErr != nil {
		return e.publishErr
	}
	data, _ := json.Marshal(map[string]interface{}{"id": closingBead, "status": map[bool]string{true: "closed", false: "in_progress"}[e.f.closed]})
	writeFile(e.f.t, wt, ".beads/issues.jsonl", string(data)+"\n")
	return e.CommitPaths(wt, "publish close", paths)
}
func (e *windowExecutor) CleanupBead(id, target string) error {
	if e.cleanupErr != nil {
		return e.cleanupErr
	}
	return e.MindspecExecutor.CleanupBead(id, target)
}

type windowWorktrees struct{ f *windowFixture }

func (w windowWorktrees) Create(string, string) error { return errors.New("unexpected create") }
func (w windowWorktrees) List() ([]bead.WorktreeListEntry, error) {
	if _, err := os.Stat(w.f.beadWT); os.IsNotExist(err) {
		return nil, nil
	}
	return []bead.WorktreeListEntry{{Name: "worktree-" + closingBead, Branch: "bead/" + closingBead, Path: w.f.beadWT}}, nil
}
func (w windowWorktrees) Remove(string) error {
	// Completion has already refused user dirt. Match the tracker remover's
	// ability to discard tool artifacts that remain after publication.
	gitRun(w.f.t, w.f.root, "worktree", "remove", "--force", w.f.beadWT)
	return nil
}

func newWindowFixture(t *testing.T) *windowFixture {
	return newWindowFixtureLayout(t, true)
}

func newWindowFixtureLayout(t *testing.T, flat bool) *windowFixture {
	t.Helper()
	saveAndRestore(t)
	fakeBdOnPath(t)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	f := &windowFixture{t: t, root: setupTempRoot(t), meta: map[string]interface{}{}}
	stubPhaseEpic(t, closingSpec, "epic-window")
	resolveTargetFn = func(string, string) (string, error) { return closingSpec, nil }
	findLocalRootFn = func() (string, error) { return f.root, nil }
	writeFile(t, f.root, ".mindspec/config.yaml", "source_globs:\n  - internal/**/*.go\n")
	specs := ".mindspec/specs/"
	if !flat {
		specs = ".mindspec/docs/specs/"
	}
	writeFile(t, f.root, specs+closingSpec+"/spec.md", "# Spec\n\n## Impacted Domains\n\n- widget\n")
	writeFile(t, f.root, ".gitignore", ".worktrees/\n")
	writeFile(t, f.root, "README.md", "before\n")
	writeFile(t, f.root, ".beads/issues.jsonl", "{\"id\":\"window-1\",\"status\":\"in_progress\"}\n")
	gitRun(t, f.root, "init", "-q", "-b", "main")
	gitRun(t, f.root, "config", "user.email", "test@example.invalid")
	gitRun(t, f.root, "config", "user.name", "Test")
	gitRun(t, f.root, "add", "-A")
	gitRun(t, f.root, "commit", "-qm", "base")
	f.parent = gateRevParse(t, f.root, "HEAD")
	gitRun(t, f.root, "branch", "spec/"+closingSpec)
	gitRun(t, f.root, "checkout", "-qb", "bead/"+closingBead)
	writeFile(t, f.root, "README.md", "after\n")
	writeFile(t, f.root, "feature.txt", "feature\n")
	gitRun(t, f.root, "add", "-A")
	gitRun(t, f.root, "commit", "-qm", "implement")
	f.tip = gateRevParse(t, f.root, "HEAD")
	gitRun(t, f.root, "checkout", "-q", "main")
	cfg, err := config.Load(f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.specWT, err = workspace.SpecWorktreePath(f.root, cfg, closingSpec)
	if err != nil {
		t.Fatal(err)
	}
	f.beadWT = filepath.Join(f.specWT, ".worktrees", "worktree-"+closingBead)
	gitRun(t, f.root, "worktree", "add", f.specWT, "spec/"+closingSpec)
	gitRun(t, f.root, "worktree", "add", f.beadWT, "bead/"+closingBead)
	ex := executor.NewMindspecExecutor(f.root)
	ex.WorktreeOps = windowWorktrees{f}
	f.ex = &windowExecutor{MindspecExecutor: ex, f: f}
	worktreeListFn = ex.WorktreeOps.List
	closeBeadFn = func(...string) error { f.closeCalls++; f.closed = true; return nil }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		status := "in_progress"
		if f.closed {
			status = "closed"
		}
		return next.BeadInfo{ID: id, Status: status}, nil
	}
	completeGetMetadataFn = func(string) (map[string]interface{}, error) { return f.meta, nil }
	completeMergeMetadataFn = func(_ string, updates map[string]interface{}) error {
		for k, v := range updates {
			f.meta[k] = v
		}
		return nil
	}
	origDelete := completeDeleteMetadataFn
	t.Cleanup(func() { completeDeleteMetadataFn = origDelete })
	completeDeleteMetadataFn = func(_ string, keys ...string) error {
		for _, k := range keys {
			delete(f.meta, k)
		}
		return nil
	}
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	// The fixture owns the only worktree and uses no real tracker.
	runBDFn = func(...string) ([]byte, error) { return []byte("[]"), nil }
	return f
}

func (f *windowFixture) run(opts CompleteOpts) (*Result, error) {
	f.t.Helper()
	return Run(f.root, closingBead, closingSpec, "", f.ex, opts)
}
func (f *windowFixture) merge() string {
	f.t.Helper()
	gitRun(f.t, f.specWT, "merge", "--no-ff", "-qm", "Merge bead/"+closingBead, "bead/"+closingBead)
	return gateRevParse(f.t, f.root, "spec/"+closingSpec)
}
func (f *windowFixture) mergedOnly() string {
	f.t.Helper()
	r, err := f.run(CompleteOpts{MergeOnly: true})
	if err != nil {
		f.t.Fatal(err)
	}
	if r.BeadClosed || f.closed || !r.MergeOnly {
		f.t.Fatalf("merge-only closed bead: %+v", r)
	}
	return r.MergeSHA
}
func requireWindowRefusal(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("want refusal %q, got %v", fragment, err)
	}
}

func TestComplete_MergeOnlyKeepsBeadOpen(t *testing.T) {
	f := newWindowFixture(t)
	m := f.mergedOnly()
	if f.meta[mergeOnlySHAKey] != m || f.meta[mergeOnlyPendingKey] != nil || f.meta["mindspec_landed_merge_sha"] != nil {
		t.Fatalf("wrong metadata: %v", f.meta)
	}
	if _, err := os.Stat(f.beadWT); err != nil {
		t.Fatal(err)
	}
	data, err := f.ex.FileAtRef(m, ".beads/issues.jsonl")
	if err != nil || !strings.Contains(string(data), "in_progress") {
		t.Fatalf("merge publishes close: %s %v", data, err)
	}
}
func TestComplete_PendingKeyPlainCompleteStaysOpen(t *testing.T) {
	f := newWindowFixture(t)
	f.meta[mergeOnlyPendingKey] = "pending"
	r, err := f.run(CompleteOpts{})
	if err != nil || f.closed || !r.MergeOnly {
		t.Fatalf("pending closed: %+v %v", r, err)
	}
}
func TestComplete_MergeOnlyBranchlessRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	gitRun(t, f.root, "worktree", "remove", f.beadWT)
	gitRun(t, f.root, "branch", "-D", "bead/"+closingBead)
	_, err := f.run(CompleteOpts{MergeOnly: true})
	requireWindowRefusal(t, err, "there is no bead branch")
	if f.closed {
		t.Fatal("closed branchless bead")
	}
}
func TestComplete_MergeOnlyHandCommitStaysOpen(t *testing.T) {
	f := sourceConflictWindow(t)
	_, err := f.run(CompleteOpts{MergeOnly: true})
	requireWindowRefusal(t, err, "--merge-only --resolve-merge")
	if !gitutil.MergeInProgress(f.specWT) || f.meta[mergeOnlyPendingKey] != "pending" || f.closed {
		t.Fatal("merge-only did not preserve the conflict and pending mode")
	}
	writeFile(t, f.specWT, "README.md", "resolved by hand\n")
	gitRun(t, f.specWT, "add", "README.md")
	gitRun(t, f.specWT, "commit", "--no-edit")
	m := gateRevParse(t, f.root, "spec/"+closingSpec)
	r, err := f.run(CompleteOpts{MergeOnly: true, ResolveMerge: true})
	if err != nil || f.closed || !r.MergeOnly || r.MergeSHA != m || f.meta[mergeOnlySHAKey] != m || f.meta[mergeOnlyPendingKey] != nil {
		t.Fatalf("hand-committed conflict did not record merge-only: %+v %v metadata=%v", r, err, f.meta)
	}
}
func TestComplete_HandMergeTakesClosingPath(t *testing.T) {
	f := newWindowFixture(t)
	m := f.merge()
	r, err := f.run(CompleteOpts{})
	if err != nil || !f.closed || !r.WorktreeRemoved || f.meta["mindspec_landed_merge_sha"] != m {
		t.Fatalf("close failed: %+v %v %v", r, err, f.meta)
	}
}

type handMergeGateExecutor struct {
	*windowExecutor
	merges, cleanups int
}

func (e *handMergeGateExecutor) MergeBead(id, target, reason string, resolve bool, opts executor.MergeBeadOptions) error {
	e.merges++
	return e.windowExecutor.MergeBead(id, target, reason, resolve, opts)
}

func (e *handMergeGateExecutor) CleanupBead(id, target string) error {
	e.cleanups++
	return e.windowExecutor.CleanupBead(id, target)
}

func TestComplete_HandMergedSourceRenameRefusesBeforeClose(t *testing.T) {
	f := newWindowFixture(t)
	gitRun(t, f.root, "config", "diff.renames", "true")
	const source = "internal/widget/widget.go"
	const contents = "package widget\n\nconst Value = 1\n"
	writeFile(t, f.root, source, contents)
	writeFile(t, f.root, ".mindspec/domains/widget/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
	writeFile(t, f.root, ".mindspec/domains/widget/architecture.md", "Widget owns its source.\n")
	gitRun(t, f.root, "add", "-A")
	gitRun(t, f.root, "commit", "-qm", "shared owned source")
	gitRun(t, f.specWT, "merge", "--no-edit", "main")
	gitRun(t, f.beadWT, "merge", "--no-edit", "main")
	destination := ".mindspec/specs/" + closingSpec + "/reviews/rename/dispositions.jsonl"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(f.beadWT, destination)), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, f.beadWT, "mv", source, destination)
	gitRun(t, f.beadWT, "commit", "-qm", "move source to review artifact")
	f.tip = gateRevParse(t, f.root, "bead/"+closingBead)
	merge := f.merge()
	ex := &handMergeGateExecutor{windowExecutor: f.ex}
	result, err := Run(f.root, closingBead, closingSpec, "", ex, CompleteOpts{})
	t.Logf("close_calls=%d merge_calls=%d cleanup_calls=%d closed=%v result=%+v err=%v", f.closeCalls, ex.merges, ex.cleanups, f.closed, result, err)
	requireWindowRefusal(t, err, "internal sources in domain")
	requireWindowRefusal(t, err, source)
	if f.closeCalls != 0 || ex.merges != 0 || ex.cleanups != 0 || f.closed {
		t.Fatal("content gate must refuse before close, merge, or cleanup")
	}
	if gateRevParse(t, f.root, "bead/"+closingBead) != f.tip || gateRevParse(t, f.root, "spec/"+closingSpec) != merge {
		t.Fatal("refusal moved a branch tip")
	}
	for _, wt := range []string{f.beadWT, f.specWT} {
		data, readErr := os.ReadFile(filepath.Join(wt, destination))
		if readErr != nil || string(data) != contents {
			t.Fatalf("refusal lost committed bytes in %s: %q, %v", wt, data, readErr)
		}
	}
	if f.meta["mindspec_landed_merge_sha"] != nil {
		t.Fatal("refusal bound the bead as landed")
	}
}
func TestComplete_ForeignMarkerRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.meta[mergeOnlySHAKey] = f.parent
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "--unset-metadata mindspec_merge_only_sha")
	if f.closed {
		t.Fatal("closed with foreign marker")
	}
}
func TestComplete_BeadAdvancedAfterMergeOnlyRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	writeFile(t, f.beadWT, "fix.txt", "fix\n")
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "fix after verification")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "fix after verification")
	if f.closed {
		t.Fatal("closed moved tip")
	}
}
func TestComplete_UserDirtAfterMergeOnlyRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	writeFile(t, f.beadWT, "fix.txt", "fix\n")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "uncommitted user changes")
	if f.closed {
		t.Fatal("closed dirty bead")
	}
}
func TestComplete_RepeatedMergeOnlyNoNewMerge(t *testing.T) {
	f := newWindowFixture(t)
	mergeMeta := completeMergeMetadataFn
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if _, present := updates[mergeOnlySHAKey]; present {
			return errors.New("marker write failed")
		}
		return mergeMeta(id, updates)
	}
	_, err := f.run(CompleteOpts{MergeOnly: true})
	requireWindowRefusal(t, err, "marker write failed")
	m := gateRevParse(t, f.root, "spec/"+closingSpec)
	if f.closed || f.meta[mergeOnlySHAKey] != nil || f.meta[mergeOnlyPendingKey] != "pending" {
		t.Fatalf("wrong state after failed marker write: %v", f.meta)
	}
	completeMergeMetadataFn = mergeMeta
	r, err := f.run(CompleteOpts{MergeOnly: true})
	if err != nil || r.MergeSHA != m || f.closed || gateRevParse(t, f.root, "spec/"+closingSpec) != m || f.meta[mergeOnlySHAKey] != m || f.meta[mergeOnlyPendingKey] != nil {
		t.Fatalf("repeat must record the marker without merging or closing: %+v %v metadata=%v", r, err, f.meta)
	}
	rows, err := namingMerges(f.ex, "spec/"+closingSpec, closingBead)
	if err != nil || len(rows) != 1 {
		t.Fatalf("repeat naming merges: %v %v", rows, err)
	}
}
func TestComplete_SecondMergeOnlyRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	writeFile(t, f.beadWT, "fix.txt", "fix\n")
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "later fix")
	before := gateRevParse(t, f.root, "spec/"+closingSpec)
	_, err := f.run(CompleteOpts{MergeOnly: true})
	requireWindowRefusal(t, err, "bead tip moved")
	if gateRevParse(t, f.root, "spec/"+closingSpec) != before {
		t.Fatal("made a second merge")
	}
}
func (f *windowFixture) registerLedger() string {
	return f.registerLedgerWithExclude(true)
}

func (f *windowFixture) registerLedgerWithExclude(exclude bool) string {
	f.t.Helper()
	// Model local review ignores only in tests that register a local panel.
	// Source-safety fixtures intentionally leave lookalike paths unignored.
	if exclude {
		writeFile(f.t, f.root, ".git/info/exclude", ".mindspec/specs/*/reviews/**\n.mindspec/reviews/**\n")
	}
	specDir, err := workspace.SpecDir(f.root, closingSpec)
	if err != nil {
		f.t.Fatal(err)
	}
	p := panel.Panel{BeadID: windowBeadPointer(), Spec: closingSpec, Target: "bead/" + closingBead, Gate: "bead", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: gateRevParse(f.t, f.root, "bead/"+closingBead)}
	writeCoLocatedPanel(f.t, specDir, "closing-panel", p, approveVerdicts(2))
	dir := filepath.Join(specDir, "reviews", "closing-panel")
	core3Cover(f.t, panel.Registration{Dir: dir, Panel: p})
	return filepath.ToSlash(filepath.Join(".mindspec", "specs", closingSpec, "reviews", "closing-panel", "dispositions.jsonl"))
}
func (f *windowFixture) assertPublished(ledger string) {
	f.t.Helper()
	data, err := f.ex.FileAtRef("spec/"+closingSpec, ".beads/issues.jsonl")
	var row struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err != nil || json.Unmarshal(data, &row) != nil || row.ID != closingBead || row.Status != "closed" {
		f.t.Fatalf("close not published: %s %v", data, err)
	}
	if ledger != "" {
		current, readErr := os.ReadFile(filepath.Join(f.specWT, ledger))
		if readErr != nil || len(current) == 0 {
			f.t.Fatalf("local ledger missing: %s %v", current, readErr)
		}
		if _, err := f.ex.FileAtRef("spec/"+closingSpec, ledger); err == nil {
			f.t.Fatal("local ledger was committed")
		}
	}
}
func TestComplete_ToolArtifactDirtAfterMergeOnlyCloses(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	writeFile(t, f.beadWT, ".beads/issues.jsonl", "{\"id\":\"window-1\",\"status\":\"in_progress\",\"metadata\":{}}\n")
	artifacts, user, err := next.CheckDirtyTreeDetail(f.beadWT, f.beadWT)
	if err != nil || len(artifacts) == 0 || len(user) != 0 {
		t.Fatalf("fixture needs artifact dirt only: %v %v %v", artifacts, user, err)
	}
	r, err := f.run(CompleteOpts{})
	if err != nil || !f.closed || !r.WorktreeRemoved {
		t.Fatalf("artifact dirt blocked close: %+v %v", r, err)
	}
	f.assertPublished("")
}

func TestComplete_AfterMergeOnlyPublishesClose(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	ledger := f.registerLedger()
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed {
		t.Fatalf("%+v %v", r, err)
	}
	data, err := f.ex.FileAtRef("spec/"+closingSpec, ".beads/issues.jsonl")
	if err != nil || !strings.Contains(string(data), "\"closed\"") {
		t.Fatalf("close not published: %s %v", data, err)
	}
	if f.meta[mergeOnlySHAKey] != nil || f.meta[mergeOnlyPendingKey] != nil {
		t.Fatalf("markers remain: %v", f.meta)
	}
	if _, err := os.Stat(f.beadWT); !os.IsNotExist(err) {
		t.Fatalf("worktree remains: %v", err)
	}
	f.assertPublished(ledger)
}

func TestComplete_AfterMergeOnlyExportFailureConverges(t *testing.T) {
	f := newWindowFixture(t)
	m := f.mergedOnly()
	ledger := f.registerLedger()
	f.ex.publishErr = errors.New("export unavailable")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "closing step 4")
	if !f.closed {
		t.Fatal("close did not persist")
	}
	if _, err := os.Stat(f.beadWT); err != nil {
		t.Fatal("cleanup started before publication")
	}
	f.ex.publishErr = nil
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.WorktreeRemoved || f.meta["mindspec_landed_merge_sha"] != m {
		t.Fatalf("%+v %v", r, err)
	}
	f.assertPublished(ledger)
}

func TestComplete_ClosingPathSpecWorktreeBusyRefuses(t *testing.T) {
	for _, kind := range []string{"staged", "merge"} {
		t.Run(kind, func(t *testing.T) {
			f := newWindowFixture(t)
			f.mergedOnly()
			if kind == "staged" {
				writeFile(t, f.specWT, "busy.txt", "busy\n")
				gitRun(t, f.specWT, "add", "busy.txt")
			} else {
				gitRun(t, f.root, "checkout", "-qb", "foreign")
				writeFile(t, f.root, "other.txt", "other\n")
				gitRun(t, f.root, "add", "-A")
				gitRun(t, f.root, "commit", "-qm", "foreign")
				gitRun(t, f.root, "checkout", "-q", "main")
				gitRun(t, f.specWT, "merge", "--no-ff", "--no-commit", "foreign")
			}
			_, err := f.run(CompleteOpts{})
			requireWindowRefusal(t, err, "spec worktree is busy")
			if f.closed {
				t.Fatal("closed busy bead")
			}
		})
	}
}
func TestComplete_ClosedBeadFinishesCleanup(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	f.ex.cleanupErr = errors.New("cleanup interrupted")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "closing step 6")
	f.ex.cleanupErr = nil
	f.ex.publishErr = errors.New("export must be a no-op after publication")
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.WorktreeRemoved {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestComplete_ClosedBeadLaterRevertResumesCleanup(t *testing.T) {
	f := newWindowFixture(t)
	m := f.mergedOnly()
	f.ex.cleanupErr = errors.New("cleanup interrupted")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "closing step 6")
	gitRun(t, f.specWT, "revert", "-m", "1", "--no-edit", m)
	f.ex.cleanupErr = nil
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.WorktreeRemoved {
		t.Fatalf("closed resume ran presence check: %+v %v", r, err)
	}
}
func TestComplete_ClosedBeadResumesAllSteps(t *testing.T) {
	for step := 2; step <= 6; step++ {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			f := newWindowFixture(t)
			m := f.mergedOnly()
			ledger := f.registerLedger()
			opts := CompleteOpts{AllowDocSkew: "reviewed docs", OverrideADR: "reviewed design"}
			orig := closingStepHook
			t.Cleanup(func() { closingStepHook = orig })
			closingStepHook = func(s int) error {
				if s == step {
					return errors.New("interrupted")
				}
				return nil
			}
			_, err := f.run(opts)
			requireWindowRefusal(t, err, fmt.Sprintf("closing step %d", step))
			closingStepHook = nil
			r, err := f.run(opts)
			if err != nil || !r.WorktreeRemoved {
				t.Fatalf("%+v %v", r, err)
			}
			f.assertPublished(ledger)
			if f.meta["mindspec_landed_merge_sha"] != m || f.meta["mindspec_landed_second_parent"] != f.tip || f.meta["mindspec_landed_reviewed_sha"] != f.tip || f.meta["mindspec_doc_skew_reason"] != opts.AllowDocSkew || f.meta["mindspec_adr_override_reason"] != opts.OverrideADR {
				t.Fatalf("resume lost binding or override metadata: %v", f.meta)
			}
		})
	}
}

func TestComplete_RevertedMergeOnlyRefuses(t *testing.T) {
	f := newWindowFixture(t)
	m := f.mergedOnly()
	gitRun(t, f.specWT, "revert", "-m", "1", "--no-edit", m)
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "reverted paths")
	if f.closed {
		t.Fatal("closed reverted bead")
	}
	requireWindowRefusal(t, err, m)
}
func TestComplete_PartlyRevertedMergeOnlyRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	writeFile(t, f.specWT, "README.md", "before\n")
	gitRun(t, f.specWT, "commit", "-qam", "partly revert")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "README.md")
	if f.closed {
		t.Fatal("closed partly reverted bead")
	}
}
func TestComplete_PresenceCheckLaterEditNotRevert(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	writeFile(t, f.specWT, "README.md", "later edit\n")
	gitRun(t, f.specWT, "commit", "-qam", "later edit")
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestComplete_PresenceCheckLaterMergeExemptsPath(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	gitRun(t, f.root, "checkout", "-qb", "bead/other-1", "spec/"+closingSpec)
	writeFile(t, f.root, "README.md", "before\n")
	gitRun(t, f.root, "commit", "-qam", "other change")
	gitRun(t, f.root, "checkout", "-q", "main")
	gitRun(t, f.specWT, "merge", "--no-ff", "-qm", "Merge bead/other-1", "bead/other-1")
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestComplete_MergeOnlyRefusalEscapes(t *testing.T) {
	f := newWindowFixture(t)
	f.mergedOnly()
	writeFile(t, f.beadWT, "fix.txt", "fix\n")
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "fix\x1b[31m")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "fix")
	if strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("raw control in refusal: %q", err)
	}
	tip := gateRevParse(t, f.root, "bead/"+closingBead)
	if !strings.Contains(err.Error(), tip) {
		t.Fatal("missing full SHA")
	}
}
func TestComplete_ClosedBeadUserDirtRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.closed = true
	writeFile(t, f.beadWT, "dirty.txt", "dirty\n")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "uncommitted user changes")
	if f.meta["mindspec_landed_merge_sha"] != nil {
		t.Fatal("mutated binding before refusal")
	}
}
func TestComplete_ClosedMergedForeignMergeInProgressRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.closed = true
	gitRun(t, f.root, "checkout", "-qb", "foreign")
	writeFile(t, f.root, "foreign.txt", "foreign\n")
	gitRun(t, f.root, "add", "-A")
	gitRun(t, f.root, "commit", "-qm", "foreign")
	gitRun(t, f.root, "checkout", "-q", "main")
	gitRun(t, f.specWT, "merge", "--no-ff", "--no-commit", "foreign")
	_, err := f.run(CompleteOpts{ResolveMerge: true})
	requireWindowRefusal(t, err, "spec worktree is busy")
	if !gitutil.MergeInProgress(f.specWT) {
		t.Fatal("foreign merge changed")
	}
}
func TestComplete_ClosedOwnMergeInProgressAborts(t *testing.T) {
	f := newWindowFixture(t)
	m := f.merge()
	f.closed = true
	// A later spec commit permits another merge state for the same source.
	link, readErr := os.ReadFile(filepath.Join(f.specWT, ".git"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(string(link), "gitdir:"))
	writeFile(t, gitDir, "MERGE_HEAD", f.tip+"\n")
	_, err := f.run(CompleteOpts{ResolveMerge: true})
	requireWindowRefusal(t, err, "merge --abort")
	if gateRevParse(t, f.root, "spec/"+closingSpec) != m || !gitutil.MergeInProgress(f.specWT) {
		t.Fatal("busy path mutated merge")
	}

	writeFile(t, f.beadWT, "README.md", "advanced tip\n")
	gitRun(t, f.beadWT, "commit", "-qam", "advance while another merge is pending")
	_, err = f.run(CompleteOpts{ResolveMerge: true})
	requireWindowRefusal(t, err, "spec worktree is busy")
	if strings.Contains(err.Error(), "merge --abort") {
		t.Fatal("a merge head different from the current bead tip must not get the own-merge abort exit")
	}
}
func TestComplete_ClosedNonFlatAbsentWorktreeRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.closed = true
	// Use a canonical on-disk directory. The merge identity stays unchanged.
	if err := os.Rename(filepath.Join(f.root, ".mindspec", "specs"), filepath.Join(f.root, ".mindspec", "saved-specs")); err != nil {
		t.Fatal(err)
	}
	makeCanonicalSpecDir(t, f.root, closingSpec)
	gitRun(t, f.root, "worktree", "remove", f.beadWT)
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "no panel registration survives")
}
func TestComplete_ClosedNonFlatAbsentWorktreeGateOffAllows(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.closed = true
	if err := os.Rename(filepath.Join(f.root, ".mindspec", "specs"), filepath.Join(f.root, ".mindspec", "saved-specs")); err != nil {
		t.Fatal(err)
	}
	makeCanonicalSpecDir(t, f.root, closingSpec)
	writeFile(t, f.root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n")
	config.ResetCache()
	gitRun(t, f.root, "worktree", "remove", f.beadWT)
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.WorktreeRemoved {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestComplete_ClosedHandMergeNoPanelFailsOpen(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.closed = true
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.WorktreeRemoved {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestComplete_ClosedHandMergeStalePanelRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.closed = true
	p := panel.Panel{BeadID: windowBeadPointer(), Spec: closingSpec, Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: f.parent}
	specDir, _ := workspace.SpecDir(f.root, closingSpec)
	writeCoLocatedPanel(t, specDir, "stale-panel", p, approveVerdicts(2))
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "commits landed after review")
	if f.meta["mindspec_landed_merge_sha"] != nil {
		t.Fatal("stale panel wrote binding")
	}
}
func TestComplete_ClosedHandMergeForgedReviewedKeyRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.merge()
	f.closed = true
	f.meta["mindspec_landed_reviewed_sha"] = f.tip
	p := panel.Panel{BeadID: windowBeadPointer(), Spec: closingSpec, Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: f.parent}
	specDir, _ := workspace.SpecDir(f.root, closingSpec)
	writeCoLocatedPanel(t, specDir, "stale-panel", p, approveVerdicts(2))
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "commits landed after review")
}
func TestComplete_ClosingPathGatesMergeRange(t *testing.T) {
	f := newWindowFixture(t)
	// A source-only change at M must fail doc-sync. A later doc commit cannot
	// change M's measured range.
	writeFile(t, f.beadWT, "internal/widget/widget.go", "package widget\n")
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "source")
	m := f.merge()
	writeFile(t, f.specWT, ".mindspec/domains/widget/architecture.md", "Widget source.\n")
	gitRun(t, f.specWT, "add", "-A")
	gitRun(t, f.specWT, "commit", "-qm", "later docs")
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "doc-sync")
	requireWindowRefusal(t, err, "A later commit does not change the merge")
	if f.closed || f.meta["mindspec_landed_merge_sha"] != nil {
		t.Fatal("gate failure mutated close")
	}
	requireWindowRefusal(t, err, m)
}
func TestComplete_OverrideRunRecoveryRepeatsFlag(t *testing.T) {
	for _, step := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			f := newWindowFixture(t)
			f.merge()
			orig := closingStepHook
			t.Cleanup(func() { closingStepHook = orig })
			closingStepHook = func(s int) error {
				if s == step {
					return errors.New("fault")
				}
				return nil
			}
			_, err := f.run(CompleteOpts{AllowDocSkew: "reviewed docs", OverrideADR: "reviewed design"})
			requireWindowRefusal(t, err, "--allow-doc-skew 'reviewed docs'")
			requireWindowRefusal(t, err, "--override-adr 'reviewed design'")
			closingStepHook = nil
			r, err := f.run(CompleteOpts{AllowDocSkew: "reviewed docs", OverrideADR: "reviewed design"})
			if err != nil || !r.BeadClosed {
				t.Fatalf("%+v %v", r, err)
			}
			if f.meta["mindspec_doc_skew_reason"] != "reviewed docs" || f.meta["mindspec_adr_override_reason"] != "reviewed design" {
				t.Fatal(f.meta)
			}
		})
	}
}
func TestComplete_BranchlessReconcileUnchanged(t *testing.T) {
	f := newWindowFixture(t)
	m := f.merge()
	gitRun(t, f.root, "worktree", "remove", f.beadWT)
	gitRun(t, f.root, "branch", "-D", "bead/"+closingBead)
	f.meta["mindspec_landed_merge_sha"] = m
	f.meta["mindspec_landed_second_parent"] = f.tip
	mergedUnclosedFn = func(string, string, string) (*lifecycle.LandedMerge, bool, error) {
		return &lifecycle.LandedMerge{SHA: m, FirstParent: f.parent, SecondParent: f.tip}, true, nil
	}
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed || f.meta["mindspec_reconcile_landed_merge_sha"] != m {
		t.Fatalf("%+v %v %v", r, err, f.meta)
	}
}

func windowBeadPointer() *string { id := closingBead; return &id }

func TestComplete_FollowupRecoveryOrder(t *testing.T) {
	f := newWindowFixture(t)
	m := f.mergedOnly()
	writeFile(t, f.beadWT, "fix.txt", "fix")
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "fix")
	_, err := f.run(CompleteOpts{})
	if err == nil {
		t.Fatal("expected moved-tip refusal")
	}
	text := err.Error()
	resetLine := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, " reset --keep ") {
			resetLine = line
		}
	}
	fields := strings.Fields(resetLine)
	want := gateRevParse(t, f.root, m+"^2")
	if len(fields) == 0 || fields[len(fields)-1] != want {
		t.Fatalf("reset operand must be merge second parent %s: %s", want, resetLine)
	}
	last := -1
	for _, fragment := range []string{"recovery: git branch ", " reset --keep ", "recovery: mindspec complete ", "recovery: bd create ", "recovery: edit plan.md", "recovery: mindspec next ", " cherry-pick ", "recovery: git branch -D "} {
		pos := strings.Index(text, fragment)
		if pos <= last {
			t.Fatalf("recovery order: missing or misplaced %q in %s", fragment, text)
		}
		last = pos
	}
}

func sourceConflictWindow(t *testing.T) *windowFixture {
	t.Helper()
	f := newWindowFixture(t)
	writeFile(t, f.specWT, "README.md", "spec changed\n")
	gitRun(t, f.specWT, "commit", "-qam", "spec change")
	return f
}
func TestComplete_MergeOnlyConflictStaysMergeOnly(t *testing.T) {
	f := sourceConflictWindow(t)
	_, err := f.run(CompleteOpts{MergeOnly: true})
	requireWindowRefusal(t, err, "--merge-only --resolve-merge")
	if f.closed || !gitutil.MergeInProgress(f.specWT) {
		t.Fatal("merge-only conflict lost state")
	}
	_, err = f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "--merge-only --resolve-merge")
	if f.closed {
		t.Fatal("plain unresolved re-entry closed the pending bead")
	}
	writeFile(t, f.specWT, "README.md", "resolved\n")
	gitRun(t, f.specWT, "add", "README.md")
	r, err := f.run(CompleteOpts{ResolveMerge: true})
	if err != nil || f.closed || !r.MergeOnly {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestComplete_PlainConflictResolveFinishes(t *testing.T) {
	f := sourceConflictWindow(t)
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "--resolve-merge")
	if !f.closed || !gitutil.MergeInProgress(f.specWT) {
		t.Fatal("plain conflict changed close-before-merge semantics")
	}
	writeFile(t, f.specWT, "README.md", "resolved\n")
	gitRun(t, f.specWT, "add", "README.md")
	r, err := f.run(CompleteOpts{ResolveMerge: true})
	if err != nil || !r.BeadClosed {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestComplete_ClosedUnmergedRerunMerges(t *testing.T) {
	f := newWindowFixture(t)
	list := phaseEpicListJSONStub(closingSpec, "epic-window", state.ModeImplement)
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg == "--parent" {
				return json.Marshal([]phase.ChildInfo{{ID: closingBead, Status: "closed"}})
			}
		}
		return list(args...)
	})
	t.Cleanup(restoreList)
	ledger := f.registerLedger()
	f.closed = true
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed {
		t.Fatalf("%+v %v", r, err)
	}
	rows, err := namingMerges(f.ex, "spec/"+closingSpec, closingBead)
	if err != nil || len(rows) != 1 {
		t.Fatalf("closed unmerged bead did not merge: %v %v", rows, err)
	}

	f.assertPublished(ledger)
	m := rows[0]
	if f.meta["mindspec_landed_merge_sha"] != m.SHA || f.meta["mindspec_landed_second_parent"] != m.Parents[1] || f.meta["mindspec_landed_reviewed_sha"] != m.Parents[1] {
		t.Fatalf("binding missing: %v", f.meta)
	}
	if !r.WorktreeRemoved || f.closeCalls != 0 {
		t.Fatalf("cleanup or duplicate close: %+v closes=%d", r, f.closeCalls)
	}
	if _, err := os.Stat(f.beadWT); !os.IsNotExist(err) {
		t.Fatalf("worktree survives: %v", err)
	}
	present, err := gitutil.BranchExistsIn(f.root, "bead/"+closingBead)
	if err != nil || present {
		t.Fatalf("branch survives: %v %v", present, err)
	}
}

func TestComplete_MergeOnlyMarkerFailureRetries(t *testing.T) {
	f := newWindowFixture(t)
	mergeMeta := completeMergeMetadataFn
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if _, present := updates[mergeOnlySHAKey]; present {
			return errors.New("marker write failed")
		}
		return mergeMeta(id, updates)
	}
	var err error
	output := captureStdout(t, func() { _, err = f.run(CompleteOpts{MergeOnly: true}) })
	requireWindowRefusal(t, err, "marker write failed")
	merge := gateRevParse(t, f.root, "spec/"+closingSpec)
	if f.closed || f.meta[mergeOnlyPendingKey] != "pending" {
		t.Fatal("failed marker write lost pending mode")
	}
	for _, value := range []string{merge, f.tip, f.parent} {
		if !strings.Contains(output, value) {
			t.Fatalf("merge identity missing before marker failure: %q lacks %s", output, value)
		}
	}
	completeMergeMetadataFn = mergeMeta
	r, err := f.run(CompleteOpts{})
	if err != nil || r.MergeSHA != merge || f.closed || gateRevParse(t, f.root, "spec/"+closingSpec) != merge {
		t.Fatalf("marker retry made a new merge or closed: %+v %v", r, err)
	}
}

func TestComplete_HandMergeImpactedDomainsExitCloses(t *testing.T) {
	f := newWindowFixture(t)
	specPath := ".mindspec/specs/" + closingSpec + "/spec.md"
	writeFile(t, f.beadWT, ".mindspec/specs/"+closingSpec+"/plan.md", "---\nstatus: Approved\nspec_id: 132-window\nversion: \"1\"\nadr_citations:\n  - ADR-9001\n---\n")
	writeFile(t, f.beadWT, ".mindspec/adr/ADR-9001.md", "# ADR-9001: Gadget\n\n- **Status**: Accepted\n- **Domain(s)**: gadget\n\n## Decision\nOwn gadget.\n")
	writeFile(t, f.beadWT, ".mindspec/domains/gadget/OWNERSHIP.yaml", "paths:\n  - internal/gadget/**\n  - feature.txt\n")
	writeFile(t, f.beadWT, ".mindspec/domains/gadget/architecture.md", "Gadget owns its source.\n")
	writeFile(t, f.beadWT, "internal/gadget/gadget.go", "package gadget\n")
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "gadget implementation")
	m := f.merge()
	_, err := f.run(CompleteOpts{})
	for _, fragment := range []string{"adr-divergence-unowned", "internal/gadget/gadget.go", "add that domain to the spec's Impacted Domains and commit the spec", "commit --amend --no-edit", "--unset-metadata " + mergeOnlySHAKey, "mindspec complete " + closingBead, "--override-adr", "A later commit does not change the merge", m} {
		requireWindowRefusal(t, err, fragment)
	}
	if f.closed || f.closeCalls != 0 || f.meta["mindspec_landed_merge_sha"] != nil {
		t.Fatal("ownership refusal closed or bound the bead")
	}
	writeFile(t, f.specWT, specPath, "# Spec\n\n## Impacted Domains\n\n- widget\n- gadget\n")
	gitRun(t, f.specWT, "add", specPath)
	gitRun(t, f.specWT, "commit", "-qm", "declare gadget impact")
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed || !r.WorktreeRemoved || f.meta["mindspec_landed_merge_sha"] != m {
		t.Fatalf("Impacted Domains exit did not close the original merge: %+v %v %v", r, err, f.meta)
	}
	f.assertPublished("")
}

func TestComplete_FreshInitIgnoredLedger(t *testing.T) {
	for _, mergeOnly := range []bool{false, true} {
		t.Run(fmt.Sprint(mergeOnly), func(t *testing.T) {
			f := newWindowFixture(t)
			if _, err := bootstrap.Run(f.specWT, false); err != nil {
				t.Fatal(err)
			}
			if err := f.ex.CommitAll(f.specWT, "initialize local review ignores"); err != nil {
				t.Fatal(err)
			}
			ledger := f.registerLedgerWithExclude(false)
			before, err := os.ReadFile(filepath.Join(f.specWT, ledger))
			if err != nil {
				t.Fatal(err)
			}
			ignoreBytes, err := exec.Command("git", "-C", f.specWT, "check-ignore", "-v", "--", ledger).CombinedOutput()
			if err != nil {
				t.Fatalf("check-ignore: %s %v", ignoreBytes, err)
			}
			ignore := string(ignoreBytes)
			if !strings.HasPrefix(ignore, ".gitignore:") {
				t.Fatalf("review ignore must come from initialized .gitignore: %s", ignore)
			}
			if mergeOnly {
				f.mergedOnly()
			}
			result, err := f.run(CompleteOpts{})
			if err != nil || result == nil || !result.BeadClosed {
				t.Fatalf("ignored ledger blocked completion: %+v %v", result, err)
			}
			if mergeOnly {
				f.assertPublished(ledger)
			}
			if _, err := f.ex.FileAtRef("spec/"+closingSpec, ledger); err == nil {
				t.Fatal("ignored ledger was committed")
			}
			gitRun(t, f.specWT, "check-ignore", "-q", "--", ledger)
			if got, err := gitutil.LsFiles(f.specWT, ledger); err != nil || len(got) != 0 {
				t.Fatalf("ignored ledger became tracked: %v %v", got, err)
			}
			if after, err := os.ReadFile(filepath.Join(f.specWT, ledger)); err != nil || string(after) != string(before) {
				t.Fatalf("working ledger changed: %q %v", after, err)
			}
		})
	}
}

func TestComplete_LegacyLedgerSurvivesBeadRemoval(t *testing.T) {
	f := newWindowFixtureLayout(t, false)
	// This test checks lifecycle persistence; panel decisions have separate tests.
	writeFile(t, f.root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n")
	originalSkip := panelSkipEnvFn
	t.Cleanup(func() { panelSkipEnvFn = originalSkip })
	panelSkipEnvFn = func() bool { return true }
	dir := filepath.Join(f.beadWT, "review", "legacy")
	id := closingBead
	if err := panel.Create(dir, panel.CreateInput{
		BeadID: &id, Spec: closingSpec, Gate: "bead", Target: "bead/" + id,
		Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: f.tip, SubjectDigest: "sha256:fixture",
		Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "dispositions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// The real executor's lifecycle funnel commits the registration and ledger together.
	if err := f.ex.CommitAll(f.beadWT, "lifecycle artifacts"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"panel.json", "dispositions.jsonl"} {
		gitRun(t, f.beadWT, "cat-file", "-e", "HEAD:review/legacy/"+name)
	}
	result, err := f.run(CompleteOpts{})
	if err != nil || result == nil || !result.BeadClosed {
		t.Fatalf("legacy completion: %+v %v", result, err)
	}
	if _, err := os.Stat(f.beadWT); !os.IsNotExist(err) {
		t.Fatalf("bead worktree not removed: %v", err)
	}
	survivor := filepath.Join(f.specWT, "review", "legacy")
	if after, err := os.ReadFile(filepath.Join(survivor, "dispositions.jsonl")); err != nil || string(after) != string(before) {
		t.Fatalf("legacy ledger lost at removal: %q %v", after, err)
	}
	if legacy, err := panel.LegacyPanel(survivor); err != nil || legacy {
		t.Fatalf("approval's ledger integrity check failed: legacy=%t %v", legacy, err)
	}
	registrations := panel.Scan(f.specWT)
	if len(registrations) != 1 {
		t.Fatalf("surviving registrations: %v", registrations)
	}
	if _, err := panel.CheckHandEdits(survivor, registrations[0].Panel); err != nil {
		t.Fatalf("approval's snapshot/ledger check failed: %v", err)
	}
}
