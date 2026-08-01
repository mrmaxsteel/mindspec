package gitutil

// commit_signing_test.go — spec 127 final review, S1-1 (MAJOR).
//
// `git commit-tree` does NOT honor commit.gpgsign, while `git commit
// --no-edit` and `git merge --no-ff` — which produced every commit the
// R5(d) drift-collapse REPLACES — do. So in a signing-configured
// repository the collapse used to swap two SIGNED merges for one UNSIGNED
// commit at the branch tip, silently, with the resulting push rejected by
// a "require signed commits" branch protection and no mindspec diagnostic.
//
// This file is DELIBERATELY OUTSIDE the plan's blanket signing isolation
// (its test-environment rule mandates GIT_CONFIG_GLOBAL=/dev/null +
// GIT_CONFIG_NOSYSTEM=1 plus a per-repo `commit.gpgsign false` in every
// fixture — which is precisely why no test in the suite could observe this
// behavior). These repositories configure signing ON, locally, so the
// isolation cannot neutralize them: local repo config wins over the
// global/system tiers those env vars suppress.
//
// The two tests are the positive and the negative half of the same claim:
//
//	SignedRepo   — the collapsed object actually carries a gpgsig header
//	               (RED before the fix: header absent, error nil).
//	SigningFails — with signing configured but the signing program
//	               erroring, the collapse REFUSES instead of producing a
//	               silently-unsigned tip (RED before the fix: exit 0 and a
//	               SHA, because commit-tree never attempted a signature at
//	               all — S1's own control).

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// signingRepoGit runs git in dir with an identity but WITHOUT touching any
// signing configuration — the fixtures below set that themselves.
func signingRepoGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newSigningFixture builds a one-commit repo and returns (dir, tree, head).
func newSigningFixture(t *testing.T) (string, string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	signingRepoGit(t, dir, "init", "-q", "-b", "main")
	signingRepoGit(t, dir, "config", "user.email", "test@example.invalid")
	signingRepoGit(t, dir, "config", "user.name", "mindspec-test")
	// Explicitly OFF for the root commit so the fixture builds identically
	// regardless of the developer's own global config; each test turns
	// signing on afterward, for the collapse itself.
	signingRepoGit(t, dir, "config", "commit.gpgsign", "false")
	signingRepoGit(t, dir, "commit", "-q", "--allow-empty", "-m", "root")
	tree := signingRepoGit(t, dir, "rev-parse", "HEAD^{tree}")
	head := signingRepoGit(t, dir, "rev-parse", "HEAD")
	return dir, tree, head
}

// gpgsigHeaderPresent reports whether sha's raw commit object carries a
// gpgsig header — read from the object itself, never inferred from
// `git log --show-signature`'s verification result (an unverifiable
// signature is still a PRESENT signature, and this test is about presence).
func gpgsigHeaderPresent(t *testing.T, dir, sha string) bool {
	t.Helper()
	raw := signingRepoGit(t, dir, "cat-file", "commit", sha)
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			return false // end of the header block
		}
		if strings.HasPrefix(line, "gpgsig") {
			return true
		}
	}
	return false
}

// TestCommitTreeMerge_SignedRepoProducesSignedObject is the positive half:
// in an ssh-signing repository, the object CommitTreeMerge produces must
// carry a signature, exactly like the `git merge --no-ff` commits the
// collapse replaces.
func TestCommitTreeMerge_SignedRepoProducesSignedObject(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	dir, tree, head := newSigningFixture(t)

	keyDir := t.TempDir()
	key := filepath.Join(keyDir, "signing-key")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen could not generate a key: %v\n%s", err, out)
	}

	signingRepoGit(t, dir, "config", "gpg.format", "ssh")
	signingRepoGit(t, dir, "config", "user.signingkey", key+".pub")
	signingRepoGit(t, dir, "config", "commit.gpgsign", "true")

	// Control on the fixture's own premise: an ordinary `git commit` here
	// really does sign, so a difference below is CommitTreeMerge's, not
	// the repository's.
	signingRepoGit(t, dir, "commit", "-q", "--allow-empty", "-m", "signed baseline")
	baseline := signingRepoGit(t, dir, "rev-parse", "HEAD")
	if !gpgsigHeaderPresent(t, dir, baseline) {
		t.Skipf("this git (%s) did not sign an ordinary commit in the ssh-signing fixture — the test's own premise does not hold here", runtime.GOOS)
	}

	sha, err := CommitTreeMerge(dir, tree, head, baseline, "Merge collapse under signing")
	if err != nil {
		t.Fatalf("CommitTreeMerge in a signing-configured repo: %v", err)
	}
	if !gpgsigHeaderPresent(t, dir, sha) {
		t.Fatalf("the collapsed merge commit %s carries NO gpgsig header while every commit it replaces is signed — `--resolve-merge` would silently downgrade the branch tip to an unsigned commit and the push would be rejected with no mindspec diagnostic", sha)
	}
	// The collapse must still be a real two-parent merge.
	parents := strings.Fields(signingRepoGit(t, dir, "log", "-1", "--format=%P", sha))
	if len(parents) != 2 || parents[0] != head || parents[1] != baseline {
		t.Fatalf("collapsed commit parents = %v, want [%s %s]", parents, head, baseline)
	}
}

// TestCommitTreeMerge_SigningFailureRefusesRatherThanDowngrades is the
// negative half, and it is hermetic — no real key material, no gpg, no
// ssh-keygen. commit.gpgsign is on and gpg.program points at a script that
// always fails, so `git commit` itself cannot commit. CommitTreeMerge must
// behave the same way: refuse.
//
// Before the fix this test RED-ed by PASSING through: commit-tree never
// attempted the signature, so it exited 0 and returned a SHA for an
// unsigned object under a configuration where git's own porcelain refuses
// to commit at all.
func TestCommitTreeMerge_SigningFailureRefusesRatherThanDowngrades(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the failing-signer fixture is a POSIX shell script")
	}
	dir, tree, head := newSigningFixture(t)

	// A genuine SECOND parent, committed while signing is still off, so
	// the collapse below is a real two-parent merge rather than a
	// duplicate-parent call git would fold to one parent.
	signingRepoGit(t, dir, "commit", "-q", "--allow-empty", "-m", "second parent")
	secondParent := signingRepoGit(t, dir, "rev-parse", "HEAD")

	binDir := t.TempDir()
	failing := filepath.Join(binDir, "failing-signer")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\necho 'FAKESIGNER-CALLED' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write failing signer: %v", err)
	}

	signingRepoGit(t, dir, "config", "gpg.program", failing)
	signingRepoGit(t, dir, "config", "commit.gpgsign", "true")

	// Premise control: git's own porcelain REFUSES under this config.
	probe := exec.Command("git", "commit", "--allow-empty", "-m", "should not commit")
	probe.Dir = dir
	if out, err := probe.CombinedOutput(); err == nil {
		t.Skipf("`git commit` succeeded under a failing gpg.program — this git does not honor the fixture's premise:\n%s", out)
	}

	sha, err := CommitTreeMerge(dir, tree, head, secondParent, "Merge collapse with a broken signer")
	if err == nil {
		t.Fatalf("CommitTreeMerge returned %s with no error under a configuration where `git commit` itself refuses to sign — the collapse produced a silently UNSIGNED commit that would become the branch tip", sha)
	}
}
