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
