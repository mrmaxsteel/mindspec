package approve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134ImplApprove_DoneAfterGateRepair(t *testing.T) {
	for _, kind := range []string{"bead", "final"} {
		t.Run(kind, func(t *testing.T) {
			f, ex, _ := s134ApproveFixture(t)
			f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
			f.Must(os.WriteFile(filepath.Join(f.Root, ".beads", "config.yaml"), []byte("types.custom: gate,step\n"), 0644))
			gate := f.Items[0] + ".review"
			if kind == "final" {
				gate = f.Epic + ".final"
			}
			r := f.Fake.Records[f.ID(gate)]
			delete(r.Metadata, kernel.KeyClose)
			f.Fake.Records[r.ID] = r
			_, err := kernel.Pass(kernel.Rules(), f.State(), true)
			f.Must(err)
			_, err = kernel.Pass(kernel.Rules(), f.State(), true)
			f.Must(err)
			if !kernel.RepairedClose(f.State(), gate) {
				t.Fatal("not repaired")
			}
			_, err = ApproveImpl(f.Root, f.SpecID, ex)
			f.Must(err)
			if f.State().Records[f.Epic].Status != "closed" {
				t.Fatal("approval did not close epic")
			}
			for _, apply := range []bool{false, true} {
				out, err := kernel.NextAction(f.State(), apply)
				f.Must(err)
				if out.Status != "done" || kernel.OutputExitCode(out) != 5 {
					t.Fatalf("successful ApproveImpl left next-action status=%s exit=%d flags=%+v actions=%+v run=%+v", out.Status, kernel.OutputExitCode(out), out.Flags, out.Actions, out.Run)
				}
			}
		})
	}
}
