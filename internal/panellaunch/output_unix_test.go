//go:build !windows

package panellaunch

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestAttemptReadersRefuseUnsafeFiles(t *testing.T) {
	for _, api := range []string{"Verdict", "Digests", "ReadExit", "Quota"} {
		for _, kind := range []string{"symlink", "fifo", "directory"} {
			t.Run(api+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				name := "verdict.json"
				if api == "ReadExit" {
					name = "exit.json"
				}
				if api == "Quota" {
					name = "stderr.log"
				}
				path := filepath.Join(dir, name)
				switch kind {
				case "symlink":
					target := filepath.Join(t.TempDir(), name)
					raw := "{}"
					if api == "Quota" {
						raw = "LIMIT"
					}
					if err := os.WriteFile(target, []byte(raw), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				case "fifo":
					if err := syscall.Mkfifo(path, 0o600); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				done := make(chan bool, 1)
				go func() {
					var refused bool
					switch api {
					case "Verdict":
						_, err := Verdict(dir, "file", "")
						refused = err != nil
					case "Digests":
						_, err := Digests(dir)
						refused = err != nil
					case "ReadExit":
						_, err := ReadExit(dir)
						refused = err != nil
					case "Quota":
						refused = !Quota(dir, []string{"LIMIT"})
					}
					done <- refused
				}()
				select {
				case refused := <-done:
					if !refused {
						t.Fatal("unsafe attempt output accepted")
					}
				case <-time.After(200 * time.Millisecond):
					// Release and join this probe's reader before reporting a regression.
					pipe, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0o600)
					if err != nil {
						t.Fatal(err)
					}
					_, _ = pipe.Write([]byte("{}"))
					_ = pipe.Close()
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Fatal("probe reader did not stop")
					}
					t.Fatal("attempt reader blocked on FIFO")
				}
			})
		}
	}
}
