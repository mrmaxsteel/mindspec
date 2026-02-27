package next

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/state"
)

// --- ParseBeadsJSON tests ---

func TestParseBeadsJSON_SingleItem(t *testing.T) {
	input := `[{
		"id": "mindspec-25p",
		"title": "Test bead for parsing",
		"status": "open",
		"priority": 4,
		"issue_type": "task",
		"owner": "max@enubiq.com",
		"created_at": "2026-02-12T08:50:30Z",
		"updated_at": "2026-02-12T08:50:30Z"
	}]`

	items, err := ParseBeadsJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "mindspec-25p" {
		t.Errorf("expected ID mindspec-25p, got %s", items[0].ID)
	}
	if items[0].IssueType != "task" {
		t.Errorf("expected issue_type task, got %s", items[0].IssueType)
	}
	if items[0].Priority != 4 {
		t.Errorf("expected priority 4, got %d", items[0].Priority)
	}
}

func TestParseBeadsJSON_MultipleItems(t *testing.T) {
	input := `[
		{"id": "a", "title": "First", "status": "open", "priority": 1, "issue_type": "task", "owner": "", "created_at": "", "updated_at": ""},
		{"id": "b", "title": "Second", "status": "open", "priority": 2, "issue_type": "feature", "owner": "", "created_at": "", "updated_at": ""}
	]`

	items, err := ParseBeadsJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[1].IssueType != "feature" {
		t.Errorf("expected second item type feature, got %s", items[1].IssueType)
	}
}

func TestParseBeadsJSON_EmptyArray(t *testing.T) {
	items, err := ParseBeadsJSON([]byte("[]"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestParseBeadsJSON_InvalidJSON(t *testing.T) {
	_, err := ParseBeadsJSON([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseBeadsJSON_MoleculeReadyPayload(t *testing.T) {
	input := `{
		"molecule_id": "mol-123",
		"steps": [
			{"issue": {"id":"mol-123","title":"Parent","status":"in_progress","issue_type":"epic"}},
			{"issue": {"id":"impl-1","title":"Implement","status":"open","issue_type":"task"}},
			{"issue": {"id":"closed-1","title":"Closed","status":"closed","issue_type":"task"}}
		]
	}`

	items, err := ParseBeadsJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "impl-1" {
		t.Errorf("expected impl-1, got %s", items[0].ID)
	}
}

// --- SelectWork tests ---

func TestSelectWork_SingleItem(t *testing.T) {
	items := []BeadInfo{{ID: "a", Title: "Only one"}}
	result, err := SelectWork(items, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "a" {
		t.Errorf("expected ID a, got %s", result.ID)
	}
}

func TestSelectWork_MultipleDefaultsToFirst(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
	}
	result, err := SelectWork(items, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "a" {
		t.Errorf("expected ID a, got %s", result.ID)
	}
}

func TestSelectWork_PickSpecific(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
		{ID: "c", Title: "Third"},
	}
	result, err := SelectWork(items, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "b" {
		t.Errorf("expected ID b, got %s", result.ID)
	}
}

func TestSelectWork_PickOutOfRange(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
	}
	_, err := SelectWork(items, 5)
	if err == nil {
		t.Fatal("expected error for out of range pick")
	}
}

func TestSelectWork_EmptyList(t *testing.T) {
	_, err := SelectWork([]BeadInfo{}, 0)
	if err == nil {
		t.Fatal("expected error for empty list")
	}
}

func TestFormatWorkList(t *testing.T) {
	items := []BeadInfo{
		{ID: "abc", Title: "Do something", Priority: 2, IssueType: "task"},
		{ID: "def", Title: "Plan feature", Priority: 1, IssueType: "feature"},
	}
	result := FormatWorkList(items)
	if result == "" {
		t.Fatal("expected non-empty format output")
	}
	if !contains(result, "abc") || !contains(result, "def") {
		t.Errorf("format output missing item IDs: %s", result)
	}
	if !contains(result, "1.") || !contains(result, "2.") {
		t.Errorf("format output missing numbering: %s", result)
	}
}

// --- ResolveMode tests ---

func TestResolveMode_Task(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "005-next: Implement something", IssueType: "task"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "implement" {
		t.Errorf("expected implement, got %s", result.Mode)
	}
	if result.SpecID != "005-next" {
		t.Errorf("expected spec ID 005-next, got %s", result.SpecID)
	}
}

func TestResolveMode_Bug(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "003-context: Fix rendering", IssueType: "bug"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "implement" {
		t.Errorf("expected implement, got %s", result.Mode)
	}
	if result.SpecID != "003-context" {
		t.Errorf("expected spec ID 003-context, got %s", result.SpecID)
	}
}

func TestResolveMode_Feature_NoSpec(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "099-future: New feature", IssueType: "feature"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "spec" {
		t.Errorf("expected spec, got %s", result.Mode)
	}
}

func TestResolveMode_Feature_ApprovedSpec(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := "# Spec\n\n## Approval\n\n- **Status**: APPROVED\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	bead := BeadInfo{ID: "x", Title: "010-test: Plan a feature", IssueType: "feature"}
	result := ResolveMode(tmp, bead)
	if result.Mode != "plan" {
		t.Errorf("expected plan, got %s", result.Mode)
	}
}

func TestResolveMode_Feature_DraftSpec(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := "# Spec\n\n## Approval\n\n- **Status**: DRAFT\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	bead := BeadInfo{ID: "x", Title: "010-test: Draft feature", IssueType: "feature"}
	result := ResolveMode(tmp, bead)
	if result.Mode != "spec" {
		t.Errorf("expected spec, got %s", result.Mode)
	}
}

func TestResolveMode_NoColonInTitle(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "No colon here", IssueType: "task"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "implement" {
		t.Errorf("expected implement, got %s", result.Mode)
	}
	if result.SpecID != "" {
		t.Errorf("expected empty spec ID, got %s", result.SpecID)
	}
}

// --- parseSpecID tests ---

func TestParseSpecID(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"[IMPL 009-feature.1] Chunk title", "009-feature"},
		{"[IMPL 009-workflow-gaps.2] Approval enhancements", "009-workflow-gaps"},
		{"[SPEC 008b-gates] Human Gates Feature", "008b-gates"},
		{"[PLAN 009-feature] Plan decomposition", "009-feature"},
		{"[IMPL 001.3] Simple numeric", "001"},
		// No-tag bracket format: [specID] Bead N: title
		{"[049-hook-command] Bead 1: Core hook infrastructure", "049-hook-command"},
		{"[010-spec-init] Bead 3: Worktree creation", "010-spec-init"},
		{"005-next: Implement work selection", "005-next"},
		{"003-context: Fix rendering bug", "003-context"},
		{"No colon here", ""},
		{"simple:", "simple"},
		{": leading colon", ""},
	}
	for _, tt := range tests {
		result := parseSpecID(tt.title)
		if result != tt.expected {
			t.Errorf("parseSpecID(%q) = %q, want %q", tt.title, result, tt.expected)
		}
	}
}

// --- QueryReady tests ---

func TestQueryReady_UsesMoleculeFromState(t *testing.T) {
	origRunBD := runBDFn
	origReadState := readStateFn
	defer func() {
		runBDFn = origRunBD
		readStateFn = origReadState
	}()

	// Set up temp state with molecule
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	state.Write(tmp, &state.State{
		Mode:           state.ModeImplement,
		ActiveSpec:     "test",
		ActiveMolecule: "mol-123",
	})
	readStateFn = func(root string) (*state.State, error) {
		return state.Read(tmp)
	}

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "ready" && args[1] == "--mol" && args[2] == "mol-123" {
			items := []BeadInfo{
				{ID: "child-1", Title: "[IMPL test.1] First chunk"},
				{ID: "child-2", Title: "[IMPL test.2] Second chunk"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}

	items, err := QueryReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "child-1" {
		t.Errorf("items[0].ID: got %q, want %q", items[0].ID, "child-1")
	}
}

func TestQueryReady_FallsBackWithoutMolecule(t *testing.T) {
	origRunBD := runBDFn
	origReadState := readStateFn
	defer func() {
		runBDFn = origRunBD
		readStateFn = origReadState
	}()

	// No molecule in state
	readStateFn = func(root string) (*state.State, error) {
		return &state.State{Mode: state.ModeIdle}, nil
	}

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "ready" && args[1] == "--json" {
			items := []BeadInfo{
				{ID: "standalone-1", Title: "Standalone work"},
			}
			return json.Marshal(items)
		}
		// worktree info can fail
		return nil, fmt.Errorf("not available")
	}

	items, err := QueryReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "standalone-1" {
		t.Errorf("items[0].ID: got %q, want %q", items[0].ID, "standalone-1")
	}
}

// --- ClaimBead tests ---

func TestClaimBead_CallsRunBDCombined(t *testing.T) {
	origRunBDComb := runBDCombFn
	defer func() { runBDCombFn = origRunBDComb }()

	var capturedArgs []string
	runBDCombFn = func(args ...string) ([]byte, error) {
		capturedArgs = args
		return nil, nil
	}

	err := ClaimBead("bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedArgs) != 3 || capturedArgs[0] != "update" || capturedArgs[1] != "bead-abc" || capturedArgs[2] != "--status=in_progress" {
		t.Errorf("unexpected args: %v", capturedArgs)
	}
}

func TestClaimBead_PropagatesError(t *testing.T) {
	origRunBDComb := runBDCombFn
	defer func() { runBDCombFn = origRunBDComb }()

	runBDCombFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bd update failed")
	}

	err := ClaimBead("bead-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- EnsureWorktree tests ---

// stubWorktreeHelpers saves and restores function variables used by EnsureWorktree.
func stubWorktreeHelpers(t *testing.T) {
	t.Helper()
	origList := worktreeList
	origCreate := worktreeCreate
	origConfig := loadConfigFn
	origBranch := createBranchFn
	origExists := branchExistsFn
	origGitignore := ensureGitignore
	origState := readStateFn
	t.Cleanup(func() {
		worktreeList = origList
		worktreeCreate = origCreate
		loadConfigFn = origConfig
		createBranchFn = origBranch
		branchExistsFn = origExists
		ensureGitignore = origGitignore
		readStateFn = origState
	})

	// Defaults: config returns defaults, no spec branch, branch doesn't exist.
	loadConfigFn = func(root string) (*config.Config, error) { return config.DefaultConfig(), nil }
	createBranchFn = func(name, from string) error { return nil }
	branchExistsFn = func(name string) bool { return false }
	ensureGitignore = func(root, entry string) error { return nil }
	readStateFn = func(root string) (*state.State, error) {
		return &state.State{Mode: state.ModeImplement}, nil
	}
}

func TestEnsureWorktree_CreatesNew(t *testing.T) {
	stubWorktreeHelpers(t)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".worktrees"), 0755)

	listCallCount := 0
	worktreeList = func() ([]bead.WorktreeListEntry, error) {
		listCallCount++
		if listCallCount == 1 {
			return []bead.WorktreeListEntry{
				{Name: "mindspec", Path: root, Branch: "main", IsMain: true},
			}, nil
		}
		return []bead.WorktreeListEntry{
			{Name: "mindspec", Path: root, Branch: "main", IsMain: true},
			{Name: "worktree-bead-abc", Path: filepath.Join(root, ".worktrees", "worktree-bead-abc"), Branch: "bead/bead-abc", IsMain: false},
		}, nil
	}

	var createdName, createdBranch string
	worktreeCreate = func(name, branch string) error {
		createdName = name
		createdBranch = branch
		return nil
	}

	path, err := EnsureWorktree(root, "bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPath := filepath.Join(root, ".worktrees", "worktree-bead-abc")
	if path != expectedPath {
		t.Errorf("path: got %q, want %q", path, expectedPath)
	}
	if createdName != ".worktrees/worktree-bead-abc" {
		t.Errorf("created name: got %q, want %q", createdName, ".worktrees/worktree-bead-abc")
	}
	if createdBranch != "bead/bead-abc" {
		t.Errorf("created branch: got %q, want %q", createdBranch, "bead/bead-abc")
	}
}

func TestEnsureWorktree_BranchesFromSpecBranch(t *testing.T) {
	stubWorktreeHelpers(t)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".worktrees"), 0755)

	readStateFn = func(root string) (*state.State, error) {
		return &state.State{
			Mode:       state.ModeImplement,
			SpecBranch: "spec/046-test",
		}, nil
	}

	var branchFrom string
	createBranchFn = func(name, from string) error {
		branchFrom = from
		return nil
	}

	worktreeList = func() ([]bead.WorktreeListEntry, error) {
		return nil, nil
	}
	worktreeCreate = func(name, branch string) error { return nil }

	_, err := EnsureWorktree(root, "bead-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branchFrom != "spec/046-test" {
		t.Errorf("branch created from %q, want %q", branchFrom, "spec/046-test")
	}
}

func TestEnsureWorktree_ReusesExisting(t *testing.T) {
	stubWorktreeHelpers(t)
	root := t.TempDir()

	worktreeList = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "mindspec", Path: root, Branch: "main", IsMain: true},
			{Name: "worktree-bead-abc", Path: filepath.Join(root, ".worktrees", "worktree-bead-abc"), Branch: "bead/bead-abc", IsMain: false},
		}, nil
	}

	worktreeCreate = func(name, branch string) error {
		t.Error("worktreeCreate should not be called when worktree exists")
		return nil
	}

	path, err := EnsureWorktree(root, "bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPath := filepath.Join(root, ".worktrees", "worktree-bead-abc")
	if path != expectedPath {
		t.Errorf("path: got %q, want %q", path, expectedPath)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- FetchBeadByID tests ---

func TestFetchBeadByID_ArrayResponse(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "show" && args[1] == "bead-abc" {
			items := []BeadInfo{{
				ID:    "bead-abc",
				Title: "[IMPL 047.1] Test bead",
			}}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}

	info, err := FetchBeadByID("bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "bead-abc" {
		t.Errorf("ID = %q, want %q", info.ID, "bead-abc")
	}
	if info.Title != "[IMPL 047.1] Test bead" {
		t.Errorf("Title = %q", info.Title)
	}
}

func TestFetchBeadByID_SingleObjectResponse(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		item := BeadInfo{ID: "bead-xyz", Title: "Single object"}
		return json.Marshal(item)
	}

	info, err := FetchBeadByID("bead-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "bead-xyz" {
		t.Errorf("ID = %q, want %q", info.ID, "bead-xyz")
	}
}

func TestFetchBeadByID_NotFound(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bead not found")
	}

	_, err := FetchBeadByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent bead")
	}
}

func TestFetchBeadByID_EmptyArray(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}

	_, err := FetchBeadByID("bead-empty")
	if err == nil {
		t.Fatal("expected error for empty array response")
	}
}
