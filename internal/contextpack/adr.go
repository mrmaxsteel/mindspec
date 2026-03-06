package contextpack

import (
	"github.com/mrmaxsteel/mindspec/internal/adr"
)

// Re-export types and functions from the adr package for backward compatibility.
type ADR = adr.ADR

var ScanADRs = adr.ScanADRs
var FilterADRs = adr.FilterADRs
