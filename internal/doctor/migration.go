package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type lineageManifest struct {
	RunID   string                 `json:"run_id"`
	Entries []lineageManifestEntry `json:"entries"`
}

type lineageManifestEntry struct {
	Source    string `json:"source"`
	Canonical string `json:"canonical"`
	Archive   string `json:"archive"`
}

type runState struct {
	Stage string `json:"stage"`
}

func checkMigrationMetadata(r *Report, root string) {
	archiveDir := filepath.Join(root, "docs_archive")
	lineagePath := filepath.Join(root, ".mindspec", "lineage", "manifest.json")

	hasArchive := dirExists(archiveDir)
	hasLineage := fileExists(lineagePath)
	if !hasArchive && !hasLineage {
		return
	}

	if hasArchive {
		r.Checks = append(r.Checks, Check{Name: "docs_archive/", Status: OK})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "docs_archive/",
			Status:  Missing,
			Message: "create docs_archive/<run-id>/... from brownfield apply",
		})
	}

	manifestName := filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json"))
	if !hasLineage {
		r.Checks = append(r.Checks, Check{
			Name:    manifestName,
			Status:  Missing,
			Message: "run brownfield apply to emit lineage manifest",
		})
		return
	}

	var manifest lineageManifest
	if err := readJSONFile(lineagePath, &manifest); err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    manifestName,
			Status:  Error,
			Message: err.Error(),
		})
		return
	}
	if manifest.RunID == "" {
		r.Checks = append(r.Checks, Check{
			Name:    manifestName,
			Status:  Error,
			Message: "missing run_id",
		})
		return
	}
	if len(manifest.Entries) == 0 {
		r.Checks = append(r.Checks, Check{
			Name:    manifestName,
			Status:  Error,
			Message: "entries must not be empty",
		})
		return
	}
	r.Checks = append(r.Checks, Check{
		Name:    manifestName,
		Status:  OK,
		Message: fmt.Sprintf("(run-id=%s, entries=%d)", manifest.RunID, len(manifest.Entries)),
	})

	runDir := filepath.Join(root, ".mindspec", "migrations", manifest.RunID)
	required := []string{"inventory.json", "classification.json", "state.json", "lineage.json"}
	for _, name := range required {
		path := filepath.Join(runDir, name)
		checkName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", manifest.RunID, name))
		if fileExists(path) {
			r.Checks = append(r.Checks, Check{Name: checkName, Status: OK})
			continue
		}
		r.Checks = append(r.Checks, Check{
			Name:    checkName,
			Status:  Missing,
			Message: "missing migration checkpoint artifact",
		})
	}

	statePath := filepath.Join(runDir, "state.json")
	if !fileExists(statePath) {
		return
	}

	var state runState
	if err := readJSONFile(statePath, &state); err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    filepath.ToSlash(filepath.Join(".mindspec", "migrations", manifest.RunID, "state.stage")),
			Status:  Error,
			Message: err.Error(),
		})
		return
	}

	stageName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", manifest.RunID, "state.stage"))
	switch state.Stage {
	case "applied":
		r.Checks = append(r.Checks, Check{Name: stageName, Status: OK, Message: state.Stage})
	case "":
		r.Checks = append(r.Checks, Check{Name: stageName, Status: Warn, Message: "stage missing"})
	default:
		r.Checks = append(r.Checks, Check{Name: stageName, Status: Warn, Message: state.Stage})
	}
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.ToSlash(path), err)
	}
	return nil
}
