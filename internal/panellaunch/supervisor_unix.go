//go:build !windows

package panellaunch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const supervisorMode = "__mindspec_panel_supervisor"

var stopOrphanSignal = syscall.Kill

var errRunLocked = errors.New("attempt lock is held")

func Supported() error { return nil }

func init() {
	if len(os.Args) == 3 && os.Args[1] == supervisorMode {
		os.Exit(supervise(os.Args[2], os.NewFile(3, "run.lock"), os.NewFile(4, "start"), nil))
	}
}

func takeRunLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) //nolint:gosec // G115: fd is a small process-local descriptor
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = f.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("%w: %s", errRunLocked, path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Run retains its descriptor until the supervisor exits. Closing the parent's
// duplicate must not explicitly unlock the supervisor's shared description.
func Run(in Request) error {
	lock, err := takeRunLock(filepath.Join(in.Dir, "run.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer reader.Close()
	defer writer.Close()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, supervisorMode, in.Dir)
	if len(in.SupervisorCommand) > 0 {
		cmd = exec.Command(in.SupervisorCommand[0], append(in.SupervisorCommand[1:], in.Dir)...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.ExtraFiles = []*os.File{lock, reader}
	cmd.Env = ChildEnv(os.Environ())
	startSupervisor := func() (int, error) {
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		_ = reader.Close()
		if in.Started != nil {
			return cmd.Process.Pid, in.Started(cmd.Process.Pid)
		}
		return cmd.Process.Pid, nil
	}
	if in.Register != nil {
		err = in.Register(startSupervisor)
	} else {
		_, err = startSupervisor()
	}
	if err != nil {
		_ = writer.Close()
		if cmd.Process != nil {
			_ = cmd.Wait()
		}
		return err
	}
	if err := json.NewEncoder(writer).Encode(struct {
		Start string
		Request
	}{"start", in}); err != nil {
		_ = writer.Close()
		_ = cmd.Wait()
		return err
	}
	_ = writer.Close()
	// A normal supervisor ends by killing its group, including itself.
	_ = cmd.Wait()
	return nil
}

// beforePgid is nil except in process fault tests.
func supervise(dir string, lock, start *os.File, beforePgid func()) int {
	defer func() { _ = syscall.Kill(-os.Getpid(), syscall.SIGKILL); runtime.KeepAlive(lock) }()
	syscall.CloseOnExec(int(lock.Fd()))  //nolint:gosec // G115: fd is a small process-local descriptor
	syscall.CloseOnExec(int(start.Fd())) //nolint:gosec // G115: fd is a small process-local descriptor
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() { <-signals; _ = syscall.Kill(-os.Getpid(), syscall.SIGKILL) }()
	if beforePgid != nil {
		beforePgid()
	}
	if err := atomicFile(filepath.Join(dir, "pgid"), []byte(strconv.Itoa(os.Getpid())+"\n")); err != nil {
		return 1
	}
	var input struct {
		Start string
		Request
	}
	if err := json.NewDecoder(start).Decode(&input); err != nil || input.Start != "start" {
		return 1
	}
	_ = start.Close()
	in := input.Request
	if len(in.Argv) == 0 {
		return 1
	}
	stdin := os.DevNull
	if in.Prompt == "stdin" {
		stdin = filepath.Join(dir, "prompt.md")
	}
	f, err := os.Open(stdin)
	if err != nil {
		return 1
	}
	defer f.Close()
	out, err := os.OpenFile(filepath.Join(dir, "stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return 1
	}
	defer out.Close()
	stderr, err := os.OpenFile(filepath.Join(dir, "stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return 1
	}
	defer stderr.Close()
	cmd := exec.Command(in.Argv[0], in.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = in.SlotDir, ChildEnv(os.Environ()), f, out, stderr
	exit := Exit{Code: 1}
	if err := cmd.Start(); err == nil {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		timeout := in.Timeout
		if timeout <= 0 {
			timeout = time.Hour
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err := <-done:
			if err == nil {
				exit.Code = 0
			} else if e, ok := err.(*exec.ExitError); ok {
				exit.Code = e.ExitCode()
			}
		case <-timer.C:
			exit.Timeout = true
		}
	}
	raw, _ := json.Marshal(exit)
	if err := atomicFile(filepath.Join(dir, "exit.json"), raw); err != nil {
		return 1
	}
	return 0
}

// StopOrphan signals only while the supervisor's inherited lock is held.
// A caller must hold the slot lock, so a live launch cannot still own it.
func StopOrphan(dir string, pgid int) error {
	path := filepath.Join(dir, "run.lock")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	f, err := takeRunLock(path)
	if err == nil {
		return f.Close()
	}
	if !errors.Is(err, errRunLocked) {
		return err
	}
	if pgid <= 1 || pgid == syscall.Getpgrp() {
		return fmt.Errorf("cannot stop attempt %s\nrecovery: inspect the attempt directory and its live supervisor", termsafe.Escape(dir))
	}
	if err := stopOrphanSignal(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		f, err = takeRunLock(path)
		if err == nil {
			return f.Close()
		}
		if !errors.Is(err, errRunLocked) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("attempt remains locked: %s\nrecovery: inspect the attempt directory and its live supervisor", termsafe.Escape(dir))
		}
	}
}
