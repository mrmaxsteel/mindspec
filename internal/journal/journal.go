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
// tree, so a redaction MISS can never egress to the shared remote. The
// MINDSPEC_STATE_DIR override is subjected to the SAME enclosing-git-tree
// guard (HC-3): an override that resolves inside a git work tree / project
// tree is REJECTED and the write FAILS CLOSED (nothing on disk). Files are
// created 0600 (owner-only).
//
// # Append-only design (§Storage Contract)
//
// journal.jsonl is APPEND-ONLY: one REDACTED event per line, written in a
// SINGLE O_APPEND write(2) of a sub-PIPE_BUF JSONL record. There is NO
// in-file collapse, NO count field, and NO read-modify-rewrite — so two
// concurrent mindspec processes can never lost-update each other (each
// append is POSIX-atomic for a short line), and every line PRESERVES its
// own version stamp. The COLLAPSE (count, first/last version,
// resolved_in_version) is Bead 3's job over reports.jsonl, derived from
// these per-event journal lines; ListReports/ReadEvents surface the raw
// event Records for that consolidation.
//
// # Storm cap (Req 8 — per session)
//
// The cap is PER-FINGERPRINT-PER-SESSION (spec Req 8 / §Storage Contract):
// within ONE process invocation, at most JournalStormCapL lines are
// appended for a given fingerprint; appends beyond the cap are dropped.
// This bounds a runaway in-process loop without any cross-process state or
// file lock. Cross-session growth is bounded by real usage (one line per
// actual gated-override command), and Bead 3's report layer re-collapses.
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
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/redact"
)

// JournalStormCapL is the per-fingerprint-PER-SESSION append cap (Req 8).
// Within a single process invocation, once this many lines have been
// appended for a given fingerprint, further appends for that fingerprint
// are dropped so a runaway in-process loop cannot bloat the journal. It is
// an exported named constant a deterministic test asserts against (firing
// L+1 times within one session → exactly L lines on disk).
const JournalStormCapL = 50

// journalSchemaVersion is the on-disk record schema version (the "v"
// field of the §Storage Contract journal record).
const journalSchemaVersion = 1

// fileMode is the owner-only mode every store file is created with (HC-8).
const fileMode os.FileMode = 0o600

// StateDirEnv lets an operator (and the test suite) pin the journal's
// state directory explicitly, bypassing the config.GlobalConfigDir()
// XDG/HOME resolution. The override is STILL guarded against landing
// inside a git/project tree (HC-3): a value that resolves inside a git
// work tree is rejected and the write fails closed. Primarily a
// deterministic test seam.
const StateDirEnv = "MINDSPEC_STATE_DIR"

// journalFileName is the fixed journal filename under the state dir.
const journalFileName = "journal.jsonl"

// mu serializes in-process appends so two goroutines never interleave a
// partial line and the per-session storm counter is race-free. The write
// itself is a SINGLE O_APPEND write(2); cross-process appends are atomic
// for a sub-PIPE_BUF line, so no file lock is needed (§Storage Contract).
var mu sync.Mutex

// sessionCounts tracks how many lines THIS process has appended per
// fingerprint, to enforce the per-session storm cap (Req 8). It is reset
// only by process exit (a "session" == one mindspec invocation). Guarded
// by mu.
var sessionCounts = map[string]int{}

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
// text). One Record is serialized per line, append-only, to
// journal.jsonl. Each line carries its OWN version + ts, so Bead 3 can
// derive first/last-seen version per identity by reading the history.
// There is NO count field on the journal record (count lives on Bead 3's
// reports.jsonl, derived by collapsing these lines).
type Record struct {
	V           int      `json:"v"`
	TS          string   `json:"ts"`
	Argv0       string   `json:"argv0"`
	Command     string   `json:"command"`
	EscapeHatch string   `json:"escape_hatch"`
	Subcommand  string   `json:"subcommand"`
	Fingerprint string   `json:"fingerprint"`
	Identity    Identity `json:"identity"`
	Version     string   `json:"version"`
}

// nowRFC3339 is the timestamp source, overridable in tests. It returns an
// RFC3339 string (the §Storage Contract "ts" field).
var nowRFC3339 = func() string { return time.Now().UTC().Format(time.RFC3339) }

// Dir resolves the dedicated, non-synced state directory for the friction
// store. The MINDSPEC_STATE_DIR env override wins (a deterministic test /
// explicit-operator seam) BUT is still subjected to the enclosing-git-tree
// guard (HC-3): an override that resolves inside a git work tree / project
// tree is REJECTED (error) so the journal can never land under a
// committable tree. Without the override it is config.GlobalConfigDir()
// (os.UserConfigDir()/mindspec — XDG-honoring, git-tree-guarded). The
// directory is NOT created here; AppendSuccessEvent creates it lazily 0700.
func Dir() (string, error) {
	if d := os.Getenv(StateDirEnv); d != "" {
		// HC-3: the override must never resolve inside a git/project work
		// tree, where journal.jsonl could be tracked/committed/pushed. Apply
		// the SAME guard config.GlobalConfigDir() uses, and FAIL CLOSED.
		if root := config.EnclosingGitTree(d); root != "" {
			return "", fmt.Errorf("refusing MINDSPEC_STATE_DIR %q: it resolves inside the git work tree at %q (HC-3: the friction store must never be under a committable tree)", d, root)
		}
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
// stamps the bare version + an RFC3339 ts, and APPENDS exactly ONE
// redacted JSONL line to the isolated 0600 journal in a single atomic
// O_APPEND write (no collapse, no rewrite — §Storage Contract).
//
// FAIL-CLOSED (HC-7): if RedactEvent drops the event (a field that cannot
// be confidently classified), NOTHING is written and a nil error is
// returned — the drop is a privacy SUCCESS, not a journaling failure. A
// guarded MINDSPEC_STATE_DIR (one inside a git tree) also fails closed:
// Dir() returns an error and NOTHING is written. A genuine I/O error
// (open/write) IS returned so the best-effort caller can swallow it.
//
// STORM CAP (Req 8): within this process, at most JournalStormCapL lines
// are appended per fingerprint; an append beyond the cap is dropped (nil
// error). The redaction governs DATA EMISSION (drop the datum); it never
// affects the caller's command exit.
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
		// A guarded override (git-tree reject) or unresolvable dir fails
		// closed: nothing is written. The best-effort caller swallows it.
		return err
	}

	rec := Record{
		V:           journalSchemaVersion,
		TS:          nowRFC3339(),
		Argv0:       redacted.Argv0,
		Command:     redacted.Command,
		EscapeHatch: redacted.EscapeHatch,
		Subcommand:  redacted.Subcommand,
		Fingerprint: redacted.Fingerprint,
		Identity: Identity{
			Command:     redacted.Identity.Command,
			EscapeHatch: redacted.Identity.EscapeHatch,
			Subcommand:  redacted.Identity.Subcommand,
		},
		Version: redacted.Version,
	}

	line, err := json.Marshal(&rec)
	if err != nil {
		return fmt.Errorf("journal: marshal record: %w", err)
	}
	line = append(line, '\n')

	mu.Lock()
	defer mu.Unlock()

	// Per-session storm cap (Req 8): drop appends past L for this
	// fingerprint within THIS process invocation.
	if sessionCounts[redacted.Fingerprint] >= JournalStormCapL {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("journal: create state dir: %w", err)
	}

	if err := appendLine(path, line); err != nil {
		return err
	}
	sessionCounts[redacted.Fingerprint]++
	return nil
}

// appendLine appends one already-serialized JSONL line to path in a single
// O_APPEND write(2). O_APPEND makes the write position-atomic, so two
// concurrent processes appending sub-PIPE_BUF lines never interleave or
// lost-update (§Storage Contract). The file is created 0600 (HC-8); when
// it pre-existed under a permissive umask the mode is re-asserted via the
// open fd. Caller holds mu.
func appendLine(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, fileMode)
	if err != nil {
		return fmt.Errorf("journal: open %s: %w", path, err)
	}
	// Belt-and-suspenders: O_CREATE only applies mode on creation, so a
	// pre-existing file under a permissive umask is forced back to 0600.
	if err := f.Chmod(fileMode); err != nil {
		_ = f.Close()
		return fmt.Errorf("journal: chmod %s: %w", path, err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("journal: append %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("journal: close %s: %w", path, err)
	}
	return nil
}

// ReadEvents reads and returns every append-only journal Record in file
// order (oldest first). It is the raw read surface Bead 3 consolidates
// over: Bead 3 collapses these per-event lines by identity+fingerprint
// into reports.jsonl, deriving count + first/last version per identity
// from the version-stamped history. Returns an empty slice when the
// journal does not exist yet. Malformed lines (a partial cross-process
// write, in theory) are SKIPPED, never fatal.
func ReadEvents() ([]Record, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	return readRecords(path)
}

// ListReports is the §API Contract read surface name Bead 3's `report
// list` consolidates over. In Bead 2 it surfaces the raw append-only
// journal Records (Bead 3 layers the collapsed reports.jsonl Report type
// on top). Equivalent to ReadEvents.
func ListReports() ([]Record, error) {
	return ReadEvents()
}

// MarkResolved is the §API Contract Bead-3 SEAM (resolve a friction report
// by fingerprint + resolved-in version). It operates on Bead 3's REPORTS
// layer (reports.jsonl), NOT by mutating the append-only journal.jsonl —
// the journal is immutable history. In Bead 2 this is a minimal stub that
// Bead 3 completes once reports.jsonl exists; it is wired here so Bead 3
// integrates against a settled signature.
func MarkResolved(fp string, ver string) error {
	// Bead 3 implements the reports.jsonl resolve. The journal itself is
	// append-only and never mutated here. Intentionally a no-op stub.
	_ = fp
	_ = ver
	return nil
}

// readRecords parses every JSONL line of path into a Record slice. A
// missing file is the empty-journal case (no error). Malformed lines are
// SKIPPED rather than aborting the whole read — the append-only journal
// tolerates an interleaved/partial line, so one bad line must not lose the
// rest. Caller holds mu.
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
