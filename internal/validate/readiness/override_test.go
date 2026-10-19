package readiness

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

func overrideReport(records ...string) *Report {
	return &Report{BeadID: "mindspec-fixture.2", Signals: []Signal{
		{ID: SignalPlanSection, Pass: true}, {ID: SignalTokens, Pass: true},
		{ID: SignalDependencies, Pass: len(records) == 0, Failures: records, Detail: "dependency failed"},
		{ID: SignalBlocking, Pass: true},
	}}
}

func TestApplyOverride(t *testing.T) {
	a := DependencyFailureRecord("mindspec-fixture.1", FailureNoCandidate, "")
	b := DependencyFailureRecord("mindspec-fixture.3", FailureNotClosed, "")
	indeterminate := DependencyFailureRecord("mindspec-fixture.1", FailureIndeterminate, "")
	markerFor := func(report *Report) bead.OverrideMarker {
		t.Helper()
		ids := []string{}
		fps := map[string][]string{}
		for _, s := range report.FailingSignals() {
			ids = append(ids, s.ID)
			fps[s.ID] = FailureRecordDigests(s)
		}
		m, err := bead.NewOverrideMarkerV2(ids, fps, bead.OverrideEvidence{Path: "evidence\n\x1b[31m", SHA256: strings.Repeat("a", 64), Bytes: 8}, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	tests := []struct {
		name               string
		current, recorded  *Report
		validity           bead.MarkerValidity
		covered, uncovered []string
		want               string
	}{
		{"matching", overrideReport(a), overrideReport(a), bead.MarkerValid, []string{"MF-3"}, nil, `MF-3: OVERRIDDEN (recorded 2026-10-19T00:00:00Z; evidence aaaaaaaaaaaa "evidence\n\x1b[31m")`},
		{"changed", overrideReport(b), overrideReport(a), bead.MarkerValid, nil, []string{"MF-3"}, "MF-3: FAIL — dependency failed"},
		{"growth", overrideReport(a, b), overrideReport(a), bead.MarkerValid, nil, []string{"MF-3"}, "MF-3: FAIL"},
		{"shrinkage", overrideReport(a), overrideReport(a, b), bead.MarkerValid, []string{"MF-3"}, nil, "MF-3: OVERRIDDEN"},
		{"passing", overrideReport(), overrideReport(a), bead.MarkerValid, nil, nil, "MF-3: PASS (override recorded but no longer needed)"},
		{"indeterminate", overrideReport(indeterminate), overrideReport(indeterminate), bead.MarkerValid, nil, []string{"MF-3"}, "MF-3: FAIL"},
		{"legacy", overrideReport(a), overrideReport(a), bead.MarkerLegacy, nil, []string{"MF-3"}, "override marker is legacy (no evidence) — not honored"},
		{"malformed", overrideReport(a), overrideReport(a), bead.MarkerMalformed, nil, []string{"MF-3"}, "override marker is malformed — not honored"},
		{"absent", overrideReport(a), overrideReport(a), bead.MarkerAbsent, nil, []string{"MF-3"}, "MF-3: FAIL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := Render(tt.current)
			got := ApplyOverride(tt.current, markerFor(tt.recorded), tt.validity)
			if !reflect.DeepEqual(got.Covered, tt.covered) || !reflect.DeepEqual(got.Uncovered, tt.uncovered) {
				t.Fatalf("coverage = %+v", got)
			}
			if !strings.Contains(got.Message, tt.want) {
				t.Fatalf("missing %q:\n%s", tt.want, got.Message)
			}
			if strings.ContainsAny(got.Message, "\x1b") {
				t.Fatal("raw terminal control byte")
			}
			if Render(tt.current) != before {
				t.Fatal("override mutated the mechanical report")
			}
		})
	}
	t.Run("blocking-occurrence-growth", func(t *testing.T) {
		r := overrideReport()
		r.Signals[3] = Signal{ID: SignalBlocking, Failures: []string{"token:description:-:1:" + strings.Repeat("a", 64)}}
		marker := markerFor(r)
		r.Signals[3].Failures = append(r.Signals[3].Failures, "token:description:-:2:"+strings.Repeat("a", 64))
		if got := ApplyOverride(r, marker, bead.MarkerValid); !reflect.DeepEqual(got.Uncovered, []string{"MF-4"}) {
			t.Fatalf("second occurrence covered: %+v", got)
		}
	})
	t.Run("empty-failure-set-fails-closed", func(t *testing.T) {
		r := overrideReport(a)
		m := markerFor(r)
		r.Signals[2].Failures = nil
		if got := ApplyOverride(r, m, bead.MarkerValid); len(got.Uncovered) != 1 {
			t.Fatalf("empty failure set covered: %+v", got)
		}
	})
}
