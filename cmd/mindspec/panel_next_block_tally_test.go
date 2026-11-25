package main

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// TestPanelTally_NextBlockArtifactRoute runs the real `panel tally` on a spec
// gate. A finding whose artifact path is absent from the round tree takes the
// operator route: the operator-action line and no fix step. The same finding
// with its artifact committed takes the fix route.
func TestPanelTally_NextBlockArtifactRoute(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "artifact absent"
		if committed {
			name = "artifact committed"
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
			rebindGit(t, root, "init", "-q", "-b", "spec/900-example")
			rebindGit(t, root, "config", "user.email", "test@example.invalid")
			rebindGit(t, root, "config", "user.name", "Test")
			rebindGit(t, root, "config", "commit.gpgsign", "false")
			rebindWrite(t, root, ".mindspec/specs/900-example/spec.md", validVerbFixtureSpecMD("900-example"))
			rebindWrite(t, root, ".gitignore", ".mindspec/specs/*/reviews/\n")
			if committed {
				rebindWrite(t, root, "evidence/bench.json", "{}\n")
			}
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
				if slot.Slot == "R1" {
					v.Verdict = panel.VerdictRequestChanges
					v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "add the benchmark output", Basis: "traced", ArtifactPath: "evidence/bench.json"}}
				}
				if _, err := panel.Submit(panel.SubmitTarget{Dir: reg.Dir, Slot: slot.Slot, Round: 1}, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
					t.Fatal(err)
				}
			}
			out, err := runPanelVerbCmd("tally", "review")
			if err == nil {
				t.Fatalf("an unresolved finding exited 0:\n%s", out)
			}
			text := out + err.Error()
			if !strings.Contains(text, "Next:") {
				t.Fatalf("no Next block:\n%s", text)
			}
			if strings.Contains(text, panelOperatorActionLine) == committed {
				t.Errorf("operator-action line present=%v, want %v:\n%s", committed, !committed, text)
			}
			for _, m := range append([]string{"fix:"}, fixStepMarkers...) {
				if strings.Contains(text, m) != committed {
					t.Errorf("fix step %q present=%v, want %v:\n%s", m, !committed, committed, text)
				}
			}
		})
	}
}
