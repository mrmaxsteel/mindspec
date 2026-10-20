package main

// panel_disposition_store_test.go: spec 117 final-review fixup — CLI
// integration tests for `mindspec panel disposition append`:
//   - FR-3: the leaf DERIVES the canonical content-hash id, overriding any
//     operator-supplied (or absent) id.
//   - M4: a hostile --panel / --spec is rejected by the leaf's gating
//     (validatePanelSlug / idvalidate.SpecID), so removing that gating
//     reddens a test.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// mkDispositionRoot creates a `.mindspec`-marked workspace root with a
// pre-existing flat spec directory (.mindspec/specs/<specID>) so
// workspace.SpecDir resolves the append destination, chdirs into it, and
// returns the root.
func mkDispositionRoot(t *testing.T, specID string) string {
	t.Helper()
	root := mkPanelTestRoot(t, "")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs", specID), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	withTestChdir(t, root)
	return root
}

// runDispositionAppend runs `mindspec panel disposition append` with the
// given flags via the global rootCmd, returning its combined error.
func runDispositionAppend(t *testing.T, specID, panelName, data string) error {
	t.Helper()
	var out, errBuf bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"panel", "disposition", "append", "--spec", specID, "--panel", panelName, "--data", data})
	return rootCmd.Execute()
}

// readStoredRow reads the single stored disposition line under
// <root>/.mindspec/specs/<specID>/reviews/<panel>/dispositions.jsonl and
// unmarshals it.
func readStoredRow(t *testing.T, root, specID, panelName string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(root, ".mindspec", "specs", specID, "reviews", panelName, "dispositions.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading stored store %s: %v", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		t.Fatalf("store %s is empty", path)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(trimmed, &m); err != nil {
		t.Fatalf("unmarshaling stored row %q: %v", trimmed, err)
	}
	return m
}

// TestPanelDispositionAppend_DerivesCanonicalID proves FR-3: whether the
// operator supplies a WRONG id or omits it entirely, the stored row's id
// is the CLI-derived panel.DispositionRowID(spec,panel,reviewer,summary),
// so live capture enforces R2's stable-content-id (and R6
// retry-idempotency) automatically.
func TestPanelDispositionAppend_DerivesCanonicalID(t *testing.T) {
	const specID = "117-panel-review-telemetry"
	const panelName = "p117-fr3"
	const spec, reviewer, summary = "117", "S1", "escape unpinned — mutation proven"
	wantID := panel.DispositionRowID(spec, panelName, reviewer, summary)

	rowFmt := `{"record":"disposition",%s"spec":"117","gate":"bead","panel":"p117-fr3","reviewer":"S1","model":"sonnet","severity":"major","summary":"escape unpinned — mutation proven","convergent_with":[],"disposition":"confirmed-fixed","created_at":"2026-07-20T00:00:00Z","backfilled":false}`

	cases := []struct {
		name string
		idKV string
	}{
		{"wrong-id", `"id":"d-WRONGWRONGWRONG",`},
		{"absent-id", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := mkDispositionRoot(t, specID)
			data := strings.Replace(rowFmt, "%s", tc.idKV, 1)
			if err := runDispositionAppend(t, specID, panelName, data); err != nil {
				t.Fatalf("append: %v", err)
			}
			m := readStoredRow(t, root, specID, panelName)
			gotID, _ := m["id"].(string)
			if gotID != wantID {
				t.Errorf("stored id = %q, want the CLI-derived canonical id %q (operator id must be overridden)", gotID, wantID)
			}
			if gotID == "d-WRONGWRONGWRONG" {
				t.Errorf("stored id is the operator-supplied wrong id; the CLI must override it")
			}
		})
	}
}

// TestPanelDispositionAppend_HostileFlagsRejected proves M4: the leaf
// rejects a hostile --panel (validatePanelSlug) and a hostile --spec
// (idvalidate.SpecID, via resolveDispositionSpecDir → workspace.SpecDir),
// and writes nothing. Removing either gate reddens this test.
func TestPanelDispositionAppend_HostileFlagsRejected(t *testing.T) {
	const specID = "117-panel-review-telemetry"
	// A schema-valid row so the ONLY thing that can reject the append is the
	// slug/spec gating, not the record content.
	const validRow = `{"record":"disposition","spec":"117","gate":"bead","panel":"p","reviewer":"S1","model":"sonnet","severity":"major","summary":"x","convergent_with":[],"disposition":"confirmed-fixed","created_at":"2026-07-20T00:00:00Z","backfilled":false}`

	t.Run("hostile-panel", func(t *testing.T) {
		mkDispositionRoot(t, specID)
		if err := runDispositionAppend(t, specID, "../evil", validRow); err == nil {
			t.Fatal("append with hostile --panel = nil, want a rejection (path-separator slug)")
		}
	})

	t.Run("hostile-spec", func(t *testing.T) {
		mkDispositionRoot(t, specID)
		if err := runDispositionAppend(t, "../evil", "p117-m4", validRow); err == nil {
			t.Fatal("append with hostile --spec = nil, want a rejection (invalid spec id)")
		}
	})
}

func ledgerCLIFixtures(t *testing.T, name string) [][]byte {
	t.Helper()
	const timestamp = "2026-10-20T10:00:00Z"
	ref := panel.Refutation{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), FindingText: "blocking item", Ground: "disproven", Reason: "covered", Evidence: "abc123", By: "operator", CreatedAt: timestamp}
	values := []interface{}{
		panel.RoundRecord{Record: panel.RecordRound, Spec: "130", Panel: name, Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:baseline", Target: "main", Gate: "final_review", Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}}, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: timestamp},
		panel.RefutationRecord{Record: panel.RecordRefutation, Spec: "130", Panel: name, Refutation: ref},
		panel.DeferralRecord{Record: panel.RecordDeferral, Spec: "130", Panel: name, Deferral: panel.Deferral{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: strings.Repeat("b", 64), FindingText: "blocking item", Bead: "mindspec-followup", By: "operator", CreatedAt: timestamp}},
		panel.RebindRecord{Record: panel.RecordRebind, Spec: "130", Panel: name, SubjectDigest: "sha256:baseline", Rebind: panel.Rebind{Round: 1, FromSHA: "abc123", ToSHA: "def456", Paths: []string{"OWNERSHIP.yaml"}, Reason: "claim", CreatedAt: timestamp}},
		panel.AbandonRecord{Record: panel.RecordAbandon, Spec: "130", Panel: name, Reason: "halt", By: "operator", CreatedAt: timestamp},
	}
	records := make([][]byte, len(values))
	for i, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		records[i] = data
	}
	return records
}

func runLedgerDisposition(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(append([]string{"panel", "disposition"}, args...))
	err := rootCmd.Execute()
	return out.String(), err
}

func TestDispositionValidate_LedgerKinds(t *testing.T) {
	mkDispositionRoot(t, "130-panel-substrate-integrity")
	for _, raw := range ledgerCLIFixtures(t, "p") {
		var disc struct {
			Record string `json:"record"`
		}
		if err := json.Unmarshal(raw, &disc); err != nil {
			t.Fatal(err)
		}
		t.Run(disc.Record, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "dispositions.jsonl")
			if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := runLedgerDisposition(t, "validate", path)
			if err != nil || !strings.Contains(out, "all lines valid") {
				t.Fatalf("validate failed: %v %s", err, out)
			}
			var bad map[string]interface{}
			if err := json.Unmarshal(raw, &bad); err != nil {
				t.Fatal(err)
			}
			delete(bad, "spec")
			data, err := json.Marshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := runLedgerDisposition(t, "validate", path); err == nil {
				t.Fatal("validate accepted malformed event")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "unknown.jsonl")
	if err := os.WriteFile(path, []byte(`{"record":"unknown","spec":"130","panel":"p"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runLedgerDisposition(t, "validate", path); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestDispositionCheck_LedgerKinds(t *testing.T) {
	const specID = "130-panel-substrate-integrity"
	root := mkDispositionRoot(t, specID)
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	dir := filepath.Join(specDir, "reviews", "p")
	records := ledgerCLIFixtures(t, "p")
	// An event never substitutes for the required disposition coverage row.
	manifest := []byte(`{"record":"panel","spec":"130","gate":"bead","panel":"p","round":1,"slots":[{"slot":"R1","model":"model-a","verdict":"REQUEST_CHANGES"}],"backfilled":false}`)
	records = append(records, manifest)
	if err := panel.AppendRecords(dir, records...); err != nil {
		t.Fatal(err)
	}
	if _, err := runLedgerDisposition(t, "check", "--spec", specID, "--panel", "p"); err == nil || !strings.Contains(err.Error(), "R1") {
		t.Fatalf("missing disposition did not fail: %v", err)
	}
	row := []byte(`{"record":"disposition","id":"d-check","spec":"130","gate":"bead","panel":"p","reviewer":"R1","model":"model-a","severity":"blocking","summary":"blocking item","convergent_with":[],"disposition":"audited-refuted","created_at":"2026-10-20T10:00:00Z","backfilled":false}`)
	if err := panel.AppendRecords(dir, row); err != nil {
		t.Fatal(err)
	}
	out, err := runLedgerDisposition(t, "check", "--spec", specID, "--panel", "p")
	if err != nil || !strings.Contains(out, "completeness floor satisfied") {
		t.Fatalf("mixed check failed: %v %s", err, out)
	}
	good, err := os.ReadFile(filepath.Join(dir, "dispositions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range ledgerCLIFixtures(t, "p") {
		var bad map[string]interface{}
		if err := json.Unmarshal(raw, &bad); err != nil {
			t.Fatal(err)
		}
		t.Run(bad["record"].(string), func(t *testing.T) {
			bad["created_at"] = "invalid"
			data, err := json.Marshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "dispositions.jsonl"), append(append([]byte(nil), good...), data...), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := runLedgerDisposition(t, "check", "--spec", specID, "--panel", "p"); err == nil {
				t.Fatal("check ignored malformed event")
			}
		})
	}
}

func TestDispositionAppend_RefusesVerbKinds(t *testing.T) {
	const specID = "130-panel-substrate-integrity"
	for _, raw := range ledgerCLIFixtures(t, "p") {
		var disc struct {
			Record string `json:"record"`
		}
		if err := json.Unmarshal(raw, &disc); err != nil {
			t.Fatal(err)
		}
		t.Run(disc.Record, func(t *testing.T) {
			root := mkDispositionRoot(t, specID)
			// Establish schema validity, so refusal cannot pass for malformed input.
			if err := panel.Validate(raw); err != nil {
				t.Fatal(err)
			}
			if err := panel.HygienePredicate(raw); err != nil {
				t.Fatal(err)
			}
			err := runDispositionAppend(t, specID, "p", string(raw))
			if err == nil || !strings.Contains(err.Error(), "reserved to the panel verbs") {
				t.Fatalf("append must refuse verb kind %s: %v", disc.Record, err)
			}
			path := filepath.Join(root, ".mindspec", "specs", specID, "reviews", "p", "dispositions.jsonl")
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("refusal wrote ledger: %v", err)
			}
		})
	}
	t.Run("manifest-non-legacy", func(t *testing.T) {
		root := mkDispositionRoot(t, specID)
		dir := filepath.Join(root, ".mindspec", "specs", specID, "reviews", "p")
		round := ledgerCLIFixtures(t, "p")[0]
		if err := panel.AppendRecords(dir, round); err != nil {
			t.Fatal(err)
		}
		var snapshot panel.Panel
		if err := json.Unmarshal(round, &snapshot); err != nil {
			t.Fatal(err)
		}
		if err := panel.WriteSnapshot(dir, snapshot); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(filepath.Join(dir, "dispositions.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		manifest := `{"record":"panel","spec":"130","gate":"bead","panel":"p","round":1,"slots":[{"slot":"R1","model":"model-a","verdict":"APPROVE"}],"backfilled":false}`
		if err := runDispositionAppend(t, specID, "p", manifest); err == nil || !strings.Contains(err.Error(), "reserved to panel tally") {
			t.Fatalf("non-legacy manifest accepted: %v", err)
		}
		after, err := os.ReadFile(filepath.Join(dir, "dispositions.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("manifest refusal changed store")
		}
	})
	t.Run("manifest-legacy", func(t *testing.T) {
		root := mkDispositionRoot(t, specID)
		manifest := `{"record":"panel","spec":"130","gate":"bead","panel":"p","round":1,"slots":[{"slot":"R1","model":"model-a","verdict":"APPROVE"}],"backfilled":false}`
		if err := runDispositionAppend(t, specID, "p", manifest); err != nil {
			t.Fatalf("legacy manifest refused: %v", err)
		}
		path := filepath.Join(root, ".mindspec", "specs", specID, "reviews", "p", "dispositions.jsonl")
		stored, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read legacy manifest at canonical destination: %v", err)
		}
		if !bytes.Equal(bytes.TrimSpace(stored), []byte(manifest)) {
			t.Fatalf("stored legacy manifest = %s, want %s", stored, manifest)
		}
	})
}
