package validate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Severity levels for validation issues.
type Severity int

const (
	SevError   Severity = iota // validation failure
	SevWarning                 // advisory, not a failure
)

func (s Severity) String() string {
	switch s {
	case SevError:
		return "ERROR"
	case SevWarning:
		return "WARN"
	default:
		return "UNKNOWN"
	}
}

// Issue represents a single validation finding.
type Issue struct {
	Name     string   `json:"name"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Result holds the output of a validation run.
type Result struct {
	SubCommand string  `json:"sub_command"`
	TargetID   string  `json:"target_id,omitempty"`
	Issues     []Issue `json:"issues"`
}

// HasFailures returns true if any issue has error severity.
func (r *Result) HasFailures() bool {
	for _, i := range r.Issues {
		if i.Severity == SevError {
			return true
		}
	}
	return false
}

// AddError appends an error-severity issue.
func (r *Result) AddError(name, message string) {
	r.Issues = append(r.Issues, Issue{Name: name, Severity: SevError, Message: message})
}

// AddWarning appends a warning-severity issue.
func (r *Result) AddWarning(name, message string) {
	r.Issues = append(r.Issues, Issue{Name: name, Severity: SevWarning, Message: message})
}

// ToJSON returns the result as formatted JSON.
func (r *Result) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling result: %w", err)
	}
	return string(data), nil
}

// FormatText returns the result as human-readable text.
func (r *Result) FormatText() string {
	if len(r.Issues) == 0 {
		target := r.TargetID
		if target == "" {
			target = r.SubCommand
		}
		return fmt.Sprintf("%s: all checks passed\n", target)
	}

	var b strings.Builder
	target := r.TargetID
	if target == "" {
		target = r.SubCommand
	}
	b.WriteString(fmt.Sprintf("%s: %d issue(s) found\n\n", target, len(r.Issues)))

	for _, issue := range r.Issues {
		b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", issue.Severity, issue.Name, issue.Message))
	}

	return b.String()
}
