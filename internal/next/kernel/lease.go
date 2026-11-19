package kernel

// LeaseDeps supplies the workflow worktree adapter without importing it.
type LeaseDeps struct {
	EnsureWorktree func(root, workItem, specID string) (string, error)
}
type LeaseOptions struct {
	Step    string
	Renew   string
	Release string
	Take    bool
	Reason  string
}
type LeaseResult struct {
	Token    string
	Lease    LeaseRecord
	Worktree string
}

func Lease(state State, options LeaseOptions, deps LeaseDeps) (LeaseResult, error) {
	if !state.Enrolled {
		return LeaseResult{}, ErrPreKernel
	}
	return LeaseResult{}, ErrNotBuilt
}
