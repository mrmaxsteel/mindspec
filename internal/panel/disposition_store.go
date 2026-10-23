// disposition_store.go implements the transactional panel ledger store.
// AppendRecord retains the spec-directory adapter for legacy callers;
// AppendRecords accepts an exact panel directory and validates an entire batch
// under one dispositions.lock hold before appending any data (Spec 130 R5.1).
package panel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

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
// {spec,panel,round,created_at}, not id) or for any input it cannot decode as a
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

// WriteTerminalManifest retains the legacy spec-directory adapter. New tally
// callers use WriteCoverageManifest, which appends only changed derivations.
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
	return CheckCompletenessAt(panelStoreDir(specDir, panelName), panelName)
}

// CheckCompletenessAt checks a panel in any supported directory layout.
func CheckCompletenessAt(dir, panelName string) error {
	path := filepath.Join(dir, dispositionsFileName)
	lines, err := readDispositionLines(path)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return fmt.Errorf("panel %s: no dispositions.jsonl found (no terminal coverage manifest recorded)%s", termsafe.Escape(panelName), tallyFirstHint(panelName))
	}

	var manifest *CoverageManifest
	var rows []DispositionRow
	var boundDeferrals []Deferral
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
		case RecordDeferral:
			var event DeferralRecord
			if err := json.Unmarshal(line, &event); err != nil {
				return err
			}
			if event.SourceVerdict != "" || event.SourceFileSHA256 != "" {
				boundDeferrals = append(boundDeferrals, event.Deferral)
			}
		case RecordRound, RecordRefutation, RecordRebind, RecordAbandon:
			// Events are validated above; only disposition rows cover slots.
			continue
		case RecordDisposition:
			row, rowErr := decodeDispositionRow(line)
			if rowErr != nil {
				return fmt.Errorf("panel %s: %s", termsafe.Escape(panelName), rowErr.Error())
			}
			rows = append(rows, row)
		}
	}

	if manifest == nil {
		return fmt.Errorf("panel %s: dispositions.jsonl has no record:\"panel\" coverage manifest%s", termsafe.Escape(panelName), tallyFirstHint(panelName))
	}

	// A deferral row covers only the submission its event discharged. Minor
	// APPROVE notes remain audit records, including in legacy ledgers.
	var effective []Deferral
	if len(boundDeferrals) > 0 {
		res, err := Tally(dir)
		if err != nil {
			return err
		}
		effective = res.EffectiveDeferrals()
	}
	for _, row := range rows {
		if row.Disposition == "confirmed-deferred" {
			if row.Severity == "minor" {
				continue
			}
			superseded := false
			for _, d := range boundDeferrals {
				if d.Slot != row.Reviewer || d.CreatedAt != row.CreatedAt || d.Bead != row.EvidenceRef || row.Round == nil || d.Round != *row.Round {
					continue
				}
				current := false
				for _, live := range effective {
					if live == d {
						current = true
						break
					}
				}
				if !current {
					superseded = true
				}
			}
			if superseded {
				continue
			}
		}
		covered[row.Reviewer] = true
		for _, c := range row.ConvergentWith {
			covered[c] = true
		}
	}

	for _, slot := range manifest.Slots {
		if isTerminalRC(slot.Verdict) && !covered[slot.Slot] {
			return &UncoveredSlotError{Panel: panelName, Slot: slot.Slot, Verdict: slot.Verdict}
		}
	}
	return nil
}

// DeriveCoverageManifest derives coverage from roster slots and latest verdicts.
// Roster order and roster model identity are authoritative; unexpected files
// never contribute slots. Legacy registrations do not require coverage.
func DeriveCoverageManifest(res *Result) CoverageManifest {
	p := res.Panel
	m := CoverageManifest{Record: RecordPanelManifest, Spec: p.Spec, Gate: p.Gate,
		Panel: filepath.Base(res.Dir), Round: res.LatestRound, Slots: []ManifestSlot{}}
	if m.Spec == "" {
		m.Spec = "adhoc"
	}
	if m.Gate == "" {
		m.Gate = "adhoc"
	}
	bySlot := make(map[string]Verdict)
	for _, v := range res.Verdicts {
		bySlot[v.Slot] = v
	}
	for _, slot := range p.Roster {
		if v, ok := bySlot[slot.Slot]; ok {
			m.Slots = append(m.Slots, ManifestSlot{Slot: slot.Slot, Model: slot.Model, Verdict: v.Verdict})
		}
	}
	return m
}

// WriteCoverageManifest is tally's append-only manifest writer. Freshness is
// deliberately absent: coverage can be recorded while a non-bead target is stale.
func WriteCoverageManifest(res *Result) error {
	if res == nil || res.Panel == nil || res.Panel.Abandoned || !res.PassRule(true) {
		return nil
	}
	legacy, err := LegacyPanel(res.Dir)
	if err != nil {
		return err
	}
	if legacy {
		return nil
	}

	derived := DeriveCoverageManifest(res)
	stamped := derived
	stamped.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(stamped)
	if err != nil {
		return err
	}
	unchanged := errors.New("coverage unchanged")
	err = AppendRecordsWithCheck(res.Dir, func() error {
		_, manifests, err := loadDispositionFile(filepath.Join(res.Dir, dispositionsFileName))
		if err != nil {
			return err
		}
		for i := len(manifests) - 1; i >= 0; i-- {
			if manifests[i].Round != derived.Round {
				continue
			}
			previous := manifests[i]
			previous.CreatedAt = ""
			if reflect.DeepEqual(previous, derived) {
				return unchanged
			}
			break
		}
		return nil
	}, data)
	if errors.Is(err, unchanged) {
		return nil
	}
	return err

}

// tallyFirstHint tells the caller how to write the missing coverage
// manifest: only panel tally writes it, so a check before the first tally
// has none.
func tallyFirstHint(panelName string) string {
	return fmt.Sprintf("; run mindspec panel tally %s first (only tally writes the coverage manifest)", termsafe.Escape(panelName))
}

// CheckCore3 checks required coverage, without a gate decision or git reads.
// Callers decide which registrations to skip before invoking it.
func CheckCore3(reg Registration) error {
	res, err := Tally(reg.Dir)
	if err != nil {
		return err
	}
	if res.PanelErr != nil {
		return res.PanelErr
	}
	if res.Panel == nil {
		return fmt.Errorf("panel registration missing")
	}
	_, manifests, err := loadDispositionFile(filepath.Join(reg.Dir, dispositionsFileName))
	if err != nil {
		return err
	}
	if len(manifests) == 0 {
		return fmt.Errorf("no coverage manifest recorded%s", tallyFirstHint(reg.Slug()))
	}
	last := manifests[len(manifests)-1]
	derived := DeriveCoverageManifest(res)
	if last.Round != res.LatestRound || last.Round != res.Panel.Round {
		return fmt.Errorf("coverage manifest round %d differs from current round %d", last.Round, res.Panel.Round)
	}
	if !res.Complete() || res.RoundMismatch {
		return fmt.Errorf("coverage verdict files are incomplete or from another round")
	}
	if last.Spec != derived.Spec || last.Panel != derived.Panel || last.Gate != derived.Gate || !reflect.DeepEqual(last.Slots, derived.Slots) {
		return fmt.Errorf("coverage manifest differs from the roster and verdict files (slots %s)", coverageSlotNames(derived))
	}
	return CheckCompletenessAt(reg.Dir, reg.Slug())
}

func coverageSlotNames(m CoverageManifest) string {
	slots := make([]string, 0, len(m.Slots))
	for _, s := range m.Slots {
		slots = append(slots, termsafe.Escape(s.Slot))
	}
	return strings.Join(slots, ", ")
}

// UncoveredSlotError names the slot whose durable disposition coverage is absent.
// Lifecycle callers use its slot to construct a concrete discharge recovery.
type UncoveredSlotError struct{ Panel, Slot, Verdict string }

func (e *UncoveredSlotError) Error() string {
	return fmt.Sprintf("panel %s: completeness floor violated — slot %s (terminal verdict %s) has no covering disposition row (reviewer or convergent_with)", termsafe.Escape(e.Panel), termsafe.Escape(e.Slot), termsafe.Escape(e.Verdict))
}

// DispositionRowID derives a disposition row's canonical DispositionRow.ID:
// a stable content hash of {spec, panel, reviewer, summary} (R2's id
// contract — a row carries no round, so round is deliberately excluded
// from the key). The same four inputs always mint the same id, so both
// the Bead-4 migration replay AND the live `panel disposition append`
// leaf are retry-idempotent through AppendRecord's own id-keyed dedup
// (R6). This is the SINGLE derivation shared by both writers — the CLI
// append leaf overrides any operator-supplied id with this value so live
// capture enforces the same stable-content-id contract the migration does.
func DispositionRowID(spec, panelName, reviewer, summary string) string {
	h := sha256.Sum256([]byte(spec + "\x00" + panelName + "\x00" + reviewer + "\x00" + summary))
	return "d-" + hex.EncodeToString(h[:])[:16]
}
