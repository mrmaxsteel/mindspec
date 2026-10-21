package phase

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// ErrSpecResolution marks a spec binding that needs repair, not a tracker read failure.
var ErrSpecResolution = errors.New("spec resolution failed")

// ErrNoSpecID marks an epic with no spec ID source.
var ErrNoSpecID = errors.New("epic has no spec ID source")

// lineageReadError escapes external error text and preserves its cause.
type lineageReadError struct{ cause error }

func (e *lineageReadError) Error() string { return termsafe.Escape(e.cause.Error()) }
func (e *lineageReadError) Unwrap() error { return e.cause }

// LineageProvenance describes the edges read in the bead's one show call.
type LineageProvenance struct {
	ParentChild   bool
	NonEpicParent bool
}

func titleSpecID(title string) string {
	start := strings.Index(title, "[SPEC ")
	if start < 0 {
		return ""
	}
	tail := title[start+6:]
	end := strings.Index(tail, "]")
	if end < 0 {
		return ""
	}
	return tail[:end]
}

func legacySpecID(epic EpicInfo) (string, error) {
	var num int
	switch v := epic.Metadata["spec_num"].(type) {
	case float64:
		num = int(v)
	case int:
		num = v
	}
	title, _ := epic.Metadata["spec_title"].(string)
	if num == 0 || title == "" {
		return "", nil
	}
	return SpecIDFromMetadata(num, title)
}

func specRepair(epic EpicInfo, id string, cause error) error {
	hint := ""
	if idvalidate.SpecID(id) != nil {
		id = "<spec-directory-name>"
		hint = "; use the spec directory name as the ID"
	}
	return fmt.Errorf("%w: cannot resolve spec for epic %s: %w%s\nrecovery: bd update %s --set-metadata spec_id=%s", ErrSpecResolution, idrender.Bead(epic.ID), &lineageReadError{cause: cause}, hint, idrender.Bead(epic.ID), idrender.Spec(id))
}

// ResolveSpec resolves an epic through spec_id, the legacy pair, then its title.
// A stored full ID decides. Conflicting legacy and title IDs require repair.
func ResolveSpec(epic EpicInfo) (string, error) {
	if raw, present := epic.Metadata["spec_id"]; present {
		id, ok := raw.(string)
		if !ok || idvalidate.SpecID(id) != nil {
			return "", specRepair(epic, titleSpecID(epic.Title), fmt.Errorf("%w: invalid spec_id %s", ErrMalformedSpecMetadata, termsafe.Escape(fmt.Sprint(raw))))
		}
		return id, nil
	}
	legacy, err := legacySpecID(epic)
	if err != nil {
		return "", specRepair(epic, titleSpecID(epic.Title), err)
	}
	title := titleSpecID(epic.Title)
	if title != "" && idvalidate.SpecID(title) != nil {
		if legacy == "" {
			return "", specRepair(epic, "", fmt.Errorf("%w: invalid title ID %s", ErrMalformedSpecMetadata, termsafe.Escape(title)))
		}
		title = ""
	}
	if legacy != "" && title != "" && legacy != title {
		return "", specRepair(epic, title, fmt.Errorf("legacy ID %s disagrees with title ID %s", idrender.Spec(legacy), idrender.Spec(title)))
	}
	if legacy != "" {
		return legacy, nil
	}
	if title != "" {
		return title, nil
	}
	return "", specRepair(epic, "", ErrNoSpecID)
}

// FindEpicLineageWithCache reads one bead and returns its one parent-child epic.
// Lookup errors stay distinct from a successful lookup with no lineage.
func FindEpicLineageWithCache(c *Cache, beadID string) (epicID, specID string, provenance LineageProvenance, err error) {
	if e := idvalidate.BeadID(beadID); e != nil {
		return "", "", provenance, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), e)
	}
	out, e := runBDFn("show", beadID, "--json")
	if e != nil {
		return "", "", provenance, fmt.Errorf("bd show %s failed: %w\nrecovery: bd show %s --json", idrender.Bead(beadID), &lineageReadError{cause: e}, idrender.Bead(beadID))
	}
	var rows []struct {
		Dependencies []struct {
			ID             string `json:"id"`
			IssueType      string `json:"issue_type"`
			DependencyType string `json:"dependency_type"`
		} `json:"dependencies"`
	}
	if e := json.Unmarshal(out, &rows); e != nil {
		return "", "", provenance, fmt.Errorf("cannot read lineage for %s: %s\nrecovery: bd show %s --json", idrender.Bead(beadID), termsafe.Escape(e.Error()), idrender.Bead(beadID))
	}
	if len(rows) == 0 {
		return "", "", provenance, fmt.Errorf("bead %s not found: %w", idrender.Bead(beadID), ErrNoEpicLineage)
	}
	var parents []string
	for _, dep := range rows[0].Dependencies {
		if dep.DependencyType != "parent-child" {
			continue
		}
		if !strings.EqualFold(dep.IssueType, "epic") {
			provenance.NonEpicParent = true
			if _, e := c.FindEpic(dep.ID); e != nil {
				return "", "", provenance, fmt.Errorf("cannot read parent %s: %w\nrecovery: bd show %s --json", idrender.Bead(dep.ID), &lineageReadError{cause: e}, idrender.Bead(dep.ID))
			}
			continue
		}
		if e := idvalidate.BeadID(dep.ID); e != nil {
			return "", "", provenance, fmt.Errorf("invalid parent epic %s: %w\nrecovery: bd show %s --json", idrender.Bead(dep.ID), e, idrender.Bead(beadID))
		}
		parents = append(parents, dep.ID)
	}
	if len(parents) > 1 {
		var recoveries []string
		for _, parent := range parents {
			recoveries = append(recoveries, fmt.Sprintf("recovery: bd dep remove %s %s", idrender.Bead(beadID), idrender.Bead(parent)))
		}
		return "", "", provenance, fmt.Errorf("bead %s has multiple parent-child epics: %s\n%s", idrender.Bead(beadID), strings.Join(parents, ", "), strings.Join(recoveries, "\n"))
	}
	if len(parents) == 0 {
		return "", "", provenance, fmt.Errorf("no epic found for bead %s: %w", idrender.Bead(beadID), ErrNoEpicLineage)
	}
	provenance.ParentChild = true
	epicID = parents[0]
	epic, e := c.FindEpic(epicID)
	if e != nil {
		return epicID, "", provenance, fmt.Errorf("resolving parent epic %s for bead %s: %w\nrecovery: bd show %s --json", idrender.Bead(epicID), idrender.Bead(beadID), &lineageReadError{cause: e}, idrender.Bead(epicID))
	}
	if epic == nil {
		return epicID, "", provenance, fmt.Errorf("parent epic %s is absent\nrecovery: bd show %s --json", idrender.Bead(epicID), idrender.Bead(epicID))
	}
	specID, err = ResolveSpec(*epic)
	if err == nil {
		if _, lookupErr := c.FindEpicBySpecID(specID); lookupErr != nil && !errors.Is(lookupErr, ErrNoEpicForSpec) {
			err = lookupErr
		}
	}
	return
}
