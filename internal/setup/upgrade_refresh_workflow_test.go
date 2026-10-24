package setup

// upgrade_refresh_workflow_test.go — every release that shipped
// ms-panel.js (v0.11.0 through v0.16.0) shipped the same body, git blob
// 5004544e. That body is snapshotted in historical_workflows so an
// unmodified existing install refreshes in place, and a user-modified one
// stays in place with a notice.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

// git show 5004544e | shasum -a 256
const workflowPresurfaceDigest = "2dff14fb1b67fcacc7c7c66cc0980b7cc7aa080eef352748115341d28417ef08"

func readWorkflowPresurface(t *testing.T) string {
	t.Helper()
	data, err := historicalWorkflowsFS.ReadFile("historical_workflows/ms-panel.presurface.js")
	if err != nil {
		t.Fatalf("reading the ms-panel.js snapshot: %v", err)
	}
	return string(data)
}

func installWorkflowWithPrior(t *testing.T, prior string) (string, *Result) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".claude", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ms-panel.js"), []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Result{}
	if err := installWorkflows(dir, filepath.Join(".claude", "workflows"), pluginmindspec.WorkflowFiles(), false, r); err != nil {
		t.Fatalf("installWorkflows: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "ms-panel.js"))
	if err != nil {
		t.Fatal(err)
	}
	return string(got), r
}

func TestUpgradeRefreshWorkflow_DigestOracle(t *testing.T) {
	sum := sha256.Sum256([]byte(readWorkflowPresurface(t)))
	if got := hex.EncodeToString(sum[:]); got != workflowPresurfaceDigest {
		t.Errorf("ms-panel.js snapshot digest mismatch:\n  got:  %s\n  want: %s", got, workflowPresurfaceDigest)
	}
}

func TestUpgradeRefreshWorkflow_V0170Refreshes(t *testing.T) {
	prior, err := historicalWorkflowsFS.ReadFile("historical_workflows/ms-panel.pre131.js")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(prior)
	if hex.EncodeToString(sum[:]) != "fa8d6e59a9b0893aa4f11f2685316177c1ffd6ddba84e2e49a59d768b54b6bec" {
		t.Fatal("release snapshot digest differs")
	}
	want := pluginmindspec.WorkflowFiles()["ms-panel.js"]
	if string(prior) == want {
		t.Fatal("release snapshot equals current workflow")
	}
	got, result := installWorkflowWithPrior(t, string(prior))
	if got != want || !containsPath(result.Refreshed, filepath.Join(".claude", "workflows", "ms-panel.js")) {
		t.Fatalf("release workflow did not refresh: %+v", result)
	}
}

func TestUpgradeRefreshWorkflow_UnmodifiedOldBodyRefreshes(t *testing.T) {
	prior := readWorkflowPresurface(t)
	want := pluginmindspec.WorkflowFiles()["ms-panel.js"]
	if prior == want {
		t.Fatal("snapshot equals the canonical body; the refresh assertion would be vacuous")
	}
	got, r := installWorkflowWithPrior(t, prior)
	relPath := filepath.Join(".claude", "workflows", "ms-panel.js")
	if !containsPath(r.Refreshed, relPath) {
		t.Errorf("expected Refreshed to record %s; got Refreshed=%v Notices=%v", relPath, r.Refreshed, r.Notices)
	}
	if got != want {
		t.Error("refreshed workflow does not equal the canonical body")
	}
}

func TestUpgradeRefreshWorkflow_UserModifiedPreserved(t *testing.T) {
	userBody := readWorkflowPresurface(t) + "\n// local operator note\n"
	got, r := installWorkflowWithPrior(t, userBody)
	relPath := filepath.Join(".claude", "workflows", "ms-panel.js")
	if containsPath(r.Refreshed, relPath) {
		t.Error("a user-modified workflow must not be refreshed")
	}
	if got != userBody {
		t.Error("a user-modified workflow must be left in place unmodified")
	}
	found := false
	for _, n := range r.Notices {
		if strings.HasPrefix(n, relPath) && strings.Contains(n, "user-modified") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a user-modified notice for %s; got Notices=%v", relPath, r.Notices)
	}
}

func TestUpgradeRefreshWorkflow_BeforeBriefRefreshes(t *testing.T) {
	prior, err := historicalWorkflowsFS.ReadFile("historical_workflows/ms-panel.prebrief.js")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(prior)
	if hex.EncodeToString(sum[:]) != "9a19eb10b8a94391141771dfdbeb84c296acad85112a72681d65dbc699755dbe" {
		t.Fatal("prior workflow snapshot digest differs")
	}
	want := pluginmindspec.WorkflowFiles()["ms-panel.js"]
	if string(prior) == want {
		t.Fatal("snapshot equals current workflow")
	}
	got, result := installWorkflowWithPrior(t, string(prior))
	if got != want || !containsPath(result.Refreshed, filepath.Join(".claude", "workflows", "ms-panel.js")) || len(result.Notices) != 0 {
		t.Fatalf("prior workflow did not refresh cleanly: %+v", result)
	}
}
