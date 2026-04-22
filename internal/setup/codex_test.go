package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunCodex_PatchesBeadsConfig mirrors the Claude/Copilot tests — a project
// that already ran `bd init` should get a mindspec-ready .beads/config.yaml
// after `mindspec setup codex`. Codex chains `bd setup codex`, so this covers
// the post-chain config patch.
func TestRunCodex_PatchesBeadsConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existing := "issue-prefix: \"cdx\"\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := RunCodex(root, false)
	if err != nil {
		t.Fatalf("RunCodex: %v", err)
	}
	if r.BeadsConfig == nil {
		t.Fatalf("expected BeadsConfig populated, got nil (err=%v)", r.BeadsConfErr)
	}
	added := map[string]bool{}
	for _, k := range r.BeadsConfig.Added {
		added[k] = true
	}
	for _, k := range []string{"types.custom", "status.custom", "export.git-add"} {
		if !added[k] {
			t.Errorf("expected %q in Added, got %v", k, r.BeadsConfig.Added)
		}
	}
	data, _ := os.ReadFile(filepath.Join(beadsDir, "config.yaml"))
	got := string(data)
	for _, f := range []string{"issue-prefix:", "cdx", "types.custom:", "status.custom:", "export.git-add:"} {
		if !strings.Contains(got, f) {
			t.Errorf("config.yaml missing %q; full content:\n%s", f, got)
		}
	}
}

// TestRunCodex_BeadsConfigIdempotent verifies running setup twice on an
// already-mindspec-ready config is a byte-identical no-op.
func TestRunCodex_BeadsConfigIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	ready := `issue-prefix: "proj"
types.custom: "gate"
status.custom: "resolved"
export.git-add: false
`
	cfgPath := filepath.Join(beadsDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(ready), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(cfgPath)

	if _, err := RunCodex(root, false); err != nil {
		t.Fatalf("first RunCodex: %v", err)
	}
	r2, err := RunCodex(root, false)
	if err != nil {
		t.Fatalf("second RunCodex: %v", err)
	}
	if r2.BeadsConfig == nil {
		t.Fatal("expected BeadsConfig on second run")
	}
	if n := len(r2.BeadsConfig.Added); n != 0 {
		t.Errorf("second run added %d keys: %v", n, r2.BeadsConfig.Added)
	}
	if n := len(r2.BeadsConfig.UserAuthored); n != 0 {
		t.Errorf("second run reported drift: %+v", r2.BeadsConfig.UserAuthored)
	}
	after, _ := os.ReadFile(cfgPath)
	if string(before) != string(after) {
		t.Errorf("config.yaml changed on idempotent run:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestRunCodex_NoBeadsDir verifies RunCodex leaves BeadsConfig nil and
// produces no error when the project has no .beads/ directory.
func TestRunCodex_NoBeadsDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	r, err := RunCodex(root, false)
	if err != nil {
		t.Fatalf("RunCodex: %v", err)
	}
	if r.BeadsConfig != nil {
		t.Errorf("expected BeadsConfig=nil without .beads/, got %+v", r.BeadsConfig)
	}
	if r.BeadsConfErr != nil {
		t.Errorf("unexpected BeadsConfErr: %v", r.BeadsConfErr)
	}
}

// TestRunCodex_CheckModeScansWithoutMutating verifies --check returns a
// read-only scan but does not touch .beads/config.yaml on disk.
func TestRunCodex_CheckModeScansWithoutMutating(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := "issue-prefix: \"proj\"\n"
	cfgPath := filepath.Join(beadsDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := RunCodex(root, true)
	if err != nil {
		t.Fatalf("RunCodex(check=true): %v", err)
	}
	if r.BeadsConfig == nil {
		t.Fatal("check mode should scan and return a ConfigResult, got nil")
	}
	if !r.BeadsScan {
		t.Error("BeadsScan should be true in check mode")
	}
	data, _ := os.ReadFile(cfgPath)
	if string(data) != original {
		t.Errorf("check mode modified config.yaml:\nwant: %q\ngot:  %q", original, string(data))
	}
}
