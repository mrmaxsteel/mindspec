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

func (m *MockExecutor) FinalizeEpic(epicID, specID, specBranch string) (FinalizeResult, error) {
	m.record("FinalizeEpic", epicID, specID, specBranch)
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

// Compile-time interface check.
var _ Executor = (*MockExecutor)(nil)
