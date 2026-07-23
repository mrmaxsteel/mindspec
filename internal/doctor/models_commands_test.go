package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// models_commands_test.go — spec 123 R6/R7c: the models: guidance-parity
// stack (AC-11/AC-12) and its commands: sibling, extended by the shared
// three-state scaffolder's PF-2 identity/parity pin.

const modelsCheckName = ".mindspec/config.yaml models"
const commandsCheckName = ".mindspec/config.yaml commands"

// TestMissingModels_AllThreeStates pins spec 123 AC-11(i)/(iv): the
// missing-models Warn fires when config.yaml is absent, present without
// the field, or present with an empty map; and clears once a non-empty
// map exists. RED on today's main: no models check exists at all.
func TestMissingModels_AllThreeStates(t *testing.T) {
	t.Run("config file absent", func(t *testing.T) {
		root := t.TempDir()
		config.ResetCache()

		r := &Report{}
		checkModels(r, root)

		c := findCheck(r, modelsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-models Warn, got %+v", c)
		}
		if !strings.Contains(c.Message, "missing-models") {
			t.Errorf("message missing the warn name: %q", c.Message)
		}
		if !strings.Contains(c.Message, "declared-and-inert") {
			t.Errorf("message must disclose the declared-and-inert status: %q", c.Message)
		}
		if !strings.Contains(c.Message, "mindspec models populate") {
			t.Errorf("message must hint models populate: %q", c.Message)
		}
	})

	t.Run("config present without field", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "merge_strategy: auto\n")
		config.ResetCache()

		r := &Report{}
		checkModels(r, root)

		c := findCheck(r, modelsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-models Warn, got %+v", c)
		}
	})

	t.Run("config present with empty map", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "models: {}\n")
		config.ResetCache()

		r := &Report{}
		checkModels(r, root)

		c := findCheck(r, modelsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-models Warn for empty map, got %+v", c)
		}
	})

	t.Run("config present with blank-valued key still warns (FX-2)", func(t *testing.T) {
		root := t.TempDir()
		// map-length 1 but the model id is blank → NOT declared.
		writeConfig(t, root, "models:\n  authoring: \"\"\n")
		config.ResetCache()

		r := &Report{}
		checkModels(r, root)

		c := findCheck(r, modelsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-models Warn for a blank-valued models key, got %+v", c)
		}
	})

	t.Run("config present with populated map clears the warn", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "models:\n  authoring: claude-opus-4-8\n")
		config.ResetCache()

		r := &Report{}
		checkModels(r, root)

		c := findCheck(r, modelsCheckName)
		if c == nil {
			t.Fatal("expected a models check")
		}
		if c.Status != OK {
			t.Errorf("expected OK (no warn) when models populated, got status %d msg %q", c.Status, c.Message)
		}
	})
}

// TestScaffoldModelsBlock_ThreeStates mirrors
// TestScaffoldSourceGlobs_ThreeStates for the models: block (AC-11(ii)):
// file absent -> created verbatim; file present without the field ->
// appended, prior bytes preserved; file present with the field (empty or
// populated) -> byte-identical no-op (operator bytes never rewritten).
func TestScaffoldModelsBlock_ThreeStates(t *testing.T) {
	t.Run("file absent → created with exactly the block", func(t *testing.T) {
		root := t.TempDir()
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		if err := scaffoldModelsBlock(configPath); err != nil {
			t.Fatal(err)
		}
		config.ResetCache()

		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != modelsBlock {
			t.Errorf("created file must equal the block exactly.\n--- got ---\n%s\n--- want ---\n%s", got, modelsBlock)
		}
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Models) != 0 {
			t.Errorf("scaffolded block must be empty, got %v", cfg.Models)
		}
	})

	t.Run("file present without field → block appended, prior bytes unchanged", func(t *testing.T) {
		root := t.TempDir()
		prior := "merge_strategy: auto\nworktree_root: .worktrees\n"
		writeConfig(t, root, prior)
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		if err := scaffoldModelsBlock(configPath); err != nil {
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
		if !strings.Contains(string(got), "models: {}") {
			t.Errorf("appended file must contain the models block: %s", got)
		}
	})

	t.Run("file present with field → byte-identical (no-op)", func(t *testing.T) {
		root := t.TempDir()
		prior := "models:\n  authoring: claude\nmerge_strategy: auto\n"
		writeConfig(t, root, prior)
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		before, _ := os.ReadFile(configPath)
		if err := scaffoldModelsBlock(configPath); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(configPath)
		if string(before) != string(after) {
			t.Errorf("file with existing models must be byte-identical.\nbefore=%q\nafter=%q", before, after)
		}
	})
}

// TestModelsFixWiring mirrors TestSourceGlobsFixWiring: the
// missing-models Warn is fixable, and running the fix scaffolds the
// documented block verbatim.
func TestModelsFixWiring(t *testing.T) {
	root := t.TempDir()
	config.ResetCache()

	r := &Report{}
	checkModels(r, root)
	c := findCheck(r, modelsCheckName)
	if c == nil || c.FixFunc == nil {
		t.Fatal("expected a fixable missing-models check")
	}
	r.Fix()
	config.ResetCache()

	configPath := filepath.Join(root, ".mindspec", "config.yaml")
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("fixer did not create config.yaml: %v", err)
	}
	if string(got) != modelsBlock {
		t.Errorf("fixer must write the documented block verbatim, got:\n%s", got)
	}
}

// TestModelsBlockHonesty pins spec 123 AC-12: the scaffolded models:
// block asserts the key is declared-AND-inert and names the follow-up
// enforcement path; this test fails if the block instead claims (or
// stops disclaiming) enforcement.
func TestModelsBlockHonesty(t *testing.T) {
	for _, want := range []string{
		"DECLARED-AND-INERT",
		"nothing in",
		"this binary reads this key",
		"orchestration skills",
		"follow-up",
	} {
		if !strings.Contains(modelsBlock, want) {
			t.Errorf("modelsBlock missing honesty phrase %q:\n%s", want, modelsBlock)
		}
	}
	// Negative pin: the block must never claim the key drives behavior.
	for _, forbidden := range []string{"is enforced", "enforces", "changes behavior"} {
		if strings.Contains(modelsBlock, forbidden) {
			t.Errorf("modelsBlock must not claim enforcement (%q found):\n%s", forbidden, modelsBlock)
		}
	}
}

// TestMissingCommands_AllThreeStates mirrors TestMissingModels_AllThreeStates
// for spec 123 R7c's commands: guidance-parity Warn.
func TestMissingCommands_AllThreeStates(t *testing.T) {
	t.Run("config file absent", func(t *testing.T) {
		root := t.TempDir()
		config.ResetCache()

		r := &Report{}
		checkCommands(r, root)

		c := findCheck(r, commandsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-commands Warn, got %+v", c)
		}
		if !strings.Contains(c.Message, "missing-commands") {
			t.Errorf("message missing the warn name: %q", c.Message)
		}
		if !strings.Contains(c.Message, "mindspec commands populate") {
			t.Errorf("message must hint commands populate: %q", c.Message)
		}
	})

	t.Run("config present without field", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "merge_strategy: auto\n")
		config.ResetCache()

		r := &Report{}
		checkCommands(r, root)

		c := findCheck(r, commandsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-commands Warn, got %+v", c)
		}
	})

	t.Run("config present with empty map", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "commands: {}\n")
		config.ResetCache()

		r := &Report{}
		checkCommands(r, root)

		c := findCheck(r, commandsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-commands Warn for empty map, got %+v", c)
		}
	})

	t.Run("config present with blank-valued key still warns (FX-2)", func(t *testing.T) {
		root := t.TempDir()
		// map-length 1 but the command is blank → NOT declared, and the
		// managed Build & Test section must stay omitted.
		writeConfig(t, root, "commands:\n  build: \"\"\n")
		config.ResetCache()

		r := &Report{}
		checkCommands(r, root)

		c := findCheck(r, commandsCheckName)
		if c == nil || c.Status != Warn {
			t.Fatalf("expected missing-commands Warn for a blank-valued commands key, got %+v", c)
		}
	})

	t.Run("config present with populated map clears the warn", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "commands:\n  build: make build\n")
		config.ResetCache()

		r := &Report{}
		checkCommands(r, root)

		c := findCheck(r, commandsCheckName)
		if c == nil {
			t.Fatal("expected a commands check")
		}
		if c.Status != OK {
			t.Errorf("expected OK (no warn) when commands populated, got status %d msg %q", c.Status, c.Message)
		}
	})
}

// TestScaffoldCommandsBlock_ThreeStates mirrors
// TestScaffoldModelsBlock_ThreeStates for the commands: block — the
// PF-2 "commands could be omitted or privately reimplemented" gap this
// bead closes: a per-key three-state test proves the commands consumer
// gets the SAME write discipline as source_globs/models.
func TestScaffoldCommandsBlock_ThreeStates(t *testing.T) {
	t.Run("file absent → created with exactly the block", func(t *testing.T) {
		root := t.TempDir()
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		if err := scaffoldCommandsBlock(configPath); err != nil {
			t.Fatal(err)
		}
		config.ResetCache()

		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != commandsBlock {
			t.Errorf("created file must equal the block exactly.\n--- got ---\n%s\n--- want ---\n%s", got, commandsBlock)
		}
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Commands) != 0 {
			t.Errorf("scaffolded block must be empty, got %v", cfg.Commands)
		}
	})

	t.Run("file present without field → block appended, prior bytes unchanged", func(t *testing.T) {
		root := t.TempDir()
		prior := "merge_strategy: auto\nworktree_root: .worktrees\n"
		writeConfig(t, root, prior)
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		if err := scaffoldCommandsBlock(configPath); err != nil {
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
		if !strings.Contains(string(got), "commands: {}") {
			t.Errorf("appended file must contain the commands block: %s", got)
		}
	})

	t.Run("file present with field → byte-identical (no-op)", func(t *testing.T) {
		root := t.TempDir()
		prior := "commands:\n  build: make build\nmerge_strategy: auto\n"
		writeConfig(t, root, prior)
		config.ResetCache()
		configPath := filepath.Join(root, ".mindspec", "config.yaml")

		before, _ := os.ReadFile(configPath)
		if err := scaffoldCommandsBlock(configPath); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(configPath)
		if string(before) != string(after) {
			t.Errorf("file with existing commands must be byte-identical.\nbefore=%q\nafter=%q", before, after)
		}
	})
}

// TestCommandsFixWiring mirrors TestModelsFixWiring for commands:.
func TestCommandsFixWiring(t *testing.T) {
	root := t.TempDir()
	config.ResetCache()

	r := &Report{}
	checkCommands(r, root)
	c := findCheck(r, commandsCheckName)
	if c == nil || c.FixFunc == nil {
		t.Fatal("expected a fixable missing-commands check")
	}
	r.Fix()
	config.ResetCache()

	configPath := filepath.Join(root, ".mindspec", "config.yaml")
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("fixer did not create config.yaml: %v", err)
	}
	if string(got) != commandsBlock {
		t.Errorf("fixer must write the documented block verbatim, got:\n%s", got)
	}
}

// TestScaffoldConfigBlock_ThreeStates_AllKeys is the PF-2 shared-
// scaffolder identity/parity pin: a table-driven test drives the ONE
// scaffoldConfigBlock function — through each key's own public
// entrypoint (scaffoldSourceGlobs / scaffoldModelsBlock /
// scaffoldCommandsBlock) — across the three write-discipline states,
// proving all three consumers get IDENTICAL behavior. A private
// per-key reimplementation of any one consumer that diverges from this
// contract (e.g. rewriting operator bytes, or failing to preserve
// prior content on append) fails this test even though each key's own
// missing-X Warn might still individually appear to work.
func TestScaffoldConfigBlock_ThreeStates_AllKeys(t *testing.T) {
	cases := []struct {
		name       string
		scaffold   func(string) error
		block      string
		keyLiteral string // the top-level "key:" text expected in the block
	}{
		{"source_globs", scaffoldSourceGlobs, sourceGlobsBlock, "source_globs: []"},
		{"models", scaffoldModelsBlock, modelsBlock, "models: {}"},
		{"commands", scaffoldCommandsBlock, commandsBlock, "commands: {}"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/file absent", func(t *testing.T) {
			root := t.TempDir()
			configPath := filepath.Join(root, ".mindspec", "config.yaml")
			if err := tc.scaffold(configPath); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.block {
				t.Errorf("%s: created file must equal the block exactly:\n%s", tc.name, got)
			}
		})

		t.Run(tc.name+"/file present without key → appended, prior bytes preserved", func(t *testing.T) {
			root := t.TempDir()
			prior := "merge_strategy: auto\n"
			writeConfig(t, root, prior)
			configPath := filepath.Join(root, ".mindspec", "config.yaml")
			if err := tc.scaffold(configPath); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(got), prior) {
				t.Errorf("%s: prior bytes must be preserved at the head:\n%s", tc.name, got)
			}
			if !strings.Contains(string(got), tc.keyLiteral) {
				t.Errorf("%s: appended file must contain %q:\n%s", tc.name, tc.keyLiteral, got)
			}
		})

		t.Run(tc.name+"/file present with key → byte-identical no-op, operator bytes never rewritten", func(t *testing.T) {
			root := t.TempDir()
			prior := "merge_strategy: auto\n" + tc.keyLiteral + "\n"
			writeConfig(t, root, prior)
			configPath := filepath.Join(root, ".mindspec", "config.yaml")
			before, _ := os.ReadFile(configPath)
			if err := tc.scaffold(configPath); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(configPath)
			if string(before) != string(after) {
				t.Errorf("%s: file with existing key must be byte-identical.\nbefore=%q\nafter=%q", tc.name, before, after)
			}
		})
	}
}

// TestADR0040ConsumerIdentityClauseCited pins spec 123 AC-18's
// config-block half: the models: and commands: schema blocks each cite
// ADR-0040 by name, so the ADR-divergence gate sees the declared
// touchpoint at these config-block sites (R9's citation requirement).
// The ADR text itself, and the managed-content citation sites in
// internal/bootstrap and internal/setup, are pinned by
// cmd/mindspec's TestADR0040ConsumerIdentityClause_Anchored (the
// cross-package anchor test).
func TestADR0040ConsumerIdentityClauseCited(t *testing.T) {
	if !strings.Contains(modelsBlock+builtinModelsDisclosure, "ADR-0040") {
		t.Error("modelsBlock/builtinModelsDisclosure must cite ADR-0040")
	}
	if !strings.Contains(commandsBlock, "ADR-0040") {
		t.Error("commandsBlock must cite ADR-0040")
	}
}
