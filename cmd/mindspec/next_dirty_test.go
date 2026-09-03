package main

// Bead 9 punch-list M6 (Bead 7 panel, surviving mutant): cmd-layer pin
// for `mindspec next`'s dirty-tree path. internal/next's unit tests only
// see the DirtyTreeFailure MESSAGE — reverting the old direct-to-stderr
// "Recovery steps: 1..3" block in cmd/mindspec/next.go ALONGSIDE the new
// guard failure (double emission) would survive them.
//
// Spec 128 R5.1 (#179, Bead 7) removed the block itself for THIS
// unrelated-dirt shape: an unrelated dirty user file in the invoking
// checkout no longer trips DirtyTreeFailure at all — it is merely named
// on stderr (Max's standing directive, 2026-09-02) and the claim
// proceeds. This fixture has no `.beads/` (bd is never initialized
// here), so the command still exits non-zero — but for an UNRELATED
// reason (no beads database), never because of the dirty tree. The
// legacy pre-092 "Recovery steps:"/`git restore .` block remains a
// regression pin regardless.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGitInDir runs git with args in dir, with a pinned test identity so
// commits work without host config.
func runGitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=test@example.invalid",
		"-c", "user.name=mindspec-test",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestNextCmd_DirtyTree_UnrelatedDirtNoLongerBlocks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	bin := buildMindspecBinary(t)

	// A workspace with committed baseline content, then user dirt.
	dir := t.TempDir()
	runGitInDir(t, dir, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("clean baseline\n"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}
	runGitInDir(t, dir, "add", "-A")
	runGitInDir(t, dir, "commit", "-m", "baseline")
	// The user's uncommitted dirt — what the old block advised discarding,
	// and what R5.1 (spec 128) now merely names instead of blocking.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("user WIP\n"), 0o644); err != nil {
		t.Fatalf("dirty notes.txt: %v", err)
	}

	cmd := exec.Command(bin, "next")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	combined := stdout.String() + stderr.String()

	// The command still fails here — this fixture has no beads database
	// at all — but that failure must be UNRELATED to the dirty tree.
	if err == nil {
		t.Fatalf("expected mindspec next to fail for an unrelated reason (no beads database in this fixture); got exit 0:\n%s", combined)
	}

	// The OLD unconditional dirty-tree block (Reqs 8/12) must be GONE for
	// this unrelated-dirt shape: R5.1 removed it for the worktree-creating
	// claim path (spec 128, #179 — ADR-0025's clarifying amendment).
	if strings.Contains(combined, "cannot claim work") {
		t.Errorf("unrelated user dirt must no longer BLOCK the worktree-creating claim (spec 128 R5.1); got:\n%s", combined)
	}

	// Max's standing directive (2026-09-02): the ignored dirt is still
	// named, on stderr, as a one-line informational notice — visibility
	// without the block. Round-1 panel G5/R1: pin the notice's
	// IDENTIFYING PREFIX too, not just the path — a silent rewording (or
	// a bare path dump with no notice framing at all) must go red here,
	// not just at the internal/next unit level.
	if !strings.Contains(stderr.String(), "claiming despite unrelated dirty path(s)") {
		t.Errorf("expected the ignored-dirt notice's stable prefix in stderr; stderr:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "notes.txt") {
		t.Errorf("expected the ignored-dirt notice to name notes.txt; stderr:\n%s", stderr.String())
	}

	// ...and the pre-092 stderr block stays GONE regardless — not emitted
	// alongside (the M6 double-emission mutant) or on its own.
	for _, banned := range []string{"Recovery steps:", "git restore ."} {
		if strings.Contains(combined, banned) {
			t.Errorf("output still contains the legacy %q advice (M6 double-emission); got:\n%s", banned, combined)
		}
	}
}
