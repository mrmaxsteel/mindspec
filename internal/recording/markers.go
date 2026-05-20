package recording

import (
	"fmt"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/ndjson"
)

// MarkerEvent is the on-disk NDJSON shape of a lifecycle marker.
//
// Spec 084 Bead 3: this type replaces the previous
// `bench.CollectedEvent` (a type alias of
// `github.com/mrmaxsteel/agentmind/wire.CollectedEvent`). After the
// bench/agentmind subsystems were deleted from mindspec, the recording
// package owns its own NDJSON schema — it is the only writer left and
// the on-disk JSON shape (`ts`, `event`, `data`) is unchanged so
// downstream consumers (replay tools, agentmind, etc.) keep parsing the
// same fields. The `resource` field that the OTLP-side wire type
// carried is unused by markers and intentionally omitted here.
type MarkerEvent struct {
	TS    string         `json:"ts"`
	Event string         `json:"event"`
	Data  map[string]any `json:"data,omitempty"`
}

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

	e := MarkerEvent{
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
	// FileMode 0o600 enforces SEC-7: the writer chmods after OpenFile so the
	// file is created (or re-tightened) at 0o600 even if a previous writer
	// left it looser.
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
