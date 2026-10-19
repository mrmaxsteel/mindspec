package bead

import (
	"strconv"
	"strings"
)

// PlanBeadIndex returns the positive decimal index in exactly epicID.N.
// N has no leading zero or further segments. It checks form only; callers
// separately check lineage and positional plan mapping (spec 129 R1.1).
func PlanBeadIndex(epicID, id string) (n int, ok bool) {
	if epicID == "" || !strings.HasPrefix(id, epicID+".") {
		return 0, false
	}
	suffix := strings.TrimPrefix(id, epicID+".")
	if suffix == "" || suffix[0] == '0' {
		return 0, false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}
