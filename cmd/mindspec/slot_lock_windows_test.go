//go:build windows

package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func windowsNoSlotLocks(t *testing.T, f slotFixture) {
	t.Helper()
	err := filepath.WalkDir(filepath.Join(f.root, f.isolation, ".locks"), func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			t.Errorf("slot lock remains after verb: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSlotLock_WindowsLifecycle(t *testing.T) {
	f := newDispatchFixture(t)
	l := f.layout(t, "R1")
	// Get a known exited pid with a local child of this test executable.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe, "-test.list=^$")
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("local pid fixture: %v %s", err, out)
	}
	deadPID := child.ProcessState.Pid()

	for cycle := 0; cycle < 20; cycle++ {
		t.Run(fmt.Sprintf("cycle-%02d", cycle), func(t *testing.T) {
			// A dead holder must not block checkout. Its file must be replaced.
			slotWrite(t, l.Lock, fmt.Sprintln(deadPID))
			held, err := panel.ProbeSlotLock(l.Lock)
			if err != nil || held {
				t.Fatalf("stale lock probe: held=%t err=%v", held, err)
			}
			args := []string{"--slot", "R1", "--dispatch"}
			if cycle > 0 {
				args = append(args, "--replace")
			}
			checkoutOK(t, args...)
			windowsNoSlotLocks(t, f)
			dispatchVerify(t, f, "running")
			windowsNoSlotLocks(t, f)
			attempts, err := panel.SlotAttempts(f.dir, 1)
			if err != nil {
				t.Fatal(err)
			}
			k := attempts["R1"][len(attempts["R1"])-1].Attempt
			file := dispatchVerdictFile(t, f, k)
			if out, err := dispatchSubmit(t, file); err != nil {
				t.Fatalf("submit: %v %s", err, out)
			}
			windowsNoSlotLocks(t, f)
			dispatchVerify(t, f, "final")
			windowsNoSlotLocks(t, f)
			checkoutOK(t, "--slot", "R1", "--dispatch", "--replace")
			windowsNoSlotLocks(t, f)
			dispatchVerify(t, f, "running")
			windowsNoSlotLocks(t, f)
			checkoutOK(t, "--slot", "R1", "--prune")
			windowsNoSlotLocks(t, f)
			if _, err := os.Stat(l.Dir); !os.IsNotExist(err) {
				t.Fatalf("prune retained slot: %v", err)
			}
		})
	}

	t.Run("empty and malformed locks stay held", func(t *testing.T) {
		for _, contents := range []string{"", "not-a-pid\n"} {
			slotWrite(t, l.Lock, contents)
			held, err := panel.ProbeSlotLock(l.Lock)
			if err != nil || !held {
				t.Fatalf("invalid lock probe: held=%t err=%v", held, err)
			}
			lock, err := panel.TrySlotLock(l.Lock)
			if err == nil {
				lock.Release()
				t.Fatal("invalid lock was acquired")
			}
			if got := string(slotRead(t, l.Lock)); got != contents {
				t.Fatal("invalid lock changed")
			}
			if err := os.Rename(l.Lock, filepath.Join(t.TempDir(), "held-lock")); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("two contenders for one stale lock", func(t *testing.T) {
		for round := 0; round < 20; round++ {
			slotWrite(t, l.Lock, fmt.Sprintln(deadPID))
			start := make(chan struct{})
			type result struct {
				lock *panel.SlotLock
				err  error
			}
			results := make(chan result, 2)
			for i := 0; i < 2; i++ {
				go func() { <-start; lock, err := panel.TrySlotLock(l.Lock); results <- result{lock, err} }()
			}
			close(start)
			first, second := <-results, <-results
			// Neither contender releases before both have tried to acquire.
			holders := 0
			for _, r := range []result{first, second} {
				if r.err == nil {
					holders++
					if r.lock == nil {
						t.Fatal("nil lock holder")
					}
				}
			}
			for _, r := range []result{first, second} {
				if r.lock != nil {
					if err := r.lock.Release(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if holders != 1 {
				t.Fatalf("%d contenders acquired stale lock, want exactly one: %v / %v", holders, first.err, second.err)
			}
			windowsNoSlotLocks(t, f)
		}
	})
}
