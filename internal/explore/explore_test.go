package explore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	// Create .mindspec marker dir
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	// Create docs/specs for spec-init
	os.MkdirAll(filepath.Join(tmp, "docs", "specs"), 0755)
	return tmp
}

func TestEnter_FromIdle(t *testing.T) {
	root := setupTestProject(t)
	// Set idle focus
	state.WriteFocus(root, &state.Focus{Mode: state.ModeIdle})

	if err := Enter(root, "test idea"); err != nil {
		t.Fatalf("Enter failed: %v", err)
	}

	mc, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if mc.Mode != state.ModeExplore {
		t.Errorf("mode: got %q, want %q", mc.Mode, state.ModeExplore)
	}
}

func TestEnter_FromNoState(t *testing.T) {
	root := setupTestProject(t)
	// No focus file exists

	if err := Enter(root, "test idea"); err != nil {
		t.Fatalf("Enter failed: %v", err)
	}

	mc, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if mc.Mode != state.ModeExplore {
		t.Errorf("mode: got %q, want %q", mc.Mode, state.ModeExplore)
	}
}

func TestEnter_RejectsNonIdle(t *testing.T) {
	root := setupTestProject(t)
	state.WriteFocus(root, &state.Focus{Mode: state.ModeSpec, ActiveSpec: "001-test"})

	err := Enter(root, "test idea")
	if err == nil {
		t.Fatal("expected error when entering explore from spec mode")
	}
	if !strings.Contains(err.Error(), "currently in") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDismiss_FromExplore(t *testing.T) {
	root := setupTestProject(t)
	state.WriteFocus(root, &state.Focus{Mode: state.ModeExplore})

	if err := Dismiss(root); err != nil {
		t.Fatalf("Dismiss failed: %v", err)
	}

	mc, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if mc.Mode != state.ModeIdle {
		t.Errorf("mode: got %q, want %q", mc.Mode, state.ModeIdle)
	}
}

func TestDismiss_RejectsNonExplore(t *testing.T) {
	root := setupTestProject(t)
	state.WriteFocus(root, &state.Focus{Mode: state.ModeIdle})

	err := Dismiss(root)
	if err == nil {
		t.Fatal("expected error when dismissing from idle mode")
	}
	if !strings.Contains(err.Error(), "not in explore mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPromote_RejectsNonExplore(t *testing.T) {
	root := setupTestProject(t)
	state.WriteFocus(root, &state.Focus{Mode: state.ModeIdle})

	err := Promote(root, "042-test", "")
	if err == nil {
		t.Fatal("expected error when promoting from idle mode")
	}
	if !strings.Contains(err.Error(), "not in explore mode") {
		t.Errorf("unexpected error: %v", err)
	}
}
