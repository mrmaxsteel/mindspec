package panel

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const ledgerTime = "2026-10-20T10:00:00Z"
const ledgerLater = "2026-10-20T11:00:00Z"

func ledgerFixtures(t *testing.T, name string) [][]byte {
	t.Helper()
	ref := Refutation{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), FindingText: "a finding", Ground: "disproven", Reason: "covered", Evidence: "abc123", By: "operator", CreatedAt: ledgerTime}
	def := Deferral{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: strings.Repeat("b", 64), FindingText: "a finding", Bead: "mindspec-followup", By: "operator", CreatedAt: ledgerTime}
	rebind := Rebind{Round: 1, FromSHA: "abc123", ToSHA: "def456", Paths: []string{"OWNERSHIP.yaml"}, Reason: "claim", CreatedAt: ledgerTime}
	records := []interface{}{
		RoundRecord{Record: RecordRound, Spec: "130", Panel: name, Round: 1, ReviewedHeadSHA: "abc123", Target: "bead/mindspec-example", Gate: "bead", Roster: []RosterSlot{{"R1", "model-a", "correctness"}, {"R2", "model-b", "architecture"}}, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: ledgerTime},
		RefutationRecord{Record: RecordRefutation, Spec: "130", Panel: name, Refutation: ref},
		DeferralRecord{Record: RecordDeferral, Spec: "130", Panel: name, Deferral: def},
		RebindRecord{Record: RecordRebind, Spec: "130", Panel: name, Rebind: rebind},
		AbandonRecord{Record: RecordAbandon, Spec: "130", Panel: name, Reason: "halt", By: "operator", CreatedAt: ledgerTime},
	}
	out := make([][]byte, len(records))
	for i, r := range records {
		out[i] = mustMarshal(t, r)
	}
	return out
}

func changeLedger(t *testing.T, data []byte, key string, value interface{}) []byte {
	t.Helper()
	m, err := decodeRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	m[key] = value
	return mustMarshal(t, m)
}

func TestLedger_RecordKeys(t *testing.T) {
	for _, raw := range ledgerFixtures(t, "p") {
		m, _ := decodeRecord(raw)
		kind := m["record"].(string)
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "p")
			if err := AppendRecords(dir, raw, raw); err != nil {
				t.Fatal(err)
			}
			lines, err := ReadLedger(dir)
			if err != nil || len(lines) != 1 {
				t.Fatalf("dedup: lines=%d err=%v", len(lines), err)
			}
			retry := changeLedger(t, raw, "created_at", ledgerLater)
			want := 1
			if kind == RecordRound || kind == RecordAbandon {
				want = 2
			}
			if err := AppendRecords(dir, retry); err != nil {
				t.Fatal(err)
			}
			lines, err = ReadLedger(dir)
			if err != nil || len(lines) != want {
				t.Fatalf("timestamp retry: lines=%d want=%d err=%v", len(lines), want, err)
			}
			// Mutate a non-key field: every kind must refuse a conflicting payload.
			field, value := "reason", "different reason"
			switch kind {
			case RecordRound:
				field, value = "target", "other-target"
			case RecordDeferral:
				field, value = "bead", "mindspec-other"
			}
			before, err := os.ReadFile(filepath.Join(dir, dispositionsFileName))
			if err != nil {
				t.Fatal(err)
			}
			if err := AppendRecords(dir, changeLedger(t, raw, field, value)); err == nil {
				t.Fatal("accepted same-key changed payload")
			}
			after, err := os.ReadFile(filepath.Join(dir, dispositionsFileName))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("conflict changed ledger")
			}
			// Every specified key component must distinguish a new event.
			var changes map[string]interface{}
			switch kind {
			case RecordRound:
				changes = map[string]interface{}{"round": 2, "created_at": "2026-10-20T12:00:00Z"}
			case RecordRefutation, RecordDeferral:
				changes = map[string]interface{}{"slot": "R2", "round": 2, "finding": "ccr[1]", "finding_sha256": strings.Repeat("c", 64)}
			case RecordRebind:
				changes = map[string]interface{}{"to_sha": "new-sha"}
			case RecordAbandon:
				changes = map[string]interface{}{"created_at": "2026-10-20T12:00:00Z"}
			}
			for field, value := range changes {
				if err := AppendRecords(dir, changeLedger(t, raw, field, value)); err != nil {
					t.Fatalf("new key field %s: %v", field, err)
				}
			}
			lines, err = ReadLedger(dir)
			if err != nil || len(lines) != want+len(changes) {
				t.Fatalf("new keys: lines=%d want=%d err=%v", len(lines), want+len(changes), err)
			}
		})
	}
}

func TestLedger_RecordKinds(t *testing.T) {
	for _, layout := range []string{"flat/reviews/p", "review/p", "adhoc/p"} {
		t.Run(layout, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), layout)
			records := ledgerFixtures(t, "p")
			if strings.HasPrefix(layout, "adhoc") {
				for i, raw := range records {
					records[i] = changeLedger(t, raw, "spec", "adhoc")
				}
			}
			if err := AppendRecords(dir, records...); err != nil {
				t.Fatal(err)
			}
			lines, err := ReadLedger(dir)
			if err != nil || len(lines) != len(records) {
				t.Fatalf("got %d records: %v", len(lines), err)
			}
			for i, line := range lines {
				if !bytes.Equal(line, records[i]) {
					t.Errorf("record %d changed", i)
				}
			}
		})
	}
}

func TestLedger_BatchRefusesBeforeAppend(t *testing.T) {
	for _, failure := range []string{"schema", "hygiene", "conflict", "within-batch", "destination"} {
		t.Run(failure, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "p")
			records := ledgerFixtures(t, "p")
			if err := AppendRecords(dir, records[0]); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(dir, dispositionsFileName))
			if err != nil {
				t.Fatal(err)
			}
			bad := []byte(`{"record":"unknown"}`)
			switch failure {
			case "hygiene":
				bad = changeLedger(t, records[4], "reason", "at /Users/operator/private")
			case "conflict":
				bad = changeLedger(t, records[0], "target", "changed")
			case "within-batch":
				bad = changeLedger(t, records[1], "reason", "changed")
			case "destination":
				bad = changeLedger(t, records[4], "panel", "other")
			}
			if err := AppendRecords(dir, records[1], bad); err == nil {
				t.Fatal("accepted invalid batch")
			}
			after, err := os.ReadFile(filepath.Join(dir, dispositionsFileName))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("partial batch appended")
			}
		})
	}
}

func TestLedger_ConcurrentBatches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	records := ledgerFixtures(t, "p")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- AppendRecords(dir, records...) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	lines, err := ReadLedger(dir)
	if err != nil || len(lines) != len(records) {
		t.Fatalf("concurrent batches: %d records, %v", len(lines), err)
	}
}

func TestLedger_ValidateKinds(t *testing.T) {
	for _, raw := range ledgerFixtures(t, "p") {
		m, _ := decodeRecord(raw)
		kind := m["record"].(string)
		t.Run(kind, func(t *testing.T) {
			if err := Validate(raw); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"spec", "panel", "created_at"} {
				bad, _ := decodeRecord(raw)
				delete(bad, field)
				if err := Validate(mustMarshal(t, bad)); err == nil {
					t.Errorf("accepted missing %s", field)
				}
			}
			for _, field := range []string{"spec", "panel", "created_at", "round", "finding_sha256", "roster", "carried", "paths", "ground"} {
				if _, ok := m[field]; !ok {
					continue
				}
				if err := Validate(changeLedger(t, raw, field, true)); err == nil {
					t.Errorf("accepted wrong type for %s", field)
				}
			}
		})
	}
}

func TestPanel_LegacyJSONRoundTrip(t *testing.T) {
	const legacy = `{"bead_id":null,"spec":"130","target":"main","round":1,"expected_reviewers":2,"reviewed_head_sha":"abc123","refutations":[{"slot":"R1","round":1,"reason":"old","evidence":"commit"}]}`
	var p Panel
	if err := json.Unmarshal([]byte(legacy), &p); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != legacy {
		t.Fatalf("legacy round trip changed bytes:\n%s", data)
	}
	dir := t.TempDir()
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	expected, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, append(expected, '\n')) {
		t.Fatal("snapshot bytes differ")
	}
	p.Round = 2
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	reg := load(dir)
	if reg.Err != nil || reg.Panel.Round != 2 {
		t.Fatalf("snapshot replacement: %+v", reg)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".panel-*.json"))
	if err != nil || len(files) != 0 {
		t.Fatalf("left temp snapshots: %v %v", files, err)
	}
}

func TestLedger_RejectsTrailingData(t *testing.T) {
	raw := ledgerFixtures(t, "p")[0]
	for _, suffix := range []string{" garbage", " {}"} {
		dir := filepath.Join(t.TempDir(), "p")
		if err := AppendRecords(dir, append(append([]byte(nil), raw...), suffix...)); err == nil {
			t.Fatal("accepted trailing data")
		}
		if _, err := os.Stat(filepath.Join(dir, dispositionsFileName)); !os.IsNotExist(err) {
			t.Fatalf("rejected append created data file: %v", err)
		}
	}
}
