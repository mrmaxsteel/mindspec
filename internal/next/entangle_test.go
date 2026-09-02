package next

// entangle_test.go — spec 128 (lifecycle-verb-state-honesty) Bead 7,
// R5.1 / AC-8: the proportional claim guard. CheckDirtyTree/
// CheckDirtyTreeDetail (guard_test.go) are UNCHANGED — release.go and
// any other dirty-tree guard still call them directly and still block
// on ANY user dirt. CheckDirtyTreeProportional is the new, narrower
// entry point `mindspec next`'s worktree-creating claim path alone uses.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// --- entangleableClaimPaths (R5.1's derived enumeration) ---

func TestEntangleableClaimPaths_EmptyByDefault(t *testing.T) {
	got := EntangleableClaimPaths()
	if len(got) != 0 {
		t.Errorf("EntangleableClaimPaths() = %v, want empty — R5.1 derived the claim-entangleable set as empty today (session.json is gitignored, the beads export is the exempt named artifact, and the bead worktree dir is itself gitignored)", got)
	}
}

func TestEntangleableClaimPaths_ReturnsCopyNotSharedSlice(t *testing.T) {
	restore := SetEntangleableClaimPathsForTest([]string{"a.txt"})
	defer restore()

	got := EntangleableClaimPaths()
	got[0] = "mutated"
	if entangleableClaimPaths[0] != "a.txt" {
		t.Errorf("mutating the caller's copy affected internal state: %v", entangleableClaimPaths)
	}
}

// --- CheckDirtyTreeProportional ---

func TestCheckDirtyTreeProportional_Clean(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{""}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 || len(ignored) != 0 {
		t.Errorf("blocking=%v ignored=%v, want both empty on a clean tree", blocking, ignored)
	}
}

// AC-8(i): an unrelated dirty user file no longer blocks — it is
// returned as ignoredDirt, never blockingDirt, since R5.1's derived
// enumeration is empty today.
func TestCheckDirtyTreeProportional_UnrelatedUserDirtIgnoredNotBlocked(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{" M foo.txt\n"}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty — the entangleable set is empty today", blocking)
	}
	if !reflect.DeepEqual(ignored, []string{"foo.txt"}) {
		t.Errorf("ignoredDirt = %v, want [foo.txt]", ignored)
	}
}

// AC-8(i) variant: an untracked directory tree reads as multiple
// porcelain entries, all ignored the same way as a single file.
func TestCheckDirtyTreeProportional_UntrackedDirectoryTreeIgnored(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{"?? notes/draft.md\n?? notes/scratch.md\n"}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty", blocking)
	}
	want := []string{"notes/draft.md", "notes/scratch.md"}
	if !reflect.DeepEqual(ignored, want) {
		t.Errorf("ignoredDirt = %v, want %v", ignored, want)
	}
}

// Artifact-only dirt never even reaches the entanglement split — it
// normalizes away via bd export exactly as CheckDirtyTree already did.
func TestCheckDirtyTreeProportional_ArtifactAloneProceedsSilently(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{" M .beads/issues.jsonl\n", ""}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 || len(ignored) != 0 {
		t.Errorf("blocking=%v ignored=%v, want both empty (artifact-only dirt is auto-handled, ADR-0025)", blocking, ignored)
	}
}

func TestCheckDirtyTreeProportional_PropagatesUnderlyingError(t *testing.T) {
	g := &fakeGuard{porcelainErr: fmt.Errorf("git status boom")}
	g.install(t)

	_, _, err := CheckDirtyTreeProportional("/repo", "/repo")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

// AC-8(iii): the seam-injected refusal leg. With a path injected into
// the enumeration and dirt on that path, the claim still refuses — the
// block machinery stays live and exercised even though the real
// enumeration is empty. An unrelated dirty path alongside it is still
// merely ignored, not blocking.
func TestCheckDirtyTreeProportional_InjectedEntanglementBlocks(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{" M entangled.txt\n M unrelated.txt\n"}}
	g.install(t)
	restore := SetEntangleableClaimPathsForTest([]string{"entangled.txt"})
	defer restore()

	blocking, ignored, err := CheckDirtyTreeProportional("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(blocking, []string{"entangled.txt"}) {
		t.Errorf("blockingDirt = %v, want [entangled.txt]", blocking)
	}
	if !reflect.DeepEqual(ignored, []string{"unrelated.txt"}) {
		t.Errorf("ignoredDirt = %v, want [unrelated.txt]", ignored)
	}

	// The preserved DirtyTreeFailure contract, unchanged, over the
	// blocking subset only.
	failure := DirtyTreeFailure("/repo", blocking, "")
	if failure == nil {
		t.Fatal("DirtyTreeFailure returned nil")
	}
	msg := failure.Error()
	assertDirtyTreeFailureInvariants(t, msg)
	if !strings.Contains(msg, "entangled.txt") {
		t.Errorf("failure must name the blocking dirty path: %q", msg)
	}
	if strings.Contains(msg, "unrelated.txt") {
		t.Errorf("failure must NOT name the merely-ignored path: %q", msg)
	}
}

// --- IgnoredDirtNotice ---

func TestIgnoredDirtNotice_NamesEveryPathOnOneLine(t *testing.T) {
	msg := IgnoredDirtNotice([]string{"foo.txt", "bar/baz.txt"})
	if !strings.Contains(msg, "foo.txt") || !strings.Contains(msg, "bar/baz.txt") {
		t.Errorf("notice must name every ignored path: %q", msg)
	}
	if strings.Contains(msg, "\n") {
		t.Errorf("notice must be exactly ONE line: %q", msg)
	}
}

// R4/ADR-0042: each ignored path is agent-writable porcelain text —
// escape per path so a hostile filename cannot forge control bytes or
// extra output lines into the notice.
func TestIgnoredDirtNotice_EscapesHostilePath(t *testing.T) {
	hostile := "evil\x1b[31mred.txt"
	msg := IgnoredDirtNotice([]string{hostile})
	if strings.Contains(msg, "\x1b") {
		t.Errorf("notice must escape control bytes in a hostile filename: %q", msg)
	}
	if strings.Contains(msg, "\n") {
		t.Errorf("an escaped hostile path must not introduce a real newline: %q", msg)
	}
}
