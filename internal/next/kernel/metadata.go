package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const MetadataLimit = 16 << 20

// CheckMetadataSize checks the complete metadata before a write is attempted.
func CheckMetadataSize(key string, metadata map[string]any) ([]byte, error) {
	return checkMetadataSize(key, metadata, "size")
}

func checkMetadataSize(key string, metadata map[string]any, sizeKind string) ([]byte, error) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, pathFailure(err, "cannot encode metadata: ", "submit metadata that can be encoded as JSON")
	}
	if len(data) <= MetadataLimit {
		return data, nil
	}
	largest, size := "", -1
	for k, v := range metadata {
		value, err := json.Marshal(v)
		if err != nil {
			return nil, pathFailure(err, "cannot encode metadata: ", "submit metadata that can be encoded as JSON")
		}
		if len(value) > size || (len(value) == size && k < largest) {
			largest, size = k, len(value)
		}
	}
	tuple, _, _ := Decode[Tuple](metadata, KeyTuple)
	if tuple.Gate == "" {
		tuple, _ = kernelid.Parse(key)
	}
	return nil, metadataSizeFailure(tuple.Gate, key, largest, int64(len(data)), sizeKind)
}

func metadataSizeFailure(gate, step, key string, size int64, sizeKind string) error {
	recovery := "halt the spec"
	switch key {
	case KeyRecord, KeyReturns:
		recovery = "submit a shorter record"
	case KeyFindings:
		recovery = "shorten an unpinned verdict for a new review attempt, or halt the spec"
	}
	return termsafe.NewFailure(fmt.Sprintf("metadata-too-large: gate %s step %s key %s %s %d bytes exceeds %d bytes", gate, step, key, sizeKind, size, MetadataLimit), recovery)
}

// KernelTempDir returns the external directory for metadata files and actor records.
func KernelTempDir(root string) (string, error) {
	return kernelTempDir(root, true)
}

func kernelTempDir(root string, create bool) (string, error) {
	root, err := absoluteSuppliedPath(root)
	if err != nil {
		return "", pathFailure(err, "cannot resolve project root: ", "restore access to the project root, then retry")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", pathFailure(err, "cannot resolve project root: ", "restore access to the project root, then retry")
	}
	checked, err := checkSuppliedPath("", os.TempDir(), tempPathRecovery)
	if err != nil {
		return "", tempDirFailure(err)
	}
	if !checked.info.IsDir() {
		return "", tempDirFailure(fmt.Errorf("temporary path is not a directory"))
	}
	roots, err := temporaryRoots(root)
	if err != nil {
		return "", tempDirFailure(err)
	}
	for _, worktree := range roots {
		for _, name := range []string{checked.clean, checked.resolved} {
			inside, err := insidePath(worktree, name)
			if err != nil {
				return "", tempDirFailure(err)
			}
			if inside {
				return "", tempDirFailure(fmt.Errorf("kernel temporary directory must be outside every worktree of this repository"))
			}
		}
	}
	base := checked.resolved
	sum := sha256.Sum256([]byte(root))
	dir := filepath.Join(base, "mindspec-kernel-"+hex.EncodeToString(sum[:12]))
	if create {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", tempDirFailure(err)
		}
	}
	info, err := os.Lstat(dir)
	if !create && os.IsNotExist(err) {
		return dir, nil
	}
	if err != nil {
		return "", tempDirFailure(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !privateTempDir(info) {
		return "", tempDirFailure(fmt.Errorf("kernel temporary directory must be a private directory owned by the current user"))
	}
	return dir, nil
}

func tempDirFailure(err error) error {
	return pathFailure(err, "", tempPathRecovery)
}

const tempPathRecovery = "set TMPDIR to a private directory outside every worktree of the repository; use an existing path without .. segments"

// absoluteSuppliedPath retains dot segments until symlinks have been resolved.
func absoluteSuppliedPath(name string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}
	base, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return base + string(filepath.Separator) + name, nil
}

// insidePath compares file identities so filesystem aliases keep their containment.
// Callers first check path spelling, then test the cleaned path and resolved target.
func insidePath(root, name string) (bool, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false, pathFailure(err, "cannot inspect containment root: ", "restore access to the path and repository worktrees")
	}
	for {
		info, err := os.Stat(name)
		if err != nil {
			return false, pathFailure(err, "cannot inspect path ancestor: ", "restore access to the path and repository worktrees")
		}
		if os.SameFile(rootInfo, info) {
			return true, nil
		}
		parent := filepath.Dir(name)
		if parent == name {
			return false, nil
		}
		name = parent
	}
}

// SweepMetadata removes only metadata files owned by processes that no longer exist.
// Actor records in the records subdirectory are never swept.
func SweepMetadata(root string) error {
	return sweepMetadata(root, os.Remove)
}

func sweepMetadata(root string, remove func(string) error) error {
	dir, err := KernelTempDir(root)
	if err != nil {
		return tempDirFailure(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return tempDirFailure(err)
	}
	var failures []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "metadata-") {
			continue
		}
		parts := strings.Split(entry.Name(), "-")
		if len(parts) != 3 {
			continue
		}
		pid, err := strconv.Atoi(parts[1])
		if err != nil || pid <= 0 {
			continue
		}
		if processRunning(pid) {
			continue
		}
		if err := remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return tempDirFailure(errors.Join(failures...))
	}
	return nil
}

func (s *Store) metadataArg(key string, metadata map[string]any) (string, func(), error) {
	data, err := CheckMetadataSize(key, metadata)
	if err != nil {
		return "", nil, err
	}
	dir, err := KernelTempDir(s.Root)
	if err != nil {
		return "", nil, tempDirFailure(err)
	}
	f, err := os.CreateTemp(dir, fmt.Sprintf("metadata-%d-", os.Getpid()))
	if err != nil {
		return "", nil, tempDirFailure(err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return "", nil, tempDirFailure(err)
	}
	return "@" + f.Name(), cleanup, nil
}

func mergedMetadata(base, updates map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(updates))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range updates {
		result[k] = v
	}
	return result
}
