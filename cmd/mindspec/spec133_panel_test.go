package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestS133PanelTarget_RefusesCommitForEveryGate(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
			for _, target := range []string{strings.Repeat("a", 40), "spec/901-other"} {
				t.Run(policy+"/"+gate+"/"+target, func(t *testing.T) {
					resetPanelCreateFlags(t)
					root := mkFlatPanelTestRoot(t)
					withTestChdir(t, root)
					rebindWrite(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
					config.ResetCache()
					t.Cleanup(config.ResetCache)
					out, err := runPanelCmd("create", "review", "--spec", "900-example", "--gate", gate, "--target", target)
					if err == nil || otelExitCode(err) == 0 {
						t.Fatalf("target %q accepted: %s", target, out)
					}
					message := err.Error()
					if !strings.Contains(message, "--target spec/900-example") || !strings.Contains(message, gate) {
						t.Fatalf("wrong refusal: %s", message)
					}
					if regexp.MustCompile(`(?i)ADR-[0-9]+|spec[ -][0-9]+|\bR[0-9]+\.[0-9]+|mindspec-[a-z0-9]+|#[0-9]+`).MatchString(strings.ReplaceAll(message, "900-example", "<id>")) {
						t.Fatalf("internal reference: %s", message)
					}
					if _, err := os.Stat(filepath.Join(root, ".mindspec/specs/900-example/reviews/review/panel.json")); !os.IsNotExist(err) {
						t.Fatal("refusal wrote registration")
					}
				})
			}
		}
	}
}

func TestS133PanelDigest_CreateRebindVerify(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
			t.Run(policy+"/"+gate, func(t *testing.T) {
				resetPanelCreateFlags(t)
				resetV2Flags(t)
				stubWorktreeListEmpty(t)
				root := mkFlatPanelTestRoot(t)
				withTestChdir(t, root)
				rebindGit(t, root, "init", "-q", "-b", "spec/900-example")
				rebindWrite(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
				const spec = ".mindspec/specs/900-example/spec.md"
				rebindWrite(t, root, spec, "---\nstatus: Draft\ntitle: 'Example'\npriority: 1\n---\n# Spec\n")
				rebindWrite(t, root, ".gitignore", ".mindspec/specs/*/reviews/\n")
				rebindGit(t, root, "add", ".")
				rebindGit(t, root, "commit", "-qm", "base")
				rebindGit(t, root, "tag", "spec/900-example")
				rebindGit(t, root, "commit", "--allow-empty", "-qm", "advance branch beyond shadow tag")
				config.ResetCache()
				t.Cleanup(config.ResetCache)
				lsTreeForPanelFn = func(workdir, ref string) (string, error) {
					return executor.NewMindspecExecutor(workdir).LsTree(workdir, ref)
				}
				if out, err := runPanelCmd("create", "review", "--spec", "900-example", "--gate", gate, "--target", "spec/900-example", "--reviewers", "2"); err != nil {
					t.Fatalf("create: %v %s", err, out)
				}
				reg, err := findPanelRegistration(root, "review")
				if err != nil {
					t.Fatal(err)
				}
				checkDigest := func() {
					t.Helper()
					p := rebindRegistration(t, reg.Dir).Panel
					if p.ReviewedHeadSHA != rebindGit(t, root, "rev-parse", "HEAD") {
						t.Fatal("panel followed the shadow tag instead of the spec branch")
					}
					exec := executor.NewMindspecExecutor(root)
					listing, err := exec.LsTree(root, "HEAD")
					if err != nil {
						t.Fatal(err)
					}
					want, err := panel.DocumentSubjectDigest(listing, panel.Subject(gate, ".mindspec/specs/900-example"), ".mindspec/specs/900-example", gate, func(name string) ([]byte, error) { return exec.FileAtRef("HEAD", name) })
					if err != nil || p.SubjectDigest != want {
						t.Errorf("digest=%s want %s err=%v", p.SubjectDigest, want, err)
					}
				}
				checkDigest()
				for _, slot := range reg.Panel.Roster {
					v := panel.VerdictV2{Schema: 2, Slot: slot.Slot, Round: 1, ReviewedSHA: reg.Panel.ReviewedHeadSHA, Status: "final", Verdict: "APPROVE", Model: slot.Model, Rationale: "reviewed", ConcreteChangesRequired: []panel.ChangeRequired{}, CommandsRun: []panel.CommandRun{}}
					raw := mustJSONS133(t, v)
					if _, err := panel.Submit(panel.SubmitTarget{Dir: reg.Dir, Slot: slot.Slot, Round: 1}, raw, func(_ panel.Panel, s panel.RosterSlot) (panel.SubmitResolution, error) {
						return panel.SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "harness", RunnerSource: "default"}, nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				rebindWrite(t, root, spec, "---\nstatus: Draft\ntitle: 'Example'\npriority: 1\n---\n# Spec\nBody edit\n")
				rebindGit(t, root, "add", spec)
				rebindGit(t, root, "commit", "-qm", "edit")
				if out, err := runPanelVerbCmd("rebind", "review", "--reason", "operator: reviewed body edit"); err != nil {
					t.Fatalf("rebind: %v %s", err, out)
				}
				checkDigest()
				rebindWrite(t, root, spec, "---\npriority: 1\ntitle: Example\nstatus: Draft\n---\n# Spec\nBody edit\n")
				rebindGit(t, root, "add", spec)
				rebindGit(t, root, "commit", "-qm", "reorder")
				if out, err := runPanelVerbCmd("verify", "review"); err != nil || !strings.Contains(out, "\nPASS\n") {
					t.Fatalf("verify after reorder: %v %s", err, out)
				}
			})
		}
	}
}

func mustJSONS133(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
