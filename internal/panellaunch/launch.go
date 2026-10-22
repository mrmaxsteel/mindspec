// Package panellaunch runs isolated reviewer processes without repository I/O.
package panellaunch

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/safefile"
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

// ReadAttemptFile reads a bounded regular file without following its final path link.
func ReadAttemptFile(dir, name string, limit int64) ([]byte, error) {
	f, err := safefile.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%s exceeds size limit", name)
	}
	return raw, nil
}

// Digests binds all existing output files, including empty files.
func Digests(dir string) (map[string]string, error) {
	out := map[string]string{}
	var readErr error
	for _, name := range []string{"stdout.log", "stderr.log", "verdict.json"} {
		f, err := safefile.Open(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			readErr = errors.Join(readErr, err)
			continue
		}
		// Bound the read to the opened file's size. Appended bytes cannot extend it.
		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			readErr = errors.Join(readErr, err)
			continue
		}
		h := sha256.New()
		_, err = io.CopyN(h, f, info.Size())
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			readErr = errors.Join(readErr, err, closeErr)
			continue
		}
		out[name] = hex.EncodeToString(h.Sum(nil))
	}
	return out, readErr
}

func ReadExit(dir string) (Exit, error) {
	var out Exit
	raw, err := ReadAttemptFile(dir, "exit.json", 65536)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}

// Extract accepts one complete marker pair and binds its nonce by digest.
func Extract(raw []byte, nonceHash string) ([]byte, error) {
	return extractReader(bytes.NewReader(raw), nonceHash)
}

const maxVerdictBytes = 1 << 20

// extractReader discards diagnostic lines as it reads them. Only the verdict
// body and a possible marker line are retained, each with a fixed size bound.
func extractReader(input io.Reader, nonceHash string) ([]byte, error) {
	reader := bufio.NewReader(input)
	var nonce string
	var body, marker bytes.Buffer
	started, ended, lineStart, markerLine := false, false, true, false
	for {
		fragment, err := reader.ReadSlice('\n')
		if err != nil && err != bufio.ErrBufferFull && err != io.EOF {
			return nil, err
		}
		if len(fragment) > 0 {
			if lineStart {
				markerLine = bytes.HasPrefix(fragment, []byte("MINDSPEC-VERDICT-BEGIN ")) || bytes.HasPrefix(fragment, []byte("MINDSPEC-VERDICT-END "))
			}
			switch {
			case markerLine:
				if marker.Len()+len(fragment) > maxVerdictBytes {
					return nil, fmt.Errorf("verdict marker exceeds size limit")
				}
				marker.Write(fragment)
			case started && !ended:
				if body.Len()+len(fragment) > maxVerdictBytes {
					return nil, fmt.Errorf("verdict block exceeds size limit")
				}
				body.Write(fragment)
			}
		}
		lineStart = err != bufio.ErrBufferFull
		if lineStart && markerLine && marker.Len() > 0 {
			line := strings.TrimSuffix(strings.TrimSuffix(marker.String(), "\n"), "\r")
			if strings.HasPrefix(line, "MINDSPEC-VERDICT-BEGIN ") {
				if started || ended {
					return nil, fmt.Errorf("multiple verdict blocks")
				}
				nonce = strings.TrimPrefix(line, "MINDSPEC-VERDICT-BEGIN ")
				if Hash([]byte(nonce)) != nonceHash {
					return nil, fmt.Errorf("verdict nonce differs")
				}
				started = true
			} else {
				if !started || ended || line != "MINDSPEC-VERDICT-END "+nonce {
					return nil, fmt.Errorf("invalid verdict end marker")
				}
				ended = true
			}
			marker.Reset()
		}
		if err == io.EOF {
			break
		}
	}
	if !started || !ended {
		return nil, fmt.Errorf("one complete verdict block is required")
	}
	return bytes.TrimSpace(body.Bytes()), nil
}

// Verdict reads only the current attempt. File bytes are preserved exactly.
func Verdict(dir, output, nonceHash string) ([]byte, error) {
	if output == "file" {
		return ReadAttemptFile(dir, "verdict.json", maxVerdictBytes)
	}
	f, err := safefile.Open(filepath.Join(dir, "stdout.log"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	// Appended diagnostics cannot extend the transcript being read.
	return extractReader(io.LimitReader(f, info.Size()), nonceHash)
}

func Quota(dir string, signals []string) bool {
	f, err := safefile.Open(filepath.Join(dir, "stderr.log"))
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
	raw, err := io.ReadAll(io.LimitReader(f, 65536))
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
