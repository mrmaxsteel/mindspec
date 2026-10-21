package panel

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const ledgerTime = "2026-10-20T10:00:00Z"
const ledgerLater = "2026-10-20T11:00:00Z"

func TestLedger_SubmissionHardBlockOptional(t *testing.T) {
	legacy := []byte(`{"record":"verdict","spec":"test","panel":"p","slot":"R1","round":1,"seq":1,"status":"final","verdict":"APPROVE","family":"harness","model":"m","runner_kind":"harness","runner_source":"default","via":"direct","attempt":0,"reason":"","file_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","created_at":"2026-10-20T10:00:00Z"}`)
	if err := Validate(legacy); err != nil {
		t.Fatalf("old record refused: %v", err)
	}
	var record SubmissionRecord
	if err := json.Unmarshal(legacy, &record); err != nil {
		t.Fatal(err)
	}
	if got := mustMarshal(t, record); !bytes.Equal(got, legacy) {
		t.Fatalf("old record bytes changed:\n%s\n%s", legacy, got)
	}
	for _, value := range []any{true, false, "true", nil, 1} {
		raw := changeLedger(t, legacy, "hard_block", value)
		_, valid := value.(bool)
		if err := Validate(raw); (err == nil) != valid {
			t.Fatalf("hard_block=%v validation: %v", value, err)
		}
	}
}

func ledgerFixtures(t *testing.T, name string) [][]byte {
	t.Helper()
	ref := Refutation{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), FindingText: "a finding", Ground: "disproven", AC: "AC-13", Reason: "covered", Evidence: "abc123", By: "operator", CreatedAt: ledgerTime}
	def := Deferral{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: strings.Repeat("b", 64), FindingText: "a finding", Bead: "mindspec-followup", By: "operator", CreatedAt: ledgerTime}
	rebind := Rebind{Round: 1, FromSHA: "abc123", ToSHA: "def456", Paths: []string{"OWNERSHIP.yaml"}, Reason: "claim", CreatedAt: ledgerTime}
	records := []interface{}{
		RoundRecord{Record: RecordRound, Spec: "130", Panel: name, Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:baseline", Target: "bead/mindspec-example", Gate: "bead", Roster: []RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}}, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: ledgerTime},
		RefutationRecord{Record: RecordRefutation, Spec: "130", Panel: name, Refutation: ref},
		DeferralRecord{Record: RecordDeferral, Spec: "130", Panel: name, Deferral: def},
		RebindRecord{Record: RecordRebind, Spec: "130", Panel: name, SubjectDigest: "sha256:baseline", Rebind: rebind},
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
	records := append(ledgerFixtures(t, "p"), mustMarshal(t, CoverageManifest{
		Record: RecordPanelManifest, Spec: "130", Panel: "p", Gate: "bead", Round: 1,
		Slots: []ManifestSlot{{Slot: "R1", Model: "model-a", Verdict: VerdictApprove}}, CreatedAt: ledgerTime,
	}))
	for _, raw := range records {
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
			if kind == RecordRound || kind == RecordAbandon || kind == RecordPanelManifest {
				want = 2
			}
			if err := AppendRecords(dir, retry); err != nil {
				t.Fatal(err)
			}
			lines, err = ReadLedger(dir)
			if err != nil || len(lines) != want {
				t.Fatalf("timestamp retry: lines=%d want=%d err=%v", len(lines), want, err)
			}
			// Keep the R5.1 keys independent of recordKey: widening a key must
			// not turn a changed payload into a new event.
			keyFields := map[string][]string{
				RecordPanelManifest: {"spec", "panel", "round", "created_at"},
				RecordRound:         {"round", "created_at"},
				RecordRefutation:    {"slot", "round", "finding", "finding_sha256"},
				RecordDeferral:      {"slot", "round", "finding", "finding_sha256"},
				RecordRebind:        {"to_sha"},
				RecordAbandon:       {"created_at"},
			}
			excluded := map[string]bool{"created_at": true}
			for _, field := range keyFields[kind] {
				excluded[field] = true
			}
			for field, original := range m {
				if excluded[field] {
					continue
				}
				t.Run("conflict/"+field, func(t *testing.T) {
					var value interface{}
					switch field {
					case "gate":
						value = "adhoc"
					case "ground":
						value = "not-load-bearing"
					case "slots":
						value = []ManifestSlot{{Slot: "R1", Model: "other-model", Verdict: VerdictApprove}}
					case "roster":
						value = []RosterSlot{{Slot: "R1", Model: "other-model", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}}
					case "carried":
						value = CarriedEntries{Refutations: []Refutation{{Slot: "R1", Round: 1, Reason: "legacy"}}}
					case "paths":
						value = []string{"other.yaml"}
					default:
						switch v := original.(type) {
						case string:
							value = v + "-changed"
						case json.Number:
							n, err := v.Int64()
							if err != nil {
								t.Fatal(err)
							}
							value = n + 1
						case bool:
							value = !v
						case nil:
							value = "mindspec-other"
						default:
							t.Fatalf("no mutation for %s (%T)", field, original)
						}
					}
					changed := changeLedger(t, raw, field, value)
					// Some fields have only one schema-valid value (telemetry),
					// or destination constraints (panel). Pin their key exclusion
					// too, so schema rejection cannot hide an expanded key.
					if field != "record" { // record selects the kind itself.
						changedMap, err := decodeRecord(changed)
						if err != nil {
							t.Fatal(err)
						}
						_, originalKey := recordKey(m)
						_, changedKey := recordKey(changedMap)
						if originalKey != changedKey {
							t.Errorf("non-key field %s changed the R5.1 key", field)
						}
					}
					before, err := os.ReadFile(filepath.Join(dir, dispositionsFileName))
					if err != nil {
						t.Fatal(err)
					}
					if err := AppendRecords(dir, changed); err == nil {
						t.Error("accepted same-key changed payload")
					}
					after, err := os.ReadFile(filepath.Join(dir, dispositionsFileName))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(before, after) {
						t.Error("conflict changed ledger")
					}
				})
			}
			// Every specified key component must distinguish a new event.
			var changes map[string]interface{}
			switch kind {
			case RecordPanelManifest:
				changes = map[string]interface{}{"spec": "131", "panel": "other", "round": 2, "created_at": "2026-10-20T12:00:00Z"}
				// The destination fixes panel, so its key component is checked
				// directly rather than appended to this panel's ledger.
				other, err := decodeRecord(changeLedger(t, raw, "panel", "other"))
				if err != nil {
					t.Fatal(err)
				}
				_, key := recordKey(m)
				_, otherKey := recordKey(other)
				if key == otherKey {
					t.Fatal("panel must distinguish manifest keys")
				}
				delete(changes, "panel")
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

func TestLedger_RejectsWhitespaceSpecBeforeAppend(t *testing.T) {
	records := map[string][]byte{
		"disposition": mustMarshal(t, testRow("blank-spec", "p", "R1", nil)),
		"manifest": mustMarshal(t, testManifest("p", 1, []ManifestSlot{
			{Slot: "R1", Model: "model-a", Verdict: VerdictApprove},
		})),
	}
	for kind, raw := range records {
		t.Run(kind, func(t *testing.T) {
			for _, state := range []string{"missing", "existing"} {
				t.Run(state, func(t *testing.T) {
					dir := filepath.Join(t.TempDir(), "p")
					path := filepath.Join(dir, dispositionsFileName)
					var before []byte
					if state == "existing" {
						if err := AppendRecords(dir, ledgerFixtures(t, "p")[0]); err != nil {
							t.Fatal(err)
						}
						var err error
						before, err = os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
					}
					bad := changeLedger(t, raw, "spec", "  ")
					// A valid first record must not be written before the bad one is refused.
					err := AppendRecords(dir, ledgerFixtures(t, "p")[1], bad)
					want := "panel: record for panel p has an empty spec field (every record must carry a non-empty spec)"
					if err == nil || err.Error() != want {
						t.Errorf("append error = %v, want %q", err, want)
					}
					after, err := os.ReadFile(path)
					if state == "missing" {
						if !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("refused batch created ledger: read error = %v", err)
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						if !bytes.Equal(before, after) {
							t.Fatal("refused batch changed ledger bytes")
						}
					}
				})
			}
		})
	}
}

func TestLedger_AppendRecordsWithCheck(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	records := ledgerFixtures(t, "p")
	if err := AppendRecords(dir, records[0]); err != nil {
		t.Fatal(err)
	}
	attempting := make(chan struct{})
	finished := make(chan struct{})
	lockResult := make(chan error, 1)
	checked := false
	err := AppendRecordsWithCheck(dir, func() error {
		checked = true
		go func() {
			close(attempting)
			unlock, err := acquireDispositionLock(filepath.Join(dir, dispositionsLockName))
			if err == nil {
				unlock()
			}
			lockResult <- err
			close(finished)
		}()
		<-attempting
		// The contender must block during the callback, then succeed once
		// AppendRecordsWithCheck releases the lock. Only the blocked leg
		// uses a short observation window; there are no scheduling sleeps.
		select {
		case <-finished:
			t.Error("dispositions lock was available while check was running")
		case <-time.After(100 * time.Millisecond):
		}
		return nil
	}, records[1])
	if err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("append did not run check")
	}
	select {
	case <-finished:
		if err := <-lockResult; err != nil {
			t.Fatalf("contender lock: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("dispositions lock remained held after append returned")
	}
	lines, err := ReadLedger(dir)
	if err != nil || len(lines) != 2 {
		t.Fatalf("checked append: %d records, %v", len(lines), err)
	}
	path := filepath.Join(dir, dispositionsFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checkErr := errors.New("check refused append")
	err = AppendRecordsWithCheck(dir, func() error { return checkErr }, records[2])
	if !errors.Is(err, checkErr) {
		t.Fatalf("failing check returned %v, want %v", err, checkErr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failing check changed dispositions.jsonl bytes")
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
	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("snapshot mode: got %04o want 0644", info.Mode().Perm())
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

func TestLedger_SnapshotUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	dir := t.TempDir()
	p := Panel{Round: 1}
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Errorf("restore directory mode: %v", err)
		}
	})
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	p.Round = 2
	if err := WriteSnapshot(dir, p); err == nil {
		t.Error("snapshot write succeeded in unwritable directory")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("failed snapshot write changed old panel.json bytes")
	}
}

func TestLedger_ValidateRejectsMalformedShapes(t *testing.T) {
	records := ledgerFixtures(t, "p")
	manifest := mustMarshal(t, CoverageManifest{
		Record: RecordPanelManifest, Spec: "130", Panel: "p", Gate: "bead", Round: 1,
		Slots: []ManifestSlot{{Slot: "R1", Model: "model-a", Verdict: VerdictApprove}}, CreatedAt: ledgerTime,
	})
	for _, tc := range []struct {
		name  string
		raw   []byte
		field string
		bad   interface{}
		want  string
	}{
		{"uppercase-hash", records[1], "finding_sha256", strings.Repeat("A", 64), "finding_sha256 must be a lowercase sha256"},
		{"short-hash", records[1], "finding_sha256", strings.Repeat("a", 62), "finding_sha256 must be a lowercase sha256"},
		{"finding-id", records[1], "finding", "ccr[-1]", "finding must be slot or ccr[i]"},
		{"refutation-ground", records[1], "ground", "unknown", "refutation ground is unknown"},
		{"nonarray-rebind-paths", records[3], "paths", "source.go", "must be an array"},
		{"round-gate", records[0], "gate", "noncanonical", "round gate must be empty or a canonical gate"},
		{"round-telemetry", records[0], "telemetry", "optional", "round telemetry must be required"},
		{"manifest-created-at", manifest, "created_at", "not-a-timestamp", "panel created_at must be RFC-3339"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.raw); err != nil {
				t.Fatalf("valid control: %v", err)
			}
			err := Validate(changeLedger(t, tc.raw, tc.field, tc.bad))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate malformed %s: err=%v want %q", tc.field, err, tc.want)
			}
		})
	}
}
