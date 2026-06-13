// Package journal is the always-on, isolated, redacted friction sink for
// the self-improvement loop (spec 094, Bead 2 — Req 2 / Req 3 / Req 8;
// HC-1 / HC-3 / HC-6 / HC-7 / HC-8).
//
// The success-path self-emit in cmd/mindspec/root.go's PersistentPostRunE
// calls AppendSuccessEvent on a friction event (an escape-hatch override
// flag that was set on a command that SUCCEEDED, or a completed
// `repair phase`). Every field is scrubbed at WRITE time through
// internal/redact (HC-1) and the write is fail-closed (HC-7): a field the
// redactor cannot confidently classify drops the WHOLE entry — the raw
// value is never persisted, logged, or emitted as a fallback.
//
// # Store isolation (HC-3 / HC-8)
//
// The journal lives at <state-dir>/journal.jsonl where <state-dir> is the
// machine-global, NON-SYNCED config.GlobalConfigDir() (os.UserConfigDir()/
// mindspec, XDG-honoring on Linux) — NEVER under any project/git/bd/dolt
// tree, so a redaction MISS can never egress to the shared remote. Files
// are created 0600 (owner-only).
//
// # Storm cap (Req 8)
//
// Entries collapse by the normalized identity + fingerprint into ONE
// record carrying an occurrence count, capped at JournalStormCapL so a
// runaway loop cannot bloat the journal: firing the same fingerprint
// M < L times yields one entry with count == M; firing L+1 times yields
// one entry whose count stops at L (count == L, not L+1).
//
// # Best-effort (plan §API Contract)
//
// AppendSuccessEvent returns an error so callers can decide; the
// PersistentPostRunE caller treats it as BEST-EFFORT / NON-FATAL — a
// journal error never converts an already-successful, side-effecting
// command into a post-mutation failure.
package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/redact"
)

// JournalStormCapL is the per-fingerprint occurrence cap (Req 8). Once a
// fingerprint's collapsed entry reaches this count it stops growing, so a
// friction storm (a runaway loop firing the same event) cannot bloat the
// journal. It is an exported named constant a deterministic test asserts
// against (firing L+1 times → count == L).
const JournalStormCapL = 50

// journalSchemaVersion is the on-disk record schema version (the "v"
// field of the §Storage Contract journal record).
const journalSchemaVersion = 1

// fileMode is the owner-only mode every store file is created with (HC-8).
const fileMode os.FileMode = 0o600

// StateDirEnv lets an operator (and the test suite) pin the journal's
// state directory explicitly, bypassing the config.GlobalConfigDir()
// XDG/HOME resolution. It is still REQUIRED to be a non-synced location;
// the caller owns that contract. Primarily a deterministic test seam.
const StateDirEnv = "MINDSPEC_STATE_DIR"

// journalFileName is the fixed journal filename under the state dir.
const journalFileName = "journal.jsonl"

// mu serializes in-process read-modify-write of the journal file. The
// store is a SINGLE appended file collapsed by fingerprint, so the
// counted-collapse is not a pure O_APPEND; this mutex guards the
// read-modify-rewrite within one process. Cross-process concurrency is
// bounded by the §Storage Contract: a lost-count race is at worst an
// undercount (consolidation re-collapses by identity+fingerprint), never
// corruption — each rewrite writes a complete, well-formed file.
var mu sync.Mutex

// Identity mirrors redact.Identity (the normalized closed-set event
// tuple). Re-exported on the journal record so a hash collision cannot
// silently alias two distinct events (DQ5) — both the tuple and the hash
// are persisted.
type Identity struct {
	Command     string `json:"command"`
	EscapeHatch string `json:"escape_hatch"`
	Subcommand  string `json:"subcommand"`
}

// Event is the enum-first input to AppendSuccessEvent — mindspec's own
// closed-set tokens plus the raw argv0 (reduced to basename + scrubbed by
// redact). It NEVER carries a flag VALUE, an override reason, or any
// user-supplied free text (M4): the caller populates ONLY structured enum
// fields.
type Event struct {
	Argv0       string
	Command     string
	EscapeHatch string
	Subcommand  string
	Version     string
	OS          string
}

// Record is the §Storage Contract journal record (enum-only — no free
// text). It is what is serialized, one per line, to journal.jsonl.
type Record struct {
	V           int      `json:"v"`
	Argv0       string   `json:"argv0"`
	Command     string   `json:"command"`
	EscapeHatch string   `json:"escape_hatch"`
	Subcommand  string   `json:"subcommand"`
	Fingerprint string   `json:"fingerprint"`
	Identity    Identity `json:"identity"`
	Count       int      `json:"count"`
	Version     string   `json:"version"`
}

// Dir resolves the dedicated, non-synced state directory for the friction
// store. The MINDSPEC_STATE_DIR env override wins (a deterministic test /
// explicit-operator seam); otherwise it is config.GlobalConfigDir()
// (os.UserConfigDir()/mindspec — XDG-honoring, git-tree-guarded, NEVER
// under a project/bd/dolt tree per HC-3). The directory is NOT created
// here; AppendSuccessEvent creates it lazily 0700.
func Dir() (string, error) {
	if d := os.Getenv(StateDirEnv); d != "" {
		return d, nil
	}
	return config.GlobalConfigDir()
}

// Path returns the absolute path to journal.jsonl under Dir().
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, journalFileName), nil
}

// AppendSuccessEvent scrubs ev through internal/redact at WRITE time,
// collapses by the normalized identity + fingerprint into one counted
// entry (capped at JournalStormCapL), stamps the bare version, and
// persists it 0600 to the isolated journal.
//
// FAIL-CLOSED (HC-7): if RedactEvent drops the event (a field that cannot
// be confidently classified), NOTHING is written and a nil error is
// returned — the drop is a privacy SUCCESS, not a journaling failure, so
// the best-effort caller is not nudged to retry with raw data. A
// genuine I/O error (open/read/write) IS returned so the best-effort
// caller can swallow it explicitly.
//
// The redaction governs DATA EMISSION (drop the datum); it never affects
// the caller's command exit (the caller treats any returned error as
// non-fatal).
func AppendSuccessEvent(ev Event) error {
	redacted, ok := redact.RedactEvent(redact.Event{
		Argv0:       ev.Argv0,
		Command:     ev.Command,
		EscapeHatch: ev.EscapeHatch,
		Subcommand:  ev.Subcommand,
		Version:     ev.Version,
		OS:          ev.OS,
	})
	if !ok {
		// Fail-closed DROP: never persist a raw or partial value.
		return nil
	}

	path, err := Path()
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("journal: create state dir: %w", err)
	}

	records, err := readRecords(path)
	if err != nil {
		return err
	}

	// Collapse by identity + fingerprint into one counted entry, capped.
	id := Identity{
		Command:     redacted.Identity.Command,
		EscapeHatch: redacted.Identity.EscapeHatch,
		Subcommand:  redacted.Identity.Subcommand,
	}
	idx := -1
	for i := range records {
		if records[i].Fingerprint == redacted.Fingerprint && records[i].Identity == id {
			idx = i
			break
		}
	}
	if idx >= 0 {
		// Storm cap: stop counting past L (count == L, never L+1).
		if records[idx].Count < JournalStormCapL {
			records[idx].Count++
		}
		// Refresh the version stamp to the most recent observation so a
		// later run on a newer build is reflected (first/last derivation is
		// Bead 3's job over the journal; here we keep the latest).
		records[idx].Version = redacted.Version
		records[idx].Argv0 = redacted.Argv0
	} else {
		records = append(records, Record{
			V:           journalSchemaVersion,
			Argv0:       redacted.Argv0,
			Command:     redacted.Command,
			EscapeHatch: redacted.EscapeHatch,
			Subcommand:  redacted.Subcommand,
			Fingerprint: redacted.Fingerprint,
			Identity:    id,
			Count:       1,
			Version:     redacted.Version,
		})
	}

	return writeRecords(path, records)
}

// ListReports reads and returns the collapsed journal records. It is the
// read surface Bead 3's `report list` consolidates over. (Bead 3 layers
// the reports.jsonl store + regression/stale on top; in Bead 2 this
// simply surfaces the journal contents for the read API the §API Contract
// pins.) Returns an empty slice when the journal does not exist yet.
func ListReports() ([]Record, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	return readRecords(path)
}

// readRecords parses every JSONL line of path into a Record slice. A
// missing file is the empty-journal case (no error). Malformed lines are
// SKIPPED rather than aborting the whole read — the journal tolerates
// interleaved/partial lines from a cross-process append race (§Storage
// Contract), so one bad line must not lose the rest. Caller holds mu.
func readRecords(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("journal: read %s: %w", path, err)
	}
	var records []Record
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			// Skip an unparseable line (partial cross-process write); do not
			// abort — losing one line is an undercount, never corruption.
			continue
		}
		records = append(records, r)
	}
	return records, nil
}

// writeRecords serializes records as JSONL and atomically replaces path
// via a temp file + rename, so a concurrent reader never sees a partial
// file and a crash mid-write cannot truncate the journal. The temp file
// is created 0600 (HC-8); the rename preserves it. Caller holds mu.
func writeRecords(path string, records []Record) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".journal-*.tmp")
	if err != nil {
		return fmt.Errorf("journal: create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we bail before the rename.
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("journal: chmod temp: %w", err)
	}

	enc := json.NewEncoder(tmp)
	for i := range records {
		if err := enc.Encode(&records[i]); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("journal: encode record: %w", err)
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("journal: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("journal: rename temp: %w", err)
	}
	// The rename target inherits the temp file's 0600; re-assert defensively
	// in case path pre-existed with a looser mode under a permissive umask.
	if err := os.Chmod(path, fileMode); err != nil {
		return fmt.Errorf("journal: chmod %s: %w", path, err)
	}
	return nil
}

// splitLines splits raw NDJSON bytes on '\n' without allocating an
// intermediate string per line. A trailing newline yields no extra empty
// entry beyond the natural last split (handled by the len==0 skip in
// readRecords).
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
