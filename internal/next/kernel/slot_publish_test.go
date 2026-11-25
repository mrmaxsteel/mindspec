package kernel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// attemptVerdict writes data as the verdict of a slot attempt directory, the
// path that the slot prompt names. No commit holds it.
func attemptVerdict(t *testing.T, f *kerneltest.Fixture, slot, data string) string {
	t.Helper()
	iso, err := config.IsolationRootFor(f.Root)
	f.Must(err)
	layout, err := panel.NewSlotLayout(f.Root, iso, f.SpecID, "panel", 1, slot)
	f.Must(err)
	file := filepath.Join(layout.Dir, "attempt-1", "verdict.json")
	f.Must(os.MkdirAll(filepath.Dir(file), 0755))
	f.Must(os.WriteFile(file, []byte(data), 0644))
	return file
}

func TestS134Submit_SlotAttemptVerdictPublished(t *testing.T) {
	f, id, lease, subject := slotFixture(t)
	gate := f.Items[0] + ".review"
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
	data := verdictBytes(subject, "R1")
	file := attemptVerdict(t, f, "R1", data)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("the printed slot submit was refused: %v", err)
	}
	if status := actorRecord(t, f, id).Status; status != "closed" {
		t.Fatalf("slot status %q, want closed", status)
	}
	receipt := actorReceipt(t, f, id)
	want := ".mindspec/specs/" + f.SpecID + "/reviews/panel/R1-round-1.json"
	if receipt.Path != want {
		t.Fatalf("receipt path %q, want the registered path %q", receipt.Path, want)
	}
	if held := actorCommand(t, f, "show", receipt.Commit+":"+want); held != data {
		t.Fatalf("published bytes differ: %q", held)
	}
	if parent := actorCommand(t, f, "rev-parse", receipt.Commit+"^1"); parent != subject {
		t.Fatalf("published commit parent %s, want the round subject %s", parent, subject)
	}
	blob := actorCommand(t, f, "rev-parse", receipt.Commit+":"+want)
	if ref := actorCommand(t, f, "rev-parse", kernel.SlotVerdictRef+id+"/"+blob); ref != receipt.Commit {
		t.Fatalf("verdict ref %s does not keep commit %s", ref, receipt.Commit)
	}
	if head := actorCommand(t, f, "rev-parse", "HEAD"); head != subject {
		t.Fatalf("publish moved HEAD to %s", head)
	}
}

func TestS134Submit_SlotAttemptOfOtherSlotNamesCommand(t *testing.T) {
	f, id, lease, subject := slotFixture(t)
	gate := f.Items[0] + ".review"
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
	file := attemptVerdict(t, f, "R2", verdictBytes(subject, "R1"))
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	assertSubmitRefusal(t, err, "artifact has no holding commit", "mindspec panel prompt 'panel' --slot R1 --spec '"+f.SpecID+"', then write the verdict to the path it names and run the submit command it prints")
	if actorRecord(t, f, id).Status == "closed" {
		t.Fatal("a verdict of another slot closed this slot")
	}
	if refs := actorCommand(t, f, "for-each-ref", kernel.SlotVerdictRef); refs != "" {
		t.Fatalf("a refused submit published a verdict: %s", refs)
	}
}
