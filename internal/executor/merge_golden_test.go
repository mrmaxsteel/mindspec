package executor

// Spec 127 bead 1 (AC-8(i), R4(d)'s "common path untouched" leg, R6's
// Testing Strategy): the ordinary-merge golden. A committed, byte-exact
// transcript of an ORDINARY bead→spec merge through PRODUCTION
// CompleteBead, captured BEFORE any of this spec's later beads touch a
// merge producer — the baseline every later bead's own tests replay
// against to prove the common path never regressed.
//
// Determinism, applied identically at capture and replay (plan-gate
// P2-plan-4): every git invocation in this fixture — including
// CompleteBead's OWN internal git subprocess calls, which inherit the
// test process's environment because internal/gitutil's exec.Cmd.Env is
// left nil — runs under GIT_CONFIG_GLOBAL=/dev/null + GIT_CONFIG_NOSYSTEM=1
// (so a developer's global commit.gpgsign=true, or any other commit-
// object-altering config, can never leak into the captured bytes), a
// SCRUBBED set of Git plumbing/config/template environment inputs that
// can override those two in-process (spec 127 bead-1 fix round, G1-4;
// fix round 2, O3c-1/G1-4 added GIT_TEMPLATE_DIR and GIT_DEFAULT_HASH —
// see buildAC8iDeterministicEnv's doc comment), an explicit
// --object-format=sha1 on the fixture's own `git init` (so the OID width
// itself cannot vary), and a FIXED GIT_AUTHOR_*/GIT_COMMITTER_* identity
// and date. With tree content, parents, message, identity, date, and
// object format all pinned, the resulting merge commit's SHA is a pure
// function of this file's fixture — reproducible byte-for-byte across
// every hostile-env vector this bead has found and fixtured (see
// TestAC8i_OrdinaryMergeGolden_HermeticAgainstHostileEnv), though
// buildAC8iDeterministicEnv's own doc comment states the one residual
// class this scrub-list approach cannot rule out by construction: a
// future git env var this file's authors have not yet named. The fixture
// root (a t.TempDir() path, different every run) is templated out of the
// captured transcript before comparison.
//
// The capture (ac8iCapture) records the merge commit's OWN SHA
// (MergeCommitSHA, %H) and its parent tips (ParentTips, %P) — not merely
// a parent COUNT — plus the result tree's OID (ResultTreeOID, %T) in
// addition to its path listing (spec 127 bead-1 fix round,
// O3-1/G1-3/F1/O2-3: the delivered golden recorded subject, parent COUNT,
// and path NAMES only, which a `go test -overlay` mutation proved blind
// to a change in the bead work's own content, its parent tips, or a
// leaked commit.gpgsign=true — none of the omitted fields, verified
// empirically deterministic under this fixture's pinned env). Regenerated
// in THIS bead specifically because a golden's evidentiary value is that
// it was captured BEFORE the change it guards; retrofitting the field
// later would capture post-change bytes and prove nothing about
// pre-change behavior.
//
// TestAC8i_OrdinaryMergeGolden_IsDeterministic proves the determinism
// claim directly (builds the fixture TWICE, in two independent temp
// dirs, and asserts the normalized captures — including the merge SHA
// itself — are byte-identical) rather than merely asserting it in a
// comment.
// TestAC8i_OrdinaryMergeGolden_ReplayMatchesCommittedGolden is the actual
// AC-8(i) golden replay against internal/executor/testdata/
// ac8i_ordinary_merge_golden.json.
// TestAC8i_OrdinaryMergeGolden_DetectsContentDrift and
// TestAC8i_OrdinaryMergeGolden_HermeticAgainstHostileEnv are this golden's
// own falsifiability checks (never taken on trust): they prove the
// captured fields actually discriminate a content change and that the
// determinism claim survives a hostile ambient environment, respectively.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const ac8iGoldenPath = "testdata/ac8i_ordinary_merge_golden.json"

// ac8iFixtureRootPlaceholder replaces the fixture's own temp-dir root in
// the captured transcript, so two runs in two different t.TempDir()s
// compare equal, and so the committed golden never embeds a path private
// to the machine that captured it.
const ac8iFixtureRootPlaceholder = "<AC8I_FIXTURE_ROOT>"

// ac8iCapture is the AC-8(i) transcript shape: exit code (success/failure
// as CompleteBead's returned error, nil-or-not), normalized stdout/stderr,
// the merge commit's own SHA, subject, parent count AND tips, the result
// tree's OID, and the resulting spec-branch tree's full path listing
// (spec 127 bead-1 fix round, O3-1/G1-3/F1/O2-3 — see the package doc
// comment for why MergeCommitSHA/ParentTips/ResultTreeOID were added).
type ac8iCapture struct {
	Provenance struct {
		Description string `json:"description"`
		CapturedAt  string `json:"captured_at_tree"`
	} `json:"provenance"`
	ExitCode        int      `json:"exit_code"`
	Stdout          string   `json:"stdout"`
	Stderr          string   `json:"stderr"`
	MergeCommitSHA  string   `json:"merge_commit_sha"`
	MergeSubject    string   `json:"merge_commit_subject"`
	ParentCount     int      `json:"parent_count"`
	ParentTips      []string `json:"parent_tips"`
	ResultTreeOID   string   `json:"result_tree_oid"`
	ResultTreePaths []string `json:"result_tree_paths"`
}

// ac8iRunGit runs a git command with NO explicit Env override — it
// inherits the test process's environment (which
// buildAC8iDeterministicEnv below has already pinned via t.Setenv), the
// exact same inheritance CompleteBead's own internal git calls rely on
// (internal/gitutil's exec.Cmd.Env is left nil throughout).
func ac8iRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s (%v)", args, out, err)
	}
	return string(out)
}

// ac8iScrubEnv unsets key from the process environment for the duration
// of t, restoring its prior value (or absence) via t.Cleanup — spec 127
// bead-1 fix round, G1-4: t.Setenv cannot express "no value at all". An
// empty string is itself a value some of these vars treat as a HARD
// FAILURE, not "unset" (verified: GIT_INDEX_FILE="" makes git fail with
// "fatal: unable to write new index file", not fall back to the repo's
// real index) — so this does the save/restore os.Setenv/os.Unsetenv
// dance t.Setenv performs internally, but for genuine removal.
func ac8iScrubEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// buildAC8iDeterministicEnv pins every env var that can influence a
// commit object's bytes (identity, date) or git's own config resolution
// (GIT_CONFIG_GLOBAL/GIT_CONFIG_NOSYSTEM — isolating a developer's or
// CI runner's own global/system git config, e.g. commit.gpgsign=true,
// from ever reaching the captured commit). t.Setenv scopes these to the
// test and its subtests only.
//
// Scrubs Git plumbing/config/repository-location environment inputs
// FIRST (spec 127 bead-1 fix round, G1-4): the golden's determinism claim
// is not actually hermetic against an INHERITED caller environment —
// GIT_CONFIG_GLOBAL=/dev/null + GIT_CONFIG_NOSYSTEM=1 isolate FILE-based
// config only, not env/CLI-injected config, and neither touches
// GIT_INDEX_FILE at all. Verified both ways: `env
// GIT_INDEX_FILE=/tmp/caller-index go test ...` corrupted the SECOND
// independent build in the determinism test (both builds shared one
// caller-owned index across independent repositories); `env
// GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=commit.gpgsign
// GIT_CONFIG_VALUE_0=true go test ...` made git attempt GPG signing.
// GIT_INDEX_FILE/GIT_DIR/GIT_WORK_TREE/GIT_OBJECT_DIRECTORY/
// GIT_ALTERNATE_OBJECT_DIRECTORIES/GIT_COMMON_DIR are repository/
// object/worktree location overrides; GIT_CONFIG_PARAMETERS/
// GIT_CONFIG_COUNT plus every GIT_CONFIG_KEY_N/GIT_CONFIG_VALUE_N pair
// are CLI-equivalent config injection (git's own `-c` mechanism, exposed
// via env for exactly this kind of ambient-inheritance hazard).
// GIT_TEMPLATE_DIR (spec 127 bead-1 fix round 2, O3c-1) is a THIRD config
// tier neither GIT_CONFIG_GLOBAL=/dev/null nor GIT_CONFIG_NOSYSTEM=1 can
// isolate: `git init` copies the template directory's `config` file into
// the NEW repository's LOCAL config, which beats both of those defences
// (verified: a template config setting commit.gpgsign=true made the
// fixture's very first `git commit --allow-empty` fail with a GPG-signing
// error, before CompleteBead ever ran).
//
// Each scrubbed/pinned name here is a DENY-LIST entry, not drawn from an
// explicit allowlist (spec 127 bead-1 fix round 2, ruling 4/ruling 1's
// same inversion principle applied here too) — this function has now
// missed a distinct git config-injection vector on TWO separate
// occasions (GIT_TEMPLATE_DIR here; GIT_DEFAULT_HASH below), which is
// exactly the pattern ruling 1 named for diffNameStatusBuckets' status
// switch. An allowlist-based child environment (keep a small known-good
// set, scrub everything else by default) would exclude the NEXT unknown
// git env var without this function's authors having to anticipate it by
// name first; it is not attempted here because CompleteBead's own git
// calls inherit the FULL test-process environment by design (internal/
// gitutil's exec.Cmd.Env is left nil), so building a true allowlist would
// mean reconstructing the test binary's entire ambient environment
// (PATH, HOME/USERPROFILE, TMPDIR, locale, proxy/cert vars a CI runner's
// git may need) rather than scrubbing a handful of named git-specific
// variables — a materially larger, riskier change than this fix round's
// scope. Left as a known, stated limitation rather than a silently
// narrower deny-list.
func buildAC8iDeterministicEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"GIT_INDEX_FILE", "GIT_DIR", "GIT_WORK_TREE",
		"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_COMMON_DIR", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
		"GIT_TEMPLATE_DIR",
	} {
		ac8iScrubEnv(t, key)
	}
	for _, kv := range os.Environ() {
		key := strings.SplitN(kv, "=", 2)[0]
		if strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			ac8iScrubEnv(t, key)
		}
	}

	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "mindspec-golden")
	t.Setenv("GIT_AUTHOR_EMAIL", "golden@mindspec.test")
	t.Setenv("GIT_AUTHOR_DATE", "2020-01-01T00:00:00+00:00")
	t.Setenv("GIT_COMMITTER_NAME", "mindspec-golden")
	t.Setenv("GIT_COMMITTER_EMAIL", "golden@mindspec.test")
	t.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00+00:00")
	// GIT_DEFAULT_HASH (spec 127 bead-1 fix round 2, G1-4): a sha256-
	// defaulting machine or ambient env changes every OID this fixture
	// captures (MergeCommitSHA, ParentTips, ResultTreeOID all become
	// 64-hex instead of 40-hex). Pinned here as defense-in-depth for any
	// OTHER `git init` this fixture's dependency chain might one day
	// call without an explicit --object-format; the load-bearing
	// guarantee is buildAC8iOrdinaryMergeFixtureWithContent's own `git
	// init --object-format=sha1` below, which overrides this env var
	// (and any ambient init.defaultObjectFormat config) for THIS
	// fixture's actual repository regardless.
	t.Setenv("GIT_DEFAULT_HASH", "sha1")
}

// ac8iDefaultBeadContent is the golden's own bead-work content — pulled
// out as a named constant so
// TestAC8i_OrdinaryMergeGolden_DetectsContentDrift can build a SECOND
// fixture differing only in this one byte string, without duplicating
// the rest of the builder.
const ac8iDefaultBeadContent = "ac8i golden bead work\n"

// buildAC8iOrdinaryMergeFixture builds the fixture with the golden's own
// default bead-work content — see buildAC8iOrdinaryMergeFixtureWithContent.
func buildAC8iOrdinaryMergeFixture(t *testing.T) ac8iCapture {
	t.Helper()
	return buildAC8iOrdinaryMergeFixtureWithContent(t, ac8iDefaultBeadContent)
}

// buildAC8iOrdinaryMergeFixtureWithContent builds the fixture in a fresh
// temp dir and drives PRODUCTION CompleteBead over it (msg=="" — the
// merge-only path; no auto-commit/export leg, so no bd/PATH dependency at
// all), returning the normalized capture. The bead work content is a
// parameter (spec 127 bead-1 fix round: factored out so
// TestAC8i_OrdinaryMergeGolden_DetectsContentDrift can build a variant
// fixture without duplicating this builder) — everything else about the
// fixture is fixed, not random, so two builds with the SAME content are
// identical every time — the whole point of the determinism check below.
func buildAC8iOrdinaryMergeFixtureWithContent(t *testing.T, beadContent string) ac8iCapture {
	t.Helper()
	buildAC8iDeterministicEnv(t)

	dir := t.TempDir()
	// --object-format=sha1 (spec 127 bead-1 fix round 2, G1-4) is the
	// LOAD-BEARING guarantee against a sha256-defaulting machine or
	// ambient GIT_DEFAULT_HASH/init.defaultObjectFormat: it fixes THIS
	// repository's object format at creation time regardless of the
	// caller's environment or config, and every subsequent git command in
	// this fixture (including CompleteBead's own) operates on this
	// already-initialized repository, so it inherits sha1 from the repo
	// itself — never re-derived from ambient env per-command.
	ac8iRunGit(t, dir, "init", "-q", "-b", "main", "--object-format=sha1")
	ac8iRunGit(t, dir, "commit", "--allow-empty", "-m", "root")

	const specID = "999-ac8i-golden"
	const specBranch = "spec/" + specID
	const beadID = "mindspec-ac8igolden.1"
	const beadBranch = "bead/" + beadID

	ac8iRunGit(t, dir, "branch", specBranch)

	// Bead work, authored in a temporary worktree (removed before
	// CompleteBead runs, matching the real fleet shape — the bead branch
	// survives unchecked-out) so the merge below is a genuine two-parent
	// merge of REAL, committed content.
	ac8iRunGit(t, dir, "branch", beadBranch, specBranch)
	beadWt := filepath.Join(dir, ".wt-bead-ac8i")
	ac8iRunGit(t, dir, "worktree", "add", beadWt, beadBranch)
	if err := os.WriteFile(filepath.Join(beadWt, "bead-work.txt"), []byte(beadContent), 0o644); err != nil {
		t.Fatalf("write bead-work.txt: %v", err)
	}
	ac8iRunGit(t, beadWt, "add", ".")
	ac8iRunGit(t, beadWt, "commit", "-m", "bead work")
	ac8iRunGit(t, dir, "worktree", "remove", "--force", beadWt)

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-"+specID)
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir .worktrees: %v", err)
	}
	ac8iRunGit(t, dir, "worktree", "add", specWtPath, specBranch)
	t.Cleanup(func() {
		_, _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).CombinedOutput()
	})

	// Production callers invoke the executor from the repo root
	// (gitutil's BranchExists/DeleteBranch operate on $PWD) — the same
	// invariant every other in-package executor test reproduces.
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	fake := &fakeWorktreeOps{}
	stubMergeBindingSeams(t)
	g := &MindspecExecutor{Root: dir, WorktreeOps: fake}

	stdout, stderr, callErr := ac8iCaptureOutput(func() error {
		return g.CompleteBead(beadID, specBranch, "", "", false)
	})

	exitCode := 0
	if callErr != nil {
		exitCode = 1
	}

	mergeSHA := strings.TrimSpace(ac8iRunGit(t, dir, "rev-parse", specBranch))
	subject := strings.TrimSpace(ac8iRunGit(t, dir, "log", "-1", "--format=%s", mergeSHA))
	parentsRaw := strings.TrimSpace(ac8iRunGit(t, dir, "log", "-1", "--format=%P", mergeSHA))
	parents := strings.Fields(parentsRaw)
	treeOID := strings.TrimSpace(ac8iRunGit(t, dir, "log", "-1", "--format=%T", mergeSHA))

	treeOut := ac8iRunGit(t, dir, "ls-tree", "-r", "--name-only", mergeSHA)
	var paths []string
	for _, line := range strings.Split(treeOut, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	sort.Strings(paths)

	var cap ac8iCapture
	cap.Provenance.Description = "spec 127 bead 1 AC-8(i): the ordinary bead->spec merge golden, captured before any producer change."
	cap.Provenance.CapturedAt = "spec 127 bead 1 (mindspec-2vtk.1), descended from main@09f62bd9"
	cap.ExitCode = exitCode
	cap.Stdout = ac8iNormalize(stdout, dir)
	cap.Stderr = ac8iNormalize(stderr, dir)
	cap.MergeCommitSHA = mergeSHA
	cap.MergeSubject = subject
	cap.ParentCount = len(parents)
	cap.ParentTips = parents
	cap.ResultTreeOID = treeOID
	cap.ResultTreePaths = paths
	return cap
}

// ac8iNormalize templates the fixture's own temp-dir root out of s.
func ac8iNormalize(s, root string) string {
	return strings.ReplaceAll(s, root, ac8iFixtureRootPlaceholder)
}

// ac8iCaptureOutput redirects os.Stdout/os.Stderr for the duration of f,
// returning what was written to each plus f's own error.
func ac8iCaptureOutput(f func() error) (stdout, stderr string, err error) {
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW

	fErr := f()

	os.Stdout, os.Stderr = origOut, origErr
	_ = outW.Close()
	_ = errW.Close()
	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(outR)
	_, _ = errBuf.ReadFrom(errR)
	return outBuf.String(), errBuf.String(), fErr
}

// TestAC8i_OrdinaryMergeGolden_IsDeterministic builds the fixture TWICE,
// in two independent temp dirs, and asserts the normalized captures —
// INCLUDING the merge commit's own SHA, parent tips, and result tree OID,
// all of which depend on git's own SHA1 hashing of pinned tree/parent/
// identity/date data — are byte-identical. This is the direct empirical
// proof behind the package doc comment's determinism claim, not an
// assertion taken on faith (spec 127 bead-1 fix round, O3-1/O2-3: the
// prior version of this test's own comment claimed to compare
// hash-dependent fields, but its capture at the time carried none — a
// subject string and an integer count depend on no hashing at all).
func TestAC8i_OrdinaryMergeGolden_IsDeterministic(t *testing.T) {
	first := buildAC8iOrdinaryMergeFixture(t)
	second := buildAC8iOrdinaryMergeFixture(t)

	firstJSON, err := json.MarshalIndent(first, "", "  ")
	if err != nil {
		t.Fatalf("marshal first capture: %v", err)
	}
	secondJSON, err := json.MarshalIndent(second, "", "  ")
	if err != nil {
		t.Fatalf("marshal second capture: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("the fixture is not deterministic across two independent builds:\nfirst:\n%s\nsecond:\n%s", firstJSON, secondJSON)
	}
}

// TestAC8i_OrdinaryMergeGolden_ReplayMatchesCommittedGolden is the actual
// AC-8(i) pin: replay the fixture on THIS tree and assert byte-identity
// against the committed golden captured at the bead-1 tree (descended
// from 09f62bd9, before any producer change — O2-r2-7). A later bead that
// changes CompleteBead's ordinary-merge behavior must fail this test
// unless it deliberately regenerates the golden (a visible, reviewed
// diff) — R4(d)'s "common path untouched" leg, machine-checked.
func TestAC8i_OrdinaryMergeGolden_ReplayMatchesCommittedGolden(t *testing.T) {
	// Captured BEFORE buildAC8iOrdinaryMergeFixture: the builder chdirs
	// into its fixture repo and only restores cwd via t.Cleanup, which
	// (registered on this same *testing.T) does not run until THIS test
	// function returns — so the golden path must be resolved to an
	// absolute path against the package directory now, not read as a
	// bare relative path after the builder returns.
	startDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	goldenAbsPath := filepath.Join(startDir, ac8iGoldenPath)

	got := buildAC8iOrdinaryMergeFixture(t)
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal captured golden: %v", err)
	}
	gotJSON = append(gotJSON, '\n')

	want, err := os.ReadFile(goldenAbsPath)
	if err != nil {
		t.Fatalf("reading committed golden %s: %v", goldenAbsPath, err)
	}

	if string(gotJSON) != string(want) {
		t.Errorf("replayed AC-8(i) golden does NOT match the committed golden (%s).\nThis means CompleteBead's ordinary-merge behavior changed — if that is INTENTIONAL, regenerate the golden deliberately and review the diff; it must never be greened by silently overwriting it.\n--- got ---\n%s\n--- want ---\n%s", ac8iGoldenPath, gotJSON, want)
	}
}

// TestAC8i_OrdinaryMergeGolden_DetectsContentDrift is the golden's own
// falsifiability check (spec 127 bead-1 fix round, O3-1/G1-3/F1/O2-3):
// the exact mutation class O3 used via `go test -overlay` to prove the
// PRIOR capture (subject + parent count + path names only) was blind to
// a content change at the same path — reproduced here as a committed
// regression instead of a one-off adversarial probe. Two builds that
// differ ONLY in the bead-work file's CONTENT (same path, "bead-work.txt"
// both times) must produce a different MergeCommitSHA, different
// ParentTips (the bead commit's own tip SHA changes when its tree
// changes), and a different ResultTreeOID — and therefore a different
// full JSON capture, so a committed golden captured under one content
// value can never spuriously match a replay built under a different one.
func TestAC8i_OrdinaryMergeGolden_DetectsContentDrift(t *testing.T) {
	baseline := buildAC8iOrdinaryMergeFixtureWithContent(t, ac8iDefaultBeadContent)
	mutated := buildAC8iOrdinaryMergeFixtureWithContent(t, "TOTALLY DIFFERENT MERGED CONTENT\n")

	if baseline.MergeCommitSHA == "" || mutated.MergeCommitSHA == "" {
		t.Fatal("fixture invariant broken: MergeCommitSHA must be populated on both builds")
	}
	if baseline.MergeCommitSHA == mutated.MergeCommitSHA {
		t.Error("MergeCommitSHA must differ when the bead work's content differs — the golden must not be blind to a same-path content change")
	}
	if len(baseline.ParentTips) != 2 || len(mutated.ParentTips) != 2 {
		t.Fatalf("fixture invariant broken: ParentTips must have 2 entries on an ordinary two-parent merge, got baseline=%v mutated=%v", baseline.ParentTips, mutated.ParentTips)
	}
	if reflect.DeepEqual(baseline.ParentTips, mutated.ParentTips) {
		t.Error("ParentTips must differ — the bead branch's own tip commit SHA changes when its tree content changes")
	}
	if baseline.ResultTreeOID == mutated.ResultTreeOID {
		t.Error("ResultTreeOID must differ — the merged tree's content differs")
	}
	if baseline.ResultTreePaths[0] != mutated.ResultTreePaths[0] {
		t.Fatalf("fixture invariant broken: both builds must touch the SAME path (bead-work.txt) — this test's whole point is a same-path content change, not a same-path-different-name one; got baseline=%v mutated=%v", baseline.ResultTreePaths, mutated.ResultTreePaths)
	}

	baselineJSON, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		t.Fatalf("marshal baseline: %v", err)
	}
	mutatedJSON, err := json.MarshalIndent(mutated, "", "  ")
	if err != nil {
		t.Fatalf("marshal mutated: %v", err)
	}
	if string(baselineJSON) == string(mutatedJSON) {
		t.Error("the full captures must differ — a committed golden must not stay green across a content change at the same path")
	}
}

// TestAC8i_OrdinaryMergeGolden_HermeticAgainstHostileEnv is G1-4's own
// regression: reproduces BOTH hostile-environment vectors G1 used to
// break the fixture's determinism claim (a caller-owned GIT_INDEX_FILE
// pointing elsewhere; a GIT_CONFIG_COUNT/KEY/VALUE triple forcing
// commit.gpgsign=true) and asserts the golden still replays successfully
// and byte-identically against the committed golden, because
// buildAC8iDeterministicEnv now scrubs both classes before either the
// fixture's own git commands or CompleteBead's inherited-env production
// calls run.
//
// Extended (spec 127 bead-1 fix round 2, G1-4/O3c-1's own hostile-env
// legs, added to the SAME test rather than duplicated so all four hostile
// vectors are asserted against one shared replay): a hostile
// GIT_DEFAULT_HASH=sha256 (would otherwise make every OID this fixture
// captures 64-hex instead of 40-hex) and a hostile GIT_TEMPLATE_DIR
// pointing at a template whose `config` sets commit.gpgsign=true (O3c-1's
// verified vector — it defeats the fixture at its very FIRST commit,
// before CompleteBead runs, if unscrubbed).
func TestAC8i_OrdinaryMergeGolden_HermeticAgainstHostileEnv(t *testing.T) {
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "caller-owned-index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "commit.gpgsign")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
	t.Setenv("GIT_DEFAULT_HASH", "sha256")

	templateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(templateDir, "config"), []byte("[commit]\n\tgpgsign = true\n"), 0o644); err != nil {
		t.Fatalf("writing hostile GIT_TEMPLATE_DIR config: %v", err)
	}
	t.Setenv("GIT_TEMPLATE_DIR", templateDir)

	startDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	goldenAbsPath := filepath.Join(startDir, ac8iGoldenPath)

	got := buildAC8iOrdinaryMergeFixture(t)
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal captured golden: %v", err)
	}
	gotJSON = append(gotJSON, '\n')

	want, err := os.ReadFile(goldenAbsPath)
	if err != nil {
		t.Fatalf("reading committed golden %s: %v", goldenAbsPath, err)
	}

	if string(gotJSON) != string(want) {
		t.Errorf("the golden must replay byte-identically even under a hostile ambient GIT_INDEX_FILE/GIT_CONFIG_* environment (G1-4).\n--- got ---\n%s\n--- want ---\n%s", gotJSON, want)
	}
}
