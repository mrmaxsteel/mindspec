package panel

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// SlotLock is an OS-visible lock. Release is called once by its holder.
type SlotLock struct{ release func() error }

func (l *SlotLock) Release() error {
	if l == nil || l.release == nil {
		return nil
	}
	f := l.release
	l.release = nil
	return f()
}

var ErrSlotLocked = errors.New("slot lock is held")

func heldSlotError(path string) error {
	return termsafe.NewWrappedFailure(ErrSlotLocked, fmt.Sprintf("%v: %s", ErrSlotLocked, path), "run mindspec panel verify; delete the lock file only after you make sure no mindspec process uses the slot")
}
func AcquireSlotLock(path string) (*SlotLock, error) {
	deadline := time.Now().Add(time.Second)
	for {
		lock, err := TrySlotLock(path)
		if !errors.Is(err, ErrSlotLocked) {
			return lock, err
		}
		if !time.Now().Before(deadline) {
			return nil, err
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type staleLockOps struct {
	create func(string, int) error
	read   func(string) ([]byte, error)
	rename func(string, string) error
	link   func(string, string) error
	remove func(string) error
	live   func(int) bool
	unique func(string) string
}

func lockPID(raw []byte) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	return n, err == nil && n > 0
}
func uniqueLockName(path string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s.stale-%d-%d", path, os.Getpid(), time.Now().UnixNano())
	}
	return path + ".stale-" + hex.EncodeToString(b[:])
}

// decideStaleLock is shared with the Windows binding. Rename and re-read stop
// a stale observation from deleting a replacement holder's lock. Put-back uses
// link, never rename-overwrite. Every failure preserves any live moved inode.
func decideStaleLock(path string, pid int, ops staleLockOps) (bool, error) {
	if err := ops.create(path, pid); err == nil {
		return true, nil
	} else if !os.IsExist(err) {
		return false, err
	}
	raw, err := ops.read(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	stale, valid := lockPID(raw)
	if !valid || ops.live(stale) {
		return false, nil
	}
	moved := ops.unique(path)
	if err := ops.rename(path, moved); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	raw, err = ops.read(moved)
	if err != nil {
		return false, fmt.Errorf("read moved lock %s: %w", termsafe.Escape(moved), err)
	}
	current, valid := lockPID(raw)
	if valid && current == stale && !ops.live(current) {
		if err := ops.remove(moved); err != nil {
			return false, err
		}
		if err := ops.create(path, pid); err != nil {
			if os.IsExist(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	if err := ops.link(moved, path); err == nil {
		// The lock inode is back at its original name. Remove only the extra link.
		return false, ops.remove(moved)
	} else if !os.IsExist(err) {
		return false, fmt.Errorf("cannot restore lock %s from %s: %w", termsafe.Escape(path), termsafe.Escape(moved), err)
	}
	if valid && !ops.live(current) {
		return false, ops.remove(moved)
	}
	return false, termsafe.NewFailure(fmt.Sprintf("lock files %s and %s need inspection", path, moved), "delete a file only after you make sure no mindspec process uses it")
}
