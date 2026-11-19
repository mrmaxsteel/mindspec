package kernelid

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

// Witnesses contains the independent enrolment witnesses.
type Witnesses struct {
	Epic         string
	RunPresent   bool
	IDs          []string
	Keys         []string
	Orchestrated bool
}

// Enrolled returns enrolment and whether its pinned policy is missing.
func Enrolled(w Witnesses) (enrolled, markerMissing bool) {
	enrolled = w.RunPresent || w.Orchestrated
	for _, id := range append(append([]string{}, w.IDs...), w.Keys...) {
		if UnderEpic(id, w.Epic) {
			enrolled = true
		}
	}
	return enrolled, enrolled && !w.RunPresent
}

// PanelOrchestrated reads panel witnesses below a spec directory.
func PanelOrchestrated(specDir string) (bool, error) {
	found := false
	err := filepath.WalkDir(specDir, func(path string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "panel.json" {
			return nil
		}
		data, err := os.ReadFile(path) // #nosec G122 -- These same-user repository witnesses are not a filesystem access boundary.
		if err != nil {
			return err
		}
		var p struct {
			Orchestrated bool `json:"orchestrated"`
		}
		if err = json.Unmarshal(data, &p); err != nil {
			return err
		}
		found = found || p.Orchestrated
		return nil
	})
	return found, err
}
