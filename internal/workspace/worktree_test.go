package workspace

import (
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func TestSpecBranch(t *testing.T) {
	tests := []struct {
		specID string
		want   string
	}{
		{"053-drop-state-json", "spec/053-drop-state-json"},
		{"001-skeleton", "spec/001-skeleton"},
	}
	for _, tt := range tests {
		got := SpecBranch(tt.specID)
		if got != tt.want {
			t.Errorf("SpecBranch(%q) = %q, want %q", tt.specID, got, tt.want)
		}
	}
}

func TestBeadBranch(t *testing.T) {
	got := BeadBranch("mindspec-c8q0")
	want := "bead/mindspec-c8q0"
	if got != want {
		t.Errorf("BeadBranch = %q, want %q", got, want)
	}
}

func TestSpecWorktreeName(t *testing.T) {
	got := SpecWorktreeName("053-foo")
	want := "worktree-spec-053-foo"
	if got != want {
		t.Errorf("SpecWorktreeName = %q, want %q", got, want)
	}
}

func TestBeadWorktreeName(t *testing.T) {
	got := BeadWorktreeName("mindspec-c8q0")
	want := "worktree-mindspec-c8q0"
	if got != want {
		t.Errorf("BeadWorktreeName = %q, want %q", got, want)
	}
}

func TestFinalizeBranch(t *testing.T) {
	got := FinalizeBranch("053-foo")
	want := "chore/finalize-053-foo"
	if got != want {
		t.Errorf("FinalizeBranch = %q, want %q", got, want)
	}
}

func TestFinalizeWorktreeName(t *testing.T) {
	got := FinalizeWorktreeName("053-foo")
	want := "worktree-finalize-053-foo"
	if got != want {
		t.Errorf("FinalizeWorktreeName = %q, want %q", got, want)
	}
}

func TestSpecWorktreePath(t *testing.T) {
	got := SpecWorktreePath("/project", config.DefaultConfig(), "053-foo")
	want := filepath.Join("/project", ".worktrees", "worktree-spec-053-foo")
	if got != want {
		t.Errorf("SpecWorktreePath = %q, want %q", got, want)
	}
}

func TestBeadWorktreePath(t *testing.T) {
	specWT := filepath.Join("/project", ".worktrees", "worktree-spec-053-foo")
	got := BeadWorktreePath(specWT, config.DefaultConfig(), "mindspec-mol-07lst")
	want := filepath.Join(specWT, ".worktrees", "worktree-mindspec-mol-07lst")
	if got != want {
		t.Errorf("BeadWorktreePath = %q, want %q", got, want)
	}
}

func TestFinalizeWorktreePath(t *testing.T) {
	got := FinalizeWorktreePath("/project", config.DefaultConfig(), "053-foo")
	want := filepath.Join("/project", ".worktrees", "worktree-finalize-053-foo")
	if got != want {
		t.Errorf("FinalizeWorktreePath = %q, want %q", got, want)
	}
}

func TestFinalizeWorktreePath_HonorsCustomWorktreeRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = ".trees"
	got := FinalizeWorktreePath("/project", cfg, "053-foo")
	want := filepath.Join("/project", ".trees", "worktree-finalize-053-foo")
	if got != want {
		t.Errorf("FinalizeWorktreePath with custom root = %q, want %q", got, want)
	}
}

func TestSpecWorktreePath_HonorsCustomWorktreeRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = ".trees"
	got := SpecWorktreePath("/project", cfg, "053-foo")
	want := filepath.Join("/project", ".trees", "worktree-spec-053-foo")
	if got != want {
		t.Errorf("SpecWorktreePath with custom root = %q, want %q", got, want)
	}
}

func TestBeadWorktreePath_HonorsCustomWorktreeRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = ".trees"
	specWT := filepath.Join("/project", ".trees", "worktree-spec-053-foo")
	got := BeadWorktreePath(specWT, cfg, "mindspec-c8q0")
	want := filepath.Join(specWT, ".trees", "worktree-mindspec-c8q0")
	if got != want {
		t.Errorf("BeadWorktreePath with custom root = %q, want %q", got, want)
	}
}

func TestSpecWorktreePath_NilConfigUsesDefault(t *testing.T) {
	got := SpecWorktreePath("/project", nil, "053-foo")
	want := filepath.Join("/project", ".worktrees", "worktree-spec-053-foo")
	if got != want {
		t.Errorf("SpecWorktreePath(nil cfg) = %q, want %q", got, want)
	}
}

func TestWorktreesDir(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = ".trees"
	got := WorktreesDir("/project", cfg)
	want := filepath.Join("/project", ".trees")
	if got != want {
		t.Errorf("WorktreesDir = %q, want %q", got, want)
	}
}

func TestDefaultWorktreesDir(t *testing.T) {
	got := DefaultWorktreesDir("/project")
	want := filepath.Join("/project", ".worktrees")
	if got != want {
		t.Errorf("DefaultWorktreesDir = %q, want %q", got, want)
	}
}
