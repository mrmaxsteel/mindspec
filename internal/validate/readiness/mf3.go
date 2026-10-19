package readiness

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
)

type dependencyFailure struct {
	id, class, candidate, detail, recovery string
}

var dependencyFailurePrecedence = map[string]int{
	FailureIndeterminate: 0, FailureNotClosed: 1, FailureNoCandidate: 2,
	FailureOffFirstParent: 3, FailureNoEvidence: 4, FailureContradiction: 5, FailureReverted: 6,
}

func aggregateDependencyFailures(failures []dependencyFailure, confirmations []string) Signal {
	s := Signal{ID: SignalDependencies, Pass: len(failures) == 0}
	if s.Pass {
		s.Detail = strings.Join(confirmations, "; ")
		return s
	}
	sort.Slice(failures, func(i, j int) bool {
		a, b := failures[i], failures[j]
		if a.class != b.class {
			return dependencyFailurePrecedence[a.class] < dependencyFailurePrecedence[b.class]
		}
		return a.id < b.id
	})
	s.Recovery = failures[0].recovery
	var details []string
	for _, f := range failures {
		s.Failures = append(s.Failures, DependencyFailureRecord(f.id, f.class, f.candidate))
		details = append(details, fmt.Sprintf("dependency %s [%s]: %s; recovery for %s: %s", f.id, f.class, f.detail, f.id, f.recovery))
	}
	s.Failures = sortedUnique(s.Failures)
	s.Detail = strings.Join(details, "\n")
	return s
}

// The lifecycle contradiction/revert sentinels carry no structured identity.
// Extract their diagnostic's git IDs for display and failure fingerprints ONLY;
// errors.Is determines the class, never this text. Preserve the full diagnostic
// as evidence even when a future diagnostic format does not match.
var diagnosticMergeRE = regexp.MustCompile(`\bmerge ([0-9a-fA-F]{40}|[0-9a-fA-F]{64})\b`)
var diagnosticSecondParentRE = regexp.MustCompile(`\bsecond parent ([0-9a-fA-F]{40}|[0-9a-fA-F]{64})\b`)

func landedDiagnosticIdentity(err error) (merge, secondParent string) {
	if m := diagnosticMergeRE.FindStringSubmatch(err.Error()); m != nil {
		merge = m[1]
	}
	if m := diagnosticSecondParentRE.FindStringSubmatch(err.Error()); m != nil {
		secondParent = m[1]
	}
	return
}

func dependencyOverride(beadID string) string {
	return fmt.Sprintf("mindspec next %s --allow-not-ready=MF-3 --evidence <path-to-justification>", idrender.Bead(beadID))
}
