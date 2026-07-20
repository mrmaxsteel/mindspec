//go:build windows

// disposition_lock_windows.go: spec 117 Bead 2 — the Windows fallback
// OS-visible lock for the R6(b) transactional disposition append.
// Windows has no flock(2), so this REPLICATES internal/journal's
// lock_windows.go idiom (an O_EXCL lockfile with bounded retry) rather
// than importing it: internal/panel is a stdlib+termsafe+idvalidate
// leaf (TestPanelLeafImports_StdlibPlusTermsafeOnly) that may import no
// other internal package. The create succeeds for exactly one process
// at a time, which is the cross-process mutual-exclusion primitive.
package panel

import (
	"fmt"
	"os"
	"time"
)

// acquireDispositionLock takes a cross-process lock by atomically
// creating an O_EXCL lockfile at path (the create succeeds for one
// process at a time). It retries with a short backoff if another
// holder has the lockfile, and returns an unlock func that removes it.
// A bounded total wait prevents an indefinite hang if a crashed holder
// left a stale lockfile; on timeout it returns an error rather than
// silently proceeding (fail-closed).
func acquireDispositionLock(path string) (func(), error) {
	const (
		retry   = 5 * time.Millisecond
		maxWait = 30 * time.Second
	)
	deadline := time.Now().Add(maxWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, dispositionsFileMode)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("panel: create exclusive disposition lockfile %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("panel: timed out waiting for disposition lock (stale %s?)", path)
		}
		time.Sleep(retry)
	}
}
