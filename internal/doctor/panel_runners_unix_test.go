//go:build !windows

package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func TestDoctor_StaleCheckoutsUnsafeMetadata(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "directory", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			cfg := config.DefaultConfig()
			layout := doctorSlot(t, root, "adhoc", "p", 1)
			// The scan must still report a later, valid stale checkout.
			peer := doctorSlot(t, root, "adhoc", "z", 1)
			if err := os.Rename(layout.Metadata, layout.Metadata+".saved"); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "fifo":
				err = syscall.Mkfifo(layout.Metadata, 0o600)
			case "symlink":
				err = os.Symlink(layout.Metadata+".saved", layout.Metadata)
			case "directory":
				err = os.Mkdir(layout.Metadata, 0o700)
			case "oversize":
				err = os.WriteFile(layout.Metadata, []byte(strings.Repeat(" ", (64<<10)+1)), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan *Report, 1)
			go func() {
				r := &Report{}
				checkStaleCheckouts(r, root, cfg)
				done <- r
			}()
			select {
			case r := <-done:
				requireDoctorCheck(t, r, "Panel checkouts", Warn, layout.Metadata)
				requireDoctorCheck(t, r, "Panel checkouts", Warn, "recovery:")
				requireDoctorCheck(t, r, "Stale panel checkout", Warn, peer.Dir)
				if len(doctorChecks(r, "Panel checkouts")) != 1 || len(doctorChecks(r, "Stale panel checkout")) != 1 {
					t.Fatalf("unsafe metadata must warn once and scanning must continue: %+v", r.Checks)
				}
			case <-time.After(5 * time.Second):
				pipe, err := os.OpenFile(layout.Metadata, os.O_RDWR|syscall.O_NONBLOCK, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = pipe.Write([]byte("{}"))
				_ = pipe.Close()
				select {
				case <-done:
				case <-time.After(30 * time.Second):
					t.Fatal("doctor metadata reader did not stop")
				}
				t.Fatalf("doctor blocked on FIFO %s", filepath.Base(layout.Metadata))
			}
		})
	}
}

func TestDoctor_LocalConfigFIFO(t *testing.T) {
	for _, operation := range []string{"load", "doctor"} {
		t.Run(operation, func(t *testing.T) {
			root := doctorRunnerRoot(t, "")
			path := filepath.Join(root, localConfigPath)
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{}, 1)
			go func() {
				defer func() { done <- struct{}{} }()
				if operation == "load" {
					_, err := config.Load(root)
					if err == nil || !strings.Contains(err.Error(), "config.local.yaml") || !strings.HasSuffix(err.Error(), "recovery: set only panel.runners and models in .mindspec/config.local.yaml") {
						t.Errorf("FIFO needs a local config load error and recovery: %v", err)
					}
				} else {
					r := doctorConfigReport(root, Options{})
					for _, c := range doctorChecks(r, "MindSpec config") {
						if c.Status == Error && strings.Contains(c.Message, "config.local.yaml") && strings.Contains(c.Message, "recovery: set only panel.runners and models") {
							return
						}
					}
					t.Errorf("doctor must report local config load Error: %+v", r.Checks)
				}
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				if err := os.Rename(path, path+".fifo"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
				pipe, err := os.OpenFile(path+".fifo", os.O_RDWR|syscall.O_NONBLOCK, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = pipe.Write([]byte("{}\n"))
				_ = pipe.Close()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("local config reader did not stop")
				}
				t.Fatal(operation + " blocked on config.local.yaml FIFO")
			}
		})
	}
}
