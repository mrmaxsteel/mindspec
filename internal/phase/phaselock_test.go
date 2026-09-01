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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestWithPhaseWriteLock_ExclusiveAcrossGoroutines is the barrier-
// controlled two-contender exclusivity test: exactly one goroutine is
// ever inside the critical section at a time, and the second (loser)
// blocks until the first releases.
func TestWithPhaseWriteLock_ExclusiveAcrossGoroutines(t *testing.T) {
	root := t.TempDir()
	store := newFakeEpicStore("epic-1", nil)
	store.stub(t)

	var mu sync.Mutex
	var insideCount, maxInside int
	enter := func() {
		mu.Lock()
		insideCount++
		if insideCount > maxInside {
			maxInside = insideCount
		}
		mu.Unlock()
	}
	leave := func() {
		mu.Lock()
		insideCount--
		mu.Unlock()
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = WithPhaseWriteLock(root, "epic-1", store.merge, func(string) (map[string]interface{}, error) {
			enter()
			close(firstEntered)
			<-releaseFirst
			leave()
			return map[string]interface{}{"mindspec_phase": "plan"}, nil
		})
	}()

	<-firstEntered
	// Give the second goroutine a generous head start to reach the lock
	// and block on it before we release the first holder.
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		close(releaseFirst)
	}()

	secondDone := make(chan struct{})
	go func() {
		_ = WithPhaseWriteLock(root, "epic-1", store.merge, func(string) (map[string]interface{}, error) {
			enter()
			leave()
			return map[string]interface{}{"mindspec_phase": "implement"}, nil
		})
		close(secondDone)
	}()

	wg.Wait()
	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("second WithPhaseWriteLock call never completed")
	}

	if maxInside > 1 {
		t.Errorf("maxInside = %d, want 1 — both goroutines were inside the critical section simultaneously", maxInside)
	}
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
	if err := WithPhaseWriteLock(root, "epic-1", store.merge, func(string) (map[string]interface{}, error) {
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
