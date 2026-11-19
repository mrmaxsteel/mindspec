// Package kernelid defines kernel identities without workflow dependencies.
package kernelid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Tuple is the identity of a kernel record.
type Tuple struct {
	Epic     string `json:"epic"`
	WorkItem string `json:"work_item,omitempty"`
	Gate     string `json:"gate,omitempty"`
	Kind     string `json:"kind"`
	Round    int    `json:"round,omitempty"`
	Slot     int    `json:"slot,omitempty"`
	Attempt  int    `json:"attempt"`
	Cause    string `json:"cause,omitempty"`
}

var shape = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_-]*-[A-Za-z0-9]+)(?:\.([1-9][0-9]*))?\.(impl|review|final)(?:(\.r([1-9][0-9]*)\.(R([1-9][0-9]*)|fix))|(?:\.decide\.([a-z][a-z-]*)\.r([1-9][0-9]*))|(\.judge))?(?:\.a([1-9][0-9]*))?$`)

// Parse accepts exact kernel shapes. Numeric children are foreign.
func Parse(id string) (Tuple, error) {
	m := shape.FindStringSubmatch(id)
	if m == nil {
		return Tuple{}, fmt.Errorf("invalid kernel id %q", id)
	}
	for _, index := range []int{5, 7, 9, 11} {
		if m[index] != "" {
			if _, err := strconv.Atoi(m[index]); err != nil {
				return Tuple{}, fmt.Errorf("kernel counter is too large")
			}
		}
	}
	t := Tuple{Epic: m[1], Attempt: 1}
	if m[2] != "" {
		t.WorkItem = m[1] + "." + m[2]
	}
	switch m[3] {
	case "impl":
		if t.WorkItem == "" || m[4] != "" || m[8] != "" || m[10] != "" {
			return Tuple{}, fmt.Errorf("invalid implement id %q", id)
		}
		t.Kind = "implement"
	case "review":
		if t.WorkItem == "" {
			return Tuple{}, fmt.Errorf("review needs a work item")
		}
		t.Gate = t.WorkItem + ".review"
		t.Kind = "bead-gate"
	case "final":
		if t.WorkItem != "" {
			return Tuple{}, fmt.Errorf("final needs an epic")
		}
		t.Gate = t.Epic + ".final"
		t.Kind = "final-gate"
	}
	if m[5] != "" {
		t.Round, _ = strconv.Atoi(m[5])
		t.Kind = "fix"
		if m[7] != "" {
			t.Kind = "review-slot"
			t.Slot, _ = strconv.Atoi(m[7])
		}
	}
	if m[8] != "" {
		t.Kind = "decision"
		t.Cause = m[8]
		t.Round, _ = strconv.Atoi(m[9])
	}
	if m[10] != "" {
		t.Kind = "change-judge"
	}
	if m[11] != "" {
		t.Attempt, _ = strconv.Atoi(m[11])
	}
	if t.Attempt < 1 || ((m[5] != "" || m[9] != "") && t.Round < 1) || (m[7] != "" && t.Slot < 1) {
		return Tuple{}, fmt.Errorf("kernel counter is too large")
	}
	return t, nil
}

// IsKernel reports whether id is exactly a kernel identity.
func IsKernel(id string) bool { _, err := Parse(id); return err == nil }

// Format produces the canonical identity for a tuple.
func Format(t Tuple) (string, error) {
	var id string
	switch t.Kind {
	case "implement":
		id = t.WorkItem + ".impl"
	case "bead-gate":
		id = t.WorkItem + ".review"
	case "final-gate":
		id = t.Epic + ".final"
	case "review-slot":
		id = fmt.Sprintf("%s.r%d.R%d", t.Gate, t.Round, t.Slot)
	case "fix":
		id = fmt.Sprintf("%s.r%d.fix", t.Gate, t.Round)
	case "decision":
		id = fmt.Sprintf("%s.decide.%s.r%d", t.Gate, t.Cause, t.Round)
	case "change-judge":
		id = t.Gate + ".judge"
	default:
		return "", fmt.Errorf("unknown kernel kind %q", t.Kind)
	}
	if t.Attempt > 1 || t.Kind == "implement" || t.Kind == "change-judge" {
		id += fmt.Sprintf(".a%d", max(1, t.Attempt))
	}
	parsed, err := Parse(id)
	if err != nil {
		return "", err
	}
	if parsed.Epic != t.Epic || (t.WorkItem != "" && parsed.WorkItem != t.WorkItem) {
		return "", fmt.Errorf("inconsistent kernel tuple")
	}
	return id, nil
}

// UnderEpic rejects a kernel witness from a different tree.
func UnderEpic(id, epic string) bool {
	t, err := Parse(id)
	return err == nil && t.Epic == strings.TrimSpace(epic)
}
