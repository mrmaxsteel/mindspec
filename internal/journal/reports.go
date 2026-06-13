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
// derived status (open/regression/stale). `mindspec report`
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
// classification is computed at report-list time by Classify(), keyed by
// fingerprint ALONE — collision-safe BECAUSE fingerprint = H(identity)
// (redact.Fingerprint is the SHA-256 over the NUL-framed identity tuple, so
// distinct identities yield distinct fingerprints by construction; it is NOT an
// opaque/arbitrary hash that could alias two events). The identity tuple is
// persisted on the record for display/audit only, not as a second key (DQ5).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/redact"
	"github.com/mrmaxsteel/mindspec/internal/version"
)

// reportSchemaVersion is the on-disk reports.jsonl record schema version
// (the "v" field of the §Storage Contract friction-report record).
const reportSchemaVersion = 1

// reportsFileName is the fixed consolidated-reports filename under Dir().
const reportsFileName = "reports.jsonl"

// writeRereadHook is a test-only seam (nil in production, mirroring the
// nowRFC3339 seam) invoked inside writeReportsLocked AFTER the on-disk re-read
// and BEFORE the temp+rename. A concurrency test sets it to widen that window
// so two in-process writers deterministically interleave, proving the OS lock
// (not timing luck) serializes the read-modify-write. It is NEVER set in
// non-test code.
var writeRereadHook func()

// Status is the derived triage status of a consolidated friction report
// (Req 5). It is computed by Classify() from the report's resolved_in
// version vs its last-seen version (Req 3), NOT persisted as ground truth —
// only resolved_in_version is persisted; the status is always re-derived so
// a stale stored status can never lie.
//
// The status model is exactly {open, regression, stale} — the faithful
// 3-state realization of spec Req-3's two-way resolved split plus the
// unresolved case (codex-completeness #1):
//
//   - open       — no resolved_in_version yet (never triaged);
//   - regression — resolved at X but the last occurrence is >= X (the loop
//     did not close), OR a dev/unparseable operand (DQ4);
//   - stale      — resolved at X and the last occurrence is < X (suppressed).
//
// There is NO standalone "resolved" status: a resolved report with no later
// recurrence is already represented by resolved_in_version being set with a
// last_version < X (i.e. `stale`). Earlier drafts of the help/ADR text spoke
// of a fourth `resolved` state, but Classify never returned it — it was DEAD.
// The language is now reconciled to this 3-state model across code + help +
// ADR-0038 so code, docs, and spec agree.
type Status string

const (
	// StatusOpen — a never-resolved report (no resolved_in_version).
	StatusOpen Status = "open"
	// StatusRegression — resolved_in_version set but a later occurrence at a
	// running/last version >= X reopened it (the >= boundary), OR the version
	// is dev/unparseable (unbounded-newest → fail toward surfacing).
	StatusRegression Status = "regression"
	// StatusStale — resolved_in_version set and the last occurrence is older
	// than X (suppressed; kept for the historical record). This subsumes the
	// "resolved, no recurrence since" case — there is no separate `resolved`.
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
//   - first_version / first_seen_ts: from the EARLIEST OCCURRENCE of that
//     fingerprint (the chronologically-first journal event);
//   - last_version / last_seen_ts: from the LATEST OCCURRENCE.
//
// first/last are derived by OCCURRENCE ORDER, NOT by semver extrema
// (codex-consolidation #1). The spec/plan say "first/last version SEEN" and
// ADR-0038 says "first_version comes from the earliest EVENT" — so the
// EARLIEST event's version (whatever it is) is first_version, paired with that
// event's ts, and the LATEST event's version is last_version with its ts. The
// version and its paired *_seen_ts ALWAYS move together. Deriving by semver
// min/max instead would report the wrong first/last and mismatched timestamps
// for an out-of-order stream (e.g. events appended newest-first, or a
// downgrade build). Occurrence order is authoritative over the earlier
// semver-extrema reading.
//
// Occurrence order is by ts (RFC3339, lexicographically sortable), tie-broken
// by file/append order — the append-only journal is written oldest-first, so
// ReadEvents already yields append order; we use a STABLE sort on ts so equal
// timestamps preserve that append order.
//
// It PRESERVES any prior resolved_in_version mark for a fingerprint that
// already exists in reports.jsonl, so consolidating again after a
// MarkResolved does not erase the resolution. Fingerprints present only in
// the old reports file but absent from the journal are retained (the journal
// is the live history; a resolved report whose lines were pruned should not
// vanish), but in v1 the journal is never pruned so this is defensive.
//
// READ-SIDE re-validation (privacy defense-in-depth): journal.jsonl is treated
// as UNTRUSTED input on READ — a legitimately-written line was enum-validated
// + redacted at write time (AppendSuccessEvent → redact.RedactEvent), but a
// HAND-PLANTED / tampered / corrupt journal line could carry raw paths,
// secrets, emails, IPs, or shell metachars in any field. Each parsed Record is
// re-run through the SAME validation AppendSuccessEvent applies at WRITE
// (validRecord → redact.RedactEvent over the persisted enum/version/fingerprint
// shape); a record that FAILS is DROPPED (skipped). A legitimately-written
// record ALWAYS passes (it was enum-validated at write), so only tampered/
// corrupt records are dropped — fail-closed. This extends the existing
// malformed-line tolerance (unparseable JSON skipped in readRecords) to
// "parseable but non-conforming", so reports.jsonl can NEVER persist a
// non-enum / non-64hex-fingerprint / non-canonical-semver value copied from a
// hostile journal.
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

	// Order events by occurrence: stable-sort on ts so equal timestamps keep
	// their append (file) order. A stable sort over a copy leaves ReadEvents'
	// append order intact for any same-ts run.
	ordered := make([]Record, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].TS < ordered[j].TS
	})

	byFP := map[string]*Report{}
	var order []string
	for _, raw := range ordered {
		// READ-SIDE re-validation: drop any journal record that does not pass
		// the SAME enum/version/fingerprint validation applied at write time.
		// A hand-planted/tampered/corrupt line (raw path/secret/email/IP/
		// metachar in any field) fails and is skipped; only conforming,
		// write-validated values survive into reports.jsonl (fail-closed).
		ev, ok := validRecord(raw)
		if !ok {
			continue
		}
		fp := ev.Fingerprint
		r, ok := byFP[fp]
		if !ok {
			// First occurrence in chronological order → first_version/ts.
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
		// Each later occurrence in chronological order advances last_*; version
		// and its paired ts move together.
		r.LastVersion = ev.Version
		r.LastSeenTS = ev.TS
	}

	reports := make([]Report, 0, len(order))
	for _, fp := range order {
		reports = append(reports, *byFP[fp])
	}
	return reports, nil
}

// validRecord re-validates ONE parsed journal Record on READ with the SAME
// enum/shape validation AppendSuccessEvent applies at WRITE time, returning a
// SANITIZED Record (canonical values) + ok. It treats journal.jsonl as
// untrusted input (privacy defense-in-depth): a hand-planted / tampered /
// corrupt line that carries a raw path, secret, email, IP, or shell-metachar
// in ANY field is REJECTED (ok=false) so Consolidate drops it before any field
// can be copied into reports.jsonl. A legitimately-written record always
// passes, because it was produced by RedactEvent at write time.
//
// The check reconstructs the redact.Event from the record's persisted enum
// fields + argv0 + version (OS is not persisted on the journal record, so it
// is left "" — a member of the OS allowlist) and runs redact.RedactEvent,
// which:
//   - validates Command/EscapeHatch/Subcommand against their closed-set
//     allowlists (a non-enum token DROPS);
//   - canonicalizes Version to a bare semver / "dev"/"" (a decorated or
//     metachar-bearing version DROPS);
//   - reduces argv0 to a scrubbed basename (a non-classifiable argv0 DROPS);
//   - recomputes the Fingerprint over the normalized identity.
//
// On top of RedactEvent the record is additionally rejected unless:
//   - its persisted fingerprint EXACTLY equals the recomputed H(identity)
//     (a planted/forged fingerprint that does not match the identity is not a
//     real fingerprint — drop, so the render-layer 64-hex check is never the
//     only line);
//   - its persisted identity tuple equals the (already enum-validated)
//     top-level command/escape_hatch/subcommand tokens (a record whose
//     identity disagrees with its display tokens is malformed — drop).
//
// The returned sanitized Record carries ONLY the canonical, re-derived values,
// so even a record that "passes" can never leak a raw stored value.
func validRecord(r Record) (Record, bool) {
	red, ok := redact.RedactEvent(redact.Event{
		Argv0:       r.Argv0,
		Command:     r.Command,
		EscapeHatch: r.EscapeHatch,
		Subcommand:  r.Subcommand,
		Version:     r.Version,
		OS:          "", // OS is not persisted on the journal record.
	})
	if !ok {
		return Record{}, false
	}
	// The persisted identity tuple must agree with the (enum-validated)
	// top-level tokens, and the persisted fingerprint must be exactly the
	// recomputed H(identity) — a forged/mismatched fingerprint or identity is
	// not a legitimately-written record.
	if r.Identity.Command != r.Command ||
		r.Identity.EscapeHatch != r.EscapeHatch ||
		r.Identity.Subcommand != r.Subcommand {
		return Record{}, false
	}
	if r.Fingerprint != red.Fingerprint {
		return Record{}, false
	}
	// The timestamp is copied into reports.jsonl (first/last_seen_ts) and is
	// the one record field NOT covered by RedactEvent, so a planted `ts`
	// carrying a path/secret/metachar would otherwise ride through. Require it
	// to parse as RFC3339 (its only legitimate shape, written by nowRFC3339);
	// anything else is not a real timestamp → drop the record. Re-emit the
	// canonical RFC3339 form so only that exact shape is persisted.
	ts, err := time.Parse(time.RFC3339, r.TS)
	if err != nil {
		return Record{}, false
	}
	canonTS := ts.UTC().Format(time.RFC3339)
	// Rebuild from canonical values only — never the raw stored fields.
	return Record{
		V:           r.V,
		TS:          canonTS,
		Argv0:       red.Argv0,
		Command:     red.Command,
		EscapeHatch: red.EscapeHatch,
		Subcommand:  red.Subcommand,
		Fingerprint: red.Fingerprint,
		Identity: Identity{
			Command:     red.Identity.Command,
			EscapeHatch: red.Identity.EscapeHatch,
			Subcommand:  red.Identity.Subcommand,
		},
		Version: red.Version,
	}, true
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
	return readReportsRaw(path)
}

// readReportsRaw parses every reports.jsonl line at path into a Report slice
// (file order; malformed lines skipped; missing file → empty, no error). It
// does NOT acquire mu, so callers already holding the lock (WriteReports'
// cross-process merge re-read) can reuse it.
func readReportsRaw(path string) ([]Report, error) {
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
// rewritten wholesale (the §Storage Contract's 2-file design), done
// write-to-temp + rename for crash-atomicity.
//
// Cross-process safety (codex-consolidation #1/#2): the in-process mu does NOT
// span processes, so a stale writer (`report`: Consolidate→WriteReports) could
// clobber a concurrent `report list --resolve` from ANOTHER process and erase
// its resolved_in_version, or DROP an on-disk-only row added after its snapshot.
// The whole read-modify-write is therefore serialized by an OS-VISIBLE advisory
// lock (withReportsLock → flock on unix, an O_EXCL lockfile on windows) held
// across the on-disk re-read + union-merge + temp+rename. Under the lock we
// UNION the fresh on-disk view with the incoming slice (writeReportsLocked):
//
//   - a NON-EMPTY resolved_in_version on disk wins over an empty incoming slot
//     (a concurrent resolve is never erased); and
//   - any fingerprint present ON DISK but ABSENT from the incoming slice is
//     CARRIED FORWARD (a stale snapshot can never DELETE a row another process
//     wrote — e.g. a newly-resolved fingerprint added after this writer's read).
//
// No stale snapshot can lose another process's update.
func WriteReports(reports []Report) error {
	return withReportsLock(func() error {
		return writeReportsLocked(reports)
	})
}

// writeReportsLocked is the read-modify-write body that MUST run while holding
// withReportsLock (the OS-visible cross-process lock). It re-reads the fresh
// on-disk reports, unions them with the incoming slice (carrying forward both
// resolved-state AND on-disk-only rows), then atomically temp+renames the
// merged view into place. Callers already inside withReportsLock (MarkResolved's
// consolidate→resolve→write) call this directly so the entire sequence is one
// critical section. The whole re-read+union+rename runs while the OS lock is
// held, so no other process can interleave between the re-read and the rename.
func writeReportsLocked(reports []Report) error {
	path, err := ReportsPath()
	if err != nil {
		return err
	}

	// Re-read the fresh on-disk view under the lock and UNION it with the
	// incoming slice. Two merges, neither of which can lose another writer's
	// update:
	//   (1) carry forward a non-empty on-disk resolved_in_version onto an
	//       incoming row whose slot is empty (a concurrent resolve survives);
	//   (2) APPEND any on-disk fingerprint not present in the incoming slice
	//       (a stale snapshot never deletes a row another process added).
	if onDisk, rerr := readReportsRaw(path); rerr == nil {
		// Test seam (nil in production, mirrors nowRFC3339): widen the
		// re-read→rename window so a concurrency test can deterministically
		// interleave two writers and prove the OS lock — not luck — serializes
		// the read-modify-write. Never set outside tests.
		if writeRereadHook != nil {
			writeRereadHook()
		}
		present := map[string]int{} // fingerprint → index in reports
		for i := range reports {
			present[reports[i].Fingerprint] = i
		}
		for _, d := range onDisk {
			if idx, ok := present[d.Fingerprint]; ok {
				// (1) on-disk resolution wins over an empty incoming slot.
				if reports[idx].ResolvedInVersion == "" && d.ResolvedInVersion != "" {
					reports[idx].ResolvedInVersion = d.ResolvedInVersion
				}
				continue
			}
			// (2) on-disk-only fingerprint: carry the whole row forward.
			reports = append(reports, d)
			present[d.Fingerprint] = len(reports) - 1
		}
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
// then sets the resolution keyed by fingerprint ALONE. This is collision-safe
// BECAUSE fingerprint = H(identity) (the SHA-256 over the NUL-framed identity
// tuple), so distinct identities cannot share a fingerprint and a resolve can
// never silently apply to two distinct events — DQ5. The identity is persisted
// alongside for display/audit only, not as a second key. Then it rewrites
// reports.jsonl. An unknown fingerprint is an error (you cannot resolve a
// report that was never observed).
//
// The ENTIRE consolidate→resolve→write runs inside ONE withReportsLock critical
// section (codex-consolidation #1/#2): the consolidation reads the FRESH journal
// + reports.jsonl and the write unions on-disk-only rows back, so two processes
// resolving DIFFERENT fingerprints concurrently both survive (neither can read a
// stale snapshot and clobber the other's resolve), and a resolve never deletes a
// row another process added.
//
// The resolve VERSION is NORMALIZED at the source (R1 / Req 7 / HC-4): only a
// concrete semver (canonicalized to bare `major.minor.patch`) or the explicit
// dev/current policy value is ever PERSISTED. Anything else — most importantly
// a shell-metacharacter payload like `1.0.0; rm -rf /` — is REJECTED with an
// error and never written to reports.jsonl, so a live executable user string
// can never reach the copy-pasteable resolve-echo or the RESOLVED-IN render
// column. This closes the slot at the SOURCE rather than relying on the render
// scrub alone (the render scrub does not neutralize shell metacharacters).
func MarkResolved(fp string, ver string) error {
	if fp == "" {
		return fmt.Errorf("journal: MarkResolved requires a fingerprint")
	}
	norm, ok := normalizeResolveVersion(ver)
	if !ok {
		// Do NOT echo the rejected value back: a shell-metachar payload like
		// `1.0.0; rm -rf /` must never reach the (copy-pasteable) error surface
		// either (R1). The error names the CONTRACT, not the offending input.
		return fmt.Errorf("journal: MarkResolved: invalid resolve version — pass a concrete semver (e.g. 1.4.2) or %q", version.Current())
	}
	// Consolidate→resolve→write under ONE cross-process lock so a concurrent
	// resolve in another process cannot be lost (codex-consolidation #1/#2).
	return withReportsLock(func() error {
		reports, err := Consolidate()
		if err != nil {
			return err
		}
		found := false
		for i := range reports {
			if reports[i].Fingerprint == fp {
				reports[i].ResolvedInVersion = norm
				found = true
				break
			}
		}
		if !found {
			// Do NOT echo the raw user-supplied fingerprint back: like the
			// sibling invalid-version error above (R1), a path/secret/
			// shell-metachar payload smuggled into the --resolve argument must
			// never reach the (copy-pasteable) error surface. Name the CONTRACT,
			// not the offending input — a non-64-hex value is not a real
			// fingerprint anyway, so there is nothing legitimate to echo.
			return fmt.Errorf("journal: no friction report matches the given fingerprint to resolve (run `mindspec report` first, then `mindspec report list` to see the fingerprints)")
		}
		return writeReportsLocked(reports)
	})
}

// normalizeResolveVersion validates + canonicalizes a resolve-in version so
// ONLY a well-formed value is ever persisted as resolved_in_version (R1). It
// accepts:
//
//   - a concrete semver (with or without a leading `v`, with an optional
//     `-prerelease`/`+build` suffix) → canonicalized to bare `major.minor.patch`;
//   - the literal dev/current policy token (version.Current(), or the bare
//     "dev" default) → kept verbatim as the DQ4 unbounded-newest sentinel.
//
// Everything else — empty, a non-semver string, or a shell-metachar payload —
// is rejected (ok=false). This is the SOURCE-side neutralization of the one
// user-controlled free-text slot in v1 (the --version flag).
func normalizeResolveVersion(ver string) (string, bool) {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return "", false
	}
	if sv, ok := version.Parse(ver); ok {
		// Re-emit the bare canonical form so a decorated/`v`-prefixed/suffixed
		// input cannot persist anything but `major.minor.patch`.
		return fmt.Sprintf("%d.%d.%d", sv.Major, sv.Minor, sv.Patch), true
	}
	// Not a concrete semver: accept ONLY the explicit dev/current sentinel
	// (the DQ4 unbounded-newest policy value), nothing else.
	if strings.EqualFold(ver, "dev") || ver == version.Current() {
		return ver, true
	}
	return "", false
}
