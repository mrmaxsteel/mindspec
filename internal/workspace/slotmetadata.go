package workspace

import (
	"fmt"
	"io"

	"github.com/mrmaxsteel/mindspec/internal/safefile"
)

// ReadSlotMetadata reads a regular slot.json without following its final
// symlink or waiting on a pipe. It refuses files larger than 64 KiB.
func ReadSlotMetadata(path string) ([]byte, error) {
	f, err := safefile.Open(path)
	if err != nil {
		return nil, err
	}
	const maxMetadataBytes = 64 << 10
	data, err := io.ReadAll(io.LimitReader(f, maxMetadataBytes+1))
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(data) > maxMetadataBytes {
		return nil, fmt.Errorf("slot metadata exceeds 64 KiB")
	}
	return data, nil
}
