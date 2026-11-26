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

func TestS134Submit_RecordedPanelDirectory(t *testing.T) {
	for _, mode := range []string{"recorded-root", "recorded-flat", "old-flat", "unrecorded-root", "unrecorded-flat", "unrecorded-prefix-sibling"} {
		t.Run(mode, func(t *testing.T) {
			f, id, lease, subject := slotFixture(t)
			gate := f.Items[0] + ".review"
			flat, err := filepath.Rel(f.Root, filepath.Join(f.SpecDir, "reviews", "panel"))
			f.Must(err)
			dir := "review/panel"
			reg := kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}, Directories: []string{dir}}
			switch mode {
			case "recorded-flat":
				dir = filepath.ToSlash(flat)
				reg.Directories = []string{dir}
			case "old-flat":
				dir = filepath.ToSlash(flat)
				reg.Directories = nil
			case "unrecorded-root":
				dir = "review/other"
			case "unrecorded-prefix-sibling":
				dir = "review/panelx"
			case "unrecorded-flat":
				dir = filepath.ToSlash(flat)
			}
			f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, reg)))
			path := filepath.Join(dir, "verdict.json")
			actorCommit(t, f, path, verdictBytes(subject, "R1"), "")
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: filepath.Join(f.Root, path)})
			if strings.HasPrefix(mode, "unrecorded") {
				if err == nil || !strings.Contains(err.Error(), "verdict must be the slot attempt verdict or a file in its registered panel directory") {
					t.Fatalf("unrecorded verdict: %v", err)
				}
				if actorRecord(t, f, id).Status == "closed" {
					t.Fatal("unrecorded verdict closed slot")
				}
				return
			}
			f.Must(err)
			receipt := actorReceipt(t, f, id)
			row := actorRecord(t, f, id)
			if row.Status != "closed" || receipt.RecordSHA256 == "" || receipt.Path != "" || kernel.ValidReceipt(f.Root, receipt, row.Metadata) != nil {
				t.Fatalf("missing stored verdict receipt: %+v", receipt)
			}
		})
	}
}

func TestS134Judge_RecordedPanelDirectory(t *testing.T) {
	for _, mode := range []string{"external-record", "worktree-record", "linked-worktree-record"} {
		t.Run(mode, func(t *testing.T) {
			f, gate, old := judgeFixture(t)
			reg, _, err := kernel.Decode[kernel.RegistrationRecord](actorRecord(t, f, gate).Metadata, kernel.KeyRegistration)
			f.Must(err)
			reg.Directories = []string{"review/panel"}
			f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, reg)))
			newest := judgeChange(t, f)
			reduce(t, f, true)
			id := gate + ".judge.a1"
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			wt := filepath.Join(t.TempDir(), "evidence")
			reducerGit(t, f, "worktree", "add", "--detach", wt, newest)
			data, err := json.Marshal(kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same result"})
			f.Must(err)
			path := "review/panel/judgement.json"
			if mode == "unrecorded-root" {
				path = "review/other/judgement.json"
			}
			if mode == "unrecorded-prefix-sibling" {
				path = "review/panelx/judgement.json"
			}
			if mode == "unrecorded-flat" {
				rel, e := filepath.Rel(f.Root, filepath.Join(f.SpecDir, "reviews", reg.Slug, "judgement.json"))
				f.Must(e)
				path = filepath.ToSlash(rel)
			}
			path = filepath.Join(t.TempDir(), "judgement.json")
			if mode == "worktree-record" {
				path = filepath.Join(f.Root, "judgement.json")
			}
			if mode == "linked-worktree-record" {
				path = filepath.Join(wt, "judgement.json")
			}
			f.Must(os.WriteFile(path, data, 0600))
			before := kerneltest.Snapshot(t, f)
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path})
			if mode != "external-record" {
				if err == nil || !strings.Contains(err.Error(), "outside every worktree") {
					t.Fatalf("unrecorded judgement: %v", err)
				}
				if before != kerneltest.Snapshot(t, f) {
					t.Fatal("location refusal wrote state")
				}
				return
			}
			f.Must(err)
			accepted, err := kernel.AcceptedSubject(f.State(), gate)
			f.Must(err)
			if accepted != newest {
				t.Fatalf("accepted=%s want=%s", accepted, newest)
			}
		})
	}
}
