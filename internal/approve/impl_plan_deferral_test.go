package approve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestS133DeferToPlan_DeferralPreflightIgnoresPlanEntries(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			root := core3ApproveRoot(t)
			writeSpecDir(t, root, "010-test")
			writePlanWithBeads(t, root, "010-test", []string{"bead-1"})
			saveAndRestore(t)
			oldSkip := implPanelSkipEnvFn
			t.Cleanup(func() { implPanelSkipEnvFn = oldSkip })
			implPanelSkipEnvFn = func() bool { return false }
			if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_presence: "+policy+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			config.ResetCache()
			t.Cleanup(config.ResetCache)
			// Keep the final-review target and head current while this fixture
			// isolates plan entries from bead-deferral checks.
			final := core3Fixture(t, root, "final", "final_review", false, "010-test")
			core3Manifest(t, final)
			oldExecutor := lifecycleExecutorFn
			t.Cleanup(func() { lifecycleExecutorFn = oldExecutor })
			lifecycleExecutorFn = func(string) executor.Executor {
				return &executor.MockExecutor{
					RevParseRefResult: final.Panel.ReviewedHeadSHA,
					LsTreeResult:      "100644 blob 0000000000000000000000000000000000000000\tfixture.txt\n",
				}
			}
			reg := core3Fixture(t, root, "deferred", "spec_approve", false, "010-test")
			v := panel.FindingVerdict{Verdict: panel.VerdictRequestChanges, ConcreteChangesRequired: []string{"plan the test"}}
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			fs, err := panel.Findings(v)
			if err != nil {
				t.Fatal(err)
			}
			f := fs[0]
			d := panel.PlanDeferral{ID: panel.PlanDeferralID(reg.Panel.Spec, reg.Slug(), "R1", 1, f), Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, By: "operator", CreatedAt: core3Time}
			reg.Panel.PlanDeferrals = []panel.PlanDeferral{d}
			core3Append(t, reg.Dir, panel.PlanDeferralRecord{Record: panel.RecordDeferredToPlan, Spec: reg.Panel.Spec, Panel: reg.Slug(), PlanDeferral: d},
				panel.DispositionRow{Record: panel.RecordDisposition, ID: d.ID + ":disposition", Spec: reg.Panel.Spec, Gate: reg.Panel.Gate, Panel: reg.Slug(), Reviewer: "R1", Model: "model-a", Severity: "blocking", Summary: f.Text, ConvergentWith: []string{}, Disposition: "confirmed-deferred", EvidenceRef: "plan:" + d.ID, CreatedAt: core3Time})
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			core3Manifest(t, reg)
			if err := implPanelDeferralPreflight(root, "010-test"); err != nil {
				t.Fatalf("plan entry entered bead checks: %v", err)
			}
			mock := &executor.MockExecutor{CommitCountResult: 5, FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5}}
			if _, err := ApproveImpl(root, "010-test", mock); err != nil {
				t.Fatalf("plan deferral refused approval: %v", err)
			}
			if len(mock.CallsTo("FinalizeEpic")) != 1 {
				t.Fatal("approval did not finalize")
			}
		})
	}
}
