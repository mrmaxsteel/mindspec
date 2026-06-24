package layout

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// stage names a durable checkpoint boundary in the mover's run-state machine
// (Req 4 / AC8). Every mutation boundary writes the current stage to the run
// state record so a crashed run can be resumed and so `mindspec doctor` can
// report progress. The "applied" terminal value is the doctor schema's OK
// stage (internal/doctor/migration.go).
type stage string

const (
	stagePreRun             stage = "pre-run"
	stageBeforeMv           stage = "before-mv"
	stageAfterMv            stage = "after-mv"
	stageAfterMoveCommit    stage = "after-move-commit"
	stageAfterRewrite       stage = "after-rewrite"
	stageAfterRewriteCommit stage = "after-rewrite-commit"
	stageRootRewrite        stage = "root-rewrite"
	stageFinalize           stage = "finalize"
	stageApplied            stage = "applied"
	stageLinkCheckFailed    stage = "link-check-failed"
)

// State is the durable per-run checkpoint record written under
// .mindspec/migrations/<run-id>/state.json. The Stage field is the
// doctor-schema field (migration.go's runState.Stage) so the record parses
// under the existing migration-metadata schema; the remaining fields drive
// crash-resume (PreRunRef for rollback, Published for the refuse-after-publish
// rule, Group/GroupStage for progress visibility).
type State struct {
	RunID      string `json:"run_id"`
	Stage      string `json:"stage"`
	PreRunRef  string `json:"pre_run_ref"`
	Published  bool   `json:"published"`
	Group      int    `json:"group"`
	GroupStage string `json:"group_stage"`
}

// LineageManifest is the move-provenance manifest written under
// .mindspec/lineage/manifest.json. Field names and shape match the doctor
// migration-metadata schema (internal/doctor/migration.go's lineageManifest /
// lineageManifestEntry) so a completed run's manifest parses under it (AC9).
type LineageManifest struct {
	RunID   string         `json:"run_id"`
	Entries []LineageEntry `json:"entries"`
}

// LineageEntry records one move group's source→canonical(dest) mapping. The
// Archive field is part of the doctor schema; the flatten archives nothing, so
// it is left empty.
type LineageEntry struct {
	Source    string `json:"source"`
	Canonical string `json:"canonical"`
	Archive   string `json:"archive"`
}

// runDir returns the per-run state directory .mindspec/migrations/<run-id>.
func runDir(root, runID string) string {
	return filepath.Join(root, ".mindspec", "migrations", runID)
}

// statePath returns the run's state.json path.
func statePath(root, runID string) string {
	return filepath.Join(runDir(root, runID), "state.json")
}

// lineageManifestPath returns the canonical lineage manifest path the doctor
// schema reads (.mindspec/lineage/manifest.json).
func lineageManifestPath(root string) string {
	return filepath.Join(root, ".mindspec", "lineage", "manifest.json")
}

// loadState reads the run-state record if it exists. The bool is false (with a
// nil error) when no record exists yet (a fresh run).
func loadState(root, runID string) (State, bool, error) {
	data, err := os.ReadFile(statePath(root, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, false, nil
		}
		return State{}, false, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, false, err
	}
	return s, true, nil
}

// writeJSON marshals v (indented) to path, creating parent dirs.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
