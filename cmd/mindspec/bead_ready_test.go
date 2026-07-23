package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/harness"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

// writeReadinessTestWorkspace writes spec.md/plan.md for specID under root
// at the on-disk path workspace.SpecDir resolves for a plain,
// non-worktree, non-.mindspec root: docs/specs/<id>/ (the same convention
// internal/validate/readiness's own fixtures.go uses).
func writeReadinessTestWorkspace(t *testing.T, root, specID, specMD, planMD string) error {
	t.Helper()
	dir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specMD), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "plan.md"), []byte(planMD), 0o644)
}

// runBeadReadyCheck invokes beadReadyCheckCmd's RunE in-process (the same
// object rootCmd wires up), capturing stdout the way a real invocation
// would print it.
func runBeadReadyCheck(t *testing.T, beadID string) (stdout string, err error) {
	t.Helper()
	return captureStdout(t, func() error {
		return beadReadyCheckCmd.RunE(beadReadyCheckCmd, []string{beadID})
	})
}

func gitPorcelainCmd(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain: %v: %s", err, out)
	}
	return string(out)
}

func gitBranchesCmd(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "branch", "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list: %v: %s", err, out)
	}
	return string(out)
}

// TestBeadReadyCheck_NegativeFixture pins AC-1: the negative fixture exits
// non-zero with a FAIL line (+ recovery) for each of MF-1..MF-4.
func TestBeadReadyCheck_NegativeFixture(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildNegativeFixture(root)
	if err != nil {
		t.Fatalf("BuildNegativeFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)
	chdir(t, root)

	_, runErr := runBeadReadyCheck(t, fx.BeadID)
	if runErr == nil {
		t.Fatal("expected a non-zero-exit error for the negative fixture, got nil")
	}
	msg := runErr.Error()
	for _, want := range []string{"MF-1: FAIL", "MF-2: FAIL", "MF-3: FAIL", "MF-4: FAIL"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q:\n%s", want, msg)
		}
	}
	if got := strings.Count(msg, "recovery: "); got < 4 {
		t.Errorf("expected at least 4 recovery: lines (one per FAILing signal), got %d:\n%s", got, msg)
	}
}

// TestBeadReadyCheck_PositiveFixture pins AC-2: the positive fixture exits
// 0, is idempotent, and performs no mutation (git status/branches
// byte-identical before/after two consecutive invocations).
func TestBeadReadyCheck_PositiveFixture(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildPositiveFixture(root)
	if err != nil {
		t.Fatalf("BuildPositiveFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)
	chdir(t, root)

	statusBefore := gitPorcelainCmd(t, root)
	branchesBefore := gitBranchesCmd(t, root)

	out1, err1 := runBeadReadyCheck(t, fx.BeadID)
	if err1 != nil {
		t.Fatalf("expected exit 0 for the positive fixture, got: %v\n%s", err1, out1)
	}
	for _, want := range []string{"MF-1: PASS", "MF-2: PASS", "MF-3: PASS", "MF-4: PASS"} {
		if !strings.Contains(out1, want) {
			t.Errorf("stdout missing %q:\n%s", want, out1)
		}
	}

	out2, err2 := runBeadReadyCheck(t, fx.BeadID)
	if err2 != nil {
		t.Fatalf("2nd invocation: expected exit 0, got: %v\n%s", err2, out2)
	}
	if out1 != out2 {
		t.Errorf("ready-check is not idempotent:\n1st: %s\n2nd: %s", out1, out2)
	}

	statusAfter := gitPorcelainCmd(t, root)
	branchesAfter := gitBranchesCmd(t, root)
	if statusBefore != statusAfter {
		t.Errorf("git status --porcelain changed across invocations: before=%q after=%q", statusBefore, statusAfter)
	}
	if branchesBefore != branchesAfter {
		t.Errorf("git branch --list changed across invocations: before=%q after=%q", branchesBefore, branchesAfter)
	}
}

// TestBeadReadyCheck_MalformedBeadID pins the ingress refusal: a hostile
// bead-ID argument is refused at idvalidate, forced-quoted, before any
// root resolution or evaluation is attempted (no fixture/root needed).
func TestBeadReadyCheck_MalformedBeadID(t *testing.T) {
	hostileIDs := []string{
		"mindspec-1\n--help",
		"mindspec-1;evil",
		"../../etc/passwd",
		"",
	}
	for _, id := range hostileIDs {
		_, err := runBeadReadyCheck(t, id)
		if err == nil {
			t.Errorf("runBeadReadyCheck(%q): expected a refusal error, got nil", id)
			continue
		}
		msg := err.Error()
		// termsafe.Escape's contract is printable-ASCII-is-a-no-op — a
		// semicolon or slash is printable ASCII and legitimately renders
		// raw (it cannot forge a terminal line on its own). The property
		// that actually matters is: a raw NEWLINE embedded in a hostile
		// id must never surface as a second real line (it must be folded
		// into the escaped two-byte `\n` sequence via the whole-string
		// quoting).
		if strings.Contains(id, "\n") {
			wantQuoted := strconv.Quote(id)
			if !strings.Contains(msg, wantQuoted) {
				t.Errorf("runBeadReadyCheck(%q): expected the forced-quoted id %q present in %q", id, wantQuoted, msg)
			}
		}
	}
}

// TestBeadReadyCheck_HostileDescriptionEscaped pins AC-8: a bd
// description/plan-section line embedding terminal-escape bytes reaches
// the rendered report only in its termsafe-escaped (quoted) form — never
// as raw control bytes that could forge extra terminal lines.
func TestBeadReadyCheck_HostileDescriptionEscaped(t *testing.T) {
	root := t.TempDir()
	const specID = "994-fixture-hostile"
	const beadID = "mindspec-hostile.1"
	const epicID = "mindspec-hostile"

	hostileLine := "Note: TBD - \x1b[31mHACKED\x1b[0m the retry count."
	specMD := "# Spec 994-fixture-hostile\n\n## Acceptance Criteria\n\n- [ ] AC-1 — a criterion.\n"
	planMD := "---\nstatus: Approved\nspec_id: " + specID + "\nversion: \"1\"\nwork_chunks:\n" +
		"  - id: 1\n    depends_on: []\n    key_file_paths:\n      - internal/fixture/hostile.go\n---\n" +
		"# Plan: " + specID + "\n\n## ADR Fitness\n\nNo ADRs relevant.\n\n## Testing Strategy\n\nUnit tests.\n\n" +
		"## Bead 1: Hostile fixture\n\n**Steps**\n1. Step one\n\n**Acceptance Criteria**\n" +
		"- [ ] Provide a concrete, testable outcome for this fixture case.\n\n" + hostileLine + "\n"

	if err := writeReadinessTestWorkspace(t, root, specID, specMD, planMD); err != nil {
		t.Fatalf("writing workspace: %v", err)
	}

	store := readiness.NewFakeBDStore()
	store.Lineage[beadID] = readiness.FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = readiness.FakeBeadRecord{Description: ""}
	restore := store.Install()
	t.Cleanup(restore)
	chdir(t, root)

	if err := exec.Command("git", "-C", root, "init", "-b", "main").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	_, runErr := runBeadReadyCheck(t, beadID)
	if runErr == nil {
		t.Fatal("expected a non-zero-exit error (the hostile line is also a genuine TBD marker)")
	}
	msg := runErr.Error()
	if strings.ContainsRune(msg, '\x1b') {
		t.Errorf("raw ESC control byte leaked into the rendered report: %q", msg)
	}
	// strconv.Quote (termsafe.Escape's whole-string quoting for any
	// non-printable byte) renders a raw ESC as the four-byte textual
	// escape `\x1b` — proving the hostile byte survived in its escaped
	// form rather than being silently dropped.
	if !strings.Contains(msg, `\x1b`) {
		t.Errorf("expected the escaped ESC sequence `\\x1b` present in the report; got:\n%s", msg)
	}
	if !strings.Contains(msg, "HACKED") {
		t.Errorf("expected the surrounding hostile-line text to survive (escaped) in the report; got:\n%s", msg)
	}
}

// TestBeadReadyCheck_TemporalFlow_RealPlanApproveAndComplete pins AC-10:
// readiness is a TEMPORAL fact re-derived per invocation, not a cached
// approval-time judgment. A real two-bead plan (bead 2 depends_on bead 1)
// is approved via the real `mindspec plan approve` path (real bd, real
// git): immediately after approval, `ready-check` PASSes bead 1 and FAILs
// bead 2 on MF-3 (dependency not yet landed-merged); after bead 1 is
// landed via a real `mindspec complete` (branch deleted), `ready-check`
// PASSes bead 2 — proving MF-3 tolerates the deleted branch on the real
// completion path, not just the fixture's synthetic corroboration.
//
// This drives the REAL mindspec binary end-to-end (a freshly-built copy,
// spec 084 Bead 3's buildMindspecBinary helper) against a real bd +
// git sandbox (internal/harness.NewSandbox) — no fake-bd shim substitutes
// for `plan approve`/`complete`'s own bd/git mutation chain, which cannot
// be faithfully faked without reimplementing bd. Per the repo's own
// no-skip-gating convention for real-bd/real-git flows (see
// internal/harness/scenario_finalize_pr_test.go's hideRealGH,
// internal/harness/sandbox.go's initBeads), a missing `bd`/`git` on PATH
// is a hard failure (t.Fatalf), never a t.Skip.
func TestBeadReadyCheck_TemporalFlow_RealPlanApproveAndComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-bd/real-git end-to-end flow under -short (matches internal/harness's own -short gating convention)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Fatalf("real bd required for this AC-10 temporal-flow test (no-skip-gating, spec 124 plan-gate F3-1): %v", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("real git required: %v", err)
	}

	// Build a real mindspec binary and put its directory FIRST on the
	// live process PATH (restored via t.Cleanup): harness.Sandbox.Env()
	// re-reads os.Getenv("PATH") on every call, and mustRunWithBin's
	// nil-Env subprocess calls (used when the conventional bin/mindspec
	// isn't present) inherit the process environment — so this makes
	// `mindspec` resolvable to every subprocess the sandbox spawns
	// (git hooks included) without requiring a prior `make build`.
	binPath := buildMindspecBinary(t)
	binDir := filepath.Dir(binPath)
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	sb := harness.NewSandbox(t)

	const specID = "997-readiness-temporal"
	epicID := sb.CreateSpecEpic(specID)

	specBranch := "spec/" + specID
	if out, err := sb.Run("git", "branch", specBranch); err != nil {
		t.Fatalf("git branch %s: %v\n%s", specBranch, err, out)
	}
	specWtDir := ".worktrees/worktree-spec-" + specID
	if out, err := sb.Run("git", "worktree", "add", specWtDir, specBranch); err != nil {
		t.Fatalf("git worktree add: %v\n%s", out, err)
	}

	specMD := `---
title: Readiness Temporal Flow Fixture
status: Approved
---
# Readiness Temporal Flow Fixture

## Acceptance Criteria
- [ ] AC-1 — bead 1 lands real work
- [ ] AC-2 — bead 2 depends on bead 1
`
	planMD := `---
status: Draft
spec_id: 997-readiness-temporal
version: "1"
adr_citations: []
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/fixture/temporal_bead1.go
  - id: 2
    depends_on: [1]
    key_file_paths:
      - internal/fixture/temporal_bead2.go
---
# Plan: 997-readiness-temporal

## ADR Fitness

No existing ADRs are impacted; this is a self-contained test fixture.

## Testing Strategy

Unit tests via ` + "`go test`" + `.

## Bead 1: Land first

**Steps**
1. Create internal/fixture/temporal_bead1.go
2. Implement the fixture function
3. Add a unit test

**Verification**
- [ ] ` + "`go test ./internal/fixture/`" + ` passes

**Acceptance Criteria**
- [ ] AC-1 — bead 1 lands real work

**Depends on**
None

## Bead 2: Depends on Bead 1

**Steps**
1. Create internal/fixture/temporal_bead2.go
2. Implement the fixture function
3. Add a unit test

**Verification**
- [ ] ` + "`go test ./internal/fixture/`" + ` passes

**Acceptance Criteria**
- [ ] AC-2 — bead 2 depends on bead 1

**Depends on**
Bead 1
`
	sb.WriteFile(specWtDir+"/.mindspec/specs/"+specID+"/spec.md", specMD)
	sb.WriteFile(specWtDir+"/.mindspec/specs/"+specID+"/plan.md", planMD)

	specWtAbs := filepath.Join(sb.Root, specWtDir)
	if out, err := exec.Command("git", "-C", specWtAbs, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add (spec worktree): %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", specWtAbs, "commit", "-m", "setup: draft plan").CombinedOutput(); err != nil {
		t.Fatalf("git commit (spec worktree): %v\n%s", err, out)
	}
	sb.Commit("setup: spec worktree state")

	if out, err := sb.Run("mindspec", "plan", "approve", specID); err != nil {
		t.Fatalf("mindspec plan approve %s: %v\n%s", specID, err, out)
	}

	bead1 := epicID + ".1"
	bead2 := epicID + ".2"

	// Immediately after approval: bead 1 PASSes, bead 2 FAILs MF-3 (its
	// dependency is not yet landed-merged).
	out1, err1 := sb.Run("mindspec", "bead", "ready-check", bead1)
	if err1 != nil {
		t.Fatalf("ready-check %s expected exit 0 right after approval, got error: %v\n%s", bead1, err1, out1)
	}
	out2, err2 := sb.Run("mindspec", "bead", "ready-check", bead2)
	if err2 == nil {
		t.Fatalf("ready-check %s expected non-zero exit right after approval (dependency not landed), got exit 0:\n%s", bead2, out2)
	}
	if !strings.Contains(out2, "MF-3: FAIL") {
		t.Errorf("ready-check %s expected an MF-3 FAIL right after approval, got:\n%s", bead2, out2)
	}

	// Claim + land bead 1 via the real next/complete lifecycle.
	if out, err := sb.Run("mindspec", "next", bead1); err != nil {
		t.Fatalf("mindspec next %s: %v\n%s", bead1, err, out)
	}
	bead1WtDir := ".worktrees/worktree-" + bead1
	if !sb.FileExists(bead1WtDir) {
		t.Fatalf("expected bead 1 worktree at %s after `mindspec next`", bead1WtDir)
	}
	sb.WriteFile(bead1WtDir+"/internal/fixture/temporal_bead1.go", "package fixture\n\n// TemporalBead1 is the AC-10 fixture's landed artifact.\nfunc TemporalBead1() string { return \"bead1\" }\n")
	bead1WtAbs := filepath.Join(sb.Root, bead1WtDir)
	if out, err := exec.Command("git", "-C", bead1WtAbs, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add (bead 1 worktree): %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", bead1WtAbs, "commit", "-m", "bead1: land fixture work").CombinedOutput(); err != nil {
		t.Fatalf("git commit (bead 1 worktree): %v\n%s", err, out)
	}

	if out, err := sb.Run("mindspec", "complete", bead1, "land bead 1",
		"--allow-doc-skew", "test fixture, no docs domain",
		"--override-adr", "test fixture, no OWNERSHIP.yaml domain"); err != nil {
		t.Fatalf("mindspec complete %s: %v\n%s", bead1, err, out)
	}

	// After bead 1 lands (branch deleted per the normal completion
	// path), bead 2's dependency is now closed AND landed-merged:
	// ready-check PASSes, re-derived fresh (not a cached judgment).
	out3, err3 := sb.Run("mindspec", "bead", "ready-check", bead2)
	if err3 != nil {
		t.Fatalf("ready-check %s expected exit 0 after bead 1 landed, got error: %v\n%s", bead2, err3, out3)
	}
	if !strings.Contains(out3, "MF-3: PASS") {
		t.Errorf("ready-check %s expected MF-3: PASS after bead 1 landed, got:\n%s", bead2, out3)
	}
}
