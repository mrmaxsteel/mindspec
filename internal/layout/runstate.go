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

	// PlanResolved records that the full move plan (the static flatten/dogfood
	// groups PLUS the content-aware review-co-location groups) has been computed
	// and frozen into Plan. A resumed run reuses the FROZEN plan rather than
	// re-deriving the review groups from a partially-moved tree (where the
	// review/<slug> sources have already moved), keeping the group→checkpoint
	// index space stable across a crash-resume. These fields are extra to the
	// doctor migration-metadata schema (migration.go reads only Stage), so they
	// parse there as ignored unknown keys.
	PlanResolved   bool        `json:"plan_resolved,omitempty"`
	Plan           []MoveGroup `json:"plan,omitempty"`
	SkippedReviews []string    `json:"skipped_reviews,omitempty"`
}

// LineageManifest is the move-provenance manifest written under
// .mindspec/lineage/manifest.json. Field names and shape match the doctor
// migration-metadata schema (internal/doctor/migration.go's lineageManifest /
// lineageManifestEntry) so a completed run's manifest parses under it (AC9).
type LineageManifest struct {
	RunID   string         `json:"run_id"`
	Entries []LineageEntry `json:"entries"`
	// Skipped lists the repo-root review/<slug> directories the
	// review-co-location step could NOT attribute to a spec (no panel.json
	// `spec`, no spec-id slug prefix, or a loose non-directory entry) and
	// therefore left in place rather than failing the run. It is provenance,
	// not a move; the doctor schema ignores the unknown key.
	Skipped []string `json:"skipped,omitempty"`
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

// IsResumable reports whether a run-state record for runID exists and has NOT
// reached the terminal "applied" stage — i.e. a re-run should RESUME it rather
// than start fresh (panel R5). The migrate-layout CLI consults this BEFORE
// enforcing the fresh-run clean-tree precondition, so a crash AFTER the
// rewrite-but-before-commit step (which leaves dirty moved markdown) resumes to
// completion instead of being refused by the clean-tree check. An absent record
// is not resumable (a fresh run).
func IsResumable(root, runID string) (bool, error) {
	s, found, err := loadState(root, runID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	return s.Stage != string(stageApplied), nil
}

// FindResumableRun scans .mindspec/migrations/<run-id>/ for an in-progress
// (non-terminal) run to resume when the operator did not pass an explicit
// --run-id (panel R5). Run-id directory names are UTC timestamps, so the
// lexical max is the newest; the newest resumable run wins. A
// terminal/absent/unreadable run is skipped. Returns ("", false, nil) when no
// resumable run exists.
func FindResumableRun(root string) (runID string, found bool, err error) {
	migrationsDir := filepath.Join(root, ".mindspec", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	best := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, ok, lerr := loadState(root, e.Name())
		if lerr != nil || !ok { // unreadable/unparseable/absent → not a live recovery
			continue
		}
		if s.Stage == "" || s.Stage == string(stageApplied) {
			continue
		}
		if e.Name() > best {
			best = e.Name()
		}
	}
	return best, best != "", nil
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
