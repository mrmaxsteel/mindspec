// disposition_store.go: spec 117 Bead 2 — the R6(b) transactional
// append op, the R6(a) terminal coverage-manifest write, and the
// R1(b) completeness floor read from the durable per-panel store.
//
// A per-panel store lives at <spec-dir>/reviews/<panel>/dispositions.jsonl
// (disposition.go's doc comment). This file adds the SINGLE write path
// onto that store — AppendRecord — plus the two callers built on it
// (WriteTerminalManifest, CheckCompleteness). No other code in this
// repo may append to or create a dispositions.jsonl file; the
// `/ms-panel-tally` skill and Bead 4's migration transform both funnel
// through AppendRecord so the gate-before-mutate + idempotency +
// concurrency guarantees below hold for every writer.
//
// Concurrency/atomicity (R6(b), AC7): every AppendRecord call runs, under
// ONE lock held on a DEDICATED <panel-dir>/dispositions.lock file (NEVER
// on dispositions.jsonl itself — see disposition_lock_unix.go /
// disposition_lock_windows.go), as a single indivisible unit: (a)
// Validate + HygienePredicate the incoming record BEFORE touching the
// data file at all; (b) read the CURRENT dispositions.jsonl and check
// the incoming record's stable key (a disposition row's `id`; a coverage
// manifest's `{spec,panel,round}`) against every existing record's key;
// (c) if the key already exists, no-op (idempotent); otherwise append
// the record with a single atomic O_APPEND write. A validation/hygiene
// refusal returns BEFORE step (b)/(c) — the data file is left
// byte-unchanged, and is never created (O_CREATE'd) if it did not
// already exist, because the lock acquisition only ever opens/creates
// the LOCKFILE, not the data file.
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
	dir := panelStoreDir(specDir, panelName)
	if err := os.MkdirAll(dir, dispositionsDirMode); err != nil {
		return fmt.Errorf("panel: create panel directory %s: %w", termsafe.Escape(dir), err)
	}

	lockPath := dispositionsLockPath(specDir, panelName)
	unlock, err := acquireDispositionLock(lockPath)
	if err != nil {
		return fmt.Errorf("panel: acquire disposition lock: %w", err)
	}
	defer unlock()

	return fn()
}

// recordKey identifies the decoded record m's uniqueness key: for a
// disposition row, its `id`; for a coverage manifest, the composite
// `{spec,panel,round}` (round rendered via json.Number.String() to avoid
// a lossy float round-trip). The returned kind is the `record`
// discriminator; two records collide only when BOTH kind and key match
// (R6(b)) — a row and a manifest never collide even if a string field
// happened to coincide.
func recordKey(m map[string]interface{}) (kind, key string) {
	record, _ := m["record"].(string)
	switch record {
	case RecordDisposition:
		id, _ := m["id"].(string)
		return RecordDisposition, id
	case RecordPanelManifest:
		spec, _ := m["spec"].(string)
		panelField, _ := m["panel"].(string)
		round := ""
		if rv, ok := m["round"]; ok {
			if num, ok := rv.(json.Number); ok {
				round = num.String()
			}
		}
		return RecordPanelManifest, spec + "\x00" + panelField + "\x00" + round
	default:
		return record, ""
	}
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

// AppendRecord is the R6(b) canonical, transactional append op: the
// SOLE way any code in this repo may add a line to
// <specDir>/reviews/<panel>/dispositions.jsonl. record is one raw JSON
// line — either a `record:"disposition"` row or a `record:"panel"`
// coverage manifest.
//
// It performs, under ONE lock on that panel's dedicated
// dispositions.lock file, as a single indivisible unit:
//
//  1. Validate(record) then HygienePredicate(record) — a failure of
//     EITHER returns immediately, before the data file is read or
//     written at all (gate-before-mutate, ADR-0041).
//  2. A uniqueness/idempotency check of record's key (a row's `id`; a
//     manifest's `{spec,panel,round}`) against every record currently in
//     dispositions.jsonl. A match is a no-op: AppendRecord returns nil
//     without touching the file.
//  3. Otherwise, one atomic O_APPEND of record onto dispositions.jsonl.
//
// panelName must be non-empty; it is used verbatim as a directory
// segment (CLI callers validate it is a bare slug before reaching here —
// see cmd/mindspec's validatePanelSlug).
func AppendRecord(specDir, panelName string, record []byte) error {
	if strings.TrimSpace(panelName) == "" {
		return fmt.Errorf("panel: panel name must not be empty")
	}

	return withDispositionLock(specDir, panelName, func() error {
		if err := Validate(record); err != nil {
			return err
		}
		if err := HygienePredicate(record); err != nil {
			return err
		}

		newMap, err := decodeRecord(record)
		if err != nil {
			// Validate already parsed record successfully, so this path
			// is unreachable in practice; handled defensively rather than
			// panicking.
			return err
		}

		// Record↔destination consistency (FR-2, gate-before-mutate): the
		// record's OWN `panel` field must name the destination panel — a
		// row/manifest for panel X must never land in panel Y's file,
		// which would silently corrupt every floor/Q attribution that
		// reads the record's own fields rather than its on-disk location.
		// Validate (above) already proved `panel` is a non-empty string;
		// `spec` likewise. We additionally require a non-empty `spec` here
		// explicitly so the invariant is self-documenting at the write
		// boundary. (The bare `spec` field intentionally differs from the
		// <NNN-slug> spec DIRECTORY — the migration and its tests write to
		// arbitrary target dirs — so specDir is not a reliable carrier of
		// the expected bare spec, and the destination check is keyed on
		// the panel segment, which specDir DOES encode verbatim.)
		recPanel, _ := newMap["panel"].(string)
		if recPanel != panelName {
			return fmt.Errorf("panel: record's panel field %s does not match the destination panel %s (a record must be appended to its own panel's store)", termsafe.Escape(recPanel), termsafe.Escape(panelName))
		}
		if recSpec, _ := newMap["spec"].(string); strings.TrimSpace(recSpec) == "" {
			return fmt.Errorf("panel: record for panel %s has an empty spec field (every record must carry a non-empty spec)", termsafe.Escape(panelName))
		}

		newKind, newKey := recordKey(newMap)

		path := dispositionsPath(specDir, panelName)
		existing, err := readDispositionLines(path)
		if err != nil {
			return err
		}
		for _, line := range existing {
			m, decErr := decodeRecord(line)
			if decErr != nil {
				// A pre-existing line that no longer decodes is not this
				// op's job to repair; it simply cannot collide with the
				// incoming record's key.
				continue
			}
			kind, key := recordKey(m)
			if kind == newKind && key == newKey {
				// Same (kind,key): a TRUE retry (byte-identical semantic
				// content) is an idempotent no-op (R6). But an append-only
				// store must never silently DROP changed content under an
				// existing key — that is a conflict, not a retry — so
				// compare the full payload (excluding the volatile
				// created_at timestamp) and surface a mismatch (FR-4).
				if recordsSemanticallyEqual(newMap, m) {
					return nil // idempotent no-op: this exact content is already stored.
				}
				return fmt.Errorf("panel: a record with this id/key already exists with different content — the append-only store treats changed content under an existing key as a conflict, not a retry (key %s)", termsafe.Escape(newKey))
			}
		}

		return appendDispositionLine(path, record)
	})
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
