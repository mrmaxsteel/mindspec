//go:build !windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/panellaunch"
)

func TestPanelLauncherActor(t *testing.T) {
	if os.Getenv("PANEL_CMD_ACTOR") != "1" {
		return
	}
	config.ResetCache()
	if os.Getenv("PANEL_CMD_SHORT_TIMEOUT") == "1" {
		panelLaunchProcessFn = func(in panellaunch.Request) error { in.Timeout = 2 * time.Second; return panellaunch.Run(in) }
	}
	_, err := launchRun(t, panelLaunchCmd, "run", "R1", "--replace")
	if err != nil {
		t.Fatal(err)
	}
}
func launchAwait(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("bounded launch wait expired")
}
func (f launchFixture) external(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPanelLauncherActor$")
	cmd.Dir = f.root
	cmd.Env = append(os.Environ(), "PANEL_CMD_ACTOR=1")
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		lock, e := panel.AcquireSlotLock(f.layout(t).Lock)
		if e == nil {
			defer lock.Release()
			_ = checkProcessOrphans(f.root, panel.Registration{Dir: f.dir, Panel: f.p}, f.layout(t))
		}
	})
	launchAwait(t, func() bool { _, e := os.Stat(filepath.Join(f.layout(t).Dir, "marker")); return e == nil })
	return cmd
}
func (f launchFixture) stopLauncher(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if e := cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	launchAwait(t, func() bool { held, e := panel.ProbeSlotLock(f.layout(t).Lock); return e == nil && !held })
}

func TestPanelLaunch_LedgerPgidOnly(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			f := newLaunchFixture(t)
			f.runner(t, "touch marker\nsleep 20", 0)
			cmd := f.external(t)
			events := f.events(t)
			pid := events[0].PGID
			// The supervisor waits for its sleeping child with run.lock held.

			f.stopLauncher(t, cmd)
			pgid := filepath.Join(f.layout(t).AttemptDir(1), "pgid")
			if changed {
				slotWrite(t, pgid, strconv.Itoa(syscall.Getpgrp()))
			} else {
				if e := os.Rename(pgid, pgid+".quarantine"); e != nil {
					t.Fatal(e)
				}
			}
			f.runner(t, "", 0)
			if out, e := f.run(t, true); e != nil {
				t.Fatalf("ledger pgid %v %s", e, out)
			}
			f.status(t, "final")
			events = f.events(t)
			if len(events) != 4 || events[1].Status != "launch_failed" || events[1].PGID != pid || events[2].Attempt != 2 {
				t.Fatalf("orphan outcome=%+v", events)
			}
			if held, e := panel.ProbeSlotLock(filepath.Join(f.layout(t).AttemptDir(1), "run.lock")); e != nil || held {
				t.Fatalf("old supervisor lock held: %v %v", held, e)
			}
		})
	}
}
func TestPanelLaunch_OrphanOutcomeDigests(t *testing.T) {
	f := newLaunchFixture(t)
	f.configure(t, "file", "", "")
	f.runner(t, "touch marker\nsleep 0.2", 1)
	cmd := f.external(t)
	f.stopLauncher(t, cmd)
	dir := f.layout(t).AttemptDir(1)
	launchAwait(t, func() bool { _, e := os.Stat(filepath.Join(dir, "exit.json")); return e == nil })
	// A missing brief stops the next launch after the orphan check and before
	// another started event. Recover must then bind the original dead attempt.
	brief := filepath.Join(f.dir, "BRIEF.md")
	if e := os.Rename(brief, brief+".quarantine"); e != nil {
		t.Fatal(e)
	}
	if _, e := f.run(t, false); e == nil || !strings.Contains(e.Error(), "brief") {
		t.Fatalf("next launch did not stop after orphan check: %v", e)
	}
	if e := os.Rename(brief+".quarantine", brief); e != nil {
		t.Fatal(e)
	}
	events := f.events(t)
	if len(events) != 2 || events[1].Status != "launch_failed" || events[1].ExitCode == nil || *events[1].ExitCode != 1 || events[1].StdoutSHA256 == "" || events[1].StderrSHA256 == "" || events[1].VerdictSHA256 == "" {
		t.Fatalf("unbound orphan output=%+v", events)
	}
	if out, e := launchRun(t, panelRecoverCmd, "run", "R1"); e != nil {
		t.Fatalf("orphan recover %v %s", e, out)
	}
	f.status(t, "final")
}
func TestPanelLaunch_VerifyLoopNoRefusal(t *testing.T) {
	f := newLaunchFixture(t)
	unrelated := exec.Command("/bin/sleep", "30")
	unrelated.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if e := unrelated.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() }()
	start := panel.LaunchRecord{Record: panel.RecordLaunch, Spec: panel.RecordSpec(f.p), Panel: "run", Slot: "R1", Round: 1, Attempt: 1, Family: "a", Model: "m", Status: "started", RunnerKind: "process", RunnerSource: "config.local.yaml", PGID: unrelated.Process.Pid, ArgvSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64), CreatedAt: paneltest.CreatedAt}
	paneltest.Seed(t, f.dir, start)
	dir := f.layout(t).AttemptDir(1)
	if e := os.MkdirAll(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	stop := make(chan struct{})
	done := make(chan error, 1)
	var verifies atomic.Int64
	go func() {
		for {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			res, e := panel.Tally(f.dir)
			if e == nil {
				_, e = panelSlotStatuses(f.root, res)
			}
			if e != nil {
				done <- e
				return
			}
			verifies.Add(1)
		}
	}()
	defer func() {
		close(stop)
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	for i := 0; i < 20; i++ {
		if out, e := f.run(t, true); e != nil {
			t.Fatalf("launch %d %v %s", i, e, out)
		}
	}
	if verifies.Load() < 20 {
		t.Fatal("verify loop did not overlap the launches")
	}
	if e := syscall.Kill(unrelated.Process.Pid, 0); e != nil {
		t.Fatal("stale pgid group was signalled")
	}
	f.status(t, "final")
}
func TestPanelLaunch_SetsidDescendant(t *testing.T) {
	f := newLaunchFixture(t)
	// A local child makes a new session. Its lifetime must not retain run.lock.
	f.runner(t, "'"+os.Args[0]+"' -test.run=^TestSetsidActor$ > /dev/null 2>&1 &\nwhile [ ! -e escaped ]; do sleep 0.01; done", 0)
	t.Setenv("PANEL_SETSID_ACTOR", "1")
	out, e := f.run(t, false)
	if e != nil {
		t.Fatalf("%v %s", e, out)
	}
	f.status(t, "final")
	pidRaw := slotRead(t, filepath.Join(f.layout(t).Dir, "escaped"))
	pid, _ := strconv.Atoi(string(pidRaw))
	defer syscall.Kill(pid, syscall.SIGKILL)
	if held, e := panel.ProbeSlotLock(filepath.Join(f.layout(t).AttemptDir(1), "run.lock")); e != nil || held {
		t.Fatalf("escaped descendant held lock: %v %v", held, e)
	}
	resetPanelCreateFlags(t)
	out, e = runPanelCmd("create", "run", "--gate", "adhoc", "--target", "main", "--round", "2")
	if e != nil {
		t.Fatalf("new round blocked by escaped child: %v %s", e, out)
	}
	if e := syscall.Kill(pid, 0); e != nil {
		t.Fatal("completed attempt signalled escaped process")
	}
}
func TestSetsidActor(t *testing.T) {
	if os.Getenv("PANEL_SETSID_ACTOR") != "1" {
		return
	}
	if _, e := syscall.Setsid(); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile("escaped", []byte(strconv.Itoa(os.Getpid())), 0o600); e != nil {
		t.Fatal(e)
	}
	time.Sleep(20 * time.Second)
}
func TestSlotLock_StableAcrossPrune(t *testing.T) {
	f := newLaunchFixture(t)
	layout := f.layout(t)
	if e := ensureSlotCheckout(f.root, panel.Registration{Dir: f.dir, Panel: f.p}, layout, "R1"); e != nil {
		t.Fatal(e)
	}
	lock, e := panel.AcquireSlotLock(layout.Lock)
	if e != nil {
		t.Fatal(e)
	}
	_ = lock.Release()
	paused, e := os.OpenFile(layout.Lock, os.O_RDWR, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer paused.Close()
	before, _ := paused.Stat()
	release, e := cleanupPanelCheckouts(f.root, panel.Registration{Dir: f.dir, Panel: f.p}, f.isolation, 0, "R1")
	if e != nil {
		t.Fatal(e)
	}
	release()
	after, e := os.Stat(layout.Lock)
	if e != nil || !os.SameFile(before, after) {
		t.Fatalf("prune changed lock inode: %v", e)
	}
	if e := syscall.Flock(int(paused.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		t.Fatal(e)
	}
	defer syscall.Flock(int(paused.Fd()), syscall.LOCK_UN)
	if next, e := panel.TrySlotLock(layout.Lock); e == nil {
		_ = next.Release()
		t.Fatal("paused launcher and new launcher locked different inodes")
	}
	f.runner(t, "printf x >> launch-count", 0)
	if _, e := f.run(t, false); e == nil {
		t.Fatal("second launcher passed the paused launcher's lock")
	}
	if len(f.events(t)) != 0 {
		t.Fatal("refused launcher started an attempt")
	}
	if e := syscall.Flock(int(paused.Fd()), syscall.LOCK_UN); e != nil {
		t.Fatal(e)
	}
	if out, e := f.run(t, false); e != nil {
		t.Fatalf("resumed launch %v %s", e, out)
	}
	if string(slotRead(t, filepath.Join(layout.Dir, "launch-count"))) != "x" {
		t.Fatal("more than one runner started across prune")
	}
	f.status(t, "final")
}
func TestPanelCleanup_RefusesHeldLock(t *testing.T) {
	for _, verb := range []string{"prune", "create", "abandon"} {
		t.Run(verb, func(t *testing.T) {
			f := newLaunchFixture(t)
			f.runner(t, "touch marker\nsleep 20", 0)
			_ = f.external(t)
			// Create all stable lock inodes before comparing the refusal snapshot.
			for _, slot := range []string{"R2", "fixer"} {
				layout, _ := checkoutLayout(f.root, f.isolation, panel.Registration{Dir: f.dir, Panel: f.p}, slot)
				l, e := panel.AcquireSlotLock(layout.Lock)
				if e != nil {
					t.Fatal(e)
				}
				_ = l.Release()
			}
			before := slotSnapshot(t, f.dir)
			scratch := slotSnapshot(t, filepath.Join(f.root, f.isolation))
			var out string
			var e error
			switch verb {
			case "prune":
				out, e = runPanelCmd("checkout", "run", "--prune")
			case "create":
				resetPanelCreateFlags(t)
				out, e = runPanelCmd("create", "run", "--gate", "adhoc", "--target", "main", "--round", "2")
			case "abandon":
				out, e = runPanelCmd("abandon", "run", "--reason", "test", "--by", "operator")
			}
			if e == nil || !strings.Contains(e.Error()+out, "R1") {
				t.Fatalf("cleanup did not name held slot: %v %s", e, out)
			}
			if !reflect.DeepEqual(before, slotSnapshot(t, f.dir)) || !reflect.DeepEqual(scratch, slotSnapshot(t, filepath.Join(f.root, f.isolation))) {
				t.Fatal("refused cleanup changed files")
			}
		})
	}
}
func TestPanelLaunch_LiveVerification(t *testing.T) {
	f := newLaunchFixture(t)
	if _, e := f.run(t, false); e != nil {
		t.Fatal(e)
	}
	f.runner(t, "touch marker\nsleep 20", 1)
	cmd := f.external(t)
	start := time.Now()
	f.status(t, "running")
	if time.Since(start) > time.Second {
		t.Fatal("verify waited for launch")
	}
	// Submit to another slot while the first slot's reviewer holds its lock.
	v := f.v
	v.Slot = "R2"
	v.Rationale = "second"
	if _, e := panel.Submit(panel.SubmitTarget{Dir: f.dir, Panel: "run", Slot: "R2", Round: 1}, paneltest.Bytes(t, v), paneltest.Resolve); e != nil {
		t.Fatal(e)
	}
	f.stopLauncher(t, cmd)
	f.status(t, "running")
	lock, e := panel.AcquireSlotLock(f.layout(t).Lock)
	if e != nil {
		t.Fatal(e)
	}
	e = checkProcessOrphans(f.root, panel.Registration{Dir: f.dir, Panel: f.p}, f.layout(t))
	_ = lock.Release()
	if e != nil {
		t.Fatal(e)
	}
	f.status(t, "launch_failed")
	if held, e := panel.ProbeSlotLock(filepath.Join(f.layout(t).AttemptDir(2), "run.lock")); e != nil || held {
		t.Fatalf("orphan run lock held: %v %v", held, e)
	}
	if len(f.events(t)) != 4 {
		t.Fatal("orphan outcome not recorded")
	}
}

func TestPanelVerify_RealKilledLaunchLifecycle(t *testing.T) {
	f := newLaunchFixture(t)
	f.runner(t, "touch marker\nsleep 20", 0)
	t.Setenv("PANEL_CMD_SHORT_TIMEOUT", "1")
	cmd := f.external(t)
	f.stopLauncher(t, cmd)
	f.status(t, "running")
	for _, want := range []struct {
		status string
		code   int
	}{{"running", 1}, {"launch_failed", 2}} {
		if want.status == "launch_failed" {
			launchAwait(t, func() bool {
				held, e := panel.ProbeSlotLock(filepath.Join(f.layout(t).AttemptDir(1), "run.lock"))
				return e == nil && !held
			})
		}
		_ = panelVerifyCmd.Flags().Set("json", "false")
		_ = panelVerifyCmd.Flags().Set("spec", "")
		var out bytes.Buffer
		panelVerifyCmd.SetOut(&out)
		err := panelVerifyCmd.RunE(panelVerifyCmd, []string{"run"})
		panelVerifyCmd.SetOut(nil)
		if otelExitCode(err) != want.code || !strings.Contains(out.String(), "R1 "+want.status+" ") {
			t.Fatalf("verify wanted %s exit %d, got %v %s", want.status, want.code, err, out.String())
		}
	}
	if len(f.events(t)) != 1 {
		t.Fatal("verify appended an orphan outcome")
	}
}

func TestPanelLaunch_KilledOrphanCleanup(t *testing.T) {
	f := newLaunchFixture(t)
	f.runner(t, "touch marker\nsleep 20", 0)
	cmd := f.external(t)
	f.stopLauncher(t, cmd)
	resetPanelCreateFlags(t)
	out, e := runPanelCmd("create", "run", "--gate", "adhoc", "--target", "main", "--round", "2")
	if e != nil {
		t.Fatalf("orphan cleanup: %v %s", e, out)
	}
	if _, e := os.Stat(f.layout(t).Dir); !os.IsNotExist(e) {
		t.Fatal("prior-round scratch remains")
	}
	events, e := panel.SlotAttempts(f.dir, 1)
	if e != nil || len(events["R1"]) != 2 || events["R1"][1].Status != "launch_failed" {
		t.Fatalf("orphan not closed before new round: %+v %v", events, e)
	}
}
