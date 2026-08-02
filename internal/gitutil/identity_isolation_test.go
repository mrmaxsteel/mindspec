// identity_isolation_test.go — spec 127 CI-identity fix round.
//
// THE recurrence guard for the "green locally, red in the real
// environment" class this spec exists to close, in its LOCAL-blind
// direction: bead 6 shipped a regression invisible to a fully-green CI;
// this is its mirror, a regression invisible to a fully-green LOCAL run.
//
// Every commit git creates needs an author and committer identity, and
// git reads that identity from its ordinary config cascade. A developer's
// machine carries one in the GLOBAL tier, so a fixture that configures no
// identity of its own still commits successfully — the ambient identity
// silently stands in. A CI runner carries none in ANY tier, and (unlike a
// developer's dotted hostname) cannot even auto-detect one, so the same
// fixture fails there with git's "Please tell me who you are". That is
// exactly how three packages' worth of fixtures passed on every developer
// machine and turned CI red on spec 127's PR.
//
// The fixtures themselves were fixed by configuring REPO-LOCAL identity —
// the one tier both the test's own git commands and the PRODUCT's git
// subprocesses read. This file pins the property that fix depends on, for
// the commit-creating primitives every product path funnels through.
package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// identityDiagnosticAnchor is the line git prints on every identity
// refusal, across the wordings that differ between "identity unknown"
// (author vs committer) and between git versions. It is asserted against
// the LIVE git first (see stripAmbientGitIdentity's control probe) and
// only then used as the expected substring of a primitive's own error, so
// a future git that changes this wording fails the probe with a clear
// message rather than silently hollowing out the assertions below.
const identityDiagnosticAnchor = "*** Please tell me who you are."

// stripAmbientGitIdentity removes every ambient source of a git identity
// for the remainder of t, so any git command spawned by this test — or by
// PRODUCT code under it — sees exactly what a CI runner sees.
//
// It then PROVES the strip took effect, and fails the test loudly if it
// did not. That control probe is the point: without it, a strip that
// quietly stopped stripping (a git that reads a tier this misses, an
// env var that turns out not to be authoritative) would leave every
// assertion in this file passing vacuously against an ambient identity —
// the same silent-pass failure mode the file exists to catch.
//
// The isolation is derived from the environment rather than assumed:
// paths come from t.TempDir(), and whether git actually refuses is
// measured by running git, not predicted.
//
// `user.useConfigOnly=true` is what makes this reproduce CI on a
// developer machine. Emptying the config tiers alone is not enough: git
// falls back to guessing an identity from username+hostname, which
// SUCCEEDS on a developer's dotted hostname (`host.lan`) and fails on a
// CI runner's undotted one. useConfigOnly disables that guess, so the
// refusal is the same on both. It is set in the GLOBAL tier, so a
// REPO-LOCAL identity still wins — this strips ambient identity only,
// which is precisely the distinction the fixture fix turns on.
func stripAmbientGitIdentity(t *testing.T) {
	t.Helper()

	globalCfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(globalCfg, []byte("[user]\n\tuseConfigOnly = true\n"), 0o644); err != nil {
		t.Fatalf("write isolated global gitconfig: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalCfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, key := range []string{
		"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL",
		"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
		"EMAIL",
	} {
		key := key
		if old, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { os.Setenv(key, old) })
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("unset %s: %v", key, err)
			}
		}
	}

	// The control probe. A repo with no identity in any tier must be
	// unable to commit at all.
	probe := t.TempDir()
	mustGitNoIdentity(t, probe, "init", "-q", "-b", "main")
	out, err := exec.Command("git", "-C", probe, "commit", "--allow-empty", "-m", "probe").CombinedOutput()
	if err == nil {
		t.Fatalf("ISOLATION BROKEN: git committed in %s with no configured identity, so this test's "+
			"ambient-identity strip is no longer stripping and every assertion below would pass "+
			"vacuously. Fix the strip before trusting this file. git output:\n%s", probe, out)
	}
	if !strings.Contains(string(out), identityDiagnosticAnchor) {
		t.Fatalf("git refused to commit, but not with the identity diagnostic this file matches on "+
			"(%q) — git's wording has changed and identityDiagnosticAnchor must be updated. "+
			"git output:\n%s", identityDiagnosticAnchor, out)
	}
}

// mustGitNoIdentity runs a git command that does NOT create a commit (and
// so needs no identity) in dir, under whatever environment the caller has
// established.
func mustGitNoIdentity(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
	}
}

// mustGitExplicitIdentity runs a git command in dir with an identity
// supplied EXPLICITLY on the command line (`-c user.name=… -c
// user.email=…`), used only to build fixture history in a repo that
// deliberately has no identity configured. It never writes that identity
// into the repo's config, so the repo remains identity-less for the
// primitive under test.
func mustGitExplicitIdentity(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-C", dir,
		"-c", "user.name=fixture",
		"-c", "user.email=fixture@example.invalid",
		"-c", "commit.gpgsign=false",
	}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", full, err, out)
	}
}

// identitylessRepo builds a repo with one commit on main and a divergent
// `side` branch, carrying NO identity in its own config. Its history is
// created with an explicitly-supplied identity so the fixture can exist
// at all; the repo itself stays identity-less, which is the condition the
// primitives are then run against.
func identitylessRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGitNoIdentity(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root\n"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	mustGitExplicitIdentity(t, dir, "add", "root.txt")
	mustGitExplicitIdentity(t, dir, "commit", "-q", "-m", "root")

	mustGitExplicitIdentity(t, dir, "checkout", "-q", "-b", "side")
	if err := os.WriteFile(filepath.Join(dir, "side.txt"), []byte("side\n"), 0o644); err != nil {
		t.Fatalf("write side file: %v", err)
	}
	mustGitExplicitIdentity(t, dir, "add", "side.txt")
	mustGitExplicitIdentity(t, dir, "commit", "-q", "-m", "side work")
	mustGitExplicitIdentity(t, dir, "checkout", "-q", "main")

	// Assert the fixture premise the whole file rests on, rather than
	// trusting that `git init` wrote no identity: if some future git
	// seeded one, these primitives would succeed for a reason that has
	// nothing to do with what is being tested.
	for _, key := range []string{"user.name", "user.email"} {
		if out, err := exec.Command("git", "-C", dir, "config", "--get", key).Output(); err == nil {
			t.Fatalf("fixture invariant broken: repo %s already carries %s=%q, so it is not the "+
				"identity-less repo these assertions require", dir, key, strings.TrimSpace(string(out)))
		}
	}
	return dir
}

// configureRepoLocalIdentity applies the fix the fixtures across this
// repo now use: identity in the REPO-LOCAL config tier, the one tier a
// product git subprocess reads without inheriting anything from the test
// process.
func configureRepoLocalIdentity(t *testing.T, dir string) {
	t.Helper()
	mustGitNoIdentity(t, dir, "config", "user.email", "test@example.invalid")
	mustGitNoIdentity(t, dir, "config", "user.name", "test")
	mustGitNoIdentity(t, dir, "config", "commit.gpgsign", "false")
}

// TestCommitPrimitives_WithNoAmbientGitIdentity is the guard proper. For
// each commit-creating primitive, under a proven-stripped ambient
// identity:
//
//   - with NO identity anywhere, the primitive must FAIL, and its error
//     must carry git's own identity diagnostic — never a bare exit
//     status. CommitTreeMerge used `cmd.Output()`, which discards stderr,
//     so its CI failure arrived as "exit status 128" naming neither cause
//     nor remedy; that is what made this class expensive to diagnose.
//   - with REPO-LOCAL identity configured, the primitive must SUCCEED.
//     This is the contract CI depends on and the exact property the
//     fixture fix supplies.
func TestCommitPrimitives_WithNoAmbientGitIdentity(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, dir string) error
	}{
		{
			name: "CommitTreeMerge",
			run: func(t *testing.T, dir string) error {
				t.Helper()
				tree, err := TreeSHA(dir, "main")
				if err != nil {
					t.Fatalf("TreeSHA: %v", err)
				}
				parent1, err := RevParseRef(dir, "main")
				if err != nil {
					t.Fatalf("RevParseRef(main): %v", err)
				}
				parent2, err := RevParseRef(dir, "side")
				if err != nil {
					t.Fatalf("RevParseRef(side): %v", err)
				}
				_, err = CommitTreeMerge(dir, tree, parent1, parent2, "Merge side")
				return err
			},
		},
		{
			name: "CommitPaths",
			run: func(t *testing.T, dir string) error {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("content\n"), 0o644); err != nil {
					t.Fatalf("write staged file: %v", err)
				}
				return CommitPaths(dir, "commit the staged path", []string{"staged.txt"})
			},
		},
		{
			name: "MergeBranch",
			run: func(t *testing.T, dir string) error {
				t.Helper()
				return MergeBranch(dir, "side", "main")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/no_identity_anywhere_fails_with_gits_own_diagnostic", func(t *testing.T) {
			stripAmbientGitIdentity(t)
			dir := identitylessRepo(t)

			err := tc.run(t, dir)
			if err == nil {
				t.Fatalf("%s succeeded in a repo with no identity in any config tier — it must not, "+
					"and on a CI runner it does not", tc.name)
			}
			if !strings.Contains(err.Error(), identityDiagnosticAnchor) {
				t.Errorf("%s failed, but its error does not carry git's own identity diagnostic (%q), "+
					"so an operator sees a failure naming neither the cause nor the remedy. got: %v",
					tc.name, identityDiagnosticAnchor, err)
			}
		})

		t.Run(tc.name+"/repo_local_identity_succeeds", func(t *testing.T) {
			stripAmbientGitIdentity(t)
			dir := identitylessRepo(t)
			configureRepoLocalIdentity(t, dir)

			if err := tc.run(t, dir); err != nil {
				t.Fatalf("%s must succeed on REPO-LOCAL identity alone, with no ambient identity to "+
					"fall back on — this is the exact condition CI runs every fixture under: %v",
					tc.name, err)
			}
		})
	}
}
