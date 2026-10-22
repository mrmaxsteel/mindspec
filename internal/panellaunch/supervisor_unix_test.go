//go:build !windows

package panellaunch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// TestLaunchActor is a local test executable, never a model runner.
func TestLaunchActor(t *testing.T) {
	if os.Getenv("PANEL_LAUNCH_ACTOR") != "1" {
		return
	}
	dir := os.Getenv("PANEL_LAUNCH_DIR")
	in := Request{Dir: dir, SlotDir: dir, Argv: []string{"/bin/sh", "-c", os.Getenv("PANEL_LAUNCH_SCRIPT")}, Prompt: "stdin", Timeout: 3 * time.Second, Started: func(pid int) error {
		if err := os.WriteFile(filepath.Join(dir, "started"), []byte(strconv.Itoa(pid)), 0o600); err != nil {
			return err
		}
		if os.Getenv("PANEL_LAUNCH_PAUSE") == "1" {
			_ = syscall.Kill(os.Getpid(), syscall.SIGSTOP)
		}
		return nil
	}}
	if os.Getenv("PANEL_SUPERVISOR_PAUSE") == "1" {
		in.SupervisorCommand = []string{os.Args[0], "-test.run=^TestSupervisorActor$", "--"}
	}
	if err := Run(in); err != nil {
		t.Fatal(err)
	}
}
func TestSupervisorActor(t *testing.T) {
	if os.Getenv("PANEL_SUPERVISOR_PAUSE") != "1" {
		return
	}
	dir := os.Args[len(os.Args)-1]
	code := supervise(dir, os.NewFile(3, "run.lock"), os.NewFile(4, "start"), func() {
		_ = os.WriteFile(filepath.Join(dir, "before-pgid"), []byte(strconv.Itoa(os.Getpid())), 0o600)
		for {
			if _, err := os.Stat(filepath.Join(dir, "resume")); err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
	os.Exit(code)
}

func await(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("bounded process wait expired")
}
func readPID(t *testing.T, path string) int {
	t.Helper()
	var pid int
	await(t, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		value, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil || value <= 1 {
			return false
		}
		pid = value
		return true
	})
	return pid
}
func runActor(t *testing.T, script string, pauseLaunch, pauseSupervisor bool) (string, *exec.Cmd) {
	t.Helper()
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("test"), 0o600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLaunchActor$")
	cmd.Env = append(os.Environ(), "PANEL_LAUNCH_ACTOR=1", "PANEL_LAUNCH_DIR="+dir, "PANEL_LAUNCH_SCRIPT="+script, fmt.Sprintf("PANEL_LAUNCH_PAUSE=%d", boolInt(pauseLaunch)), fmt.Sprintf("PANEL_SUPERVISOR_PAUSE=%d", boolInt(pauseSupervisor)))
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if raw, e := os.ReadFile(filepath.Join(dir, "started")); e == nil {
			pid, _ := strconv.Atoi(string(raw))
			_ = StopOrphan(dir, pid)
		}
	})
	return dir, cmd
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func lockFree(dir string) bool {
	f, e := takeRunLock(filepath.Join(dir, "run.lock"))
	if e != nil {
		return false
	}
	_ = f.Close()
	return true
}
func markerStops(t *testing.T, path string) {
	t.Helper()
	before, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(80 * time.Millisecond)
	after, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	if before.Size() != after.Size() {
		t.Fatal("orphan still writes after stop")
	}
}

const grow = `while :; do printf x >> marker || exit 0; sleep 0.02; done`

func TestPanelLaunch_SupervisorTimeoutAfterLaunchDeath(t *testing.T) {
	dir, cmd := runActor(t, grow, false, false)
	_ = readPID(t, filepath.Join(dir, "started"))
	await(t, func() bool { _, e := os.Stat(filepath.Join(dir, "marker")); return e == nil })
	if e := cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	await(t, func() bool { _, e := os.Stat(filepath.Join(dir, "exit.json")); return e == nil })
	await(t, func() bool { return lockFree(dir) })
	exit, e := ReadExit(dir)
	if e != nil || !exit.Timeout {
		t.Fatalf("timeout=%+v %v", exit, e)
	}
	markerStops(t, filepath.Join(dir, "marker"))
}
func TestPanelLaunch_KillsOrphan(t *testing.T) {
	dir, cmd := runActor(t, grow, false, false)
	pid := readPID(t, filepath.Join(dir, "started"))
	await(t, func() bool { _, e := os.Stat(filepath.Join(dir, "marker")); return e == nil })
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	began := time.Now()
	if e := StopOrphan(dir, pid); e != nil {
		t.Fatal(e)
	}
	if time.Since(began) > 1800*time.Millisecond {
		t.Fatal("orphan stopped only at its timeout")
	}

	if !lockFree(dir) {
		t.Fatal("orphan lock held")
	}
	markerStops(t, filepath.Join(dir, "marker"))
}
func TestPanelLaunch_FaultBeforePgid(t *testing.T) {
	dir, cmd := runActor(t, "touch argv-ran", false, true)
	pid := readPID(t, filepath.Join(dir, "before-pgid"))
	if e := syscall.Kill(-pid, syscall.SIGKILL); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	if _, e := os.Stat(filepath.Join(dir, "pgid")); !os.IsNotExist(e) {
		t.Fatal("pgid renamed before fault")
	}
	if _, e := os.Stat(filepath.Join(dir, "argv-ran")); !os.IsNotExist(e) {
		t.Fatal("argv ran before start")
	}
	if !lockFree(dir) {
		t.Fatal("dead supervisor lock held")
	}
	if e := Run(Request{Dir: makeAttempt(t), SlotDir: dir, Argv: []string{"/bin/true"}}); e != nil {
		t.Fatal(e)
	}
}
func TestPanelLaunch_LaunchDiesBeforeStart(t *testing.T) {
	dir, cmd := runActor(t, "touch argv-ran", true, false)
	_ = readPID(t, filepath.Join(dir, "started"))
	_ = readPID(t, filepath.Join(dir, "pgid"))
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	await(t, func() bool { return lockFree(dir) })
	if _, e := os.Stat(filepath.Join(dir, "argv-ran")); !os.IsNotExist(e) {
		t.Fatal("argv ran before start pipe")
	}
	if e := Run(Request{Dir: makeAttempt(t), SlotDir: dir, Argv: []string{"/bin/true"}}); e != nil {
		t.Fatal(e)
	}
}
func TestPanelLaunch_NoEventNoPgidRefuses(t *testing.T) {
	dir, cmd := runActor(t, "touch argv-ran", true, true)
	pid := readPID(t, filepath.Join(dir, "before-pgid"))
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if e := StopOrphan(dir, 0); e == nil || !strings.Contains(e.Error(), dir) {
		t.Fatalf("missing target refusal=%v", e)
	}
	if _, e := os.Stat(filepath.Join(dir, "argv-ran")); !os.IsNotExist(e) {
		t.Fatal("argv ran")
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	await(t, func() bool { return lockFree(dir) })
}
func TestPanelLaunch_SupervisorTermKillsGroup(t *testing.T) {
	dir, cmd := runActor(t, "sleep 5 & echo $! > sleeper\n"+grow, false, false)
	pid := readPID(t, filepath.Join(dir, "started"))
	sleeper := readPID(t, filepath.Join(dir, "sleeper"))
	await(t, func() bool { _, e := os.Stat(filepath.Join(dir, "marker")); return e == nil })
	if e := syscall.Kill(pid, syscall.SIGTERM); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	await(t, func() bool { return lockFree(dir) })
	markerStops(t, filepath.Join(dir, "marker"))
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		if err := syscall.Kill(sleeper, 0); err == syscall.ESRCH {
			break
		}
		// A killed, unreaped child cannot run. Some test hosts reap it later.
		out, err := exec.Command("ps", "-p", strconv.Itoa(sleeper), "-o", "stat=").Output()
		if err != nil || strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("group descendant still runs after supervisor termination")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func makeAttempt(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("prompt\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	return dir
}
func TestPanelLaunch_ChildStdinNull(t *testing.T) {
	dir := makeAttempt(t)
	in := Request{Dir: dir, SlotDir: dir, Argv: []string{"/bin/sh", "-c", "if read x; then exit 4; fi"}, Prompt: "file", Timeout: time.Second}
	if e := Run(in); e != nil {
		t.Fatal(e)
	}
	exit, e := ReadExit(dir)
	if e != nil || exit.Code != 0 || exit.Timeout {
		t.Fatalf("stdin=%+v %v", exit, e)
	}
	// script gives the launcher a real terminal. Keep its input open, so EOF
	// must come from the child's null device, not from the harness closing stdin.
	script, err := exec.LookPath("script")
	if err != nil {
		t.Fatal("script is required for the terminal fixture")
	}
	args := []string{"-q", os.DevNull, os.Args[0], "-test.run=^TestTerminalActor$", "--", makeAttempt(t)}
	if runtime.GOOS == "linux" {
		var command []string
		for _, arg := range args[2:] {
			command = append(command, "'"+strings.ReplaceAll(arg, "'", "'\"'\"'")+"'")
		}
		args = []string{"-q", "-e", "-c", strings.Join(command, " "), os.DevNull}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, args...)
	cmd.Env = append(os.Environ(), "PANEL_TERMINAL_ACTOR=1")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	cmd.Stdin = reader
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "TTY-PASS") {
		t.Fatalf("terminal fixture: %v %s", err, out)
	}
}

func TestTerminalActor(t *testing.T) {
	if os.Getenv("PANEL_TERMINAL_ACTOR") != "1" {
		return
	}
	terminal := exec.Command("/bin/sh", "-c", "test -t 0")
	terminal.Stdin = os.Stdin
	if err := terminal.Run(); err != nil {
		t.Fatal("launcher stdin is not a terminal")
	}
	dir := os.Args[len(os.Args)-1]
	if err := Run(Request{Dir: dir, SlotDir: dir, Argv: []string{"/bin/sh", "-c", "if read x; then exit 4; fi"}, Prompt: "file", Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	exit, err := ReadExit(dir)
	if err != nil || exit.Code != 0 || exit.Timeout {
		t.Fatalf("terminal child stdin: %+v %v", exit, err)
	}
	fmt.Println("TTY-PASS")
}
func TestPanelLaunch_NoSignalStalePgid(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "20")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	for _, pid := range []int{0, 1, cmd.Process.Pid} {
		dir := makeAttempt(t)
		f, e := takeRunLock(filepath.Join(dir, "run.lock"))
		if e != nil {
			t.Fatal(e)
		}
		_ = f.Close()
		if e := StopOrphan(dir, pid); e != nil {
			t.Fatal(e)
		}
		if e := syscall.Kill(cmd.Process.Pid, 0); e != nil {
			t.Fatal("free-lock path signalled unrelated group")
		}
	}
}
func TestLockDescriptorActor(t *testing.T) {
	if os.Getenv("PANEL_LOCK_DESCRIPTOR_ACTOR") != "1" {
		return
	}
	info, err := os.Stat(filepath.Join(os.Args[len(os.Args)-1], "run.lock"))
	if err != nil {
		t.Fatal(err)
	}
	want := info.Sys().(*syscall.Stat_t)
	for fd := 3; fd < 1024; fd++ {
		var stat syscall.Stat_t
		if syscall.Fstat(fd, &stat) == nil && stat.Dev == want.Dev && stat.Ino == want.Ino {
			t.Fatalf("reviewer inherited attempt lock on fd %d", fd)
		}
	}
}
func TestPanelLaunch_LocksNotInherited(t *testing.T) {
	dir := makeAttempt(t)
	t.Setenv("PANEL_LOCK_DESCRIPTOR_ACTOR", "1")
	if e := Run(Request{Dir: dir, SlotDir: dir, Argv: []string{os.Args[0], "-test.run=^TestLockDescriptorActor$", "--", dir}, Timeout: 10 * time.Second}); e != nil {
		t.Fatal(e)
	}
	if !lockFree(dir) {
		t.Fatal("reviewer inherited attempt lock")
	}
	exit, e := ReadExit(dir)
	if e != nil || exit.Code != 0 {
		t.Fatalf("inherited descriptor check: %+v %v\n%s", exit, e, mustRead(t, filepath.Join(dir, "stdout.log")))
	}
}
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestPanelLaunch_UnsafeOrphanGroup(t *testing.T) {
	for _, pgid := range []int{0, 1, syscall.Getpgrp()} {
		t.Run(strconv.Itoa(pgid), func(t *testing.T) {
			dir := makeAttempt(t)
			holder, err := takeRunLock(filepath.Join(dir, "run.lock"))
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Close()
			old := stopOrphanSignal
			defer func() { stopOrphanSignal = old }()
			calls := 0
			stopOrphanSignal = func(int, syscall.Signal) error { calls++; return holder.Close() }
			err = StopOrphan(dir, pgid)
			if err == nil || !strings.Contains(err.Error(), dir) || calls != 0 {
				t.Fatalf("unsafe orphan group: error=%v signals=%d", err, calls)
			}
		})
	}
}

func TestPanelLaunch_OrphanRemainsLockedTerminalSafe(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "José-hostile\x1bc\nrecovery: printf INJECTED")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	holder, err := takeRunLock(filepath.Join(dir, "run.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	old := stopOrphanSignal
	t.Cleanup(func() { stopOrphanSignal = old })
	pgid := syscall.Getpgrp() + 1
	calls := 0
	stopOrphanSignal = func(pid int, sig syscall.Signal) error {
		calls++
		if pid != -pgid || sig != syscall.SIGKILL {
			t.Errorf("unexpected signal: pid=%d signal=%v", pid, sig)
		}
		return nil
	}
	err = StopOrphan(dir, pgid)
	if err == nil || !strings.Contains(err.Error(), "attempt remains locked: ") || calls != 1 {
		t.Fatalf("missing held-lock timeout: error=%v signals=%d", err, calls)
	}
	text := err.Error()
	want := termsafe.Escape("attempt remains locked: "+dir) + "\nrecovery: inspect the attempt directory and its live supervisor"
	if text != want {
		t.Errorf("path must be escaped once: got %q want %q", text, want)
	}
	if strings.Contains(text, "\x1b") || strings.Contains(text, "\nrecovery: printf INJECTED") {
		t.Errorf("unsafe held-lock refusal: %q", text)
	}
	const recovery = "\nrecovery: inspect the attempt directory and its live supervisor"
	if strings.Count(text, "\nrecovery:") != 1 || !strings.HasSuffix(text, recovery) {
		t.Errorf("held-lock refusal must end with one real recovery line: %q", text)
	}
}

func TestPanelLaunch_OrphanSmallPGIDRefusal(t *testing.T) {
	for _, pgid := range []int{0, 1} {
		t.Run(strconv.Itoa(pgid), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "José-hostile\x1b\nrecovery: forged")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			holder, err := takeRunLock(filepath.Join(dir, "run.lock"))
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Close()
			old := stopOrphanSignal
			t.Cleanup(func() { stopOrphanSignal = old })
			calls := 0
			stopOrphanSignal = func(int, syscall.Signal) error { calls++; return holder.Close() }
			err = StopOrphan(dir, pgid)
			want := termsafe.Escape("cannot stop attempt "+dir) + "\nrecovery: inspect the attempt directory and its live supervisor"
			if err == nil || err.Error() != want || calls != 0 {
				t.Fatalf("small pgid must refuse without a signal: error=%v want=%q signals=%d", err, want, calls)
			}
		})
	}
}
