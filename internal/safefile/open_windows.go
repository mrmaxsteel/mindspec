//go:build windows

package safefile

import (
	"fmt"
	"os"
	"syscall"
)

// Open opens a regular file without following its final path link or waiting on a pipe.
func Open(path string) (*os.File, error) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(ptr, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	var info syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(h, &info)
	if err != nil || info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return f, nil
}
