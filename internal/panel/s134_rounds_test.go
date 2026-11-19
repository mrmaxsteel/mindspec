package panel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134Rounds_AcrossRegistrations(t *testing.T) {
	for _, gate := range []string{"bead", "final_review", "spec_approve", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			root := t.TempDir()
			input := roundCreateInput()
			input.Spec, input.Target, input.Gate = "x", "spec/x", gate
			bead := "mindspec-work.1"
			if gate == "bead" {
				input.Target, input.BeadID = "bead/"+bead, &bead
			}
			dirA, dirB := filepath.Join(root, "reviews", "a"), filepath.Join(root, "reviews", "b")
			for i := 1; i <= 3; i++ {
				input.Round = i
				createMust(t, dirA, input)
			}
			// Use the same ledger-before-snapshot writes as panel abandon.
			createAppend(t, dirA, AbandonRecord{Record: RecordAbandon, Spec: "x", Panel: "a", Reason: "replaced", By: "operator", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			old := load(dirA).Panel
			old.Abandoned, old.AbandonReason = true, "replaced"
			if err := WriteSnapshot(dirA, old); err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 2; i++ {
				input.Round = i
				createMust(t, dirB, input)
			}
			// A launch and a recover submission are redispatch, not new rounds.
			createAppend(t, dirB, struct {
				LaunchRecord
				Gate   string  `json:"gate"`
				BeadID *string `json:"bead_id"`
			}{Gate: gate, BeadID: input.BeadID, LaunchRecord: LaunchRecord{Record: RecordLaunch, Spec: "x", Panel: "b", Slot: "R1", Round: 2, Attempt: 1, Family: "harness", Model: "model-0", Status: "dispatched", RunnerKind: "harness", RunnerSource: "default", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}})
			createAppend(t, dirB, struct {
				SubmissionRecord
				Gate   string  `json:"gate"`
				BeadID *string `json:"bead_id"`
			}{Gate: gate, BeadID: input.BeadID, SubmissionRecord: SubmissionRecord{Record: RecordVerdict, Spec: "x", Panel: "b", Slot: "R1", Round: 2, Seq: 1, Status: "final", Verdict: "APPROVE", Family: "harness", Model: "model-0", RunnerKind: "harness", RunnerSource: "default", Via: "recover", FileSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}})
			// Unrelated item, spec and gate records must not inflate the result.
			other := input
			other.Spec, other.Target, other.Round = "y", "spec/y", 1
			otherBead := "mindspec-work.2"
			if gate == "bead" {
				other.Target, other.BeadID = "bead/"+otherBead, &otherBead
			}
			createMust(t, filepath.Join(root, "reviews", "other"), other)
			if gate == "bead" {
				other.Spec = "x"
				for i := 1; i <= 2; i++ {
					other.Round = i
					createMust(t, filepath.Join(root, "reviews", "second-bead"), other)
				}
			}
			other = roundCreateInput()
			other.Spec, other.Target, other.Gate = "x", "spec/x", "adhoc"
			createMust(t, filepath.Join(root, "reviews", "adhoc"), other)
			if gate == "bead" {
				// Historical rows can carry a bead id on another gate.
				// Keep this hand-written fixture independent of create's checks.
				createAppend(t, filepath.Join(root, "reviews", "adhoc"), json.RawMessage(`{"record":"round","spec":"x","panel":"adhoc","round":2,"reviewed_head_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","subject_digest":"fixture","target":"spec/x","gate":"adhoc","bead_id":"mindspec-work.1","roster":[{"slot":"R1","model":"model-0","lens":"correctness"},{"slot":"R2","model":"model-1","lens":"correctness"}],"expected_reviewers":2,"telemetry":"required","carried":{},"created_at":"2026-11-19T00:00:00Z"}`))
			}
			regs := Scan(root)
			regs = append(regs, regs[0]) // overlapping roots must not double count
			got, err := RoundsForGate(regs, "x", gate, bead)
			if err != nil || got != 5 {
				t.Fatalf("RoundsForGate = %d, %v; want 5", got, err)
			}
			// Snapshot round, abandonment, and identity are not the ledger history.
			for i := range regs {
				regs[i].Panel = Panel{Round: 99}
			}
			got, err = RoundsForGate(regs, "x", gate, bead)
			if err != nil || got != 5 {
				t.Fatalf("counter trusted snapshots: %d %v", got, err)
			}
		})
	}
}

func TestS134Rounds_LedgerOnlyAndErrors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a")
	input := roundCreateInput()
	createMust(t, dir, input)
	// A crash or damaged snapshot cannot reset the ledger count.
	if err := os.Rename(filepath.Join(dir, FileName), filepath.Join(dir, "saved-panel.json")); err != nil {
		t.Fatal(err)
	}
	got, err := RoundsForGate([]Registration{{Dir: dir}}, input.Spec, input.Gate, "")
	if err != nil || got != 1 {
		t.Fatalf("ledger-only count: %d %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, dispositionsFileName), []byte("{broken\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := RoundsForGate([]Registration{{Dir: dir}}, input.Spec, input.Gate, ""); err == nil {
		t.Fatal("malformed ledger silently reset counter")
	}
	for _, tc := range [][3]string{{"x", "unknown", ""}, {"x", "bead", ""}, {"", "final_review", ""}} {
		if _, err := RoundsForGate(nil, tc[0], tc[1], tc[2]); err == nil {
			t.Fatalf("invalid gate identity accepted: %v", tc)
		}
	}
}

func TestS134Rounds_DecodeErrorNamesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad\x1b[31m")
	createMust(t, dir, roundCreateInput())
	lines, err := ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(lines[0], &row); err != nil {
		t.Fatal(err)
	}
	// The ledger permits this extra field, but RoundRecord requires an int.
	row["verdict_schema"] = "bad"
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, dispositionsFileName), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = RoundsForGate([]Registration{{Dir: dir}}, "x", "final_review", "")
	if err == nil || !strings.Contains(err.Error(), termsafe.Escape(canonicalPath(dir))) || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("decode error must name the escaped directory: %v", err)
	}
	var typeError *json.UnmarshalTypeError
	if !errors.As(err, &typeError) {
		t.Fatalf("decode error lost its cause: %v", err)
	}
}
