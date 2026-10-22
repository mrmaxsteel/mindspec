//go:build windows

package panel

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

func windowsPIDLive(pid int) bool {
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return err != syscall.Errno(87)
	}
	defer syscall.CloseHandle(h)
	result, err := syscall.WaitForSingleObject(h, 0)
	return err != nil || result == uint32(syscall.WAIT_TIMEOUT)
}
func windowsLockOps(identity *os.FileInfo) staleLockOps {
	return staleLockOps{
		create: func(path string, pid int) error {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			_, err = f.WriteString(strconv.Itoa(pid) + "\n")
			if err == nil {
				*identity, err = f.Stat()
			}
			closeErr := f.Close()
			if err != nil {
				return err
			}
			return closeErr
		}, read: os.ReadFile, rename: os.Rename, link: os.Link, remove: os.Remove, live: windowsPIDLive, unique: uniqueLockName,
	}
}
func TrySlotLock(path string) (*SlotLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	var identity os.FileInfo
	acquired, err := decideStaleLock(path, os.Getpid(), windowsLockOps(&identity))
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, heldSlotError(path)
	}
	return &SlotLock{release: func() error {
		current, err := os.Stat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if identity == nil || !os.SameFile(identity, current) {
			return heldSlotError(path)
		}
		return os.Remove(path)
	}}, nil
}
func ProbeSlotLock(path string) (bool, error) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	h, err := syscall.CreateFile(ptr, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		return false, err
	}
	stat, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || !stat.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular lock file", path)
	}
	raw, err := io.ReadAll(io.LimitReader(f, 64))
	if err != nil {
		return false, err
	}
	pid, valid := lockPID(raw)
	return !valid || windowsPIDLive(pid), nil
}
