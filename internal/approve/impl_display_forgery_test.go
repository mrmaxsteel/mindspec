package approve

// Spec 120 R4 (converging pass): implOrphanRefusal is the impl.go call
// site that renders lifecycle.Orphan fields directly (o.BeadID,
// o.BeadBranch, o.SpecBranch — all unvalidated bd-list data, see
// internal/lifecycle/orphans.go). These tests pin that a hostile Orphan
// can never render unescaped through this specific function, independent
// of the full ApproveImpl orphan-gate wiring already covered by
// orphan_gate_test.go.

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

func TestImplOrphanRefusal_HostileOrphanFieldsForcedSafe(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")

	hostileBeadID := "bead-x\n--force"
	hostileBranch := "bead/bead-x\x1b[31mFAKE\x1b[0m;evil"
	o := lifecycle.Orphan{
		BeadID:     hostileBeadID,
		BeadBranch: hostileBranch,
		SpecBranch: "spec/010-test",
	}

	err := implOrphanRefusal(tmp, "010-test", o)
	if err == nil {
		t.Fatal("implOrphanRefusal returned nil")
	}
	msg := err.Error()

	// The hostile bead ID is an ID-typed position: it must be forced
	// through strconv.Quote (idrender.Bead), not rendered raw.
	wantQuoted := strconv.Quote(hostileBeadID)
	if !strings.Contains(msg, wantQuoted) {
		t.Errorf("refusal message missing forced-quoted bead ID %q:\n%s", wantQuoted, msg)
	}
	// The raw hostile branch (a control-byte-bearing free-text position)
	// must be escaped, not rendered raw — termsafe.Escape strips/encodes
	// the ESC bytes so they can never move the cursor or fake a line.
	if strings.ContainsRune(msg, 0x1b) {
		t.Errorf("refusal message contains a raw ESC control byte from BeadBranch:\n%q", msg)
	}
	// guard.NewFailure joins the body and o.RecoveryCommand() with exactly
	// one structural newline (ADR-0035's recovery-line convention); the
	// hostile newline embedded in BeadID must be folded into the escaped
	// `\n` two-byte sequence by idrender.Bead's strconv.Quote — never
	// surface as a SECOND real newline that forges an extra terminal
	// line. (guard.FormatFailure would itself panic on a recovery command
	// containing a raw newline — proving idrender.Bead is load-bearing
	// here, not merely cosmetic.)
	if got := strings.Count(msg, "\n"); got != 1 {
		t.Errorf("refusal message has %d real newlines, want exactly 1 (body/recovery-line separator); hostile newline leaked raw: %q", got, msg)
	}
	if !strings.HasSuffix(msg, "recovery: mindspec complete "+wantQuoted) {
		t.Errorf("refusal message's final recovery line was not the forced-quoted RecoveryCommand:\n%s", msg)
	}
}

// TestImplOrphanRefusal_CleanOrphanByteIdentical is the clean-fixture
// counterpart (F3 discipline): a genuine bead ID and ordinary branch names
// must still render byte-identically through the escaping helpers.
func TestImplOrphanRefusal_CleanOrphanByteIdentical(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")

	const cleanBeadID = "mindspec-9cyu.1"
	o := lifecycle.Orphan{
		BeadID:     cleanBeadID,
		BeadBranch: "bead/" + cleanBeadID,
		SpecBranch: "spec/010-test",
	}

	err := implOrphanRefusal(tmp, "010-test", o)
	if err == nil {
		t.Fatal("implOrphanRefusal returned nil")
	}
	msg := err.Error()
	wantPrefix := "bead " + cleanBeadID + " (branch bead/" + cleanBeadID + ") was closed without running mindspec complete and is not merged into spec/010-test"
	if !strings.HasPrefix(msg, wantPrefix) {
		t.Errorf("clean-fixture message changed:\ngot:  %s\nwant prefix: %s", msg, wantPrefix)
	}
}
