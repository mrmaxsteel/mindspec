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
		{"valid explore", ModeExplore, "", "", false},
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

func TestSetMode_PreservesMoleculeMetadataForSameSpec(t *testing.T) {
	tmp := setupTestProject(t)

	if err := Write(tmp, &State{
		Mode:           ModeSpec,
		ActiveSpec:     "004-instruct",
		ActiveMolecule: "mol-123",
		StepMapping: map[string]string{
			"spec":         "step-1",
			"spec-approve": "step-2",
		},
	}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := SetMode(tmp, ModePlan, "004-instruct", ""); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}

	got, err := Read(tmp)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got.ActiveMolecule != "mol-123" {
		t.Errorf("activeMolecule: got %q, want %q", got.ActiveMolecule, "mol-123")
	}
	if got.StepMapping["spec"] != "step-1" {
		t.Errorf("stepMapping[spec]: got %q, want %q", got.StepMapping["spec"], "step-1")
	}
}

func TestSetMode_DifferentSpecDoesNotCarryMoleculeMetadata(t *testing.T) {
	tmp := setupTestProject(t)
	os.MkdirAll(filepath.Join(tmp, "docs", "specs", "005-other"), 0755)
	os.WriteFile(filepath.Join(tmp, "docs", "specs", "005-other", "spec.md"), []byte("# Spec 005"), 0644)

	if err := Write(tmp, &State{
		Mode:           ModeSpec,
		ActiveSpec:     "004-instruct",
		ActiveMolecule: "mol-123",
		StepMapping: map[string]string{
			"spec": "step-1",
		},
	}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := SetMode(tmp, ModePlan, "005-other", ""); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}

	got, err := Read(tmp)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got.ActiveMolecule != "" {
		t.Errorf("activeMolecule: got %q, want empty", got.ActiveMolecule)
	}
	if len(got.StepMapping) != 0 {
		t.Errorf("expected empty stepMapping, got %v", got.StepMapping)
	}
}

func TestNeedsClear_RoundTrip(t *testing.T) {
	tmp := setupTestProject(t)

	// Write state with NeedsClear = true
	s := &State{
		Mode:       ModeImplement,
		ActiveSpec: "004-instruct",
		ActiveBead: "bead-1",
		NeedsClear: true,
	}
	if err := Write(tmp, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read back — should be true
	got, err := Read(tmp)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !got.NeedsClear {
		t.Error("expected NeedsClear to be true after write")
	}

	// Clear it
	if err := ClearNeedsClear(tmp); err != nil {
		t.Fatalf("ClearNeedsClear failed: %v", err)
	}

	// Read back — should be false
	got2, err := Read(tmp)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got2.NeedsClear {
		t.Error("expected NeedsClear to be false after ClearNeedsClear")
	}
}

func TestNeedsClear_OmittedWhenFalse(t *testing.T) {
	tmp := setupTestProject(t)

	s := &State{
		Mode:       ModeIdle,
		NeedsClear: false,
	}
	if err := Write(tmp, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".mindspec", "state.json"))
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}

	// omitempty means the field should not appear in JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := parsed["needs_clear"]; ok {
		t.Error("expected needs_clear to be omitted when false")
	}
}

func TestSetModeWithMetadata_UsesProvidedValues(t *testing.T) {
	tmp := setupTestProject(t)

	steps := map[string]string{
		"plan":         "step-1",
		"plan-approve": "step-2",
	}
	if err := SetModeWithMetadata(tmp, ModePlan, "004-instruct", "", "mol-xyz", steps); err != nil {
		t.Fatalf("SetModeWithMetadata failed: %v", err)
	}

	steps["plan"] = "mutated"

	got, err := Read(tmp)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got.ActiveMolecule != "mol-xyz" {
		t.Errorf("activeMolecule: got %q, want %q", got.ActiveMolecule, "mol-xyz")
	}
	if got.StepMapping["plan"] != "step-1" {
		t.Errorf("stepMapping[plan]: got %q, want %q", got.StepMapping["plan"], "step-1")
	}
}
