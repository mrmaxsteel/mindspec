package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

func writeBoundSpec(t *testing.T, root, specID string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	spec := `---
molecule_id: mol-parent
step_mapping:
  spec: step-spec
  spec-approve: step-spec-approve
  plan: step-plan
  plan-approve: step-plan-approve
  implement: step-impl
  review: step-review
  spec-lifecycle: mol-parent
---
# Spec ` + specID + `
`
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

func TestApproveImpl_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")

	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	// Set state to review mode
	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	defer func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
	}()

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			payload := []map[string]string{{"status": "open"}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}

	var closed []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) == 2 && args[0] == "close" {
			closed = append(closed, args[1])
			return []byte("ok"), nil
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SpecID != "010-test" {
		t.Errorf("SpecID: got %q, want %q", result.SpecID, "010-test")
	}
	got := append([]string(nil), closed...)
	sort.Strings(got)
	want := []string{"mol-parent", "step-impl", "step-plan", "step-plan-approve", "step-review", "step-spec", "step-spec-approve"}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("closed IDs mismatch\ngot:  %v\nwant: %v", got, want)
	}

	// Verify state is now idle
	s, err := state.Read(tmp)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if s.Mode != state.ModeIdle {
		t.Errorf("mode: got %q, want %q", s.Mode, state.ModeIdle)
	}
}

func TestApproveImpl_WrongMode(t *testing.T) {
	tmp := t.TempDir()

	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeImplement,
		ActiveSpec: "010-test",
		ActiveBead: "bead-1",
	})

	_, err := ApproveImpl(tmp, "010-test")
	if err == nil {
		t.Fatal("expected error for wrong mode")
	}
	if !strings.Contains(err.Error(), "expected review mode") {
		t.Errorf("error should mention expected review mode: %v", err)
	}
}

func TestApproveImpl_WrongSpec(t *testing.T) {
	tmp := t.TempDir()

	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

	_, err := ApproveImpl(tmp, "011-other")
	if err == nil {
		t.Fatal("expected error for wrong spec")
	}
	if !strings.Contains(err.Error(), "active spec") {
		t.Errorf("error should mention active spec mismatch: %v", err)
	}
}

func TestApproveImpl_PartialCloseFailureWarnsAndContinues(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	defer func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
	}()

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}

	var closed []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("unexpected args: %v", args)
		}
		if args[1] == "step-plan" {
			return nil, fmt.Errorf("boom")
		}
		closed = append(closed, args[1])
		return []byte("ok"), nil
	}

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for failed close")
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "step-plan") {
		t.Errorf("expected warning to mention failed member: %v", result.Warnings)
	}
	if len(closed) == 0 {
		t.Fatal("expected other members to still be closed")
	}
}
