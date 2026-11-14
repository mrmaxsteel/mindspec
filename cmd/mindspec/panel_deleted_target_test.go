package main

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"strings"
	"testing"
)

func TestPanelDeletedNamedTargetTracksSpecBranch(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/changed=%t", gate, changed), func(t *testing.T) {
				root, _, reviewed := setupRebind(t, "plan-pinned")
				rebindGit(t, root, "tag", "review-target", reviewed)
				dir := createObjectPinPanel(t, root, gate, "review-target", reviewed)
				rebindGit(t, root, "tag", "-d", "review-target")
				if changed {
					rebindGit(t, root, "checkout", "-q", "spec/900-demo")
					name := "plan.md"
					if gate == "spec_approve" {
						name = "spec.md"
					}
					rebindWrite(t, root, ".mindspec/specs/900-demo/"+name, "# unreviewed document edit\n")
					rebindGit(t, root, "add", ".mindspec/specs/900-demo/"+name)
					rebindGit(t, root, "commit", "-qm", "unreviewed document edit")
					rebindGit(t, root, "checkout", "-q", "main")
				}
				want := panel.Allow
				if changed {
					want = panel.Block
				}
				if d := rebindGate(t, root, dir); d.Action != want {
					t.Fatalf("deleted target decision=%+v, want %v", d, want)
				}
				out, err := runPanelVerbCmd("verify", "object-pin")
				if changed {
					if strings.Contains(out, "\nPASS") || strings.HasPrefix(out, "PASS") || !strings.Contains(out, "BLOCK:") {
						t.Fatalf("verify passed unreviewed edits: %v %s", err, out)
					}
				} else if err != nil || !strings.Contains(out, "PASS") || strings.Contains(out, "advisory") {
					t.Fatalf("unchanged subject must stand: %v %s", err, out)
				}
				out, err = runPanelVerbCmd("tally", "object-pin")
				if changed {
					if err == nil || !strings.Contains(out, "decision: BLOCK") {
						t.Fatalf("tally passed unreviewed edits: %v %s", err, out)
					}
				} else if err != nil || !strings.Contains(out, "decision: PASS") {
					t.Fatalf("unchanged tally: %v %s", err, out)
				}
				if err := panel.CheckLifecycleTarget(gate, "900-demo", "review-target"); err == nil {
					t.Fatal("approval must continue refusing legacy named targets")
				}
			})
		}
	}
}

func TestPanelDeletedNamedTargetAndAbsentBranchKeepsAdvisory(t *testing.T) {
	root, _, reviewed := setupRebind(t, "plan-pinned")
	rebindGit(t, root, "tag", "review-target", reviewed)
	dir := createObjectPinPanel(t, root, "plan_approve", "review-target", reviewed)
	rebindGit(t, root, "tag", "-d", "review-target")
	rebindGit(t, root, "branch", "-D", "spec/900-demo")
	if d := rebindGate(t, root, dir); d.Action != panel.Warn {
		t.Fatalf("both refs absent must retain advisory: %+v", d)
	}
}
