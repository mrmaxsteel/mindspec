package panel

import (
	"encoding/json"
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// RoundsForGate counts ledger round records across registrations for a gate.
// For bead gates, beadID identifies the work item. Other gates use spec.
// Abandonment and snapshot edits do not erase history. Launch and recovery
// records do not add rounds. Callers supply all registration directories,
// including ledger-only directories left by an interrupted create.
func RoundsForGate(regs []Registration, spec, gate, beadID string) (int, error) {
	switch gate {
	case "bead":
		if beadID == "" {
			return 0, fmt.Errorf("a bead gate needs a work item")
		}
	case "final_review", "spec_approve", "plan_approve":
		if spec == "" {
			return 0, fmt.Errorf("a lifecycle gate needs a spec")
		}
	default:
		return 0, fmt.Errorf("unknown round-counter gate %q", gate)
	}
	count := 0
	seen := map[string]bool{}
	for _, reg := range regs {
		dir := canonicalPath(reg.Dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		lines, err := ReadLedger(dir)
		if err != nil {
			return 0, fmt.Errorf("read panel rounds in %s: %w", termsafe.Escape(dir), err)
		}
		for _, line := range lines {
			var round RoundRecord
			if err := json.Unmarshal(line, &round); err != nil {
				return 0, fmt.Errorf("read panel rounds in %s: %w", termsafe.Escape(dir), err)
			}
			if round.Record != RecordRound {
				continue
			}
			if gate == "bead" {
				if (round.Gate == "bead" || round.Gate == "") && round.BeadID != nil && *round.BeadID == beadID {
					count++
				}
			} else if round.Spec == spec && round.Gate == gate {
				count++
			}
		}
	}
	return count, nil
}
