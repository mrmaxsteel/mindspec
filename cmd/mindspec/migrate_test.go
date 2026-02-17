package main

import "testing"

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
