package recording

import (
	"fmt"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bench"
	"github.com/mrmaxsteel/mindspec/internal/ndjson"
)

// EmitMarker appends a lifecycle marker event to events.ndjson.
//
// Each call opens, writes, and closes the events file. The deferred Close is
// the durability boundary — callers (short-lived CLI invocations) can rely on
// the marker being on disk by the time EmitMarker returns, because Close runs
// before the function returns. Do not hoist the writer out of this function
// without preserving that guarantee; recording tests read the events file
// immediately after EmitMarker returns.
func EmitMarker(root, specID, event string, data map[string]any) error {
	if !IsEnabled(root) {
		return nil
	}
	if !HasRecording(root, specID) {
		return nil // no-op if no recording exists
	}

	if data == nil {
		data = make(map[string]any)
	}
	data["spec_id"] = specID

	e := bench.CollectedEvent{
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Event: event,
		Data:  data,
	}

	eventsPath, err := EventsPath(root, specID)
	if err != nil {
		return fmt.Errorf("resolving events path: %w", err)
	}
	// BufSize == 0 keeps Emit synchronous to the file descriptor; defer Close
	// closes the FD before this function returns, providing per-call durability.
	// FileMode 0o600 enforces SEC-7: the writer chmods after OpenFile so that
	// even if AgentMind's collector created the file first with a looser umask,
	// the marker write path re-tightens it to 0600.
	w, err := ndjson.New(eventsPath, ndjson.Opts{
		Append:   true,
		FileMode: 0o600,
	})
	if err != nil {
		return fmt.Errorf("opening events file: %w", err)
	}
	defer w.Close() //nolint:errcheck

	if err := w.Emit(e); err != nil {
		return fmt.Errorf("writing marker: %w", err)
	}
	return nil
}

// EmitPhaseMarker emits a lifecycle.phase marker.
func EmitPhaseMarker(root, specID, from, to string) error {
	return EmitMarker(root, specID, "lifecycle.phase", map[string]any{
		"from": from,
		"to":   to,
	})
}

// EmitBeadMarker emits a lifecycle.bead.start or lifecycle.bead.complete marker.
func EmitBeadMarker(root, specID, action, beadID string) error {
	event := "lifecycle.bead." + action
	return EmitMarker(root, specID, event, map[string]any{
		"bead_id": beadID,
	})
}
