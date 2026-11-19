package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func judgeCmdFixture(t *testing.T) (*kerneltest.Fixture, string, string, string) {
	t.Helper()
	f, _ := actorCmdFixture(t)
	f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".beads", "config.yaml"), []byte("types.custom: gate,step\n"), 0644))
	branch, err := workspace.BeadBranch(f.Items[0])
	f.Must(err)
	actorCmdGit(t, f, "switch", branch)
	old := judgeCmdCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      count: 3\n")
	gate := f.Items[0] + ".review"
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: old, Slots: []string{"R1", "R2", "R3"}})
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: old, SlotCount: 3, Slots: []string{}})
	// Close the implement step so the fixture models a completed passing round.
	f.Must(f.Store.WriteReceipt(f.Items[0]+".impl.a1", kernel.Receipt{Kind: "passed", Retally: "seeded-implementation"}))
	f.Must(f.Store.SetStatusIf(f.Items[0]+".impl.a1", "open", "closed"))
	kerneltest.PassGate(f, gate, old)
	newest := judgeCmdCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      count: 3\n  lease_ttl: 3h\n")
	return f, gate, old, newest
}
func judgeCmdCommit(t *testing.T, f *kerneltest.Fixture, path, content string) string {
	t.Helper()
	file := filepath.Join(f.Root, path)
	f.Must(os.MkdirAll(filepath.Dir(file), 0755))
	f.Must(os.WriteFile(file, []byte(content), 0644))
	actorCmdGit(t, f, "add", "--", path)
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "judge fixture")
	return actorCmdGit(t, f, "rev-parse", "HEAD")
}
func judgeCmdApply(t *testing.T, f *kerneltest.Fixture, bin string) kernel.Output {
	t.Helper()
	raw, code := actorCmdRun(t, f, bin, "next-action", f.SpecID, "--apply", "--json")
	if code != 0 {
		t.Fatalf("apply: %d %s", code, raw)
	}
	var out kernel.Output
	f.Must(json.Unmarshal([]byte(raw), &out))
	return out
}
func judgeCmdLease(t *testing.T, f *kerneltest.Fixture, bin, id string) string {
	t.Helper()
	out, code := actorCmdRun(t, f, bin, "lease", id)
	fields := strings.Fields(out)
	if code != 0 || len(fields) != 3 {
		t.Fatalf("lease: %d %s", code, out)
	}
	return fields[2]
}
func judgeCmdArtifact(t *testing.T, f *kerneltest.Fixture, j kernel.Judgement) string {
	t.Helper()
	data, err := json.Marshal(j)
	f.Must(err)
	path := filepath.Join(".mindspec", "specs", f.SpecID, "reviews", "panel", "judgement.json")
	judgeCmdCommit(t, f, path, string(data))
	return filepath.Join(f.Root, path)
}
func TestS134JudgeCmd_CreateSupersedeAndCarry(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, gate, old, newest := judgeCmdFixture(t)
	store := actorCmdStore(t, f)
	out := judgeCmdApply(t, f, bin)
	id := gate + ".judge.a1"
	if len(out.Actions) != 1 || out.Actions[0].Step != id || out.Actions[0].Subject.Commit != newest || !out.Actions[0].Exists || len(out.Run) != 0 {
		t.Fatalf("judge action: %+v", out)
	}
	token := judgeCmdLease(t, f, bin, id)
	newest = judgeCmdCommit(t, f, "app.go", "package app\n")
	judgeCmdApply(t, f, bin)
	receipt, _, err := kernel.Decode[kernel.Receipt](actorCmdRead(t, store, id).Metadata, kernel.KeyClose)
	f.Must(err)
	if receipt.Kind != "superseded" || receipt.Subject != newest {
		t.Fatalf("superseded receipt: %+v", receipt)
	}
	artifact := judgeCmdArtifact(t, f, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "review covers the new change"})
	raw, code := actorCmdRun(t, f, bin, "submit", id, "--lease", token, "--artifact", artifact)
	if code == 0 {
		t.Fatalf("stale submit: %s", raw)
	}
	id = gate + ".judge.a2"
	token = judgeCmdLease(t, f, bin, id)
	raw, code = actorCmdRun(t, f, bin, "submit", id, "--lease", token, "--artifact", artifact)
	if code != 0 {
		t.Fatalf("carry-over: %d %s", code, raw)
	}
	row := actorCmdRead(t, store, gate)
	receipt, _, err = kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
	f.Must(err)
	if row.Status != "closed" || receipt.Kind != "passed" || len(receipt.CarryOvers) != 1 || receipt.CarryOvers[0].OldSubject != old || receipt.CarryOvers[0].NewSubject != newest || receipt.CarryOvers[0].Receipt == nil {
		t.Fatalf("carry receipt: %+v", receipt)
	}
	out = judgeCmdApply(t, f, bin)
	if len(out.Actions) != 0 {
		t.Fatalf("judgement artifact created a judge: %+v", out)
	}
	before, err := os.ReadFile(store)
	f.Must(err)
	raw, code = actorCmdRun(t, f, bin, "submit", id, "--lease", token, "--artifact", artifact)
	after, err := os.ReadFile(store)
	f.Must(err)
	if code != 0 || string(before) != string(after) {
		t.Fatalf("retry wrote: %d %s", code, raw)
	}
}
func TestS134JudgeCmd_InvalidThenRepanel(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, n := range []int{2, 3} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			f, gate, old, newest := judgeCmdFixture(t)
			store := actorCmdStore(t, f)
			judgeCmdApply(t, f, bin)
			id := gate + ".judge.a1"
			token := judgeCmdLease(t, f, bin, id)
			for _, bad := range []kernel.Judgement{
				{Value: "re-panel", SlotCount: 1, Paths: []string{".mindspec/config.yaml"}},
				{Value: "re-panel", SlotCount: 4, Paths: []string{".mindspec/config.yaml"}},
				{Value: "other"}, {Value: "carry-over"}, {Value: "re-panel", SlotCount: 2},
			} {
				bad.OldSubject = old
				bad.NewSubject = newest
				artifact := judgeCmdArtifact(t, f, bad)
				before, err := os.ReadFile(store)
				f.Must(err)
				raw, code := actorCmdRun(t, f, bin, "submit", id, "--lease", token, "--artifact", artifact)
				after, err := os.ReadFile(store)
				f.Must(err)
				if code == 0 || string(before) != string(after) || actorCmdRead(t, store, id).Status != "in_progress" {
					t.Fatalf("bad judgement changed step: %d %s", code, raw)
				}
			}
			paths := []string{".mindspec/config.yaml"}
			artifact := judgeCmdArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: n, Paths: paths})
			raw, code := actorCmdRun(t, f, bin, "submit", id, "--lease", token, "--artifact", artifact)
			if code != 0 {
				t.Fatalf("re-panel: %d %s", code, raw)
			}
			var out kernel.Output
			f.Must(json.Unmarshal([]byte(raw), &out))
			row := actorCmdRead(t, store, gate+".a2")
			from, _, err := kernel.Decode[kernel.From](row.Metadata, kernel.KeyFrom)
			f.Must(err)
			round, _, err := kernel.Decode[kernel.GateRound](row.Metadata, kernel.KeyRound)
			f.Must(err)
			if string(from) != gate || round.Round != 2 || round.Subject != newest || round.SlotCount != n || !reflect.DeepEqual(round.Paths, paths) || len(out.Run) != 1 || !reflect.DeepEqual(out.Run[0].Paths, paths) || !reflect.DeepEqual(out.Run[0].Argv, []string{"mindspec", "panel", "create", f.Items[0]}) {
				t.Fatalf("wrong re-panel authorisation: %+v %+v", row, out)
			}
		})
	}
}
