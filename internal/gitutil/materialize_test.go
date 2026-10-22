package gitutil

// Spec 128 Bead 6: direct, dedicated unit tests for the prospective-
// snapshot protocol's git primitives (materialize.go). These pin each
// primitive's own contract independent of internal/complete's
// higher-level fixture legs (i)-(xx), which exercise the full
// M/R/S/G/T sequence end to end.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func gitStatusPorcelainT(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status in %s: %v\n%s", dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func lsTreeNames(t *testing.T, dir, tree string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "ls-tree", "-r", "--name-only", tree).Output()
	if err != nil {
		t.Fatalf("ls-tree -r --name-only %s: %v", tree, err)
	}
	return splitLines(string(out))
}

func blobAt(t *testing.T, dir, tree, path string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "show", tree+":"+path).Output()
	if err != nil {
		t.Fatalf("show %s:%s: %v", tree, path, err)
	}
	return string(out)
}

func containsPath(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// TestSnapshotWorktreeTree_SeedingPreservesExcludedArtifact is bead 6's
// round-2 G1 regression pin: an UNSEEDED temp index plus the exclusion
// would leave the tracked artifact ABSENT from T (nothing ever staged
// it back in). Seeded from HEAD first, the exclusion means
// "unchanged at tip" — the artifact survives in T byte-identical to its
// tip blob, even while an untracked file and a modified tracked file
// are picked up by the same snapshot.
func TestSnapshotWorktreeTree_SeedingPreservesExcludedArtifact(t *testing.T) {
	dir := initGitRepo(t)
	mustWrite(t, dir, ".beads/issues.jsonl", `{"id":"a"}`+"\n")
	mustWrite(t, dir, "internal/widget/widget.go", "package widget\n")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-q", "-m", "tip")

	// Modify a tracked file and add an untracked one, WITHOUT touching
	// the artifact.
	mustWrite(t, dir, "internal/widget/widget.go", "package widget\n\nfunc New() {}\n")
	mustWrite(t, dir, "internal/widget/new.go", "package widget\n\nfunc Extra() {}\n")

	tree, err := SnapshotWorktreeTree(dir, []string{".beads/issues.jsonl"})
	if err != nil {
		t.Fatalf("SnapshotWorktreeTree: %v", err)
	}

	names := lsTreeNames(t, dir, tree)
	if !containsPath(names, ".beads/issues.jsonl") {
		t.Fatalf("excluded artifact must survive in T (round-2 G1 regression), got tree entries: %v", names)
	}
	if got := blobAt(t, dir, tree, ".beads/issues.jsonl"); got != `{"id":"a"}`+"\n" {
		t.Errorf("excluded artifact must be byte-identical to its tip blob, got %q", got)
	}
	if !containsPath(names, "internal/widget/new.go") {
		t.Errorf("expected the untracked file to be staged into T, got entries: %v", names)
	}
	if got := blobAt(t, dir, tree, "internal/widget/widget.go"); !strings.Contains(got, "func New()") {
		t.Errorf("expected the modified tracked file's new content in T, got %q", got)
	}
}

// TestSnapshotWorktreeTree_WorktreeRealIndexAndRefsUntouched: the
// snapshot's ONLY effect is unreferenced loose objects. The real index
// (git status), HEAD, and every ref are byte-identical before and
// after.
func TestSnapshotWorktreeTree_WorktreeRealIndexAndRefsUntouched(t *testing.T) {
	dir := initGitRepo(t)
	mustWrite(t, dir, "tracked.txt", "modified but uncommitted\n")
	mustWrite(t, dir, "untracked.txt", "new\n")

	beforeStatus := gitStatusPorcelainT(t, dir)
	beforeHead := mustRevParse(t, dir, "HEAD")

	if _, err := SnapshotWorktreeTree(dir, nil); err != nil {
		t.Fatalf("SnapshotWorktreeTree: %v", err)
	}

	afterStatus := gitStatusPorcelainT(t, dir)
	afterHead := mustRevParse(t, dir, "HEAD")

	if beforeStatus != afterStatus {
		t.Errorf("real index/worktree status changed:\nbefore: %q\nafter:  %q", beforeStatus, afterStatus)
	}
	if beforeHead != afterHead {
		t.Errorf("HEAD moved: before=%s after=%s", beforeHead, afterHead)
	}
}

// TestChangedFilesTree_TreeVsCommit: the tree-vs-tree diff leg accepts
// a bare tree OID on the right and a commit-ish on the left,
// transparently — exactly the base(commit)..T(tree) shape the content
// gates measure.
func TestChangedFilesTree_TreeVsCommit(t *testing.T) {
	dir := initGitRepo(t)
	base := mustRevParse(t, dir, "HEAD")
	mustWrite(t, dir, "new-file.go", "package x\n")
	tree, err := SnapshotWorktreeTree(dir, nil)
	if err != nil {
		t.Fatalf("SnapshotWorktreeTree: %v", err)
	}
	files, err := ChangedFilesTree(dir, base, tree)
	if err != nil {
		t.Fatalf("ChangedFilesTree: %v", err)
	}
	if !containsPath(files, "new-file.go") {
		t.Errorf("expected new-file.go in the tree-vs-tree diff, got %v", files)
	}
}

// TestChangedFilesTree_IdenticalTreesAreEmpty: diffing a ref against
// its own tree-vs-tip snapshot with no worktree changes reports zero
// changed files — the elision precondition bead 6's complete.go
// machinery relies on to decide a stage is a no-op.
func TestChangedFilesTree_IdenticalTreesAreEmpty(t *testing.T) {
	dir := initGitRepo(t)
	tip := mustRevParse(t, dir, "HEAD")
	tree, err := SnapshotWorktreeTree(dir, nil)
	if err != nil {
		t.Fatalf("SnapshotWorktreeTree: %v", err)
	}
	files, err := ChangedFilesTree(dir, tip, tree)
	if err != nil {
		t.Fatalf("ChangedFilesTree: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected zero changed files for an unchanged worktree, got %v", files)
	}
}

func TestChangedFilesTree_ExactPathsAndRenameEndpoints(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	dir := initGitRepo(t)
	mustRunGit(t, dir, "config", "diff.renames", "true")
	const source = "user-source.txt"
	const destination = ".mindspec/specs/132-test/reviews/race/dispositions.jsonl"
	mustWrite(t, dir, source, "user bytes\n")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-qm", "source")
	base := mustRevParse(t, dir, "HEAD")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, destination)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, source), filepath.Join(dir, destination)); err != nil {
		t.Fatal(err)
	}
	want := []string{source, destination, ".beads/issues.jsonl ", ".beads/issues.jsonl\t", ".beads/issues.jsonl\n", " leading.txt", "unicode-雪.txt"}
	for _, path := range want[2:] {
		mustWrite(t, dir, path, "user bytes\n")
	}
	tree, err := SnapshotWorktreeTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ChangedFilesTree(dir, base, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Errorf("path count: got %q, want %q", got, want)
	}
	for _, path := range want {
		if !containsPath(got, path) {
			t.Errorf("missing exact path %q in %q", path, got)
		}
	}
	t.Logf("changed paths: %q", got)
}

// TestMaterializeStageCommit_TrailerRoundTrip: the pinned Mindspec-Stage
// trailer round-trips through ReadStageTrailer, and the commit is
// parented exactly as requested.
func TestMaterializeStageCommit_TrailerRoundTrip(t *testing.T) {
	dir := initGitRepo(t)
	parent := mustRevParse(t, dir, "HEAD")
	tree := mustRevParse(t, dir, "HEAD^{tree}")

	c, err := MaterializeStageCommit(dir, tree, parent, StageMaterialization, "mindspec: materialize bead completion")
	if err != nil {
		t.Fatalf("MaterializeStageCommit: %v", err)
	}
	stage, present, err := ReadStageTrailer(dir, c)
	if err != nil {
		t.Fatalf("ReadStageTrailer: %v", err)
	}
	if !present || stage != StageMaterialization {
		t.Errorf("expected stage=%q present=true, got stage=%q present=%v", StageMaterialization, stage, present)
	}
	if got := mustRevParse(t, dir, c+"^"); got != parent {
		t.Errorf("expected %s's parent to be %s, got %s", c, parent, got)
	}
}

// TestReadStageTrailer_ForeignCommitIsAbsentNotError: an ordinary
// commit with no Mindspec-Stage trailer reads back present==false with
// a nil error — never guessed at from its tree diff.
func TestReadStageTrailer_ForeignCommitIsAbsentNotError(t *testing.T) {
	dir := initGitRepo(t)
	stage, present, err := ReadStageTrailer(dir, "HEAD")
	if err != nil {
		t.Fatalf("ReadStageTrailer: %v", err)
	}
	if present || stage != "" {
		t.Errorf("expected an ordinary commit to carry no stage trailer, got stage=%q present=%v", stage, present)
	}
}

// TestReadStageTrailer_DuplicateIsAmbiguous is fix-round G2's pin: a
// commit carrying TWO Mindspec-Stage trailer lines — even consistent
// ones — is an AMBIGUOUS encoding (a well-formed write-ahead commit
// this protocol builds carries exactly one), reported as absent
// (present==false, stage=="") rather than resolved by picking either
// value. This is what routes a crafted/corrupt marker commit down the
// SAME foreign-clear path as a no-trailer commit in
// resolveMaterializationDiscriminator, never a resume.
func TestReadStageTrailer_DuplicateIsAmbiguous(t *testing.T) {
	dir := initGitRepo(t)
	mustRunGit(t, dir, "commit", "--allow-empty", "-q", "-m",
		"crafted subject\n\nMindspec-Stage: materialization\nMindspec-Stage: artifact-sync\n")
	stage, present, err := ReadStageTrailer(dir, "HEAD")
	if err != nil {
		t.Fatalf("ReadStageTrailer: %v", err)
	}
	if present || stage != "" {
		t.Errorf("expected a duplicate-trailer commit to read as ambiguous (absent), got stage=%q present=%v", stage, present)
	}
}

// TestReadStageTrailer_MalformedValueIsPresentButNotAKnownStage: a
// single trailer line with an unrecognized value is PRESENT (the
// encoding itself is well-formed — exactly one trailer) but names a
// value outside the closed {materialization, artifact-sync} set.
// Whether that value is a legitimate stage is resolveMaterializationDiscriminator's
// job (isKnownStage), not ReadStageTrailer's — this test pins that
// ReadStageTrailer itself does no validation of the VALUE, only of the
// trailer's cardinality.
func TestReadStageTrailer_MalformedValueIsPresentButNotAKnownStage(t *testing.T) {
	dir := initGitRepo(t)
	mustRunGit(t, dir, "commit", "--allow-empty", "-q", "-m",
		"crafted subject\n\nMindspec-Stage: some-bogus-value\n")
	stage, present, err := ReadStageTrailer(dir, "HEAD")
	if err != nil {
		t.Fatalf("ReadStageTrailer: %v", err)
	}
	if !present || stage != "some-bogus-value" {
		t.Errorf("expected present=true stage=%q (verbatim, unvalidated), got present=%v stage=%q", "some-bogus-value", present, stage)
	}
}

// TestReadMaterializationMarker_AbsentIsNotError: no marker ref yet
// published reads back present==false, err==nil — the same "genuinely
// absent" contract as RevParseRef's ErrRefNotFound, wrapped into a
// plain bool.
func TestReadMaterializationMarker_AbsentIsNotError(t *testing.T) {
	dir := initGitRepo(t)
	oid, stage, present, err := ReadMaterializationMarker(dir, "mindspec-test.1")
	if err != nil {
		t.Fatalf("ReadMaterializationMarker: %v", err)
	}
	if present || oid != "" || stage != "" {
		t.Errorf("expected an absent marker, got oid=%q stage=%q present=%v", oid, stage, present)
	}
}

// TestPublishMaterializationMarker_CreateFailsIfAlreadyExists: the
// absent→C leg's CREATE semantics (empty expectedOld) atomically fail
// when the marker already exists — one invocation can never overwrite
// another's recovery evidence.
func TestPublishMaterializationMarker_CreateFailsIfAlreadyExists(t *testing.T) {
	dir := initGitRepo(t)
	c := mustRevParse(t, dir, "HEAD")
	beadID := "mindspec-test.1"

	if err := PublishMaterializationMarker(dir, beadID, c, ""); err != nil {
		t.Fatalf("first CREATE publish: %v", err)
	}
	// A second CREATE attempt (simulating a concurrent invocation) must
	// fail — the marker already exists.
	if err := PublishMaterializationMarker(dir, beadID, c, ""); err == nil {
		t.Fatal("expected the second CREATE publish to fail (marker already exists)")
	}
	// The first invocation's marker OID must survive byte-identical.
	oid, _, present, err := ReadMaterializationMarker(dir, beadID)
	if err != nil || !present || oid != c {
		t.Errorf("expected the marker to remain %s, got oid=%q present=%v err=%v", c, oid, present, err)
	}
}

// TestPublishMaterializationMarker_ExpectedOldMismatchFails: the C→D
// leg's expected-old CAS refuses when the marker's current value
// doesn't match, without disturbing the existing marker.
func TestPublishMaterializationMarker_ExpectedOldMismatchFails(t *testing.T) {
	dir := initGitRepo(t)
	c := mustRevParse(t, dir, "HEAD")
	beadID := "mindspec-test.1"
	if err := PublishMaterializationMarker(dir, beadID, c, ""); err != nil {
		t.Fatalf("CREATE publish: %v", err)
	}
	if err := PublishMaterializationMarker(dir, beadID, "0123456789012345678901234567890123456789", "not-the-real-old-value"); err == nil {
		t.Fatal("expected the expected-old mismatch to fail")
	}
	oid, _, present, err := ReadMaterializationMarker(dir, beadID)
	if err != nil || !present || oid != c {
		t.Errorf("expected the marker to remain %s after the refused CAS, got oid=%q present=%v err=%v", c, oid, present, err)
	}
}

// TestClearMaterializationMarker_Idempotent: clearing with expectedOID
// == "" (nothing believed to exist) is a no-op success; CAS-clearing a
// present marker against its OWN observed OID removes it.
func TestClearMaterializationMarker_Idempotent(t *testing.T) {
	dir := initGitRepo(t)
	beadID := "mindspec-test.1"
	if err := ClearMaterializationMarker(dir, beadID, ""); err != nil {
		t.Fatalf("clearing with expectedOID==\"\" must no-op, got: %v", err)
	}
	c := mustRevParse(t, dir, "HEAD")
	if err := PublishMaterializationMarker(dir, beadID, c, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := ClearMaterializationMarker(dir, beadID, c); err != nil {
		t.Fatalf("clearing a present marker (CAS against its own OID): %v", err)
	}
	_, _, present, err := ReadMaterializationMarker(dir, beadID)
	if err != nil || present {
		t.Errorf("expected the marker to be gone, present=%v err=%v", present, err)
	}
	// Clearing the now-absent marker again (idempotent: this call's own
	// effect) with the SAME expectedOID must still succeed.
	if err := ClearMaterializationMarker(dir, beadID, c); err != nil {
		t.Errorf("re-clearing an already-absent marker (idempotent) must not error, got: %v", err)
	}
}

// TestClearMaterializationMarker_CASMismatchFails: clearing against a
// STALE expectedOID (one the marker no longer holds) fails and leaves
// the CURRENT marker untouched — a clear can never delete evidence it
// does not own.
func TestClearMaterializationMarker_CASMismatchFails(t *testing.T) {
	dir := initGitRepo(t)
	beadID := "mindspec-test.1"
	c := mustRevParse(t, dir, "HEAD")
	if err := PublishMaterializationMarker(dir, beadID, c, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	staleOID := "0123456789012345678901234567890123456789"
	if err := ClearMaterializationMarker(dir, beadID, staleOID); err == nil {
		t.Fatal("expected a CAS mismatch to fail the clear")
	}
	oid, _, present, err := ReadMaterializationMarker(dir, beadID)
	if err != nil || !present || oid != c {
		t.Errorf("expected the marker to remain %s after the refused CAS clear, got oid=%q present=%v err=%v", c, oid, present, err)
	}
}

// TestClearMaterializationMarker_InterleavingCannotDeleteNewerMarker is
// fix-round G4's interleaving pin: an invocation that observed marker X
// (captured its OID early) must NOT be able to delete a marker Y a
// LATER, concurrent invocation has since published — even though, by
// the time the stale clear runs, some marker again exists (Y, not X).
func TestClearMaterializationMarker_InterleavingCannotDeleteNewerMarker(t *testing.T) {
	dir := initGitRepo(t)
	beadID := "mindspec-test.1"
	x := mustRevParse(t, dir, "HEAD")
	if err := PublishMaterializationMarker(dir, beadID, x, ""); err != nil {
		t.Fatalf("publish X: %v", err)
	}
	// This invocation observed X early (as a stale clear would).
	staleObservedOID := x

	// A "concurrent invocation" clears X and publishes a NEW marker Y in
	// between — genuinely CAS-clearing X first (as a real invocation
	// would), then publishing Y fresh.
	if err := ClearMaterializationMarker(dir, beadID, x); err != nil {
		t.Fatalf("concurrent invocation's own clear of X: %v", err)
	}
	mustWrite(t, dir, "extra.txt", "y\n")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-q", "-m", "second")
	y := mustRevParse(t, dir, "HEAD")
	if err := PublishMaterializationMarker(dir, beadID, y, ""); err != nil {
		t.Fatalf("publish Y: %v", err)
	}

	// The STALE clear (still holding its early-observed X) must fail —
	// it must NOT delete Y.
	if err := ClearMaterializationMarker(dir, beadID, staleObservedOID); err == nil {
		t.Fatal("expected the stale clear (observed X) to fail rather than delete the newer marker Y")
	}
	oid, _, present, err := ReadMaterializationMarker(dir, beadID)
	if err != nil || !present || oid != y {
		t.Errorf("expected marker Y (%s) to survive the stale clear untouched, got oid=%q present=%v err=%v", y, oid, present, err)
	}
}

// TestAdvanceBranchExpectedOld_MismatchRefused: a concurrent mover
// (simulated: an already-moved branch) makes the CAS fail rather than
// clobber.
func TestAdvanceBranchExpectedOld_MismatchRefused(t *testing.T) {
	dir := initGitRepo(t)
	a := mustRevParse(t, dir, "HEAD")
	mustWrite(t, dir, "extra.txt", "x\n")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-q", "-m", "second")
	realTip := mustRevParse(t, dir, "HEAD")

	// Attempt to advance "main" expecting it to still be at `a`, when it
	// has actually already moved to realTip.
	if err := AdvanceBranchExpectedOld(dir, "main", "0123456789012345678901234567890123456789", a); err == nil {
		t.Fatal("expected the stale expected-old to be refused")
	}
	if got := mustRevParse(t, dir, "main"); got != realTip {
		t.Errorf("branch must be untouched by the refused CAS, got %s want %s", got, realTip)
	}
}

// TestResetIndexOnlyMixed_ClearsPhantomStagedDeletion: bead 6's round-5
// G2 mechanic. Advancing a branch ref directly to a commit-tree-built
// commit (never checked out via `git commit`) leaves the real index
// stale — `git status` shows the newly-tracked path as a staged
// deletion AND an untracked addition. `ResetIndexOnlyMixed` clears that
// without touching the worktree file itself.
func TestResetIndexOnlyMixed_ClearsPhantomStagedDeletion(t *testing.T) {
	dir := initGitRepo(t)
	a := mustRevParse(t, dir, "HEAD")
	mustWrite(t, dir, "new-tracked.go", "package x\n")
	tree, err := SnapshotWorktreeTree(dir, nil)
	if err != nil {
		t.Fatalf("SnapshotWorktreeTree: %v", err)
	}
	c, err := MaterializeStageCommit(dir, tree, a, StageMaterialization, "mindspec: materialize bead completion")
	if err != nil {
		t.Fatalf("MaterializeStageCommit: %v", err)
	}
	if err := AdvanceBranchExpectedOld(dir, "main", c, a); err != nil {
		t.Fatalf("AdvanceBranchExpectedOld: %v", err)
	}

	preReset := gitStatusPorcelainT(t, dir)
	if !strings.Contains(preReset, "new-tracked.go") {
		t.Fatalf("expected the pre-reset index to be out of sync (phantom entries) for new-tracked.go, got status:\n%s", preReset)
	}

	if err := ResetIndexOnlyMixed(dir); err != nil {
		t.Fatalf("ResetIndexOnlyMixed: %v", err)
	}
	postReset := gitStatusPorcelainT(t, dir)
	if postReset != "" {
		t.Errorf("expected a clean status after the index-only reset, got:\n%s", postReset)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "new-tracked.go")); err != nil || string(data) != "package x\n" {
		t.Errorf("expected the worktree file to survive the index-only reset untouched, got data=%q err=%v", data, err)
	}
}

// --- shared test helpers -----------------------------------------------

func mustWrite(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", rel, err)
	}
}

func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func mustRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func TestSnapshotWorktreeTree_LedgerGlobMatchesArtifactMatcher(t *testing.T) {
	dir := initGitRepo(t)
	reviews := ".mindspec/specs/130-test/reviews/p/"
	ledger := reviews + "dispositions.jsonl"
	nested := reviews + "nested/dispositions.jsonl"
	registration := reviews + "panel.json"
	for _, file := range []string{ledger, nested, registration} {
		mustWrite(t, dir, file, "{}\n")
	}
	tree, err := SnapshotWorktreeTree(dir, panel.ToolArtifactPaths(filepath.Join(dir, ".mindspec", "specs")))
	if err != nil {
		t.Fatal(err)
	}
	names := lsTreeNames(t, dir, tree)
	if containsPath(names, ledger) {
		t.Errorf("matched untracked ledger entered snapshot: %v", names)
	}
	for _, file := range []string{nested, registration} {
		if !containsPath(names, file) {
			t.Errorf("non-artifact %s missing from snapshot: %v", file, names)
		}
	}
}
