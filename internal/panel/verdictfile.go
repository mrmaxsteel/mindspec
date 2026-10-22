package panel

import (
	"fmt"
	"io"

	"github.com/mrmaxsteel/mindspec/internal/safefile"
)

// ReadVerdictFile reads a regular verdict file without following its final
// symlink or waiting on a pipe. Only v2 rounds enforce MaxVerdictBytes.
func ReadVerdictFile(path string, schema int) ([]byte, error) {
	f, err := safefile.Open(path)
	if err != nil {
		return nil, err
	}
	var reader io.Reader = f
	if schema == 2 {
		reader = io.LimitReader(f, MaxVerdictBytes+1)
	}
	data, err := io.ReadAll(reader)
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if schema == 2 && len(data) > MaxVerdictBytes {
		return nil, fmt.Errorf("verdict file exceeds 1 MiB")
	}
	return data, nil
}
