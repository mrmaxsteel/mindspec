// Package panellaunch runs isolated reviewer processes without repository I/O.
package panellaunch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Request contains only values needed by the supervisor. Started runs before
// the start pipe is released, so the caller can register the process group.
type Request struct {
	Dir      string
	SlotDir  string
	Argv     []string
	Prompt   string
	Timeout  time.Duration
	Started  func(int) error                 `json:"-"`
	Register func(func() (int, error)) error `json:"-"`
	// SupervisorCommand permits a local supervisor substitute in process tests.
	SupervisorCommand []string `json:"-"`
}

type Exit struct {
	Code    int  `json:"exit_code"`
	Timeout bool `json:"timeout"`
}

func Hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// Digests binds all existing output files, including empty files.
func Digests(dir string) (map[string]string, error) {
	out := map[string]string{}
	for _, name := range []string{"stdout.log", "stderr.log", "verdict.json"} {
		f, err := os.Open(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		out[name] = hex.EncodeToString(h.Sum(nil))
	}
	return out, nil
}

func ReadExit(dir string) (Exit, error) {
	var out Exit
	raw, err := os.ReadFile(filepath.Join(dir, "exit.json"))
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}

// Extract accepts one complete marker pair and binds its nonce by digest.
func Extract(raw []byte, nonceHash string) ([]byte, error) {
	var nonce string
	var body bytes.Buffer
	started, ended := false, false
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		s := strings.TrimSuffix(string(line), "\r")
		if strings.HasPrefix(s, "MINDSPEC-VERDICT-BEGIN ") {
			if started || ended {
				return nil, fmt.Errorf("multiple verdict blocks")
			}
			nonce = strings.TrimPrefix(s, "MINDSPEC-VERDICT-BEGIN ")
			if Hash([]byte(nonce)) != nonceHash {
				return nil, fmt.Errorf("verdict nonce differs")
			}
			started = true
			continue
		}
		if strings.HasPrefix(s, "MINDSPEC-VERDICT-END ") {
			if !started || ended || s != "MINDSPEC-VERDICT-END "+nonce {
				return nil, fmt.Errorf("invalid verdict end marker")
			}
			ended = true
			continue
		}
		if started && !ended {
			body.Write(line)
			body.WriteByte('\n')
		}
	}
	if !started || !ended {
		return nil, fmt.Errorf("one complete verdict block is required")
	}
	return bytes.TrimSpace(body.Bytes()), nil
}

// Verdict reads only the current attempt. File bytes are preserved exactly.
func Verdict(dir, output, nonceHash string) ([]byte, error) {
	name := "stdout.log"
	if output == "file" {
		name = "verdict.json"
	}
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// A stdout transcript may include reviewer diagnostics around its block.
	const maxChannelBytes = 16 << 20
	raw, err := io.ReadAll(io.LimitReader(f, maxChannelBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxChannelBytes {
		return nil, fmt.Errorf("verdict channel exceeds size limit")
	}
	if output == "file" {
		return raw, nil
	}
	return Extract(raw, nonceHash)
}

func Quota(dir string, signals []string) bool {
	f, err := os.Open(filepath.Join(dir, "stderr.log"))
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false
	}
	if info.Size() > 65536 {
		if _, err := f.Seek(-65536, io.SeekEnd); err != nil {
			return false
		}
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return false
	}
	for _, signal := range signals {
		if strings.Contains(string(raw), signal) {
			return true
		}
	}
	return false
}

func atomicFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".supervisor-*")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// ChildEnv removes both framework state and repository-location overrides.
func ChildEnv(env []string) []string {
	git := map[string]bool{"GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_CONFIG": true, "GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_COUNT": true, "GIT_OBJECT_DIRECTORY": true, "GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_IMPLICIT_WORK_TREE": true, "GIT_GRAFT_FILE": true, "GIT_INDEX_FILE": true, "GIT_NO_REPLACE_OBJECTS": true, "GIT_REPLACE_REF_BASE": true, "GIT_PREFIX": true, "GIT_INTERNAL_SUPER_PREFIX": true, "GIT_SHALLOW_FILE": true, "GIT_COMMON_DIR": true, "GIT_NAMESPACE": true}
	var out []string
	for _, value := range env {
		key, _, _ := strings.Cut(value, "=")
		if !strings.HasPrefix(key, "MINDSPEC_") && !strings.HasPrefix(key, "GIT_CONFIG") && !git[key] {
			out = append(out, value)
		}
	}
	return out
}
