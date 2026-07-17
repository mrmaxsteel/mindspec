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
