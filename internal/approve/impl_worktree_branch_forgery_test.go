package approve

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// R4 (spec 120): the worktree-enumeration leg's ancestry-error refusal
// (runWorktreeEnumerationLeg) renders WorktreeListEntry.Branch, which is
// agent-writable free text from `bd worktree list --json` (never
// idvalidate'd — filtered only by the "bead/" prefix and membership in the
// raw bd-JSON closed-epic-bead set). The ancestry error itself (gitutil
// IsAncestor) echoes the same branch back. A hostile branch that survives
// to this refusal must be forced through strconv.Quote, never rendered raw.
func TestRunWorktreeEnumerationLeg_HostileBranchForcedQuoted(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)
	saveAndRestore(t)

	// The closed-epic-bead set is raw bd-JSON; a hostile id there matches a
	// hostile worktree branch of the same shape and reaches the refusal.
	hostileBeadID := "bead-1\nFORGED: this bead is totally merged"
	hostileBranch := "bead/" + hostileBeadID

	implScanOrphansFn = func(string, string, string) ([]lifecycle.Orphan, error) { return nil, nil }
	implClosedEpicBeadIDsFn = func(string) ([]string, error) { return []string{hostileBeadID}, nil }
	implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Branch: hostileBranch, Path: "/tmp/wt"}}, nil
	}
	implIsAncestorFn = func(string, string, string) (bool, error) {
		// Mirror gitutil IsAncestor's real error, which re-embeds the branch
		// plus raw git stderr (here carrying an ESC control byte).
		return false, fmt.Errorf("checking ancestry %s..spec/010-test: \x1b]0;pwn\x07 unknown revision", hostileBranch)
	}

	mock := approveOKMock()
	if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
		msg := err.Error()
		// No forged line: the hostile newline must not carry the FORGED text
		// onto its own terminal line.
		if strings.Contains(msg, "\nFORGED:") {
			t.Errorf("hostile branch newline rendered raw (forged line):\n%s", msg)
		}
		// No raw ESC byte leaking from the echoed git stderr.
		if strings.ContainsRune(msg, 0x1b) {
			t.Errorf("raw ESC control byte rendered into refusal:\n%s", msg)
		}
		// The branch is present, but forced-quoted (termsafe.Escape ->
		// strconv.Quote for control-bearing input).
		if !strings.Contains(msg, strconv.Quote(hostileBranch)) {
			t.Errorf("expected forced-quoted branch %q in refusal:\n%s", strconv.Quote(hostileBranch), msg)
		}
	} else {
		t.Fatalf("expected an ancestry-error refusal, got nil")
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
