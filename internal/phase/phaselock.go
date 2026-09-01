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
)

// PhaseLockRelPath is the repo-relative path to the phase write lock
// file. It is listed in the managed .gitignore block (beside
// .mindspec/session.json) and carries no meaningful content of its own —
// the OS-level lock (flock / LockFileEx) is the mutual-exclusion
// primitive, never the file's bytes.
const PhaseLockRelPath = ".mindspec/phase.lock"

// phaseLockWait is the bounded, poll-with-deadline wait for lock
// acquisition. On timeout, acquisition fails with ErrPhaseLockTimeout;
// callers that surface this to an operator name the lock path and a
// re-run recovery line (ADR-0035) — NEVER a delete-the-lock instruction:
// a crashed holder's kernel-held lock has already released on its own,
// and a live holder means the write is genuinely contended, so deleting
// the file changes nothing about who holds the lock.
const phaseLockWait = 10 * time.Second

// phaseLockPollInterval is the poll granularity within phaseLockWait.
const phaseLockPollInterval = 25 * time.Millisecond

// ErrPhaseLockTimeout is returned (wrapped, so errors.Is matches) by
// WithPhaseWriteLock when the bounded wait to acquire the lock expires.
//
// This package cannot format an ADR-0035-shaped recovery line itself:
// internal/guard imports internal/phase (see derive.go's stored-vs-
// derived warning, which is hand-formatted for the identical reason), so
// the error returned here is a plain, unformatted error — callers that
// surface a lock timeout to an operator (e.g. `mindspec repair phase`,
// already routed through guard.NewFailure for every other failure mode
// on that path) wrap it with their own recovery line naming a plain
// re-run.
var ErrPhaseLockTimeout = errors.New("timed out waiting for the phase write lock")

// MergeMetadataFunc is phase.MergeMetadataFunc's type (defined in
// migrate.go) — the shape of every mindspec_phase writer's own
// bead.MergeMetadata seam. WithPhaseWriteLock takes the CALLER's seam as
// a parameter (rather than hardcoding bead.MergeMetadata) so every
// existing per-package fault-injection test (planMergeMetadataFn,
// completeMergeMetadataFn, implPhaseMetadataFn, adoptMergeMetadataFn,
// repairMergeMetadataFn, migrate.go's own mergeMetadataFn) keeps working
// completely unchanged — only the CALL SITE moves inside the lock's
// critical section; the seam being called does not change.

// WithPhaseWriteLock acquires the repo-scoped phase write lock rooted at
// root, reads epicID's CURRENT stored mindspec_phase fresh (via a Cache
// constructed for this call alone — never a Cache shared across the
// invocation, so the value fn sees can never be a snapshot taken before
// lock acquisition), and calls fn with that stored value.
//
// fn decides what to write:
//   - a non-nil `updates` map performs a merge-write via mergeMetadata
//     (the caller's own MergeMetadata seam) before the lock releases;
//   - a nil `updates` map with a nil error is a no-op (fn decided nothing
//     needs writing — e.g. "already consistent", or a refused backward
//     move);
//   - a non-nil error aborts with NO write.
//
// The lock is released (via defer) whether fn errors, no-ops, or writes,
// and mergeMetadata's own error (if the write is attempted and fails) is
// returned to the caller.
func WithPhaseWriteLock(root, epicID string, mergeMetadata MergeMetadataFunc, fn func(stored string) (updates map[string]interface{}, err error)) error {
	unlock, err := acquirePhaseLock(root)
	if err != nil {
		return err
	}
	defer unlock()

	stored := readStoredPhaseWithCache(NewCache(), epicID)
	updates, err := fn(stored)
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
