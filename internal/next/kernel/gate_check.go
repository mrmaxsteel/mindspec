package kernel

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// CheckPassedGate checks the highest attempt, its children and accepted subject.
// It reports every failed check so a child cannot hide a moved subject.
func CheckPassedGate(s State, base string) error {
	gate := activeGate(s, base)
	g := s.Records[gate]
	var failures []error
	if !owned(s, gate) || !passed(s, g) {
		failures = append(failures, fmt.Errorf("gate %s has no passing receipt", termsafe.Escape(gate)))
	}
	parent := g.Parent
	if parent == "" {
		parent = s.Epic
	}
	var children []string
	for key, r := range s.Records {
		if r.Status != "closed" && (r.Parent == parent || r.Parent == g.ID || r.Parent == gate) && key != gate {
			children = append(children, key)
		}
	}
	sort.Strings(children)
	for _, key := range children {
		failures = append(failures, fmt.Errorf("open child: %s", termsafe.Escape(key)))
	}
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		failures = append(failures, err)
	} else if accepted != "" && subject(s, gate) != "" {
		newest, err := newestJudgeSubject(s, gate)
		if err != nil {
			failures = append(failures, err)
		} else if newest != accepted {
			failures = append(failures, fmt.Errorf("head %s differs from accepted subject %s for gate %s", termsafe.Escape(subject(s, gate)), termsafe.Escape(accepted), termsafe.Escape(gate)))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%w\nrecovery: mindspec next-action %s --apply", errors.Join(failures...), idrender.Spec(s.SpecID))
}

// ImplementationStep selects the newest implementation attempt of a work item.
func ImplementationStep(s State, item string) string {
	selected, attempt := "", 0
	for _, id := range s.Steps {
		t, ok, err := s.Records[id].Tuple()
		if err == nil && ok && t.Kind == "implement" && t.WorkItem == item && t.Attempt > attempt {
			selected, attempt = id, t.Attempt
		}
	}
	return selected
}

// CheckHalt refuses mutation after a durable halt answer.
func CheckHalt(s State) error { return refuseHalt(s) }

// GateBase returns the base review identity for a recorded work item.
func GateBase(item string) string { return strings.TrimSpace(item) + ".review" }

// CheckWorkItem checks holds and upstream blockers before a work item closes.
func CheckWorkItem(s State, item string) error { return actorBlockers(s, s.Records[item]) }
