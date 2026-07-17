package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// excludedSpecPathPrefixes lists path prefixes that the dry-run-migration
// reporter must NOT descend into (HC-4). MindSpec spec directories live
// under .mindspec/docs/specs/, never under viz/, agentmind/, or bench/;
// the guard makes the contract explicit so a future restructure that
// accidentally co-locates spec-shaped directories under those trees does
// not silently feed them through the reporter.
var excludedSpecPathPrefixes = []string{"viz/", "agentmind/", "bench/"}

// checkDryRunMigration walks the spec enumeration root and reports each
// legacy spec (one whose lifecycle epic lacks the mindspec_phase
// metadata key) that would migrate on its next lifecycle command.
//
// The walk root is TIER-AWARE (Spec 106 Bead 4): workspace.SpecsDir resolves
// the flat .mindspec/specs, canonical .mindspec/docs/specs, or legacy
// docs/specs root via first-exists-wins, so the reporter sees the same spec
// inventory on a flat tree as on a canonical one (no silent drop). For a
// canonical/legacy tree with no flat tree present this is byte-for-byte the
// pre-spec .mindspec/docs/specs path.
//
// Output: one Check per legacy spec, of the form
//
//	Check{
//	  Name:    "would-migrate: spec=<spec-id>",
//	  Status:  Warn,
//	  Message: "epic=<epic-id> phase=<derived>",
//	}
//
// Writes nothing. Per ADR-0034 and spec 089 Requirement 11, this
// reporter is the pre-mutation visibility surface for the auto-migrator
// in internal/phase/migrate.go. A single shared phase.Cache is used so
// the walk costs one bd list per epic-set rather than N show calls.
func checkDryRunMigration(r *Report, root string) {
	specsRoot := workspace.SpecsDir(root)
	entries, err := os.ReadDir(specsRoot)
	if err != nil {
		// No specs dir = nothing to migrate; reporter is a no-op.
		// Surfacing an Error here would conflate "no specs" with
		// "broken workspace" and would also trip HasFailures, which
		// violates spec 089 Requirement 11.
		return
	}

	// Stable order so the report is deterministic across runs.
	names := make([]string, 0, len(entries))
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		names = append(names, ent.Name())
	}
	sort.Strings(names)

	cache := phase.NewCache()
	for _, specID := range names {
		// HC-4 excluded-tree guard. The walk is rooted at
		// .mindspec/docs/specs/ which is itself outside the excluded
		// trees; this check is defensive against a future spec
		// directory naming that begins with one of the excluded
		// prefixes (e.g. a hypothetical `bench/<spec>` artifact
		// accidentally placed under the specs tree).
		skip := false
		for _, prefix := range excludedSpecPathPrefixes {
			if strings.HasPrefix(specID, strings.TrimSuffix(prefix, "/")+"-") ||
				strings.HasPrefix(specID, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		epicID, err := phase.FindEpicBySpecIDWithCache(cache, specID)
		if err != nil || epicID == "" {
			continue // no epic yet, nothing to migrate
		}

		epic, err := cache.FindEpic(epicID)
		if err != nil || epic == nil {
			continue
		}
		// Already-migrated (or post-080 native) epics carry
		// mindspec_phase in metadata. Skip without report.
		if epic.Metadata != nil {
			if raw, ok := epic.Metadata["mindspec_phase"]; ok {
				if s, ok := raw.(string); ok && s != "" {
					continue
				}
			}
		}

		children, _ := cache.GetChildren(epicID)
		derived := phase.DerivePhaseFromChildren(children)

		r.Checks = append(r.Checks, Check{
			Name:    fmt.Sprintf("would-migrate: spec=%s", specID),
			Status:  Warn,
			Message: fmt.Sprintf("epic=%s phase=%s", epicID, derived),
		})
	}
}

// checkLayout reports the detected docs layout (Spec 106 Bead 4 / Req 8),
// WARNs when a canonical/legacy tree would flatten on the next
// `mindspec migrate layout` (analogous to the would-migrate spec reporter), and
// ERRORs when the SAME spec id exists under two layouts — the stale-duplicate
// read hazard a half-migrated tree creates (the flat-first resolver would read
// the flat copy and silently mask the canonical/legacy one). It reuses the
// shared workspace.DetectLayout probe and writes nothing.
func checkLayout(r *Report, root string) {
	// DetectLayout returns the kind alongside any mixed-tree error; the kind is
	// what we report. The dual-layout-duplicate scan below is the precise
	// ERROR; the mixed error itself is surfaced through that scan + the kind.
	layout, _ := workspace.DetectLayout(root)
	r.Checks = append(r.Checks, Check{
		Name:    "layout",
		Status:  OK,
		Message: string(layout),
	})

	switch layout {
	case workspace.LayoutCanonical, workspace.LayoutLegacy:
		r.Checks = append(r.Checks, Check{
			Name:   "would-migrate-layout",
			Status: Warn,
			Message: fmt.Sprintf(
				"layout=%s would flatten to .mindspec/{specs,adr,domains,core} (+ context-map.md) on `mindspec migrate layout`",
				layout),
		})
	}

	for _, dup := range dualLayoutDuplicateSpecIDs(root) {
		r.Checks = append(r.Checks, Check{
			Name:   fmt.Sprintf("dual-layout-spec: %s", dup.id),
			Status: Error,
			Message: fmt.Sprintf(
				"spec %s exists under multiple layouts (%s) — remove the stale copy; the flat-first resolver would silently mask the others",
				dup.id, strings.Join(dup.layouts, ", ")),
		})
	}
}

// dualLayoutDup records a spec id found under more than one layout tier.
type dualLayoutDup struct {
	id      string
	layouts []string
}

// dualLayoutDuplicateSpecIDs returns the spec ids present under MORE THAN ONE
// of the three spec-tree roots — flat (.mindspec/specs), canonical
// (.mindspec/docs/specs), and legacy (docs/specs). It enumerates each root
// DIRECTLY (not via the first-exists-wins workspace.SpecsDir, which would
// surface only the highest-precedence one) so the stale-duplicate hazard is
// caught. Results are sorted by id for a deterministic report.
func dualLayoutDuplicateSpecIDs(root string) []dualLayoutDup {
	roots := []struct {
		layout string
		dir    string
	}{
		{"flat", filepath.Join(workspace.MindspecDir(root), "specs")},
		{"canonical", filepath.Join(workspace.CanonicalDocsDir(root), "specs")},
		{"legacy", filepath.Join(workspace.LegacyDocsDir(root), "specs")},
	}
	idLayouts := map[string][]string{}
	for _, rt := range roots {
		entries, err := os.ReadDir(rt.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				idLayouts[e.Name()] = append(idLayouts[e.Name()], rt.layout)
			}
		}
	}
	var dups []dualLayoutDup
	for id, layouts := range idLayouts {
		if len(layouts) >= 2 {
			dups = append(dups, dualLayoutDup{id: id, layouts: layouts})
		}
	}
	sort.Slice(dups, func(i, j int) bool { return dups[i].id < dups[j].id })
	return dups
}

// lineageManifest matches BOTH the global lineage manifest
// (.mindspec/lineage/manifest.json) and the per-run copy
// (.mindspec/migrations/<run-id>/lineage.json) written by the CURRENT
// `mindspec migrate layout` mover (internal/layout.LineageManifest /
// Mover.writeLineage). The retired classify/plan/apply pipeline (inventory,
// classification, extraction, plan, validation, apply, docs_archive/) is
// gone; this is the only lineage schema doctor validates.
type lineageManifest struct {
	RunID   string                 `json:"run_id"`
	Entries []lineageManifestEntry `json:"entries"`
}

type lineageManifestEntry struct {
	Source    string `json:"source"`
	Canonical string `json:"canonical"`
	Archive   string `json:"archive"`
}

// runState matches the per-run state.json record (internal/layout.State).
// Only Stage is read; extra fields (pre_run_ref, published, group, ...) are
// ignored unknown keys.
type runState struct {
	Stage string `json:"stage"`
}

// checkMigrationMetadata validates the CURRENT `mindspec migrate layout`
// contract: a completed run leaves exactly three durable artifacts — the
// global .mindspec/lineage/manifest.json, and per-run
// .mindspec/migrations/<run-id>/{lineage.json,state.json}. It no longer
// checks for docs_archive/ or the retired classify/plan/apply pipeline
// (inventory.json, classification.json, extraction.json, plan.json,
// plan.md, validation.json, apply.json) and emits no `migrate apply` hint.
//
// The check is ARMED by the global manifest OR any per-run
// lineage.json/state.json under .mindspec/migrations/<run-id>/, so deleting
// the global manifest out from under an otherwise-present run does not
// silence validation — it instead surfaces as a Missing finding for the
// manifest itself.
func checkMigrationMetadata(r *Report, root string) {
	lineagePath := filepath.Join(root, ".mindspec", "lineage", "manifest.json")
	hasGlobalManifest := fileExists(lineagePath)

	migrationsRoot := filepath.Join(root, ".mindspec", "migrations")
	evidenceRuns := runsWithEvidence(migrationsRoot)

	if !hasGlobalManifest && len(evidenceRuns) == 0 {
		return
	}

	manifestName := filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json"))

	var manifest lineageManifest
	manifestValid := false
	switch {
	case !hasGlobalManifest:
		r.Checks = append(r.Checks, Check{
			Name:    manifestName,
			Status:  Missing,
			Message: "run `mindspec migrate layout` to emit the lineage manifest",
		})
	default:
		if err := readJSONFile(lineagePath, &manifest); err != nil {
			r.Checks = append(r.Checks, Check{Name: manifestName, Status: Error, Message: err.Error()})
		} else if manifest.RunID == "" {
			r.Checks = append(r.Checks, Check{Name: manifestName, Status: Error, Message: "missing run_id"})
		} else if len(manifest.Entries) == 0 {
			r.Checks = append(r.Checks, Check{Name: manifestName, Status: Error, Message: "entries must not be empty"})
		} else {
			r.Checks = append(r.Checks, Check{
				Name:    manifestName,
				Status:  OK,
				Message: fmt.Sprintf("(run-id=%s, entries=%d)", manifest.RunID, len(manifest.Entries)),
			})
			manifestValid = true
		}
	}

	// Determine which run's per-run artifacts to validate: the manifest's
	// run_id when the global manifest is itself valid, otherwise the sole
	// run directory that armed this check (so a deleted/malformed global
	// manifest does not silence per-run validation of an otherwise-present
	// run).
	runID := ""
	switch {
	case manifestValid:
		runID = manifest.RunID
	case len(evidenceRuns) == 1:
		runID = evidenceRuns[0]
	}
	if runID == "" {
		return
	}

	runDir := filepath.Join(migrationsRoot, runID)

	// Per-run lineage.json: current schema, non-empty, and — when the
	// global manifest is itself valid — the SAME run_id as the manifest.
	lineageRunName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "lineage.json"))
	lineagePathRun := filepath.Join(runDir, "lineage.json")
	if !fileExists(lineagePathRun) {
		r.Checks = append(r.Checks, Check{
			Name:    lineageRunName,
			Status:  Missing,
			Message: "missing per-run lineage manifest",
		})
	} else {
		var runLineage lineageManifest
		if err := readJSONFile(lineagePathRun, &runLineage); err != nil {
			r.Checks = append(r.Checks, Check{Name: lineageRunName, Status: Error, Message: err.Error()})
		} else if runLineage.RunID == "" {
			r.Checks = append(r.Checks, Check{Name: lineageRunName, Status: Error, Message: "missing run_id"})
		} else if len(runLineage.Entries) == 0 {
			r.Checks = append(r.Checks, Check{Name: lineageRunName, Status: Error, Message: "entries must not be empty"})
		} else if manifestValid && runLineage.RunID != manifest.RunID {
			r.Checks = append(r.Checks, Check{
				Name:   lineageRunName,
				Status: Error,
				Message: fmt.Sprintf("run_id %q does not match manifest run_id %q",
					runLineage.RunID, manifest.RunID),
			})
		} else {
			r.Checks = append(r.Checks, Check{
				Name:    lineageRunName,
				Status:  OK,
				Message: fmt.Sprintf("(run-id=%s, entries=%d)", runLineage.RunID, len(runLineage.Entries)),
			})
		}
	}

	// Per-run state.json: current schema, parseable, and the terminal stage.
	statePathRun := filepath.Join(runDir, "state.json")
	stateName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.json"))
	stageName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.stage"))
	if !fileExists(statePathRun) {
		r.Checks = append(r.Checks, Check{
			Name:    stateName,
			Status:  Missing,
			Message: "missing run-state record",
		})
		return
	}

	var state runState
	if err := readJSONFile(statePathRun, &state); err != nil {
		r.Checks = append(r.Checks, Check{Name: stateName, Status: Error, Message: err.Error()})
		return
	}
	r.Checks = append(r.Checks, Check{Name: stateName, Status: OK})

	switch state.Stage {
	case "applied":
		r.Checks = append(r.Checks, Check{Name: stageName, Status: OK, Message: state.Stage})
	case "":
		r.Checks = append(r.Checks, Check{Name: stageName, Status: Warn, Message: "stage missing"})
	default:
		r.Checks = append(r.Checks, Check{
			Name:    stageName,
			Status:  Warn,
			Message: fmt.Sprintf("run in progress (stage=%s)", state.Stage),
		})
	}
}

// runsWithEvidence returns the run-id directory names directly under
// migrationsRoot that contain a lineage.json or state.json — i.e. current-run
// evidence that a deleted/malformed global manifest must not be allowed to
// silence. Sorted for determinism.
func runsWithEvidence(migrationsRoot string) []string {
	entries, err := os.ReadDir(migrationsRoot)
	if err != nil {
		return nil
	}
	var runs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(migrationsRoot, e.Name())
		if fileExists(filepath.Join(dir, "lineage.json")) || fileExists(filepath.Join(dir, "state.json")) {
			runs = append(runs, e.Name())
		}
	}
	sort.Strings(runs)
	return runs
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
