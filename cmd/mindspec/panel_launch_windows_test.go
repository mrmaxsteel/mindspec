//go:build windows

package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestSlotLock_WindowsLifecycle(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	for i := 0; i < 20; i++ {
		checkoutOK(t, "--slot", "R1", "--dispatch", "--replace")
		layout := f.layout(t, "R1")
		res, e := panel.Tally(f.dir)
		if e != nil {
			t.Fatal(e)
		}
		slots, e := panelSlotStatuses(f.root, res)
		if e != nil || slots[0].Status != "running" {
			t.Fatalf("dispatch status=%+v %v", slots, e)
		}
		if _, e := os.Stat(layout.Lock); !os.IsNotExist(e) {
			t.Fatal("lock file stays after checkout or verify")
		}
		k, e := panel.NextAttemptNumber(f.dir, 1, "R1", layout.Dir)
		if e != nil {
			t.Fatal(e)
		}
		v := paneltest.Verdict("APPROVE")
		v.ReviewedSHA = f.p.ReviewedHeadSHA
		path := filepath.Join(layout.AttemptDir(k-1), "verdict.json")
		slotWrite(t, path, string(paneltest.Bytes(t, v)))
		if out, e := runPanelVerbCmd("submit", "--file", path); e != nil {
			t.Fatalf("submit %v %s", e, out)
		}
		res, e = panel.Tally(f.dir)
		if e != nil {
			t.Fatal(e)
		}
		slots, e = panelSlotStatuses(f.root, res)
		if e != nil || slots[0].Status != "final" {
			t.Fatalf("submit status=%+v %v", slots, e)
		}
		if _, e := os.Stat(layout.Lock); !os.IsNotExist(e) {
			t.Fatal("lock file stays after submit")
		}
		checkoutOK(t, "--prune", "--slot", "R1")
		if _, e := os.Stat(layout.Lock); !os.IsNotExist(e) {
			t.Fatal("lock file stays after prune")
		}
	}
	layout := f.layout(t, "R1")
	slotWrite(t, layout.Lock, "2147483647\n")
	if held, e := panel.ProbeSlotLock(layout.Lock); e != nil || held {
		t.Fatalf("dead pid counts as held: %v %v", held, e)
	}
	lock, e := panel.AcquireSlotLock(layout.Lock)
	if e != nil {
		t.Fatal(e)
	}
	_ = lock.Release()
	if _, e := os.Stat(layout.Lock); !os.IsNotExist(e) {
		t.Fatal("stale file stays after release")
	}
	slotWrite(t, layout.Lock, "")
	if held, e := panel.ProbeSlotLock(layout.Lock); e != nil || !held {
		t.Fatalf("empty file counts as free: %v %v", held, e)
	}
	if e := os.Rename(layout.Lock, layout.Lock+".empty"); e != nil {
		t.Fatal(e)
	}
	slotWrite(t, layout.Lock, "2147483647\n")
	var wait sync.WaitGroup
	results := make(chan *panel.SlotLock, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() { defer wait.Done(); l, _ := panel.TrySlotLock(layout.Lock); results <- l }()
	}
	wait.Wait()
	close(results)
	wins := 0
	for l := range results {
		if l != nil {
			wins++
			defer l.Release()
		}
	}
	if wins != 1 {
		t.Fatalf("stale-file race holders=%d, want 1", wins)
	}
}
