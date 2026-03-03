package phase

import (
	"testing"

	"github.com/mindspec/mindspec/internal/state"
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
