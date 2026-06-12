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

// scaffoldSourceGlobs writes the source_globs config block per the
// three Req 11 states (spec 091):
//
//   - file absent              → create config.yaml with exactly the block
//   - file present, no field   → APPEND the block; prior bytes unchanged
//   - file present, has field  → leave the file byte-identical (no-op)
//
// The absent-vs-present decision inspects RAW file bytes (V2-2) — the
// typed loader cannot distinguish an absent key from `source_globs: []`.
// config.ResetCache() is called after any write so a fix-then-recheck
// flow in the same process observes the new state.
func scaffoldSourceGlobs(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// State 1: file absent — create with exactly the block.
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(configPath, []byte(sourceGlobsBlock), 0o644); err != nil {
			return err
		}
		config.ResetCache()
		return nil
	}

	// State 3: field already present (raw-byte key-presence check) —
	// leave the file untouched.
	if sourceGlobsKeyRE.Match(data) {
		return nil
	}

	// State 2: file present without the field — APPEND the block,
	// preserving every prior byte. Ensure a separating newline so the
	// appended block starts on its own line.
	out := data
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, []byte(sourceGlobsBlock)...)
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return err
	}
	config.ResetCache()
	return nil
}
