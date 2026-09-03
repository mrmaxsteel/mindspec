package main

// next_claim_guard_test.go — spec 128 (lifecycle-verb-state-honesty)
// Bead 7, R5.1/AC-8: the real-shape, end-to-end legs of the proportional
// claim guard. internal/next/entangle_test.go covers the unit-level
// classification and the seam-injected refusal leg (AC-8(iii)); THIS
// file drives the real mindspec binary against a real bd + git sandbox
// (the next_ready_gate_test.go precedent — nextReadyGateSetupSandbox /
// nextReadyGateApproveSingleBeadPlan) so AC-8(i)'s "a worktree-creating
// claim succeeds" and AC-8(ii)'s enumeration pin exercise the REAL claim
// path, not a mock. Each test builds its OWN sandbox repo and claims
// only inside it — never in the repo running this test suite.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/harness"
	"github.com/mrmaxsteel/mindspec/internal/next"
)

// AC-8(i): three real-shape variants of unrelated dirt in the invoking
// checkout (mirroring the three real #179 hits this spec's evidence
// recorded: an untracked file, an untracked directory tree, and a
// modified tracked file) — with no active bead worktree, a
// worktree-creating claim SUCCEEDS despite the dirt, and the one-line
// ignored-dirt notice (Max's standing directive, 2026-09-02) names it.
func TestNextCmd_ClaimGuard_UnrelatedDirtVariantsProceedWithNotice(t *testing.T) {
	variants := []struct {
		name     string
		specID   string
		setup    func(sb *harness.Sandbox)
		wantPath string
	}{
		{
			name:   "untracked_file",
			specID: "991-claim-guard-untracked-file",
			setup: func(sb *harness.Sandbox) {
				sb.WriteFile("agent-notes.txt", "human WIP, unrelated to the claim\n")
			},
			wantPath: "agent-notes.txt",
		},
		{
			name:   "untracked_directory_tree",
			specID: "990-claim-guard-untracked-dir",
			setup: func(sb *harness.Sandbox) {
				sb.WriteFile("scratch/draft.md", "draft\n")
				sb.WriteFile("scratch/more.md", "more\n")
			},
			wantPath: "scratch/",
		},
		{
			name:   "modified_tracked_file",
			specID: "989-claim-guard-modified-tracked",
			setup: func(sb *harness.Sandbox) {
				// AGENTS.md is written and committed by bootstrap during
				// sandbox setup, so this is a genuinely MODIFIED tracked
				// file, not an untracked one.
				sb.WriteFile("AGENTS.md", "# modified by the human mid-session, unrelated to the claim\n")
			},
			wantPath: "AGENTS.md",
		},
	}

	for _, v := range variants {
		v := v
		t.Run(v.name, func(t *testing.T) {
			sb := nextReadyGateSetupSandbox(t)
			specMD := nextReadyGatePositiveSpecMD
			planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", v.specID)
			epicID := nextReadyGateApproveSingleBeadPlan(t, sb, v.specID, specMD, planMD)
			beadID := epicID + ".1"

			v.setup(sb)

			out, err := sb.Run("mindspec", "next", beadID)
			if err != nil {
				t.Fatalf("expected the claim to SUCCEED despite unrelated dirt (spec 128 R5.1), got: %v\n%s", err, out)
			}
			if strings.Contains(out, "cannot claim work") {
				t.Errorf("unrelated dirt must not BLOCK the worktree-creating claim (spec 128 R5.1); got:\n%s", out)
			}
			if !strings.Contains(out, v.wantPath) {
				t.Errorf("expected the ignored-dirt notice to name %q, got:\n%s", v.wantPath, out)
			}

			status, _ := nextReadyGateBDStatus(t, sb, beadID)
			if status != "in_progress" {
				t.Errorf("expected bead %s to be claimed (in_progress) despite the unrelated dirt, got status %q", beadID, status)
			}
			if !sb.WorktreeExists("worktree-" + beadID) {
				t.Errorf("expected a worktree for %s to have been created despite the unrelated dirt", beadID)
			}
		})
	}
}

// dirtyPorcelainPaths runs `git status --porcelain` in sb and returns the
// non-empty path field of every line, excluding ADR-0025's exempt named
// artifact (auto-handled dirt, not part of R5.1's entanglement
// enumeration under test in the pin functions below).
func dirtyPorcelainPaths(t *testing.T, sb *harness.Sandbox) []string {
	t.Helper()
	out, err := sb.Run("git", "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status --porcelain: %v\n%s", err, out)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if p == "" || p == ".beads/issues.jsonl" {
			continue
		}
		paths = append(paths, p)
	}
	return paths
}

// assertPathsMatchEnumeration fails the test unless got and want contain
// exactly the same set of paths (order-independent).
func assertPathsMatchEnumeration(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("a real claim run wrote %v, but the predicted enumeration is %v — stale (spec 128 R5.1 anti-drift)", got, want)
	}
	for _, p := range got {
		found := false
		for _, w := range want {
			if w == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("a real claim run wrote %q, but the predicted enumeration does not include it: %v", p, want)
		}
	}
}

// TestNextCmd_ClaimGuard_EnumerationPin is AC-8(ii)'s baseline leg
// (round-1 panel G2's default fixture): derive the repo-relative paths a
// REAL claim run actually writes in the invoking checkout (this
// sandbox's root — never the bead worktree, and never the repo running
// this test suite) and assert they match
// next.ComputeEntangleablePaths(root, specID) exactly. Under this
// fixture's config (recording off, .gitignore already carries the
// .worktrees entry from sandbox bootstrap) that predicted set is empty
// — the two dynamic-leg tests below exercise the NON-empty predictions
// round-1 panel G1/F1 found missing.
func TestNextCmd_ClaimGuard_EnumerationPin(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	specID := "988-claim-guard-enumeration-pin"
	planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", specID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, specID, nextReadyGatePositiveSpecMD, planMD)
	beadID := epicID + ".1"

	if before := dirtyPorcelainPaths(t, sb); len(before) != 0 {
		t.Fatalf("fixture sanity: sandbox root must be CLEAN before the claim, got: %v", before)
	}

	out, err := sb.Run("mindspec", "next", beadID)
	if err != nil {
		t.Fatalf("expected the claim to succeed, got: %v\n%s", err, out)
	}

	assertPathsMatchEnumeration(t, dirtyPorcelainPaths(t, sb), next.ComputeEntangleablePaths(sb.Root, sb.Root, specID))
}

// TestNextCmd_ClaimGuard_EnumerationPin_RecordingEnabled is round-1
// panel G2's mandated recording-enabled leg: with recording.enabled and
// the target spec's recording dir already materialized (manifest.json
// seeded and committed, mirroring what a real `spec create` with
// recording on would have left behind), a real claim writes BOTH
// recording paths — manifest.json (read-modify-write: the bead ID gets
// appended to the current phase) and events.ndjson (a freshly created,
// untracked append target) — and the diff matches
// next.ComputeEntangleablePaths(root, cwd, specID) exactly (round-1
// panel G1/F1: this is the exact write class the original "exactly
// (a)(b)(c)" enumeration missed).
//
// This claim ALSO runs FROM WITHIN THE SPEC WORKTREE (same reasoning as
// EnumerationPin_GitignoreEntryAbsent's doc comment): workspace.SpecDir
// (which recording.EventsPath/ManifestPath resolve through) prefers the
// spec worktree's OWN copy of `.mindspec/specs/<specID>` whenever one
// exists on disk — so the manifest must be SEEDED there, and the write
// is only OBSERVABLE from that same working tree.
func TestNextCmd_ClaimGuard_EnumerationPin_RecordingEnabled(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	specID := "987-claim-guard-recording-enabled"
	planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", specID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, specID, nextReadyGatePositiveSpecMD, planMD)
	beadID := epicID + ".1"
	specWtDir := filepath.Join(sb.Root, ".worktrees", "worktree-spec-"+specID)
	gitCommandInDir(t, specWtDir, "config", "user.email", "test@mindspec.dev")
	gitCommandInDir(t, specWtDir, "config", "user.name", "MindSpec Test")

	sb.WriteFile(".mindspec/config.yaml", "recording:\n  enabled: true\n")
	// config.Load caches per absolute root for this TEST PROCESS's
	// lifetime (internal/config/config.go) — harness.NewSandbox's own
	// bootstrap already populated that cache for sb.Root with the
	// PRE-recording config, before this rewrite. The mindspec SUBPROCESS
	// below is unaffected (its own fresh process memory), but the
	// in-process next.ComputeEntangleablePaths call further down would
	// otherwise read the stale cached config instead of the file just
	// written.
	config.ResetCache()

	writeFileInDir(t, specWtDir, ".mindspec/specs/"+specID+"/recording/manifest.json",
		`{"spec_id":"`+specID+`","started_at":"2026-01-01T00:00:00Z","status":"recording","phases":[{"phase":"implement","started_at":"2026-01-01T00:00:00Z"}]}`+"\n")
	gitCommandInDir(t, specWtDir, "add", "-A")
	gitCommandInDir(t, specWtDir, "commit", "-m", "setup: seed manifest for "+specID)

	if before := gitStatusPaths(t, specWtDir); len(before) != 0 {
		t.Fatalf("fixture sanity: spec worktree must be CLEAN before the claim, got: %v", before)
	}

	out, err := runMindspecInDir(t, sb, specWtDir, "next", beadID)
	if err != nil {
		t.Fatalf("expected the claim to succeed, got: %v\n%s", err, out)
	}

	got := gitStatusPaths(t, specWtDir)
	want := next.ComputeEntangleablePaths(sb.Root, specWtDir, specID)
	assertPathsMatchEnumeration(t, got, want)
	if len(want) != 2 {
		t.Fatalf("fixture sanity: expected the recording-enabled prediction to name exactly 2 paths (manifest.json + events.ndjson), got %v", want)
	}
}

// TestNextCmd_ClaimGuard_EnumerationPin_GitignoreEntryAbsent is round-1
// panel G2's mandated .gitignore-entry-absent leg: with the
// worktree-root entry committed ABSENT from .gitignore (a clean tree —
// no PRE-EXISTING dirt to collide with), a real claim's own
// EnsureGitignoreEntry call adds the entry, and the diff matches
// next.ComputeEntangleablePaths(root, cwd, specID) exactly (round-1
// panel G1: the second missed write class).
//
// This claim runs FROM WITHIN THE SPEC WORKTREE, not the sandbox root:
// DispatchBead's EnsureGitignoreEntry writes at resolveAnchorRoot(specID)
// — the spec worktree, since nextReadyGateApproveSingleBeadPlan already
// created one (the realistic shape: a bead only exists under an
// already-plan-approved, and therefore worktree-having, spec) — so the
// entanglement class is only OBSERVABLE (and only correctly predicted)
// from that same working tree (round-1 fix-round hardening: this exact
// mismatch is what this bead's OWN e2e fixture caught against the first
// draft of ComputeEntangleablePaths, which naively assumed repoRoot).
func TestNextCmd_ClaimGuard_EnumerationPin_GitignoreEntryAbsent(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	specID := "986-claim-guard-gitignore-absent"
	planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", specID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, specID, nextReadyGatePositiveSpecMD, planMD)
	beadID := epicID + ".1"

	specWtDir := filepath.Join(sb.Root, ".worktrees", "worktree-spec-"+specID)

	// The spec worktree's OWN .gitignore, inherited from the branch point,
	// already carries `.worktrees/` (harness/sandbox.go's bootstrap
	// content) — rewrite it WITHOUT that entry and commit WITHIN the spec
	// worktree, leaving THAT tree clean with a genuinely entry-less
	// .gitignore.
	gitCommandInDir(t, specWtDir, "config", "user.email", "test@mindspec.dev")
	gitCommandInDir(t, specWtDir, "config", "user.name", "MindSpec Test")
	if err := os.WriteFile(filepath.Join(specWtDir, ".gitignore"), []byte(".beads/\n.harness/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n.mindspec/phase.lock\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommandInDir(t, specWtDir, "add", "-A")
	gitCommandInDir(t, specWtDir, "commit", "-m", "setup: remove .worktrees entry from .gitignore for "+specID)

	if before := gitStatusPaths(t, specWtDir); len(before) != 0 {
		t.Fatalf("fixture sanity: spec worktree must be CLEAN before the claim, got: %v", before)
	}

	// Predict BEFORE the claim runs: ComputeEntangleablePaths reflects
	// the CURRENT .gitignore entry-presence state, which the claim's own
	// write is about to change (present -> absent would falsely read as
	// "nothing to entangle" if computed afterward).
	want := next.ComputeEntangleablePaths(sb.Root, specWtDir, specID)
	if !reflect.DeepEqual(want, []string{".gitignore"}) {
		t.Fatalf("fixture sanity: expected the gitignore-absent prediction to be exactly [.gitignore], got %v", want)
	}

	out, err := runMindspecInDir(t, sb, specWtDir, "next", beadID)
	if err != nil {
		t.Fatalf("expected the claim to succeed, got: %v\n%s", err, out)
	}

	got := gitStatusPaths(t, specWtDir)
	assertPathsMatchEnumeration(t, got, want)
	// The claim's OWN write must have re-added the entry (round-1 panel
	// G1/gitutil.HasGitignoreEntry contract) — .worktrees/ is correctly
	// ignored again by the time this check runs, so it must not appear
	// as a stray untracked entry alongside .gitignore.
	content, err := os.ReadFile(filepath.Join(specWtDir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore unexpectedly missing after the claim: %v", err)
	}
	if !strings.Contains(string(content), ".worktrees") {
		t.Errorf(".gitignore must have the .worktrees entry re-added by the claim, got:\n%s", content)
	}
}

// gitCommandInDir runs a real git command with dir as its working
// directory (unlike sb.Run, which always operates at sb.Root) — used for
// setup steps that must land ON THE SPEC WORKTREE'S OWN BRANCH.
func gitCommandInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v (in %s): %v\n%s", args, dir, err, out)
	}
}

// mindspecCmdName is the PATH-resolved binary name runMindspecInDir
// invokes. Held in a variable rather than inlined as a exec.Command
// literal — the same indirection harness.Sandbox.Run already uses for
// its own `name` parameter — so this satisfies spec 084 Hard
// Constraint #2 (internal/specgate's TestNoAgentmindExecLiteral, an
// AST scan for a literal "mindspec"/"agentmind" first argument to
// exec.Command/exec.CommandContext/exec.LookPath/os.StartProcess).
const mindspecCmdName = "mindspec"

// runMindspecInDir runs the built mindspec binary with dir as its
// working directory and sb's shim/bin PATH — the real anchor-observing
// counterpart to sb.Run (which always runs at sb.Root).
func runMindspecInDir(t *testing.T, sb *harness.Sandbox, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(mindspecCmdName, args...)
	cmd.Dir = dir
	cmd.Env = sb.Env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// gitStatusPaths runs `git status --porcelain` with dir as the working
// directory and returns the non-empty path field of every line,
// excluding ADR-0025's exempt named artifact.
func gitStatusPaths(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain (in %s): %v\n%s", dir, err, out)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if p == "" || p == ".beads/issues.jsonl" {
			continue
		}
		paths = append(paths, p)
	}
	return paths
}
