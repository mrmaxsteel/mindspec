package main

// doctor_migration_test.go — Bead 3 of spec 118 (mindspec-qqv1.3): B3-V4
// command-level proof for the rekeyed `checkMigrationMetadata` contract
// (AC-7, AC-7b, AC-24 command behavior). Runs the real `mindspec doctor`
// binary against a hermetic flat workspace fixture and asserts exit code +
// rendered output, including the absence of any docs_archive/ or retired
// classify/plan/apply pipeline finding and any `migrate apply` hint.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mkMigrationWorkspace builds a hermetic, otherwise-healthy flat mindspec
// workspace: a `.mindspec/` marker with empty `domains/`/`specs/` enumeration
// roots (satisfies checkDocs' required dirs with zero domains to check) and
// a `.beads/` dir with a durable-state file (satisfies checkBeads). Neither
// a real `.git` repository nor a real `bd` install is required — every other
// doctor check either no-ops or Warns on an otherwise-empty fixture like
// this one (see internal/doctor: checkGit tolerates git-command failure as
// "not tracked", checkOwnershipManifests/checkOrphanedBeads skip when there
// are no domains/specs, checkBeadsMergeDriver skips without .gitattributes).
// This isolates the exit-code assertion to the migration-metadata check.
func mkMigrationWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dirs := []string{
		filepath.Join(".mindspec", "domains"),
		filepath.Join(".mindspec", "specs"),
		".beads",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".beads", "issues.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}
	return root
}

// writeMigrationArtifacts writes the CURRENT `mindspec migrate layout`
// contract's three artifacts into root: the global manifest and the per-run
// lineage.json + state.json (with the given stage), all sharing runID and a
// single matching lineage entry.
func writeMigrationArtifacts(t *testing.T, root, runID, stage string) {
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
	write(".mindspec/lineage/manifest.json", `{"run_id": "`+runID+`", "entries": [`+entry+`]}`)
	write(".mindspec/migrations/"+runID+"/lineage.json", `{"run_id": "`+runID+`", "entries": [`+entry+`]}`)
	write(".mindspec/migrations/"+runID+"/state.json", `{"run_id": "`+runID+`", "stage": "`+stage+`"}`)
}

// runDoctor runs the built `mindspec doctor` binary with cwd=root and
// returns combined stdout+stderr and the process exit code (0 on success).
func runDoctor(t *testing.T, bin, root string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, "doctor")
	cmd.Dir = root
	cmd.Env = strippedEnv(t)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return out.String(), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return out.String(), ee.ExitCode()
	}
	t.Fatalf("run doctor: %v\noutput=%s", err, out.String())
	return "", -1
}

// TestDoctorMigrationMetadata_CurrentContract is B3-V4: command-level
// exit-code and rendered-output proof for AC-7, AC-7b, and AC-24.
func TestDoctorMigrationMetadata_CurrentContract(t *testing.T) {
	bin := buildMindspecBinary(t)
	const runID = "run-cur"

	t.Run("healthy_completed_run_exits_zero", func(t *testing.T) {
		root := mkMigrationWorkspace(t)
		writeMigrationArtifacts(t, root, runID, "applied")

		out, code := runDoctor(t, bin, root)
		if code != 0 {
			t.Fatalf("expected exit 0 for a healthy completed run, got %d\noutput:\n%s", code, out)
		}

		for _, want := range []string{
			".mindspec/lineage/manifest.json: [OK]",
			".mindspec/migrations/" + runID + "/lineage.json: [OK]",
			".mindspec/migrations/" + runID + "/state.json: [OK]",
			".mindspec/migrations/" + runID + "/state.stage: [OK] applied",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("expected output to contain %q\noutput:\n%s", want, out)
			}
		}

		if strings.Contains(out, "docs_archive") {
			t.Errorf("unexpected docs_archive/ reference in output:\n%s", out)
		}
		if strings.Contains(out, "migrate apply") {
			t.Errorf("unexpected `migrate apply` hint in output:\n%s", out)
		}
		for _, obsolete := range []string{
			"inventory.json", "classification.json", "extraction.json",
			"plan.json", "plan.md", "validation.json", "apply.json",
		} {
			if strings.Contains(out, obsolete) {
				t.Errorf("unexpected obsolete-artifact %q in output:\n%s", obsolete, out)
			}
		}
	})

	t.Run("missing_global_manifest_exits_nonzero", func(t *testing.T) {
		root := mkMigrationWorkspace(t)
		writeMigrationArtifacts(t, root, runID, "applied")
		if err := os.Remove(filepath.Join(root, ".mindspec", "lineage", "manifest.json")); err != nil {
			t.Fatal(err)
		}

		out, code := runDoctor(t, bin, root)
		if code == 0 {
			t.Fatalf("expected nonzero exit for a missing global manifest\noutput:\n%s", out)
		}
		if !strings.Contains(out, ".mindspec/lineage/manifest.json: [MISSING]") {
			t.Errorf("expected a MISSING finding for the global manifest\noutput:\n%s", out)
		}
	})

	t.Run("malformed_state_json_exits_nonzero", func(t *testing.T) {
		root := mkMigrationWorkspace(t)
		writeMigrationArtifacts(t, root, runID, "applied")
		path := filepath.Join(root, ".mindspec", "migrations", runID, "state.json")
		if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
			t.Fatal(err)
		}

		out, code := runDoctor(t, bin, root)
		if code == 0 {
			t.Fatalf("expected nonzero exit for a malformed state.json\noutput:\n%s", out)
		}
		if !strings.Contains(out, ".mindspec/migrations/"+runID+"/state.json: [ERROR]") {
			t.Errorf("expected an ERROR finding for the malformed state.json\noutput:\n%s", out)
		}
	})

	// AC-24 command behavior: a parseable non-"applied" stage (an
	// in-progress run) stays Warn/non-fatal — exit 0 — and must not be
	// reported as healthy/completed/applied.
	t.Run("non_applied_finalize_stays_warn_exit_zero", func(t *testing.T) {
		root := mkMigrationWorkspace(t)
		writeMigrationArtifacts(t, root, runID, "finalize")

		out, code := runDoctor(t, bin, root)
		if code != 0 {
			t.Fatalf("expected exit 0 for a non-applied (in-progress) stage, got %d\noutput:\n%s", code, out)
		}
		if !strings.Contains(out, ".mindspec/migrations/"+runID+"/state.stage: [WARN]") {
			t.Errorf("expected a WARN finding for the finalize stage\noutput:\n%s", out)
		}
		for _, line := range strings.Split(out, "\n") {
			if !strings.HasPrefix(line, ".mindspec/migrations/"+runID+"/state.stage:") {
				continue
			}
			lower := strings.ToLower(line)
			for _, forbidden := range []string{"healthy", "completed", "applied"} {
				if strings.Contains(lower, forbidden) {
					t.Errorf("stage line must not claim %q: %q", forbidden, line)
				}
			}
		}
	})
}
