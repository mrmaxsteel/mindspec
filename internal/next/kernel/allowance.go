package kernel

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// Allowance returns the pinned limit plus durable continuations for this gate.
func Allowance(s State, gate string) int {
	limit := s.Policy.RoundLimit
	tuple, err := kernelid.Parse(gate)
	if err != nil {
		return limit
	}
	for id, answer := range s.Answers() {
		t, _, _ := s.Records[id].Tuple()
		if t.Gate != tuple.Gate {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(answer.Option, "continue:"))
		if strings.HasPrefix(answer.Option, "continue:") && err == nil && n > 0 && n <= int(^uint(0)>>1)-limit {
			limit += n
		}
	}
	return limit
}

// RoundsForGate includes abandoned and ledger-only registrations.
// A damaged ledger refuses the whole count, including an unrelated ledger.
func RoundsForGate(s State, gate string) (int, error) {
	t, err := kernelid.Parse(gate)
	if err != nil {
		return 0, err
	}
	kind := "bead"
	if t.WorkItem == "" {
		kind = "final_review"
	}
	regs := append([]panel.Registration{}, s.Panels...)
	seen := map[string]bool{}
	for _, r := range regs {
		seen[r.Dir] = true
	}
	for dir := range s.Ledgers {
		if !seen[dir] {
			regs = append(regs, panel.Registration{Dir: dir})
		}
	}
	return panel.RoundsForGate(regs, s.SpecID, kind, t.WorkItem)
}

func panelDirectories(root, specDir string) ([]panel.Registration, error) {
	roots := []string{root, filepath.Join(root, ".mindspec"), specDir}
	regs := panel.Scan(roots...)
	seen := map[string]bool{}
	for _, r := range regs {
		seen[r.Dir] = true
	}
	for _, base := range roots {
		for _, segment := range []string{"review", "reviews"} {
			paths, err := filepath.Glob(filepath.Join(base, segment, "*", "dispositions.jsonl"))
			if err != nil {
				return nil, fmt.Errorf("find panel ledgers: %w", err)
			}
			for _, path := range paths {
				dir := filepath.Dir(path)
				if !seen[dir] {
					regs = append(regs, panel.Registration{Dir: dir})
					seen[dir] = true
				}
			}
		}
	}
	sort.Slice(regs, func(i, j int) bool { return regs[i].Dir < regs[j].Dir })
	return regs, nil
}
