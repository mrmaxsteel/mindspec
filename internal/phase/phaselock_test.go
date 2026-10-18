package phase

// phaselock_test.go — spec 128 Bead 1 (R3.3) tests over the real lock
// primitive: real-goroutine contention under -race, timeout error
// propagation, the barrier-controlled two-contender exclusivity test, a
// leftover-file leg, and the no-blockable-git-dirt assertion.
//
// The AC-5(v) concurrent-writer leg (repair phase vs. a fresh advancing
// write) is covered in cmd/mindspec/repair_test.go, where the guard logic
// under test lives. The AC-5(vii) legacy-migration contention leg is
// covered in migrate_test.go, beside EnsureMigrated's other tests.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestWithPhaseWriteLock_ExclusiveAcrossGoroutines is the barrier-
// controlled two-contender exclusivity test (round-1 panel G6 / round-2
// panel G4.1: deterministic barriers, no sleep-based scheduling, and the
// "attempted acquisition" proof must come from INSIDE the actual
// kernel-level lock-acquisition path — a channel the caller closes
// immediately before invoking WithPhaseWriteLock does NOT prove the
// goroutine was ever scheduled far enough to reach the real syscall
// attempt, so SetAcquireAttemptHookForTest fires from the real
// acquirePhaseFileLock poll loop itself): a bounded negative window
// proves the second contender CANNOT enter before release; a positive
// channel wait proves it DOES enter afterward; both calls' returned
// errors are checked.
func TestWithPhaseWriteLock_ExclusiveAcrossGoroutines(t *testing.T) {
	root := t.TempDir()
	store := newFakeEpicStore("epic-1", nil)
	store.stub(t)

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- WithPhaseWriteLock(root, "epic-1", store.merge, func(string, bool) (map[string]interface{}, error) {
			close(firstEntered)
			<-releaseFirst
			return map[string]interface{}{"mindspec_phase": "plan"}, nil
		})
	}()
	<-firstEntered

	// Install the real acquisition-attempt hook AFTER the first holder is
	// already inside — so only the SECOND contender's attempts fire it.
	secondAttempting := make(chan struct{})
	var attemptOnce sync.Once
	t.Cleanup(SetAcquireAttemptHookForTest(func() {
		attemptOnce.Do(func() { close(secondAttempting) })
	}))

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- WithPhaseWriteLock(root, "epic-1", store.merge, func(string, bool) (map[string]interface{}, error) {
			close(secondEntered)
			return map[string]interface{}{"mindspec_phase": "implement"}, nil
		})
	}()
	// Deterministic: the second contender has genuinely reached the
	// real, kernel-level lock-acquisition syscall attempt — not merely
	// "the goroutine function started running" — before we ever release
	// the first holder.
	select {
	case <-secondAttempting:
	case <-time.After(5 * time.Second):
		t.Fatal("second contender never reached a lock-acquisition attempt")
	}

	// Deterministic negative window: with the first holder still
	// holding, the second contender must NOT have entered — a bounded
	// PROOF window (asserts it did not happen within a generous margin),
	// not a scheduling nudge; the forward-progress claim is proven by
	// the channel waits below, not by this window's duration.
	select {
	case <-secondEntered:
		t.Fatal("second contender entered the critical section before the first released — mutual exclusion violated")
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Errorf("first WithPhaseWriteLock call returned an error: %v", err)
	}

	select {
	case <-secondEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("second contender never entered after the first released")
	}
	if err := <-secondDone; err != nil {
		t.Errorf("second WithPhaseWriteLock call returned an error: %v", err)
	}
}

// TestPhaseLock_CrashedHolderReleases_Unix is the plan step-2 (round-1
// panel G4/R1/R2) crashed-holder recovery leg: a REAL subprocess
// acquires the lock at a fixture path and is SIGKILLed while holding it;
// the parent's next acquirePhaseFileLock must succeed promptly (well
// under phaseLockWait) — flock is kernel-held, so a killed holder's lock
// releases with its process: no PID/timestamp content, no takeover, no
// deletion step is ever needed.
//
// The plan's Windows-leg disposition is seam-driven on non-Windows CI
// (this repo's CI has no Windows runner —.github/workflows/ci.yml runs
// linux/darwin only): shouldRetryWindowsLockError's classification
// (round-1 panel G7) is tested directly, on every platform, in
// phaselock_windows_errors_test.go. A real Windows subprocess
// crashed-holder test analogous to this one is not exercised here —
// LockFileEx's crashed-holder-releases behavior is documented Win32
// kernel semantics (a handle's byte-range lock is released when the
// process exits, same guarantee this test proves for flock), not this
// package's own code, so the residual is a platform-semantics claim, not
// an untested code path.
func TestPhaseLock_CrashedHolderReleases_Unix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix leg — flock kernel-held-lock semantics; see the doc comment for the Windows disposition")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "phase.lock")
	heldMarker := filepath.Join(dir, "held")

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess_AcquirePhaseLockAndHang$")
	cmd.Env = append(os.Environ(),
		"MINDSPEC_PHASELOCK_HELPER=1",
		"MINDSPEC_PHASELOCK_HELPER_PATH="+path,
		"MINDSPEC_PHASELOCK_HELPER_MARKER="+heldMarker,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting helper process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(heldMarker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper process never signaled it held the lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("killing helper process: %v", err)
	}
	_ = cmd.Wait()

	start := time.Now()
	unlock, err := acquirePhaseFileLock(path, 2*time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("acquiring after crashed holder: %v", err)
	}
	defer unlock()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("acquisition after crashed holder took %v, want near-instant (flock releases with the killed process)", elapsed)
	}
}

// TestHelperProcess_AcquirePhaseLockAndHang is invoked as a SUBPROCESS by
// TestPhaseLock_CrashedHolderReleases_Unix — it is not a real test.
// Guarded by the MINDSPEC_PHASELOCK_HELPER env var so a normal `go test`
// run (which never sets it) just skips it immediately.
func TestHelperProcess_AcquirePhaseLockAndHang(t *testing.T) {
	if os.Getenv("MINDSPEC_PHASELOCK_HELPER") != "1" {
		t.Skip("not invoked as a phaselock crashed-holder helper process")
	}
	path := os.Getenv("MINDSPEC_PHASELOCK_HELPER_PATH")
	marker := os.Getenv("MINDSPEC_PHASELOCK_HELPER_MARKER")
	unlock, err := acquirePhaseFileLock(path, 5*time.Second, 10*time.Millisecond)
	if err != nil {
		os.Exit(1)
	}
	defer unlock()
	if err := os.WriteFile(marker, []byte(""), 0o600); err != nil {
		os.Exit(1)
	}
	// Block "forever" (a sleep, not select{} — an empty select in the
	// sole goroutine trips Go's own deadlock detector and would
	// self-terminate the process instead of genuinely hanging until the
	// parent's SIGKILL).
	time.Sleep(time.Hour)
}

// TestAcquirePhaseFileLock_TimesOutWithoutDeleteInstruction pins the
// bounded-wait timeout contract: acquisition against an already-held
// lock returns promptly at the deadline, wraps ErrPhaseLockTimeout, names
// the lock path, and NEVER suggests deleting the lock file.
func TestAcquirePhaseFileLock_TimesOutWithoutDeleteInstruction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "phase.lock")

	unlock, err := acquirePhaseFileLock(path, time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer unlock()

	start := time.Now()
	_, err = acquirePhaseFileLock(path, 200*time.Millisecond, 10*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a timeout error while the lock is held")
	}
	if !errors.Is(err, ErrPhaseLockTimeout) {
		t.Errorf("error does not wrap ErrPhaseLockTimeout: %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("timeout error must name the lock path %s: %v", path, err)
	}
	// Check the message body only, not the embedded lock path — t.TempDir()
	// derives its path from this test's OWN name, which itself contains
	// "Delete" (TimesOutWithoutDeleteInstruction), so a substring check
	// against the full error text would false-positive on the path.
	body := strings.ToLower(strings.Replace(err.Error(), path, "<path>", 1))
	for _, forbidden := range []string{"delete", "remove", "rm -"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("timeout error must never advise deleting/removing the lock file: %v", err)
		}
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("timeout fired too early: %v (wanted ~200ms)", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Errorf("timeout took too long: %v (wanted ~200ms)", elapsed)
	}
}

// TestDescribeLockFailure is round-1 panel G3: a best-effort caller
// (release.go's/complete.go's step-6.5 sync) must surface, not discard,
// a WithPhaseWriteLock failure — DescribeLockFailure is the shared
// renderer both use. A lock TIMEOUT gets the full ADR-0035 treatment
// (names the lock path, ends with a `recovery: ` line that never
// advises deletion); any other error (a genuine merge-write failure) is
// rendered as-is with no manufactured recovery line.
func TestDescribeLockFailure(t *testing.T) {
	const recoveryCommand = "mindspec release mindspec-abc123"

	t.Run("nil error renders empty", func(t *testing.T) {
		if got := DescribeLockFailure(nil, recoveryCommand); got != "" {
			t.Errorf("DescribeLockFailure(nil, ...) = %q, want empty", got)
		}
	})

	t.Run("lock timeout names the path and ends with the caller's recovery COMMAND", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, PhaseLockRelPath)
		timeoutErr := fmt.Errorf("phase: acquire write lock at %s: %w", path, ErrPhaseLockTimeout)

		got := DescribeLockFailure(timeoutErr, recoveryCommand)
		if !strings.Contains(got, PhaseLockRelPath) {
			t.Errorf("must name the lock path %s: %q", PhaseLockRelPath, got)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		last := lines[len(lines)-1]
		// Round-2 panel G3: the final line must be an actual copy-pastable
		// COMMAND (the caller's own recoveryCommand argument), not prose.
		if last != "recovery: "+recoveryCommand {
			t.Errorf("last line = %q, want %q", last, "recovery: "+recoveryCommand)
		}
		if strings.Contains(strings.ToLower(got), "delete") || strings.Contains(strings.ToLower(got), "remove") {
			t.Errorf("must never advise deleting/removing the lock file: %q", got)
		}
	})

	t.Run("a non-timeout error is rendered as-is with no manufactured recovery line", func(t *testing.T) {
		plain := errors.New("bd metadata merge-read failed for epic-1: connection refused")
		got := DescribeLockFailure(plain, recoveryCommand)
		if !strings.Contains(got, plain.Error()) {
			t.Errorf("must contain the underlying error text: %q", got)
		}
		if strings.Contains(got, "recovery: ") {
			t.Errorf("a plain (non-timeout) failure must not get a manufactured recovery line: %q", got)
		}
	})
}

// TestAcquirePhaseFileLock_LeftoverEmptyFileDoesNotBlock is the
// leftover-file leg: a pre-existing empty/partial lock file must never
// block acquisition — the FILE is not the lock (the OS-level lock is).
func TestAcquirePhaseFileLock_LeftoverEmptyFileDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "phase.lock")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("seeding leftover empty lockfile: %v", err)
	}

	start := time.Now()
	unlock, err := acquirePhaseFileLock(path, time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("acquisition against a leftover empty lockfile must succeed: %v", err)
	}
	defer unlock()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("acquisition against a leftover empty lockfile took %v, want near-instant", elapsed)
	}

	// A partial (non-empty, garbage-content) leftover file must not block
	// either.
	dir2 := t.TempDir()
	path2 := filepath.Join(dir2, "phase.lock")
	if err := os.WriteFile(path2, []byte("stale-pid-or-garbage-content"), 0o600); err != nil {
		t.Fatalf("seeding leftover partial lockfile: %v", err)
	}
	unlock2, err := acquirePhaseFileLock(path2, time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("acquisition against a leftover partial lockfile must succeed: %v", err)
	}
	unlock2()
}

// TestWithPhaseWriteLock_NoBlockableGitDirt asserts that acquiring and
// releasing the phase lock leaves ZERO blockable dirt in a git worktree —
// the lock path must be gitignored (this bead's .gitignore addition) and
// the lock's own bytes carry no content that would surface as an
// untracked/modified file.
func TestWithPhaseWriteLock_NoBlockableGitDirt(t *testing.T) {
	root := t.TempDir()
	run := func(name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "test")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o700); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	// The managed .gitignore block this bead adds (mirrors the real
	// project .gitignore's entry for .mindspec/phase.lock).
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(PhaseLockRelPath+"\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	run("git", "add", ".gitignore")
	run("git", "commit", "-m", "init")

	store := newFakeEpicStore("epic-1", nil)
	store.stub(t)
	if err := WithPhaseWriteLock(root, "epic-1", store.merge, func(string, bool) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_phase": "plan"}, nil
	}); err != nil {
		t.Fatalf("WithPhaseWriteLock: %v", err)
	}

	status := run("git", "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Errorf("git status --porcelain not clean after phase lock use:\n%s", status)
	}
}

// fakeEpicStore is a thread-safe in-memory epic metadata store shared by
// the phaselock/migrate contention tests: its merge method is passed
// directly as WithPhaseWriteLock's mergeMetadata parameter, and its
// snapshot feeds SetRunBDForTest/SetListJSONForTest so a fresh
// readStoredPhaseWithCache(NewCache(), epicID) call always observes the
// LATEST merged state, exactly like a real bd process would.
type fakeEpicStore struct {
	mu       sync.Mutex
	epicID   string
	metadata map[string]interface{}
}

func newFakeEpicStore(epicID string, metadata map[string]interface{}) *fakeEpicStore {
	m := map[string]interface{}{}
	for k, v := range metadata {
		m[k] = v
	}
	return &fakeEpicStore{epicID: epicID, metadata: m}
}

func (s *fakeEpicStore) merge(id string, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range updates {
		s.metadata[k] = v
	}
	return nil
}

func (s *fakeEpicStore) get(key string) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metadata[key]
}

// epicJSON marshals the current metadata as a single bd-shaped epic JSON
// object (not wrapped in an array — callers wrap per bd command shape).
func (s *fakeEpicStore) epicJSON() []byte {
	s.mu.Lock()
	meta := make(map[string]interface{}, len(s.metadata))
	for k, v := range s.metadata {
		meta[k] = v
	}
	epicID := s.epicID
	s.mu.Unlock()
	b, _ := json.Marshal(EpicInfo{ID: epicID, Title: "fixture", Status: "open", IssueType: "epic", Metadata: meta})
	return b
}

// stub wires SetRunBDForTest/SetListJSONForTest so every phase-package
// read (readStoredPhaseWithCache, FindEpic, ...) observes this store's
// CURRENT metadata, restoring both on t.Cleanup.
func (s *fakeEpicStore) stub(t *testing.T) {
	t.Helper()
	t.Cleanup(SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[" + string(s.epicJSON()) + "]"), nil
	}))
	t.Cleanup(SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[" + string(s.epicJSON()) + "]"), nil
	}))
}
