package kernel_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Allowance_LimitAndContinue(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	counted(t, f, 5)
	closePass(t, f, gate+".r6.fix")
	if got := kernel.Allowance(f.State(), gate); got != 5 {
		t.Fatalf("limit=%d", got)
	}
	if out := reduce(t, f, true); hasRun(out, "mindspec", "panel", "create", f.Items[0]) {
		t.Fatal("round past allowance listed")
	}
	d := kerneltest.OpenDecision(f, gate, "limit", 5, "continue:<n>", "halt")
	kerneltest.Answer(f, d, "continue:1")
	if got := kernel.Allowance(f.State(), gate); got != 6 {
		t.Fatalf("allowance=%d", got)
	}
	out := reduce(t, f, true)
	if !hasRun(out, "mindspec", "panel", "create", f.Items[0]) {
		t.Fatalf("continuation did not release round: %+v", out)
	}
	other := f.Items[1] + ".review"
	if kernel.Allowance(f.State(), other) != 5 {
		t.Fatal("continuation leaked to other gate")
	}
	// Final-gate answers have their own allowance.
	final := f.Epic + ".final"
	d = kerneltest.OpenDecision(f, final, "limit", 5, "continue:<n>", "halt")
	kerneltest.Answer(f, d, "continue:2")
	if kernel.Allowance(f.State(), final) != 7 || kernel.Allowance(f.State(), gate) != 6 {
		t.Fatal("final allowance is not local")
	}
}
func TestS134Allowance_LedgerOnlyAndMalformed(t *testing.T) {
	t.Run("ledger-only", func(t *testing.T) {
		f := reducerFixture(t)
		gate := f.Items[0] + ".review"
		counted(t, f, 5)
		closePass(t, f, gate+".r6.fix")
		path := filepath.Join(f.SpecDir, "reviews", "panel", "panel.json")
		f.Must(os.Rename(path, path+".saved"))
		state := f.State()
		n, err := kernel.RoundsForGate(state, gate)
		f.Must(err)
		if n != 5 {
			t.Fatalf("snapshot deletion erased rounds: %d", n)
		}
		if out := reduce(t, f, true); hasRun(out, "mindspec", "panel", "create", f.Items[0]) {
			t.Fatal("ledger-only count bypassed allowance")
		}
	})
	t.Run("unrelated-malformed", func(t *testing.T) {
		f := reducerFixture(t)
		gate := f.Items[0] + ".review"
		closePass(t, f, f.Items[0]+".impl.a1")
		state := f.State()
		dir := filepath.Join(f.Root, ".mindspec", "reviews", "unrelated")
		put(t, filepath.Join(dir, "dispositions.jsonl"), "{not json\n")
		// The direct caller and a fresh state read both fail closed and name the directory.
		state.Ledgers[dir] = nil
		n, err := kernel.RoundsForGate(state, gate)
		if err == nil || n != 0 || !strings.Contains(err.Error(), dir) {
			t.Fatalf("unrelated malformed ledger not named: %d %v", n, err)
		}
		before := kerneltest.Snapshot(t, f)
		_, err = kernel.NextAction(state, true)
		if err == nil || !strings.Contains(err.Error(), dir) || before != kerneltest.Snapshot(t, f) {
			t.Fatalf("malformed ledger wrote or was hidden: %v", err)
		}
	})
}
func TestS134Allowance_AcrossRegistrations(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	for i := 1; i <= 5; i++ {
		slug := "a"
		round := i
		if i > 3 {
			slug = "b"
			round = i - 3
		}
		kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: slug, Round: round, Subject: reducerGit(t, f, "rev-parse", "bead/"+f.Items[0]), Slots: []string{"R1", "R2", "R3"}})
	}
	n, err := kernel.RoundsForGate(f.State(), gate)
	f.Must(err)
	if n != 5 {
		t.Fatal(fmt.Sprintf("cross-registration count=%d", n))
	}
}
