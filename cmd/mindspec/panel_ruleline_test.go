package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func ruleLineFixture(t *testing.T) (*panel.Result, panel.GateFacts, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "review", "p")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data := `{"round":1,"expected_reviewers":7,"target":"main","approve_threshold":"7"}`
	if err := os.WriteFile(filepath.Join(dir, "panel.json"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	for _, slot := range []string{"R1", "R2", "R3", "R4", "R5", "R6", "R7"} {
		v := `{"verdict":"APPROVE"}`
		if slot == "R6" {
			v = `{"verdict":"REQUEST_CHANGES","concrete_changes_required":["fix it"]}`
		}
		if slot == "R7" {
			v = `{"verdict":"INVALID"}`
		}
		if err := os.WriteFile(filepath.Join(dir, slot+"-round-1.json"), []byte(v), 0644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	facts := panel.GateFacts{Reg: &panel.Registration{Dir: dir, Panel: *r.Panel}, Res: r, WorktreeAbsent: true}
	return r, facts, root
}
func TestRuleLine_SharedRender(t *testing.T) {
	r, facts, root := ruleLineFixture(t)
	tally, _ := renderPanelTally(r, facts, nil)
	verify, _ := renderPanelVerify(r, facts)
	state := instruct.PanelState(root)
	for name, out := range map[string]string{"tally": tally, "verify": verify, "instruct": state} {
		if strings.Count(out, panel.RuleLine(r)) != 1 {
			t.Errorf("%s missing shared rule or renders twice:\n%s", name, out)
		}
		if !strings.Contains(out, "approve_threshold 7 ignored (legacy; not decision-bearing)") {
			t.Errorf("%s missing legacy notice:\n%s", name, out)
		}
	}
}
func TestRuleLine_INVALIDCount(t *testing.T) {
	r, _, _ := ruleLineFixture(t)
	line := panel.RuleLine(r)
	if !strings.Contains(line, "5 APPROVE, 0 resolved, 1 unresolved") || !strings.HasSuffix(line, ", 1 INVALID") {
		t.Fatal(line)
	}
}

func TestPanelTally_UnexpectedSlotOutput(t *testing.T) {
	for _, payload := range []string{`{"verdict":"APPROVE"}`, `{not json`} {
		t.Run(payload, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"panel.json":            `{"round":1,"expected_reviewers":2,"target":"main","roster":[{"slot":"R1"},{"slot":"R2"}]}`,
				"R1-round-1.json":       `{"verdict":"APPROVE"}`,
				"R2-round-1.json":       `{"verdict":"APPROVE"}`,
				"zz-extra-round-1.json": payload,
			}
			for name, data := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
					t.Fatal(err)
				}
			}
			r, err := panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			facts := panel.GateFacts{Reg: &panel.Registration{Dir: dir, Panel: *r.Panel}, Res: r, WorktreeAbsent: true}
			out, d := renderPanelTally(r, facts, nil)
			want := "unexpected slot: zz-extra (zz-extra-round-1.json)\n"
			if payload == "{not json" {
				want = "malformed (counted as missing): zz-extra-round-1.json\n"
			}
			if d.Action != panel.Allow || !strings.Contains(out, want) || !strings.HasSuffix(out, "decision: PASS") {
				t.Fatalf("unexpected file tally: %+v\n%s", d, out)
			}
		})
	}
}
