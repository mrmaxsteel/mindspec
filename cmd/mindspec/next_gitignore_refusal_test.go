package main

// next_gitignore_refusal_test.go — spec 128 (lifecycle-verb-state-honesty)
// Bead 7, round-1 panel G1/G6: real claim-path coverage for a dirty
// .gitignore whose worktree-root entry is currently absent, across the
// four porcelain shapes the panel named — modified, deleted, renamed-away,
// and symlinked. Each variant proves the claim REFUSES (round-1 panel's
// preserved DirtyTreeFailure contract) and that the refusal is a total
// no-op: bd status/branches/worktrees and every file's bytes are
// identical before and after.
//
// Every variant dirties .gitignore INSIDE THE SPEC WORKTREE, not the
// sandbox root, and claims from there: DispatchBead's EnsureGitignoreEntry
// write is anchored at resolveAnchorRoot(specID), which prefers the
// target spec's worktree whenever one already exists on disk — the
// realistic shape, since a bead only exists under an already
// plan-approved (and therefore worktree-having) spec. This is exactly
// the anchor/cwd agreement TestNextCmd_ClaimGuard_EnumerationPin_
// GitignoreEntryAbsent's doc comment explains.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotFile reads relPath under dir, returning (content, existed). A
// missing file is not a test failure — several variants legitimately
// remove or rename .gitignore.
func snapshotFile(t *testing.T, dir, relPath string) (content string, existed bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, relPath))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// readSymlink returns the target of the symlink at relPath under dir,
// or "" if relPath is not a symlink / doesn't exist.
func readSymlink(dir, relPath string) string {
	target, err := os.Readlink(filepath.Join(dir, relPath))
	if err != nil {
		return ""
	}
	return target
}

// writeFileInDir writes relPath under dir (creating parent directories),
// unlike sb.WriteFile which always writes relative to sb.Root.
func writeFileInDir(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", relPath, err)
	}
}

func TestNextCmd_ClaimGuard_GitignoreVariantsRefuseByteIdentical(t *testing.T) {
	baseGitignoreNoWorktrees := ".beads/\n.harness/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n.mindspec/phase.lock\n"

	type check struct {
		relPath string
	}
	variants := []struct {
		name    string
		specID  string
		dirty   func(t *testing.T, specWtDir string)
		checked []check // files whose bytes must be byte-identical pre/post refusal
	}{
		{
			name:   "modified",
			specID: "985-claim-guard-gitignore-modified",
			dirty: func(t *testing.T, specWtDir string) {
				// Edit the tracked .gitignore to remove the entry,
				// WITHOUT committing — a genuinely dirty, entry-absent
				// .gitignore.
				writeFileInDir(t, specWtDir, ".gitignore", baseGitignoreNoWorktrees)
			},
			checked: []check{{".gitignore"}},
		},
		{
			name:   "deleted",
			specID: "984-claim-guard-gitignore-deleted",
			dirty: func(t *testing.T, specWtDir string) {
				if err := os.Remove(filepath.Join(specWtDir, ".gitignore")); err != nil {
					t.Fatalf("removing .gitignore: %v", err)
				}
			},
			checked: []check{{".gitignore"}},
		},
		{
			name:   "renamed_away",
			specID: "983-claim-guard-gitignore-renamed",
			dirty: func(t *testing.T, specWtDir string) {
				gitCommandInDir(t, specWtDir, "mv", ".gitignore", ".gitignore.bak")
			},
			checked: []check{{".gitignore"}, {".gitignore.bak"}},
		},
		{
			name:   "symlinked",
			specID: "982-claim-guard-gitignore-symlinked",
			dirty: func(t *testing.T, specWtDir string) {
				// Setup (committed, clean): .gitignore is a SYMLINK to a
				// tracked target that STILL carries the entry.
				writeFileInDir(t, specWtDir, "gitignore-target-with-entry.txt", ".beads/\n.harness/\n.worktrees/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n.mindspec/phase.lock\n")
				writeFileInDir(t, specWtDir, "gitignore-target-without-entry.txt", baseGitignoreNoWorktrees)
				if err := os.Remove(filepath.Join(specWtDir, ".gitignore")); err != nil {
					t.Fatalf("removing regular .gitignore before symlinking: %v", err)
				}
				if err := os.Symlink("gitignore-target-with-entry.txt", filepath.Join(specWtDir, ".gitignore")); err != nil {
					t.Fatalf("creating .gitignore symlink: %v", err)
				}
				gitCommandInDir(t, specWtDir, "add", "-A")
				gitCommandInDir(t, specWtDir, "commit", "-m", "setup: .gitignore is now a symlink (with entry)")

				// Dirty: repoint the symlink at the entry-LESS target,
				// uncommitted.
				if err := os.Remove(filepath.Join(specWtDir, ".gitignore")); err != nil {
					t.Fatalf("removing symlink before repointing: %v", err)
				}
				if err := os.Symlink("gitignore-target-without-entry.txt", filepath.Join(specWtDir, ".gitignore")); err != nil {
					t.Fatalf("repointing .gitignore symlink: %v", err)
				}
			},
			checked: []check{{"gitignore-target-with-entry.txt"}, {"gitignore-target-without-entry.txt"}},
		},
	}

	for _, v := range variants {
		v := v
		t.Run(v.name, func(t *testing.T) {
			sb := nextReadyGateSetupSandbox(t)
			planMD := strings.ReplaceAll(nextReadyGatePositivePlanMD, "995-readiness-gate-positive", v.specID)
			epicID := nextReadyGateApproveSingleBeadPlan(t, sb, v.specID, nextReadyGatePositiveSpecMD, planMD)
			beadID := epicID + ".1"
			specWtDir := filepath.Join(sb.Root, ".worktrees", "worktree-spec-"+v.specID)
			gitCommandInDir(t, specWtDir, "config", "user.email", "test@mindspec.dev")
			gitCommandInDir(t, specWtDir, "config", "user.name", "MindSpec Test")

			v.dirty(t, specWtDir)

			statusBefore, _ := nextReadyGateBDStatus(t, sb, beadID)
			branchesBefore := sb.ListBranches("bead/")
			worktreesBefore := sb.ListWorktrees()
			isSymlinkBefore := readSymlink(specWtDir, ".gitignore")
			fileSnapshotsBefore := map[string]string{}
			fileExistedBefore := map[string]bool{}
			for _, c := range v.checked {
				content, existed := snapshotFile(t, specWtDir, c.relPath)
				fileSnapshotsBefore[c.relPath] = content
				fileExistedBefore[c.relPath] = existed
			}

			out, err := runMindspecInDir(t, sb, specWtDir, "next", beadID)
			if err == nil {
				t.Fatalf("expected the claim to REFUSE (dirty, entry-absent .gitignore, round-1 panel G1); got exit 0:\n%s", out)
			}
			if !strings.Contains(out, "cannot claim work") {
				t.Errorf("expected the preserved DirtyTreeFailure contract, got:\n%s", out)
			}

			statusAfter, _ := nextReadyGateBDStatus(t, sb, beadID)
			if statusAfter != statusBefore {
				t.Errorf("bd status changed across the refused invocation: before=%q after=%q", statusBefore, statusAfter)
			}
			if !equalStringSlices(branchesBefore, sb.ListBranches("bead/")) {
				t.Errorf("bead branch list changed across the refused invocation: before=%v after=%v", branchesBefore, sb.ListBranches("bead/"))
			}
			if !equalStringSlices(worktreesBefore, sb.ListWorktrees()) {
				t.Errorf("worktree list changed across the refused invocation: before=%v after=%v", worktreesBefore, sb.ListWorktrees())
			}
			if got := readSymlink(specWtDir, ".gitignore"); got != isSymlinkBefore {
				t.Errorf(".gitignore symlink target changed across the refused invocation: before=%q after=%q", isSymlinkBefore, got)
			}
			for _, c := range v.checked {
				content, existed := snapshotFile(t, specWtDir, c.relPath)
				if existed != fileExistedBefore[c.relPath] {
					t.Errorf("%s existence changed across the refused invocation: before=%v after=%v", c.relPath, fileExistedBefore[c.relPath], existed)
				}
				if content != fileSnapshotsBefore[c.relPath] {
					t.Errorf("%s bytes changed across the refused invocation (a refusal must be a total no-op)", c.relPath)
				}
			}
		})
	}
}
