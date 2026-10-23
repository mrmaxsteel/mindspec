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
	writePlanWithBeads(t, root, reg.Panel.Spec, []string{"mindspec-core3.1"})
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
		if gate != "bead" {
			return map[string]interface{}{}, nil
		}
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
	for _, identity := range []string{"current", "legacy", "half", "other spec"} {
		for _, abandoned := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/abandoned=%t", identity, abandoned), func(t *testing.T) {
				root, reg, fake := implDeferralFixture(t, "bead")
				entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0]}
				if identity == "current" {
					entry.Panel, entry.Spec = reg.Slug(), reg.Panel.Spec
				}
				if identity == "half" {
					entry.Spec = reg.Panel.Spec
				}
				if identity == "other spec" {
					entry.Panel, entry.Spec = reg.Slug(), "131-other"
					restore := phase.SetListJSONForTest(func(...string) ([]byte, error) {
						return []byte(`[{"id":"epic-x","metadata":{"spec_num":130,"spec_title":"core3"}},{"id":"epic-other","metadata":{"spec_num":131,"spec_title":"other"}}]`), nil
					})
					t.Cleanup(restore)
				}
				reads := 0
				implGetMetadataFn = func(string) (map[string]interface{}, error) {
					reads++
					return map[string]interface{}{"panel_deferred_entries": []complete.PanelDeferredEntry{entry}}, nil
				}
				setIssue := func(status string) {
					implDeferralIssue(t, fake, status)
					if identity == "other spec" {
						core3JSON(t, fake, "follow-up.json", []interface{}{map[string]interface{}{
							"id": "follow-up", "status": status, "description": "test finding /tmp/x",
							"dependencies": []map[string]string{
								{"id": "epic-x", "dependency_type": "discovered-from"},
								{"id": "epic-other", "dependency_type": "discovered-from"},
							},
						}})
					}
				}
				setIssue("open")
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
				setIssue("closed")
				reads = 0
				err := implPanelDeferralPreflight(root, reg.Panel.Spec)
				if reads != 1 {
					t.Errorf("read bead audit %d times; want once", reads)
				}
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

func TestImplPreflight_BeadAuditSurvivesMissingRegistration(t *testing.T) {
	for _, change := range []string{"renamed", "panel.json absent"} {
		t.Run(change, func(t *testing.T) {
			root, reg, fake := implDeferralFixture(t, "bead")
			entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0], Spec: reg.Panel.Spec, Panel: reg.Slug()}
			implGetMetadataFn = func(string) (map[string]interface{}, error) {
				return map[string]interface{}{"panel_deferred_entries": []complete.PanelDeferredEntry{entry}}, nil
			}
			core3Cover(t, reg)
			core3Manifest(t, reg)
			implDeferralIssue(t, fake, "closed")
			if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err != nil {
				t.Fatalf("coverage baseline: %v", err)
			}
			if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err == nil || !strings.Contains(err.Error(), "bd reopen") {
				t.Fatalf("closed baseline must block: %v", err)
			}
			if change == "renamed" {
				renamed := filepath.Join(filepath.Dir(reg.Dir), "deferred-renamed")
				if err := os.Rename(reg.Dir, renamed); err != nil {
					t.Fatal(err)
				}
				res, err := panel.Tally(renamed)
				if err != nil {
					t.Fatal(err)
				}
				if err := panel.WriteCoverageManifest(res); err != nil {
					t.Fatal(err)
				}
			} else {
				second := core3Fixture(t, root, "second-panel", "bead", false)
				core3Manifest(t, second)
				if err := os.Rename(filepath.Join(reg.Dir, panel.FileName), filepath.Join(t.TempDir(), panel.FileName)); err != nil {
					t.Fatal(err)
				}
			}
			if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err != nil {
				t.Fatalf("coverage must pass after tally recovery: %v", err)
			}
			if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err == nil || !strings.Contains(err.Error(), "bd reopen") || !strings.Contains(err.Error(), "panel "+entry.Panel+" slot") {
				t.Fatalf("audit must block and name its stored panel: %v", err)
			}
		})
	}
}

func TestImplPreflight_PlanBeadAuditWithoutRegistration(t *testing.T) {
	for _, missing := range []string{"panel.json", "directory"} {
		for _, identity := range []string{"identified", "legacy"} {
			for _, status := range []string{"closed", "open"} {
				t.Run(missing+"/"+identity+"/"+status, func(t *testing.T) {
					root, reg, fake := implDeferralFixture(t, "bead")
					entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0], Spec: reg.Panel.Spec, Panel: reg.Slug()}
					if identity == "legacy" {
						entry.Spec, entry.Panel = "", ""
					}
					reads := 0
					implGetMetadataFn = func(id string) (map[string]interface{}, error) {
						if id != *reg.Panel.BeadID {
							t.Fatalf("unexpected bead %q", id)
						}
						reads++
						return map[string]interface{}{"panel_deferred_entries": []complete.PanelDeferredEntry{entry}}, nil
					}
					implDeferralIssue(t, fake, status)
					source := reg.Dir
					if missing == "panel.json" {
						source = filepath.Join(reg.Dir, panel.FileName)
					}
					if err := os.Rename(source, filepath.Join(t.TempDir(), "saved")); err != nil {
						t.Fatal(err)
					}
					if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err != nil {
						t.Fatalf("coverage: %v", err)
					}
					err := implPanelDeferralPreflight(root, reg.Panel.Spec)
					if reads != 1 {
						t.Errorf("audit reads=%d; want 1", reads)
					}
					if status == "open" {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), "bd reopen 'follow-up'") {
						t.Fatalf("closed follow-up must block: %v", err)
					}
					if identity == "legacy" {
						want := "bead " + *reg.Panel.BeadID + " slot R1 finding ccr[0] follow-up follow-up: follow-up bead must exist and be open\nrecovery: bd reopen 'follow-up'"
						if err.Error() != want {
							t.Fatalf("legacy refusal = %q; want %q", err.Error(), want)
						}
					}
					if identity == "identified" && !strings.Contains(err.Error(), "panel "+entry.Panel+" slot") {
						t.Fatalf("stored panel missing: %v", err)
					}
				})
			}
		}
	}
}

func TestImplPreflight_PlanBeadReadFailure(t *testing.T) {
	root, reg, _ := implDeferralFixture(t, "bead")
	writePlanWithBeads(t, root, reg.Panel.Spec, []string{"bad/id"})
	err := implPanelDeferralPreflight(root, reg.Panel.Spec)
	if err == nil {
		t.Fatal("unreadable plan bead list must block")
	}
}

func TestImplPreflight_EmptySpecMatchesAbandonedRegistration(t *testing.T) {
	root, reg, fake := implDeferralFixture(t, "bead")
	entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0], Panel: reg.Slug()}
	implGetMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"panel_deferred_entries": []complete.PanelDeferredEntry{entry}}, nil
	}
	implDeferralIssue(t, fake, "closed")
	reg.Panel.Abandoned = true
	reg.Panel.AbandonReason = "rework"
	core3Append(t, reg.Dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: reg.Panel.Spec, Panel: reg.Slug(), Reason: "rework", By: "max", CreatedAt: core3Time})
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err != nil {
		t.Fatalf("empty spec must use current spec: %v", err)
	}
}

func TestImplPreflight_ClosedBeadAuditRecovery(t *testing.T) {
	for _, failure := range []string{"read", "decode"} {
		t.Run(failure, func(t *testing.T) {
			root, reg, _ := implDeferralFixture(t, "bead")
			implGetMetadataFn = func(string) (map[string]interface{}, error) {
				if failure == "read" {
					return nil, fmt.Errorf("audit unavailable")
				}
				return map[string]interface{}{"panel_deferred_entries": "bad"}, nil
			}
			err := implPanelDeferralPreflight(root, reg.Panel.Spec)
			want := "recovery: bd reopen " + *reg.Panel.BeadID
			if err == nil || !strings.HasSuffix(err.Error(), want) || strings.Contains(err.Error(), "mindspec complete") {
				t.Fatalf("closed bead recovery: %v; want %q", err, want)
			}
		})
	}
}

func TestImplPreflight_FinalReviewRefutationLeavesOtherFinding(t *testing.T) {
	root, reg, fake := implDeferralFixture(t, "final_review")
	v := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding /tmp/x", "second ask"}}
	core3JSON(t, reg.Dir, "R1-round-1.json", v)
	f, err := panel.SlotFinding(v)
	if err != nil {
		t.Fatal(err)
	}
	d := reg.Panel.Deferrals[0]
	d.Finding, d.FindingSHA256, d.FindingText = f.ID, f.SHA256, f.Text
	reg.Panel.Deferrals = []panel.Deferral{d}
	core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	// Refute only the first member. The slot deferral still covers the second.
	fs, err := panel.Findings(v)
	if err != nil {
		t.Fatal(err)
	}
	ref := panel.Refutation{Slot: "R1", Round: 1, Finding: fs[0].ID, FindingSHA256: fs[0].SHA256, FindingText: fs[0].Text, Ground: "disproven", Reason: "fixed", Evidence: "abc123", By: "max", CreatedAt: core3Time}
	core3Append(t, reg.Dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: reg.Panel.Spec, Panel: reg.Slug(), Refutation: ref})
	reg.Panel.Refutations = []panel.Refutation{ref}
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	implDeferralIssue(t, fake, "closed")
	if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err == nil || !strings.Contains(err.Error(), "bd reopen") {
		t.Fatalf("remaining finding must block: %v", err)
	}
	implDeferralIssue(t, fake, "open") // Description has only the refuted finding.
	if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err == nil {
		t.Fatal("missing unrefuted text must block")
	}
	core3JSON(t, fake, "follow-up.json", []interface{}{map[string]interface{}{"id": "follow-up", "status": "open", "description": "second ask", "dependencies": []map[string]string{{"id": "epic-x", "dependency_type": "discovered-from"}}}})
	if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err != nil {
		t.Fatalf("only effective member is required: %v", err)
	}
}

func TestImplPreflight_AuditRegistrationLedgerIdentity(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", binary, "./cmd/mindspec")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build panel writer: %v %s", err, out)
	}
	for _, swap := range []bool{false, true} {
		t.Run(fmt.Sprintf("swap=%t", swap), func(t *testing.T) {
			root, reg, fake := implDeferralFixture(t, "bead")
			entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0], Spec: reg.Panel.Spec, Panel: reg.Slug()}
			implGetMetadataFn = func(string) (map[string]interface{}, error) {
				return map[string]interface{}{"panel_deferred_entries": []complete.PanelDeferredEntry{entry}}, nil
			}
			core3Cover(t, reg)
			core3Manifest(t, reg)
			implDeferralIssue(t, fake, "closed")
			if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err == nil || !strings.Contains(err.Error(), "bd reopen") {
				t.Fatalf("live panel must block: %v", err)
			}
			abandoned := reg
			if swap {
				abandoned = core3Fixture(t, root, "throwaway", "bead", false)
			}
			cmd := exec.Command(binary, "panel", "abandon", abandoned.Slug(), "--spec", reg.Panel.Spec, "--reason", "rework", "--by", "max")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("abandon: %v %s", err, out)
			}
			if swap {
				old := reg.Dir + "-old"
				if err := os.Rename(reg.Dir, old); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(abandoned.Dir, reg.Dir); err != nil {
					t.Fatal(err)
				}
				res, err := panel.Tally(old)
				if err != nil {
					t.Fatal(err)
				}
				if err := panel.WriteCoverageManifest(res); err != nil {
					t.Fatal(err)
				}
			}
			if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err != nil {
				t.Fatalf("coverage after tally: %v", err)
			}
			err := implPanelDeferralPreflight(root, reg.Panel.Spec)
			if swap {
				if err == nil || !strings.HasSuffix(err.Error(), "recovery: bd reopen 'follow-up'") {
					t.Fatalf("renamed abandonment must not skip the audit: %v", err)
				}
			} else if err != nil {
				t.Fatalf("own verb-backed abandonment must skip the audit: %v", err)
			}
		})
	}
}

func TestImplPreflight_PendingDeferralAudit(t *testing.T) {
	for _, status := range []string{"closed", "open"} {
		t.Run(status, func(t *testing.T) {
			root, reg, fake := implDeferralFixture(t, "bead")
			entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0], Spec: reg.Panel.Spec, Panel: reg.Slug()}
			implGetMetadataFn = func(string) (map[string]interface{}, error) {
				return map[string]interface{}{"deferral_pending_entries": []complete.PanelDeferredEntry{entry}}, nil
			}
			implDeferralIssue(t, fake, status)
			err := implPanelDeferralPreflight(root, reg.Panel.Spec)
			if status == "closed" {
				if err == nil || !strings.HasSuffix(err.Error(), "recovery: bd reopen 'follow-up'") {
					t.Fatalf("pending deferral must block: %v", err)
				}
			} else if err != nil {
				t.Fatalf("open pending deferral must pass: %v", err)
			}
		})
	}
}

func TestImplPreflight_RejectsDuplicatePanelIdentity(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", binary, "./cmd/mindspec")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build panel writer: %v %s", err, out)
	}
	for _, layout := range []string{"legacy", "flat"} {
		for _, segment := range []string{"review", "reviews"} {
			t.Run(layout+"/"+segment, func(t *testing.T) {
				root, reg, fake := implDeferralFixture(t, "bead")
				entry := complete.PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0], Spec: reg.Panel.Spec, Panel: reg.Slug()}
				implGetMetadataFn = func(string) (map[string]interface{}, error) {
					return map[string]interface{}{"panel_deferred_entries": []complete.PanelDeferredEntry{entry}}, nil
				}
				core3Cover(t, reg)
				core3Manifest(t, reg)
				implDeferralIssue(t, fake, "closed")
				specDir := filepath.Join(root, "docs", "specs", reg.Panel.Spec)
				if layout == "flat" {
					flatDir := filepath.Join(root, ".mindspec", "specs", reg.Panel.Spec)
					if err := os.MkdirAll(flatDir, 0700); err != nil {
						t.Fatal(err)
					}
					plan, err := os.ReadFile(filepath.Join(specDir, "plan.md"))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(flatDir, "plan.md"), plan, 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(filepath.Join(root, "docs"), filepath.Join(t.TempDir(), "docs")); err != nil {
						t.Fatal(err)
					}
					liveSegment := "reviews"
					if segment == "reviews" {
						liveSegment = "review"
					}
					live := filepath.Join(flatDir, liveSegment, reg.Slug())
					if err := os.MkdirAll(filepath.Dir(live), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(reg.Dir, live); err != nil {
						t.Fatal(err)
					}
					reg.Dir, specDir = live, flatDir
				}
				if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err == nil || !strings.HasSuffix(err.Error(), "recovery: bd reopen 'follow-up'") {
					t.Fatalf("closed follow-up must block before the copy: %v", err)
				}
				live := reg.Dir
				aside := filepath.Join(t.TempDir(), "live")
				if err := os.Rename(live, aside); err != nil {
					t.Fatal(err)
				}
				if err := os.CopyFS(live, os.DirFS(aside)); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(binary, "panel", "abandon", reg.Slug(), "--spec", reg.Panel.Spec, "--reason", "rework", "--by", "max")
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("abandon byte copy: %v %s", err, out)
				}
				duplicate := filepath.Join(specDir, segment, reg.Slug())
				if err := os.MkdirAll(filepath.Dir(duplicate), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(live, duplicate); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(aside, live); err != nil {
					t.Fatal(err)
				}
				regs := panel.Scan(complete.PanelGateRoots(root, "", reg.Panel.Spec)...)
				if len(regs) != 2 {
					t.Fatalf("want live and abandoned registrations: %+v", regs)
				}
				for _, current := range regs {
					field, err := panel.CheckHandEdits(current.Dir, current.Panel)
					if err != nil || field != "" || !complete.PanelDeferredRegistrationMatches(current, reg.Panel.Spec, reg.Slug()) {
						t.Fatalf("copy must have intact ledger identity: %+v %q %v", current, field, err)
					}
				}
				if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err != nil {
					t.Fatalf("registration integrity must pass: %v", err)
				}
				err := implPanelDeferralPreflight(root, reg.Panel.Spec)
				for _, want := range []string{"panel " + reg.Slug() + " for spec " + reg.Panel.Spec + " is registered at 2 paths:", live, duplicate, "remove the duplicates"} {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Errorf("duplicate identity must block and name both paths; want %q: %v", want, err)
					}
				}
				// A single verb-backed abandonment still skips the closed follow-up.
				if err := os.Rename(live, filepath.Join(t.TempDir(), "live")); err != nil {
					t.Fatal(err)
				}
				if err := implPanelDeferralPreflight(root, reg.Panel.Spec); err != nil {
					t.Fatalf("single abandoned registration must skip: %v", err)
				}
			})
		}
	}
}

func TestImplApprove_SourceBoundDeferralCurrency(t *testing.T) {
	for _, source := range []string{panel.VerdictApprove, panel.VerdictRequestChanges} {
		t.Run(source, func(t *testing.T) {
			t.Setenv(panel.SkipPanelEnv, "0")
			root := core3ApproveRoot(t)
			reg := core3Fixture(t, root, "bound-deferral", "final_review", false)
			v := panel.FindingVerdict{Verdict: panel.VerdictRequestChanges, ConcreteChangesRequired: []string{"same finding"}}
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			res, err := panel.Tally(reg.Dir)
			if err != nil {
				t.Fatal(err)
			}
			f := res.Verdicts[0].Findings[0]
			d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "follow-up", By: "operator", CreatedAt: core3Time, SourceVerdict: source, SourceFileSHA256: res.Verdicts[0].FileSHA256}
			reg.Panel.Deferrals = []panel.Deferral{d}
			core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			round := 1
			row := panel.DispositionRow{Record: panel.RecordDisposition, ID: "bound-row", Spec: reg.Panel.Spec, Gate: reg.Panel.Gate, Panel: reg.Slug(), Reviewer: "R1", Model: "model-a", Severity: "blocking", Summary: f.Text, ConvergentWith: []string{}, Disposition: "confirmed-deferred", EvidenceRef: d.Bead, CreatedAt: d.CreatedAt, Round: &round}
			core3Append(t, reg.Dir, row, panel.DeriveCoverageManifest(res))
			if source == panel.VerdictRequestChanges {
				if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err != nil {
					t.Fatalf("current bound deferral must pass: %v", err)
				}
			}
			// Same finding and terminal verdict, but different submission bytes.
			v.Rationale = "resubmitted"
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err == nil || !strings.Contains(err.Error(), "R1") {
				t.Fatalf("impl approve accepted ineffective %s deferral: %v", source, err)
			}
		})
	}
}
