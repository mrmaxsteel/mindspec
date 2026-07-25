package main

// spec_create_autonumber_test.go: GH #219 — `spec create` accepts a bare
// kebab-case slug and auto-numbers it <NNN>-<slug> as max(existing spec
// numbers)+1 (zero-padded to 3), while an explicit <NNN>-<slug> is used
// verbatim. Existing numbers are gathered from the SpecsDir enumeration
// root AND from spec dirs inside worktrees under the default worktrees
// root (a pre-epic spec exists only on its branch's worktree).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkSpecDirs creates each named spec directory under dir.
func mkSpecDirs(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(dir, n), 0o755); err != nil {
			t.Fatalf("mkdir %s/%s: %v", dir, n, err)
		}
	}
}

func TestResolveSpecCreateID_AutoNumber(t *testing.T) {
	cases := []struct {
		name     string
		mainSpec []string // dirs under <root>/.mindspec/specs
		wtSpec   []string // dirs under <root>/.worktrees/worktree-spec-x/.mindspec/specs
		arg      string
		want     string
		wantErr  string
	}{
		{
			name: "bare slug, empty workspace, allocates 001",
			arg:  "seam-x", want: "001-seam-x",
		},
		{
			name:     "bare slug allocates max+1",
			mainSpec: []string{"001-a", "002-b"},
			arg:      "seam-x", want: "003-seam-x",
		},
		{
			name:     "gaps are not reused (max+1, never first-free)",
			mainSpec: []string{"001-a", "007-b"},
			arg:      "seam-x", want: "008-seam-x",
		},
		{
			name:     "letter-suffixed spec dirs count by their number",
			mainSpec: []string{"008b-old", "003-a"},
			arg:      "seam-x", want: "009-seam-x",
		},
		{
			name:     "worktree-only (pre-epic) spec bumps the allocation",
			mainSpec: []string{"125-shipped"},
			wtSpec:   []string{"126-inflight"},
			arg:      "seam-x", want: "127-seam-x",
		},
		{
			name:     "non-numeric entries are skipped",
			mainSpec: []string{"notaspec", "004-a"},
			arg:      "seam-x", want: "005-seam-x",
		},
		{
			name:     "4-digit numbers grow past the pad width",
			mainSpec: []string{"1234-big"},
			arg:      "seam-x", want: "1235-seam-x",
		},
		{
			name:     "explicit <NNN>-<slug> used verbatim, never renumbered",
			mainSpec: []string{"001-a", "007-b"},
			arg:      "005-explicit", want: "005-explicit",
		},
		{
			name: "explicit letter-suffixed ID used verbatim",
			arg:  "008b-explicit", want: "008b-explicit",
		},
		{
			name: "invalid as slug AND as ID is refused with both forms named",
			arg:  "Seam_X", wantErr: "must match <NNN>-<slug>",
		},
		{
			name: "path separator refused",
			arg:  "seam/x", wantErr: "path separator",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			// Flat layout marker + enumeration root.
			mkSpecDirs(t, filepath.Join(root, ".mindspec", "specs"), tc.mainSpec...)
			if len(tc.wtSpec) > 0 {
				mkSpecDirs(t, filepath.Join(root, ".worktrees", "worktree-spec-126-inflight", ".mindspec", "specs"), tc.wtSpec...)
			}

			got, err := resolveSpecCreateID(root, tc.arg)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveSpecCreateID(%q) = %q, want error containing %q", tc.arg, got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantErr)
				}
				if !strings.Contains(err.Error(), "bare kebab-case slug") {
					t.Errorf("refusal must name the bare-slug form too: %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSpecCreateID(%q): %v", tc.arg, err)
			}
			if got != tc.want {
				t.Errorf("resolveSpecCreateID(%q) = %q, want %q", tc.arg, got, tc.want)
			}
		})
	}
}

// TestSpecCreateUsage_NamesBothForms pins the --help contract (GH #219):
// the usage line must advertise BOTH the bare slug (auto-numbered) and
// the explicit <NNN>-<slug> form.
func TestSpecCreateUsage_NamesBothForms(t *testing.T) {
	if !strings.Contains(specCreateCmd.Use, "<slug>") || !strings.Contains(specCreateCmd.Use, "<NNN>-<slug>") {
		t.Errorf("spec create Use = %q, must name both <slug> and <NNN>-<slug>", specCreateCmd.Use)
	}
	if !strings.Contains(specCreateCmd.Long, "auto-numbered") {
		t.Errorf("spec create Long must document auto-numbering, got: %q", specCreateCmd.Long)
	}
}
