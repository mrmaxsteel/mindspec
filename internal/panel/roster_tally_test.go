package panel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTally_OffRosterSlotCountsNothing(t *testing.T) {
	dir, p := passFixture(t, 5, VerdictApprove)
	// Move the missing roster verdict to a name outside the verdict pattern.
	if err := os.Rename(filepath.Join(dir, "R5-round-1.json"), filepath.Join(dir, "saved-R5")); err != nil {
		t.Fatal(err)
	}
	writePassJSON(t, dir, "zz-extra-round-1.json", FindingVerdict{Verdict: VerdictApprove})
	r := passTally(t, dir)
	if r.Approves != 4 || len(r.Verdicts) != 4 || r.Complete() || len(r.Unexpected) != 1 || r.Unexpected[0].Slot != "zz-extra" {
		t.Fatalf("unexpected APPROVE counted: %+v", r)
	}
	if passDecision(t, dir).Action != Block {
		t.Fatal("unexpected slot completed panel")
	}
	writePassJSON(t, dir, "zz-extra-round-2.json", FindingVerdict{Verdict: VerdictApprove})
	if r = passTally(t, dir); r.LatestRound != 2 || !r.RoundMismatch || r.Approves != 0 {
		t.Fatalf("unexpected file round ignored: %+v (round %d)", r, p.Round)
	}
}
func TestTally_OffRosterRejectBlocks(t *testing.T) {
	testUnexpectedVeto(t, FindingVerdict{Verdict: VerdictReject})
}
func TestTally_OffRosterInvalidBlocks(t *testing.T) {
	for _, v := range []FindingVerdict{{Verdict: "INVALID"}, {Verdict: VerdictRequestChanges}, {Verdict: VerdictApprove, HardBlock: true}} {
		testUnexpectedVeto(t, v)
	}
}
func testUnexpectedVeto(t *testing.T, v FindingVerdict) {
	t.Helper()
	dir, _ := passFixture(t, 2, VerdictApprove)
	writePassJSON(t, dir, "zz-extra-round-1.json", v)
	r := passTally(t, dir)
	if len(r.Verdicts) != 2 || r.Approves != 2 {
		t.Fatalf("unexpected counted: %+v", r)
	}
	if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "zz-extra-round-1.json") {
		t.Fatalf("unexpected veto: %+v", d)
	}
}
func TestTally_LegacyNoRosterUnchanged(t *testing.T) {
	dir, p := passFixture(t, 2, VerdictApprove)
	p.Roster = nil
	writePassJSON(t, dir, FileName, p)
	writePassJSON(t, dir, "codex-R4-round-1.json", FindingVerdict{Verdict: VerdictApprove})
	r := passTally(t, dir)
	if r.Approves != 3 || len(r.Verdicts) != 3 || !r.Complete() || len(r.Unexpected) != 0 {
		t.Fatalf("legacy counts: %+v", r)
	}
}

func TestTally_RosterCompleteness(t *testing.T) {
	dir, p := passFixture(t, 3, VerdictApprove)
	p.ExpectedReviewers = 2
	writePassJSON(t, dir, FileName, p)
	if err := os.Rename(filepath.Join(dir, "R3-round-1.json"), filepath.Join(dir, "saved-R3")); err != nil {
		t.Fatal(err)
	}
	if r := passTally(t, dir); r.Complete() {
		t.Fatal("missing roster slot passed via a smaller legacy expected count")
	}
}
