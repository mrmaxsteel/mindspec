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
			Message: "create docs_archive/<run-id>/... from migrate apply",
		})
	}

	manifestName := filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json"))
	if !hasLineage {
		r.Checks = append(r.Checks, Check{
			Name:    manifestName,
			Status:  Missing,
			Message: "run migrate apply to emit lineage manifest",
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
	required := []string{"inventory.json", "classification.json", "extraction.json", "plan.json", "plan.md", "validation.json", "state.json", "lineage.json", "apply.json"}
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
