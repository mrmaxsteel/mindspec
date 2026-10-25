package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

type s133ApprovalExecutor struct {
	*executor.MindspecExecutor
	root string
}

func (e s133ApprovalExecutor) CommitAll(_, message string) error {
	return e.MindspecExecutor.CommitAll(e.root, message)
}
func (e s133ApprovalExecutor) IsTreeClean(string) error {
	return e.MindspecExecutor.IsTreeClean(e.root)
}

func TestS133DocGate_ApproveThenVerify(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, findings := range []bool{false, true} {
			name := policy + "/pass"
			if findings {
				name = policy + "/defer"
			}
			t.Run(name, func(t *testing.T) {
				for k, v := range map[string]string{"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null", "GIT_TERMINAL_PROMPT": "0", "GIT_ASKPASS": "/usr/bin/false", panel.SkipPanelEnv: ""} {
					t.Setenv(k, v)
				}
				resetPanelCreateFlags(t)
				resetV2Flags(t)
				resetDischargeFlags(t)
				stubWorktreeListEmpty(t)
				lsTreeForPanelFn = func(workdir, ref string) (string, error) {
					return executor.NewMindspecExecutor(workdir).LsTree(workdir, ref)
				}
				root := mkFlatPanelTestRoot(t)
				withTestChdir(t, root)
				stubApproveBeadsForVerbTest(t)
				var audit map[string]interface{}
				t.Cleanup(approve.SetLifecycleMergeMetadataForTest(func(_ string, m map[string]interface{}) error { audit = m; return nil }))
				bin := t.TempDir()
				if err := os.WriteFile(filepath.Join(bin, "bd"), []byte("#!/bin/sh\n[ \"$1\" = export ]\n"), 0755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
				rebindGit(t, root, "init", "-q", "-b", "spec/900-example")
				rebindGit(t, root, "config", "user.email", "test@example.invalid")
				rebindGit(t, root, "config", "user.name", "Test")
				rebindGit(t, root, "config", "commit.gpgsign", "false")
				rebindWrite(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
				text := strings.Replace(validVerbFixtureSpecMD("900-example"), "spec_id: 900-example\nstatus: Draft\nversion: 1", "version: '1'\nstatus: Draft\nspec_id: '900-example'", 1)
				text = strings.Replace(text, "Pending.\n", "- **Status**: DRAFT\n- **Approved By**: -\n- **Approval Date**: -\n- **Notes**: -\n", 1)
				rebindWrite(t, root, ".mindspec/specs/900-example/spec.md", text)
				rebindWrite(t, root, ".gitignore", ".mindspec/specs/*/reviews/\n")
				rebindGit(t, root, "add", "-A")
				rebindGit(t, root, "commit", "-qm", "base")
				config.ResetCache()
				t.Cleanup(config.ResetCache)
				if out, err := runPanelCmd("create", "review", "--spec", "900-example", "--gate", "spec_approve", "--target", "spec/900-example", "--reviewers", "2"); err != nil {
					t.Fatalf("create: %v %s", err, out)
				}
				reg, err := findPanelRegistration(root, "review")
				if err != nil {
					t.Fatal(err)
				}
				for _, slot := range reg.Panel.Roster {
					v := paneltest.Verdict(panel.VerdictApprove)
					v.Slot, v.Model, v.ReviewedSHA = slot.Slot, slot.Model, reg.Panel.ReviewedHeadSHA
					if findings && slot.Slot == "R1" {
						v.Verdict = panel.VerdictRequestChanges
						v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "add evidence", Basis: "traced", ArtifactPath: "evidence/missing.txt"}, {Text: "add a plan test", Basis: "traced"}}
					}
					if _, err := panel.Submit(panel.SubmitTarget{Dir: reg.Dir, Slot: slot.Slot, Round: 1}, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
						t.Fatal(err)
					}
				}
				res, err := panel.Tally(reg.Dir)
				if err != nil {
					t.Fatal(err)
				}
				if err := panel.WriteCoverageManifest(res); err != nil {
					t.Fatal(err)
				}
				real := s133ApprovalExecutor{MindspecExecutor: executor.NewMindspecExecutor(root), root: root}
				if findings {
					if _, err := approve.ApproveSpec(root, "900-example", "operator", real); err == nil || !strings.Contains(err.Error(), "evidence/missing.txt") {
						t.Fatalf("artifact refusal: %v", err)
					}
					if out, err := runPanelVerbCmd("defer", "review", "--slot", "R1", "--round", "1", "--to-plan", "--by", "operator"); err != nil {
						t.Fatalf("defer: %v %s", err, out)
					}
					if out, err := runPanelVerbCmd("tally", "review"); err != nil {
						t.Fatalf("tally: %v %s", err, out)
					}
					if out, err := runPanelVerbCmd("disposition", "check", "--spec", "900-example", "--panel", "review"); err != nil {
						t.Fatalf("coverage: %v %s", err, out)
					}
				}
				if _, err := approve.ApproveSpec(root, "900-example", "operator", real); err != nil {
					t.Fatal(err)
				}
				if out, err := runPanelVerbCmd("verify", "review"); err != nil || !strings.Contains(out, "\nPASS\n") {
					t.Fatalf("verify after approval rewrite: %v %s", err, out)
				}
				if _, err := approve.ApproveSpec(root, "900-example", "operator", real); err != nil {
					t.Fatalf("gate after rewrite: %v", err)
				}
				if findings {
					ids, ok := audit["plan_deferral_ids"].([]string)
					if !ok || len(ids) != 2 {
						t.Fatalf("missing accepted ids: %v", audit)
					}
					res, err := panel.Tally(reg.Dir)
					if err != nil {
						t.Fatal(err)
					}
					forged := res.Panel.PlanDeferrals[0]
					forged.ID = "plan-forged"
					res.Panel.PlanDeferrals = append(res.Panel.PlanDeferrals, forged)
					if err := panel.WriteSnapshot(reg.Dir, *res.Panel); err != nil {
						t.Fatal(err)
					}
					if _, err := approve.ApproveSpec(root, "900-example", "operator", real); err == nil || !strings.Contains(err.Error(), "integrity") {
						t.Fatalf("unbacked deferral passed: %v", err)
					}
				}
			})
		}
	}
}
