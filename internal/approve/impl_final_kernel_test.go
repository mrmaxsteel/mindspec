package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func finalKernelCommit(t *testing.T, f *kerneltest.Fixture, name string) string {
	t.Helper()
	_, err := f.Command("git", "switch", "spec/"+f.SpecID)
	f.Must(err)
	path := filepath.Join(f.Root, name)
	f.Must(os.MkdirAll(filepath.Dir(path), 0755))
	f.Must(os.WriteFile(path, []byte("# subject changed\n"), 0600))
	_, err = f.Command("git", "add", name)
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "subject")
	f.Must(err)
	out, err := f.Command("git", "rev-parse", "HEAD")
	f.Must(err)
	return strings.TrimSpace(string(out))
}
func TestS134ImplFinalGate_AcceptedSubject(t *testing.T) {
	for _, carry := range []bool{false, true} {
		t.Run(map[bool]string{true: "carry-over", false: "moved"}[carry], func(t *testing.T) {
			f, ex, _ := s134ApproveFixture(t)
			gate := f.Epic + ".final"
			old, err := kernel.AcceptedSubject(f.State(), gate)
			f.Must(err)
			head := finalKernelCommit(t, f, ".mindspec/config.yaml")
			judge := kerneltest.Judge(f, gate, head, 1)
			_, _, err = implFinalReviewGate(f.Root, f.SpecID, f.Epic, "review", f.SpecDir, ex, implPlaceholderWrite{})
			if err == nil || !strings.Contains(err.Error(), head) || !strings.Contains(err.Error(), old) || !strings.Contains(err.Error(), judge) {
				t.Fatalf("missing subject or judge: %v", err)
			}
			if !carry {
				return
			}
			r, err := f.Store.Lookup(gate)
			f.Must(err)
			receipt, _, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
			f.Must(err)
			receipt.CarryOvers = []kernel.CarryOver{{OldSubject: old, NewSubject: head}}
			f.Must(f.Store.WriteReceipt(gate, receipt))
			f.Must(f.Store.WriteReceipt(judge, kernel.Receipt{Kind: "superseded", Rebind: "carry-over"}))
			f.Must(f.Store.SetStatusIf(judge, "open", "closed"))
			_, _, err = implFinalReviewGate(f.Root, f.SpecID, f.Epic, "review", f.SpecDir, ex, implPlaceholderWrite{})
			f.Must(err)
			_, err = ApproveImpl(f.Root, f.SpecID, ex)
			f.Must(err)
		})
	}
}
func TestS134ImplFinalGate_PanelOnlySubject(t *testing.T) {
	f, ex, _ := s134ApproveFixture(t)
	gate := f.Epic + ".final"
	old, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "final", Round: 1, Subject: old, Slots: []string{"R1", "R2"}})
	finalKernelCommit(t, f, filepath.ToSlash(filepath.Join(".mindspec/specs", f.SpecID, "reviews/final/note.txt")))
	_, _, err = implFinalReviewGate(f.Root, f.SpecID, f.Epic, "review", f.SpecDir, ex, implPlaceholderWrite{})
	f.Must(err)
	_, err = ApproveImpl(f.Root, f.SpecID, ex)
	f.Must(err)
}
func TestS134ImplFinalGate_JudgeNamesSubjectInApproval(t *testing.T) {
	f, ex, _ := s134ApproveFixture(t)
	gate := f.Items[0] + ".review"
	old, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	_, err = f.Command("git", "switch", "bead/"+f.Items[0])
	f.Must(err)
	f.Must(os.WriteFile(filepath.Join(f.Root, "work.txt"), []byte("change"), 0600))
	_, err = f.Command("git", "add", "work.txt")
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "change")
	f.Must(err)
	raw, err := f.Command("git", "rev-parse", "HEAD")
	f.Must(err)
	head := strings.TrimSpace(string(raw))
	judge := kerneltest.Judge(f, gate, head, 1)
	_, err = ApproveImpl(f.Root, f.SpecID, ex)
	if err == nil || !strings.Contains(err.Error(), judge) || !strings.Contains(err.Error(), head) || !strings.Contains(err.Error(), old) {
		t.Fatalf("approval did not name all checks: %v", err)
	}
}
