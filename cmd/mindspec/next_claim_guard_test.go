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
	"strings"
	"testing"

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

// TestNextCmd_ClaimGuard_EnumerationPin is AC-8(ii): derive the
// repo-relative paths a REAL claim run actually writes in the invoking
// checkout (this sandbox's root — never the bead worktree, and never
// the repo running this test suite) and assert they match
// next.EntangleableClaimPaths() exactly, ADR-0025's exempt named
// artifact aside. Red the moment a future claim step writes a new
// tracked path this list omits.
func TestNextCmd_ClaimGuard_EnumerationPin(t *testing.T) {
	sb := nextReadyGateSetupSandbox(t)

	specID := "988-claim-guard-enumeration-pin"
	planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", specID)
	epicID := nextReadyGateApproveSingleBeadPlan(t, sb, specID, nextReadyGatePositiveSpecMD, planMD)
	beadID := epicID + ".1"

	before, err := sb.Run("git", "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status --porcelain (before): %v\n%s", err, before)
	}
	if strings.TrimSpace(before) != "" {
		t.Fatalf("fixture sanity: sandbox root must be CLEAN before the claim, got:\n%s", before)
	}

	out, err := sb.Run("mindspec", "next", beadID)
	if err != nil {
		t.Fatalf("expected the claim to succeed, got: %v\n%s", err, out)
	}

	after, err := sb.Run("git", "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status --porcelain (after): %v\n%s", err, after)
	}

	// Exclude ADR-0025's own exempt named artifact — it is auto-handled
	// dirt, not part of R5.1's entanglement enumeration under test here.
	var writtenPaths []string
	for _, line := range strings.Split(after, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if p == "" || p == ".beads/issues.jsonl" {
			continue
		}
		writtenPaths = append(writtenPaths, p)
	}

	enumeration := next.EntangleableClaimPaths()
	if len(writtenPaths) != len(enumeration) {
		t.Fatalf("a real claim run wrote non-artifact path(s) %v in the invoking checkout, but next.EntangleableClaimPaths() = %v — the pinned enumeration is stale (spec 128 R5.1 anti-drift)", writtenPaths, enumeration)
	}
	for _, p := range writtenPaths {
		found := false
		for _, e := range enumeration {
			if e == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("a real claim run wrote %q in the invoking checkout, but the pinned enumeration does not include it: %v", p, enumeration)
		}
	}
}
