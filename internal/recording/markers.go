package recording

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bench"
)

// EmitMarker appends a lifecycle marker event to events.ndjson.
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

	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshaling marker: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(EventsPath(root, specID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening events file: %w", err)
	}
	defer f.Close()

	_, err = f.Write(line)
	return err
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
