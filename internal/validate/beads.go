// Package validate runs workflow validation checks.
//
// This package may not import os/exec or internal/gitutil. Process I/O
// routes through internal/executor; bd I/O routes through internal/bead.
package validate

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
)

// CheckBeadExists verifies a bead ID exists in Beads by routing through
// internal/bead (the bd boundary; see ADR-0030). The (bool, error)
// contract is: (true, nil) if found; (false, nil) if bd reported the
// bead missing; (false, err) only if bd itself is unavailable.
func CheckBeadExists(id string) (bool, error) {
	exists, err := bead.BeadExists(id)
	if err != nil {
		return false, fmt.Errorf("running bd show: %w", err)
	}
	return exists, nil
}

// checkBeadIDs verifies each bead ID in the plan frontmatter exists.
func checkBeadIDs(r *Result, ids []string) {
	if len(ids) == 0 {
		return
	}

	for _, id := range ids {
		// id is a bead_ids entry from the agent-authored plan.md
		// frontmatter — render it through idrender.Bead (spec 120 R4):
		// byte-identical for a genuine bead ID, forced-safe otherwise,
		// since this exact check exists BECAUSE the ID may not be valid.
		safeID := idrender.Bead(id)
		exists, err := CheckBeadExists(id)
		if err != nil {
			r.AddWarning("bead-id-check", fmt.Sprintf("cannot verify bead %s (Beads unavailable): %v", safeID, err))
			return // don't keep checking if bd is unavailable
		}
		if !exists {
			r.AddError("bead-id-missing", fmt.Sprintf("bead ID %s not found in Beads", safeID))
		}
	}
}
