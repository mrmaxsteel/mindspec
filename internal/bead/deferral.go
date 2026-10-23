package bead

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

// DeferralRequirementsError lists every requirement a follow-up bead
// misses, so one refusal names all of them. Recovery is the next command
// to run.
type DeferralRequirementsError struct {
	Missing  []string
	Recovery string
}

func (e *DeferralRequirementsError) Error() string {
	return "follow-up bead does not meet the deferral requirements:\n- " + strings.Join(e.Missing, "\n- ")
}

// DeferralIssue contains the live facts used at write time and at both gates.
type DeferralIssue struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Description  string `json:"description"`
	Parent       string `json:"parent"`
	Dependencies []struct {
		ID   string `json:"id"`
		Type string `json:"dependency_type"`
	} `json:"dependencies"`
}

func readDeferralIssue(id string) (DeferralIssue, error) {
	if err := idvalidate.BeadID(id); err != nil {
		return DeferralIssue{}, err
	}
	raw, err := RunBD("show", id, "--json")
	if err != nil {
		return DeferralIssue{}, fmt.Errorf("cannot read follow-up bead: %w", err)
	}
	var issues []DeferralIssue
	if err := json.Unmarshal(raw, &issues); err != nil {
		return DeferralIssue{}, err
	}
	if len(issues) != 1 || issues[0].ID != id {
		return DeferralIssue{}, fmt.Errorf("follow-up bead does not exist")
	}
	return issues[0], nil
}

// CheckDeferral checks every ancestor, not just the direct parent. The caller
// supplies the same path replacement used for stored findings.
func CheckDeferral(id, epic, panelBead string, texts []string, redact func(string) string) error {
	if err := idvalidate.BeadID(epic); err != nil {
		return fmt.Errorf("owning epic is unavailable: %w", err)
	}
	b, err := readDeferralIssue(id)
	if err != nil {
		return err
	}
	if b.Status == "" || strings.EqualFold(b.Status, "closed") || strings.EqualFold(b.Status, "done") || strings.EqualFold(b.Status, "tombstone") {
		return fmt.Errorf("follow-up bead must exist and be open")
	}
	seen := map[string]bool{}
	queue := []DeferralIssue{b}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.ID == epic {
			return fmt.Errorf("follow-up bead must be outside the owning epic")
		}
		if seen[current.ID] {
			return fmt.Errorf("follow-up parent chain contains a cycle")
		}
		seen[current.ID] = true
		parents := []string{}
		if current.Parent != "" {
			parents = append(parents, current.Parent)
		}
		for _, dep := range current.Dependencies {
			if dep.Type == "parent-child" && dep.ID != current.Parent {
				parents = append(parents, dep.ID)
			}
		}
		for _, parent := range parents {
			if parent == epic {
				return fmt.Errorf("follow-up bead must be outside the owning epic")
			}
			ancestor, err := readDeferralIssue(parent)
			if err != nil {
				return err
			}
			queue = append(queue, ancestor)
		}
	}
	edge := false
	for _, dep := range b.Dependencies {
		if dep.Type == "discovered-from" && (dep.ID == epic || panelBead != "" && dep.ID == panelBead) {
			edge = true
		}
	}
	var missing, recovery []string
	if !edge {
		link := fmt.Sprintf("bd dep add %s %s --type discovered-from", id, epic)
		missing = append(missing, "follow-up bead needs a discovered-from edge to the owning epic or reviewed bead; run: "+link)
		recovery = append(recovery, link)
	}
	description := redact(b.Description)
	var absent []string
	for _, text := range texts {
		if strings.TrimSpace(text) == "" || !strings.Contains(description, redact(text)) {
			absent = append(absent, redact(text))
		}
	}
	if len(absent) > 0 {
		missing = append(missing, "follow-up description must contain every deferred finding; add this text to the description of "+id+":\n"+strings.Join(absent, "\n"))
		recovery = append(recovery, "add the listed finding text to the description of "+id)
	}
	if len(missing) > 0 {
		recovery = append(recovery, "retry the panel verb")
		return &DeferralRequirementsError{Missing: missing, Recovery: strings.Join(recovery, ", then ")}
	}
	return nil
}
