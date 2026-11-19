package kernel

// recordParent uses the parent field, with a parent-child dependency as fallback.
func recordParent(r Record) string {
	if r.Parent != "" {
		return r.Parent
	}
	parent := ""
	for _, d := range r.Dependencies {
		if d.Type == "parent-child" {
			parent = d.ID
			if d.DependsOnID != "" {
				parent = d.DependsOnID
			}
		}
	}
	return parent
}

// Subtree selects an epic and all descendants from a complete record list.
func Subtree(all []Record, epic string) map[string]bool {
	subtree := map[string]bool{epic: true}
	for changed := true; changed; {
		changed = false
		for _, r := range all {
			if !subtree[r.ID] && subtree[recordParent(r)] {
				subtree[r.ID] = true
				changed = true
			}
		}
	}
	return subtree
}
