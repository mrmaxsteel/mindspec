package recording

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/bench"
)

func TestManifestRoundTrip(t *testing.T) {
	root := t.TempDir()
	specID := "001-test-spec"

	m := &Manifest{
		SpecID:        specID,
		StartedAt:     "2026-01-01T00:00:00Z",
		CollectorPID:  12345,
		CollectorPort: 4318,
		Status:        "recording",
		Phases: []Phase{
			{Phase: "spec", StartedAt: "2026-01-01T00:00:00Z"},
		},
	}

	// Create spec dir structure
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs", specID, "recording"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifest(root, specID)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if got.SpecID != specID {
		t.Errorf("SpecID = %q, want %q", got.SpecID, specID)
	}
	if got.CollectorPID != 12345 {
		t.Errorf("CollectorPID = %d, want 12345", got.CollectorPID)
	}
	if got.Status != "recording" {
		t.Errorf("Status = %q, want %q", got.Status, "recording")
	}
	if len(got.Phases) != 1 {
		t.Errorf("len(Phases) = %d, want 1", len(got.Phases))
	}
}

func TestHasRecording(t *testing.T) {
	root := t.TempDir()
	specID := "001-test-spec"

	if HasRecording(root, specID) {
		t.Error("HasRecording should return false for missing recording")
	}

	// Create manifest
	if err := os.MkdirAll(RecordingDir(root, specID), 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	if !HasRecording(root, specID) {
		t.Error("HasRecording should return true after creating manifest")
	}
}

func TestEmitMarker(t *testing.T) {
	root := t.TempDir()
	specID := "001-test-spec"

	// Create recording dir and manifest
	if err := os.MkdirAll(RecordingDir(root, specID), 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	// Emit a marker
	err := EmitMarker(root, specID, "lifecycle.start", map[string]any{"phase": "spec"})
	if err != nil {
		t.Fatalf("EmitMarker: %v", err)
	}

	// Read events file
	data, err := os.ReadFile(EventsPath(root, specID))
	if err != nil {
		t.Fatalf("reading events: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var e bench.CollectedEvent
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("parsing NDJSON: %v", err)
	}

	if e.Event != "lifecycle.start" {
		t.Errorf("Event = %q, want %q", e.Event, "lifecycle.start")
	}
	if e.Data["spec_id"] != specID {
		t.Errorf("Data[spec_id] = %v, want %q", e.Data["spec_id"], specID)
	}
	if e.Data["phase"] != "spec" {
		t.Errorf("Data[phase] = %v, want %q", e.Data["phase"], "spec")
	}
	if e.TS == "" {
		t.Error("TS should not be empty")
	}
}

func TestEmitMarkerNoRecording(t *testing.T) {
	root := t.TempDir()

	// Should be a no-op with no error
	err := EmitMarker(root, "nonexistent", "lifecycle.start", nil)
	if err != nil {
		t.Errorf("EmitMarker on non-existent spec should not error: %v", err)
	}
}

func TestEmitPhaseMarker(t *testing.T) {
	root := t.TempDir()
	specID := "001-test-spec"

	if err := os.MkdirAll(RecordingDir(root, specID), 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	if err := EmitPhaseMarker(root, specID, "spec", "plan"); err != nil {
		t.Fatalf("EmitPhaseMarker: %v", err)
	}

	data, err := os.ReadFile(EventsPath(root, specID))
	if err != nil {
		t.Fatal(err)
	}

	var e bench.CollectedEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &e); err != nil {
		t.Fatal(err)
	}

	if e.Event != "lifecycle.phase" {
		t.Errorf("Event = %q, want lifecycle.phase", e.Event)
	}
	if e.Data["from"] != "spec" {
		t.Errorf("from = %v, want spec", e.Data["from"])
	}
	if e.Data["to"] != "plan" {
		t.Errorf("to = %v, want plan", e.Data["to"])
	}
}

func TestEmitBeadMarker(t *testing.T) {
	root := t.TempDir()
	specID := "001-test-spec"

	if err := os.MkdirAll(RecordingDir(root, specID), 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	if err := EmitBeadMarker(root, specID, "start", "T-abc"); err != nil {
		t.Fatalf("EmitBeadMarker: %v", err)
	}

	data, err := os.ReadFile(EventsPath(root, specID))
	if err != nil {
		t.Fatal(err)
	}

	var e bench.CollectedEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &e); err != nil {
		t.Fatal(err)
	}

	if e.Event != "lifecycle.bead.start" {
		t.Errorf("Event = %q, want lifecycle.bead.start", e.Event)
	}
	if e.Data["bead_id"] != "T-abc" {
		t.Errorf("bead_id = %v, want T-abc", e.Data["bead_id"])
	}
}

func TestEnsureOTLPIdempotent(t *testing.T) {
	root := t.TempDir()

	// Create .claude directory
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}

	// First call should write config
	wrote, err := EnsureOTLP(root)
	if err != nil {
		t.Fatalf("EnsureOTLP first call: %v", err)
	}
	if !wrote {
		t.Error("expected first call to write config")
	}

	// Read and verify
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal("expected env block in settings")
	}

	if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://localhost:4318" {
		t.Errorf("endpoint = %v, want http://localhost:4318", env["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}

	// Second call should be idempotent
	wrote, err = EnsureOTLP(root)
	if err != nil {
		t.Fatalf("EnsureOTLP second call: %v", err)
	}
	if wrote {
		t.Error("expected second call to be a no-op")
	}
}

func TestEnsureOTLPPreservesExisting(t *testing.T) {
	root := t.TempDir()
	settingsDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write existing settings with other fields
	existing := map[string]any{
		"permissions": map[string]any{"allow": []string{"Read"}},
		"env": map[string]any{
			"MY_VAR": "keep-me",
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.local.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	wrote, err := EnsureOTLP(root)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Error("expected config to be written")
	}

	// Verify existing fields are preserved
	data, _ = os.ReadFile(filepath.Join(settingsDir, "settings.local.json"))
	var result map[string]any
	json.Unmarshal(data, &result) //nolint:errcheck

	if result["permissions"] == nil {
		t.Error("permissions field was lost")
	}

	env := result["env"].(map[string]any)
	if env["MY_VAR"] != "keep-me" {
		t.Error("existing env var was lost")
	}
	if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://localhost:4318" {
		t.Error("OTLP endpoint not written")
	}
}

func TestUpdatePhase(t *testing.T) {
	root := t.TempDir()
	specID := "001-test-spec"

	if err := os.MkdirAll(RecordingDir(root, specID), 0755); err != nil {
		t.Fatal(err)
	}

	m := &Manifest{
		SpecID: specID,
		Status: "recording",
		Phases: []Phase{
			{Phase: "spec", StartedAt: "2026-01-01T00:00:00Z"},
		},
	}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	if err := UpdatePhase(root, specID, "spec", "plan"); err != nil {
		t.Fatal(err)
	}

	got, err := ReadManifest(root, specID)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(got.Phases))
	}
	if got.Phases[0].EndedAt == "" {
		t.Error("first phase should have EndedAt set")
	}
	if got.Phases[1].Phase != "plan" {
		t.Errorf("second phase = %q, want plan", got.Phases[1].Phase)
	}
}

func TestAddBeadToPhase(t *testing.T) {
	root := t.TempDir()
	specID := "001-test-spec"

	if err := os.MkdirAll(RecordingDir(root, specID), 0755); err != nil {
		t.Fatal(err)
	}

	m := &Manifest{
		SpecID: specID,
		Status: "recording",
		Phases: []Phase{
			{Phase: "implement", StartedAt: "2026-01-01T00:00:00Z"},
		},
	}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	if err := AddBeadToPhase(root, specID, "T-abc"); err != nil {
		t.Fatal(err)
	}

	got, err := ReadManifest(root, specID)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Phases[0].Beads) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(got.Phases[0].Beads))
	}
	if got.Phases[0].Beads[0] != "T-abc" {
		t.Errorf("bead = %q, want T-abc", got.Phases[0].Beads[0])
	}
}

func TestPathHelpers(t *testing.T) {
	root := "/project"
	specID := "027-spec-recording"

	recDir := RecordingDir(root, specID)
	if recDir != "/project/docs/specs/027-spec-recording/recording" {
		t.Errorf("RecordingDir = %q", recDir)
	}

	manifest := ManifestPath(root, specID)
	if manifest != "/project/docs/specs/027-spec-recording/recording/manifest.json" {
		t.Errorf("ManifestPath = %q", manifest)
	}

	events := EventsPath(root, specID)
	if events != "/project/docs/specs/027-spec-recording/recording/events.ndjson" {
		t.Errorf("EventsPath = %q", events)
	}
}

func TestHealthCheckNoRecording(t *testing.T) {
	root := t.TempDir()

	status, err := HealthCheck(root, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if status != HealthNoRecording {
		t.Errorf("expected HealthNoRecording, got %d", status)
	}
}
