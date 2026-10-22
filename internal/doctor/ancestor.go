package doctor

import (
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

func addAncestorCheck(r *Report, finding lifecycle.ReattestRefusal) {
	message := finding.Detail
	for _, command := range finding.Recovery {
		message += "\nrecovery: " + command
	}
	r.Checks = append(r.Checks, Check{
		Name:    "ancestor without naming merge: " + idrender.Bead(finding.BeadID),
		Status:  Warn,
		Message: message,
	})
}
