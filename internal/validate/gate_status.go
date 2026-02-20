package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
)

var runBDGateStatusFn = bead.RunBD

func readGateStatus(gateID string) (string, error) {
	out, err := runBDGateStatusFn("show", gateID, "--json")
	if err != nil {
		return "", fmt.Errorf("bd show %s failed: %w", gateID, err)
	}

	var payload []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parsing bd show output for %s: %w", gateID, err)
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("no issue data returned for %s", gateID)
	}

	return strings.ToLower(strings.TrimSpace(payload[0].Status)), nil
}
