//go:build !windows

// disposition_lock_unix.go: spec 117 Bead 2 — the unix (darwin/linux)
// OS-visible advisory lock for the R6(b) transactional disposition
// append. This REPLICATES internal/journal's lock_unix.go idiom
// (syscall.Flock LOCK_EX on a dedicated lockfile) rather than importing
// it: internal/panel is a stdlib+termsafe+idvalidate leaf
// (TestPanelLeafImports_StdlibPlusTermsafeOnly, spec 116 AC7 amended by
// spec 120 R2) that may import no other internal package, so the idiom
// is duplicated here by design, not by drift — see
// disposition_store.go's withDispositionLock for the shared caller.
package panel

import (
	"fmt"
	"os"
	"syscall"
)

// acquireDispositionLock opens (creating 0600 if absent) the lockfile at
// path and takes an exclusive, cross-process advisory lock (LOCK_EX,
// BLOCKING until any other holder releases). It returns an unlock func
// that releases the lock and closes the descriptor.
//
// Each call OPENS A FRESH file description via os.OpenFile: flock(2)
// binds the lock to the open file description, not the process or the
// path, so two separate descriptors of the SAME lockfile path contend
// even within one process. The T1/T2/T3 in-process concurrency proofs
// (disposition_store_test.go) and the T4 cross-process proof both
// depend on this — a goroutine-local sync.Mutex would not serialize
// across processes, and a single shared *os.File would not exercise the
// real cross-process contention path.
func acquireDispositionLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, dispositionsFileMode)
	if err != nil {
		return nil, fmt.Errorf("panel: open disposition lockfile %s: %w", path, err)
	}
	// Re-assert 0600 in case the lockfile pre-existed under a permissive
	// umask (mirrors internal/journal/lock_unix.go).
	_ = f.Chmod(dispositionsFileMode)
	// os.File.Fd() returns a uintptr that is, in practice, a small
	// non-negative process file descriptor number (never anywhere near
	// the int/uintptr width boundary); syscall.Flock's signature requires
	// an int. This is the SAME pre-existing, pre-Bead-117 conversion
	// internal/journal/lock_unix.go carries (bead mindspec-8ud6, tracked,
	// P3) — replicated here per this bead's mandate to mirror that exact
	// idiom, not a new risk.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil { //nolint:gosec // G115: fd is a small process-local descriptor, see comment above
		_ = f.Close()
		return nil, fmt.Errorf("panel: flock LOCK_EX on %s: %w", path, err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // G115: fd is a small process-local descriptor, see acquireDispositionLock's doc comment
		_ = f.Close()
	}, nil
}
