package kernel

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// CheckPassedGate checks the highest attempt, its children and accepted subject.
// It reports every failed check so a child cannot hide a moved subject.
func CheckPassedGate(s State, base string) error {
	gate := activeGate(s, base)
	g := s.Records[gate]
	tuple, _ := kernelid.Parse(gate)
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
	ended, endErr := judgeEnded(s, gate)
	if endErr != nil {
		failures = append(failures, endErr)
	}
	moved := false
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		failures = append(failures, err)
	} else if accepted == "" {
		failures = append(failures, fmt.Errorf("gate %s has no accepted subject", termsafe.Escape(gate)))
	} else if rawSubject(s, gate) != "" || (ended && tuple.WorkItem != "") {
		newest, err := effectiveSubject(s, gate)
		if err != nil {
			failures = append(failures, err)
		} else if newest != accepted && !finalizeReplay(s, gate, accepted) {
			moved = true
			failures = append(failures, fmt.Errorf("head %s differs from accepted subject %s for gate %s", termsafe.Escape(rawSubject(s, gate)), termsafe.Escape(accepted), termsafe.Escape(gate)))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	if ended && moved {
		branch, err := workspace.SpecBranch(s.SpecID)
		if tuple.WorkItem != "" {
			branch, err = workspace.BeadBranch(tuple.WorkItem)
		}
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		return fmt.Errorf("%w\nrecovery: restore branch %s to accepted subject %s with git, then retry", errors.Join(failures...), termsafe.Escape(branch), termsafe.Escape(accepted))
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

// finalizeReplay accepts only the recorded publication of a closed final gate.
// The checkpoint may include panel evidence committed before finalization.
func finalizeReplay(s State, gate, accepted string) bool {
	t, err := kernelid.Parse(gate)
	if err != nil || t.WorkItem != "" || s.Records[s.Epic].Status != "closed" {
		return false
	}
	meta := s.Records[s.Epic].Metadata
	before, _ := meta["mindspec_pre_finalize_tip"].(string)
	published, _ := meta["mindspec_finalize_tip"].(string)
	if !objectID(before) || !objectID(published) || rawSubject(s, gate) != published {
		return false
	}
	checkpoint, err := effectiveSubjectAt(s, gate, before)
	if err != nil || checkpoint != accepted {
		return false
	}
	_, err = actorGit(s.Root, "merge-base", "--is-ancestor", before, published)
	if err != nil {
		return false
	}
	data, err := actorGitBytes(s.Root, "diff", "--no-renames", "--name-only", "-z", accepted, published, "--")
	if err != nil {
		return false
	}
	panelDirs := FinalizePanelPaths(s)
	for _, name := range strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00") {
		if name != "" && !panel.FinalizeArtifactPath(name, s.SpecDir, panelDirs...) {
			return false
		}
	}
	return true
}
