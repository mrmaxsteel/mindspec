package phase

// phaselock.go — spec 128 (lifecycle-verb-state-honesty) Bead 1, R3.3: the
// repo-scoped, kernel-held write lock that serializes EVERY
// mindspec_phase writer — `plan approve` (plan.go:455), `spec approve`'s
// existing-epic backfill (spec.go:76), `complete`'s step-6.5 sync
// (complete.go:1366), `release`'s step-6.5 sync (release.go:152),
// `impl approve`'s reconcile + done writes (impl.go:577-579/:611-613),
// `impl adopt`'s done write (adopt.go:424/:437), `EnsureMigrated`'s
// ADR-0034 backfill (migrate.go), and `mindspec repair phase`
// (repair.go:189-191) — so a concurrent writer can never land a stale
// value over a fresher one (AC-5(v)).
//
// Kernel-held on both platforms so a crashed holder's lock releases with
// its process — there is NO PID/timestamp content, NO takeover, and NO
// deletion step:
//   - unix: flock(2) LOCK_EX (phaselock_unix.go).
//   - windows: LockFileEx (exclusive byte-range lock) on a RETAINED
//     handle opened WITHOUT FILE_SHARE_DELETE, held for the whole
//     critical section (phaselock_windows.go). This moots the
//     stale-lockfile-content problem entirely: the FILE is not the lock
//     (the byte-range lock is), so a pre-existing empty/partial leftover
//     lockfile never blocks acquisition.
//
// This deliberately does NOT carry internal/journal's O_EXCL-lockfile
// idiom (lock_windows.go there uses a create-and-remove dance) — only
// that package's withReportsLock READ-MODIFY-WRITE SHAPE is imitated
// here (internal/journal/lock.go:55-61: acquire → run the critical
// section → release via defer). The phase lock is kernel-held on BOTH
// platforms.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// PhaseLockRelPath is the repo-relative path to the phase write lock
// file. It is listed in the managed .gitignore block (beside
// .mindspec/session.json) and carries no meaningful content of its own —
// the OS-level lock (flock / LockFileEx) is the mutual-exclusion
// primitive, never the file's bytes.
const PhaseLockRelPath = ".mindspec/phase.lock"

// phaseLockWait/phaseLockPollInterval are the bounded, poll-with-deadline
// wait for lock acquisition and its poll granularity. Package-level VARS
// (not consts) so SetLockWaitForTest can shrink them for a fast,
// deterministic real-lock-contention test in ANY package (round-1 panel
// G3/G6: a 10s production wait would make a real-timeout integration
// test for e.g. release.go's/complete.go's best-effort sync writers
// impractically slow). Production code never calls SetLockWaitForTest;
// the shipped defaults are unchanged. On timeout, acquisition fails with
// ErrPhaseLockTimeout; callers that surface this to an operator name the
// lock path and a re-run recovery line (ADR-0035) — NEVER a
// delete-the-lock instruction: a crashed holder's kernel-held lock has
// already released on its own, and a live holder means the write is
// genuinely contended, so deleting the file changes nothing about who
// holds the lock.
var (
	phaseLockWait         = 10 * time.Second
	phaseLockPollInterval = 25 * time.Millisecond
)

// SetLockWaitForTest overrides the bounded wait/poll-interval pair for
// the duration of one test (callable from any package — root/wait tuning
// is a cross-package test need, e.g. a fast real-lock-timeout test for a
// best-effort sync writer in cmd/mindspec or internal/complete) and
// returns a restore func. Never called from production code.
func SetLockWaitForTest(wait, poll time.Duration) func() {
	origWait, origPoll := phaseLockWait, phaseLockPollInterval
	phaseLockWait, phaseLockPollInterval = wait, poll
	return func() { phaseLockWait, phaseLockPollInterval = origWait, origPoll }
}

// ErrPhaseLockTimeout is returned (wrapped, so errors.Is matches) by
// WithPhaseWriteLock when the bounded wait to acquire the lock expires.
//
// This package cannot format an ADR-0035-shaped recovery line itself:
// internal/guard imports internal/phase (see derive.go's stored-vs-
// derived warning, which is hand-formatted for the identical reason), so
// the error returned here is a plain, unformatted error — callers that
// surface a lock timeout to an operator (e.g. `mindspec repair phase`,
// already routed through guard.NewFailure for every other failure mode
// on that path, or a best-effort sync writer via DescribeLockFailure
// below) wrap it with their own recovery line naming a plain re-run.
var ErrPhaseLockTimeout = errors.New("timed out waiting for the phase write lock")

// DescribeLockFailure renders err — as returned by WithPhaseWriteLock —
// for a caller that cannot fail its whole command on a lock/write
// failure (a best-effort sync, e.g. complete's/release's step-6.5 phase
// cache refresh) but must still SURFACE it rather than silently
// discarding it (round-1 panel G3). A lock-acquisition TIMEOUT
// (errors.Is(err, ErrPhaseLockTimeout)) gets the ADR-0035 treatment: the
// rendered message names the lock path and ends with a copy-pastable
// `recovery: ` line that only ever advises a re-run, never deleting the
// lock file. Any other error (e.g. a genuine merge-write failure) is
// rendered as-is, with no recovery line manufactured for it — that
// belongs to whatever wrote the failing value, not to this generic
// lock-failure describer. Returns "" for a nil err.
func DescribeLockFailure(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrPhaseLockTimeout) {
		return fmt.Sprintf("could not sync phase metadata: %v\nrecovery: re-run once the lock at %s clears (never delete it — a live holder means genuine contention, and a crashed holder's kernel-held lock has already released on its own)", err, PhaseLockRelPath)
	}
	return fmt.Sprintf("could not sync phase metadata: %v", err)
}

// phaseReadOutcome is WithPhaseWriteLock's fresh under-lock epic read.
// Three cases are kept distinct (round-1 panel G1/G5-R5's error/presence-
// aware read): a genuine lookup/decode FAILURE propagates as an error
// from WithPhaseWriteLock itself, before fn is ever invoked and before
// any write is attempted; the field is genuinely ABSENT (Present==false,
// Stored==""); and the field is PRESENT but fails validation
// (Present==true, Stored=="") — an invalid-but-present value is NEVER
// silently converted to absence, because a caller like EnsureMigrated's
// backfill must treat "present but garbage" and "genuinely missing"
// differently (only the latter is a legitimate backfill target).
type phaseReadOutcome struct {
	Stored  string
	Present bool
}

// readPhaseForLock performs WithPhaseWriteLock's fresh, error-propagating,
// presence-aware epic read. It is deliberately NOT readStoredPhaseWithCache
// (derive.go): that helper's established, WIDELY-depended-on contract
// collapses "error", "absent", and "present-but-invalid" into the same ""
// result for its many read-only derivation callers, and changing that
// contract is out of this bead's scope. This is a narrower, lock-specific
// read with a stricter contract.
func readPhaseForLock(c *Cache, epicID string) (phaseReadOutcome, error) {
	epic, err := c.FindEpic(epicID)
	if err != nil {
		return phaseReadOutcome{}, fmt.Errorf("phase: reading epic %s: %w", idrender.Bead(epicID), err)
	}
	if epic == nil || epic.Metadata == nil {
		// Successfully determined absent (FindEpic's own contract: a
		// nil epic with a nil error means "looked up, not found" — a
		// real, expected absence, not a lookup failure).
		return phaseReadOutcome{}, nil
	}
	raw, ok := epic.Metadata["mindspec_phase"]
	if !ok {
		return phaseReadOutcome{}, nil
	}
	s, ok := raw.(string)
	if !ok || !state.IsValidPhase(s) {
		// Present but invalid — NOT absence.
		return phaseReadOutcome{Present: true}, nil
	}
	return phaseReadOutcome{Stored: s, Present: true}, nil
}

// WithPhaseWriteLock acquires the repo-scoped phase write lock rooted at
// root, reads epicID's CURRENT stored mindspec_phase fresh (via a Cache
// constructed for this call alone — never a Cache shared across the
// invocation, so the value fn sees can never be a snapshot taken before
// lock acquisition), and calls fn with that stored value plus whether the
// field was present at all (round-1 panel G1: an invalid-but-present
// value and a genuinely-absent field are NOT the same thing — see
// phaseReadOutcome's doc comment).
//
// A genuine lookup/decode FAILURE while reading the epic is propagated
// as WithPhaseWriteLock's own return value — fn is never invoked and no
// write is attempted (round-1 panel G1: a failed discovery must never
// silently become a sanctioned write of a value derived from nothing).
//
// fn decides what to write:
//   - a non-nil `updates` map performs a merge-write via mergeMetadata
//     (the caller's own MergeMetadata seam) before the lock releases;
//   - a nil `updates` map with a nil error is a no-op (fn decided nothing
//     needs writing — e.g. "already consistent", or a refused backward
//     move);
//   - a non-nil error aborts with NO write.
//
// The lock is released (via defer) whether the read fails, fn errors,
// no-ops, or writes, and mergeMetadata's own error (if the write is
// attempted and fails) is returned to the caller.
//
// Caller obligations (round-1 panel G8 — read both before adding a new
// caller):
//
//  1. The freshness guarantee covers ONLY the stored mindspec_phase value
//     (and its presence bit) passed to fn. Every OTHER input fn's
//     decision depends on — a child-bead status set, a semantics marker,
//     a target/derived value computed by re-deriving from children — is
//     NOT covered by this call's critical section unless fn itself reads
//     it fresh, inside fn, with its own Cache (see SanctionedDerivedSync
//     and `mindspec repair phase`'s guard for the house pattern: a fresh
//     phase.NewCache() constructed INSIDE fn, never reused from before
//     the lock was acquired). A target value computed before calling
//     WithPhaseWriteLock and merely closed over by fn is a STALE read
//     with respect to this lock, exactly the race this lock exists to
//     close.
//  2. The lock is NOT re-entrant. A caller already inside a
//     WithPhaseWriteLock critical section that calls WithPhaseWriteLock
//     again (directly, or transitively — e.g. calling
//     SanctionedDerivedSync from inside another WithPhaseWriteLock's fn)
//     self-blocks on its own held lock until the bounded wait expires,
//     surfacing as ErrPhaseLockTimeout — never a deadlock detection, and
//     never automatic reentry.
func WithPhaseWriteLock(root, epicID string, mergeMetadata MergeMetadataFunc, fn func(stored string, present bool) (updates map[string]interface{}, err error)) error {
	unlock, err := acquirePhaseLock(root)
	if err != nil {
		return err
	}
	defer unlock()

	outcome, err := readPhaseForLock(NewCache(), epicID)
	if err != nil {
		return err
	}
	updates, err := fn(outcome.Stored, outcome.Present)
	if err != nil {
		return err
	}
	if updates == nil {
		return nil
	}
	return mergeMetadata(epicID, updates)
}

// acquirePhaseLock acquires the kernel-held phase lock at
// <root>/.mindspec/phase.lock, creating the .mindspec directory first if
// necessary, and returns an unlock func. See acquirePhaseFileLock
// (phaselock_unix.go / phaselock_windows.go) for the platform-specific
// primitive.
func acquirePhaseLock(root string) (func(), error) {
	path := filepath.Join(root, PhaseLockRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("phase: create lock dir: %w", err)
	}
	unlock, err := acquirePhaseFileLock(path, phaseLockWait, phaseLockPollInterval)
	if err != nil {
		return nil, fmt.Errorf("phase: acquire write lock at %s: %w", path, err)
	}
	return unlock, nil
}
