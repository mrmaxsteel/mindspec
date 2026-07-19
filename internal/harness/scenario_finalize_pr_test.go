package harness

// Spec 121 Bead 3: the deterministic (no-LLM) end-to-end harness tests for
// the finalize-PR automation. Each drives the REAL built `mindspec` binary
// through a protected-main finalize (the spec branch already fast-
// forwarded onto a real bare origin's `main` BEFORE `impl approve` runs,
// mirroring internal/executor/finalize_orphan_test.go's fixture shape)
// with the scripted fake `gh` from scenario_finalize_pr.go installed
// behind the recording shim.
//
// Note on scope: a literal SECOND `mindspec impl approve <id>` after a
// FIRST fully successful run cannot be used to exercise re-run/adoption
// end-to-end here — a successful finalize deletes the local spec branch,
// and the doc-sync gate's merge-base computation against it then refuses
// before ever reaching the automation. AC-2's adoption path is instead
// exercised in ONE run by pre-seeding the fake gh's state file with an
// already-open PR before `impl approve` ever executes (see
// TestFinalizePR_HarnessAdoptsExistingPR) — the reconcile-by-query /
// degrade-fault-matrix polarities (AC-6/AC-21) are pinned exhaustively at
// the cmd/mindspec seam level (finalize_pr_test.go), including a REAL
// bare-origin merge for AC-4.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// finalizePRFixture holds the sandbox + identifiers a protected-main
// finalize test needs.
type finalizePRFixture struct {
	sandbox *Sandbox
	specID  string
	epicID  string
	beadID  string
}

// setupFinalizePRFixture builds a real, closed-bead, review-mode spec
// (mirrors ScenarioImplApprove's fixture shape) with a REAL bare origin
// remote whose `main` the spec branch has already been fast-forwarded
// onto — "the impl PR already merged" (mindspec-uxl4) — so `impl approve`
// routes through FinalizeEpic's protected-main orphan path and sets
// result.FinalizeBranch, the finalize-PR automation's trigger.
func setupFinalizePRFixture(t *testing.T, specID string) finalizePRFixture {
	t.Helper()
	sandbox := NewSandbox(t)

	epicID := sandbox.CreateSpecEpic(specID)
	beadID := sandbox.CreateBead("["+specID+"] Implement feature", "task", epicID)
	sandbox.ClaimBead(beadID)
	requireValidBeadID(sandbox.t, beadID)
	sandbox.runBDMust("close", beadID)

	// ADR-divergence coverage triple (mirrors ScenarioImplApprove): done.go
	// lands at the root after the spec-branch merge, so the unconditional
	// CheckADRDivergence gate needs an owning domain + citing ADR, or this
	// fixture would fail that gate rather than pin the finalize-PR wiring.
	writeSandboxDomainCoverage(sandbox, "done.go")
	sandbox.Commit("setup: sandbox domain coverage (ownership + ADR-0001)")

	// The bare origin is wired HERE — right after the last local-main-only
	// commit and BEFORE the spec branch forks — so the spec branch and
	// origin/main share the exact same fork point. Any LATER local-main-
	// only commit (the "setup: review mode" --allow-empty commit below)
	// never gets pushed to origin, so it cannot make the spec branch's
	// later push onto origin/main a non-fast-forward.
	addBareOriginRemote(t, sandbox)

	wt := setupWorktrees(sandbox, specID, "", "plan")
	sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/spec.md", `---
title: Finalize PR Fixture
status: Approved
---
# Finalize PR Fixture
A completed feature.
`)
	sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
adr_citations:
- ADR-0001
---
# Plan
## Bead 1: Implement feature
Create done.go.
`, specID, beadID))
	sandbox.WriteFile(wt.SpecWtDir+"/done.go", `package main

func Done() string { return "done" }
`)
	mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
	mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "impl: implement feature")
	sandbox.Commit("setup: review mode")

	// Protected-main finalize trigger: fast-forward the spec branch's tip
	// directly onto origin's main — "the impl PR already merged"
	// (mindspec-uxl4), before impl approve ever runs. No faked ancestry:
	// this is a genuine git fast-forward push.
	pushBranchToOriginMain(t, sandbox, "spec/"+specID)

	return finalizePRFixture{sandbox: sandbox, specID: specID, epicID: epicID, beadID: beadID}
}

// TestFinalizePR_HarnessAutoOpensTemplatedPR pins AC-1 end-to-end: the
// real binary, on a protected-main finalize, invokes `gh pr create` with
// the templated epicID-bearing title and surfaces the PR URL.
func TestFinalizePR_HarnessAutoOpensTemplatedPR(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess-heavy end-to-end scenario — skipped in -short")
	}

	_, callLog, restoreGH := writeFakeGH(t)
	defer restoreGH()

	fx := setupFinalizePRFixture(t, "121-hscen")

	out, err := fx.sandbox.Run("mindspec", "impl", "approve", fx.specID)
	if err != nil {
		t.Fatalf("mindspec impl approve %s failed: %v\n%s", fx.specID, err, out)
	}
	if !strings.Contains(out, "chore/finalize-"+fx.specID) {
		t.Fatalf("expected the finalize-branch NOTE naming the chore carrier; output:\n%s", out)
	}

	calls := fakeGHCalls(t, callLog)
	var createArgs []string
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "pr" && c[1] == "create" {
			createArgs = c
		}
	}
	if createArgs == nil {
		t.Fatalf("expected a recorded `gh pr create` invocation; calls: %v", calls)
	}
	wantHead := "chore/finalize-" + fx.specID
	wantTitle := "chore(beads): finalize epic " + fx.epicID + " for spec " + fx.specID
	if !containsAdjacent(createArgs, "--head", wantHead) {
		t.Errorf("pr create argv missing --head %s: %v", wantHead, createArgs)
	}
	if !containsAdjacent(createArgs, "--base", "main") {
		t.Errorf("pr create argv missing --base main: %v", createArgs)
	}
	if !containsAdjacent(createArgs, "--title", wantTitle) {
		t.Errorf("pr create argv missing --title %q: %v", wantTitle, createArgs)
	}
	if !strings.Contains(out, "acme/mindspec-fixture/pull/121") {
		t.Errorf("expected the PR URL surfaced in output; got:\n%s", out)
	}
}

// TestFinalizePR_HarnessAdoptsExistingPR pins AC-2's adoption pin
// end-to-end: pre-seeding the fake gh's state file with an already-open
// PR for the exact head/base BEFORE `impl approve` ever runs means the
// automation's R1 lookup finds it and adopts it — no `pr create` at all.
func TestFinalizePR_HarnessAdoptsExistingPR(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess-heavy end-to-end scenario — skipped in -short")
	}

	stateDir, callLog, restoreGH := writeFakeGH(t)
	defer restoreGH()

	fx := setupFinalizePRFixture(t, "121-hscadopt")
	seedFakeGHOpenPR(t, stateDir, "chore/finalize-"+fx.specID)

	out, err := fx.sandbox.Run("mindspec", "impl", "approve", fx.specID)
	if err != nil {
		t.Fatalf("mindspec impl approve %s failed: %v\n%s", fx.specID, err, out)
	}

	for _, c := range fakeGHCalls(t, callLog) {
		if len(c) >= 2 && c[0] == "pr" && c[1] == "create" {
			t.Errorf("adoption case must never call `gh pr create`: %v", c)
		}
	}
	if !strings.Contains(out, "acme/mindspec-fixture/pull/121") {
		t.Errorf("expected the adopted PR's URL surfaced in output; got:\n%s", out)
	}
}

// seedFakeGHOpenPR pre-seeds fakeGHScript's per-head state file as
// already OPEN, using the exact same sanitization the script applies to
// derive its state filename from $head.
func seedFakeGHOpenPR(t *testing.T, stateDir, head string) {
	t.Helper()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("creating fake gh state dir: %v", err)
	}
	sanitized := sanitizeForShellFilename(head)
	if err := os.WriteFile(filepath.Join(stateDir, "pr-"+sanitized), []byte("OPEN\n"), 0o644); err != nil {
		t.Fatalf("seeding fake gh state file: %v", err)
	}
}

// sanitizeForShellFilename mirrors fakeGHScript's
// `tr -c 'a-zA-Z0-9._-' '_'` transform.
func sanitizeForShellFilename(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '_', c == '-':
			out[i] = c
		default:
			out[i] = '_'
		}
	}
	return string(out)
}

// containsAdjacent reports whether args contains flag immediately
// followed by value.
func containsAdjacent(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// TestFinalizePR_HarnessGHAbsentDegrades pins AC-3's end-to-end shape:
// with no `gh` resolvable at all (not even the fake), `impl approve`
// still succeeds and prints the shipped manual NOTE.
func TestFinalizePR_HarnessGHAbsentDegrades(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess-heavy end-to-end scenario — skipped in -short")
	}

	// hideRealGH must wrap the ENTIRE test, not just NewSandbox: the
	// shipped recording shim only gets installed for a command NewSandbox
	// finds on PATH at construction time, but Sandbox.Env() re-reads
	// os.Getenv("PATH") on every sandbox.Run call too — restoring PATH
	// early would let the host's real `gh` back in for the actual
	// `impl approve` invocation below.
	defer hideRealGH(t)()

	fx := setupFinalizePRFixture(t, "121-hscen2")

	out, err := fx.sandbox.Run("mindspec", "impl", "approve", fx.specID)
	if err != nil {
		t.Fatalf("mindspec impl approve %s failed: %v\n%s", fx.specID, err, out)
	}
	if !strings.Contains(out, "gh pr create --head chore/finalize-"+fx.specID) {
		t.Errorf("expected the shipped manual NOTE command; output:\n%s", out)
	}
}

// hideRealGH constructs a PATH that resolves `git`/`bd`/`mindspec`/
// `python3` (via symlinks/the project bin dir, so their real locations
// need not appear on PATH at all) but has NO entry resolving `gh` — the
// host's real `gh` and `git` are co-located under the same directory
// (e.g. /opt/homebrew/bin on a Homebrew install), so simply removing that
// one directory from PATH would break git too. Returns a restore func;
// the caller is expected to defer-call it for the WHOLE test (see the
// call site's comment).
func hideRealGH(t *testing.T) func() {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locating real git: %v", err)
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Fatalf("locating real bd: %v", err)
	}

	dir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		t.Fatalf("symlinking git: %v", err)
	}
	if err := os.Symlink(bdPath, filepath.Join(dir, "bd")); err != nil {
		t.Fatalf("symlinking bd: %v", err)
	}

	parts := []string{dir}
	// The pre-commit hook shim (and Sandbox.Run("mindspec", ...) itself)
	// exec a bare `mindspec`; keep the project's bin/ (built by
	// `make build`) reachable — NewSandbox's OWN internal commits (initial
	// commit, gitignore commit) and every sandbox.Run call resolve
	// executables via THIS process-wide PATH at call time (Go's
	// exec.Command resolves PATH immediately, not from cmd.Env), not just
	// NewSandbox's own narrow shim-install-time PATH prepend.
	if mindspecBin := projectBinDir(); mindspecBin != "" {
		parts = append(parts, mindspecBin)
	}
	// The recording shim script shells out to a bare `python3` for JSON
	// encoding; keep its real directory reachable (harmless — it never
	// contains a `gh` binary).
	if pyPath, pyErr := exec.LookPath("python3"); pyErr == nil {
		parts = append(parts, filepath.Dir(pyPath))
	}
	parts = append(parts, "/usr/bin", "/bin")

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
	return func() { os.Setenv("PATH", origPath) }
}
