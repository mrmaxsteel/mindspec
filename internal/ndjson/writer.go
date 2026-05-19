// Package ndjson provides a goroutine-safe NDJSON writer over a single file.
//
// One JSON object per line, '\n'-terminated. Callers MUST call Close to flush
// any buffered bytes and release the file descriptor. Close is the durability
// boundary: with the synchronous mode (Opts.BufSize == 0) each Emit performs a
// direct write to the file descriptor, but durable visibility across process
// exit still requires Close (or process-level fsync). Callers needing per-call
// durability should `defer w.Close()` in the same scope as ndjson.New.
package ndjson

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// ErrClosed is returned by Emit and Flush when the Writer has already been closed.
var ErrClosed = errors.New("ndjson: writer is closed")

// Opts configures a Writer.
type Opts struct {
	// Append opens the file with O_APPEND. Default false (creates or truncates).
	Append bool

	// BufSize is the bufio.Writer buffer size in bytes. Zero means unbuffered:
	// each Emit writes directly to the file descriptor before returning.
	// Negative values are treated as zero. Recommended: 64 << 10 for
	// high-throughput collectors.
	BufSize int

	// FlushInterval, if > 0, starts a background goroutine that flushes the
	// buffer every FlushInterval. Ignored when BufSize == 0. The goroutine
	// exits on Close. Zero disables periodic flush.
	FlushInterval time.Duration

	// FileMode for newly-created files. Defaults to 0644 when zero.
	FileMode os.FileMode
}

// Writer is a goroutine-safe NDJSON writer over a single *os.File.
//
// Marshaling happens outside the writer's mutex; only the line write itself
// is serialized. Close, Emit, and Flush are safe to call from any goroutine.
type Writer struct {
	mu     sync.Mutex
	f      *os.File
	bw     *bufio.Writer // nil if unbuffered (Opts.BufSize == 0)
	closed bool
	stop   chan struct{} // closed in Close to stop flusher; nil if no flusher
	done   chan struct{} // signaled by flushLoop on exit
}

// New opens path and returns a Writer. The file is created if it does not exist.
// The caller MUST call Close to flush any buffered bytes and release the FD.
func New(path string, opts Opts) (*Writer, error) {
	flag := os.O_CREATE | os.O_WRONLY
	if opts.Append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	mode := opts.FileMode
	if mode == 0 {
		mode = 0644
	}
	f, err := os.OpenFile(path, flag, mode)
	if err != nil {
		return nil, fmt.Errorf("ndjson: open %s: %w", path, err)
	}
	// Belt-and-suspenders: O_CREATE only applies mode on creation. If the file
	// pre-existed (e.g., another writer got there first with a looser umask),
	// chmod via the open fd to guarantee the requested mode regardless of who
	// created the file. Only enforced when FileMode was explicitly set, since
	// the default 0644 should not be forced over an existing 0600 file.
	if opts.FileMode != 0 {
		if err := f.Chmod(opts.FileMode); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("ndjson: chmod %s: %w", path, err)
		}
	}
	w := &Writer{f: f}
	if opts.BufSize > 0 {
		w.bw = bufio.NewWriterSize(f, opts.BufSize)
		if opts.FlushInterval > 0 {
			w.stop = make(chan struct{})
			w.done = make(chan struct{})
			go w.flushLoop(opts.FlushInterval)
		}
	}
	return w, nil
}

// Emit marshals v as JSON and writes it as one NDJSON line.
//
// When the writer is unbuffered (Opts.BufSize == 0), Emit writes directly to
// the file descriptor (no bufio layer) before returning. Note that this does
// not call fsync — durability across process exit still requires Close (or a
// process-level fsync). Callers needing per-call durability should defer Close
// in the same scope as New so that Close runs before the function returns.
//
// After Close, Emit returns ErrClosed and the value is not written.
func (w *Writer) Emit(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("ndjson: marshal: %w", err)
	}
	data = append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrClosed
	}
	if w.bw != nil {
		_, err = w.bw.Write(data)
	} else {
		_, err = w.f.Write(data)
	}
	if err != nil {
		return fmt.Errorf("ndjson: write: %w", err)
	}
	return nil
}

// Flush forces a flush of the buffer to the OS. No-op if unbuffered.
// Returns ErrClosed if the writer has been closed.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrClosed
	}
	return w.flushLocked()
}

func (w *Writer) flushLocked() error {
	if w.bw == nil {
		return nil
	}
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("ndjson: flush: %w", err)
	}
	return nil
}

// Close flushes any buffered bytes, stops the background flusher (if any),
// and closes the underlying file. Idempotent: subsequent calls return nil.
func (w *Writer) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	stop := w.stop
	done := w.done
	flushErr := w.flushLocked()
	closeErr := w.f.Close()
	w.mu.Unlock()

	// Stop the flush goroutine after marking closed so any in-flight tick
	// either sees closed==true or completes before we drop our reference.
	if stop != nil {
		close(stop)
		<-done
	}

	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

// flushLoop runs in its own goroutine, flushing the buffer at d intervals
// until Close signals stop.
func (w *Writer) flushLoop(d time.Duration) {
	defer close(w.done)
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			w.mu.Lock()
			if w.closed {
				w.mu.Unlock()
				return
			}
			_ = w.flushLocked()
			w.mu.Unlock()
		}
	}
}
