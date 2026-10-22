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

func TestTally_UnparseableCountsNothing(t *testing.T) {
	for _, placement := range []string{"legacy extra", "off roster", "on roster"} {
		t.Run(placement, func(t *testing.T) {
			dir, p := passFixture(t, 2, VerdictApprove)
			name := "zz-extra-round-1.json"
			wantAction := Allow
			if placement == "legacy extra" {
				p.Roster = nil
				writePassJSON(t, dir, FileName, p)
			}
			if placement == "on roster" {
				name = "R1-round-1.json"
				wantAction = Block
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte("{not json"), 0644); err != nil {
				t.Fatal(err)
			}
			r := passTally(t, dir)
			if len(r.Malformed) != 1 || r.Malformed[0] != name || r.InvalidCount() != 0 || len(r.Unexpected) != 0 {
				t.Fatalf("unparseable file must only be listed: %+v", r)
			}
			if !strings.HasSuffix(RuleLine(r), ", 0 INVALID") {
				t.Fatal(RuleLine(r))
			}
			if d := passDecision(t, dir); d.Action != wantAction {
				t.Fatalf("unparseable file: %+v", d)
			}
			vote, _ := r.VoteDecision()
			if (vote == VotePass) != (wantAction == Allow) {
				t.Fatalf("vote disagrees: %v", vote)
			}
			if placement == "on roster" && (r.Complete() || r.MissingCount() != 1) {
				t.Fatalf("unparseable roster verdict must count as missing: %+v", r)
			}
		})
	}
}

func TestTally_UnsafeVerdictIsInvalid(t *testing.T) {
	for _, name := range []string{"R1-round-1.json", "zz-extra-round-1.json"} {
		t.Run(name, func(t *testing.T) {
			dir, _ := passFixture(t, 2, VerdictApprove)
			if name == "R1-round-1.json" {
				if err := os.Rename(filepath.Join(dir, name), filepath.Join(dir, name+".saved")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(filepath.Join(dir, "absent"), filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			r := passTally(t, dir)
			if r.InvalidCount() != 1 || len(r.Malformed) != 0 || r.PassRule(true) {
				t.Fatalf("unsafe verdict must be INVALID: %+v", r)
			}
			verdicts := r.Verdicts
			if name != "R1-round-1.json" {
				verdicts = r.Unexpected
			}
			if len(verdicts) == 0 || verdicts[0].Verdict != "INVALID" || verdicts[0].ReadError == "" {
				t.Fatalf("unsafe verdict must retain its read error: %+v", verdicts)
			}
			if d := passDecision(t, dir); d.Action != Block {
				t.Fatalf("unsafe verdict must block: %+v", d)
			}
		})
	}
}

func TestTally_ParseableInvalidBlocks(t *testing.T) {
	for _, payload := range []string{`{"verdict":""}`, `{"verdict":"  "}`, `{}`, `{"verdict":"unknown"}`} {
		for _, name := range []string{"R1-round-1.json", "zz-extra-round-1.json"} {
			t.Run(name+payload, func(t *testing.T) {
				dir, _ := passFixture(t, 2, VerdictApprove)
				if err := os.WriteFile(filepath.Join(dir, name), []byte(payload), 0644); err != nil {
					t.Fatal(err)
				}
				r := passTally(t, dir)
				if r.InvalidCount() != 1 || len(r.Malformed) != 0 || !r.Complete() {
					t.Fatalf("parseable INVALID is present, not missing: %+v", r)
				}
				if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "INVALID recorded") || strings.Contains(d.Message, "HARD block / REJECT") {
					t.Fatalf("INVALID veto: %+v", d)
				} else if name == "zz-extra-round-1.json" && !strings.Contains(d.Message, name) {
					t.Fatalf("unexpected INVALID file not named: %+v", d)
				}
				if vote, msg := r.VoteDecision(); vote != VoteBlock || !strings.Contains(msg, "INVALID recorded") || strings.Contains(msg, "REJECT") || strings.Contains(msg, "hard_block") {
					t.Fatalf("INVALID vote veto: %v %s", vote, msg)
				}
			})
		}
	}
}

func TestTally_FindingShapeOnlyInvalidatesRequestChanges(t *testing.T) {
	for _, verdict := range []string{VerdictApprove, VerdictReject, VerdictRequestChanges} {
		t.Run(verdict, func(t *testing.T) {
			dir, _ := passFixture(t, 2, VerdictApprove)
			writePassJSON(t, dir, "R1-round-1.json", map[string]interface{}{
				"verdict": verdict, "concrete_changes_required": []interface{}{map[string]int{"x": 1}},
			})
			r := passTally(t, dir)
			wantInvalid := 0
			wantAction := Block
			if verdict == VerdictRequestChanges {
				wantInvalid = 1
			}
			if verdict == VerdictApprove {
				wantAction = Allow
			}
			if r.InvalidCount() != wantInvalid || !r.Complete() {
				t.Fatalf("finding shape: %+v", r)
			}
			if d := passDecision(t, dir); d.Action != wantAction {
				t.Fatalf("finding shape: %+v", d)
			}
		})
	}
}
