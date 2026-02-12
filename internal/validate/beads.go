package validate

import (
	"fmt"
	"os/exec"
)

// CheckBeadExists verifies a bead ID exists in Beads by running `bd show <id> --json`.
func CheckBeadExists(id string) (bool, error) {
	err := exec.Command("bd", "show", id, "--json").Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil // command ran but bead not found
		}
		return false, fmt.Errorf("running bd show: %w", err) // bd not available
	}
	return true, nil
}

// checkBeadIDs verifies each bead ID in the plan frontmatter exists.
func checkBeadIDs(r *Result, ids []string) {
	if len(ids) == 0 {
		return
	}

	for _, id := range ids {
		exists, err := CheckBeadExists(id)
		if err != nil {
			r.AddWarning("bead-id-check", fmt.Sprintf("cannot verify bead %s (Beads unavailable): %v", id, err))
			return // don't keep checking if bd is unavailable
		}
		if !exists {
			r.AddError("bead-id-missing", fmt.Sprintf("bead ID %s not found in Beads", id))
		}
	}
}
