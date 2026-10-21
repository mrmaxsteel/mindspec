package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// The old numeric title convention is used only to diagnose missing edges.
func lineageTitleNumber(title string) (int, bool) {
	start := strings.Index(title, "[")
	if start < 0 {
		return 0, false
	}
	tail := title[start+1:]
	end := strings.Index(tail, "]")
	if end < 0 {
		return 0, false
	}
	var number int
	_, err := fmt.Sscanf(tail[:end], "%d", &number)
	return number, err == nil
}

func checkLineage(r *Report, root string) {
	// An export or config alone does not provide a queryable store.
	store := false
	for _, name := range []string{"metadata.json", "redirect", "beads.db", "bd.db", "dolt"} {
		if _, err := os.Stat(filepath.Join(root, ".beads", name)); err == nil {
			store = true
			break
		}
	}
	if !store {
		return
	}
	c := phase.NewCache()
	epics, err := c.AllEpics()
	if err != nil {
		r.Checks = append(r.Checks, Check{Name: "Lineage", Status: Warn, Message: "cannot list epics: " + termsafe.Escape(err.Error())})
		return
	}
	bySpec := map[string][]string{}
	var order []string
	for _, epic := range epics {
		id, err := phase.ResolveSpec(epic)
		if err != nil {
			continue
		}
		if len(bySpec[id]) == 0 {
			order = append(order, id)
		}
		bySpec[id] = append(bySpec[id], idrender.Bead(epic.ID))
	}
	for _, id := range order {
		if len(bySpec[id]) > 1 {
			r.Checks = append(r.Checks, Check{Name: "Duplicate spec epics", Status: Warn, Message: fmt.Sprintf("multiple epics resolve to spec %s: %s; keep one epic bound to this spec\nrecovery: bd show %s --json", idrender.Spec(id), strings.Join(bySpec[id], ", "), bySpec[id][0])})
		}
	}
	rows, err := c.OpenBeads()
	if err != nil {
		r.Checks = append(r.Checks, Check{Name: "Lineage", Status: Warn, Message: "cannot list open beads: " + termsafe.Escape(err.Error())})
		return
	}
	for _, row := range rows {
		if strings.EqualFold(row.IssueType, "epic") {
			continue
		}
		_, _, _, err := phase.FindEpicLineageWithCache(c, row.ID)
		if !errors.Is(err, phase.ErrNoEpicLineage) {
			continue
		}
		number, matched := lineageTitleNumber(row.Title)
		if !matched {
			continue
		}
		for _, epic := range epics {
			n, _ := phase.ExtractSpecMetadata(epic)
			if n != number {
				continue
			}
			r.Checks = append(r.Checks, Check{Name: "Title-only lineage: " + idrender.Bead(row.ID), Status: Warn, Message: fmt.Sprintf("bead %s has no parent-child edge to an epic; its title matches epic %s\nrecovery: bd update %s --parent %s", idrender.Bead(row.ID), idrender.Bead(epic.ID), idrender.Bead(row.ID), idrender.Bead(epic.ID))})
		}
	}
}
