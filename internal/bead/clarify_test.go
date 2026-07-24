package bead

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// clarify_test.go pins spec 124 (impl-readiness-gate) Bead 3's
// WriteAttemptRecord — the R8 clarification-loop write path. Hermetic:
// every test swaps the package's own execCommand seam (the bdcli_test.go
// convention) to an in-memory metadata store, so no real bd is consulted
// and nothing is ever t.Skip'd for a missing bd.

// clarifyFakeStore returns a stateful execCommand fake: `bd show` reads
// the current in-memory metadata map (JSON round-tripped, mirroring a
// real `bd show <id> --json` response shape), and `bd update --metadata`
// REPLACES it with the write's payload — so a SECOND read within the
// same test observes the first write, proving the categorical per-bead
// cap holds even across multiple WriteAttemptRecord calls in one process
// (the cmd-level fresh-process/restart-proof half is pinned separately
// by cmd/mindspec/bead_clarify_test.go against a real bd).
func clarifyFakeStore(t *testing.T, seed map[string]interface{}) (capturedUpdates *[]map[string]interface{}) {
	t.Helper()
	origExec := execCommand
	t.Cleanup(func() { execCommand = origExec })

	current := map[string]interface{}{}
	for k, v := range seed {
		current[k] = v
	}
	var updates []map[string]interface{}

	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "show" {
			data, _ := json.Marshal([]map[string]interface{}{{"metadata": current}})
			return exec.Command("echo", string(data))
		}
		if len(args) > 0 && args[0] == "update" {
			// args: ["update", id, "--metadata", json]
			if len(args) >= 4 {
				var merged map[string]interface{}
				if err := json.Unmarshal([]byte(args[3]), &merged); err == nil {
					current = merged
					updates = append(updates, merged)
				}
			}
			return exec.Command("echo", "updated")
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return exec.Command("echo", "")
	}

	return &updates
}

func validReport() []ReportEntry {
	return []ReportEntry{
		{Ordinal: 1, Signal: "SR-2", Reason: "AC-7 has no decidable check"},
	}
}

func validClarifications() []ClarificationEntry {
	return []ClarificationEntry{
		{Ordinal: 1, Reason: "AC-7 has no decidable check", Answer: "AC-7 is verified by `go test ./foo -run TestBar`", Span: "plan.md §Bead 2: \"AC-7 — go test ./foo -run TestBar passes\""},
	}
}

// TestWriteAttemptRecord_Success pins the happy path: exactly one
// MergeMetadata write, carrying the full report + clarification entries
// under MetaKeyReadinessAttempt.
func TestWriteAttemptRecord_Success(t *testing.T) {
	updates := clarifyFakeStore(t, nil)

	record := AttemptRecord{Report: validReport(), Clarifications: validClarifications()}
	if err := WriteAttemptRecord("mindspec-abcd.3", record); err != nil {
		t.Fatalf("WriteAttemptRecord: %v", err)
	}
	if len(*updates) != 1 {
		t.Fatalf("expected exactly one metadata write, got %d: %v", len(*updates), *updates)
	}
	written, ok := (*updates)[0][MetaKeyReadinessAttempt]
	if !ok {
		t.Fatalf("expected key %q in the written metadata, got %v", MetaKeyReadinessAttempt, (*updates)[0])
	}
	writtenJSON, _ := json.Marshal(written)
	if !strings.Contains(string(writtenJSON), "AC-7 has no decidable check") {
		t.Errorf("expected the report reason to survive the round-trip, got: %s", writtenJSON)
	}
	if !strings.Contains(string(writtenJSON), "plan.md") {
		t.Errorf("expected the clarification span to survive the round-trip, got: %s", writtenJSON)
	}
}

// TestWriteAttemptRecord_PreservesUnrelatedMetadataKeys proves the write
// goes through MergeMetadata's read-merge-write (not a blind replace):
// an unrelated pre-existing metadata key survives.
func TestWriteAttemptRecord_PreservesUnrelatedMetadataKeys(t *testing.T) {
	updates := clarifyFakeStore(t, map[string]interface{}{"mindspec_phase": "implement"})

	record := AttemptRecord{Report: validReport(), Clarifications: validClarifications()}
	if err := WriteAttemptRecord("mindspec-abcd.3", record); err != nil {
		t.Fatalf("WriteAttemptRecord: %v", err)
	}
	if len(*updates) != 1 {
		t.Fatalf("expected exactly one write, got %d", len(*updates))
	}
	if (*updates)[0]["mindspec_phase"] != "implement" {
		t.Errorf("expected the unrelated mindspec_phase key to survive, got %v", (*updates)[0])
	}
}

// TestWriteAttemptRecord_RefusesWhenAlreadyPresent pins the categorical
// per-bead cap (R8d): a bead already carrying MetaKeyReadinessAttempt
// refuses a second write, with NO update call issued.
func TestWriteAttemptRecord_RefusesWhenAlreadyPresent(t *testing.T) {
	seed := map[string]interface{}{
		MetaKeyReadinessAttempt: map[string]interface{}{"report": []interface{}{}, "clarifications": []interface{}{}},
	}
	updates := clarifyFakeStore(t, seed)

	record := AttemptRecord{Report: validReport(), Clarifications: validClarifications()}
	err := WriteAttemptRecord("mindspec-abcd.3", record)
	if err == nil {
		t.Fatal("expected a refusal when an attempt record already exists")
	}
	if !strings.Contains(err.Error(), "already carries a readiness-attempt record") {
		t.Errorf("expected the cap-refusal message, got: %v", err)
	}
	if len(*updates) != 0 {
		t.Errorf("expected NO metadata write on a refused second attempt, got %d: %v", len(*updates), *updates)
	}
}

// TestWriteAttemptRecord_RefusesWhenAlreadyPresent_EvenWithNewReasons
// pins that the cap is CATEGORICAL, not keyed to "the same reason": a
// record citing an entirely different ordinal/reason is STILL refused
// once any attempt record exists.
func TestWriteAttemptRecord_RefusesWhenAlreadyPresent_EvenWithNewReasons(t *testing.T) {
	seed := map[string]interface{}{
		MetaKeyReadinessAttempt: map[string]interface{}{"report": []interface{}{}, "clarifications": []interface{}{}},
	}
	updates := clarifyFakeStore(t, seed)

	record := AttemptRecord{
		Report:         []ReportEntry{{Ordinal: 7, Signal: "SR-4", Reason: "a brand-new, never-before-seen reason"}},
		Clarifications: []ClarificationEntry{{Ordinal: 7, Reason: "a brand-new, never-before-seen reason", Answer: "resolved", Span: "spec.md §R1"}},
	}
	if err := WriteAttemptRecord("mindspec-abcd.3", record); err == nil {
		t.Fatal("expected a refusal even for a record with entirely new ordinals/reasons")
	}
	if len(*updates) != 0 {
		t.Errorf("expected NO metadata write, got %d", len(*updates))
	}
}

// TestWriteAttemptRecord_RefusesUnknownOrdinal pins the source-span-
// grounding ingress: a clarification citing an ordinal absent from the
// record's own report is refused, zero write.
func TestWriteAttemptRecord_RefusesUnknownOrdinal(t *testing.T) {
	updates := clarifyFakeStore(t, nil)

	record := AttemptRecord{
		Report:         validReport(),
		Clarifications: []ClarificationEntry{{Ordinal: 99, Reason: "does not exist", Answer: "x", Span: "spec.md §R1"}},
	}
	if err := WriteAttemptRecord("mindspec-abcd.3", record); err == nil {
		t.Fatal("expected a refusal for an unknown ordinal")
	}
	if len(*updates) != 0 {
		t.Errorf("expected NO metadata write, got %d", len(*updates))
	}
}

// TestWriteAttemptRecord_RefusesMissingSpan pins the presence-only span
// check (R8b): an empty/whitespace-only Span is refused.
func TestWriteAttemptRecord_RefusesMissingSpan(t *testing.T) {
	for _, span := range []string{"", "   "} {
		updates := clarifyFakeStore(t, nil)
		record := AttemptRecord{
			Report:         validReport(),
			Clarifications: []ClarificationEntry{{Ordinal: 1, Reason: "AC-7 has no decidable check", Answer: "x", Span: span}},
		}
		if err := WriteAttemptRecord("mindspec-abcd.3", record); err == nil {
			t.Errorf("expected a refusal for span %q", span)
		}
		if len(*updates) != 0 {
			t.Errorf("expected NO metadata write for span %q, got %d", span, len(*updates))
		}
	}
}

// TestWriteAttemptRecord_RefusesEmptyReport pins that an attempt record
// with no report entries at all is refused (nothing to clarify against).
func TestWriteAttemptRecord_RefusesEmptyReport(t *testing.T) {
	updates := clarifyFakeStore(t, nil)
	if err := WriteAttemptRecord("mindspec-abcd.3", AttemptRecord{}); err == nil {
		t.Fatal("expected a refusal for an empty report")
	}
	if len(*updates) != 0 {
		t.Errorf("expected NO metadata write, got %d", len(*updates))
	}
}

// TestWriteAttemptRecord_RefusesDuplicateOrNonPositiveOrdinal pins the
// report's own ordinal-shape ingress.
func TestWriteAttemptRecord_RefusesDuplicateOrNonPositiveOrdinal(t *testing.T) {
	cases := []struct {
		name   string
		report []ReportEntry
	}{
		{"duplicate", []ReportEntry{
			{Ordinal: 1, Signal: "SR-1", Reason: "a"},
			{Ordinal: 1, Signal: "SR-2", Reason: "b"},
		}},
		{"zero", []ReportEntry{{Ordinal: 0, Signal: "SR-1", Reason: "a"}}},
		{"negative", []ReportEntry{{Ordinal: -1, Signal: "SR-1", Reason: "a"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updates := clarifyFakeStore(t, nil)
			if err := WriteAttemptRecord("mindspec-abcd.3", AttemptRecord{Report: tc.report}); err == nil {
				t.Fatal("expected a refusal")
			}
			if len(*updates) != 0 {
				t.Errorf("expected NO metadata write, got %d", len(*updates))
			}
		})
	}
}

// TestWriteAttemptRecord_MalformedBeadID pins the ingress refusal: a
// hostile bead-ID argument is refused at idvalidate before any bd read.
func TestWriteAttemptRecord_MalformedBeadID(t *testing.T) {
	origExec := execCommand
	t.Cleanup(func() { execCommand = origExec })
	execCommand = func(name string, args ...string) *exec.Cmd {
		t.Fatalf("no bd call expected for a malformed bead id: %s %v", name, args)
		return exec.Command("echo", "")
	}

	for _, id := range []string{"mindspec-1\n--help", "mindspec-1;evil", ""} {
		if err := WriteAttemptRecord(id, AttemptRecord{Report: validReport()}); err == nil {
			t.Errorf("WriteAttemptRecord(%q): expected a refusal", id)
		}
	}
}

// TestWriteAttemptRecord_ReadFailurePropagates pins fail-closed on a
// metadata-read error (mirrors MergeMetadata's own fail-closed contract):
// no write is attempted when the existing-record check cannot be
// performed.
func TestWriteAttemptRecord_ReadFailurePropagates(t *testing.T) {
	origExec := execCommand
	t.Cleanup(func() { execCommand = origExec })
	updateCalled := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "show" {
			return exec.Command("false")
		}
		if len(args) > 0 && args[0] == "update" {
			updateCalled = true
		}
		return exec.Command("echo", "unexpected")
	}

	if err := WriteAttemptRecord("mindspec-abcd.3", AttemptRecord{Report: validReport()}); err == nil {
		t.Fatal("expected an error on a metadata read failure")
	}
	if updateCalled {
		t.Error("a read failure must not proceed to a write")
	}
}
