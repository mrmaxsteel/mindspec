package next

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

type lineageInfo struct {
	epic, spec, why                         string
	none, nonEpicParent, dependency, mapped bool
}

// NearMiss is a linked ready row that cannot be selected as a plan bead.
type NearMiss struct {
	Bead       BeadInfo
	Class      string
	Message    string
	Recoveries []string
	Blocking   bool
	Lineage    string
	NoLineage  bool
}

func (m NearMiss) String() string { return m.Message + "\n" + strings.Join(m.Recoveries, "\n") }

// hasEpicDependency distinguishes dependency resolution from the title fallback.
// FindEpicForBead deliberately exposes only the resolved identity, so the
// ambiguous foreign-epic case inspects the dependency list rather than guessing.
func hasEpicDependency(id, epic string) (bool, error) {
	if err := idvalidate.BeadID(id); err != nil {
		return false, err
	}
	out, err := runBDFn("show", id, "--json")
	if err != nil {
		return false, err
	}
	var rows []struct {
		Dependencies []struct {
			ID        string
			IssueType string `json:"issue_type"`
		}
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, fmt.Errorf("bead %s disappeared during lineage read", idrender.Bead(id))
	}
	for _, dep := range rows[0].Dependencies {
		if dep.ID == epic && strings.EqualFold(dep.IssueType, "epic") {
			return true, nil
		}
	}
	return false, nil
}

func classifyNearMiss(item BeadInfo, epic, spec string, n int, form bool, l lineageInfo) (NearMiss, bool) {
	// A mapped row with this lineage is a plan bead even if a ready
	// producer unexpectedly reports it in a non-open status.
	if form && !l.nonEpicParent && l.epic == epic && l.mapped {
		return NearMiss{}, false
	}
	id, e, s := idrender.Bead(item.ID), idrender.Bead(epic), idrender.Spec(spec)
	link := ""
	switch {
	case item.Parent == epic:
		link = "parent edge"
	case containsSpecID(item.Title, spec):
		link = "title"
	case item.Metadata["spec_id"] == spec:
		link = "metadata.spec_id"
	}
	if !form && link == "" {
		return NearMiss{}, false
	}
	m := NearMiss{Bead: item, Lineage: l.epic, NoLineage: l.none, Recoveries: []string{fmt.Sprintf("recovery: bd show %s --json", id)}}
	prefix := fmt.Sprintf("bead %s has the plan-bead form for epic %s and plan.md maps Bead %d, but ", id, e, n)
	switch {
	case form && l.nonEpicParent:
		m.Class, m.Blocking = "N2a", true
		// This class does not depend on whether the plan maps N.
		m.Message = fmt.Sprintf("bead %s has the plan-bead form for epic %s, but its parent is %s", id, e, idrender.Bead(item.Parent))
	case form && l.dependency:
		m.Class, m.Blocking = "N2c", true
		m.Message = fmt.Sprintf("bead %s has the plan-bead form for epic %s, but its lineage resolves to %s through its dependency on epic %s", id, e, idrender.Bead(l.epic), idrender.Bead(l.epic))
	case form && !l.mapped:
		m.Class = "N4"
		m.Message = fmt.Sprintf("bead %s is a follow-up child of epic %s that plan.md does not map (%s); readiness would fail MF-1, so mindspec next does not select it", id, e, l.why)
		if l.none {
			m.Message += "; it also has no lineage, so readiness cannot evaluate it at all"
		}
	case form && l.epic != "" && l.epic != epic:
		m.Class, m.Blocking = "N2b", true
		number := strings.SplitN(l.spec, "-", 2)[0]
		m.Message = prefix + fmt.Sprintf("its lineage resolves to %s by title number %s", idrender.Bead(l.epic), number)
		reparent := fmt.Sprintf("bd update %s --parent %s", id, e)
		if number == strings.SplitN(spec, "-", 2)[0] {
			m.Recoveries = []string{"recovery: " + reparent}
		} else {
			m.Recoveries = append(m.Recoveries, fmt.Sprintf("recovery (only if %s belongs to %s): %s", id, s, reparent))
		}
	case form && l.none:
		m.Class, m.Blocking = "N1", true
		m.Message = prefix + "it has no lineage (no parent edge and no spec tag in its title)"
		m.Recoveries = []string{fmt.Sprintf("recovery: bd update %s --parent %s", id, e)}
	default:
		if link == "" {
			return NearMiss{}, false
		}
		m.Class = "N3"
		m.Blocking = item.Parent == epic && phase.IsLifecycleIssueType(item.IssueType)
		m.Message = fmt.Sprintf("bead %s is bd-ready and linked to spec %s by %s, but it is not a plan bead (no %s.<N> ID with lineage %s); mindspec next claims only plan beads", id, s, link, e, e)
	}
	return m, true
}
