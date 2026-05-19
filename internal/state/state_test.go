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

// --- Convention function tests ---
//
// TestSpecBranch, TestSpecWorktreePath, and TestBeadWorktreePath were
// relocated to internal/workspace/worktree_test.go along with the
// helpers themselves (ARCH-7 / mindspec-c8q0).

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
