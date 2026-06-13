//go:build windows

package journal

// lock_windows.go — the Windows fallback OS-visible lock for the reports.jsonl
// read-modify-write. Windows has no flock(2), so we use an O_EXCL lockfile with
// bounded retry: O_CREATE|O_EXCL fails if the lockfile already exists, which is
// the cross-process mutual-exclusion primitive (the create succeeds for exactly
// one process at a time). We retry with a short backoff until the holder
// releases (removes) the lockfile, then remove it on unlock. NTFS
// reparse-point handling is out of scope here, as it is in safeio.

import (
	"fmt"
	"os"
	"time"
)

// acquireFileLock takes a cross-process lock by atomically creating an O_EXCL
// lockfile (the create succeeds for one process at a time). It retries with a
// short backoff if another process holds it, and returns an unlock func that
// removes the lockfile. A bounded total wait prevents an indefinite hang if a
// crashed holder left a stale lockfile; on timeout it returns an error rather
// than silently proceeding (fail-closed).
func acquireFileLock(path string) (func(), error) {
	const (
		retry   = 5 * time.Millisecond
		maxWait = 30 * time.Second
	)
	deadline := time.Now().Add(maxWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, fileMode)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("create exclusive lockfile: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for reports lock (stale %s?)", path)
		}
		time.Sleep(retry)
	}
}
