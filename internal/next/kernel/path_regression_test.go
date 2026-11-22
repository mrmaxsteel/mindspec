package kernel_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134Submit_PathFailureEscapes(t *testing.T) {
	for _, mode := range []string{"record", "artifact", "TMPDIR"} {
		t.Run(mode, func(t *testing.T) {
			kind := "lens"
			if mode == "artifact" {
				kind = "R1"
			}
			f, id, lease, _ := recordFixture(t, kind)
			name := filepath.Join(t.TempDir(), "missing\x1b[31m\nrecovery: injected")
			before := actorRecord(t, f, id)
			options := kernel.SubmitOptions{Step: id, Lease: lease.Token}
			var err error
			switch mode {
			case "record":
				options.Record = name
				_, err = kernel.Submit(f.State(), options)
			case "artifact":
				options.Artifact = name
				_, err = kernel.Submit(f.State(), options)
			case "TMPDIR":
				t.Setenv("TMPDIR", name)
				_, err = kernel.KernelTempDir(f.Root)
			}
			if err == nil || !strings.Contains(err.Error(), `\x1b`) || !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsafe path or lost cause: %v", err)
			}
			var failure *termsafe.Failure
			if !errors.As(err, &failure) || len(strings.Split(err.Error(), "\n")) != 2 || strings.Count(err.Error(), "\nrecovery:") != 1 || strings.ContainsAny(err.Error(), "\x1b\r") {
				t.Fatalf("wanted one escaped failure with one recovery: %q", err.Error())
			}
			if strings.Contains(err.Error(), `\\x1b`) {
				t.Fatalf("control byte escaped twice: %q", err.Error())
			}
			if !jsonEqual(before, actorRecord(t, f, id)) {
				t.Fatal("refused path changed the step")
			}
		})
	}
}

func TestS134Store_TempWorktrees(t *testing.T) {
	for _, mode := range []string{"linked", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
			parent := t.TempDir()
			wt := filepath.Join(parent, "linked")
			reducerGit(t, f, "worktree", "add", "--detach", wt, "HEAD")
			base := wt
			want := "outside every worktree"
			if mode == "unreadable" {
				f.Must(os.Chmod(parent, 0))
				t.Cleanup(func() { f.Must(os.Chmod(parent, 0700)) })
				base = t.TempDir()
				want = "listed worktree"
			}
			t.Setenv("TMPDIR", base)
			_, err := kernel.KernelTempDir(f.Root)
			if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("unsafe listed worktree temp path accepted or wrong refusal: %v", err)
			}
		})
	}
}

func TestS134Submit_RecordPathRecovery(t *testing.T) {
	for _, mode := range []string{"missing", "unreadable", "listed-worktree"} {
		t.Run(mode, func(t *testing.T) {
			f, id, lease, data := recordFixture(t, "lens")
			path := recordFile(t, data)
			want := "missing or unreadable"
			recovery := "give --record an existing readable regular file outside every worktree"
			switch mode {
			case "missing":
				path += "-missing"
			case "unreadable":
				want = "record is unreadable"
				f.Must(os.Chmod(path, 0))
				t.Cleanup(func() { f.Must(os.Chmod(path, 0600)) })
			case "listed-worktree":
				parent := t.TempDir()
				reducerGit(t, f, "worktree", "add", "--detach", filepath.Join(parent, "tree"), "HEAD")
				f.Must(os.Chmod(parent, 0))
				t.Cleanup(func() { f.Must(os.Chmod(parent, 0700)) })
				want = "listed worktree"
				recovery = "restore access to the listed worktree"
			}
			before := actorRecord(t, f, id)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path})
			if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), recovery) {
				t.Fatalf("wrong path recovery: %v", err)
			}
			if !jsonEqual(before, actorRecord(t, f, id)) {
				t.Fatal("unreadable record changed step")
			}
		})
	}
}

// The projected record fits. Only the receipt and closing pin exceed the bound.
func TestS134Submit_FinalMetadataBound(t *testing.T) {
	f, id, lease, data := recordFixture(t, "verify")
	f.Must(f.Store.WriteMetadata(id, map[string]any{"pad": ""}))
	projected := map[string]any{}
	for k, v := range actorRecord(t, f, id).Metadata {
		projected[k] = v
	}
	projected[kernel.KeyRecord] = data
	raw, err := json.Marshal(projected)
	f.Must(err)
	pad := strings.Repeat("p", kernel.MetadataLimit-len(raw)-100)
	f.Must(f.Store.WriteMetadata(id, map[string]any{"pad": pad}))
	projected["pad"] = pad
	if _, err := kernel.CheckMetadataSize(id, projected); err != nil {
		t.Fatalf("fixture exceeds projection bound: %v", err)
	}
	before := actorRecord(t, f, id)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, data)})
	if err == nil || !strings.Contains(err.Error(), "metadata-too-large") {
		t.Fatalf("final metadata bound did not refuse: %v", err)
	}
	if !jsonEqual(before, actorRecord(t, f, id)) {
		t.Fatal("final size refusal wrote the step or mindspec_closing")
	}
}
