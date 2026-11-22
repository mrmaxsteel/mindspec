package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	if len(data) <= MetadataLimit {
		return data, nil
	}
	largest, size := "", -1
	for k, v := range metadata {
		value, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		if len(value) > size || (len(value) == size && k < largest) {
			largest, size = k, len(value)
		}
	}
	tuple, _, _ := Decode[Tuple](metadata, KeyTuple)
	if tuple.Gate == "" {
		tuple, _ = kernelid.Parse(key)
	}
	return nil, actorFailure(fmt.Sprintf("metadata-too-large: gate %s step %s key %s size %d bytes exceeds %d bytes", termsafe.Escape(tuple.Gate), termsafe.Escape(key), termsafe.Escape(largest), len(data), MetadataLimit))
}

// KernelTempDir returns the external directory for metadata files and actor records.
func KernelTempDir(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	base, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", err
	}
	if insidePath(root, base) {
		return "", actorFailure("kernel temporary directory must be outside this repository")
	}
	sum := sha256.Sum256([]byte(root))
	dir := filepath.Join(base, "mindspec-kernel-"+hex.EncodeToString(sum[:12]))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func insidePath(root, name string) bool {
	rel, err := filepath.Rel(root, name)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// SweepMetadata removes only metadata files owned by processes that no longer exist.
// Actor records in the records subdirectory are never swept.
func SweepMetadata(root string) error {
	dir, err := KernelTempDir(root)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
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
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
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
		return "", nil, err
	}
	f, err := os.CreateTemp(dir, fmt.Sprintf("metadata-%d-", os.Getpid()))
	if err != nil {
		return "", nil, err
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
		return "", nil, err
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
