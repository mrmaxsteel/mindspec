package recording

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustRecordingDir is a test helper that fails the test on validation error.
func mustRecordingDir(t *testing.T, root, specID string) string {
	t.Helper()
	p, err := RecordingDir(root, specID)
	if err != nil {
		t.Fatalf("RecordingDir(%q): %v", specID, err)
	}
	return p
}

// mustEventsPath is a test helper that fails the test on validation error.
func mustEventsPath(t *testing.T, root, specID string) string {
	t.Helper()
	p, err := EventsPath(root, specID)
	if err != nil {
		t.Fatalf("EventsPath(%q): %v", specID, err)
	}
	return p
}

// mustManifestPath is a test helper that fails the test on validation error.
func mustManifestPath(t *testing.T, root, specID string) string {
	t.Helper()
	p, err := ManifestPath(root, specID)
	if err != nil {
		t.Fatalf("ManifestPath(%q): %v", specID, err)
	}
	return p
}

// enableRecording writes a config that enables recording in the temp root.
func enableRecording(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("recording:\n  enabled: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

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
	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0755); err != nil {
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
	enableRecording(t, root)
	specID := "001-test-spec"

	// Create recording dir and manifest
	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0755); err != nil {
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
	data, err := os.ReadFile(mustEventsPath(t, root, specID))
	if err != nil {
		t.Fatalf("reading events: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var e MarkerEvent
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
	enableRecording(t, root)
	specID := "001-test-spec"

	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	if err := EmitPhaseMarker(root, specID, "spec", "plan"); err != nil {
		t.Fatalf("EmitPhaseMarker: %v", err)
	}

	data, err := os.ReadFile(mustEventsPath(t, root, specID))
	if err != nil {
		t.Fatal(err)
	}

	var e MarkerEvent
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
	enableRecording(t, root)
	specID := "001-test-spec"

	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	if err := EmitBeadMarker(root, specID, "start", "t-abc"); err != nil {
		t.Fatalf("EmitBeadMarker: %v", err)
	}

	data, err := os.ReadFile(mustEventsPath(t, root, specID))
	if err != nil {
		t.Fatal(err)
	}

	var e MarkerEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &e); err != nil {
		t.Fatal(err)
	}

	if e.Event != "lifecycle.bead.start" {
		t.Errorf("Event = %q, want lifecycle.bead.start", e.Event)
	}
	if e.Data["bead_id"] != "t-abc" {
		t.Errorf("bead_id = %v, want T-abc", e.Data["bead_id"])
	}
}

func TestEnsureOTLPIdempotent(t *testing.T) {
	root := t.TempDir()
	enableRecording(t, root)

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
	enableRecording(t, root)
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
	enableRecording(t, root)
	specID := "001-test-spec"

	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0755); err != nil {
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
	enableRecording(t, root)
	specID := "001-test-spec"

	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0755); err != nil {
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

	if err := AddBeadToPhase(root, specID, "t-abc"); err != nil {
		t.Fatal(err)
	}

	got, err := ReadManifest(root, specID)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Phases[0].Beads) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(got.Phases[0].Beads))
	}
	if got.Phases[0].Beads[0] != "t-abc" {
		t.Errorf("bead = %q, want T-abc", got.Phases[0].Beads[0])
	}
}

func TestPathHelpers(t *testing.T) {
	root := "/project"
	specID := "027-spec-recording"

	recDir, err := RecordingDir(root, specID)
	if err != nil {
		t.Fatalf("RecordingDir: %v", err)
	}
	if recDir != "/project/.mindspec/docs/specs/027-spec-recording/recording" {
		t.Errorf("RecordingDir = %q", recDir)
	}

	manifest, err := ManifestPath(root, specID)
	if err != nil {
		t.Fatalf("ManifestPath: %v", err)
	}
	if manifest != "/project/.mindspec/docs/specs/027-spec-recording/recording/manifest.json" {
		t.Errorf("ManifestPath = %q", manifest)
	}

	events, err := EventsPath(root, specID)
	if err != nil {
		t.Fatalf("EventsPath: %v", err)
	}
	if events != "/project/.mindspec/docs/specs/027-spec-recording/recording/events.ndjson" {
		t.Errorf("EventsPath = %q", events)
	}
}

func TestRecordingFileModes(t *testing.T) {
	root := t.TempDir()
	enableRecording(t, root)
	specID := "001-mode-test"

	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0o700); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}
	if err := EmitMarker(root, specID, "lifecycle.start", nil); err != nil {
		t.Fatalf("EmitMarker: %v", err)
	}

	cases := []struct {
		path string
		want os.FileMode
	}{
		{mustManifestPath(t, root, specID), 0o600},
		{mustEventsPath(t, root, specID), 0o600},
	}
	for _, tc := range cases {
		info, err := os.Stat(tc.path)
		if err != nil {
			t.Fatalf("stat %s: %v", tc.path, err)
		}
		if got := info.Mode().Perm(); got != tc.want {
			t.Errorf("%s mode = %#o, want %#o", tc.path, got, tc.want)
		}
	}

	dirInfo, err := os.Stat(mustRecordingDir(t, root, specID))
	if err != nil {
		t.Fatal(err)
	}
	// Allow umask to remove bits but reject any group/other bits.
	if dirInfo.Mode().Perm()&0o077 != 0 {
		t.Errorf("recording dir mode = %#o, expected no group/other bits",
			dirInfo.Mode().Perm())
	}
}

// TestRecordingFileModesAgentMindFirst simulates the case where AgentMind's
// collector creates events.ndjson first with a looser mode (0644). The
// belt-and-suspenders chmod in EmitMarker must tighten the existing file.
func TestRecordingFileModesAgentMindFirst(t *testing.T) {
	root := t.TempDir()
	enableRecording(t, root)
	specID := "001-agentmind-first"

	if err := os.MkdirAll(mustRecordingDir(t, root, specID), 0o700); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{SpecID: specID, Status: "recording"}
	if err := WriteManifest(root, specID, m); err != nil {
		t.Fatal(err)
	}

	// Pre-create events.ndjson with loose mode to simulate AgentMind creating
	// it first.
	if err := os.WriteFile(mustEventsPath(t, root, specID), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EmitMarker(root, specID, "lifecycle.start", nil); err != nil {
		t.Fatalf("EmitMarker: %v", err)
	}

	info, err := os.Stat(mustEventsPath(t, root, specID))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("post-emit events mode = %#o, want 0600 (chmod must tighten existing file)", got)
	}
}

// TestRecordingWriteGates is spec 120 AC-23 (the round-5 structured-
// persistence class, ADR-0042 §7): EmitBeadMarker/AddBeadToPhase with an
// invalid specID OR beadID perform NO events.ndjson/manifest mutation and
// surface one escaped warning via the existing error channel; a valid
// dotted child (mindspec-9cyu.1) writes byte-identically to today.
func TestRecordingWriteGates(t *testing.T) {
	const validSpecID = "001-test-spec"

	setup := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		enableRecording(t, root)
		if err := os.MkdirAll(mustRecordingDir(t, root, validSpecID), 0755); err != nil {
			t.Fatal(err)
		}
		if err := WriteManifest(root, validSpecID, &Manifest{SpecID: validSpecID, Status: "recording", Phases: []Phase{{Phase: "implement"}}}); err != nil {
			t.Fatal(err)
		}
		return root
	}

	hostileIDs := []string{"--help", "x;evil", "x\x00\x1b[31m\nrecovery: forged"}

	t.Run("EmitBeadMarker skips on invalid specID", func(t *testing.T) {
		root := setup(t)
		if err := EmitBeadMarker(root, "not a spec id", "start", "mindspec-9cyu.1"); err != nil {
			t.Fatalf("EmitBeadMarker must skip (nil error), got: %v", err)
		}
		if _, err := os.Stat(mustEventsPath(t, root, validSpecID)); !os.IsNotExist(err) {
			t.Errorf("expected NO events.ndjson write for an invalid specID, stat err = %v", err)
		}
	})

	t.Run("EmitBeadMarker skips on invalid beadID", func(t *testing.T) {
		for _, hostile := range hostileIDs {
			root := setup(t)
			if err := EmitBeadMarker(root, validSpecID, "start", hostile); err != nil {
				t.Fatalf("EmitBeadMarker(%q) must skip (nil error), got: %v", hostile, err)
			}
			if _, err := os.Stat(mustEventsPath(t, root, validSpecID)); !os.IsNotExist(err) {
				t.Errorf("EmitBeadMarker(%q): expected NO events.ndjson write, stat err = %v", hostile, err)
			}
		}
	})

	t.Run("AddBeadToPhase skips on invalid beadID", func(t *testing.T) {
		for _, hostile := range hostileIDs {
			root := setup(t)
			if err := AddBeadToPhase(root, validSpecID, hostile); err != nil {
				t.Fatalf("AddBeadToPhase(%q) must skip (nil error), got: %v", hostile, err)
			}
			m, err := ReadManifest(root, validSpecID)
			if err != nil {
				t.Fatalf("ReadManifest: %v", err)
			}
			if len(m.Phases[0].Beads) != 0 {
				t.Errorf("AddBeadToPhase(%q): expected NO manifest mutation, got Beads=%v", hostile, m.Phases[0].Beads)
			}
		}
	})

	t.Run("valid dotted-child bead writes byte-identically", func(t *testing.T) {
		root := setup(t)
		if err := EmitBeadMarker(root, validSpecID, "start", "mindspec-9cyu.1"); err != nil {
			t.Fatalf("EmitBeadMarker: %v", err)
		}
		data, err := os.ReadFile(mustEventsPath(t, root, validSpecID))
		if err != nil {
			t.Fatalf("expected events.ndjson to exist for a valid write: %v", err)
		}
		var e MarkerEvent
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &e); err != nil {
			t.Fatal(err)
		}
		if e.Data["bead_id"] != "mindspec-9cyu.1" {
			t.Errorf("bead_id = %v, want mindspec-9cyu.1", e.Data["bead_id"])
		}

		if err := AddBeadToPhase(root, validSpecID, "mindspec-9cyu.1"); err != nil {
			t.Fatalf("AddBeadToPhase: %v", err)
		}
		m, err := ReadManifest(root, validSpecID)
		if err != nil {
			t.Fatalf("ReadManifest: %v", err)
		}
		if len(m.Phases[0].Beads) != 1 || m.Phases[0].Beads[0] != "mindspec-9cyu.1" {
			t.Errorf("expected Beads=[mindspec-9cyu.1], got %v", m.Phases[0].Beads)
		}
	})
}

// TestHealthCheckNoRecording was deleted with internal/recording/health.go
// in spec 084 Bead 3. mindspec no longer manages a collector subprocess,
// so the HealthCheck / RestartIfDead / HealthStatus surface is gone.
// Recording presence on disk is still observable via HasRecording.

// TestWriteManifestRejectsHostilePayloadIDs is the spec 120 final-review
// G2-1 regression: WriteManifest is the class-5 structured-persistence
// consumer for the agent-writable recording manifest, and previously
// validated only the PATH specID — not the ID-bearing payload fields. A
// read-modify-write cycle (StopRecording/UpdatePhase/AddBeadToPhase) over
// an agent-edited manifest.json must never serialize a hostile spec_id or
// phase bead id back into the durable file (ADR-0042 persistence
// doctrine: validate durable ID fields at write).
func TestWriteManifestRejectsHostilePayloadIDs(t *testing.T) {
	specID := "001-test-spec"

	cases := []struct {
		name string
		m    *Manifest
	}{
		{"nil manifest", nil},
		{"hostile payload spec_id (option-like)", &Manifest{
			SpecID: "--help", Status: "recording",
		}},
		{"hostile payload spec_id (control bytes)", &Manifest{
			SpecID: "120-x\n[FORGED] fake", Status: "recording",
		}},
		{"hostile phase bead id (metachar)", &Manifest{
			SpecID: specID, Status: "recording",
			Phases: []Phase{{Phase: "implement", Beads: []string{"x;evil"}}},
		}},
		{"hostile phase bead id (option-like)", &Manifest{
			SpecID: specID, Status: "recording",
			Phases: []Phase{{Phase: "implement", Beads: []string{"mindspec-9cyu.1", "--help"}}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := WriteManifest(root, specID, tc.m); err == nil {
				t.Fatal("WriteManifest accepted a hostile/nil payload")
			}
			mp, err := ManifestPath(root, specID)
			if err != nil {
				t.Fatalf("ManifestPath: %v", err)
			}
			if _, statErr := os.Stat(mp); statErr == nil {
				t.Error("hostile manifest payload was persisted to disk")
			}
		})
	}

	t.Run("valid payload still writes", func(t *testing.T) {
		root := t.TempDir()
		m := &Manifest{
			SpecID: specID, Status: "recording",
			Phases: []Phase{{Phase: "implement", Beads: []string{"mindspec-9cyu.1", "mindspec-0ke"}}},
		}
		if err := WriteManifest(root, specID, m); err != nil {
			t.Fatalf("WriteManifest rejected a valid payload: %v", err)
		}
		got, err := ReadManifest(root, specID)
		if err != nil {
			t.Fatalf("ReadManifest: %v", err)
		}
		if got.SpecID != specID || len(got.Phases) != 1 || len(got.Phases[0].Beads) != 2 {
			t.Errorf("valid manifest did not round-trip: %+v", got)
		}
	})
}
