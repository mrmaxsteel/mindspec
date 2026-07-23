package config

import (
	"strings"
	"testing"
)

// commands_test.go — spec 123 R7b: the Commands config field and its two
// renderers, CommandLines (stable ordering + termsafe escaping) and
// RenderBuildTestSection (the managed-content Build & Test section every
// call site — bootstrap's starterAgentsMD/appendAgentsBlock, setup's
// agentsMDManagedBlock — shares).

func TestConfig_CommandsDefaultEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Commands) != 0 {
		t.Errorf("DefaultConfig().Commands: want empty, got %v", cfg.Commands)
	}
}

// TestCommandLines_StableOrder pins the commandOrder contract: build,
// test (when present), then every other declared key sorted — so two
// independent renderers can never disagree about order.
func TestCommandLines_StableOrder(t *testing.T) {
	cfg := &Config{Commands: map[string]string{
		"lint":  "golangci-lint run",
		"test":  "go test ./...",
		"build": "go build ./...",
	}}
	lines := cfg.CommandLines()
	want := []string{
		"go build ./...   # build",
		"go test ./...   # test",
		"golangci-lint run   # lint",
	}
	if len(lines) != len(want) {
		t.Fatalf("CommandLines(): got %d lines, want %d: %v", len(lines), len(want), lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("CommandLines()[%d]: got %q, want %q", i, lines[i], w)
		}
	}
}

// TestCommandLines_OnlyTestDeclared covers the "build absent, test
// present" partial-declaration shape: build is skipped (never a
// placeholder), test renders alone.
func TestCommandLines_OnlyTestDeclared(t *testing.T) {
	cfg := &Config{Commands: map[string]string{"test": "npm test"}}
	lines := cfg.CommandLines()
	if len(lines) != 1 || lines[0] != "npm test   # test" {
		t.Errorf("CommandLines(): got %v, want exactly [\"npm test   # test\"]", lines)
	}
}

func TestCommandLines_EmptyReturnsNil(t *testing.T) {
	cfg := &Config{}
	if got := cfg.CommandLines(); got != nil {
		t.Errorf("CommandLines() on unset Commands: got %v, want nil", got)
	}
}

// TestCommandLines_EscapesHostileValue covers the termsafe routing (spec
// 116 AC6): a command value carrying a control byte must never reach the
// rendered line raw — the same class of value `mindspec config show`
// already escapes for models.
func TestCommandLines_EscapesHostileValue(t *testing.T) {
	cfg := &Config{Commands: map[string]string{"build": "echo hi\x1b[2Jinjected"}}
	lines := cfg.CommandLines()
	if len(lines) != 1 {
		t.Fatalf("expected exactly one line, got %v", lines)
	}
	if strings.ContainsRune(lines[0], 0x1b) {
		t.Errorf("CommandLines() must not emit a raw ESC byte: %q", lines[0])
	}
}

func TestRenderBuildTestSection_EmptyReturnsEmptyString(t *testing.T) {
	cfg := &Config{}
	if got := cfg.RenderBuildTestSection(2); got != "" {
		t.Errorf("RenderBuildTestSection(2) on unset Commands: got %q, want \"\"", got)
	}
}

// TestCommandLines_AllBlankValuesReturnsNil pins spec 123 FX-2
// (empty≠declared): a commands: map whose only entries carry blank
// (trimmed-empty) values declares NO runnable command — CommandLines must
// return nil so RenderBuildTestSection omits the section entirely (never
// a runnable-command-less "Build & Test" block).
func TestCommandLines_AllBlankValuesReturnsNil(t *testing.T) {
	cfg := &Config{Commands: map[string]string{"build": "", "test": "   "}}
	if got := cfg.CommandLines(); got != nil {
		t.Errorf("CommandLines() on all-blank Commands: got %v, want nil", got)
	}
	if got := cfg.RenderBuildTestSection(2); got != "" {
		t.Errorf("RenderBuildTestSection(2) on all-blank Commands: got %q, want \"\"", got)
	}
}

// TestCommandLines_SkipsBlankKeepsNonBlank confirms a mix: a blank build
// value is skipped while a non-blank test value still renders.
func TestCommandLines_SkipsBlankKeepsNonBlank(t *testing.T) {
	cfg := &Config{Commands: map[string]string{"build": "", "test": "go test ./..."}}
	lines := cfg.CommandLines()
	if len(lines) != 1 || lines[0] != "go test ./...   # test" {
		t.Errorf("CommandLines() must skip the blank build and keep test, got %v", lines)
	}
}

// TestHasDeclaredCommandsModels_BlankNotDeclared pins the FX-2
// completeness predicate: an all-blank map is NOT declared.
func TestHasDeclaredCommandsModels_BlankNotDeclared(t *testing.T) {
	blankCmds := &Config{Commands: map[string]string{"build": ""}}
	if blankCmds.HasDeclaredCommands() {
		t.Error("HasDeclaredCommands() must be false for a blank-valued map")
	}
	realCmds := &Config{Commands: map[string]string{"build": "make"}}
	if !realCmds.HasDeclaredCommands() {
		t.Error("HasDeclaredCommands() must be true for a non-blank map")
	}
	blankModels := &Config{Models: map[string]string{"authoring": "   "}}
	if blankModels.HasDeclaredModels() {
		t.Error("HasDeclaredModels() must be false for a blank-valued map")
	}
	realModels := &Config{Models: map[string]string{"authoring": "claude"}}
	if !realModels.HasDeclaredModels() {
		t.Error("HasDeclaredModels() must be true for a non-blank map")
	}
}

// TestRenderBuildTestSection_HeadingLevel pins the two heading depths
// real call sites use: level 2 for a top-level managed block, level 3
// for content nested under a parent heading.
func TestRenderBuildTestSection_HeadingLevel(t *testing.T) {
	cfg := &Config{Commands: map[string]string{"build": "make build", "test": "make test"}}

	got2 := cfg.RenderBuildTestSection(2)
	if !strings.Contains(got2, "## Build & Test") {
		t.Errorf("level 2: expected \"## Build & Test\" heading, got:\n%s", got2)
	}
	if strings.Contains(got2, "### Build & Test") {
		t.Errorf("level 2: must not render an H3 heading, got:\n%s", got2)
	}

	got3 := cfg.RenderBuildTestSection(3)
	if !strings.Contains(got3, "### Build & Test") {
		t.Errorf("level 3: expected \"### Build & Test\" heading, got:\n%s", got3)
	}

	for _, want := range []string{"```bash\n", "make build   # build\n", "make test   # test\n", "```\n"} {
		if !strings.Contains(got2, want) {
			t.Errorf("RenderBuildTestSection(2) missing %q, got:\n%s", want, got2)
		}
	}
}
