package journal

// reports.go — spec 094 Bead 3 (Req 4 / Req 5; the regression/stale loop of
// Req 3): the CONSOLIDATED friction-report view (§Storage Contract
// reports.jsonl), built by collapsing the append-only journal.jsonl by the
// normalized event identity + fingerprint.
//
// # Two-file design (§Storage Contract)
//
// journal.jsonl (Bead 2) is APPEND-ONLY, immutable history — one redacted
// event per line, each carrying its OWN version. reports.jsonl (this file)
// is the CONSOLIDATED, mutable VIEW: one Report per fingerprint with the
// occurrence count, first/last version seen, a resolved-in version, and a
// derived status (open/regression/stale/resolved). `mindspec report`
// rebuilds reports.jsonl from the journal (preserving any prior
// resolved_in_version marks); `mindspec report list` reads + classifies it;
// MarkResolved stamps a resolved-in version.
//
// Both files live in the SAME dedicated, non-synced, 0600 store dir (Dir())
// — NEVER under any project/git/bd/dolt tree (HC-3 / HC-8). reports.jsonl is
// derived ONLY from already-redacted journal lines, so it carries no value
// the journal did not already scrub; the report-command RENDER surface
// (cmd/mindspec/report.go) applies a defense-in-depth render scrub on top
// (Req 7 / HC-4).
//
// # resolved_in_version + the regression/stale loop (Req 3 / DQ4)
//
// A report MarkResolved'd at version X records resolved_in_version=X. A
// LATER occurrence (a journal line whose version, the report's last_version,
// is >= X via version.Compare) is a REGRESSION; one at < X is stale
// (suppressed); a dev/unparseable running version is treated as
// unbounded-newest → REGRESSION (fail toward surfacing, never suppress). The
// classification is computed at report-list time by Classify(), keyed by the
// normalized identity + fingerprint (NOT the opaque hash alone — collision
// safety, DQ5).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/version"
)

// reportSchemaVersion is the on-disk reports.jsonl record schema version
// (the "v" field of the §Storage Contract friction-report record).
const reportSchemaVersion = 1

// reportsFileName is the fixed consolidated-reports filename under Dir().
const reportsFileName = "reports.jsonl"

// Status is the derived triage status of a consolidated friction report
// (Req 5). It is computed by Classify() from the report's resolved_in
// version vs its last-seen version (Req 3), NOT persisted as ground truth —
// only resolved_in_version is persisted; the status is always re-derived so
// a stale stored status can never lie.
type Status string

const (
	// StatusOpen — a never-resolved report (no resolved_in_version).
	StatusOpen Status = "open"
	// StatusResolved — resolved_in_version set and the last occurrence is
	// strictly OLDER than the resolution (no recurrence since).
	StatusResolved Status = "resolved"
	// StatusRegression — resolved_in_version set but a later occurrence at a
	// running/last version >= X reopened it (the >= boundary), OR the version
	// is dev/unparseable (unbounded-newest → fail toward surfacing).
	StatusRegression Status = "regression"
	// StatusStale — resolved_in_version set and the last occurrence is older
	// than X (suppressed; kept for the historical record).
	StatusStale Status = "stale"
)

// Report is the §Storage Contract consolidated friction-report record
// (reports.jsonl). One Report per fingerprint, collapsing the append-only
// journal lines of that identity. It carries NO free text — only the
// closed-set enum tokens the journal already validated, plus derived
// counts/versions.
type Report struct {
	V                 int      `json:"v"`
	Fingerprint       string   `json:"fingerprint"`
	Identity          Identity `json:"identity"`
	Command           string   `json:"command"`
	EscapeHatch       string   `json:"escape_hatch"`
	Subcommand        string   `json:"subcommand"`
	Count             int      `json:"count"`
	FirstVersion      string   `json:"first_version"`
	FirstSeenTS       string   `json:"first_seen_ts"`
	LastSeenTS        string   `json:"last_seen_ts"`
	LastVersion       string   `json:"last_version"`
	ResolvedInVersion string   `json:"resolved_in_version,omitempty"`
}

// ReportsPath returns the absolute path to reports.jsonl under Dir().
func ReportsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, reportsFileName), nil
}

// Consolidate reads the append-only journal (ReadEvents) and collapses it by
// the normalized identity + fingerprint into a slice of Reports, deriving:
//
//   - count: number of journal lines for that fingerprint;
//   - first_version / first_seen_ts: from the EARLIEST line (Bead 2 preserved
//     a per-line version, so first-seen is meaningful);
//   - last_version / last_seen_ts: from the LATEST line.
//
// It PRESERVES any prior resolved_in_version mark for a fingerprint that
// already exists in reports.jsonl, so consolidating again after a
// MarkResolved does not erase the resolution. Fingerprints present only in
// the old reports file but absent from the journal are retained (the journal
// is the live history; a resolved report whose lines were pruned should not
// vanish), but in v1 the journal is never pruned so this is defensive.
//
// "Version order" uses version.Compare where both versions are concrete; a
// dev/unparseable version is treated as NEWEST (unbounded-newest, DQ4) so the
// last-seen reflects the most recent dev build. Ties and same-version lines
// fall back to file/TS order (the journal is appended oldest-first).
func Consolidate() ([]Report, error) {
	events, err := ReadEvents()
	if err != nil {
		return nil, err
	}

	prior, err := ReadReports()
	if err != nil {
		return nil, err
	}
	priorResolved := map[string]string{}
	for _, r := range prior {
		if r.ResolvedInVersion != "" {
			priorResolved[r.Fingerprint] = r.ResolvedInVersion
		}
	}

	byFP := map[string]*Report{}
	var order []string
	for _, ev := range events {
		fp := ev.Fingerprint
		r, ok := byFP[fp]
		if !ok {
			r = &Report{
				V:                 reportSchemaVersion,
				Fingerprint:       fp,
				Identity:          ev.Identity,
				Command:           ev.Command,
				EscapeHatch:       ev.EscapeHatch,
				Subcommand:        ev.Subcommand,
				Count:             0,
				FirstVersion:      ev.Version,
				FirstSeenTS:       ev.TS,
				LastVersion:       ev.Version,
				LastSeenTS:        ev.TS,
				ResolvedInVersion: priorResolved[fp],
			}
			byFP[fp] = r
			order = append(order, fp)
		}
		r.Count++
		// first-seen: keep the OLDER version (or older TS on a tie).
		if versionOlder(ev.Version, r.FirstVersion) {
			r.FirstVersion = ev.Version
			r.FirstSeenTS = ev.TS
		}
		// last-seen: keep the NEWER version (dev == newest), TS breaks ties.
		if versionNewer(ev.Version, r.LastVersion) {
			r.LastVersion = ev.Version
			r.LastSeenTS = ev.TS
		}
	}

	reports := make([]Report, 0, len(order))
	for _, fp := range order {
		reports = append(reports, *byFP[fp])
	}
	return reports, nil
}

// versionNewer reports whether candidate is strictly newer than current under
// the DQ4 policy: when both parse as concrete semver, by Compare; a
// dev/unparseable candidate is unbounded-newest (newer than any concrete);
// dev-vs-dev and concrete-vs-dev(current) are NOT strictly newer (the tie /
// the existing dev stays). This keeps last-seen at the most recent build.
func versionNewer(candidate, current string) bool {
	cmp, ok := version.Compare(candidate, current)
	if ok {
		return cmp > 0
	}
	// One side is dev/unparseable.
	candDev := !parses(candidate)
	curDev := !parses(current)
	switch {
	case candDev && curDev:
		return false // dev vs dev: keep the first (TS order already preserved)
	case candDev && !curDev:
		return true // a dev candidate is unbounded-newest over a concrete current
	default:
		return false // concrete candidate vs dev current: dev stays newest
	}
}

// versionOlder reports whether candidate is strictly older than current under
// the inverse DQ4 policy for first-seen: a dev/unparseable version is NEWEST,
// so it is never "older"; a concrete candidate is older than a dev current.
func versionOlder(candidate, current string) bool {
	cmp, ok := version.Compare(candidate, current)
	if ok {
		return cmp < 0
	}
	candDev := !parses(candidate)
	curDev := !parses(current)
	switch {
	case candDev && curDev:
		return false
	case candDev && !curDev:
		return false // a dev candidate is newest, never older
	default:
		return true // concrete candidate is older than a dev current
	}
}

func parses(v string) bool {
	_, ok := version.Parse(v)
	return ok
}

// Classify derives the triage Status of a report from its resolved_in_version
// vs its last-seen version (Req 3 / DQ4). It is computed at report-list time,
// never persisted, so a stored status can never go stale:
//
//   - no resolved_in_version            → open
//   - last >= resolved (concrete, >= boundary) → regression
//   - last <  resolved (concrete)              → stale (suppressed)
//   - last or resolved dev/unparseable         → regression (unbounded-newest)
//
// The == boundary is REGRESSION (a re-occurrence at the resolving version is
// the loop not closing); a dev running/last version fails toward surfacing.
func (r Report) Classify() Status {
	if r.ResolvedInVersion == "" {
		return StatusOpen
	}
	cmp, ok := version.Compare(r.LastVersion, r.ResolvedInVersion)
	if !ok {
		// dev/unparseable on either side → unbounded-newest → regression.
		return StatusRegression
	}
	if cmp >= 0 {
		return StatusRegression // >= boundary: re-occurred at/after resolution
	}
	return StatusStale // last occurrence predates the resolution
}

// ReadReports reads every reports.jsonl record (consolidated friction
// reports) in file order. A missing file is the empty case (no error).
// Malformed lines are SKIPPED, never fatal (mirrors readRecords).
func ReadReports() ([]Report, error) {
	path, err := ReportsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("journal: read %s: %w", path, err)
	}
	var reports []Report
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var r Report
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		reports = append(reports, r)
	}
	return reports, nil
}

// WriteReports atomically replaces reports.jsonl with the given reports (one
// JSONL line each), created 0600 under the isolated store dir (HC-8). Unlike
// the append-only journal, reports.jsonl is the CONSOLIDATED VIEW and IS
// rewritten wholesale (the §Storage Contract's 2-file design). The write is
// guarded by mu so a concurrent consolidate/resolve cannot interleave, and is
// done write-to-temp + rename for crash-atomicity.
func WriteReports(reports []Report) error {
	path, err := ReportsPath()
	if err != nil {
		return err
	}

	var buf []byte
	for i := range reports {
		if reports[i].V == 0 {
			reports[i].V = reportSchemaVersion
		}
		line, err := json.Marshal(&reports[i])
		if err != nil {
			return fmt.Errorf("journal: marshal report: %w", err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}

	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("journal: create state dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), reportsFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("journal: create temp reports file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("journal: chmod temp reports file: %w", err)
	}
	if _, err := tmp.Write(buf); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("journal: write temp reports file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("journal: close temp reports file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("journal: rename reports file into place: %w", err)
	}
	// Re-assert 0600 in case the rename target pre-existed under a permissive
	// umask (belt-and-suspenders, mirrors appendLine).
	if err := os.Chmod(path, fileMode); err != nil {
		return fmt.Errorf("journal: chmod reports file: %w", err)
	}
	return nil
}

// MarkResolved stamps resolved_in_version=ver on the report whose fingerprint
// matches fp, persisted to reports.jsonl. It is the §API Contract Bead-3
// resolve SEAM (Bead 2 shipped a no-op stub over the immutable journal; this
// is the real implementation over the reports LAYER — the journal is NEVER
// mutated).
//
// It first CONSOLIDATES (so the report exists even if `report` was never run),
// then sets the resolution keyed by fingerprint (the normalized identity is
// persisted alongside, so a hash collision cannot silently resolve two
// distinct events — DQ5), then rewrites reports.jsonl. An unknown fingerprint
// is an error (you cannot resolve a report that was never observed). A blank
// ver is rejected (a resolution needs a concrete-or-dev version to compare
// against).
func MarkResolved(fp string, ver string) error {
	if fp == "" {
		return fmt.Errorf("journal: MarkResolved requires a fingerprint")
	}
	if ver == "" {
		return fmt.Errorf("journal: MarkResolved requires a resolved-in version")
	}
	reports, err := Consolidate()
	if err != nil {
		return err
	}
	found := false
	for i := range reports {
		if reports[i].Fingerprint == fp {
			reports[i].ResolvedInVersion = ver
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("journal: no friction report with fingerprint %q to resolve (run `mindspec report` first, then `mindspec report list`)", fp)
	}
	return WriteReports(reports)
}
