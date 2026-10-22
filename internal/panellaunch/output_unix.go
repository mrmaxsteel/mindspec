//go:build !windows

package panellaunch

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// O_NOFOLLOW and O_NONBLOCK apply to the open itself. Stat checks the same
// descriptor, so replacing the path cannot turn the check into a different open.
func openAttemptFile(dir, name string) (*os.File, error) {
	path := filepath.Join(dir, name)
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
		return nil, fmt.Errorf("%s is not a regular attempt file", path)
	}
	return f, nil
}
