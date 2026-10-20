package next

// entangle_test.go — spec 128 (lifecycle-verb-state-honesty) Bead 7,
// R5.1 / AC-8, ROUND-1 PANEL FIX. CheckDirtyTree/CheckDirtyTreeDetail
// (guard_test.go) are UNCHANGED — release.go and any other dirty-tree
// guard still call them directly, on the newline porcelain form, and
// still block on ANY user dirt. CheckDirtyTreeProportional is the
// narrower entry point `mindspec next`'s worktree-creating claim path
// alone uses, and (round-1 panel fix) it now parses `-z` porcelain and
// consults a per-spec, config-aware entanglement enumeration instead of
// a fixed empty list.
//
// Round-1 panel findings fixed here:
//   - G1/F1 (R3/R6, hard block): the enumeration was factually wrong —
//     two real claim-path writes exist (recording markers; .gitignore).
//     ComputeEntangleablePaths now derives both, conditionally.
//   - G3/G6 (R5/R6): porcelain conservatism — rename/copy blocks on
//     EITHER side; a collapsed untracked directory blocks when it is an
//     ancestor of an entangleable path; -z parsing closes the
//     " -> "-substring forgery risk the newline form has.
//   - G5 (R1): the notice's identifying wording is pinned.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// --- entangleableClaimPaths (the test-injectable residual) ---

func TestEntangleableClaimPaths_EmptyByDefault(t *testing.T) {
	got := EntangleableClaimPaths()
	if len(got) != 0 {
		t.Errorf("EntangleableClaimPaths() = %v, want empty — the test-injectable residual carries nothing in production", got)
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

// --- ComputeEntangleablePaths (round-1 panel G1/F1: the corrected,
// per-spec-aware, dynamic enumeration) ---

// writeConfigYAML writes .mindspec/config.yaml AND marks the tree as a
// flat layout (an immediate .mindspec/specs/ directory — see
// workspace.flatTreePresent) so workspace.SpecDir's DEFAULT branch for a
// not-yet-materialized spec resolves to the flat
// `.mindspec/specs/<specID>` shape every real spec in this repo uses,
// keeping the recording-path expectations below deterministic.
func writeConfigYAML(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec/specs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
}

func TestComputeEntangleablePaths_NoConfigNoGitignoreIncludesGitignoreOnly(t *testing.T) {
	root := t.TempDir()
	got := ComputeEntangleablePaths(root, root, "010-test")
	if !reflect.DeepEqual(got, []string{".gitignore"}) {
		t.Errorf("got %v, want [.gitignore] — default WorktreeRoot (.worktrees) is absent from a nonexistent .gitignore, and recording defaults to disabled", got)
	}
}

func TestComputeEntangleablePaths_GitignoreEntryPresentExcludesGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ComputeEntangleablePaths(root, root, "010-test")
	for _, p := range got {
		if p == ".gitignore" {
			t.Errorf("got %v, .gitignore must NOT be entangleable once the entry is already present (EnsureGitignoreEntry would no-op)", got)
		}
	}
}

// Round-1 panel G1 (F1): recording.enabled + a known target spec ->
// the spec's two recording paths are entangleable.
func TestComputeEntangleablePaths_RecordingEnabledIncludesSpecPaths(t *testing.T) {
	root := t.TempDir()
	writeConfigYAML(t, root, "recording:\n  enabled: true\n")
	// Satisfy HasGitignoreEntry so the assertion below is exact (isolate
	// the recording class).
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ComputeEntangleablePaths(root, root, "010-test")
	want := []string{
		filepath.ToSlash(filepath.Join(".mindspec", "specs", "010-test", "recording", "events.ndjson")),
		filepath.ToSlash(filepath.Join(".mindspec", "specs", "010-test", "recording", "manifest.json")),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Recording disabled (the default): even with a valid target spec, the
// recording class is omitted entirely.
func TestComputeEntangleablePaths_RecordingDisabledOmitsSpecPaths(t *testing.T) {
	root := t.TempDir()
	writeConfigYAML(t, root, "recording:\n  enabled: false\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ComputeEntangleablePaths(root, root, "010-test")
	if len(got) != 0 {
		t.Errorf("got %v, want empty — recording disabled must omit the recording class", got)
	}
}

// specID == "" (next.go could not resolve a target spec before the
// guard runs): the recording class is omitted even if recording is
// enabled — a conservative "unknown, so not claimed as entangled" choice.
func TestComputeEntangleablePaths_NoSpecIDOmitsRecordingClass(t *testing.T) {
	root := t.TempDir()
	writeConfigYAML(t, root, "recording:\n  enabled: true\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ComputeEntangleablePaths(root, root, "")
	if len(got) != 0 {
		t.Errorf("got %v, want empty — no resolved target spec must omit the recording class", got)
	}
}

func TestComputeEntangleablePaths_TestResidualUnionedIn(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := SetEntangleableClaimPathsForTest([]string{"future-tracked-path.txt"})
	defer restore()

	got := ComputeEntangleablePaths(root, root, "")
	if !reflect.DeepEqual(got, []string{"future-tracked-path.txt"}) {
		t.Errorf("got %v, want the test residual unioned in", got)
	}
}

// --- fakeGuardZ: the -z porcelain seam double ---

// zRecord builds one `git status --porcelain -z` field for a plain
// (non-rename) entry: "XY path\0".
func zRecord(status, path string) string {
	return status + " " + path + "\x00"
}

// zRenameRecord builds one rename/copy record: "XY newPath\0origPath\0"
// — verified empirically against real git (spec 128 Bead 7 round-1: the
// NEW path comes first, then the ORIGINAL, both NUL-terminated).
func zRenameRecord(status, newPath, origPath string) string {
	return status + " " + newPath + "\x00" + origPath + "\x00"
}

type fakeGuardZ struct {
	porcelainResponses []string // raw -z blobs, popped front-first per call
	porcelainErr       error
	porcelainCalls     int
	exportCalled       bool
	exportErr          error
	exportRoot         string
}

func (f *fakeGuardZ) install(t *testing.T) {
	t.Helper()
	origStatus := statusPorcelainZFn
	origExport := exportBeadsFn
	t.Cleanup(func() {
		statusPorcelainZFn = origStatus
		exportBeadsFn = origExport
	})
	statusPorcelainZFn = func(cwd string) (string, error) {
		if f.porcelainErr != nil {
			return "", f.porcelainErr
		}
		if f.porcelainCalls >= len(f.porcelainResponses) {
			f.porcelainCalls++
			return "", nil
		}
		resp := f.porcelainResponses[f.porcelainCalls]
		f.porcelainCalls++
		return resp, nil
	}
	exportBeadsFn = func(workdir string) error {
		f.exportCalled = true
		f.exportRoot = workdir
		return f.exportErr
	}
}

// --- parsePorcelainZ ---

func TestParsePorcelainZ_PlainEntries(t *testing.T) {
	out := zRecord(" M", "foo.txt") + zRecord("??", "bar.txt")
	got := parsePorcelainZ(out)
	want := []dirtyEntry{
		{Status: " M", Path: "foo.txt"},
		{Status: "??", Path: "bar.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParsePorcelainZ_RenamePreservesBothPaths(t *testing.T) {
	out := zRenameRecord("RM", "new.txt", "old.txt")
	got := parsePorcelainZ(out)
	want := []dirtyEntry{{Status: "RM", Path: "new.txt", OrigPath: "old.txt"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParsePorcelainZ_CollapsedUntrackedDirectory(t *testing.T) {
	out := zRecord("??", "untrackeddir/")
	got := parsePorcelainZ(out)
	if len(got) != 1 || !got[0].dirCollapsed() {
		t.Errorf("got %+v, want one dirCollapsed entry", got)
	}
}

func TestParsePorcelainZ_HostilePathWithArrowSubstringNotMisparsedAsRename(t *testing.T) {
	// A hostile UNTRACKED filename containing a literal " -> " substring
	// must NOT be misread as a rename record — the newline parser's
	// weakness (round-1 panel G3/G6) that -z's dedicated field boundary
	// closes. Only the STATUS field (R/C) triggers OrigPath consumption.
	hostile := "evil -> forged.txt"
	out := zRecord("??", hostile)
	got := parsePorcelainZ(out)
	want := []dirtyEntry{{Status: "??", Path: hostile}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v (the arrow substring must stay part of ONE path)", got, want)
	}
}

func TestParsePorcelainZ_Empty(t *testing.T) {
	if got := parsePorcelainZ(""); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// --- CheckDirtyTreeProportional ---

func TestCheckDirtyTreeProportional_Clean(t *testing.T) {
	root := t.TempDir()
	g := &fakeGuardZ{porcelainResponses: []string{""}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 || len(ignored) != 0 {
		t.Errorf("blocking=%v ignored=%v, want both empty on a clean tree", blocking, ignored)
	}
}

// AC-8(i): an unrelated dirty user file no longer blocks — it is
// returned as ignoredDirt, never blockingDirt.
func TestCheckDirtyTreeProportional_UnrelatedUserDirtIgnoredNotBlocked(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", "foo.txt")}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty", blocking)
	}
	if !reflect.DeepEqual(ignored, []string{"foo.txt"}) {
		t.Errorf("ignoredDirt = %v, want [foo.txt]", ignored)
	}
}

// AC-8(i) variant: an untracked directory tree reads as multiple
// porcelain entries, all ignored the same way as a single file — as
// long as none of them is an ancestor of an entangleable path.
func TestCheckDirtyTreeProportional_UntrackedDirectoryTreeIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRecord("??", "notes/")}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty", blocking)
	}
	if !reflect.DeepEqual(ignored, []string{"notes/"}) {
		t.Errorf("ignoredDirt = %v, want [notes/]", ignored)
	}
}

// Round-1 panel G1: a genuinely dirty, entry-absent .gitignore BLOCKS —
// the coming EnsureGitignoreEntry write would collide with it.
func TestCheckDirtyTreeProportional_DirtyGitignoreEntryAbsentBlocks(t *testing.T) {
	root := t.TempDir()
	// No .gitignore on disk at all -> entry absent -> entangleable.
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", ".gitignore")}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(blocking, []string{".gitignore"}) {
		t.Errorf("blockingDirt = %v, want [.gitignore]", blocking)
	}
	if len(ignored) != 0 {
		t.Errorf("ignoredDirt = %v, want empty", ignored)
	}
}

// Round-1 panel G1: a dirty .gitignore that ALREADY has the entry does
// NOT block — the coming EnsureGitignoreEntry call is a no-op, so
// there's no collision.
func TestCheckDirtyTreeProportional_DirtyGitignoreEntryPresentIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\nsomething-else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", ".gitignore")}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty (entry already present, no collision)", blocking)
	}
	if !reflect.DeepEqual(ignored, []string{".gitignore"}) {
		t.Errorf("ignoredDirt = %v, want [.gitignore]", ignored)
	}
}

// Round-1 panel G1/F1: a dirty manifest.json for the TARGET spec blocks
// when recording is enabled.
func TestCheckDirtyTreeProportional_RecordingEnabledDirtyManifestBlocks(t *testing.T) {
	root := t.TempDir()
	writeConfigYAML(t, root, "recording:\n  enabled: true\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestRel := filepath.ToSlash(filepath.Join(".mindspec", "specs", "010-test", "recording", "manifest.json"))
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", manifestRel)}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(blocking, []string{manifestRel}) {
		t.Errorf("blockingDirt = %v, want [%s]", blocking, manifestRel)
	}
	if len(ignored) != 0 {
		t.Errorf("ignoredDirt = %v, want empty", ignored)
	}
}

// The SAME dirty manifest for a DIFFERENT spec than the resolved target
// does not block — it isn't this claim's recording path.
func TestCheckDirtyTreeProportional_RecordingEnabledDirtyManifestForOtherSpecIgnored(t *testing.T) {
	root := t.TempDir()
	writeConfigYAML(t, root, "recording:\n  enabled: true\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	otherManifestRel := filepath.ToSlash(filepath.Join(".mindspec", "specs", "999-other", "recording", "manifest.json"))
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", otherManifestRel)}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty", blocking)
	}
	if !reflect.DeepEqual(ignored, []string{otherManifestRel}) {
		t.Errorf("ignoredDirt = %v, want [%s]", ignored, otherManifestRel)
	}
}

// Artifact-only dirt never even reaches the entanglement split — it
// normalizes away via bd export exactly as CheckDirtyTree already did.
func TestCheckDirtyTreeProportional_ArtifactAloneProceedsSilently(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{
		zRecord(" M", ".beads/issues.jsonl"),
		"",
	}}
	g.install(t)

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 || len(ignored) != 0 {
		t.Errorf("blocking=%v ignored=%v, want both empty (artifact-only dirt is auto-handled, ADR-0025)", blocking, ignored)
	}
	if !g.exportCalled {
		t.Error("expected bd export to be invoked when the artifact path is dirty")
	}
}

func TestCheckDirtyTreeProportional_PropagatesUnderlyingError(t *testing.T) {
	root := t.TempDir()
	g := &fakeGuardZ{porcelainErr: fmt.Errorf("git status boom")}
	g.install(t)

	_, _, err := CheckDirtyTreeProportional(root, root, "")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

// AC-8(iii): the seam-injected refusal leg. With a path injected into
// the test-residual and dirt on that path, the claim still refuses —
// the block machinery stays live and exercised even though the two
// dynamic classes are both quiescent here. An unrelated dirty path
// alongside it is still merely ignored, not blocking.
func TestCheckDirtyTreeProportional_InjectedEntanglementBlocks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRecord(" M", "entangled.txt") + zRecord(" M", "unrelated.txt")}}
	g.install(t)
	restore := SetEntangleableClaimPathsForTest([]string{"entangled.txt"})
	defer restore()

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
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
	failure := DirtyTreeFailure(root, blocking, "")
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

// Round-1 panel G3/G6: a rename/copy blocks when EITHER side intersects
// an entangleable path — the OLD name here.
func TestCheckDirtyTreeProportional_RenameAwayFromEntangledPathBlocks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRenameRecord("RM", "somewhere-else.txt", "entangled.txt")}}
	g.install(t)
	restore := SetEntangleableClaimPathsForTest([]string{"entangled.txt"})
	defer restore()

	blocking, _, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 1 || !strings.Contains(blocking[0], "entangled.txt") || !strings.Contains(blocking[0], "somewhere-else.txt") {
		t.Errorf("blockingDirt = %v, want a rename entry naming both entangled.txt (orig) and somewhere-else.txt (new)", blocking)
	}
}

// The symmetric case: a rename/copy blocks when the NEW name intersects
// an entangleable path.
func TestCheckDirtyTreeProportional_RenameIntoEntangledPathBlocks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRenameRecord("RM", "entangled.txt", "old-name.txt")}}
	g.install(t)
	restore := SetEntangleableClaimPathsForTest([]string{"entangled.txt"})
	defer restore()

	blocking, _, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 1 {
		t.Errorf("blockingDirt = %v, want exactly one blocking entry", blocking)
	}
}

// A rename that touches NEITHER side of any entangleable path is merely
// ignored, not blocking.
func TestCheckDirtyTreeProportional_UnrelatedRenameIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRenameRecord("RM", "new-name.txt", "old-name.txt")}}
	g.install(t)
	restore := SetEntangleableClaimPathsForTest([]string{"entangled.txt"})
	defer restore()

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty", blocking)
	}
	if len(ignored) != 1 {
		t.Errorf("ignoredDirt = %v, want exactly one ignored entry", ignored)
	}
}

// Round-1 panel G3/G6: a collapsed untracked directory blocks when it is
// an ANCESTOR of an entangleable path (git collapses an entirely
// untracked directory into ONE "?? dir/" entry rather than enumerating
// its contents).
func TestCheckDirtyTreeProportional_UntrackedDirAncestorOfEntangledPathBlocks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRecord("??", "somedir/")}}
	g.install(t)
	restore := SetEntangleableClaimPathsForTest([]string{"somedir/nested/entangled.txt"})
	defer restore()

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(blocking, []string{"somedir/"}) {
		t.Errorf("blockingDirt = %v, want [somedir/]", blocking)
	}
	if len(ignored) != 0 {
		t.Errorf("ignoredDirt = %v, want empty", ignored)
	}
}

// An untracked directory that is NOT an ancestor of any entangleable
// path stays merely ignored.
func TestCheckDirtyTreeProportional_UntrackedDirNotAncestorIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGuardZ{porcelainResponses: []string{zRecord("??", "unrelated-dir/")}}
	g.install(t)
	restore := SetEntangleableClaimPathsForTest([]string{"somedir/nested/entangled.txt"})
	defer restore()

	blocking, ignored, err := CheckDirtyTreeProportional(root, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocking) != 0 {
		t.Errorf("blockingDirt = %v, want empty", blocking)
	}
	if !reflect.DeepEqual(ignored, []string{"unrelated-dir/"}) {
		t.Errorf("ignoredDirt = %v, want [unrelated-dir/]", ignored)
	}
}

// --- IgnoredDirtNotice ---

// Round-1 panel G5: the notice's stable, identifying substrings are
// pinned so a silent rewording (or replacement with a bare path dump)
// goes red.
func TestIgnoredDirtNotice_PinsStableWording(t *testing.T) {
	msg := IgnoredDirtNotice([]string{"foo.txt"})
	for _, want := range []string{
		"claiming despite unrelated dirty path(s)",
		"(not blocked)",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("notice missing stable substring %q: %q", want, msg)
		}
	}
}

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
// render it unconditionally quoted so a hostile filename cannot forge
// control bytes, extra output lines, or (round-1 panel G3/G5/G6) a fake
// boundary between two DIFFERENT paths in the comma-joined list.
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

// Round-1 panel G3/G5/G6: a hostile path containing a literal ", " must
// not be able to masquerade as a boundary between two separate entries
// — each entry is quoted, so the reader can always find the true
// boundaries by matching un-escaped quote characters.
func TestIgnoredDirtNotice_CommaSpaceCannotForgeBoundary(t *testing.T) {
	hostile := "foo.txt, evil.txt"
	msg := IgnoredDirtNotice([]string{hostile, "real-second-path.txt"})
	if !strings.Contains(msg, `"foo.txt, evil.txt"`) {
		t.Errorf("expected the hostile path to be rendered as ONE quoted unit: %q", msg)
	}
	if !strings.Contains(msg, `"real-second-path.txt"`) {
		t.Errorf("expected the second path to be independently quoted: %q", msg)
	}
}
