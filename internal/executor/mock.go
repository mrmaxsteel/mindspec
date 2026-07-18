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
	RevParseRefFn     func(workdir, ref string) (string, error)
	RevParseRefResult string
	RevParseRefErr    error
	StatusFn          func(workdir string) (string, error)
	StatusResult      string
	StatusErr         error
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

func (m *MockExecutor) CompleteBead(beadID, specBranch, msg string) error {
	m.record("CompleteBead", beadID, specBranch, msg)
	return m.CompleteBeadErr
}

func (m *MockExecutor) FinalizeEpic(epicID, specID, specBranch string, lifecycleAllowSet []string) (FinalizeResult, error) {
	m.record("FinalizeEpic", epicID, specID, specBranch, lifecycleAllowSet)
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

func (m *MockExecutor) ChangedFiles(base, head string) ([]string, error) {
	m.record("ChangedFiles", base, head)
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

// Compile-time interface check.
var _ Executor = (*MockExecutor)(nil)
