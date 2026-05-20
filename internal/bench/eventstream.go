package bench

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mrmaxsteel/agentmind/client"
	"github.com/mrmaxsteel/agentmind/wire"

	"github.com/mrmaxsteel/mindspec/internal/ndjson"
)

// StreamConsumer is the bench-side live consumer of the agentmind
// subprocess's stdout NDJSON event stream.
//
// Spec 083 Bead 3b ("load-bearing read-side rewire") replaces the
// previous file-tail-after-the-fact path with a live stdout-pipe
// consumer: AutoStart returns a Handle whose Stdout io.Reader IS the
// subprocess's `exec.Cmd.StdoutPipe()`, and StreamConsumer.Run reads
// that pipe via `client.ReadEvents` (Hard Constraint #3 — outbound
// channel is stdout-pipe NDJSON, NOT file-tail).
//
// The events are written to an NDJSON file at `outputPath` in the
// same canonical line format so post-run aggregations
// (`ParseSessionByLabel`, `ExtractSessionIDs`, `countEventsByLabel`)
// keep working unchanged against the on-disk file.
type StreamConsumer struct {
	// Reader is the io.Reader to consume. In production this MUST be
	// `Handle.Stdout` (a subprocess stdout pipe from
	// `exec.Cmd.StdoutPipe()`); tests substitute synthetic readers.
	Reader io.Reader
	// OutputPath is the NDJSON file the consumer writes received
	// events into. Existing file-based aggregations
	// (`ParseSessionByLabel` etc.) continue to read this path.
	OutputPath string

	once     sync.Once
	doneCh   chan struct{}
	startErr error
	writer   *ndjson.Writer
}

// Run starts a goroutine that reads NDJSON `wire.CollectedEvent`
// records from c.Reader via `client.ReadEvents` and writes them as
// canonical NDJSON to c.OutputPath. Run is non-blocking and idempotent
// — calling Run twice is a no-op after the first invocation.
//
// Returns nil if the goroutine launched successfully; a non-nil error
// only when the output file could not be opened. The goroutine exits
// when the upstream channel closes (subprocess exited or pipe closed).
func (c *StreamConsumer) Run() error {
	c.once.Do(func() {
		c.doneCh = make(chan struct{})
		w, err := ndjson.New(c.OutputPath, ndjson.Opts{
			BufSize:       64 << 10,
			FlushInterval: 500 * time.Millisecond,
		})
		if err != nil {
			c.startErr = fmt.Errorf("open %s: %w", c.OutputPath, err)
			close(c.doneCh)
			return
		}
		c.writer = w
		// IMPORTANT: c.Reader MUST be a subprocess stdout pipe (e.g.
		// the Stdout field of a client.Handle returned by AutoStart).
		// Spec 083 Hard Constraint #3 prohibits file-tailing the
		// agentmind `--output` file path from any consumer.
		events := client.ReadEvents(c.Reader)
		go func() {
			// CRITICAL: close the writer BEFORE signaling Done.
			// `Done()` is documented as "the consumer goroutine has
			// exited and the output writer is closed". Go defers run
			// LIFO, so the previous form
			//   defer w.Close(); defer close(c.doneCh)
			// closed the doneCh first and the writer second, letting
			// callers blocking on <-Done() observe the file before the
			// final flush had landed. Closing explicitly before the
			// doneCh close makes the Done signal a true post-flush
			// happens-after edge (panel bead-3b-v1 REV-2).
			defer close(c.doneCh)
			for ev := range events {
				_ = w.Emit(ev)
			}
			_ = w.Close()
		}()
	})
	return c.startErr
}

// Flush forces a flush of buffered events to disk. Safe to call any
// time before Close. No-op if Run never succeeded.
func (c *StreamConsumer) Flush() error {
	if c.writer == nil {
		return nil
	}
	return c.writer.Flush()
}

// Done returns a channel that closes when the consumer goroutine
// exits (upstream channel closed, output writer closed). Tests use
// this to deterministically wait for the consumer to flush before
// asserting file contents.
func (c *StreamConsumer) Done() <-chan struct{} {
	if c.doneCh == nil {
		// Run was never invoked or returned an early error before
		// allocating; return an already-closed channel so callers
		// don't deadlock.
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return c.doneCh
}

// ConsumeHandleToFile wires a `client.Handle` returned by
// `client.AutoStart` to an NDJSON file at outputPath via
// `client.ReadEvents`. It is the production entry point used by
// `runner.go` (via `startBenchCollector`) for the bead 3b read-side
// rewire. (Panel bead-3b-v1 REV-5: a parallel session-side helper
// `ConsumeSessionStream` was removed as dead code; the runner-level
// consumer is the only production caller.)
//
// The io.Reader fed into `client.ReadEvents` is `handle.Stdout`
// (a subprocess stdout pipe) — spec 083 Hard Constraint #3.
//
// Returns nil + a no-op consumer when handle is nil or handle.Stdout
// is nil (degraded path or reused-instance path, where no stdout pipe
// exists to read).
func ConsumeHandleToFile(handle *client.Handle, outputPath string) (*StreamConsumer, error) {
	if handle == nil || handle.Stdout == nil {
		return nil, nil
	}
	c := &StreamConsumer{
		Reader:     handle.Stdout, // subprocess stdout pipe — NOT a file handle
		OutputPath: outputPath,
	}
	if err := c.Run(); err != nil {
		return nil, err
	}
	return c, nil
}

// _ is a compile-time assertion that the wire type imported here is
// the same one the consumer channel produces (defensive against
// future refactors that might decouple wire and client).
var _ = wire.CollectedEvent{}
