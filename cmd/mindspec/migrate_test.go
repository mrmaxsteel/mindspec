package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveArchiveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		want      string
		expectErr bool
	}{
		{name: "default empty", in: "", want: "copy"},
		{name: "copy", in: "copy", want: "copy"},
		{name: "move", in: "move", want: "move"},
		{name: "trim whitespace", in: "  copy  ", want: "copy"},
		{name: "invalid", in: "zip", expectErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveArchiveMode(tc.in)
			if tc.expectErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("archive mode mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestValidateMigrateApplyFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runID       string
		archive     string
		wantRunID   string
		wantArchive string
		expectErr   bool
	}{
		{
			name:      "run-id required",
			archive:   "copy",
			expectErr: true,
		},
		{
			name:        "default archive",
			runID:       "run-1",
			wantRunID:   "run-1",
			wantArchive: "copy",
		},
		{
			name:        "archive move",
			runID:       "run-2",
			archive:     "move",
			wantRunID:   "run-2",
			wantArchive: "move",
		},
		{
			name:      "invalid archive",
			runID:     "run-3",
			archive:   "tar",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotRunID, gotArchive, err := validateMigrateApplyFlags(tc.runID, tc.archive)
			if tc.expectErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotRunID != tc.wantRunID {
				t.Fatalf("run-id mismatch: got %q want %q", gotRunID, tc.wantRunID)
			}
			if gotArchive != tc.wantArchive {
				t.Fatalf("archive mismatch: got %q want %q", gotArchive, tc.wantArchive)
			}
		})
	}
}

func TestReadMigrationArtifactJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runID := "run-1"
	path := filepath.Join(root, ".mindspec", "migrations", runID, "plan.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{\"run_id\":\"run-1\"}\n"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	got, err := readMigrationArtifactJSON(root, runID, "plan.json")
	if err != nil {
		t.Fatalf("readMigrationArtifactJSON: %v", err)
	}
	if got != "{\"run_id\":\"run-1\"}" {
		t.Fatalf("artifact mismatch: got %q", got)
	}
}

func TestReadMigrationArtifactJSON_Missing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	_, err := readMigrationArtifactJSON(root, "run-404", "plan.json")
	if err == nil {
		t.Fatal("expected missing artifact error")
	}
}
