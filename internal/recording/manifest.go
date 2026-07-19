package recording

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
// Returns an error if specID is not a well-formed spec identifier.
func RecordingDir(root, specID string) (string, error) {
	return workspace.RecordingDir(root, specID)
}

// ManifestPath returns the path to manifest.json for a spec.
// Returns an error if specID is not a well-formed spec identifier.
func ManifestPath(root, specID string) (string, error) {
	dir, err := RecordingDir(root, specID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "manifest.json"), nil
}

// EventsPath returns the path to events.ndjson for a spec.
// Returns an error if specID is not a well-formed spec identifier.
func EventsPath(root, specID string) (string, error) {
	dir, err := RecordingDir(root, specID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "events.ndjson"), nil
}

// HasRecording returns true if a recording directory exists for the spec.
//
// On an invalid spec ID this returns false (the safe default — callers
// treat "no recording" as a non-action). Keeping a bool return keeps
// the ergonomic call sites (e.g. `if !recording.HasRecording(...) { ... }`)
// unchanged. Callers that need to distinguish "invalid ID" from "no recording"
// should call ManifestPath directly.
func HasRecording(root, specID string) bool {
	p, err := ManifestPath(root, specID)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// ReadManifest reads the manifest from disk.
func ReadManifest(root, specID string) (*Manifest, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	p, err := ManifestPath(root, specID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
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
//
// Persistence gate (spec 120 final-review G2-1, ADR-0042 class-5
// structured persistence): the manifest is agent-writable on disk and is
// round-tripped through read-modify-write cycles (StopRecording,
// UpdatePhase, AddBeadToPhase), so validating only the PATH specID is not
// enough — the ID-bearing PAYLOAD fields (m.SpecID and every
// m.Phases[*].Beads entry) are validated here, at the write boundary,
// before serialization. A hostile ID that was hand-edited into
// manifest.json fails closed: the write is refused and the durable file
// is never re-minted with the hostile value.
func WriteManifest(root, specID string, m *Manifest) error {
	if err := validate.SpecID(specID); err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("writing manifest: nil manifest")
	}
	if err := validate.SpecID(m.SpecID); err != nil {
		return fmt.Errorf("refusing to write manifest: payload spec_id: %w", err)
	}
	for _, ph := range m.Phases {
		for _, b := range ph.Beads {
			if err := idvalidate.BeadID(b); err != nil {
				return fmt.Errorf("refusing to write manifest: phase %q bead id: %w", ph.Phase, err)
			}
		}
	}
	dir, err := RecordingDir(root, specID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating recording dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	data = append(data, '\n')
	mp, err := ManifestPath(root, specID)
	if err != nil {
		return err
	}
	return os.WriteFile(mp, data, 0o600)
}
