package resolve

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestResolveTarget_ExplicitFlag(t *testing.T) {
	// Explicit --spec always wins, regardless of active specs
	got, err := ResolveTarget("/nonexistent", "042-my-spec")
	if err != nil {
		t.Fatalf("ResolveTarget() error: %v", err)
	}
	if got != "042-my-spec" {
		t.Errorf("ResolveTarget() = %q, want %q", got, "042-my-spec")
	}
}

func TestErrAmbiguousTarget_Message(t *testing.T) {
	err := &ErrAmbiguousTarget{
		Active: []SpecStatus{
			{SpecID: "001-alpha", Mode: "spec"},
			{SpecID: "002-beta", Mode: "plan"},
		},
	}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	for _, id := range []string{"001-alpha", "002-beta"} {
		if !strings.Contains(msg, id) {
			t.Errorf("error message should contain %q: %s", id, msg)
		}
	}
	if !strings.Contains(msg, "--spec") {
		t.Errorf("error message should mention --spec flag: %s", msg)
	}
}

func TestResolveTarget_NoActiveSpecs(t *testing.T) {
	// Stub bd to return no epics
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	_, err := ResolveTarget(t.TempDir(), "")
	if err == nil {
		t.Fatal("expected error when no active specs found")
	}
	if !strings.Contains(err.Error(), "no active specs") {
		t.Errorf("error should mention 'no active specs': %v", err)
	}
}

func TestResolveTarget_NoActiveSpecs_SuggestsFlag(t *testing.T) {
	// Stub bd to return no epics
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	_, err := ResolveTarget(t.TempDir(), "")
	if err == nil {
		t.Fatal("expected error when no active specs found")
	}
	if !strings.Contains(err.Error(), "--spec") {
		t.Errorf("error should suggest --spec flag: %v", err)
	}
}

func TestErrAmbiguousTarget_IsDetectable(t *testing.T) {
	err := &ErrAmbiguousTarget{Active: []SpecStatus{{SpecID: "a"}}}
	var ambErr *ErrAmbiguousTarget
	if !isAmbiguousError(err, &ambErr) {
		t.Error("expected ErrAmbiguousTarget to be detectable via type assertion")
	}
}

// isAmbiguousError is a helper for type assertion tests.
func isAmbiguousError(err error, target **ErrAmbiguousTarget) bool {
	e, ok := err.(*ErrAmbiguousTarget)
	if ok {
		*target = e
	}
	return ok
}
