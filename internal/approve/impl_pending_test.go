package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func TestS134ImplApprove_PendingSourceBeforeMutation(t *testing.T) {
	for _, status := range []string{"?? source.go", " M source.go", " D source.go"} {
		t.Run(status[:2], func(t *testing.T) {
			f, ex, _ := s134ApproveFixture(t)
			ex.StatusResult = status
			before := kerneltest.Snapshot(t, f)
			_, err := ApproveImpl(f.Root, f.SpecID, ex)
			if err == nil || !strings.Contains(err.Error(), "source.go") || !strings.Contains(err.Error(), "normal workflow") {
				t.Fatalf("pending source did not refuse: %v", err)
			}
			if !strings.Contains(err.Error(), "mindspec next-action "+f.SpecID+" --apply") || !strings.Contains(err.Error(), "mindspec impl approve "+f.SpecID) {
				t.Fatalf("missing runnable recovery: %v", err)
			}
			for _, line := range strings.Split(err.Error(), "\n") {
				if strings.HasPrefix(line, "recovery:") && len(guard.FindFloorMatches(line)) > 0 {
					t.Fatalf("recovery matches the destructive floor: %s", line)
				}
			}
			if before != kerneltest.Snapshot(t, f) || len(ex.CallsTo("FinalizeEpic")) != 0 {
				t.Fatal("refusal changed state or finalized")
			}
		})
	}
}

func TestS134ImplApprove_PendingBeadSourceBeforeMutation(t *testing.T) {
	f, ex, _ := s134ApproveFixture(t)
	dir := filepath.Join(t.TempDir(), "bead")
	f.Must(os.MkdirAll(dir, 0755))
	implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Branch: "bead/" + f.Items[0], Path: dir}}, nil
	}
	wrapped := &pendingBeadExecutor{MockExecutor: ex, dirty: dir}
	before := kerneltest.Snapshot(t, f)
	_, err := ApproveImpl(f.Root, f.SpecID, wrapped)
	if err == nil || !strings.Contains(err.Error(), "source.go") {
		t.Fatalf("pending bead source did not refuse: %v", err)
	}
	if before != kerneltest.Snapshot(t, f) || len(ex.CallsTo("FinalizeEpic")) != 0 {
		t.Fatal("refusal changed state or finalized")
	}
}

type pendingBeadExecutor struct {
	*executor.MockExecutor
	dirty string
}

func (e *pendingBeadExecutor) Status(dir string) (string, error) {
	if dir == e.dirty {
		return "?? source.go", nil
	}
	return "", nil
}

func TestS134ImplApprove_ArtifactPreflightBadID(t *testing.T) {
	ex := &executor.MockExecutor{}
	err := implKernelArtifactPreflight(t.TempDir(), kernel.State{SpecID: "../outside"}, ex)
	if err == nil {
		t.Fatal("invalid spec ID reached artifact preflight")
	}
	if len(ex.CallsTo("Status")) != 0 {
		t.Fatal("invalid spec ID read a worktree")
	}
}

func TestS134ImplApprove_SupersedeADRBeforeMutation(t *testing.T) {
	f, ex, _ := s134ApproveFixture(t)
	before := kerneltest.Snapshot(t, f)
	path, err := workspace.ADRFilePath(f.Root, "ADR-9000")
	f.Must(err)
	_, err = ApproveImpl(f.Root, f.SpecID, ex, ImplOpts{SupersedeADR: "ADR-9000"})
	if err == nil || !strings.Contains(err.Error(), "approval cannot create an ADR outside the accepted subject") {
		t.Fatalf("supersede did not refuse: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("refusal wrote an ADR: %v", err)
	}
	if before != kerneltest.Snapshot(t, f) || len(ex.CallsTo("FinalizeEpic")) != 0 {
		t.Fatal("refusal changed state or finalized")
	}
}

func TestS134ImplApprove_PendingPanelAuthority(t *testing.T) {
	for _, dir := range []string{"review/evil", "reviews/evil", "reviews/other-spec-panel", "recorded-bead", "recorded-final"} {
		for _, file := range []string{"x.go", "x_test.go"} {
			t.Run(dir+"/"+file, func(t *testing.T) {
				f, ex, _ := s134ApproveFixture(t)
				f.Fake.Records["other-1.final"] = kernel.Record{ID: "other-1.final", Parent: "other-1", Type: "step", Status: "closed", Metadata: map[string]any{kernel.KeyTuple: kernel.Tuple{Epic: "other-1", Gate: "other-1.final", Kind: "final-gate", Attempt: 1}, kernel.KeyRegistration: kernel.RegistrationRecord{Slug: "other-spec-panel", Directories: []string{"reviews/other-spec-panel"}}}}
				name := dir + "/" + file
				allowed := strings.HasPrefix(dir, "recorded-")
				if allowed {
					gate := f.Epic + ".final"
					if dir == "recorded-bead" {
						gate = f.Items[0] + ".review"
					}
					reg, _, err := kernel.Decode[kernel.RegistrationRecord](f.State().Records[gate].Metadata, kernel.KeyRegistration)
					f.Must(err)
					reg.Directories = []string{"review/honest"}
					f.Must(os.MkdirAll(filepath.Join(f.Root, "review/honest"), 0755))
					f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, reg)))
					name = "review/honest/" + file
				}
				ex.StatusResult = "?? " + name
				before := kerneltest.Snapshot(t, f)
				_, err := ApproveImpl(f.Root, f.SpecID, ex)
				if allowed {
					if err != nil || len(ex.CallsTo("FinalizeEpic")) != 1 {
						t.Fatalf("recorded panel directory refused: %v", err)
					}
				} else {
					if err == nil || !strings.Contains(err.Error(), name) {
						t.Fatalf("unregistered panel path was not refused: %v", err)
					}
					if before != kerneltest.Snapshot(t, f) || len(ex.CallsTo("FinalizeEpic")) != 0 {
						t.Fatal("refusal changed state or finalized")
					}
				}
			})
		}
	}
}
