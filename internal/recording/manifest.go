package recording

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mindspec/mindspec/internal/validate"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Manifest tracks recording state for a spec.
type Manifest struct {
	SpecID        string  `json:"spec_id"`
	StartedAt     string  `json:"started_at"`
	CollectorPID  int     `json:"collector_pid"`
	CollectorPort int     `json:"collector_port"`
	ProcessName   string  `json:"process_name,omitempty"` // expected binary name for PID verification
	Status        string  `json:"status"`                 // recording, stopped, complete, stale
	Phases        []Phase `json:"phases"`
}

// Phase tracks a lifecycle phase within a recording.
type Phase struct {
	Phase     string   `json:"phase"`
	StartedAt string   `json:"started_at"`
	EndedAt   string   `json:"ended_at,omitempty"`
	Beads     []string `json:"beads,omitempty"`
}

// RecordingDir returns the recording directory for a spec.
func RecordingDir(root, specID string) string {
	return workspace.RecordingDir(root, specID)
}

// ManifestPath returns the path to manifest.json for a spec.
func ManifestPath(root, specID string) string {
	return filepath.Join(RecordingDir(root, specID), "manifest.json")
}

// EventsPath returns the path to events.ndjson for a spec.
func EventsPath(root, specID string) string {
	return filepath.Join(RecordingDir(root, specID), "events.ndjson")
}

// HasRecording returns true if a recording directory exists for the spec.
func HasRecording(root, specID string) bool {
	_, err := os.Stat(ManifestPath(root, specID))
	return err == nil
}

// ReadManifest reads the manifest from disk.
func ReadManifest(root, specID string) (*Manifest, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(ManifestPath(root, specID))
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &m, nil
}

// WriteManifest writes the manifest to disk.
func WriteManifest(root, specID string, m *Manifest) error {
	if err := validate.SpecID(specID); err != nil {
		return err
	}
	dir := RecordingDir(root, specID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating recording dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(ManifestPath(root, specID), data, 0644)
}
