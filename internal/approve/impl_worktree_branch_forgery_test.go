package approve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// R4 (spec 120 Bead 5) merged with Bead 2's reverse-derivation gate
// (AC-23): the worktree-enumeration leg (runWorktreeEnumerationLeg)
// parses a beadID back OUT of an agent-creatable worktree branch and now
// SKIPS any malformed candidate — a hostile branch can no longer reach
// the ancestry refusal at all. The ancestry-error refusal itself still
// re-embeds free-text git stderr (gitutil IsAncestor), so Bead 5's R4
// escaping must keep neutralizing control bytes and forged lines there
// even for a clean (gate-passing) branch.
func TestRunWorktreeEnumerationLeg_HostileBranchForcedQuoted(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)
	saveAndRestore(t)

	// The closed-epic-bead set is raw bd-JSON; a hostile id there matches a
	// hostile worktree branch of the same shape.
	hostileBeadID := "bead-1\nFORGED: this bead is totally merged"
	hostileBranch := "bead/" + hostileBeadID
	cleanBranch := "bead/bead-1"

	implScanOrphansFn = func(string, string, string) ([]lifecycle.Orphan, error) { return nil, nil }
	implClosedEpicBeadIDsFn = func(string) ([]string, error) {
		return []string{hostileBeadID, "bead-1"}, nil
	}
	implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Branch: hostileBranch, Path: "/tmp/wt-hostile"},
			{Branch: cleanBranch, Path: "/tmp/wt"},
		}, nil
	}
	var sawBranches []string
	implIsAncestorFn = func(_, branch, _ string) (bool, error) {
		sawBranches = append(sawBranches, branch)
		// Mirror gitutil IsAncestor's real error, which re-embeds the branch
		// plus raw git stderr (here carrying an ESC control byte and a
		// forged-line payload).
		return false, fmt.Errorf("checking ancestry %s..spec/010-test: \x1b]0;pwn\x07 unknown revision\nFORGED: this bead is totally merged", branch)
	}

	err := runWorktreeEnumerationLeg(tmp, "010-test", "spec/010-test")
	if err == nil {
		t.Fatal("expected an ancestry-error refusal for the clean branch")
	}
	msg := err.Error()

	// Bead 2's reverse-derivation gate: the malformed derived beadID is
	// skipped — the hostile branch must never reach the ancestry check.
	for _, b := range sawBranches {
		if b == hostileBranch {
			t.Errorf("hostile branch reached the ancestry check (reverse-derivation gate bypassed)")
		}
	}
	// Bead 5's R4 escaping on the refusal: no forged standalone line, no
	// raw ESC byte leaking from the echoed git stderr.
	if strings.Contains(msg, "\nFORGED:") {
		t.Errorf("hostile git-stderr newline rendered raw (forged line):\n%s", msg)
	}
	if strings.ContainsRune(msg, 0x1b) {
		t.Errorf("raw ESC control byte rendered into refusal:\n%s", msg)
	}
	// The clean branch still renders byte-identically in the refusal body.
	if !strings.Contains(msg, "could not verify worktree branch "+cleanBranch) {
		t.Errorf("expected the clean branch byte-identical in refusal:\n%s", msg)
	}
}

// A genuine (clean) branch that reaches the same refusal renders
// byte-identical — termsafe.Escape is the identity on printable input, so
// the operator-facing message is unchanged for real refs.
func TestRunWorktreeEnumerationLeg_CleanBranchByteIdentical(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)
	saveAndRestore(t)

	cleanBranch := "bead/bead-1"

	implScanOrphansFn = func(string, string, string) ([]lifecycle.Orphan, error) { return nil, nil }
	implClosedEpicBeadIDsFn = func(string) ([]string, error) { return []string{"bead-1"}, nil }
	implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Branch: cleanBranch, Path: "/tmp/wt"}}, nil
	}
	implIsAncestorFn = func(string, string, string) (bool, error) {
		return false, fmt.Errorf("plain git failure")
	}

	mock := approveOKMock()
	if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
		msg := err.Error()
		// The clean branch appears verbatim (not quoted), proving the R4
		// escape is a no-op for genuine refs.
		if !strings.Contains(msg, "worktree branch "+cleanBranch+" is merged") {
			t.Errorf("clean branch not rendered byte-identical:\n%s", msg)
		}
	} else {
		t.Fatalf("expected an ancestry-error refusal, got nil")
	}
}
