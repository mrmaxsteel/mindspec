package phase

// phaselock_windows_errors_test.go — round-1 panel G7: exercises
// shouldRetryWindowsLockError on every CI platform (no build
// constraint — see phaselock_windows_errors.go's doc comment for why
// this is the seam-driven non-Windows-CI disposition the plan calls
// for).

import "testing"

func TestShouldRetryWindowsLockError(t *testing.T) {
	cases := []struct {
		name string
		code uintptr
		want bool
	}{
		{"ERROR_LOCK_VIOLATION (33) retries", 33, true},
		{"ERROR_ACCESS_DENIED (5) does not retry", 5, false},
		{"ERROR_INVALID_HANDLE (6) does not retry", 6, false},
		{"ERROR_SHARING_VIOLATION (32, a DIFFERENT code — must not be conflated with 33) does not retry", 32, false},
		{"zero (LockFileEx succeeded; classification is never reached in that case, but must not accidentally retry) does not retry", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRetryWindowsLockError(tc.code); got != tc.want {
				t.Errorf("shouldRetryWindowsLockError(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}
