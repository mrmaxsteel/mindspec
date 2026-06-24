package doctor

import (
	"path/filepath"
	"testing"
)

// TestLineageManifestParsesUnderSchema asserts that a lineage manifest +
// run-state record in the exact shape the spec-106 `migrate layout` mover
// writes (internal/layout) parses cleanly under the existing doctor
// migration-metadata schema (AC9). The mover writes:
//   - .mindspec/lineage/manifest.json: {run_id, entries:[{source,canonical,archive}]}
//   - .mindspec/migrations/<run-id>/state.json: {run_id, stage:"applied", ...}
//
// checkMigrationMetadata must read the manifest as OK (non-empty run_id +
// entries) and read the terminal stage as OK ("applied").
func TestLineageManifestParsesUnderSchema(t *testing.T) {
	root := t.TempDir()
	const runID = "run-106"

	manifest := `{
  "run_id": "run-106",
  "entries": [
    {"source": ".mindspec/docs/specs", "canonical": ".mindspec/specs", "archive": ""},
    {"source": ".mindspec/docs/adr", "canonical": ".mindspec/adr", "archive": ""}
  ]
}
`
	writeFileAt(t, root, filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json")), manifest)
	writeFileAt(t, root, filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.json")),
		`{"run_id":"run-106","stage":"applied","pre_run_ref":"abc123","published":false,"group":-1,"group_stage":""}`)

	r := &Report{}
	checkMigrationMetadata(r, root)

	manifestName := filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json"))
	stageName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.stage"))

	var manifestOK, stageOK bool
	for _, c := range r.Checks {
		if c.Name == manifestName {
			if c.Status != OK {
				t.Errorf("manifest check status = %v (%s), want OK", c.Status, c.Message)
			}
			manifestOK = true
		}
		if c.Name == stageName {
			if c.Status != OK || c.Message != "applied" {
				t.Errorf("stage check = {%v, %q}, want {OK, applied}", c.Status, c.Message)
			}
			stageOK = true
		}
	}
	if !manifestOK {
		t.Errorf("no manifest check emitted; checks=%+v", r.Checks)
	}
	if !stageOK {
		t.Errorf("no terminal-stage check emitted; checks=%+v", r.Checks)
	}
}
