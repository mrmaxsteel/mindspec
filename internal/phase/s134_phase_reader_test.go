package phase

import (
	"errors"
	"testing"
	"time"
)

func TestS134Next_PhaseReaderUnderLock(t *testing.T) {
	t.Cleanup(SetRunBDForTest(func(...string) ([]byte, error) {
		t.Error("scoped reader fell through to default store")
		return nil, errors.New("unexpected default read")
	}))
	t.Cleanup(SetListJSONForTest(func(...string) ([]byte, error) {
		t.Error("scoped reader fell through to default list")
		return nil, errors.New("unexpected default list")
	}))
	t.Cleanup(SetLockWaitForTest(20*time.Millisecond, time.Millisecond))
	readFailure := errors.New("scoped read failed")
	for _, tc := range []struct {
		name, raw, want string
		present         bool
		err             error
	}{
		{name: "valid", raw: "plan", want: "plan", present: true},
		{name: "invalid-present", raw: "garbage", present: true},
		{name: "absent"},
		{name: "read-error", err: readFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			read, called, merged := false, false, false
			err := WithPhaseWriteLock(root, "test-epic", func(string, map[string]any) error { merged = true; return nil }, func(stored string, present bool) (map[string]any, error) {
				called = true
				if !read || stored != tc.want || present != tc.present {
					t.Fatalf("reader outcome: read=%t stored=%q present=%t", read, stored, present)
				}
				return map[string]any{"mindspec_phase": "implement"}, nil
			}, func() (string, bool, error) {
				read = true
				entered := false
				lockErr := WithPhaseLock(root, func() error { entered = true; return nil })
				if entered || !errors.Is(lockErr, ErrPhaseLockTimeout) {
					t.Fatalf("reader ran outside lock: entered=%t err=%v", entered, lockErr)
				}
				return tc.raw, tc.present, tc.err
			})
			if !errors.Is(err, tc.err) || !read || called != (tc.err == nil) || merged != (tc.err == nil) {
				t.Fatalf("reader failure propagation: err=%v read=%t called=%t merged=%t", err, read, called, merged)
			}
		})
	}
}
