package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// findCheckContaining returns the first check whose Message contains
// substr, or nil.
func findCheckContaining(r *Report, substr string) *Check {
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Message, substr) {
			return &r.Checks[i]
		}
	}
	return nil
}

const sourceGlobsCheckName = ".mindspec/config.yaml source_globs"

// TestMissingSourceGlobs_AllThreeStates exercises spec 091 Req 18: the
// missing-source-globs Warn fires when config.yaml is absent, present
// without the field, or present with an empty list; and clears once a
// non-empty entry exists.
func TestMissingSourceGlobs_AllThreeStates(t *testing.T) {
	t.Run("config file absent", func(t *testing.T) {
		root := t.TempDir()
		config.ResetCache()

		r := &Report{}
		checkSourceGlobs(r, root)

		c := findCheck(r, sourceGlobsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-source-globs Warn, got %+v", c)
		}
		if !strings.Contains(c.Message, "missing-source-globs") {
			t.Errorf("message missing the warn name: %q", c.Message)
		}
		if !strings.Contains(c.Message, ".mindspec/config.yaml") {
			t.Errorf("message must name the config path even when absent: %q", c.Message)
		}
		if !strings.Contains(c.Message, "built-in default") {
			t.Errorf("message must disclose the built-in default: %q", c.Message)
		}
		if !strings.Contains(c.Message, "mindspec source populate") {
			t.Errorf("message must hint source populate: %q", c.Message)
		}
	})

	t.Run("config present without field", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "merge_strategy: auto\n")
		config.ResetCache()

		r := &Report{}
		checkSourceGlobs(r, root)

		c := findCheck(r, sourceGlobsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-source-globs Warn, got %+v", c)
		}
		if !strings.Contains(c.Message, "built-in default") {
			t.Errorf("message must disclose the built-in default: %q", c.Message)
		}
	})

	t.Run("config present with empty list", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "source_globs: []\n")
		config.ResetCache()

		r := &Report{}
		checkSourceGlobs(r, root)

		c := findCheck(r, sourceGlobsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-source-globs Warn for empty list, got %+v", c)
		}
	})

	t.Run("config present with populated list clears the warn", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "source_globs:\n  - cmd/**\n")
		config.ResetCache()

		r := &Report{}
		checkSourceGlobs(r, root)

		c := findCheck(r, sourceGlobsCheckName)
		if c == nil {
			t.Fatal("expected a source_globs check")
		}
		if c.Status != OK {
			t.Errorf("expected OK (no warn) when source_globs populated, got status %d msg %q", c.Status, c.Message)
		}
	})
}

// TestScaffoldSourceGlobs_ThreeStates exercises spec 091 Req 11 fixer
// behavior across the three config states, including byte-identity of
// prior content on the append path (V2-2 raw-byte detection).
func TestScaffoldSourceGlobs_ThreeStates(t *testing.T) {
	t.Run("file absent → created with exactly the block", func(t *testing.T) {
		root := t.TempDir()
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		if err := scaffoldSourceGlobs(configPath); err != nil {
			t.Fatal(err)
		}
		config.ResetCache()

		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != sourceGlobsBlock {
			t.Errorf("created file must equal the block exactly.\n--- got ---\n%s\n--- want ---\n%s", got, sourceGlobsBlock)
		}
		// The typed loader now sees an empty (absent-equivalent) slice.
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.SourceGlobs) != 0 {
			t.Errorf("scaffolded block must be empty, got %v", cfg.SourceGlobs)
		}
	})

	t.Run("file present without field → block appended, prior bytes unchanged", func(t *testing.T) {
		root := t.TempDir()
		prior := "merge_strategy: auto\nworktree_root: .worktrees\n"
		writeConfig(t, root, prior)
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		if err := scaffoldSourceGlobs(configPath); err != nil {
			t.Fatal(err)
		}
		config.ResetCache()

		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(got), prior) {
			t.Errorf("prior bytes must be preserved verbatim at the head.\n--- got ---\n%s", got)
		}
		if !strings.Contains(string(got), "source_globs: []") {
			t.Errorf("appended file must contain the source_globs block: %s", got)
		}
	})

	t.Run("file present with field → byte-identical (no-op)", func(t *testing.T) {
		root := t.TempDir()
		prior := "source_globs:\n  - cmd/**\nmerge_strategy: auto\n"
		writeConfig(t, root, prior)
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		before, _ := os.ReadFile(configPath)
		if err := scaffoldSourceGlobs(configPath); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(configPath)
		if string(before) != string(after) {
			t.Errorf("file with existing source_globs must be byte-identical.\nbefore=%q\nafter=%q", before, after)
		}
	})

	t.Run("file present with empty field → byte-identical (no-op)", func(t *testing.T) {
		root := t.TempDir()
		prior := "merge_strategy: auto\nsource_globs: []\n"
		writeConfig(t, root, prior)
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		before, _ := os.ReadFile(configPath)
		if err := scaffoldSourceGlobs(configPath); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(configPath)
		if string(before) != string(after) {
			t.Errorf("file with empty source_globs must be byte-identical.\nbefore=%q\nafter=%q", before, after)
		}
	})
}

// TestSourceGlobsFixWiring proves the missing-source-globs Warn is
// fixable and that running the fix clears the Warn on the next check.
func TestSourceGlobsFixWiring(t *testing.T) {
	root := t.TempDir()
	config.ResetCache()

	r := &Report{}
	checkSourceGlobs(r, root)
	c := findCheck(r, sourceGlobsCheckName)
	if c == nil || c.FixFunc == nil {
		t.Fatal("expected a fixable missing-source-globs check")
	}
	r.Fix()
	config.ResetCache()

	// After scaffolding, the block is empty so the Warn still fires
	// (Req 18 collapses empty to the warn state) — but the file now
	// exists with the documented block.
	configPath := filepath.Join(root, ".mindspec", "config.yaml")
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("fixer did not create config.yaml: %v", err)
	}
	if string(got) != sourceGlobsBlock {
		t.Errorf("fixer must write the documented block verbatim, got:\n%s", got)
	}
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
