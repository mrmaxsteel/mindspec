package kernel

import (
	"sort"
)

func init() {
	for _, r := range []struct {
		name string
		fn   Rule
	}{
		{"run-marker-missing", markerFlag}, {"foreign-child-open", childFlags},
		{"foreign-blocker", blockerFlags}, {"id-collision", collisionFlags}, {"raw-claim", ancestorClaimFlags},
	} {
		if err := RegisterRule(r.name, FlagOnly, r.fn); err != nil {
			panic(err)
		}
	}
}

type edge struct{ down, up, kind string }

func graph(s State) (map[string]Record, []edge) {
	records := map[string]Record{}
	keys := map[string]string{}
	for _, r := range s.Graph {
		keys[r.ID] = r.Key()
		records[r.Key()] = r
	}
	for key, r := range s.Records {
		keys[r.ID] = key
		records[key] = r
	}
	key := func(id string) string {
		if k, ok := keys[id]; ok {
			return k
		}
		return id
	}
	edges := []edge{}
	for id, r := range records {
		if r.Parent != "" {
			edges = append(edges, edge{key(r.Parent), id, "parent-child"})
		}
		for _, d := range r.Dependencies {
			up := d.ID
			if d.DependsOnID != "" {
				up = d.DependsOnID
			}
			switch d.Type {
			case "blocks":
				edges = append(edges, edge{id, key(up), "blocks"})
			case "parent-child":
				if r.Parent == "" {
					edges = append(edges, edge{key(up), id, "parent-child"})
				}
			}
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.down != b.down {
			return a.down < b.down
		}
		if a.up != b.up {
			return a.up < b.up
		}
		return a.kind < b.kind
	})
	return records, edges
}
func hasOpenChild(s State, id string) bool {
	records, edges := graph(s)
	for _, e := range edges {
		if e.kind == "parent-child" && e.down == id && records[e.up].Status != "closed" {
			return true
		}
	}
	return false
}
func hasOpenBlocker(s State, id string) bool {
	records, edges := graph(s)
	for _, e := range edges {
		if e.kind == "blocks" && e.down == id && records[e.up].Status != "closed" {
			return true
		}
	}
	return false
}
func blocked(s State, id string) bool {
	records, edges := graph(s)
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(node string) bool {
		if seen[node] {
			return false
		}
		seen[node] = true
		r := records[node]
		if node != id && (r.Status == "blocked" || r.Status == "deferred" || (r.Status == "in_progress" && !owned(s, node))) {
			return true
		}
		for _, e := range edges {
			if e.down == node && e.kind == "blocks" {
				if owned(s, e.up) {
					if !itemDone(s, e.up) {
						return true
					}
				} else if records[e.up].Status != "closed" {
					return true
				}
			}
			if e.up == node && e.kind == "parent-child" && visit(e.down) {
				return true
			}
		}
		return r.Status == "blocked" || r.Status == "deferred"
	}
	return visit(id)
}
func requiredLinks(s State) []edge {
	links := []edge{}
	for _, l := range s.Links {
		for _, up := range l.Upstream {
			links = append(links, edge{l.ID, up, "blocks"})
		}
	}
	for _, item := range s.Items {
		links = append(links, edge{s.Epic + ".final", item, "blocks"})
	}
	return links
}
func linkExists(edges []edge, w edge) bool {
	for _, e := range edges {
		if e == w {
			return true
		}
	}
	return false
}
func cycle(s State, w edge) ([]edge, bool) {
	_, edges := graph(s)
	seen := map[string]bool{}
	var path func(string) ([]edge, bool)
	path = func(id string) ([]edge, bool) {
		if id == w.down {
			return nil, true
		}
		if seen[id] {
			return nil, false
		}
		seen[id] = true
		for _, e := range edges {
			if e.down == id {
				if tail, ok := path(e.up); ok {
					return append([]edge{e}, tail...), true
				}
			}
		}
		return nil, false
	}
	// Blocks cannot connect an ancestor and a descendant in either direction.
	var childPath func(string, string, map[string]bool) ([]edge, bool)
	childPath = func(from, to string, seen map[string]bool) ([]edge, bool) {
		if from == to {
			return nil, true
		}
		if seen[from] {
			return nil, false
		}
		seen[from] = true
		for _, e := range edges {
			if e.kind == "parent-child" && e.down == from {
				if tail, ok := childPath(e.up, to, seen); ok {
					return append([]edge{e}, tail...), true
				}
			}
		}
		return nil, false
	}
	if p, ok := childPath(w.down, w.up, map[string]bool{}); ok {
		return p, true
	}
	if p, ok := childPath(w.up, w.down, map[string]bool{}); ok {
		return p, true
	}
	return path(w.up)
}
func missingLinks(s State) (Changes, error) {
	c := Changes{}
	for _, w := range requiredLinks(s) {
		_, edges := graph(s)
		if !owned(s, w.down) || !owned(s, w.up) || linkExists(edges, w) {
			continue
		}
		if _, cyclic := cycle(s, w); cyclic {
			continue
		}
		write := Write{Kind: "link", ID: w.down, Upstream: w.up}
		c.Writes = append(c.Writes, write)
		if err := overlay(&s, write); err != nil {
			return c, err
		}
	}
	return c, nil
}
func markerFlag(s State) (Changes, error) {
	c := Changes{}
	if s.MarkerMissing {
		c.Flags = []Flag{{Code: "run-marker-missing", Step: s.Epic, Repair: []string{"mindspec", "state", "show"}}}
	}
	return c, nil
}
func childFlags(s State) (Changes, error) {
	c := Changes{}
	records, edges := graph(s)
	seen := map[string]bool{}
	for _, e := range edges {
		if e.kind == "parent-child" && owned(s, e.down) && !owned(s, e.up) && records[e.up].Status != "closed" && !seen[e.up] {
			c.Flags = append(c.Flags, Flag{Code: "foreign-child-open", Step: e.up, Repair: []string{"bd", "close", records[e.up].ID}})
			seen[e.up] = true
		}
	}
	return c, nil
}
func blockerFlags(s State) (Changes, error) {
	c := Changes{}
	records, edges := graph(s)
	seen := map[edge]bool{}
	add := func(e edge) {
		if seen[e] {
			return
		}
		seen[e] = true
		c.Flags = append(c.Flags, Flag{Code: "foreign-blocker", Step: e.down, Repair: []string{"bd", "dep", "remove", records[e.down].ID, records[e.up].ID}})
	}
	for _, w := range requiredLinks(s) {
		if linkExists(edges, w) {
			continue
		}
		if path, cyclic := cycle(s, w); cyclic {
			for i := len(path) - 1; i >= 0; i-- {
				e := path[i]
				if !owned(s, e.down) || !owned(s, e.up) {
					add(e)
					break
				}
			}
		}
	}
	for _, e := range edges {
		if e.kind == "blocks" && owned(s, e.down) && !owned(s, e.up) && records[e.up].Status != "closed" {
			add(e)
		}
	}
	return c, nil
}
func collisionFlags(s State) (Changes, error) {
	c := Changes{}
	for _, id := range s.Collisions {
		r := s.Records[id]
		c.Flags = append(c.Flags, Flag{Code: "id-collision", Step: id, Repair: []string{"bd", "delete", r.ID}})
	}
	return c, nil
}

func ancestorClaimFlags(s State) (Changes, error) {
	c := Changes{}
	ids := append([]string{s.Epic}, s.Items...)
	ids = append(ids, s.Gates...)
	ids = append(ids, s.Steps...)
	for _, id := range ids {
		r := s.Records[id]
		lease, leased, err := Decode[LeaseRecord](r.Metadata, KeyLease)
		step, _, _ := r.Tuple()
		actorLease := step.Kind != "" && step.Kind != "bead-gate" && step.Kind != "final-gate" && step.Kind != "decision" && err == nil && leased && lease.Generation > 0 && lease.TokenHash != ""
		if r.Status == "in_progress" && !actorLease {
			c.Flags = append(c.Flags, Flag{Code: "raw-claim", Step: id, Repair: []string{"bd", "update", r.ID, "--status", "open"}})
		}
	}
	return c, nil
}
