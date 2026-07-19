package approve

// Spec 120 R4 cluster 2 (round-5 fix-up) merged with Bead 2's AC-25
// read-gate: readPlanBeadIDs reads bead_ids from the AGENT-AUTHORED
// plan.md YAML frontmatter (internal/approve/impl.go) and now
// idvalidate's every entry at the read boundary — a malformed entry is
// REFUSED before any functional bd lookup can consume it, and the
// refusal's DISPLAY position forces the malformed ID through
// strconv.Quote (idrender.Bead), never rendering it raw. Genuine plan
// bead IDs still flow through unchanged to the FUNCTIONAL bd lookups
// (readBeadStatus's `bd show`), and the downstream display positions
// (status gate 1/3, Leg 3 recourse) keep their idrender discipline —
// byte-identical for valid IDs.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// hostilePlanBeadID is a YAML-plain-scalar-safe (no colon, no leading
// dash-space ambiguity) malformed-but-printable bead ID, mirroring the
// idrender_test.go "120-x;evil" discriminator: termsafe.Escape alone
// passes it through unchanged (it is printable ASCII), so only
// idrender.Bead's idvalidate-keyed identity forces it to quote.
const hostilePlanBeadID = "bead-1;evil"

func TestApproveImpl_HostilePlanBeadID_OpenStatusForcedQuoted(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{hostilePlanBeadID})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	saveAndRestore(t)

	var sawFunctionalHostileID bool
	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			if args[1] == hostilePlanBeadID {
				sawFunctionalHostileID = true
			}
			payload := []map[string]string{{"status": "in_progress"}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := approveOKMock()
	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected the AC-25 read-gate to refuse a malformed plan-frontmatter bead ID")
	}
	msg := err.Error()

	// Bead 2's AC-25 read-gate refuses BEFORE any functional bd lookup —
	// the hostile ID must never reach a `bd show` argv.
	if sawFunctionalHostileID {
		t.Fatal("a malformed plan-frontmatter bead ID must be refused before reaching readBeadStatus's bd show")
	}

	// Bead 5's R4 display discipline on the refusal itself: the malformed
	// ID renders forced-quoted (idrender.Bead), never raw.
	wantQuoted := strconv.Quote(hostilePlanBeadID)
	if !strings.Contains(msg, fmt.Sprintf("plan frontmatter bead_ids entry %s is not a valid bead ID", wantQuoted)) {
		t.Errorf("read-gate refusal must render the forced-quoted bead ID, got:\n%s", msg)
	}
	if strings.Contains(msg, "entry "+hostilePlanBeadID+" is not") {
		t.Errorf("read-gate refusal rendered the malformed bead ID raw:\n%s", msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("expected a final recovery line: %v", msg)
	}
}

func TestApproveImpl_HostilePlanBeadID_StatusReadErrorForcedQuoted(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	// Bead 2's AC-25 read-gate means a malformed entry can no longer reach
	// readBeadStatus at all — the status-read-error DISPLAY position is
	// exercised with a genuine ID (byte-identical through idrender.Bead),
	// while the hostile-ID case is pinned as a read-gate refusal above.
	const cleanID = "mindspec-9cyu.2"
	writePlanWithBeads(t, tmp, "010-test", []string{cleanID})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == cleanID {
			return nil, fmt.Errorf("bd unavailable")
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}

	mock := approveOKMock()
	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected an error when readBeadStatus itself fails")
	}
	msg := err.Error()
	want := fmt.Sprintf("checking bead %s status: bd unavailable", cleanID)
	if !strings.Contains(msg, want) {
		t.Errorf("status-read-error wrap must render the clean bead ID byte-identically, got:\n%s\nwant substring: %s", msg, want)
	}
}

// TestApproveImpl_CleanPlanBeadID_OpenStatusByteIdentical is the
// clean-fixture counterpart (F3 discipline): a genuine bead ID renders
// byte-identically through the same gate.
func TestApproveImpl_CleanPlanBeadID_OpenStatusByteIdentical(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	const cleanID = "mindspec-9cyu.1"
	writePlanWithBeads(t, tmp, "010-test", []string{cleanID})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			payload := []map[string]string{{"status": "in_progress"}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := approveOKMock()
	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected an error when the plan bead is still open")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bead "+cleanID+" is still \"in_progress\"") {
		t.Errorf("clean bead ID must render byte-identically, got:\n%s", msg)
	}
	wantRecovery := "recovery: mindspec complete " + cleanID
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	if got := lines[len(lines)-1]; got != wantRecovery {
		t.Errorf("final recovery line = %q, want %q", got, wantRecovery)
	}
}

// TestImplObligationRefusal_HostileBeadIDForcedQuoted pins the Leg 3
// durable-obligation backstop's recourse (implObligationRefusal) directly:
// beadID flows in from planBeadIDs unvalidated, so a hostile value — here
// one carrying a RAW NEWLINE, a shape plan.md's YAML text cannot itself
// carry but downstream metadata/bd-store corruption could surface as a
// beadID string — must never reach guard.NewFailure raw. Proof it is
// load-bearing, not cosmetic: guard.FormatFailure PANICS on a recovery
// command containing a raw newline (internal/guard/recovery.go), so an
// un-escaped beadID here would crash the gate instead of merely
// mis-rendering it.
func TestImplObligationRefusal_HostileBeadIDForcedQuoted(t *testing.T) {
	saveAndRestore(t)
	hostileID := "bead-1\nrm -rf /;evil"
	wantQuoted := strconv.Quote(hostileID)
	// cause mirrors CheckPendingObligations' OWN already-fixed rendering
	// (panel_advisory.go, R4 cluster 2): its message body already carries
	// idrender.Bead(beadID), never the raw ID. implObligationRefusal
	// passes cause.Error() through verbatim — this test's job is the
	// recourse LINES implObligationRefusal itself constructs, not
	// re-proving CheckPendingObligations' own escape.
	cause := fmt.Errorf("bead %s carries an unresolved refutation_pending obligation", wantQuoted)

	t.Run("branch exists — bare recovery command forced-quoted", func(t *testing.T) {
		implBranchExistsFn = func(name string) bool { return true }
		err := implObligationRefusal(hostileID, cause)
		if err == nil {
			t.Fatal("implObligationRefusal returned nil")
		}
		msg := err.Error()
		if strings.Contains(msg, "\nrm -rf /;evil") {
			t.Errorf("raw newline from beadID leaked into the message: %q", msg)
		}
		wantRecovery := "recovery: mindspec complete " + wantQuoted
		if !strings.HasSuffix(msg, wantRecovery) {
			t.Errorf("expected suffix %q, got: %q", wantRecovery, msg)
		}
	})

	t.Run("branch missing — restoration line names the forced-quoted bead ID", func(t *testing.T) {
		implBranchExistsFn = func(name string) bool { return false }
		err := implObligationRefusal(hostileID, cause)
		if err == nil {
			t.Fatal("implObligationRefusal returned nil")
		}
		msg := err.Error()
		if strings.Contains(msg, "\nrm -rf /;evil") {
			t.Errorf("raw newline from beadID leaked into the message: %q", msg)
		}
		wantBranchMention := "restore the bead/" + wantQuoted + " branch ref"
		if !strings.Contains(msg, wantBranchMention) {
			t.Errorf("branch-less recourse must name the forced-quoted branch, got:\n%s\nwant substring: %s", msg, wantBranchMention)
		}
		wantRecovery := "recovery: mindspec complete " + wantQuoted
		if !strings.HasSuffix(msg, wantRecovery) {
			t.Errorf("expected suffix %q, got: %q", wantRecovery, msg)
		}
	})
}

// TestImplObligationRefusal_CleanBeadIDByteIdentical is the clean-fixture
// counterpart: a genuine bead ID's recourse is unchanged from before the
// escape.
func TestImplObligationRefusal_CleanBeadIDByteIdentical(t *testing.T) {
	saveAndRestore(t)
	const cleanID = "mindspec-9cyu.1"
	cause := fmt.Errorf("bead %s carries an unresolved refutation_pending obligation", cleanID)

	implBranchExistsFn = func(name string) bool { return true }
	err := implObligationRefusal(cleanID, cause)
	if err == nil {
		t.Fatal("implObligationRefusal returned nil")
	}
	want := "recovery: mindspec complete " + cleanID
	if got := err.Error(); !strings.HasSuffix(got, want) {
		t.Errorf("clean-fixture recovery line changed:\ngot:  %s\nwant suffix: %s", got, want)
	}

	implBranchExistsFn = func(name string) bool { return false }
	err = implObligationRefusal(cleanID, cause)
	if err == nil {
		t.Fatal("implObligationRefusal returned nil")
	}
	wantBranchMention := "restore the bead/" + cleanID + " branch ref"
	if got := err.Error(); !strings.Contains(got, wantBranchMention) {
		t.Errorf("clean-fixture branch mention changed:\ngot:  %s\nwant substring: %s", got, wantBranchMention)
	}
}
