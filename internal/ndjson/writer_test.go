package ndjson

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type sample struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestEmitThenRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 4096})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	want := []sample{{1, "a"}, {2, "b"}, {3, "c"}}
	for _, s := range want {
		if err := w.Emit(s); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		var got sample
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}
		if got != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestUnbufferedEmitIsDurable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 0})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if err := w.Emit(sample{1, "a"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// File should already contain the line, even before Close, because the
	// writer is unbuffered. This is the contract recording markers relies on.
	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("got %d lines before Close, want 1 (unbuffered Emit must hit FD)", len(lines))
	}
	var got sample
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got != (sample{1, "a"}) {
		t.Errorf("got %+v, want {1 a}", got)
	}
}

func TestBufferedNotVisibleUntilFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 64 << 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := w.Emit(sample{1, "a"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// Before flush, file should be empty (buffered, small payload).
	if lines := readLines(t, path); len(lines) != 0 {
		t.Fatalf("before Flush: got %d lines, want 0 (still buffered)", len(lines))
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if lines := readLines(t, path); len(lines) != 1 {
		t.Fatalf("after Flush: got %d lines, want 1", len(lines))
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCloseFlushes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 64 << 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := w.Emit(sample{i, "x"}); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if lines := readLines(t, path); len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
}

func TestPeriodicFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 1 << 20, FlushInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if err := w.Emit(sample{1, "a"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// Poll for visibility up to 1s — sturdier than a fixed sleep on slow CI.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if lines := readLines(t, path); len(lines) >= 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("periodic flush did not surface event within 1s")
}

func TestConcurrentEmit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 64 << 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const goroutines = 8
	const perG = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				if err := w.Emit(sample{g*perG + i, "x"}); err != nil {
					t.Errorf("Emit: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != goroutines*perG {
		t.Fatalf("got %d lines, want %d", len(lines), goroutines*perG)
	}
	for i, line := range lines {
		var got sample
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d torn or invalid: %q: %v", i, line, err)
		}
	}
}

func TestAppendMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")

	w1, err := New(path, Opts{})
	if err != nil {
		t.Fatalf("New 1: %v", err)
	}
	if err := w1.Emit(sample{1, "a"}); err != nil {
		t.Fatalf("Emit 1: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}

	w2, err := New(path, Opts{Append: true})
	if err != nil {
		t.Fatalf("New 2: %v", err)
	}
	if err := w2.Emit(sample{2, "b"}); err != nil {
		t.Fatalf("Emit 2: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close 2: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	var s1, s2 sample
	if err := json.Unmarshal([]byte(lines[0]), &s1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &s2); err != nil {
		t.Fatal(err)
	}
	if s1 != (sample{1, "a"}) || s2 != (sample{2, "b"}) {
		t.Errorf("got %+v, %+v; want {1 a}, {2 b}", s1, s2)
	}
}

func TestTruncateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")

	w1, err := New(path, Opts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := w1.Emit(sample{1, "a"}); err != nil {
		t.Fatal(err)
	}
	if err := w1.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen without Append → should truncate.
	w2, err := New(path, Opts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := w2.Emit(sample{2, "b"}); err != nil {
		t.Fatal(err)
	}
	if err := w2.Close(); err != nil {
		t.Fatal(err)
	}

	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (truncate should drop prior write)", len(lines))
	}
}

func TestCloseIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close should be a no-op, got: %v", err)
	}
}

func TestEmitAfterCloseReturnsErrClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	err = w.Emit(sample{1, "a"})
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("Emit after Close: got %v, want ErrClosed", err)
	}
}

func TestFlushAfterCloseReturnsErrClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Flush after Close: got %v, want ErrClosed", err)
	}
}

func TestNewOpenError(t *testing.T) {
	// Opening a directory for write should fail.
	dir := t.TempDir()
	if _, err := New(dir, Opts{}); err == nil {
		t.Fatal("expected error opening directory as file")
	}
}

func TestPeriodicFlushStopsOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := New(path, Opts{BufSize: 1 << 20, FlushInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Emit(sample{1, "a"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// If the flush goroutine did not stop, the race detector or a panic on
	// the closed file would surface. Sleeping briefly here ensures any rogue
	// tick after Close would have a chance to misbehave.
	time.Sleep(20 * time.Millisecond)
}
