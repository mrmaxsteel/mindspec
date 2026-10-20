// disposition_store.go implements the transactional panel ledger store.
// AppendRecord retains the spec-directory adapter for legacy callers;
// AppendRecords accepts an exact panel directory and validates an entire batch
// under one dispositions.lock hold before appending any data (Spec 130 R5.1).
package panel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const (
	// dispositionsFileName is the per-panel store's basename.
	dispositionsFileName = "dispositions.jsonl"
	// dispositionsLockName is the DEDICATED lockfile basename that
	// serializes AppendRecord across processes — never dispositions.jsonl
	// itself, so a rename/rewrite of the data file can never invalidate a
	// held lock descriptor (mirrors internal/journal/lock.go's
	// reports.lock idiom).
	dispositionsLockName = "dispositions.lock"
	// dispositionsDirMode is the panel directory's owner-only mode,
	// created before the lockfile so the lockfile itself can be created.
	dispositionsDirMode os.FileMode = 0o700
	// dispositionsFileMode is the owner-only mode every store file
	// (data file and lockfile alike) is created with.
	dispositionsFileMode os.FileMode = 0o600
)

// panelStoreDir returns <specDir>/reviews/<panel>, the directory holding
// one panel's dispositions.jsonl + dispositions.lock.
func panelStoreDir(specDir, panelName string) string {
	return filepath.Join(specDir, "reviews", panelName)
}

// dispositionsPath returns <specDir>/reviews/<panel>/dispositions.jsonl.
func dispositionsPath(specDir, panelName string) string {
	return filepath.Join(panelStoreDir(specDir, panelName), dispositionsFileName)
}

// dispositionsLockPath returns <specDir>/reviews/<panel>/dispositions.lock
// — the dedicated lockfile AppendRecord serializes on.
func dispositionsLockPath(specDir, panelName string) string {
	return filepath.Join(panelStoreDir(specDir, panelName), dispositionsLockName)
}

// withDispositionLock creates the panel directory (0700) if needed,
// acquires the exclusive cross-process lock on that panel's DEDICATED
// dispositions.lock file (acquireDispositionLock — build-tagged unix
// flock / windows O_EXCL-retry), runs fn while holding it, and releases
// it afterward. The lockfile is opened/created here; the DATA file
// (dispositions.jsonl) is never touched by this function — only fn may
// open it — so lock acquisition itself never violates gate-before-mutate.
func withDispositionLock(specDir, panelName string, fn func() error) error {
	return withPanelDispositionLock(panelStoreDir(specDir, panelName), fn)
}

func withPanelDispositionLock(dir string, fn func() error) error {
	if err := os.MkdirAll(dir, dispositionsDirMode); err != nil {
		return fmt.Errorf("panel: create panel directory %s: %w", termsafe.Escape(dir), err)
	}

	lockPath := filepath.Join(dir, dispositionsLockName)
	unlock, err := acquireDispositionLock(lockPath)
	if err != nil {
		return fmt.Errorf("panel: acquire disposition lock: %w", err)
	}
	defer unlock()

	return fn()
}

// recordKey returns the kind and a JSON tuple of that kind's R5.1 key
// components. Keys are scoped to a panel directory. JSON tuples preserve number
// literals and cannot collide through delimiters embedded in string fields.
func recordKey(m map[string]interface{}) (kind, key string) {
	kind, _ = m["record"].(string)
	var fields []string
	switch kind {
	case RecordDisposition:
		fields = []string{"id"}
	case RecordPanelManifest:
		fields = []string{"spec", "panel", "round", "created_at"}
	case RecordRound:
		fields = []string{"round", "created_at"}
	case RecordRefutation, RecordDeferral:
		fields = []string{"slot", "round", "finding", "finding_sha256"}
	case RecordRebind:
		fields = []string{"to_sha"}
	case RecordAbandon:
		fields = []string{"created_at"}
	default:
		return kind, ""
	}
	values := make([]interface{}, len(fields))
	for i, f := range fields {
		values[i] = m[f]
	}
	data, _ := json.Marshal(values)
	return kind, string(data)
}

// readDispositionLines reads path and returns its non-blank lines
// (trimmed of surrounding whitespace), or (nil, nil) if the file does
// not exist yet — an absent store is simply an empty one, not an error.
func readDispositionLines(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("panel: read %s: %w", termsafe.Escape(path), err)
	}
	var lines [][]byte
	for _, raw := range bytes.Split(data, []byte("\n")) {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 {
			continue
		}
		// Copy out of the backing array: bytes.Split's slices alias data,
		// and appendDispositionLine below mutates its own buffer only,
		// but callers (recordKey via decodeRecord) should never hold a
		// reference into a buffer we are about to discard.
		line := make([]byte, len(trimmed))
		copy(line, trimmed)
		lines = append(lines, line)
	}
	return lines, nil
}

// appendDispositionLine appends one already-validated JSON line to path
// as a single O_APPEND write(2) — position-atomic, so two concurrent
// processes appending sub-PIPE_BUF lines never interleave (mirrors
// internal/journal's appendLine). The panel directory is created (0700)
// defensively (withDispositionLock already created it under the lock,
// but this function has no other caller path that skips that).
func appendDispositionLine(path string, record []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dispositionsDirMode); err != nil {
		return fmt.Errorf("panel: create panel directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, dispositionsFileMode)
	if err != nil {
		return fmt.Errorf("panel: open %s: %w", termsafe.Escape(path), err)
	}
	// Belt-and-suspenders: O_CREATE only applies mode on creation, so a
	// pre-existing file under a permissive umask is forced back to 0600.
	if err := f.Chmod(dispositionsFileMode); err != nil {
		_ = f.Close()
		return fmt.Errorf("panel: chmod %s: %w", termsafe.Escape(path), err)
	}
	trimmed := bytes.TrimRight(record, "\n")
	line := make([]byte, 0, len(trimmed)+1)
	line = append(line, trimmed...)
	line = append(line, '\n')
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("panel: append %s: %w", termsafe.Escape(path), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("panel: close %s: %w", termsafe.Escape(path), err)
	}
	return nil
}

// AppendRecord adapts a legacy spec-directory/panel-name destination to the
// exact-directory batch API. The same key and conflict rules apply to both.
func AppendRecord(specDir, panelName string, record []byte) error {
	if strings.TrimSpace(panelName) == "" {
		return fmt.Errorf("panel: panel name must not be empty")
	}
	return AppendRecords(panelStoreDir(specDir, panelName), record)
}

// recordsSemanticallyEqual reports whether two already-decoded records
// carry the same payload IGNORING the volatile `created_at` timestamp
// (FR-4). Two writers minting the same content-derived id/key at
// different wall-clock instants (a genuine retry) differ ONLY in
// created_at, so excluding it lets a true retry no-op while any OTHER
// field difference surfaces as a conflict.
func recordsSemanticallyEqual(a, b map[string]interface{}) bool {
	return reflect.DeepEqual(withoutVolatileFields(a), withoutVolatileFields(b))
}

// withoutVolatileFields returns a shallow copy of m with the volatile
// `created_at` field removed. Nested values are shared, not cloned —
// safe because the result is only ever read by reflect.DeepEqual.
func withoutVolatileFields(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if k == "created_at" {
			continue
		}
		out[k] = v
	}
	return out
}

// CanonicalizeDispositionRowID rewrites a `record:"disposition"` row's
// `id` to the canonical content-derived DispositionRowID(spec, panel,
// reviewer, summary), OVERRIDING any operator-supplied id, and returns
// the re-marshaled line (FR-3). The live `panel disposition append` CLI
// leaf calls this before AppendRecord so live capture enforces R2's
// stable-content-id + R6 retry-idempotency automatically — the operator
// never computes the hash, and a wrong or absent id is corrected here.
//
// It is a no-op passthrough for a coverage manifest (keyed on
// {spec,panel,round}, not id) or for any input it cannot decode as a
// disposition record — those cases are left to AppendRecord's own
// Validate gate to accept or reject, so this helper never becomes a
// second, divergent validation authority.
func CanonicalizeDispositionRowID(record []byte) []byte {
	m, err := decodeRecord(record)
	if err != nil {
		return record
	}
	if r, _ := m["record"].(string); r != RecordDisposition {
		return record
	}
	spec, _ := m["spec"].(string)
	panelField, _ := m["panel"].(string)
	reviewer, _ := m["reviewer"].(string)
	summary, _ := m["summary"].(string)
	m["id"] = DispositionRowID(spec, panelField, reviewer, summary)

	out, err := json.Marshal(m)
	if err != nil {
		return record
	}
	return out
}

// WriteTerminalManifest is a thin wrapper over AppendRecord (R6(a)):
// EVERY terminal panel calls this exactly once, including a
// finding-less all-APPROVE panel (zero disposition rows written, one
// manifest line still written). manifest.Record is forced to
// RecordPanelManifest so a caller can never accidentally submit a
// mistagged record through this entry point.
func WriteTerminalManifest(specDir, panelName string, manifest CoverageManifest) error {
	manifest.Record = RecordPanelManifest
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("panel: marshal coverage manifest: %w", err)
	}
	return AppendRecord(specDir, panelName, data)
}

// decodeDispositionRow strictly unmarshals one already-validated JSONL
// line into a DispositionRow (used by CheckCompleteness, which only
// ever reads records this package itself wrote via AppendRecord).
func decodeDispositionRow(line []byte) (DispositionRow, error) {
	var row DispositionRow
	if err := json.Unmarshal(line, &row); err != nil {
		return DispositionRow{}, fmt.Errorf("panel: decoding disposition row: %s", termsafe.Escape(err.Error()))
	}
	return row, nil
}

// decodeCoverageManifestLine strictly unmarshals one already-validated
// JSONL line into a CoverageManifest (used by CheckCompleteness).
func decodeCoverageManifestLine(line []byte) (CoverageManifest, error) {
	var manifest CoverageManifest
	if err := json.Unmarshal(line, &manifest); err != nil {
		return CoverageManifest{}, fmt.Errorf("panel: decoding coverage manifest: %s", termsafe.Escape(err.Error()))
	}
	return manifest, nil
}

// isTerminalRC reports whether verdict is one of the two terminal
// verdicts the R1(b) floor requires coverage for.
func isTerminalRC(verdict string) bool {
	return verdict == "REQUEST_CHANGES" || verdict == "REJECT"
}

// CheckCompleteness implements the R1(b) mechanical completeness floor
// for exactly one panel: it reads ONLY that panel's dispositions.jsonl
// (its coverage manifest plus its disposition rows) — never a raw
// verdict file, which is non-durable (R1(b), OQ4/OQ5) — and requires
// that every manifest slot whose terminal verdict is REQUEST_CHANGES or
// REJECT is named (as `reviewer` or in `convergent_with[]`) by at least
// one disposition row. It returns an error naming panelName and the
// first uncovered slot token on violation, or nil if the floor holds
// (including the vacuous case of a manifest with zero RC/REJECT slots).
func CheckCompleteness(specDir, panelName string) error {
	path := dispositionsPath(specDir, panelName)
	lines, err := readDispositionLines(path)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return fmt.Errorf("panel %s: no dispositions.jsonl found (no terminal coverage manifest recorded)", termsafe.Escape(panelName))
	}

	var manifest *CoverageManifest
	covered := map[string]bool{}

	for _, line := range lines {
		if err := Validate(line); err != nil {
			return fmt.Errorf("panel %s: malformed record: %w", termsafe.Escape(panelName), err)
		}
		if err := HygienePredicate(line); err != nil {
			return err
		}
		m, decErr := decodeRecord(line)
		if decErr != nil {
			return fmt.Errorf("panel %s: dispositions.jsonl contains a malformed record: %s", termsafe.Escape(panelName), decErr.Error())
		}
		record, _ := m["record"].(string)
		switch record {
		case RecordPanelManifest:
			cm, cmErr := decodeCoverageManifestLine(line)
			if cmErr != nil {
				return fmt.Errorf("panel %s: %s", termsafe.Escape(panelName), cmErr.Error())
			}
			manifest = &cm
		case RecordRound, RecordRefutation, RecordDeferral, RecordRebind, RecordAbandon:
			// Events are validated above; only disposition rows cover slots.
			continue
		case RecordDisposition:
			row, rowErr := decodeDispositionRow(line)
			if rowErr != nil {
				return fmt.Errorf("panel %s: %s", termsafe.Escape(panelName), rowErr.Error())
			}
			covered[row.Reviewer] = true
			for _, c := range row.ConvergentWith {
				covered[c] = true
			}
		}
	}

	if manifest == nil {
		return fmt.Errorf("panel %s: dispositions.jsonl has no record:\"panel\" coverage manifest", termsafe.Escape(panelName))
	}

	for _, slot := range manifest.Slots {
		if isTerminalRC(slot.Verdict) && !covered[slot.Slot] {
			return fmt.Errorf("panel %s: completeness floor violated — slot %s (terminal verdict %s) has no covering disposition row (reviewer or convergent_with)", termsafe.Escape(panelName), termsafe.Escape(slot.Slot), termsafe.Escape(slot.Verdict))
		}
	}
	return nil
}
