package main

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS134PanelPrompt_GeneratedBody(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	actorCmdGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
	head = judgeCmdCommit(t, f, "in-scope.go", "scope content")
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	for _, slot := range []string{"R1", "R2"} {
		out, err = kernelPanelRun(t, panelPromptCmd, "1", "--spec", f.SpecID, "--slot", slot)
		if err != nil {
			t.Fatalf("slot prompt refused: %v", err)
		}
		for _, want := range []string{"<!-- mindspec:panel-header -->", "## Files in Scope", "## Summary", "in-scope.go"} {
			if !strings.Contains(out, want) {
				t.Errorf("prompt lacks %q: %s", want, out)
			}
		}
	}
}

func TestS134PanelCreate_RejectsBriefFile(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	file := filepath.Join(t.TempDir(), "body.md")
	f.Must(os.WriteFile(file, []byte("Filled review body."), 0600))
	for _, value := range []string{file, ""} {
		before := kerneltest.Snapshot(t, f)
		out, err := kernelPanelCreate(t, f, "1", "--brief-file", value)
		kernelPanelRefusal(t, out, err, "--brief-file is not accepted")
		if before != kerneltest.Snapshot(t, f) {
			t.Fatal("refused brief file changed state")
		}
	}
}

func TestS134PanelPrompt_PassedPrior(t *testing.T) {
	f, gate, old, dir := judgePanelFixture(t, false)
	actorCmdGit(t, f, "add", "-f", "--", dir)
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "pin pre-refutation panel")
	newest := judgeCmdCommit(t, f, "changed.txt", "J")
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	panelRefuteF(t, f)
	panelJudgeSubmit(t, f, gate, old, newest, "re-panel", 2, []string{"changed.txt"})
	r, _, err := kernel.Decode[kernel.GateRound](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if r.Prior == nil || r.Prior.Verify != "" {
		t.Fatal("fixture must have a prior without verify")
	}
	// Replace the original verdict in a committed panel-only edit after authorization.
	path := filepath.Join(dir, "R5-round-1.json")
	f.Must(os.WriteFile(path, []byte(`{"verdict":"APPROVE","rationale":"edited-head-value"}`), 0600))
	actorCmdGit(t, f, "add", "-f", "--", dir)
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "replace prior verdict")
	out, err := kernelPanelCreate(t, f, "review", "--gate", "bead", "--bead", f.Items[0])
	kernelPanelRequire(t, out, err)
	for _, slot := range []string{"R1", "R2"} {
		out, err = kernelPanelRun(t, panelPromptCmd, "review", "--spec", f.SpecID, "--slot", slot)
		kernelPanelRequire(t, out, err)
		for _, want := range []string{"missing test", "Return: refuted", "Reason: checked", "Evidence: proof.txt", "changed.txt"} {
			if !strings.Contains(out, want) {
				t.Fatalf("missing frozen prior %q: %s", want, out)
			}
		}
		if strings.Contains(out, "edited-head-value") || strings.Contains(out, "Prior-round items could not be read") {
			t.Fatal("prompt reads mutable prior verdict")
		}
	}
}

func TestS134PanelPrompt_RetalliedPrior(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		t.Run(fmt.Sprint(frozen), func(t *testing.T) {
			f, gate, old, _ := judgePanelFixture(t, false)
			newest := judgeCmdCommit(t, f, "changed.txt", "J")
			panelJudgeSubmit(t, f, gate, old, newest, "re-panel", 2, []string{"changed.txt"})
			v := kernelPanelVerifyRecord(t, f, gate, 99)
			if frozen {
				kernelPanelSubmitVerify(t, f, v)
			}
			panelRefuteF(t, f)
			out, err := kernelPanelCreate(t, f, "review", "--gate", "bead", "--bead", f.Items[0])
			kernelPanelRequire(t, out, err)
			out, err = kernelPanelRun(t, panelPromptCmd, "review", "--spec", f.SpecID, "--slot", "R1")
			kernelPanelRequire(t, out, err)
			if !strings.Contains(out, "missing test") || !strings.Contains(out, "Return: refuted") {
				t.Fatalf("retally lost asks: %s", out)
			}
			if frozen && !strings.Contains(out, "Return: reproduced") {
				t.Fatalf("frozen retally lost verify return: %s", out)
			}
		})
	}
}

func TestS134BriefCmd_ActorRecords(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	for _, slot := range []string{"R1", "R2"} {
		kernelPanelVerdict(t, f, gate, head, "review", slot, "REQUEST_CHANGES", 1)
	}
	v := kernelPanelVerifyRecord(t, f, gate, 1)
	_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: v.Step}, kernel.LeaseDeps{})
	f.Must(err)
	actorCmdStore(t, f)
	store := actorCmdStore(t, f)
	before, err := os.ReadFile(store)
	f.Must(err)
	raw, err := f.BinaryCommand(bin, "brief", v.Step)
	f.Must(err)
	for _, want := range []string{head, "Finding: R1", "Finding: R2", v.Step + ".g1.json", "--record"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("verify lacks %q: %s", want, raw)
		}
	}
	after, err := os.ReadFile(store)
	f.Must(err)
	if string(before) != string(after) {
		t.Fatal("brief changed store")
	}
	// A release makes the test's real submit use its own token.
	lease, _, err := kernel.Decode[kernel.LeaseRecord](kernelPanelRecord(t, f, v.Step).Metadata, kernel.KeyLease)
	f.Must(err)
	lease.Expires = kernel.Now()
	f.Must(f.Store.WriteMetadata(v.Step, kernel.Metadata(kernel.KeyLease, lease)))
	kernelPanelSubmitVerify(t, f, v)
	actorCmdStore(t, f)
	raw, err = f.BinaryCommand(bin, "brief", gate+".r2.fix")
	f.Must(err)
	if strings.Contains(string(raw), "Finding: R2") || !strings.HasSuffix(string(raw), "mindspec complete '"+f.Items[0]+"' --check\n") {
		t.Fatalf("wrong fix brief: %s", raw)
	}
}

func TestS134PanelCreate_LegacyAndAdhocBrief(t *testing.T) {
	for _, enrolled := range []bool{false, true} {
		for _, filled := range []bool{false, true} {
			t.Run(fmt.Sprintf("enrolled=%v/filled=%v", enrolled, filled), func(t *testing.T) {
				f, _, _ := kernelPanelFixture(t)
				kind := "adhoc"
				if !enrolled {
					kind = "bead"
					f = kerneltest.PreKernelSpec(t, kerneltest.WithFake(true))
					withTestChdir(t, f.Root)
					rebindWrite(t, f.Root, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      model: test-model\n      count: 2\n")
					actorCmdGit(t, f, "add", ".")
					actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "legacy brief fixture")
					actorCmdGit(t, f, "branch", "bead/"+f.Items[0])
					old := panelKernelStateFn
					panelKernelStateFn = func(string, string) (*kernel.State, error) {
						s := f.State()
						if s.Enrolled {
							t.Fatal("legacy fixture enrolled")
						}
						return nil, nil
					}
					defer func() { panelKernelStateFn = old }()
				}
				args := []string{"legacy", "--gate", kind, "--target", "bead/" + f.Items[0]}
				if !enrolled {
					args = append(args, "--spec", f.SpecID)
				}
				if filled {
					file := filepath.Join(t.TempDir(), "brief.md")
					f.Must(os.WriteFile(file, []byte("Filled authored review body."), 0600))
					args = append(args, "--brief-file", file)
				}
				out, err := kernelPanelRun(t, panelCreateCmd, args...)
				kernelPanelRequire(t, out, err)
				prompt := []string{"legacy", "--slot", "R1"}
				if !enrolled {
					prompt = append(prompt, "--spec", f.SpecID)
				}
				out, err = kernelPanelRun(t, panelPromptCmd, prompt...)
				if filled {
					kernelPanelRequire(t, out, err)
					if !strings.Contains(out, "Filled authored review body.") {
						t.Fatal("authored body missing")
					}
				} else {
					kernelPanelRefusal(t, out, err, "TODO(skill) stubs")
				}
			})
		}
	}
}

func TestS134PanelPrompt_FinalSummaryAndScope(t *testing.T) {
	f, _, base := kernelPanelFixture(t)
	actorCmdGit(t, f, "symbolic-ref", "HEAD", "refs/heads/spec/"+f.SpecID)
	rebindWrite(t, f.Root, filepath.Join(".mindspec", "specs", f.SpecID, "spec.md"), "# Final spec summary\nWhole feature acceptance.\n")
	head := judgeCmdCommit(t, f, "final-feature.go", "final feature")
	gate := f.Epic + ".final"
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, f.SpecID)
	kernelPanelRequire(t, out, err)
	out, err = kernelPanelRun(t, panelPromptCmd, f.SpecID, "--spec", f.SpecID, "--slot", "R1")
	kernelPanelRequire(t, out, err)
	for _, want := range []string{"Final spec summary", "final-feature.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("final prompt lacks %q: %s", want, out)
		}
	}
	if strings.Contains(out, "- Path: proof.txt") {
		t.Fatalf("final scope includes a base-only file at %s", base)
	}
}

func TestS134BriefCmd_ChangeJudge(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, gate, old, newest := judgeCmdFixture(t)
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	id := gate + ".judge.a1"
	_, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	store := actorCmdStore(t, f)
	before, err := os.ReadFile(store)
	f.Must(err)
	raw, err := f.BinaryCommand(bin, "brief", id)
	f.Must(err)
	for _, want := range []string{"Accepted subject: " + old, "New subject: " + newest, "Slot ceiling: 3", "lease_ttl: 3h", id + ".g1.json", "--record"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("judge lacks %q: %s", want, raw)
		}
	}
	after, err := os.ReadFile(store)
	f.Must(err)
	if string(before) != string(after) {
		t.Fatal("judge brief changed store")
	}
}
