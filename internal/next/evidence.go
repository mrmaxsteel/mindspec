package next

import (
	"crypto/sha256"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"io"
	"os"
)

// ReadOverrideEvidence reads a bounded regular file and retains only its audit
// identity. Text never enters tracker metadata (ADR-0025; spec 129 R6.2).
func ReadOverrideEvidence(path string) (bead.OverrideEvidence, error) {
	bad := func(reason string) (bead.OverrideEvidence, error) {
		return bead.OverrideEvidence{}, fmt.Errorf("readiness override requires --evidence <path>: %s", reason)
	}
	if path == "" {
		return bad("missing --evidence")
	}
	info, err := os.Stat(path)
	if err != nil {
		return bad(termsafe.Escape(err.Error()))
	}
	if !info.Mode().IsRegular() {
		return bad("evidence must be a readable regular file")
	}
	if info.Size() < 1 || info.Size() > 65536 {
		return bad("evidence must contain 1 to 65536 bytes")
	}
	f, err := os.Open(path)
	if err != nil {
		return bad(termsafe.Escape(err.Error()))
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return bad(termsafe.Escape(err.Error()))
	}
	if !info.Mode().IsRegular() {
		return bad("evidence must be a readable regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return bad(termsafe.Escape(err.Error()))
	}
	if len(data) < 1 || len(data) > 65536 {
		return bad("evidence must contain 1 to 65536 bytes")
	}
	return bead.OverrideEvidence{Path: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Bytes: len(data)}, nil
}
