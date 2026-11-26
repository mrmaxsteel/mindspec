package kernel_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134PassExcept_GateAttempts(t *testing.T) {
	for _, kind := range []string{"bead", "final"} {
		t.Run(kind, func(t *testing.T) {
			f, gate, old := judgeFixture(t)
			parent := f.Items[0]
			if kind == "final" {
				gate, parent = f.Epic+".final", f.Epic
				reducerGit(t, f, "switch", "spec/"+f.SpecID)
				old = judgeCommit(t, f, "final.go", "package final\n")
				for _, item := range f.Items {
					kerneltest.PassGate(f, item+".review", old)
					f.Must(f.Store.SetStatusIf(item, "open", "closed"))
					f.Must(f.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: old}))
					reducerGit(t, f, "update-ref", "refs/heads/bead/"+item, old)
				}
				kerneltest.PassGate(f, gate, old)
			}
			newest := judgeChange(t, f)
			successor := gate + ".a2"
			f.Create(successor, parent)
			kerneltest.Authorise(f, successor, kernel.GateRound{Round: 2, Subject: newest, SlotCount: 2, Slots: []string{gate + ".r2.R1", gate + ".r2.R2"}})
			kerneltest.Register(f, successor, kernel.RegistrationRecord{Slug: "successor", Round: 2, Subject: newest, Slots: []string{"R1", "R2"}})
			state := f.State()
			paths := kernel.RegisteredPanelPaths(state, gate)
			reg := kernel.NewRegistry("probe")
			register(t, reg, "probe", kernel.Ordinary, func(s kernel.State) (kernel.Changes, error) {
				if s.Records[successor].Status != "open" {
					t.Fatal("excluded gate record is not visible")
				}
				for _, id := range s.Gates {
					if id == successor {
						t.Fatal("excluded gate is a rule target")
					}
				}
				if accepted, err := kernel.AcceptedSubject(s, gate); err != nil || accepted != newest {
					t.Fatalf("selected old passing attempt: %s %v", accepted, err)
				}
				if err := kernel.CheckPassedGate(s, gate); err == nil || !strings.Contains(err.Error(), "gate "+successor+" has no passing receipt") {
					t.Fatalf("gate check lost highest attempt: %v", err)
				}
				if got := kernel.RegisteredPanelPaths(s, gate); !reflect.DeepEqual(got, paths) {
					t.Fatalf("excluded registration paths: %v, want %v", got, paths)
				}
				return kernel.Changes{Writes: []kernel.Write{{Kind: "metadata", ID: successor, Metadata: map[string]any{"probe": true}}}}, nil
			})
			before := kerneltest.Snapshot(t, f)
			for _, apply := range []bool{false, true} {
				result, err := kernel.PassExcept(reg, state, apply, successor)
				f.Must(err)
				if len(result.Writes) != 0 || kerneltest.Snapshot(t, f) != before {
					t.Fatal("pass wrote to excluded gate")
				}
			}
			// Completion omits the work item; gate selection must stay the same.
			for _, excluded := range []string{successor, f.Items[0]} {
				for _, apply := range []bool{false, true} {
					result, err := kernel.PassExcept(kernel.Rules(), f.State(), apply, excluded)
					f.Must(err)
					for _, write := range result.Writes {
						if write.ID == excluded || strings.HasPrefix(write.ID, gate+".judge.") {
							t.Fatalf("excluded %s produced wrong write: %+v", excluded, write)
						}
					}
				}
			}
		})
	}
}
