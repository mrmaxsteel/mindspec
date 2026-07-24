package config

import (
	"os"
	"path/filepath"
	"runtime"
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

// TestRenderBuildTestSection_CiRendersInVocabularyPosition pins spec 126
// R8a/AC-17: RenderBuildTestSection (config.go:320) on a fixture declaring
// build, test, ci, AND an extension key that sorts lexically BEFORE "ci"
// (fixture key "aa") must render the lines in exactly build, test, ci,
// aa order — "ci" in ITS OWN vocabulary position, ahead of the
// lexically-earlier extension key. The sorted-rest fallback in
// CommandLines already renders any declared key regardless of the
// commandOrder entry, so presence of the "ci" line ALONE cannot pass this
// test — only the ORDER can. Reverting the commandOrder "ci" entry
// (config.go:233) reorders "ci" after "aa" and REDs this test.
func TestRenderBuildTestSection_CiRendersInVocabularyPosition(t *testing.T) {
	cfg := &Config{Commands: map[string]string{
		"aa":    "aa-command",
		"ci":    "ci-command",
		"test":  "test-command",
		"build": "build-command",
	}}
	got := cfg.RenderBuildTestSection(2)

	wantOrder := []string{
		"build-command   # build",
		"test-command   # test",
		"ci-command   # ci",
		"aa-command   # aa",
	}
	var idx []int
	for _, w := range wantOrder {
		i := strings.Index(got, w)
		if i < 0 {
			t.Fatalf("RenderBuildTestSection(2) missing line %q, got:\n%s", w, got)
		}
		idx = append(idx, i)
	}
	for i := 1; i < len(idx); i++ {
		if idx[i] <= idx[i-1] {
			t.Fatalf("RenderBuildTestSection(2) lines out of order — want exactly build, test, ci, aa; got:\n%s", got)
		}
	}
}

// TestCommandsFieldComment_DocumentsCiKey is the spec 126 R8a/AC-11
// config-half pin: the Commands field's doc comment in config.go must
// name the "ci" vocabulary key beside build/test, AND must document the
// undeclared-ci -> test fallback via the DISTINCTIVE "commands.test"
// token — not the bare vocabulary word "test", which already appears
// earlier in the same comment (in the "build", "test", and "ci" key
// list) and would make a bare-"test" guard pass vacuously even after the
// fallback sentence itself is deleted. A source-level grep
// (runtime.Caller(0)-resolved, the same technique
// internal/panel/leaf_imports_test.go uses to locate its own package
// file) rather than a mirrored string constant, so a future comment
// rewrite that silently drops "ci" is caught without this test also
// having to duplicate the exact wording.
func TestCommandsFieldComment_DocumentsCiKey(t *testing.T) {
	src := readConfigGoSource(t)
	comment := commandsFieldComment(t, src)

	if !strings.Contains(comment, `"ci"`) {
		t.Errorf("Commands field comment does not name the \"ci\" vocabulary key:\n%s", comment)
	}
	if !commandsFieldCommentDocumentsFallback(comment) {
		t.Errorf("Commands field comment does not document the commands.test fallback semantics (missing the distinctive \"commands.test\" token):\n%s", comment)
	}
}

// TestCommandsFieldComment_FallbackSentenceDeletionReds is the mutation
// probe for the guard above: deleting ONLY the undeclared-ci -> test
// fallback sentence (while leaving the "build"/"test"/"ci" vocabulary
// list intact — the bare word "test" still appears there) must turn
// commandsFieldCommentDocumentsFallback false. Before the "commands.test"
// token was introduced, the guard used `!Contains("commands.test") &&
// !Contains(\`"test"\`)`, which this exact deletion left green (the
// vocabulary word alone satisfied it).
func TestCommandsFieldComment_FallbackSentenceDeletionReds(t *testing.T) {
	src := readConfigGoSource(t)
	comment := commandsFieldComment(t, src)

	if !commandsFieldCommentDocumentsFallback(comment) {
		t.Fatal("precondition failed: the shipped Commands field comment must currently document the fallback")
	}

	const fallbackSentence = "when \"ci\" is undeclared,\n\t// commands.test is the documented fallback."
	if !strings.Contains(comment, fallbackSentence) {
		t.Fatalf("fixture assumption broken: could not locate the exact fallback sentence to delete in:\n%s", comment)
	}
	mutated := strings.Replace(comment, fallbackSentence, "", 1)

	// Sanity: the vocabulary word "test" still survives the deletion (it
	// lives in the earlier "build"/"test"/"ci" key list), so a bare-word
	// guard would stay green — only the distinctive-token guard may RED.
	if !strings.Contains(mutated, `"test"`) {
		t.Fatal("fixture assumption broken: deleting the fallback sentence must not remove the earlier vocabulary-list \"test\" mention")
	}
	if commandsFieldCommentDocumentsFallback(mutated) {
		t.Errorf("mutation probe failed: deleting the fallback sentence should turn commandsFieldCommentDocumentsFallback false, got true for:\n%s", mutated)
	}
}

// readConfigGoSource reads config.go's full source from disk, resolving
// its path relative to this test file via runtime.Caller(0).
func readConfigGoSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to resolve this test file's path")
	}
	configPath := filepath.Join(filepath.Dir(thisFile), "config.go")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}
	return string(data)
}

// commandsFieldComment extracts the Commands field's doc comment block
// from config.go's source text.
func commandsFieldComment(t *testing.T, src string) string {
	t.Helper()
	commentStart := strings.Index(src, "// Commands declares the CONSUMER's build/test guidance")
	if commentStart < 0 {
		t.Fatal("could not locate the Commands field's doc comment in config.go")
	}
	fieldIdx := strings.Index(src[commentStart:], "Commands map[string]string")
	if fieldIdx < 0 {
		t.Fatal("could not locate the Commands field declaration following its doc comment")
	}
	return src[commentStart : commentStart+fieldIdx]
}

// commandsFieldCommentDocumentsFallback requires the DISTINCTIVE
// "commands.test" token, never the bare vocabulary word "test" (which
// the comment's earlier "build"/"test"/"ci" key list already contains
// regardless of whether the fallback sentence exists).
func commandsFieldCommentDocumentsFallback(comment string) bool {
	return strings.Contains(comment, "commands.test")
}
