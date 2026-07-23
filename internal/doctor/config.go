package doctor

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// sourceGlobsBlock is the LITERAL scaffolded default for the
// source_globs config field (spec 091 Req 11), verbatim — comment plus
// `source_globs: []`. The framework never scaffolds any glob values;
// the agent/operator populates them via `mindspec source populate`.
// This constant is the doctor fixer's scaffolding responsibility (the
// sole scaffolder of this block per Req 11); the typed field lives in
// internal/config.
const sourceGlobsBlock = `# source_globs: which path globs count as "source" for
# the doc-sync gate. OVERRIDE semantics: a non-empty
# list FULLY REPLACES mindspec's built-in default
# (never a union with it). While this list is empty,
# mindspec falls back to its built-in classifier:
# files under cmd/ or internal/ ending in .go,
# excluding _test.go. The framework does NOT guess
# repo-specific globs — operator/agent declares them
# (run ` + "`mindspec source populate`" + ` for an agent prompt).
# While empty, the unclaimed-source Warn is disabled
# (doctor fires missing-source-globs as a nudge).
source_globs: []
`

// builtinSourceDefaultDisclosure is the human-readable description of
// the built-in source classifier, used by the missing-source-globs
// Warn (Req 18) to DISCLOSE the active default. The literal phrase
// "built-in default" is asserted by tests.
const builtinSourceDefaultDisclosure = "doc-sync is classifying source with the built-in default: .go files under cmd/ and internal/, excluding _test.go"

// sourceGlobsKeyRE matches a top-level (unindented, uncommented)
// source_globs: key in raw config.yaml bytes. Raw-byte inspection is
// required (spec 091 V2-2): the typed config.Load cannot distinguish
// an absent source_globs: key from an explicit `source_globs: []` —
// both unmarshal to an empty slice.
//
// The optional [ \t]* before the colon is load-bearing: YAML permits
// whitespace before a mapping-key colon (`source_globs : []`,
// `source_globs\t: []`), normalizing all of them to the key
// `source_globs`. A raw-byte regex that demanded an immediately-adjacent
// colon would classify such a hand-edited key as ABSENT and append a
// duplicate `source_globs:` block — turning a loadable config into an
// unparseable one ("mapping key already defined") on `doctor --fix`.
// The (?m)^ multiline anchor is also load-bearing: it keeps commented
// (`# source_globs:`), indented (`  source_globs:`), and suffixed
// (`extra_source_globs:`) keys correctly classified as NON-matches.
var sourceGlobsKeyRE = regexp.MustCompile(`(?m)^source_globs[ \t]*:`)

// checkSourceGlobs emits the missing-source-globs Warn (spec 091
// Req 18) when .mindspec/config.yaml is absent, lacks the
// source_globs: field, or declares it empty. The Warn discloses the
// active built-in classifier and hints `mindspec source populate`. It
// is fixable: --fix scaffolds the literal source_globs block per the
// three Req 11 states (file absent / field absent / field present),
// never reordering or rewriting operator-authored content.
func checkSourceGlobs(r *Report, root string) {
	cfg, err := config.Load(root)
	if err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    ".mindspec/config.yaml source_globs",
			Status:  Warn,
			Message: "could not load .mindspec/config.yaml: " + err.Error(),
		})
		return
	}

	// Req 18 collapses all three states to len == 0; config.Load
	// delivers that. A non-empty list clears the Warn.
	if len(cfg.SourceGlobs) > 0 {
		r.Checks = append(r.Checks, Check{
			Name:    ".mindspec/config.yaml source_globs",
			Status:  OK,
			Message: "source_globs declared; doc-sync uses operator-declared classification",
		})
		return
	}

	configPath := filepath.Join(root, ".mindspec", "config.yaml")
	r.Checks = append(r.Checks, Check{
		Name:   ".mindspec/config.yaml source_globs",
		Status: Warn,
		Message: "missing-source-globs: source_globs not set in .mindspec/config.yaml — " +
			builtinSourceDefaultDisclosure +
			"; run 'mindspec source populate' to emit an agent prompt that declares your own",
		FixFunc: func() error {
			return scaffoldSourceGlobs(configPath)
		},
	})
}

// scaffoldConfigBlock writes block into configPath per the three
// ADR-0036 states — originally spec 091 Req 11 for source_globs, now
// generalized (spec 123 R6c/R7c) into the ONE scaffolder every one of
// the three doctor --fix legs (source_globs, models, commands) routes
// through, so a private per-key reimplementation cannot silently
// diverge from this write discipline (PF-2's shared-scaffolder identity
// pin — see TestScaffoldConfigBlock_ThreeStates_AllKeys):
//
//   - file absent              → create config.yaml with exactly block
//   - file present, no key      → APPEND block; prior bytes unchanged
//   - file present, has key     → leave the file byte-identical (no-op)
//
// The absent-vs-present decision inspects RAW file bytes (V2-2) — the
// typed loader cannot distinguish an absent key from an explicit empty
// value (`source_globs: []`, `models: {}`, `commands: {}`) — all three
// unmarshal to a zero-length collection. config.ResetCache() is called
// after any write so a fix-then-recheck flow in the same process
// observes the new state.
func scaffoldConfigBlock(configPath string, keyRE *regexp.Regexp, block string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// State 1: file absent — create with exactly the block.
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(configPath, []byte(block), 0o644); err != nil {
			return err
		}
		config.ResetCache()
		return nil
	}

	// State 3: key already present (raw-byte key-presence check) —
	// leave the file untouched.
	if keyRE.Match(data) {
		return nil
	}

	// State 2: file present without the key — APPEND the block,
	// preserving every prior byte. Ensure a separating newline so the
	// appended block starts on its own line.
	out := data
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, []byte(block)...)
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return err
	}
	config.ResetCache()
	return nil
}

// scaffoldSourceGlobs retains its exact spec-091 name and signature
// (existing tests pin it byte-for-byte) but now routes through the
// shared scaffoldConfigBlock.
func scaffoldSourceGlobs(configPath string) error {
	return scaffoldConfigBlock(configPath, sourceGlobsKeyRE, sourceGlobsBlock)
}

// modelsBlock is the LITERAL scaffolded default for the models: config
// field (spec 123 R6a), mirroring sourceGlobsBlock's shape — a
// commented schema doc plus `models: {}`. Per AC-12's honesty pin, the
// text asserts the key is declared-AND-INERT today (nothing in this
// binary reads it) and names its authoritative consumers (the
// orchestration skills) — the ADR-0040 consumer-identity clause's own
// requirement that a declared-but-inert key say so everywhere it is
// surfaced.
const modelsBlock = `# models: a free-form phase -> model-id map (advisory
# vocabulary: authoring, implementation, review — not an
# enforced enum). DECLARED-AND-INERT TODAY: nothing in
# this binary reads this key to change behavior; the
# authoritative consumers of the model protocol remain
# the orchestration skills (ms-bead-impl, ms-panel-run,
# ...), not the mindspec binary. Wiring in-binary
# enforcement is a named follow-up (ADR-0040's
# consumer-identity clause: a declared-but-inert key
# must say so everywhere it is surfaced). The framework
# proposes no model ids — operator/agent declares them
# (run ` + "`mindspec models populate`" + ` for an agent prompt).
models: {}
`

// commandsBlock is the LITERAL scaffolded default for the commands:
// config field (spec 123 R7c), the consumer build/test declaration —
// ADR-0036's ZFC stack extended a second time (after source_globs and
// models) so a repo-specific fact (the build/test commands) has a
// declared L2 home instead of being hardcoded into managed content
// (ADR-0040's consumer-identity clause). UNLIKE modelsBlock, this key
// is NOT inert: a populated commands: entry changes what init/setup
// render into AGENTS.md's "Build & Test" section today.
const commandsBlock = `# commands: your build/test guidance (task -> shell
# command), rendered into the managed AGENTS.md "Build
# & Test" section by ` + "`mindspec init`" + ` and every
# ` + "`mindspec setup <agent>`" + ` verb. Vocabulary keys: build,
# test (documented, not an enforced enum). While unset,
# the Build & Test section is OMITTED entirely — mindspec
# never guesses your build system (ADR-0036 ZFC;
# ADR-0040's consumer-identity clause: managed content
# carries only framework-generic guidance or your own
# declared values, never mindspec-the-framework's own
# build). Populate via ` + "`mindspec commands populate`" + `.
commands: {}
`

// builtinModelsDisclosure is the human-readable description the
// missing-models Warn (R6c) uses to disclose the key's inert status —
// mirroring builtinSourceDefaultDisclosure's role for source_globs.
const builtinModelsDisclosure = "models: is declared-and-inert today — nothing in the binary reads it; the orchestration skills remain the authoritative consumers of the model protocol (ADR-0040)"

// buildTestGuidanceDisclosure is the human-readable description the
// missing-commands Warn (R7c) uses to disclose why the managed AGENTS.md
// documents carry no Build & Test section yet.
const buildTestGuidanceDisclosure = "the managed AGENTS.md \"Build & Test\" section is omitted until commands: is declared — mindspec never guesses your build system (ADR-0036 ZFC)"

// modelsKeyRE matches a top-level (unindented, uncommented) models: key
// in raw config.yaml bytes, mirroring sourceGlobsKeyRE's rationale: the
// typed config.Load cannot distinguish an absent models: key from an
// explicit `models: {}` — both unmarshal to an empty map.
var modelsKeyRE = regexp.MustCompile(`(?m)^models[ \t]*:`)

// commandsKeyRE matches a top-level (unindented, uncommented) commands:
// key in raw config.yaml bytes, mirroring sourceGlobsKeyRE/modelsKeyRE.
var commandsKeyRE = regexp.MustCompile(`(?m)^commands[ \t]*:`)

// checkModels emits the missing-models Warn (spec 123 R6c) when
// .mindspec/config.yaml is absent, lacks the models: field, or declares
// it empty — mirroring checkSourceGlobs. The Warn discloses the
// declared-and-inert status and hints `mindspec models populate`. Fixed
// via the shared three-state scaffolder.
func checkModels(r *Report, root string) {
	cfg, err := config.Load(root)
	if err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    ".mindspec/config.yaml models",
			Status:  Warn,
			Message: "could not load .mindspec/config.yaml: " + err.Error(),
		})
		return
	}

	// FX-2 (empty≠declared): a blank-valued entry (e.g. `models:\n
	// authoring: ""`) has map-length 1 but declares no real model id, so
	// HasDeclaredModels (non-blank key+value) — not a bare len() > 0 — is
	// the completeness predicate; the missing-models Warn still fires.
	if cfg.HasDeclaredModels() {
		r.Checks = append(r.Checks, Check{
			Name:    ".mindspec/config.yaml models",
			Status:  OK,
			Message: "models declared (still declared-and-inert — see `mindspec config show`)",
		})
		return
	}

	configPath := filepath.Join(root, ".mindspec", "config.yaml")
	r.Checks = append(r.Checks, Check{
		Name:   ".mindspec/config.yaml models",
		Status: Warn,
		Message: "missing-models: models not set in .mindspec/config.yaml — " +
			builtinModelsDisclosure +
			"; run 'mindspec models populate' to emit an agent prompt that declares your own",
		FixFunc: func() error {
			return scaffoldModelsBlock(configPath)
		},
	})
}

// scaffoldModelsBlock routes the models: schema-block scaffold through
// the shared three-state scaffoldConfigBlock (PF-2's identity pin).
func scaffoldModelsBlock(configPath string) error {
	return scaffoldConfigBlock(configPath, modelsKeyRE, modelsBlock)
}

// checkCommands emits the missing-commands Warn (spec 123 R7c) when
// .mindspec/config.yaml is absent, lacks the commands: field, or
// declares it empty — mirroring checkSourceGlobs/checkModels. The Warn
// discloses why the managed AGENTS.md documents carry no Build & Test
// section and hints `mindspec commands populate`. Fixed via the shared
// three-state scaffolder.
func checkCommands(r *Report, root string) {
	cfg, err := config.Load(root)
	if err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    ".mindspec/config.yaml commands",
			Status:  Warn,
			Message: "could not load .mindspec/config.yaml: " + err.Error(),
		})
		return
	}

	// FX-2 (empty≠declared): a blank-valued entry (e.g. `commands:\n
	// build: ""`) has map-length 1 but declares no runnable command, so
	// HasDeclaredCommands (non-blank key+value) — not a bare len() > 0 —
	// is the completeness predicate; the missing-commands Warn still
	// fires and the managed Build & Test section stays omitted.
	if cfg.HasDeclaredCommands() {
		r.Checks = append(r.Checks, Check{
			Name:    ".mindspec/config.yaml commands",
			Status:  OK,
			Message: "commands declared; managed AGENTS.md documents render the Build & Test section",
		})
		return
	}

	configPath := filepath.Join(root, ".mindspec", "config.yaml")
	r.Checks = append(r.Checks, Check{
		Name:   ".mindspec/config.yaml commands",
		Status: Warn,
		Message: "missing-commands: commands not set in .mindspec/config.yaml — " +
			buildTestGuidanceDisclosure +
			"; run 'mindspec commands populate' to emit an agent prompt that declares your own",
		FixFunc: func() error {
			return scaffoldCommandsBlock(configPath)
		},
	})
}

// scaffoldCommandsBlock routes the commands: schema-block scaffold
// through the shared three-state scaffoldConfigBlock (PF-2's identity
// pin).
func scaffoldCommandsBlock(configPath string) error {
	return scaffoldConfigBlock(configPath, commandsKeyRE, commandsBlock)
}
