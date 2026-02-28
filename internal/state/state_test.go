package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Session tests ---

func TestSessionFile_RoundTrip(t *testing.T) {
	tmp := t.TempDir()

	s := &Session{
		SessionSource:    "startup",
		SessionStartedAt: "2026-02-27T00:00:00Z",
		BeadClaimedAt:    "2026-02-27T00:01:00Z",
	}
	if err := WriteSessionFile(tmp, s); err != nil {
		t.Fatalf("WriteSessionFile failed: %v", err)
	}

	got, err := ReadSession(tmp)
	if err != nil {
		t.Fatalf("ReadSession failed: %v", err)
	}
	if got.SessionSource != "startup" {
		t.Errorf("sessionSource: got %q, want %q", got.SessionSource, "startup")
	}
	if got.SessionStartedAt != "2026-02-27T00:00:00Z" {
		t.Errorf("sessionStartedAt: got %q, want %q", got.SessionStartedAt, "2026-02-27T00:00:00Z")
	}
	if got.BeadClaimedAt != "2026-02-27T00:01:00Z" {
		t.Errorf("beadClaimedAt: got %q, want %q", got.BeadClaimedAt, "2026-02-27T00:01:00Z")
	}
}

func TestSessionFile_MissingReturnsZero(t *testing.T) {
	tmp := t.TempDir()

	got, err := ReadSession(tmp)
	if err != nil {
		t.Fatalf("ReadSession failed: %v", err)
	}
	if got.SessionSource != "" {
		t.Errorf("expected empty sessionSource, got %q", got.SessionSource)
	}
}

func TestSessionFile_OmitsEmptyFields(t *testing.T) {
	tmp := t.TempDir()

	s := &Session{SessionSource: "clear"}
	if err := WriteSessionFile(tmp, s); err != nil {
		t.Fatalf("WriteSessionFile failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".mindspec", "session.json"))
	if err != nil {
		t.Fatalf("reading session.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := parsed["beadClaimedAt"]; ok {
		t.Error("expected beadClaimedAt to be omitted when empty")
	}
}

// --- Lifecycle tests ---

func TestLifecycle_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".mindspec", "docs", "specs", "054-test")

	lc := &Lifecycle{
		Phase:  ModeImplement,
		EpicID: "mindspec-xyz",
	}
	if err := WriteLifecycle(specDir, lc); err != nil {
		t.Fatalf("WriteLifecycle failed: %v", err)
	}

	got, err := ReadLifecycle(specDir)
	if err != nil {
		t.Fatalf("ReadLifecycle failed: %v", err)
	}
	if got.Phase != ModeImplement {
		t.Errorf("phase: got %q, want %q", got.Phase, ModeImplement)
	}
	if got.EpicID != "mindspec-xyz" {
		t.Errorf("epic_id: got %q, want %q", got.EpicID, "mindspec-xyz")
	}
}

func TestLifecycle_MissingReturnsNil(t *testing.T) {
	tmp := t.TempDir()

	got, err := ReadLifecycle(tmp)
	if err != nil {
		t.Fatalf("ReadLifecycle failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing lifecycle.yaml, got %+v", got)
	}
}

func TestLifecycle_PhaseOnly(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "specs", "test")

	lc := &Lifecycle{Phase: ModeSpec}
	if err := WriteLifecycle(specDir, lc); err != nil {
		t.Fatalf("WriteLifecycle failed: %v", err)
	}

	got, err := ReadLifecycle(specDir)
	if err != nil {
		t.Fatalf("ReadLifecycle failed: %v", err)
	}
	if got.Phase != ModeSpec {
		t.Errorf("phase: got %q, want %q", got.Phase, ModeSpec)
	}
	if got.EpicID != "" {
		t.Errorf("expected empty epic_id, got %q", got.EpicID)
	}
}

// --- Focus tests (formerly Focus) ---

func TestFocus_RoundTrip(t *testing.T) {
	tmp := t.TempDir()

	f := &Focus{
		Mode:           ModeImplement,
		ActiveSpec:     "054-simplify",
		ActiveBead:     "beads-abc",
		ActiveWorktree: "/path/to/worktree",
		SpecBranch:     "spec/054-simplify",
	}
	if err := WriteFocus(tmp, f); err != nil {
		t.Fatalf("WriteFocus failed: %v", err)
	}

	got, err := ReadFocus(tmp)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if got.Mode != ModeImplement {
		t.Errorf("mode: got %q, want %q", got.Mode, ModeImplement)
	}
	if got.ActiveSpec != "054-simplify" {
		t.Errorf("activeSpec: got %q, want %q", got.ActiveSpec, "054-simplify")
	}
	if got.ActiveBead != "beads-abc" {
		t.Errorf("activeBead: got %q, want %q", got.ActiveBead, "beads-abc")
	}
	if got.Timestamp == "" {
		t.Error("timestamp should be set")
	}
}

func TestFocus_MissingReturnsNil(t *testing.T) {
	tmp := t.TempDir()

	got, err := ReadFocus(tmp)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing focus, got %+v", got)
	}
}

func TestFocus_SetsTimestamp(t *testing.T) {
	tmp := t.TempDir()

	f := &Focus{Mode: ModeIdle}
	if err := WriteFocus(tmp, f); err != nil {
		t.Fatalf("WriteFocus failed: %v", err)
	}
	if f.Timestamp == "" {
		t.Error("WriteFocus should set Timestamp")
	}
}

func TestFocus_IgnoresLegacyModeCachePath(t *testing.T) {
	tmp := t.TempDir()

	// Write directly to old mode-cache path (should be ignored)
	dir := filepath.Join(tmp, ".mindspec")
	os.MkdirAll(dir, 0755)
	data := []byte(`{"mode":"plan","activeSpec":"old-spec","timestamp":"2026-01-01T00:00:00Z"}`)
	os.WriteFile(filepath.Join(dir, "mode-cache"), data, 0644)

	// ReadFocus should NOT find it — mode-cache fallback is removed
	got, err := ReadFocus(tmp)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil (mode-cache should be ignored), got %+v", got)
	}
}

// --- Convention function tests ---

func TestSpecBranch(t *testing.T) {
	tests := []struct {
		specID string
		want   string
	}{
		{"053-drop-state-json", "spec/053-drop-state-json"},
		{"001-skeleton", "spec/001-skeleton"},
	}
	for _, tt := range tests {
		got := SpecBranch(tt.specID)
		if got != tt.want {
			t.Errorf("SpecBranch(%q) = %q, want %q", tt.specID, got, tt.want)
		}
	}
}

func TestSpecWorktreePath(t *testing.T) {
	got := SpecWorktreePath("/project", "053-foo")
	want := filepath.Join("/project", ".worktrees", "worktree-spec-053-foo")
	if got != want {
		t.Errorf("SpecWorktreePath = %q, want %q", got, want)
	}
}

func TestBeadWorktreePath(t *testing.T) {
	specWT := filepath.Join("/project", ".worktrees", "worktree-spec-053-foo")
	got := BeadWorktreePath(specWT, "mindspec-mol-07lst")
	want := filepath.Join(specWT, ".worktrees", "worktree-mindspec-mol-07lst")
	if got != want {
		t.Errorf("BeadWorktreePath = %q, want %q", got, want)
	}
}

func TestIsValidMode(t *testing.T) {
	for _, m := range ValidModes {
		if !IsValidMode(m) {
			t.Errorf("IsValidMode(%q) = false, want true", m)
		}
	}
	if IsValidMode("invalid") {
		t.Error("IsValidMode(\"invalid\") = true, want false")
	}
}
