package contextpack

import (
	"github.com/mrmaxsteel/mindspec/internal/adr"
)

// ADR is re-exported for backward compatibility with existing types.
type ADR = adr.ADR

// NewADRStore creates a file-based ADR store for the given project root.
func NewADRStore(root string) adr.Store {
	return adr.NewFileStore(root)
}

// ScanADRs is kept for backward compatibility. Prefer using adr.Store.
var ScanADRs = adr.ScanADRs

// FilterADRs is kept for backward compatibility. Prefer using adr.Store.
var FilterADRs = adr.FilterADRs
