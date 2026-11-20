package kernel_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Seed_Goldens(t *testing.T) {
	for _, name := range []string{"enrolled", "PassGate", "Authorise", "OpenDecision", "Answer", "Register", "Repaired", "Judge"} {
		t.Run(name, func(t *testing.T) {
			kerneltest.SetClock(t, fixedTime)
			f := kerneltest.Enrolled(t, kerneltest.WithFake(true))
			gate := f.Items[0] + ".review"
			id := gate
			switch name {
			case "enrolled":
				golden(t, "seed-enrolled", f.State().Records)
				return
			case "PassGate":
				kerneltest.PassGate(f, gate, strings.Repeat("a", 40))
			case "Authorise":
				kerneltest.Authorise(f, gate, kernel.GateRound{Round: 2, Subject: "subject", SlotCount: 2, Slots: []string{"R1", "R2"}, Paths: []string{"app.go"}})
			case "OpenDecision":
				id = kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
			case "Answer":
				id = kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
				kerneltest.Answer(f, id, "halt")
				if !f.State().Halted() {
					t.Fatal("answer receipt does not halt")
				}
			case "Register":
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 2, Subject: "subject", Slots: []string{"R1", "R2"}})
			case "Repaired":
				id = f.Items[0] + ".impl.a1"
				f.Create(id, f.Items[0])
				kerneltest.Repaired(f, id, kernel.Receipt{Kind: "superseded", Subject: "new"})
			case "Judge":
				id = kerneltest.Judge(f, gate, "new", 1)
			}
			if name == "Register" {
				state := f.State()
				if len(state.Panels) != 1 || len(state.Ledgers) != 1 {
					t.Fatalf("registered evidence missing: %+v", state)
				}
				data, e := os.ReadFile(filepath.Join(f.SpecDir, "reviews", "panel", "panel.json"))
				if e != nil {
					t.Fatal(e)
				}
				var snapshot any
				if e = json.Unmarshal(data, &snapshot); e != nil {
					t.Fatal(e)
				}
				golden(t, "seed-panel", snapshot)
				for _, lines := range state.Ledgers {
					if len(lines) != 1 {
						t.Fatal("registration ledger missing")
					}
					var event any
					if e = json.Unmarshal(lines[0], &event); e != nil {
						t.Fatal(e)
					}
					golden(t, "seed-round", event)
				}
			}
			record, err := f.Store.Lookup(id)
			if err != nil {
				t.Fatal(err)
			}
			golden(t, "seed-"+name, record)
		})
	}
}
