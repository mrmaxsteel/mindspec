package executor

import "sync"

// MockExecutor implements Executor for testing. It records all method calls
// and returns configurable results. Safe for concurrent use.
type MockExecutor struct {
	mu    sync.Mutex
	Calls []MockCall

	// Return values (set before test).
	InitSpecWorkspaceResult WorkspaceInfo
	InitSpecWorkspaceErr    error
	HandoffEpicErr          error
	DispatchBeadResult      WorkspaceInfo
	DispatchBeadErr         error
	CompleteBeadErr         error
	FinalizeEpicResult      FinalizeResult
	FinalizeEpicErr         error
	CleanupErr              error
	IsTreeCleanErr          error
	DiffStatResult          string
	DiffStatErr             error
	CommitCountResult       int
	CommitCountErr          error
	CommitAllErr            error
	ChangedFilesResult      []string
	ChangedFilesErr         error

	// ChangedFilesOptionsFn lets rebind tests inspect the diff options.
	ChangedFilesOptionsFn func(base, head string, options []ChangedFilesOptions) ([]string, error)

	// ChangedFilesFn, when non-nil, takes precedence over
	// ChangedFilesResult/ChangedFilesErr so tests can return
	// range-dependent diffs (e.g. the per-bead gate anchoring tests,
	// which must distinguish the bead range from any other range).
	ChangedFilesFn  func(base, head string) ([]string, error)
	FileAtRefResult []byte
	FileAtRefErr    error
	// FileAtRefOrAbsentFn, when non-nil, fully drives FileAtRefOrAbsent
	// so tests can return ref- and path-dependent presence/absence and
	// operational errors (the ref-anchored OWNERSHIP loader's three-way
	// classification: present / absent / operational error). When nil,
	// the simple result fields below are returned.
	FileAtRefOrAbsentFn      func(ref, path string) ([]byte, bool, error)
	FileAtRefOrAbsentData    []byte
	FileAtRefOrAbsentPresent bool
	FileAtRefOrAbsentErr     error
	// TreeDirsAtRefFn, when non-nil, fully drives TreeDirsAtRef so tests
	// can return ref-dependent domain-directory enumerations. When nil,
	// the simple result fields below are returned.
	TreeDirsAtRefFn     func(ref, dirPath string) ([]string, error)
	TreeDirsAtRefResult []string
	TreeDirsAtRefErr    error
	MergeBaseResult     string
	MergeBaseErr        error
	// Panel-gate git seams (spec 099). RevParseRefFn / StatusFn / IsRefNotFoundFn,
	// when non-nil, fully drive the corresponding method so tests can inject
	// staleness / dirty-tree / ref-not-found facts; otherwise the simple result
	// fields are returned.
	LsTreeFn          func(workdir, rev string) (string, error)
	LsTreeResult      string
	LsTreeErr         error
	RevParseRefFn     func(workdir, ref string) (string, error)
	RevParseRefResult string
	RevParseRefErr    error
	StatusFn          func(workdir string) (string, error)
	StatusResult      string
	StatusErr         error
	CheckIgnoreFn     func(workdir, file string) error
	CheckIgnoreErr    error
	IsRefNotFoundFn   func(error) bool

	// Layout-mover git primitives (spec 106 Bead 3). The *Fn hooks, when
	// non-nil, fully drive the corresponding method so tests can inject
	// ref-discovery / fingerprint facts and per-call errors; otherwise the
	// simple result/err fields are returned.
	GitMvFn                  func(workdir, src, dst string) error
	GitMvErr                 error
	ResetHardFn              func(workdir, ref string) error
	ResetHardErr             error
	CleanForceFn             func(workdir string) error
	CleanForceErr            error
	CleanForcePathsFn        func(workdir string, paths []string) error
	CleanForcePathsErr       error
	CommitPathsFn            func(workdir, msg string, paths []string) error
	CommitPathsErr           error
	LocalBranchRefsFn        func(workdir string) ([]string, error)
	LocalBranchRefsResult    []string
	LocalBranchRefsErr       error
	RemoteTrackingRefsFn     func(workdir string) ([]string, error)
	RemoteTrackingRefsResult []string
	RemoteTrackingRefsErr    error

	// Prospective-snapshot / write-ahead materialization seams (spec 128
	// Bead 6). The *Fn hooks, when non-nil, fully drive the corresponding
	// method; otherwise the simple result/err fields are returned. Every
	// field defaults to its Go zero value, which drives the "nothing to
	// materialize" (both stages elide) path for any test that never
	// configures these.
	SnapshotWorktreeTreeFn       func(wtPath string, exclude []string) (string, error)
	SnapshotWorktreeTreeResult   string
	SnapshotWorktreeTreeErr      error
	ChangedFilesTreeFn           func(workdir, base, treeOID string) ([]string, error)
	ChangedFilesTreeResult       []string
	ChangedFilesTreeErr          error
	MaterializeStageCommitFn     func(wtPath, tree, parent, stage, subject string) (string, error)
	MaterializeStageCommitResult string
	MaterializeStageCommitErr    error
	PublishStageMarkerFn         func(beadID, oid, expectedOld string) error
	PublishStageMarkerErr        error
	ReadStageMarkerFn            func(beadID string) (string, string, bool, error)
	ClearStageMarkerFn           func(beadID, expectedOID string) error
	ClearStageMarkerErr          error
	AdvanceBranchExpectedOldFn   func(branch, newSHA, expectedOld string) error
	AdvanceBranchExpectedOldErr  error
	ResetIndexToHeadFn           func(wtPath string) error
	ResetIndexToHeadErr          error
}

// MockCall records a single method invocation.
type MockCall struct {
	Method string
	Args   []interface{}
}

func (m *MockExecutor) record(method string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, MockCall{Method: method, Args: args})
}

// CallsTo returns all recorded calls to the named method.
func (m *MockExecutor) CallsTo(method string) []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []MockCall
	for _, c := range m.Calls {
		if c.Method == method {
			result = append(result, c)
		}
	}
	return result
}

func (m *MockExecutor) InitSpecWorkspace(specID string) (WorkspaceInfo, error) {
	m.record("InitSpecWorkspace", specID)
	return m.InitSpecWorkspaceResult, m.InitSpecWorkspaceErr
}

func (m *MockExecutor) HandoffEpic(epicID, specID string, beadIDs []string) error {
	m.record("HandoffEpic", epicID, specID, beadIDs)
	return m.HandoffEpicErr
}

func (m *MockExecutor) DispatchBead(beadID, specID string) (WorkspaceInfo, error) {
	m.record("DispatchBead", beadID, specID)
	return m.DispatchBeadResult, m.DispatchBeadErr
}

func (m *MockExecutor) CompleteBead(beadID, specBranch, msg, overrideReason string, resolveMerge bool) error {
	m.record("CompleteBead", beadID, specBranch, msg, overrideReason, resolveMerge)
	return m.CompleteBeadErr
}

func (m *MockExecutor) FinalizeEpic(epicID, specID, specBranch string, lifecycleAllowSet []string, overrideReason string, resolveMerge bool) (FinalizeResult, error) {
	m.record("FinalizeEpic", epicID, specID, specBranch, lifecycleAllowSet, overrideReason, resolveMerge)
	return m.FinalizeEpicResult, m.FinalizeEpicErr
}

func (m *MockExecutor) Cleanup(specID string, force bool) error {
	m.record("Cleanup", specID, force)
	return m.CleanupErr
}

func (m *MockExecutor) IsTreeClean(path string) error {
	m.record("IsTreeClean", path)
	return m.IsTreeCleanErr
}

func (m *MockExecutor) DiffStat(base, head string) (string, error) {
	m.record("DiffStat", base, head)
	return m.DiffStatResult, m.DiffStatErr
}

func (m *MockExecutor) CommitCount(base, head string) (int, error) {
	m.record("CommitCount", base, head)
	return m.CommitCountResult, m.CommitCountErr
}

func (m *MockExecutor) CommitAll(path, msg string) error {
	m.record("CommitAll", path, msg)
	return m.CommitAllErr
}

func (m *MockExecutor) ChangedFiles(base, head string, options ...ChangedFilesOptions) ([]string, error) {
	m.record("ChangedFiles", base, head)
	if m.ChangedFilesOptionsFn != nil {
		return m.ChangedFilesOptionsFn(base, head, options)
	}
	if m.ChangedFilesFn != nil {
		return m.ChangedFilesFn(base, head)
	}
	return m.ChangedFilesResult, m.ChangedFilesErr
}

func (m *MockExecutor) FileAtRef(ref, path string) ([]byte, error) {
	m.record("FileAtRef", ref, path)
	return m.FileAtRefResult, m.FileAtRefErr
}

func (m *MockExecutor) FileAtRefOrAbsent(ref, path string) ([]byte, bool, error) {
	m.record("FileAtRefOrAbsent", ref, path)
	if m.FileAtRefOrAbsentFn != nil {
		return m.FileAtRefOrAbsentFn(ref, path)
	}
	return m.FileAtRefOrAbsentData, m.FileAtRefOrAbsentPresent, m.FileAtRefOrAbsentErr
}

func (m *MockExecutor) TreeDirsAtRef(ref, dirPath string) ([]string, error) {
	m.record("TreeDirsAtRef", ref, dirPath)
	if m.TreeDirsAtRefFn != nil {
		return m.TreeDirsAtRefFn(ref, dirPath)
	}
	return m.TreeDirsAtRefResult, m.TreeDirsAtRefErr
}

func (m *MockExecutor) MergeBase(a, b string) (string, error) {
	m.record("MergeBase", a, b)
	return m.MergeBaseResult, m.MergeBaseErr
}

func (m *MockExecutor) LsTree(workdir, rev string) (string, error) {
	m.record("LsTree", workdir, rev)
	if m.LsTreeFn != nil {
		return m.LsTreeFn(workdir, rev)
	}
	return m.LsTreeResult, m.LsTreeErr
}

func (m *MockExecutor) RevParseRef(workdir, ref string) (string, error) {
	m.record("RevParseRef", workdir, ref)
	if m.RevParseRefFn != nil {
		return m.RevParseRefFn(workdir, ref)
	}
	return m.RevParseRefResult, m.RevParseRefErr
}

func (m *MockExecutor) Status(workdir string) (string, error) {
	m.record("Status", workdir)
	if m.StatusFn != nil {
		return m.StatusFn(workdir)
	}
	return m.StatusResult, m.StatusErr
}

func (m *MockExecutor) IsRefNotFound(err error) bool {
	m.record("IsRefNotFound", err)
	if m.IsRefNotFoundFn != nil {
		return m.IsRefNotFoundFn(err)
	}
	return false
}

func (m *MockExecutor) GitMv(workdir, src, dst string) error {
	m.record("GitMv", workdir, src, dst)
	if m.GitMvFn != nil {
		return m.GitMvFn(workdir, src, dst)
	}
	return m.GitMvErr
}

func (m *MockExecutor) ResetHard(workdir, ref string) error {
	m.record("ResetHard", workdir, ref)
	if m.ResetHardFn != nil {
		return m.ResetHardFn(workdir, ref)
	}
	return m.ResetHardErr
}

func (m *MockExecutor) CleanForce(workdir string) error {
	m.record("CleanForce", workdir)
	if m.CleanForceFn != nil {
		return m.CleanForceFn(workdir)
	}
	return m.CleanForceErr
}

func (m *MockExecutor) CleanForcePaths(workdir string, paths []string) error {
	m.record("CleanForcePaths", workdir, paths)
	if m.CleanForcePathsFn != nil {
		return m.CleanForcePathsFn(workdir, paths)
	}
	return m.CleanForcePathsErr
}

func (m *MockExecutor) CommitPaths(workdir, msg string, paths []string) error {
	m.record("CommitPaths", workdir, msg, paths)
	if m.CommitPathsFn != nil {
		return m.CommitPathsFn(workdir, msg, paths)
	}
	return m.CommitPathsErr
}

func (m *MockExecutor) LocalBranchRefs(workdir string) ([]string, error) {
	m.record("LocalBranchRefs", workdir)
	if m.LocalBranchRefsFn != nil {
		return m.LocalBranchRefsFn(workdir)
	}
	return m.LocalBranchRefsResult, m.LocalBranchRefsErr
}

func (m *MockExecutor) RemoteTrackingRefs(workdir string) ([]string, error) {
	m.record("RemoteTrackingRefs", workdir)
	if m.RemoteTrackingRefsFn != nil {
		return m.RemoteTrackingRefsFn(workdir)
	}
	return m.RemoteTrackingRefsResult, m.RemoteTrackingRefsErr
}

// --- prospective-snapshot / write-ahead materialization (spec 128 Bead 6) --

func (m *MockExecutor) SnapshotWorktreeTree(wtPath string, exclude []string) (string, error) {
	m.record("SnapshotWorktreeTree", wtPath, exclude)
	if m.SnapshotWorktreeTreeFn != nil {
		return m.SnapshotWorktreeTreeFn(wtPath, exclude)
	}
	return m.SnapshotWorktreeTreeResult, m.SnapshotWorktreeTreeErr
}

// ChangedFilesTree records its own call, then dispatches: an explicit
// ChangedFilesTreeFn always wins; an explicitly-configured (non-nil, even
// if empty) ChangedFilesTreeResult or a non-nil ChangedFilesTreeErr wins
// next; otherwise it DELEGATES to ChangedFiles(base, treeOID) — reusing
// whatever ChangedFilesResult/ChangedFilesFn a test already configured.
// This default keeps every PRE-bead-6 test that drives the per-bead
// content gates via ChangedFilesResult/ChangedFilesFn working unchanged
// after those gates moved onto the tree-vs-tree leg (spec 128 bead 6) —
// only a test that cares about tree-vs-tree-specific behavior need
// configure the ChangedFilesTree* fields at all.
func (m *MockExecutor) ChangedFilesTree(workdir, base, treeOID string) ([]string, error) {
	m.record("ChangedFilesTree", workdir, base, treeOID)
	if m.ChangedFilesTreeFn != nil {
		return m.ChangedFilesTreeFn(workdir, base, treeOID)
	}
	if m.ChangedFilesTreeResult != nil || m.ChangedFilesTreeErr != nil {
		return m.ChangedFilesTreeResult, m.ChangedFilesTreeErr
	}
	return m.ChangedFiles(base, treeOID)
}

func (m *MockExecutor) MaterializeStageCommit(wtPath, tree, parent, stage, subject string) (string, error) {
	m.record("MaterializeStageCommit", wtPath, tree, parent, stage, subject)
	if m.MaterializeStageCommitFn != nil {
		return m.MaterializeStageCommitFn(wtPath, tree, parent, stage, subject)
	}
	return m.MaterializeStageCommitResult, m.MaterializeStageCommitErr
}

func (m *MockExecutor) PublishStageMarker(beadID, oid, expectedOld string) error {
	m.record("PublishStageMarker", beadID, oid, expectedOld)
	if m.PublishStageMarkerFn != nil {
		return m.PublishStageMarkerFn(beadID, oid, expectedOld)
	}
	return m.PublishStageMarkerErr
}

func (m *MockExecutor) ReadStageMarker(beadID string) (string, string, bool, error) {
	m.record("ReadStageMarker", beadID)
	if m.ReadStageMarkerFn != nil {
		return m.ReadStageMarkerFn(beadID)
	}
	return "", "", false, nil
}

func (m *MockExecutor) ClearStageMarker(beadID, expectedOID string) error {
	m.record("ClearStageMarker", beadID, expectedOID)
	if m.ClearStageMarkerFn != nil {
		return m.ClearStageMarkerFn(beadID, expectedOID)
	}
	return m.ClearStageMarkerErr
}

func (m *MockExecutor) AdvanceBranchExpectedOld(branch, newSHA, expectedOld string) error {
	m.record("AdvanceBranchExpectedOld", branch, newSHA, expectedOld)
	if m.AdvanceBranchExpectedOldFn != nil {
		return m.AdvanceBranchExpectedOldFn(branch, newSHA, expectedOld)
	}
	return m.AdvanceBranchExpectedOldErr
}

func (m *MockExecutor) ResetIndexToHead(wtPath string) error {
	m.record("ResetIndexToHead", wtPath)
	if m.ResetIndexToHeadFn != nil {
		return m.ResetIndexToHeadFn(wtPath)
	}
	return m.ResetIndexToHeadErr
}

// Compile-time interface check.
var _ Executor = (*MockExecutor)(nil)

var _ LandedReviewSetter = (*MockExecutor)(nil)

// SetLandedReview records the reviewed identity supplied before CompleteBead.
func (m *MockExecutor) SetLandedReview(sha, panel string) {
	m.record("SetLandedReview", sha, panel)
}

// CheckIgnore records the ledger preflight; default is a non-ignored path.
func (m *MockExecutor) CheckIgnore(workdir, file string) error {
	m.record("CheckIgnore", workdir, file)
	if m.CheckIgnoreFn != nil {
		return m.CheckIgnoreFn(workdir, file)
	}
	if m.CheckIgnoreErr != nil {
		return m.CheckIgnoreErr
	}
	return mockNotIgnored{}
}

type mockNotIgnored struct{}

func (mockNotIgnored) Error() string { return "exit status 1" }
func (mockNotIgnored) ExitCode() int { return 1 }
