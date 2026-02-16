package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	// Create .mindspec marker dir
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	// Create a spec directory
	os.MkdirAll(filepath.Join(tmp, "docs", "specs", "004-instruct"), 0755)
	os.WriteFile(filepath.Join(tmp, "docs", "specs", "004-instruct", "spec.md"), []byte("# Spec 004\n\n## Approval\n\n- **Status**: APPROVED\n"), 0644)
	return tmp
}

func TestReadWrite(t *testing.T) {
	tmp := setupTestProject(t)

	s := &State{
		Mode:       ModeSpec,
		ActiveSpec: "004-instruct",
	}

	if err := Write(tmp, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(tmp)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.Mode != ModeSpec {
		t.Errorf("mode: got %q, want %q", got.Mode, ModeSpec)
	}
	if got.ActiveSpec != "004-instruct" {
		t.Errorf("activeSpec: got %q, want %q", got.ActiveSpec, "004-instruct")
	}
	if got.LastUpdated == "" {
		t.Error("lastUpdated should be set")
	}
}

func TestReadNoFile(t *testing.T) {
	tmp := t.TempDir()

	_, err := Read(tmp)
	if err != ErrNoState {
		t.Errorf("expected ErrNoState, got %v", err)
	}
}

func TestWriteCreatesDir(t *testing.T) {
	tmp := t.TempDir()

	s := &State{Mode: ModeIdle}
	if err := Write(tmp, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify .mindspec dir was created
	info, err := os.Stat(filepath.Join(tmp, ".mindspec"))
	if err != nil {
		t.Fatalf("expected .mindspec dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected .mindspec to be a directory")
	}
}

func TestWriteValidJSON(t *testing.T) {
	tmp := setupTestProject(t)

	s := &State{
		Mode:       ModePlan,
		ActiveSpec: "004-instruct",
	}
	if err := Write(tmp, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".mindspec", "state.json"))
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("state.json is not valid JSON: %v", err)
	}

	if parsed["mode"] != "plan" {
		t.Errorf("mode: got %v, want %q", parsed["mode"], "plan")
	}
}

func TestSetModeValidation(t *testing.T) {
	tmp := setupTestProject(t)

	tests := []struct {
		name    string
		mode    string
		spec    string
		bead    string
		wantErr bool
	}{
		{"valid idle", ModeIdle, "", "", false},
		{"valid spec", ModeSpec, "004-instruct", "", false},
		{"valid plan", ModePlan, "004-instruct", "", false},
		{"valid implement", ModeImplement, "004-instruct", "beads-001", false},
		{"invalid mode", "invalid", "", "", true},
		{"spec without spec id", ModeSpec, "", "", true},
		{"plan without spec id", ModePlan, "", "", true},
		{"implement without bead", ModeImplement, "004-instruct", "", true},
		{"spec with nonexistent spec", ModeSpec, "999-fake", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetMode(tmp, tt.mode, tt.spec, tt.bead)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetMode(%q, %q, %q): error = %v, wantErr = %v", tt.mode, tt.spec, tt.bead, err, tt.wantErr)
			}
		})
	}
}

func TestSetModeWritesState(t *testing.T) {
	tmp := setupTestProject(t)

	if err := SetMode(tmp, ModePlan, "004-instruct", ""); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}

	s, err := Read(tmp)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if s.Mode != ModePlan {
		t.Errorf("mode: got %q, want %q", s.Mode, ModePlan)
	}
	if s.ActiveSpec != "004-instruct" {
		t.Errorf("activeSpec: got %q, want %q", s.ActiveSpec, "004-instruct")
	}
}
