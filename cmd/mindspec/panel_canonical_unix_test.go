//go:build !windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// Join a blocked old reader before failing. Restore the path first so that a
// later read in the same operation cannot block on the same pipe again.
func canonicalReadWithin[T any](t *testing.T, path string, raw []byte, read func() T) T {
	t.Helper()
	done := make(chan T, 1)
	go func() { done <- read() }()
	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		if err := os.Rename(path, path+".fifo"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		pipe, err := os.OpenFile(path+".fifo", os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = pipe.Write(raw)
		_ = pipe.Close()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("canonical verdict reader did not stop")
		}
		t.Fatal("canonical verdict read blocked on a FIFO")
		var zero T
		return zero
	}
}

func unsafeCanonicalFixture(t *testing.T, kind string) (cliV2Fixture, *panel.Result, string, []byte) {
	t.Helper()
	f := newCLIV2(t)
	file := f.file(t, f.verdict())
	if out, err := runPanelVerbCmd(f.args(file)...); err != nil {
		t.Fatalf("submit: %v %s", err, out)
	}
	peer := f.verdict()
	peer.Slot = "R2"
	f.submit(t, peer)
	res, err := panel.Tally(f.dir)
	if err != nil || !res.PassRule(true) {
		t.Fatalf("final baseline must pass: %+v %v", res, err)
	}
	path := filepath.Join(f.dir, "R1-round-1.json")
	raw := slotRead(t, path)
	if err := os.Rename(path, path+".saved"); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "fifo":
		err = syscall.Mkfifo(path, 0o600)
	case "symlink":
		err = os.Symlink(path+".saved", path)
	case "directory":
		err = os.Mkdir(path, 0o700)
	case "oversize":
		err = os.WriteFile(path, append(raw, []byte(strings.Repeat(" ", panel.MaxVerdictBytes))...), 0o600)
	default:
		t.Fatalf("unknown fixture kind %s", kind)
	}
	if err != nil {
		t.Fatal(err)
	}
	return f, res, path, raw
}

func requireDamagedCanonicalRoster(t *testing.T, slots []panel.SlotStatus, err error) {
	t.Helper()
	if err != nil || len(slots) != 2 {
		t.Fatalf("unsafe verdict lost the roster: %+v %v", slots, err)
	}
	if slots[0].Slot != "R1" || slots[0].Status != "damaged" ||
		!strings.Contains(strings.Join(slots[0].Annotations, " "), "verdict read failed:") ||
		slots[1].Slot != "R2" || slots[1].Status != "final" || len(slots[1].Annotations) != 0 {
		t.Fatalf("read error must affect only its slot: %+v", slots)
	}
}

func TestPanelVerify_CanonicalFIFO(t *testing.T) {
	_, _, path, raw := unsafeCanonicalFixture(t, "fifo")
	type result struct {
		out string
		err error
	}
	got := canonicalReadWithin(t, path, raw, func() result {
		out, err := runPanelVerbCmd("verify", "p", "--json")
		return result{out, err}
	})
	if got.err == nil {
		t.Fatal("damaged verdict must make verify exit nonzero")
	}
	var slots []panel.SlotStatus
	if err := json.Unmarshal([]byte(got.out), &slots); err != nil {
		t.Fatalf("verify must return the roster: %v %s", err, got.out)
	}
	requireDamagedCanonicalRoster(t, slots, nil)
}

func TestPanelVerify_ReadSlotStatusesUnsafeCanonical(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "directory", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			_, res, path, raw := unsafeCanonicalFixture(t, kind)
			type result struct {
				slots []panel.SlotStatus
				err   error
			}
			got := canonicalReadWithin(t, path, raw, func() result {
				slots, err := panel.ReadSlotStatuses(res, nil, time.Now())
				return result{slots, err}
			})
			requireDamagedCanonicalRoster(t, got.slots, got.err)
		})
	}
}

func TestPanelTally_UnsafeCanonical(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "directory", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			f, _, path, raw := unsafeCanonicalFixture(t, kind)
			type result struct {
				res *panel.Result
				err error
			}
			got := canonicalReadWithin(t, path, raw, func() result {
				res, err := panel.Tally(f.dir)
				return result{res, err}
			})
			if got.err != nil || got.res.InvalidCount() != 1 || got.res.Approves != 1 || got.res.PassRule(true) {
				t.Fatalf("unsafe verdict must be INVALID: %+v %v", got.res, got.err)
			}
			facts := panel.GateFacts{Reg: &panel.Registration{Dir: f.dir, Panel: f.p}, Res: got.res, HeadSHA: f.p.ReviewedHeadSHA, WorktreeAbsent: true}
			out, _ := renderPanelTally(got.res, facts, nil)
			if !strings.Contains(out, "R1: INVALID") || !strings.Contains(out, "verdict read failed:") || !strings.Contains(out, "recovery:") {
				t.Fatalf("unsafe verdict needs a clear error and recovery: %s", out)
			}
		})
	}
}
