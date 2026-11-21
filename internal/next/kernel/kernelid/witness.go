package kernelid

import (
	"encoding/json"
	"fmt"
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
// A valid witness takes priority over parse errors in other files.
// Without a witness, it returns the first parse error with its file path.
func PanelOrchestrated(specDir string) (bool, error) {
	found := false
	var parseErr error
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
			if parseErr == nil {
				parseErr = fmt.Errorf("parse %s: %w", path, err)
			}
			return nil
		}
		found = found || p.Orchestrated
		return nil
	})
	if found {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, parseErr
}
