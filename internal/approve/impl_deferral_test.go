package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func implDeferralFixture(t *testing.T, gate string) (string, panel.Registration, string) {
	t.Helper()
	root := core3ApproveRoot(t)
	reg := core3Fixture(t, root, "deferred", gate, false)
	v := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding /tmp/x"}}
	core3JSON(t, reg.Dir, "R1-round-1.json", v)
	findings, err := panel.Findings(v)
	if err != nil {
		t.Fatal(err)
	}
	f := findings[0]
	d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "follow-up", By: "max", CreatedAt: core3Time}
	reg.Panel.Deferrals = []panel.Deferral{d}
	core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	orig := implGetMetadataFn
	t.Cleanup(func() { implGetMetadataFn = orig })
	implGetMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"panel_deferred_entries": []panel.Deferral{d}}, nil
	}
	restore := phase.SetListJSONForTest(func(...string) ([]byte, error) {
		return []byte(`[{"id":"epic-x","metadata":{"spec_num":130,"spec_title":"core3"}}]`), nil
	})
	t.Cleanup(restore)
	skip := implPanelSkipEnvFn
	t.Cleanup(func() { implPanelSkipEnvFn = skip })
	implPanelSkipEnvFn = func() bool { return false }
	fake := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\nshow) cat '" + fake + "/'\"$2\"'.json';;\n*) exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(fake, "bd"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	implDeferralIssue(t, fake, "open")
	return root, reg, fake
}
func implDeferralIssue(t *testing.T, fake, change string) {
	t.Helper()
	issue := map[string]interface{}{"id": "follow-up", "status": "open", "description": "test finding /tmp/x", "dependencies": []map[string]string{{"id": "epic-x", "dependency_type": "discovered-from"}}}
	switch change {
	case "closed":
		issue["status"] = "closed"
	case "edge removed":
		issue["dependencies"] = []interface{}{}
	case "text removed":
		issue["description"] = "other"
	case "reparented":
		issue["parent"] = "child-x"
		core3JSON(t, fake, "child-x.json", []interface{}{map[string]interface{}{"id": "child-x", "parent": "epic-x"}})
		core3JSON(t, fake, "epic-x.json", []interface{}{map[string]interface{}{"id": "epic-x", "status": "open"}})
	case "deleted":
		core3JSON(t, fake, "follow-up.json", []interface{}{})
		return
	case "bd error":
		if err := os.WriteFile(filepath.Join(fake, "follow-up.json"), []byte("bad json"), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	core3JSON(t, fake, "follow-up.json", []interface{}{issue})
}
func TestGateTime_DeferralRecheck(t *testing.T) {
	for _, gate := range []string{"bead", "final_review", "plan_approve"} {
		for _, change := range []string{"reparented", "deleted", "edge removed", "text removed", "bd error"} {
			t.Run(gate+"/"+change, func(t *testing.T) {
				root, _, fake := implDeferralFixture(t, gate)
				if err := implPanelDeferralPreflight(root, "130-core3"); err != nil {
					t.Fatalf("open baseline: %v", err)
				}
				implDeferralIssue(t, fake, change)
				err := implPanelDeferralPreflight(root, "130-core3")
				if change == "reparented" && (err == nil || !strings.Contains(err.Error(), "outside the owning epic")) {
					t.Fatalf("reparented deferral must reach epic comparison: %v", err)
				}
				for _, want := range []string{"R1", "ccr[0]", "follow-up"} {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("changed deferral did not block: %v", err)
					}
				}
			})
		}
	}
}
func TestGateTime_DeferralClosedBead(t *testing.T) {
	for _, gate := range []string{"bead", "final_review"} {
		t.Run(gate, func(t *testing.T) {
			root, _, fake := implDeferralFixture(t, gate)
			implDeferralIssue(t, fake, "closed")
			err := implPanelDeferralPreflight(root, "130-core3")
			if err == nil || !strings.Contains(err.Error(), "bd reopen 'follow-up'") {
				t.Fatalf("closed deferral allowed: %v", err)
			}
			if strings.Contains(err.Error(), "mindspec panel refute") != (gate != "bead") {
				t.Fatalf("wrong audit recovery: %v", err)
			}
		})
	}
}
func implRefuteDeferred(t *testing.T, reg *panel.Registration, kind string) {
	t.Helper()
	d := reg.Panel.Deferrals[0]
	ref := panel.Refutation{Slot: d.Slot, Round: d.Round, Finding: d.Finding, FindingSHA256: d.FindingSHA256, FindingText: d.FindingText, Ground: "disproven", Reason: "fixed", Evidence: "abc123", By: "max", CreatedAt: core3Time}
	switch kind {
	case "other finding":
		ref.Finding = "ccr[9]"
	case "different text":
		ref.FindingSHA256 = strings.Repeat("a", 64)
	case "slot":
		v, _ := panel.ReadFindingVerdict(reg.Dir, "R1", 1)
		f, _ := panel.SlotFinding(v)
		ref.Finding = f.ID
		ref.FindingSHA256 = f.SHA256
		ref.FindingText = f.Text
	}
	reg.Panel.Refutations = []panel.Refutation{ref}
	core3Append(t, reg.Dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: reg.Panel.Spec, Panel: reg.Slug(), Refutation: ref})
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
}
func TestGateTime_RefuteSupersedesDeferral(t *testing.T) {
	for _, gate := range []string{"bead", "final_review"} {
		for _, kind := range []string{"same", "other finding", "different text", "slot"} {
			t.Run(gate+"/"+kind, func(t *testing.T) {
				root, reg, fake := implDeferralFixture(t, gate)
				implDeferralIssue(t, fake, "closed")
				implRefuteDeferred(t, &reg, kind)
				err := implPanelDeferralPreflight(root, "130-core3")
				wantAllowed := gate != "bead" && (kind == "same" || kind == "slot")
				if (err == nil) != wantAllowed {
					t.Fatalf("audit/current supersession %s %s: %v", gate, kind, err)
				}
			})
		}
	}
}
func TestGateTime_SlotLevelDeferralRecovery(t *testing.T) {
	root, reg, fake := implDeferralFixture(t, "final_review")
	v, _ := panel.ReadFindingVerdict(reg.Dir, "R1", 1)
	f, _ := panel.SlotFinding(v)
	d := reg.Panel.Deferrals[0]
	d.Finding = f.ID
	d.FindingSHA256 = f.SHA256
	d.FindingText = f.Text
	reg.Panel.Deferrals = []panel.Deferral{d}
	core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	implDeferralIssue(t, fake, "closed")
	err := implPanelDeferralPreflight(root, "130-core3")
	if err == nil || strings.Contains(err.Error(), "--finding") || !strings.Contains(err.Error(), "mindspec panel refute") {
		t.Fatalf("slot recovery: %v", err)
	}
	implRefuteDeferred(t, &reg, "slot")
	if err := implPanelDeferralPreflight(root, "130-core3"); err != nil {
		t.Fatal(err)
	}
}
func TestGateTime_DeferralAbandonedSkipped(t *testing.T) {
	root, reg, fake := implDeferralFixture(t, "bead")
	implDeferralIssue(t, fake, "closed")
	reg.Panel.Abandoned = true
	reg.Panel.AbandonReason = "rework"
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	if err := implPanelDeferralPreflight(root, "130-core3"); err == nil || !strings.Contains(err.Error(), "outside the panel verbs") {
		t.Fatalf("integrity must precede abandonment: %v", err)
	}
	core3Append(t, reg.Dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: reg.Panel.Spec, Panel: reg.Slug(), Reason: "rework", By: "max", CreatedAt: core3Time})
	if err := implPanelDeferralPreflight(root, "130-core3"); err != nil {
		t.Fatal(err)
	}
}
func TestGateTime_DeferralRedactedText(t *testing.T) {
	for _, gate := range []string{"bead", "final_review"} {
		t.Run(gate, func(t *testing.T) {
			root, _, _ := implDeferralFixture(t, gate)
			if err := implPanelDeferralPreflight(root, "130-core3"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestGateTime_DeferralHatches(t *testing.T) {
	for _, hatch := range []string{"env", "config"} {
		t.Run(hatch, func(t *testing.T) {
			root, _, fake := implDeferralFixture(t, "bead")
			implDeferralIssue(t, fake, "closed")
			if hatch == "env" {
				implPanelSkipEnvFn = func() bool { return true }
			} else {
				if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_gate: false\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := implPanelDeferralPreflight(root, "130-core3"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestGateTime_DeferralCorruptAudit(t *testing.T) {
	root, _, _ := implDeferralFixture(t, "bead")
	for _, raw := range []interface{}{nil, "bad", []interface{}{map[string]interface{}{"slot": "R1"}}} {
		implGetMetadataFn = func(string) (map[string]interface{}, error) {
			return map[string]interface{}{"panel_deferred_entries": raw}, nil
		}
		if err := implPanelDeferralPreflight(root, "130-core3"); err == nil {
			b, _ := json.Marshal(raw)
			t.Fatalf("corrupt audit allowed: %s", b)
		}
	}
}

func TestGateTime_DeferralPreflightBeforeMutation(t *testing.T) {
	for _, gate := range []string{"bead", "final_review"} {
		t.Run(gate, func(t *testing.T) {
			root := core3ApproveRoot(t)
			writeSpecDir(t, root, "010-test")
			writePlanWithBeads(t, root, "010-test", []string{"bead-1"})
			saveAndRestore(t)
			skip := implPanelSkipEnvFn
			t.Cleanup(func() { implPanelSkipEnvFn = skip })
			implPanelSkipEnvFn = func() bool { return false }
			reg := core3Fixture(t, root, "deferred", gate, false, "010-test")
			v := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding"}}
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			findings, _ := panel.Findings(v)
			f := findings[0]
			d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "follow-up", By: "max", CreatedAt: core3Time}
			reg.Panel.Deferrals = []panel.Deferral{d}
			core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			core3Cover(t, reg)
			core3Manifest(t, reg)
			implGetMetadataFn = func(string) (map[string]interface{}, error) {
				return map[string]interface{}{"panel_deferred_entries": []panel.Deferral{d}}, nil
			}
			fake := t.TempDir()
			if err := os.WriteFile(filepath.Join(fake, "bd"), []byte("#!/bin/sh\ncat '"+filepath.Join(fake, "issue.json")+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
			core3JSON(t, fake, "issue.json", []interface{}{map[string]interface{}{"id": "follow-up", "status": "open", "description": "test finding", "dependencies": []map[string]string{{"id": "epic-parent", "dependency_type": "discovered-from"}}}})
			if err := implPanelDeferralPreflight(root, "010-test"); err != nil {
				t.Fatalf("open baseline: %v", err)
			}
			// The edge disappears after completion recorded the durable bead audit.
			core3JSON(t, fake, "issue.json", []interface{}{map[string]interface{}{"id": "follow-up", "status": "open", "description": "test finding"}})
			mock := &executor.MockExecutor{CommitCountResult: 5, FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5}}
			mutated := false
			implRunBDCombinedFn = func(...string) ([]byte, error) { mutated = true; return []byte("ok"), nil }
			_, err := ApproveImpl(root, "010-test", mock)
			for _, want := range []string{"R1", "ccr[0]", "follow-up"} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("ApproveImpl must block the changed deferral: %v", err)
				}
			}
			if mutated || len(mock.CallsTo("FinalizeEpic")) != 0 {
				t.Fatal("approval mutated before the deferral refusal")
			}
		})
	}
}

func TestImplPreflight_BeadAuditPanelScope(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", binary, "./cmd/mindspec")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build abandonment writer: %v %s", err, out)
	}
	for _, identity := range []string{"current", "legacy"} {
		for _, abandoned := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/abandoned=%t", identity, abandoned), func(t *testing.T) {
				root, reg, fake := implDeferralFixture(t, "bead")
				entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0]}
				if identity == "current" {
					entry.Panel, entry.Spec = reg.Slug(), reg.Panel.Spec
				}
				implGetMetadataFn = func(string) (map[string]interface{}, error) {
					return map[string]interface{}{"panel_deferred_entries": []complete.PanelDeferredEntry{entry}}, nil
				}
				if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err != nil {
					t.Fatalf("open baseline: %v", err)
				}
				if abandoned {
					cmd := exec.Command(binary, "panel", "abandon", reg.Slug(), "--spec", reg.Panel.Spec, "--reason", "rework", "--by", "max")
					cmd.Dir = root
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("abandon panel: %v %s", err, out)
					}
				}
				core3Fixture(t, root, "second-panel", "bead", false)
				implDeferralIssue(t, fake, "closed")
				err := implPanelDeferralPreflight(root, reg.Panel.Spec)
				if abandoned && identity == "current" {
					if err != nil {
						t.Fatalf("another panel revived the abandoned panel's audit: %v", err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "bd reopen") {
					t.Fatalf("active or legacy deferral must still block: %v", err)
				}
			})
		}
	}
}
