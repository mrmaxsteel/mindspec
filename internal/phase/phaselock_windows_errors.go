package phase

// phaselock_windows_errors.go — round-1 panel G7: the Win32 error
// classification LockFileEx's synchronous (LOCKFILE_FAIL_IMMEDIATELY)
// retry loop needs, kept in a file WITHOUT a windows build constraint so
// it is testable from every CI platform even though it is only EXERCISED
// by the Windows-only lock acquisition path (phaselock_windows.go,
// //go:build windows). This repo's CI has no Windows runner
// (.github/workflows/ci.yml runs only linux/darwin), so this
// classification-only extraction — never a real Windows subprocess test
// — is the "seam-driven on non-Windows CI" disposition plan.md's Bead 1
// step 2 calls for on this leg.

// errorLockViolation is the Win32 error code ERROR_LOCK_VIOLATION (33).
// LockFileEx's synchronous LOCKFILE_FAIL_IMMEDIATELY path returns this
// SPECIFIC code when the requested byte range is presently locked by
// another process/handle — the only error the poll-with-deadline retry
// loop should treat as "try again". Any OTHER LockFileEx failure
// (invalid handle, access denied, ...) is a real, non-transient failure
// that must surface immediately — silently retrying it until the
// deadline would misreport a genuine failure as a lock TIMEOUT
// (ErrPhaseLockTimeout), hiding the real cause and inviting exactly the
// "just re-run it" advice that is wrong for a non-transient error.
const errorLockViolation = 33

// shouldRetryWindowsLockError reports whether a Win32 error code
// returned by LockFileEx's synchronous call means "the lock is presently
// held elsewhere — try again" (true only for ERROR_LOCK_VIOLATION) as
// opposed to a genuine, non-transient failure that must surface
// immediately.
func shouldRetryWindowsLockError(code uintptr) bool {
	return code == errorLockViolation
}
