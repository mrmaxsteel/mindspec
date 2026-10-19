package readiness

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// CoverageResult is dispatch-ingress data, never claim authority. Applying
// an override does not change the underlying mechanical report.
type CoverageResult struct {
	Message   string
	Covered   []string
	Uncovered []string
	Failing   []string
}

// ApplyOverride covers only a validated marker's recorded failure set.
// Growth uncovers; shrinkage stays covered; indeterminate reads never cover.
// EvaluateReadiness does not read markers: the caller passes this audit data.
func ApplyOverride(report *Report, marker bead.OverrideMarker, validity bead.MarkerValidity) CoverageResult {
	result := CoverageResult{}
	lines := strings.Split(Render(report), "\n")
	named := map[string]bool{}
	if validity == bead.MarkerValid {
		for _, id := range marker.Signals {
			named[id] = true
		}
	}
	for i, signal := range report.Signals {
		if signal.Pass {
			if named[signal.ID] {
				lines[i+1] = signal.ID + ": PASS (override recorded but no longer needed)"
			}
			continue
		}
		result.Failing = append(result.Failing, signal.ID)
		covered := named[signal.ID] && len(signal.Failures) > 0
		for _, record := range signal.Failures {
			if IsIndeterminateRecord(record) {
				covered = false
			}
		}
		recorded := map[string]bool{}
		for _, digest := range marker.Fingerprints[signal.ID] {
			recorded[digest] = true
		}
		for _, digest := range FailureRecordDigests(signal) {
			if !recorded[digest] {
				covered = false
			}
		}
		if covered {
			result.Covered = append(result.Covered, signal.ID)
			lines[i+1] = fmt.Sprintf("%s: OVERRIDDEN (recorded %s; evidence %.12s %s)",
				signal.ID, termsafe.Escape(marker.Overridden), marker.Evidence.SHA256, termsafe.Escape(marker.Evidence.Path))
		} else {
			result.Uncovered = append(result.Uncovered, signal.ID)
		}
	}
	switch validity {
	case bead.MarkerLegacy:
		lines = append(lines, "override marker is legacy (no evidence) — not honoured") //nolint:misspell // spec 129 R6.3 pins this text verbatim
	case bead.MarkerMalformed:
		lines = append(lines, "override marker is malformed — not honoured") //nolint:misspell // spec 129 R6.3 pins this text verbatim
	}
	result.Message = strings.Join(lines, "\n")
	return result
}
