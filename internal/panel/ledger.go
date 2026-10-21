package panel

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const (
	RecordVerdict      = "verdict"
	RecordLaunch       = "launch"
	RecordSubstitution = "substitution"
	RecordRound        = "round"
	RecordRefutation   = "refutation"
	RecordDeferral     = "deferral"
	RecordRebind       = "rebind"
	RecordAbandon      = "abandon"
)

// IsVerbRecord identifies event kinds reserved to panel verbs.
func IsVerbRecord(kind string) bool {
	switch kind {
	case RecordRound, RecordRefutation, RecordDeferral, RecordRebind, RecordAbandon, RecordVerdict, RecordLaunch, RecordSubstitution:
		return true
	}
	return false
}

// CarriedEntries is the first round event's baseline for legacy snapshot entries.
type CarriedEntries struct {
	Refutations []Refutation `json:"refutations,omitempty"`
	Deferrals   []Deferral   `json:"deferrals,omitempty"`
	Rebinds     []Rebind     `json:"rebinds,omitempty"`
}

// RoundRecord captures the registration established by create.
type RoundRecord struct {
	VerdictSchema     int            `json:"verdict_schema,omitempty"`
	Record            string         `json:"record"`
	Spec              string         `json:"spec"`
	Panel             string         `json:"panel"`
	Round             int            `json:"round"`
	ReviewedHeadSHA   string         `json:"reviewed_head_sha"`
	SubjectDigest     string         `json:"subject_digest"`
	Target            string         `json:"target"`
	Gate              string         `json:"gate"`
	BeadID            *string        `json:"bead_id"`
	Roster            []RosterSlot   `json:"roster"`
	ExpectedReviewers int            `json:"expected_reviewers"`
	Telemetry         string         `json:"telemetry"`
	Carried           CarriedEntries `json:"carried"`
	CreatedAt         string         `json:"created_at"`
}

// RefutationRecord and DeferralRecord share their payload with the snapshot.
type RefutationRecord struct {
	Record string `json:"record"`
	Spec   string `json:"spec"`
	Panel  string `json:"panel"`
	Refutation
}
type DeferralRecord struct {
	Record string `json:"record"`
	Spec   string `json:"spec"`
	Panel  string `json:"panel"`
	Deferral
}

type RebindRecord struct {
	Record        string `json:"record"`
	Spec          string `json:"spec"`
	Panel         string `json:"panel"`
	SubjectDigest string `json:"subject_digest"`
	Rebind
}

type AbandonRecord struct {
	Record    string `json:"record"`
	Spec      string `json:"spec"`
	Panel     string `json:"panel"`
	Reason    string `json:"reason"`
	By        string `json:"by"`
	CreatedAt string `json:"created_at"`
}

func validateLedgerMap(m map[string]interface{}, kind string) error {
	fields := []string{"spec", "panel", "created_at"}
	switch kind {
	case RecordVerdict:
		fields = append(fields, "slot", "status", "verdict", "family", "model", "runner_kind", "runner_source", "via", "file_sha256")
	case RecordLaunch:
		fields = append(fields, "slot", "family", "model", "status", "runner_kind", "runner_source")
	case RecordSubstitution:
		fields = append(fields, "slot", "model")
	case RecordRound:
		fields = append(fields, "reviewed_head_sha", "subject_digest", "target", "telemetry")
	case RecordRefutation, RecordDeferral:
		fields = append(fields, "slot", "finding", "finding_sha256", "finding_text", "by")
		if kind == RecordRefutation {
			fields = append(fields, "ground", "reason", "evidence")
		} else {
			fields = append(fields, "bead")
		}
	case RecordRebind:
		fields = append(fields, "from_sha", "to_sha", "subject_digest", "reason")
	case RecordAbandon:
		fields = append(fields, "reason", "by")
	}
	for _, f := range fields {
		s, err := requireString(m, f, kind)
		if err != nil {
			return err
		}
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s field %s must not be blank", kind, f)
		}
	}
	if _, err := time.Parse(time.RFC3339, m["created_at"].(string)); err != nil {
		return fmt.Errorf("%s created_at must be RFC-3339", kind)
	}
	if kind != RecordAbandon && kind != RecordSubstitution {
		if err := requireInt(m, "round", kind); err != nil {
			return err
		}
		n, _ := m["round"].(json.Number).Int64()
		if n < 1 {
			return fmt.Errorf("%s round must be positive", kind)
		}
	}
	switch kind {
	case RecordVerdict, RecordLaunch, RecordSubstitution:
		return validateExecutionRecord(m, kind)
	case RecordRound:
		gate, ok := m["gate"].(string)
		if !ok {
			return fmt.Errorf("round gate must be a string")
		}
		// An omitted create --gate retains the snapshot's empty gate. It still
		// uses the adhoc roster defaults, and must match its round event exactly.
		if gate != "" && !isValidGate(gate) {
			return fmt.Errorf("round gate must be empty or a canonical gate")
		}
		if m["telemetry"] != "required" {
			return fmt.Errorf("round telemetry must be required")
		}
		bead, ok := m["bead_id"]
		if !ok {
			return fmt.Errorf("round missing bead_id")
		}
		if bead != nil {
			if _, ok := bead.(string); !ok {
				return fmt.Errorf("round bead_id must be a string or null")
			}
		}
		roster, ok := m["roster"].([]interface{})
		if !ok || len(roster) < 2 {
			return fmt.Errorf("round roster must have at least two slots")
		}
		for i, raw := range roster {
			slot, ok := raw.(map[string]interface{})
			if !ok {
				return fmt.Errorf("round roster entry must be an object")
			}
			for _, f := range []string{"slot", "model", "lens"} {
				if _, err := requireString(slot, f, "round roster"); err != nil {
					return err
				}
			}
			if slot["slot"] != fmt.Sprintf("R%d", i+1) {
				return fmt.Errorf("round roster slot must be R%d", i+1)
			}
		}
		if err := requireInt(m, "expected_reviewers", kind); err != nil {
			return err
		}
		n, _ := m["expected_reviewers"].(json.Number).Int64()
		if n != int64(len(roster)) {
			return fmt.Errorf("round expected_reviewers must equal roster length")
		}
		carried, ok := m["carried"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("round carried must be an object")
		}
		// Legacy entries may lack fields required of new events, but retain their
		// snapshot types. Reject nulls and wrong types without imposing new semantics.
		for _, f := range []string{"refutations", "deferrals", "rebinds"} {
			if v, exists := carried[f]; exists {
				if _, ok := v.([]interface{}); !ok {
					return fmt.Errorf("round carried.%s must be an array", f)
				}
			}
		}
		data, _ := json.Marshal(carried)
		var entries CarriedEntries
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("round carried: %s", termsafe.Escape(err.Error()))
		}
	case RecordRefutation, RecordDeferral:
		finding := m["finding"].(string)
		if !validFindingID(finding) {
			return fmt.Errorf("%s finding must be slot or ccr[i]", kind)
		}
		hash := m["finding_sha256"].(string)
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != 32 || strings.ToLower(hash) != hash {
			return fmt.Errorf("%s finding_sha256 must be a lowercase sha256", kind)
		}
		if kind == RecordRefutation {
			switch m["ground"] {
			case "disproven", "not-load-bearing":
			case "out_of_acceptance_scope":
				if _, err := requireString(m, "ac", kind); err != nil {
					return err
				}
			default:
				return fmt.Errorf("refutation ground is unknown")
			}
			if err := optionalString(m, "ac", kind); err != nil {
				return err
			}
		}
	case RecordRebind:
		if err := validateStringArray(m, "paths", kind); err != nil {
			return err
		}
	}
	return nil
}

// ReadLedger reads and validates every record in a panel directory, in append
// order. A missing ledger is empty; malformed content fails closed.
func ReadLedger(dir string) ([][]byte, error) {
	lines, err := readDispositionLines(filepath.Join(dir, dispositionsFileName))
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if err := Validate(line); err != nil {
			return nil, fmt.Errorf("ledger line %d: %w", i+1, err)
		}
		if err := HygienePredicate(line); err != nil {
			return nil, fmt.Errorf("ledger line %d: %w", i+1, err)
		}
	}
	return lines, nil
}

// AppendRecords appends a batch to an exact panel directory (all layouts). All
// validation, hygiene and conflict checks finish under one dispositions.lock
// hold before any data is appended. Retries ignore created_at in payloads, but
// created_at remains part of round and abandon keys as prescribed by R5.1.
func AppendRecords(dir string, records ...[]byte) error {
	return AppendRecordsWithCheck(dir, nil, records...)
}

// AppendRecordsWithCheck runs check under the same lock as the batch append,
// before reading or writing the ledger. A failing check leaves the ledger intact.
// The callback must not acquire the panel's disposition lock again.
func AppendRecordsWithCheck(dir string, check func() error, records ...[]byte) error {
	if len(records) == 0 {
		return nil
	}
	return withPanelDispositionLock(dir, func() error {
		if check != nil {
			if err := check(); err != nil {
				return err
			}
		}
		return appendRecordsLocked(dir, records...)
	})
}

// WriteSnapshot is the verb-side panel.json writer. Call it only after the
// ledger batch succeeds. A crash before rename leaves the previous snapshot
// intact, and the event keys allow a verb rerun to converge.
func WriteSnapshot(dir string, p Panel) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, dispositionsDirMode); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".panel-*.json")
	if err != nil {
		return err
	}
	// Keep failed temporary files recoverable; they do not register a panel.
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, FileName))
}

// appendRecordsLocked requires the caller to hold the panel ledger lock.
func appendRecordsLocked(dir string, records ...[]byte) error {
	existing, err := ReadLedger(dir)
	if err != nil {
		return err
	}
	known := make(map[string]map[string]interface{}, len(existing)+len(records))
	for _, line := range existing {
		m, err := decodeRecord(line)
		if err != nil {
			return err
		}
		kind, key := ledgerRecordKey(m)
		known[kind+"\x00"+key] = m
	}
	var pending bytes.Buffer
	for _, raw := range records {
		if err := Validate(raw); err != nil {
			return err
		}
		if err := HygienePredicate(raw); err != nil {
			return err
		}
		m, err := decodeRecord(raw)
		if err != nil {
			return err
		}
		if m["panel"] != filepath.Base(filepath.Clean(dir)) {
			return fmt.Errorf("panel: record's panel field does not match the destination panel")
		}
		if recSpec, _ := m["spec"].(string); strings.TrimSpace(recSpec) == "" {
			return fmt.Errorf("panel: record for panel %s has an empty spec field (every record must carry a non-empty spec)", termsafe.Escape(filepath.Base(filepath.Clean(dir))))
		}
		kind, key := ledgerRecordKey(m)
		composite := kind + "\x00" + key
		if old, exists := known[composite]; exists {
			if recordsSemanticallyEqual(m, old) {
				continue
			}
			return fmt.Errorf("panel: record key %s already exists with different content", termsafe.Escape(key))
		}
		known[composite] = m
		// Compact each object: pretty-printed input must still be one JSONL line.
		var line bytes.Buffer
		if err := json.Compact(&line, raw); err != nil {
			return err
		}
		pending.Write(line.Bytes())
		pending.WriteByte('\n')
	}
	if pending.Len() == 0 {
		return nil
	}
	return appendDispositionLine(filepath.Join(dir, dispositionsFileName), pending.Bytes())
}

// submitLockHeldHook runs before Submit reads any mutable state. Tests only.
var submitLockHeldHook func()

// SubmissionRecord binds one version of a slot verdict to exact file bytes.
type SubmissionRecord struct {
	Record       string `json:"record"`
	Spec         string `json:"spec"`
	Panel        string `json:"panel"`
	Slot         string `json:"slot"`
	Round        int    `json:"round"`
	Seq          int    `json:"seq"`
	Status       string `json:"status"`
	Verdict      string `json:"verdict"`
	Family       string `json:"family"`
	Model        string `json:"model"`
	RunnerKind   string `json:"runner_kind"`
	RunnerSource string `json:"runner_source"`
	Via          string `json:"via"`
	Attempt      int    `json:"attempt"`
	Reason       string `json:"reason"`
	FileSHA256   string `json:"file_sha256"`
	CreatedAt    string `json:"created_at"`
}

type LaunchRecord struct {
	Record        string `json:"record"`
	Spec          string `json:"spec"`
	Panel         string `json:"panel"`
	Slot          string `json:"slot"`
	Round         int    `json:"round"`
	Attempt       int    `json:"attempt"`
	Family        string `json:"family"`
	Model         string `json:"model"`
	Status        string `json:"status"`
	Seq           int    `json:"seq,omitempty"`
	ExitCode      *int   `json:"exit_code,omitempty"`
	PGID          int    `json:"pgid,omitempty"`
	RunnerKind    string `json:"runner_kind"`
	RunnerSource  string `json:"runner_source"`
	ArgvSHA256    string `json:"argv_sha256,omitempty"`
	NonceSHA256   string `json:"nonce_sha256,omitempty"`
	StdoutSHA256  string `json:"stdout_sha256,omitempty"`
	StderrSHA256  string `json:"stderr_sha256,omitempty"`
	VerdictSHA256 string `json:"verdict_sha256,omitempty"`
	CreatedAt     string `json:"created_at"`
}
type SubstitutionRecord struct {
	Record     string `json:"record"`
	Spec       string `json:"spec"`
	Panel      string `json:"panel"`
	Slot       string `json:"slot"`
	Round      int    `json:"round"`
	FromFamily string `json:"from_family"`
	ToFamily   string `json:"to_family"`
	Model      string `json:"model"`
	CreatedAt  string `json:"created_at"`
}

func ledgerRecordKey(m map[string]interface{}) (string, string) {
	kind, _ := m["record"].(string)
	var fields []string
	switch kind {
	case RecordVerdict:
		fields = []string{"slot", "round", "seq"}
	case RecordLaunch:
		fields = []string{"slot", "round", "attempt", "status"}
	case RecordSubstitution:
		fields = []string{"slot", "round"}
	default:
		return recordKey(m)
	}
	values := make([]any, len(fields))
	for i, k := range fields {
		values[i] = m[k]
	}
	b, _ := json.Marshal(values)
	return kind, string(b)
}
func validateExecutionRecord(m map[string]interface{}, kind string) error {
	positive := func(key string, zero bool) error {
		if err := requireInt(m, key, kind); err != nil {
			return err
		}
		n, _ := m[key].(json.Number).Int64()
		if n < 0 || (!zero && n == 0) {
			return fmt.Errorf("%s %s is out of range", kind, key)
		}
		return nil
	}
	in := func(key string, choices ...string) error {
		for _, c := range choices {
			if m[key] == c {
				return nil
			}
		}
		return fmt.Errorf("%s %s is invalid", kind, key)
	}
	hash := func(key string, required bool) error {
		v, exists := m[key]
		if !exists && !required {
			return nil
		}
		s, ok := v.(string)
		b, err := hex.DecodeString(s)
		if !ok || err != nil || len(b) != 32 || strings.ToLower(s) != s {
			return fmt.Errorf("%s %s must be a lowercase sha256", kind, key)
		}
		return nil
	}
	if kind == RecordSubstitution {
		for _, field := range []string{"from_family", "to_family"} {
			s, err := requireString(m, field, kind)
			if err != nil {
				return err
			}
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("%s field %s must not be blank", kind, field)
			}
		}
		return positive("round", false)
	}
	if err := in("runner_kind", "process", "harness"); err != nil {
		return err
	}
	if err := in("runner_source", "default", "config.yaml", "config.local.yaml"); err != nil {
		return err
	}
	if kind == RecordVerdict {
		if err := positive("seq", false); err != nil {
			return err
		}
		if err := positive("attempt", true); err != nil {
			return err
		}
		if err := in("status", "draft", "final"); err != nil {
			return err
		}
		if err := in("verdict", "APPROVE", "REQUEST_CHANGES", "REJECT", "VOID"); err != nil {
			return err
		}
		if err := in("via", "direct", "launch", "recover"); err != nil {
			return err
		}
		if err := optionalString(m, "reason", kind); err != nil {
			return err
		}
		return hash("file_sha256", true)
	}
	if err := positive("attempt", false); err != nil {
		return err
	}
	if err := in("status", "started", "dispatched", "submitted", "quota_wall", "launch_failed", "malformed"); err != nil {
		return err
	}
	if m["status"] == "submitted" {
		if err := positive("seq", false); err != nil {
			return err
		}
	}
	if m["status"] == "started" {
		if err := positive("pgid", false); err != nil {
			return err
		}
	}
	for _, k := range []string{"argv_sha256", "nonce_sha256", "stdout_sha256", "stderr_sha256", "verdict_sha256"} {
		if err := hash(k, m["status"] == "started" && (k == "argv_sha256" || k == "nonce_sha256")); err != nil {
			return err
		}
	}
	if _, exists := m["exit_code"]; exists {
		return requireInt(m, "exit_code", kind)
	}
	return nil
}

// LatestSubmissions reads without a lock so a locked submit can reuse it.
func LatestSubmissions(dir string, round int) (map[string]SubmissionRecord, error) {
	lines, err := ReadLedger(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]SubmissionRecord{}
	for _, line := range lines {
		var r SubmissionRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, err
		}
		if r.Record == RecordVerdict && r.Round == round && r.Seq > out[r.Slot].Seq {
			out[r.Slot] = r
		}
	}
	return out, nil
}

// SlotAttempt preserves ledger order, including its position against submissions.
type SlotAttempt struct {
	LaunchRecord
	LedgerIndex           int
	AfterLatestSubmission bool
}

// SlotAttempts returns only this round's launch events. It takes no lock.
func SlotAttempts(dir string, round int) (map[string][]SlotAttempt, error) {
	lines, err := ReadLedger(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]SlotAttempt{}
	latest := map[string]int{}
	seq := map[string]int{}
	for i, line := range lines {
		var r LaunchRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, err
		}
		if r.Round != round {
			continue
		}
		if r.Record == RecordVerdict && r.Seq > seq[r.Slot] {
			latest[r.Slot] = i
			seq[r.Slot] = r.Seq
		}
		if r.Record == RecordLaunch {
			out[r.Slot] = append(out[r.Slot], SlotAttempt{LaunchRecord: r, LedgerIndex: i})
		}
	}
	for slot, events := range out {
		for i := range events {
			at, ok := latest[slot]
			events[i].AfterLatestSubmission = !ok || events[i].LedgerIndex > at
		}
		out[slot] = events
	}
	return out, nil
}
