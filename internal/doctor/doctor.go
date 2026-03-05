package doctor

import "fmt"

// Status represents the result of a single health check.
type Status int

const (
	OK      Status = iota
	Missing        // expected artifact is absent
	Error          // something is wrong and needs action
	Warn           // advisory, not a failure
	Fixed          // was broken, auto-repaired by --fix
)

// Check represents a single health check result.
type Check struct {
	Name    string
	Status  Status
	Message string
	FixFunc func() error // if non-nil, --fix can auto-repair this check
}

// Report holds the results of all doctor checks.
type Report struct {
	Checks []Check
}

// HasFailures returns true if any check has Error or Missing status.
func (r *Report) HasFailures() bool {
	for _, c := range r.Checks {
		if c.Status == Error || c.Status == Missing {
			return true
		}
	}
	return false
}

// Fix runs FixFunc on all checks that have one and are in Error status.
// Fixed checks are updated to Fixed status.
func (r *Report) Fix() {
	for i := range r.Checks {
		c := &r.Checks[i]
		if c.FixFunc != nil && c.Status == Error {
			if err := c.FixFunc(); err != nil {
				c.Message += fmt.Sprintf(" (fix failed: %v)", err)
			} else {
				c.Status = Fixed
			}
		}
	}
}

// Run executes all doctor checks against the given project root.
func Run(root string) *Report {
	r := &Report{}
	checkDocs(r, root)
	checkBeads(r, root)
	checkGit(r, root)
	checkHooks(r, root)
	return r
}
