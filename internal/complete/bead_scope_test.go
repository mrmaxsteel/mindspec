package complete

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// writeBeadScopeFixture builds a spec fixture under root with TWO
// domains — "workflow" (claims internal/approve/**) and "execution"
// (claims internal/executor/**) — both declared as Impacted Domains, and
// deliberately NO plan.md (CheckADRDivergence degrades to a no-op on a
// missing plan.md, exactly like a missing spec.md — see its doc comment
// — so the ADR-divergence gate never needs an override in this fixture).
func writeBeadScopeFixture(t *testing.T, root, specID string) string {
	t.Helper()

	specDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	specMD := "# Spec " + specID + "\n\n## Impacted Domains\n\n- workflow\n- execution\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specMD), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	workflowDir := filepath.Join(root, ".mindspec", "docs", "domains", "workflow")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow domain dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "OWNERSHIP.yaml"),
		[]byte("paths:\n  - internal/approve/**\n"), 0o644); err != nil {
		t.Fatalf("write workflow OWNERSHIP.yaml: %v", err)
	}

	executionDir := filepath.Join(root, ".mindspec", "docs", "domains", "execution")
	if err := os.MkdirAll(executionDir, 0o755); err != nil {
		t.Fatalf("mkdir execution domain dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(executionDir, "OWNERSHIP.yaml"),
		[]byte("paths:\n  - internal/executor/**\n"), 0o644); err != nil {
		t.Fatalf("write execution OWNERSHIP.yaml: %v", err)
	}

	return specDir
}

// TestRun_BeadScopeAdvisoryWarn_CrossDomain is the Spec 119 AC-22 proof:
// a bead whose DECLARED scope (its `file_paths` bead-metadata, Bead 4's
// key_file_paths piping) attributes only to the "workflow" domain, but
// whose ACTUAL changed files also touch an "execution"-owned file (both
// domains are within the spec's Impacted Domains), gets a non-fatal WARN
// naming the file and both domains — and `mindspec complete` still
// succeeds (exit code unchanged from the no-warn case, run as a second
// sub-test below with the cross-domain file omitted).
func TestRun_BeadScopeAdvisoryWarn_CrossDomain(t *testing.T) {
	run := func(t *testing.T, changedFiles []string) (string, error) {
		saveAndRestore(t)
		root := setupTempRoot(t)
		specID := "119-bead-scope-test"
		writeBeadScopeFixture(t, root, specID)
		stubPhaseEpic(t, specID, "epic-119")

		resolveTargetFn = func(r, flag string) (string, error) { return specID, nil }
		worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
		closeBeadFn = func(ids ...string) error { return nil }
		runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

		// The bead's DECLARED scope (Bead 4's file_paths metadata):
		// attributes ONLY to "workflow".
		beadScopeGetMetadataFn = func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"file_paths": []string{"internal/approve/plan.go"},
			}, nil
		}

		mock := newMockExec()
		serveRefFromDisk(mock, root)
		mock.ChangedFilesResult = changedFiles

		origWarn := warnWriter
		var buf bytes.Buffer
		warnWriter = &buf
		t.Cleanup(func() { warnWriter = origWarn })

		_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{AllowDocSkew: "test setup"})
		return buf.String(), err
	}

	t.Run("no_warn_baseline", func(t *testing.T) {
		out, err := run(t, []string{"internal/approve/plan.go"})
		if err != nil {
			t.Fatalf("baseline (same-domain only) run failed: %v", err)
		}
		if strings.Contains(out, "bead-scope") {
			t.Errorf("expected NO bead-scope WARN for a same-domain-only diff, got:\n%s", out)
		}
	})

	t.Run("cross_domain_warn", func(t *testing.T) {
		out, err := run(t, []string{
			"internal/approve/plan.go",
			"internal/executor/mindspec_executor.go",
		})
		// AC-22: exit code (err-ness) unchanged from the no-warn baseline.
		if err != nil {
			t.Fatalf("cross-domain run must still succeed (advisory, non-fatal), got: %v", err)
		}
		if !strings.Contains(out, "WARN bead-scope:") {
			t.Fatalf("expected a bead-scope WARN line, got:\n%s", out)
		}
		if !strings.Contains(out, "internal/executor/mindspec_executor.go") {
			t.Errorf("WARN must name the cross-domain file, got:\n%s", out)
		}
		if !strings.Contains(out, "execution") {
			t.Errorf("WARN must name the file's owning domain (execution), got:\n%s", out)
		}
		if !strings.Contains(out, "workflow") {
			t.Errorf("WARN must name the bead's declared domain (workflow), got:\n%s", out)
		}
	})
}

// TestBeadScopeWarnAdvisory_TermsafeEscaped pins the R11/spec-116
// requirement directly at the unit level: any control byte in the bead
// ID reaching beadScopeWarnAdvisory's output is rendered through
// internal/termsafe.Escape (a single-line, double-quoted Go string
// literal) rather than passed through raw — the same defensive posture
// panel_advisory.go already applies to its own path/ID-bearing output.
func TestBeadScopeWarnAdvisory_TermsafeEscaped(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	specID := "119-bead-scope-termsafe"
	specDir := writeBeadScopeFixture(t, root, specID)

	hostileBeadID := "bead\x1b[31m-1"

	beadScopeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"file_paths": []string{"internal/approve/plan.go"},
		}, nil
	}
	mock := newMockExec()
	serveRefFromDisk(mock, root)
	mock.ChangedFilesResult = []string{
		"internal/approve/plan.go",
		"internal/executor/mindspec_executor.go",
	}

	var buf bytes.Buffer
	beadScopeWarnAdvisory(mock, root, specDir, hostileBeadID, "bead/"+hostileBeadID, "base-sha", "head-sha", &buf)

	out := buf.String()
	if out == "" {
		t.Fatal("expected a WARN line, got no output")
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("raw ESC control byte leaked into WARN output (must be termsafe-escaped): %q", out)
	}
	quoted := strconv.Quote(hostileBeadID)
	if !strings.Contains(out, quoted) {
		t.Errorf("expected the termsafe-escaped (strconv.Quote) bead ID %q in output, got:\n%s", quoted, out)
	}
}
