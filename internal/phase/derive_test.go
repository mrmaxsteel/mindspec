package phase

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

// ---------------------------------------------------------------------------
// DerivePhaseFromChildren — pure logic, no beads dependency
// ---------------------------------------------------------------------------

func TestDerivePhaseFromChildren(t *testing.T) {
	tests := []struct {
		name     string
		children []ChildInfo
		want     string
	}{
		{
			name:     "no children → plan (spec approved, plan being drafted)",
			children: nil,
			want:     state.ModePlan,
		},
		{
			name:     "empty slice → plan",
			children: []ChildInfo{},
			want:     state.ModePlan,
		},
		{
			name: "all children open → plan (plan approved, ready to claim)",
			children: []ChildInfo{
				{ID: "b1", Status: "open"},
				{ID: "b2", Status: "open"},
				{ID: "b3", Status: "open"},
			},
			want: state.ModePlan,
		},
		{
			name: "one child in_progress → implement",
			children: []ChildInfo{
				{ID: "b1", Status: "open"},
				{ID: "b2", Status: "in_progress"},
				{ID: "b3", Status: "open"},
			},
			want: state.ModeImplement,
		},
		{
			name: "multiple children in_progress → implement",
			children: []ChildInfo{
				{ID: "b1", Status: "in_progress"},
				{ID: "b2", Status: "in_progress"},
			},
			want: state.ModeImplement,
		},
		{
			name: "all children closed → review",
			children: []ChildInfo{
				{ID: "b1", Status: "closed"},
				{ID: "b2", Status: "closed"},
				{ID: "b3", Status: "closed"},
			},
			want: state.ModeReview,
		},
		{
			name: "some closed, some open, none in_progress → plan (next bead ready)",
			children: []ChildInfo{
				{ID: "b1", Status: "closed"},
				{ID: "b2", Status: "open"},
				{ID: "b3", Status: "closed"},
			},
			want: state.ModePlan,
		},
		{
			name: "some closed, one in_progress → implement",
			children: []ChildInfo{
				{ID: "b1", Status: "closed"},
				{ID: "b2", Status: "in_progress"},
				{ID: "b3", Status: "open"},
			},
			want: state.ModeImplement,
		},
		{
			name: "status with whitespace trimmed",
			children: []ChildInfo{
				{ID: "b1", Status: " In_Progress "},
			},
			want: state.ModeImplement,
		},
		{
			name: "single open child → plan",
			children: []ChildInfo{
				{ID: "b1", Status: "open"},
			},
			want: state.ModePlan,
		},
		{
			name: "single closed child → review",
			children: []ChildInfo{
				{ID: "b1", Status: "closed"},
			},
			want: state.ModeReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DerivePhaseFromChildren(tt.children)
			if got != tt.want {
				t.Errorf("DerivePhaseFromChildren() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SpecIDFromMetadata
// ---------------------------------------------------------------------------

func TestSpecIDFromMetadata(t *testing.T) {
	tests := []struct {
		num   int
		title string
		want  string
	}{
		{60, "eliminate-focus-lifecycle", "060-eliminate-focus-lifecycle"},
		{1, "init", "001-init"},
		{999, "big-number", "999-big-number"},
		{0, "zero", "000-zero"},
		// Slugification: titles from beads metadata need lowercasing + hyphenation
		{73, "Llm Test Coverage", "073-llm-test-coverage"},
		{72, "Hook Cleanup", "072-hook-cleanup"},
		{71, "Recording Config Flag", "071-recording-config-flag"},
		{68, "Lifecycle Yaml Cleanup", "068-lifecycle-yaml-cleanup"},
		{1, "Test Feature", "001-test-feature"},
		{5, "Some_Underscored_Title", "005-some-underscored-title"},
	}

	for _, tt := range tests {
		got := SpecIDFromMetadata(tt.num, tt.title)
		if got != tt.want {
			t.Errorf("SpecIDFromMetadata(%d, %q) = %q, want %q", tt.num, tt.title, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ParseSpecFromTitle
// ---------------------------------------------------------------------------

func TestParseSpecFromTitle(t *testing.T) {
	tests := []struct {
		title     string
		wantNum   int
		wantTitle string
	}{
		{
			title:     "[SPEC 060-eliminate-focus-lifecycle] Eliminate Focus",
			wantNum:   60,
			wantTitle: "eliminate-focus-lifecycle",
		},
		{
			title:     "[SPEC 001-init] Initialize",
			wantNum:   1,
			wantTitle: "init",
		},
		{
			title:     "Some random title",
			wantNum:   0,
			wantTitle: "",
		},
		{
			title:     "[SPEC ] Missing number",
			wantNum:   0,
			wantTitle: "",
		},
		{
			title:     "[SPEC 042-multi-word-slug] Multi Word Feature",
			wantNum:   42,
			wantTitle: "multi-word-slug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			num, title := ParseSpecFromTitle(tt.title)
			if num != tt.wantNum || title != tt.wantTitle {
				t.Errorf("ParseSpecFromTitle(%q) = (%d, %q), want (%d, %q)",
					tt.title, num, title, tt.wantNum, tt.wantTitle)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExtractSpecMetadata
// ---------------------------------------------------------------------------

func TestExtractSpecMetadata(t *testing.T) {
	tests := []struct {
		name      string
		epic      EpicInfo
		wantNum   int
		wantTitle string
	}{
		{
			name: "metadata present",
			epic: EpicInfo{
				Metadata: map[string]interface{}{
					"spec_num":   float64(60),
					"spec_title": "eliminate-focus-lifecycle",
				},
			},
			wantNum:   60,
			wantTitle: "eliminate-focus-lifecycle",
		},
		{
			name: "metadata integer (not float)",
			epic: EpicInfo{
				Metadata: map[string]interface{}{
					"spec_num":   42,
					"spec_title": "some-feature",
				},
			},
			wantNum:   42,
			wantTitle: "some-feature",
		},
		{
			name: "no metadata, fallback to title",
			epic: EpicInfo{
				Title: "[SPEC 060-eliminate-focus-lifecycle] Eliminate Focus",
			},
			wantNum:   60,
			wantTitle: "eliminate-focus-lifecycle",
		},
		{
			name: "no metadata, no valid title",
			epic: EpicInfo{
				Title: "Random epic title",
			},
			wantNum:   0,
			wantTitle: "",
		},
		{
			name: "metadata with missing spec_title",
			epic: EpicInfo{
				Title: "[SPEC 060-eliminate-focus-lifecycle] Fallback",
				Metadata: map[string]interface{}{
					"spec_num": float64(60),
				},
			},
			wantNum:   60,
			wantTitle: "eliminate-focus-lifecycle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, title := ExtractSpecMetadata(tt.epic)
			if num != tt.wantNum || title != tt.wantTitle {
				t.Errorf("ExtractSpecMetadata() = (%d, %q), want (%d, %q)",
					num, title, tt.wantNum, tt.wantTitle)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveContextFromDir — path-based resolution
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// DerivePhaseWithStatus — epic-level status check
// ---------------------------------------------------------------------------

func TestDerivePhaseWithStatus_ClosedEpic(t *testing.T) {
	// Closed epic WITHOUT done marker → derives from children (auto-closed by beads)
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "show" {
			// No mindspec_done metadata
			return []byte(`[{"id":"epic-1","title":"test","status":"closed","issue_type":"epic"}]`), nil
		}
		// No children → plan
		return []byte("[]"), nil
	})
	defer restore()

	got, err := DerivePhaseWithStatus("epic-1", "closed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != state.ModePlan {
		t.Errorf("DerivePhaseWithStatus(closed, no done marker) = %q, want %q", got, state.ModePlan)
	}
}

func TestDerivePhaseWithStatus_ClosedEpicWithDoneMarker(t *testing.T) {
	// Closed epic WITH done marker → done (explicitly finalized by impl approve)
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "show" {
			return []byte(`[{"id":"epic-1","title":"test","status":"closed","issue_type":"epic","metadata":{"mindspec_done":true}}]`), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	got, err := DerivePhaseWithStatus("epic-1", "closed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != state.ModeDone {
		t.Errorf("DerivePhaseWithStatus(closed, done marker) = %q, want %q", got, state.ModeDone)
	}
}

func TestDerivePhaseWithStatus_OpenEpicFallsThrough(t *testing.T) {
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		// Return children for open epic
		if args[0] == "list" {
			return []byte(`[{"id":"b1","title":"bead","status":"open","issue_type":"task"}]`), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	got, err := DerivePhaseWithStatus("epic-1", "open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != state.ModePlan {
		t.Errorf("DerivePhaseWithStatus(open) = %q, want %q", got, state.ModePlan)
	}
}

func TestDerivePhase_ClosedEpicViaBDShow(t *testing.T) {
	// Closed epic via bd show, no done marker, with closed children → review
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "show" {
			return []byte(`[{"id":"epic-1","title":"test","status":"closed","issue_type":"epic"}]`), nil
		}
		if args[0] == "list" {
			return []byte(`[{"id":"b1","title":"bead","status":"closed","issue_type":"task"}]`), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	got, err := DerivePhase("epic-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != state.ModeReview {
		t.Errorf("DerivePhase(closed epic, no done marker) = %q, want %q", got, state.ModeReview)
	}
}

func TestResolveContextFromDir_MainWorktree(t *testing.T) {
	// Stub runBDFn to return no epics (idle)
	origRunBD := runBDFn
	defer func() { runBDFn = origRunBD }()

	runBDFn = func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}

	ctx, err := ResolveContextFromDir("/some/project", "/some/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Phase != state.ModeIdle {
		t.Errorf("expected idle, got %q", ctx.Phase)
	}
}

func TestFindEpicBySpecID_ClosedEpic(t *testing.T) {
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "--type=epic" {
			// Only the "closed" status query returns the epic
			for _, a := range args {
				if a == "--status=closed" {
					return []byte(`[{"id":"epic-99","title":"[SPEC 099-done] Done","status":"closed","issue_type":"epic","metadata":{"spec_num":99,"spec_title":"done"}}]`), nil
				}
			}
			return []byte("[]"), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	epicID, err := FindEpicBySpecID("099-done")
	if err != nil {
		t.Fatalf("expected to find closed epic, got error: %v", err)
	}
	if epicID != "epic-99" {
		t.Errorf("epicID: got %q, want %q", epicID, "epic-99")
	}
}

func TestFindEpicBySpecID_DeduplicatesAcrossStatuses(t *testing.T) {
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "--type=epic" {
			// Return same epic for both open and in_progress queries
			return []byte(`[{"id":"epic-1","title":"[SPEC 001-test] Test","status":"open","issue_type":"epic","metadata":{"spec_num":1,"spec_title":"test"}}]`), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	epicID, err := FindEpicBySpecID("001-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if epicID != "epic-1" {
		t.Errorf("epicID: got %q, want %q", epicID, "epic-1")
	}
}

func TestResolveContextFromDir_SpecWorktree_NoEpic(t *testing.T) {
	origRunBD := runBDFn
	defer func() { runBDFn = origRunBD }()

	runBDFn = func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}

	dir := "/project/.worktrees/worktree-spec-060-eliminate-focus-lifecycle"
	ctx, err := ResolveContextFromDir("/project", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.SpecID != "060-eliminate-focus-lifecycle" {
		t.Errorf("expected spec ID 060-eliminate-focus-lifecycle, got %q", ctx.SpecID)
	}
	if ctx.Phase != state.ModeSpec {
		t.Errorf("expected spec phase, got %q", ctx.Phase)
	}
}

func TestResolveContextFromDir_BeadWorktree(t *testing.T) {
	origRunBD := runBDFn
	defer func() { runBDFn = origRunBD }()

	callIdx := 0
	runBDFn = func(args ...string) ([]byte, error) {
		callIdx++
		// First call: show bead
		if len(args) > 0 && args[0] == "show" {
			return []byte(`[{"title":"[060] Bead 1","dependencies":[]}]`), nil
		}
		// List epics
		if len(args) > 0 && args[0] == "list" {
			return []byte(`[{"id":"epic-1","title":"[SPEC 060-test-spec] Test","status":"open","issue_type":"epic","metadata":{"spec_num":60,"spec_title":"test-spec"}}]`), nil
		}
		return []byte("[]"), nil
	}

	dir := "/project/.worktrees/worktree-spec-060-test-spec/.worktrees/worktree-mindspec-abc123"
	ctx, err := ResolveContextFromDir("/project", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.BeadID != "mindspec-abc123" {
		t.Errorf("expected bead ID mindspec-abc123, got %q", ctx.BeadID)
	}
}
