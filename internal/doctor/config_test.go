package doctor

import (
	"os"
	"path/filepath"
	"regexp"
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

// TestScaffoldSourceGlobs_WhitespaceBeforeColon is the panel R3
// regression (BLOCKER + MUT-G). YAML permits whitespace before a
// mapping-key colon, so `source_globs : []`, `source_globs\t: []`, and
// `source_globs  :  []` all normalize to the key `source_globs` and
// config.Load succeeds with an empty slice. The raw-byte key detector
// MUST agree: it must treat these as PRESENT (State 3 no-op), never
// append a duplicate block. Appending a duplicate top-level
// `source_globs:` key makes config.Load fail "mapping key already
// defined" — corrupting a loadable config via --fix.
//
// This test fails against the 74943ab regex `(?m)^source_globs:` (which
// requires an immediately-adjacent colon) and passes after the
// `(?m)^source_globs[ \t]*:` fix. It also pins the (?m)^ anchor: the
// commented control case must STILL be classified absent (fixer appends)
// AND the result must config.Load cleanly with exactly one live key.
func TestScaffoldSourceGlobs_WhitespaceBeforeColon(t *testing.T) {
	// Whitespace-before-colon variants: each is a PRESENT key, so the
	// fixer must be a strict byte-identical no-op and config.Load must
	// stay clean (no duplicate key introduced).
	presentVariants := []struct {
		name  string
		prior string
	}{
		{"space before colon", "source_globs : []\n"},
		{"tab before colon", "source_globs\t: []\n"},
		{"two spaces around colon", "source_globs  :  []\n"},
		{"space before colon after other content", "merge_strategy: auto\nsource_globs : []\n"},
		{"populated with space before colon", "source_globs :\n  - cmd/**\n"},
	}
	for _, tc := range presentVariants {
		t.Run("present/"+tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, tc.prior)
			config.ResetCache()
			configPath := filepath.Join(root, ".mindspec", "config.yaml")

			// config.Load must succeed BEFORE the fix (a loadable config).
			if _, err := config.Load(root); err != nil {
				t.Fatalf("precondition: config.Load must succeed before fix, got %v", err)
			}
			config.ResetCache()

			before, _ := os.ReadFile(configPath)
			if err := scaffoldSourceGlobs(configPath); err != nil {
				t.Fatalf("scaffoldSourceGlobs: %v", err)
			}
			after, _ := os.ReadFile(configPath)

			// State-3 no-op: bytes identical, no duplicate key appended.
			if string(before) != string(after) {
				t.Errorf("whitespace-before-colon key must be treated as PRESENT (byte-identical no-op).\nbefore=%q\nafter=%q", before, after)
			}
			if n := strings.Count(string(after), "source_globs"); n != 1 {
				t.Errorf("expected exactly one source_globs key, got %d occurrences:\n%s", n, after)
			}

			// config.Load must STILL succeed after the fix (not corrupted).
			config.ResetCache()
			if _, err := config.Load(root); err != nil {
				t.Errorf("config.Load must succeed after --fix; --fix corrupted a loadable config: %v", err)
			}
			config.ResetCache()
		})
	}

	// Control: a COMMENTED key is functionally absent (the (?m)^ anchor
	// must NON-match it), so the fixer APPENDS — and the result must
	// config.Load cleanly with exactly one LIVE source_globs key.
	t.Run("absent/commented key gets appended and loads clean", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "# source_globs: []\nmerge_strategy: auto\n")
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		if err := scaffoldSourceGlobs(configPath); err != nil {
			t.Fatalf("scaffoldSourceGlobs: %v", err)
		}
		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		// Pin the (?m)^ anchor independently of the production regex: a
		// line-leading uncommented `source_globs:` key MUST have been
		// appended (the commented key must NOT have suppressed the
		// append). If the ^ anchor is dropped from sourceGlobsKeyRE, the
		// commented line matches → no append → this count is 0 → RED.
		liveKeyRE := regexp.MustCompile(`(?m)^source_globs[ \t]*:`)
		if liveKeys := liveKeyRE.FindAllString(string(got), -1); len(liveKeys) != 1 {
			t.Errorf("expected exactly one LIVE (line-leading) source_globs key after append, got %d:\n%s", len(liveKeys), got)
		}
		config.ResetCache()
		if _, err := config.Load(root); err != nil {
			t.Errorf("config.Load must succeed after appending to a commented-key file: %v", err)
		}
		config.ResetCache()
	})
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
