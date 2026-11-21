package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/spf13/cobra"
)

func nextKernelFixture(t *testing.T) (*kerneltest.Fixture, string) {
	t.Helper()
	f, step := actorCmdFixture(t)
	withTestChdir(t, f.Root)
	rebindWrite(t, f.Root, ".gitignore", ".mindspec/phase.lock\n")
	rebindWrite(t, f.Root, ".beads/config.yaml", "types.custom: gate,step\n")
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	old, oldGate, oldWT, oldOrphans := nextKernelStatesFn, nextGateReadinessFn, nextEnsureWorktreeFn, nextKernelOrphansFn
	nextKernelOrphansFn = func(string) error { return nil }
	nextKernelStatesFn = func(string, string) ([]kernel.State, error) { return []kernel.State{f.State()}, nil }
	nextGateReadinessFn = func(string, string, next.GateOptions) (*next.GateResult, error) {
		return &next.GateResult{SpecID: f.SpecID}, nil
	}
	nextEnsureWorktreeFn = func(string, string, string, executor.Executor) (string, error) { return f.Root, nil }
	t.Cleanup(func() {
		nextKernelStatesFn, nextGateReadinessFn, nextEnsureWorktreeFn, nextKernelOrphansFn = old, oldGate, oldWT, oldOrphans
	})
	return f, step
}
func TestS134Next_LeaseAndNoForeignWrites(t *testing.T) {
	f, step := nextKernelFixture(t)
	foreign := kernel.Record{ID: "foreign-note", Parent: f.Epic, Type: "task", Status: "open", Title: "note"}
	f.Fake.Records[foreign.ID] = foreign
	bug := kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open"}
	f.Fake.Records[bug.ID] = bug
	downstream := f.Fake.Records[f.Items[1]]
	downstream.Dependencies = append(downstream.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
	f.Fake.Records[downstream.ID] = downstream
	f.Must(f.Store.SetStatusIf(step, "open", "in_progress"))
	beforeRecords := map[string]json.RawMessage{}
	raw, err := json.Marshal(f.Fake.Records)
	f.Must(err)
	f.Must(json.Unmarshal(raw, &beforeRecords))
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	printed, err := kernelPanelRun(t, nextCmd, "1")
	out.WriteString(printed)
	if err != nil || !strings.Contains(out.String(), "generation 1\n1.") {
		t.Fatalf("next: %s %v", out.String(), err)
	}
	lease, ok, err := kernel.Decode[kernel.LeaseRecord](f.State().Records[step].Metadata, kernel.KeyLease)
	f.Must(err)
	if !ok || lease.Generation != 1 {
		t.Fatalf("no lease: %+v", lease)
	}
	if !reflect.DeepEqual(f.Fake.Records[foreign.ID], foreign) {
		t.Fatal("foreign child changed")
	}
	if !reflect.DeepEqual(f.Fake.Records[bug.ID], bug) {
		t.Fatal("foreign bug changed")
	}
	for id, r := range f.Fake.Records {
		after, err := json.Marshal(r)
		f.Must(err)
		if string(beforeRecords[id]) != string(after) && id != f.ID(step) && id != f.Epic {
			t.Fatalf("claim changed unrelated record %s", id)
		}
	}
	epic := f.Fake.Records[f.Epic]
	if epic.Metadata["mindspec_phase"] != "implement" || epic.Metadata[phase.PhaseSemanticsV2MarkerKey] != phase.PhaseSemanticsV2Value {
		t.Fatalf("claim did not advance phase: %v", epic.Metadata)
	}
	before := kerneltest.Snapshot(t, f)
	_, err = runKernelNext(cmd, f.Root, "", "1", 0, nil, false, "")
	if err == nil || !strings.Contains(err.Error(), "lease --take") {
		t.Fatalf("second next: %v", err)
	}
	actorCmdStore(t, f)
	_, err = kernelPanelRun(t, leaseCmd, step)
	if err == nil || !strings.Contains(err.Error(), "lease --take") {
		t.Fatalf("lease after next: %v", err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("refused claim wrote")
	}
}
func TestS134Next_HaltAndActiveSelection(t *testing.T) {
	halted, _ := nextKernelFixture(t)
	decision := kerneltest.OpenDecision(halted, halted.Items[0]+".review", "reject", 1, "fix", "halt")
	kerneltest.Answer(halted, decision, "halt")
	before := kerneltest.Snapshot(t, halted)
	actorCmdStore(t, halted)
	_, leaseErr := kernelPanelRun(t, leaseCmd, kernel.ImplementationStep(halted.State(), halted.Items[0]))
	if leaseErr == nil || !strings.Contains(leaseErr.Error(), "halt") {
		t.Fatalf("halted lease: %v", leaseErr)
	}
	_, err := runKernelNext(&cobra.Command{}, halted.Root, "", "2", 0, nil, false, "")
	if err == nil || !strings.Contains(err.Error(), "halt") {
		t.Fatalf("next halted: %v", err)
	}
	if kerneltest.Snapshot(t, halted) != before {
		t.Fatal("halted next wrote")
	}
	active, step := nextKernelFixture(t)
	active.SpecID = "003-z"
	record := active.Fake.Records[active.Epic]
	record.Metadata["spec_id"] = active.SpecID
	active.Fake.Records[active.Epic] = record
	nextKernelStatesFn = func(string, string) ([]kernel.State, error) {
		return []kernel.State{halted.State(), active.State()}, nil
	}
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	handled, err := runKernelNext(command, active.Root, "", "", 0, nil, false, "")
	if !handled || err != nil {
		t.Fatalf("active next: %t %v", handled, err)
	}
	lease, ok, err := kernel.Decode[kernel.LeaseRecord](active.State().Records[step].Metadata, kernel.KeyLease)
	active.Must(err)
	if !ok || lease.Generation != 1 {
		t.Fatal("active item not selected")
	}
	if kerneltest.Snapshot(t, halted) != before {
		t.Fatal("selection touched halted spec")
	}
}
func TestS134EnrolFault_FinalLinkCycleAfterCreate(t *testing.T) {
	f, _ := nextKernelFixture(t)
	final := f.Epic + ".final"
	// This is the durable boundary after the final gate create and before its links.
	r := f.Fake.Records[f.ID(final)]
	r.Dependencies = nil
	f.Fake.Records[r.ID] = r
	bug := kernel.Record{ID: "foreign-bug", Status: "open", Type: "bug", Dependencies: []kernel.Dependency{{ID: f.ID(final), Type: "blocks"}}}
	f.Fake.Records[bug.ID] = bug
	item := f.Fake.Records[f.Items[2]]
	item.Dependencies = append(item.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
	f.Fake.Records[item.ID] = item
	out, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	data, err := json.Marshal(out.Flags)
	f.Must(err)
	if !strings.Contains(string(data), "foreign-blocker") || !strings.Contains(string(data), bug.ID) || !strings.Contains(string(data), f.ID(final)) {
		t.Fatalf("cycle flag: %s", data)
	}
	current, _ := f.Store.Lookup(final)
	for _, dep := range current.Dependencies {
		if dep.ID == f.Items[2] {
			t.Fatal("cycle link written")
		}
	}
	if !reflect.DeepEqual(f.Fake.Records[bug.ID], bug) {
		t.Fatal("foreign blocker changed")
	}
	bug.Dependencies = nil // The operator removes the foreign link.
	f.Fake.Records[bug.ID] = bug
	out, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	data, err = json.Marshal(out.Flags)
	f.Must(err)
	if strings.Contains(string(data), f.ID(final)) {
		t.Fatalf("stale cycle flag: %s", data)
	}
	current, _ = f.Store.Lookup(final)
	found := false
	for _, dep := range current.Dependencies {
		found = found || dep.ID == f.Items[2]
	}
	if !found {
		t.Fatal("missing repaired final link")
	}
}
func TestS134Next_CheckHaltWritesNothing(t *testing.T) {
	f, _ := nextKernelFixture(t)
	decision := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
	kerneltest.Answer(f, decision, "halt")
	before := kerneltest.Snapshot(t, f)
	if err := kernel.CheckHalt(f.State()); err == nil {
		t.Fatal("halt not refused")
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("halt wrote")
	}
	// Keep all fixture artifacts in the isolated repository.
	if _, err := os.Stat(filepath.Join(f.Root, ".beads/config.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestS134Next_OrphanGuardDefault(t *testing.T) {
	if reflect.ValueOf(nextKernelOrphansFn).Pointer() != reflect.ValueOf(checkUnmergedBeads).Pointer() {
		t.Fatal("kernel next must keep the orphan guard")
	}
}
func TestS134Next_HaltKeepsReadVerbsAvailable(t *testing.T) {
	binary := buildMindspecBinary(t)
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "halt-read", "--gate", "bead", "--bead", f.Items[0], "--target", "bead/"+f.Items[0])
	kernelPanelRequire(t, out, err)
	d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
	kerneltest.Answer(f, d, "halt")
	actorCmdStore(t, f)
	for _, entry := range []struct {
		cmd  *cobra.Command
		args []string
	}{
		{panelTallyCmd, []string{"halt-read", "--spec", f.SpecID}},
		{panelVerifyCmd, []string{"halt-read", "--spec", f.SpecID}},
		{configShowCmd, nil},
	} {
		out, err := kernelPanelRun(t, entry.cmd, entry.args...)
		if out == "" && err == nil {
			t.Fatalf("%s produced no read result", entry.cmd.Name())
		}
		if err != nil && strings.Contains(err.Error(), "spec is halted") {
			t.Fatalf("read verb refused for halt: %s %v", entry.cmd.Name(), err)
		}
	}
	out, err = runBinary(t, binary, f.Root, "doctor", "--ci")
	if out == "" || strings.Contains(out, "error: spec is halted") {
		t.Fatalf("halted doctor did not read: %s %v", out, err)
	}
}

func TestS134Next_ResolveEnrolledStateWithoutSpec(t *testing.T) {
	f, _ := nextKernelFixture(t)
	f.Fake.Records["legacy-2"] = kernel.Record{ID: "legacy-2", Type: "epic", Status: "open", Metadata: map[string]any{"spec_id": "002-y"}}
	actorCmdStore(t, f)
	states, err := readNextKernelStates(f.Root, "")
	f.Must(err)
	if len(states) != 1 || states[0].SpecID != f.SpecID || !states[0].Enrolled || !reflect.DeepEqual(states[0].Items, f.Items) {
		t.Fatalf("unscoped resolution: %+v", states)
	}
}

func TestS134Next_LeasePhaseRecovery(t *testing.T) {
	f, step := nextKernelFixture(t)
	f.Fake.Hook = func(args []string) error {
		if len(args) > 2 && args[0] == "update" && args[1] == f.Epic {
			return errors.New("phase write failed")
		}
		return nil
	}
	out, err := kernelPanelRun(t, nextCmd, "1")
	if err != nil || !strings.Contains(out, "generation 1\n1.") || !strings.Contains(out, "recovery: mindspec repair phase "+f.SpecID) {
		t.Fatalf("durable lease token and warning: %s %v", out, err)
	}
	lease, _, err := kernel.Decode[kernel.LeaseRecord](f.State().Records[step].Metadata, kernel.KeyLease)
	f.Must(err)
	if lease.Generation != 1 || f.Fake.Records[f.Epic].Metadata["mindspec_phase"] != "plan" {
		t.Fatalf("cache failure lost claim or wrote phase: %+v", lease)
	}
	var token string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "1.") {
			token = line
		}
	}
	f.Fake.Hook = nil
	result, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: step, Renew: token}, kernel.LeaseDeps{})
	f.Must(err)
	if len(result.Warnings) != 0 || f.Fake.Records[f.Epic].Metadata["mindspec_phase"] != "implement" {
		t.Fatalf("renew did not repair cache: %+v", result)
	}
	for _, phaseValue := range []string{"review", "done"} {
		f.Must(f.Store.WriteMetadata(f.Epic, map[string]any{"mindspec_phase": phaseValue}))
		_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: step, Renew: token}, kernel.LeaseDeps{})
		f.Must(err)
		if f.Fake.Records[f.Epic].Metadata["mindspec_phase"] != phaseValue {
			t.Fatalf("renew moved phase backwards from %s", phaseValue)
		}
	}
}

func TestS134Next_PreKernelWithEnrolledSibling(t *testing.T) {
	f, _ := nextKernelFixture(t)
	legacy := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	// Both epics and their recorded items are in the same store.
	ids := map[string]string{}
	for id := range legacy.Fake.Records {
		ids[id] = strings.Replace(id, legacy.Epic, "legacy-1", 1)
	}
	for id, row := range legacy.Fake.Records {
		row.ID = ids[id]
		if row.Parent != "" {
			row.Parent = ids[row.Parent]
		}
		for i := range row.Dependencies {
			row.Dependencies[i].ID = ids[row.Dependencies[i].ID]
		}
		f.Fake.Records[row.ID] = row
	}
	legacy.Epic = ids[legacy.Epic]
	for i, id := range legacy.Items {
		legacy.Items[i] = ids[id]
	}
	planPath := filepath.Join(legacy.SpecDir, "plan.md")
	plan, err := os.ReadFile(planPath)
	f.Must(err)
	for old, newID := range ids {
		plan = []byte(strings.ReplaceAll(string(plan), old, newID))
	}
	f.Must(os.WriteFile(planPath, plan, 0600))
	legacy.Store = kernel.NewStore(f.Root, legacy.Epic, f.Fake.Run)
	legacy.Store.NativeIDs = kernel.NativeIDs
	legacyState, err := kernel.ReadState(legacy.Store, legacy.SpecID, legacy.SpecDir)
	f.Must(err)
	if legacyState.Enrolled || legacyState.Records[legacy.Items[0]].ID != legacy.Items[0] {
		t.Fatal("pre-kernel fixture missing from shared store")
	}
	handled, err := runKernelNext(&cobra.Command{}, f.Root, "", legacy.Items[0], 0, nil, false, "")
	if handled || err != nil {
		t.Fatalf("pre-kernel item captured: %t %v", handled, err)
	}
}
func TestS134Next_AllHalted(t *testing.T) {
	f, _ := nextKernelFixture(t)
	decision := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
	kerneltest.Answer(f, decision, "halt")
	before := kerneltest.Snapshot(t, f)
	handled, err := runKernelNext(&cobra.Command{}, f.Root, "", "", 0, nil, false, "")
	if !handled || err == nil || !strings.Contains(err.Error(), "halt") || !strings.Contains(err.Error(), decision) || !strings.Contains(err.Error(), "recovery: mindspec next-action "+f.SpecID) {
		t.Fatalf("missing halt and recovery: %t %v", handled, err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("halted selection wrote")
	}
}
func TestS134Next_SkipsLiveLease(t *testing.T) {
	f, step := nextKernelFixture(t)
	second := f.Fake.Records[f.Items[1]]
	second.Dependencies = nil
	f.Fake.Records[second.ID] = second
	step2 := f.Items[1] + ".impl.a1"
	f.Create(step2, f.Items[1])
	actorCmdGit(t, f, "branch", "bead/"+f.Items[1])
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	_, err := runKernelNext(cmd, f.Root, "", "1", 0, nil, false, "")
	f.Must(err)
	first, _, err := kernel.Decode[kernel.LeaseRecord](f.State().Records[step].Metadata, kernel.KeyLease)
	f.Must(err)
	output.Reset()
	_, err = runKernelNext(cmd, f.Root, "", "", 0, nil, false, "")
	f.Must(err)
	leased, ok, err := kernel.Decode[kernel.LeaseRecord](f.State().Records[step2].Metadata, kernel.KeyLease)
	f.Must(err)
	if !ok || leased.Generation != 1 || !strings.Contains(output.String(), "generation 1") {
		t.Fatalf("second item not leased: %s %+v", output.String(), leased)
	}
	unchanged, _, err := kernel.Decode[kernel.LeaseRecord](f.State().Records[step].Metadata, kernel.KeyLease)
	f.Must(err)
	if !reflect.DeepEqual(first, unchanged) {
		t.Fatal("selection changed first lease")
	}
	_, err = runKernelNext(cmd, f.Root, "", "", 0, nil, false, "")
	if err == nil || !strings.Contains(err.Error(), "lease --take") {
		t.Fatalf("all leased: %v", err)
	}
}

func TestS134Next_SkipsBlockedItem(t *testing.T) {
	f, step := nextKernelFixture(t)
	second := f.Fake.Records[f.Items[1]]
	second.Dependencies = nil
	f.Fake.Records[second.ID] = second
	step2 := f.Items[1] + ".impl.a1"
	f.Create(step2, f.Items[1])
	actorCmdGit(t, f, "branch", "bead/"+f.Items[1])
	bug := kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open"}
	f.Fake.Records[bug.ID] = bug
	first := f.Fake.Records[f.Items[0]]
	first.Dependencies = append(first.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
	f.Fake.Records[first.ID] = first
	before := f.State().Records[step]
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	_, err := runKernelNext(cmd, f.Root, "", "", 0, nil, false, "")
	f.Must(err)
	lease, ok, err := kernel.Decode[kernel.LeaseRecord](f.State().Records[step2].Metadata, kernel.KeyLease)
	f.Must(err)
	if !ok || lease.Generation != 1 || !strings.Contains(out.String(), "generation 1") {
		t.Fatal("unblocked item not leased")
	}
	if !reflect.DeepEqual(before, f.State().Records[step]) || !reflect.DeepEqual(bug, f.Fake.Records[bug.ID]) {
		t.Fatal("blocked step or foreign blocker changed")
	}
}

func TestS134Next_NoReadyNamesLegacySelection(t *testing.T) {
	f, step := nextKernelFixture(t)
	f.Must(f.Store.SetStatusIf(step, "open", "closed"))
	before := kerneltest.Snapshot(t, f)
	handled, err := runKernelNext(&cobra.Command{}, f.Root, "", "", 0, nil, false, "")
	if !handled || err == nil || !strings.Contains(err.Error(), "mindspec next <id>") || !strings.Contains(err.Error(), "mindspec next --spec <spec>") {
		t.Fatalf("missing explicit selection recovery: %v", err)
	}
	if before != kerneltest.Snapshot(t, f) {
		t.Fatal("empty selection wrote")
	}
}

func TestS134Next_UnrelatedBrokenWitness(t *testing.T) {
	f, _ := nextKernelFixture(t)
	f.Fake.Records["broken-3"] = kernel.Record{ID: "broken-3", Type: "epic", Status: "open", Metadata: map[string]any{"spec_id": "003-z", kernel.KeyRun: "broken"}}
	actorCmdStore(t, f)
	states, err := readNextKernelStates(f.Root, "")
	if err != nil || len(states) != 1 || states[0].Epic != f.Epic {
		t.Fatalf("unrelated witness blocked next: %+v %v", states, err)
	}
	if _, err := readNextKernelStates(f.Root, "003-z"); err == nil {
		t.Fatal("named broken witness was hidden")
	}
}
