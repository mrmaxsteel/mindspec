package journal

// lock.go — spec 094 Bead 3 (codex-consolidation #1/#2): a cross-process
// advisory file lock for the reports.jsonl read-modify-write.
//
// reports.jsonl is the CONSOLIDATED VIEW and is rewritten WHOLESALE (temp +
// rename), unlike the append-only journal whose O_APPEND gives byte-atomic
// concurrency (cdk8.2). A wholesale rewrite has a cross-process TOCTOU: two
// processes that each read a snapshot, modify it, and rename can lose one
// another's update (a stale `report` consolidator clobbering a concurrent
// `report list --resolve`, or dropping an on-disk-only row added after its
// snapshot). The in-process sync.Mutex does NOT span processes, so the FULL
// read-modify-write must be serialized by an OS-VISIBLE lock.
//
// The lock is held on a dedicated lockfile (reports.lock) under the SAME
// isolated 0600 store dir, NEVER on reports.jsonl itself (so the temp+rename
// that replaces reports.jsonl cannot disturb the held descriptor). The
// platform-specific acquire/release live in lock_unix.go (syscall.Flock
// LOCK_EX advisory lock — the same OS-visible idiom safeio uses for its
// platform split) and lock_windows.go (an O_EXCL lockfile-with-retry, since
// Windows has no flock; NTFS reparse handling is out of scope here as it is in
// safeio).

import (
	"fmt"
	"os"
	"path/filepath"
)

// reportsLockName is the dedicated lockfile under Dir() that serializes the
// reports.jsonl read-modify-write across processes. It is NOT reports.jsonl
// itself, so the wholesale temp+rename cannot invalidate the held lock.
const reportsLockName = "reports.lock"

// reportsLockPath returns the absolute path to the reports.jsonl lockfile under
// Dir(). It also ensures the store dir exists (0700) so the lockfile can be
// created before reports.jsonl is first written.
func reportsLockPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, reportsLockName), nil
}

// withReportsLock runs fn while holding the OS-visible cross-process lock on the
// reports.jsonl lockfile. The lock is held for the ENTIRE read-modify-write
// (read journal + read reports.jsonl resolved-state + consolidate/merge +
// temp+rename) and released after fn returns, so no concurrent process can
// observe a partial state or clobber the rename.
//
// It does NOT take the in-process sync.Mutex (mu): fn (e.g. MarkResolved's
// Consolidate) re-enters mu-guarded reads like ReadEvents, and mu is
// non-reentrant. The OS file lock serializes BOTH cross-process AND
// same-process holders — flock(2) on unix associates the lock with the open
// file description, so two separate OpenFile descriptors of the same lockfile
// (even within one process) contend; the windows O_EXCL lockfile likewise
// admits exactly one holder at a time — so a separate intra-process mutex is
// unnecessary. The per-file-op reads/writes fn invokes remain individually
// mu-guarded for their own atomicity.
func withReportsLock(fn func() error) error {
	path, err := reportsLockPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("journal: create state dir: %w", err)
	}

	unlock, err := acquireFileLock(path)
	if err != nil {
		return fmt.Errorf("journal: acquire reports lock: %w", err)
	}
	defer unlock()

	return fn()
}
