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
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/panellaunch"
)

func TestPanelSubmit_UnsafeSlotMetadata(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newCLIV2(t)
			file := f.shortSetup(t, 1)
			path := filepath.Join(f.layout.Dir, "slot.json")
			raw := slotRead(t, path)
			if err := os.Rename(path, path+".saved"); err != nil {
				t.Fatal(err)
			}
			if kind == "fifo" {
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(path+".saved", path); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := runPanelVerbCmd("submit", "--file", file); done <- err }()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "invalid slot directory") || !strings.Contains(err.Error(), "recovery:") {
					t.Fatalf("unsafe metadata needs recovery: %v", err)
				}
			case <-time.After(300 * time.Millisecond):
				pipe, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = pipe.Write(raw)
				_ = pipe.Close()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("submit reader did not stop")
				}
				t.Fatal("absolute file submit blocked on FIFO metadata")
			}
			latest, err := panel.LatestSubmissions(f.dir, 1)
			if err != nil || latest["R1"].Seq != 0 {
				t.Fatalf("unsafe metadata created a submission: %+v %v", latest, err)
			}
		})
	}
}

func TestPanelVerify_UnsafeRunLockKeepsFinalRoster(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			f := newLaunchFixture(t)
			if out, err := f.run(t, false); err != nil {
				t.Fatalf("launch: %v %s", err, out)
			}
			path := filepath.Join(f.layout(t).AttemptDir(1), "run.lock")
			if err := os.Rename(path, path+".saved"); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "fifo":
				err = syscall.Mkfifo(path, 0o600)
			case "symlink":
				err = os.Symlink(path+".saved", path)
			case "directory":
				err = os.Mkdir(path, 0o700)
			}
			if err != nil {
				t.Fatal(err)
			}
			resetV2Flags(t)
			out, err := runPanelVerbCmd("verify", "run", "--json")
			if err != nil {
				t.Fatalf("unsafe run lock broke verify: %v %s", err, out)
			}
			var slots []panel.SlotStatus
			if err := json.Unmarshal([]byte(out), &slots); err != nil {
				t.Fatal(err)
			}
			if len(slots) != 2 {
				t.Fatalf("roster lost slots: %s", out)
			}
			for _, slot := range slots {
				if slot.Status != "final" {
					t.Fatalf("final status lost: %s", out)
				}
				annotated := strings.Contains(strings.Join(slot.Annotations, " "), "run.lock")
				if annotated != (slot.Slot == "R1") {
					t.Fatalf("lock error must stay on its slot: %s", out)
				}
			}
		})
	}
}

func TestPanelVerify_UnsafeRunLockIsNotLive(t *testing.T) {
	f := newLaunchFixture(t)
	if out, err := f.run(t, false); err != nil {
		t.Fatalf("launch: %v %s", err, out)
	}
	start := f.events(t)[0].LaunchRecord
	start.Attempt = 2
	paneltest.Seed(t, f.dir, start)
	dir := f.layout(t).AttemptDir(2)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "run.lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := panel.Tally(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	slots, err := panelSlotStatuses(f.root, res)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 2 || slots[0].Status != "launch_failed" || slots[1].Status != "final" ||
		!strings.Contains(strings.Join(slots[0].Annotations, " "), "run.lock") {
		t.Fatalf("unsafe lock counted as live or lost its peer: %+v", slots)
	}
}

func TestPanelLaunch_UnsafeOutputClassification(t *testing.T) {
	for _, tc := range []struct {
		name          string
		code          int
		quota, orphan bool
		want          string
	}{
		{"zero", 0, false, false, "malformed"},
		{"nonzero", 7, false, false, "launch_failed"},
		{"quota", 7, true, false, "quota_wall"},
		{"orphan-zero", 0, false, true, "launch_failed"},
		{"orphan-nonzero", 7, false, true, "launch_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLaunchFixture(t)
			f.configure(t, "file", "LIMIT", "")
			prefix := ""
			if tc.quota {
				prefix = "printf LIMIT >&2"
			}
			f.runner(t, prefix, tc.code)
			original := panelLaunchProcessFn
			t.Cleanup(func() { panelLaunchProcessFn = original })
			panelLaunchProcessFn = func(in panellaunch.Request) error {
				if err := panellaunch.Run(in); err != nil {
					return err
				}
				path := filepath.Join(in.Dir, "verdict.json")
				if err := os.Rename(path, path+".saved"); err != nil {
					return err
				}
				if err := os.Symlink(path+".saved", path); err != nil {
					return err
				}
				if tc.orphan {
					return os.ErrClosed
				}
				return nil
			}
			if out, err := f.run(t, false); err == nil {
				t.Fatalf("unsafe output counted as final: %s", out)
			}
			if tc.orphan {
				if err := checkProcessOrphans(f.root, panel.Registration{Dir: f.dir, Panel: f.p}, f.layout(t)); err != nil {
					t.Fatal(err)
				}
			}
			events := f.events(t)
			count := 2
			if tc.want == "malformed" {
				count = 4
			}
			if len(events) != count || events[len(events)-1].Status != tc.want {
				t.Fatalf("exit %d orphan=%v: want %s with %d events, got %+v", tc.code, tc.orphan, tc.want, count, events)
			}
			outcome := events[len(events)-1]
			if outcome.ExitCode == nil || *outcome.ExitCode != tc.code {
				t.Fatalf("outcome lost exit code %d: %+v", tc.code, outcome)
			}
			if f.latest(t).Seq != 0 {
				t.Fatal("unsafe output created a submission")
			}
			f.status(t, tc.want)
		})
	}
}

func TestPanelSubmit_OversizeInputParity(t *testing.T) {
	var messages []string
	for _, stdin := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "stdin"}[stdin], func(t *testing.T) {
			f := newCLIV2(t)
			file := f.file(t, f.verdict())
			data := append(paneltest.Bytes(t, f.verdict()), []byte(strings.Repeat(" ", panel.MaxVerdictBytes))...)
			if err := os.WriteFile(file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if stdin {
				file = "-"
				rootCmd.SetIn(strings.NewReader(string(data)))
				t.Cleanup(func() { rootCmd.SetIn(nil) })
			}
			_, err := runPanelVerbCmd(f.args(file)...)
			if err == nil || !strings.Contains(err.Error(), "/: file exceeds 1 MiB") || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("oversize input needs validator pointer and recovery: %v", err)
			}
			messages = append(messages, err.Error())
		})
	}
	if len(messages) == 2 && messages[0] != messages[1] {
		t.Fatalf("file and stdin messages differ: %q", messages)
	}
}
