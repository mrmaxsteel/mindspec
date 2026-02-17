package main

import "testing"

func TestResolveInitMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		brownfield  bool
		reportOnly  bool
		apply       bool
		archive     string
		resume      string
		wantMode    initMode
		wantArchive string
		expectErr   bool
	}{
		{
			name:     "greenfield default",
			wantMode: initModeGreenfield,
		},
		{
			name:       "brownfield default report-only",
			brownfield: true,
			wantMode:   initModeBrownfieldReport,
		},
		{
			name:       "brownfield explicit report-only",
			brownfield: true,
			reportOnly: true,
			wantMode:   initModeBrownfieldReport,
		},
		{
			name:        "brownfield apply default archive",
			brownfield:  true,
			apply:       true,
			wantMode:    initModeBrownfieldApply,
			wantArchive: "copy",
		},
		{
			name:        "brownfield apply with archive move",
			brownfield:  true,
			apply:       true,
			archive:     "move",
			wantMode:    initModeBrownfieldApply,
			wantArchive: "move",
		},
		{
			name:       "reject report-only without brownfield",
			reportOnly: true,
			expectErr:  true,
		},
		{
			name:      "reject apply without brownfield",
			apply:     true,
			expectErr: true,
		},
		{
			name:      "reject archive without brownfield",
			archive:   "copy",
			expectErr: true,
		},
		{
			name:       "reject report-only and apply together",
			brownfield: true,
			reportOnly: true,
			apply:      true,
			expectErr:  true,
		},
		{
			name:       "brownfield resume report-only",
			brownfield: true,
			resume:     "run-1",
			wantMode:   initModeBrownfieldReport,
		},
		{
			name:        "brownfield resume apply",
			brownfield:  true,
			apply:       true,
			resume:      "run-1",
			wantMode:    initModeBrownfieldApply,
			wantArchive: "copy",
		},
		{
			name:       "reject archive without apply",
			brownfield: true,
			archive:    "copy",
			expectErr:  true,
		},
		{
			name:       "reject invalid archive",
			brownfield: true,
			apply:      true,
			archive:    "zip",
			expectErr:  true,
		},
		{
			name:      "reject resume without brownfield",
			resume:    "run-1",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mode, archive, err := resolveInitMode(tc.brownfield, tc.reportOnly, tc.apply, tc.archive, tc.resume)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil (mode=%s archive=%s)", mode, archive)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tc.wantMode {
				t.Fatalf("mode mismatch: got %s want %s", mode, tc.wantMode)
			}
			if archive != tc.wantArchive {
				t.Fatalf("archive mismatch: got %s want %s", archive, tc.wantArchive)
			}
		})
	}
}
