package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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
	// Final-review O3-1 (spec 120): the header renders r.TargetID, which is
	// set from the UNGATED CLI arg BEFORE the SpecID/BeadID ingress gates
	// run (ValidateSpec/ValidatePlan/ValidateDivergence) and survives
	// intact on the gate-FAIL path — so the target must be escaped at BOTH
	// header render sites below, the same by-construction backstop the
	// issue.Message render already has. Escape is byte-identical for every
	// genuine (printable, single-line) target.
	if len(r.Issues) == 0 {
		target := r.TargetID
		if target == "" {
			target = r.SubCommand
		}
		return fmt.Sprintf("%s: all checks passed\n", termsafe.Escape(target))
	}

	var b strings.Builder
	target := r.TargetID
	if target == "" {
		target = r.SubCommand
	}
	b.WriteString(fmt.Sprintf("%s: %d issue(s) found\n\n", termsafe.Escape(target), len(r.Issues)))

	for _, issue := range r.Issues {
		// R4 (spec 120): issue.Message is the terminal-facing choke point for
		// every validator. Individual validators interpolate agent-writable
		// values (on-disk domain-dir basenames, plan/ADR YAML entries, bd
		// metadata); several escape at the source, but termsafe.Escape here
		// is the by-construction backstop so NO validator can forge a
		// terminal line through this render. Escape is byte-identical for the
		// (always single-line, printable) genuine messages and only quotes a
		// control-bearing one — it never double-escapes already-safe content.
		b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", issue.Severity, issue.Name, termsafe.Escape(issue.Message)))
	}

	return b.String()
}
