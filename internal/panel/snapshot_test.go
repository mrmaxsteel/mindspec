package panel

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestGateV2_LedgerSnapshotReplacement(t *testing.T) {
	for _, reader := range []string{"tally", "statuses"} {
		t.Run(reader, func(t *testing.T) {
			dir, target, v := gateV2Fixture(t)
			gateV2Submit(t, target, v)
			if !gateV2Read(t, dir).PassRule(true) {
				t.Fatal("prerequisite: all original approvals must pass")
			}
			seedTestRecords(t, dir, submitLaunch(1, 1, "dispatched"))
			before := gateV2Read(t, dir)
			if before.PassRule(true) {
				t.Fatal("prerequisite: a dispatched replacement must block")
			}
			reads := 0
			readSnapshot := func(dir string) ([][]byte, error) {
				reads++
				lines, err := ReadLedger(dir)
				if err != nil {
					return nil, err
				}
				if reads != 1 {
					t.Fatal("reader took more than one ledger snapshot")
				}
				v.Verdict = "REQUEST_CHANGES"
				v.ConcreteChangesRequired = []ChangeRequired{{Text: "new defect", Basis: "traced"}}
				raw, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				crash := errors.New("stop before verdict rename")
				target.AfterAppend = func() error { return crash }
				if _, err := Submit(target, raw, gateV2Resolve); !errors.Is(err, crash) {
					t.Fatalf("replacement did not append before rename: %v", err)
				}
				return lines, nil
			}
			var result *Result
			if reader == "tally" {
				var err error
				result, err = tally(dir, readSnapshot)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				slots, err := readSlotStatuses(before, nil, time.Now(), readSnapshot)
				if err != nil {
					t.Fatal(err)
				}
				if len(slots) != 4 || slots[0].Status != "running" || VerifyExitCode(slots) != 1 {
					t.Errorf("replacement in the snapshot must remain running: %+v", slots)
				}
				result = before
			}
			if reads != 1 {
				t.Fatalf("snapshot seam was not used: %d reads", reads)
			}
			for _, res := range []*Result{result, gateV2Read(t, dir)} {
				reg := Registration{Dir: dir, Panel: *res.Panel}
				decision := PanelGateDecision(GateFacts{Reg: &reg, Res: res, HeadSHA: res.Panel.ReviewedHeadSHA, WorktreeAbsent: true})
				if res.PassRule(true) || decision.Action != Block {
					t.Errorf("replacement must block: pass=%v gate=%v approvals=%d", res.PassRule(true), decision.Action, res.Approves)
				}
			}
		})
	}
}
