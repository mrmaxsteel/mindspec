//go:build !windows

package safeio

import "syscall"

// nofollowFlag returns syscall.O_NOFOLLOW so OpenAppendNoSymlink fails at the
// kernel layer if the path turned into a symlink between Lstat and OpenFile.
func nofollowFlag() int { return syscall.O_NOFOLLOW }
