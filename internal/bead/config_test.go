package bead

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeBeadsConfig(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCustomStatuses_SingleScalar(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `status.custom: "resolved"`+"\n")

	got := CustomStatuses(tmp)
	if !reflect.DeepEqual(got, []string{"resolved"}) {
		t.Errorf("got %v, want [resolved]", got)
	}
}

func TestCustomStatuses_CommaSeparated(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `status.custom: "resolved, paused, waiting"`+"\n")

	got := CustomStatuses(tmp)
	want := []string{"resolved", "paused", "waiting"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCustomStatuses_YAMLList(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, "status.custom:\n  - resolved\n  - paused\n")

	got := CustomStatuses(tmp)
	want := []string{"resolved", "paused"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCustomStatuses_MissingConfigIsEmpty(t *testing.T) {
	got := CustomStatuses(t.TempDir())
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestCustomStatuses_MalformedConfigIsEmpty(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, "not: yaml: [")

	got := CustomStatuses(tmp)
	if len(got) != 0 {
		t.Errorf("expected empty slice on malformed config, got %v", got)
	}
}

func TestAllStatuses_MergesBuiltinsAndCustom(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `status.custom: "resolved, paused"`+"\n")

	got := AllStatuses(tmp)
	want := []string{"open", "in_progress", "blocked", "closed", "resolved", "paused"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAllStatuses_DedupsAgainstBuiltins(t *testing.T) {
	tmp := t.TempDir()
	// Config redeclares `closed` as custom — must not appear twice.
	writeBeadsConfig(t, tmp, `status.custom: "closed, resolved"`+"\n")

	got := AllStatuses(tmp)
	want := []string{"open", "in_progress", "blocked", "closed", "resolved"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAllStatuses_NoConfigReturnsBuiltinsOnly(t *testing.T) {
	got := AllStatuses(t.TempDir())
	want := []string{"open", "in_progress", "blocked", "closed"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
