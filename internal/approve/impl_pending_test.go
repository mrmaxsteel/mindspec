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
