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
	RecordRound      = "round"
	RecordRefutation = "refutation"
	RecordDeferral   = "deferral"
	RecordRebind     = "rebind"
	RecordAbandon    = "abandon"
)

// IsVerbRecord identifies the five event kinds reserved to panel verbs (R5.1).
func IsVerbRecord(kind string) bool {
	switch kind {
	case RecordRound, RecordRefutation, RecordDeferral, RecordRebind, RecordAbandon:
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
	case RecordRound:
		fields = append(fields, "reviewed_head_sha", "subject_digest", "target", "gate", "telemetry")
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
	if kind != RecordAbandon {
		if err := requireInt(m, "round", kind); err != nil {
			return err
		}
		n, _ := m["round"].(json.Number).Int64()
		if n < 1 {
			return fmt.Errorf("%s round must be positive", kind)
		}
	}
	switch kind {
	case RecordRound:
		if !isValidGate(m["gate"].(string)) {
			return fmt.Errorf("round gate must be a canonical gate")
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
		paths, err := requireStringArray(m, "paths", kind)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return fmt.Errorf("rebind paths must not be empty")
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
			kind, key := recordKey(m)
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
			kind, key := recordKey(m)
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
