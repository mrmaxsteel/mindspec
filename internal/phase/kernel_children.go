package phase

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"gopkg.in/yaml.v3"
)

// lifecycleChildrenWithCache selects recorded work and kernel identities once enrolled.
// The legacy type rule remains in DerivePhaseFromChildren for older approvals.
func lifecycleChildrenWithCache(c *Cache, epicID string, children []ChildInfo) ([]ChildInfo, error) {
	epic, err := c.FindEpic(epicID)
	if err != nil {
		return nil, err
	}
	if epic == nil {
		return children, nil
	}
	w := kernelid.Witnesses{Epic: epicID}
	_, w.RunPresent = epic.Metadata["mindspec_run"]
	recorded := map[string]bool{}
	var links []struct {
		ID string `json:"id"`
	}
	if raw, ok := epic.Metadata["mindspec_links"]; ok {
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &links); err != nil {
			return nil, err
		}
		for _, link := range links {
			recorded[link.ID] = true
		}
	}
	// Optional files that cannot prove enrolment are not witnesses.
	if specID, err := ResolveSpec(*epic); err == nil {
		if dir, err := workspace.SpecDir(c.repositoryRoot(), specID); err == nil {
			w.Orchestrated, _ = kernelid.PanelOrchestrated(dir)
			if len(recorded) == 0 {
				data, err := os.ReadFile(filepath.Join(dir, "plan.md"))
				if err == nil {
					var plan struct {
						IDs []string `yaml:"bead_ids"`
					}
					parts, _, _ := frontmatter.Parse(data)
					if err = yaml.Unmarshal(parts, &plan); err == nil {
						for _, id := range plan.IDs {
							recorded[id] = true
						}
					}
				}
			}
		}
	}
	addWitnesses := func(items []ChildInfo) {
		for _, child := range items {
			w.IDs = append(w.IDs, child.ID)
			if key, ok := child.Metadata["mindspec_key"].(string); ok {
				w.Keys = append(w.Keys, key)
			}
		}
	}
	addWitnesses(children)
	enrolled, _ := kernelid.Enrolled(w)
	if !enrolled {
		// Find closed or retyped nested identities in one read, without a
		// child query for each work item of an older approval.
		data, err := listJSONFn("--all", "-n", "0")
		if err != nil {
			return nil, err
		}
		var records []ChildInfo
		if err = json.Unmarshal(data, &records); err != nil {
			return nil, err
		}
		addWitnesses(records)
		enrolled, _ = kernelid.Enrolled(w)
	}
	if !enrolled {
		return children, nil
	}
	candidates := append([]ChildInfo{}, children...)
	// Include nested kernel steps so a leased actor advances the phase.
	queried := map[string]bool{}
	for i := 0; i < len(candidates); i++ {
		child := candidates[i]
		key, _ := child.Metadata["mindspec_key"].(string)
		if queried[child.ID] || !(recorded[child.ID] || kernelid.UnderEpic(child.ID, epicID) || kernelid.UnderEpic(key, epicID)) {
			continue
		}
		queried[child.ID] = true
		nested, err := c.GetChildren(child.ID)
		if err != nil {
			return nil, err
		}
		for _, item := range nested {
			if !queried[item.ID] {
				candidates = append(candidates, item)
			}
		}
	}
	selected := []ChildInfo{}
	for _, child := range candidates {
		key, _ := child.Metadata["mindspec_key"].(string)
		if recorded[child.ID] || kernelid.UnderEpic(child.ID, epicID) || kernelid.UnderEpic(key, epicID) {
			child.IssueType = "task"
			selected = append(selected, child)
		}
	}
	return selected, nil
}
