package trace

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// globalRunID is set once per Init() call, shared across all events in this invocation.
var globalRunID string

// global is the active tracer. Defaults to no-op.
var global Tracer = noopTracer{}

// Tracer emits structured trace events.
type Tracer interface {
	Emit(e Event)
	Close() error
}

// Emit sends an event to the global tracer.
func Emit(e Event) {
	global.Emit(e)
}

// Close flushes and closes the global tracer.
func Close() error {
	return global.Close()
}

// Init activates tracing to the given path. Use "-" for stderr.
// Generates a new run ID for this invocation.
func Init(path string) error {
	globalRunID = generateRunID()

	var w io.WriteCloser
	if path == "-" {
		w = nopCloser{os.Stderr}
	} else {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("trace init: %w", err)
		}
		w = f
	}

	global = &ndjsonTracer{w: w}
	return nil
}

// SetGlobal replaces the global tracer. Returns the previous one.
// Intended for testing.
func SetGlobal(t Tracer) Tracer {
	prev := global
	global = t
	return prev
}

// SetRunID overrides the global run ID. Intended for testing.
func SetRunID(id string) {
	globalRunID = id
}

// ndjsonTracer writes one JSON line per event.
type ndjsonTracer struct {
	mu sync.Mutex
	w  io.WriteCloser
}

func (t *ndjsonTracer) Emit(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	data = append(data, '\n')

	t.mu.Lock()
	defer t.mu.Unlock()
	t.w.Write(data) //nolint:errcheck
}

func (t *ndjsonTracer) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.w.Close()
}

// nopCloser wraps a writer that shouldn't be closed (e.g., stderr).
type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

func generateRunID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("%x", b)
}
