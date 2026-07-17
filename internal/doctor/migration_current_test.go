package doctor

// migration_current_test.go — Bead 3 of spec 118 (mindspec-qqv1.3): tests for
// the rekeyed checkMigrationMetadata contract (AC-7, AC-7b, AC-24). The
// retired classify/plan/apply pipeline (inventory.json, classification.json,
// extraction.json, plan.json, plan.md, validation.json, apply.json,
// docs_archive/) is gone; only the CURRENT `mindspec migrate layout` mover's
// three artifacts are validated: the global .mindspec/lineage/manifest.json
// plus per-run .mindspec/migrations/<run-id>/{lineage.json,state.json}
// (internal/layout: Mover.writeLineage + runstate.go).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// obsoleteArtifactNames are the retired classify/plan/apply pipeline
// artifacts checkMigrationMetadata must never mention again.
var obsoleteArtifactNames = []string{
	"inventory.json",
	"classification.json",
	"extraction.json",
	"plan.json",
	"plan.md",
	"validation.json",
	"apply.json",
}

// writeCurrentMigrationFixture writes the three CURRENT-contract migration
// artifacts under root: the global manifest and the per-run lineage.json +
// state.json (with the given stage). All three carry the same runID and a
// single matching lineage entry.
func writeCurrentMigrationFixture(t *testing.T, root, runID, stage string) {
	t.Helper()

	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entry := `{"source": ".mindspec/docs/specs", "canonical": ".mindspec/specs", "archive": ""}`

	write(".mindspec/lineage/manifest.json", `{
  "run_id": "`+runID+`",
  "entries": [`+entry+`]
}
`)
	write(".mindspec/migrations/"+runID+"/lineage.json", `{
  "run_id": "`+runID+`",
  "entries": [`+entry+`]
}
`)
	write(".mindspec/migrations/"+runID+"/state.json", `{"run_id": "`+runID+`", "stage": "`+stage+`"}`)
}

// TestCheckMigrationMetadata_CurrentCompletedRun is B3-V1 (AC-7): a completed
// current-run fixture (global manifest + per-run lineage.json + state.json
// stage "applied") is healthy — no Missing/Error findings, and no mention of
// docs_archive/, any obsolete artifact, or a `migrate apply` hint anywhere in
// the report.
func TestCheckMigrationMetadata_CurrentCompletedRun(t *testing.T) {
	root := t.TempDir()
	const runID = "run-cur"
	writeCurrentMigrationFixture(t, root, runID, "applied")

	r := &Report{}
	checkMigrationMetadata(r, root)

	if r.HasFailures() {
		t.Fatalf("expected a healthy completed run, got failures: %+v", r.Checks)
	}

	manifestName := filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json"))
	lineageName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "lineage.json"))
	stateName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.json"))
	stageName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.stage"))

	var sawManifest, sawLineage, sawState, sawStage bool
	for _, c := range r.Checks {
		switch c.Name {
		case manifestName:
			sawManifest = true
			if c.Status != OK {
				t.Errorf("manifest check status = %v, want OK (%s)", c.Status, c.Message)
			}
		case lineageName:
			sawLineage = true
			if c.Status != OK {
				t.Errorf("per-run lineage check status = %v, want OK (%s)", c.Status, c.Message)
			}
		case stateName:
			sawState = true
			if c.Status != OK {
				t.Errorf("state.json check status = %v, want OK (%s)", c.Status, c.Message)
			}
		case stageName:
			sawStage = true
			if c.Status != OK || c.Message != "applied" {
				t.Errorf("stage check = {%v, %q}, want {OK, applied}", c.Status, c.Message)
			}
		}

		if strings.Contains(c.Name, "docs_archive") || strings.Contains(c.Message, "docs_archive") {
			t.Errorf("unexpected docs_archive/ reference in check: %+v", c)
		}
		if strings.Contains(c.Message, "migrate apply") {
			t.Errorf("unexpected `migrate apply` hint in check: %+v", c)
		}
		for _, obsolete := range obsoleteArtifactNames {
			if strings.Contains(c.Name, obsolete) {
				t.Errorf("unexpected obsolete-artifact finding %q", c.Name)
			}
		}
	}

	if !sawManifest || !sawLineage || !sawState || !sawStage {
		t.Fatalf("missing expected current-contract checks; sawManifest=%v sawLineage=%v sawState=%v sawStage=%v; checks=%+v",
			sawManifest, sawLineage, sawState, sawStage, r.Checks)
	}
}

// TestCheckMigrationMetadata_CurrentContractMutations is B3-V2/B3-V3
// (AC-7b, AC-24): table-driven mutations of the AC-7 completed-run fixture.
// Every mutation of a required artifact (malformed manifest/state/lineage,
// lineage run_id mismatch against the manifest, or removal of any required
// artifact) must emit an Error or Missing finding, with doctor's HasFailures
// true. The lone exception is a parseable non-"applied" stage (e.g.
// "finalize"), which must remain Warn/non-fatal (HasFailures false) and must
// never be reported as healthy/completed/applied.
func TestCheckMigrationMetadata_CurrentContractMutations(t *testing.T) {
	const runID = "run-cur"

	newFixture := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		writeCurrentMigrationFixture(t, root, runID, "applied")
		return root
	}

	cases := []struct {
		name   string
		mutate func(t *testing.T, root string)
		wantFn func(t *testing.T, r *Report)
	}{
		{
			name: "malformed_manifest",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "lineage", "manifest.json")
				if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectFailure,
		},
		{
			name: "malformed_state_json",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "state.json")
				if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectFailure,
		},
		{
			name: "malformed_per_run_lineage",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "lineage.json")
				if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectFailure,
		},
		{
			name: "lineage_run_id_mismatch",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "lineage.json")
				if err := os.WriteFile(path, []byte(`{
  "run_id": "some-other-run",
  "entries": [{"source": ".mindspec/docs/specs", "canonical": ".mindspec/specs", "archive": ""}]
}
`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectFailure,
		},
		{
			// O2: pins that a valid-parse global manifest with an empty
			// run_id is still rejected — removing the code guard at
			// migration.go (manifest.RunID == "") must turn this row RED.
			// Asserts the SPECIFIC manifest check is Error (not just that
			// SOME failure exists downstream), so a coincidental cascade
			// failure elsewhere can't mask the guard's removal.
			name: "manifest_empty_run_id",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "lineage", "manifest.json")
				content := `{
  "run_id": "",
  "entries": [{"source": ".mindspec/docs/specs", "canonical": ".mindspec/specs", "archive": ""}]
}
`
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectCheckStatus(
				filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json")),
				Error,
			),
		},
		{
			// O2: pins that a valid-parse global manifest with empty
			// entries is still rejected — removing the code guard at
			// migration.go (len(manifest.Entries) == 0) must turn this
			// row RED.
			name: "manifest_empty_entries",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "lineage", "manifest.json")
				content := `{
  "run_id": "` + runID + `",
  "entries": []
}
`
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectCheckStatus(
				filepath.ToSlash(filepath.Join(".mindspec", "lineage", "manifest.json")),
				Error,
			),
		},
		{
			// O2: pins that a valid-parse per-run lineage.json with an
			// empty run_id is still rejected — removing the code guard
			// (runLineage.RunID == "") must turn this row RED. Asserts the
			// SPECIFIC per-run lineage check is Error, not just that some
			// failure exists.
			//
			// The global manifest is ALSO removed here (leaving the sole
			// evidence run to arm validation with manifestValid=false):
			// with a valid global manifest present, the downstream
			// run_id-mismatch guard ("" != manifest's non-empty run_id)
			// would independently catch this mutation and mask whether
			// the empty-run_id guard itself was doing any work. Dropping
			// the manifest isolates the guard under test.
			name: "per_run_lineage_empty_run_id",
			mutate: func(t *testing.T, root string) {
				manifestPath := filepath.Join(root, ".mindspec", "lineage", "manifest.json")
				if err := os.Remove(manifestPath); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, ".mindspec", "migrations", runID, "lineage.json")
				content := `{
  "run_id": "",
  "entries": [{"source": ".mindspec/docs/specs", "canonical": ".mindspec/specs", "archive": ""}]
}
`
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectCheckStatus(
				filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "lineage.json")),
				Error,
			),
		},
		{
			// O2: pins that a valid-parse per-run lineage.json with empty
			// entries is still rejected — removing the code guard
			// (len(runLineage.Entries) == 0) must turn this row RED.
			name: "per_run_lineage_empty_entries",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "lineage.json")
				content := `{
  "run_id": "` + runID + `",
  "entries": []
}
`
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectCheckStatus(
				filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "lineage.json")),
				Error,
			),
		},
		{
			// S3: an ambiguous multi-run state (no valid global manifest
			// to disambiguate, ≥2 evidence-bearing run directories) must
			// not silently skip validation of ALL of them. This seeds a
			// second run directory with its own evidence and corrupts the
			// ORIGINAL run's state.json; the hardened checkMigrationMetadata
			// must still validate every candidate run and surface an Error
			// for the corrupted one specifically. If the multi-run
			// hardening is reverted (runID stays "" and the function
			// returns without validating any run when the manifest is
			// invalid/absent and more than one evidence run exists), no
			// Error check for this run's state.json is emitted and the row
			// goes RED.
			name: "multi_run_ambiguous_one_corrupt",
			mutate: func(t *testing.T, root string) {
				manifestPath := filepath.Join(root, ".mindspec", "lineage", "manifest.json")
				if err := os.Remove(manifestPath); err != nil {
					t.Fatal(err)
				}

				secondRunDir := filepath.Join(root, ".mindspec", "migrations", "run-second")
				if err := os.MkdirAll(secondRunDir, 0o755); err != nil {
					t.Fatal(err)
				}
				secondState := `{"run_id": "run-second", "stage": "applied"}`
				if err := os.WriteFile(filepath.Join(secondRunDir, "state.json"), []byte(secondState), 0o644); err != nil {
					t.Fatal(err)
				}

				corruptStatePath := filepath.Join(root, ".mindspec", "migrations", runID, "state.json")
				if err := os.WriteFile(corruptStatePath, []byte("{not valid json"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: func(t *testing.T, r *Report) {
				if !r.HasFailures() {
					t.Fatalf("expected an Error/Missing finding, got: %+v", r.Checks)
				}
				stateName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.json"))
				var found bool
				for _, c := range r.Checks {
					if c.Name == stateName && c.Status == Error {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected an Error finding for the corrupted run's state.json (ambiguous multi-run must validate every evidence run, not skip all); checks=%+v", r.Checks)
				}
			},
		},
		{
			name: "missing_global_manifest",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "lineage", "manifest.json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectFailure,
		},
		{
			name: "missing_per_run_lineage",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "lineage.json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectFailure,
		},
		{
			name: "missing_per_run_state",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "state.json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectFailure,
		},
		{
			// B3-V3 / AC-24: a parseable non-empty non-"applied" stage
			// (an in-progress run) is Warn/non-fatal, exit 0, and MUST NOT
			// be reported as healthy/completed/applied.
			name: "non_applied_finalize_warns",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "state.json")
				if err := os.WriteFile(path, []byte(`{"run_id": "`+runID+`", "stage": "finalize"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: func(t *testing.T, r *Report) {
				if r.HasFailures() {
					t.Fatalf("expected non-applied stage to remain non-fatal, got failures: %+v", r.Checks)
				}
				stageName := filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.stage"))
				var found bool
				for _, c := range r.Checks {
					if c.Name != stageName {
						continue
					}
					found = true
					if c.Status != Warn {
						t.Errorf("stage check status = %v, want Warn", c.Status)
					}
					lower := strings.ToLower(c.Message)
					for _, forbidden := range []string{"healthy", "completed", "applied"} {
						if strings.Contains(lower, forbidden) {
							t.Errorf("stage message must not claim %q: %q", forbidden, c.Message)
						}
					}
				}
				if !found {
					t.Fatalf("expected a stage check for the in-progress run; checks=%+v", r.Checks)
				}
			},
		},
		{
			// G1 (spec 118 final-review): the mover never writes an empty
			// stage (every internal/layout/runstate.go stage constant is
			// non-empty), so a parseable state.json with stage "" is a
			// malformed/incomplete artifact, not a benign in-progress run.
			// It must be Error (HasFailures true, nonzero exit), consistent
			// with the empty-run_id/empty-entries manifest/lineage guards
			// above — not the Warn treatment reserved for a genuine
			// non-empty in-progress stage (see non_applied_finalize_warns).
			name: "empty_stage_is_error",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, ".mindspec", "migrations", runID, "state.json")
				if err := os.WriteFile(path, []byte(`{"run_id": "`+runID+`", "stage": ""}`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFn: expectCheckStatus(
				filepath.ToSlash(filepath.Join(".mindspec", "migrations", runID, "state.stage")),
				Error,
			),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := newFixture(t)
			tc.mutate(t, root)

			r := &Report{}
			checkMigrationMetadata(r, root)
			tc.wantFn(t, r)
		})
	}
}

// expectFailure asserts the report carries at least one Error or Missing
// finding (HasFailures true) — the shared assertion for every AC-7b mutation
// row.
func expectFailure(t *testing.T, r *Report) {
	t.Helper()
	if !r.HasFailures() {
		t.Fatalf("expected an Error/Missing finding, got: %+v", r.Checks)
	}
}

// expectCheckStatus returns a wantFn that asserts the Check with the given
// name has the given status. Unlike expectFailure's generic HasFailures
// check, this pins the failure to a SPECIFIC finding — necessary for the
// empty-run_id/empty-entries guard rows (O2 fix-up), where a removed guard
// can still leave HasFailures true via an unrelated downstream cascade
// (e.g. an empty run_id resolving to a nonexistent per-run directory,
// which independently reports Missing artifacts). Asserting the exact
// Check/status ensures the row goes RED specifically when the guard under
// test is reverted, not merely when some other, coincidental failure path
// fires.
func expectCheckStatus(name string, status Status) func(t *testing.T, r *Report) {
	return func(t *testing.T, r *Report) {
		t.Helper()
		for _, c := range r.Checks {
			if c.Name == name {
				if c.Status != status {
					t.Fatalf("check %q status = %v, want %v (message=%q)", name, c.Status, status, c.Message)
				}
				return
			}
		}
		t.Fatalf("expected a check named %q, got: %+v", name, r.Checks)
	}
}
