// Package recording — lockfile contract.
//
// CONTRACT (owned by mindspec, honored by the AgentMind binary):
//
// On startup, AgentMind MUST write ${HOME}/.mindspec/agentmind.lock with mode
// 0600 inside a 0700 directory. The file is JSON:
//
//	{
//	  "pid":        12345,           // os.Getpid()
//	  "otlp_port":  4318,
//	  "ui_port":    8420,
//	  "token":      "<32 hex bytes>",// crypto/rand
//	  "started_at": "2026-05-14T..." // RFC3339
//	}
//
// On clean shutdown, AgentMind SHOULD remove the file. mindspec tolerates
// stale files: a lockfile whose PID is no longer alive is treated as absent.
//
// Spec 083 Phase 5 (Bead 5): this code was moved out of the deleted
// `internal/agentmind/` package and into `internal/recording/` because
// mindspec owns the lockfile contract — the standalone agentmind binary is
// the writer per the contract above, but the wrapper at
// `cmd/mindspec/viz.go` still writes the file during the pre-1.0
// transition (the contract docstring notes "When AgentMind is extracted
// into its own binary, this file...MUST be copied/re-exported").
package recording

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// LockfileDirName is the per-user mindspec config directory (relative to $HOME).
	LockfileDirName = ".mindspec"
	// LockfileBaseName is the file name of the AgentMind lockfile inside LockfileDirName.
	LockfileBaseName = "agentmind.lock"
)

// Lockfile is the on-disk record of a running AgentMind process.
type Lockfile struct {
	PID       int       `json:"pid"`
	OTLPPort  int       `json:"otlp_port"`
	UIPort    int       `json:"ui_port"`
	Token     string    `json:"token"`
	StartedAt time.Time `json:"started_at"`
}

// LockfileDir returns ${HOME}/.mindspec — the per-user mindspec runtime dir.
func LockfileDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory for lockfile: %w", err)
	}
	if home == "" {
		return "", errors.New("home directory empty; cannot resolve lockfile path")
	}
	return filepath.Join(home, LockfileDirName), nil
}

// LockfilePath returns ${HOME}/.mindspec/agentmind.lock.
func LockfilePath() (string, error) {
	dir, err := LockfileDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, LockfileBaseName), nil
}

// WriteLockfile writes lf to disk atomically, ensuring the parent directory
// is 0700 and the file is 0600. Existing files are replaced.
func WriteLockfile(lf Lockfile) error {
	dir, err := LockfileDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	// Tighten perms even if MkdirAll respected an earlier permissive umask.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("chmod %s: %w", dir, err)
	}

	path := filepath.Join(dir, LockfileBaseName)
	tmp, err := os.CreateTemp(dir, ".agentmind.lock.*")
	if err != nil {
		return fmt.Errorf("creating temp lockfile in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// best-effort cleanup if rename never happens
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp lockfile: %w", err)
	}

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&lf); err != nil {
		tmp.Close()
		return fmt.Errorf("encoding lockfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp lockfile: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming lockfile into place: %w", err)
	}
	// Re-chmod the final file in case rename preserved a different mode.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// RemoveLockfile deletes the lockfile, ignoring not-exist errors.
func RemoveLockfile() error {
	path, err := LockfilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// NewToken returns a 32-byte crypto/rand value as 64 hex characters.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reading crypto/rand: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
