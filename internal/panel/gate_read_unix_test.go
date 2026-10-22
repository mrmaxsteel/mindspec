//go:build !windows

package panel

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestGate_PreV2FIFOVerdict(t *testing.T) {
	for _, roster := range []bool{true, false} {
		name := "off roster"
		if roster {
			name = "on roster"
		}
		t.Run(name, func(t *testing.T) {
			target, _, _ := submitFixtureSchema(t, 0)
			createWrite(t, target.Dir, "R2-round-1.json", `{"verdict":"APPROVE"}`)
			slot := "R1"
			if !roster {
				slot = "outsider"
				createWrite(t, target.Dir, "R1-round-1.json", `{"verdict":"APPROVE"}`)
			}
			path := filepath.Join(target.Dir, slot+"-round-1.json")
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{}, 1)
			go func() {
				defer func() { done <- struct{}{} }()
				r, err := Tally(target.Dir)
				if err != nil {
					t.Error(err)
					return
				}
				wantMissing := 0
				if roster {
					wantMissing = 1
				}
				if r.MissingCount() != wantMissing || r.InvalidCount() != 0 || len(r.Unexpected) != 0 || !reflect.DeepEqual(r.Malformed, []string{slot + "-round-1.json"}) || r.PassRule(true) == roster {
					t.Errorf("FIFO must follow historical malformed rules: %+v", r)
				}
				slots, err := ReadSlotStatuses(r, nil, time.Now())
				want := "final"
				if roster {
					want = "malformed"
				}
				if err != nil || len(slots) != 2 || slots[0].Status != want {
					t.Errorf("historical status: %+v %v", slots, err)
				}
				prior, err := PriorRoundItems(target.Dir, slot, 2)
				if err != nil || len(prior.Items) != 0 || prior.Warning != "" {
					t.Errorf("historical unreadable prior items: %+v %v", prior, err)
				}
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				// Unblock an old reader, then join it before failing.
				if err := os.Rename(path, path+".fifo"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
					t.Fatal(err)
				}
				pipe, err := os.OpenFile(path+".fifo", os.O_RDWR|syscall.O_NONBLOCK, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = pipe.Write([]byte(`{}`))
				_ = pipe.Close()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("reader did not stop")
				}
				t.Fatal("historical verdict reader blocked on FIFO")
			}
		})
	}
}
