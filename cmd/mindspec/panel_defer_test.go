package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// fakeDeferralBD is mutable between the write and the lifecycle re-check.
func fakeDeferralBD(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rebindWrite(t, dir, "bd", "#!/bin/sh\ncase \"$1\" in\nlist) cat '"+dir+"/epics.json';;\nshow) cat '"+dir+"/'\"$2\"'.json';;\n*) exit 1;;\nesac\n")
	if err := os.Chmod(filepath.Join(dir, "bd"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	rebindWrite(t, dir, "epics.json", `[{"id":"epic-x","metadata":{"spec_num":900,"spec_title":"demo"}}]`)
	rebindWrite(t, dir, "follow-up.json", `[{"id":"follow-up","status":"open","description":"missing test\nsecond ask","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
	return dir
}
func deferArgs() []string {
	return []string{"defer", "p", "--slot", "R1", "--round", "1", "--bead", "follow-up", "--by", "max"}
}
func TestPanelDefer_BDChecks(t *testing.T) {
	for _, name := range []string{"closed", "missing", "epic", "child", "grandchild", "parent-child dependency", "reviewed bead edge", "partial slot text", "no edge", "wrong edge", "wrong text", "bd error"} {
		t.Run(name, func(t *testing.T) {
			root, _ := setupDischarge(t)
			fake := fakeDeferralBD(t)
			issue := map[string]interface{}{"id": "follow-up", "status": "open", "description": "missing test\nsecond ask", "dependencies": []map[string]string{{"id": "epic-x", "dependency_type": "discovered-from"}}}
			args := deferArgs()
			switch name {
			case "closed":
				issue["status"] = "closed"
			case "epic":
				args = replaceRefuteArg(args, "--bead", "epic-x")
				issue["id"] = "epic-x"
			case "child":
				issue["parent"] = "epic-x"
			case "grandchild":
				issue["parent"] = "child-x"
				rebindWrite(t, fake, "child-x.json", `[{"id":"child-x","parent":"epic-x"}]`)
			case "parent-child dependency":
				issue["dependencies"] = []map[string]string{{"id": "child-x", "dependency_type": "parent-child"}, {"id": "epic-x", "dependency_type": "discovered-from"}}
				rebindWrite(t, fake, "child-x.json", `[{"id":"child-x","parent":"epic-x"}]`)
			case "reviewed bead edge":
				issue["dependencies"] = []map[string]string{{"id": "probe-x.1", "dependency_type": "discovered-from"}}
			case "partial slot text":
				issue["description"] = "missing test"
			case "no edge":
				issue["dependencies"] = []interface{}{}
			case "wrong edge":
				issue["dependencies"] = []map[string]string{{"id": "unrelated", "dependency_type": "discovered-from"}}
			case "wrong text":
				issue["description"] = "other"
			}
			raw, _ := json.Marshal([]interface{}{issue})
			file := "follow-up.json"
			if name == "epic" {
				file = "epic-x.json"
			}
			if name == "missing" {
				raw = []byte(`[]`)
			}
			if name == "bd error" {
				raw = []byte(`bad json`)
			}
			rebindWrite(t, fake, file, string(raw))
			if name == "reviewed bead edge" {
				requirePanelVerb(t, args...)
			} else {
				assertPanelRefusal(t, root, args...)
			}
		})
	}
}
func TestPanelDefer_SharedRefusals(t *testing.T) {
	for _, name := range []string{"APPROVE", "REJECT", "hard_block", "INVALID", "empty RC", "absent slot", "stale round", "future round", "unknown finding", "empty by", "invalid bead"} {
		t.Run(name, func(t *testing.T) {
			root, dir := setupDischarge(t)
			fakeDeferralBD(t)
			args := deferArgs()
			switch name {
			case "APPROVE", "REJECT", "INVALID":
				writeDischargeVerdict(t, dir, "R1", panel.FindingVerdict{Verdict: name})
			case "hard_block":
				writeDischargeVerdict(t, dir, "R1", panel.FindingVerdict{Verdict: "REQUEST_CHANGES", HardBlock: true, Rationale: "missing test"})
			case "empty RC":
				writeDischargeVerdict(t, dir, "R1", panel.FindingVerdict{Verdict: "REQUEST_CHANGES"})
			case "absent slot":
				args = replaceRefuteArg(args, "--slot", "R9")
			case "future round":
				args = replaceRefuteArg(args, "--round", "2")
			case "stale round":
				res, err := panel.Tally(dir)
				if err != nil {
					t.Fatal(err)
				}
				if err := paneltest.CreateV1(dir, panel.CreateInput{Spec: res.Panel.Spec, Gate: res.Panel.Gate, Target: res.Panel.Target, Round: 2, ReviewedHeadSHA: res.Panel.ReviewedHeadSHA, SubjectDigest: res.Panel.SubjectDigest, Roster: res.Panel.Roster, ExpectedReviewers: 2}); err != nil {
					t.Fatal(err)
				}
				for _, slot := range []string{"R1", "R2"} {
					raw, err := os.ReadFile(filepath.Join(dir, slot+"-round-1.json"))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, slot+"-round-2.json"), raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "unknown finding":
				args = append(args, "--finding", "ccr[9]")
			case "empty by":
				args = replaceRefuteArg(args, "--by", " ")
			case "invalid bead":
				args = replaceRefuteArg(args, "--bead", "--bad")
			}
			assertPanelRefusal(t, root, args...)
		})
	}
}
func TestPanelDefer_NoEpicGates(t *testing.T) {
	for _, gate := range []string{"spec_approve", "adhoc"} {
		t.Run(gate, func(t *testing.T) {
			root, dir := setupDischarge(t)
			// Isolate the gate-kind refusal from the snapshot integrity refusal.
			res, _ := panel.Tally(dir)
			res.Panel.Gate = gate
			if err := panel.WriteSnapshot(dir, *res.Panel); err != nil {
				t.Fatal(err)
			}
			before := panelTreeBytes(t, root)
			out, err := runPanelVerbCmd(deferArgs()...)
			if err == nil || !strings.Contains(err.Error(), "mindspec panel refute") {
				t.Fatalf("want refute recovery: %v %s", err, out)
			}
			if !reflect.DeepEqual(before, panelTreeBytes(t, root)) {
				t.Fatal("refusal wrote files")
			}
		})
	}
}
func TestPanelDefer_Writes(t *testing.T) {
	root, dir := setupDischarge(t)
	fakeDeferralBD(t)
	requirePanelVerb(t, deferArgs()...)
	before := panelTreeBytes(t, root)
	requirePanelVerb(t, deferArgs()...)
	if !reflect.DeepEqual(before, panelTreeBytes(t, root)) {
		t.Fatal("rerun did not converge")
	}
	lines, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("want one batch: %s", lines)
	}
	var event panel.DeferralRecord
	var row panel.DispositionRow
	if err := json.Unmarshal(lines[1], &event); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lines[2], &row); err != nil {
		t.Fatal(err)
	}
	if event.Record != "deferral" || event.Spec != "900-demo" || event.Panel != "p" || event.Bead != "follow-up" || row.Disposition != "confirmed-deferred" || row.Reviewer != "R1" || row.Model != "claude" || row.ID != "p:R1:r1:slot:"+event.FindingSHA256+":"+event.SourceVerdict+":"+event.SourceFileSHA256+":deferral" {
		t.Fatalf("bad write: %+v %+v", event, row)
	}
	res, _ := panel.Tally(dir)
	if !res.PassRule(true) || len(res.EffectiveDeferrals()) != 1 || res.IntegrityErr != nil {
		t.Fatalf("defer did not resolve: %+v", res)
	}
	res.Panel.Deferrals = nil
	if err := panel.WriteSnapshot(dir, *res.Panel); err != nil {
		t.Fatal(err)
	}
	requirePanelVerb(t, deferArgs()...)
	after, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err = panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lines, after) || res.IntegrityErr != nil || len(res.EffectiveDeferrals()) != 1 {
		t.Fatal("crash retry did not converge")
	}
}
func TestGateTime_DeferThenCloseBlocks(t *testing.T) {
	for _, change := range []string{"closed", "edge removed"} {
		t.Run(change, func(t *testing.T) {
			_, dir := setupDischarge(t)
			fake := fakeDeferralBD(t)
			requirePanelVerb(t, deferArgs()...)
			res, _ := panel.Tally(dir)
			reg := panel.Registration{Dir: dir, Panel: *res.Panel}
			if err := complete.CheckPanelDeferrals(reg, res.EffectiveDeferrals(), false); err != nil {
				t.Fatal(err)
			}
			body := `[{"id":"follow-up","status":"closed","description":"missing test\nsecond ask","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`
			if change == "edge removed" {
				body = `[{"id":"follow-up","status":"open","description":"missing test\nsecond ask"}]`
			}
			rebindWrite(t, fake, "follow-up.json", body)
			err := complete.CheckPanelDeferrals(reg, res.EffectiveDeferrals(), false)
			if err == nil || !strings.Contains(err.Error(), "follow-up") || !strings.Contains(err.Error(), "R1") {
				t.Fatalf("changed deferral allowed: %v", err)
			}
		})
	}
}

func TestLedger_RecordKinds(t *testing.T) {
	for _, kind := range []string{"round", "refutation", "deferral", "abandon", "rebind"} {
		t.Run(kind, func(t *testing.T) {
			var dir string
			if kind == "rebind" {
				root, d, _ := setupRebind(t)
				dir = d
				rebindWrite(t, root, "source.go", "package changed\n")
				rebindGit(t, root, "add", "source.go")
				rebindGit(t, root, "commit", "-qm", "change")
				if out, err := runPanelVerbCmd("rebind", "demo", "--reason", "max: checked"); err != nil {
					t.Fatalf("%v %s", err, out)
				}
			} else {
				_, dir = setupDischarge(t)
				switch kind {
				case "refutation":
					requirePanelVerb(t, refuteArgs()...)
				case "deferral":
					fakeDeferralBD(t)
					requirePanelVerb(t, deferArgs()...)
				case "abandon":
					requirePanelVerb(t, "abandon", "p", "--reason", "rework", "--by", "max")
				}
			}
			lines, err := panel.ReadLedger(dir)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range lines {
				var row map[string]interface{}
				if err := json.Unmarshal(line, &row); err != nil {
					t.Fatal(err)
				}
				if row["record"] == kind {
					found = true
					if row["spec"] != "900-demo" || row["panel"] != filepath.Base(dir) {
						t.Fatalf("missing provenance: %s", line)
					}
				}
			}
			if !found {
				t.Fatalf("verb did not append %s", kind)
			}
		})
	}
	t.Run("adhoc", func(t *testing.T) {
		root := setupCreateIntegrity(t)
		dir := filepath.Join(root, "review", "adhoc")
		if err := paneltest.CreateV1(dir, panel.CreateInput{Gate: "adhoc", Target: "main", Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", ExpectedReviewers: 2, Roster: []panel.RosterSlot{{Slot: "R1", Model: "model", Lens: "correctness"}, {Slot: "R2", Model: "model", Lens: "tests"}}}); err != nil {
			t.Fatal(err)
		}
		lines, err := panel.ReadLedger(dir)
		if err != nil {
			t.Fatal(err)
		}
		var event panel.RoundRecord
		if err := json.Unmarshal(lines[0], &event); err != nil {
			t.Fatal(err)
		}
		if event.Spec != "adhoc" {
			t.Fatalf("adhoc provenance: %+v", event)
		}
	})
}

func TestPanelDefer_RechecksInsideLock(t *testing.T) {
	root, _ := setupDischarge(t)
	fake := fakeDeferralBD(t)
	rebindWrite(t, fake, "closed.json", `[{"id":"follow-up","status":"closed"}]`)
	// The first show succeeds. The next show sees the follow-up closed.
	script := "#!/bin/sh\ncase \"$1\" in\nlist) cat '" + fake + "/epics.json';;\nshow) if [ -f '" + fake + "/read-once' ]; then cat '" + fake + "/closed.json'; else touch '" + fake + "/read-once'; cat '" + fake + "/follow-up.json'; fi;;\n*) exit 1;;\nesac\n"
	rebindWrite(t, fake, "bd", script)
	if err := os.Chmod(filepath.Join(fake, "bd"), 0700); err != nil {
		t.Fatal(err)
	}
	before := panelTreeBytes(t, root)
	out, err := runPanelVerbCmd(deferArgs()...)
	if err == nil || !strings.Contains(err.Error(), "must exist and be open") {
		t.Fatalf("in-lock closure must refuse: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(fake, "read-once")); err != nil {
		t.Fatal("pre-lock read did not run:", err)
	}
	if !reflect.DeepEqual(before, panelTreeBytes(t, root)) {
		t.Fatal("in-lock refusal wrote panel files")
	}
}

// TestPanelDefer_ApproveSlotMinorFinding: a MINOR item on an APPROVE verdict
// can go to a follow-up bead. The old verb refused it with refute wording.
func TestPanelDefer_ApproveSlotMinorFinding(t *testing.T) {
	_, dir := setupDischarge(t)
	fake := fakeDeferralBD(t)
	writeDischargeVerdict(t, dir, "R2", panel.FindingVerdict{Verdict: panel.VerdictApprove, ConcreteChangesRequired: []string{"minor nit"}})
	rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"minor nit","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
	args := replaceRefuteArg(deferArgs(), "--slot", "R2")
	args = append(args, "--finding", "ccr[0]")
	requirePanelVerb(t, args...)
	lines, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	var event panel.DeferralRecord
	var row panel.DispositionRow
	if len(lines) != 3 {
		t.Fatalf("want round, deferral, disposition: %s", lines)
	}
	if err := json.Unmarshal(lines[1], &event); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lines[2], &row); err != nil {
		t.Fatal(err)
	}
	if event.Slot != "R2" || event.Finding != "ccr[0]" || event.FindingText != "minor nit" || event.Bead != "follow-up" {
		t.Fatalf("bad deferral event: %+v", event)
	}
	if row.Disposition != "confirmed-deferred" || row.Severity != "minor" || row.Model != "codex" {
		t.Fatalf("bad disposition row: %+v", row)
	}
}

// TestPanelDefer_RefusalNamesDefer: a defer refusal names the defer verb,
// not refute.
func TestPanelDefer_RefusalNamesDefer(t *testing.T) {
	root, dir := setupDischarge(t)
	fakeDeferralBD(t)
	writeDischargeVerdict(t, dir, "R1", panel.FindingVerdict{Verdict: panel.VerdictReject, Rationale: "no"})
	msg := assertPanelRefusal(t, root, deferArgs()...)
	if !strings.Contains(msg, "can be deferred") || strings.Contains(msg, "refuted") {
		t.Fatalf("defer refusal must name defer: %s", msg)
	}
}

// TestPanelDefer_RefusalListsEveryMissingRequirement: with no
// discovered-from edge and no finding text in the description, one
// refusal names both and its recovery line gives the exact edge command
// instead of the generic "correct the panel arguments" line.
func TestPanelDefer_RefusalListsEveryMissingRequirement(t *testing.T) {
	root, _ := setupDischarge(t)
	fake := fakeDeferralBD(t)
	rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"other","dependencies":[]}]`)
	msg := assertPanelRefusal(t, root, deferArgs()...)
	for _, want := range []string{"discovered-from edge", "must contain every deferred finding", "missing test", "recovery: bd dep add follow-up epic-x --type discovered-from"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal lacks %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "correct the panel arguments") {
		t.Errorf("refusal must not fall back to the generic recovery:\n%s", msg)
	}
}
