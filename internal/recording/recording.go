package recording

import (
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// IsEnabled checks the project config to determine if recording is active.
func IsEnabled(root string) bool {
	cfg, err := config.Load(root)
	if err != nil {
		return false
	}
	return cfg.Recording.Enabled
}

// StartRecording creates the recording directory, starts the collector,
// and emits the lifecycle.start marker.
func StartRecording(root, specID string) error {
	if !IsEnabled(root) {
		return nil
	}
	dir := RecordingDir(root, specID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating recording dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	m := &Manifest{
		SpecID:    specID,
		StartedAt: now,
		Status:    "recording",
		Phases: []Phase{
			{Phase: "spec", StartedAt: now},
		},
	}
	if err := WriteManifest(root, specID, m); err != nil {
		return fmt.Errorf("writing initial manifest: %w", err)
	}

	if err := StartCollector(root, specID); err != nil {
		return fmt.Errorf("starting collector: %w", err)
	}

	if err := EmitMarker(root, specID, "lifecycle.start", map[string]any{
		"phase": "spec",
	}); err != nil {
		return fmt.Errorf("emitting start marker: %w", err)
	}

	return nil
}

// StopRecording emits the lifecycle.end marker, stops the collector,
// and finalizes the manifest.
func StopRecording(root, specID string) error {
	if !IsEnabled(root) {
		return nil
	}
	if !HasRecording(root, specID) {
		return nil // no-op
	}

	if err := EmitMarker(root, specID, "lifecycle.end", nil); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not emit end marker: %v\n", err)
	}

	return StopCollector(root, specID)
}

// UpdatePhase closes the current phase and opens a new one in the manifest.
func UpdatePhase(root, specID, from, to string) error {
	if !IsEnabled(root) {
		return nil
	}
	if !HasRecording(root, specID) {
		return nil
	}

	m, err := ReadManifest(root, specID)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Close current phase
	if len(m.Phases) > 0 {
		last := &m.Phases[len(m.Phases)-1]
		if last.EndedAt == "" {
			last.EndedAt = now
		}
	}

	// Open new phase
	m.Phases = append(m.Phases, Phase{
		Phase:     to,
		StartedAt: now,
	})

	return WriteManifest(root, specID, m)
}

// AddBeadToPhase adds a bead ID to the current phase in the manifest.
func AddBeadToPhase(root, specID, beadID string) error {
	if !IsEnabled(root) {
		return nil
	}
	if !HasRecording(root, specID) {
		return nil
	}

	m, err := ReadManifest(root, specID)
	if err != nil {
		return err
	}

	if len(m.Phases) > 0 {
		last := &m.Phases[len(m.Phases)-1]
		last.Beads = append(last.Beads, beadID)
	}

	return WriteManifest(root, specID, m)
}
