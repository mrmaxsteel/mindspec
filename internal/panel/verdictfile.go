package panel

import (
	"fmt"
	"io"

	"github.com/mrmaxsteel/mindspec/internal/safefile"
)

// ReadVerdictFile reads a regular verdict file without following its final
// symlink or waiting on a pipe. It refuses files larger than MaxVerdictBytes.
func ReadVerdictFile(path string) ([]byte, error) {
	f, err := safefile.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxVerdictBytes+1))
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(data) > MaxVerdictBytes {
		return nil, fmt.Errorf("verdict file exceeds 1 MiB")
	}
	return data, nil
}
