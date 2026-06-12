package bead

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- BeadInfo JSON parsing tests ---

func TestBeadInfo_JSONRoundTrip(t *testing.T) {
	original := BeadInfo{
		ID:          "mindspec-abc",
		Title:       "[SPEC 006-validate] Workflow Validation",
		Description: "Summary: Add validation\nSpec: docs/specs/006-validate/spec.md",
		Status:      "open",
		Priority:    2,
		IssueType:   "feature",
		Owner:       "user@example.com",
		CreatedAt:   "2026-02-12T10:00:00Z",
		UpdatedAt:   "2026-02-12T10:30:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed BeadInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.ID != original.ID {
		t.Errorf("ID: got %q, want %q", parsed.ID, original.ID)
	}
	if parsed.Title != original.Title {
		t.Errorf("Title: got %q, want %q", parsed.Title, original.Title)
	}
	if parsed.Description != original.Description {
		t.Errorf("Description: got %q, want %q", parsed.Description, original.Description)
	}
	if parsed.Status != original.Status {
		t.Errorf("Status: got %q, want %q", parsed.Status, original.Status)
	}
	if parsed.Priority != original.Priority {
		t.Errorf("Priority: got %d, want %d", parsed.Priority, original.Priority)
	}
	if parsed.IssueType != original.IssueType {
		t.Errorf("IssueType: got %q, want %q", parsed.IssueType, original.IssueType)
	}
}

// --- Preflight tests ---

func TestPreflight_MissingBeadsDir(t *testing.T) {
	tmp := t.TempDir()
	// Init a git repo but no .beads/
	cmd := exec.Command("git", "init")
	cmd.Dir = tmp
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	err := Preflight(tmp)
	if err == nil {
		t.Fatal("expected error for missing .beads/")
	}
	if !strings.Contains(err.Error(), ".beads/") {
		t.Errorf("error should mention .beads/: %v", err)
	}
	if !strings.Contains(err.Error(), "beads init") {
		t.Errorf("error should suggest 'beads init': %v", err)
	}
}

func TestPreflight_NotGitRepo(t *testing.T) {
	tmp := t.TempDir()
	// No .git, but add .beads/ to test git check runs first
	os.MkdirAll(filepath.Join(tmp, ".beads"), 0755)

	err := Preflight(tmp)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("error should mention git: %v", err)
	}
}

func TestPreflight_Success(t *testing.T) {
	tmp := t.TempDir()
	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmp
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	// Create .beads/
	os.MkdirAll(filepath.Join(tmp, ".beads"), 0755)

	// bd must be on PATH for this test (skip if not available)
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH, skipping Preflight success test")
	}

	err := Preflight(tmp)
	if err != nil {
		t.Fatalf("unexpected preflight error: %v", err)
	}
}

// --- Close tests ---

func TestClose_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "closed")
	}

	err := Close("bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "close", "bead-abc"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestClose_MultipleIDs(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "closed")
	}

	err := Close("bead-1", "bead-2", "bead-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "close", "bead-1", "bead-2", "bead-3"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestClose_NoIDs(t *testing.T) {
	err := Close()
	if err == nil {
		t.Fatal("expected error for no IDs")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Errorf("error should mention 'at least one': %v", err)
	}
}

// --- WorktreeCreate tests ---

func TestWorktreeCreate_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "created")
	}

	err := WorktreeCreate("worktree-bead-abc", "bead/bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "worktree", "create", "worktree-bead-abc", "--branch=bead/bead-abc"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

// TestWorktreeCreate_SharesMainDB is a real-bd regression test for the
// "redirect gap" described in spec 082 bead 4. A worktree created via
// `bd worktree create` must be able to query the main repo's beads DB from
// inside the worktree (the whole point of a shared-beads worktree).
//
// bd 1.0.4's embedded Dolt mode spawns no sidecar server, so the bd 1.0.2
// sidecar-server issue that previously forced a skip here is gone and the
// test runs for real.
func TestWorktreeCreate_SharesMainDB(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skipf("bd not on PATH: %v", err)
	}

	tmp := t.TempDir()

	// Initialize a git repo + bd project at tmp.
	runCmd := func(dir, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v failed in %s: %v\n%s", name, args, dir, err, out)
		}
	}
	runCmd(tmp, "git", "init")
	runCmd(tmp, "git", "config", "user.email", "test@example.com")
	runCmd(tmp, "git", "config", "user.name", "test")
	runCmd(tmp, "git", "commit", "--allow-empty", "-m", "init")
	runCmd(tmp, "bd", "init")

	// Create a throwaway issue so the shared DB is non-trivial to observe.
	runCmd(tmp, "bd", "create", "--title", "probe", "--description", "probe", "--type", "task")

	// Create a worktree branch and worktree. `bd worktree create` takes the
	// relative worktree path as its first positional arg.
	runCmd(tmp, "git", "branch", "feature-x")
	runCmd(tmp, "bd", "worktree", "create", ".worktrees/feature-x", "--branch=feature-x")

	// From INSIDE the worktree, bd list must succeed and see the probe issue.
	wt := filepath.Join(tmp, ".worktrees", "feature-x")
	listCmd := exec.Command("bd", "list", "--json")
	listCmd.Dir = wt
	out, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd list from worktree failed (redirect gap reproduced): %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "probe") {
		t.Errorf("bd list from worktree did not see main-repo issues; got:\n%s", out)
	}
}

func TestWorktreeCreate_NoBranch(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "created")
	}

	err := WorktreeCreate("worktree-abc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have --branch when branch is empty
	for _, arg := range capturedArgs {
		if strings.HasPrefix(arg, "--branch") {
			t.Error("should not include --branch when branch is empty")
		}
	}
}

// --- WorktreeList tests ---

func TestWorktreeList_ParsesJSON(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	listJSON := `[
		{"name":"mindspec","path":"/home/user/mindspec","branch":"main","is_main":true,"beads_state":"shared"},
		{"name":"worktree-bead-abc","path":"/home/user/worktree-bead-abc","branch":"bead/bead-abc","is_main":false,"beads_state":"shared"}
	]`

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", listJSON)
	}

	entries, err := WorktreeList()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Name != "mindspec" {
		t.Errorf("entry[0].Name: got %q, want %q", entries[0].Name, "mindspec")
	}
	if !entries[0].IsMain {
		t.Error("entry[0] should be main")
	}
	if entries[1].Name != "worktree-bead-abc" {
		t.Errorf("entry[1].Name: got %q, want %q", entries[1].Name, "worktree-bead-abc")
	}
	if entries[1].Branch != "bead/bead-abc" {
		t.Errorf("entry[1].Branch: got %q, want %q", entries[1].Branch, "bead/bead-abc")
	}
	if entries[1].IsMain {
		t.Error("entry[1] should not be main")
	}
}

func TestWorktreeList_Empty(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}

	entries, err := WorktreeList()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestWorktreeList_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "[]")
	}

	_, _ = WorktreeList()

	expected := []string{"bd", "worktree", "list", "--json"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

// --- ListJSON tests ---

func TestListJSON_ValidJSONPassthrough(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `[{"id":"mindspec-abc","title":"x"}]`)
	}
	out, err := ListJSON("--status", "ready")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(string(out), "mindspec-abc") {
		t.Fatalf("got %s", out)
	}
}

func TestListJSON_EmptyResults(t *testing.T) {
	cases := []string{"", "[]", "No issues found."}
	for _, c := range cases {
		c := c
		t.Run(c, func(t *testing.T) {
			origExec := execCommand
			defer func() { execCommand = origExec }()
			execCommand = func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", c)
			}
			out, err := ListJSON()
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if strings.TrimSpace(string(out)) != "[]" {
				t.Errorf("want [], got %q", out)
			}
		})
	}
}

func TestListJSON_NonJSONFailsLoudly(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Simulate an old bd printing a human-readable table.
		return exec.Command("echo", "○ mindspec-abc ● P2 ready  Some title")
	}
	_, err := ListJSON()
	if err == nil {
		t.Fatal("expected error for non-JSON output")
	}
	if !strings.Contains(err.Error(), "mindspec doctor") {
		t.Errorf("error should point at doctor: %v", err)
	}
	if !strings.Contains(err.Error(), "brew upgrade beads") &&
		!strings.Contains(err.Error(), "upgrade") {
		t.Errorf("error should suggest upgrade: %v", err)
	}
}

func TestListJSON_ExecErrorPropagates(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		// `false` exits 1 with no output.
		return exec.Command("false")
	}
	_, err := ListJSON()
	if err == nil {
		t.Fatal("expected error from failed bd invocation")
	}
}

// --- WorktreeRemove tests ---

func TestWorktreeRemove_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "removed")
	}

	err := WorktreeRemove("worktree-bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "worktree", "remove", "worktree-bead-abc", "--force"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

// --- BeadExists tests ---
//
// Covers the three branches that justify this helper's existence:
//   (1) success -> (true, nil)
//   (2) bd ran, non-zero exit (bead missing) -> (false, nil) — the
//       *exec.ExitError type-switch that keeps os/exec out of callers.
//   (3) bd unavailable / other invocation failure -> (false, err).

func TestBeadExists_Found(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `[{"id":"mindspec-abc"}]`)
	}

	exists, err := BeadExists("mindspec-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true for successful bd show")
	}
}

func TestBeadExists_NotFound(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	// `false` exits non-zero with no output: bd-ran-but-bead-missing.
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	exists, err := BeadExists("mindspec-missing")
	if err != nil {
		t.Fatalf("expected nil err for *exec.ExitError, got: %v", err)
	}
	if exists {
		t.Error("expected exists=false when bd exits non-zero")
	}
}

func TestBeadExists_BdUnavailable(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	// Non-existent binary: cmd.Output() returns a non-ExitError
	// (an *exec.Error / *fs.PathError). That branch must surface
	// the error to the caller so warnings (not errors) are emitted.
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("/nonexistent/path/to/bd-binary")
	}

	exists, err := BeadExists("mindspec-anything")
	if err == nil {
		t.Fatal("expected error when bd binary is unavailable")
	}
	if exists {
		t.Error("expected exists=false on bd invocation failure")
	}
}

// --- MergeMetadata tests (Spec 092 Bead 3, Req 19) ---

// TestMergeMetadata_PreservesUnrelatedKeys is the before/after diff
// half of the spec AC "repair unit (Req 19)": MergeMetadata performs a
// read-merge-write, so unrelated metadata keys (mindspec_migrated_at,
// spec binding keys, audit keys) survive a phase update. This is the
// reason every recovery line emits `mindspec repair phase <spec-id>`
// instead of a raw `bd update` metadata command (replace semantics).
func TestMergeMetadata_PreservesUnrelatedKeys(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	before := map[string]interface{}{
		"mindspec_phase":            "implement",
		"mindspec_migrated_at":      "2026-01-01T00:00:00Z",
		"mindspec_impl_skew_reason": "audit trail",
		"spec_num":                  float64(92),
		"spec_title":                "agent-contract-hardening",
	}
	beforeJSON, err := json.Marshal([]map[string]interface{}{{"metadata": before}})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	var capturedUpdate []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "show" {
			return exec.Command("echo", string(beforeJSON))
		}
		if len(args) > 0 && args[0] == "update" {
			capturedUpdate = append([]string{name}, args...)
			return exec.Command("echo", "updated")
		}
		t.Errorf("unexpected command: %s %v", name, args)
		return exec.Command("echo", "")
	}

	if err := MergeMetadata("epic-92", map[string]interface{}{"mindspec_phase": "review"}); err != nil {
		t.Fatalf("MergeMetadata: %v", err)
	}

	if len(capturedUpdate) < 5 || capturedUpdate[0] != "bd" || capturedUpdate[1] != "update" || capturedUpdate[2] != "epic-92" || capturedUpdate[3] != "--metadata" {
		t.Fatalf("unexpected update invocation: %v", capturedUpdate)
	}
	var after map[string]interface{}
	if err := json.Unmarshal([]byte(capturedUpdate[4]), &after); err != nil {
		t.Fatalf("parse written metadata: %v", err)
	}

	// Diff before vs after: exactly one key changed (the phase), every
	// other key preserved byte-for-byte.
	if got := after["mindspec_phase"]; got != "review" {
		t.Errorf("mindspec_phase = %v, want review", got)
	}
	for k, want := range before {
		if k == "mindspec_phase" {
			continue
		}
		got, ok := after[k]
		if !ok {
			t.Errorf("unrelated key %q wiped by MergeMetadata", k)
			continue
		}
		if got != want {
			t.Errorf("unrelated key %q changed: got %v, want %v", k, got, want)
		}
	}
	if len(after) != len(before) {
		t.Errorf("metadata key count changed: got %d keys (%v), want %d", len(after), after, len(before))
	}
}

// TestMergeMetadata_WriteFailureMessageOmitsRawCommand pins the Req 19
// hygiene on the plumbing error: the failure text describes the
// operation without quoting a pasteable raw metadata-replace command.
func TestMergeMetadata_WriteFailureMessageOmitsRawCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "show" {
			return exec.Command("echo", `[{"metadata":{"mindspec_phase":"implement"}}]`)
		}
		return exec.Command("false")
	}

	err := MergeMetadata("epic-92", map[string]interface{}{"mindspec_phase": "review"})
	if err == nil {
		t.Fatal("expected error when the update write fails")
	}
	if strings.Contains(err.Error(), "bd update --metadata") {
		t.Errorf("plumbing error quotes the banned raw command (Req 19): %v", err)
	}
}
