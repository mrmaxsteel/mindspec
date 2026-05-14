// Package safeio provides minimal filesystem helpers that refuse to follow
// symlinks at write sites. It exists so callers handling user-config files
// (CLAUDE.md, AGENTS.md, .github/copilot-instructions.md, .claude/settings.json,
// ...) can't be tricked by a planted symlink into appending or rewriting an
// unintended target such as ~/.ssh/authorized_keys.
//
// Helpers:
//
//   - OpenAppendNoSymlink: os.OpenFile(O_APPEND|O_WRONLY) replacement that
//     Lstats first and (on Unix) also passes O_NOFOLLOW to the open call.
//   - WriteFileNoSymlink:  os.WriteFile replacement that Lstats first and
//     writes via tempfile-in-same-dir + rename (mirrors the atomicWrite
//     convention used in internal/bead/config.go).
//
// All symlink refusals wrap ErrSymlinkRefused so callers can branch with
// errors.Is.
package safeio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrSymlinkRefused is returned when a write target is a symlink. We refuse
// rather than follow so an attacker can't aim a planted ./CLAUDE.md (or any
// other managed-config path) at a sensitive file.
var ErrSymlinkRefused = errors.New("refusing to write through symlink")

// OpenAppendNoSymlink opens path for append+write, failing if path is a
// symlink. On Unix the open also uses O_NOFOLLOW as belt-and-suspenders
// against a TOCTOU race between Lstat and OpenFile. On Windows there is no
// O_NOFOLLOW; the Lstat pre-check is the only line of defence (NTFS
// reparse-point hardening is out of scope).
func OpenAppendNoSymlink(path string, perm os.FileMode) (*os.File, error) {
	if err := refuseIfSymlink(path); err != nil {
		return nil, err
	}
	flags := os.O_APPEND | os.O_WRONLY | nofollowFlag()
	return os.OpenFile(path, flags, perm)
}

// WriteFileNoSymlink writes data to path atomically (tempfile + rename in
// the same directory). It first refuses if path is a symlink. The rename
// would otherwise silently replace the symlink with a regular file; the
// pre-check ensures we never get that far.
//
// On a successful run the temp file is renamed onto the target and the
// deferred cleanup is a no-op. On any error before rename, the temp file is
// removed.
func WriteFileNoSymlink(path string, data []byte, perm os.FileMode) error {
	if err := refuseIfSymlink(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mindspec.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// Removed after a successful rename (no-op then) or to clean up on error.
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func refuseIfSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // creating fresh is fine
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s (delete or move the symlink and retry)", ErrSymlinkRefused, path)
	}
	return nil
}
