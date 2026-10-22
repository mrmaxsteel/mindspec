//go:build !windows

package safefile

import (
	"fmt"
	"os"
	"syscall"
)

// O_NOFOLLOW and O_NONBLOCK apply to the open itself. Stat checks the same
// descriptor, so replacing the path cannot turn the check into a different open.
// Open opens a regular file without following its final path link or waiting on a pipe.
func Open(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return f, nil
}
